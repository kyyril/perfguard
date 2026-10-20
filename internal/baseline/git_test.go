package baseline

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type fakeGitClient struct {
	files map[string]map[string][]byte // branch -> path -> content
}

func newFakeGitClient() *fakeGitClient {
	return &fakeGitClient{
		files: make(map[string]map[string][]byte),
	}
}

func (f *fakeGitClient) ReadBranchFile(_ context.Context, branch, path string) ([]byte, error) {
	branchFiles, ok := f.files[branch]
	if !ok {
		return nil, os.ErrNotExist
	}
	content, ok := branchFiles[filepath.ToSlash(path)]
	if !ok {
		return nil, os.ErrNotExist
	}
	return content, nil
}

func (f *fakeGitClient) ListBranchFiles(_ context.Context, branch string) ([]string, error) {
	branchFiles, ok := f.files[branch]
	if !ok {
		return nil, nil
	}
	var res []string
	for p := range branchFiles {
		res = append(res, p)
	}
	return res, nil
}

func (f *fakeGitClient) CommitBranchFiles(_ context.Context, branch, _ string, addFiles map[string][]byte, removePaths []string) error {
	if _, ok := f.files[branch]; !ok {
		f.files[branch] = make(map[string][]byte)
	}
	for p, content := range addFiles {
		f.files[branch][filepath.ToSlash(p)] = content
	}
	for _, p := range removePaths {
		delete(f.files[branch], filepath.ToSlash(p))
	}
	return nil
}

func TestGitStore_SaveLoadListPrune(t *testing.T) {
	client := newFakeGitClient()
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	store := NewGitStore(client, "perfguard-data", WithGitNowFunc(func() time.Time { return now }))
	ctx := context.Background()

	// 1. Save and load
	r1 := makeTestRun("sha1", "linux", "amd64", "Xeon", 10, now.Add(-2*time.Hour))
	key1, err := store.Save(ctx, r1)
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := store.Load(ctx, key1)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if loaded.Git.SHA != "sha1" {
		t.Errorf("expected sha1, got %s", loaded.Git.SHA)
	}

	// 2. Load missing
	_, err = store.Load(ctx, Key{OS: "linux", Arch: "amd64", EnvHash: "hash", CommitSHA: "missing"})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	// 3. Save second and list
	r2 := makeTestRun("sha2", "linux", "amd64", "Xeon", 10, now.Add(-1*time.Hour))
	if _, err := store.Save(ctx, r2); err != nil {
		t.Fatal(err)
	}

	all, err := store.List(ctx, Filter{})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 items, got %d", len(all))
	}
	if all[0].CommitSHA != "sha2" || all[1].CommitSHA != "sha1" {
		t.Errorf("unexpected sort order: %s, %s", all[0].CommitSHA, all[1].CommitSHA)
	}

	// 4. Prune keep 1
	deleted, err := store.Prune(ctx, PruneOptions{Keep: 1})
	if err != nil {
		t.Fatalf("prune failed: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}

	remaining, err := store.List(ctx, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 1 || remaining[0].CommitSHA != "sha2" {
		t.Errorf("expected only sha2 remaining, got %+v", remaining)
	}
}

func TestGitStore_ContextCancelled(t *testing.T) {
	client := newFakeGitClient()
	store := NewGitStore(client, "perfguard-data")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	run := makeTestRun("ctx1", "linux", "amd64", "cpu", 5, time.Now())
	if _, err := store.Save(ctx, run); err == nil {
		t.Error("expected error with cancelled context on Save")
	}
	if _, err := store.Load(ctx, Key{}); err == nil {
		t.Error("expected error with cancelled context on Load")
	}
	if _, err := store.List(ctx, Filter{}); err == nil {
		t.Error("expected error with cancelled context on List")
	}
	if _, err := store.Prune(ctx, PruneOptions{Keep: 1}); err == nil {
		t.Error("expected error with cancelled context on Prune")
	}
}
