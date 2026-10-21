package report

import (
	"fmt"
	"io"

	"github.com/kyyril/perfguard/internal/model"
)

// Format defines the output report serialization format.
type Format string

const (
	FormatText   Format = "text"
	FormatJSON   Format = "json"
	FormatMD     Format = "md"
	FormatGitHub Format = "github"
)

// Render writes a formatted representation of cmp to w in the specified format.
func Render(w io.Writer, cmp model.Comparison, format Format) error {
	switch format {
	case FormatText, "":
		return RenderText(w, cmp)
	case FormatJSON:
		return RenderJSON(w, cmp)
	case FormatMD:
		return RenderMD(w, cmp)
	case FormatGitHub:
		return RenderGitHub(w, cmp)
	default:
		return fmt.Errorf("report: unsupported format %q", format)
	}
}
