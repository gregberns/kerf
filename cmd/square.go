package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var squareCmd = &cobra.Command{
	Use:   "square <codename>",
	Short: "Verify work completeness against jig requirements",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("kerf square: not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(squareCmd)
}
