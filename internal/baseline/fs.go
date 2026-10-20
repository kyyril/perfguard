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

// FSOption configures an FSStore.
type FSOption func(*FSStore)

// WithNowFunc sets a custom clock function for timestamping and pruning.
func WithNowFunc(fn func() time.Time) FSOption {
	return func(s *FSStore) {
		if fn != nil {
			s.nowFunc = fn
		}
	}
}

// FSStore implements Store on the local filesystem.
type FSStore struct {
	rootDir string
	nowFunc func() time.Time
}

// NewFSStore creates a filesystem-backed baseline store rooted at rootDir.
func NewFSStore(rootDir string, opts ...FSOption) *FSStore {
	store := &FSStore{
		rootDir: rootDir,
		nowFunc: time.Now,
	}
	for _, opt := range opts {
		opt(store)
	}
	return store
}

// Save writes a run to the filesystem atomically using a temporary file and rename.
func (s *FSStore) Save(ctx context.Context, r model.Run) (Key, error) {
	if err := ctx.Err(); err != nil {
		return Key{}, fmt.Errorf("baseline save: %w", err)
	}

	key, err := KeyFromRun(r)
	if err != nil {
		return Key{}, fmt.Errorf("baseline save: %w", err)
	}

	targetDir := filepath.Join(s.rootDir, fmt.Sprintf("%s-%s", key.OS, key.Arch), key.EnvHash)
	if err := os.MkdirAll(targetDir, 0o750); err != nil {
		return Key{}, fmt.Errorf("baseline save: create directory %s: %w", targetDir, err)
	}

	// Create temporary file in the target directory to ensure same filesystem for atomic rename
	tmpFile, err := os.CreateTemp(targetDir, "baseline-*.tmp")
	if err != nil {
		return Key{}, fmt.Errorf("baseline save: create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	cleanup := true
	defer func() {
		if cleanup {
			_ = tmpFile.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return Key{}, fmt.Errorf("baseline save: marshal run json: %w", err)
	}

	if _, err := tmpFile.Write(data); err != nil {
		return Key{}, fmt.Errorf("baseline save: write data: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return Key{}, fmt.Errorf("baseline save: close temp file: %w", err)
	}

	finalPath := filepath.Join(targetDir, fmt.Sprintf("%s.json", key.CommitSHA))
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return Key{}, fmt.Errorf("baseline save: rename %s to %s: %w", tmpPath, finalPath, err)
	}

	cleanup = false
	return key, nil
}

// Load reads and parses a run from the filesystem for the given key.
func (s *FSStore) Load(ctx context.Context, key Key) (model.Run, error) {
	if err := ctx.Err(); err != nil {
		return model.Run{}, fmt.Errorf("baseline load: %w", err)
	}

	relPath := key.RelativePath()
	targetPath := filepath.Join(s.rootDir, relPath)

	data, err := os.ReadFile(targetPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.Run{}, fmt.Errorf("baseline load %s: %w", relPath, ErrNotFound)
		}
		return model.Run{}, fmt.Errorf("baseline load %s: %w", relPath, err)
	}

	var r model.Run
	if err := json.Unmarshal(data, &r); err != nil {
		return model.Run{}, fmt.Errorf("baseline load %s: unmarshal json: %w", relPath, err)
	}

	return r, nil
}

// List scans the baseline directory and returns summary information matching the filter.
func (s *FSStore) List(ctx context.Context, filter Filter) ([]Info, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("baseline list: %w", err)
	}

	if _, err := os.Stat(s.rootDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Info{}, nil
		}
		return nil, fmt.Errorf("baseline list: stat %s: %w", s.rootDir, err)
	}

	var results []Info

	err := filepath.Walk(s.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".json") {
			return nil
		}

		rel, err := filepath.Rel(s.rootDir, path)
		if err != nil {
			return err
		}

		// Expected path layout: <os-arch>/<env-hash>/<commit-sha>.json
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) != 3 {
			return nil
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

		// Parse metadata from file
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
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
			return nil
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
			Path:      path,
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

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("baseline list walk: %w", err)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})

	return results, nil
}

// Prune deletes baselines exceeding retention limits or age thresholds.
func (s *FSStore) Prune(ctx context.Context, opts PruneOptions) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, fmt.Errorf("baseline prune: %w", err)
	}

	if opts.Keep <= 0 && opts.OlderThan <= 0 {
		return 0, nil
	}

	all, err := s.List(ctx, Filter{})
	if err != nil {
		return 0, fmt.Errorf("baseline prune list: %w", err)
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
			// Group by OS-Arch / EnvHash
			groups := make(map[string][]Info)
			for _, item := range all {
				grp := fmt.Sprintf("%s-%s/%s", item.OS, item.Arch, item.EnvHash)
				groups[grp] = append(groups[grp], item)
			}

			for _, items := range groups {
				// Items are already sorted newest first
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

	deletedCount := 0
	for path := range toDelete {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return deletedCount, fmt.Errorf("baseline prune remove %s: %w", path, err)
		}
		deletedCount++
		// Clean up parent directory if empty
		parent := filepath.Dir(path)
		_ = os.Remove(parent)
		_ = os.Remove(filepath.Dir(parent))
	}

	return deletedCount, nil
}
