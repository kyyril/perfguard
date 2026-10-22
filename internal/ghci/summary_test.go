package ghci_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kyyril/perfguard/internal/ghci"
	"github.com/kyyril/perfguard/internal/model"
	"github.com/kyyril/perfguard/internal/report"
)

func makeTestComparison() model.Comparison {
	return model.Comparison{
		SchemaVersion: model.CurrentSchemaVersion,
		Base: model.RunSummary{
			ID:        "run-base",
			Side:      model.SideBase,
			CreatedAt: time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC),
			Git: model.GitInfo{
				SHA: "1111111222222233333334444444555555566666",
				Ref: "main",
			},
		},
		Head: model.RunSummary{
			ID:        "run-head",
			Side:      model.SideHead,
			CreatedAt: time.Date(2026, 10, 20, 10, 5, 0, 0, time.UTC),
			Git: model.GitInfo{
				SHA: "7777777888888899999990000000111111122222",
				Ref: "feat/perf",
			},
		},
		Verdict: model.VerdictPass,
		Items: []model.MetricComparison{
			{
				Benchmark:  "BenchmarkSummary",
				Metric:     "sec/op",
				BaseMedian: 50.0,
				HeadMedian: 50.5,
				DeltaPct:   1.0,
				PValue:     0.5000,
				Status:     model.StatusUnchanged,
				Reason:     model.ReasonNotSignificant,
			},
		},
	}
}

func TestWriteStepSummaryFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	summaryPath := filepath.Join(tmpDir, "step_summary.md")

	cmp := makeTestComparison()
	err := ghci.WriteStepSummaryFile(summaryPath, cmp)
	if err != nil {
		t.Fatalf("WriteStepSummaryFile failed: %v", err)
	}

	content, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("failed reading step summary: %v", err)
	}

	var expected bytes.Buffer
	if err := report.RenderMD(&expected, cmp); err != nil {
		t.Fatalf("RenderMD failed: %v", err)
	}

	if !strings.Contains(string(content), expected.String()) {
		t.Errorf("expected summary to contain RenderMD output.\nGot:\n%s\nWant:\n%s",
			string(content), expected.String())
	}

	// Verify hidden marker is NOT present in standard step summary
	if strings.Contains(string(content), report.CommentMarker) {
		t.Errorf("step summary should not contain comment marker %q", report.CommentMarker)
	}
}

func TestWriteStepSummaryFile_Append(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	summaryPath := filepath.Join(tmpDir, "step_summary.md")

	cmp := makeTestComparison()

	// First write
	if err := ghci.WriteStepSummaryFile(summaryPath, cmp); err != nil {
		t.Fatalf("first write failed: %v", err)
	}

	// Second write (append)
	if err := ghci.WriteStepSummaryFile(summaryPath, cmp); err != nil {
		t.Fatalf("second write failed: %v", err)
	}

	content, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("failed reading step summary: %v", err)
	}

	count := strings.Count(string(content), "## PerfGuard: PASS")
	if count != 2 {
		t.Errorf("expected 2 headers after appending, got %d", count)
	}
}

func TestWriteStepSummaryFile_EmptyPath(t *testing.T) {
	t.Parallel()

	cmp := makeTestComparison()
	err := ghci.WriteStepSummaryFile("", cmp)
	if err == nil {
		t.Fatal("expected error on empty path, got nil")
	}
}

func TestWriteStepSummary_EnvUnset(t *testing.T) {
	t.Setenv(ghci.EnvStepSummary, "")

	cmp := makeTestComparison()
	err := ghci.WriteStepSummary(cmp)
	if err != nil {
		t.Fatalf("expected nil when env var is unset, got %v", err)
	}
}

func TestWriteStepSummary_EnvSet(t *testing.T) {
	tmpDir := t.TempDir()
	summaryPath := filepath.Join(tmpDir, "env_step_summary.md")
	t.Setenv(ghci.EnvStepSummary, summaryPath)

	cmp := makeTestComparison()
	err := ghci.WriteStepSummary(cmp)
	if err != nil {
		t.Fatalf("expected write to succeed, got %v", err)
	}

	content, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("failed reading summary file: %v", err)
	}

	if !strings.Contains(string(content), "## PerfGuard: PASS") {
		t.Errorf("expected content in summary file, got %s", string(content))
	}
}
