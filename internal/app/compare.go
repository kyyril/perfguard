package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kyyril/perfguard/internal/config"
	"github.com/kyyril/perfguard/internal/decision"
	"github.com/kyyril/perfguard/internal/model"
)

// CompareOptions specifies options for comparing two benchmark runs.
type CompareOptions struct {
	Config         *config.Config
	BaseResultFile string
	HeadResultFile string
	BaseRun        *model.Run
	HeadRun        *model.Run
	OutputDir      string // destination file or directory for comparison.json
}

// Compare compares base and head benchmark runs and saves comparison.json if output is requested.
func Compare(ctx context.Context, opts CompareOptions) (model.Comparison, string, error) {
	select {
	case <-ctx.Done():
		return model.Comparison{}, "", ctx.Err()
	default:
	}

	cfg := opts.Config
	if cfg == nil {
		cfg = config.Default()
	}

	baseRun, err := resolveRun(opts.BaseRun, opts.BaseResultFile, "base")
	if err != nil {
		return model.Comparison{}, "", fmt.Errorf("app: load base run: %w", err)
	}

	headRun, err := resolveRun(opts.HeadRun, opts.HeadResultFile, "head")
	if err != nil {
		return model.Comparison{}, "", fmt.Errorf("app: load head run: %w", err)
	}

	cmp, err := decision.CompareRuns(*baseRun, *headRun, cfg)
	if err != nil {
		return model.Comparison{}, "", fmt.Errorf("app: compare runs: %w", err)
	}

	var outFile string
	if opts.OutputDir != "" {
		written, err := writeComparisonJSON(opts.OutputDir, cmp)
		if err != nil {
			return cmp, "", fmt.Errorf("app: save comparison.json: %w", err)
		}
		outFile = written
	}

	return cmp, outFile, nil
}

func resolveRun(run *model.Run, filePath, side string) (*model.Run, error) {
	if run != nil {
		if run.SchemaVersion != model.CurrentSchemaVersion {
			return nil, fmt.Errorf("unsupported %s schema version %d (expected %d)",
				side, run.SchemaVersion, model.CurrentSchemaVersion)
		}
		return run, nil
	}

	if filePath == "" {
		return nil, fmt.Errorf("%s run result file or object is required", side)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read %s result file %s: %w", side, filePath, err)
	}

	var loaded model.Run
	if err := json.Unmarshal(data, &loaded); err != nil {
		return nil, fmt.Errorf("parse %s result file %s: %w", side, filePath, err)
	}

	if loaded.SchemaVersion != model.CurrentSchemaVersion {
		return nil, fmt.Errorf("unsupported %s schema version %d in %s (expected %d)",
			side, loaded.SchemaVersion, filePath, model.CurrentSchemaVersion)
	}

	return &loaded, nil
}

func writeComparisonJSON(outDest string, cmp model.Comparison) (string, error) {
	var targetFile string
	switch {
	case strings.HasSuffix(strings.ToLower(outDest), ".json"):
		targetFile = outDest
	default:
		targetFile = filepath.Join(outDest, "comparison.json")
	}

	if err := os.MkdirAll(filepath.Dir(targetFile), 0o750); err != nil {
		return "", fmt.Errorf("create directory: %w", err)
	}

	data, err := json.MarshalIndent(cmp, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal comparison json: %w", err)
	}

	tmpFile := targetFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0o600); err != nil {
		return "", fmt.Errorf("write temporary comparison file: %w", err)
	}

	if err := os.Rename(tmpFile, targetFile); err != nil {
		_ = os.Remove(tmpFile)
		return "", fmt.Errorf("commit comparison file: %w", err)
	}

	return targetFile, nil
}
