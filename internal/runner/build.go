package runner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type pkgInfo struct {
	importPath string
	dir        string
	hasTests   bool
}

// Build compiles a test binary for each package containing test files.
// Packages without test files are skipped.
// If compilation fails, an ExecutionError wrapping ErrExecution is returned.
func (r *Runner) Build(ctx context.Context, spec BuildSpec) ([]TestBinary, error) {
	workDir := spec.WorkDir
	if workDir == "" {
		workDir = "."
	}
	absWorkDir, err := filepath.Abs(workDir)
	if err != nil {
		return nil, fmt.Errorf("runner: resolve work dir: %w", err)
	}

	packages := spec.Packages
	if len(packages) == 0 {
		packages = []string{"./..."}
	}

	outDir := spec.OutDir
	if outDir == "" {
		tmp, err := os.MkdirTemp("", "perfguard-bin-*")
		if err != nil {
			return nil, fmt.Errorf("runner: create temp output dir: %w", err)
		}
		outDir = tmp
	}
	absOutDir, err := filepath.Abs(outDir)
	if err != nil {
		return nil, fmt.Errorf("runner: resolve output dir: %w", err)
	}

	pkgs, err := r.listPackages(ctx, absWorkDir, packages)
	if err != nil {
		return nil, err
	}

	var binaries []TestBinary
	for _, p := range pkgs {
		if !p.hasTests {
			continue
		}

		binName := sanitizePkgName(p.importPath)
		if runtime.GOOS == "windows" {
			binName += ".exe"
		}
		binPath := filepath.Join(absOutDir, binName)

		if err := r.compilePackage(ctx, p.dir, binPath); err != nil {
			for _, b := range binaries {
				_ = b.Remove()
			}
			return nil, err
		}

		binaries = append(binaries, TestBinary{
			Package: p.importPath,
			Path:    binPath,
			Dir:     p.dir,
		})
	}

	return binaries, nil
}

func (r *Runner) listPackages(ctx context.Context, workDir string, patterns []string) ([]pkgInfo, error) {
	args := append([]string{"list", "-f", "{{.ImportPath}}|{{.Dir}}|{{len .TestGoFiles}}|{{len .XTestGoFiles}}"}, patterns...)
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = workDir
	cmd.Env = os.Environ()
	cmd.WaitDelay = 5 * time.Second

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		out := strings.TrimSpace(stderr.String())
		if out == "" {
			out = strings.TrimSpace(stdout.String())
		}
		return nil, &ExecutionError{
			Err:    fmt.Errorf("runner: package listing failed: %w", ErrExecution),
			Output: out,
		}
	}

	var results []pkgInfo
	lines := strings.Split(stdout.String(), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 4 {
			continue
		}
		tCount, _ := strconv.Atoi(parts[2])
		xtCount, _ := strconv.Atoi(parts[3])
		results = append(results, pkgInfo{
			importPath: parts[0],
			dir:        parts[1],
			hasTests:   (tCount + xtCount) > 0,
		})
	}

	return results, nil
}

func (r *Runner) compilePackage(ctx context.Context, pkgDir string, binPath string) error {
	cmd := exec.CommandContext(ctx, "go", "test", "-c", "-o", binPath, ".")
	cmd.Dir = pkgDir
	cmd.Env = os.Environ()
	cmd.WaitDelay = 15 * time.Second

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		out := strings.TrimSpace(stderr.String())
		if out == "" {
			out = strings.TrimSpace(stdout.String())
		}
		return &ExecutionError{
			Err:    fmt.Errorf("runner: build failed for package in %s: %w", pkgDir, ErrExecution),
			Output: out,
		}
	}

	return nil
}

func sanitizePkgName(s string) string {
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	s = strings.ReplaceAll(s, ".", "_")
	s = strings.ReplaceAll(s, ":", "_")
	return s
}
