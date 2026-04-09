package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config [key] [value]",
	Short: "View or modify bench configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("kerf config: not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}
