package gitx

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ReadBranchFile reads the contents of a file on the given branch ref without checking out.
func (c *Client) ReadBranchFile(ctx context.Context, branch, path string) ([]byte, error) {
	refPath := fmt.Sprintf("refs/heads/%s:%s", branch, filepath.ToSlash(path))
	cmd := exec.CommandContext(ctx, "git", "show", refPath)
	cmd.Dir = c.dir
	cmd.Env = os.Environ()
	cmd.WaitDelay = 5 * time.Second

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if strings.Contains(msg, "does not exist") || strings.Contains(msg, "exists on disk, but not in") || strings.Contains(msg, "fatal: path") || strings.Contains(msg, "fatal: invalid object name") {
			return nil, fmt.Errorf("%w: %s on branch %s", os.ErrNotExist, path, branch)
		}
		return nil, fmt.Errorf("git show %s: %w: %s", refPath, err, msg)
	}

	return stdout.Bytes(), nil
}

// ListBranchFiles lists all file paths on the given branch.
func (c *Client) ListBranchFiles(ctx context.Context, branch string) ([]string, error) {
	ref := fmt.Sprintf("refs/heads/%s", branch)
	out, err := c.runGit(ctx, "ls-tree", "-r", "--name-only", ref)
	if err != nil {
		if strings.Contains(err.Error(), "Not a valid object name") || strings.Contains(err.Error(), "fatal: not a valid object name") {
			return nil, nil
		}
		return nil, fmt.Errorf("gitx: list branch files: %w", err)
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

// CommitBranchFiles commits additions and removals to an orphan or existing branch.
func (c *Client) CommitBranchFiles(ctx context.Context, branch, commitMsg string, addFiles map[string][]byte, removePaths []string) error {
	if err := c.ensureBranchExists(ctx, branch); err != nil {
		return fmt.Errorf("gitx: ensure branch %s exists: %w", branch, err)
	}

	tmpDir, err := os.MkdirTemp("", "perfguard-branch-*")
	if err != nil {
		return fmt.Errorf("gitx: allocate temp dir for branch: %w", err)
	}
	_ = os.Remove(tmpDir)

	if _, err := c.runGit(ctx, "worktree", "add", tmpDir, branch); err != nil {
		_ = os.RemoveAll(tmpDir)
		return fmt.Errorf("gitx: create worktree for branch %s: %w", branch, err)
	}

	defer func() {
		// Clean up worktree independently with a background context
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = c.runGit(cleanupCtx, "worktree", "remove", "--force", tmpDir)
		_ = os.RemoveAll(tmpDir)
	}()

	// 1. Add new or updated files
	for relPath, content := range addFiles {
		destPath := filepath.Join(tmpDir, filepath.FromSlash(relPath))
		if err := os.MkdirAll(filepath.Dir(destPath), 0o750); err != nil {
			return fmt.Errorf("gitx: create directory for %s: %w", relPath, err)
		}
		if err := os.WriteFile(destPath, content, 0o600); err != nil {
			return fmt.Errorf("gitx: write file %s: %w", relPath, err)
		}
		if _, err := c.runGitInDir(ctx, tmpDir, "add", filepath.FromSlash(relPath)); err != nil {
			return fmt.Errorf("gitx: stage file %s: %w", relPath, err)
		}
	}

	// 2. Remove specified paths
	for _, relPath := range removePaths {
		if _, err := c.runGitInDir(ctx, tmpDir, "rm", "-f", "--ignore-unmatch", filepath.FromSlash(relPath)); err != nil {
			return fmt.Errorf("gitx: remove file %s: %w", relPath, err)
		}
	}

	// 3. Check status
	status, err := c.runGitInDir(ctx, tmpDir, "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("gitx: check worktree status: %w", err)
	}
	if len(strings.TrimSpace(status)) == 0 {
		return nil
	}

	// 4. Commit changes with fallback identity
	if _, err := c.runGitInDir(ctx, tmpDir, "-c", "user.name=PerfGuard", "-c", "user.email=perfguard@localhost", "commit", "-m", commitMsg); err != nil {
		return fmt.Errorf("gitx: commit to branch %s: %w", branch, err)
	}

	return nil
}

func (c *Client) ensureBranchExists(ctx context.Context, branch string) error {
	ref := fmt.Sprintf("refs/heads/%s", branch)
	if _, err := c.runGit(ctx, "rev-parse", "--verify", ref); err == nil {
		return nil
	}

	// Compute empty tree hash dynamically
	cmd := exec.CommandContext(ctx, "git", "mktree")
	cmd.Dir = c.dir
	cmd.Env = os.Environ()
	cmd.Stdin = bytes.NewReader(nil)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	var emptyTree string
	if err := cmd.Run(); err == nil && len(strings.TrimSpace(stdout.String())) > 0 {
		emptyTree = strings.TrimSpace(stdout.String())
	} else {
		emptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	}

	initCommit, err := c.runGit(ctx, "-c", "user.name=PerfGuard", "-c", "user.email=perfguard@localhost", "commit-tree", emptyTree, "-m", "init "+branch)
	if err != nil {
		return fmt.Errorf("create empty commit for branch %s: %w", branch, err)
	}

	if _, err := c.runGit(ctx, "update-ref", ref, initCommit); err != nil {
		return fmt.Errorf("update-ref for branch %s: %w", branch, err)
	}

	return nil
}

func (c *Client) runGitInDir(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
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
