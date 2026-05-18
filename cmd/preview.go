package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gberns/kerf/internal/bench"
	"github.com/gberns/kerf/internal/cmdutil"
	"github.com/gberns/kerf/internal/jig"
)

var previewCmd = &cobra.Command{
	Use:   "preview <codename> <status>",
	Short: "Render a future pass's instructions without advancing status",
	Args:  cobra.ExactArgs(2),
	RunE:  runPreview,
}

func init() {
	rootCmd.AddCommand(previewCmd)
}

func runPreview(cmd *cobra.Command, args []string) error {
	codename := args[0]
	status := args[1]

	projectID, err := cmdutil.ResolveProject(projectFlag)
	if err != nil {
		return err
	}

	s, _, err := cmdutil.LoadWorkWithChecks(projectID, codename)
	if err != nil {
		return fmt.Errorf("work '%s' not found in project '%s'", codename, projectID)
	}

	bp, _ := bench.BenchPath()
	jigsDir := filepath.Join(bp, "jigs")
	jigDef, _, err := jig.Resolve(s.Jig, jigsDir)
	if err != nil || jigDef == nil {
		return fmt.Errorf("jig '%s' not found for work '%s'", s.Jig, codename)
	}

	pass := jigDef.PassForStatus(status)
	if pass == nil {
		known := strings.Join(jigDef.StatusValues, ", ")
		return fmt.Errorf("status '%s' is not declared in jig '%s'. Known statuses: %s", status, jigDef.Name, known)
	}

	// Header — marks this as read-only so an agent does not mistake it for a
	// transition confirmation. Format per specs/commands.md §kerf preview.
	fmt.Printf("PREVIEW (read-only)\n")
	fmt.Printf("Preview for %s — pass: %s (read-only, status unchanged)\n\n", codename, pass.Name)

	// Body — reuse the same renderer kerf show uses.
	instructions := jigDef.InstructionsForPass(pass.Name)
	if instructions != "" {
		fmt.Println(instructions)
		fmt.Println()
	}

	// Output line — show all of this pass's outputs so the agent knows what
	// artifacts the pass produces.
	for _, out := range pass.Output {
		fmt.Printf("Output: %s\n", out)
	}

	return nil
}
