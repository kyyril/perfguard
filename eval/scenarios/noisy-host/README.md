# Scenario: noisy-host

**Type**: Edge case (noise handling)  
**Purpose**: Validate behavior under high measurement noise (simulated shared runner)

## Description

This scenario introduces synthetic background CPU load that runs concurrently with benchmarks, creating measurement variability. The noise generator spawns workers on all CPU cores that periodically perform bursts of computation, simulating a noisy shared CI runner.

## Changes

The patch starts a `NoiseGenerator` in `init()` before benchmarks run. Base code has no noise, head code runs benchmarks while background workers consume CPU cycles intermittently.

## Expected Behavior

- **Verdict**: `pass` (not `fail` - noise should not be classified as regression)
- **Status**: `inconclusive` (high CV triggers adaptive sampling, then noise.on_noisy rule)
- **Reason**: `noisy` after extra samples exhausted
- Decision rule 5: CV exceeds noise.max_cv

## Ground Truth

Base and head compute the same deterministic result. The only difference is background CPU contention in head, which increases measurement variance (CV) but does not represent a true performance regression in the benchmark code itself.

With default `noise.max_cv: 10` (percent):
- Base CV: typically <5% (stable measurement)
- Head CV: likely >10% (noisy from background load)

Adaptive sampling will add rounds up to `execution.max_extra_runs` (default 10), but if CV remains high, the result should be `inconclusive` per `noise.on_noisy: inconclusive`.

## Validation

Measure CV in both sides:

```bash
cd base
go test -bench=. -count=20 | tee base.txt

git apply ../change.patch
go test -bench=. -count=20 | tee head.txt

# Compute CV manually or with benchstat
benchstat base.txt head.txt
```

Base should show low CV (<5%). Head should show high CV (>10%) and large variance between runs.

## Notes on Default max_cv

Per PRD Q-04, this scenario helps answer: "Appropriate default `max_cv` value for GitHub hosted runners."

- Default `max_cv: 10` is set for stable dedicated hardware
- GitHub Actions shared runners may see higher baseline noise
- If this scenario frequently produces `inconclusive` even without synthetic load, consider:
  - Raising default `max_cv` to 15-20 for CI environments
  - Documenting that self-hosted runners are recommended for performance tests
  - Adding environment-specific config profiles

Decision documented in P8-T7 after running eval suite on reference and shared runners.

## Implementation Details

**NoiseGenerator**:
- Spawns one goroutine per CPU core
- Each worker performs math-heavy bursts every 10ms
- Simulates intermittent CPU steal from other processes
- Started in `init()` so it runs during the entire benchmark pass

**Why this tests resilience**:
- Shared CI runners experience real background load
- A tool that reports "regression" under noise loses user trust
- The `inconclusive` verdict is the honest answer when measurement quality is poor

## Expected Trial Outcomes

With 20 trials:
- Most trials should report `inconclusive` (CV >10%)
- Some trials may report `warn` if noise.on_noisy is configured as `warn`
- No trial should report `fail` (that would be a false positive)
- Verdict should be `pass` (inconclusive does not fail the check)

## Validation Commands

Test that noise actually increases CV:

```bash
cd base

# Base: no noise, stable
go test -bench=BenchmarkCompute -count=10

# Head: with noise, unstable
git apply ../change.patch
go test -bench=BenchmarkCompute -count=10
```

Observe variance in ns/op across runs. Head should show much higher variance.
