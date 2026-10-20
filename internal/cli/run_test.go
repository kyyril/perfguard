package cli_test

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/kyyril/perfguard/internal/cli"
)

func TestRunCommand_InvalidSide(t *testing.T) {
	t.Parallel()

	cmd := cli.NewRootCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"run", "--side", "invalid-side"})

	err := cmd.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("expected error on invalid side, got nil")
	}

	if cli.ExitCode(err) != cli.ExitConfig {
		t.Errorf("expected exit code %d (ExitConfig), got %d", cli.ExitConfig, cli.ExitCode(err))
	}
}

func TestRunCommand_ExplicitMissingConfig(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	missingConfig := filepath.Join(tmpDir, "does-not-exist.yaml")

	cmd := cli.NewRootCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"run", "--config", missingConfig})

	err := cmd.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("expected error on missing explicit config, got nil")
	}

	if cli.ExitCode(err) != cli.ExitConfig {
		t.Errorf("expected exit code %d (ExitConfig), got %d", cli.ExitConfig, cli.ExitCode(err))
	}
}

func TestRunCommand_Help(t *testing.T) {
	t.Parallel()

	cmd := cli.NewRootCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"run", "--help"})

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("run --help failed: %v", err)
	}

	if !bytes.Contains(buf.Bytes(), []byte("Run benchmarks")) {
		t.Errorf("expected help output to mention 'Run benchmarks', got: %s", buf.String())
	}
}
