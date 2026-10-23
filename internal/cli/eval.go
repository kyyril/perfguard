package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/kyyril/perfguard/internal/app"
	"github.com/kyyril/perfguard/internal/config"
	"github.com/kyyril/perfguard/internal/eval"
)

func newEvalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "eval",
		Short: "Run evaluation scenarios",
		Long:  "Run evaluation scenarios to measure detection quality and false positive rate.",
	}

	cmd.AddCommand(newEvalRunCmd())
	cmd.AddCommand(newEvalReportCmd())

	return cmd
}

func newEvalRunCmd() *cobra.Command {
	var (
		scenarioDir string
		outputDir   string
		configFile  string
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run a single evaluation scenario",
		Long: `Run a single evaluation scenario for the specified number of trials.

Each trial creates a temporary git repository from the scenario's base/
directory, applies change.patch, and runs perfguard check in same-runner mode.
Results are written to the output directory as JSON.`,
		Example: `  perfguard eval run --scenario eval/scenarios/algo-slowdown --output eval/results
  perfguard eval run --scenario eval/scenarios/no-change-aa`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()

			if scenarioDir == "" {
				return WrapConfig(fmt.Errorf("--scenario is required"))
			}

			cfg, err := loadConfigFromFile(configFile)
			if err != nil {
				return WrapConfig(err)
			}

			outDir := outputDir
			if outDir == "" {
				wd, _ := os.Getwd()
				outDir = filepath.Join(wd, ".perfguard", "eval")
			}

			result, err := app.EvalRun(ctx, app.EvalRunOptions{
				ScenarioDir: scenarioDir,
				OutputDir:   outDir,
				Config:      cfg,
			})

			if err != nil {
				if errors.Is(err, eval.ErrScenario) {
					return WrapConfig(err)
				}
				return WrapExecution(err)
			}

			successCount := 0
			for _, tr := range result.Trials {
				if tr.Error == "" {
					successCount++
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Scenario: %s\n", result.ScenarioID)
			fmt.Fprintf(cmd.OutOrStdout(), "Trials: %d (%d succeeded, %d failed)\n",
				len(result.Trials), successCount, len(result.Trials)-successCount)
			fmt.Fprintf(cmd.OutOrStdout(), "Duration: %v\n", result.FinishedAt.Sub(result.StartedAt))
			fmt.Fprintf(cmd.OutOrStdout(), "Results written to: %s/%s.json\n", outDir, result.ScenarioID)

			return nil
		},
	}

	cmd.Flags().StringVar(&scenarioDir, "scenario", "", "Path to scenario directory (required)")
	cmd.Flags().StringVar(&outputDir, "output", "", "Output directory for results (default: .perfguard/eval)")
	cmd.Flags().StringVar(&configFile, "config", "", "Path to perfguard.yaml config file")

	return cmd
}

func newEvalReportCmd() *cobra.Command {
	var (
		outputDir     string
		scenariosGlob string
		reportPath    string
	)

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate evaluation report from scenario results",
		Long: `Aggregate results from multiple scenario runs and generate a markdown report.

The report includes detection rates, false positive rates, diagnosis hit rates,
and confidence intervals for all metrics.`,
		Example: `  perfguard eval report --output eval/results --scenarios "eval/scenarios/*"
  perfguard eval report --output .perfguard/eval --report eval/RESULTS.md`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()

			if outputDir == "" {
				wd, _ := os.Getwd()
				outputDir = filepath.Join(wd, ".perfguard", "eval")
			}

			if scenariosGlob == "" {
				scenariosGlob = "eval/scenarios/*"
			}

			if reportPath == "" {
				reportPath = "eval/RESULTS.md"
			}

			// List all scenarios
			matches, err := filepath.Glob(scenariosGlob)
			if err != nil {
				return WrapConfig(fmt.Errorf("glob scenarios: %w", err))
			}

			if len(matches) == 0 {
				return WrapConfig(fmt.Errorf("no scenarios found matching: %s", scenariosGlob))
			}

			var scenarioResults []eval.ScenarioResult
			for _, scenarioPath := range matches {
				info, err := os.Stat(scenarioPath)
				if err != nil || !info.IsDir() {
					continue
				}

				scenario, err := eval.LoadScenario(scenarioPath)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: skip %s: %v\n", scenarioPath, err)
					continue
				}

				sr, err := eval.AggregateResults(ctx, scenario, outputDir)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to aggregate %s: %v\n", scenario.ID, err)
					continue
				}

				scenarioResults = append(scenarioResults, sr)
			}

			if len(scenarioResults) == 0 {
				return WrapExecution(fmt.Errorf("no scenario results available"))
			}

			reportData := eval.ReportData{
				Scenarios:   scenarioResults,
				Environment: eval.CaptureEnvironment(),
			}

			report := eval.GenerateReport(reportData)

			if err := os.MkdirAll(filepath.Dir(reportPath), 0755); err != nil {
				return WrapExecution(fmt.Errorf("create report directory: %w", err))
			}

			if err := os.WriteFile(reportPath, []byte(report), 0o600); err != nil {
				return WrapExecution(fmt.Errorf("write report: %w", err))
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Report generated: %s\n", reportPath)
			fmt.Fprintf(cmd.OutOrStdout(), "Scenarios: %d\n", len(scenarioResults))

			return nil
		},
	}

	cmd.Flags().StringVar(&outputDir, "output", "", "Output directory containing trial results (default: .perfguard/eval)")
	cmd.Flags().StringVar(&scenariosGlob, "scenarios", "", "Glob pattern for scenario directories (default: eval/scenarios/*)")
	cmd.Flags().StringVar(&reportPath, "report", "", "Path to write markdown report (default: eval/RESULTS.md)")

	return cmd
}

func loadConfigFromFile(path string) (*config.Config, error) {
	if path == "" {
		path = config.ResolveConfigPath()
	}

	cfg, err := config.Load(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return config.Default(), nil
		}
		return nil, fmt.Errorf("load config: %w", err)
	}
	return cfg, nil
}
