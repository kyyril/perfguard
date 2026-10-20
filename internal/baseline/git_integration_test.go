//go:build integration

package baseline_test

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/kyyril/perfguard/internal/baseline"
	"github.com/kyyril/perfguard/internal/gitx"
	"github.com/kyyril/perfguard/internal/model"
)

func runGitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\nOutput: %s", strings.Join(args, " "), err, string(out))
	}
	return strings.TrimSpace(string(out))
}

func TestGitStore_Integration_OrphanBranchPushFree(t *testing.T) {
	repoDir := t.TempDir()

	// 1. Initialize local git repository
	runGitCmd(t, repoDir, "init")
	runGitCmd(t, repoDir, "config", "user.name", "PerfGuard Test")
	runGitCmd(t, repoDir, "config", "user.email", "test@example.com")
	runGitCmd(t, repoDir, "config", "commit.gpgsign", "false")
	runGitCmd(t, repoDir, "config", "core.autocrlf", "false")

	// Create an initial commit on main
	runGitCmd(t, repoDir, "commit", "--allow-empty", "-m", "initial main commit")

	client := gitx.New(repoDir)
	branchName := "perfguard-data"
	now := time.Date(2026, 10, 20, 14, 0, 0, 0, time.UTC)
	store := baseline.NewGitStore(client, branchName, baseline.WithGitNowFunc(func() time.Time { return now }))
	ctx := context.Background()

	// 2. Save baseline 1
	run1 := model.Run{
		SchemaVersion: model.CurrentSchemaVersion,
		ID:            "run-sha1",
		Side:          model.SideBase,
		CreatedAt:     now.Add(-2 * time.Hour),
		Git: model.GitInfo{
			SHA: "1111111111111111111111111111111111111111",
			Ref: "refs/heads/main",
		},
		Env: model.Environment{
			OS:         "linux",
			Arch:       "amd64",
			GoVersion:  "go1.24.1",
			CPUModel:   "AMD EPYC",
			GOMAXPROCS: 16,
		},
		Params: model.RunParams{
			Count:     10,
			Benchtime: "500ms",
			CPU:       16,
			Include:   "BenchmarkCompute",
			Mode:      "stored",
		},
		Benchmarks: []model.BenchmarkResult{
			{
				Package: "example.com/math",
				Name:    "BenchmarkCompute",
				Procs:   16,
				Samples: []model.Sample{
					{
						Iterations: 5000,
						Values:     map[string]float64{"sec/op": 0.0005},
					},
				},
			},
		},
	}

	key1, err := store.Save(ctx, run1)
	if err != nil {
		t.Fatalf("failed to save baseline to git store: %v", err)
	}

	// 3. Verify orphan branch was created push-free in local repository
	branches := runGitCmd(t, repoDir, "branch", "--list")
	if !strings.Contains(branches, branchName) {
		t.Fatalf("expected orphan branch %s to exist in local branches: %s", branchName, branches)
	}

	// Verify working directory is unaffected (still on main, clean)
	currentBranch := runGitCmd(t, repoDir, "rev-parse", "--abbrev-ref", "HEAD")
	if currentBranch != "main" && currentBranch != "master" {
		t.Errorf("expected working directory to stay on main branch, got: %s", currentBranch)
	}

	// 4. Verify data is reloadable
	loaded1, err := store.Load(ctx, key1)
	if err != nil {
		t.Fatalf("failed to load baseline from git store: %v", err)
	}
	if loaded1.Git.SHA != run1.Git.SHA {
		t.Errorf("expected SHA %s, got %s", run1.Git.SHA, loaded1.Git.SHA)
	}
	if loaded1.Env.CPUModel != run1.Env.CPUModel {
		t.Errorf("expected CPUModel %s, got %s", run1.Env.CPUModel, loaded1.Env.CPUModel)
	}
	if len(loaded1.Benchmarks) != 1 || loaded1.Benchmarks[0].Name != "BenchmarkCompute" {
		t.Errorf("unexpected benchmarks: %+v", loaded1.Benchmarks)
	}

	// 5. Save baseline 2
	run2 := run1
	run2.ID = "run-sha2"
	run2.Git.SHA = "2222222222222222222222222222222222222222"
	run2.CreatedAt = now.Add(-1 * time.Hour)

	key2, err := store.Save(ctx, run2)
	if err != nil {
		t.Fatalf("failed to save run2: %v", err)
	}

	// 6. Verify list lists both
	listItems, err := store.List(ctx, baseline.Filter{})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(listItems) != 2 {
		t.Fatalf("expected 2 items, got %d", len(listItems))
	}
	if listItems[0].CommitSHA != run2.Git.SHA || listItems[1].CommitSHA != run1.Git.SHA {
		t.Errorf("unexpected list order: %s, %s", listItems[0].CommitSHA, listItems[1].CommitSHA)
	}

	// 7. Prune keeping only the newest 1
	deleted, err := store.Prune(ctx, baseline.PruneOptions{Keep: 1})
	if err != nil {
		t.Fatalf("prune failed: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 baseline pruned, got %d", deleted)
	}

	// 8. Verify run2 is still loaded, run1 is not found
	if _, err := store.Load(ctx, key2); err != nil {
		t.Errorf("expected run2 to remain loaded: %v", err)
	}
	if _, err := store.Load(ctx, key1); !errors.Is(err, baseline.ErrNotFound) {
		t.Errorf("expected ErrNotFound for pruned run1, got: %v", err)
	}
}
