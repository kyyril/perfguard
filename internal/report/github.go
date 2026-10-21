package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/kyyril/perfguard/internal/model"
)

// CommentMarker is the hidden HTML comment marker used to identify PerfGuard PR comments.
const CommentMarker = "<!-- perfguard:comment -->"

// MaxGitHubCommentLength is the maximum character limit for a GitHub PR comment.
// GitHub's limit is 65,536 characters; 65,000 gives a safe margin.
const MaxGitHubCommentLength = 65000

// TruncationNotice is appended when a report exceeds the character limit.
const TruncationNotice = "\n\n*Report truncated to fit GitHub comment limit. Full report is available in artifacts.*\n"

// RenderGitHub writes a GitHub PR comment formatted report of cmp to w.
// It includes CommentMarker and truncates long reports with a notice if they exceed MaxGitHubCommentLength.
func RenderGitHub(w io.Writer, cmp model.Comparison) error {
	return RenderGitHubWithLimit(w, cmp, MaxGitHubCommentLength)
}

// RenderGitHubWithLimit writes a GitHub PR comment truncated at maxLen.
func RenderGitHubWithLimit(w io.Writer, cmp model.Comparison, maxLen int) error {
	header, rows, notes, diagnosis := renderMarkdownSections(cmp)

	fullLen := len(CommentMarker) + 2 + len(header) + len(notes) + len(diagnosis)
	for _, r := range rows {
		fullLen += len(r)
	}

	var sb strings.Builder
	sb.WriteString(CommentMarker)
	sb.WriteString("\n\n")

	if fullLen <= maxLen {
		sb.WriteString(header)
		for _, r := range rows {
			sb.WriteString(r)
		}
		sb.WriteString(notes)
		sb.WriteString(diagnosis)
	} else {
		sb.WriteString(header)
		remaining := maxLen - sb.Len() - len(TruncationNotice)

		truncated := false
		for _, r := range rows {
			if len(r) <= remaining {
				sb.WriteString(r)
				remaining -= len(r)
			} else {
				truncated = true
				break
			}
		}

		if !truncated {
			if len(notes) <= remaining {
				sb.WriteString(notes)
				remaining -= len(notes)
				if len(diagnosis) <= remaining {
					sb.WriteString(diagnosis)
				} else {
					truncated = true
				}
			} else {
				truncated = true
			}
		}

		if truncated {
			sb.WriteString(TruncationNotice)
		}
	}

	if _, err := io.WriteString(w, sb.String()); err != nil {
		return fmt.Errorf("report: write github report: %w", err)
	}
	return nil
}
