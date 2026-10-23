# P8-T7: Threshold Review and Correction Decision

## Task

Review default thresholds and `correction` setting based on evaluation results (Q-01).

## Current Defaults

From `internal/config/config.go`:

```go
Thresholds: ThresholdsConfig{
    Alpha:      0.05,
    MinSamples: 6,
    Correction: "none",
    ZeroBase:   "warn",
    Metrics: map[string]ThresholdValue{
        "sec/op":    {Warn: 5, Fail: 15},
        "B/op":      {Warn: 10, Fail: 25},
        "allocs/op": {Warn: 5, Fail: 20},
    },
}
```

## Q-01: Default Multiple Testing Correction

**Question:** Should the default `thresholds.correction` be `none` or `holm`?

**Trade-offs:**

### No Correction (Current Default)

**Pros:**
- Simpler mental model for users
- More sensitive to real regressions
- The effect size gate (5%, 10%, 15%, 20%, 25% thresholds) already provides protection
- Typical Go projects have 5-20 benchmarks, not hundreds

**Cons:**
- Higher false positive rate when many benchmarks and metrics are tested
- Family-wise error rate scales with number of comparisons

### Holm-Bonferroni Correction

**Pros:**
- Controls family-wise error rate across all comparisons
- More conservative, fewer false positives
- Good for projects with many benchmarks (50+)

**Cons:**
- Reduces statistical power (more false negatives)
- Can miss moderate regressions when many tests are performed
- More complex to explain to users

## Decision Framework

The decision depends on the false positive rate measured in the A/A scenario (`no-change-aa`):

1. **If FP rate ≤ 5%** with `correction: none`:
   - Keep default as `none`
   - Rationale: The effect size gates are sufficient
   - Users with many benchmarks can opt into `holm`

2. **If FP rate > 5% but ≤ 10%** with `correction: none`:
   - Keep default as `none` with documentation
   - Document that projects with 50+ benchmarks should use `holm`
   - Add warning in report when many comparisons made

3. **If FP rate > 10%** with `correction: none`:
   - Change default to `holm`
   - Rationale: Unacceptably high false positive rate
   - Document how to disable for maximum sensitivity

## Expected Baseline False Positive Rate

Theoretical calculation for `no-change-aa` scenario:

- Alpha = 0.05 (95% confidence)
- Single benchmark, 3 metrics (sec/op, B/op, allocs/op)
- Effect size gates: 5%, 10%, 5%

**Without correction:**
- Probability of any false positive ≈ 1 - (1 - 0.05)³ ≈ 14.3%
- BUT: effect size gate reduces this significantly
- On truly identical code, effect = 0%, so p-value alone doesn't trigger

**Expected:** < 5% false positive rate because effect size gate catches noise

**With Holm correction:**
- Adjusted alpha for 3 tests: α/3, α/2, α/1
- Even lower false positive rate
- Expected: < 2%

## Measurement Plan

Run nightly eval workflow for 1-2 weeks to collect:

1. **Detection rate** on positive scenarios (algo-slowdown, alloc-bloat, lock-contention)
   - Target: ≥85% with `correction: none`
   - If < 85%, analyze which scenarios fail and why

2. **False positive rate** on no-change-aa (30 trials)
   - Measure with `correction: none`
   - Measure with `correction: holm`
   - Compare against targets

3. **Sensitivity analysis** on below-threshold scenario
   - Should pass (change is ~3%, below 5% threshold)
   - Verify both `none` and `holm` behave correctly

## Preliminary Recommendation (Before Data)

**Default: `correction: none`**

Rationale:
1. Effect size gates are the primary safeguard
2. Mann-Whitney U test is already conservative
3. Typical Go projects have modest benchmark counts
4. False negatives (missing real regressions) are more costly than false positives
5. Users can opt into `holm` for high-comparison-count projects

**Conditions to change mind:**
- If A/A false positive rate exceeds 10% in eval results
- If real-world dogfooding shows excessive false alarms

## Alternative Approaches Considered

### Adaptive Correction
Automatically enable Holm when number of comparisons > threshold (e.g., 20).

**Rejected because:**
- Adds hidden complexity
- Makes results dependent on unrelated benchmarks
- Users should make explicit choices

### Benjamini-Hochberg (FDR Control)
Less conservative than Holm, controls false discovery rate instead of family-wise error rate.

**Rejected because:**
- More complex to explain
- `none` vs `holm` is sufficient choice space
- Can be added later if needed

### Bayesian Approach
Use prior probabilities and posterior odds.

**Rejected because:**
- Much more complex implementation
- Requires choosing priors
- Out of scope for v1.0

## Documentation Updates Needed

Once decision is finalized:

1. **ARCHITECTURE.md section 7**: Update with final default and rationale
2. **Config schema**: Document when to use each option
3. **FAQ**: Add "Should I use Holm correction?" entry
4. **Report**: Add note when many comparisons made without correction

## Timeline

1. **Week 1-2**: Run nightly eval, collect results
2. **Review data**: Analyze detection and false positive rates
3. **Make decision**: Keep `none` or change to `holm`
4. **Document**: Update ARCHITECTURE with rationale and data
5. **If changed**: Update config default and regenerate golden tests

## Current Status

**Blocked:** Requires nightly eval results from P8-T6 workflow.

**Next step:** Wait for 5-10 nightly runs to accumulate, then analyze:
- `eval/RESULTS.md` from each run
- Aggregate false positive rate across all A/A trials
- Detection rates for each positive scenario
- Variability across runs

**Estimated completion:** 2026-11-01 (after 1 week of nightly data)
