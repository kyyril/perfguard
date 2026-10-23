# Schema Version 1 - Frozen for v1.0

**Freeze Date:** 2026-10-23  
**Status:** FROZEN - No breaking changes allowed

## Overview

This document defines the schema contract for PerfGuard v1.0. All v1.x releases must maintain backward compatibility with these schemas.

## Schema Files

- `schemas/result.v1.json` - Output format for `perfguard run`
- `schemas/comparison.v1.json` - Output format for `perfguard compare` and `perfguard check`
- `schemas/config.v1.json` - Configuration file format

## Compatibility Guarantees

### What is guaranteed

1. **Forward compatibility:** PerfGuard v1.x can read files written by any earlier v1.y (where y < x)
2. **Schema version:** All JSON outputs include `"schema_version": 1`
3. **Required fields:** All fields marked `required` in the schema will always be present
4. **Field types:** Field types will not change (string remains string, number remains number)
5. **Enum values:** Existing enum values will not be removed or renamed

### What is allowed

1. **New optional fields:** May be added to any object
2. **New enum values:** May be added to enums (e.g., new `Status` or `Reason` codes)
3. **Additional array items:** Arrays may contain more items than before
4. **Optional field presence:** Optional fields may be absent even if they were present before

### What is forbidden

1. **Removing required fields:** Would break readers expecting them
2. **Changing field types:** Breaks deserialization
3. **Renaming fields:** Equivalent to remove + add
4. **Changing enum values:** Breaks status/verdict matching
5. **Changing schema_version:** Must remain 1 for all v1.x

## Schema Version History

| Version | Released | Status |
|---|---|---|
| 1 | 2026-10-23 (v1.0) | Active, frozen |

Future major versions (v2.0+) may use `schema_version: 2` with migration path documented.

## Current Schema Details

### result.v1.json

**Purpose:** Raw benchmark execution results

**Key fields:**
- `schema_version`: Always 1
- `id`: Unique run identifier
- `side`: "base" or "head"
- `created_at`: ISO 8601 timestamp
- `git`: Commit SHA, ref, dirty flag
- `env`: OS, arch, Go version, CPU info
- `params`: Benchmark execution parameters
- `benchmarks`: Array of benchmark results with samples
- `profiles`: Optional pprof profile references

**Immutable characteristics:**
- Sample structure: `{iterations: int, values: {metric: float}}`
- Environment required fields: os, arch, go_version, num_cpu, cpu_model, gomaxprocs
- Git required fields: sha, ref, dirty

### comparison.v1.json

**Purpose:** Statistical comparison between base and head

**Key fields:**
- `schema_version`: Always 1
- `base`, `head`: Run summaries (subset of Run)
- `items`: Array of metric comparisons
- `verdict`: "pass", "inconclusive", "warn", or "fail"
- `diagnosis`: Optional array of pprof analysis results
- `notes`: Optional warnings and messages

**Immutable characteristics:**
- MetricComparison structure: 17 required fields including benchmark, metric, medians, delta_pct, ci, p_value, cv, n, status, reason
- Verdict enum: 4 values (pass, inconclusive, warn, fail)
- Status enum: 7 values (unchanged, improved, warn, fail, inconclusive, new, removed)
- CI array: exactly 2 elements [lower, upper]

**Diagnosis structure:**
- Candidate: function name, flat_delta, cum_delta, touched flag
- Notes: array of strings

### config.v1.json

**Purpose:** User configuration file

**Key fields:**
- `version`: Always 1
- `benchmarks`: Package selection and flags
- `execution`: Mode and sampling strategy
- `thresholds`: Statistical and effect size thresholds
- `noise`: CV limits and behavior
- `profiling`: pprof collection settings
- `baseline`: Storage configuration

**Validation rules:**
- `benchmarks.count >= thresholds.min_samples`
- `thresholds.fail >= thresholds.warn` for each metric
- `thresholds.alpha` in (0, 1)
- `thresholds.min_samples >= 2`
- Custom metrics require `direction` field

## Reason Codes (Fixed)

From `internal/model/model.go`:

```
new_benchmark
removed_benchmark
insufficient_samples
env_mismatch
noisy
zero_base
not_significant
exceeds_fail
exceeds_warn
improvement
below_threshold
```

New codes may be added in v1.x. Code meaning must not change.

## Testing

Compatibility is verified by:

1. **Unit tests:** `internal/model/compatibility_test.go`
   - TestSchemaCompatibility_V0x_Result
   - TestSchemaCompatibility_V0x_Comparison
   - TestSchemaCompatibility_RoundTrip
   - TestSchemaVersion_Const

2. **Fixtures:** `testdata/fixtures/v0x-*.json`
   - Simulated v0.x output
   - Must remain readable by v1.x

3. **JSON Schema validation:** CI validates all test fixtures against schemas

## Migration Path (Future)

When v2.0 is released:

1. v2.0 writes `"schema_version": 2`
2. v2.0 can read both version 1 and 2
3. Users can convert v1 → v2 with `perfguard migrate --to v2`
4. v1.x continues to use `"schema_version": 1`

## Breaking Changes (Requires v2.0)

Examples of changes that would require schema version 2:

- Renaming `delta_pct` to `delta_percent`
- Changing `ci` from array to object with `lower`/`upper` fields
- Removing `status` field
- Changing `p_value` from float to object with p-value and test name
- Splitting `env` into `hardware` and `software` objects

## Review Process

Any change to model structs, schemas, or JSON output requires:

1. Verify no required fields removed
2. Verify no field types changed
3. Verify no enum values removed/renamed
4. Run compatibility tests
5. Update this document if new optional fields added

## Contact

Questions about schema compatibility: see `docs/ARCHITECTURE.md` or open an issue.

---

**This schema is frozen for v1.0. No breaking changes allowed without major version bump.**
