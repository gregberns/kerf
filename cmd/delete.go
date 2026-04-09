package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <codename>",
	Short: "Permanently remove a work from the bench",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("kerf delete: not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}
