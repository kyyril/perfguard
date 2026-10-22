package report_test

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kyyril/perfguard/internal/model"
	"github.com/kyyril/perfguard/internal/report"
)

var updateGolden = flag.Bool("update", false, "update golden test files")

func makeBaseComparison(id string) model.Comparison {
	return model.Comparison{
		SchemaVersion: model.CurrentSchemaVersion,
		Base: model.RunSummary{
			ID:        "run-base-" + id,
			Side:      model.SideBase,
			CreatedAt: time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC),
			Git: model.GitInfo{
				SHA:   "1111111222222233333334444444555555566666",
				Ref:   "origin/main",
				Dirty: false,
			},
			Env: model.Environment{
				OS:         "linux",
				Arch:       "amd64",
				GoVersion:  "go1.23.0",
				NumCPU:     8,
				CPUModel:   "AMD EPYC",
				GOMAXPROCS: 8,
			},
		},
		Head: model.RunSummary{
			ID:        "run-head-" + id,
			Side:      model.SideHead,
			CreatedAt: time.Date(2026, 10, 20, 10, 10, 0, 0, time.UTC),
			Git: model.GitInfo{
				SHA:   "7777777888888899999990000000111111122222",
				Ref:   "feat/improve",
				Dirty: false,
			},
			Env: model.Environment{
				OS:         "linux",
				Arch:       "amd64",
				GoVersion:  "go1.23.0",
				NumCPU:     8,
				CPUModel:   "AMD EPYC",
				GOMAXPROCS: 8,
			},
		},
	}
}

func getScenarios() map[string]model.Comparison {
	scenarios := make(map[string]model.Comparison)

	// 1. Pass
	passCmp := makeBaseComparison("pass")
	passCmp.Verdict = model.VerdictPass
	passCmp.Items = []model.MetricComparison{
		{
			Benchmark:  "BenchmarkFast",
			Metric:     "sec/op",
			BaseMedian: 100.0,
			HeadMedian: 100.5,
			DeltaPct:   0.5,
			CI:         [2]float64{0.1, 0.9},
			PValue:     0.8500,
			CVBase:     1.2,
			CVHead:     1.3,
			NBase:      10,
			NHead:      10,
			Status:     model.StatusUnchanged,
			Reason:     model.ReasonNotSignificant,
		},
	}
	scenarios["pass"] = passCmp

	// 2. Warn
	warnCmp := makeBaseComparison("warn")
	warnCmp.Verdict = model.VerdictWarn
	warnCmp.Items = []model.MetricComparison{
		{
			Benchmark:  "BenchmarkBuffer",
			Metric:     "sec/op",
			BaseMedian: 200.0,
			HeadMedian: 213.0,
			DeltaPct:   6.5,
			CI:         [2]float64{5.0, 8.0},
			PValue:     0.0050,
			CVBase:     2.0,
			CVHead:     2.1,
			NBase:      10,
			NHead:      10,
			Status:     model.StatusWarn,
			Reason:     model.ReasonExceedsWarn,
		},
	}
	scenarios["warn"] = warnCmp

	// 3. Fail
	failCmp := makeBaseComparison("fail")
	failCmp.Verdict = model.VerdictFail
	failCmp.Items = []model.MetricComparison{
		{
			Benchmark:  "BenchmarkEncode",
			Metric:     "sec/op",
			BaseMedian: 100.0,
			HeadMedian: 125.0,
			DeltaPct:   25.0,
			CI:         [2]float64{22.0, 28.0},
			PValue:     0.0001,
			CVBase:     1.8,
			CVHead:     2.2,
			NBase:      10,
			NHead:      10,
			Status:     model.StatusFail,
			Reason:     model.ReasonExceedsFail,
		},
	}
	failCmp.Diagnosis = []model.Diagnosis{
		{
			Benchmark:   "BenchmarkEncode",
			ProfileType: "cpu",
			Candidates: []model.Candidate{
				{
					Function:  "example.com/pkg.GrowBuffer",
					FlatDelta: 15.2,
					CumDelta:  22.4,
					Touched:   true,
				},
				{
					Function:  "runtime.mallocgc",
					FlatDelta: 8.5,
					CumDelta:  8.5,
					Touched:   false,
				},
			},
		},
	}
	scenarios["fail"] = failCmp

	// 4. Inconclusive
	inconCmp := makeBaseComparison("inconclusive")
	inconCmp.Verdict = model.VerdictInconclusive
	inconCmp.Items = []model.MetricComparison{
		{
			Benchmark:  "BenchmarkFlaky",
			Metric:     "sec/op",
			BaseMedian: 150.0,
			HeadMedian: 155.0,
			DeltaPct:   3.33,
			CI:         [2]float64{-5.0, 12.0},
			PValue:     0.0,
			CVBase:     14.5,
			CVHead:     16.2,
			NBase:      10,
			NHead:      10,
			Status:     model.StatusInconclusive,
			Reason:     model.ReasonNoisy,
		},
	}
	inconCmp.Notes = []string{
		"CPU model difference: AMD EPYC vs Intel Xeon",
	}
	scenarios["inconclusive"] = inconCmp

	// 5. New
	newCmp := makeBaseComparison("new")
	newCmp.Verdict = model.VerdictPass
	newCmp.Items = []model.MetricComparison{
		{
			Benchmark:  "BenchmarkNewFeature",
			Metric:     "sec/op",
			BaseMedian: 0.0,
			HeadMedian: 50.0,
			DeltaPct:   0.0,
			CI:         [2]float64{0.0, 0.0},
			PValue:     0.0,
			CVBase:     0.0,
			CVHead:     1.5,
			NBase:      0,
			NHead:      10,
			Status:     model.StatusNew,
			Reason:     model.ReasonNewBenchmark,
		},
	}
	scenarios["new"] = newCmp

	// 6. Removed
	removedCmp := makeBaseComparison("removed")
	removedCmp.Verdict = model.VerdictPass
	removedCmp.Items = []model.MetricComparison{
		{
			Benchmark:  "BenchmarkLegacy",
			Metric:     "sec/op",
			BaseMedian: 80.0,
			HeadMedian: 0.0,
			DeltaPct:   0.0,
			CI:         [2]float64{0.0, 0.0},
			PValue:     0.0,
			CVBase:     2.1,
			CVHead:     0.0,
			NBase:      10,
			NHead:      0,
			Status:     model.StatusRemoved,
			Reason:     model.ReasonRemovedBenchmark,
		},
	}
	scenarios["removed"] = removedCmp

	return scenarios
}

func TestRender_Golden(t *testing.T) {
	scenarios := getScenarios()
	testdataDir := filepath.Join("..", "..", "testdata", "report")

	if *updateGolden {
		_ = os.MkdirAll(testdataDir, 0o750)
	}

	formats := []struct {
		name      string
		format    report.Format
		extension string
		renderFn  func(cmp model.Comparison) ([]byte, error)
	}{
		{
			name:      "text",
			format:    report.FormatText,
			extension: ".text.golden",
			renderFn: func(cmp model.Comparison) ([]byte, error) {
				var buf bytes.Buffer
				err := report.RenderText(&buf, cmp)
				return buf.Bytes(), err
			},
		},
		{
			name:      "json",
			format:    report.FormatJSON,
			extension: ".json.golden",
			renderFn: func(cmp model.Comparison) ([]byte, error) {
				var buf bytes.Buffer
				err := report.RenderJSON(&buf, cmp)
				return buf.Bytes(), err
			},
		},
		{
			name:      "md",
			format:    report.FormatMD,
			extension: ".md.golden",
			renderFn: func(cmp model.Comparison) ([]byte, error) {
				var buf bytes.Buffer
				err := report.RenderMD(&buf, cmp)
				return buf.Bytes(), err
			},
		},
		{
			name:      "github",
			format:    report.FormatGitHub,
			extension: ".github.golden",
			renderFn: func(cmp model.Comparison) ([]byte, error) {
				var buf bytes.Buffer
				err := report.RenderGitHub(&buf, cmp)
				return buf.Bytes(), err
			},
		},
	}

	for caseName, cmp := range scenarios {
		caseName := caseName
		cmp := cmp

		for _, fmtSpec := range formats {
			fmtSpec := fmtSpec
			testName := caseName + "_" + fmtSpec.name

			t.Run(testName, func(t *testing.T) {
				actual, err := fmtSpec.renderFn(cmp)
				if err != nil {
					t.Fatalf("render failed: %v", err)
				}

				goldenFile := filepath.Join(testdataDir, caseName+fmtSpec.extension)

				if *updateGolden {
					if err := os.WriteFile(goldenFile, actual, 0o600); err != nil {
						t.Fatalf("failed updating golden file %s: %v", goldenFile, err)
					}
					return
				}

				expected, err := os.ReadFile(goldenFile)
				if err != nil {
					t.Fatalf("failed reading golden file %s: %v (run with -update to create)", goldenFile, err)
				}

				// Normalize line endings for cross-platform comparison
				actualNorm := bytes.ReplaceAll(actual, []byte("\r\n"), []byte("\n"))
				expectedNorm := bytes.ReplaceAll(expected, []byte("\r\n"), []byte("\n"))

				if !bytes.Equal(actualNorm, expectedNorm) {
					t.Errorf("golden mismatch for %s\nGot:\n%s\nWant:\n%s",
						testName, string(actualNorm), string(expectedNorm))
				}
			})
		}
	}
}

func TestRender_UnsupportedFormat(t *testing.T) {
	t.Parallel()

	cmp := makeBaseComparison("unsupported")
	var buf bytes.Buffer
	err := report.Render(&buf, cmp, report.Format("unknown_fmt"))
	if err == nil {
		t.Fatal("expected error on unsupported format, got nil")
	}
}

func TestRender_DefaultFormat(t *testing.T) {
	t.Parallel()

	cmp := makeBaseComparison("default")
	cmp.Verdict = model.VerdictPass
	var buf bytes.Buffer
	err := report.Render(&buf, cmp, "")
	if err != nil {
		t.Fatalf("expected default render to succeed, got %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("OVERALL VERDICT: PASS")) {
		t.Errorf("expected text output on default format, got %s", buf.String())
	}
}

func TestRenderGitHub_Truncation_SmallLimit(t *testing.T) {
	t.Parallel()

	cmp := makeBaseComparison("trunc-small")
	cmp.Verdict = model.VerdictPass
	for i := 0; i < 20; i++ {
		cmp.Items = append(cmp.Items, model.MetricComparison{
			Benchmark:  fmt.Sprintf("BenchmarkItem%03d", i),
			Metric:     "sec/op",
			BaseMedian: 100.0,
			HeadMedian: 100.5,
			DeltaPct:   0.5,
			PValue:     0.8500,
			Status:     model.StatusUnchanged,
			Reason:     model.ReasonNotSignificant,
		})
	}

	var buf bytes.Buffer
	const limit = 600
	err := report.RenderGitHubWithLimit(&buf, cmp, limit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := buf.String()
	if len(content) > limit {
		t.Errorf("output length %d exceeds limit %d", len(content), limit)
	}
	if !strings.HasPrefix(content, report.CommentMarker) {
		t.Errorf("expected comment marker at start, got %s", content[:50])
	}
	if !strings.Contains(content, strings.TrimSpace(report.TruncationNotice)) {
		t.Errorf("expected truncation notice in output")
	}
}

func TestRenderGitHub_Truncation_MaxCommentLength(t *testing.T) {
	t.Parallel()

	cmp := makeBaseComparison("trunc-large")
	cmp.Verdict = model.VerdictFail
	// 2000 items would generate ~200KB of markdown without truncation
	for i := 0; i < 2000; i++ {
		cmp.Items = append(cmp.Items, model.MetricComparison{
			Benchmark:  fmt.Sprintf("BenchmarkVeryLongBenchmarkNameForTruncationTesting%04d", i),
			Metric:     "sec/op",
			BaseMedian: 100.0,
			HeadMedian: 125.0,
			DeltaPct:   25.0,
			PValue:     0.0001,
			Status:     model.StatusFail,
			Reason:     model.ReasonExceedsFail,
		})
	}
	cmp.Notes = []string{"Note that will be evaluated"}
	cmp.Diagnosis = []model.Diagnosis{
		{
			Benchmark:   "BenchmarkVeryLongBenchmarkNameForTruncationTesting0001",
			ProfileType: "cpu",
			Candidates: []model.Candidate{
				{
					Function:  "example.com/pkg.Grow",
					FlatDelta: 10.0,
					CumDelta:  15.0,
					Touched:   true,
				},
			},
		},
	}

	var buf bytes.Buffer
	err := report.RenderGitHub(&buf, cmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := buf.String()
	if len(content) > report.MaxGitHubCommentLength {
		t.Errorf("output length %d exceeds MaxGitHubCommentLength %d", len(content), report.MaxGitHubCommentLength)
	}
	if !strings.HasPrefix(content, report.CommentMarker) {
		t.Errorf("expected comment marker at start")
	}
	if !strings.Contains(content, strings.TrimSpace(report.TruncationNotice)) {
		t.Errorf("expected truncation notice in output")
	}

	// Verify that table rows are intact (valid markdown table)
	lines := strings.Split(content, "\n")
	tableStarted := false
	for _, line := range lines {
		if strings.HasPrefix(line, "| Benchmark |") {
			tableStarted = true
			continue
		}
		if tableStarted {
			if strings.HasPrefix(line, "|") {
				// Must have 8 columns (9 pipe separators)
				pipeCount := strings.Count(line, "|")
				if pipeCount != 9 {
					t.Errorf("broken table row with %d pipes: %s", pipeCount, line)
				}
			} else if line == "" || strings.HasPrefix(line, "*") {
				// Reached end of table or notice
				break
			}
		}
	}
}

func TestDiagnosis_ShortProfileWarningAndNotes(t *testing.T) {
	t.Parallel()

	cmp := makeBaseComparison("short-profile")
	cmp.Verdict = model.VerdictFail
	cmp.Diagnosis = []model.Diagnosis{
		{
			Benchmark:   "BenchmarkShort",
			ProfileType: "cpu",
			Candidates: []model.Candidate{
				{Function: "main.slow", FlatDelta: 5.0, CumDelta: 10.0, Touched: true},
			},
			Notes: []string{
				"warning: profile too short (80 samples, minimum 200)",
			},
		},
	}

	var mdBuf bytes.Buffer
	if err := report.RenderMD(&mdBuf, cmp); err != nil {
		t.Fatalf("RenderMD failed: %v", err)
	}
	mdStr := mdBuf.String()
	if !strings.Contains(mdStr, "warning: profile too short (80 samples, minimum 200)") {
		t.Errorf("expected short profile warning in markdown, got:\n%s", mdStr)
	}
	if !strings.Contains(mdStr, "go tool pprof -diff_base") {
		t.Errorf("expected reproduction command in markdown, got:\n%s", mdStr)
	}
	if !strings.Contains(mdStr, "contribution is not proof of causation") {
		t.Errorf("expected causation disclaimer in markdown, got:\n%s", mdStr)
	}

	var textBuf bytes.Buffer
	if err := report.RenderText(&textBuf, cmp); err != nil {
		t.Fatalf("RenderText failed: %v", err)
	}
	textStr := textBuf.String()
	if !strings.Contains(textStr, "warning: profile too short (80 samples, minimum 200)") {
		t.Errorf("expected short profile warning in text report, got:\n%s", textStr)
	}
	if !strings.Contains(textStr, "go tool pprof -diff_base") {
		t.Errorf("expected reproduction command in text report, got:\n%s", textStr)
	}
}
