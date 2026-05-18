// Package cmd — `kerf bootstrap-filters` subcommand.
//
// Plan 019 / B5 (kerf-a7t). Implements `kerf bootstrap-filters` per
// specs/commands.md §"kerf bootstrap-filters".
//
// Scans every active work in the project, identifies those whose resolved
// bead_filter matches zero open beads (rank labels `empty` / `unwired`), and
// asks the internal/labelsample package (B4 / kerf-iak) to propose a clause
// per eligible work. Default mode is dry-run: it prints the proposals and
// exits without changes. With `--apply` (or `--yes`, which implies `--apply`),
// the accepted proposals are written via the same comment-preserving mutator
// path that `kerf work edit --bead-filter-add` uses (spec.AddBeadFilterClause).
package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gberns/kerf/internal/beads"
	"github.com/gberns/kerf/internal/cmdutil"
	"github.com/gberns/kerf/internal/config"
	"github.com/gberns/kerf/internal/labelsample"
	"github.com/gberns/kerf/internal/spec"
)

var (
	bootstrapFiltersApply    bool
	bootstrapFiltersYes      bool
	bootstrapFiltersCodename []string
	bootstrapFiltersFormat   string
)

var bootstrapFiltersCmd = &cobra.Command{
	Use:   "bootstrap-filters",
	Short: "Propose a bead_filter for every work whose resolved filter matches zero open beads",
	Long: `Propose a bead_filter for every work whose resolved filter matches zero open beads.

bootstrap-filters is the one-shot remediation for the 'empty' and 'unwired'
rank labels surfaced by 'kerf next'. For each eligible work, the sampler
considers a candidate label set (the codename, the codename combined with
common prefixes — codename:, subsystem:, area:, kind: — and the bare slug),
counts open-bead matches per candidate, and proposes either a single dominant
clause or an 'any:' union.

Default mode is dry-run: the proposals are printed and no spec.yaml is
touched. Pass --apply to write the proposals (with a confirmation prompt),
or --yes to apply without prompting.

Examples:
  kerf bootstrap-filters                            Dry-run preview for every eligible work
  kerf bootstrap-filters --codename bridge          Restrict to a single work
  kerf bootstrap-filters --yes                      Apply all proposals without prompting
  kerf bootstrap-filters --apply                    Apply with a single confirmation prompt
`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBootstrapFilters(cmd.OutOrStdout(), cmd.InOrStdin())
	},
}

func init() {
	bootstrapFiltersCmd.Flags().BoolVar(&bootstrapFiltersApply, "apply", false,
		"Mutate spec.yaml for each accepted proposal. Without --apply, the command is a dry-run preview.")
	bootstrapFiltersCmd.Flags().BoolVar(&bootstrapFiltersYes, "yes", false,
		"Skip the confirmation prompt and apply all proposals. Implies --apply.")
	bootstrapFiltersCmd.Flags().StringArrayVar(&bootstrapFiltersCodename, "codename", nil,
		"Restrict bootstrap to the named work(s). Repeatable.")
	bootstrapFiltersCmd.Flags().StringVar(&bootstrapFiltersFormat, "format", "text",
		"Output format: text (default) or json.")
	rootCmd.AddCommand(bootstrapFiltersCmd)
}

// proposalReport is the per-work record the command renders (text or json).
type proposalReport struct {
	Codename     string                `json:"codename"`
	Reason       string                `json:"reason"` // labelsample.Reason.String()
	Clauses      []string              `json:"clauses,omitempty"`
	MatchCount   int                   `json:"match_count"`
	HadFilter    bool                  `json:"had_filter"`
	Applied      bool                  `json:"applied,omitempty"`
	ApplyError   string                `json:"apply_error,omitempty"`
	Candidates   []candidateReport     `json:"candidates,omitempty"`
}

type candidateReport struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

func runBootstrapFilters(stdout io.Writer, stdin io.Reader) error {
	switch bootstrapFiltersFormat {
	case "text", "json":
	default:
		return fmt.Errorf("unknown --format %q. Supported: text, json", bootstrapFiltersFormat)
	}

	// --yes implies --apply (per spec: "no error").
	apply := bootstrapFiltersApply || bootstrapFiltersYes

	projectID, err := cmdutil.ResolveProject(projectFlag)
	if err != nil {
		return err
	}

	r, err := cmdutil.Resolver(projectID)
	if err != nil {
		return err
	}

	// Load tool name for the bead store (honors project.yaml tools.tasks).
	toolName := beads.DefaultToolName
	var projectFilter *beads.Filter
	if cfg, cerr := config.LoadProjectConfig(r.ProjectConfigPath()); cerr == nil && cfg != nil {
		toolName = beads.ResolveToolName(cfg.Tools)
		projectFilter = cfg.BeadFilter
	}

	allBeads, err := beads.ListNamed(toolName)
	if err != nil {
		return fmt.Errorf("cannot read bead store: %v", err)
	}

	// The sampler operates on open beads only — closed beads do not count
	// as evidence for a proposal.
	openBeads := make([]beads.Bead, 0, len(allBeads))
	for _, b := range allBeads {
		if !isClosedStatus(b.Status) {
			openBeads = append(openBeads, b)
		}
	}

	// Build the list of works to consider.
	allCodenames, err := r.ListWorks()
	if err != nil {
		return err
	}
	sort.Strings(allCodenames)

	want := map[string]bool{}
	if len(bootstrapFiltersCodename) > 0 {
		for _, cn := range bootstrapFiltersCodename {
			want[cn] = true
		}
		// Validate that every supplied --codename exists.
		known := map[string]bool{}
		for _, cn := range allCodenames {
			known[cn] = true
		}
		for cn := range want {
			if !known[cn] {
				return fmt.Errorf("work '%s' not found in project '%s'", cn, projectID)
			}
		}
	}

	// Identify eligible works: those whose resolved filter matches zero open
	// beads. This includes works with no per-work filter (the default filter
	// applies and matches nothing in a typical bootstrap scenario) and works
	// with an explicit but empty-yielding filter.
	type eligible struct {
		codename  string
		specPath  string
		hadFilter bool
	}
	var elig []eligible
	for _, cn := range allCodenames {
		if len(want) > 0 && !want[cn] {
			continue
		}
		specPath := filepath.Join(r.WorkDir(cn), "spec.yaml")
		s, err := spec.Read(specPath)
		if err != nil {
			// Corrupt or missing spec.yaml — skip silently; `kerf list`
			// already surfaces the warning.
			continue
		}
		resolved := beads.Resolve(s.BeadFilter, projectFilter)
		matches := beads.ForWorkWithFilter(openBeads, cn, resolved)
		if len(matches) == 0 {
			elig = append(elig, eligible{
				codename:  cn,
				specPath:  specPath,
				hadFilter: s.BeadFilter != nil,
			})
		}
	}

	// Run the sampler against each eligible work.
	reports := make([]proposalReport, 0, len(elig))
	for _, e := range elig {
		p := labelsample.ProposeFilter(openBeads, e.codename)
		rep := proposalReport{
			Codename:   e.codename,
			Reason:     p.Reason.String(),
			MatchCount: p.MatchCount,
			HadFilter:  e.hadFilter,
		}
		if p.Filter != nil {
			rep.Clauses = filterToClauseStrings(p.Filter)
		}
		for _, c := range p.Candidates {
			rep.Candidates = append(rep.Candidates, candidateReport{Label: c.Label, Count: c.Count})
		}
		reports = append(reports, rep)
	}

	// If apply mode is on, optionally prompt and then mutate.
	if apply && hasProposals(reports) {
		if !bootstrapFiltersYes {
			if !confirmApply(stdout, stdin, reports, projectID) {
				fmt.Fprintln(stdout, "Aborted; no changes made.")
				return nil
			}
		}
		for i := range reports {
			if len(reports[i].Clauses) == 0 {
				continue
			}
			specPath := filepath.Join(r.WorkDir(reports[i].Codename), "spec.yaml")
			applied := true
			for _, clause := range reports[i].Clauses {
				if err := spec.AddBeadFilterClause(specPath, clause); err != nil {
					reports[i].ApplyError = err.Error()
					applied = false
					break
				}
			}
			reports[i].Applied = applied
		}
	}

	// Render.
	if bootstrapFiltersFormat == "json" {
		return emitBootstrapJSON(stdout, projectID, reports, apply)
	}
	return emitBootstrapText(stdout, projectID, reports, apply)
}

// hasProposals reports whether at least one report carries a clause to write.
func hasProposals(reports []proposalReport) bool {
	for _, r := range reports {
		if len(r.Clauses) > 0 {
			return true
		}
	}
	return false
}

// confirmApply prints a brief preview and asks the operator to confirm. The
// preview shape matches the text-mode summary so the operator sees exactly
// what would be written. Returns true on "y" / "yes" (case-insensitive).
func confirmApply(stdout io.Writer, stdin io.Reader, reports []proposalReport, projectID string) bool {
	fmt.Fprintf(stdout, "About to write bead_filter proposals for %s:\n\n", projectID)
	for _, r := range reports {
		if len(r.Clauses) == 0 {
			continue
		}
		fmt.Fprintf(stdout, "  %-12s %s\n", r.Codename, formatClauseSummary(r))
	}
	fmt.Fprint(stdout, "\nProceed? [y/N]: ")

	var resp string
	fmt.Fscanln(stdin, &resp)
	resp = strings.TrimSpace(strings.ToLower(resp))
	return resp == "y" || resp == "yes"
}

// filterToClauseStrings serialises a Filter into the clause string form
// accepted by spec.AddBeadFilterClause / `kerf work edit --bead-filter-add`.
// For a leaf filter this returns one element; for an `any:` union it returns
// one element per member (the mutator will lift them into the union shape on
// successive calls).
func filterToClauseStrings(f *beads.Filter) []string {
	if f == nil {
		return nil
	}
	if len(f.Any) > 0 {
		var out []string
		for i := range f.Any {
			out = append(out, filterToClauseStrings(&f.Any[i])...)
		}
		return out
	}
	switch {
	case f.Label != "":
		return []string{"label=" + f.Label}
	case f.IDPrefix != "":
		return []string{"id_prefix=" + f.IDPrefix}
	}
	return nil
}

// formatClauseSummary renders the clause-and-match-count phrase used in both
// the dry-run preview and the apply-confirmation prompt.
func formatClauseSummary(r proposalReport) string {
	switch len(r.Clauses) {
	case 0:
		return ""
	case 1:
		return fmt.Sprintf("%-30s (would match %d open beads)", r.Clauses[0], r.MatchCount)
	default:
		return fmt.Sprintf("any: [%s] (would match %d open beads)", strings.Join(r.Clauses, ", "), r.MatchCount)
	}
}

// reasonNote renders the explanatory line for a no-proposal report.
func reasonNote(r proposalReport) string {
	switch r.Reason {
	case "no-match":
		return fmt.Sprintf("no proposal — no label resembles '%s' in the bead store", r.Codename)
	case "below-floor":
		return fmt.Sprintf("no proposal — candidate labels matched but no clause cleared the confidence floor")
	}
	return "no proposal"
}

func emitBootstrapText(stdout io.Writer, projectID string, reports []proposalReport, apply bool) error {
	if len(reports) == 0 {
		fmt.Fprintf(stdout, "No works in %s need bootstrap — every active work's bead_filter already matches at least one open bead.\n", projectID)
		return nil
	}

	fmt.Fprintf(stdout, "Bootstrap proposals for %s:\n\n", projectID)
	proposed, applied, noProp := 0, 0, 0
	for _, r := range reports {
		if len(r.Clauses) == 0 {
			noProp++
			fmt.Fprintf(stdout, "  %-12s %s\n", r.Codename, reasonNote(r))
			continue
		}
		proposed++
		line := formatClauseSummary(r)
		if apply {
			switch {
			case r.Applied:
				applied++
				fmt.Fprintf(stdout, "  %-12s applied %s\n", r.Codename, line)
			case r.ApplyError != "":
				fmt.Fprintf(stdout, "  %-12s ERROR applying %s: %s\n", r.Codename, line, r.ApplyError)
			default:
				fmt.Fprintf(stdout, "  %-12s skipped %s\n", r.Codename, line)
			}
		} else {
			fmt.Fprintf(stdout, "  %-12s proposes %s\n", r.Codename, line)
		}
	}
	fmt.Fprintln(stdout)
	if apply {
		fmt.Fprintf(stdout, "Summary: %d proposed, %d applied, %d without proposal.\n", proposed, applied, noProp)
	} else {
		fmt.Fprintf(stdout, "Summary: %d proposed, %d without proposal.\n", proposed, noProp)
		fmt.Fprintln(stdout, "Dry-run: no changes made. Re-run with --apply to write the proposals to spec.yaml.")
	}
	return nil
}

func emitBootstrapJSON(stdout io.Writer, projectID string, reports []proposalReport, apply bool) error {
	record := struct {
		ProjectID string           `json:"project_id"`
		Apply     bool             `json:"apply"`
		Reports   []proposalReport `json:"reports"`
	}{ProjectID: projectID, Apply: apply, Reports: reports}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(record)
}

// isClosedStatus mirrors the (unexported) isComplete in internal/beads — kept
// local so cmd does not reach into beads' private surface.
func isClosedStatus(s string) bool {
	switch strings.ToLower(s) {
	case "closed", "done", "complete":
		return true
	}
	return false
}

