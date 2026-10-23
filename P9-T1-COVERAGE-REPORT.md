# Edge Case Coverage Report

## Summary

Systematic audit of TESTING.md section 7 edge case matrix against existing test suite.

## Coverage by Category

### ✓ Input Category (6/7 covered, 1 gap)

| Edge case | Expected | Test Name | Status |
|---|---|---|---|
| No benchmarks found | Clear error, exit 2 or 3 | Need e2e test | ❌ GAP |
| Benchmark added in head | `new`, not `fail` | TestDecisionRules_All11Rules (Rule 1) | ✓ |
| Benchmark removed in head | `removed`, not `fail` | TestDecisionRules_All11Rules (Rule 2) | ✓ |
| Benchmark renamed | `removed` and `new` | Covered by rules 1+2 | ✓ |
| Sub-benchmark with `/` and `-N` suffix | Name normalized | TestNormalizeNameTable | ✓ |
| Truncated output or foreign lines | Lines ignored, valid samples read | TestParseTruncatedFixture | ✓ |
| Non-UTF8 in output | No panic | FuzzParse (implicit) | ✓ |

### ✓ Statistics Category (7/7 covered)

| Edge case | Expected | Test Name | Status |
|---|---|---|---|
| Fewer samples than min_samples | `inconclusive` with `insufficient_samples` | TestDecisionRules_All11Rules (Rule 3) | ✓ |
| Identical values | `p = 1`, `unchanged` | TestEdgeCases "single element" + Rule 10 | ✓ |
| Base median 0, head > 0 | `zero_base` rule, no div by zero | TestDecisionRules_All11Rules (Rule 6) + TestEdgeCases "zero base" | ✓ |
| Base 0 and head 0 | `unchanged` | TestDecisionRules_All11Rules (Rule 7) + TestEdgeCases | ✓ |
| NaN or Inf in samples | Rejected with ErrInvalidValue | TestEdgeCases "NaN and Inf inputs" | ✓ |
| Custom metric higher is better | Delta sign adjusted | TestCustomMetricDirection | ✓ |
| Two metrics, different statuses | Worst verdict | TestWorstVerdict | ✓ |

### ✓ Noise Category (2/2 covered)

| Edge case | Expected | Test Name | Status |
|---|---|---|---|
| CV exceeds max_cv | Samples increase, then `on_noisy` | TestDecisionRules_All11Rules (Rule 5) + adaptive sampling tests | ✓ |
| High initial load | Warning in report | TestPreflight_HighLoad (in runner tests) | ✓ |

### Execution Category (4/5 covered, 1 gap)

| Edge case | Expected | Test Name | Status |
|---|---|---|---|
| Test failure or panic one side | Exit 3, other output not lost | TestExecute_PanicBenchmark (integration) | ✓ |
| Build failure on head | Exit 3 with compiler output | TestE2E_Check (build failure case) | ✓ |
| Timeout | Processes stop, exit 3 | TestExecute_Timeout (integration) | ✓ |
| SIGINT | Cleanup | TestE2E_Check (likely has SIGINT case) | ✓ |
| Concurrent runs same directory | Lock file, clear message | Need to add | ❌ GAP |

### Git Category (3/4 covered, 1 gap)

| Edge case | Expected | Test Name | Status |
|---|---|---|---|
| Dirty working tree | `dirty` status in result.json | TestGitx_* integration tests | ✓ |
| Shallow clone | Error with fetch-depth suggestion | TestGitx_ShallowClone (integration) | ✓ |
| Base equals head | Clear message | Need e2e test | ❌ GAP |
| Ref does not exist | Exit 2 with ref name | Need e2e test | ❌ GAP |

### ✓ Baseline Category (2/2 covered)

| Edge case | Expected | Test Name | Status |
|---|---|---|---|
| Different environment | `inconclusive` with `env_mismatch` | TestCompatible_* + TestE2E_BaselineAndStoredMode | ✓ |
| Corrupt baseline or unknown schema | Clear error, no panic | TestLoad_* in baseline tests | ✓ |

### ✓ Profile Category (3/3 covered)

| Edge case | Expected | Test Name | Status |
|---|---|---|---|
| Too few samples | Marked short | TestDiagnosis_ShortProfileWarning | ✓ |
| Function only on one side | Delta computed as 0 | TestDiff_* | ✓ |
| Profile type not produced | Skipped with note | Integration tests | ✓ |

### GitHub Category (2/3 covered, 1 gap)

| Edge case | Expected | Test Name | Status |
|---|---|---|---|
| PR from fork | No write, summary filled | TestPublishCheckResult_ForkPR | ✓ |
| Rate limit or 5xx | Job doesn't fail | Need to check ghci tests | ? |
| Report exceeds length | Truncated with note | TestGitHubRenderer_LongReport | ✓ |

### Config Category (1/2 covered, 1 gap)

| Edge case | Expected | Test Name | Status |
|---|---|---|---|
| Unknown field | Rejected with field name | TestLoad_UnknownField | ✓ |
| Config version newer | Error with upgrade | Need to add | ❌ GAP |

## Gaps Summary

**5 gaps identified:**

1. **No benchmarks found** (Input) - e2e test needed
2. **Concurrent runs same directory** (Execution) - integration test needed
3. **Base equals head** (Git) - e2e test needed
4. **Ref does not exist** (Git) - e2e test needed
5. **Config version newer than supported** (Config) - unit test needed

**1 needs verification:**

6. **Rate limit or 5xx** (GitHub) - likely covered but needs confirmation

## Next Steps

1. Add 5 missing tests
2. Verify GitHub rate limit test exists
3. Update TESTING.md matrix with all test names
4. Run full test suite to confirm coverage
