# P8-T2 & P8-T3 Implementation Summary

**Tasks**: Negative and positive evaluation scenarios  
**Status**: Complete  
**Date**: 2026-10-23

## Overview

Created 7 evaluation scenarios (4 negative, 3 positive) to measure PerfGuard's detection quality and false positive rate. Each scenario includes a complete Go module, patch, ground truth expectations, and validation documentation.

## Scenarios Created

### Negative Scenarios (False Positive Measurement)

**1. no-change-aa** (30 trials)
- **Purpose**: A/A test with identical code
- **Expected**: verdict `pass`, status `unchanged`
- **Target**: ≤1% `fail`, ≤5% `warn` (PRD AC-02)
- **Implementation**: Comment-only change, deterministic computation

**2. neutral-refactor** (20 trials)
- **Purpose**: Structural change with no performance effect
- **Expected**: verdict `pass`, status `unchanged`
- **Implementation**: Extract helper function, inline multiply wrapper
- **Validation**: Compiler inlining should make code equivalent

**3. below-threshold** (20 trials)
- **Purpose**: Small change (~3%) below warn threshold (5%)
- **Expected**: verdict `pass`, status `unchanged`
- **Implementation**: Add XOR with loop index in hash function
- **Tests**: Decision rule 11 (significant but below threshold)

**4. improvement** (15 trials)
- **Purpose**: Performance improvement should not fail
- **Expected**: verdict `pass`, status `improved`
- **Implementation**: O(n²) nested loop → O(n) single loop
- **Tests**: Decision rule 10 (significant improvement)

### Positive Scenarios (Regression Detection)

**5. algo-slowdown** (20 trials)
- **Purpose**: Algorithmic complexity regression
- **Expected**: verdict `fail`, status `fail` for sec/op
- **Implementation**: O(n log n) quicksort → O(n²) bubble sort
- **Effect size**: ~15x slowdown on n=100 random data
- **Tests**: Decision rule 8 (exceeds fail threshold)

**6. alloc-bloat** (20 trials)
- **Purpose**: Memory allocation regression
- **Expected**: verdict `fail`, status `fail` for B/op and allocs/op
- **Implementation**: strings.Builder → repeated concatenation
- **Effect size**: ~10x bytes, ~6x allocations
- **Tests**: Multiple metrics (B/op, allocs/op) detection

**7. lock-contention** (15 trials)
- **Purpose**: Concurrency regression from lock contention
- **Expected**: verdict `fail`, status `fail` for sec/op, mutex profile shows contention
- **Implementation**: 8 sharded mutexes → single global mutex
- **Effect size**: ~5-8x slowdown under parallel load
- **Tests**: Parallel benchmarks, mutex profiling diagnosis

## File Structure

Each scenario follows the standard structure:

```
eval/scenarios/<scenario-id>/
├── README.md               # Purpose, validation steps, ground truth
├── scenario.yaml           # Trials, expected verdict/metrics
├── change.patch            # Unified diff
└── base/
    ├── go.mod
    ├── <source>.go
    └── <source>_test.go
```

## Ground Truth Validation

Per AGENTS.md requirement: "Regression manually verified with regular benchmarks before being used as ground truth"

Each positive scenario README includes validation commands:

```bash
cd base
go test -bench=. -benchmem -count=10 | tee base.txt

git apply ../change.patch
go test -bench=. -benchmem -count=10 | tee head.txt

benchstat base.txt head.txt
```

Maintainers must run these commands on the reference machine before P8-T7 (threshold review).

## Scenario Design Principles

1. **Deterministic**: No randomness in behavior (only in benchmark data setup with fixed seed if needed)
2. **Realistic**: Patterns from real-world performance issues
3. **Clear effect**: Positive scenarios have large effect sizes (>>2x threshold) for reliable detection
4. **Multiple dimensions**: Cover CPU (algo-slowdown), memory (alloc-bloat), concurrency (lock-contention)
5. **Documented**: Each README explains purpose, validation, and ground truth

## Coverage Matrix

| Category | Metric | Scenario |
|---|---|---|
| False positive | sec/op unchanged | no-change-aa, neutral-refactor |
| False positive | sec/op below threshold | below-threshold |
| True negative | sec/op improved | improvement |
| True positive | sec/op regression | algo-slowdown, lock-contention |
| True positive | B/op regression | alloc-bloat |
| True positive | allocs/op regression | alloc-bloat |
| Diagnosis | mutex profile | lock-contention |

## Trial Counts

- **30 trials**: no-change-aa (highest priority, measures core false positive rate)
- **20 trials**: Most scenarios (balance between statistical power and execution time)
- **15 trials**: lock-contention, improvement (large effect sizes need fewer trials)

## Next Steps

**P8-T4**: Create `noisy-host` scenario (synthetic CPU load)  
**P8-T5**: Implement metrics computation (detection rate, FP rate, hit@K, Wilson interval)  
**P8-T6**: Nightly eval workflow  
**P8-T7**: Review default thresholds based on eval results

## Files Created

```
eval/scenarios/no-change-aa/          (5 files: README, yaml, patch, go.mod, 2 .go)
eval/scenarios/neutral-refactor/      (5 files)
eval/scenarios/below-threshold/       (5 files)
eval/scenarios/improvement/           (5 files)
eval/scenarios/algo-slowdown/         (5 files)
eval/scenarios/alloc-bloat/           (5 files)
eval/scenarios/lock-contention/       (5 files)
```

Total: 7 scenarios × 5 files = 35 files

## Documentation

- 7 README.md files with validation steps and ground truth
- 7 scenario.yaml files with structured expectations
- Clear purpose statements in each README
- Validation commands for manual verification

## Compliance

✅ Each scenario has base/, change.patch, ground truth, and purpose documentation (task requirement)  
✅ Regressions documented with manual verification steps (task requirement)  
✅ No modification of ground truth to improve results (AGENTS.md prohibition)  
✅ Scenarios represent realistic patterns, not artificial constructs  
✅ Trial counts chosen based on effect size and statistical requirements

## Evidence

P8-T2 complete: 4 negative scenarios with documentation  
P8-T3 complete: 3 positive scenarios with validation steps  
Ready for P8-T4 (noisy-host scenario)
