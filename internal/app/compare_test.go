package app_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kyyril/perfguard/internal/app"
	"github.com/kyyril/perfguard/internal/model"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

func makeTestRun(id string, side model.Side, values []float64) model.Run {
	samples := make([]model.Sample, len(values))
	for i, v := range values {
		samples[i] = model.Sample{
			Iterations: 1000,
			Values:     map[string]float64{"sec/op": v},
		}
	}
	return model.Run{
		SchemaVersion: model.CurrentSchemaVersion,
		ID:            id,
		Side:          side,
		CreatedAt:     time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC),
		Git: model.GitInfo{
			SHA:   "abcdef1234567890abcdef1234567890abcdef12",
			Ref:   "refs/heads/main",
			Dirty: false,
		},
		Env: model.Environment{
			OS:         "linux",
			Arch:       "amd64",
			GoVersion:  "go1.23.0",
			NumCPU:     8,
			CPUModel:   "AMD Ryzen",
			GOMAXPROCS: 8,
		},
		Params: model.RunParams{
			Count:          len(values),
			Benchtime:      "1s",
			CPU:            8,
			Include:        ".*",
			Mode:           "same-runner",
			WarmupRuns:     0,
			MaxExtraRuns:   4,
			ExtraRoundStep: 2,
		},
		Benchmarks: []model.BenchmarkResult{
			{
				Package: "example.com/mod/pkg",
				Name:    "BenchmarkWork",
				Procs:   8,
				Samples: samples,
			},
		},
	}
}

func TestCompare_SuccessAndSchemaValidation(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	baseRun := makeTestRun("base-run", model.SideBase, []float64{10.0, 10.1, 9.9, 10.0, 10.2, 9.8})
	headRun := makeTestRun("head-run", model.SideHead, []float64{12.0, 12.1, 11.9, 12.0, 12.2, 11.8})

	baseFile := filepath.Join(tmpDir, "base.result.json")
	headFile := filepath.Join(tmpDir, "head.result.json")

	baseData, err := json.Marshal(baseRun)
	if err != nil {
		t.Fatalf("marshal base run: %v", err)
	}
	if err := os.WriteFile(baseFile, baseData, 0o600); err != nil {
		t.Fatalf("write base file: %v", err)
	}

	headData, err := json.Marshal(headRun)
	if err != nil {
		t.Fatalf("marshal head run: %v", err)
	}
	if err := os.WriteFile(headFile, headData, 0o600); err != nil {
		t.Fatalf("write head file: %v", err)
	}

	outComp := filepath.Join(tmpDir, "out", "comparison.json")

	cmp, outFile, err := app.Compare(context.Background(), app.CompareOptions{
		BaseResultFile: baseFile,
		HeadResultFile: headFile,
		OutputDir:      outComp,
	})
	if err != nil {
		t.Fatalf("app.Compare failed: %v", err)
	}

	if outFile != outComp {
		t.Errorf("expected outFile %s, got %s", outComp, outFile)
	}

	if cmp.SchemaVersion != model.CurrentSchemaVersion {
		t.Errorf("expected schema version %d, got %d", model.CurrentSchemaVersion, cmp.SchemaVersion)
	}
	if cmp.Verdict != model.VerdictFail {
		t.Errorf("expected fail verdict for +20%% regression, got %s", cmp.Verdict)
	}
	if len(cmp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(cmp.Items))
	}
	if cmp.Items[0].Status != model.StatusFail {
		t.Errorf("expected item status fail, got %s", cmp.Items[0].Status)
	}

	// Validate written file against schemas/comparison.v1.json
	compData, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read comparison output file: %v", err)
	}

	schemaPath, err := filepath.Abs("../../schemas/comparison.v1.json")
	if err != nil {
		t.Fatalf("resolve comparison schema path: %v", err)
	}

	compiler := jsonschema.NewCompiler()
	sch, err := compiler.Compile(schemaPath)
	if err != nil {
		t.Fatalf("compile comparison schema: %v", err)
	}

	var rawDoc any
	if err := json.Unmarshal(compData, &rawDoc); err != nil {
		t.Fatalf("unmarshal raw comparison doc: %v", err)
	}
	if err := sch.Validate(rawDoc); err != nil {
		t.Fatalf("comparison.json failed schema validation: %v", err)
	}
}

func TestCompare_DirectRuns(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	baseRun := makeTestRun("base-1", model.SideBase, []float64{10.0, 10.0, 10.0, 10.0, 10.0, 10.0})
	headRun := makeTestRun("head-1", model.SideHead, []float64{10.0, 10.0, 10.0, 10.0, 10.0, 10.0})

	cmp, outFile, err := app.Compare(context.Background(), app.CompareOptions{
		BaseRun:   &baseRun,
		HeadRun:   &headRun,
		OutputDir: tmpDir,
	})
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	expectedOut := filepath.Join(tmpDir, "comparison.json")
	if outFile != expectedOut {
		t.Errorf("expected outFile %s, got %s", expectedOut, outFile)
	}
	if cmp.Verdict != model.VerdictPass {
		t.Errorf("expected pass verdict, got %s", cmp.Verdict)
	}
}

func TestCompare_Errors(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	t.Run("context cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, _, err := app.Compare(ctx, app.CompareOptions{})
		if err == nil {
			t.Fatal("expected error on cancelled context, got nil")
		}
	})

	t.Run("missing base", func(t *testing.T) {
		_, _, err := app.Compare(context.Background(), app.CompareOptions{
			HeadResultFile: "head.json",
		})
		if err == nil {
			t.Fatal("expected error on missing base, got nil")
		}
	})

	t.Run("missing head", func(t *testing.T) {
		baseRun := makeTestRun("base", model.SideBase, []float64{10.0})
		_, _, err := app.Compare(context.Background(), app.CompareOptions{
			BaseRun: &baseRun,
		})
		if err == nil {
			t.Fatal("expected error on missing head, got nil")
		}
	})

	t.Run("unsupported schema version in base object", func(t *testing.T) {
		badBase := makeTestRun("base", model.SideBase, []float64{10.0})
		badBase.SchemaVersion = 99
		headRun := makeTestRun("head", model.SideHead, []float64{10.0})

		_, _, err := app.Compare(context.Background(), app.CompareOptions{
			BaseRun: &badBase,
			HeadRun: &headRun,
		})
		if err == nil {
			t.Fatal("expected error on unsupported schema version, got nil")
		}
	})

	t.Run("unsupported schema version in head file", func(t *testing.T) {
		baseRun := makeTestRun("base", model.SideBase, []float64{10.0})
		badHead := makeTestRun("head", model.SideHead, []float64{10.0})
		badHead.SchemaVersion = 99

		headFile := filepath.Join(tmpDir, "badhead.json")
		data, _ := json.Marshal(badHead)
		_ = os.WriteFile(headFile, data, 0o600)

		_, _, err := app.Compare(context.Background(), app.CompareOptions{
			BaseRun:        &baseRun,
			HeadResultFile: headFile,
		})
		if err == nil {
			t.Fatal("expected error on unsupported head schema version, got nil")
		}
	})

	t.Run("unreadable file", func(t *testing.T) {
		_, _, err := app.Compare(context.Background(), app.CompareOptions{
			BaseResultFile: filepath.Join(tmpDir, "nonexistent.json"),
		})
		if err == nil {
			t.Fatal("expected error on nonexistent file, got nil")
		}
	})

	t.Run("invalid json file", func(t *testing.T) {
		corruptFile := filepath.Join(tmpDir, "corrupt.json")
		_ = os.WriteFile(corruptFile, []byte("{broken json"), 0o600)

		_, _, err := app.Compare(context.Background(), app.CompareOptions{
			BaseResultFile: corruptFile,
		})
		if err == nil {
			t.Fatal("expected error on corrupted file, got nil")
		}
	})
}
