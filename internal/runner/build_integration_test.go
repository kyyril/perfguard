//go:build integration

package runner_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kyyril/perfguard/internal/runner"
)

func TestIntegrationBuildFixture(t *testing.T) {
	ctx := context.Background()
	fixtureDir, err := filepath.Abs("../../testdata/fixtures/benchmod")
	if err != nil {
		t.Fatalf("failed to resolve fixture directory: %v", err)
	}

	r := runner.New()

	// 1. Build packages with tests and without tests
	t.Run("BuildAndSkipNoTests", func(t *testing.T) {
		outDir := t.TempDir()
		spec := runner.BuildSpec{
			WorkDir:  fixtureDir,
			Packages: []string{"./pkgwithtests", "./pkgnotests"},
			OutDir:   outDir,
		}

		binaries, err := r.Build(ctx, spec)
		if err != nil {
			t.Fatalf("expected build to succeed, got: %v", err)
		}

		// Exactly 1 binary should be produced (pkgnotests skipped)
		if len(binaries) != 1 {
			t.Fatalf("expected 1 binary (pkgnotests skipped), got %d", len(binaries))
		}

		bin := binaries[0]
		if bin.Package != "example.com/benchmod/pkgwithtests" {
			t.Errorf("expected package example.com/benchmod/pkgwithtests, got %s", bin.Package)
		}

		if _, err := os.Stat(bin.Path); err != nil {
			t.Errorf("binary does not exist at %s: %v", bin.Path, err)
		}

		// Verify binary cleanup
		if err := bin.Remove(); err != nil {
			t.Fatalf("failed to remove binary: %v", err)
		}
		if _, err := os.Stat(bin.Path); !os.IsNotExist(err) {
			t.Errorf("expected binary %s to be removed, got err: %v", bin.Path, err)
		}
	})

	// 2. Build failure mapping to ErrExecution
	t.Run("BuildFailureMapsToErrExecution", func(t *testing.T) {
		outDir := t.TempDir()
		spec := runner.BuildSpec{
			WorkDir:  fixtureDir,
			Packages: []string{"./pkgbroken"},
			OutDir:   outDir,
		}

		_, err := r.Build(ctx, spec)
		if err == nil {
			t.Fatal("expected build to fail for pkgbroken, got nil")
		}

		if !errors.Is(err, runner.ErrExecution) {
			t.Errorf("expected error to match runner.ErrExecution, got: %v", err)
		}

		var coder interface{ ExitCode() int }
		if errors.As(err, &coder) {
			if coder.ExitCode() != 3 {
				t.Errorf("expected exit code 3, got %d", coder.ExitCode())
			}
		} else {
			t.Error("expected error to implement ErrorCoder interface")
		}
	})
}
