package cli

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// ParseLogLevel converts a level string to slog.Level.
func ParseLogLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	case "info":
		return slog.LevelInfo
	default:
		return slog.LevelInfo
	}
}

// SetupLogger initializes the global structured logger using slog.
// Output defaults to os.Stderr if out is nil.
func SetupLogger(level string, format string, out io.Writer) *slog.Logger {
	if out == nil {
		out = os.Stderr
	}

	envLevel := os.Getenv("PERFGUARD_LOG_LEVEL")
	if envLevel != "" && level == "" {
		level = envLevel
	}

	lvl := ParseLogLevel(level)
	opts := &slog.HandlerOptions{
		Level: lvl,
	}

	var handler slog.Handler
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		handler = slog.NewJSONHandler(out, opts)
	case "text":
		handler = slog.NewTextHandler(out, opts)
	default:
		handler = slog.NewTextHandler(out, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}
