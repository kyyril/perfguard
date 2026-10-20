//go:build e2e

package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kyyril/perfguard/internal/config"
	"github.com/kyyril/perfguard/internal/model"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

func buildBinary(t *testing.T) string {
	t.Helper()

	binDir := t.TempDir()
	binName := "perfguard"
	if os.Getenv("GOOS") == "windows" || filepath.Separator == '\\' {
		binName += ".exe"
	}
	binPath := filepath.Join(binDir, binName)

	cmd := exec.Command("go", "build", "-o", binPath, "github.com/kyyril/perfguard/cmd/perfguard")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed building perfguard binary: %v\nOutput: %s", err, string(output))
	}
	return binPath
}

func TestE2E_VersionAndInit(t *testing.T) {
	binPath := buildBinary(t)

	t.Run("version command", func(t *testing.T) {
		cmd := exec.Command(binPath, "version")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("perfguard version failed: %v\nOutput: %s", err, string(output))
		}
		if !strings.HasPrefix(string(output), "perfguard") {
			t.Errorf("expected version output to start with 'perfguard', got: %s", string(output))
		}
	})

	t.Run("init creates valid configuration", func(t *testing.T) {
		workDir := t.TempDir()

		// Run init in clean directory
		cmd := exec.Command(binPath, "init")
		cmd.Dir = workDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("perfguard init failed: %v\nOutput: %s", err, string(output))
		}

		configPath := filepath.Join(workDir, "perfguard.yaml")
		if _, err := os.Stat(configPath); err != nil {
			t.Fatalf("perfguard.yaml was not created: %v", err)
		}

		cfg, err := config.Load(configPath)
		if err != nil {
			t.Fatalf("generated perfguard.yaml failed validation: %v", err)
		}
		if cfg.Version != 1 {
			t.Errorf("expected version 1, got %d", cfg.Version)
		}

		// Second init without force must exit with code 2
		cmdSecond := exec.Command(binPath, "init")
		cmdSecond.Dir = workDir
		errSecond := cmdSecond.Run()
		if errSecond == nil {
			t.Fatal("expected non-zero exit code on duplicate init, got nil")
		}
		if exitErr, ok := errSecond.(*exec.ExitError); ok {
			if exitErr.ExitCode() != 2 {
				t.Errorf("expected exit code 2 (ExitConfig), got %d", exitErr.ExitCode())
			}
		} else {
			t.Fatalf("expected *exec.ExitError, got: %T", errSecond)
		}

		// Third init with force must succeed with exit code 0
		cmdForce := exec.Command(binPath, "init", "--force")
		cmdForce.Dir = workDir
		if err := cmdForce.Run(); err != nil {
			t.Fatalf("perfguard init --force failed: %v", err)
		}
	})
}

func TestE2E_Run(t *testing.T) {
	binPath := buildBinary(t)

	fixtureDir, err := filepath.Abs("../../testdata/fixtures/benchmod")
	if err != nil {
		t.Fatalf("failed to resolve fixture directory: %v", err)
	}

	schemaPath, err := filepath.Abs("../../schemas/result.v1.json")
	if err != nil {
		t.Fatalf("failed to resolve schema path: %v", err)
	}

	compiler := jsonschema.NewCompiler()
	sch, err := compiler.Compile(schemaPath)
	if err != nil {
		t.Fatalf("failed compiling result schema: %v", err)
	}

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "perfguard.yaml")
	cfgContent := `version: 1
benchmarks:
  packages: ["./pkgwithtests"]
  count: 2
  benchtime: "10ms"
thresholds:
  min_samples: 2
execution:
  warmup_runs: 0
`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o600); err != nil {
		t.Fatalf("failed writing test config: %v", err)
	}

	t.Run("run head side writes valid result.json", func(t *testing.T) {
		outResult := filepath.Join(tmpDir, "head.result.json")

		cmd := exec.Command(binPath, "run", "--config", cfgPath, "--side", "head", "-o", outResult)
		cmd.Dir = fixtureDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("perfguard run head failed: %v\nOutput: %s", err, string(output))
		}

		if !strings.Contains(string(output), "Recorded 1 benchmarks (head side)") {
			t.Errorf("unexpected output message: %s", string(output))
		}

		data, err := os.ReadFile(outResult)
		if err != nil {
			t.Fatalf("failed reading head result file: %v", err)
		}

		// Validate against JSON Schema
		var rawDoc any
		if err := json.Unmarshal(data, &rawDoc); err != nil {
			t.Fatalf("failed unmarshaling raw doc: %v", err)
		}
		if err := sch.Validate(rawDoc); err != nil {
			t.Fatalf("head result.json failed schema validation: %v", err)
		}

		// Validate unmarshaled model.Run fields
		var run model.Run
		if err := json.Unmarshal(data, &run); err != nil {
			t.Fatalf("failed unmarshaling model.Run: %v", err)
		}

		if run.SchemaVersion != 1 {
			t.Errorf("expected schema version 1, got %d", run.SchemaVersion)
		}
		if run.Side != model.SideHead {
			t.Errorf("expected side head, got %s", run.Side)
		}
		if run.Git.SHA == "" {
			t.Error("expected non-empty Git SHA")
		}
		if run.Env.OS == "" || run.Env.GoVersion == "" {
			t.Errorf("expected non-empty Env metadata: %+v", run.Env)
		}
		if run.Params.Count != 2 {
			t.Errorf("expected count 2, got %d", run.Params.Count)
		}
		if len(run.Benchmarks) != 1 {
			t.Fatalf("expected 1 benchmark, got %d", len(run.Benchmarks))
		}
		bm := run.Benchmarks[0]
		if bm.Name != "BenchmarkFast" {
			t.Errorf("expected benchmark BenchmarkFast, got %s", bm.Name)
		}
		if len(bm.Samples) != 2 {
			t.Fatalf("expected 2 samples, got %d", len(bm.Samples))
		}
		for i, sample := range bm.Samples {
			if sample.Iterations <= 0 {
				t.Errorf("sample %d: expected iterations > 0, got %d", i, sample.Iterations)
			}
			if sample.Values["sec/op"] <= 0 {
				t.Errorf("sample %d: expected sec/op > 0, got %v", i, sample.Values["sec/op"])
			}
		}
	})

	t.Run("run base side writes valid result.json", func(t *testing.T) {
		outResult := filepath.Join(tmpDir, "base.result.json")

		cmd := exec.Command(binPath, "run", "--config", cfgPath, "--side", "base", "-o", outResult)
		cmd.Dir = fixtureDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("perfguard run base failed: %v\nOutput: %s", err, string(output))
		}

		data, err := os.ReadFile(outResult)
		if err != nil {
			t.Fatalf("failed reading base result file: %v", err)
		}

		var rawDoc any
		if err := json.Unmarshal(data, &rawDoc); err != nil {
			t.Fatalf("failed unmarshaling raw doc: %v", err)
		}
		if err := sch.Validate(rawDoc); err != nil {
			t.Fatalf("base result.json failed schema validation: %v", err)
		}

		var run model.Run
		if err := json.Unmarshal(data, &run); err != nil {
			t.Fatalf("failed unmarshaling model.Run: %v", err)
		}
		if run.Side != model.SideBase {
			t.Errorf("expected side base, got %s", run.Side)
		}
	})

	t.Run("run with invalid side fails with exit code 2", func(t *testing.T) {
		cmd := exec.Command(binPath, "run", "--config", cfgPath, "--side", "invalid")
		cmd.Dir = fixtureDir
		err := cmd.Run()
		if err == nil {
			t.Fatal("expected error on invalid side, got nil")
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() != 2 {
				t.Errorf("expected exit code 2, got %d", exitErr.ExitCode())
			}
		} else {
			t.Fatalf("expected *exec.ExitError, got: %T", err)
		}
	})
}

func TestE2E_Compare(t *testing.T) {
	binPath := buildBinary(t)

	baseFixture, err := filepath.Abs("../../testdata/compare/base.result.json")
	if err != nil {
		t.Fatalf("failed to resolve base fixture path: %v", err)
	}

	headFixture, err := filepath.Abs("../../testdata/compare/head.result.json")
	if err != nil {
		t.Fatalf("failed to resolve head fixture path: %v", err)
	}

	goldenPath, err := filepath.Abs("../../testdata/compare/comparison.golden.json")
	if err != nil {
		t.Fatalf("failed to resolve golden path: %v", err)
	}

	schemaPath, err := filepath.Abs("../../schemas/comparison.v1.json")
	if err != nil {
		t.Fatalf("failed to resolve schema path: %v", err)
	}

	compiler := jsonschema.NewCompiler()
	sch, err := compiler.Compile(schemaPath)
	if err != nil {
		t.Fatalf("failed compiling comparison schema: %v", err)
	}

	tmpDir := t.TempDir()
	outComp := filepath.Join(tmpDir, "comparison.json")

	cmd := exec.Command(binPath, "compare", baseFixture, headFixture, "-o", outComp)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("perfguard compare failed: %v\nOutput: %s", err, string(output))
	}

	if !strings.Contains(string(output), "fail verdict") {
		t.Errorf("expected output message to contain 'fail verdict', got: %s", string(output))
	}

	actualData, err := os.ReadFile(outComp)
	if err != nil {
		t.Fatalf("failed reading output comparison file: %v", err)
	}

	// 1. Validate against JSON Schema
	var rawDoc any
	if err := json.Unmarshal(actualData, &rawDoc); err != nil {
		t.Fatalf("failed unmarshaling raw comparison doc: %v", err)
	}
	if err := sch.Validate(rawDoc); err != nil {
		t.Fatalf("comparison.json failed schema validation: %v", err)
	}

	// 2. Validate against golden file
	expectedData, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("failed reading golden comparison file: %v", err)
	}

	var actualCmp, expectedCmp model.Comparison
	if err := json.Unmarshal(actualData, &actualCmp); err != nil {
		t.Fatalf("failed unmarshaling actual comparison: %v", err)
	}
	if err := json.Unmarshal(expectedData, &expectedCmp); err != nil {
		t.Fatalf("failed unmarshaling expected comparison: %v", err)
	}

	if actualCmp.SchemaVersion != expectedCmp.SchemaVersion {
		t.Errorf("schema version mismatch: got %d, want %d", actualCmp.SchemaVersion, expectedCmp.SchemaVersion)
	}
	if actualCmp.Verdict != expectedCmp.Verdict {
		t.Errorf("verdict mismatch: got %s, want %s", actualCmp.Verdict, expectedCmp.Verdict)
	}
	if len(actualCmp.Items) != len(expectedCmp.Items) {
		t.Fatalf("items length mismatch: got %d, want %d", len(actualCmp.Items), len(expectedCmp.Items))
	}
	if actualCmp.Items[0].Benchmark != expectedCmp.Items[0].Benchmark {
		t.Errorf("benchmark mismatch: got %s, want %s", actualCmp.Items[0].Benchmark, expectedCmp.Items[0].Benchmark)
	}
	if actualCmp.Items[0].Status != expectedCmp.Items[0].Status {
		t.Errorf("status mismatch: got %s, want %s", actualCmp.Items[0].Status, expectedCmp.Items[0].Status)
	}
	if actualCmp.Items[0].Reason != expectedCmp.Items[0].Reason {
		t.Errorf("reason mismatch: got %s, want %s", actualCmp.Items[0].Reason, expectedCmp.Items[0].Reason)
	}
	if actualCmp.Items[0].DeltaPct != expectedCmp.Items[0].DeltaPct {
		t.Errorf("delta_pct mismatch: got %v, want %v", actualCmp.Items[0].DeltaPct, expectedCmp.Items[0].DeltaPct)
	}
}
