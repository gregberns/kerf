package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gberns/kerf/internal/cmdutil"
	"github.com/gberns/kerf/internal/snapshot"
)

var historyCmd = &cobra.Command{
	Use:   "history <codename>",
	Short: "Show the version history of a work",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		codenameArg := args[0]

		projectID, err := cmdutil.ResolveProject(projectFlag)
		if err != nil {
			return err
		}

		_, workDir, err := cmdutil.LoadWorkWithChecks(projectID, codenameArg)
		if err != nil {
			return err
		}

		snapshots, err := snapshot.List(workDir)
		if err != nil {
			return fmt.Errorf("reading history: %w", err)
		}

		if len(snapshots) == 0 {
			fmt.Printf("No snapshots found for work '%s'.\n", codenameArg)
			return nil
		}

		fmt.Printf("History for %s:\n", codenameArg)
		for _, s := range snapshots {
			fmt.Printf("  %-40s %s\n", s.Name, s.Status)
		}
		fmt.Println()
		fmt.Println("Commands:")
		fmt.Printf("  kerf restore %s <snapshot>    Restore to a previous snapshot\n", codenameArg)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(historyCmd)
}
