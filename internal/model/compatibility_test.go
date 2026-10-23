package model_test

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/kyyril/perfguard/internal/model"
)

func TestSchemaCompatibility_V0x_Result(t *testing.T) {
	// Verify that v1.0 can read result.json files from v0.x releases
	data, err := os.ReadFile("../../testdata/fixtures/v0x-result.json")
	if err != nil {
		t.Fatalf("read v0x fixture: %v", err)
	}

	var run model.Run
	if err := json.Unmarshal(data, &run); err != nil {
		t.Fatalf("unmarshal v0x result: %v", err)
	}

	// Verify schema version
	if run.SchemaVersion != 1 {
		t.Errorf("schema_version = %d, want 1", run.SchemaVersion)
	}

	// Verify basic fields
	if run.ID != "run-v0x-fixture" {
		t.Errorf("id = %q, want %q", run.ID, "run-v0x-fixture")
	}

	if run.Side != model.SideBase {
		t.Errorf("side = %q, want %q", run.Side, model.SideBase)
	}

	// Verify git info
	if run.Git.SHA != "abc123def456" {
		t.Errorf("git.sha = %q, want %q", run.Git.SHA, "abc123def456")
	}

	if run.Git.Dirty {
		t.Error("git.dirty = true, want false")
	}

	// Verify environment
	if run.Env.OS != "linux" {
		t.Errorf("env.os = %q, want %q", run.Env.OS, "linux")
	}

	if run.Env.NumCPU != 8 {
		t.Errorf("env.num_cpu = %d, want 8", run.Env.NumCPU)
	}

	// Verify params
	if run.Params.Count != 10 {
		t.Errorf("params.count = %d, want 10", run.Params.Count)
	}

	if run.Params.Mode != "same-runner" {
		t.Errorf("params.mode = %q, want %q", run.Params.Mode, "same-runner")
	}

	// Verify benchmarks
	if len(run.Benchmarks) != 1 {
		t.Fatalf("len(benchmarks) = %d, want 1", len(run.Benchmarks))
	}

	bench := run.Benchmarks[0]
	if bench.Name != "BenchmarkEncode" {
		t.Errorf("benchmark.name = %q, want %q", bench.Name, "BenchmarkEncode")
	}

	if len(bench.Samples) != 3 {
		t.Fatalf("len(samples) = %d, want 3", len(bench.Samples))
	}

	// Verify sample structure
	sample := bench.Samples[0]
	if sample.Iterations != 1000000 {
		t.Errorf("sample.iterations = %d, want 1000000", sample.Iterations)
	}

	if _, ok := sample.Values["sec/op"]; !ok {
		t.Error("sample.values missing sec/op")
	}

	if _, ok := sample.Values["B/op"]; !ok {
		t.Error("sample.values missing B/op")
	}

	if _, ok := sample.Values["allocs/op"]; !ok {
		t.Error("sample.values missing allocs/op")
	}
}

func TestSchemaCompatibility_V0x_Comparison(t *testing.T) {
	// Verify that v1.0 can read comparison.json files from v0.x releases
	data, err := os.ReadFile("../../testdata/fixtures/v0x-comparison.json")
	if err != nil {
		t.Fatalf("read v0x fixture: %v", err)
	}

	var comp model.Comparison
	if err := json.Unmarshal(data, &comp); err != nil {
		t.Fatalf("unmarshal v0x comparison: %v", err)
	}

	// Verify schema version
	if comp.SchemaVersion != 1 {
		t.Errorf("schema_version = %d, want 1", comp.SchemaVersion)
	}

	// Verify base summary
	if comp.Base.ID != "run-base-v0x" {
		t.Errorf("base.id = %q, want %q", comp.Base.ID, "run-base-v0x")
	}

	if comp.Base.Side != model.SideBase {
		t.Errorf("base.side = %q, want %q", comp.Base.Side, model.SideBase)
	}

	// Verify head summary
	if comp.Head.ID != "run-head-v0x" {
		t.Errorf("head.id = %q, want %q", comp.Head.ID, "run-head-v0x")
	}

	if comp.Head.Side != model.SideHead {
		t.Errorf("head.side = %q, want %q", comp.Head.Side, model.SideHead)
	}

	// Verify verdict
	if comp.Verdict != model.VerdictFail {
		t.Errorf("verdict = %q, want %q", comp.Verdict, model.VerdictFail)
	}

	// Verify items
	if len(comp.Items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(comp.Items))
	}

	// Verify first item (regression)
	item := comp.Items[0]
	if item.Benchmark != "BenchmarkEncode" {
		t.Errorf("item[0].benchmark = %q, want %q", item.Benchmark, "BenchmarkEncode")
	}

	if item.Metric != "sec/op" {
		t.Errorf("item[0].metric = %q, want %q", item.Metric, "sec/op")
	}

	if item.Status != model.StatusFail {
		t.Errorf("item[0].status = %q, want %q", item.Status, model.StatusFail)
	}

	if item.Reason != model.ReasonExceedsFail {
		t.Errorf("item[0].reason = %q, want %q", item.Reason, model.ReasonExceedsFail)
	}

	// Verify confidence interval structure
	if len(item.CI) != 2 {
		t.Fatalf("len(item.ci) = %d, want 2", len(item.CI))
	}

	// Verify second item (unchanged)
	item2 := comp.Items[1]
	if item2.Status != model.StatusUnchanged {
		t.Errorf("item[1].status = %q, want %q", item2.Status, model.StatusUnchanged)
	}
}

func TestSchemaCompatibility_RoundTrip(t *testing.T) {
	// Create a Run, serialize it, deserialize it, verify equality
	original := model.Run{
		SchemaVersion: model.CurrentSchemaVersion,
		ID:            "test-run-123",
		Side:          model.SideBase,
		CreatedAt:     time.Now().UTC().Truncate(time.Second),
		Git: model.GitInfo{
			SHA:   "abc123",
			Ref:   "refs/heads/main",
			Dirty: false,
		},
		Env: model.Environment{
			OS:         "linux",
			Arch:       "amd64",
			GoVersion:  "go1.24.0",
			NumCPU:     8,
			CPUModel:   "Test CPU",
			GOMAXPROCS: 8,
		},
		Params: model.RunParams{
			Count:          10,
			Benchtime:      "1s",
			CPU:            0,
			Include:        ".",
			Mode:           "same-runner",
			WarmupRuns:     1,
			MaxExtraRuns:   10,
			ExtraRoundStep: 2,
		},
		Benchmarks: []model.BenchmarkResult{
			{
				Package: "github.com/test/pkg",
				Name:    "BenchmarkTest",
				Procs:   1,
				Samples: []model.Sample{
					{
						Iterations: 1000,
						Values: map[string]float64{
							"sec/op":    0.001,
							"B/op":      100,
							"allocs/op": 2,
						},
					},
				},
			},
		},
	}

	// Marshal
	data, err := json.MarshalIndent(original, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Unmarshal
	var decoded model.Run
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Compare
	if diff := cmp.Diff(original, decoded); diff != "" {
		t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
	}
}

func TestSchemaVersion_Const(t *testing.T) {
	// Verify schema version is frozen at 1 for v1.0
	if model.CurrentSchemaVersion != 1 {
		t.Errorf("CurrentSchemaVersion = %d, want 1 (frozen for v1.0)", model.CurrentSchemaVersion)
	}
}

func TestSchemaVersion_InJSON(t *testing.T) {
	// Verify that schema_version is always written to JSON
	run := model.Run{
		SchemaVersion: model.CurrentSchemaVersion,
		ID:            "test",
		Side:          model.SideBase,
		CreatedAt:     time.Now(),
		Git:           model.GitInfo{SHA: "abc", Ref: "main", Dirty: false},
		Env: model.Environment{
			OS: "linux", Arch: "amd64", GoVersion: "go1.24", NumCPU: 1,
			CPUModel: "test", GOMAXPROCS: 1,
		},
		Params: model.RunParams{
			Count: 10, Benchtime: "1s", CPU: 0, Include: ".",
			Mode: "same-runner", WarmupRuns: 1, MaxExtraRuns: 10, ExtraRoundStep: 2,
		},
		Benchmarks: []model.BenchmarkResult{},
	}

	data, err := json.Marshal(run)
	if err != nil {
		t.Fatal(err)
	}

	// Check that schema_version appears in JSON
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}

	version, ok := raw["schema_version"]
	if !ok {
		t.Error("schema_version not present in JSON output")
	}

	versionFloat, ok := version.(float64)
	if !ok {
		t.Errorf("schema_version is %T, want float64", version)
	}

	if int(versionFloat) != 1 {
		t.Errorf("schema_version in JSON = %v, want 1", versionFloat)
	}
}
