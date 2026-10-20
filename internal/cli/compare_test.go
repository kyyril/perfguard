package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kyyril/perfguard/internal/cli"
	"github.com/kyyril/perfguard/internal/model"
)

func createResultFixture(t *testing.T, dir, filename string, side model.Side, values []float64) string {
	t.Helper()
	samples := make([]model.Sample, len(values))
	for i, v := range values {
		samples[i] = model.Sample{
			Iterations: 1000,
			Values:     map[string]float64{"sec/op": v},
		}
	}

	run := model.Run{
		SchemaVersion: model.CurrentSchemaVersion,
		ID:            string(side) + "-run",
		Side:          side,
		CreatedAt:     time.Now().UTC(),
		Git: model.GitInfo{
			SHA:   "1234567890123456789012345678901234567890",
			Ref:   "refs/heads/main",
			Dirty: false,
		},
		Env: model.Environment{
			OS:         "linux",
			Arch:       "amd64",
			GoVersion:  "go1.23.0",
			NumCPU:     4,
			CPUModel:   "Intel Core",
			GOMAXPROCS: 4,
		},
		Params: model.RunParams{
			Count:          len(values),
			Benchtime:      "1s",
			CPU:            4,
			Include:        ".*",
			Mode:           "same-runner",
			WarmupRuns:     0,
			MaxExtraRuns:   0,
			ExtraRoundStep: 1,
		},
		Benchmarks: []model.BenchmarkResult{
			{
				Package: "example.com/mod",
				Name:    "BenchmarkTest",
				Procs:   4,
				Samples: samples,
			},
		},
	}

	data, err := json.Marshal(run)
	if err != nil {
		t.Fatalf("failed marshaling run fixture: %v", err)
	}

	filePath := filepath.Join(dir, filename)
	if err := os.WriteFile(filePath, data, 0o600); err != nil {
		t.Fatalf("failed writing run fixture: %v", err)
	}
	return filePath
}

func TestCompareCommand_MissingArgs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
	}{
		{"no args", []string{"compare"}},
		{"one arg", []string{"compare", "base.json"}},
		{"three args", []string{"compare", "base.json", "head.json", "extra.json"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := cli.NewRootCommand()
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetArgs(tc.args)

			err := cmd.ExecuteContext(context.Background())
			if err == nil {
				t.Fatal("expected error on invalid arg count, got nil")
			}
			if cli.ExitCode(err) != cli.ExitConfig {
				t.Errorf("expected exit code %d (ExitConfig), got %d", cli.ExitConfig, cli.ExitCode(err))
			}
		})
	}
}

func TestCompareCommand_ExplicitMissingConfig(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	missingConfig := filepath.Join(tmpDir, "missing.yaml")

	cmd := cli.NewRootCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"compare", "--config", missingConfig, "base.json", "head.json"})

	err := cmd.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("expected error on missing config, got nil")
	}
	if cli.ExitCode(err) != cli.ExitConfig {
		t.Errorf("expected exit code %d (ExitConfig), got %d", cli.ExitConfig, cli.ExitCode(err))
	}
}

func TestCompareCommand_ExecutionError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "nonexistent-base.json")
	headPath := filepath.Join(tmpDir, "nonexistent-head.json")

	cmd := cli.NewRootCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"compare", basePath, headPath})

	err := cmd.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("expected error on nonexistent files, got nil")
	}
	if cli.ExitCode(err) != cli.ExitExecution {
		t.Errorf("expected exit code %d (ExitExecution), got %d", cli.ExitExecution, cli.ExitCode(err))
	}
}

func TestCompareCommand_Success(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	baseFile := createResultFixture(t, tmpDir, "base.result.json", model.SideBase, []float64{10, 10, 10, 10, 10, 10})
	headFile := createResultFixture(t, tmpDir, "head.result.json", model.SideHead, []float64{10, 10, 10, 10, 10, 10})
	outFile := filepath.Join(tmpDir, "out", "comparison.json")

	cmd := cli.NewRootCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"compare", baseFile, headFile, "-o", outFile})

	err := cmd.ExecuteContext(context.Background())
	if err != nil {
		t.Fatalf("compare command failed: %v", err)
	}

	if _, err := os.Stat(outFile); err != nil {
		t.Fatalf("expected output comparison file %s to exist: %v", outFile, err)
	}

	if !bytes.Contains(buf.Bytes(), []byte("pass verdict")) {
		t.Errorf("expected output to mention pass verdict, got: %s", buf.String())
	}
}
