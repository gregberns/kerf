package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var finalizeCmd = &cobra.Command{
	Use:   "finalize <codename> --branch <name>",
	Short: "Complete a work and hand off to implementation",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("kerf finalize: not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(finalizeCmd)
}
