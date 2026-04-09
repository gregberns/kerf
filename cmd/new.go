package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var newCmd = &cobra.Command{
	Use:   "new [codename]",
	Short: "Create a new work on the bench",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("kerf new: not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(newCmd)
}
