package report

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/kyyril/perfguard/internal/model"
)

// RenderJSON serializes cmp to JSON with 2-space indentation and writes to w.
func RenderJSON(w io.Writer, cmp model.Comparison) error {
	data, err := json.MarshalIndent(cmp, "", "  ")
	if err != nil {
		return fmt.Errorf("report: marshal json: %w", err)
	}
	if _, err := w.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("report: write json: %w", err)
	}
	return nil
}
