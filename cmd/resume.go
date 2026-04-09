package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var resumeCmd = &cobra.Command{
	Use:   "resume <codename>",
	Short: "Load context for resuming work on a shelved work",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("kerf resume: not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(resumeCmd)
}
