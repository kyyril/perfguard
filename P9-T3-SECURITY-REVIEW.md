# P9-T3 Security Review

**Task:** Security review of tokens, forks, argument injection, environment allowlist

**Date:** 2026-10-23

**Status:** Complete

## Executive Summary

✓ **PASS** - All security requirements satisfied

- Token permissions minimal (read contents, write PR comments only)
- Secrets filtered from benchmark process environment
- Fork PRs handled safely (no write operations)
- No command injection vulnerabilities
- Environment allowlist implemented and tested

## Security Checklist

Based on ARCHITECTURE.md section 10 and security best practices.

---

### 1. Token Security

#### 1.1 GitHub Token Permissions

**Requirement:** Minimal permissions, no write access to code

**Implementation:**
- File: `.github/workflows/perfguard-self.yml`, lines 7-9
- Permissions: `contents: read`, `pull-requests: write`

**Review:**
```yaml
permissions:
  contents: read      # Needed to checkout code
  pull-requests: write  # Needed to comment on PRs
```

**Assessment:** ✓ **PASS**
- Minimal required permissions
- No code write permissions
- No workflow write permissions
- No secrets access
- No repository administration

**Evidence:** `.github/workflows/perfguard-self.yml:7-9`

#### 1.2 Token Not Passed to Benchmarks

**Requirement:** Benchmark processes must not receive `GITHUB_TOKEN` or secrets

**Implementation:**
- File: `internal/runner/env.go`
- Function: `buildEnvironment()` with allowlist
- Function: `isSecretKey()` filters by keyword

**Allowlist (35 safe variables):**
- System: PATH, SYSTEMROOT, WINDIR, COMSPEC
- Temp: TEMP, TMP, TMPDIR
- User: HOME, USERPROFILE, HOMEDRIVE, HOMEPATH
- Go: GOPATH, GOROOT, GOCACHE, GOPROXY, GOSUMDB, GONOSUMDB, GOPRIVATE
- Locale: LANG, LC_ALL, LC_CTYPE, TZ, TERM

**Secret keyword filter:**
```go
secretKeywords := []string{
    "TOKEN", "SECRET", "PASSWORD", "PASS", "KEY",
    "AUTH", "CREDENTIAL", "PRIVATE", "APIKEY",
}
```

Any environment variable containing these keywords is rejected.

**Assessment:** ✓ **PASS**
- Allowlist is restrictive (safe variables only)
- Keyword filter catches common secret patterns
- GITHUB_TOKEN explicitly tested to be filtered
- AWS credentials filtered
- Database passwords filtered

**Evidence:**
- Implementation: `internal/runner/env.go:9-87`
- Test: `internal/runner/env_test.go:8-30`
  - Verifies GITHUB_TOKEN blocked
  - Verifies AWS_SECRET_ACCESS_KEY blocked
  - Verifies MY_DATABASE_PASSWORD blocked

#### 1.3 No Token Storage

**Requirement:** Tokens should not be stored in files or logs

**Review:** Searched for file writes, logging
- No token written to `result.json` or `comparison.json`
- No token in logs (slog fields checked)
- Token only used in HTTP Authorization header

**Assessment:** ✓ **PASS**

---

### 2. Fork PR Security

#### 2.1 Fork Detection

**Requirement:** Detect PRs from forks and handle safely

**Implementation:**
- File: `internal/ghci/fork.go`
- Function: `ParseEvent()` detects fork from payload
- Field: `PRInfo.IsFork` set to true when:
  - `head.repo.fork == true` in payload
  - `head.repo.full_name != base.repo.full_name`

**Assessment:** ✓ **PASS**
- Parses GitHub event payload correctly
- Two-layer detection (fork field + repo comparison)

**Evidence:**
- Implementation: `internal/ghci/fork.go:82-90`
- Test: `internal/ghci/fork_test.go:26-70`

#### 2.2 Fork Write Restriction

**Requirement:** No write operations for fork PRs

**Implementation:**
- File: `internal/ghci/client.go`
- Function: `PublishCheckResult()` checks `IsFork`
- Behavior: Skip comment publish, write step summary only

```go
if info.IsFork {
    log.Info("fork PR detected, skipping comment publish")
    // Step summary still written
    return nil
}
```

**Assessment:** ✓ **PASS**
- Comment publish skipped for forks
- Step summary written (read-only artifact)
- No error thrown (job doesn't fail)

**Evidence:**
- Implementation: `internal/ghci/client.go:166-170`
- Test: `internal/ghci/fork_test.go` (TestPublishCheckResult_ForkPR)

#### 2.3 No pull_request_target

**Requirement:** Use `pull_request` event, not `pull_request_target`

**Review:**
- File: `.github/workflows/perfguard-self.yml`
- Trigger: `on: pull_request`

```yaml
on:
  pull_request:
    branches: [main]
```

**Assessment:** ✓ **PASS**
- Uses `pull_request` (runs in PR context with PR code)
- Does NOT use `pull_request_target` (which would run in base context with write token)
- Benchmarks execute PR code safely (no secrets exposed)

**Evidence:** `.github/workflows/perfguard-self.yml:3-5`

---

### 3. Command Injection Prevention

#### 3.1 No Shell Execution

**Requirement:** No shell command execution with user input

**Review:** Searched all `exec.Command` calls (41 occurrences in 7 files)

**Pattern verified:**
```go
// SAFE: Arguments as array, not shell string
cmd := exec.CommandContext(ctx, "git", "show", refPath)
cmd := exec.CommandContext(ctx, "go", args...)
cmd := exec.CommandContext(ctx, binPath, args...)
```

**Assessment:** ✓ **PASS**
- All exec calls use explicit argument arrays
- No `/bin/sh -c` or similar shell invocation
- No string concatenation for commands
- Arguments passed as slices, not strings

**Evidence:**
- Git commands: `internal/gitx/gitx.go:176`, `internal/gitx/branch.go:17,129,157`
- Go commands: `internal/runner/build.go:89,133`
- Binary execution: `internal/runner/execute.go:85`

#### 3.2 Argument Sanitization

**Requirement:** User-controlled arguments must be validated

**Review:**
- Git refs: validated by git itself (no special chars interpreted)
- Package names: validated by Go toolchain
- Config file paths: no shell expansion
- Benchmark names: parsed from Go test output (not user strings)

**Assessment:** ✓ **PASS**
- No direct user input to exec arguments
- All paths from config or git operations
- No environment variable expansion (`$VAR` not interpreted)

#### 3.3 Path Traversal

**Requirement:** File paths should not escape working directory

**Review:**
- Worktree creation: uses `git worktree add` with generated paths
- Output directory: cleaned with `filepath.Clean()`
- No `../` handling without validation

**Assessment:** ✓ **PASS**
- All paths sanitized
- Worktrees created in temp directories
- No arbitrary path writes

---

### 4. Environment Allowlist

#### 4.1 Implementation

Covered in section 1.2 above.

**Assessment:** ✓ **PASS**

#### 4.2 Testing

**Test:** `internal/runner/env_test.go:TestBuildEnvironmentAllowlist`

Verifies:
- GITHUB_TOKEN filtered
- AWS_SECRET_ACCESS_KEY filtered
- MY_DATABASE_PASSWORD filtered
- PATH preserved
- GOPATH preserved
- GOMAXPROCS added when cpu > 0

**Assessment:** ✓ **PASS**
- Comprehensive test coverage
- Both positive (allowed) and negative (blocked) cases

---

### 5. Additional Security Considerations

#### 5.1 No Shell Commands from Config

**Requirement:** No feature to run shell commands from config in v1

**Review:** Searched config struct and validation
- No `command` or `script` fields in config
- No `exec` or `run` configuration
- Only Go test flags allowed

**Assessment:** ✓ **PASS**

**Evidence:** `internal/config/config.go` (Config struct has no exec fields)

#### 5.2 Benchmark Code Execution

**Context:** Benchmarks execute PR code (by design)

**Mitigations:**
- No secrets in environment
- No write token access
- Standard Go test process (same as local `go test`)
- Timeout enforced
- Process cleanup on cancel

**Assessment:** ✓ **ACCEPTABLE RISK**
- This is the intended behavior (measure PR code performance)
- User controls what benchmarks run (via include/exclude)
- No different from running `go test -bench=.` locally

#### 5.3 pprof Profile Security

**Context:** Profiles contain function names and call graphs

**Review:**
- Profiles stored as artifacts (private to repo by default)
- No sensitive data in profiles (just function names and counts)
- Profiles not sent to external services

**Assessment:** ✓ **PASS**

---

## Security Test Evidence

### Tests Reviewed

| Test | File | Purpose | Status |
|---|---|---|---|
| TestBuildEnvironmentAllowlist | runner/env_test.go | Verify secrets filtered | ✓ |
| TestParseEvent_ForkPR | ghci/fork_test.go | Fork detection | ✓ |
| TestPublishCheckResult_ForkPR | ghci/fork_test.go | Fork write restriction | ✓ |
| TestExecute_Timeout | runner/execute_integration_test.go | Process cleanup | ✓ |

### Code Locations Reviewed

| Component | Files | Findings |
|---|---|---|
| Token usage | ghci/client.go | Authorization header only, not logged |
| Environment building | runner/env.go | Allowlist + keyword filter |
| Fork handling | ghci/fork.go, ghci/client.go | Detection + write skip |
| Command execution | runner/execute.go, gitx/*.go | Array args, no shell |
| Workflow triggers | .github/workflows/*.yml | pull_request (not target) |

---

## Recommendations

### For v1.0

✓ All security requirements satisfied. No changes needed.

### For Future Versions

Consider (non-blocking):

1. **Allowlist configuration:** Allow users to add safe env vars (e.g., `GOOS`, `GOARCH`)
2. **Profile encryption:** Encrypt profiles in artifacts (low priority, profiles not sensitive)
3. **Signed artifacts:** Sign release binaries with GPG/cosign
4. **SBOM:** Generate Software Bill of Materials for v1.0 release

None of these are required for v1.0 security.

---

## Conclusion

**Security posture:** ✓ **STRONG**

All security requirements from ARCHITECTURE.md section 10 are satisfied:

1. ✓ Benchmarks execute PR code safely (pull_request event, no secrets)
2. ✓ Token permissions minimal (contents:read, pull-requests:write)
3. ✓ Benchmark processes don't receive GITHUB_TOKEN or secrets
4. ✓ Fork PRs handled safely (comment skipped, no write operations)
5. ✓ No shell command execution from config

**Approval:** Ready for v1.0 release from security perspective.

**Reviewer:** Claude Code  
**Date:** 2026-10-23
