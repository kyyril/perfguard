package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/kyyril/perfguard/internal/app"
	"github.com/kyyril/perfguard/internal/baseline"
	"github.com/kyyril/perfguard/internal/config"
	"github.com/kyyril/perfguard/internal/gitx"
	"github.com/kyyril/perfguard/internal/model"
)

func resolveBaselineStore(cfg *config.Config, workDir string) baseline.Store {
	if cfg.Baseline.Store == "git" {
		branch := cfg.Baseline.Branch
		if branch == "" {
			branch = "perfguard-data"
		}
		return baseline.NewGitStore(gitx.New(workDir), branch)
	}
	dir := cfg.Baseline.Dir
	if dir == "" {
		dir = filepath.Join(config.DefaultOutputDir, "baselines")
	}
	return baseline.NewFSStore(dir)
}

func newBaselineCmd(globalOpts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "baseline",
		Short: "Manage stored baselines",
	}

	cmd.AddCommand(newBaselineSaveCmd(globalOpts))
	cmd.AddCommand(newBaselineListCmd(globalOpts))
	cmd.AddCommand(newBaselineShowCmd(globalOpts))
	cmd.AddCommand(newBaselinePruneCmd(globalOpts))

	return cmd
}

func newBaselineSaveCmd(globalOpts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "save [result.json]",
		Short: "Save benchmark results as a stored baseline",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(globalOpts.ConfigPath)
			if err != nil {
				if !cmd.Flags().Changed("config") && errors.Is(err, os.ErrNotExist) {
					cfg = config.Default()
				} else {
					return WrapConfig(fmt.Errorf("failed to load configuration: %w", err))
				}
			}

			store := resolveBaselineStore(cfg, ".")

			var run model.Run
			if len(args) == 1 {
				// Load run from file
				data, err := os.ReadFile(args[0])
				if err != nil {
					return WrapConfig(fmt.Errorf("failed to read result file %s: %w", args[0], err))
				}
				if err := json.Unmarshal(data, &run); err != nil {
					return WrapConfig(fmt.Errorf("invalid result json in %s: %w", args[0], err))
				}
			} else {
				// Run benchmarks on current head
				resRun, _, err := app.Run(cmd.Context(), app.RunOptions{
					Config:  cfg,
					Side:    "head",
					WorkDir: ".",
				})
				if err != nil {
					return WrapExecution(fmt.Errorf("failed running benchmark: %w", err))
				}
				run = resRun
			}

			key, err := store.Save(cmd.Context(), run)
			if err != nil {
				return WrapExecution(fmt.Errorf("failed saving baseline: %w", err))
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Saved baseline for commit %s (key: %s)\n", run.Git.SHA, key.String())
			return nil
		},
	}
}

type listFlags struct {
	os   string
	arch string
	sha  string
}

func newBaselineListCmd(globalOpts *GlobalOptions) *cobra.Command {
	flags := &listFlags{}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List stored baselines",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(globalOpts.ConfigPath)
			if err != nil {
				if !cmd.Flags().Changed("config") && errors.Is(err, os.ErrNotExist) {
					cfg = config.Default()
				} else {
					return WrapConfig(fmt.Errorf("failed to load configuration: %w", err))
				}
			}

			store := resolveBaselineStore(cfg, ".")
			filter := baseline.Filter{
				OS:        flags.os,
				Arch:      flags.arch,
				CommitSHA: flags.sha,
			}

			items, err := store.List(cmd.Context(), filter)
			if err != nil {
				return WrapExecution(fmt.Errorf("failed listing baselines: %w", err))
			}

			if len(items) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No stored baselines found.")
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
			_, _ = fmt.Fprintln(w, "COMMIT\tREF\tOS-ARCH\tENV HASH\tCREATED")
			for _, item := range items {
				sha := item.CommitSHA
				if len(sha) > 10 {
					sha = sha[:10]
				}
				ref := item.Ref
				if ref == "" {
					ref = "-"
				}
				osArch := fmt.Sprintf("%s-%s", item.OS, item.Arch)
				created := item.CreatedAt.Format(time.RFC3339)
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", sha, ref, osArch, item.EnvHash, created)
			}
			return w.Flush()
		},
	}

	cmd.Flags().StringVar(&flags.os, "os", "", "Filter by operating system")
	cmd.Flags().StringVar(&flags.arch, "arch", "", "Filter by CPU architecture")
	cmd.Flags().StringVar(&flags.sha, "sha", "", "Filter by commit SHA")

	return cmd
}

func newBaselineShowCmd(globalOpts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "show <commit-sha>",
		Short: "Show details of a stored baseline",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(globalOpts.ConfigPath)
			if err != nil {
				if !cmd.Flags().Changed("config") && errors.Is(err, os.ErrNotExist) {
					cfg = config.Default()
				} else {
					return WrapConfig(fmt.Errorf("failed to load configuration: %w", err))
				}
			}

			store := resolveBaselineStore(cfg, ".")
			sha := args[0]

			items, err := store.List(cmd.Context(), baseline.Filter{CommitSHA: sha})
			if err != nil {
				return WrapExecution(fmt.Errorf("failed searching baseline: %w", err))
			}
			if len(items) == 0 {
				return WrapConfig(fmt.Errorf("baseline for commit %q not found", sha))
			}

			run, err := store.Load(cmd.Context(), items[0].Key)
			if err != nil {
				return WrapExecution(fmt.Errorf("failed loading baseline: %w", err))
			}

			data, err := json.MarshalIndent(run, "", "  ")
			if err != nil {
				return WrapExecution(fmt.Errorf("failed formatting json: %w", err))
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}
}

type pruneFlags struct {
	keep      int
	olderThan string
	perGroup  bool
}

func newBaselinePruneCmd(globalOpts *GlobalOptions) *cobra.Command {
	flags := &pruneFlags{}

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Prune old stored baselines",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(globalOpts.ConfigPath)
			if err != nil {
				if !cmd.Flags().Changed("config") && errors.Is(err, os.ErrNotExist) {
					cfg = config.Default()
				} else {
					return WrapConfig(fmt.Errorf("failed to load configuration: %w", err))
				}
			}

			store := resolveBaselineStore(cfg, ".")

			var duration time.Duration
			if flags.olderThan != "" {
				s := flags.olderThan
				if strings.HasSuffix(s, "d") {
					var days int
					if _, err := fmt.Sscanf(s, "%dd", &days); err == nil {
						duration = time.Duration(days) * 24 * time.Hour
					}
				}
				if duration == 0 {
					d, err := time.ParseDuration(flags.olderThan)
					if err != nil {
						return WrapConfig(fmt.Errorf("invalid duration --older-than %q: %w", flags.olderThan, err))
					}
					duration = d
				}
			}

			opts := baseline.PruneOptions{
				Keep:      flags.keep,
				OlderThan: duration,
				PerGroup:  flags.perGroup,
			}

			deleted, err := store.Prune(cmd.Context(), opts)
			if err != nil {
				return WrapExecution(fmt.Errorf("failed pruning baselines: %w", err))
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Pruned %d baseline(s)\n", deleted)
			return nil
		},
	}

	cmd.Flags().IntVar(&flags.keep, "keep", 0, "Number of newest baselines to keep")
	cmd.Flags().StringVar(&flags.olderThan, "older-than", "", "Age cutoff duration (e.g. '30d', '720h')")
	cmd.Flags().BoolVar(&flags.perGroup, "per-group", false, "Apply keep limit per environment group")

	return cmd
}
