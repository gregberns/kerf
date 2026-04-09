package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var jigCmd = &cobra.Command{
	Use:   "jig",
	Short: "Manage jig definitions",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("kerf jig: not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(jigCmd)
}
