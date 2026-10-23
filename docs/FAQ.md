# Frequently Asked Questions

## General

### What is PerfGuard?

PerfGuard is a performance regression detection tool for Go projects. It runs benchmarks on base and head code, compares them statistically, and reports whether changes caused performance regressions.

### How does it differ from benchstat?

| Feature | PerfGuard | benchstat |
|---|---|---|
| CI integration | GitHub Action with PR comments | Manual |
| Baseline storage | Built-in with git or filesystem | Manual file management |
| Diagnosis | pprof diff with contributor candidates | Manual pprof analysis |
| Decision rules | Configurable thresholds and corrections | Manual interpretation |
| Exit codes | Automated CI gates | N/A |

PerfGuard builds on benchstat's statistical foundation and adds automation.

### Does PerfGuard find all performance issues?

No. PerfGuard only detects regressions visible in Go benchmarks. It does not:
- Find issues that don't show up in microbenchmarks
- Detect memory leaks that accumulate over hours
- Monitor production performance
- Test under production-like load

See README Limitations section.

## Usage

### What benchmarks should I write?

Write benchmarks that:
1. Exercise code paths critical to performance
2. Run in reasonable time (milliseconds to seconds)
3. Have stable behavior (deterministic operations)
4. Test realistic scenarios, not just the fastest case

Avoid:
- Benchmarks with network or disk I/O
- Highly variable operations (random sampling, caches)
- Benchmarks with global state or dependencies between iterations

### How many samples do I need?

Default is 10 (`benchmarks.count: 10`). Increase if:
- Results are often `inconclusive`
- You want tighter confidence intervals
- Shared runners have high noise

Decrease if:
- Benchmarks take too long
- Results are consistently stable

Minimum is `thresholds.min_samples` (default 6).

### What thresholds should I use?

Start with defaults:
```yaml
thresholds:
  metrics:
    sec/op:    { warn: 5,  fail: 15 }
    B/op:      { warn: 10, fail: 25 }
    allocs/op: { warn: 5,  fail: 20 }
```

Adjust based on:
- Your tolerance for performance changes
- Benchmark stability on your infrastructure
- False positive rate from eval results

Run evaluation suite to measure actual detection quality.

### When should I use Holm correction?

Use `thresholds.correction: holm` when:
- Your project has 50+ benchmarks
- You see frequent false positives
- You prefer low false positive rate over sensitivity

Keep `correction: none` (default) when:
- Typical project size (5-20 benchmarks)
- Effect size gates provide sufficient protection
- Missing real regressions is more costly than false alarms

## CI Integration

### Why use pull_request not pull_request_target?

`pull_request_target` runs in the base branch context with a write token. Since PerfGuard executes PR code (benchmarks), this would give PR code access to repository secrets.

Always use `pull_request` which runs PR code in a read-only context.

### Do fork PRs work?

Yes. PerfGuard detects fork PRs and:
- Skips comment publishing (no write token)
- Writes step summary instead
- Job doesn't fail

Fork PR authors see results in the Actions summary tab.

### Why are results inconclusive?

`inconclusive` status means:
1. **High noise:** CV exceeds `noise.max_cv` (default 10%)
2. **Insufficient samples:** Fewer than `thresholds.min_samples`
3. **Environment mismatch:** Stored baseline from different hardware

Solutions:
- Increase `benchmarks.count`
- Use dedicated runners (not shared)
- Check `noise.max_cv` setting
- Use `same-runner` mode

### How do I handle inconclusive results in CI?

Options:

1. **Accept as passing** (default):
   ```bash
   perfguard check --base $BASE  # exit 0 on inconclusive
   ```

2. **Fail the build:**
   ```bash
   perfguard check --base $BASE --strict  # exit 4 on inconclusive
   ```

3. **Allow warnings but block failures:**
   ```bash
   perfguard check --base $BASE --fail-on fail
   ```

## Statistics

### What statistical tests does PerfGuard use?

- **Mann-Whitney U test**: Non-parametric test for difference in distributions
- **Bootstrap confidence interval**: Resampling-based CI for effect size
- **Coefficient of variation (CV)**: Noise measure

Why Mann-Whitney U:
- Robust to outliers
- No normality assumption
- Works with small samples

### What does p-value mean?

p-value < 0.05 means: if there were no real difference, we'd see a change this large less than 5% of the time.

**Important:** PerfGuard uses both p-value AND effect size. Small p-value with <5% change still reports `unchanged`.

### What is the confidence interval?

95% CI [+3.2%, +8.7%] means: we're 95% confident the true effect is between +3.2% and +8.7%.

Wider intervals indicate more uncertainty (often from high noise or small sample size).

### Does PerfGuard correct for multiple comparisons?

Optional. Set `thresholds.correction: holm` for Holm-Bonferroni correction.

Default is `none` because:
- Effect size gates provide primary protection
- Typical projects have modest comparison counts
- False negatives more costly than false positives

## Diagnosis

### What are "contributor candidates"?

Functions that show the largest change in CPU or allocation profiles. Called "candidates" not "causes" because:
- Correlation ≠ causation
- Inlining can attribute cost to caller
- Sampling introduces uncertainty
- Indirect effects (cache misses, allocations) may not point to root cause

Always manually verify findings.

### Why isn't the expected function in the diagnosis?

Possible reasons:
1. **Inlining:** Function inlined, cost attributed to caller
2. **Sampling:** Profile didn't capture enough samples
3. **Below min_share:** Change too small relative to total
4. **Wrong profile type:** Use `allocs` for memory, `mutex` for contention
5. **Indirect effect:** Change is in a library, not the benchmark itself

Check raw profiles with `go tool pprof -diff_base`.

### Should I trust the diagnosis?

Use diagnosis as a starting point, not conclusion. Always:
1. Review the code changes in the PR
2. Check if suggested functions were modified
3. Reproduce with `go tool pprof -diff_base`
4. Profile locally with additional instrumentation if needed

## Configuration

### Can I override thresholds per benchmark?

Yes, use `thresholds.overrides`:

```yaml
thresholds:
  overrides:
    - benchmark: "BenchmarkCriticalPath"
      metrics:
        sec/op: { warn: 2, fail: 5 }  # Stricter
    
    - benchmark: "BenchmarkColdPath"
      metrics:
        sec/op: { warn: 20, fail: 50 }  # More lenient
```

### Can I skip certain benchmarks?

Yes, use `benchmarks.exclude`:

```yaml
benchmarks:
  exclude:
    - "BenchmarkFlaky"
    - ".*Unstable.*"  # Regex
```

### How do I test only specific packages?

```yaml
benchmarks:
  packages:
    - "./internal/core"
    - "./pkg/api"
```

Or use CLI:
```bash
perfguard check --base origin/main --packages "./internal/core"
```

## Baseline Management

### When should I use stored mode?

Use `stored` mode when:
- You have a dedicated baseline runner
- You want to compare all PRs against one reference
- Infrastructure changes between runs

Use `same-runner` mode (default) when:
- Running in CI on same infrastructure
- Want to minimize env mismatch issues
- Have consistent shared runners

### How do I update stored baselines?

```bash
# On main branch after release
perfguard baseline save --name v1.2.0

# Set default baseline
perfguard baseline save --name main --set-default
```

### What happens if I change hardware?

Environment mismatch detected and results marked `inconclusive`. Fields checked:
- OS and architecture
- Go version
- CPU model
- Number of cores
- GOMAXPROCS

## Troubleshooting

### Build fails with "package not found"

Ensure you're running from repository root with valid `go.mod`.

### Benchmarks panic or timeout

Check:
1. Benchmarks work locally: `go test -bench=. -timeout=5m ./...`
2. Increase `benchmarks.timeout` in config
3. Review benchmark code for race conditions

### Results differ between local and CI

Shared CI runners have variable performance. This is expected. Consider:
- Increasing `benchmarks.count`
- Using `stored` mode with dedicated runner
- Accepting higher `inconclusive` rate

### Git error: "shallow clone detected"

In GitHub Actions:
```yaml
- uses: actions/checkout@v4
  with:
    fetch-depth: 0  # Add this
```

### High memory usage during profiling

Profile collection requires memory. If benchmarks allocate heavily:
1. Reduce `profiling.runs` from 3 to 2
2. Profile fewer benchmarks (filter with `profiling.filter`)
3. Increase runner memory

## Evaluation

### What is the evaluation suite?

8 scenarios with intentional regressions used to measure PerfGuard's detection quality:
- Detection rate: % of real regressions caught
- False positive rate: % of false alarms
- Diagnosis hit rate: % where expected function in top 3

Run with:
```bash
make eval
```

### What's a good detection rate?

Target ≥85% for clear regressions (>10% change, 20+ trials).

Lower rates may indicate:
- Thresholds too high
- Sample count too low
- High noise environment

### What's acceptable for false positives?

Target ≤5% for fail verdicts, ≤10% for warn verdicts.

Higher rates suggest:
- Thresholds too low
- Need Holm correction
- A/A test noise too high

## Advanced

### Can I run PerfGuard outside GitHub Actions?

Yes:
```bash
perfguard check --base origin/main
```

Works on GitLab, local, or any git repository.

### Can I integrate with other CI systems?

Yes. PerfGuard is just a CLI tool. Integration points:
- Exit codes (0=pass, 1=fail, 2=error, 3=execution, 4=inconclusive)
- JSON output: `perfguard check --format json`
- Text output: parseable by CI systems

### How do I debug statistical decisions?

Enable debug logging:
```bash
PERFGUARD_LOG_LEVEL=debug perfguard check --base origin/main
```

Or use JSON output and inspect:
```bash
perfguard check --base origin/main --format json > result.json
jq '.items[] | select(.status == "fail")' result.json
```

### Can I customize the report format?

Yes:
```bash
perfguard report comparison.json --format md > report.md
perfguard report comparison.json --format json > report.json
```

Supported formats: `text`, `md`, `json`, `github`.

## Contributing

### How can I contribute?

See [AGENTS.md](../AGENTS.md) for contribution guidelines. Areas:
- Additional eval scenarios
- Documentation improvements
- Bug reports with reproduction
- Feature requests with use cases

### How do I report a bug?

Include:
1. PerfGuard version: `perfguard version`
2. Go version: `go version`
3. OS and architecture
4. Minimal reproduction case
5. Expected vs. actual behavior

Open issue at: https://github.com/kyyril/perfguard/issues
