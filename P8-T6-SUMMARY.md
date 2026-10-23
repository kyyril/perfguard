# P8-T6 Implementation Summary

**Task:** Nightly eval workflow

**Status:** ✓ Complete

**Date:** 2026-10-23

## Overview

Implemented a GitHub Actions workflow that runs the evaluation suite nightly, measuring PerfGuard's detection quality across all scenarios. The workflow runs on a schedule, executes all scenarios, generates an aggregated report, and uploads results as artifacts.

## Files Created

### Workflow

**`.github/workflows/nightly-eval.yml`** (64 lines)

Scheduled workflow with the following characteristics:

**Triggers:**
- Schedule: daily at 2 AM UTC (`cron: '0 2 * * *'`)
- Manual: `workflow_dispatch` for on-demand runs

**Permissions:**
- `contents: read` (minimal, read-only access)

**Job: eval**
- Runner: `ubuntu-latest`
- Timeout: 120 minutes (2 hours)
- Steps:
  1. Checkout code
  2. Set up Go from `go.mod`
  3. Build PerfGuard with `make build`
  4. Create output directory
  5. Run all scenarios (loop over `eval/scenarios/*`)
  6. Generate aggregated report
  7. Upload results artifact (90-day retention)
  8. Upload report artifact (90-day retention)

**Key Features:**
- Loops over all scenario directories
- Continues on scenario failure (`|| true`)
- Always uploads artifacts (`if: always()`)
- Separate artifacts for full results and report
- Artifacts numbered by run: `eval-results-${{ github.run_number }}`

### Test

**`internal/cli/nightly_eval_test.go`** (126 lines)

Validates workflow YAML structure:
- `TestNightlyEvalWorkflowYML`: comprehensive workflow validation
  - Workflow name and metadata
  - Triggers (schedule and workflow_dispatch)
  - Permissions (contents: read)
  - Job configuration (name, runner, timeout)
  - Required steps present
  - Artifact upload steps have `if: always()`
  - Scenario loop structure
  - Report generation command

## Workflow Design

### Schedule Choice

Runs at 2 AM UTC to avoid peak hours:
- Reduced runner queue times
- Stable performance environment
- Results available by morning for most time zones

### Artifact Strategy

Two artifact uploads:
1. **Full results** (`eval-results-*`): all trial JSON files and report
2. **Report only** (`eval-report-*`): just the markdown report

Retention: 90 days (tracks trends over 3 months)

### Failure Handling

- Individual scenario failures do not stop the workflow
- `|| true` allows partial results
- `if: always()` ensures artifacts upload even on failure
- Timeout prevents runaway scenarios (2 hours total)

### Scenario Loop

```bash
for scenario in eval/scenarios/*; do
  if [ -d "$scenario" ]; then
    echo "Running scenario: $(basename $scenario)"
    ./bin/perfguard eval run --scenario "$scenario" --output eval/results || true
  fi
done
```

Processes each scenario directory:
- Checks if path is a directory
- Logs scenario name for traceability
- Continues on failure

## Workflow Usage

### Automatic Execution

Runs every night at 2 AM UTC without intervention.

### Manual Trigger

From GitHub UI:
1. Navigate to Actions tab
2. Select "Nightly Evaluation" workflow
3. Click "Run workflow"
4. Select branch and run

From GitHub CLI:
```bash
gh workflow run nightly-eval.yml
```

### Viewing Results

Download artifacts:
```bash
gh run list --workflow=nightly-eval.yml
gh run download <run-id>
```

Or from GitHub UI: Actions > workflow run > Artifacts section

## Integration with Evaluation Suite

The workflow uses commands implemented in previous tasks:

**P8-T1 (eval runner):**
- `perfguard eval run --scenario <dir> --output <dir>`
- Runs N trials, writes JSON results per trial

**P8-T5 (metrics and report):**
- `perfguard eval report --output <dir> --scenarios <glob> --report <path>`
- Aggregates results, computes metrics, generates markdown

**P8-T2, P8-T3, P8-T4 (scenarios):**
- 8 scenarios in `eval/scenarios/`
- Each runs independently
- Ground truth in `scenario.yaml`

## Expected Runtime

Approximate times per scenario (20 trials):
- Negative scenarios: 5-10 minutes
- Positive scenarios: 10-15 minutes
- Noisy scenario: 15-20 minutes

Total runtime: 60-90 minutes for all 8 scenarios

## Verification

Test validates:
- YAML parses correctly
- All required triggers present
- Correct permissions (read-only)
- Job configured properly
- All steps present
- Artifact uploads have `if: always()`
- Scenario loop and report generation commands correct

## Next Steps

**P8-T7:** Review default thresholds based on results
- Run nightly eval for 1-2 weeks
- Collect detection rates and false positive rates
- Adjust `min_effect`, `significance`, `correction` if needed
- Document decision in ARCHITECTURE

## Notes

- Workflow continues on scenario failure to maximize data collection
- 90-day retention allows quarterly trend analysis
- Manual trigger enables ad-hoc testing before releases
- Read-only permissions follow least privilege principle
- Separate report artifact for quick access without downloading full results
