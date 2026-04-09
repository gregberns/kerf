package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var historyCmd = &cobra.Command{
	Use:   "history <codename>",
	Short: "Show the version history of a work",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("kerf history: not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(historyCmd)
}
