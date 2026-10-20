package runner

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ExecSpec defines runtime parameters for a single benchmark process execution.
type ExecSpec struct {
	BenchRegex string        // regex for -test.bench (default ".")
	Benchtime  string        // e.g. "1s" or "100x"
	CPU        int           // GOMAXPROCS (0 means system default)
	Timeout    time.Duration // process timeout
	ExtraEnv   []string      // custom non-secret env vars
}

// RawOutput contains stdout, stderr, and elapsed wall-clock duration of an execution.
type RawOutput struct {
	Stdout   []byte
	Stderr   []byte
	Duration time.Duration
}

// Execute runs the test binary once for a single sample (-test.count=1).
// Execution is sequential and isolated in its own process.
func (r *Runner) Execute(ctx context.Context, bin TestBinary, spec ExecSpec) (RawOutput, error) {
	if spec.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, spec.Timeout)
		defer cancel()
	}

	benchRegex := spec.BenchRegex
	if benchRegex == "" {
		benchRegex = "."
	}

	args := []string{
		"-test.run=^$",
		"-test.bench=" + benchRegex,
		"-test.count=1",
		"-test.benchmem",
	}

	if spec.Benchtime != "" {
		args = append(args, "-test.benchtime="+spec.Benchtime)
	}
	if spec.CPU > 0 {
		args = append(args, fmt.Sprintf("-test.cpu=%d", spec.CPU))
	}

	// #nosec G204 -- bin.Path is the compiled test binary.
	cmd := exec.CommandContext(ctx, bin.Path, args...)
	cmd.Dir = bin.Dir
	cmd.Env = buildEnvironment(spec.CPU, spec.ExtraEnv)
	cmd.WaitDelay = 3 * time.Second
	configureProcessGroup(cmd)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	raw := RawOutput{
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
		Duration: duration,
	}

	if err != nil {
		if ctx.Err() != nil {
			return raw, &ExecutionError{
				Err:    fmt.Errorf("runner: process execution cancelled or timed out: %w", ErrExecution),
				Output: fmt.Sprintf("context error: %v; stderr: %s", ctx.Err(), stderr.String()),
			}
		}

		out := strings.TrimSpace(stderr.String())
		if out == "" {
			out = strings.TrimSpace(stdout.String())
		}
		return raw, &ExecutionError{
			Err:    fmt.Errorf("runner: benchmark process exited with error: %w", ErrExecution),
			Output: out,
		}
	}

	return raw, nil
}
