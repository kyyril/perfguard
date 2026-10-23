package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/kyyril/perfguard/internal/model"
	"github.com/kyyril/perfguard/internal/profile"
)

type profileDiffFlags struct {
	opsBase    int64
	opsHead    int64
	sampleType string
	minShare   float64
	top        int
	format     string
}

// newProfileCmd creates the "profile" parent command.
func newProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Profile analysis commands",
	}
	cmd.AddCommand(newProfileDiffCmd())
	return cmd
}

// newProfileDiffCmd creates the "profile diff" subcommand.
func newProfileDiffCmd() *cobra.Command {
	flags := &profileDiffFlags{}

	cmd := &cobra.Command{
		Use:   "diff <base.pb.gz> <head.pb.gz>",
		Short: "Diff two pprof profiles and print contributor candidates",
		Long: "Loads two pprof profiles, normalizes each by the given operation counts, " +
			"and prints functions ranked by the magnitude of their per-operation cost change.",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 2 {
				return WrapConfig(fmt.Errorf("profile diff requires two arguments: <base.pb.gz> <head.pb.gz>"))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			basePath := args[0]
			headPath := args[1]

			baseProf, err := profile.Load(basePath)
			if err != nil {
				return WrapExecution(fmt.Errorf("profile diff: load base profile: %w", err))
			}
			headProf, err := profile.Load(headPath)
			if err != nil {
				return WrapExecution(fmt.Errorf("profile diff: load head profile: %w", err))
			}

			sampleIdx, err := profile.FindSampleIndex(baseProf, flags.sampleType)
			if err != nil {
				return WrapConfig(fmt.Errorf("profile diff: %w", err))
			}

			opsBase := flags.opsBase
			if opsBase <= 0 {
				opsBase = 1
			}
			opsHead := flags.opsHead
			if opsHead <= 0 {
				opsHead = 1
			}

			candidates, err := profile.Diff(baseProf, headProf, sampleIdx, opsBase, opsHead, profile.DiffOptions{
				MinShare: flags.minShare,
				Top:      flags.top,
			})
			if err != nil {
				return WrapExecution(fmt.Errorf("profile diff: %w", err))
			}

			// Determine the sample type name for display
			sampleTypeName := flags.sampleType
			if sampleTypeName == "" && len(baseProf.SampleType) > sampleIdx {
				sampleTypeName = baseProf.SampleType[sampleIdx].Type
			}

			switch strings.ToLower(flags.format) {
			case "json":
				return renderProfileDiffJSON(cmd, candidates, sampleTypeName)
			default:
				return renderProfileDiffText(cmd, candidates, sampleTypeName, basePath, headPath)
			}
		},
	}

	cmd.Flags().Int64Var(&flags.opsBase, "ops-base", 1, "Total operations executed in the base profile")
	cmd.Flags().Int64Var(&flags.opsHead, "ops-head", 1, "Total operations executed in the head profile")
	cmd.Flags().StringVar(&flags.sampleType, "sample-type", "", "pprof sample type to diff (empty uses profile default)")
	cmd.Flags().Float64Var(&flags.minShare, "min-share", 0, "Minimum share of total base metric required, in percent (e.g. 1.0 for 1%)")
	cmd.Flags().IntVar(&flags.top, "top", 20, "Maximum number of candidates to output (0 for unlimited)")
	cmd.Flags().StringVar(&flags.format, "format", "text", "Output format: 'text' or 'json'")

	return cmd
}

func renderProfileDiffText(cmd *cobra.Command, candidates []model.Candidate, sampleType, basePath, headPath string) error {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Profile diff: %s\n", sampleType)
	fmt.Fprintf(w, "  base: %s\n", basePath)
	fmt.Fprintf(w, "  head: %s\n\n", headPath)

	if len(candidates) == 0 {
		fmt.Fprintln(w, "No significant differences found.")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "FUNCTION\tFLAT DELTA\tCUM DELTA\tTOUCHED")
	fmt.Fprintln(tw, "--------\t----------\t---------\t-------")

	// Determine sign prefix for display
	for _, c := range candidates {
		touched := "no"
		if c.Touched {
			touched = "yes"
		}
		fmt.Fprintf(tw, "%s\t%+.4f\t%+.4f\t%s\n", c.Function, c.FlatDelta, c.CumDelta, touched)
	}
	return tw.Flush()
}

func renderProfileDiffJSON(cmd *cobra.Command, candidates []model.Candidate, sampleType string) error {
	type jsonOutput struct {
		SampleType string            `json:"sample_type"`
		Candidates []model.Candidate `json:"candidates"`
	}

	out := jsonOutput{
		SampleType: sampleType,
		Candidates: candidates,
	}
	if out.Candidates == nil {
		out.Candidates = []model.Candidate{}
	}

	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
