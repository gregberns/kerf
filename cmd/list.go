package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Show all works on the bench",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("kerf list: not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
