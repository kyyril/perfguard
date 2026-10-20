package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/kyyril/perfguard/internal/app"
	"github.com/kyyril/perfguard/internal/config"
	"github.com/kyyril/perfguard/internal/report"
)

type checkFlags struct {
	baseRef string
	failOn  string
	strict  bool
	format  string
	mode    string
}

func newCheckCmd(globalOpts *GlobalOptions) *cobra.Command {
	flags := &checkFlags{}

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Run benchmarks against base reference, compare, and report regressions",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(globalOpts.ConfigPath)
			if err != nil {
				if !cmd.Flags().Changed("config") && errors.Is(err, os.ErrNotExist) {
					cfg = config.Default()
				} else {
					return WrapConfig(fmt.Errorf("failed to load configuration: %w", err))
				}
			}

			if flags.failOn != "fail" && flags.failOn != "warn" {
				return WrapConfig(fmt.Errorf("invalid --fail-on value %q: must be 'fail' or 'warn'", flags.failOn))
			}

			outDir := globalOpts.OutputDir
			if outDir == "" {
				outDir = config.DefaultOutputDir
			}

			res, err := app.Check(cmd.Context(), app.CheckOptions{
				Config:    cfg,
				BaseRef:   flags.baseRef,
				Mode:      flags.mode,
				OutputDir: filepath.Join(outDir, "runs"),
			})
			if err != nil {
				// Base ref resolve failures or config issues map to ExitConfig
				if config.IsValidationError(err) || errors.Is(err, ErrConfig) {
					return WrapConfig(err)
				}
				// All build, run, git execution failures map to ExitExecution
				return WrapExecution(err)
			}

			// Render report to stdout
			fmtFormat := report.Format(flags.format)
			if fmtFormat == "" {
				fmtFormat = report.FormatText
			}
			if err := report.Render(cmd.OutOrStdout(), res.Comparison, fmtFormat); err != nil {
				return WrapExecution(fmt.Errorf("failed rendering report: %w", err))
			}

			// Determine exit code from verdict
			vErr := &VerdictError{
				Verdict: res.Comparison.Verdict,
				Strict:  flags.strict,
				FailOn:  flags.failOn,
			}
			if vErr.ExitCode() != ExitPass {
				return vErr
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&flags.baseRef, "base", "", "Base git reference to compare against (defaults to config branch or origin/main)")
	cmd.Flags().StringVar(&flags.mode, "mode", "", "Execution mode: 'same-runner' (default) or 'stored'")
	cmd.Flags().StringVar(&flags.failOn, "fail-on", "fail", "Failure threshold for exit code 1: 'fail' or 'warn'")
	cmd.Flags().BoolVar(&flags.strict, "strict", false, "Exit with code 4 on inconclusive verdict")
	cmd.Flags().StringVar(&flags.format, "format", "text", "Report output format: 'text' or 'json'")

	return cmd
}
