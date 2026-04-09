package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/gberns/kerf/internal/cmdutil"
	"github.com/gberns/kerf/internal/snapshot"
)

var restoreCmd = &cobra.Command{
	Use:   "restore <codename> <snapshot>",
	Short: "Restore a work to a previous snapshot state",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		codenameArg := args[0]
		snapshotArg := args[1]

		projectID, err := cmdutil.ResolveProject(projectFlag)
		if err != nil {
			return err
		}

		s, workDir, err := cmdutil.LoadWorkWithChecks(projectID, codenameArg)
		if err != nil {
			return err
		}

		preRestorePath, err := snapshot.Restore(workDir, snapshotArg)
		if err != nil {
			return fmt.Errorf("Error: snapshot '%s' not found in work '%s'. Run 'kerf history %s' to see available snapshots", snapshotArg, codenameArg, codenameArg)
		}

		relPreRestore, _ := filepath.Rel(workDir, preRestorePath)
		fmt.Printf("Restored %s to snapshot %s.\n", codenameArg, snapshotArg)
		fmt.Printf("Pre-restore state saved to: %s/\n", relPreRestore)

		// Active session warning.
		if s.ActiveSession != nil {
			fmt.Println()
			fmt.Println("Warning: active session in progress. Restored spec.yaml reflects the")
			fmt.Println("snapshot's status and metadata, but session tracking is preserved from")
			fmt.Println("the current state.")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(restoreCmd)
}
