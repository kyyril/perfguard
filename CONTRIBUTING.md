# Contributing to PerfGuard

Thank you for your interest in contributing to PerfGuard!

## Development Setup

### Prerequisites
- Go 1.24 or newer
- Git
- golangci-lint (for linting)
- Linux or macOS (Windows not yet supported)

### Getting Started

1. Fork and clone the repository
```bash
git clone https://github.com/YOUR_USERNAME/perfguard.git
cd perfguard
```

2. Install dependencies
```bash
go mod download
```

3. Build and test
```bash
make build
make check
```

## Project Structure

See `README.md` section "Repository structure" for package organization.

Key directories:
- `cmd/perfguard/` - CLI entry point
- `internal/` - Core packages (see ARCHITECTURE.md for dependency rules)
- `docs/` - Documentation
- `test/` - Integration and E2E tests

## Development Workflow

### Before Starting Work

1. Check existing issues or create one describing what you want to work on
2. Comment on the issue to let others know you're working on it
3. Create a feature branch: `git checkout -b feature/your-feature-name`

### Coding Guidelines

**All code must follow the rules in `AGENTS.md`**. Key points:

- Follow Effective Go and Go Code Review Comments
- One responsibility per package
- `context.Context` is the first parameter for I/O functions
- No mutable global state
- Wrap errors with context: `fmt.Errorf("operation: %w", err)`
- Table-driven tests for logic with multiple cases
- Minimum 80% test coverage (90% for stats, decision, benchparse, config)

### Writing Tests

- Every behavior change needs a test
- Bug fixes start with a failing test
- Use `t.Run()` for table tests
- No `time.Sleep` for synchronization (use channels or fake clock)
- Tests must pass with `-race` and `-shuffle=on`

### Running Tests

```bash
# Unit tests
make test

# With race detector
make test-race

# Integration tests
make test-integration

# E2E tests
make test-e2e

# All quality gates
make check
```

### Performance Testing

Before submitting a PR that might affect performance:

```bash
# Run PerfGuard on itself
perfguard check --base main
```

### Commit Messages

Use Conventional Commits format:

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

Types: `feat`, `fix`, `docs`, `test`, `refactor`, `perf`, `chore`, `ci`

Examples:
```
feat(stats): add Wilcoxon signed-rank test
fix(runner): prevent race in alternating execution
docs(quickstart): add GitHub Actions example
```

### Pull Request Process

1. Ensure all tests pass: `make check`
2. Update documentation if needed
3. Add tests for new features or bug fixes
4. Keep PRs focused - one feature or fix per PR
5. Fill out the PR template completely
6. Link related issues

### PR Review

Reviewers will check:
- Code follows AGENTS.md guidelines
- Tests are adequate and pass
- Documentation is updated
- No breaking changes without discussion
- Performance impact is acceptable

## Architecture Decisions

Major design changes should be discussed in an issue first. Include:
- Problem statement
- Proposed solution
- Alternatives considered
- Trade-offs

See `docs/ARCHITECTURE.md` for current design principles.

## Questions?

- Check `docs/FAQ.md`
- Search existing issues
- Open a new issue with your question

## Code of Conduct

Be respectful and professional. We're all here to make performance testing better.

## License

By contributing, you agree that your contributions will be licensed under the same license as the project (see LICENSE file).
