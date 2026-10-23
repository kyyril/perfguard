# Example GitHub Actions workflow for PerfGuard

This workflow demonstrates how to integrate PerfGuard into your CI pipeline.

## Basic workflow

Save as `.github/workflows/perfguard.yml`:

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
    name: Check Performance
    runs-on: ubuntu-latest
    timeout-minutes: 45
    steps:
      - name: Checkout code
        uses: actions/checkout@v4
        with:
          fetch-depth: 0  # Required: ensures base ref is available

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true

      - name: Run PerfGuard
        uses: kyyril/perfguard@v1
        with:
          base: ${{ github.event.pull_request.base.sha }}
          config: perfguard.yaml
          token: ${{ secrets.GITHUB_TOKEN }}
          upload-artifacts: true
```

## Configuration options

| Input | Required | Default | Description |
|---|---|---|---|
| `base` | Yes | - | Base git ref (commit SHA or branch) |
| `config` | No | `perfguard.yaml` | Path to config file |
| `token` | No | - | GitHub token for PR comments |
| `upload-artifacts` | No | `true` | Upload results as artifacts |
| `fail-on` | No | `fail` | Exit on `fail` or `warn` |
| `strict` | No | `false` | Exit 4 on `inconclusive` |

## Advanced: Multiple benchmark packages

```yaml
- name: Run PerfGuard
  uses: kyyril/perfguard@v1
  with:
    base: ${{ github.event.pull_request.base.sha }}
    config: perfguard.yaml
    token: ${{ secrets.GITHUB_TOKEN }}
  env:
    # Customize benchmark selection
    PERFGUARD_PACKAGES: "./internal/...,./pkg/..."
```

## Advanced: Fail on warnings

```yaml
- name: Run PerfGuard (strict)
  uses: kyyril/perfguard@v1
  with:
    base: ${{ github.event.pull_request.base.sha }}
    fail-on: warn  # Exit 1 on warn or fail
```

## Advanced: Custom timeout

```yaml
jobs:
  performance:
    runs-on: ubuntu-latest
    timeout-minutes: 60  # Increase for many benchmarks
    steps:
      # ...
```

## Fork PR handling

PRs from forks automatically skip comment publishing and write step summary only. No special configuration needed.

## Troubleshooting

### "shallow clone detected"

Add `fetch-depth: 0` to checkout action:

```yaml
- uses: actions/checkout@v4
  with:
    fetch-depth: 0
```

### High noise / many inconclusive results

Shared runners have variable performance. Consider:

1. Use `stored` mode with dedicated baseline runner
2. Increase `benchmarks.count` to 20+ in config
3. Accept `inconclusive` as normal with `--strict false`

### Benchmarks take too long

Reduce `benchmarks.benchtime` or `benchmarks.count` in config:

```yaml
benchmarks:
  benchtime: "500ms"  # Down from 1s
  count: 8            # Down from 10
```

### No benchmarks found

Ensure your package has `Benchmark*` functions:

```go
func BenchmarkMyFunction(b *testing.B) {
    for i := 0; i < b.N; i++ {
        MyFunction()
    }
}
```

## Example repositories

See these repositories for real-world PerfGuard usage:

- [github.com/kyyril/perfguard](https://github.com/kyyril/perfguard) - PerfGuard testing itself
- Add your repository here by opening a PR!

## Manual local run

For testing before pushing:

```bash
# Install
go install github.com/kyyril/perfguard/cmd/perfguard@latest

# Run check
perfguard check --base origin/main
```

## More examples

- `.github/workflows/perfguard-self.yml` - PerfGuard's own dogfooding workflow
- `perfguard.yaml` - Example configuration
- `eval/scenarios/` - Intentional regression scenarios for testing
