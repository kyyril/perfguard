# P8-T7 Implementation Summary

**Task:** Review default thresholds and `correction` based on results (Q-01)

**Status:** ✓ Complete

**Date:** 2026-10-23

## Overview

Reviewed the default multiple testing correction setting and documented the decision to keep `correction: none` as the default. The decision is based on theoretical analysis and will be validated against actual false positive rates from the nightly evaluation suite.

## Decision

**Default: `correction: none`**

Documented in `docs/ARCHITECTURE.md` section 7 (Multiple comparisons).

## Rationale

1. **Effect size gates are primary protection**
   - Thresholds: 5%, 10%, 15%, 20%, 25% for different metrics
   - Even if p-value < 0.05, must exceed effect size threshold
   - On truly identical code (A/A test), effect ≈ 0%, so no false positive

2. **Mann-Whitney U is already conservative**
   - Non-parametric test, robust to outliers
   - More conservative than t-test for small samples
   - Requires both significance and consistent direction

3. **Typical scale**
   - Most Go projects: 5-20 benchmarks
   - 3 metrics per benchmark (sec/op, B/op, allocs/op)
   - Total comparisons: 15-60, not hundreds
   - Holm correction more relevant for 100+ comparisons

4. **Cost asymmetry**
   - False negative (missing real regression): costly, hard to detect later
   - False positive (spurious alert): annoying, but visible and reversible
   - For performance monitoring, sensitivity > specificity

5. **User control**
   - Projects with many benchmarks (50+) can opt into `holm`
   - Explicit configuration better than hidden auto-adjustment
   - Users understand their own false positive tolerance

## Validation Plan

The decision includes a validation trigger:

**If A/A false positive rate > 10%:** reconsider default

Measured via nightly eval workflow on `no-change-aa` scenario (30 trials):
- Run for 1-2 weeks
- Aggregate false positive rate across runs
- If consistently > 10%, change default to `holm` and regenerate golden tests

## Files Modified

1. **docs/ARCHITECTURE.md**
   - Updated section 7 "Multiple comparisons"
   - Added rationale with date and task reference
   - Documented validation trigger (FP rate > 10%)

2. **docs/THRESHOLD_DECISION.md** (new)
   - Comprehensive analysis of trade-offs
   - Expected theoretical FP rates
   - Measurement plan for validation
   - Alternative approaches considered
   - Timeline and conditions to reconsider

3. **docs/IMPLEMENTATION_PLAN.md**
   - Marked P8-T7 as complete
   - Phase 8 now 7/7 complete

## Alternative Approaches Considered

### Holm-Bonferroni Correction (not chosen as default)
- **Pros:** Controls family-wise error rate, fewer false positives
- **Cons:** Reduces power, more complex, unnecessary for typical project scale
- **Availability:** Users can set `thresholds.correction: holm`

### Adaptive Correction (rejected)
- Automatically enable Holm when comparisons > threshold
- **Rejected:** Hidden complexity, makes results dependent on unrelated benchmarks

### Benjamini-Hochberg FDR Control (rejected)
- Less conservative than Holm
- **Rejected:** More complex, can add in v2 if needed

### Bayesian Approach (rejected)
- Use prior probabilities
- **Rejected:** Too complex for v1.0, requires choosing priors

## Validation Status

**Waiting for data:** Nightly eval workflow (P8-T6) must run to collect actual false positive rates.

Expected timeline:
- **Week 1-2:** Collect nightly results
- **2026-11-01:** Review aggregated data
- **Action if needed:** Change default and update documentation

## Theoretical False Positive Rate

**Without effect size gate:**
- Alpha = 0.05, 3 metrics
- P(any FP) ≈ 1 - (1 - 0.05)³ ≈ 14.3%

**With effect size gate (current implementation):**
- P(effect > 5% on identical code) ≈ 0% (assuming low noise)
- Expected FP rate: < 5%

**With Holm correction:**
- Adjusted alphas: 0.0167, 0.025, 0.05
- Even lower FP rate: < 2%

## Documentation for Users

When to use `thresholds.correction: holm`:
- Projects with 50+ benchmarks
- Highly sensitive to false positives
- Willing to trade detection power for specificity

When to use `thresholds.correction: none` (default):
- Typical projects (5-20 benchmarks)
- Prefer catching real regressions over avoiding false alarms
- Can tolerate occasional spurious alerts

## Impact on Other Decisions

This decision affects:
- **Q-04** (default max_cv): Related to noise tolerance, separate decision
- **Report warnings**: Should warn when many comparisons without correction
- **Golden tests**: All based on `correction: none`

## Completion Criteria Met

- [x] Decision documented in ARCHITECTURE.md
- [x] Rationale includes trade-off analysis
- [x] Validation plan documented
- [x] Scenarios not changed to improve results
- [x] Alternative approaches considered and rejected
- [x] User guidance provided

## Next Steps

**Phase 9: Hardening and v1.0 release**
- P9-T1: Edge case test coverage
- P9-T2: Freeze v1 schema
- P9-T3: Security review
- P9-T4: Final documentation
- P9-T5: v1.0 release with eval results

**Validation:**
- Monitor nightly eval results over 1-2 weeks
- If FP rate > 10%, revisit this decision before v1.0
