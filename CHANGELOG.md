# Changelog

All notable changes to PerfGuard will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-10-23

### Added

#### Core Functionality
- Statistical benchmark comparison using Mann-Whitney U test
- Bootstrap confidence intervals for effect size estimation
- 11 decision rules with configurable thresholds
- Automatic noise detection with coefficient of variation (CV)
- Adaptive sampling when CV exceeds threshold
- Support for custom metrics with configurable direction
- Per-benchmark threshold overrides
- Multiple comparison correction (Holm-Bonferroni, optional)

#### Commands
- `perfguard init` - Generate configuration file
- `perfguard run` - Execute benchmarks and capture results
- `perfguard compare` - Statistical comparison of two runs
- `perfguard check` - Full workflow: build, measure, compare, diagnose
- `perfguard baseline save|list|show|prune` - Baseline management
- `perfguard profile diff` - Compare pprof profiles
- `perfguard report` - Render comparison in multiple formats
- `perfguard eval run|report` - Evaluation suite execution and reporting
- `perfguard version` - Display version information

#### Execution Modes
- `same-runner` mode - Compare base and head on same infrastructure (default)
- `stored` mode - Compare against saved baseline with environment compatibility check
- Interleaved round execution for fair comparison
- Warmup runs to reduce cold-start effects
- Process isolation with timeout and cleanup

#### Diagnosis
- pprof profile collection (CPU, allocs, mutex, block)
- Profile merging and per-operation normalization
- Flat and cumulative delta computation
- Contributor candidate ranking with `min_share` filtering
- Annotation of functions in changed files
- Reproduction command for manual verification
- Short profile detection and warnings

#### GitHub Integration
- Composite GitHub Action with automatic installation
- PR comment with benchmark comparison table
- Step summary for all runs
- Fork PR detection with safe fallback (no write operations)
- Artifact upload for detailed results
- Clear exit codes (0=pass, 1=fail, 2=config, 3=execution, 4=inconclusive)

#### Baseline Management
- Filesystem storage with atomic writes
- Git orphan branch storage (local only)
- Environment compatibility checking
- Baseline listing and pruning
- Multiple baseline support with names

#### Output Formats
- Text format for terminal
- Markdown format for documents
- JSON format for programmatic access
- GitHub format for PR comments with length limits

#### Evaluation Suite
- 8 evaluation scenarios with intentional regressions
- Detection rate measurement with Wilson confidence intervals
- False positive rate measurement on A/A tests
- Diagnosis hit rate (top-K accuracy)
- Environment fingerprinting in reports
- Nightly evaluation workflow

#### Configuration
- YAML configuration file (`perfguard.yaml`)
- JSON Schema validation (`schemas/config.v1.json`)
- Environment variable overrides
- CLI flag overrides
- Configuration validation with field-path error messages

#### Quality Assurance
- 36 edge cases tested and documented
- Unit tests with 80%+ coverage (90%+ for critical packages)
- Integration tests for adapters (git, runner, GitHub)
- End-to-end tests on temporary repositories
- Statistical simulation tests with fixed seeds
- Fuzz testing for parsers
- Race detector in CI
- Shuffle mode for test order independence

#### Documentation
- Comprehensive README with quick start
- Architecture documentation with design rationale
- Testing strategy with edge case matrix
- Quick start guide (5-minute setup)
- GitHub Actions examples
- FAQ with 40+ questions
- Implementation plan with task tracking
- PRD with requirements and acceptance criteria
- Schema freeze documentation
- Security review documentation

#### Security
- Minimal GitHub token permissions (contents:read, pull-requests:write)
- Environment variable allowlist (35 safe variables)
- Secret keyword filtering (TOKEN, SECRET, PASSWORD, KEY, AUTH, etc.)
- Fork PR write restriction
- No shell command execution from config
- Command injection prevention (array arguments only)
- No `pull_request_target` usage

### Fixed
- N/A (initial release)

### Changed
- N/A (initial release)

### Deprecated
- N/A (initial release)

### Removed
- N/A (initial release)

### Security
- Environment allowlist prevents secret exposure to benchmarks
- Fork PRs automatically skip write operations
- All exec calls use array arguments (no shell injection)
- Tokens never logged or written to files

## Release Statistics

- **Implementation tasks**: 45/45 complete (100%)
- **Test coverage**: 80%+ overall, 90%+ for stats/decision/benchparse/config
- **Edge cases**: 36/36 covered
- **Documentation pages**: 9 documents, ~2000 lines
- **Evaluation scenarios**: 8 scenarios, 160 total trials
- **Supported Go versions**: 1.24+
- **Supported platforms**: Linux (fully), macOS (best effort)

## Future Plans

See [docs/IMPLEMENTATION_PLAN.md](docs/IMPLEMENTATION_PLAN.md) open questions section for planned enhancements.

Potential v1.1+ features based on user feedback:
- Windows support
- Additional profile types
- More evaluation scenarios
- Performance optimizations
- Additional CI platform integrations

[1.0.0]: https://github.com/kyyril/perfguard/releases/tag/v1.0.0
