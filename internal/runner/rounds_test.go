package runner_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kyyril/perfguard/internal/model"
	"github.com/kyyril/perfguard/internal/runner"
)

type fakeExecutor struct {
	calls []fakeCall
	fail  error
}

type fakeCall struct {
	pkg  string
	spec runner.ExecSpec
}

func (f *fakeExecutor) Execute(_ context.Context, bin runner.TestBinary, spec runner.ExecSpec) (runner.RawOutput, error) {
	if f.fail != nil {
		return runner.RawOutput{}, f.fail
	}
	f.calls = append(f.calls, fakeCall{pkg: bin.Package, spec: spec})

	// Generate deterministic benchmark output
	output := "BenchmarkSample-8 1000 1500 ns/op 32 B/op 2 allocs/op\n"
	return runner.RawOutput{
		Stdout: []byte(output),
	}, nil
}

func TestInterleavedRoundOrchestration(t *testing.T) {
	ctx := context.Background()
	fake := &fakeExecutor{}
	orchestrator := runner.NewOrchestrator(fake)

	baseBin := runner.TestBinary{Package: "pkg/base", Path: "/bin/base"}
	headBin := runner.TestBinary{Package: "pkg/head", Path: "/bin/head"}

	spec := runner.RoundSpec{
		BaseBinaries: []runner.TestBinary{baseBin},
		HeadBinaries: []runner.TestBinary{headBin},
		Count:        4,
		WarmupRuns:   1,
		Interleave:   true,
		ExecSpec: runner.ExecSpec{
			BenchRegex: ".",
			Benchtime:  "1s",
			CPU:        4,
		},
	}

	res, err := orchestrator.RunRounds(ctx, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 1 warmup (2 steps: base, head) + 4 measure rounds (8 steps) = 10 total steps
	if len(res.ExecutionLog) != 10 {
		t.Fatalf("expected 10 steps in execution log, got %d", len(res.ExecutionLog))
	}

	// Verify warmup order (round 1: base then head)
	if res.ExecutionLog[0].Phase != "warmup" || res.ExecutionLog[0].Side != model.SideBase {
		t.Errorf("step 0: expected warmup base, got %+v", res.ExecutionLog[0])
	}
	if res.ExecutionLog[1].Phase != "warmup" || res.ExecutionLog[1].Side != model.SideHead {
		t.Errorf("step 1: expected warmup head, got %+v", res.ExecutionLog[1])
	}

	// Verify alternating measurement rounds
	expectedOrder := []struct {
		round int
		side  model.Side
	}{
		{round: 1, side: model.SideBase},
		{round: 1, side: model.SideHead},
		{round: 2, side: model.SideHead}, // alternated: head first
		{round: 2, side: model.SideBase},
		{round: 3, side: model.SideBase}, // base first
		{round: 3, side: model.SideHead},
		{round: 4, side: model.SideHead}, // alternated: head first
		{round: 4, side: model.SideBase},
	}

	for i, exp := range expectedOrder {
		step := res.ExecutionLog[2+i]
		if step.Phase != "measure" {
			t.Errorf("step %d: expected phase 'measure', got %s", 2+i, step.Phase)
		}
		if step.Round != exp.round {
			t.Errorf("step %d: expected round %d, got %d", 2+i, exp.round, step.Round)
		}
		if step.Side != exp.side {
			t.Errorf("step %d: expected side %s, got %s", 2+i, exp.side, step.Side)
		}
	}

	// Verify sample counts: exactly count (4) samples per side
	if len(res.BaseResults) != 1 {
		t.Fatalf("expected 1 base benchmark, got %d", len(res.BaseResults))
	}
	if len(res.BaseResults[0].Samples) != 4 {
		t.Errorf("expected 4 base samples, got %d", len(res.BaseResults[0].Samples))
	}

	if len(res.HeadResults) != 1 {
		t.Fatalf("expected 1 head benchmark, got %d", len(res.HeadResults))
	}
	if len(res.HeadResults[0].Samples) != 4 {
		t.Errorf("expected 4 head samples, got %d", len(res.HeadResults[0].Samples))
	}
}

func TestNonInterleavedExecutionOrder(t *testing.T) {
	ctx := context.Background()
	fake := &fakeExecutor{}
	orchestrator := runner.NewOrchestrator(fake)

	spec := runner.RoundSpec{
		BaseBinaries: []runner.TestBinary{{Package: "pkg/base"}},
		HeadBinaries: []runner.TestBinary{{Package: "pkg/head"}},
		Count:        2,
		WarmupRuns:   0,
		Interleave:   false,
	}

	res, err := orchestrator.RunRounds(ctx, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Round 1: Base then Head
	if res.ExecutionLog[0].Side != model.SideBase || res.ExecutionLog[1].Side != model.SideHead {
		t.Errorf("round 1 expected base then head, got %s then %s", res.ExecutionLog[0].Side, res.ExecutionLog[1].Side)
	}
	// Round 2: Still Base then Head because Interleave is false
	if res.ExecutionLog[2].Side != model.SideBase || res.ExecutionLog[3].Side != model.SideHead {
		t.Errorf("round 2 expected base then head, got %s then %s", res.ExecutionLog[2].Side, res.ExecutionLog[3].Side)
	}
}

func TestRoundFailurePropagation(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("simulated process failure")
	fake := &fakeExecutor{fail: expectedErr}
	orchestrator := runner.NewOrchestrator(fake)

	spec := runner.RoundSpec{
		BaseBinaries: []runner.TestBinary{{Package: "pkg/base"}},
		HeadBinaries: []runner.TestBinary{{Package: "pkg/head"}},
		Count:        2,
	}

	_, err := orchestrator.RunRounds(ctx, spec)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected %v, got %v", expectedErr, err)
	}
}
