package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Build information populated via ldflags.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, _ []string) {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "perfguard %s (%s, built %s) %s %s/%s\n",
				Version, Commit, Date, runtime.Version(), runtime.GOOS, runtime.GOARCH)
		},
	}
}
