package runner_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kyyril/perfguard/internal/runner"
)

func TestExecutionError(t *testing.T) {
	inner := errors.New("underlying compiler error")
	execErr := &runner.ExecutionError{
		Err:    inner,
		Output: "undefined: Foo",
	}

	if !errors.Is(execErr, runner.ErrExecution) {
		t.Error("expected ExecutionError to match ErrExecution")
	}
	if !errors.Is(execErr, inner) {
		t.Error("expected ExecutionError to unwrap to inner error")
	}
	if execErr.ExitCode() != 3 {
		t.Errorf("expected exit code 3, got %d", execErr.ExitCode())
	}
	if execErr.Error() != "underlying compiler error: undefined: Foo" {
		t.Errorf("unexpected error string: %s", execErr.Error())
	}
}

func TestTestBinaryRemove(t *testing.T) {
	tmpDir := t.TempDir()
	dummyFile := filepath.Join(tmpDir, "dummy_test_bin")
	if err := os.WriteFile(dummyFile, []byte("binary data"), 0o600); err != nil {
		t.Fatalf("failed to write dummy file: %v", err)
	}

	bin := runner.TestBinary{
		Package: "example.com/dummy",
		Path:    dummyFile,
		Dir:     tmpDir,
	}

	if err := bin.Remove(); err != nil {
		t.Fatalf("failed to remove binary: %v", err)
	}

	if _, err := os.Stat(dummyFile); !os.IsNotExist(err) {
		t.Errorf("expected file %s to be removed, stat err: %v", dummyFile, err)
	}

	// Second remove should be a no-op without error
	if err := bin.Remove(); err != nil {
		t.Errorf("second remove returned unexpected error: %v", err)
	}
}
