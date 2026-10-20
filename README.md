# PerfGuard

Performance regression detection for Go projects. PerfGuard runs benchmarks before and after a code change, compares them statistically, then shows evidence: how large the change is, how confident the result is, and which functions contribute.

> Status: pre-alpha. Design is complete, implementation is progressing per [docs/IMPLEMENTATION_PLAN.md](docs/IMPLEMENTATION_PLAN.md). Commands and configuration below are the target interface. Only features marked `[x]` in the plan are complete.

## Problem solved

A pull request passes all unit tests, but raises latency by 35% and memory allocations by 60%. Nobody notices until production.

PerfGuard closes that gap:

1. Runs benchmarks on the base code and the changed code under controlled conditions.
2. Determines whether the difference is real or just noise.
3. Gives a `pass`, `warn`, or `fail` status based on configurable rules.
4. Collects pprof profiles and shows which functions contribute most to the change.

PerfGuard does not claim to find all performance regressions. Its scope is Go benchmarks (`testing.B`) run in a controlled environment. See the [Limitations](#limitations) section.

## Features

| Area | Contents |
|---|---|
| Measurement | Interleaved A/B benchmark execution, one binary per side, full metadata (commit SHA, Go version, environment, parameters) |
| Statistics | Median, CV, Mann-Whitney U test, confidence interval, noise gate, automatic extra samples |
| Decision | `warn` and `fail` thresholds per metric, per-benchmark overrides, custom metrics (e.g. `p95-ms`) |
| Baseline | `same-runner` mode (recommended) and `stored` mode with environment compatibility check |
| CI | GitHub Action, PR comment, step summary, clear exit codes |
| Diagnosis | CPU, allocation, mutex, and block profile diff with pprof. Results are called contributor candidates, not definitive causes |
| Evaluation | Suite of intentional regression scenarios, measured with detection rate and false positive rate |

## How it works in brief

```
git ref base ──► worktree ──► build test binary ─┐
                                                 ├─► interleaved run ──► parse ──► stats ──► decision ──► report
git ref head ──► worktree ──► build test binary ─┘                                              │
                                                                                                └─► profile pass (only on regression) ──► pprof diff
```

Details in [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Quick start

Prerequisites: Go 1.24 or newer (exact version in `go.mod`), git, Linux (fully supported), macOS (best effort).

```bash
# Install (target)
go install github.com/kyyril/perfguard/cmd/perfguard@latest

# In the root of the Go repository to test
perfguard init                      # creates perfguard.yaml
perfguard check --base origin/main  # compare HEAD against origin/main
```

Replace `kyyril` with your fork owner if running from a fork.

For a benchmark to be measurable, the target package must have `Benchmark*` functions that run with `go test -bench`.

### Example output

Numbers and function names below are illustrative only, to show the format. They are not actual benchmark results.

```
PerfGuard: FAIL (2 fail, 0 warn, 11 unchanged)
base 3f2a9c1 (origin/main)   head 8be41d7   go1.25 linux/amd64   count=10

Benchmark               Metric      Base     Head     Delta    p-value  Status
BenchmarkHandleRequest  p95-ms      80.0     108.0    +35.0%   0.0002   FAIL
BenchmarkHandleRequest  allocs/op   412      659      +60.0%   0.0002   FAIL
BenchmarkParse          sec/op      1.21us   1.22us   +0.8%    0.4100   unchanged

Contributor candidates (cpu, BenchmarkHandleRequest):
  Func                                        Flat delta   Cum delta   Changed by PR
  example.com/app/codec.(*Encoder).grow       +9.8ms       +14.1ms     yes
  runtime.mallocgc                            +6.2ms       +8.4ms      no

Note: contribution is not proof of causation. Manual reproduction:
  go tool pprof -diff_base .perfguard/.../base.cpu.pb.gz .perfguard/.../head.cpu.pb.gz
```

The `p95-ms` metric is only present when the benchmark reports it itself via `b.ReportMetric(v, "p95-ms")`. PerfGuard does not compute p95 from average `ns/op`.

## Configuration

Minimal `perfguard.yaml`:

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

All keys, default values, and validation rules are in [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md#11-konfigurasi). JSON Schema: `schemas/config.v1.json`.

Environment variables:

| Variable | Purpose |
|---|---|
| `PERFGUARD_CONFIG` | Config path. Default `perfguard.yaml` |
| `PERFGUARD_LOG_LEVEL` | `debug`, `info`, `warn`, `error`. Default `info` |
| `PERFGUARD_OUTPUT_DIR` | Output directory. Default `.perfguard` |
| `GITHUB_TOKEN` | Used for PR comments. Optional |

## GitHub Actions integration

```yaml
name: perfguard
on:
  pull_request:
    branches: [main]

permissions:
  contents: read
  pull-requests: write

jobs:
  perf:
    runs-on: ubuntu-latest
    timeout-minutes: 45
    steps:
      - uses: actions/checkout@v5
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v6
        with:
          go-version-file: go.mod
      - uses: kyyril/perfguard@v0
        with:
          base: ${{ github.event.pull_request.base.sha }}
          config: perfguard.yaml
```

Adjust the action version to the latest. Important notes:

- `fetch-depth: 0` is required so the base ref is available.
- PRs from forks do not have a write token. PerfGuard automatically falls back to step summary without a PR comment.
- Do not use `pull_request_target` with head code checkout. Benchmarks execute code from the PR.
- Shared runners have high noise. Use `same-runner` mode and watch for `inconclusive` status.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | `pass`, or `warn` when `--fail-on fail` (default) |
| 1 | `fail`, or `warn` when `--fail-on warn` |
| 2 | Usage or configuration error |
| 3 | Execution failure (build failed, benchmark panicked, timeout) |
| 4 | `inconclusive`, only when `--strict` |

## Commands

| Command | Purpose |
|---|---|
| `perfguard init` | Create `perfguard.yaml` |
| `perfguard run` | Run benchmarks, write `result.json` |
| `perfguard compare BASE.json HEAD.json` | Compare two results, write `comparison.json` |
| `perfguard check --base REF` | Full flow: build, measure, compare, profile, report, exit code |
| `perfguard baseline save\|list\|show\|prune` | Manage baselines for `stored` mode |
| `perfguard profile diff BASE.pb.gz HEAD.pb.gz` | Diff two pprof profiles |
| `perfguard report COMPARISON.json --format md\|text\|json\|github` | Render a report |
| `perfguard eval run\|report` | Run and report the evaluation suite |
| `perfguard version` | Build version |

## Repository structure

```
.
├── cmd/perfguard/          entrypoint
├── internal/
│   ├── cli/                flag parsing, error-to-exit-code mapping
│   ├── app/                use cases: Check, Run, Compare (orchestration)
│   ├── config/             load and validate perfguard.yaml
│   ├── model/              domain types, no internal dependencies
│   ├── runner/             build and execute test binaries
│   ├── benchparse/         benchmark output parser
│   ├── stats/              pure statistics
│   ├── decision/           decision rules and verdict
│   ├── profile/            collect, merge, diff pprof
│   ├── baseline/           baseline storage
│   ├── report/             text, md, json renderers
│   ├── ghci/               GitHub integration
│   ├── gitx/               git operations
│   ├── envinfo/            environment fingerprint
│   └── eval/               evaluation suite runner
├── eval/scenarios/         intentional regression scenarios
├── schemas/                JSON Schema for config and result
├── test/e2e/               end-to-end tests
├── testdata/               shared fixtures
├── docs/
├── action.yml
├── AGENTS.md
└── Makefile
```

## Development

```bash
make build        # bin/perfguard
make fmt          # gofmt and goimports
make lint         # golangci-lint
make vet          # go vet
make test         # unit tests
make test-race    # unit tests with -race
make check        # fmt-check, vet, lint, test-race, vuln
```

Contribution rules for humans and AI are in [AGENTS.md](AGENTS.md).

## Running tests

```bash
go test -race -shuffle=on -count=1 ./...                 # unit
go test -tags=integration -race -count=1 ./...           # integration
go test -tags=e2e -count=1 ./test/e2e/...                # end-to-end
make eval                                                # evaluation suite (slow)
```

Full strategy, edge cases, and quality gates are in [docs/TESTING.md](docs/TESTING.md).

## Limitations

- Only Go benchmarks based on `testing.B`. Not load testing and not production monitoring.
- Microbenchmarks do not always represent production behavior.
- Shared CI runners have high noise. Results may be `inconclusive`.
- pprof diagnosis gives contributor candidates. Inlining, sampling, and indirect effects can mislead.
- p95 and similar metrics are only available when the benchmark reports them.
- Detection capability is measured on scenarios in `eval/scenarios/`, and applies only to those scenarios.
- Windows is not yet supported.

## Documentation

| File | Question answered |
|---|---|
| [docs/PRD.md](docs/PRD.md) | What is being built, for whom, and how to judge whether results are correct |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | How components work and why they were chosen |
| [docs/IMPLEMENTATION_PLAN.md](docs/IMPLEMENTATION_PLAN.md) | Implementation order and work status |
| [docs/TESTING.md](docs/TESTING.md) | How to prove the implementation is correct |
| [AGENTS.md](AGENTS.md) | Rules for coding agents |

## License

Apache-2.0.
