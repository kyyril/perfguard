# Implementation Plan: PerfGuard

This document is the source of truth for work status. References: `docs/PRD.md`, `docs/ARCHITECTURE.md`, `docs/TESTING.md`.

## How to use this document

Status legend:

| Symbol | Meaning |
|---|---|
| `[ ]` | Not started |
| `[~]` | In progress (at most one per agent) |
| `[x]` | Done with evidence |
| `[!]` | Blocked. Reason written in the Evidence column |

Rules:

1. Work on tasks whose all dependencies are `[x]`.
2. Mark `[x]` only if the done criteria are met and the Evidence column is filled: passing test names, file paths, or PR links.
3. Do not mark done based on assumptions or because the code compiles.
4. If a specification is ambiguous, write it in "Open questions" and stop.
5. Tasks that turn out too large are split. Add sub-tasks with new IDs; do not change old IDs.
6. Every task meets the common gate: `make check` green, new tests exist, affected documents updated.

## Release map

| Release | Phase | Demonstrable outcome |
|---|---|---|
| v0.1 | P0 to P4 | `perfguard check` locally with verdict and exit code |
| v0.2 | P5, P6 | Automated check on GitHub PR with baseline |
| v0.3 | P7 | pprof diagnosis in report |
| v1.0 | P8, P9 | Published evaluation results and stable schema |

## Progress summary

| Phase | Done | Total |
|---|---|---|
| P0 Foundation | 5 | 5 |
| P1 Parsing and environment | 3 | 3 |
| P2 Runner | 5 | 5 |
| P3 Statistics and decision | 5 | 6 |
| P4 Check end-to-end | 0 | 3 |
| P5 Baseline | 0 | 4 |
| P6 GitHub integration | 0 | 6 |
| P7 Diagnosis | 0 | 7 |
| P8 Evaluation suite | 0 | 7 |
| P9 Hardening and release | 0 | 5 |

Update this table whenever a status changes.

## P0 Foundation

| ID | Status | Task | Depends | Output | Done if | Evidence |
|---|---|---|---|---|---|---|
| P0-T1 | `[x]` | Initialize module, directory layout, Makefile, `.golangci.yml`, basic CI | - | Repo that builds and lints in CI | `make check` green in CI on an empty branch | `make check` green, `.github/workflows/ci.yml`, commit 2ce7607 |
| P0-T2 | `[x]` | `model` package: domain types, JSON tags, `schema_version` | P0-T1 | `internal/model` | JSON round-trip and golden tests pass. No imports of other internal packages | `make check` green, 100% coverage, golden tests pass, commit dbdd22b |
| P0-T3 | `[x]` | `config` package: load YAML, defaults, validation, error messages with field paths | P0-T2 | `internal/config`, `schemas/config.v1.json` | Valid and invalid table tests pass. Fuzz test 30 seconds without panic. Example config passes schema validation | `make check` green, 97.6% coverage, 30s fuzzing passed, schema validated, commit e0acdec |
| P0-T4 | `[x]` | Error types, exit code mapping, `slog` setup | P0-T1 | `internal/cli` (error part), log helper | Test mapping `ErrConfig` to 2 and `ErrExecution` to 3 | `make check` green, 90.6% coverage, error mapping tests pass, commit 7717694 |
| P0-T5 | `[x]` | CLI skeleton: root, `version`, `init` | P0-T3, P0-T4 | `cmd/perfguard`, `internal/cli` | E2e test: `version` runs, `init` produces a config that passes validation | `make check` green, e2e tests pass, `version` and `init` verified, commit f3c47ca |

## P1 Parsing and environment

| ID | Status | Task | Depends | Output | Done if | Evidence |
|---|---|---|---|---|---|---|
| P1-T1 | `[x]` | `benchparse`: parse benchmark output, name normalization (`-N` suffix, sub-benchmarks), custom metrics | P0-T2 | `internal/benchparse` | Tests with real output fixtures pass. Fuzz test without panic. `p95-ms` metric is read | `make check` green, 97.3% coverage, fuzz passed, fixtures verified, commit 265827a |
| P1-T2 | `[x]` | `envinfo`: fingerprint and preflight (load, governor, cgroup) | P0-T2 | `internal/envinfo` | Tests with fake filesystem for `/proc` and `/sys`. Unreadable values produce empty fields, not errors | `make check` green, 100% coverage, fake /proc and /sys tests pass, commit 62f9640 |
| P1-T3 | `[x]` | `gitx`: resolve ref, worktree, changed files, shallow clone detection | P0-T2 | `internal/gitx` | Integration test on temporary repo: worktree is created and cleaned up, shallow clone detected with a clear message | `make check` and integration tests green, worktree and shallow clone verified, commit 7652835 |

## P2 Runner

| ID | Status | Task | Depends | Output | Done if | Evidence |
|---|---|---|---|---|---|---|
| P2-T1 | `[x]` | Build test binary per package (`go test -c`) for base and head | P1-T3 | `internal/runner` (build) | Integration test: binary produced for fixture module, packages without tests are skipped, build error maps to `ErrExecution` | `make check` and integration tests pass, benchmod fixture tested, commit c428e4c |
| P2-T2 | `[x]` | Binary execution: context, timeout, process group, capture output, env allowlist | P2-T1 | `internal/runner` (execute) | Test: timeout stops child processes, SIGINT cleans up, secrets from env are not forwarded | `make check` and integration tests pass, timeout, panic, env allowlist verified, commit 751898c |
| P2-T3 | `[x]` | Interleaved round orchestration and warmup | P2-T2, P1-T1 | `internal/runner` (rounds) | Test with fake runner: base and head order alternates each round, sample count equals exactly `count` | `make check` and integration tests pass, alternating rounds and sample count verified, commit 985f855 |
| P2-T4 | `[x]` | Preflight noise, optional CPU pinning | P1-T2, P2-T2 | Warning in report, Linux affinity | Test: high load produces a warning. Pinning is ignored with a message on non-Linux | `make check` and integration tests pass, preflight noise and pinning validation verified, commit 5249d25 |
| P2-T5 | `[x]` | `perfguard run` writes `result.json` | P2-T3, P0-T5 | `run` command | E2e test on fixture module: `result.json` is valid against schema, contains SHA, env, params, samples | `make check` and e2e tests pass, schema validation verified, commit aed4934 |

## P3 Statistics and decision

| ID | Status | Task | Depends | Output | Done if | Evidence |
|---|---|---|---|---|---|---|
| P3-T1 | `[x]` | `stats`: median, CV, Mann-Whitney U, bootstrap CI with seed | P0-T2 | `internal/stats` | Results match reference values in `testdata/stats/` within documented tolerance. Empty, identical, zero, NaN cases handled | `make check` passes, 95% test coverage, reference tests match testdata/stats/reference.json, commit 2a69021 |
| P3-T2 | `[x]` | `decision`: 11 rules, verdict, reason codes | P3-T1, P0-T3 | `internal/decision` | One table test per rule. Rule order matches ARCHITECTURE. Worst verdict selected correctly | `make check` passes, 96.7% test coverage, 11 rules and verdict selection tested, commit 9dac16c |
| P3-T3 | `[x]` | Adaptive samples for noisy metrics | P3-T2, P2-T3 | Loop in `app.Check` | Test with fake runner: samples increase up to the limit, then status follows `on_noisy` | `make check` passes, adaptive sampling loop and on_noisy behavior tested, commit 0704ead |
| P3-T4 | `[x]` | Synthetic A/A and shift simulation as test gate | P3-T2 | Seeded simulation test | False positive rate and power meet thresholds in TESTING. Distributions: normal, lognormal, bimodal, drift | `make check` passes, 7 simulation distributions verified, commit 9ce5ace |
| P3-T5 | `[x]` | `perfguard compare` and `comparison.json` | P3-T2, P0-T5 | `compare` command | E2e test with two fixture `result.json` files: `comparison.json` is valid and golden matches | `make check`, integration, e2e tests pass, schema validation and golden comparison verified, commit 5c03d7d |
| P3-T6 | `[~]` | `text` and `json` renderers | P3-T5 | `internal/report` | Golden tests for `pass`, `warn`, `fail`, `inconclusive`, `new`, `removed` | |

## P4 Check end-to-end (v0.1 release)

| ID | Status | Task | Depends | Output | Done if | Evidence |
|---|---|---|---|---|---|---|
| P4-T1 | `[ ]` | `app.Check` use case wires runner, parse, stats, decision, report | P2-T5, P3-T3, P3-T6 | `internal/app` | Test with all fake ports covers the full flow and cleanup on failure | |
| P4-T2 | `[ ]` | `check` command with `--base`, `--fail-on`, `--strict` flags and exit codes | P4-T1 | `check` command | E2e test proves exit codes 0, 1, 2, 3, 4 per AC-06 | |
| P4-T3 | `[ ]` | Quick start verification on a clean repo, v0.1 release with goreleaser | P4-T2 | Release binary, verified README | README steps run on the example repo and succeed. Release published at tag `v0.1.0` | |

## P5 Baseline (v0.2)

| ID | Status | Task | Depends | Output | Done if | Evidence |
|---|---|---|---|---|---|---|
| P5-T1 | `[ ]` | `BaselineStore` interface and `fs` store | P0-T2 | `internal/baseline` | Tests for save, load, list, prune. Atomic write (temp file then rename) | |
| P5-T2 | `[ ]` | `git` store on orphan branch | P5-T1, P1-T3 | `git` store | Integration test: branch created, push-free (local), data reloadable | |
| P5-T3 | `[ ]` | Environment compatibility check | P5-T1, P1-T2 | `Compatible` function | Table test: each different field yields `env_mismatch` and a list of differences | |
| P5-T4 | `[ ]` | `baseline` command and `check --mode stored` | P5-T2, P5-T3, P4-T2 | CLI | E2e test for AC-09 | |

## P6 GitHub integration (v0.2)

| ID | Status | Task | Depends | Output | Done if | Evidence |
|---|---|---|---|---|---|---|
| P6-T1 | `[ ]` | `md` and `github` renderers | P3-T6 | `internal/report` | Golden tests. Markdown valid and does not exceed GitHub comment length limit (truncated with notice) | |
| P6-T2 | `[ ]` | `ghci`: upsert PR comment with marker | P6-T1 | `internal/ghci` | Test with `httptest`: publishing twice results in one comment | |
| P6-T3 | `[ ]` | Step summary | P6-T1 | Write to `$GITHUB_STEP_SUMMARY` | Test with temp file | |
| P6-T4 | `[ ]` | `action.yml` composite and artifact upload | P6-T2, P6-T3, P4-T3 | `action.yml` | Workflow on example repo runs green on a test PR | |
| P6-T5 | `[ ]` | Fork PR fallback | P6-T2 | Fork detection from payload | Test: fork payload does not call write API and job does not fail (AC-07) | |
| P6-T6 | `[ ]` | Dogfooding workflow and example workflow in docs | P6-T4 | `.github/workflows/perfguard-self.yml` | Workflow runs on a PR in this repository | |

## P7 Diagnosis (v0.3)

| ID | Status | Task | Depends | Output | Done if | Evidence |
|---|---|---|---|---|---|---|
| P7-T1 | `[ ]` | Profile pass: profile flags, `-test.benchtime=Nx`, one benchmark per execution | P2-T2, P3-T2 | `internal/profile` (collect) | Integration test: valid `.pb.gz` profiles produced for each type | |
| P7-T2 | `[ ]` | Load, merge, normalize per operation | P7-T1 | `internal/profile` (load) | Test with synthetic profiles: per-op values match manual calculation | |
| P7-T3 | `[ ]` | Flat and cum diff, `min_share` filter, candidate ranking | P7-T2 | `internal/profile` (diff) | Table test with synthetic profiles and known results | |
| P7-T4 | `[ ]` | Annotation of files changed by the PR | P7-T3, P1-T3 | `touched` field in diagnosis | Test: functions in changed files are marked, others are not | |
| P7-T5 | `[ ]` | Integration into report: warnings, reproduction command, short profile marker | P7-T3, P6-T1 | Diagnosis section in report | Golden test includes warning and `pprof -diff_base` command (AC-08) | |
| P7-T6 | `[ ]` | Validate `N + 1` normalization assumption and minimum sample count with fixture | P7-T1 | Result note in ARCHITECTURE | Experiment with deterministic benchmark proves or disproves assumption. Document updated accordingly | |
| P7-T7 | `[ ]` | `profile diff` command | P7-T3, P0-T5 | CLI | E2e test with two fixture profiles | |

## P8 Evaluation suite (v1.0)

| ID | Status | Task | Depends | Output | Done if | Evidence |
|---|---|---|---|---|---|---|
| P8-T1 | `[ ]` | Eval runner and `scenario.yaml` format | P4-T2 | `internal/eval`, `perfguard eval run` | Example scenario runs and writes raw results per trial | |
| P8-T2 | `[ ]` | Negative scenarios: `no-change-aa`, `neutral-refactor`, `below-threshold`, `improvement` | P8-T1 | `eval/scenarios/*` | Each scenario has `base/`, `change.patch`, ground truth, and purpose documentation | |
| P8-T3 | `[ ]` | Positive scenarios: `algo-slowdown`, `alloc-bloat`, `lock-contention` | P8-T1 | `eval/scenarios/*` | Regression manually verified with regular benchmarks before being used as ground truth | |
| P8-T4 | `[ ]` | `noisy-host` scenario (synthetic CPU load) | P8-T1 | `eval/scenarios/noisy-host` | Produces `inconclusive` or `warn` per ground truth. Default `max_cv` value reviewed (Q-04) | |
| P8-T5 | `[ ]` | Metrics: detection rate, false positive rate, hit@K, Wilson interval, `eval report` | P8-T1 | `eval/RESULTS.md` generator | Unit tests for metric calculations. Report includes environment fingerprint | |
| P8-T6 | `[ ]` | Nightly eval workflow | P8-T5 | `.github/workflows/nightly-eval.yml` | Workflow produces a report artifact | |
| P8-T7 | `[ ]` | Review default thresholds and `correction` based on results (Q-01) | P8-T2, P8-T3, P8-T5 | Decision written in ARCHITECTURE | Before and after adjustment results recorded. Scenarios not changed for the sake of results | |

## P9 Hardening and v1.0 release

| ID | Status | Task | Depends | Output | Done if | Evidence |
|---|---|---|---|---|---|---|
| P9-T1 | `[ ]` | All edge cases in TESTING.md have tests | P4-T2, P7-T7 | Additional tests | Edge case matrix in TESTING.md marked as covered with test names | |
| P9-T2 | `[ ]` | Freeze v1 schema and compatibility tests | P8-T7 | `schemas/*.v1.json`, compatibility tests | Fixture `result.json` from v0.x remains readable | |
| P9-T3 | `[ ]` | Security review: tokens, forks, argument injection, env allowlist | P6-T5 | Review notes, fixes | Security checklist in ARCHITECTURE section 10 satisfied with evidence | |
| P9-T4 | `[ ]` | Final documentation and example repository | P9-T1 | README, examples | Three external repositories try without direct assistance (PRD section 10) | |
| P9-T5 | `[ ]` | v1.0 release | P9-T2, P9-T3, P9-T4 | Tag `v1.0.0` | Up-to-date `eval/RESULTS.md` attached to the release | |

## Open questions

Write questions that block a task here. Format: `Task ID, question, date, decision`.

| Task | Question | Decision |
|---|---|---|
| (empty) | | |

Product-level questions are in `docs/PRD.md` section 14 (Q-01 to Q-04).

## Evidence log

Record important verification results here (brief, with date and commit).

| Date | Task | Evidence |
|---|---|---|
| 2026-10-20 | P0-T1 | Module initialized with Go 1.24, directory layout created, Makefile targets tested, golangci-lint and govulncheck passing in make check, smoke unit and e2e tests pass (commit 2ce7607) |
| 2026-10-20 | P0-T2 | model package implemented with schema_version, domain types, 100% test coverage, golden JSON shape tests, and strict boundary tests (commit dbdd22b) |
| 2026-10-20 | P0-T3 | config package implemented with YAML loader, default values, field-path validation errors, 97.6% coverage, 30s fuzzing, and schemas/config.v1.json (commit e0acdec) |
| 2026-10-20 | P0-T4 | cli package error types, exit code mapping, and slog setup implemented with 90.6% test coverage (commit 7717694) |
| 2026-10-20 | P0-T5 | CLI skeleton with root, version, and init commands, e2e tests passing (commit f3c47ca) |
| 2026-10-20 | P1-T1 | benchparse package with fuzzing and sub-benchmark support (commit 265827a) |
| 2026-10-20 | P1-T2 | envinfo package with hardware fingerprinting and preflight checks (commit 62f9640) |
| 2026-10-20 | P1-T3 | gitx package with worktrees and shallow clone detection (commit 7652835) |
| 2026-10-20 | P2-T1 | runner build with fixture module integration tests (commit c428e4c) |
| 2026-10-20 | P2-T2 | runner execution with timeout, process cleanup, and env allowlist (commit 751898c) |
| 2026-10-20 | P2-T3 | runner round orchestration with alternating interleaved execution (commit 985f855) |
| 2026-10-20 | P2-T4 | runner preflight noise checks and CPU pinning validation (commit 5249d25) |
| 2026-10-20 | P2-T5 | perfguard run command writes result.json, schema validation and e2e tests pass (commit aed4934) |
| 2026-10-20 | P3-T1 | stats package with median, CV, Mann-Whitney U, and bootstrap CI; reference values verified against scipy (commit 2a69021) |
| 2026-10-20 | P3-T2 | decision package implementing 11 rules, rule precedence, and verdict selection with 96.7% coverage (commit 9dac16c) |
