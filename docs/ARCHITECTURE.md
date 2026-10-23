# Architecture: PerfGuard

This document describes components, data flow, module responsibilities, dependencies, and technical decision rationale. The product reference is `docs/PRD.md`.

## 1. Design principles

1. Measure correctly first, then draw conclusions. Measurement quality determines everything else.
2. Pure domain core. Statistics and decisions do not perform I/O.
3. All side effects (processes, git, network, files) live in adapters behind interfaces.
4. The public contract is the CLI, `perfguard.yaml`, and the result JSON schema. No public Go API in v1.
5. Be honest about uncertainty. Status `inconclusive` is a valid result.
6. No server and no database in v1. All state is JSON files.

## 2. System context

```
                        +----------------------------+
 Developer / CI  ---->  |        perfguard CLI       |
                        +-------------+--------------+
                                      |
        +--------------+--------------+---------------+----------------+
        |              |              |               |                |
   git (worktree,  go toolchain   filesystem     GitHub REST API   go tool pprof
   diff, rev)      (go test -c)   (.perfguard/)  (comment, status) (optional, manual)
                        |
                 test binary (base and head code)
```

PerfGuard has no background service. One CLI execution completes one check.

## 3. Data flow: `perfguard check`

```
 1  load config -> validate
 2  resolve base ref -> temporary git worktree
 3  go test -c per package (base and head) -> test binary
 4  envinfo: fingerprint, preflight noise (load, governor)
 5  warmup (results discarded)
 6  rounds: for i in 1..count
        even order: base then head. odd order: head then base
        run each binary per package, -test.count=1
 7  benchparse -> Run(base), Run(head)
 8  stats + decision -> Comparison
 9  if any metric is noisy: add rounds (max max_extra_runs), repeat 7-8
10  if regression and profiling active: profile pass for regressed benchmarks
11  profile diff -> contributor candidates
12  report (text, md, json) and publish (PR comment, step summary)
13  write artifacts to .perfguard/, clean up worktree, return exit code
```

Artifacts:

```
.perfguard/
└── runs/<run_id>/
    ├── base.result.json
    ├── head.result.json
    ├── comparison.json
    ├── report.md
    └── profiles/<benchmark>/{base,head}.{cpu,allocs,mutex,block}.pb.gz
```

## 4. Modules and responsibilities

| Package | Responsibility | May depend on | Must not |
|---|---|---|---|
| `cmd/perfguard` | `main`, calls `cli.Execute` | `cli` | Business logic |
| `internal/cli` | Flags, subcommands, error-to-exit-code mapping | `app`, `config` | Run processes or direct I/O |
| `internal/app` | Use cases `Check`, `Run`, `Compare`, `ProfileDiff`, `EvalRun`. Defines ports | all domain and adapters via interfaces | Import `cli` |
| `internal/config` | Parse YAML, defaults, validation, JSON Schema | `model` | I/O other than reading the config file |
| `internal/model` | Domain types: `Run`, `Sample`, `Comparison`, `Verdict`, `Environment` | stdlib | Import other internal packages |
| `internal/envinfo` | Environment fingerprint and preflight | `model` | Change machine settings |
| `internal/gitx` | Resolve ref, worktree, changed file names, shallow clone detection | `model` | Used other than through the `VCS` port |
| `internal/runner` | Build test binary, execute, collect raw output, process cleanup | `model`, `envinfo` | Statistics, decisions |
| `internal/benchparse` | Parse benchmark output into `Sample` | `model`, `x/perf/benchfmt` | Process I/O |
| `internal/stats` | Median, CV, Mann-Whitney U, bootstrap CI | stdlib, `x/perf/benchmath` | I/O, `os/exec`, real time |
| `internal/decision` | Decision rules, verdict, reason | `model`, `stats`, `config` | I/O |
| `internal/profile` | Collect profile arguments, load, merge, normalize, diff | `model`, `google/pprof/profile` | Run processes (via `runner`) |
| `internal/baseline` | `Store` and `fs`, `git` implementations, environment compatibility check | `model`, `gitx` | Statistics |
| `internal/report` | text, md, json, github renderers | `model` | Network I/O |
| `internal/ghci` | PR comment (upsert), step summary, fork detection | `model`, `report`, `go-github` | Decision logic |
| `internal/eval` | Load scenarios, run trials, compute metrics, `RESULTS.md` | `app` via port | Change ground truth |

### Dependency rules

```
cli -> app -> { config, runner, benchparse, stats, decision, profile,
                baseline, report, ghci, gitx, envinfo, eval } -> model
```

- Arrows go downward only. No cycles.
- `stats` and `decision` do not import adapters.
- Ports are defined in `app` (consumer side) and implemented in adapters.
- Rules enforced by linter (`depguard` or `go-arch-lint`) in CI.

### Main ports

```go
// internal/app/ports.go (sketch)
type BenchRunner interface {
    Build(ctx context.Context, spec BuildSpec) (TestBinary, error)
    Execute(ctx context.Context, bin TestBinary, spec ExecSpec) (RawOutput, error)
}

type VCS interface {
    Resolve(ctx context.Context, ref string) (Commit, error)
    AddWorktree(ctx context.Context, c Commit) (Worktree, error)
    ChangedFiles(ctx context.Context, base, head Commit) ([]string, error)
}

type BaselineStore interface {
    Save(ctx context.Context, r model.Run) error
    Load(ctx context.Context, key BaselineKey) (model.Run, error)
    List(ctx context.Context, filter BaselineFilter) ([]BaselineInfo, error)
}

type Publisher interface {
    Publish(ctx context.Context, c model.Comparison, rendered Rendered) error
}

type ProfileCollector interface {
    Collect(ctx context.Context, spec ProfileSpec) ([]model.ProfileRef, error)
}
```

## 5. Data model

```go
// internal/model (sketch)
type Run struct {
    SchemaVersion int
    ID            string
    Side          Side // base | head
    CreatedAt     time.Time
    Git           GitInfo // SHA, Ref, Dirty
    Env           Environment
    Params        RunParams // count, benchtime, cpu, include, exclude, mode
    Benchmarks    []BenchmarkResult
    Profiles      []ProfileRef
}

type BenchmarkResult struct {
    Package string
    Name    string // normalized without -N suffix
    Procs   int
    Samples []Sample
}

type Sample struct {
    Iterations int64
    Values     map[string]float64 // unit -> value: "sec/op", "B/op", "allocs/op", "p95-ms"
}

type Comparison struct {
    SchemaVersion int
    Base, Head    RunSummary
    Items         []MetricComparison
    Verdict       Verdict
    Diagnosis     []Diagnosis
    Notes         []string
}

type MetricComparison struct {
    Benchmark string
    Metric    string
    BaseMedian, HeadMedian float64
    DeltaPct  float64 // positive means worse after direction adjustment
    CI        [2]float64
    PValue    float64
    CVBase, CVHead float64
    NBase, NHead   int
    Status    Status // unchanged | improved | warn | fail | inconclusive | new | removed
    Reason    string // fixed code: below_threshold, noisy, insufficient_samples, zero_base, env_mismatch
}
```

Schema rules:

- `result.json` and `comparison.json` carry `schema_version`. Starts at 1.
- Adding optional fields does not increment the version. Changing or removing fields increments the major schema version.
- Raw samples are always stored so analysis can be repeated without re-running benchmarks.

## 6. Measurement design

### Build once, run many times

`go test -c -o <bin> ./pkg` is run once per package per side. Compilation cost does not enter the measurement and both sides use a fixed binary. The binary is run with the working directory at the package folder (so `testdata` is readable) and flags prefixed with `-test.` (`-test.bench`, `-test.count=1`, `-test.benchtime`, `-test.benchmem`, `-test.run=^$`).

### Interleaved execution

Machine drift (temperature, frequency, noisy neighbors on a shared runner) biases the side that runs second. Solution: in each round, the order of base and head is alternated. The impact is distributed evenly across both sides.

Rejected alternative: run all base samples then all head samples. Simple but susceptible to drift.

### One process per sample

Each sample comes from one execution of `-test.count=1`. This keeps samples independent of each other and avoids warm-up effects across iterations within a single process. The cost is process startup time, which is small compared to `benchtime`.

### Environment control

- `GOMAXPROCS` from `benchmarks.cpu` if set. Otherwise follows the machine.
- `execution.pin_cpus` (optional, Linux) runs the binary with CPU affinity via `sched_setaffinity`.
- Preflight: read load average, CPU governor, cgroup CPU limits. If outside reasonable bounds, log a warning in the report. PerfGuard does not change machine settings.
- The benchmark process environment is built explicitly. Secrets are not forwarded.

### Time estimation

`total ≈ 2 x count x (number of benchmarks x benchtime) + build overhead`. `perfguard check` prints an estimate at the start and honors `benchmarks.timeout`. Use `include` to limit benchmarks to the critical path.

### Separate profile pass

Profiling adds overhead that changes results. Therefore the main measurement never uses profile flags. After the verdict is known, regressed benchmarks are re-run one by one (`-test.bench=^Name$`) with profile flags.

Toolchain note: `go test -cpuprofile` rejects multiple packages at once. Because PerfGuard runs binaries per package, this limitation does not apply.

## 7. Statistics and decision design

### Per-pair statistics (benchmark, metric)

- Center: median (resistant to outliers). Cross-benchmark summary uses geomean of deltas.
- Dispersion: CV per side.
- Significance: two-sided Mann-Whitney U test, `alpha` default 0.05. Does not assume normal distribution. Same approach used by `benchstat`.
- Confidence interval of the delta: bootstrap with a fixed seed for deterministic results.
- Implementation uses `golang.org/x/perf/benchmath` where it meets requirements. Additional functions are in `internal/stats` and tested against reference values.

### Decision rules

Evaluated in order for each (benchmark, metric). Stop at the first matching rule.

| # | Condition | Status | Reason |
|---|---|---|---|
| 1 | Benchmark exists only in head | `new` | `new_benchmark` |
| 2 | Benchmark exists only in base | `removed` | `removed_benchmark` |
| 3 | `N` base or head is less than `thresholds.min_samples` | `inconclusive` | `insufficient_samples` |
| 4 | Environment does not match (stored mode) | `inconclusive` | `env_mismatch` |
| 5 | CV on either side exceeds `noise.max_cv` after extra samples are exhausted | per `noise.on_noisy` (`inconclusive` or `warn`) | `noisy` |
| 6 | Base median is 0, head median is greater than 0, and significant | per `thresholds.zero_base` (`warn` or `fail`) | `zero_base` |
| 7 | Not significant (`p >= alpha`) | `unchanged` | `not_significant` |
| 8 | Significant, worse, `delta >= fail` | `fail` | `exceeds_fail` |
| 9 | Significant, worse, `delta >= warn` | `warn` | `exceeds_warn` |
| 10 | Significant, better, `delta >= warn` (absolute value) | `improved` | `improvement` |
| 11 | Otherwise | `unchanged` | `below_threshold` |

`delta` is direction-adjusted: positive always means worse. Built-in metrics (`sec/op`, `B/op`, `allocs/op`) are lower is better. Custom metrics must declare `direction`.

Final verdict: take the worst in order `fail` > `warn` > `inconclusive` > `pass`. Statuses `new`, `removed`, `improved`, `unchanged` do not raise the verdict.

### Multiple comparisons

Many benchmarks and metrics mean many tests. The combination of significance test and effect size gate (rules 8 to 11) is the primary safeguard. Holm correction is available as an option (`thresholds.correction: none | holm`).

**Default: `none`** (2026-10-23, P8-T7)

Rationale:
- Effect size thresholds (5%, 10%, 15%, 20%, 25%) provide primary false positive protection
- Mann-Whitney U test is already conservative
- Typical Go projects have 5-20 benchmarks, not hundreds
- False negatives (missing real regressions) are more costly than false positives
- Users can opt into `holm` for projects with many benchmarks (50+)

This decision will be validated against false positive rates from the evaluation suite. If A/A scenario false positive rate exceeds 10%, the default will be reconsidered. See `docs/THRESHOLD_DECISION.md` for full analysis.

### Adaptive samples

If rule 5 triggers, the runner adds rounds of `execution.extra_round_step` up to `execution.max_extra_runs`. After that, rule 5 is final. Extra samples are recorded in `Params`.

## 8. Diagnosis with pprof

Goal: show which functions changed the most, not prove causation.

Steps:

1. Select benchmarks with status `warn` or `fail`.
2. Run a profile pass for base and head with `-test.benchtime=Nx`. The value of `N` is taken from the iteration count of the main measurement and is the same for both sides. With a fixed iteration count, the total iterations executed are deterministic (`N + 1`, because the framework runs one initial iteration of `b.N = 1` followed by `b.N = N`), so per-operation normalization is accurate. This assumption was validated experimentally in task P7-T6 with deterministic counter fixtures.
3. Profile types: `cpu` (`-test.cpuprofile`), `allocs` (`-test.memprofile`, use sample types `alloc_space` and `alloc_objects`), `mutex` (`-test.mutexprofile` with fraction), `block` (`-test.blockprofile`).
4. Collect several profiles per side (`profiling.runs`), merge with `profile.Merge`.
5. Normalize values per operation.
6. Compute flat and cum delta per function: `delta = head_per_op - base_per_op`.
7. Sort by absolute delta. Drop functions where `|delta|` is below `profiling.min_share` of the total base.
8. Annotate: function is in a file changed by the PR (from `git diff --name-only`). This is a hint, not a conclusion.
9. Include a reproduction command `go tool pprof -diff_base`.

Warnings always shown:

- Contribution is not proof of causation.
- Inlining and compiler optimizations can move cost between functions.
- CPU profiles are sampled. If the sample count is below `profiling.min_samples`, the report marks the profile as too short.

## 9. Persistence

No database in v1. Rationale: the main workflow (`same-runner`) does not need history, and JSON files are easy to inspect, diff, and store in git.

`BaselineStore` has two implementations:

| Store | Location | Used for |
|---|---|---|
| `fs` | `baseline.dir` (default `.perfguard/baselines`) | Local, self-hosted runner |
| `git` | Orphan branch `baseline.branch` (default `perfguard-data`) | CI, history follows the repository |

Key layout: `<goos>-<goarch>/<env-hash>/<commit-sha>.json`.

Environment compatibility in `stored` mode checks: GOOS, GOARCH, CPU model, GOMAXPROCS, Go minor version, and benchmark configuration (parameter hash). A mismatch produces `inconclusive` with a list of differences.

`stored` mode is weaker than `same-runner` because machines differ over time. Documentation recommends `same-runner`.

## 10. GitHub integration and security

Components:

- `action.yml` (composite): install binary, run `perfguard check`, upload `.perfguard/` as artifact.
- `ghci.Publisher`: PR comment with marker `<!-- perfguard:comment -->`. If a comment with the marker exists, it is updated. Otherwise it is created.
- Step summary: write markdown to `$GITHUB_STEP_SUMMARY`.
- Status check comes from the job exit code.

Security:

- Benchmarks execute PR code. Use the `pull_request` event. Do not use `pull_request_target` with head checkout.
- Token: `contents: read`, `pull-requests: write`. No other permissions.
- Benchmark processes do not receive `GITHUB_TOKEN` or other secrets. Environment is built from an allowlist.
- Fork PR: detected from the event payload. Skip comment, write step summary only.
- No feature to run shell commands from config in v1.

## 11. Configuration

File `perfguard.yaml`, `version: 1`. Priority order: CLI flags, environment variables, file, defaults.

```yaml
version: 1
benchmarks:
  packages: ["./..."]
  include: "."            # regex for -test.bench
  exclude: []             # regexes of names to skip
  count: 10               # samples per side, at least thresholds.min_samples
  benchtime: "1s"
  cpu: 0                  # GOMAXPROCS, 0 means machine default
  timeout: "30m"
execution:
  mode: same-runner       # same-runner | stored
  interleave: true
  warmup_runs: 1
  max_extra_runs: 10
  extra_round_step: 2
  pin_cpus: ""            # example "2-3", Linux only
thresholds:
  alpha: 0.05
  min_samples: 6
  correction: none        # none | holm
  zero_base: warn         # warn | fail
  metrics:
    sec/op:    { warn: 5,  fail: 15 }
    B/op:      { warn: 10, fail: 25 }
    allocs/op: { warn: 5,  fail: 20 }
  custom:
    p95-ms:    { warn: 5, fail: 15, direction: lower_is_better }
  overrides:
    - match: "^BenchmarkHotPath"
      metric: sec/op
      warn: 3
      fail: 8
noise:
  max_cv: 10              # percent
  on_noisy: inconclusive  # inconclusive | warn
profiling:
  mode: on-regression     # off | on-regression | always
  types: [cpu, allocs]    # cpu | allocs | mutex | block
  runs: 3
  min_share: 1            # percent of total base
  min_samples: 200
  top: 10
baseline:
  store: git              # fs | git
  branch: perfguard-data
  dir: .perfguard/baselines
report:
  formats: [text, json]
github:
  comment: true
  step_summary: true
```

Threshold values are percentages. Validation rejects: `fail < warn`, `count < min_samples`, invalid regex, missing `direction` on custom metric, and unknown profile type. Error messages include the field path (e.g. `thresholds.metrics.sec/op.fail`).

## 12. Errors, exit codes, logging

- Errors are wrapped with context and mapped in `cli` to exit codes: `ErrConfig` to 2, `ErrExecution` to 3, verdict `fail` to 1, `inconclusive` with `--strict` to 4.
- Logging via `slog` to stderr. Format `text` or `json` (`--log-format`). Report to stdout.
- Every `Run` has a `run_id` that appears in logs and artifacts.

## 13. Concurrency model

- Benchmark measurements are sequential. No parallelism while measuring.
- Parallelism only for work that does not disturb measurement: parsing results, loading profiles, rendering reports. Limited by `errgroup` and runs after the measurement phase is complete.
- All operations accept `context.Context`. SIGINT and SIGTERM cancel the context, the runner stops the process group, `gitx` removes the worktree.

## 14. PerfGuard's own observability

- Structured logs with duration of each phase (build, measure, stats, profile, report).
- Internal benchmarks for `benchparse`, `stats`, and `profile diff`. CI runs PerfGuard on those benchmarks (dogfooding).
- No outbound telemetry. No data sent to third parties.

## 15. External dependencies

| Dependency | Function | Rationale |
|---|---|---|
| `github.com/spf13/cobra` | CLI | De facto standard, subcommands and completion |
| `gopkg.in/yaml.v3` | Config parsing | Stable, supports errors with line numbers |
| `golang.org/x/perf/benchfmt` | Parse benchmark format | Official parser from the Go team |
| `golang.org/x/perf/benchmath` | Benchmark statistics | Used by `benchstat`, avoids reimplementing statistical tests |
| `github.com/google/pprof/profile` | Read, merge, scale profiles | Official pprof library |
| `github.com/google/go-github` | GitHub API | Official community client, easy to test with `httptest` |
| `golang.org/x/sync/errgroup` | Bounded concurrency | Standard for goroutine groups with error handling |
| `github.com/google/go-cmp` | Comparison in tests | Clear diffs |
| `github.com/santhosh-tekuri/jsonschema` | JSON Schema validation in tests | Ensures output conforms to schema |

Exact versions are locked in `go.mod`. New dependencies require a line in this table.

Development tools: `golangci-lint`, `govulncheck`, `goreleaser`, `goimports`.

## 16. Architecture decisions (ADR brief)

| ADR | Decision | Rationale | Rejected alternatives |
|---|---|---|---|
| 001 | Wrap `go test` binary, not a custom benchmark harness | Uses existing benchmarks, no changes to user code | Custom benchmark library (forces users to rewrite) |
| 002 | `same-runner` mode with interleaved execution as default | Reduces drift bias and machine differences | Stored baseline as default (machines differ over time) |
| 003 | Mann-Whitney U and median, not t-test and mean | Benchmark distributions are often non-normal and heavy-tailed | t-test (assumes normality) |
| 004 | Effect size gate in addition to significance | Small differences can be statistically significant but unimportant | p-value only |
| 005 | Separate profile pass from measurement | Profiling overhead changes results | Profiling always active |
| 006 | No database in v1 | History requirements are small, JSON is sufficient, simple deployment | SQLite or Postgres |
| 007 | Versioned JSON schema and raw samples stored | Re-analysis and compatibility | Store summaries only |
| 008 | Thin GitHub Action over the CLI | Logic testable locally, not locked to one CI | Logic inside workflow YAML |
| 009 | Term "contributor candidates" for diagnosis results | Correlation is not proof of causation | Stating cause |
| 010 | `inconclusive` status as a valid result | Avoids false decisions in noisy environments | Always forcing pass or fail |
| 011 | No shell execution from config | Reduces attack surface | Arbitrary hooks |

Each new decision is added to this table with its rationale.

## 17. Design limitations

- Only `testing.B` benchmarks. p95 metrics require support from the benchmark itself.
- Accuracy is limited by machine noise. Shared runners do not guarantee stability.
- One machine per check. No distributed execution.
- Linux is the primary target. CPU pinning and governor reading are Linux only.
- `stored` mode is susceptible to machine differences over time.
- Diagnosis gives candidates, not causes.

## 18. Extensibility

- Machine result format is versioned so other tools can read it.
- The `BenchRunner` port allows runners for other languages in the future without touching `stats` and `decision`.
- New renderers (e.g. JUnit or SARIF) are added in `internal/report` without touching the domain.
- A new store (e.g. GitHub artifact) only needs to implement `BaselineStore`.
