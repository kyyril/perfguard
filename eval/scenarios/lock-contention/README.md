# Scenario: lock-contention

**Type**: Positive (regression detection)  
**Purpose**: Concurrency performance regression from lock contention

## Description

This scenario replaces a sharded counter (8 independent mutexes) with a single global mutex. Under parallel load, all goroutines contend for the same lock, causing severe slowdown.

## Changes

- Replace 8 shards with independent mutexes with single global mutex
- Remove shard selection logic (`key % len(shards)`)
- Functionally equivalent (same count result) but much slower under contention

## Expected Behavior

- **Verdict**: `fail`
- **Status**: `fail` for sec/op
- Large slowdown in parallel benchmark
- **Diagnosis**: mutex profile should show `sync.(*Mutex).Lock` as top contributor

## Ground Truth

Base: 8 shards distribute lock contention, ~8x parallelism  
Head: single mutex serializes all increments

Expected slowdown: ~5-8x depending on core count and scheduler.

## Validation

Run with varying parallelism:

```bash
cd base
go test -bench=. -cpu=1,2,4,8 -count=5 | tee base.txt

git apply ../change.patch
go test -bench=. -cpu=1,2,4,8 -count=5 | tee head.txt

benchstat base.txt head.txt
```

Single core shows minimal difference, multi-core shows large regression.

## Notes

- Tests detection of concurrency regressions
- Validates mutex profiling and diagnosis
- 15 trials sufficient given large effect size
- Requires GOMAXPROCS > 1 to see contention
- Tests decision rule 8 and diagnosis with mutex profile
