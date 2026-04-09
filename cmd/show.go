package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show <codename>",
	Short: "Display full details for a work",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("kerf show: not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(showCmd)
}
