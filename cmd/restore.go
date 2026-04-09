package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var restoreCmd = &cobra.Command{
	Use:   "restore <codename> <snapshot>",
	Short: "Restore a work to a previous snapshot state",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("kerf restore: not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(restoreCmd)
}
