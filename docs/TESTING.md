# Testing Strategy: PerfGuard

This document answers one question: how to prove the implementation is correct, not merely that it compiles.

A performance measurement tool has two specific risks. First, statistical logic can be wrong without causing an error. Second, tests that depend on real time can be flaky. The strategy below addresses both.

## 1. Principles

1. Tests prove behavior, not implementation.
2. Statistical logic is tested against auditable reference values, not against the output of the code itself.
3. Tests must not depend on machine speed. Use deterministic fixtures or seeded simulations.
4. Detection capability is measured with the evaluation suite, not assumed.
5. Bug fixes start with a failing test.
6. Flaky tests are treated as bugs. Fix or remove the cause; do not retry.

## 2. Test layers

| Layer | Location | Build tag | Command | Runs on |
|---|---|---|---|---|
| Unit | `internal/*/*_test.go` | none | `make test` or `go test -race -shuffle=on -count=1 ./...` | Every PR |
| Integration | `internal/*/*_integration_test.go` | `integration` | `make test-integration` | Every PR |
| End-to-end | `test/e2e/` | `e2e` | `make test-e2e` | Every PR |
| Fuzz | `Fuzz*` in relevant packages | none | `go test -fuzz=FuzzParse -fuzztime=30s ./internal/benchparse` | Nightly and local |
| Statistical simulation | `internal/stats`, `internal/decision` | none | Part of unit tests, fixed seed | Every PR |
| Evaluation suite | `eval/scenarios/` | none | `make eval` | Nightly and before release |
| Dogfooding | `.github/workflows/perfguard-self.yml` | none | PerfGuard on the repo's own benchmarks | Every PR (non-blocking initially) |

Shared fixtures are in `testdata/`. Go module fixtures (with their own `go.mod`) are in `testdata/fixtures/`. The `testdata` directory is ignored by `go build ./...`, so fixtures are safe to store there.

## 3. Unit tests

General rules:

- Table tests with `t.Run` and case names that describe the situation.
- `go-cmp` for struct comparison, not raw `reflect.DeepEqual`.
- Golden files in `testdata/*.golden` for reports. Update with `go test ./... -update`, then read the diff before committing.
- Time and randomness are injected (`Clock`, seed).
- No `time.Sleep` for synchronization.

Coverage per package:

| Package | What is tested | Notes |
|---|---|---|
| `model` | JSON round-trip, `schema_version` compatibility, defaults | Golden for JSON shape |
| `config` | Defaults, validation (`fail < warn`, bad regex, missing `direction`, `count < min_samples`), flag/env/file priority | Fuzz on parser. Error messages include field path |
| `benchparse` | Sub-benchmark names with `/`, `-N` suffix, custom metrics, non-benchmark lines, empty output, truncated output | Fixtures from real `go test` output. Fuzz |
| `stats` | Median, CV, Mann-Whitney U, bootstrap CI | See section 3.1 |
| `decision` | One table test per rule (11 rules), rule order, worst verdict, metric direction, `zero_base` | Every `Reason` is a fixed code |
| `profile` | Merge, per-operation normalization, diff, `min_share` filter, short profile marker | Synthetic profiles built in tests with manually calculated results |
| `report` | All statuses and verdicts in text, md, json, github | Golden. GitHub comment length limit |
| `baseline` | Save, load, list, prune, atomic write, environment compatibility | Temporary filesystem |
| `envinfo` | Parsing `/proc`, `/sys`, cgroup | Fake filesystem. Unreadable fields are empty without error |
| `cli` | Error-to-exit-code mapping, flag validation | Without running benchmarks |
| `app` | `Check` flow with all fake ports, cleanup on error, context cancellation | Fake ports are hand-written, not generated, for readability |

### 3.1 Statistical logic validation

Three layers:

1. Reference values. `stats` results are compared against values from an independent implementation (`scipy.stats.mannwhitneyu` and `benchstat`) on fixed datasets. Datasets, values, and the generation script (`scripts/gen_stats_reference.py`) are stored in `testdata/stats/`. CI does not run Python. Numeric tolerances are written in the test with rationale.
2. Properties. Examples: swapping base and head flips the sign of the delta. Doubling all values does not change the percent delta. Identical data produces `p = 1` and a delta of 0.
3. Seeded simulation. Generate synthetic data, run `decision`, then check error rate and detection power.

Simulations:

| Scenario | Distribution | Trials | Initial assert |
|---|---|---|---|
| Stable A/A | Normal, CV 2% | 2000 | `fail` at most 1%, `warn` at most 5% |
| Heavy-tailed A/A | Lognormal | 2000 | Same |
| Bimodal A/A | Two modes separated by 5% | 2000 | Same |
| A/A with drift | Small linear trend across run | 2000 | Same |
| Shift 2x `warn` threshold | Normal, CV 3%, n=10 | 2000 | `warn` or `fail` in 90% or more of trials |
| Below-threshold shift | 3% delta with `warn` at 5% | 2000 | `fail` at most 1% |
| High noise | CV 20% | 2000 | Majority `inconclusive`, not `fail` |

Numbers in the table are initial thresholds. If not met, fix the algorithm. Do not loosen asserts without a written reason in the PR.

## 4. Integration tests

Test adapters against real systems at small scale.

| Area | Scenario |
|---|---|
| `gitx` | Temporary repo with two commits: resolve ref, worktree created and removed, changed files correct, shallow clone detected with a message to set `fetch-depth: 0` |
| `runner` | Fixture module in `testdata/fixtures/benchmod`: build binary, execute, parse. Packages without tests are skipped. Build failure produces `ErrExecution`. Timeout stops child processes. Panicking benchmark is handled |
| `runner` interleaved | Runner wraps the fixture binary and records execution order. Order alternates each round |
| `profile` | Fixture binary produces valid `.pb.gz` profiles for `cpu`, `allocs`, `mutex`, `block` |
| `baseline` `git` store | Orphan branch created on a temporary repo, data reloadable from a different commit |
| `ghci` | `httptest` server mimicking GitHub API: upsert comment (publish twice, one comment), fork without write calls, rate limit and 5xx handled with a clear message |

The fixture module contains benchmarks with deterministic cost (e.g. a loop with a fixed number of operations and allocations), so tests check result structure, not absolute durations.

## 5. End-to-end tests

Build the binary then run it as a separate process on a temporary git repo containing the fixture module and two commits.

| Scenario | What is proved |
|---|---|
| `version`, `init` | Binary runs, generated config passes validation |
| `run` | `result.json` is valid against `schemas/result.v1.json` |
| `compare` on two fixture results | `comparison.json` matches golden |
| `check` with no regression | Exit code 0 |
| `check` with a large intentional regression | Exit code 1, report contains the correct benchmark |
| `check --fail-on warn` with a `warn` | Exit code 1 |
| Invalid config | Exit code 2 and message mentions the field path |
| Build failure on head | Exit code 3 |
| `check --strict` with noisy result | Exit code 4 |
| SIGINT during measurement | Worktree and child processes are clean, no temporary files remain |
| `baseline save` then `check --mode stored` | Works. Different environment produces `inconclusive` |

E2e tests use large regressions and deterministic fixtures for stable results. Subtle statistical behavior is tested in simulations and the evaluation suite.

## 6. Evaluation suite

Answers: does PerfGuard actually detect what it targets, and how often does it get it wrong?

Scenario structure:

```
eval/scenarios/<id>/
├── base/               complete Go module
├── change.patch        change applied to base
└── scenario.yaml       ground truth and parameters
```

Example `scenario.yaml`:

```yaml
id: alloc-bloat
description: Buffer reallocated on every call
trials: 30
expect:
  verdict: fail
  metrics:
    - { benchmark: BenchmarkEncode, metric: allocs/op, status: fail }
  diagnosis:
    profile: allocs
    top_k: 5
    function: example.com/evalmod/encoder.(*Buffer).grow
```

Scenarios:

| ID | Purpose | Expected |
|---|---|---|
| `no-change-aa` | Measure false positives on identical code | `pass` |
| `neutral-refactor` | Structural change with no performance effect | `pass` |
| `below-threshold` | Regression of about 3% below threshold | `pass` (`unchanged`) |
| `improvement` | Genuine improvement | `improved`, verdict `pass` |
| `algo-slowdown` | Slower algorithm (e.g. O(n) to O(n log n) or O(n^2)) | `fail` on `sec/op` |
| `alloc-bloat` | Excessive allocations | `fail` on `allocs/op` and `B/op` |
| `lock-contention` | Global mutex replacing sharding in a parallel benchmark | `fail`, candidate in `mutex` profile |
| `noisy-host` | Synthetic CPU load during measurement | `inconclusive` or `warn`, not `fail` |

Metrics reported in `eval/RESULTS.md`:

- Detection rate per regression class, with 95% Wilson interval.
- False positive rate (`fail` and `warn` separately) on negative scenarios.
- Diagnosis hit rate: cause function in `top_k`.
- Median time per check.
- Environment fingerprint where the evaluation was run.

Rules:

- Ground truth is set before running eval. Regressions are manually verified with regular benchmarks.
- Do not change scenarios or ground truth to make results look better. Scenario changes require a rationale in the PR and a note in the plan evidence log.
- Results from shared runners are labeled and not compared directly with the reference machine.
- Failures are published alongside successes.

## 7. Edge case matrix

Every row must have a test. The Layer column shows where it is tested. The Status column is filled with the test name when complete (P9-T1).

| Category | Edge case | Expected behavior | Layer | Status |
|---|---|---|---|---|
| Input | No benchmarks found | Clear error, exit 2 or 3 per cause | e2e | |
| Input | Benchmark added in head | `new`, not `fail` | unit, e2e | |
| Input | Benchmark removed in head | `removed`, not `fail` | unit, e2e | |
| Input | Benchmark renamed | Reported as `removed` and `new`, not a regression | unit | |
| Input | Sub-benchmark with `/` and `-N` suffix | Name normalized consistently | unit | |
| Input | Truncated benchmark output or output with foreign lines | Unknown lines ignored, valid samples still read | unit, fuzz | |
| Input | Non-UTF8 in output | No panic | fuzz | |
| Statistics | Fewer samples than `min_samples` | `inconclusive` with `insufficient_samples` | unit | |
| Statistics | Identical values in all samples | `p = 1`, `unchanged` | unit | |
| Statistics | Base median 0 and head more than 0 | `zero_base` rule, percent not computed (no division by zero) | unit | |
| Statistics | Base 0 and head 0 | `unchanged` | unit | |
| Statistics | NaN or Inf in samples | Rejected with error or dropped with a note | unit | |
| Statistics | Custom metric with direction higher is better | Delta sign adjusted | unit | |
| Statistics | Two metrics on one benchmark with different statuses | Verdict takes the worst | unit | |
| Noise | CV exceeds `max_cv` | Samples increase to the limit, then `on_noisy` | unit, integration | |
| Noise | High initial load | Warning in report | unit | |
| Execution | Test failure or benchmark panic on one side | `ErrExecution`, exit 3, other side output not lost | integration | |
| Execution | Build failure on head | Exit 3 with compiler output | e2e | |
| Execution | Timeout | Child processes stop, exit 3 | integration | |
| Execution | SIGINT | Worktree and child processes cleaned up | e2e | |
| Execution | Two `check` runs concurrently on the same output directory | Rejected with a lock file and a clear message | integration | |
| Git | Dirty working tree | `dirty` status recorded in `result.json` | integration | |
| Git | Shallow clone, base unavailable | Error with suggestion to set `fetch-depth: 0` | integration | |
| Git | Base equals head | Clear message, no strange error | e2e | |
| Git | Ref does not exist | Exit 2 with the ref name | e2e | |
| Baseline | Different environment | `inconclusive` with `env_mismatch` | unit, e2e | |
| Baseline | Corrupt baseline file or unknown `schema_version` | Clear error, no panic | unit | |
| Profile | Profile with too few samples | Marked short in report | unit | |
| Profile | Function only on one side | Delta computed as 0 on the side where it is absent | unit | |
| Profile | Profile type not produced by binary (e.g. no mutex) | Skipped with a note | integration | |
| GitHub | PR from fork | No write calls, step summary filled | unit, integration | |
| GitHub | Rate limit or 5xx | Job does not fail because of the comment, result still printed | integration | |
| GitHub | Report exceeds comment length limit | Truncated with a note and artifact link | unit | |
| Config | Unknown field | Rejected with the field name | unit | |
| Config | Config version newer than supported | Error with upgrade suggestion | unit | |

## 8. CI quality gates

All gates must be green before merge.

| Gate | Tool | Criteria |
|---|---|---|
| Format | `gofmt`, `goimports` | No diff |
| Vet | `go vet ./...` | No findings |
| Lint | `golangci-lint` | No new findings. Includes inter-package dependency boundary rules |
| Build | `go build ./...` for linux amd64 and arm64, darwin arm64 | Success |
| Unit | `go test -race -shuffle=on -count=1 ./...` | Pass |
| Integration | `go test -tags=integration -race -count=1 ./...` | Pass |
| E2E | `go test -tags=e2e -count=1 ./test/e2e/...` | Pass |
| Coverage | `scripts/check-coverage.sh` | 80% or more total. `stats`, `decision`, `benchparse`, `config` 90% or more |
| Vulnerability | `govulncheck ./cmd/...` | No reachable findings |
| Schema | Validate fixtures against `schemas/*.json` | Pass |
| Docs | Link and style check (`scripts/check-docs.sh`: no em dashes and no emojis) | Pass |
| Go mod | `go mod tidy` produces no diff | Clean |
| Dogfooding | PerfGuard on the repo's own benchmarks | Result reported. Becomes blocking once stable (noted in plan) |

Scheduled gates:

| Gate | Schedule | Criteria |
|---|---|---|
| Fuzz | Nightly, 5 minutes per target | No new crashes. Crash corpus goes into `testdata/fuzz` |
| Evaluation suite | Nightly and before release | Results below PRD targets are investigated before release. Report attached |

Coverage is a lower bound. High coverage with weak asserts does not count as evidence. Reviewers check that a test will fail if behavior is changed.

## 9. How to write proving tests

Checklist before declaring a test sufficient:

- [ ] Test fails if the implementation is removed or inverted.
- [ ] Boundary cases are present: empty, single element, zero, identical values, very large.
- [ ] Failure cases are present, not only the happy path.
- [ ] Asserts check values, not just `err == nil`.
- [ ] Does not depend on test order, real time, or network.
- [ ] Test name describes the situation and the expected result.

## 10. What is not covered

Known limitations. Stated so they are not mistaken for guarantees.

- Measurement stability on shared CI runners. Tests only prove that noise is handled per design, not that the runner is stable.
- Behavior on Windows.
- Behavior on kernels, hypervisors, or CPUs not tested (e.g. thermal throttling).
- Benchmarks that depend on network, disk, or external services.
- Benchmarks with global state or inter-iteration dependencies that make samples non-independent.
- Long-running effects (slow memory leaks, regressions that appear only under hours of load).
- Regressions invisible in microbenchmarks but visible in production.
- Diagnosis accuracy on code patterns outside eval scenarios.
- Repositories with thousands of benchmarks (NFR-02 target only tested on 500).
- No mutation testing in v1. Considered after v1.0 for `stats` and `decision`.

New limitations found are added here.
