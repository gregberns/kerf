package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/gberns/kerf/internal/cmdutil"
)

var archiveCmd = &cobra.Command{
	Use:   "archive <codename>",
	Short: "Move a work to archive storage",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		codenameArg := args[0]

		projectID, err := cmdutil.ResolveProject(projectFlag)
		if err != nil {
			return err
		}

		r, err := cmdutil.Resolver(projectID)
		if err != nil {
			return err
		}

		// Check if already archived.
		if r.IsArchived(codenameArg) {
			return fmt.Errorf("work '%s' is already archived", codenameArg)
		}

		// Check work exists.
		if !r.WorkExists(codenameArg) {
			return fmt.Errorf("work '%s' not found in project '%s'", codenameArg, projectID)
		}

		if err := r.MoveToArchive(codenameArg); err != nil {
			return fmt.Errorf("archiving work: %w", err)
		}

		relBench := filepath.Join("~/.kerf")

		fmt.Printf("Work '%s' archived.\n", codenameArg)
		fmt.Println("To un-archive, move the directory back:")
		fmt.Printf("  mv %s/archive/%s/%s/ %s/projects/%s/%s/\n",
			relBench, projectID, codenameArg,
			relBench, projectID, codenameArg)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(archiveCmd)
}
