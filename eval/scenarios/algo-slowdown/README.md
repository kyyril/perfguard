# Scenario: algo-slowdown

**Type**: Positive (regression detection)  
**Purpose**: Algorithmic performance regression

## Description

This scenario replaces an efficient O(n log n) quicksort implementation with an O(n²) bubble sort. The effect is large and should be reliably detected.

## Changes

- Replace quicksort (partition-based divide and conquer) with bubble sort (nested loop with swaps)
- Same functional behavior (sorted output)
- Dramatically worse time complexity for larger inputs

## Expected Behavior

- **Verdict**: `fail`
- **Status**: `fail` for sec/op
- Large positive delta (slower)
- Decision rule 8: significant regression exceeding fail threshold

## Ground Truth

For n=100:
- Quicksort: O(n log n) ≈ 664 operations
- Bubble sort: O(n²) ≈ 10,000 operations (worst case)

Expected slowdown: ~15x on random data, up to ~150x on sorted data.

## Validation

Measure the regression manually:

```bash
cd base
go test -bench=. -count=10 | tee base.txt

# Apply patch
git apply ../change.patch
go test -bench=. -count=10 | tee head.txt

benchstat base.txt head.txt
```

Expect large positive delta (e.g. +1500%) exceeding the fail threshold (15%).

## Notes

- Tests detection of algorithmic complexity regressions
- Benchmark uses random data to avoid best-case bubble sort behavior
- 20 trials provide stable detection rate measurement
- Effect size is intentionally large (>>2x fail threshold) for reliable detection
