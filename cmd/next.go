package cmd

// kerf next — ranked feed of bead/cleanup/warning items per Plan 006 / B6.
//
// Spec references:
//   - specs/commands.md §"kerf next" — syntax, flags, behavior, output,
//     six-element help-text contract, errors.
//   - specs/coordination.md — cleanup tie-break, filter resolution.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/gberns/kerf/internal/beads"
	"github.com/gberns/kerf/internal/cmdutil"
	"github.com/gberns/kerf/internal/config"
	"github.com/gberns/kerf/internal/dep"
	"github.com/gberns/kerf/internal/feed"
	"github.com/gberns/kerf/internal/queue"
	"github.com/gberns/kerf/internal/spec"
)

// Flag-backed variables. Slice flags are repeatable; --kinds is a comma list
// that we split on read.
var (
	nextOnly    []string
	nextInclude []string
	nextKinds   string
	nextFormat  string
)

// Empty-feed text per spec.
const nextEmptyText = "No items. Run 'kerf new' to start a work, or check 'kerf list' for in-progress works."

// Footer tip per spec sample output.
const nextFooterTip = "run with --format=json for machine output, --help for filters"

// Help-text contract — six elements in fixed order per specs/commands.md
// §"kerf next" → "Help text". Tests assert ordering; changing this text
// requires a spec change.
const nextLongHelp = `Returns a ranked feed of things to act on right now.

Item kinds:
  bead     — a ready bead to work on next.
  cleanup  — a work owes a follow-up: walk the jig, advance status, or shelve.
  warning  — a project-level issue (typically a misconfiguration). Fix config, not code.

Default action loop:
  1. Run 'kerf next'.
  2. Do the top item.
  3. Re-run 'kerf next'.

Filter flags:
  --only=<kind>      Restrict the feed to one kind. Repeatable. e.g. --only=bead
  --include=<kind>   Add a kind to the feed. Repeatable. e.g. --include=warning
  --kinds=a,b        Replace the default kind set. e.g. --kinds=bead,cleanup

Machine output:
  --format=json      Emit one record per item for scripts. Default is text.

Scoring:
  Beads rank by dependency fan-out, momentum, rework, and creation order; cleanups
  follow beads ordered by parent-work score; warnings render as a header block.
  See specs/coordination.md §"Computed Priority" for the full algorithm.`

var nextCmd = &cobra.Command{
	Use:   "next",
	Short: "Ranked feed of beads, cleanup tasks, and warnings",
	Long:  nextLongHelp,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runNext(cmd)
	},
}

func init() {
	nextCmd.Flags().StringArrayVar(&nextOnly, "only", nil, "Restrict to items of this kind (repeatable)")
	nextCmd.Flags().StringArrayVar(&nextInclude, "include", nil, "Add a kind to the feed (repeatable)")
	nextCmd.Flags().StringVar(&nextKinds, "kinds", "", "Comma-separated list of kinds to show")
	nextCmd.Flags().StringVar(&nextFormat, "format", "text", "Output format: text or json")
	rootCmd.AddCommand(nextCmd)
}

func runNext(cmd *cobra.Command) error {
	// --- Validate --format -----------------------------------------------
	format := strings.ToLower(strings.TrimSpace(nextFormat))
	if format == "" {
		format = "text"
	}
	if format != "text" && format != "json" {
		return fmt.Errorf("unknown format '%s'. Supported: text, json", nextFormat)
	}

	// --- Resolve --kinds, --only, --include into a KindSelection ---------
	var kindsList []string
	if strings.TrimSpace(nextKinds) != "" {
		for _, p := range strings.Split(nextKinds, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				kindsList = append(kindsList, p)
			}
		}
	}
	sel, err := feed.ResolveKindSelection(kindsList, nextOnly, nextInclude)
	if err != nil {
		// Spec error message: "unknown item kind '{value}'. Known kinds: ..."
		bad := firstUnknownKind(kindsList, nextOnly, nextInclude)
		return fmt.Errorf("unknown item kind '%s'. Known kinds: %s", bad, knownKindsList())
	}

	// --- Resolve project + storage ---------------------------------------
	projectID, err := cmdutil.ResolveProject(projectFlag)
	if err != nil {
		return err
	}
	r, err := cmdutil.Resolver(projectID)
	if err != nil {
		return err
	}

	// --- Detect missing project.yaml (no_project_yaml warning) -----------
	// Per specs/commands.md §"kerf next" §"Warning kinds" →
	// `no_project_yaml`: when the project resolves but project.yaml is
	// absent, emit a fatal warning and suppress feed assembly. This
	// replaces a previously silent zero-config feed.
	noProjectYAML := false
	if _, statErr := os.Stat(r.ProjectConfigPath()); statErr != nil && os.IsNotExist(statErr) {
		noProjectYAML = true
	}

	// --- Load works ------------------------------------------------------
	codenames, err := r.ListWorks()
	if err != nil {
		return err
	}
	works := make([]*spec.SpecYAML, 0, len(codenames))
	workCreated := make(map[string]time.Time, len(codenames))
	archivedOrFinalized := make(map[string]bool)
	// Collect per-work spec.yaml parse failures; surfaced as `corrupt_spec`
	// warnings rather than silently skipped (Plan 008 / B10-code;
	// specs/commands.md §"Warning kinds" → `corrupt_spec`).
	var corruptSpecs []feed.CorruptSpec
	for _, cn := range codenames {
		s, rerr := spec.Read(filepath.Join(r.WorkDir(cn), "spec.yaml"))
		if rerr != nil {
			corruptSpecs = append(corruptSpecs, feed.CorruptSpec{
				Codename:   cn,
				ParseError: rerr.Error(),
			})
			continue
		}
		works = append(works, s)
		workCreated[s.Codename] = s.Created
		// Per specs/_index.md Invariant 5, status is an open string: the CLI
		// warns on unrecognized values but does not enforce. An unknown status
		// (i.e. one not in the work's status_values) must NOT cause the work
		// to be dropped from the feed — it is treated as still actionable.
		// We therefore exclude only on the literal terminal sentinel
		// "finalized"; anything else (known intermediate values or unknown
		// strings) remains visible. See Bead kerf-1dm (Plan 008 / B4).
		if s.Status == "finalized" {
			archivedOrFinalized[s.Codename] = true
		}
	}
	// Also walk archived works for the exclusion set.
	if archived, _ := r.ListArchivedWorks(); len(archived) > 0 {
		for _, cn := range archived {
			archivedOrFinalized[cn] = true
		}
	}

	// --- Load project config (project bead_filter + queue weights) -------
	projCfg, _ := config.LoadProjectConfig(r.ProjectConfigPath())
	var projectFilter *beads.Filter
	if projCfg != nil {
		projectFilter = projCfg.BeadFilter
	}

	// --- Load beads ------------------------------------------------------
	var allBeads []beads.Bead
	if beads.IsAvailable() {
		allBeads, _ = beads.List()
	}

	// --- Build bead summaries per work + bead→work join map --------------
	// The join map (feed.Input.BeadToWork) is what feed.BeadSource consults
	// to attach beads to works. A bead matching N works appears with a
	// slice of length N — multi-match emits N items in BeadSource (Plan
	// 008 / Bead 3). Works are iterated in their slice order so the per-
	// bead match slice is deterministic across runs.
	beadsByWork := make(map[string]beads.EpicSummary)
	beadToWork := make(map[string][]string)
	for _, w := range works {
		resolvedFilter := beads.Resolve(w.BeadFilter, projectFilter)
		wb := beads.ForWorkWithFilter(allBeads, w.Codename, resolvedFilter)
		for _, b := range wb {
			beadToWork[b.ID] = append(beadToWork[b.ID], w.Codename)
		}
		if len(wb) == 0 {
			continue
		}
		summary := beads.EpicSummary{Total: len(wb)}
		for _, b := range wb {
			switch strings.ToLower(b.Status) {
			case "closed", "complete", "completed", "done":
				summary.Complete++
			case "blocked":
				summary.Blocked++
			case "in-progress", "in_progress", "active", "wip":
				summary.InProgress++
			}
		}
		summary.Rework = beads.ReworkCount(wb)
		beadsByWork[w.Codename] = summary
	}

	// --- Resolve queue weights and compute scores ------------------------
	defaults := config.ResolvedQueueWeights{
		FanOut:   queue.WeightFanOut,
		Momentum: queue.WeightMomentum,
		Creation: queue.WeightCreation,
		Rework:   queue.WeightRework,
	}
	resolved := projCfg.QueueWeights(defaults)
	weights := queue.Weights{
		FanOut:   resolved.FanOut,
		Momentum: resolved.Momentum,
		Creation: resolved.Creation,
		Rework:   resolved.Rework,
	}
	entries := queue.Compute(works, beadsByWork, weights)

	// --- Compute BlockedWorks (must-complete-first not met) --------------
	workByName := make(map[string]*spec.SpecYAML, len(works))
	for _, w := range works {
		workByName[w.Codename] = w
	}
	blocked := make(map[string]bool)
	for _, w := range works {
		for _, d := range w.DependsOn {
			if d.Relationship != "must-complete-first" {
				continue
			}
			dw, ok := workByName[d.Codename]
			if !ok {
				continue
			}
			if !dep.IsComplete(dw.Status, dw.StatusValues) {
				blocked[w.Codename] = true
				break
			}
		}
	}

	// --- Build feed.Input ------------------------------------------------
	in := feed.Input{
		Works:               works,
		AllBeads:            allBeads,
		QueueEntries:        entries,
		ProjectID:           projectID,
		ProjectFilter:       projectFilter,
		WorkCreated:         workCreated,
		BlockedWorks:        blocked,
		ArchivedOrFinalized: archivedOrFinalized,
		BeadToWork:          beadToWork,
		CorruptSpecs:        corruptSpecs,
		NoProjectYAML:       noProjectYAML,
	}

	// --- Run sources + detectors -----------------------------------------
	beadItems := feed.BeadSource(in)
	var cleanupItems []feed.Item
	for _, d := range feed.NewCleanupDetectors(projectFilter) {
		cleanupItems = append(cleanupItems, d.Detect(in)...)
	}
	var warningItems []feed.Item
	for _, d := range feed.NewWarningDetectors(projectFilter) {
		warningItems = append(warningItems, d.Detect(in)...)
	}

	// --- Assemble + exclusion (beads-then-cleanups; warnings separate) ---
	main, warnings := feed.AssembleWithWarnings(beadItems, cleanupItems, warningItems, in)

	// --- Apply kind selection --------------------------------------------
	main = feed.ApplyKindFilter(main, sel)
	if !sel.Has(feed.KindWarning) {
		warnings = nil
	}

	// --- Render ----------------------------------------------------------
	// Fatal warning handling per specs/commands.md §"Warning kinds" and
	// §"Errors": when `no_project_yaml` fires, suppress the feed listing
	// and exit non-zero with the documented error message. The warning
	// itself is printed first.
	out := cmd.OutOrStdout()
	if noProjectYAML {
		main = nil
		if format == "json" {
			if jerr := renderNextJSON(out, main, warnings); jerr != nil {
				return jerr
			}
		} else {
			if rerr := renderNextText(out, main, warnings); rerr != nil {
				return rerr
			}
		}
		return errors.New("no project.yaml — run 'kerf init'.")
	}
	switch format {
	case "json":
		return renderNextJSON(out, main, warnings)
	default:
		return renderNextText(out, main, warnings)
	}
}

// renderNextText renders the feed in compact human-readable form. Warnings
// render first as a header block, followed by the ranked main feed.
func renderNextText(out io.Writer, main, warnings []feed.Item) error {
	if len(main) == 0 && len(warnings) == 0 {
		fmt.Fprintln(out, nextEmptyText)
		return nil
	}

	// Warning header block.
	for _, w := range warnings {
		fmt.Fprintf(out, "warning: %s — %s\n", w.Title, w.Action)
		if w.Reason != "" {
			fmt.Fprintf(out, "         %s\n", w.Reason)
		}
	}
	if len(warnings) > 0 && len(main) > 0 {
		fmt.Fprintln(out)
	}

	// Ranked items.
	for i, it := range main {
		switch it.Kind {
		case feed.KindBead:
			id := ""
			if it.BeadID != nil {
				id = *it.BeadID
			}
			wc := ""
			if it.WorkCodename != nil {
				wc = *it.WorkCodename
			}
			line := fmt.Sprintf("%d. bead   %s  %q", i+1, id, it.Title)
			if wc != "" {
				line += fmt.Sprintf("  work: %s", wc)
			}
			fmt.Fprintln(out, line)
		case feed.KindCleanup:
			wc := ""
			if it.WorkCodename != nil {
				wc = *it.WorkCodename
			}
			reason := it.Reason
			if reason == "" {
				reason = it.Title
			}
			fmt.Fprintf(out, "%d. clean  %s   %s\n", i+1, wc, reason)
			if it.Action != "" {
				fmt.Fprintf(out, "          %s\n", it.Action)
			}
		case feed.KindWarning:
			// Defensive: warnings normally render in the header block.
			fmt.Fprintf(out, "%d. warn   %s — %s\n", i+1, it.Title, it.Action)
		}
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, nextFooterTip)
	return nil
}

// renderNextJSON renders the full item stream including warnings. Empty feed
// emits `[]`. WorkCodename / BeadID emit literal null for non-bead items per
// the spec (feed.Item enforces this via pointer types and no omitempty).
func renderNextJSON(out io.Writer, main, warnings []feed.Item) error {
	combined := make([]feed.Item, 0, len(main)+len(warnings))
	combined = append(combined, warnings...)
	combined = append(combined, main...)
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(combined)
}

// firstUnknownKind returns the first invalid kind token encountered while
// walking the same order ResolveKindSelection does (kinds → only → include).
// Used to produce the spec's unknown-kind error message.
func firstUnknownKind(kinds, only, include []string) string {
	for _, s := range append(append(append([]string{}, kinds...), only...), include...) {
		if _, err := feed.ParseKind(s); err != nil {
			return s
		}
	}
	return ""
}

// knownKindsList returns "bead, cleanup, warning" — the lowercase list used
// in the unknown-kind error message.
func knownKindsList() string {
	parts := make([]string, 0, len(feed.KnownKinds()))
	for _, k := range feed.KnownKinds() {
		parts = append(parts, string(k))
	}
	return strings.Join(parts, ", ")
}
