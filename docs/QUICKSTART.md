# Quick Start Guide

Get PerfGuard running in your Go project in 5 minutes.

## Prerequisites

- Go 1.24 or newer
- Git repository with benchmarks
- Linux or macOS (Windows not yet supported)

## Step 1: Install PerfGuard

```bash
go install github.com/kyyril/perfguard/cmd/perfguard@latest
```

Verify installation:

```bash
perfguard version
```

## Step 2: Initialize configuration

In your repository root:

```bash
perfguard init
```

This creates `perfguard.yaml` with sensible defaults:

```yaml
version: 1
benchmarks:
  packages: ["./..."]
  include: "."
  count: 10
  benchtime: "1s"
thresholds:
  metrics:
    sec/op:    { warn: 5,  fail: 15 }
    B/op:      { warn: 10, fail: 25 }
    allocs/op: { warn: 5,  fail: 20 }
profiling:
  mode: on-regression
  types: [cpu, allocs]
```

## Step 3: Add benchmarks (if needed)

If your project doesn't have benchmarks yet, add one:

```go
// mypackage/example_test.go
package mypackage

import "testing"

func BenchmarkMyFunction(b *testing.B) {
    for i := 0; i < b.N; i++ {
        MyFunction()
    }
}
```

Test it works:

```bash
go test -bench=. ./...
```

## Step 4: Run your first check

Compare your current code against main branch:

```bash
perfguard check --base origin/main
```

This will:
1. Build test binaries for base and head
2. Run benchmarks in interleaved rounds
3. Compute statistics
4. Report results

Example output:

```
PerfGuard: PASS (0 fail, 0 warn, 3 unchanged)
base 3f2a9c1 (origin/main)   head 8be41d7   go1.24 linux/amd64   count=10

Benchmark          Metric      Base      Head      Delta    p-value  Status
BenchmarkEncode    sec/op      1.21us    1.22us    +0.8%    0.4100   unchanged
BenchmarkEncode    B/op        512       512       0.0%     1.0000   unchanged
BenchmarkEncode    allocs/op   5         5         0.0%     1.0000   unchanged
```

## Step 5: Add GitHub Actions

Create `.github/workflows/perfguard.yml`:

```yaml
name: Performance Check

on:
  pull_request:
    branches: [main]

permissions:
  contents: read
  pull-requests: write

jobs:
  performance:
    runs-on: ubuntu-latest
    timeout-minutes: 45
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      
      - uses: kyyril/perfguard@v1
        with:
          base: ${{ github.event.pull_request.base.sha }}
          config: perfguard.yaml
          token: ${{ secrets.GITHUB_TOKEN }}
```

Commit and push:

```bash
git add .github/workflows/perfguard.yml perfguard.yaml
git commit -m "Add PerfGuard performance checks"
git push
```

## Step 6: Test on a PR

1. Create a branch with a change
2. Open a pull request
3. PerfGuard runs automatically
4. Check the PR comment for results

## Next steps

### Tune thresholds

Adjust thresholds in `perfguard.yaml` based on your tolerance:

```yaml
thresholds:
  metrics:
    sec/op:    { warn: 10, fail: 25 }  # More lenient
```

### Add custom metrics

If your benchmarks report custom metrics:

```go
func BenchmarkRequest(b *testing.B) {
    var total time.Duration
    for i := 0; i < b.N; i++ {
        start := time.Now()
        HandleRequest()
        total += time.Since(start)
    }
    p95 := calculateP95(samples)
    b.ReportMetric(p95.Milliseconds(), "p95-ms")
}
```

Configure thresholds:

```yaml
thresholds:
  custom:
    p95-ms: { warn: 5, fail: 15, direction: lower_is_better }
```

### Enable more profile types

For contention analysis:

```yaml
profiling:
  types: [cpu, allocs, mutex, block]
```

### Use stored baseline mode

For more stable results across runs:

```bash
# Save baseline
perfguard baseline save --name main-baseline

# Compare against stored baseline
perfguard check --mode stored --baseline main-baseline
```

## Common issues

### "No benchmarks found"

Ensure you have `Benchmark*` functions that run with `go test -bench=.`

### "Shallow clone detected"

In GitHub Actions, add `fetch-depth: 0` to checkout step.

### Results are "inconclusive"

Increase `benchmarks.count` or accept noise on shared runners.

### Benchmarks take too long

Reduce `benchmarks.benchtime` from "1s" to "500ms".

## Getting help

- Read full docs: `docs/ARCHITECTURE.md`, `docs/TESTING.md`
- Check examples: `docs/EXAMPLES.md`
- Open issue: https://github.com/kyyril/perfguard/issues
- Review eval scenarios: `eval/scenarios/` for reference implementations

## Success criteria

You've successfully integrated PerfGuard when:

- ✓ Local `perfguard check` runs without errors
- ✓ CI workflow runs on PRs
- ✓ PR comments show benchmark comparison
- ✓ Thresholds match your project's needs

**You're done! PerfGuard is now watching for performance regressions.**
