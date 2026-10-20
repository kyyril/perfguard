# Product Requirements Document: PerfGuard

Document version: 0.1 (draft)
Status: approved for phased implementation

## 1. Summary

PerfGuard is an open source tool in the form of a CLI and GitHub Action that detects performance regressions in Go projects. It compares base and head benchmarks, statistically decides whether a regression exists, then provides profiling data to help engineers find the cause.

Its core value is in two things: trustworthy decisions (noise is not treated as regression) and actionable evidence (profiles, not just graphs).

## 2. Problem

| Problem | Impact |
|---|---|
| Unit tests do not measure performance | PRs that slow down code still pass CI |
| Benchmarks are run manually and inconsistently | Results cannot be compared across engineers or over time |
| Noise makes simple comparison (single run) misleading | False alarms cause teams to ignore the tool, or real regressions slip through |
| After a regression is detected, engineers must run pprof themselves | Diagnosis takes long, often not done |
| No record of environment and measurement parameters | Results cannot be reproduced |

Example: a PR raises p95 latency from 80 ms to 108 ms (+35%) and memory allocations by 60%. All unit tests pass. These numbers are illustrative, not measured results.

## 3. Target users

| Persona | Need | How they use PerfGuard |
|---|---|---|
| PR author (backend engineer) | Know before merging that their change slows down code, and where | Read PR comment, open attached profile |
| Reviewer and library maintainer | Objective basis for rejecting or accepting performance-sensitive changes | Use `fail` or `warn` status as a gate, set thresholds per benchmark |
| Platform or developer productivity engineer | Performance gate installable across many repositories with minimal configuration | Install the Action, set a standard `perfguard.yaml` |
| Performance engineer | Reproducible measurements and early diagnosis | Run `perfguard check` locally, use `profile diff` |

Not a target: teams that only need production monitoring or load testing.

## 4. Use cases

| ID | Use case | Actor | Outcome |
|---|---|---|---|
| UC-1 | Check a PR against the base branch in CI | Author, reviewer | Status `pass`, `warn`, `fail`, or `inconclusive`, PR comment, profile artifact |
| UC-2 | Run the same check locally before pushing | Author | Text report in terminal, exit code |
| UC-3 | Store a baseline from the main branch and compare other commits against it | Platform engineer | Stored baseline, comparison with environment compatibility check |
| UC-4 | Investigate an already-detected regression | Performance engineer | List of contributor candidates from CPU, allocation, mutex, block diff |
| UC-5 | Set thresholds for specific benchmarks | Maintainer | Per-benchmark override in config |
| UC-6 | Measure how well PerfGuard detects | PerfGuard maintainer, prospective user | Evaluation report with detection rate and false positive rate |

## 5. Goals and non-goals

### Goals

1. Measurement results are reproducible and documented (commit, Go version, environment, parameters).
2. Normal variation is not reported as regression (low and measurable false positive rate).
3. Intentional regressions in test scenarios are detected (high and measurable detection rate).
4. Every regression is accompanied by data that helps with diagnosis.
5. Installable in another Go repository in under 10 minutes.

### Non-goals

- Detecting all performance regressions.
- Automatically proving the cause of a regression.
- Load testing, stress testing, or production monitoring.
- Supporting languages other than Go in v1.
- Web dashboard, database server, or SaaS in v1.
- Replacing `benchstat` or `pprof`. PerfGuard uses the same concepts and libraries, then adds orchestration, decisions, and CI integration.
- Running benchmark code from untrusted PRs with elevated access.
- Windows support in v1.

## 6. Release scope

| Release | Name | Contents | Plan phases |
|---|---|---|---|
| v0.1 | Measure and compare | `init`, `run`, `compare`, `check` locally, `same-runner` mode, statistics, decisions, text and json reports | P0 to P4 |
| v0.2 | CI | `stored` mode, baseline store, markdown reports, GitHub Action, PR comment, step summary | P5, P6 |
| v0.3 | Diagnose | Profile pass, pprof diff, contributor candidates, changed file annotation | P7 |
| v1.0 | Proven | Published evaluation suite results, frozen schema, security hardening, final documentation | P8, P9 |

## 7. Functional requirements

Priority: M (must), S (should), C (could).

| ID | Requirement | Priority | Release |
|---|---|---|---|
| FR-01 | `perfguard init` creates a valid `perfguard.yaml` with sensible defaults | M | v0.1 |
| FR-02 | `perfguard run` runs benchmarks per package with `count`, `benchtime`, `cpu`, `include`, `exclude`, `timeout` from config | M | v0.1 |
| FR-03 | Record environment fingerprint: GOOS, GOARCH, CPU model, GOMAXPROCS, Go version, kernel, container CPU limits, CPU governor (if readable), initial load average | M | v0.1 |
| FR-04 | Store results as versioned `result.json` (`schema_version`) containing commit SHA, ref, dirty status, environment, parameters, and all raw samples | M | v0.1 |
| FR-05 | `same-runner` mode: build base and head test binaries, run interleaved on the same machine | M | v0.1 |
| FR-06 | `stored` mode: compare head against a stored baseline, reject or mark `inconclusive` if environment does not match | M | v0.2 |
| FR-07 | Parse Go benchmark output including custom metrics from `b.ReportMetric` and sub-benchmarks | M | v0.1 |
| FR-08 | Compute median, CV, Mann-Whitney U test, and confidence interval of the delta for each (benchmark, metric) pair | M | v0.1 |
| FR-09 | Noise gate: if CV exceeds `noise.max_cv`, add samples up to `max_extra_runs`, then mark per `noise.on_noisy` | M | v0.1 |
| FR-10 | Decision rules with `warn` and `fail` thresholds per metric, per-benchmark regex overrides, and custom metrics with direction (lower or higher is better) | M | v0.1 |
| FR-11 | Verdict `pass`, `warn`, `fail`, `inconclusive` and exit codes 0 to 4 per spec | M | v0.1 |
| FR-12 | Reports in `text`, `json`, `md`, and `github` formats | M | v0.1 (text, json), v0.2 (md, github) |
| FR-13 | GitHub composite Action, PR comment updated (not recreated), step summary, upload `.perfguard/` artifact | M | v0.2 |
| FR-14 | Safe fallback for fork PRs (no write token) | M | v0.2 |
| FR-15 | Separate profile pass for regressed benchmarks: CPU, allocations (`allocs`), mutex, block | M | v0.3 |
| FR-16 | Profile diff: list of contributor candidates with flat delta, cum delta, and contribution to total delta | M | v0.3 |
| FR-17 | Mark candidates that are in files changed by the PR (`git diff`) as a hint | S | v0.3 |
| FR-18 | Report includes a `go tool pprof -diff_base` command for manual reproduction and a warning that results are not proof of causation | M | v0.3 |
| FR-19 | `perfguard baseline save`, `list`, `show`, `prune` | S | v0.2 |
| FR-20 | New benchmarks (head only) and removed benchmarks (base only) are reported as `new` and `removed`, without affecting verdict unless configured | M | v0.1 |
| FR-21 | Evaluation suite: scenarios with `base/` and `change.patch` and ground truth, run via `perfguard eval run`, report via `perfguard eval report` | M | v1.0 |
| FR-22 | All machine output (`result.json`, `comparison.json`) has a JSON Schema in `schemas/` | S | v1.0 |
| FR-23 | Multiple comparison correction (Holm) as option `thresholds.correction`. Default determined from evaluation results | C | v1.0 |

## 8. Non-functional requirements

| ID | Category | Requirement |
|---|---|---|
| NFR-01 | Reproducibility | Two `check` runs on the same commit and machine produce the same verdict on stable eval scenarios (target measured in eval, not guaranteed) |
| NFR-02 | Overhead | Time outside benchmark execution (parse, stats, report) under 2 seconds for 500 benchmarks with 10 samples |
| NFR-03 | Memory | Under 200 MB for the NFR-02 case |
| NFR-04 | Portability | Linux amd64 and arm64 fully supported. macOS best effort. Static binary without CGO |
| NFR-05 | Security | No shell execution. Minimum GitHub token (`pull-requests: write`). No secrets enter benchmark processes |
| NFR-06 | Reliability | Temporary worktrees and child processes always cleaned up, including on SIGINT, timeout, and failure |
| NFR-07 | Observability | Structured logging (`slog`), configurable level, logs and reports do not mix |
| NFR-08 | Compatibility | `schema_version` and config `version` follow SemVer. Breaking changes increment the version and provide migration or a clear message |
| NFR-09 | Usability | Error messages state the cause and action. Time from install to first result under 10 minutes on an example repository |
| NFR-10 | Dogfooding | PerfGuard repository uses PerfGuard on its own benchmarks in CI |

## 9. Acceptance criteria

Format: Given, When, Then. Each criterion has a test in `docs/TESTING.md`.

**AC-01 Controlled measurement (FR-02, FR-03, FR-04, FR-05)**
- Given a repository with benchmarks and two commits.
- When `perfguard check --base <ref>` is run.
- Then `result.json` for both sides contains SHA, Go version, environment fingerprint, parameters, and exactly `count` samples per benchmark, and base and head executions are interleaved.

**AC-02 Noise does not become regression (FR-08, FR-09, FR-10)**
- Given identical base and head code (A/A).
- When the check is repeated on the `no-change-aa` scenario.
- Then verdict `fail` appears in no more than 1% of trials and verdict `warn` in no more than 5% of trials on the reference machine. This is a measured target, not a guarantee.

**AC-03 Regression detected (FR-08, FR-10, FR-11)**
- Given scenarios `algo-slowdown`, `alloc-bloat`, and `lock-contention`.
- When the check is run with default thresholds.
- Then the targeted metrics have status `fail` in at least 90% of trials (target on reference machine, reported with confidence interval).

**AC-04 Change below threshold (FR-10)**
- Given a 3% change and `warn` at 5%.
- When the check is run.
- Then status `unchanged` and verdict `pass`.

**AC-05 High noise (FR-09)**
- Given CV exceeds `noise.max_cv` after extra samples are exhausted.
- When the decision is computed.
- Then status `inconclusive` with reason `noisy`, and exit code 4 only when `--strict`.

**AC-06 Exit codes (FR-11)**
- Given verdict `fail`.
- When the CLI exits.
- Then exit code 1. For `warn` with `--fail-on fail` exit code 0. For invalid config exit code 2. For build failure exit code 3.

**AC-07 GitHub (FR-13, FR-14)**
- Given a PR with a write token.
- When the Action runs twice on the same PR.
- Then there is one PerfGuard comment that is updated.
- Given a PR from a fork.
- Then no attempt is made to write a comment, step summary is still filled, and the job does not fail due to permissions.

**AC-08 Diagnosis (FR-15, FR-16, FR-18)**
- Given a regression scenario with a known cause function.
- When the profile pass is run.
- Then the cause function is in the top 5 of the candidate list in at least 80% of trials (target), and the report contains the warning and reproduction command.

**AC-09 Baseline mismatch (FR-06)**
- Given a baseline from a different CPU or Go version.
- When `check --mode stored` is run.
- Then status `inconclusive` with reason `env_mismatch` and a list of differences.

**AC-10 Changed benchmarks (FR-20)**
- Given a benchmark added or removed in head.
- When the report is generated.
- Then the benchmark is listed as `new` or `removed` and does not produce `fail`.

## 10. Success indicators

Detection quality (measured by the evaluation suite, published in `eval/RESULTS.md` with environment fingerprint):

| Metric | Target |
|---|---|
| False positive rate for verdict `fail` on A/A and neutral changes | 1% or less |
| False positive rate for verdict `warn` on A/A and neutral changes | 5% or less |
| Detection rate on intentional regressions with effect of 2x the `fail` threshold or more | 90% or more |
| Diagnosis hit rate (cause function in top 5) | 80% or more |

Targets are initial hypotheses. If not met, improve the algorithm or change the target with a written reason. Do not adjust scenarios to pass.

Usability and adoption:

| Metric | Target |
|---|---|
| Time from install to first result on an example repository | Under 10 minutes |
| External repositories that successfully install PerfGuard without direct assistance | At least 3 before v1.0 |
| False positive reports from users that are addressed | Every report becomes a new eval scenario or a fix |

## 11. Claims made and not made

Made:
- On scenarios in `eval/scenarios/`, on the reference machine, the tool reaches the published detection and false positive rates.
- Every measurement result can be re-examined from `result.json`.

Not made:
- Detecting all regressions.
- Results on shared CI runners are as stable as on the reference machine.
- pprof diagnosis proves causation.
- Microbenchmarks reflect production performance.

## 12. Risks and mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| CI runner noise too high | Many `inconclusive` results, tool ignored | Interleaving, adaptive samples, clear `inconclusive` status, self-hosted runner guide |
| Multiple comparisons raise false positive rate | False alarms on repositories with many benchmarks | Effect size gate in addition to significance, Holm option, measured in eval |
| Profiling overhead disturbs measurement | Biased results | Separate profile pass from measurement |
| Benchmarks run very long | Slow CI | `include`, `exclude`, `timeout`, duration estimate before start |
| Running untrusted PR code | Secret leakage | `pull_request` event only, no secrets, documented prohibition of `pull_request_target` |
| Eval scenarios too easy | Overstated capability claims | Include hard scenarios (small effect, noisy host), publish failures |
| Per-operation profile normalization assumption wrong | Misleading profile diff | Dedicated validation task (P7-T6) with fixture |

## 13. Assumptions

- Users already have Go benchmarks that are logically stable.
- The repository can be cloned with enough history to reach the base ref.
- The execution machine is not running heavy loads during measurement.

## 14. Open questions

| ID | Question | Decision at |
|---|---|---|
| Q-01 | Default `thresholds.correction`: `none` or `holm` | P8-T7, based on eval results |
| Q-02 | Need artifact mode for baseline store other than `git` and `fs` | After v0.2 based on feedback |
| Q-03 | Whether p95 needs a helper library `perfguard/bench` so benchmarks can report it easily | After v0.3 |
| Q-04 | Appropriate default `max_cv` value for GitHub hosted runners | P8-T4 |

## 15. Glossary

| Term | Meaning |
|---|---|
| Base | The comparison code (e.g. `origin/main`) |
| Head | The code being checked (e.g. the PR branch) |
| Sample | One result of one benchmark run for one metric |
| Round | One cycle of base and head execution |
| CV | Coefficient of variation, standard deviation divided by mean |
| Interleaved | Alternating base and head execution to reduce machine drift effects |
| A/A | Comparing the same code against itself to measure false positives |
| Verdict | Final result: `pass`, `warn`, `fail`, `inconclusive` |
| Contributor candidate | Function with the largest profile delta, not proof of causation |
