package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/kyyril/perfguard/internal/app"
	"github.com/kyyril/perfguard/internal/config"
	"github.com/kyyril/perfguard/internal/model"
)

type runFlags struct {
	side   string
	output string
}

func newRunCmd(globalOpts *GlobalOptions) *cobra.Command {
	flags := &runFlags{}

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run benchmarks on the current working tree and write result.json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(globalOpts.ConfigPath)
			if err != nil {
				if !cmd.Flags().Changed("config") && os.IsNotExist(err) {
					cfg = config.Default()
				} else {
					return WrapConfig(fmt.Errorf("failed to load configuration: %w", err))
				}
			}

			side := model.Side(flags.side)
			if side != model.SideBase && side != model.SideHead {
				return WrapConfig(fmt.Errorf("invalid side %q: must be 'base' or 'head'", flags.side))
			}

			outDest := flags.output
			if outDest == "" {
				outDest = filepath.Join(globalOpts.OutputDir, "runs")
			}

			runResult, outFile, err := app.Run(cmd.Context(), app.RunOptions{
				Config:    cfg,
				Side:      side,
				OutputDir: outDest,
			})
			if err != nil {
				return WrapExecution(err)
			}

			cmd.Printf("Recorded %d benchmarks (%s side) to %s\n",
				len(runResult.Benchmarks), runResult.Side, outFile)
			return nil
		},
	}

	cmd.Flags().StringVar(&flags.side, "side", string(model.SideHead), "Side to record (base or head)")
	cmd.Flags().StringVarP(&flags.output, "output", "o", "", "Destination file or directory for result.json")

	return cmd
}
