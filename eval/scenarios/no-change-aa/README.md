# Scenario: no-change-aa

**Type**: Negative (false positive measurement)  
**Purpose**: A/A test with identical base and head code to measure false positive rate

## Description

This scenario compares identical code to itself. The patch adds only a comment with no functional changes. This is the primary false positive test.

## Expected Behavior

- **Verdict**: `pass`
- **Status**: `unchanged` for all metrics
- **False positive target**: `fail` in ≤1% of trials, `warn` in ≤5% of trials

## Ground Truth

The `Compute` function performs the same deterministic calculation (sum of squares from 0 to n-1) in both base and head. Any detected regression is a false positive caused by measurement noise.

## Validation

Run regular benchmarks to verify performance is identical:

```bash
cd base
go test -bench=. -count=10
```

All runs should show similar ns/op values within normal variance.

## Notes

- 30 trials provide statistical power to detect false positive rates
- This scenario tests the core promise: noise does not become regression
- Per PRD AC-02, targets are for the reference machine, not guaranteed on all systems
