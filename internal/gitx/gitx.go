// Package gitx provides Git operations for PerfGuard, including ref resolution,
// temporary worktrees, changed file inspection, and shallow clone detection.
package gitx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/kyyril/perfguard/internal/model"
)

// Sentinel errors returned by gitx operations.
var (
	ErrRefNotFound    = errors.New("git ref not found")
	ErrShallowClone   = errors.New("shallow clone detected and base ref is unavailable")
	ErrBaseEqualsHead = errors.New("base ref equals head ref")
)

// Commit represents a resolved Git commit.
type Commit struct {
	SHA string `json:"sha"`
	Ref string `json:"ref"`
}

// Worktree represents an isolated Git worktree for measuring a commit.
type Worktree struct {
	client *Client
	Path   string
	Commit Commit
}

// VCS defines Git operations needed by PerfGuard.
type VCS interface {
	Resolve(ctx context.Context, ref string) (Commit, error)
	AddWorktree(ctx context.Context, c Commit) (*Worktree, error)
	ChangedFiles(ctx context.Context, base, head Commit) ([]string, error)
	CurrentInfo(ctx context.Context) (model.GitInfo, error)
	IsShallow(ctx context.Context) (bool, error)
}

// Client implements VCS using Git CLI commands.
type Client struct {
	dir string
}

// New creates a new Git client for the given working directory.
func New(dir string) *Client {
	return &Client{dir: dir}
}

// Resolve translates a ref (branch, tag, SHA, HEAD) into a concrete Commit.
func (c *Client) Resolve(ctx context.Context, ref string) (Commit, error) {
	if strings.TrimSpace(ref) == "" {
		return Commit{}, errors.New("gitx: ref must not be empty")
	}

	out, err := c.runGit(ctx, "rev-parse", "--verify", ref+"^{commit}")
	if err != nil {
		if isShallow, _ := c.IsShallow(ctx); isShallow {
			return Commit{}, fmt.Errorf("%w: base ref %q is unavailable; please set fetch-depth: 0 in your checkout action",
				ErrShallowClone, ref)
		}
		return Commit{}, fmt.Errorf("%w: %q: %w", ErrRefNotFound, ref, err)
	}

	return Commit{
		SHA: out,
		Ref: ref,
	}, nil
}

// IsShallow returns true if the repository is a shallow clone.
func (c *Client) IsShallow(ctx context.Context) (bool, error) {
	out, err := c.runGit(ctx, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return false, fmt.Errorf("gitx: failed to check shallow status: %w", err)
	}
	return out == "true", nil
}

// AddWorktree creates a temporary detached worktree at the given commit.
func (c *Client) AddWorktree(ctx context.Context, commit Commit) (*Worktree, error) {
	tmpDir, err := os.MkdirTemp("", "perfguard-worktree-*")
	if err != nil {
		return nil, fmt.Errorf("gitx: failed to allocate temporary directory: %w", err)
	}

	// git worktree add requires the destination directory to not exist yet.
	if err := os.Remove(tmpDir); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("gitx: failed to clear directory placeholder %s: %w", tmpDir, err)
	}

	if _, err := c.runGit(ctx, "worktree", "add", "--detach", tmpDir, commit.SHA); err != nil {
		_ = os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("gitx: failed to create worktree for %s: %w", commit.SHA, err)
	}

	return &Worktree{
		client: c,
		Path:   tmpDir,
		Commit: commit,
	}, nil
}

// Remove tears down the worktree and cleans up its directory from disk.
func (w *Worktree) Remove(ctx context.Context) error {
	var firstErr error
	if _, err := w.client.runGit(ctx, "worktree", "remove", "--force", w.Path); err != nil {
		firstErr = fmt.Errorf("gitx: failed to remove worktree: %w", err)
	}
	if err := os.RemoveAll(w.Path); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("gitx: failed to delete directory %s: %w", w.Path, err)
	}
	return firstErr
}

// Location returns the filesystem path of the temporary worktree.
func (w *Worktree) Location() string {
	return w.Path
}

// ChangedFiles returns the list of file paths modified between base and head.
func (c *Client) ChangedFiles(ctx context.Context, base, head Commit) ([]string, error) {
	if base.SHA == head.SHA {
		return nil, nil
	}

	out, err := c.runGit(ctx, "diff", "--name-only", base.SHA, head.SHA)
	if err != nil {
		return nil, fmt.Errorf("gitx: failed to list changed files: %w", err)
	}

	if out == "" {
		return nil, nil
	}

	lines := strings.Split(out, "\n")
	var files []string
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" {
			files = append(files, trimmed)
		}
	}
	return files, nil
}

// CurrentInfo returns commit SHA, branch ref, and dirty state of the working tree.
func (c *Client) CurrentInfo(ctx context.Context) (model.GitInfo, error) {
	sha, err := c.runGit(ctx, "rev-parse", "HEAD")
	if err != nil {
		return model.GitInfo{}, fmt.Errorf("gitx: failed to get HEAD SHA: %w", err)
	}

	ref, _ := c.runGit(ctx, "rev-parse", "--abbrev-ref", "HEAD")

	status, err := c.runGit(ctx, "status", "--porcelain")
	if err != nil {
		return model.GitInfo{}, fmt.Errorf("gitx: failed to check dirty status: %w", err)
	}

	return model.GitInfo{
		SHA:   sha,
		Ref:   ref,
		Dirty: len(strings.TrimSpace(status)) > 0,
	}, nil
}

func (c *Client) runGit(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = c.dir
	cmd.Env = os.Environ()
	cmd.WaitDelay = 5 * time.Second

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, msg)
	}

	return strings.TrimSpace(stdout.String()), nil
}
