package app

import (
	"context"
	"fmt"
	"time"

	"github.com/kyyril/perfguard/internal/config"
	"github.com/kyyril/perfguard/internal/decision"
	"github.com/kyyril/perfguard/internal/model"
	"github.com/kyyril/perfguard/internal/runner"
)

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

// HasNoisyMetric returns true if any metric comparison produced ReasonNoisy.
func HasNoisyMetric(cmp model.Comparison) bool {
	for _, item := range cmp.Items {
		if item.Reason == model.ReasonNoisy {
			return true
		}
	}
	return false
}

// AppendBenchmarks merges incoming benchmark results and samples into target.
func AppendBenchmarks(target *[]model.BenchmarkResult, incoming []model.BenchmarkResult) {
	index := make(map[string]int, len(*target))
	for i, b := range *target {
		key := b.Package + "::" + b.Name
		index[key] = i
	}

	for _, b := range incoming {
		key := b.Package + "::" + b.Name
		if idx, ok := index[key]; ok {
			(*target)[idx].Samples = append((*target)[idx].Samples, b.Samples...)
			if (*target)[idx].Procs == 0 && b.Procs > 0 {
				(*target)[idx].Procs = b.Procs
			}
		} else {
			index[key] = len(*target)
			*target = append(*target, b)
		}
	}
}
