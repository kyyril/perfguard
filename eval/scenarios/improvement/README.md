# Scenario: improvement

**Type**: Negative (false positive measurement)  
**Purpose**: Performance improvement should report improved, not fail

## Description

This scenario replaces an intentionally slow O(n²) implementation with an efficient O(n) linear search. The improvement is large and should be detected as `improved` status with verdict `pass`.

## Changes

- Replace nested loop (O(n²)) with single loop (O(n))
- Remove redundant index equality check
- Result is functionally identical but much faster

## Expected Behavior

- **Verdict**: `pass`
- **Status**: `improved` for sec/op
- Large negative delta (faster is better for sec/op)
- Decision rule 10: significant improvement above threshold

## Ground Truth

Base implementation: O(n²) with nested loops checking `i == j`  
Head implementation: O(n) with single loop

For n=20, base does ~400 comparisons, head does ~20. Expected speedup: ~20x.

## Validation

Verify the improvement manually:

```bash
cd base
go test -bench=. -count=5 | tee base.txt

# Apply patch
git apply ../change.patch
go test -bench=. -count=5 | tee head.txt

benchstat base.txt head.txt
```

Expect large negative delta (e.g. -95%) indicating improvement.

## Notes

- Tests that improvements do not cause false alerts
- Validates the `improved` status is reported correctly
- Ensures verdict remains `pass` for better performance
- 15 trials sufficient given large effect size
- This tests decision rule 10 (significant improvement)
