package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kyyril/perfguard/internal/model"
)

func makeFixtureRun(sha string) model.Run {
	return model.Run{
		SchemaVersion: model.CurrentSchemaVersion,
		ID:            "run-" + sha,
		Side:          model.SideBase,
		CreatedAt:     time.Now().Add(-24 * time.Hour),
		Git: model.GitInfo{
			SHA: sha,
			Ref: "refs/heads/main",
		},
		Env: model.Environment{
			OS:         "linux",
			Arch:       "amd64",
			GoVersion:  "go1.24.0",
			CPUModel:   "AMD EPYC",
			GOMAXPROCS: 8,
		},
		Params: model.RunParams{
			Count:     5,
			Benchtime: "10ms",
			CPU:       8,
			Include:   ".",
			Mode:      "stored",
		},
		Benchmarks: []model.BenchmarkResult{
			{
				Package: "example.com/math",
				Name:    "BenchmarkCompute",
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

func TestBaselineCommands_Flow(t *testing.T) {
	tmpDir := t.TempDir()
	baselineDir := filepath.Join(tmpDir, "baselines")
	cfgPath := filepath.Join(tmpDir, "perfguard.yaml")
	cfgContent := `version: 1
baseline:
  store: fs
  dir: ` + filepath.ToSlash(baselineDir) + `
`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o600); err != nil {
		t.Fatal(err)
	}

	// 1. Initial list on empty store
	cmdList := NewRootCommand()
	var outList bytes.Buffer
	cmdList.SetOut(&outList)
	cmdList.SetArgs([]string{"baseline", "list", "--config", cfgPath})
	if err := cmdList.Execute(); err != nil {
		t.Fatalf("baseline list failed: %v", err)
	}
	if !strings.Contains(outList.String(), "No stored baselines found.") {
		t.Errorf("expected empty message, got: %s", outList.String())
	}

	// 2. Save baseline from file
	runFile := filepath.Join(tmpDir, "result.json")
	run := makeFixtureRun("1234567890abcdef")
	data, _ := json.Marshal(run)
	_ = os.WriteFile(runFile, data, 0o600)

	cmdSave := NewRootCommand()
	var outSave bytes.Buffer
	cmdSave.SetOut(&outSave)
	cmdSave.SetArgs([]string{"baseline", "save", runFile, "--config", cfgPath})
	if err := cmdSave.Execute(); err != nil {
		t.Fatalf("baseline save failed: %v", err)
	}
	if !strings.Contains(outSave.String(), "Saved baseline for commit 1234567890abcdef") {
		t.Errorf("expected save confirmation, got: %s", outSave.String())
	}

	// 3. List after save
	cmdList2 := NewRootCommand()
	var outList2 bytes.Buffer
	cmdList2.SetOut(&outList2)
	cmdList2.SetArgs([]string{"baseline", "list", "--config", cfgPath})
	if err := cmdList2.Execute(); err != nil {
		t.Fatalf("baseline list failed: %v", err)
	}
	if !strings.Contains(outList2.String(), "1234567890") {
		t.Errorf("expected list to show commit, got: %s", outList2.String())
	}

	// 4. Show baseline
	cmdShow := NewRootCommand()
	var outShow bytes.Buffer
	cmdShow.SetOut(&outShow)
	cmdShow.SetArgs([]string{"baseline", "show", "1234567890abcdef", "--config", cfgPath})
	if err := cmdShow.Execute(); err != nil {
		t.Fatalf("baseline show failed: %v", err)
	}
	if !strings.Contains(outShow.String(), "BenchmarkCompute") {
		t.Errorf("expected show to include benchmark data, got: %s", outShow.String())
	}

	// 5. Show non-existent
	cmdShowBad := NewRootCommand()
	cmdShowBad.SetArgs([]string{"baseline", "show", "nonexistent", "--config", cfgPath})
	if err := cmdShowBad.Execute(); err == nil {
		t.Error("expected error showing nonexistent baseline, got nil")
	}

	// 6. Prune baselines
	cmdPrune := NewRootCommand()
	var outPrune bytes.Buffer
	cmdPrune.SetOut(&outPrune)
	cmdPrune.SetArgs([]string{"baseline", "prune", "--keep", "0", "--older-than", "1h", "--config", cfgPath})
	if err := cmdPrune.Execute(); err != nil {
		t.Fatalf("baseline prune failed: %v", err)
	}
	if !strings.Contains(outPrune.String(), "Pruned 1 baseline(s)") {
		t.Errorf("expected 1 baseline pruned, got: %s", outPrune.String())
	}
}
