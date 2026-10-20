//go:build e2e

package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kyyril/perfguard/internal/config"
)

func buildBinary(t *testing.T) string {
	t.Helper()

	binDir := t.TempDir()
	binName := "perfguard"
	if os.Getenv("GOOS") == "windows" || filepath.Separator == '\\' {
		binName += ".exe"
	}
	binPath := filepath.Join(binDir, binName)

	cmd := exec.Command("go", "build", "-o", binPath, "github.com/kyyril/perfguard/cmd/perfguard")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed building perfguard binary: %v\nOutput: %s", err, string(output))
	}
	return binPath
}

func TestE2E_VersionAndInit(t *testing.T) {
	binPath := buildBinary(t)

	t.Run("version command", func(t *testing.T) {
		cmd := exec.Command(binPath, "version")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("perfguard version failed: %v\nOutput: %s", err, string(output))
		}
		if !strings.HasPrefix(string(output), "perfguard") {
			t.Errorf("expected version output to start with 'perfguard', got: %s", string(output))
		}
	})

	t.Run("init creates valid configuration", func(t *testing.T) {
		workDir := t.TempDir()

		// Run init in clean directory
		cmd := exec.Command(binPath, "init")
		cmd.Dir = workDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("perfguard init failed: %v\nOutput: %s", err, string(output))
		}

		configPath := filepath.Join(workDir, "perfguard.yaml")
		if _, err := os.Stat(configPath); err != nil {
			t.Fatalf("perfguard.yaml was not created: %v", err)
		}

		cfg, err := config.Load(configPath)
		if err != nil {
			t.Fatalf("generated perfguard.yaml failed validation: %v", err)
		}
		if cfg.Version != 1 {
			t.Errorf("expected version 1, got %d", cfg.Version)
		}

		// Second init without force must exit with code 2
		cmdSecond := exec.Command(binPath, "init")
		cmdSecond.Dir = workDir
		errSecond := cmdSecond.Run()
		if errSecond == nil {
			t.Fatal("expected non-zero exit code on duplicate init, got nil")
		}
		if exitErr, ok := errSecond.(*exec.ExitError); ok {
			if exitErr.ExitCode() != 2 {
				t.Errorf("expected exit code 2 (ExitConfig), got %d", exitErr.ExitCode())
			}
		} else {
			t.Fatalf("expected *exec.ExitError, got: %T", errSecond)
		}

		// Third init with force must succeed with exit code 0
		cmdForce := exec.Command(binPath, "init", "--force")
		cmdForce.Dir = workDir
		if err := cmdForce.Run(); err != nil {
			t.Fatalf("perfguard init --force failed: %v", err)
		}
	})
}
