//go:build integration

package gitx_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kyyril/perfguard/internal/gitx"
)

func runGitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %s failed: %v\nOutput: %s", strings.Join(args, " "), dir, err, string(out))
	}
	return strings.TrimSpace(string(out))
}

func setupTestRepo(t *testing.T) (string, gitx.Commit, gitx.Commit) {
	t.Helper()
	repoDir := t.TempDir()

	runGitCmd(t, repoDir, "init")
	runGitCmd(t, repoDir, "config", "user.email", "perfguard-test@example.com")
	runGitCmd(t, repoDir, "config", "user.name", "PerfGuard Tester")
	runGitCmd(t, repoDir, "config", "commit.gpgsign", "false")
	runGitCmd(t, repoDir, "config", "core.autocrlf", "false")

	// Commit 1: Initial file
	file1 := filepath.Join(repoDir, "file1.txt")
	if err := os.WriteFile(file1, []byte("version 1\n"), 0o600); err != nil {
		t.Fatalf("failed to write file1: %v", err)
	}
	runGitCmd(t, repoDir, "add", "file1.txt")
	runGitCmd(t, repoDir, "commit", "-m", "initial commit")
	sha1 := runGitCmd(t, repoDir, "rev-parse", "HEAD")

	// Commit 2: Update file1 and add file2
	if err := os.WriteFile(file1, []byte("version 2\n"), 0o600); err != nil {
		t.Fatalf("failed to update file1: %v", err)
	}
	file2 := filepath.Join(repoDir, "file2.txt")
	if err := os.WriteFile(file2, []byte("new file\n"), 0o600); err != nil {
		t.Fatalf("failed to write file2: %v", err)
	}
	runGitCmd(t, repoDir, "add", "file1.txt", "file2.txt")
	runGitCmd(t, repoDir, "commit", "-m", "second commit")
	sha2 := runGitCmd(t, repoDir, "rev-parse", "HEAD")

	c1 := gitx.Commit{SHA: sha1, Ref: "HEAD~1"}
	c2 := gitx.Commit{SHA: sha2, Ref: "HEAD"}
	return repoDir, c1, c2
}

func TestIntegrationGitOperations(t *testing.T) {
	ctx := context.Background()
	repoDir, c1, c2 := setupTestRepo(t)
	client := gitx.New(repoDir)

	// 1. Resolve refs
	t.Run("Resolve", func(t *testing.T) {
		res1, err := client.Resolve(ctx, "HEAD~1")
		if err != nil {
			t.Fatalf("failed to resolve HEAD~1: %v", err)
		}
		if res1.SHA != c1.SHA {
			t.Errorf("expected SHA %s, got %s", c1.SHA, res1.SHA)
		}

		res2, err := client.Resolve(ctx, "HEAD")
		if err != nil {
			t.Fatalf("failed to resolve HEAD: %v", err)
		}
		if res2.SHA != c2.SHA {
			t.Errorf("expected SHA %s, got %s", c2.SHA, res2.SHA)
		}

		_, err = client.Resolve(ctx, "nonexistent-branch-xyz")
		if err == nil {
			t.Fatal("expected error for nonexistent ref, got nil")
		}
		if !errors.Is(err, gitx.ErrRefNotFound) {
			t.Errorf("expected ErrRefNotFound, got %v", err)
		}
	})

	// 2. Changed files
	t.Run("ChangedFiles", func(t *testing.T) {
		files, err := client.ChangedFiles(ctx, c1, c2)
		if err != nil {
			t.Fatalf("failed to get changed files: %v", err)
		}
		if len(files) != 2 {
			t.Fatalf("expected 2 changed files, got %d: %v", len(files), files)
		}

		sameFiles, err := client.ChangedFiles(ctx, c2, c2)
		if err != nil {
			t.Fatalf("unexpected error for identical commits: %v", err)
		}
		if len(sameFiles) != 0 {
			t.Errorf("expected 0 changed files for identical commits, got %d", len(sameFiles))
		}
	})

	// 3. Worktree creation and cleanup
	t.Run("WorktreeLifecycle", func(t *testing.T) {
		wt, err := client.AddWorktree(ctx, c1)
		if err != nil {
			t.Fatalf("failed to add worktree: %v", err)
		}

		// Verify worktree has version 1 of file1.txt
		wtFile1 := filepath.Join(wt.Path, "file1.txt")
		content, err := os.ReadFile(wtFile1)
		if err != nil {
			t.Fatalf("failed to read file in worktree: %v", err)
		}
		if strings.TrimSpace(string(content)) != "version 1" {
			t.Errorf("expected 'version 1', got %q", string(content))
		}

		// Clean up worktree
		if err := wt.Remove(ctx); err != nil {
			t.Fatalf("failed to remove worktree: %v", err)
		}

		// Verify directory is deleted
		if _, err := os.Stat(wt.Path); !os.IsNotExist(err) {
			t.Errorf("expected worktree path %s to be removed, but stat returned err: %v", wt.Path, err)
		}
	})

	// 4. CurrentInfo
	t.Run("CurrentInfo", func(t *testing.T) {
		info, err := client.CurrentInfo(ctx)
		if err != nil {
			t.Fatalf("failed to get current info: %v", err)
		}
		if info.SHA != c2.SHA {
			t.Errorf("expected SHA %s, got %s", c2.SHA, info.SHA)
		}
		if info.Dirty {
			t.Error("expected clean working tree, got dirty")
		}

		// Make working tree dirty
		dirtyFile := filepath.Join(repoDir, "dirty.txt")
		if err := os.WriteFile(dirtyFile, []byte("dirty content\n"), 0o600); err != nil {
			t.Fatalf("failed to write dirty file: %v", err)
		}

		infoDirty, err := client.CurrentInfo(ctx)
		if err != nil {
			t.Fatalf("failed to get dirty current info: %v", err)
		}
		if !infoDirty.Dirty {
			t.Error("expected dirty working tree, got clean")
		}
	})
}

func TestIntegrationShallowCloneDetection(t *testing.T) {
	ctx := context.Background()
	sourceDir, _, _ := setupTestRepo(t)

	// Create shallow clone with depth 1
	shallowDir := t.TempDir()
	runGitCmd(t, shallowDir, "clone", "--depth=1", "file://"+filepath.ToSlash(sourceDir), ".")

	client := gitx.New(shallowDir)

	isShallow, err := client.IsShallow(ctx)
	if err != nil {
		t.Fatalf("failed to check shallow: %v", err)
	}
	if !isShallow {
		t.Fatal("expected isShallow to be true for shallow clone")
	}

	// In shallow clone with depth 1, HEAD~1 does not exist
	_, err = client.Resolve(ctx, "HEAD~1")
	if err == nil {
		t.Fatal("expected error resolving unavailable commit in shallow clone, got nil")
	}
	if !errors.Is(err, gitx.ErrShallowClone) {
		t.Errorf("expected ErrShallowClone, got %v", err)
	}
	if !strings.Contains(err.Error(), "fetch-depth: 0") {
		t.Errorf("expected message suggesting 'fetch-depth: 0', got %v", err)
	}
}
