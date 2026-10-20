package baseline

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kyyril/perfguard/internal/model"
)

func makeTestRun(sha, osName, arch, cpuModel string, count int, createdAt time.Time) model.Run {
	return model.Run{
		SchemaVersion: model.CurrentSchemaVersion,
		ID:            "run-" + sha,
		Side:          model.SideBase,
		CreatedAt:     createdAt,
		Git: model.GitInfo{
			SHA: sha,
			Ref: "refs/heads/main",
		},
		Env: model.Environment{
			OS:         osName,
			Arch:       arch,
			GoVersion:  "go1.24.1",
			CPUModel:   cpuModel,
			GOMAXPROCS: 8,
		},
		Params: model.RunParams{
			Count:     count,
			Benchtime: "1s",
			CPU:       8,
			Include:   ".",
			Mode:      "same-runner",
		},
		Benchmarks: []model.BenchmarkResult{
			{
				Package: "example.com/pkg",
				Name:    "BenchmarkWork",
				Procs:   8,
				Samples: []model.Sample{
					{
						Iterations: 1000,
						Values:     map[string]float64{"sec/op": 0.001},
					},
				},
			},
		},
	}
}

func TestFSStore_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFSStore(tmpDir)
	ctx := context.Background()

	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	run := makeTestRun("abc1234", "linux", "amd64", "Intel Xeon", 10, now)

	// Save baseline
	key, err := store.Save(ctx, run)
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}

	if key.OS != "linux" || key.Arch != "amd64" || key.CommitSHA != "abc1234" {
		t.Errorf("unexpected key fields: %+v", key)
	}
	if len(key.EnvHash) != 12 {
		t.Errorf("expected 12-char env hash, got %q", key.EnvHash)
	}

	// Verify file was written at expected path
	expectedFile := filepath.Join(tmpDir, "linux-amd64", key.EnvHash, "abc1234.json")
	if _, err := os.Stat(expectedFile); err != nil {
		t.Fatalf("expected baseline file does not exist: %v", err)
	}

	// Verify no temporary files remain
	entries, err := os.ReadDir(filepath.Join(tmpDir, "linux-amd64", key.EnvHash))
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "baseline-") && strings.HasSuffix(entry.Name(), ".tmp") {
			t.Errorf("stray temporary file found: %s", entry.Name())
		}
	}

	// Load baseline
	loaded, err := store.Load(ctx, key)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if loaded.Git.SHA != run.Git.SHA {
		t.Errorf("expected sha %s, got %s", run.Git.SHA, loaded.Git.SHA)
	}
	if loaded.Env.CPUModel != run.Env.CPUModel {
		t.Errorf("expected cpu model %s, got %s", run.Env.CPUModel, loaded.Env.CPUModel)
	}
	if len(loaded.Benchmarks) != 1 || loaded.Benchmarks[0].Name != "BenchmarkWork" {
		t.Errorf("unexpected benchmarks: %+v", loaded.Benchmarks)
	}
}

func TestFSStore_LoadNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFSStore(tmpDir)
	ctx := context.Background()

	key := Key{
		OS:        "linux",
		Arch:      "amd64",
		EnvHash:   "nonexistent",
		CommitSHA: "0000000",
	}

	_, err := store.Load(ctx, key)
	if err == nil {
		t.Fatal("expected error on missing baseline, got nil")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestFSStore_SaveInvalidRun(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFSStore(tmpDir)
	ctx := context.Background()

	// Missing OS
	runNoOS := model.Run{
		Git: model.GitInfo{SHA: "abc"},
		Env: model.Environment{Arch: "amd64"},
	}
	if _, err := store.Save(ctx, runNoOS); !errors.Is(err, ErrInvalidRun) {
		t.Errorf("expected ErrInvalidRun for missing OS, got %v", err)
	}

	// Missing SHA and ID
	runNoSHA := model.Run{
		Env: model.Environment{OS: "linux", Arch: "amd64"},
	}
	if _, err := store.Save(ctx, runNoSHA); !errors.Is(err, ErrInvalidRun) {
		t.Errorf("expected ErrInvalidRun for missing SHA, got %v", err)
	}

	// Fallback to ID when SHA is empty
	runWithID := model.Run{
		ID:  "run-from-id",
		Env: model.Environment{OS: "linux", Arch: "amd64"},
	}
	keyID, err := KeyFromRun(runWithID)
	if err != nil {
		t.Fatalf("expected success with ID fallback, got: %v", err)
	}
	if keyID.CommitSHA != "run-from-id" {
		t.Errorf("expected CommitSHA %q, got %q", "run-from-id", keyID.CommitSHA)
	}

	// Target dir cannot be created because a file exists with the same name
	fileAsDir := filepath.Join(tmpDir, "file_as_dir")
	if err := os.WriteFile(fileAsDir, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	badStore := NewFSStore(fileAsDir)
	validRun := makeTestRun("sha1", "linux", "amd64", "cpu", 5, time.Now())
	if _, err := badStore.Save(ctx, validRun); err == nil {
		t.Error("expected error saving to invalid root dir, got nil")
	}
}

func TestFSStore_ContextCancelled(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFSStore(tmpDir)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	run := makeTestRun("ctx123", "linux", "amd64", "Xeon", 5, time.Now())

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

func TestFSStore_List(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFSStore(tmpDir)
	ctx := context.Background()

	// Empty directory
	items, err := store.List(ctx, Filter{})
	if err != nil {
		t.Fatalf("list on empty dir failed: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}

	t1 := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)

	run1 := makeTestRun("sha1", "linux", "amd64", "Xeon", 10, t1)
	run2 := makeTestRun("sha2", "linux", "amd64", "Xeon", 10, t2)
	run3 := makeTestRun("sha3", "darwin", "arm64", "Apple M3", 10, t3)

	if _, err := store.Save(ctx, run1); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save(ctx, run2); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save(ctx, run3); err != nil {
		t.Fatal(err)
	}

	// List all, must be sorted newest first: run3, run2, run1
	all, err := store.List(ctx, Filter{})
	if err != nil {
		t.Fatalf("list all failed: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 items, got %d", len(all))
	}
	if all[0].CommitSHA != "sha3" || all[1].CommitSHA != "sha2" || all[2].CommitSHA != "sha1" {
		t.Errorf("unexpected sort order: %s, %s, %s", all[0].CommitSHA, all[1].CommitSHA, all[2].CommitSHA)
	}

	// Filter by OS
	linuxItems, err := store.List(ctx, Filter{OS: "linux"})
	if err != nil {
		t.Fatalf("filter by os failed: %v", err)
	}
	if len(linuxItems) != 2 {
		t.Errorf("expected 2 linux items, got %d", len(linuxItems))
	}

	// Filter by CommitSHA
	sha2Items, err := store.List(ctx, Filter{CommitSHA: "sha2"})
	if err != nil {
		t.Fatalf("filter by sha failed: %v", err)
	}
	if len(sha2Items) != 1 || sha2Items[0].CommitSHA != "sha2" {
		t.Errorf("expected 1 item with sha2, got %+v", sha2Items)
	}
}

func TestFSStore_Prune(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	store := NewFSStore(tmpDir, WithNowFunc(func() time.Time { return now }))
	ctx := context.Background()

	// No options
	deleted, err := store.Prune(ctx, PruneOptions{})
	if err != nil || deleted != 0 {
		t.Errorf("expected 0 deleted with no options, got %d (err: %v)", deleted, err)
	}

	// Create 4 runs on same environment with different timestamps
	t1 := now.Add(-10 * 24 * time.Hour) // 10 days old
	t2 := now.Add(-5 * 24 * time.Hour)  // 5 days old
	t3 := now.Add(-2 * 24 * time.Hour)  // 2 days old
	t4 := now.Add(-1 * time.Hour)       // 1 hour old

	r1 := makeTestRun("sha1", "linux", "amd64", "Xeon", 10, t1)
	r2 := makeTestRun("sha2", "linux", "amd64", "Xeon", 10, t2)
	r3 := makeTestRun("sha3", "linux", "amd64", "Xeon", 10, t3)
	r4 := makeTestRun("sha4", "linux", "amd64", "Xeon", 10, t4)

	for _, r := range []model.Run{r1, r2, r3, r4} {
		if _, err := store.Save(ctx, r); err != nil {
			t.Fatal(err)
		}
	}

	// Prune older than 7 days: should delete r1 (10 days old)
	delCount, err := store.Prune(ctx, PruneOptions{OlderThan: 7 * 24 * time.Hour})
	if err != nil {
		t.Fatalf("prune older than failed: %v", err)
	}
	if delCount != 1 {
		t.Errorf("expected 1 deleted, got %d", delCount)
	}

	remaining, err := store.List(ctx, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 3 {
		t.Fatalf("expected 3 remaining, got %d", len(remaining))
	}

	// Prune keep 2: should delete r2 (oldest of the remaining 3)
	delCount, err = store.Prune(ctx, PruneOptions{Keep: 2})
	if err != nil {
		t.Fatalf("prune keep failed: %v", err)
	}
	if delCount != 1 {
		t.Errorf("expected 1 deleted, got %d", delCount)
	}

	remaining, err = store.List(ctx, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 2 {
		t.Fatalf("expected 2 remaining, got %d", len(remaining))
	}
	if remaining[0].CommitSHA != "sha4" || remaining[1].CommitSHA != "sha3" {
		t.Errorf("unexpected remaining items: %+v", remaining)
	}
}

func TestFSStore_PrunePerGroup(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	store := NewFSStore(tmpDir, WithNowFunc(func() time.Time { return now }))
	ctx := context.Background()

	// Group A (linux-amd64)
	rA1 := makeTestRun("a1", "linux", "amd64", "Xeon", 10, now.Add(-3*time.Hour))
	rA2 := makeTestRun("a2", "linux", "amd64", "Xeon", 10, now.Add(-2*time.Hour))
	rA3 := makeTestRun("a3", "linux", "amd64", "Xeon", 10, now.Add(-1*time.Hour))

	// Group B (darwin-arm64)
	rB1 := makeTestRun("b1", "darwin", "arm64", "M3", 10, now.Add(-3*time.Hour))
	rB2 := makeTestRun("b2", "darwin", "arm64", "M3", 10, now.Add(-2*time.Hour))
	rB3 := makeTestRun("b3", "darwin", "arm64", "M3", 10, now.Add(-1*time.Hour))

	for _, r := range []model.Run{rA1, rA2, rA3, rB1, rB2, rB3} {
		if _, err := store.Save(ctx, r); err != nil {
			t.Fatal(err)
		}
	}

	// Keep 2 per group: should delete a1 and b1 (total 2 deleted)
	delCount, err := store.Prune(ctx, PruneOptions{Keep: 2, PerGroup: true})
	if err != nil {
		t.Fatalf("prune per group failed: %v", err)
	}
	if delCount != 2 {
		t.Errorf("expected 2 deleted, got %d", delCount)
	}

	remaining, err := store.List(ctx, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 4 {
		t.Fatalf("expected 4 remaining, got %d", len(remaining))
	}
}

func TestKey_String(t *testing.T) {
	k := Key{
		OS:        "linux",
		Arch:      "amd64",
		EnvHash:   "abc123def456",
		CommitSHA: "deadbeef",
	}
	expected := "linux-amd64/abc123def456/deadbeef"
	if k.String() != expected {
		t.Errorf("expected %q, got %q", expected, k.String())
	}
}

func TestFilter_Matches(t *testing.T) {
	info := Info{
		OS:        "linux",
		Arch:      "amd64",
		EnvHash:   "hash1",
		CommitSHA: "sha1",
	}

	if (Filter{OS: "darwin"}).Matches(info) {
		t.Error("expected mismatch on OS")
	}
	if (Filter{Arch: "arm64"}).Matches(info) {
		t.Error("expected mismatch on Arch")
	}
	if (Filter{EnvHash: "hash2"}).Matches(info) {
		t.Error("expected mismatch on EnvHash")
	}
	if (Filter{CommitSHA: "sha2"}).Matches(info) {
		t.Error("expected mismatch on CommitSHA")
	}
	if !(Filter{OS: "linux", Arch: "amd64", EnvHash: "hash1", CommitSHA: "sha1"}).Matches(info) {
		t.Error("expected match on all fields")
	}
}

func TestParseGoMinorVersion(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"go1.24.1", "1.24"},
		{"go1.25", "1.25"},
		{"devel", "devel"},
		{"go1", "1"},
	}

	for _, tt := range tests {
		got := parseGoMinorVersion(tt.input)
		if got != tt.want {
			t.Errorf("parseGoMinorVersion(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFSStore_LoadMalformedJSON(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFSStore(tmpDir)
	ctx := context.Background()

	key := Key{
		OS:        "linux",
		Arch:      "amd64",
		EnvHash:   "hash",
		CommitSHA: "badjson",
	}

	targetDir := filepath.Join(tmpDir, "linux-amd64", "hash")
	if err := os.MkdirAll(targetDir, 0o750); err != nil {
		t.Fatal(err)
	}
	badFile := filepath.Join(targetDir, "badjson.json")
	if err := os.WriteFile(badFile, []byte("invalid json content"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := store.Load(ctx, key); err == nil {
		t.Error("expected error loading malformed json, got nil")
	}
}

func TestFSStore_ListNonExistentDir(t *testing.T) {
	nonExistent := filepath.Join(t.TempDir(), "nonexistent")
	store := NewFSStore(nonExistent)
	ctx := context.Background()

	items, err := store.List(ctx, Filter{})
	if err != nil {
		t.Fatalf("unexpected error on missing dir: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestFSStore_ListIgnoresIrregularFiles(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFSStore(tmpDir)
	ctx := context.Background()

	// 1. Regular file at root
	_ = os.WriteFile(filepath.Join(tmpDir, "root.json"), []byte("{}"), 0o600)

	// 2. Non-json file in subfolder
	targetDir := filepath.Join(tmpDir, "linux-amd64", "hash")
	_ = os.MkdirAll(targetDir, 0o750)
	_ = os.WriteFile(filepath.Join(targetDir, "readme.txt"), []byte("text"), 0o600)

	// 3. Corrupt json file
	_ = os.WriteFile(filepath.Join(targetDir, "corrupt.json"), []byte("bad json"), 0o600)

	items, err := store.List(ctx, Filter{})
	if err != nil {
		t.Fatalf("list error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 valid items, got %d", len(items))
	}
}
