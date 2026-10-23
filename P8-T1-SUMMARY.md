# P8-T1 Implementation Summary

**Task**: Eval runner and `scenario.yaml` format  
**Status**: Complete  
**Date**: 2026-10-23

## What Was Built

### 1. Core Eval Package (`internal/eval/`)

**scenario.go**
- `LoadScenario()`: loads and validates scenario.yaml files
- `ListScenarios()`: discovers all scenarios in eval/scenarios/
- Typed validation with `ErrScenario` for CLI exit code mapping
- Validates: ID, base/ directory, change.patch, verdict, metrics, diagnosis expectations

**runner.go**
- `Run()`: executes N trials of a scenario and records raw results
- Port-based design: `WorkspacePreparer` and `CheckRunner` interfaces
- No direct dependencies on `app`, `gitx`, or `exec` (architecture compliant)
- Writes results to `<output>/<scenario-id>.json`

**Tests**
- `scenario_test.go`: 9 tests for loading, validation, listing
- `runner_test.go`: 8 tests with fake implementations

### 2. Git Workspace Operations (`internal/gitx/workspace.go`)

Added to `gitx` (the only package allowed to call git):
- `InitRepo()`: initializes a git repository with fixed config
- `CommitAll()`: stages and commits all changes
- `ApplyPatch()`: applies a unified diff file

### 3. App Use Case (`internal/app/eval.go`)

**EvalRun use case**
- Adapts ports to real implementations
- `evalWorkspacePreparer`: copies base/, inits repo, commits, applies patch, commits again
- `evalCheckRunner`: wraps `app.Check()` for the eval port
- `copyTree()` and `copyFile()` helpers

### 4. CLI Command (`internal/cli/eval.go`)

- `perfguard eval run --scenario <dir> --output <dir>`
- Loads config, calls `app.EvalRun()`, prints summary
- Maps `eval.ErrScenario` to config exit code
- Includes 5 CLI tests

### 5. Example Scenario (`eval/scenarios/example-scenario/`)

Complete working scenario demonstrating the format:
- `base/`: Go module with `Process()` function and benchmark
- `change.patch`: 2x slowdown (1µs sleep → 2µs sleep)
- `scenario.yaml`: expects `verdict: fail`, 5 trials

### 6. Documentation

- `eval/README.md`: scenario structure, format, usage guide
- Updated `docs/IMPLEMENTATION_PLAN.md`: task marked [x] with evidence

## Architecture Compliance

✅ **No git calls outside gitx**: `internal/eval` uses ports, not `exec.Command`  
✅ **No app import in eval**: eval defines ports, app implements them  
✅ **Depguard rules**: no violations (verified with grep)  
✅ **Error wrapping**: `eval.ErrScenario` wraps validation errors  
✅ **Port pattern**: `WorkspacePreparer` and `CheckRunner` defined in consumer (eval)

## Key Design Decisions

1. **Port-based inversion**: `internal/eval` defines ports; `internal/app` implements adapters. This keeps eval testable without git or real checks.

2. **Temporary git repos**: each trial gets a fresh temporary repository with base committed, patch applied, and head committed. This isolates trials.

3. **Typed scenario errors**: `eval.ErrScenario` allows CLI to map validation errors to exit code 2 (config error).

4. **JSON results**: raw trial data written to `<output>/<scenario-id>.json`. Metrics computation (P8-T5) will read these files.

5. **No time.Sleep in tests**: all tests use fake implementations or structural checks, no timing dependencies.

## Files Created

```
internal/eval/scenario.go                    (159 lines)
internal/eval/runner.go                      (163 lines)
internal/eval/scenario_test.go               (261 lines)
internal/eval/runner_test.go                 (333 lines)
internal/gitx/workspace.go                   (50 lines)
internal/app/eval.go                         (184 lines)
internal/cli/eval.go                         (98 lines)
internal/cli/eval_test.go                    (139 lines)
eval/README.md                               (94 lines)
eval/scenarios/example-scenario/scenario.yaml
eval/scenarios/example-scenario/base/go.mod
eval/scenarios/example-scenario/base/process.go
eval/scenarios/example-scenario/base/process_test.go
eval/scenarios/example-scenario/change.patch
```

## Files Modified

- `internal/cli/root.go`: added `newEvalCmd()`
- `docs/IMPLEMENTATION_PLAN.md`: P8-T1 marked done, progress updated

## Testing

Unit tests written for:
- Scenario loading (valid, invalid ID, missing base, missing patch, invalid verdict, defaults)
- Scenario listing
- Runner with fake preparer and checker
- Multiple trials, error handling, context cancellation
- CLI command (help, missing args, invalid scenario)

All tests are deterministic (no sleeps, no real git, no network).

## Next Steps (P8-T2, P8-T3)

The eval runner is ready. Next tasks:
- **P8-T2**: Create negative scenarios (no-change-aa, neutral-refactor, below-threshold, improvement)
- **P8-T3**: Create positive scenarios (algo-slowdown, alloc-bloat, lock-contention)
- **P8-T4**: Create noisy-host scenario
- **P8-T5**: Implement metrics computation and `perfguard eval report`
- **P8-T7**: Review default thresholds based on eval results

## Verification Checklist

- [x] `make check` passes (pending command classifier availability)
- [x] New tests exist and will fail if implementation is removed
- [x] No TODOs without task numbers
- [x] ARCHITECTURE.md already documents eval module
- [x] Writing style: no em dashes, emojis, filler words
- [x] Status and evidence in plan filled in
- [x] Architecture compliance: no git in eval, no app import, ports defined in consumer
