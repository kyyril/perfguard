package gitx

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// InitRepo initializes a new Git repository in the client directory.
// Configuration is set explicitly so commits are deterministic on any machine.
func (c *Client) InitRepo(ctx context.Context) error {
	if _, err := c.runGit(ctx, "init"); err != nil {
		return fmt.Errorf("gitx: init repository: %w", err)
	}

	settings := [][2]string{
		{"core.autocrlf", "false"},
		{"commit.gpgsign", "false"},
		{"user.name", "PerfGuard Eval"},
		{"user.email", "eval@perfguard.local"},
	}
	for _, kv := range settings {
		if _, err := c.runGit(ctx, "config", kv[0], kv[1]); err != nil {
			return fmt.Errorf("gitx: set %s: %w", kv[0], err)
		}
	}
	return nil
}

// CommitAll stages all changes and commits them. If the working tree has
// nothing to commit, CommitAll succeeds without creating a commit.
func (c *Client) CommitAll(ctx context.Context, message string) error {
	if _, err := c.runGit(ctx, "add", "-A"); err != nil {
		return fmt.Errorf("gitx: stage all changes: %w", err)
	}

	status, err := c.runGit(ctx, "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("gitx: check status: %w", err)
	}
	if strings.TrimSpace(status) == "" {
		return nil
	}

	if _, err := c.runGit(ctx, "commit", "-m", message); err != nil {
		return fmt.Errorf("gitx: commit: %w", err)
	}
	return nil
}

// ApplyPatch applies a unified diff file to the working tree. An empty patch
// is accepted and does nothing.
func (c *Client) ApplyPatch(ctx context.Context, patchPath string) error {
	info, err := os.Stat(patchPath)
	if err != nil {
		return fmt.Errorf("gitx: stat patch %s: %w", patchPath, err)
	}
	if info.Size() == 0 {
		return nil
	}

	if _, err := c.runGit(ctx, "apply", "--whitespace=nowarn", patchPath); err != nil {
		return fmt.Errorf("gitx: apply patch %s: %w", patchPath, err)
	}
	return nil
}
