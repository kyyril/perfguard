package eval

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/kyyril/perfguard/internal/model"
)

func TestLoadScenario_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	scenarioDir := filepath.Join(tmpDir, "test-scenario")

	if err := os.MkdirAll(filepath.Join(scenarioDir, "base"), 0o755); err != nil {
		t.Fatalf("create base dir: %v", err)
	}

	patchContent := `diff --git a/main.go b/main.go
index 1234567..abcdefg 100644
--- a/main.go
+++ b/main.go
@@ -1,3 +1,4 @@
 package main

 func main() {}
+// comment
`
	if err := os.WriteFile(filepath.Join(scenarioDir, "change.patch"), []byte(patchContent), 0o600); err != nil {
		t.Fatalf("write patch: %v", err)
	}

	scenarioYAML := `id: test-scenario
description: A test scenario
trials: 10
expect:
  verdict: fail
  metrics:
    - benchmark: BenchmarkTest
      metric: sec/op
      status: fail
`
	if err := os.WriteFile(filepath.Join(scenarioDir, "scenario.yaml"), []byte(scenarioYAML), 0o600); err != nil {
		t.Fatalf("write scenario.yaml: %v", err)
	}

	s, err := LoadScenario(scenarioDir)
	if err != nil {
		t.Fatalf("LoadScenario: %v", err)
	}

	if s.ID != "test-scenario" {
		t.Errorf("ID = %q, want %q", s.ID, "test-scenario")
	}
	if s.Description != "A test scenario" {
		t.Errorf("Description = %q, want %q", s.Description, "A test scenario")
	}
	if s.Trials != 10 {
		t.Errorf("Trials = %d, want %d", s.Trials, 10)
	}
	if s.Expect.Verdict != model.VerdictFail {
		t.Errorf("Expect.Verdict = %q, want %q", s.Expect.Verdict, model.VerdictFail)
	}
	if len(s.Expect.Metrics) != 1 {
		t.Fatalf("len(Expect.Metrics) = %d, want 1", len(s.Expect.Metrics))
	}
	if s.Expect.Metrics[0].Benchmark != "BenchmarkTest" {
		t.Errorf("Metrics[0].Benchmark = %q, want %q", s.Expect.Metrics[0].Benchmark, "BenchmarkTest")
	}
	if s.Directory != scenarioDir {
		t.Errorf("Directory = %q, want %q", s.Directory, scenarioDir)
	}
}

func TestLoadScenario_MissingID(t *testing.T) {
	tmpDir := t.TempDir()
	scenarioDir := filepath.Join(tmpDir, "no-id")

	if err := os.MkdirAll(filepath.Join(scenarioDir, "base"), 0o755); err != nil {
		t.Fatalf("create base dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(scenarioDir, "change.patch"), []byte("patch"), 0o600); err != nil {
		t.Fatalf("write patch: %v", err)
	}

	scenarioYAML := `description: No ID scenario
trials: 1
`
	if err := os.WriteFile(filepath.Join(scenarioDir, "scenario.yaml"), []byte(scenarioYAML), 0o600); err != nil {
		t.Fatalf("write scenario.yaml: %v", err)
	}

	_, err := LoadScenario(scenarioDir)
	if err == nil {
		t.Fatal("LoadScenario succeeded, want error for missing ID")
	}
	if !isScenarioError(err) {
		t.Errorf("expected ErrScenario, got: %v", err)
	}
}

func TestLoadScenario_MissingBaseDir(t *testing.T) {
	tmpDir := t.TempDir()
	scenarioDir := filepath.Join(tmpDir, "no-base")

	if err := os.MkdirAll(scenarioDir, 0o755); err != nil {
		t.Fatalf("create scenario dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(scenarioDir, "change.patch"), []byte("patch"), 0o600); err != nil {
		t.Fatalf("write patch: %v", err)
	}

	scenarioYAML := `id: no-base
description: Missing base directory
trials: 1
`
	if err := os.WriteFile(filepath.Join(scenarioDir, "scenario.yaml"), []byte(scenarioYAML), 0o600); err != nil {
		t.Fatalf("write scenario.yaml: %v", err)
	}

	_, err := LoadScenario(scenarioDir)
	if err == nil {
		t.Fatal("LoadScenario succeeded, want error for missing base/ directory")
	}
	if !isScenarioError(err) {
		t.Errorf("expected ErrScenario, got: %v", err)
	}
}

func TestLoadScenario_MissingPatch(t *testing.T) {
	tmpDir := t.TempDir()
	scenarioDir := filepath.Join(tmpDir, "no-patch")

	if err := os.MkdirAll(filepath.Join(scenarioDir, "base"), 0o755); err != nil {
		t.Fatalf("create base dir: %v", err)
	}

	scenarioYAML := `id: no-patch
description: Missing change.patch
trials: 1
`
	if err := os.WriteFile(filepath.Join(scenarioDir, "scenario.yaml"), []byte(scenarioYAML), 0o600); err != nil {
		t.Fatalf("write scenario.yaml: %v", err)
	}

	_, err := LoadScenario(scenarioDir)
	if err == nil {
		t.Fatal("LoadScenario succeeded, want error for missing change.patch")
	}
	if !isScenarioError(err) {
		t.Errorf("expected ErrScenario, got: %v", err)
	}
}

func TestLoadScenario_InvalidVerdict(t *testing.T) {
	tmpDir := t.TempDir()
	scenarioDir := filepath.Join(tmpDir, "invalid-verdict")

	if err := os.MkdirAll(filepath.Join(scenarioDir, "base"), 0o755); err != nil {
		t.Fatalf("create base dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(scenarioDir, "change.patch"), []byte("patch"), 0o600); err != nil {
		t.Fatalf("write patch: %v", err)
	}

	scenarioYAML := `id: invalid-verdict
description: Invalid verdict
trials: 1
expect:
  verdict: unknown-verdict
`
	if err := os.WriteFile(filepath.Join(scenarioDir, "scenario.yaml"), []byte(scenarioYAML), 0o600); err != nil {
		t.Fatalf("write scenario.yaml: %v", err)
	}

	_, err := LoadScenario(scenarioDir)
	if err == nil {
		t.Fatal("LoadScenario succeeded, want error for invalid verdict")
	}
	if !isScenarioError(err) {
		t.Errorf("expected ErrScenario, got: %v", err)
	}
}

func TestLoadScenario_DefaultTrials(t *testing.T) {
	tmpDir := t.TempDir()
	scenarioDir := filepath.Join(tmpDir, "default-trials")

	if err := os.MkdirAll(filepath.Join(scenarioDir, "base"), 0o755); err != nil {
		t.Fatalf("create base dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(scenarioDir, "change.patch"), []byte("patch"), 0o600); err != nil {
		t.Fatalf("write patch: %v", err)
	}

	scenarioYAML := `id: default-trials
description: No trials specified
expect:
  verdict: pass
`
	if err := os.WriteFile(filepath.Join(scenarioDir, "scenario.yaml"), []byte(scenarioYAML), 0o600); err != nil {
		t.Fatalf("write scenario.yaml: %v", err)
	}

	s, err := LoadScenario(scenarioDir)
	if err != nil {
		t.Fatalf("LoadScenario: %v", err)
	}

	if s.Trials != 1 {
		t.Errorf("Trials = %d, want 1 (default)", s.Trials)
	}
}

func TestListScenarios(t *testing.T) {
	tmpDir := t.TempDir()
	evalRoot := tmpDir
	scenariosDir := filepath.Join(evalRoot, "scenarios")

	createScenario := func(name, id string) {
		dir := filepath.Join(scenariosDir, name)
		if err := os.MkdirAll(filepath.Join(dir, "base"), 0o755); err != nil {
			t.Fatalf("create base dir for %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "change.patch"), []byte("patch"), 0o600); err != nil {
			t.Fatalf("write patch for %s: %v", name, err)
		}
		yaml := "id: " + id + "\ndescription: Test\ntrials: 1\nexpect:\n  verdict: pass\n"
		if err := os.WriteFile(filepath.Join(dir, "scenario.yaml"), []byte(yaml), 0o600); err != nil {
			t.Fatalf("write scenario.yaml for %s: %v", name, err)
		}
	}

	createScenario("scenario-a", "scenario-a")
	createScenario("scenario-b", "scenario-b")

	regularFile := filepath.Join(scenariosDir, "not-a-scenario.txt")
	if err := os.WriteFile(regularFile, []byte("ignore"), 0o600); err != nil {
		t.Fatalf("write regular file: %v", err)
	}

	scenarios, err := ListScenarios(evalRoot)
	if err != nil {
		t.Fatalf("ListScenarios: %v", err)
	}

	if len(scenarios) != 2 {
		t.Fatalf("len(scenarios) = %d, want 2", len(scenarios))
	}

	ids := make(map[string]bool)
	for _, s := range scenarios {
		ids[s.ID] = true
	}

	if !ids["scenario-a"] {
		t.Errorf("scenario-a not found")
	}
	if !ids["scenario-b"] {
		t.Errorf("scenario-b not found")
	}
}

func TestLoadScenario_WithDiagnosis(t *testing.T) {
	tmpDir := t.TempDir()
	scenarioDir := filepath.Join(tmpDir, "with-diagnosis")

	if err := os.MkdirAll(filepath.Join(scenarioDir, "base"), 0o755); err != nil {
		t.Fatalf("create base dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(scenarioDir, "change.patch"), []byte("patch"), 0o600); err != nil {
		t.Fatalf("write patch: %v", err)
	}

	scenarioYAML := `id: with-diagnosis
description: Scenario with diagnosis expectation
trials: 5
expect:
  verdict: fail
  diagnosis:
    profile: cpu
    top_k: 5
    function: example.com/pkg.SlowFunction
`
	if err := os.WriteFile(filepath.Join(scenarioDir, "scenario.yaml"), []byte(scenarioYAML), 0o600); err != nil {
		t.Fatalf("write scenario.yaml: %v", err)
	}

	s, err := LoadScenario(scenarioDir)
	if err != nil {
		t.Fatalf("LoadScenario: %v", err)
	}

	if s.Expect.Diagnosis == nil {
		t.Fatal("Expect.Diagnosis is nil, want non-nil")
	}
	want := DiagnosisExpect{
		Profile:  "cpu",
		TopK:     5,
		Function: "example.com/pkg.SlowFunction",
	}
	if diff := cmp.Diff(want, *s.Expect.Diagnosis); diff != "" {
		t.Errorf("Diagnosis mismatch (-want +got):\n%s", diff)
	}
}

func isScenarioError(err error) bool {
	for err != nil {
		if err == ErrScenario {
			return true
		}
		type unwrapper interface {
			Unwrap() error
		}
		u, ok := err.(unwrapper)
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
