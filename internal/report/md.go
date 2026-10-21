package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/kyyril/perfguard/internal/model"
)

// RenderMD writes a markdown formatted report of cmp to w.
func RenderMD(w io.Writer, cmp model.Comparison) error {
	header, rows, notes, diagnosis := renderMarkdownSections(cmp)

	var sb strings.Builder
	sb.WriteString(header)
	for _, r := range rows {
		sb.WriteString(r)
	}
	sb.WriteString(notes)
	sb.WriteString(diagnosis)

	if _, err := io.WriteString(w, sb.String()); err != nil {
		return fmt.Errorf("report: write md report: %w", err)
	}
	return nil
}

func renderMarkdownSections(cmp model.Comparison) (header string, rows []string, notes string, diagnosis string) {
	var hsb strings.Builder

	// 1. Overall verdict
	verdictUpper := strings.ToUpper(string(cmp.Verdict))
	if verdictUpper == "" {
		verdictUpper = "UNKNOWN"
	}
	fmt.Fprintf(&hsb, "## PerfGuard: %s\n\n", verdictUpper)

	// 2. Commit metadata
	baseInfo := formatGitRef(cmp.Base.Git)
	headInfo := formatGitRef(cmp.Head.Git)
	fmt.Fprintf(&hsb, "Base: %s  \n", baseInfo)
	fmt.Fprintf(&hsb, "Head: %s\n\n", headInfo)

	// 3. Table header
	hsb.WriteString("| Benchmark | Metric | Base | Head | Delta | P-Value | Status | Reason |\n")
	hsb.WriteString("|---|---|---|---|---|---|---|---|\n")
	header = hsb.String()

	// 4. Items rows
	rows = make([]string, 0, len(cmp.Items))
	for _, it := range cmp.Items {
		baseVal, headVal, deltaStr, pValStr := formatRowValues(it)
		row := fmt.Sprintf("| %s | %s | %s | %s | %s | %s | %s | %s |\n",
			it.Benchmark,
			it.Metric,
			baseVal,
			headVal,
			deltaStr,
			pValStr,
			it.Status,
			it.Reason,
		)
		rows = append(rows, row)
	}

	// 5. Notes if any
	if len(cmp.Notes) > 0 {
		var nsb strings.Builder
		nsb.WriteString("\n### Notes\n\n")
		for _, note := range cmp.Notes {
			fmt.Fprintf(&nsb, "- %s\n", note)
		}
		notes = nsb.String()
	}

	// 6. Diagnosis if any
	if len(cmp.Diagnosis) > 0 {
		var dsb strings.Builder
		dsb.WriteString("\n### Contributor candidates\n")
		for _, d := range cmp.Diagnosis {
			fmt.Fprintf(&dsb, "\n#### %s (%s)\n\n", d.Benchmark, d.ProfileType)
			dsb.WriteString("| Function | Flat delta | Cum delta | Changed by PR |\n")
			dsb.WriteString("|---|---|---|---|\n")
			for _, c := range d.Candidates {
				changed := "no"
				if c.Touched {
					changed = "yes"
				}
				fmt.Fprintf(&dsb, "| %s | %+.2f | %+.2f | %s |\n",
					c.Function, c.FlatDelta, c.CumDelta, changed)
			}
		}
		diagnosis = dsb.String()
	}

	return header, rows, notes, diagnosis
}
