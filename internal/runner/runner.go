// Package runner manages the lifecycle of benchmark execution, including
// test binary compilation, process orchestration, timeouts, and warmup rounds.
package runner

import (
	"errors"
	"fmt"
	"os"
)

// ErrExecution indicates that a runner process or compilation failed.
var ErrExecution = errors.New("execution error")

// ExecutionError wraps process or build errors with compiler output and maps to exit code 3.
type ExecutionError struct {
	Err    error
	Output string
}

func (e *ExecutionError) Error() string {
	if e.Output != "" {
		return fmt.Sprintf("%v: %s", e.Err, e.Output)
	}
	return e.Err.Error()
}

func (e *ExecutionError) Unwrap() error {
	return e.Err
}

// ExitCode returns 3 for execution errors.
func (e *ExecutionError) ExitCode() int {
	return 3
}

// Is supports errors.Is matching against ErrExecution.
func (e *ExecutionError) Is(target error) bool {
	return target == ErrExecution
}

// BuildSpec describes parameters for building benchmark test binaries.
type BuildSpec struct {
	WorkDir  string   // root directory of the repository or worktree
	Packages []string // package patterns to build, e.g. ["./..."]
	OutDir   string   // target directory for binaries; temporary if empty
}

// TestBinary represents a compiled test binary for a specific package.
type TestBinary struct {
	Package string // import path of the package
	Path    string // absolute path to the compiled binary executable
	Dir     string // directory containing package sources and testdata
}

// Remove cleans up the binary executable from disk.
func (b *TestBinary) Remove() error {
	if b.Path != "" {
		err := os.Remove(b.Path)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("runner: failed to remove test binary %s: %w", b.Path, err)
		}
	}
	return nil
}

// Runner builds and executes benchmark test binaries.
type Runner struct{}

// New creates a new benchmark runner.
func New() *Runner {
	return &Runner{}
}
