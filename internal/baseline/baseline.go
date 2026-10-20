package baseline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/kyyril/perfguard/internal/model"
)

var (
	// ErrNotFound is returned when a requested baseline cannot be located.
	ErrNotFound = errors.New("baseline not found")

	// ErrInvalidRun is returned when a run lacks required fields to form a baseline key.
	ErrInvalidRun = errors.New("baseline run is missing required metadata")
)

// Key identifies a stored baseline uniquely.
type Key struct {
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	EnvHash   string `json:"env_hash"`
	CommitSHA string `json:"commit_sha"`
}

// RelativePath returns the canonical path for key: <goos>-<goarch>/<env-hash>/<commit-sha>.json
func (k Key) RelativePath() string {
	return filepath.Join(fmt.Sprintf("%s-%s", k.OS, k.Arch), k.EnvHash, fmt.Sprintf("%s.json", k.CommitSHA))
}

// String returns a human-readable identifier for the key.
func (k Key) String() string {
	return fmt.Sprintf("%s-%s/%s/%s", k.OS, k.Arch, k.EnvHash, k.CommitSHA)
}

// Info summarizes a stored baseline for listing.
type Info struct {
	Key       Key       `json:"key"`
	Path      string    `json:"path"`
	CreatedAt time.Time `json:"created_at"`
	CommitSHA string    `json:"commit_sha"`
	Ref       string    `json:"ref"`
	OS        string    `json:"os"`
	Arch      string    `json:"arch"`
	EnvHash   string    `json:"env_hash"`
}

// Filter specifies criteria for listing baselines.
type Filter struct {
	OS        string
	Arch      string
	EnvHash   string
	CommitSHA string
}

// Matches returns true if the info matches all non-empty filter fields.
func (f Filter) Matches(info Info) bool {
	if f.OS != "" && info.OS != f.OS {
		return false
	}
	if f.Arch != "" && info.Arch != f.Arch {
		return false
	}
	if f.EnvHash != "" && info.EnvHash != f.EnvHash {
		return false
	}
	if f.CommitSHA != "" && info.CommitSHA != f.CommitSHA {
		return false
	}
	return true
}

// PruneOptions configures baseline pruning.
type PruneOptions struct {
	// Keep specifies the number of newest baselines to retain.
	// If 0, retention count is not enforced.
	Keep int

	// OlderThan specifies the age cutoff. Baselines created before now - OlderThan will be deleted.
	// If 0, age pruning is not enforced.
	OlderThan time.Duration

	// PerGroup controls whether Keep applies per (OS-Arch, EnvHash) group or globally.
	PerGroup bool
}

// Store defines persistence operations for baseline benchmark runs.
type Store interface {
	Save(ctx context.Context, r model.Run) (Key, error)
	Load(ctx context.Context, key Key) (model.Run, error)
	List(ctx context.Context, filter Filter) ([]Info, error)
	Prune(ctx context.Context, opts PruneOptions) (int, error)
}

// KeyFromRun constructs a Key from a model.Run.
func KeyFromRun(r model.Run) (Key, error) {
	if r.Env.OS == "" || r.Env.Arch == "" {
		return Key{}, fmt.Errorf("resolve baseline key: %w: missing os or arch", ErrInvalidRun)
	}
	sha := strings.TrimSpace(r.Git.SHA)
	if sha == "" {
		sha = strings.TrimSpace(r.ID)
	}
	if sha == "" {
		return Key{}, fmt.Errorf("resolve baseline key: %w: missing git sha and run id", ErrInvalidRun)
	}

	envHash := ComputeEnvHash(r.Env, r.Params)

	return Key{
		OS:        r.Env.OS,
		Arch:      r.Env.Arch,
		EnvHash:   envHash,
		CommitSHA: sha,
	}, nil
}

// ComputeEnvHash calculates a deterministic 12-character hex hash from environment and benchmark run parameters.
func ComputeEnvHash(env model.Environment, params model.RunParams) string {
	minor := parseGoMinorVersion(env.GoVersion)
	excludeJoined := strings.Join(params.Exclude, ",")
	payload := fmt.Sprintf("cpu:%s|gomaxprocs:%d|go:%s|count:%d|time:%s|cpu_flag:%d|inc:%s|exc:%s",
		env.CPUModel, env.GOMAXPROCS, minor,
		params.Count, params.Benchtime, params.CPU, params.Include, excludeJoined,
	)
	h := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(h[:])[:12]
}

func parseGoMinorVersion(v string) string {
	v = strings.TrimPrefix(v, "go")
	parts := strings.Split(v, ".")
	if len(parts) >= 2 {
		return parts[0] + "." + parts[1]
	}
	return v
}
