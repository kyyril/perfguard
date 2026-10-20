package cli_test

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/kyyril/perfguard/internal/cli"
)

func TestCheckCommand_Help(t *testing.T) {
	t.Parallel()

	cmd := cli.NewRootCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"check", "--help"})

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("check --help failed: %v", err)
	}

	if !bytes.Contains(buf.Bytes(), []byte("--fail-on")) {
		t.Errorf("expected help output to mention '--fail-on', got: %s", buf.String())
	}
	if !bytes.Contains(buf.Bytes(), []byte("--strict")) {
		t.Errorf("expected help output to mention '--strict', got: %s", buf.String())
	}
}

func TestCheckCommand_InvalidFailOn(t *testing.T) {
	t.Parallel()

	cmd := cli.NewRootCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"check", "--fail-on", "invalid"})

	err := cmd.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("expected error on invalid --fail-on, got nil")
	}
	if cli.ExitCode(err) != cli.ExitConfig {
		t.Errorf("expected exit code %d (ExitConfig), got %d", cli.ExitConfig, cli.ExitCode(err))
	}
}

func TestCheckCommand_ExplicitMissingConfig(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	missingConfig := filepath.Join(tmpDir, "missing.yaml")

	cmd := cli.NewRootCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"check", "--config", missingConfig})

	err := cmd.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("expected error on missing config, got nil")
	}
	if cli.ExitCode(err) != cli.ExitConfig {
		t.Errorf("expected exit code %d (ExitConfig), got %d", cli.ExitConfig, cli.ExitCode(err))
	}
}
