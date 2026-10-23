package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/kyyril/perfguard/internal/config"
	"github.com/kyyril/perfguard/internal/eval"
	"github.com/kyyril/perfguard/internal/gitx"
	"github.com/kyyril/perfguard/internal/model"
)

// EvalRunOptions specifies options for running an evaluation scenario.
type EvalRunOptions struct {
	ScenarioDir string
	OutputDir   string
	Config      *config.Config

	// VCS overrides the Git client used for workspace preparation.
	VCS EvalVCS
	// Checker overrides the check executor.
	Checker       eval.CheckRunner
	WorkspaceRoot string
}

// EvalVCS is the Git surface needed to prepare eval workspaces.
type EvalVCS interface {
	InitRepo(ctx context.Context) error
	CommitAll(ctx context.Context, message string) error
	ApplyPatch(ctx context.Context, patchPath string) error
}

// EvalRun executes an eval scenario end to end and persists raw trials.
func EvalRun(ctx context.Context, opts EvalRunOptions) (eval.RunResult, error) {
	preparer := &evalWorkspacePreparer{
		root: opts.WorkspaceRoot,
		vcsFor: func(dir string) EvalVCS {
			if opts.VCS != nil {
				return opts.VCS
			}
			return gitx.New(dir)
		},
	}

	checker := opts.Checker
	if checker == nil {
		cfg := opts.Config
		if cfg == nil {
			cfg = config.Default()
		}
		checker = &evalCheckRunner{cfg: cfg}
	}

	return eval.Run(ctx, eval.RunOptions{
		ScenarioDir: opts.ScenarioDir,
		OutputDir:   opts.OutputDir,
		Preparer:    preparer,
		Checker:     checker,
	})
}

// evalWorkspacePreparer copies a scenario base directory into a temporary
// repository, commits it, then applies and commits the change patch.
type evalWorkspacePreparer struct {
	root   string
	vcsFor func(dir string) EvalVCS
}

func (p *evalWorkspacePreparer) Prepare(ctx context.Context, baseDir, patchPath string) (eval.Workspace, error) {
	root := p.root
	if root == "" {
		root = os.TempDir()
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, fmt.Errorf("create workspace root: %w", err)
	}

	dir, err := os.MkdirTemp(root, "perfguard-eval-*")
	if err != nil {
		return nil, fmt.Errorf("allocate workspace directory: %w", err)
	}

	ws := &evalWorkspace{dir: dir}
	cleanup := func() {
		_ = ws.Close(context.Background())
	}

	if err := copyTree(baseDir, dir); err != nil {
		cleanup()
		return nil, fmt.Errorf("copy scenario base: %w", err)
	}

	vcs := p.vcsFor(dir)
	if err := vcs.InitRepo(ctx); err != nil {
		cleanup()
		return nil, err
	}
	if err := vcs.CommitAll(ctx, "base: scenario source"); err != nil {
		cleanup()
		return nil, err
	}
	if err := vcs.ApplyPatch(ctx, patchPath); err != nil {
		cleanup()
		return nil, err
	}
	if err := vcs.CommitAll(ctx, "head: scenario change"); err != nil {
		cleanup()
		return nil, err
	}

	return ws, nil
}

type evalWorkspace struct {
	dir string
}

func (w *evalWorkspace) Dir() string { return w.dir }

func (w *evalWorkspace) Close(_ context.Context) error {
	// Test directory trees can exceed Windows path limits; a failed removal
	// must not mask the trial result.
	return os.RemoveAll(w.dir)
}

// evalCheckRunner adapts app.Check to the eval.CheckRunner port.
type evalCheckRunner struct {
	cfg *config.Config
}

func (r *evalCheckRunner) RunCheck(ctx context.Context, req eval.CheckRequest) (model.Comparison, error) {
	res, err := Check(ctx, CheckOptions{
		Config:    r.cfg,
		WorkDir:   req.WorkDir,
		BaseRef:   req.BaseRef,
		OutputDir: req.ArtifactDir,
		Mode:      "same-runner",
	})
	if err != nil {
		return model.Comparison{}, err
	}
	return res.Comparison, nil
}

func copyTree(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat %s: %w", src, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", src)
	}

	return filepath.WalkDir(src, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			if rel == "." {
				return nil
			}
			return os.MkdirAll(target, 0o750)
		}

		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer func() {
		// Read-only source; close errors do not affect the copy.
		_ = in.Close()
	}()

	info, err := in.Stat()
	if err != nil {
		return fmt.Errorf("stat %s: %w", src, err)
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return fmt.Errorf("copy %s: %w", src, err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("close %s: %w", dst, err)
	}
	return nil
}

var _ EvalVCS = (*gitx.Client)(nil)

var _ = errors.New
