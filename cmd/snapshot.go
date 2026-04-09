package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/gberns/kerf/internal/codename"
	"github.com/gberns/kerf/internal/cmdutil"
	"github.com/gberns/kerf/internal/config"
	"github.com/gberns/kerf/internal/snapshot"

	"github.com/gberns/kerf/internal/bench"
)

var snapshotNameFlag string

var snapshotCmd = &cobra.Command{
	Use:   "snapshot <codename>",
	Short: "Manually trigger a versioning snapshot",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		codenameArg := args[0]

		// Validate label if provided.
		if snapshotNameFlag != "" {
			if err := codename.Validate(snapshotNameFlag); err != nil {
				return fmt.Errorf("snapshot name must be lowercase alphanumeric and hyphens")
			}
		}

		projectID, err := cmdutil.ResolveProject(projectFlag)
		if err != nil {
			return err
		}

		_, workDir, err := cmdutil.LoadWorkWithChecks(projectID, codenameArg)
		if err != nil {
			return err
		}

		snapDir, err := snapshot.Take(workDir, snapshotNameFlag)
		if err != nil {
			return fmt.Errorf("taking snapshot: %w", err)
		}

		// Prune if needed.
		bp, err := bench.BenchPath()
		if err == nil {
			cfg, _ := config.Load(filepath.Join(bp, "config.yaml"))
			if cfg != nil {
				_ = snapshot.Prune(workDir, cfg.EffectiveMaxSnapshots())
			}
		}

		// Show relative path within work dir.
		relPath, _ := filepath.Rel(workDir, snapDir)
		fmt.Printf("Snapshot created: %s/\n", relPath)
		return nil
	},
}

func init() {
	snapshotCmd.Flags().StringVar(&snapshotNameFlag, "name", "", "Human-readable label for the snapshot (lowercase slug)")
	rootCmd.AddCommand(snapshotCmd)
}
