// Command kerfsim drives the kerf queue simulator.
//
// `kerfsim` is invoked separately from `kerf`. It has no shared state, no
// shared config, and no implicit project context.
//
// Subcommands:
//
//	kerfsim diff <runA-dir> <runB-dir>   compare two run outputs
//
// Future beads will add `run` and `sweep`. This binary intentionally hosts
// only what is currently implemented; missing subcommands are reported as
// "unknown command" by cobra.
//
// Spec: specs/simulator.md §CLI.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "kerfsim",
		Short:         "Queue simulator for kerf.",
		Long:          "kerfsim runs queue simulations and compares their outputs.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newDiffCmd())
	return root
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "kerfsim:", err)
		os.Exit(1)
	}
}
