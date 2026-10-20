package model_test

import (
	"encoding/json"
	"flag"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/kyyril/perfguard/internal/model"
)

var update = flag.Bool("update", false, "update golden files")

func sampleRun() model.Run {
	return model.Run{
		SchemaVersion: model.CurrentSchemaVersion,
		ID:            "run-abc-123",
		Side:          model.SideBase,
		CreatedAt:     time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC),
		Git: model.GitInfo{
			SHA:   "3f2a9c1",
			Ref:   "origin/main",
			Dirty: false,
		},
		Env: model.Environment{
			OS:         "linux",
			Arch:       "amd64",
			GoVersion:  "go1.24",
			NumCPU:     8,
			CPUModel:   "AMD EPYC",
			GOMAXPROCS: 8,
			Hostname:   "worker-01",
		},
		Params: model.RunParams{
			Count:        10,
			Benchtime:    "1s",
			CPU:          8,
			Include:      ".",
			Mode:         "same-runner",
			WarmupRuns:   1,
			MaxExtraRuns: 10,
		},
		Benchmarks: []model.BenchmarkResult{
			{
				Package: "example.com/pkg",
				Name:    "BenchmarkHandle",
				Procs:   8,
				Samples: []model.Sample{
					{
						Iterations: 1000,
						Values: map[string]float64{
							"allocs/op": 2.0,
							"B/op":      128.0,
							"sec/op":    0.001,
						},
					},
				},
			},
		},
		Profiles: []model.ProfileRef{
			{
				Benchmark: "BenchmarkHandle",
				Type:      "cpu",
				Path:      ".perfguard/runs/run-abc-123/base.cpu.pb.gz",
				Runs:      3,
			},
		},
	}
}

func sampleComparison() model.Comparison {
	run := sampleRun()
	return model.Comparison{
		SchemaVersion: model.CurrentSchemaVersion,
		Base:          run.Summary(),
		Head: model.RunSummary{
			ID:        "run-def-456",
			Side:      model.SideHead,
			CreatedAt: time.Date(2026, 10, 20, 10, 5, 0, 0, time.UTC),
			Git: model.GitInfo{
				SHA:   "8be41d7",
				Ref:   "head",
				Dirty: false,
			},
			Env: run.Env,
		},
		Items: []model.MetricComparison{
			{
				Benchmark:  "BenchmarkHandle",
				Metric:     "sec/op",
				BaseMedian: 100.0,
				HeadMedian: 135.0,
				DeltaPct:   35.0,
				CI:         [2]float64{30.0, 40.0},
				PValue:     0.0002,
				CVBase:     2.1,
				CVHead:     2.5,
				NBase:      10,
				NHead:      10,
				Status:     model.StatusFail,
				Reason:     model.ReasonExceedsFail,
			},
		},
		Verdict: model.VerdictFail,
		Diagnosis: []model.Diagnosis{
			{
				Benchmark:   "BenchmarkHandle",
				ProfileType: "cpu",
				Candidates: []model.Candidate{
					{
						Function:  "example.com/pkg.HeavyFunc",
						FlatDelta: 9.8,
						CumDelta:  14.1,
						Touched:   true,
					},
				},
			},
		},
		Notes: []string{"measured on same-runner"},
	}
}

func TestRun_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := sampleRun()
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded model.Run
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if diff := cmp.Diff(original, decoded); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestComparison_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := sampleComparison()
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded model.Comparison
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if diff := cmp.Diff(original, decoded); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestRun_Golden(t *testing.T) {
	t.Parallel()

	run := sampleRun()
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	goldenPath := filepath.Join("..", "..", "testdata", "model", "run.golden.json")
	if *update {
		if err := os.WriteFile(goldenPath, append(data, '\n'), 0o600); err != nil {
			t.Fatalf("failed to update golden file: %v", err)
		}
	}

	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("failed to read golden file: %v", err)
	}

	expectedStr := strings.ReplaceAll(strings.TrimSpace(string(expected)), "\r\n", "\n")
	dataStr := strings.ReplaceAll(strings.TrimSpace(string(data)), "\r\n", "\n")

	if diff := cmp.Diff(expectedStr, dataStr); diff != "" {
		t.Errorf("run golden mismatch (-want +got):\n%s", diff)
	}
}

func TestComparison_Golden(t *testing.T) {
	t.Parallel()

	comp := sampleComparison()
	data, err := json.MarshalIndent(comp, "", "  ")
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	goldenPath := filepath.Join("..", "..", "testdata", "model", "comparison.golden.json")
	if *update {
		if err := os.WriteFile(goldenPath, append(data, '\n'), 0o600); err != nil {
			t.Fatalf("failed to update golden file: %v", err)
		}
	}

	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("failed to read golden file: %v", err)
	}

	expectedStr := strings.ReplaceAll(strings.TrimSpace(string(expected)), "\r\n", "\n")
	dataStr := strings.ReplaceAll(strings.TrimSpace(string(data)), "\r\n", "\n")

	if diff := cmp.Diff(expectedStr, dataStr); diff != "" {
		t.Errorf("comparison golden mismatch (-want +got):\n%s", diff)
	}
}

func TestSchemaVersion(t *testing.T) {
	t.Parallel()

	run := sampleRun()
	if run.SchemaVersion != 1 {
		t.Fatalf("expected schema_version 1, got %d", run.SchemaVersion)
	}

	comp := sampleComparison()
	if comp.SchemaVersion != 1 {
		t.Fatalf("expected schema_version 1, got %d", comp.SchemaVersion)
	}
}

func TestVerdict_RankAndPrecedence(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		a, b     model.Verdict
		expected model.Verdict
	}{
		{
			name:     "fail over warn",
			a:        model.VerdictFail,
			b:        model.VerdictWarn,
			expected: model.VerdictFail,
		},
		{
			name:     "warn over pass",
			a:        model.VerdictWarn,
			b:        model.VerdictPass,
			expected: model.VerdictWarn,
		},
		{
			name:     "inconclusive over pass",
			a:        model.VerdictInconclusive,
			b:        model.VerdictPass,
			expected: model.VerdictInconclusive,
		},
		{
			name:     "warn over inconclusive",
			a:        model.VerdictInconclusive,
			b:        model.VerdictWarn,
			expected: model.VerdictWarn,
		},
		{
			name:     "fail over inconclusive",
			a:        model.VerdictFail,
			b:        model.VerdictInconclusive,
			expected: model.VerdictFail,
		},
		{
			name:     "pass over unknown",
			a:        model.VerdictPass,
			b:        model.Verdict("unknown"),
			expected: model.VerdictPass,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := model.WorstVerdict(tc.a, tc.b)
			if got != tc.expected {
				t.Errorf("WorstVerdict(%s, %s) = %s; want %s", tc.a, tc.b, got, tc.expected)
			}
			reverse := model.WorstVerdict(tc.b, tc.a)
			if reverse != tc.expected {
				t.Errorf("WorstVerdict(%s, %s) = %s; want %s", tc.b, tc.a, reverse, tc.expected)
			}
		})
	}
}

func TestNoInternalImportsInModel(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "model.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("failed to parse model.go: %v", err)
	}

	for _, imp := range node.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if strings.Contains(path, "internal") {
			t.Errorf("internal/model must not import other internal packages, found: %s", path)
		}
	}
}
