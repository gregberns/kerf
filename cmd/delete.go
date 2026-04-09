package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/gberns/kerf/internal/bench"
	"github.com/gberns/kerf/internal/cmdutil"
	"github.com/gberns/kerf/internal/snapshot"
	"github.com/gberns/kerf/internal/spec"
)

var deleteYesFlag bool

var deleteCmd = &cobra.Command{
	Use:   "delete <codename>",
	Short: "Permanently remove a work from the bench",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		codenameArg := args[0]

		projectID, err := cmdutil.ResolveProject(projectFlag)
		if err != nil {
			return err
		}

		bp, err := bench.BenchPath()
		if err != nil {
			return err
		}

		// Find the work — check active then archive.
		var workDir string
		if bench.WorkExists(bp, projectID, codenameArg) {
			workDir = bench.WorkDir(bp, projectID, codenameArg)
		} else if bench.IsArchived(bp, projectID, codenameArg) {
			workDir = bench.ArchiveDir(bp, projectID, codenameArg)
		} else {
			return fmt.Errorf("work '%s' not found in project '%s'", codenameArg, projectID)
		}

		// Read spec.yaml for summary.
		specPath := filepath.Join(workDir, "spec.yaml")
		s, err := spec.Read(specPath)
		if err != nil {
			return fmt.Errorf("reading work: %w", err)
		}

		// Count snapshots.
		snapshots, _ := snapshot.List(workDir)
		snapCount := len(snapshots)

		if !deleteYesFlag {
			title := "(none)"
			if s.Title != nil {
				title = *s.Title
			}
			fmt.Println("About to permanently delete:")
			fmt.Printf("  Codename:  %s\n", s.Codename)
			fmt.Printf("  Title:     %s\n", title)
			fmt.Printf("  Status:    %s\n", s.Status)
			fmt.Printf("  Created:   %s\n", s.Created.Format(time.RFC3339))
			fmt.Printf("  Snapshots: %d\n", snapCount)
			fmt.Println()
			fmt.Print("This cannot be undone. Continue? [y/N] ")

			reader := bufio.NewReader(os.Stdin)
			answer, _ := reader.ReadString('\n')
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer != "y" && answer != "yes" {
				return nil
			}
		}

		if err := os.RemoveAll(workDir); err != nil {
			return fmt.Errorf("deleting work: %w", err)
		}

		fmt.Printf("Work '%s' deleted.\n", codenameArg)
		return nil
	},
}

func init() {
	deleteCmd.Flags().BoolVar(&deleteYesFlag, "yes", false, "Skip confirmation prompt")
	rootCmd.AddCommand(deleteCmd)
}
