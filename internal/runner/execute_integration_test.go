//go:build integration

package runner_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kyyril/perfguard/internal/runner"
)

func TestIntegrationExecute(t *testing.T) {
	ctx := context.Background()
	fixtureDir, err := filepath.Abs("../../testdata/fixtures/benchmod")
	if err != nil {
		t.Fatalf("failed to resolve fixture directory: %v", err)
	}

	r := runner.New()

	// 1. Build pkgwithtests and pkgpanic
	outDir := t.TempDir()
	binaries, err := r.Build(ctx, runner.BuildSpec{
		WorkDir:  fixtureDir,
		Packages: []string{"./pkgwithtests", "./pkgpanic"},
		OutDir:   outDir,
	})
	if err != nil {
		t.Fatalf("failed to build test binaries: %v", err)
	}
	defer func() {
		for _, b := range binaries {
			_ = b.Remove()
		}
	}()

	var validBin, panicBin runner.TestBinary
	for _, b := range binaries {
		if strings.Contains(b.Package, "pkgwithtests") {
			validBin = b
		}
		if strings.Contains(b.Package, "pkgpanic") {
			panicBin = b
		}
	}

	if validBin.Path == "" || panicBin.Path == "" {
		t.Fatalf("expected both binaries to be built, got %+v", binaries)
	}

	// 2. Successful execution of valid benchmark
	t.Run("ExecuteValidBenchmark", func(t *testing.T) {
		t.Setenv("SECRET_LEAK_TOKEN", "should_not_forward")

		raw, err := r.Execute(ctx, validBin, runner.ExecSpec{
			BenchRegex: "BenchmarkFast",
			Benchtime:  "10x",
			CPU:        2,
		})
		if err != nil {
			t.Fatalf("expected execution to succeed, got: %v\nOutput: %s", err, raw.Stderr)
		}

		stdout := string(raw.Stdout)
		if !strings.Contains(stdout, "BenchmarkFast") {
			t.Errorf("expected stdout to contain BenchmarkFast, got: %s", stdout)
		}
		if !strings.Contains(stdout, "ns/op") {
			t.Errorf("expected stdout to contain ns/op, got: %s", stdout)
		}
		if raw.Duration <= 0 {
			t.Errorf("expected positive duration, got %v", raw.Duration)
		}
	})

	// 3. Panicking benchmark handled with ErrExecution
	t.Run("PanickingBenchmarkProducesErrExecution", func(t *testing.T) {
		_, err := r.Execute(ctx, panicBin, runner.ExecSpec{
			BenchRegex: "BenchmarkPanicking",
			Benchtime:  "10x",
		})
		if err == nil {
			t.Fatal("expected panicking benchmark to return error, got nil")
		}

		if !errors.Is(err, runner.ErrExecution) {
			t.Errorf("expected error to match ErrExecution, got: %v", err)
		}

		var coder interface{ ExitCode() int }
		if errors.As(err, &coder) {
			if coder.ExitCode() != 3 {
				t.Errorf("expected exit code 3, got %d", coder.ExitCode())
			}
		} else {
			t.Error("expected error to implement ErrorCoder")
		}
	})

	// 4. Timeout stops child process and returns ErrExecution
	t.Run("TimeoutStopsProcess", func(t *testing.T) {
		timeoutCtx, cancel := context.WithTimeout(ctx, 1*time.Millisecond)
		defer cancel()

		time.Sleep(2 * time.Millisecond) // ensure timeout already expired

		_, err := r.Execute(timeoutCtx, validBin, runner.ExecSpec{
			BenchRegex: "BenchmarkFast",
			Benchtime:  "10000000x",
		})
		if err == nil {
			t.Fatal("expected timed out execution to fail, got nil")
		}

		if !errors.Is(err, runner.ErrExecution) {
			t.Errorf("expected error to match ErrExecution on timeout, got: %v", err)
		}
	})
}
