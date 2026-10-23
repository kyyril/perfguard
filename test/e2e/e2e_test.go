//go:build e2e

package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	pprof "github.com/google/pprof/profile"

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
  include: "^BenchmarkFast$"
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

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\nOutput: %s", strings.Join(args, " "), err, string(out))
	}
	return strings.TrimSpace(string(out))
}

func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("read dir %s: %v", src, err)
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := os.MkdirAll(dstPath, 0o750); err != nil {
				t.Fatalf("mkdir %s: %v", dstPath, err)
			}
			copyDir(t, srcPath, dstPath)
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				t.Fatalf("read file %s: %v", srcPath, err)
			}
			if err := os.WriteFile(dstPath, data, 0o600); err != nil {
				t.Fatalf("write file %s: %v", dstPath, err)
			}
		}
	}
}

func setupBenchmarkGitRepo(t *testing.T) (string, string) {
	t.Helper()
	repoDir := t.TempDir()

	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.email", "perfguard-e2e@example.com")
	runGit(t, repoDir, "config", "user.name", "PerfGuard E2E")
	runGit(t, repoDir, "config", "commit.gpgsign", "false")
	runGit(t, repoDir, "config", "core.autocrlf", "false")

	fixtureDir, err := filepath.Abs("../../testdata/fixtures/benchmod")
	if err != nil {
		t.Fatalf("resolve fixture dir: %v", err)
	}
	copyDir(t, fixtureDir, repoDir)

	cfgContent := `version: 1
benchmarks:
  packages: ["./pkgwithtests"]
  include: "^BenchmarkFast$"
  count: 6
  benchtime: "10ms"
thresholds:
  min_samples: 6
execution:
  warmup_runs: 0
`
	if err := os.WriteFile(filepath.Join(repoDir, "perfguard.yaml"), []byte(cfgContent), 0o600); err != nil {
		t.Fatalf("write perfguard.yaml: %v", err)
	}

	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "initial benchmark base")
	baseSHA := runGit(t, repoDir, "rev-parse", "HEAD")

	return repoDir, baseSHA
}

func TestE2E_Check(t *testing.T) {
	binPath := buildBinary(t)

	t.Run("exit code 0 on no regression", func(t *testing.T) {
		repoDir, baseSHA := setupBenchmarkGitRepo(t)

		cmd := exec.Command(binPath, "check", "--base", baseSHA)
		cmd.Dir = repoDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("expected exit code 0 on pass, got error: %v\nOutput: %s", err, string(output))
		}
		// Accept PASS or INCONCLUSIVE: both exit 0 and indicate no regression.
		if !strings.Contains(string(output), "OVERALL VERDICT: PASS") &&
			!strings.Contains(string(output), "OVERALL VERDICT: INCONCLUSIVE") {
			t.Errorf("expected pass or inconclusive verdict in output, got: %s", string(output))
		}
	})

	t.Run("exit code 1 on regression", func(t *testing.T) {
		repoDir, baseSHA := setupBenchmarkGitRepo(t)

		// Override config: raise max_cv so the 200x regression is analyzed
		// statistically rather than classified as noisy timing.
		regrCfg := `version: 1
benchmarks:
  packages: ["./pkgwithtests"]
  include: "^BenchmarkFast$"
  count: 6
  benchtime: "10ms"
thresholds:
  min_samples: 6
execution:
  warmup_runs: 0
noise:
  max_cv: 200.0
`
		if err := os.WriteFile(filepath.Join(repoDir, "perfguard.yaml"), []byte(regrCfg), 0o600); err != nil {
			t.Fatalf("write regression config: %v", err)
		}

		// Introduce 200x slowdown regression
		workFile := filepath.Join(repoDir, "pkgwithtests", "work.go")
		slowContent := `package pkgwithtests

func DoWork() int {
	sum := 0
	for i := 0; i < 20000; i++ {
		sum += i
	}
	return sum
}
`
		if err := os.WriteFile(workFile, []byte(slowContent), 0o600); err != nil {
			t.Fatalf("failed writing slow code: %v", err)
		}

		cmd := exec.Command(binPath, "check", "--base", baseSHA)
		cmd.Dir = repoDir
		output, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("expected non-zero exit code on regression, got success\nOutput: %s", string(output))
		}

		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("expected *exec.ExitError, got: %T", err)
		}
		if exitErr.ExitCode() != 1 {
			t.Errorf("expected exit code 1, got %d", exitErr.ExitCode())
		}
		if !strings.Contains(string(output), "OVERALL VERDICT: FAIL") {
			t.Errorf("expected output to mention OVERALL VERDICT: FAIL, got: %s", string(output))
		}
		if !strings.Contains(string(output), "BenchmarkFast") {
			t.Errorf("expected output to mention BenchmarkFast, got: %s", string(output))
		}
	})

	t.Run("exit code 1 on fail-on warn", func(t *testing.T) {
		repoDir, baseSHA := setupBenchmarkGitRepo(t)

		// Set warn threshold very low (0.01%) and fail threshold very high (500%)
		// so that slight variation yields warn verdict
		cfgContent := `version: 1
benchmarks:
  packages: ["./pkgwithtests"]
  include: "^BenchmarkFast$"
  count: 6
  benchtime: "10ms"
thresholds:
  min_samples: 6
  overrides:
    - match: ".*"
      metric: "sec/op"
      warn: 0.001
      fail: 500.0
execution:
  warmup_runs: 0
noise:
  max_cv: 200.0
`
		if err := os.WriteFile(filepath.Join(repoDir, "perfguard.yaml"), []byte(cfgContent), 0o600); err != nil {
			t.Fatalf("write perfguard.yaml: %v", err)
		}

		// Make head slightly slower (e.g. 150 iterations instead of 100)
		workFile := filepath.Join(repoDir, "pkgwithtests", "work.go")
		modContent := `package pkgwithtests

func DoWork() int {
	sum := 0
	for i := 0; i < 150; i++ {
		sum += i
	}
	return sum
}
`
		if err := os.WriteFile(workFile, []byte(modContent), 0o600); err != nil {
			t.Fatalf("failed writing mod work: %v", err)
		}

		// With --fail-on warn: must exit 1
		cmdWarn := exec.Command(binPath, "check", "--base", baseSHA, "--fail-on", "warn")
		cmdWarn.Dir = repoDir
		outWarn, errWarn := cmdWarn.CombinedOutput()
		if errWarn == nil {
			t.Fatalf("expected non-zero exit code with --fail-on warn, got 0\nOutput: %s", string(outWarn))
		}
		if exitErr, ok := errWarn.(*exec.ExitError); ok {
			if exitErr.ExitCode() != 1 {
				t.Errorf("expected exit code 1 with --fail-on warn, got %d", exitErr.ExitCode())
			}
		}

		// With --fail-on fail: must exit 0
		cmdFail := exec.Command(binPath, "check", "--base", baseSHA, "--fail-on", "fail")
		cmdFail.Dir = repoDir
		outFail, errFail := cmdFail.CombinedOutput()
		if errFail != nil {
			t.Fatalf("expected exit code 0 with --fail-on fail, got %v\nOutput: %s", errFail, string(outFail))
		}
	})

	t.Run("exit code 2 on invalid config", func(t *testing.T) {
		repoDir, baseSHA := setupBenchmarkGitRepo(t)

		badCfg := filepath.Join(repoDir, "bad.yaml")
		_ = os.WriteFile(badCfg, []byte("version: 1\nbenchmarks:\n  count: -5\n"), 0o600)

		cmd := exec.Command(binPath, "check", "--config", badCfg, "--base", baseSHA)
		cmd.Dir = repoDir
		output, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("expected error on invalid config, got nil\nOutput: %s", string(output))
		}

		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("expected *exec.ExitError, got: %T", err)
		}
		if exitErr.ExitCode() != 2 {
			t.Errorf("expected exit code 2 (ExitConfig), got %d", exitErr.ExitCode())
		}
		if !strings.Contains(string(output), "benchmarks.count") {
			t.Errorf("expected message to mention 'benchmarks.count', got: %s", string(output))
		}
	})

	t.Run("exit code 3 on build failure on head", func(t *testing.T) {
		repoDir, baseSHA := setupBenchmarkGitRepo(t)

		// Break syntax in head
		workFile := filepath.Join(repoDir, "pkgwithtests", "work.go")
		brokenCode := `package pkgwithtests
invalid syntax here !!!
`
		_ = os.WriteFile(workFile, []byte(brokenCode), 0o600)

		cmd := exec.Command(binPath, "check", "--base", baseSHA)
		cmd.Dir = repoDir
		output, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("expected build failure, got success\nOutput: %s", string(output))
		}

		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("expected *exec.ExitError, got: %T", err)
		}
		if exitErr.ExitCode() != 3 {
			t.Errorf("expected exit code 3 (ExitExecution), got %d\nOutput: %s", exitErr.ExitCode(), string(output))
		}
	})

	t.Run("exit code 4 on strict with noisy result", func(t *testing.T) {
		repoDir, baseSHA := setupBenchmarkGitRepo(t)

		// Set max_cv to impossible threshold so result is marked noisy/inconclusive
		noisyCfg := `version: 1
benchmarks:
  packages: ["./pkgwithtests"]
  count: 6
  benchtime: "10ms"
thresholds:
  min_samples: 6
noise:
  max_cv: 0.0000001
  on_noisy: "inconclusive"
execution:
  warmup_runs: 0
  max_extra_runs: 0
`
		_ = os.WriteFile(filepath.Join(repoDir, "perfguard.yaml"), []byte(noisyCfg), 0o600)

		// With --strict: exit code 4
		cmdStrict := exec.Command(binPath, "check", "--base", baseSHA, "--strict")
		cmdStrict.Dir = repoDir
		outStrict, errStrict := cmdStrict.CombinedOutput()
		if errStrict == nil {
			t.Fatalf("expected exit code 4 with --strict on noisy, got 0\nOutput: %s", string(outStrict))
		}
		if exitErr, ok := errStrict.(*exec.ExitError); ok {
			if exitErr.ExitCode() != 4 {
				t.Errorf("expected exit code 4 (ExitInconclusive), got %d\nOutput: %s", exitErr.ExitCode(), string(outStrict))
			}
		}

		// Without --strict: exit code 0
		cmdNonStrict := exec.Command(binPath, "check", "--base", baseSHA)
		cmdNonStrict.Dir = repoDir
		outNonStrict, errNonStrict := cmdNonStrict.CombinedOutput()
		if errNonStrict != nil {
			t.Fatalf("expected exit code 0 without --strict, got error: %v\nOutput: %s", errNonStrict, string(outNonStrict))
		}
	})
}

func TestE2E_QuickStart(t *testing.T) {
	binPath := buildBinary(t)

	repoDir := t.TempDir()

	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.email", "quickstart@example.com")
	runGit(t, repoDir, "config", "user.name", "QuickStart Test")
	runGit(t, repoDir, "config", "commit.gpgsign", "false")
	runGit(t, repoDir, "config", "core.autocrlf", "false")

	fixtureDir, err := filepath.Abs("../../testdata/fixtures/benchmod")
	if err != nil {
		t.Fatalf("resolve fixture dir: %v", err)
	}
	copyDir(t, fixtureDir, repoDir)

	// Step 1: Run perfguard init
	cmdInit := exec.Command(binPath, "init")
	cmdInit.Dir = repoDir
	if out, err := cmdInit.CombinedOutput(); err != nil {
		t.Fatalf("perfguard init failed: %v\nOutput: %s", err, string(out))
	}

	configPath := filepath.Join(repoDir, "perfguard.yaml")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("perfguard.yaml was not created: %v", err)
	}

	// Update count and benchtime for faster test execution.
	// include restricts to BenchmarkFast to avoid noisy sec/op from BenchmarkCounted
	// and BenchmarkMixed (added for integration tests) producing INCONCLUSIVE.
	fastCfg := `version: 1
benchmarks:
  packages: ["./pkgwithtests"]
  include: "^BenchmarkFast$"
  count: 6
  benchtime: "10ms"
thresholds:
  min_samples: 6
execution:
  warmup_runs: 0
`
	if err := os.WriteFile(configPath, []byte(fastCfg), 0o600); err != nil {
		t.Fatalf("write updated perfguard.yaml: %v", err)
	}

	// Step 2: Commit base code
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "base commit")
	baseSHA := runGit(t, repoDir, "rev-parse", "HEAD")

	// Step 3: Run perfguard check --base <baseSHA>
	cmdCheck := exec.Command(binPath, "check", "--base", baseSHA)
	cmdCheck.Dir = repoDir
	output, err := cmdCheck.CombinedOutput()
	if err != nil {
		t.Fatalf("perfguard check failed on clean quick start: %v\nOutput: %s", err, string(output))
	}

	// Both PASS and INCONCLUSIVE exit 0 and indicate no regression detected.
	if !strings.Contains(string(output), "OVERALL VERDICT: PASS") &&
		!strings.Contains(string(output), "OVERALL VERDICT: INCONCLUSIVE") {
		t.Errorf("expected OVERALL VERDICT: PASS in quick start output, got: %s", string(output))
	}
}

func TestE2E_BaselineAndStoredMode(t *testing.T) {
	binPath := buildBinary(t)
	repoDir, baseSHA := setupBenchmarkGitRepo(t)

	// Step 1: Run baseline save on head side with fixture
	// First generate head.result.json using perfguard run
	headResult := filepath.Join(repoDir, "head.result.json")
	cmdRun := exec.Command(binPath, "run", "--side", "head", "-o", headResult)
	cmdRun.Dir = repoDir
	if out, err := cmdRun.CombinedOutput(); err != nil {
		t.Fatalf("perfguard run failed: %v\nOutput: %s", err, string(out))
	}

	// Save baseline from head.result.json
	cmdSave := exec.Command(binPath, "baseline", "save", headResult)
	cmdSave.Dir = repoDir
	if out, err := cmdSave.CombinedOutput(); err != nil {
		t.Fatalf("perfguard baseline save failed: %v\nOutput: %s", err, string(out))
	}

	// Verify baseline list
	cmdList := exec.Command(binPath, "baseline", "list")
	cmdList.Dir = repoDir
	outList, err := cmdList.CombinedOutput()
	if err != nil {
		t.Fatalf("perfguard baseline list failed: %v\nOutput: %s", err, string(outList))
	}
	if !strings.Contains(string(outList), "COMMIT") {
		t.Errorf("expected header COMMIT in list output, got: %s", string(outList))
	}

	// Step 2: Test matching environment stored check (AC-09 match).
	// Exit code 0 is the contract: PASS or INCONCLUSIVE are both valid
	// because sec/op timing is environment-dependent.
	cmdCheckMatch := exec.Command(binPath, "check", "--mode", "stored", "--base", baseSHA)
	cmdCheckMatch.Dir = repoDir
	outMatch, errMatch := cmdCheckMatch.CombinedOutput()
	if errMatch != nil {
		t.Fatalf("perfguard check --mode stored failed on matching env: %v\nOutput: %s", errMatch, string(outMatch))
	}
	// Accept PASS or INCONCLUSIVE: both indicate no regression detected, and both exit 0.
	if !strings.Contains(string(outMatch), "OVERALL VERDICT: PASS") &&
		!strings.Contains(string(outMatch), "OVERALL VERDICT: INCONCLUSIVE") {
		t.Errorf("expected pass or inconclusive verdict on matching stored check, got: %s", string(outMatch))
	}

	// Step 3: Test AC-09 mismatch (different Go version).
	// Switch to an fs store pointing at an isolated directory so baseline selection is
	// deterministic. The fs store sorts newest-first; there is exactly one entry.
	mismatchCfgContent := `version: 1
benchmarks:
  packages: ["./pkgwithtests"]
  include: "^BenchmarkFast$"
  count: 6
  benchtime: "10ms"
thresholds:
  min_samples: 6
execution:
  warmup_runs: 0
baseline:
  store: fs
  dir: .perfguard/mismatch-baselines
`
	if err := os.WriteFile(filepath.Join(repoDir, "perfguard.yaml"), []byte(mismatchCfgContent), 0o600); err != nil {
		t.Fatalf("write mismatch config: %v", err)
	}

	data, err := os.ReadFile(headResult)
	if err != nil {
		t.Fatal(err)
	}
	var mutatedRun model.Run
	if err := json.Unmarshal(data, &mutatedRun); err != nil {
		t.Fatal(err)
	}
	mutatedRun.Env.GoVersion = "go1.20.0"
	mutatedData, _ := json.Marshal(mutatedRun)
	mismatchResult := filepath.Join(repoDir, "mismatch.result.json")
	if err := os.WriteFile(mismatchResult, mutatedData, 0o600); err != nil {
		t.Fatal(err)
	}

	cmdSaveMismatch := exec.Command(binPath, "baseline", "save", mismatchResult)
	cmdSaveMismatch.Dir = repoDir
	if out, err := cmdSaveMismatch.CombinedOutput(); err != nil {
		t.Fatalf("save mismatch baseline failed: %v\nOutput: %s", err, string(out))
	}

	// check --mode stored --strict: isolated store has only the mismatch baseline.
	// Must exit 4 per AC-09 because env.GoVersion differs.
	cmdStrict := exec.Command(binPath, "check", "--mode", "stored", "--strict")
	cmdStrict.Dir = repoDir
	outStrict, errStrict := cmdStrict.CombinedOutput()
	if errStrict == nil {
		t.Fatalf("expected non-zero exit on env mismatch with --strict, got 0\nOutput: %s", string(outStrict))
	}
	if exitErr, ok := errStrict.(*exec.ExitError); ok {
		if exitErr.ExitCode() != 4 {
			t.Errorf("expected exit code 4 (ExitInconclusive), got %d\nOutput: %s", exitErr.ExitCode(), string(outStrict))
		}
	} else {
		t.Fatalf("expected *exec.ExitError, got %T", errStrict)
	}

	if !strings.Contains(string(outStrict), "INCONCLUSIVE") && !strings.Contains(string(outStrict), "inconclusive") {
		t.Errorf("expected inconclusive in output, got: %s", string(outStrict))
	}
	if !strings.Contains(string(outStrict), "env_mismatch") && !strings.Contains(string(outStrict), "Go minor version mismatch") {
		t.Errorf("expected env_mismatch or Go minor version mismatch in output, got: %s", string(outStrict))
	}
}

// writeE2EFixtureProfile writes a minimal synthetic pprof profile to a .pb.gz file
// with one sample type and one sample attributed to funcName with the given flat value.
func writeE2EFixtureProfile(t *testing.T, dir, name, funcName string, flatValue int64) string {
	t.Helper()
	fn := &pprof.Function{
		ID:       1,
		Name:     funcName,
		Filename: "bench_test.go",
	}
	loc := &pprof.Location{
		ID:   1,
		Line: []pprof.Line{{Function: fn}},
	}
	prof := &pprof.Profile{
		SampleType: []*pprof.ValueType{
			{Type: "samples", Unit: "count"},
		},
		PeriodType: &pprof.ValueType{Type: "cpu", Unit: "nanoseconds"},
		Sample: []*pprof.Sample{
			{Location: []*pprof.Location{loc}, Value: []int64{flatValue}},
		},
		Function: []*pprof.Function{fn},
		Location: []*pprof.Location{loc},
	}
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create fixture profile %s: %v", path, err)
	}
	defer f.Close()
	// Profile.Write gzip-compresses internally; do not wrap in another gzip.Writer.
	if err := prof.Write(f); err != nil {
		t.Fatalf("write fixture profile %s: %v", path, err)
	}
	return path
}

func TestE2E_ProfileDiff(t *testing.T) {
	binPath := buildBinary(t)

	dir := t.TempDir()

	// Write base profile: BenchmarkTarget with 100 samples, BenchmarkStable with 50 samples.
	baseFn1 := &pprof.Function{ID: 1, Name: "BenchmarkTarget", Filename: "bench_test.go"}
	baseFn2 := &pprof.Function{ID: 2, Name: "BenchmarkStable", Filename: "bench_test.go"}
	baseLoc1 := &pprof.Location{ID: 1, Line: []pprof.Line{{Function: baseFn1}}}
	baseLoc2 := &pprof.Location{ID: 2, Line: []pprof.Line{{Function: baseFn2}}}
	baseProf := &pprof.Profile{
		SampleType: []*pprof.ValueType{{Type: "samples", Unit: "count"}},
		PeriodType: &pprof.ValueType{Type: "cpu", Unit: "nanoseconds"},
		Sample: []*pprof.Sample{
			{Location: []*pprof.Location{baseLoc1}, Value: []int64{100}},
			{Location: []*pprof.Location{baseLoc2}, Value: []int64{50}},
		},
		Function: []*pprof.Function{baseFn1, baseFn2},
		Location: []*pprof.Location{baseLoc1, baseLoc2},
	}
	basePath := filepath.Join(dir, "base.pb.gz")
	{
		f, err := os.Create(basePath)
		if err != nil {
			t.Fatal(err)
		}
		// Profile.Write gzip-compresses internally.
		if err := baseProf.Write(f); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}

	// Write head profile: BenchmarkTarget with 300 samples (regression), BenchmarkStable with 50 samples.
	headFn1 := &pprof.Function{ID: 1, Name: "BenchmarkTarget", Filename: "bench_test.go"}
	headFn2 := &pprof.Function{ID: 2, Name: "BenchmarkStable", Filename: "bench_test.go"}
	headLoc1 := &pprof.Location{ID: 1, Line: []pprof.Line{{Function: headFn1}}}
	headLoc2 := &pprof.Location{ID: 2, Line: []pprof.Line{{Function: headFn2}}}
	headProf := &pprof.Profile{
		SampleType: []*pprof.ValueType{{Type: "samples", Unit: "count"}},
		PeriodType: &pprof.ValueType{Type: "cpu", Unit: "nanoseconds"},
		Sample: []*pprof.Sample{
			{Location: []*pprof.Location{headLoc1}, Value: []int64{300}},
			{Location: []*pprof.Location{headLoc2}, Value: []int64{50}},
		},
		Function: []*pprof.Function{headFn1, headFn2},
		Location: []*pprof.Location{headLoc1, headLoc2},
	}
	headPath := filepath.Join(dir, "head.pb.gz")
	{
		f, err := os.Create(headPath)
		if err != nil {
			t.Fatal(err)
		}
		// Profile.Write gzip-compresses internally.
		if err := headProf.Write(f); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}

	t.Run("text format lists candidates", func(t *testing.T) {
		cmd := exec.Command(binPath, "profile", "diff",
			basePath, headPath,
			"--ops-base", "100",
			"--ops-head", "100",
			"--format", "text",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("profile diff text failed: %v\nOutput: %s", err, string(out))
		}
		outStr := string(out)

		// BenchmarkTarget should appear with a positive flat delta (+2.0000)
		// base: 100/100=1.0, head: 300/100=3.0, delta=+2.0
		if !strings.Contains(outStr, "BenchmarkTarget") {
			t.Errorf("expected BenchmarkTarget in output, got:\n%s", outStr)
		}
		if !strings.Contains(outStr, "+2.0000") {
			t.Errorf("expected +2.0000 flat delta in output, got:\n%s", outStr)
		}
		// BenchmarkStable is identical across base and head (50/100=0.5 each), delta=0 -> omitted
		if strings.Contains(outStr, "BenchmarkStable") {
			t.Errorf("expected BenchmarkStable to be omitted (no change), got:\n%s", outStr)
		}
	})

	t.Run("json format parses correctly", func(t *testing.T) {
		cmd := exec.Command(binPath, "profile", "diff",
			basePath, headPath,
			"--ops-base", "100",
			"--ops-head", "100",
			"--format", "json",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("profile diff json failed: %v\nOutput: %s", err, string(out))
		}

		type jsonOutput struct {
			SampleType string            `json:"sample_type"`
			Candidates []model.Candidate `json:"candidates"`
		}
		var got jsonOutput
		if err := json.Unmarshal(out, &got); err != nil {
			t.Fatalf("invalid JSON: %v\nOutput:\n%s", err, string(out))
		}
		if got.SampleType == "" {
			t.Error("expected non-empty sample_type in JSON")
		}
		found := false
		for _, c := range got.Candidates {
			if c.Function == "BenchmarkTarget" {
				found = true
				if c.FlatDelta < 1.99 || c.FlatDelta > 2.01 {
					t.Errorf("BenchmarkTarget FlatDelta: want ~2.0, got %f", c.FlatDelta)
				}
			}
		}
		if !found {
			t.Errorf("BenchmarkTarget not found in JSON candidates: %+v", got.Candidates)
		}
	})

	t.Run("missing args exits non-zero", func(t *testing.T) {
		cmd := exec.Command(binPath, "profile", "diff", "only-one.pb.gz")
		if err := cmd.Run(); err == nil {
			t.Error("expected non-zero exit when only one argument provided")
		}
	})
}
