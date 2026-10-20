package runner

import (
	"bytes"
	"context"
	"fmt"

	"github.com/kyyril/perfguard/internal/benchparse"
	"github.com/kyyril/perfguard/internal/model"
)

// BinaryExecutor defines the interface for running a test binary.
type BinaryExecutor interface {
	Execute(ctx context.Context, bin TestBinary, spec ExecSpec) (RawOutput, error)
}

// StepInfo records metadata for a single executed binary step.
type StepInfo struct {
	Phase   string     `json:"phase"` // "warmup" or "measure"
	Round   int        `json:"round"` // 1-based round index
	Side    model.Side `json:"side"`  // "base" or "head"
	Package string     `json:"package"`
}

// RoundSpec defines parameters for orchestrating warmup and measurement rounds.
type RoundSpec struct {
	BaseBinaries []TestBinary
	HeadBinaries []TestBinary
	Count        int            // number of measurement samples per side
	WarmupRuns   int            // number of warmup runs (discarded)
	Interleave   bool           // whether to alternate base and head order per round
	ExecSpec     ExecSpec       // flags and parameters for each run
	OnProgress   func(StepInfo) // optional progress callback
}

// RoundResults holds parsed benchmark samples and the execution order trace.
type RoundResults struct {
	BaseResults  []model.BenchmarkResult
	HeadResults  []model.BenchmarkResult
	ExecutionLog []StepInfo
}

// Orchestrator coordinates interleaved benchmark rounds across base and head binaries.
type Orchestrator struct {
	executor BinaryExecutor
}

// NewOrchestrator creates a new round orchestrator.
func NewOrchestrator(executor BinaryExecutor) *Orchestrator {
	if executor == nil {
		executor = New()
	}
	return &Orchestrator{executor: executor}
}

// RunRounds coordinates warmup and interleaved measurement rounds.
func (r *Runner) RunRounds(ctx context.Context, spec RoundSpec) (RoundResults, error) {
	return NewOrchestrator(r).RunRounds(ctx, spec)
}

// RunRounds executes warmup rounds followed by interleaved measurement rounds.
func (o *Orchestrator) RunRounds(ctx context.Context, spec RoundSpec) (RoundResults, error) {
	if spec.Count < 0 {
		return RoundResults{}, fmt.Errorf("runner: sample count must be non-negative: %d", spec.Count)
	}

	results := RoundResults{}

	// 1. Warmup rounds (results discarded)
	for w := 1; w <= spec.WarmupRuns; w++ {
		firstSide, secondSide := getSidesForRound(w, spec.Interleave)

		if err := o.executeSide(ctx, "warmup", w, firstSide, spec, nil, &results.ExecutionLog); err != nil {
			return results, fmt.Errorf("runner: warmup round %d on %s failed: %w", w, firstSide, err)
		}
		if err := o.executeSide(ctx, "warmup", w, secondSide, spec, nil, &results.ExecutionLog); err != nil {
			return results, fmt.Errorf("runner: warmup round %d on %s failed: %w", w, secondSide, err)
		}
	}

	// 2. Measurement rounds
	baseIndex := make(map[string]int)
	headIndex := make(map[string]int)

	for r := 1; r <= spec.Count; r++ {
		firstSide, secondSide := getSidesForRound(r, spec.Interleave)

		// Execute first side of the round
		if err := o.executeAndAccumulate(ctx, "measure", r, firstSide, spec, &results, baseIndex, headIndex); err != nil {
			return results, err
		}

		// Execute second side of the round
		if err := o.executeAndAccumulate(ctx, "measure", r, secondSide, spec, &results, baseIndex, headIndex); err != nil {
			return results, err
		}
	}

	return results, nil
}

func getSidesForRound(round int, interleave bool) (first, second model.Side) {
	if !interleave || round%2 == 1 {
		// Odd round (or non-interleaved): base then head
		return model.SideBase, model.SideHead
	}
	// Even round: head then base
	return model.SideHead, model.SideBase
}

func (o *Orchestrator) executeAndAccumulate(
	ctx context.Context,
	phase string,
	round int,
	side model.Side,
	spec RoundSpec,
	results *RoundResults,
	baseIndex, headIndex map[string]int,
) error {
	var targetList *[]model.BenchmarkResult
	var targetIndex map[string]int

	if side == model.SideBase {
		targetList = &results.BaseResults
		targetIndex = baseIndex
	} else {
		targetList = &results.HeadResults
		targetIndex = headIndex
	}

	acc := func(benchmarks []model.BenchmarkResult) {
		for _, b := range benchmarks {
			key := b.Package + "::" + b.Name
			if idx, ok := targetIndex[key]; ok {
				(*targetList)[idx].Samples = append((*targetList)[idx].Samples, b.Samples...)
				if (*targetList)[idx].Procs == 0 && b.Procs > 0 {
					(*targetList)[idx].Procs = b.Procs
				}
			} else {
				targetIndex[key] = len(*targetList)
				*targetList = append(*targetList, b)
			}
		}
	}

	if err := o.executeSide(ctx, phase, round, side, spec, acc, &results.ExecutionLog); err != nil {
		return fmt.Errorf("runner: round %d on %s failed: %w", round, side, err)
	}
	return nil
}

func (o *Orchestrator) executeSide(
	ctx context.Context,
	phase string,
	round int,
	side model.Side,
	spec RoundSpec,
	onResults func([]model.BenchmarkResult),
	executionLog *[]StepInfo,
) error {
	var binaries []TestBinary
	if side == model.SideBase {
		binaries = spec.BaseBinaries
	} else {
		binaries = spec.HeadBinaries
	}

	for _, bin := range binaries {
		step := StepInfo{
			Phase:   phase,
			Round:   round,
			Side:    side,
			Package: bin.Package,
		}
		*executionLog = append(*executionLog, step)
		if spec.OnProgress != nil {
			spec.OnProgress(step)
		}

		raw, err := o.executor.Execute(ctx, bin, spec.ExecSpec)
		if err != nil {
			return err
		}

		if onResults != nil {
			parsed, err := benchparse.Parse(bytes.NewReader(raw.Stdout), bin.Package)
			if err != nil {
				return fmt.Errorf("runner: parse benchmark output in %s: %w", bin.Package, err)
			}
			onResults(parsed)
		}
	}

	return nil
}
