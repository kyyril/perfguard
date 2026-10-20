package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kyyril/perfguard/internal/config"
	"github.com/kyyril/perfguard/internal/envinfo"
	"github.com/kyyril/perfguard/internal/gitx"
	"github.com/kyyril/perfguard/internal/model"
	"github.com/kyyril/perfguard/internal/runner"
)

// RunOptions specifies options for executing a single benchmark run.
type RunOptions struct {
	Config    *config.Config
	WorkDir   string     // repository working directory
	Side      model.Side // "base" or "head" (default: "head")
	OutputDir string     // destination file or directory for result.json
	Runner    BenchRunner
	Git       VCS
}

// Run executes benchmarks for a single target and records result.json.
func Run(ctx context.Context, opts RunOptions) (model.Run, string, error) {
	workDir := opts.WorkDir
	if workDir == "" {
		workDir = "."
	}

	cfg := opts.Config
	if cfg == nil {
		cfg = config.Default()
	}

	side := opts.Side
	if side == "" {
		side = model.SideHead
	}

	r := opts.Runner
	if r == nil {
		r = runner.New()
	}

	gitClient := opts.Git
	if gitClient == nil {
		gitClient = &gitAdapter{client: gitx.New(workDir)}
	}

	runID := fmt.Sprintf("run-%d", time.Now().UnixNano())

	// 1. Git metadata
	gitInfo, err := gitClient.CurrentInfo(ctx)
	if err != nil {
		gitInfo = model.GitInfo{
			SHA:   "unknown",
			Ref:   "HEAD",
			Dirty: false,
		}
	}

	// 2. Hardware and environment fingerprint
	env := envinfo.Fingerprint()

	// 3. Build test binaries
	binaries, err := r.Build(ctx, runner.BuildSpec{
		WorkDir:  workDir,
		Packages: cfg.Benchmarks.Packages,
	})
	if err != nil {
		return model.Run{}, "", fmt.Errorf("app: build benchmark binaries: %w", err)
	}
	defer func() {
		for _, b := range binaries {
			_ = b.Remove()
		}
	}()

	// 4. Execute warmup and measurement rounds
	timeout, _ := time.ParseDuration(cfg.Benchmarks.Timeout)
	if timeout <= 0 {
		timeout = 30 * time.Minute
	}

	roundSpec := runner.RoundSpec{
		Count:      cfg.Benchmarks.Count,
		WarmupRuns: cfg.Execution.WarmupRuns,
		Interleave: false,
		ExecSpec: runner.ExecSpec{
			BenchRegex: cfg.Benchmarks.Include,
			Benchtime:  cfg.Benchmarks.Benchtime,
			CPU:        cfg.Benchmarks.CPU,
			Timeout:    timeout,
		},
	}

	// Assign binaries to the requested side
	if side == model.SideBase {
		roundSpec.BaseBinaries = binaries
	} else {
		roundSpec.HeadBinaries = binaries
	}

	results, err := r.RunRounds(ctx, roundSpec)
	if err != nil {
		return model.Run{}, "", fmt.Errorf("app: execute benchmark rounds: %w", err)
	}

	var benchmarks []model.BenchmarkResult
	if side == model.SideBase {
		benchmarks = results.BaseResults
	} else {
		benchmarks = results.HeadResults
	}
	if benchmarks == nil {
		benchmarks = []model.BenchmarkResult{}
	}

	// 5. Construct Run result
	runResult := model.Run{
		SchemaVersion: model.CurrentSchemaVersion,
		ID:            runID,
		Side:          side,
		CreatedAt:     time.Now().UTC(),
		Git:           gitInfo,
		Env:           env,
		Params: model.RunParams{
			Count:          cfg.Benchmarks.Count,
			Benchtime:      cfg.Benchmarks.Benchtime,
			CPU:            cfg.Benchmarks.CPU,
			Include:        cfg.Benchmarks.Include,
			Exclude:        cfg.Benchmarks.Exclude,
			Mode:           cfg.Execution.Mode,
			WarmupRuns:     cfg.Execution.WarmupRuns,
			MaxExtraRuns:   cfg.Execution.MaxExtraRuns,
			ExtraRoundStep: cfg.Execution.ExtraRoundStep,
		},
		Benchmarks: benchmarks,
	}

	// 6. Write result.json
	outFile, err := writeResultJSON(opts.OutputDir, runID, runResult)
	if err != nil {
		return runResult, "", fmt.Errorf("app: save result.json: %w", err)
	}

	return runResult, outFile, nil
}

func writeResultJSON(outDest, runID string, result model.Run) (string, error) {
	var targetFile string
	switch {
	case outDest == "":
		targetFile = filepath.Join(config.DefaultOutputDir, "runs", runID, "result.json")
	case strings.HasSuffix(strings.ToLower(outDest), ".json"):
		targetFile = outDest
	case outDest == config.DefaultOutputDir:
		targetFile = filepath.Join(outDest, "runs", runID, "result.json")
	case filepath.Base(outDest) == "runs":
		targetFile = filepath.Join(outDest, runID, "result.json")
	default:
		targetFile = filepath.Join(outDest, "result.json")
	}

	if err := os.MkdirAll(filepath.Dir(targetFile), 0o750); err != nil {
		return "", fmt.Errorf("create output directory: %w", err)
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal result json: %w", err)
	}

	tmpFile := targetFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0o600); err != nil {
		return "", fmt.Errorf("write temporary result file: %w", err)
	}

	if err := os.Rename(tmpFile, targetFile); err != nil {
		_ = os.Remove(tmpFile)
		return "", fmt.Errorf("commit result file: %w", err)
	}

	return targetFile, nil
}
