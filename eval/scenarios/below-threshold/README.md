# Scenario: below-threshold

**Type**: Negative (false positive measurement)  
**Purpose**: Small performance change below warn threshold

## Description

This scenario adds a minor operation (XOR with loop index) that increases execution time by approximately 3%. With default thresholds (warn: 5%, fail: 15%), this should be detected as statistically significant but classified as `unchanged` because it is below the threshold.

## Changes

- Add `h ^= uint64(i)` in the hash loop
- Introduces one additional XOR operation per byte

## Expected Behavior

- **Verdict**: `pass`
- **Status**: `unchanged` (delta ~3% < warn threshold 5%)
- Decision rule 11: significant but below threshold

## Ground Truth

The change adds real work (XOR with index), so it is measurably slower. However, the effect size is small and intentionally below the configured threshold.

## Validation

Measure the actual delta:

```bash
cd base
go test -bench=. -count=10 | tee base.txt

# Apply patch
git apply ../change.patch
go test -bench=. -count=10 | tee head.txt

# Compare with benchstat
benchstat base.txt head.txt
```

Expect delta in the range of 2-4%.

## Notes

- Tests threshold-based filtering (not just statistical significance)
- Validates that small changes do not trigger alerts
- 20 trials provide stable median estimates
- Effect size may vary by CPU; ground truth validated on reference machine
