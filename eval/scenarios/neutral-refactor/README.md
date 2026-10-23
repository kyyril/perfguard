# Scenario: neutral-refactor

**Type**: Negative (false positive measurement)  
**Purpose**: Structural code change with no performance impact

## Description

This scenario extracts a helper function and introduces an inline multiplication wrapper. The compiler should optimize these away, resulting in identical performance.

## Changes

- Extract calculation logic into `calculate()` helper
- Replace inline `v * 2` with `multiply(v, 2)` call
- Add `multiply()` helper that will be inlined

## Expected Behavior

- **Verdict**: `pass`
- **Status**: `unchanged` for sec/op
- The refactoring should not trigger regression detection

## Ground Truth

Both versions compute `result = sum(data[i] * 2)` with the same algorithmic complexity. The Go compiler inlines small functions, so the generated code should be equivalent.

## Validation

Verify with manual benchmarks:

```bash
cd base
go test -bench=. -benchmem -count=5

# Apply patch, then:
go test -bench=. -benchmem -count=5
```

Compare ns/op values. Variation should be within normal noise (CV <10%).

## Notes

- Tests that code structure changes do not cause false positives
- Validates that PerfGuard focuses on performance, not code style
- 20 trials balance statistical power with execution time
