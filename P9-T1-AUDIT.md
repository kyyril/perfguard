# Edge Case Coverage Audit

Task P9-T1: Verify all edge cases in TESTING.md section 7 have tests.

## Audit Methodology

1. Read edge case matrix from TESTING.md
2. Search for existing test coverage
3. Identify gaps
4. Write missing tests
5. Update matrix with test names

## Edge Case Matrix Status

### Input Category

| Edge case | Expected | Layer | Test Name | Status |
|---|---|---|---|---|
| No benchmarks found | Clear error, exit 2 or 3 | e2e | Need to check | ? |
| Benchmark added in head | `new`, not `fail` | unit, e2e | Need to check | ? |
| Benchmark removed in head | `removed`, not `fail` | unit, e2e | Need to check | ? |
| Benchmark renamed | `removed` and `new` | unit | Need to check | ? |
| Sub-benchmark with `/` and `-N` suffix | Name normalized | unit | TestParseLine_SubBenchmarks? | ? |
| Truncated output or foreign lines | Lines ignored, valid samples read | unit, fuzz | TestParse_MixedOutput? | ? |
| Non-UTF8 in output | No panic | fuzz | FuzzParse? | ? |

### Statistics Category

| Edge case | Expected | Layer | Test Name | Status |
|---|---|---|---|---|
| Fewer samples than min_samples | `inconclusive` with `insufficient_samples` | unit | TestCompareRuns_InsufficientSamples? | ? |
| Identical values | `p = 1`, `unchanged` | unit | TestMannWhitneyU_Identical? | ? |
| Base median 0, head > 0 | `zero_base` rule, no div by zero | unit | TestCompareRuns_ZeroBase? | ? |
| Base 0 and head 0 | `unchanged` | unit | TestCompareRuns_BothZero? | ? |
| NaN or Inf in samples | Rejected or dropped | unit | Need to check | ? |
| Custom metric higher is better | Delta sign adjusted | unit | TestCompareRuns_CustomDirection? | ? |
| Two metrics, different statuses | Worst verdict | unit | TestCompareRuns_MultiMetric? | ? |

### Noise Category

| Edge case | Expected | Layer | Test Name | Status |
|---|---|---|---|---|
| CV exceeds max_cv | Samples increase, then `on_noisy` | unit, integration | TestCheck_AdaptiveSampling? | ? |
| High initial load | Warning in report | unit | TestPreflight_HighLoad? | ? |

### Execution Category

| Edge case | Expected | Layer | Test Name | Status |
|---|---|---|---|---|
| Test failure or panic one side | Exit 3, other output not lost | integration | TestExecute_PanicBenchmark? | ? |
| Build failure on head | Exit 3 with compiler output | e2e | TestE2E_BuildFailure? | ? |
| Timeout | Processes stop, exit 3 | integration | TestExecute_Timeout? | ? |
| SIGINT | Cleanup | e2e | TestE2E_SIGINT? | ? |
| Concurrent runs same directory | Lock file, clear message | integration | Need to add | ❌ |

### Git Category

| Edge case | Expected | Layer | Test Name | Status |
|---|---|---|---|---|
| Dirty working tree | `dirty` status in result.json | integration | TestGitx_DirtyTree? | ? |
| Shallow clone | Error with fetch-depth suggestion | integration | TestGitx_ShallowClone? | ? |
| Base equals head | Clear message | e2e | Need to add | ❌ |
| Ref does not exist | Exit 2 with ref name | e2e | Need to add | ❌ |

### Baseline Category

| Edge case | Expected | Layer | Test Name | Status |
|---|---|---|---|---|
| Different environment | `inconclusive` with `env_mismatch` | unit, e2e | TestCompatible_*, TestE2E_BaselineAndStoredMode? | ? |
| Corrupt baseline or unknown schema | Clear error, no panic | unit | Need to check | ? |

### Profile Category

| Edge case | Expected | Layer | Test Name | Status |
|---|---|---|---|---|
| Too few samples | Marked short | unit | TestDiagnosis_ShortProfileWarning? | ? |
| Function only on one side | Delta computed as 0 | unit | TestDiff_*? | ? |
| Profile type not produced | Skipped with note | integration | Need to check | ? |

### GitHub Category

| Edge case | Expected | Layer | Test Name | Status |
|---|---|---|---|---|
| PR from fork | No write, summary filled | unit, integration | TestPublishCheckResult_ForkPR? | ? |
| Rate limit or 5xx | Job doesn't fail | integration | Need to check | ? |
| Report exceeds length | Truncated with note | unit | TestGitHubRenderer_LongReport? | ? |

### Config Category

| Edge case | Expected | Layer | Test Name | Status |
|---|---|---|---|---|
| Unknown field | Rejected with field name | unit | TestLoad_UnknownField? | ? |
| Config version newer | Error with upgrade | unit | Need to add | ❌ |

## Next Steps

1. Search each test file for patterns matching edge cases
2. Run tests to verify they exist and pass
3. Write missing tests (marked ❌)
4. Update TESTING.md matrix with confirmed test names
