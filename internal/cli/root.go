package cli

import (
	"context"
	"fmt"

	"github.com/kyyril/perfguard/internal/config"
	"github.com/spf13/cobra"
)

// GlobalOptions stores persistent flag values.
type GlobalOptions struct {
	ConfigPath string
	LogLevel   string
	LogFormat  string
	OutputDir  string
}

// NewRootCommand creates the top-level perfguard command.
func NewRootCommand() *cobra.Command {
	opts := &GlobalOptions{}

	rootCmd := &cobra.Command{
		Use:   "perfguard",
		Short: "Performance regression detection for Go projects",
		Long: "PerfGuard runs benchmarks before and after a code change, " +
			"compares them statistically, and diagnoses regressions using pprof.",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			SetupLogger(opts.LogLevel, opts.LogFormat, cmd.ErrOrStderr())
			return nil
		},
	}

	rootCmd.PersistentFlags().StringVarP(&opts.ConfigPath, "config", "c", config.ResolveConfigPath(), "Path to configuration file")
	rootCmd.PersistentFlags().StringVar(&opts.LogLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&opts.LogFormat, "log-format", "text", "Log format (text, json)")
	rootCmd.PersistentFlags().StringVar(&opts.OutputDir, "output-dir", config.DefaultOutputDir, "Directory for run artifacts")

	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(newInitCmd(opts))
	rootCmd.AddCommand(newRunCmd(opts))
	rootCmd.AddCommand(newCompareCmd(opts))
	rootCmd.AddCommand(newCheckCmd(opts))

	return rootCmd
}

// Execute runs the CLI root command and returns the exit code.
func Execute(ctx context.Context) int {
	cmd := NewRootCommand()
	if err := cmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), err)
		return ExitCode(err)
	}
	return ExitPass
}
