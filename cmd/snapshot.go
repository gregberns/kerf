package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var snapshotCmd = &cobra.Command{
	Use:   "snapshot <codename>",
	Short: "Manually trigger a versioning snapshot",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("kerf snapshot: not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(snapshotCmd)
}
