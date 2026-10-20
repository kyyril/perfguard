package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/kyyril/perfguard/internal/app"
	"github.com/kyyril/perfguard/internal/config"
)

type compareFlags struct {
	output string
}

func newCompareCmd(globalOpts *GlobalOptions) *cobra.Command {
	flags := &compareFlags{}

	cmd := &cobra.Command{
		Use:   "compare <base.result.json> <head.result.json>",
		Short: "Compare two benchmark results statistically and write comparison.json",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 2 {
				return WrapConfig(fmt.Errorf("compare requires two arguments: <base.result.json> <head.result.json>"))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(globalOpts.ConfigPath)
			if err != nil {
				if !cmd.Flags().Changed("config") && errors.Is(err, os.ErrNotExist) {
					cfg = config.Default()
				} else {
					return WrapConfig(fmt.Errorf("failed to load configuration: %w", err))
				}
			}

			basePath := args[0]
			headPath := args[1]

			outDest := flags.output
			if outDest == "" {
				outDest = filepath.Join(globalOpts.OutputDir, "comparison.json")
			}

			cmp, outFile, err := app.Compare(cmd.Context(), app.CompareOptions{
				Config:         cfg,
				BaseResultFile: basePath,
				HeadResultFile: headPath,
				OutputDir:      outDest,
			})
			if err != nil {
				return WrapExecution(err)
			}

			cmd.Printf("Compared %d metrics: %s verdict, written to %s\n",
				len(cmp.Items), cmp.Verdict, outFile)
			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.output, "output", "o", "", "Destination file or directory for comparison.json")

	return cmd
}
