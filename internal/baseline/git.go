package baseline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kyyril/perfguard/internal/model"
)

// GitClient defines Git operations needed for baseline storage on orphan branches.
type GitClient interface {
	ReadBranchFile(ctx context.Context, branch, path string) ([]byte, error)
	ListBranchFiles(ctx context.Context, branch string) ([]string, error)
	CommitBranchFiles(ctx context.Context, branch, commitMsg string, addFiles map[string][]byte, removePaths []string) error
}

// GitOption configures a GitStore.
type GitOption func(*GitStore)

// WithGitNowFunc sets a custom clock function for timestamping and pruning in GitStore.
func WithGitNowFunc(fn func() time.Time) GitOption {
	return func(s *GitStore) {
		if fn != nil {
			s.nowFunc = fn
		}
	}
}

// GitStore implements Store backed by an orphan or data branch in Git.
type GitStore struct {
	client  GitClient
	branch  string
	nowFunc func() time.Time
}

// NewGitStore creates a baseline store persisting on the specified Git branch.
func NewGitStore(client GitClient, branch string, opts ...GitOption) *GitStore {
	store := &GitStore{
		client:  client,
		branch:  branch,
		nowFunc: time.Now,
	}
	for _, opt := range opts {
		opt(store)
	}
	return store
}

// Save writes a run directly to the Git branch.
func (s *GitStore) Save(ctx context.Context, r model.Run) (Key, error) {
	if err := ctx.Err(); err != nil {
		return Key{}, fmt.Errorf("baseline git save: %w", err)
	}

	key, err := KeyFromRun(r)
	if err != nil {
		return Key{}, fmt.Errorf("baseline git save: %w", err)
	}

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return Key{}, fmt.Errorf("baseline git save: marshal run json: %w", err)
	}

	relPath := key.RelativePath()
	addFiles := map[string][]byte{
		relPath: data,
	}

	commitMsg := fmt.Sprintf("save baseline %s", key.CommitSHA)
	if err := s.client.CommitBranchFiles(ctx, s.branch, commitMsg, addFiles, nil); err != nil {
		return Key{}, fmt.Errorf("baseline git save: commit files: %w", err)
	}

	return key, nil
}

// Load reads and parses a run from the Git branch for the given key.
func (s *GitStore) Load(ctx context.Context, key Key) (model.Run, error) {
	if err := ctx.Err(); err != nil {
		return model.Run{}, fmt.Errorf("baseline git load: %w", err)
	}

	relPath := key.RelativePath()
	data, err := s.client.ReadBranchFile(ctx, s.branch, relPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.Run{}, fmt.Errorf("baseline git load %s: %w", relPath, ErrNotFound)
		}
		return model.Run{}, fmt.Errorf("baseline git load %s: %w", relPath, err)
	}

	var r model.Run
	if err := json.Unmarshal(data, &r); err != nil {
		return model.Run{}, fmt.Errorf("baseline git load %s: unmarshal json: %w", relPath, err)
	}

	return r, nil
}

// List scans all baseline JSON files on the Git branch and returns matched metadata.
func (s *GitStore) List(ctx context.Context, filter Filter) ([]Info, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("baseline git list: %w", err)
	}

	files, err := s.client.ListBranchFiles(ctx, s.branch)
	if err != nil {
		return nil, fmt.Errorf("baseline git list: %w", err)
	}

	var results []Info

	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !strings.HasSuffix(file, ".json") {
			continue
		}

		parts := strings.Split(filepath.ToSlash(file), "/")
		if len(parts) != 3 {
			continue
		}

		osArch := parts[0]
		envHash := parts[1]
		fileName := parts[2]
		commitSHA := strings.TrimSuffix(fileName, ".json")

		osArchParts := strings.SplitN(osArch, "-", 2)
		var itemOS, itemArch string
		if len(osArchParts) == 2 {
			itemOS = osArchParts[0]
			itemArch = osArchParts[1]
		}

		// Quick check before reading contents
		prelim := Info{
			OS:        itemOS,
			Arch:      itemArch,
			EnvHash:   envHash,
			CommitSHA: commitSHA,
		}
		if !filter.Matches(prelim) {
			continue
		}

		data, err := s.client.ReadBranchFile(ctx, s.branch, file)
		if err != nil {
			continue
		}

		var partial struct {
			CreatedAt time.Time `json:"created_at"`
			Git       struct {
				SHA string `json:"sha"`
				Ref string `json:"ref"`
			} `json:"git"`
			Env struct {
				OS   string `json:"os"`
				Arch string `json:"arch"`
			} `json:"env"`
		}
		if err := json.Unmarshal(data, &partial); err != nil {
			continue
		}

		if partial.Env.OS != "" {
			itemOS = partial.Env.OS
		}
		if partial.Env.Arch != "" {
			itemArch = partial.Env.Arch
		}
		if partial.Git.SHA != "" {
			commitSHA = partial.Git.SHA
		}

		entry := Info{
			Key: Key{
				OS:        itemOS,
				Arch:      itemArch,
				EnvHash:   envHash,
				CommitSHA: commitSHA,
			},
			Path:      file,
			CreatedAt: partial.CreatedAt,
			CommitSHA: commitSHA,
			Ref:       partial.Git.Ref,
			OS:        itemOS,
			Arch:      itemArch,
			EnvHash:   envHash,
		}

		if filter.Matches(entry) {
			results = append(results, entry)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})

	return results, nil
}

// Prune removes old baselines on the Git branch exceeding retention limits.
func (s *GitStore) Prune(ctx context.Context, opts PruneOptions) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, fmt.Errorf("baseline git prune: %w", err)
	}

	if opts.Keep <= 0 && opts.OlderThan <= 0 {
		return 0, nil
	}

	all, err := s.List(ctx, Filter{})
	if err != nil {
		return 0, fmt.Errorf("baseline git prune list: %w", err)
	}

	var toDelete = make(map[string]Info)
	now := s.nowFunc()

	// 1. Check age cutoff
	if opts.OlderThan > 0 {
		cutoff := now.Add(-opts.OlderThan)
		for _, item := range all {
			if item.CreatedAt.Before(cutoff) {
				toDelete[item.Path] = item
			}
		}
	}

	// 2. Check keep count
	if opts.Keep > 0 {
		if opts.PerGroup {
			groups := make(map[string][]Info)
			for _, item := range all {
				grp := fmt.Sprintf("%s-%s/%s", item.OS, item.Arch, item.EnvHash)
				groups[grp] = append(groups[grp], item)
			}

			for _, items := range groups {
				if len(items) > opts.Keep {
					for _, item := range items[opts.Keep:] {
						toDelete[item.Path] = item
					}
				}
			}
		} else if len(all) > opts.Keep {
			for _, item := range all[opts.Keep:] {
				toDelete[item.Path] = item
			}
		}
	}

	if len(toDelete) == 0 {
		return 0, nil
	}

	var removePaths []string
	for path := range toDelete {
		removePaths = append(removePaths, path)
	}

	if err := s.client.CommitBranchFiles(ctx, s.branch, "prune baselines", nil, removePaths); err != nil {
		return 0, fmt.Errorf("baseline git prune commit: %w", err)
	}

	return len(removePaths), nil
}
