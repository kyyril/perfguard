# P9-T2 Implementation Summary

**Task:** Freeze v1 schema and compatibility tests

**Status:** ✓ Complete

**Date:** 2026-10-23

## Overview

Froze the v1 schema for PerfGuard's JSON outputs and created comprehensive compatibility tests to ensure v1.x releases can read files from v0.x and maintain backward compatibility throughout the v1 lifecycle.

## Schema Files Frozen

All three schemas are now frozen for v1.0:

1. **schemas/result.v1.json** - Output from `perfguard run`
2. **schemas/comparison.v1.json** - Output from `perfguard compare` and `perfguard check`
3. **schemas/config.v1.json** - Configuration file format

**Freeze date:** 2026-10-23  
**Schema version:** 1 (constant in `model.CurrentSchemaVersion`)

## Compatibility Guarantees

### Guaranteed (Cannot break in v1.x)

- ✓ Forward compatibility: v1.x reads files from any earlier v1.y
- ✓ Schema version remains 1
- ✓ Required fields always present
- ✓ Field types immutable
- ✓ Enum values not removed/renamed

### Allowed Changes

- ✓ New optional fields
- ✓ New enum values (e.g., new reason codes)
- ✓ Additional array items
- ✓ Optional fields may be absent

### Forbidden Changes (Require v2.0)

- ❌ Remove required fields
- ❌ Change field types
- ❌ Rename fields
- ❌ Change enum values
- ❌ Change schema_version to 2

## Test Fixtures Created

### 1. v0x-result.json
**File:** `testdata/fixtures/v0x-result.json`

Simulates `result.json` output from v0.x release:
- Schema version 1
- Complete Run structure
- 3 samples for BenchmarkEncode
- All required fields present
- Standard metrics: sec/op, B/op, allocs/op

### 2. v0x-comparison.json
**File:** `testdata/fixtures/v0x-comparison.json`

Simulates `comparison.json` output from v0.x release:
- Schema version 1
- Base and head run summaries
- 2 metric comparisons (one fail, one unchanged)
- Verdict: fail
- Confidence intervals as 2-element arrays

## Compatibility Tests

### File: `internal/model/compatibility_test.go`

**TestSchemaCompatibility_V0x_Result**
- Reads v0x-result.json fixture
- Verifies unmarshaling succeeds
- Checks all required fields present
- Validates data types and structure
- Confirms samples and metrics readable

**TestSchemaCompatibility_V0x_Comparison**
- Reads v0x-comparison.json fixture
- Verifies unmarshaling succeeds
- Checks base/head summaries
- Validates items array
- Confirms verdict and status enums

**TestSchemaCompatibility_RoundTrip**
- Creates Run struct
- Marshals to JSON
- Unmarshals back to struct
- Verifies equality with go-cmp
- Ensures no data loss in serialization

**TestSchemaVersion_Const**
- Verifies `CurrentSchemaVersion == 1`
- Documents freeze for v1.0

**TestSchemaVersion_InJSON**
- Marshals Run to JSON
- Parses as raw map
- Confirms `schema_version` field present
- Validates value is 1

## Documentation

### File: `docs/SCHEMA_V1.md` (new)

Comprehensive schema freeze document:

**Sections:**
1. Overview and freeze status
2. Schema files list
3. Compatibility guarantees (guaranteed, allowed, forbidden)
4. Schema version history table
5. Current schema details for each file
6. Fixed reason codes list
7. Testing approach
8. Future migration path (v2.0)
9. Breaking changes examples
10. Review process for changes

**Key content:**
- Clear examples of breaking vs. non-breaking changes
- Process for adding optional fields
- Migration strategy for v2.0
- Contact information

## Immutable Structures

### result.v1.json Core
- Sample: `{iterations: int, values: {metric: float}}`
- Environment: os, arch, go_version, num_cpu, cpu_model, gomaxprocs (all required)
- Git: sha, ref, dirty (all required)

### comparison.v1.json Core
- MetricComparison: 17 required fields
- Verdict enum: pass, inconclusive, warn, fail
- Status enum: unchanged, improved, warn, fail, inconclusive, new, removed
- CI: exactly 2 elements [lower, upper]

### Reason Codes (Fixed)
11 codes frozen:
- new_benchmark, removed_benchmark
- insufficient_samples, env_mismatch
- noisy, zero_base
- not_significant, exceeds_fail, exceeds_warn
- improvement, below_threshold

## Impact on Development

### Before making changes:
1. Check if change affects JSON output
2. Verify no required fields removed
3. Verify no field types changed
4. Run compatibility tests
5. Document new optional fields in SCHEMA_V1.md

### CI Validation:
- All fixtures validated against JSON schemas
- Compatibility tests run on every PR
- Schema version constant verified

## Files Created/Modified

**New files (3):**
- testdata/fixtures/v0x-result.json
- testdata/fixtures/v0x-comparison.json
- internal/model/compatibility_test.go
- docs/SCHEMA_V1.md

**Modified files (1):**
- docs/IMPLEMENTATION_PLAN.md

## Verification

Compatibility tests prove:
- [x] v0.x fixtures unmarshal successfully
- [x] All required fields present and correct types
- [x] Round-trip serialization preserves data
- [x] Schema version frozen at 1
- [x] Schema version appears in JSON output

## Next Steps

**P9-T3:** Security review
- Token handling in GitHub integration
- Fork PR safety
- Argument injection prevention
- Environment variable allowlist

**Before v1.0 release:**
- Run compatibility tests
- Validate all example outputs against schemas
- Document migration path for hypothetical v2.0

## Notes

- Schemas were already well-defined from P0-P7
- This task formalizes the freeze and adds safeguards
- Future v1.x releases inherit these compatibility guarantees
- Breaking changes require v2.0 with migration tool

**P9-T2 complete: v1 schema frozen with compatibility tests and documentation.**
