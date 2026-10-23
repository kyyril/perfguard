# Evaluation Scenarios

This directory contains evaluation scenarios that measure PerfGuard's detection quality and false positive rate.

## Scenario Structure

Each scenario is a directory under `scenarios/` with the following structure:

```
scenarios/<scenario-id>/
├── base/               # Complete Go module (must have go.mod)
├── change.patch        # Unified diff applied to base/
└── scenario.yaml       # Ground truth and trial configuration
```

## scenario.yaml Format

```yaml
id: scenario-id
description: Brief description of what this scenario tests
trials: 10  # Number of times to run this scenario

expect:
  verdict: fail  # pass | warn | fail | inconclusive
  
  # Optional: specific metric expectations
  metrics:
    - benchmark: BenchmarkName
      metric: sec/op
      status: fail  # unchanged | improved | warn | fail | inconclusive | new | removed
  
  # Optional: diagnosis expectations
  diagnosis:
    profile: cpu  # cpu | allocs | mutex | block
    top_k: 5
    function: example.com/pkg.FunctionName
```

## Running Scenarios

Run a single scenario:

```bash
perfguard eval run --scenario eval/scenarios/example-scenario --output eval/results
```

The command creates a temporary git repository for each trial, applies the patch, runs `perfguard check`, and records results to `<output>/<scenario-id>.json`.

## Result Format

Results are written as JSON:

```json
{
  "scenario_id": "example-scenario",
  "trials": [
    {
      "trial": 1,
      "verdict": "fail",
      "duration_ms": 1234,
      "comparison": { ... },
      "error": ""
    }
  ],
  "environment": { ... },
  "started_at": "2026-10-23T09:00:00Z",
  "finished_at": "2026-10-23T09:01:00Z"
}
```

## Creating Scenarios

1. Create the scenario directory structure
2. Write a complete Go module in `base/` with benchmarks
3. Create `change.patch` with the code change to test
4. Define ground truth in `scenario.yaml`
5. Manually verify the expected behavior before setting ground truth

## Scenario Categories

Scenarios are organized by category:

- **Negative**: no regression expected (`no-change-aa`, `neutral-refactor`, `below-threshold`, `improvement`)
- **Positive**: regression expected (`algo-slowdown`, `alloc-bloat`, `lock-contention`)
- **Edge cases**: boundary conditions (`noisy-host`, `zero-base`)

See `docs/TESTING.md` section 6 for details on the evaluation suite strategy.
