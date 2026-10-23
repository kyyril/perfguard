package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEvalRunCmd_MissingScenario(t *testing.T) {
	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"eval", "run"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --scenario, got nil")
	}
}

func TestEvalRunCmd_ScenarioNotFound(t *testing.T) {
	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"eval", "run", "--scenario", "/nonexistent/scenario"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent scenario, got nil")
	}
}

func TestEvalRunCmd_ValidScenario(t *testing.T) {
	tmpDir := t.TempDir()
	scenarioDir := filepath.Join(tmpDir, "test-scenario")
	baseDir := filepath.Join(scenarioDir, "base")

	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatalf("create base dir: %v", err)
	}

	goMod := "module example.com/evaltest\n\ngo 1.24\n"
	if err := os.WriteFile(filepath.Join(baseDir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	mainGo := `package evaltest

func Sum(a, b int) int {
	return a + b
}
`
	if err := os.WriteFile(filepath.Join(baseDir, "main.go"), []byte(mainGo), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	benchGo := `package evaltest

import "testing"

func BenchmarkSum(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Sum(1, 2)
	}
}
`
	if err := os.WriteFile(filepath.Join(baseDir, "bench_test.go"), []byte(benchGo), 0o644); err != nil {
		t.Fatalf("write bench_test.go: %v", err)
	}

	patchContent := `diff --git a/main.go b/main.go
index 1234567..abcdefg 100644
--- a/main.go
+++ b/main.go
@@ -1,5 +1,5 @@
 package evaltest

 func Sum(a, b int) int {
-	return a + b
+	return a + b + 0
 }
`
	if err := os.WriteFile(filepath.Join(scenarioDir, "change.patch"), []byte(patchContent), 0o644); err != nil {
		t.Fatalf("write patch: %v", err)
	}

	scenarioYAML := `id: test-scenario
description: Test scenario for CLI
trials: 1
expect:
  verdict: pass
`
	if err := os.WriteFile(filepath.Join(scenarioDir, "scenario.yaml"), []byte(scenarioYAML), 0o644); err != nil {
		t.Fatalf("write scenario.yaml: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")

	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"eval", "run", "--scenario", scenarioDir, "--output", outputDir})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("eval run failed: %v", err)
	}

	resultFile := filepath.Join(outputDir, "test-scenario.json")
	if _, err := os.Stat(resultFile); err != nil {
		t.Errorf("result file not created: %v", err)
	}
}

func TestEvalCmd_HelpOutput(t *testing.T) {
	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"eval", "--help"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("eval --help failed: %v", err)
	}
}

func TestEvalRunCmd_HelpOutput(t *testing.T) {
	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"eval", "run", "--help"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("eval run --help failed: %v", err)
	}
}

func TestEvalRunCmd_InvalidScenario(t *testing.T) {
	tmpDir := t.TempDir()
	scenarioDir := filepath.Join(tmpDir, "bad-scenario")

	if err := os.MkdirAll(scenarioDir, 0o755); err != nil {
		t.Fatalf("create scenario dir: %v", err)
	}

	scenarioYAML := `description: Missing required ID field
trials: 1
`
	if err := os.WriteFile(filepath.Join(scenarioDir, "scenario.yaml"), []byte(scenarioYAML), 0o644); err != nil {
		t.Fatalf("write scenario.yaml: %v", err)
	}

	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"eval", "run", "--scenario", scenarioDir})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid scenario, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "id") && !strings.Contains(errStr, "ID") {
		t.Errorf("expected error about missing ID, got: %v", err)
	}
}
