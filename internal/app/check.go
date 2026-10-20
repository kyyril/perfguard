package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kyyril/perfguard/internal/config"
	"github.com/kyyril/perfguard/internal/decision"
	"github.com/kyyril/perfguard/internal/envinfo"
	"github.com/kyyril/perfguard/internal/gitx"
	"github.com/kyyril/perfguard/internal/model"
	"github.com/kyyril/perfguard/internal/report"
	"github.com/kyyril/perfguard/internal/runner"
)

// CheckOptions specifies options for executing a complete check run.
type CheckOptions struct {
	Config    *config.Config
	WorkDir   string
	BaseRef   string
	OutputDir string
	Runner    BenchRunner
	Git       VCS
}

// CheckResult contains all artifacts and outputs of a check run.
type CheckResult struct {
	Comparison     model.Comparison
	BaseRun        model.Run
	HeadRun        model.Run
	RunDir         string
	ComparisonFile string
	BaseFile       string
	HeadFile       string
	ReportText     string
}

// Check executes the complete end-to-end check data flow:
// 1. Resolve base ref and create temporary worktree
// 2. Build test binaries for base and head
// 3. Collect environment fingerprint and preflight checks
// 4. Run measurement rounds with adaptive sampling for noisy metrics
// 5. Evaluate decision rules and compute verdict
// 6. Generate reports and persist artifacts in .perfguard/runs/<run_id>/
// 7. Clean up worktree and temporary test binaries
func Check(ctx context.Context, opts CheckOptions) (CheckResult, error) {
	select {
	case <-ctx.Done():
		return CheckResult{}, ctx.Err()
	default:
	}

	workDir := opts.WorkDir
	if workDir == "" {
		workDir = "."
	}

	cfg := opts.Config
	if cfg == nil {
		cfg = config.Default()
	}

	r := opts.Runner
	if r == nil {
		r = runner.New()
	}

	gitClient := opts.Git
	if gitClient == nil {
		gitClient = &gitAdapter{client: gitx.New(workDir)}
	}

	// 1. Resolve head Git metadata
	headGit, err := gitClient.CurrentInfo(ctx)
	if err != nil {
		headGit = model.GitInfo{
			SHA:   "unknown",
			Ref:   "HEAD",
			Dirty: false,
		}
	}

	// 2. Resolve base ref
	baseRef := opts.BaseRef
	if baseRef == "" {
		baseRef = cfg.Baseline.Branch
	}
	if baseRef == "" {
		baseRef = "origin/main"
	}

	baseCommit, err := gitClient.Resolve(ctx, baseRef)
	if err != nil {
		return CheckResult{}, fmt.Errorf("app: resolve base ref %s: %w", baseRef, err)
	}

	// 3. Create temporary worktree for base
	wt, err := gitClient.CreateWorktree(ctx, baseCommit)
	if err != nil {
		return CheckResult{}, fmt.Errorf("app: create worktree for base %s: %w", baseCommit.SHA, err)
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = wt.Remove(cleanupCtx)
	}()

	// 4. Build base test binaries
	baseBinaries, err := r.Build(ctx, runner.BuildSpec{
		WorkDir:  wt.Location(),
		Packages: cfg.Benchmarks.Packages,
	})
	if err != nil {
		return CheckResult{}, fmt.Errorf("app: build base benchmark binaries: %w", err)
	}
	defer func() {
		for _, b := range baseBinaries {
			_ = b.Remove()
		}
	}()

	// 5. Build head test binaries
	headBinaries, err := r.Build(ctx, runner.BuildSpec{
		WorkDir:  workDir,
		Packages: cfg.Benchmarks.Packages,
	})
	if err != nil {
		return CheckResult{}, fmt.Errorf("app: build head benchmark binaries: %w", err)
	}
	defer func() {
		for _, b := range headBinaries {
			_ = b.Remove()
		}
	}()

	// 6. Environment fingerprint
	env := envinfo.Fingerprint()

	runID := fmt.Sprintf("run-%d", time.Now().UnixNano())
	runParams := model.RunParams{
		Count:          cfg.Benchmarks.Count,
		Benchtime:      cfg.Benchmarks.Benchtime,
		CPU:            cfg.Benchmarks.CPU,
		Include:        cfg.Benchmarks.Include,
		Exclude:        cfg.Benchmarks.Exclude,
		Mode:           cfg.Execution.Mode,
		WarmupRuns:     cfg.Execution.WarmupRuns,
		MaxExtraRuns:   cfg.Execution.MaxExtraRuns,
		ExtraRoundStep: cfg.Execution.ExtraRoundStep,
	}

	baseRun := &model.Run{
		SchemaVersion: model.CurrentSchemaVersion,
		ID:            fmt.Sprintf("%s-base", runID),
		Side:          model.SideBase,
		CreatedAt:     time.Now().UTC(),
		Git: model.GitInfo{
			SHA:   baseCommit.SHA,
			Ref:   baseRef,
			Dirty: false,
		},
		Env:    env,
		Params: runParams,
	}

	headRun := &model.Run{
		SchemaVersion: model.CurrentSchemaVersion,
		ID:            fmt.Sprintf("%s-head", runID),
		Side:          model.SideHead,
		CreatedAt:     time.Now().UTC(),
		Git:           headGit,
		Env:           env,
		Params:        runParams,
	}

	// 7. Run rounds with adaptive sampling
	cmp, err := RunWithAdaptiveSampling(ctx, r, baseBinaries, headBinaries, cfg, baseRun, headRun)
	if err != nil {
		return CheckResult{}, fmt.Errorf("app: execute benchmark rounds: %w", err)
	}

	// 8. Generate text report
	var repBuf bytes.Buffer
	_ = report.RenderText(&repBuf, cmp)
	reportText := repBuf.String()

	// 9. Save artifacts to disk
	outDir := opts.OutputDir
	if outDir == "" {
		outDir = filepath.Join(config.DefaultOutputDir, "runs", runID)
	}

	baseFile, err := writeRunJSON(outDir, "base.result.json", *baseRun)
	if err != nil {
		return CheckResult{}, fmt.Errorf("app: save base result: %w", err)
	}

	headFile, err := writeRunJSON(outDir, "head.result.json", *headRun)
	if err != nil {
		return CheckResult{}, fmt.Errorf("app: save head result: %w", err)
	}

	compFile, err := writeComparisonJSON(filepath.Join(outDir, "comparison.json"), cmp)
	if err != nil {
		return CheckResult{}, fmt.Errorf("app: save comparison result: %w", err)
	}

	reportFile := filepath.Join(outDir, "report.txt")
	if err := os.WriteFile(reportFile, []byte(reportText), 0o600); err != nil {
		return CheckResult{}, fmt.Errorf("app: save report text: %w", err)
	}

	return CheckResult{
		Comparison:     cmp,
		BaseRun:        *baseRun,
		HeadRun:        *headRun,
		RunDir:         outDir,
		ComparisonFile: compFile,
		BaseFile:       baseFile,
		HeadFile:       headFile,
		ReportText:     reportText,
	}, nil
}

func writeRunJSON(dir, filename string, r model.Run) (string, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("create directory: %w", err)
	}
	targetFile := filepath.Join(dir, filename)
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal run json: %w", err)
	}
	tmpFile := targetFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0o600); err != nil {
		return "", fmt.Errorf("write temporary run file: %w", err)
	}
	if err := os.Rename(tmpFile, targetFile); err != nil {
		_ = os.Remove(tmpFile)
		return "", fmt.Errorf("commit run file: %w", err)
	}
	return targetFile, nil
}

// RunWithAdaptiveSampling executes the measurement rounds and an adaptive sampling loop
// for noisy metrics. If any metric has CV exceeding noise.max_cv, extra rounds are added
// in steps of execution.extra_round_step up to execution.max_extra_runs.
func RunWithAdaptiveSampling(
	ctx context.Context,
	r BenchRunner,
	baseBinaries, headBinaries []runner.TestBinary,
	cfg *config.Config,
	baseRun, headRun *model.Run,
) (model.Comparison, error) {
	if cfg == nil {
		cfg = config.Default()
	}

	timeout, _ := time.ParseDuration(cfg.Benchmarks.Timeout)
	if timeout <= 0 {
		timeout = 30 * time.Minute
	}

	roundSpec := runner.RoundSpec{
		Count:        cfg.Benchmarks.Count,
		WarmupRuns:   cfg.Execution.WarmupRuns,
		Interleave:   cfg.Execution.Interleave,
		BaseBinaries: baseBinaries,
		HeadBinaries: headBinaries,
		ExecSpec: runner.ExecSpec{
			BenchRegex: cfg.Benchmarks.Include,
			Benchtime:  cfg.Benchmarks.Benchtime,
			CPU:        cfg.Benchmarks.CPU,
			Timeout:    timeout,
		},
	}

	// 1. Initial measurement rounds
	results, err := r.RunRounds(ctx, roundSpec)
	if err != nil {
		return model.Comparison{}, fmt.Errorf("app: execute initial rounds: %w", err)
	}

	baseRun.Benchmarks = results.BaseResults
	headRun.Benchmarks = results.HeadResults

	// 2. Initial comparison
	cmp, err := decision.CompareRuns(*baseRun, *headRun, cfg)
	if err != nil {
		return model.Comparison{}, fmt.Errorf("app: initial comparison: %w", err)
	}

	// 3. Adaptive sampling loop for noisy metrics
	extraRuns := 0
	maxExtra := cfg.Execution.MaxExtraRuns
	step := cfg.Execution.ExtraRoundStep
	if step < 1 {
		step = 1
	}

	for HasNoisyMetric(cmp) && extraRuns < maxExtra {
		curStep := step
		if extraRuns+curStep > maxExtra {
			curStep = maxExtra - extraRuns
		}
		if curStep <= 0 {
			break
		}

		extraSpec := roundSpec
		extraSpec.Count = curStep
		extraSpec.WarmupRuns = 0 // warmup is only done before the initial round pass

		extraResults, err := r.RunRounds(ctx, extraSpec)
		if err != nil {
			return cmp, fmt.Errorf("app: execute adaptive extra rounds: %w", err)
		}

		AppendBenchmarks(&baseRun.Benchmarks, extraResults.BaseResults)
		AppendBenchmarks(&headRun.Benchmarks, extraResults.HeadResults)
		extraRuns += curStep

		cmp, err = decision.CompareRuns(*baseRun, *headRun, cfg)
		if err != nil {
			return cmp, fmt.Errorf("app: recompute comparison after extra rounds: %w", err)
		}
	}

	baseRun.Params.ExtraRuns = extraRuns
	headRun.Params.ExtraRuns = extraRuns

	return cmp, nil
}

// HasNoisyMetric returns true if any metric comparison item in cmp has a noisy status or reason.
func HasNoisyMetric(cmp model.Comparison) bool {
	for _, it := range cmp.Items {
		if it.Reason == model.ReasonNoisy {
			return true
		}
	}
	return false
}

// AppendBenchmarks merges extra benchmark samples into target by benchmark name.
func AppendBenchmarks(target *[]model.BenchmarkResult, extra []model.BenchmarkResult) {
	byName := make(map[string]int, len(*target))
	for i, b := range *target {
		byName[b.Name] = i
	}

	for _, eb := range extra {
		if idx, ok := byName[eb.Name]; ok {
			(*target)[idx].Samples = append((*target)[idx].Samples, eb.Samples...)
		} else {
			*target = append(*target, eb)
			byName[eb.Name] = len(*target) - 1
		}
	}
}
