package app_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kyyril/perfguard/internal/app"
	"github.com/kyyril/perfguard/internal/config"
	"github.com/kyyril/perfguard/internal/ghci"
	"github.com/kyyril/perfguard/internal/gitx"
	"github.com/kyyril/perfguard/internal/model"
	"github.com/kyyril/perfguard/internal/runner"
	"gopkg.in/yaml.v3"
)

type adaptiveFakeRunner struct {
	roundCalls []runner.RoundSpec
	responses  []runner.RoundResults
	errAtCall  int
}

func (f *adaptiveFakeRunner) Build(_ context.Context, _ runner.BuildSpec) ([]runner.TestBinary, error) {
	return nil, nil
}

func (f *adaptiveFakeRunner) Execute(_ context.Context, _ runner.TestBinary, _ runner.ExecSpec) (runner.RawOutput, error) {
	return runner.RawOutput{}, nil
}

func (f *adaptiveFakeRunner) RunRounds(_ context.Context, spec runner.RoundSpec) (runner.RoundResults, error) {
	callIdx := len(f.roundCalls)
	f.roundCalls = append(f.roundCalls, spec)

	if f.errAtCall == callIdx+1 {
		return runner.RoundResults{}, errors.New("simulated round error")
	}

	if callIdx < len(f.responses) {
		return f.responses[callIdx], nil
	}

	return runner.RoundResults{}, nil
}

func createBenchmarkResult(name string, values []float64) model.BenchmarkResult {
	samples := make([]model.Sample, len(values))
	for i, v := range values {
		samples[i] = model.Sample{
			Iterations: 1000,
			Values:     map[string]float64{"sec/op": v},
		}
	}
	return model.BenchmarkResult{
		Package: "example.com/pkg",
		Name:    name,
		Procs:   4,
		Samples: samples,
	}
}

func TestAdaptiveSampling_ReachesLimitAndFollowsOnNoisy(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.Benchmarks.Count = 6
	cfg.Thresholds.MinSamples = 6
	cfg.Noise.MaxCV = 10.0
	cfg.Noise.OnNoisy = "inconclusive"
	cfg.Execution.MaxExtraRuns = 4
	cfg.Execution.ExtraRoundStep = 2

	// Highly noisy samples (CV > 50%)
	noisy6 := []float64{10.0, 50.0, 10.0, 50.0, 10.0, 50.0}
	noisy2 := []float64{10.0, 50.0}

	r := &adaptiveFakeRunner{
		responses: []runner.RoundResults{
			// Call 1: initial 6 samples
			{
				BaseResults: []model.BenchmarkResult{createBenchmarkResult("BenchmarkWork", noisy6)},
				HeadResults: []model.BenchmarkResult{createBenchmarkResult("BenchmarkWork", noisy6)},
			},
			// Call 2: extra round step 1 (2 samples)
			{
				BaseResults: []model.BenchmarkResult{createBenchmarkResult("BenchmarkWork", noisy2)},
				HeadResults: []model.BenchmarkResult{createBenchmarkResult("BenchmarkWork", noisy2)},
			},
			// Call 3: extra round step 2 (2 samples) -> total extra = 4 (limit reached)
			{
				BaseResults: []model.BenchmarkResult{createBenchmarkResult("BenchmarkWork", noisy2)},
				HeadResults: []model.BenchmarkResult{createBenchmarkResult("BenchmarkWork", noisy2)},
			},
		},
	}

	baseRun := &model.Run{ID: "base"}
	headRun := &model.Run{ID: "head"}

	cmp, err := app.RunWithAdaptiveSampling(context.Background(), r, nil, nil, cfg, baseRun, headRun)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify exactly 3 calls to RunRounds: 1 initial + 2 extra steps
	if len(r.roundCalls) != 3 {
		t.Fatalf("expected 3 calls to RunRounds, got %d", len(r.roundCalls))
	}
	if r.roundCalls[0].Count != 6 || r.roundCalls[0].WarmupRuns != 1 {
		t.Errorf("call 0: expected Count=6, Warmup=1; got %+v", r.roundCalls[0])
	}
	if r.roundCalls[1].Count != 2 || r.roundCalls[1].WarmupRuns != 0 {
		t.Errorf("call 1: expected Count=2, Warmup=0; got %+v", r.roundCalls[1])
	}
	if r.roundCalls[2].Count != 2 || r.roundCalls[2].WarmupRuns != 0 {
		t.Errorf("call 2: expected Count=2, Warmup=0; got %+v", r.roundCalls[2])
	}

	// Total samples should be 6 + 2 + 2 = 10
	if len(baseRun.Benchmarks[0].Samples) != 10 {
		t.Errorf("expected 10 samples in base, got %d", len(baseRun.Benchmarks[0].Samples))
	}
	if baseRun.Params.ExtraRuns != 4 {
		t.Errorf("expected ExtraRuns=4, got %d", baseRun.Params.ExtraRuns)
	}

	// Status must follow cfg.Noise.OnNoisy ("inconclusive")
	if len(cmp.Items) != 1 {
		t.Fatalf("expected 1 metric comparison item, got %d", len(cmp.Items))
	}
	item := cmp.Items[0]
	if item.Reason != model.ReasonNoisy {
		t.Errorf("expected ReasonNoisy, got %s", item.Reason)
	}
	if item.Status != model.StatusInconclusive {
		t.Errorf("expected StatusInconclusive, got %s", item.Status)
	}
}

func TestAdaptiveSampling_OnNoisyWarn(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.Benchmarks.Count = 6
	cfg.Thresholds.MinSamples = 6
	cfg.Noise.MaxCV = 10.0
	cfg.Noise.OnNoisy = "warn"
	cfg.Execution.MaxExtraRuns = 2
	cfg.Execution.ExtraRoundStep = 2

	noisy6 := []float64{10.0, 50.0, 10.0, 50.0, 10.0, 50.0}
	noisy2 := []float64{10.0, 50.0}

	r := &adaptiveFakeRunner{
		responses: []runner.RoundResults{
			{
				BaseResults: []model.BenchmarkResult{createBenchmarkResult("BenchmarkWork", noisy6)},
				HeadResults: []model.BenchmarkResult{createBenchmarkResult("BenchmarkWork", noisy6)},
			},
			{
				BaseResults: []model.BenchmarkResult{createBenchmarkResult("BenchmarkWork", noisy2)},
				HeadResults: []model.BenchmarkResult{createBenchmarkResult("BenchmarkWork", noisy2)},
			},
		},
	}

	baseRun := &model.Run{ID: "base"}
	headRun := &model.Run{ID: "head"}

	cmp, err := app.RunWithAdaptiveSampling(context.Background(), r, nil, nil, cfg, baseRun, headRun)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cmp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(cmp.Items))
	}
	if cmp.Items[0].Status != model.StatusWarn {
		t.Errorf("expected StatusWarn when on_noisy is 'warn', got %s", cmp.Items[0].Status)
	}
}

func TestAdaptiveSampling_StabilizesEarly(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.Benchmarks.Count = 6
	cfg.Thresholds.MinSamples = 6
	cfg.Noise.MaxCV = 10.0
	cfg.Execution.MaxExtraRuns = 8
	cfg.Execution.ExtraRoundStep = 2

	// Initial samples slightly noisy (CV around 10.95% > 10.0%)
	initial6 := []float64{9.9, 12.1, 9.9, 12.1, 9.9, 12.1}
	// Extra samples at the mean (11.0) lower overall CV to 9.26% < 10.0%
	stable2 := []float64{11.0, 11.0}

	r := &adaptiveFakeRunner{
		responses: []runner.RoundResults{
			{
				BaseResults: []model.BenchmarkResult{createBenchmarkResult("BenchmarkWork", initial6)},
				HeadResults: []model.BenchmarkResult{createBenchmarkResult("BenchmarkWork", initial6)},
			},
			{
				BaseResults: []model.BenchmarkResult{createBenchmarkResult("BenchmarkWork", stable2)},
				HeadResults: []model.BenchmarkResult{createBenchmarkResult("BenchmarkWork", stable2)},
			},
			// Call 3 should NOT be made because CV stabilizes
			{
				BaseResults: []model.BenchmarkResult{createBenchmarkResult("BenchmarkWork", stable2)},
				HeadResults: []model.BenchmarkResult{createBenchmarkResult("BenchmarkWork", stable2)},
			},
		},
	}

	baseRun := &model.Run{ID: "base"}
	headRun := &model.Run{ID: "head"}

	cmp, err := app.RunWithAdaptiveSampling(context.Background(), r, nil, nil, cfg, baseRun, headRun)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should stop after 1 extra step (total 2 calls)
	if len(r.roundCalls) != 2 {
		t.Fatalf("expected early termination after 2 calls, got %d", len(r.roundCalls))
	}
	if baseRun.Params.ExtraRuns != 2 {
		t.Errorf("expected ExtraRuns=2, got %d", baseRun.Params.ExtraRuns)
	}

	// Should not be noisy
	if cmp.Items[0].Reason == model.ReasonNoisy {
		t.Errorf("expected metric to stabilize and not be noisy, got reason %s", cmp.Items[0].Reason)
	}
}

func TestAdaptiveSampling_Errors(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.Benchmarks.Count = 6
	cfg.Noise.MaxCV = 10.0
	cfg.Execution.MaxExtraRuns = 4
	cfg.Execution.ExtraRoundStep = 2

	t.Run("initial round error", func(t *testing.T) {
		r := &adaptiveFakeRunner{errAtCall: 1}
		baseRun := &model.Run{ID: "base"}
		headRun := &model.Run{ID: "head"}

		_, err := app.RunWithAdaptiveSampling(context.Background(), r, nil, nil, cfg, baseRun, headRun)
		if err == nil {
			t.Fatal("expected error on initial round failure, got nil")
		}
	})

	t.Run("extra round error", func(t *testing.T) {
		noisy6 := []float64{10.0, 50.0, 10.0, 50.0, 10.0, 50.0}
		r := &adaptiveFakeRunner{
			errAtCall: 2, // Fails on first extra round
			responses: []runner.RoundResults{
				{
					BaseResults: []model.BenchmarkResult{createBenchmarkResult("BenchmarkWork", noisy6)},
					HeadResults: []model.BenchmarkResult{createBenchmarkResult("BenchmarkWork", noisy6)},
				},
			},
		}
		baseRun := &model.Run{ID: "base"}
		headRun := &model.Run{ID: "head"}

		_, err := app.RunWithAdaptiveSampling(context.Background(), r, nil, nil, cfg, baseRun, headRun)
		if err == nil {
			t.Fatal("expected error on extra round failure, got nil")
		}
	})
}

type fakeCheckWorktree struct {
	path    string
	removed bool
}

func (w *fakeCheckWorktree) Location() string {
	return w.path
}

func (w *fakeCheckWorktree) Remove(_ context.Context) error {
	w.removed = true
	return nil
}

type fakeCheckVCS struct {
	gitInfo       model.GitInfo
	commit        gitx.Commit
	worktree      *fakeCheckWorktree
	errInfo       error
	errResolve    error
	errCreateWork error
}

func (v *fakeCheckVCS) CurrentInfo(_ context.Context) (model.GitInfo, error) {
	if v.errInfo != nil {
		return model.GitInfo{}, v.errInfo
	}
	return v.gitInfo, nil
}

func (v *fakeCheckVCS) Resolve(_ context.Context, _ string) (gitx.Commit, error) {
	if v.errResolve != nil {
		return gitx.Commit{}, v.errResolve
	}
	return v.commit, nil
}

func (v *fakeCheckVCS) CreateWorktree(_ context.Context, _ gitx.Commit) (app.Worktree, error) {
	if v.errCreateWork != nil {
		return nil, v.errCreateWork
	}
	return v.worktree, nil
}

func createDummyBinary(t *testing.T, dir, name string) runner.TestBinary {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("fake"), 0o600); err != nil {
		t.Fatalf("failed creating dummy binary: %v", err)
	}
	return runner.TestBinary{
		Package: "example.com/pkg",
		Path:    p,
		Dir:     dir,
	}
}

type fakeCheckRunner struct {
	baseBins      []runner.TestBinary
	headBins      []runner.TestBinary
	errBuildBase  error
	errBuildHead  error
	errRunRounds  error
	roundResults  runner.RoundResults
	buildCallDirs []string
	onRunRounds   func()
}

func (r *fakeCheckRunner) Build(ctx context.Context, spec runner.BuildSpec) ([]runner.TestBinary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.buildCallDirs = append(r.buildCallDirs, spec.WorkDir)
	if len(r.buildCallDirs) == 1 {
		// First call is base
		if r.errBuildBase != nil {
			return nil, r.errBuildBase
		}
		return r.baseBins, nil
	}
	// Second call is head
	if r.errBuildHead != nil {
		return nil, r.errBuildHead
	}
	return r.headBins, nil
}

func (r *fakeCheckRunner) Execute(_ context.Context, _ runner.TestBinary, _ runner.ExecSpec) (runner.RawOutput, error) {
	return runner.RawOutput{}, nil
}

func (r *fakeCheckRunner) RunRounds(ctx context.Context, _ runner.RoundSpec) (runner.RoundResults, error) {
	if r.onRunRounds != nil {
		r.onRunRounds()
	}
	if err := ctx.Err(); err != nil {
		return runner.RoundResults{}, err
	}
	if r.errRunRounds != nil {
		return runner.RoundResults{}, r.errRunRounds
	}
	return r.roundResults, nil
}

func TestCheck_FullFlowSuccess(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	outDir := filepath.Join(tmpDir, "runs", "test-run")

	wt := &fakeCheckWorktree{path: filepath.Join(tmpDir, "wt")}
	vcs := &fakeCheckVCS{
		gitInfo:  model.GitInfo{SHA: "headsha", Ref: "refs/heads/feat"},
		commit:   gitx.Commit{SHA: "basesha", Ref: "refs/heads/main"},
		worktree: wt,
	}

	binDir := t.TempDir()
	baseBin := createDummyBinary(t, binDir, "base.test")
	headBin := createDummyBinary(t, binDir, "head.test")

	samples6 := []float64{10.0, 10.0, 10.0, 10.0, 10.0, 10.0}
	regressed6 := []float64{15.0, 15.0, 15.0, 15.0, 15.0, 15.0}

	r := &fakeCheckRunner{
		baseBins: []runner.TestBinary{baseBin},
		headBins: []runner.TestBinary{headBin},
		roundResults: runner.RoundResults{
			BaseResults: []model.BenchmarkResult{createBenchmarkResult("BenchmarkWork", samples6)},
			HeadResults: []model.BenchmarkResult{createBenchmarkResult("BenchmarkWork", regressed6)},
		},
	}

	cfg := config.Default()
	cfg.Benchmarks.Packages = []string{"./pkg"}
	cfg.Benchmarks.Count = 6
	cfg.Thresholds.MinSamples = 6

	res, err := app.Check(context.Background(), app.CheckOptions{
		Config:    cfg,
		WorkDir:   tmpDir,
		OutputDir: outDir,
		Runner:    r,
		Git:       vcs,
	})
	if err != nil {
		t.Fatalf("app.Check failed: %v", err)
	}

	// 1. Check cleanups
	if !wt.removed {
		t.Error("expected worktree to be removed on completion")
	}
	if _, err := os.Stat(baseBin.Path); !os.IsNotExist(err) {
		t.Errorf("expected base binary %s to be removed", baseBin.Path)
	}
	if _, err := os.Stat(headBin.Path); !os.IsNotExist(err) {
		t.Errorf("expected head binary %s to be removed", headBin.Path)
	}

	// 2. Check verdict and comparison
	if res.Comparison.Verdict != model.VerdictFail {
		t.Errorf("expected fail verdict for +50%% regression, got %s", res.Comparison.Verdict)
	}

	// 3. Check artifacts written to disk
	if _, err := os.Stat(res.ComparisonFile); err != nil {
		t.Errorf("comparison file does not exist: %v", err)
	}
	if _, err := os.Stat(res.BaseFile); err != nil {
		t.Errorf("base file does not exist: %v", err)
	}
	if _, err := os.Stat(res.HeadFile); err != nil {
		t.Errorf("head file does not exist: %v", err)
	}
	reportPath := filepath.Join(outDir, "report.txt")
	if _, err := os.Stat(reportPath); err != nil {
		t.Errorf("report file does not exist: %v", err)
	}

	if !strings.Contains(res.ReportText, "OVERALL VERDICT: FAIL") {
		t.Errorf("expected report text to mention OVERALL VERDICT: FAIL, got:\n%s", res.ReportText)
	}
}

func TestCheck_FailuresAndCleanup(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	t.Run("resolve failure", func(t *testing.T) {
		vcs := &fakeCheckVCS{
			errResolve: errors.New("cannot resolve base"),
		}
		_, err := app.Check(context.Background(), app.CheckOptions{
			Git: vcs,
		})
		if err == nil {
			t.Fatal("expected error on resolve failure, got nil")
		}
	})

	t.Run("create worktree failure", func(t *testing.T) {
		vcs := &fakeCheckVCS{
			commit:        gitx.Commit{SHA: "base"},
			errCreateWork: errors.New("cannot create worktree"),
		}
		_, err := app.Check(context.Background(), app.CheckOptions{
			Git: vcs,
		})
		if err == nil {
			t.Fatal("expected error on create worktree failure, got nil")
		}
	})

	t.Run("build base failure cleans worktree", func(t *testing.T) {
		wt := &fakeCheckWorktree{path: filepath.Join(tmpDir, "wt-buildbase")}
		vcs := &fakeCheckVCS{
			commit:   gitx.Commit{SHA: "base"},
			worktree: wt,
		}
		r := &fakeCheckRunner{
			errBuildBase: errors.New("build base failed"),
		}

		_, err := app.Check(context.Background(), app.CheckOptions{
			Git:    vcs,
			Runner: r,
		})
		if err == nil {
			t.Fatal("expected error on build base failure, got nil")
		}
		if !wt.removed {
			t.Error("expected worktree to be cleaned up when build base fails")
		}
	})

	t.Run("build head failure cleans worktree and base binary", func(t *testing.T) {
		wt := &fakeCheckWorktree{path: filepath.Join(tmpDir, "wt-buildhead")}
		vcs := &fakeCheckVCS{
			commit:   gitx.Commit{SHA: "base"},
			worktree: wt,
		}
		binDir := t.TempDir()
		baseBin := createDummyBinary(t, binDir, "base.test")
		r := &fakeCheckRunner{
			baseBins:     []runner.TestBinary{baseBin},
			errBuildHead: errors.New("build head failed"),
		}

		_, err := app.Check(context.Background(), app.CheckOptions{
			Git:    vcs,
			Runner: r,
		})
		if err == nil {
			t.Fatal("expected error on build head failure, got nil")
		}
		if !wt.removed {
			t.Error("expected worktree to be cleaned up when build head fails")
		}
		if _, err := os.Stat(baseBin.Path); !os.IsNotExist(err) {
			t.Errorf("expected base binary %s to be cleaned up", baseBin.Path)
		}
	})

	t.Run("run rounds failure cleans worktree and both binaries", func(t *testing.T) {
		wt := &fakeCheckWorktree{path: filepath.Join(tmpDir, "wt-runrounds")}
		vcs := &fakeCheckVCS{
			commit:   gitx.Commit{SHA: "base"},
			worktree: wt,
		}
		binDir := t.TempDir()
		baseBin := createDummyBinary(t, binDir, "base.test")
		headBin := createDummyBinary(t, binDir, "head.test")
		r := &fakeCheckRunner{
			baseBins:     []runner.TestBinary{baseBin},
			headBins:     []runner.TestBinary{headBin},
			errRunRounds: errors.New("run rounds failed"),
		}

		_, err := app.Check(context.Background(), app.CheckOptions{
			Git:    vcs,
			Runner: r,
		})
		if err == nil {
			t.Fatal("expected error on run rounds failure, got nil")
		}
		if !wt.removed {
			t.Error("expected worktree to be cleaned up when run rounds fails")
		}
		if _, err := os.Stat(baseBin.Path); !os.IsNotExist(err) {
			t.Errorf("expected base binary %s to be cleaned up", baseBin.Path)
		}
		if _, err := os.Stat(headBin.Path); !os.IsNotExist(err) {
			t.Errorf("expected head binary %s to be cleaned up", headBin.Path)
		}
	})

	t.Run("context cancelled cleans worktree and binaries", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		wt := &fakeCheckWorktree{path: filepath.Join(tmpDir, "wt-cancel")}
		vcs := &fakeCheckVCS{
			commit:   gitx.Commit{SHA: "base"},
			worktree: wt,
		}
		binDir := t.TempDir()
		baseBin := createDummyBinary(t, binDir, "base.test")
		headBin := createDummyBinary(t, binDir, "head.test")
		r := &fakeCheckRunner{
			baseBins: []runner.TestBinary{baseBin},
			headBins: []runner.TestBinary{headBin},
			onRunRounds: func() {
				cancel()
			},
		}

		_, err := app.Check(ctx, app.CheckOptions{
			Git:    vcs,
			Runner: r,
		})
		if err == nil {
			t.Fatal("expected error on cancelled context, got nil")
		}
		if !wt.removed {
			t.Error("expected worktree to be cleaned up when context is cancelled")
		}
		if _, err := os.Stat(baseBin.Path); !os.IsNotExist(err) {
			t.Errorf("expected base binary %s to be cleaned up", baseBin.Path)
		}
		if _, err := os.Stat(headBin.Path); !os.IsNotExist(err) {
			t.Errorf("expected head binary %s to be cleaned up", headBin.Path)
		}
	})
}

type fakeCheckPublisher struct {
	calls []ghci.CommentParams
}

func (f *fakeCheckPublisher) PublishComment(_ context.Context, params ghci.CommentParams) (*ghci.CommentResult, error) {
	f.calls = append(f.calls, params)
	return &ghci.CommentResult{ID: 1, Action: "created"}, nil
}

func TestCheck_ReportMDAndPublishing(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := filepath.Join(tmpDir, "runs")
	eventFile := filepath.Join(tmpDir, "event.json")
	summaryFile := filepath.Join(tmpDir, "summary.md")

	eventPayload := `{
		"pull_request": {
			"number": 12,
			"head": {
				"repo": {
					"fork": false,
					"full_name": "kyyril/perfguard"
				}
			},
			"base": {
				"repo": {
					"fork": false,
					"full_name": "kyyril/perfguard",
					"owner": {"login": "kyyril"},
					"name": "perfguard"
				}
			}
		}
	}`
	if err := os.WriteFile(eventFile, []byte(eventPayload), 0o600); err != nil {
		t.Fatalf("failed writing event file: %v", err)
	}

	t.Setenv("GITHUB_EVENT_PATH", eventFile)
	t.Setenv(ghci.EnvStepSummary, summaryFile)

	wt := &fakeCheckWorktree{path: filepath.Join(tmpDir, "wt")}
	vcs := &fakeCheckVCS{
		commit:   gitx.Commit{SHA: "1111111222222233333334444444555555566666"},
		worktree: wt,
	}

	binDir := t.TempDir()
	baseBin := createDummyBinary(t, binDir, "base.test")
	headBin := createDummyBinary(t, binDir, "head.test")
	r := &fakeCheckRunner{
		baseBins: []runner.TestBinary{baseBin},
		headBins: []runner.TestBinary{headBin},
	}

	pub := &fakeCheckPublisher{}
	cfg := config.Default()
	cfg.Benchmarks.Count = 6
	cfg.Thresholds.MinSamples = 6
	cfg.GitHub.Comment = true
	cfg.GitHub.StepSummary = true

	res, err := app.Check(context.Background(), app.CheckOptions{
		Config:    cfg,
		WorkDir:   tmpDir,
		OutputDir: outDir,
		Git:       vcs,
		Runner:    r,
		Publisher: pub,
	})
	if err != nil {
		t.Fatalf("unexpected Check error: %v", err)
	}

	// 1. Verify report.md exists in RunDir and contains markdown header
	mdPath := filepath.Join(res.RunDir, "report.md")
	mdContent, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("failed reading report.md: %v", err)
	}
	if !strings.Contains(string(mdContent), "## PerfGuard:") {
		t.Errorf("report.md does not contain expected header: %s", string(mdContent))
	}

	// 2. Verify Publisher was called
	if len(pub.calls) != 1 {
		t.Errorf("expected 1 publisher call, got %d", len(pub.calls))
	} else if pub.calls[0].PRNumber != 12 {
		t.Errorf("expected PRNumber 12, got %d", pub.calls[0].PRNumber)
	}

	// 3. Verify step summary was written
	summaryContent, err := os.ReadFile(summaryFile)
	if err != nil {
		t.Fatalf("failed reading summary file: %v", err)
	}
	if !strings.Contains(string(summaryContent), "## PerfGuard:") {
		t.Errorf("summary file does not contain expected header: %s", string(summaryContent))
	}
}

func TestCompositeActionYML(t *testing.T) {
	t.Parallel()

	// Locate action.yml at repository root
	actionPath := filepath.Join("..", "..", "action.yml")
	data, err := os.ReadFile(actionPath)
	if err != nil {
		t.Fatalf("failed reading action.yml: %v", err)
	}

	var action struct {
		Name        string                 `yaml:"name"`
		Description string                 `yaml:"description"`
		Inputs      map[string]interface{} `yaml:"inputs"`
		Runs        struct {
			Using string        `yaml:"using"`
			Steps []interface{} `yaml:"steps"`
		} `yaml:"runs"`
	}

	if err := yaml.Unmarshal(data, &action); err != nil {
		t.Fatalf("failed parsing action.yml: %v", err)
	}

	if action.Name == "" {
		t.Errorf("action.yml name should not be empty")
	}
	if action.Runs.Using != "composite" {
		t.Errorf("expected runs.using to be 'composite', got %q", action.Runs.Using)
	}

	expectedInputs := []string{"version", "base", "config", "fail-on", "strict", "mode", "token", "upload-artifacts"}
	for _, in := range expectedInputs {
		if _, ok := action.Inputs[in]; !ok {
			t.Errorf("expected input %q in action.yml", in)
		}
	}

	if len(action.Runs.Steps) == 0 {
		t.Errorf("expected at least one step in composite action")
	}
}

func TestDogfoodingWorkflowYML(t *testing.T) {
	t.Parallel()

	workflowPath := filepath.Join("..", "..", ".github", "workflows", "perfguard-self.yml")
	data, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("failed reading perfguard-self.yml: %v", err)
	}

	var wf struct {
		Name string `yaml:"name"`
		On   struct {
			PullRequest struct {
				Branches []string `yaml:"branches"`
			} `yaml:"pull_request"`
		} `yaml:"on"`
		Permissions map[string]string      `yaml:"permissions"`
		Jobs        map[string]interface{} `yaml:"jobs"`
	}

	if err := yaml.Unmarshal(data, &wf); err != nil {
		t.Fatalf("failed parsing perfguard-self.yml: %v", err)
	}

	if wf.Name == "" {
		t.Errorf("expected workflow name to not be empty")
	}
	if wf.Permissions["pull-requests"] != "write" {
		t.Errorf("expected pull-requests: write permission, got %v", wf.Permissions["pull-requests"])
	}
	if len(wf.Jobs) == 0 {
		t.Errorf("expected at least one job in workflow")
	}
}
