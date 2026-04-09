package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status <codename> [new-status]",
	Short: "Get or set a work's status",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("kerf status: not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
