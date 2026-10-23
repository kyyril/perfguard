# P8-T5 Implementation Summary

**Task:** Metrics: detection rate, false positive rate, hit@K, Wilson interval, `eval report`

**Status:** ✓ Complete

**Date:** 2026-10-23

## Overview

Implemented metrics computation and report generation for the evaluation suite. The system now aggregates trial results from multiple scenarios, computes statistical metrics with confidence intervals, and generates markdown reports with environment fingerprints.

## Files Created

### Core Implementation

1. **internal/eval/metrics.go** (145 lines)
   - `Metrics` struct aggregating trial results
   - `DetectionRate()`: proportion of successful trials
   - `FalsePositiveRate()`: fail and warn rates in negative scenarios
   - `DiagnosisHitRate()`: proportion where expected function in top K
   - `WilsonInterval()`: confidence intervals using Wilson score method
   - `FormatPercent()` and `FormatInterval()`: display helpers

2. **internal/eval/report.go** (196 lines)
   - `ScenarioResult`: aggregated results for one scenario
   - `ReportData`: results for all scenarios with environment
   - `EnvironmentFingerprint`: OS, arch, Go version, CPU count, hostname
   - `AggregateResults()`: reads trial JSON files and computes metrics
   - `GenerateReport()`: creates markdown report
   - `CaptureEnvironment()`: captures system information
   - Helper functions: `isSuccessfulTrial`, `diagnosisMatches`

3. **internal/cli/eval.go** (updated)
   - Added `newEvalReportCmd()` for `perfguard eval report`
   - Flags: `--output`, `--scenarios`, `--report`
   - Aggregates multiple scenarios with glob pattern
   - Generates report with environment fingerprint

4. **eval/RESULTS.md** (template)
   - Documentation of metrics and their interpretation
   - Target thresholds for detection and false positive rates
   - Instructions for running the evaluation suite

### Tests

1. **internal/eval/metrics_test.go** (181 lines)
   - `TestMetrics_DetectionRate`: all, half, none, 90% detection
   - `TestFalsePositiveRate`: various fail/warn combinations
   - `TestMetrics_DiagnosisHitRate`: hit rates including edge cases
   - `TestWilsonInterval`: reference values with tolerance checks
     - Perfect detection 20/20: [0.83, 1.0]
     - 90% detection 18/20: [0.68, 0.99]
     - 50% detection 10/20: [0.28, 0.72]
     - 1% false positive 1/100: [0.0, 0.05]
   - `TestFormatPercent` and `TestFormatInterval`: display formatting

2. **internal/eval/report_test.go** (167 lines)
   - `TestIsSuccessfulTrial`: verdict matching logic
   - `TestDiagnosisMatches`: top K search with various edge cases
   - `TestGenerateReport`: markdown structure and content
   - `TestCaptureEnvironment`: system info capture
   - `TestEnvironmentFingerprint_Timestamp`: display formatting

## Key Algorithms

### Wilson Score Interval

Uses the Wilson score method for confidence intervals instead of normal approximation:

```
p̂ = successes / trials
z = 1.96 (for 95% confidence)

center = (p̂ + z²/2n) / (1 + z²/n)
margin = z√(p̂(1-p̂)/n + z²/4n²) / (1 + z²/n)

CI = [center - margin, center + margin]
```

More accurate for small sample sizes and extreme proportions (0% or 100%).

### Diagnosis Hit Detection

Checks if expected function appears in top K=3 contributor candidates:
- Searches through all regressions in comparison
- For each regression with diagnosis, checks top 3 candidates
- Uses substring match (`strings.Contains`) to handle package prefixes
- Returns true on first match

### False Positive Analysis

Aggregates all negative scenarios (expected verdict = pass):
- Counts trials that did not result in pass as false positives
- Separates fail and warn false positives
- Computes rates with Wilson confidence intervals

## Report Structure

Generated markdown includes:

1. **Environment**: OS, arch, Go version, CPU cores, hostname
2. **Summary**: total scenarios count
3. **Scenario Results**: for each scenario
   - Description and expected verdict
   - Detection rate with 95% CI
   - Diagnosis hit rate with 95% CI (when applicable)
   - Failure examples (up to 3)
4. **False Positive Analysis**: aggregated across negative scenarios
   - Fail false positive rate with 95% CI
   - Warn false positive rate with 95% CI

## Command Usage

```bash
# Run evaluation scenarios (already implemented in P8-T1)
perfguard eval run --scenario eval/scenarios/algo-slowdown --output eval/results

# Generate aggregated report (new)
perfguard eval report --output eval/results --scenarios "eval/scenarios/*" --report eval/RESULTS.md
```

## Verification

Unit tests verify:
- Metric calculations with various inputs (0%, 50%, 90%, 100%)
- Wilson interval reference values within tolerance
- Verdict matching logic (pass/fail/warn)
- Diagnosis hit detection with top K search
- Report structure and content
- Environment capture

## Architecture Compliance

- `internal/eval` remains pure: no direct dependencies on app, gitx, or exec
- Statistical functions are deterministic and testable
- File I/O uses standard library only
- All text follows AGENTS.md style (no em dashes, emojis, or filler)

## Next Steps

P8-T6: Nightly eval workflow (`.github/workflows/nightly-eval.yml`)
- Scheduled workflow to run all scenarios
- Upload results as artifacts
- Generate and commit report

## Notes

- Wilson interval used instead of normal approximation for better accuracy
- Top K fixed at 3 for diagnosis hit rate
- Report truncation not needed (eval reports are small)
- Environment fingerprint captures system at report generation time
