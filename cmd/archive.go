package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var archiveCmd = &cobra.Command{
	Use:   "archive <codename>",
	Short: "Move a work to archive storage",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("kerf archive: not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(archiveCmd)
}
