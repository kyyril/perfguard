# Scenario: alloc-bloat

**Type**: Positive (regression detection)  
**Purpose**: Memory allocation and GC pressure regression

## Description

This scenario replaces efficient `strings.Builder` (grows buffer, single allocation) with naive string concatenation (reallocates on every append). The regression affects both B/op and allocs/op.

## Changes

- Replace `strings.Builder` with `result += p` concatenation
- Each concatenation creates a new string (immutable in Go)
- For 13 parts, this creates 12 intermediate strings vs 1 final string

## Expected Behavior

- **Verdict**: `fail`
- **Status**: `fail` for B/op and allocs/op
- Large increase in both bytes allocated and allocation count
- Both metrics should exceed fail threshold

## Ground Truth

Base (strings.Builder):
- Allocations: 1-2 (initial buffer + possible resize)
- Bytes: ~50 (final string size)

Head (concatenation):
- Allocations: 12 (one per append)
- Bytes: ~450 (sum of all intermediate strings)

Expected increase: ~10x for B/op, ~6x for allocs/op.

## Validation

```bash
cd base
go test -bench=. -benchmem -count=10 | tee base.txt

git apply ../change.patch
go test -bench=. -benchmem -count=10 | tee head.txt

benchstat base.txt head.txt
```

Check that both B/op and allocs/op show large increases.

## Notes

- Tests detection of memory allocation regressions
- Validates that both B/op and allocs/op metrics work correctly
- 20 trials provide stable measurement
- Real-world pattern: forgetting to use Builder in string-heavy code
