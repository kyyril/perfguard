package builder

import "strings"

// Build constructs a message from parts efficiently using strings.Builder.
func Build(parts []string) string {
	var b strings.Builder
	for _, p := range parts {
		b.WriteString(p)
	}
	return b.String()
}
