package ghci

import (
	"errors"
	"fmt"
	"os"

	"github.com/kyyril/perfguard/internal/model"
	"github.com/kyyril/perfguard/internal/report"
)

// EnvStepSummary is the environment variable pointing to the step summary file.
const EnvStepSummary = "GITHUB_STEP_SUMMARY"

// WriteStepSummary writes the markdown representation of cmp to the file specified
// by GITHUB_STEP_SUMMARY. If the environment variable is not set or empty, it returns nil.
func WriteStepSummary(cmp model.Comparison) error {
	summaryPath := os.Getenv(EnvStepSummary)
	if summaryPath == "" {
		return nil
	}
	return WriteStepSummaryFile(summaryPath, cmp)
}

// WriteStepSummaryFile appends the markdown report of cmp to the file at path.
// It creates the file if it does not exist.
func WriteStepSummaryFile(path string, cmp model.Comparison) error {
	if path == "" {
		return errors.New("ghci: step summary path is required")
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // path comes from GITHUB_STEP_SUMMARY env var set by GitHub Actions runner
	if err != nil {
		return fmt.Errorf("ghci: open step summary file: %w", err)
	}
	defer f.Close()

	if err := report.RenderMD(f, cmp); err != nil {
		return fmt.Errorf("ghci: render step summary: %w", err)
	}

	if _, err := f.WriteString("\n"); err != nil {
		return fmt.Errorf("ghci: write step summary newline: %w", err)
	}

	return nil
}
