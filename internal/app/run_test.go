package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kyyril/perfguard/internal/app"
	"github.com/kyyril/perfguard/internal/config"
	"github.com/kyyril/perfguard/internal/gitx"
	"github.com/kyyril/perfguard/internal/model"
	"github.com/kyyril/perfguard/internal/runner"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

type fakeRunner struct {
	buildErr     error
	runRoundsErr error
	binaries     []runner.TestBinary
	roundResults runner.RoundResults
	builtSpec    runner.BuildSpec
	roundSpec    runner.RoundSpec
}

func (f *fakeRunner) Build(_ context.Context, spec runner.BuildSpec) ([]runner.TestBinary, error) {
	f.builtSpec = spec
	if f.buildErr != nil {
		return nil, f.buildErr
	}
	return f.binaries, nil
}

func (f *fakeRunner) Execute(_ context.Context, _ runner.TestBinary, _ runner.ExecSpec) (runner.RawOutput, error) {
	return runner.RawOutput{}, nil
}

func (f *fakeRunner) RunRounds(_ context.Context, spec runner.RoundSpec) (runner.RoundResults, error) {
	f.roundSpec = spec
	if f.runRoundsErr != nil {
		return runner.RoundResults{}, f.runRoundsErr
	}
	return f.roundResults, nil
}

type fakeVCS struct {
	gitInfo model.GitInfo
	infoErr error
}

func (f *fakeVCS) CurrentInfo(_ context.Context) (model.GitInfo, error) {
	if f.infoErr != nil {
		return model.GitInfo{}, f.infoErr
	}
	return f.gitInfo, nil
}

func (f *fakeVCS) Resolve(_ context.Context, _ string) (gitx.Commit, error) {
	return gitx.Commit{}, nil
}

func sampleBenchmarkResults() []model.BenchmarkResult {
	return []model.BenchmarkResult{
		{
			Package: "example.com/testpkg",
			Name:    "BenchmarkWork",
			Procs:   4,
			Samples: []model.Sample{
				{
					Iterations: 1000,
					Values: map[string]float64{
						"sec/op":    0.000012,
						"B/op":      256,
						"allocs/op": 3,
					},
				},
				{
					Iterations: 1000,
					Values: map[string]float64{
						"sec/op":    0.000013,
						"B/op":      256,
						"allocs/op": 3,
					},
				},
			},
		},
	}
}

func TestRun_SuccessHead(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "head.result.json")

	cfg := config.Default()
	cfg.Benchmarks.Count = 2
	cfg.Benchmarks.Benchtime = "10ms"

	expectedGit := model.GitInfo{
		SHA:   "11223344556677889900aabbccddeeff11223344",
		Ref:   "refs/heads/feature-speed",
		Dirty: true,
	}

	benchmarks := sampleBenchmarkResults()
	r := &fakeRunner{
		binaries: []runner.TestBinary{{Package: "example.com/testpkg", Path: "bin"}},
		roundResults: runner.RoundResults{
			HeadResults: benchmarks,
		},
	}
	vcs := &fakeVCS{gitInfo: expectedGit}

	runResult, outFile, err := app.Run(context.Background(), app.RunOptions{
		Config:    cfg,
		WorkDir:   tmpDir,
		Side:      model.SideHead,
		OutputDir: outPath,
		Runner:    r,
		Git:       vcs,
	})
	if err != nil {
		t.Fatalf("unexpected app.Run error: %v", err)
	}

	if outFile != outPath {
		t.Errorf("expected outFile %s, got %s", outPath, outFile)
	}

	if runResult.SchemaVersion != 1 {
		t.Errorf("expected schema_version 1, got %d", runResult.SchemaVersion)
	}
	if runResult.Side != model.SideHead {
		t.Errorf("expected side head, got %s", runResult.Side)
	}
	if runResult.Git.SHA != expectedGit.SHA || runResult.Git.Dirty != expectedGit.Dirty {
		t.Errorf("expected git info %+v, got %+v", expectedGit, runResult.Git)
	}
	if len(runResult.Benchmarks) != 1 {
		t.Fatalf("expected 1 benchmark result, got %d", len(runResult.Benchmarks))
	}
	if runResult.Benchmarks[0].Name != "BenchmarkWork" {
		t.Errorf("expected BenchmarkWork, got %s", runResult.Benchmarks[0].Name)
	}

	// Verify written file exists and contains valid JSON
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("failed reading result file: %v", err)
	}

	var parsed model.Run
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed unmarshaling written json: %v", err)
	}
	if parsed.ID != runResult.ID {
		t.Errorf("expected id %s, got %s", runResult.ID, parsed.ID)
	}

	// Validate against schemas/result.v1.json
	schemaPath, err := filepath.Abs("../../schemas/result.v1.json")
	if err != nil {
		t.Fatalf("failed to resolve schema path: %v", err)
	}

	compiler := jsonschema.NewCompiler()
	sch, err := compiler.Compile(schemaPath)
	if err != nil {
		t.Fatalf("failed compiling result schema: %v", err)
	}

	var rawDoc any
	if err := json.Unmarshal(data, &rawDoc); err != nil {
		t.Fatalf("failed unmarshaling to raw doc: %v", err)
	}
	if err := sch.Validate(rawDoc); err != nil {
		t.Errorf("generated result.json failed schema validation: %v", err)
	}
}

func TestRun_SuccessBase(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	outDir := filepath.Join(tmpDir, "runs")

	benchmarks := sampleBenchmarkResults()
	r := &fakeRunner{
		binaries: []runner.TestBinary{{Package: "example.com/testpkg", Path: "bin"}},
		roundResults: runner.RoundResults{
			BaseResults: benchmarks,
		},
	}
	vcs := &fakeVCS{
		gitInfo: model.GitInfo{SHA: "base1234", Ref: "main", Dirty: false},
	}

	runResult, outFile, err := app.Run(context.Background(), app.RunOptions{
		Side:      model.SideBase,
		OutputDir: outDir,
		Runner:    r,
		Git:       vcs,
	})
	if err != nil {
		t.Fatalf("unexpected app.Run error: %v", err)
	}

	if runResult.Side != model.SideBase {
		t.Errorf("expected side base, got %s", runResult.Side)
	}
	if len(runResult.Benchmarks) != 1 {
		t.Fatalf("expected 1 benchmark result, got %d", len(runResult.Benchmarks))
	}
	if filepath.Base(outFile) != "result.json" {
		t.Errorf("expected result.json filename, got %s", outFile)
	}
}

func TestRun_BuildError(t *testing.T) {
	t.Parallel()

	r := &fakeRunner{
		buildErr: errors.New("compilation failed"),
	}
	vcs := &fakeVCS{}

	_, _, err := app.Run(context.Background(), app.RunOptions{
		Runner: r,
		Git:    vcs,
	})
	if err == nil {
		t.Fatal("expected error when runner.Build fails, got nil")
	}
}

func TestRun_RunRoundsError(t *testing.T) {
	t.Parallel()

	r := &fakeRunner{
		binaries:     []runner.TestBinary{{Package: "example.com/testpkg", Path: "bin"}},
		runRoundsErr: errors.New("timeout reached"),
	}
	vcs := &fakeVCS{}

	_, _, err := app.Run(context.Background(), app.RunOptions{
		Runner: r,
		Git:    vcs,
	})
	if err == nil {
		t.Fatal("expected error when runner.RunRounds fails, got nil")
	}
}

func TestRun_GitFallback(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "result.json")

	r := &fakeRunner{
		binaries: []runner.TestBinary{{Package: "example.com/testpkg", Path: "bin"}},
	}
	vcs := &fakeVCS{
		infoErr: errors.New("not a git repository"),
	}

	runResult, _, err := app.Run(context.Background(), app.RunOptions{
		OutputDir: outPath,
		Runner:    r,
		Git:       vcs,
	})
	if err != nil {
		t.Fatalf("unexpected error when VCS fails: %v", err)
	}

	if runResult.Git.SHA != "unknown" {
		t.Errorf("expected fallback sha unknown, got %s", runResult.Git.SHA)
	}
	if runResult.Git.Ref != "HEAD" {
		t.Errorf("expected fallback ref HEAD, got %s", runResult.Git.Ref)
	}
	if runResult.Git.Dirty != false {
		t.Errorf("expected fallback dirty false, got %v", runResult.Git.Dirty)
	}
}

func TestRun_WriteFailure(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	// Create a file where a directory needs to be
	blocker := filepath.Join(tmpDir, "blocker")
	if err := os.WriteFile(blocker, []byte("file"), 0o600); err != nil {
		t.Fatalf("failed to create blocker file: %v", err)
	}

	invalidDest := filepath.Join(blocker, "sub", "result.json")

	r := &fakeRunner{
		binaries: []runner.TestBinary{{Package: "example.com/testpkg", Path: "bin"}},
	}
	vcs := &fakeVCS{}

	_, _, err := app.Run(context.Background(), app.RunOptions{
		OutputDir: invalidDest,
		Runner:    r,
		Git:       vcs,
	})
	if err == nil {
		t.Fatal("expected error when writeResultJSON fails, got nil")
	}
}

func TestRun_DefaultOutputDir(t *testing.T) {
	r := &fakeRunner{
		binaries: []runner.TestBinary{{Package: "example.com/testpkg", Path: "bin"}},
	}
	vcs := &fakeVCS{}

	runResult, outFile, err := app.Run(context.Background(), app.RunOptions{
		OutputDir: "",
		Runner:    r,
		Git:       vcs,
	})
	if err != nil {
		t.Fatalf("unexpected app.Run error: %v", err)
	}

	defer func() {
		_ = os.RemoveAll(filepath.Join(config.DefaultOutputDir, "runs", runResult.ID))
	}()

	if filepath.Base(outFile) != "result.json" {
		t.Errorf("expected result.json, got %s", outFile)
	}
	if _, err := os.Stat(outFile); err != nil {
		t.Errorf("expected default output file to exist: %v", err)
	}
}
