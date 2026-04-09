package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var shelveCmd = &cobra.Command{
	Use:   "shelve [codename]",
	Short: "Pause work with state preservation",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("kerf shelve: not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(shelveCmd)
}
