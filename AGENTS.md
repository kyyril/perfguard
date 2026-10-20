# AGENTS.md

Rules for coding agents and contributors in the PerfGuard repository. Read in full before changing anything.

## Summary

PerfGuard is a Go CLI that detects performance regressions by running base and head benchmarks, testing the difference statistically, then diagnosing with pprof. Reference documents:

- `docs/PRD.md`: what is being built.
- `docs/ARCHITECTURE.md`: module design and boundaries.
- `docs/IMPLEMENTATION_PLAN.md`: work order and status.
- `docs/TESTING.md`: test strategy and quality gates.

If code contradicts a document, stop and ask. Do not decide on your own.

## Repository structure

See `README.md` section "Repository structure". Package dependency rules are in `docs/ARCHITECTURE.md`. In brief:

- `internal/model` must not import other internal packages.
- `internal/stats` and `internal/decision` are pure: no I/O, no `os/exec`, no network.
- Only `internal/runner` may run benchmark processes.
- Only `internal/gitx` may call `git`.
- Only `internal/ghci` may talk to the GitHub API.
- `internal/cli` handles only flags and exit codes. Logic lives in `internal/app`.

## Commands

Always use Makefile targets. Do not invent other commands.

| Purpose | Command |
|---|---|
| Build | `make build` |
| Format | `make fmt` |
| Lint | `make lint` |
| Vet | `make vet` |
| Unit test | `make test` |
| Unit test with race detector | `make test-race` |
| Integration test | `make test-integration` |
| End-to-end test | `make test-e2e` |
| Coverage | `make cover` |
| Vulnerability scan | `make vuln` |
| Evaluation suite (slow, do not run unless asked) | `make eval` |
| All local gates | `make check` |
| Tidy dependencies | `make tidy` |

## Go code rules

General:

- Follow Effective Go and Go Code Review Comments.
- Code, comments, names, logs, error messages, and commit messages use English.
- One responsibility per package. Package names are short, lowercase, no underscores.
- `context.Context` is always the first parameter for functions that do I/O or run long.
- No mutable global state. No `init()` with side effects.
- Interfaces are defined on the consumer side, small, and only when there is more than one implementation or when needed for testing.
- Constructors accept explicit dependencies. Use functional options when options are many.
- Time and randomness are injected (`Clock`, `rand.Source` with seed) to keep tests deterministic.
- No `any` or `interface{}` when a concrete type or generics can be used.
- No `panic` for normal flow. Panic only for bugs that cannot happen.

Errors:

- Wrap with `fmt.Errorf("context: %w", err)`.
- Error messages are lowercase, no trailing period, mention the operation and the object.
- Use typed or sentinel errors for those that need mapping to exit codes (`ErrConfig`, `ErrExecution`). Check with `errors.Is` or `errors.As`.
- Do not ignore errors. If intentionally ignored, write the reason in a comment.
- Do not log and then return the same error. Choose one.

External processes:

- Use `exec.CommandContext`. Do not go through a shell. Arguments are a slice.
- Set `Cmd.Env` explicitly. Set `Cmd.WaitDelay`. Clean up the process group when context is cancelled.
- Do not run benchmarks in parallel. Measurements must be sequential.

Logging:

- Use `log/slog`. Structured fields, not concatenated strings.
- Report output goes to stdout. Logs go to stderr.

Dependencies:

- New dependencies require a justification in the PR and an entry in the dependency table in `docs/ARCHITECTURE.md`.
- Prefer the standard library. Approved dependencies are listed in the architecture document.

## Testing rules

Summary. Details in `docs/TESTING.md`.

- Every behavior change is accompanied by a test. Bug fixes start with a failing test.
- Table tests (`t.Run`) for logic with many cases.
- Golden files for reports. Update with `go test ./... -update`, then review the diff manually.
- `-race` and `-shuffle=on` are active in CI. Tests must pass with both.
- `time.Sleep` for synchronization is forbidden. Use channels, `sync`, or a fake clock.
- Tests that depend on real-time measurements must be statistical or use deterministic fixtures. Do not assert absolute durations.
- Statistical tests use a fixed seed and auditable reference values.
- Minimum coverage: 80% total. `stats`, `decision`, `benchparse`, `config` at least 90%. Coverage is a lower bound, not a goal.

## Writing style (code, documents, commits, replies)

Applies to all text you produce.

- Get to the point. Short sentences, one idea per sentence. Clear and concise.
- Em dashes and en dashes are forbidden. Use periods, commas, colons, or parentheses.
- Emojis and emoticons are forbidden, including in commits, logs, comments, and documents.
- Filler openers and closers are forbidden: "Sure", "Here is", "Hope this helps", "Note that", "Certainly", "Great question".
- Marketing words are forbidden: powerful, seamless, robust, cutting-edge, game-changer, comprehensive, leverage, elegant, blazing fast.
- The "not only X, but also Y" pattern, forced three-item lists, and closing paragraphs that repeat the body are forbidden.
- Claims without numbers and sources are forbidden ("much faster", "more accurate").
- Code comments explain why, not what the line does.
- Do not refer to yourself as an AI or assistant in code, documents, or commits.
- Documentation uses English. Technical terms (benchmark, baseline, profile, commit) are not forced into translation.
- Commit messages: Conventional Commits (`feat(stats): add Mann-Whitney U test`). Subject at most 72 characters, imperative, no trailing period.

## Prohibitions

General:

- Do not modify files outside the scope of the task.
- Do not run `git push`, `git reset --hard`, `git clean`, or delete branches unless asked.
- Do not put secrets, tokens, or local paths in code, tests, or fixtures.
- Do not run `make eval` or long benchmarks unless asked. Results depend on the machine and take time.

Result integrity:

- Do not mark a task done in `docs/IMPLEMENTATION_PLAN.md` without evidence (passing test names, file paths, or PR links).
- Do not weaken, delete, or skip tests to make the build green.
- Do not change ground truth in `eval/scenarios/` to make eval results look better.
- Do not fabricate benchmark numbers, eval results, or dependency versions. If not yet measured, write "not yet measured".
- Do not write the claim "detects all regressions". Always state the limitations.
- Do not refer to pprof diagnosis results as a definitive cause. Use the term "contributor candidates".

Technical:

- Do not change the format of `result.json`, `comparison.json`, or `perfguard.yaml` without incrementing `schema_version` or `version` and adding compatibility tests.
- Do not change the meaning of exit codes.
- Do not add a mode that runs PR code with a write token or secrets.
- Do not add CGO dependencies.

## Agent workflow

1. Open `docs/IMPLEMENTATION_PLAN.md`. Pick one task with status `[ ]` whose all dependencies are `[x]`.
2. Change the task status to `[~]`.
3. Read the relevant sections in `docs/ARCHITECTURE.md` and `docs/PRD.md`.
4. Write tests, then implement. One task, one change.
5. Run `make check`. Fix until green. Run integration or e2e tests if the task touches runner, git, or CLI.
6. Change status to `[x]` and fill in the Evidence column.
7. If a specification is ambiguous or contradictory, stop. Write in the "Open questions" section of the plan and ask for a decision. Do not guess.

## Verification before declaring done

All must be true:

- [ ] `make check` passes.
- [ ] New tests exist and fail if the implementation is removed.
- [ ] Integration or e2e tests are run if the task touches runner, gitx, ghci, or cli.
- [ ] No TODOs without a task number in the plan.
- [ ] Affected documents are updated (README, ARCHITECTURE, TESTING, schema).
- [ ] Writing style matches the section above (no em dashes, emojis, filler).
- [ ] Status and evidence in the plan are filled in.

## Module-specific rules

Rules here supplement the general rules. A `AGENTS.md` file in a subdirectory, if present, takes precedence over this file for that directory.

`internal/stats`
- All functions are pure and deterministic. Bootstrap accepts a seed.
- Every algorithm has a test against reference values in `testdata/stats/`.
- Handle empty input, single element, identical values, zero, NaN, and Inf explicitly.

`internal/decision`
- Rule order must match `docs/ARCHITECTURE.md` section "Decision rules".
- Every status has a reason (`Reason`) that is a fixed code, not free text.

`internal/runner`
- Compile the test binary once per side per package. Execute in alternating order.
- Do not add benchmark flags that change measurement behavior without recording them in `RunParams`.
- Must clean up worktree and child processes on success, failure, or cancellation.

`internal/profile`
- Profiles are measured in a separate pass from the main measurement.
- Per-operation normalization must be tested with fixtures.

`internal/ghci`
- All GitHub calls go through an interface so they can be tested with `httptest`.
- PR comments use a hidden marker so they can be updated, not recreated.

`internal/eval`
- Scenarios contain `base/` and `change.patch`. Ground truth is in `scenario.yaml`.
- Eval results always include an environment fingerprint.
