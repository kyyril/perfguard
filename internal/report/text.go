package report

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/kyyril/perfguard/internal/model"
)

// RenderText writes a human-readable tabular text summary of cmp to w.
func RenderText(w io.Writer, cmp model.Comparison) error {
	var sb strings.Builder

	// 1. Overall verdict
	verdictUpper := strings.ToUpper(string(cmp.Verdict))
	if verdictUpper == "" {
		verdictUpper = "UNKNOWN"
	}
	fmt.Fprintf(&sb, "OVERALL VERDICT: %s\n\n", verdictUpper)

	// 2. Commit metadata
	baseInfo := formatGitRef(cmp.Base.Git)
	headInfo := formatGitRef(cmp.Head.Git)
	fmt.Fprintf(&sb, "Base: %s\n", baseInfo)
	fmt.Fprintf(&sb, "Head: %s\n\n", headInfo)

	// 3. Items table
	tw := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "BENCHMARK\tMETRIC\tBASE\tHEAD\tDELTA\tP-VALUE\tSTATUS\tREASON")

	for _, it := range cmp.Items {
		baseVal, headVal, deltaStr, pValStr := formatRowValues(it)
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			it.Benchmark,
			it.Metric,
			baseVal,
			headVal,
			deltaStr,
			pValStr,
			it.Status,
			it.Reason,
		)
	}
	if err := tw.Flush(); err != nil {
		return fmt.Errorf("report: flush table: %w", err)
	}

	// 4. Notes if any
	if len(cmp.Notes) > 0 {
		sb.WriteString("\nNotes:\n")
		for _, note := range cmp.Notes {
			fmt.Fprintf(&sb, "- %s\n", note)
		}
	}

	// 5. Diagnosis if any
	if len(cmp.Diagnosis) > 0 {
		sb.WriteString("\nDiagnosis (contributor candidates):\n")
		for _, d := range cmp.Diagnosis {
			fmt.Fprintf(&sb, "  %s (%s):\n", d.Benchmark, d.ProfileType)
			for _, c := range d.Candidates {
				touchedMarker := ""
				if c.Touched {
					touchedMarker = " [modified in PR]"
				}
				fmt.Fprintf(&sb, "    - %s (flat: %+.2f, cum: %+.2f)%s\n",
					c.Function, c.FlatDelta, c.CumDelta, touchedMarker)
			}
		}
	}

	if _, err := io.WriteString(w, sb.String()); err != nil {
		return fmt.Errorf("report: write text report: %w", err)
	}
	return nil
}

func formatGitRef(git model.GitInfo) string {
	sha := git.SHA
	if len(sha) > 7 {
		sha = sha[:7]
	}
	if sha == "" {
		sha = "(no sha)"
	}
	ref := git.Ref
	if ref == "" {
		ref = "unknown"
	}
	dirty := ""
	if git.Dirty {
		dirty = " [dirty]"
	}
	return fmt.Sprintf("%s (%s)%s", sha, ref, dirty)
}

func formatRowValues(it model.MetricComparison) (base, head, delta, pVal string) {
	switch it.Status {
	case model.StatusNew:
		return "-", fmt.Sprintf("%.2f", it.HeadMedian), "-", "-"
	case model.StatusRemoved:
		return fmt.Sprintf("%.2f", it.BaseMedian), "-", "-", "-"
	default:
		base = fmt.Sprintf("%.2f", it.BaseMedian)
		head = fmt.Sprintf("%.2f", it.HeadMedian)

		switch {
		case it.DeltaPct > 0:
			delta = fmt.Sprintf("+%.2f%%", it.DeltaPct)
		case it.DeltaPct < 0:
			delta = fmt.Sprintf("%.2f%%", it.DeltaPct)
		default:
			delta = "~0.00%"
		}

		if it.PValue <= 0 && (it.Reason == model.ReasonNoisy || it.Reason == model.ReasonInsufficientSamples) {
			pVal = "-"
		} else {
			pVal = fmt.Sprintf("%.4f", it.PValue)
		}
		return base, head, delta, pVal
	}
}
