package cli

import (
	"fmt"
	"os"

	"github.com/kyyril/perfguard/internal/config"
	"github.com/spf13/cobra"
)

const starterConfig = `version: 1
benchmarks:
  packages: ["./..."]
  include: "."
  count: 10
  benchtime: "1s"
thresholds:
  metrics:
    sec/op:    { warn: 5,  fail: 15 }
    B/op:      { warn: 10, fail: 25 }
    allocs/op: { warn: 5,  fail: 20 }
profiling:
  mode: on-regression
  types: [cpu, allocs]
`

func newInitCmd(opts *GlobalOptions) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create initial configuration file",
		Long:  "Creates a starter perfguard.yaml configuration file in the target directory.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			targetPath := opts.ConfigPath
			if targetPath == "" {
				targetPath = config.DefaultConfigPath
			}

			if _, err := os.Stat(targetPath); err == nil && !force {
				return WrapConfig(fmt.Errorf("config file %s already exists, use --force to overwrite", targetPath))
			}

			// Validate starter config against parser before writing
			if _, err := config.LoadBytes([]byte(starterConfig)); err != nil {
				return WrapExecution(fmt.Errorf("starter config template is invalid: %w", err))
			}

			if err := os.WriteFile(targetPath, []byte(starterConfig), 0o600); err != nil {
				return WrapExecution(fmt.Errorf("writing config file %s: %w", targetPath, err))
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Created %s\n", targetPath)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite existing config file")
	return cmd
}
