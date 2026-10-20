package cli_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/kyyril/perfguard/internal/cli"
)

func TestParseLogLevel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input    string
		expected slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"unknown", slog.LevelInfo},
		{"", slog.LevelInfo},
	}

	for _, tc := range cases {
		got := cli.ParseLogLevel(tc.input)
		if got != tc.expected {
			t.Errorf("ParseLogLevel(%q) = %v; want %v", tc.input, got, tc.expected)
		}
	}
}

func TestSetupLogger_Text(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := cli.SetupLogger("info", "text", &buf)
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}

	logger.Info("test message", "key", "val")
	output := buf.String()
	if !strings.Contains(output, "test message") || !strings.Contains(output, "key=val") {
		t.Errorf("unexpected log output: %s", output)
	}
}

func TestSetupLogger_JSON(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := cli.SetupLogger("info", "json", &buf)
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}

	logger.Info("json message", "run_id", "run-123")
	output := buf.String()

	var record map[string]any
	if err := json.Unmarshal([]byte(output), &record); err != nil {
		t.Fatalf("failed parsing json log record: %v\nOutput: %s", err, output)
	}

	if record["msg"] != "json message" {
		t.Errorf("expected msg 'json message', got %v", record["msg"])
	}
	if record["run_id"] != "run-123" {
		t.Errorf("expected run_id 'run-123', got %v", record["run_id"])
	}
}

func TestSetupLogger_EnvOverride(t *testing.T) {
	// Not parallel due to environment mutation.
	orig := os.Getenv("PERFGUARD_LOG_LEVEL")
	defer func() {
		_ = os.Setenv("PERFGUARD_LOG_LEVEL", orig)
	}()

	_ = os.Setenv("PERFGUARD_LOG_LEVEL", "debug")
	var buf bytes.Buffer
	logger := cli.SetupLogger("", "text", &buf)
	logger.Debug("debug message")

	if !strings.Contains(buf.String(), "debug message") {
		t.Errorf("expected debug message when PERFGUARD_LOG_LEVEL=debug, got: %s", buf.String())
	}
}
