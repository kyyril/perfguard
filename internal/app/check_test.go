package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kyyril/perfguard/internal/app"
	"github.com/kyyril/perfguard/internal/config"
	"github.com/kyyril/perfguard/internal/model"
	"github.com/kyyril/perfguard/internal/runner"
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
