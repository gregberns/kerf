package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var projectFlag string

var rootCmd = &cobra.Command{
	Use:   "kerf",
	Short: "Measure twice, cut once.",
	Long: `kerf — spec-writing CLI for AI agents.

Measure twice, cut once.

kerf manages works on a bench at ~/.kerf/. Each work follows a jig
(workflow template) through a series of passes from creation to finalization.

Standard workflow:
  kerf new               Create a new work
  kerf status <codename> Check or update work status
  kerf shelve [codename] Pause work with state preservation
  kerf resume <codename> Resume a shelved work
  kerf finalize <codename> --branch <name>  Complete and hand off to git

Useful commands:
  kerf list              Show all works on the bench
  kerf show <codename>   View full work details
  kerf square <codename> Verify work completeness
  kerf jig list          Show available jigs

Run 'kerf <command> --help' for details on any command.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("kerf — Measure twice, cut once.")
		fmt.Println()
		fmt.Println("Run 'kerf --help' for usage information.")
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&projectFlag, "project", "", "Override project inference with this project ID")
}
