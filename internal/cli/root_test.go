package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kyyril/perfguard/internal/cli"
	"github.com/kyyril/perfguard/internal/config"
)

func TestVersionCommand(t *testing.T) {
	t.Parallel()

	cmd := cli.NewRootCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"version"})

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	output := buf.String()
	if !strings.HasPrefix(output, "perfguard") {
		t.Errorf("expected version output to start with 'perfguard', got: %s", output)
	}
}

func TestInitCommand(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "perfguard.yaml")

	cmd := cli.NewRootCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"init", "--config", targetPath})

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("init command failed: %v", err)
	}

	// Verify file was written and is valid config
	cfg, err := config.Load(targetPath)
	if err != nil {
		t.Fatalf("failed loading generated config: %v", err)
	}
	if cfg.Version != 1 {
		t.Errorf("expected version 1, got %d", cfg.Version)
	}

	// Run again without force should fail with ErrConfig
	cmdSecond := cli.NewRootCommand()
	cmdSecond.SetArgs([]string{"init", "--config", targetPath})
	errSecond := cmdSecond.ExecuteContext(context.Background())
	if errSecond == nil {
		t.Fatal("expected error on duplicate init without force, got nil")
	}
	if cli.ExitCode(errSecond) != cli.ExitConfig {
		t.Errorf("expected exit code %d (ExitConfig), got %d", cli.ExitConfig, cli.ExitCode(errSecond))
	}

	// Run again with force should succeed
	cmdForce := cli.NewRootCommand()
	cmdForce.SetArgs([]string{"init", "--config", targetPath, "--force"})
	if err := cmdForce.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("init --force failed: %v", err)
	}
}

func TestExecute_Success(t *testing.T) {
	// Root without args prints help and returns ExitPass
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"perfguard", "--help"}
	code := cli.Execute(context.Background())
	if code != cli.ExitPass {
		t.Errorf("expected ExitPass, got: %d", code)
	}
}
