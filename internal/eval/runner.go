package eval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kyyril/perfguard/internal/envinfo"
	"github.com/kyyril/perfguard/internal/model"
)

// TrialResult contains the raw outcome of one trial execution.
type TrialResult struct {
	Trial      int               `json:"trial"`
	Verdict    model.Verdict     `json:"verdict,omitempty"`
	DurationMS int64             `json:"duration_ms"`
	Comparison *model.Comparison `json:"comparison,omitempty"`
	Error      string            `json:"error,omitempty"`
}

// RunResult aggregates all trials of one scenario.
type RunResult struct {
	ScenarioID  string            `json:"scenario_id"`
	Trials      []TrialResult     `json:"trials"`
	Environment model.Environment `json:"environment"`
	StartedAt   time.Time         `json:"started_at"`
	FinishedAt  time.Time         `json:"finished_at"`
}

// Workspace is an isolated repository prepared for one trial.
type Workspace interface {
	// Dir returns the repository root of the prepared workspace.
	Dir() string
	// Close removes the workspace and its temporary files.
	Close(ctx context.Context) error
}

// WorkspacePreparer creates an isolated repository from a scenario base
// directory with the change patch applied.
type WorkspacePreparer interface {
	Prepare(ctx context.Context, baseDir, patchPath string) (Workspace, error)
}

// CheckRequest describes one check execution inside a trial.
type CheckRequest struct {
	WorkDir     string
	BaseRef     string
	ArtifactDir string
}

// CheckRunner executes a base-to-head comparison in the given workspace.
type CheckRunner interface {
	RunCheck(ctx context.Context, req CheckRequest) (model.Comparison, error)
}

// RunOptions specifies options for running an evaluation scenario.
type RunOptions struct {
	ScenarioDir string
	OutputDir   string
	Preparer    WorkspacePreparer
	Checker     CheckRunner
}

// Run executes every trial of one scenario and records raw results.
// Results are written to <OutputDir>/<scenario_id>.json when OutputDir is set.
func Run(ctx context.Context, opts RunOptions) (RunResult, error) {
	if opts.Preparer == nil {
		return RunResult{}, errors.New("eval: workspace preparer is required")
	}
	if opts.Checker == nil {
		return RunResult{}, errors.New("eval: check runner is required")
	}

	scenario, err := LoadScenario(opts.ScenarioDir)
	if err != nil {
		return RunResult{}, err
	}

	result := RunResult{
		ScenarioID:  scenario.ID,
		Trials:      make([]TrialResult, 0, scenario.Trials),
		Environment: envinfo.Fingerprint(),
		StartedAt:   time.Now().UTC(),
	}

	var runErr error
	for i := 1; i <= scenario.Trials; i++ {
		if err := ctx.Err(); err != nil {
			runErr = err
			break
		}
		result.Trials = append(result.Trials, runTrial(ctx, i, scenario, opts))
	}

	result.FinishedAt = time.Now().UTC()

	if opts.OutputDir != "" {
		if err := writeRunResult(opts.OutputDir, result); err != nil {
			return result, fmt.Errorf("eval: write result: %w", err)
		}
	}

	if runErr != nil {
		return result, runErr
	}
	return result, nil
}

func runTrial(ctx context.Context, trialNum int, scenario Scenario, opts RunOptions) TrialResult {
	start := time.Now()
	tr := TrialResult{Trial: trialNum}

	workspace, err := opts.Preparer.Prepare(ctx, filepath.Join(scenario.Directory, "base"),
		filepath.Join(scenario.Directory, "change.patch"))
	if err != nil {
		tr.Error = fmt.Sprintf("prepare workspace: %v", err)
		tr.DurationMS = time.Since(start).Milliseconds()
		return tr
	}

	cmp, checkErr := opts.Checker.RunCheck(ctx, CheckRequest{
		WorkDir:     workspace.Dir(),
		BaseRef:     "HEAD~1",
		ArtifactDir: trialArtifactDir(opts.OutputDir, scenario.ID, trialNum),
	})

	closeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	err = workspace.Close(closeCtx)
	cancel()
	if err != nil {
		if checkErr == nil {
			tr.Error = fmt.Sprintf("close workspace: %v", err)
		}
	}

	tr.DurationMS = time.Since(start).Milliseconds()

	if checkErr != nil {
		tr.Error = checkErr.Error()
		return tr
	}

	tr.Comparison = &cmp
	tr.Verdict = cmp.Verdict
	return tr
}

func trialArtifactDir(outputDir, scenarioID string, trialNum int) string {
	if outputDir == "" {
		return ""
	}
	return filepath.Join(outputDir, scenarioID, fmt.Sprintf("trial-%d", trialNum))
}

func writeRunResult(outputDir string, result RunResult) error {
	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	targetFile := filepath.Join(outputDir, result.ScenarioID+".json")
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}

	tmpFile := targetFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0o600); err != nil {
		return fmt.Errorf("write temporary result file: %w", err)
	}
	if err := os.Rename(tmpFile, targetFile); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("commit result file: %w", err)
	}

	return nil
}
