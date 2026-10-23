# P8-T4 Implementation Summary

**Task**: noisy-host scenario (synthetic CPU load)  
**Status**: Complete  
**Date**: 2026-10-23

## Overview

Created the `noisy-host` scenario to validate PerfGuard's behavior under high measurement noise, simulating a shared CI runner with background CPU contention.

## Scenario Details

**Purpose**: Test that measurement noise is correctly identified as `inconclusive` rather than misclassified as a performance regression.

**Implementation**:
- `NoiseGenerator` type that spawns goroutines on all CPU cores
- Each worker performs math-heavy bursts every 10ms
- Started in `init()` so noise is present during benchmark execution
- Simulates intermittent CPU steal from other processes

**Expected Behavior**:
- **Verdict**: `pass` (noise should not fail the check)
- **Status**: `inconclusive` for sec/op
- **Reason**: `noisy` (decision rule 5: CV exceeds max_cv after extra samples)
- Tests adaptive sampling and `noise.on_noisy` configuration

## Ground Truth

Base and head code compute the same deterministic result. The only difference is background CPU contention in head, which increases measurement variance but represents environmental noise, not a code regression.

**Expected CV**:
- Base: <5% (stable measurement)
- Head: >10% (high variance from background load)

## Relationship to Q-04

Per PRD section 14, Q-04 asks: "Appropriate default `max_cv` value for GitHub hosted runners."

This scenario helps answer that question:
- Default `max_cv: 10` is set for stable dedicated hardware
- Shared runners may experience higher baseline noise
- Eval results from this scenario on both reference and shared runners inform the decision
- If `inconclusive` is too frequent on shared runners, may need:
  - Higher default `max_cv` (15-20%) for CI environments
  - Environment-specific config profiles
  - Documentation recommending self-hosted runners

Decision will be documented in P8-T7 (threshold review) after running the eval suite.

## Files Created

```
eval/scenarios/noisy-host/
├── README.md              (70 lines: purpose, validation, Q-04 discussion)
├── scenario.yaml          (9 lines)
├── change.patch           (18 lines)
└── base/
    ├── go.mod             (3 lines)
    ├── workload.go        (60 lines: Compute + NoiseGenerator)
    └── workload_test.go   (8 lines)
```

## Validation

The README includes validation commands to verify that:
1. Base measurements show low CV (<5%)
2. Head measurements show high CV (>10%)
3. Variance is visibly higher in head runs

Maintainers should run these commands on the reference machine before P8-T7.

## Testing Strategy

**20 trials** balance:
- Statistical power to measure `inconclusive` rate
- Execution time (noise generation adds overhead)
- Verification that adaptive sampling works under noise

**Expected outcomes**:
- Most trials: `inconclusive` (CV >10% after extra samples)
- Some trials: `warn` (if noise.on_noisy configured as warn)
- Zero trials: `fail` (would be a false positive)

## Next Steps

This completes the scenario creation phase. Next tasks:

**P8-T5**: Metrics computation
- Detection rate (positive scenarios)
- False positive rate (negative scenarios)
- Hit@K for diagnosis (lock-contention)
- Wilson confidence intervals
- `perfguard eval report` command

**P8-T6**: Nightly eval workflow

**P8-T7**: Review default thresholds based on eval results
- Analyze noisy-host results to inform Q-04
- Document decision on max_cv default

## Evidence

- P8-T4 marked [x] in IMPLEMENTATION_PLAN.md
- Complete scenario with documentation
- README addresses Q-04 per task requirements
- Scenario ready for P8-T5 (metrics computation)
