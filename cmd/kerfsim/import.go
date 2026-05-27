// Command kerfsim — `import` subcommand.
//
// Converts an external workload description (harmonik pilot YAML or kerf
// plan directory) into a scenario YAML the existing `kerfsim run` can
// execute. Phase 1 supports harmonik pilots; kerf-plan ingestion is
// stubbed for follow-up.
//
// Spec: specs/sim_corpus.md.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/gregberns/kerf/internal/sim/scenario_import"
)

type importOpts struct {
	outPath string
}

func newImportCmd() *cobra.Command {
	opts := &importOpts{}
	cmd := &cobra.Command{
		Use:   "import <source>",
		Short: "Convert a bead/work corpus into a kerfsim scenario YAML.",
		Long: `Import a workload description and write a scenario YAML.

<source> is either:
  - a harmonik pilot YAML file (e.g. cp-pilot-data.yaml)
  - a directory containing one or more *-pilot-data.yaml files
  - a kerf plan directory (plans/NNN_name/) — NOT YET IMPLEMENTED

Each pilot becomes one work in the output scenario. The agent_model and
bead_arrivals blocks carry placeholder defaults; tune them via Plan 012
B-step3 fitted distributions before drawing conclusions.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runImport(cmd.OutOrStdout(), args[0], opts)
		},
	}
	cmd.Flags().StringVar(&opts.outPath, "out", "", "Output scenario YAML path (required)")
	_ = cmd.MarkFlagRequired("out")
	return cmd
}

func runImport(w io.Writer, source string, opts *importOpts) error {
	kind, err := scenario_import.Detect(source)
	if err != nil {
		return err
	}
	var res *scenario_import.ImportResult
	switch kind {
	case scenario_import.SourceHarmonikPilot:
		res, err = scenario_import.ImportHarmonik(source)
	case scenario_import.SourceKerfPlan:
		res, err = scenario_import.ImportKerfPlan(source)
	default:
		return fmt.Errorf("kerfsim import: unrecognized source %s", source)
	}
	if err != nil {
		return err
	}
	if err := res.Scenario.Validate(); err != nil {
		return fmt.Errorf("kerfsim import: generated scenario failed validation: %w", err)
	}
	body, err := scenario_import.MarshalScenario(res.Scenario, source, res.Notes)
	if err != nil {
		return err
	}
	if err := os.WriteFile(opts.outPath, body, 0o644); err != nil {
		return fmt.Errorf("kerfsim import: write %s: %w", opts.outPath, err)
	}
	fmt.Fprintf(w, "Imported %d pilot(s) from %s -> %s\n", len(res.Pilots), source, opts.outPath)
	for _, p := range res.Pilots {
		fmt.Fprintf(w, "  %s: beads=%d areas=%v deps=%v edges=%d\n",
			p.Mnem, p.BeadCount, p.Areas, p.Deps, p.EdgeCount)
	}
	for _, n := range res.Notes {
		fmt.Fprintf(w, "  note: %s\n", n)
	}
	return nil
}
