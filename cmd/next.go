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
	"github.com/gberns/kerf/internal/doctor"
	"github.com/gberns/kerf/internal/drift"
	"github.com/gberns/kerf/internal/feed"
	"github.com/gberns/kerf/internal/labelsample"
	"github.com/gberns/kerf/internal/queue"
	"github.com/gberns/kerf/internal/spec"
	"github.com/gberns/kerf/internal/storage"
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

// advisorMinFloor is the absolute match-count floor the near-match advisor
// passes to labelsample.ProposeFilterWithFloor. Lower than the
// bootstrap-filters default (3) because the advisor only surfaces an inline
// hint — the user/agent reviews before applying — so a softer signal is
// acceptable. See kerf-fx5 and the comment in computeNearMatchHints.
const advisorMinFloor = 2

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
	// SilenceUsage: errors returned by runNext (notably the BEADS_TOOL_ERROR
	// surfaced when the configured `tools.tasks` subprocess fails) are user-
	// facing diagnostics, not flag-misuse signals. Suppress cobra's default
	// usage dump so scripts see only the single-line error before exit 1
	// (kerf-1d6 — sibling triage / doctor already behave this way in spirit;
	// next was the outlier that printed a help block on tool failure).
	SilenceUsage: true,
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
	// Honor project.yaml tools.tasks (default "br"). When the configured tool
	// is on PATH but invocation fails (bad store, JSON error), surface the
	// concrete error rather than silently returning zero beads — silent zero
	// was the misconfiguration trap behind plan 021.
	var allBeads []beads.Bead
	toolName := beads.DefaultToolName
	if projCfg != nil {
		toolName = beads.ResolveToolName(projCfg.Tools)
	}
	if beads.IsAvailableNamed(toolName) {
		bs, berr := beads.ListNamed(toolName)
		if berr != nil {
			return fmt.Errorf("loading beads: %w", berr)
		}
		allBeads = bs
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

	// --- Build PinAssignments from each work's spec.yaml.PinnedBeads -----
	// Single-owner invariant: a bead appears in at most one work's
	// PinnedBeads list. A two-owner conflict is defense-in-depth handled
	// by the pin_conflict warning (Plan 009 / B5); lexicographically-
	// earliest codename wins as the recorded assignment so detectors
	// converge to a stable picture (specs/coordination.md §"Pin layer").
	pinAssignments := map[string]string{}
	for _, w := range works {
		for _, bid := range w.PinnedBeads {
			if cur, ok := pinAssignments[bid]; ok {
				if w.Codename < cur {
					pinAssignments[bid] = w.Codename
				}
				continue
			}
			pinAssignments[bid] = w.Codename
		}
	}

	// Apply the pin layer to BeadToWork BEFORE BeadSource emits items
	// (specs/coordination.md §"Pin layer"; Plan 009 / Bead 5).
	beadToWork = feed.ResolvePins(beadToWork, pinAssignments)

	// --- Read drift baseline + compute drift -----------------------------
	// Cache absence is non-fatal: a zero-value Diff treats the baseline as
	// empty (first-run). The drift-summary headline is suppressed when no
	// baseline is recorded; the legacy `untriaged_beads` warning remains
	// the surface in that case (specs/commands.md §"kerf next" drift
	// summary line; coordination.md §"Drift detection").
	closedSet := map[string]bool{
		"closed":    true,
		"done":      true,
		"complete":  true,
		"completed": true,
	}
	repoRoot := ""
	if r != nil {
		repoRoot = r.RepoRoot
	}
	cachePath := drift.CachePath(repoRoot)
	var (
		driftDiff   drift.Diff
		hasBaseline bool
	)
	if cachePath != "" {
		baseline, ok, _ := drift.Read(cachePath)
		if ok {
			current := drift.Capture(allBeads, nil)
			driftDiff = drift.Compute(baseline, current, closedSet)
			hasBaseline = true
		}
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

	// --- T=0 static analyzer (Plan 014 / Bead B2 — kerf-n9vq) -----------
	// Appends graph-shape signals (critical-path, fan-out, area-overlap)
	// to each Entry.Reasons slice and returns the freshly-added strings
	// keyed by work codename. The signals later flow into each ranked
	// bead Item's `reason` field for `kerf next --format=json` consumers
	// (specs/coordination.md §"Graph signals"). Read-only decoration; no
	// scoring or ordering change in this bead — Plan 014/B3 owns that.
	graphSignals := queue.DecorateGraphSignals(entries, works)

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
		DriftResult:         driftDiff,
		PinAssignments:      pinAssignments,
	}

	// --- Run sources + detectors -----------------------------------------
	beadItems := feed.BeadSource(in)
	// Plan 014 / B2 (kerf-n9vq): surface T=0 graph signals on each bead
	// item's `reason` field. We append, joined with "; ", and only when
	// the parent work's codename has signals — beads attached to works
	// off the critical path with no fan-out / no overlap stay silent.
	if len(graphSignals) > 0 {
		for i := range beadItems {
			if beadItems[i].Kind != feed.KindBead || beadItems[i].WorkCodename == nil {
				continue
			}
			sigs := graphSignals[*beadItems[i].WorkCodename]
			if len(sigs) == 0 {
				continue
			}
			joined := strings.Join(sigs, "; ")
			if beadItems[i].Reason == "" {
				beadItems[i].Reason = joined
			} else {
				beadItems[i].Reason = beadItems[i].Reason + "; " + joined
			}
		}
	}
	var cleanupItems []feed.Item
	for _, d := range feed.NewCleanupDetectors(projectFilter) {
		cleanupItems = append(cleanupItems, d.Detect(in)...)
	}
	var warningItems []feed.Item
	for _, d := range feed.NewWarningDetectors(projectFilter) {
		warningItems = append(warningItems, d.Detect(in)...)
	}

	// --- Compute drift-summary counters (Plan 009 / Bead 11b) ------------
	// Per specs/commands.md §"kerf next" drift summary line:
	//   ! N untriaged beads · ! M beads multi-matched · ! K bead(s) changed externally — run 'kerf triage'
	// Each segment is omitted when its count is zero; the whole line is
	// omitted when all three are zero or when no baseline is recorded.
	// Counters are derived from the warning items already produced
	// (untriaged_beads → single warning, multi_matched → one per bead) and
	// from DriftResult directly (external_drift = sum of New + Deleted +
	// ClosedExternally + ReopenedExternally).
	driftSummary := computeDriftSummary(warningItems, driftDiff)

	// --- Near-match advisor (Plan 019 / B7 — kerf-d9f) -----------------
	// For each cleanup item rank-labelled `empty`, ask the B4 sampler
	// (internal/labelsample) whether a single dominant label-shape exists
	// for that codename in the bead store. When ReasonDominant fires, the
	// proposal is unambiguous — that is the exact "one prefix-swap"
	// condition the spec calls out — and we surface a `try:` hint inline
	// on the warning row. Ambiguous (union) and zero-match cases fall
	// through silently, matching specs/commands.md §"kerf next"
	// → "Near-match advisor".
	nearMatchHints := computeNearMatchHints(cleanupItems, allBeads)

	// --- Storage-drift footer (Plan 017 / B11 — kerf-cgb) -------------
	// Query the `storage-drift` detector from the doctor registry and
	// count non-green findings. The footer renders below the warning
	// stanza, just above the tail-tip footer (specs/commands.md §"kerf
	// next" → "Storage-drift footer"). Errors are swallowed: drift
	// detection is advisory on this path and must not fail `kerf next`.
	storageDriftCount := computeStorageDriftCount(projectID, r)
	// Suppression knob (Plan 017 / B13 — kerf-bwd). Either
	// `doctor.footer: false` in project.yaml or KERF_DOCTOR_FOOTER=0
	// elides the footer; the env var wins on conflict. Spec:
	// specs/architecture.md §"Project Configuration" → `doctor.footer`,
	// specs/commands.md §"Storage-drift footer".
	if !projCfg.DoctorFooterEnabled() {
		storageDriftCount = 0
	}

	// --- Assemble + exclusion (beads-then-cleanups; warnings separate) ---
	main, warnings := feed.AssembleWithWarnings(beadItems, cleanupItems, warningItems, in)

	// When the drift-summary line will render, the legacy
	// `untriaged_beads` warning is suppressed from the warning block: the
	// headline `untriaged` segment covers it (specs/commands.md §"kerf
	// next" — the older "warning: N beads match no work" line is omitted
	// from the same invocation). The legacy warning still renders when no
	// baseline is recorded and the headline is therefore suppressed.
	if hasBaseline && driftSummary.renders() {
		warnings = stripUntriagedWarning(warnings)
	}

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
			if jerr := renderNextJSON(out, main, warnings, driftSummary, hasBaseline); jerr != nil {
				return jerr
			}
		} else {
			if rerr := renderNextText(out, main, warnings, driftSummary, hasBaseline, nil, storageDriftCount); rerr != nil {
				return rerr
			}
		}
		return errors.New("no project.yaml — run 'kerf init'.")
	}
	switch format {
	case "json":
		return renderNextJSON(out, main, warnings, driftSummary, hasBaseline)
	default:
		return renderNextText(out, main, warnings, driftSummary, hasBaseline, nearMatchHints, storageDriftCount)
	}
}

// computeNearMatchHints walks cleanup items rank-labelled `empty` and asks
// labelsample.ProposeFilter for an unambiguous label-shape proposal. The
// returned map is keyed by work codename; values are the `try:` suffix
// (without the leading " — ") so the renderer can append them verbatim.
//
// Spec: specs/commands.md §"kerf next" → "Near-match advisor". The advisor
// only emits when exactly one alternate clause would lift the work out of
// `empty`; ambiguous (union) and zero-candidate cases stay silent.
//
// Implementation reuses internal/labelsample (Plan 019 / B4 — kerf-iak):
// ReasonDominant is the unambiguous case. ReasonUnion (≥ 2 viable
// candidates), ReasonBelowFloor, and ReasonNoMatch all yield no hint.
func computeNearMatchHints(cleanupItems []feed.Item, allBeads []beads.Bead) map[string]string {
	if len(cleanupItems) == 0 || len(allBeads) == 0 {
		return nil
	}
	hints := make(map[string]string)
	for _, it := range cleanupItems {
		if it.Kind != feed.KindCleanup || it.RankLabel != "empty" || it.WorkCodename == nil {
			continue
		}
		cn := *it.WorkCodename
		// kerf-fx5: the advisor uses a softer floor (2) than the
		// bootstrap-filters path (3). The hint surfaces inline on the
		// cleanup row — a human / agent reads it before applying — so a
		// thinner signal is acceptable. The strict floor caused the
		// dogfood-2026-05-18 miss (a work `gama` with `bead_filter:
		// label=gama` against 2 open beads carrying `codename:gama`
		// produced no advisor output because count<3 → ReasonBelowFloor).
		proposal := labelsample.ProposeFilterWithFloor(allBeads, cn, advisorMinFloor)
		if proposal.Reason != labelsample.ReasonDominant || proposal.Filter == nil {
			continue
		}
		clause := formatFilterClause(*proposal.Filter)
		if clause == "" {
			continue
		}
		hints[cn] = fmt.Sprintf("try: kerf work edit %s --bead-filter '%s'", cn, clause)
	}
	if len(hints) == 0 {
		return nil
	}
	return hints
}

// formatFilterClause renders a single-leaf beads.Filter back to its CLI
// string form ("label=<value>" or "id_prefix=<value>"). Returns "" for
// non-leaf shapes (Any/union) — the advisor only acts on dominant
// proposals, which are single-leaf by construction in labelsample.
func formatFilterClause(f beads.Filter) string {
	switch {
	case f.Label != "":
		return "label=" + f.Label
	case f.IDPrefix != "":
		return "id_prefix=" + f.IDPrefix
	default:
		return ""
	}
}

// driftSummaryCounts holds the three headline counters surfaced by `kerf
// next`. JSON output marshals as `drift_summary`; text rendering composes
// the headline string. See specs/commands.md §"kerf next" drift summary.
type driftSummaryCounts struct {
	Untriaged     int `json:"untriaged"`
	MultiMatched  int `json:"multi_matched"`
	ExternalDrift int `json:"external_drift"`
}

// renders reports whether the headline should appear: at least one
// non-zero counter.
func (d driftSummaryCounts) renders() bool {
	return d.Untriaged > 0 || d.MultiMatched > 0 || d.ExternalDrift > 0
}

// headline renders the spec-formatted summary line. Segments are omitted
// when their count is zero. Singular/plural forms match the spec example
// (`1 bead changed externally`, `N beads multi-matched`).
func (d driftSummaryCounts) headline() string {
	var parts []string
	if d.Untriaged > 0 {
		noun := "beads"
		if d.Untriaged == 1 {
			noun = "bead"
		}
		parts = append(parts, fmt.Sprintf("! %d untriaged %s", d.Untriaged, noun))
	}
	if d.MultiMatched > 0 {
		noun := "beads"
		if d.MultiMatched == 1 {
			noun = "bead"
		}
		parts = append(parts, fmt.Sprintf("! %d %s multi-matched", d.MultiMatched, noun))
	}
	if d.ExternalDrift > 0 {
		noun := "beads"
		if d.ExternalDrift == 1 {
			noun = "bead"
		}
		parts = append(parts, fmt.Sprintf("! %d %s changed externally", d.ExternalDrift, noun))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " · ") + " — run 'kerf triage'"
}

// computeDriftSummary derives the three counters from the already-produced
// warning items plus the raw DriftResult. Untriaged is taken from the
// single `untriaged_beads` warning's Reason field count (parsed back as a
// safety against re-implementing the detector's open-bead filter).
// Multi-matched is the number of warning items titled `multi_matched: ...`.
// External-drift sums the four `external_*` categories from drift.Diff —
// `Changed` is intentionally excluded (it surfaces via the relabel-drift
// detector and is not part of the headline per spec).
func computeDriftSummary(warningItems []feed.Item, d drift.Diff) driftSummaryCounts {
	var summary driftSummaryCounts
	for _, it := range warningItems {
		if it.Kind != feed.KindWarning {
			continue
		}
		switch {
		case it.Title == feed.WarningKindUntriagedBeads:
			summary.Untriaged = parseLeadingInt(it.Reason)
		case strings.HasPrefix(it.Title, feed.WarningKindMultiMatchedBead+":"):
			summary.MultiMatched++
		}
	}
	summary.ExternalDrift = len(d.New) + len(d.Deleted) +
		len(d.ClosedExternally) + len(d.ReopenedExternally)
	return summary
}

// parseLeadingInt returns the first integer found at the start of s, or 0
// when the string does not begin with digits. Used to recover the
// untriaged count from the detector's Reason field ("N beads match no
// work …").
func parseLeadingInt(s string) int {
	n := 0
	seen := false
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
		seen = true
	}
	if !seen {
		return 0
	}
	return n
}

// stripUntriagedWarning removes the single `untriaged_beads` warning item
// from ws. Used when the drift-summary headline is rendered: the headline
// covers the untriaged count and the legacy warning would be redundant
// (specs/commands.md §"kerf next" drift summary).
func stripUntriagedWarning(ws []feed.Item) []feed.Item {
	out := ws[:0]
	for _, w := range ws {
		if w.Kind == feed.KindWarning && w.Title == feed.WarningKindUntriagedBeads {
			continue
		}
		out = append(out, w)
	}
	return out
}

// renderNextText renders the feed in compact human-readable form. The
// payload-first ordering (Plan 019 / B3 — kerf-c1c) is per
// specs/commands.md §"kerf next" → "Default kind selection": ranked items
// render first, the one-line drift summary follows when any counter is
// non-zero, and the warning stanza renders last. This puts actionable work
// at the top of the agent's view; diagnostics tail the output.
func renderNextText(out io.Writer, main, warnings []feed.Item, summary driftSummaryCounts, hasBaseline bool, nearMatchHints map[string]string, storageDriftCount int) error {
	headlineRenders := hasBaseline && summary.renders()

	// Empty-feed fallback: nothing actionable, nothing to diagnose. The
	// storage-drift footer (Plan 017 / B11) still renders when present —
	// drift is an out-of-band signal independent of the ranked feed.
	if len(main) == 0 && len(warnings) == 0 && !headlineRenders {
		fmt.Fprintln(out, nextEmptyText)
		if storageDriftCount > 0 {
			noun := "findings"
			if storageDriftCount == 1 {
				noun = "finding"
			}
			fmt.Fprintln(out)
			fmt.Fprintf(out, "note: %d storage %s — run 'kerf doctor' for details\n", storageDriftCount, noun)
		}
		return nil
	}

	// --- Ranked payload first ------------------------------------------------
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
			// Rank label drives the leading word per specs/commands.md
			// §"kerf next" warning-block vocabulary (Plan 019 / B2):
			// empty / unwired / broken. Items without a rank label fall
			// back to "cleanup"; in practice only the
			// work_no_attached_beads detector sets a label today.
			label := it.RankLabel
			if label == "" {
				label = "cleanup"
			}
			// Near-match advisor (Plan 019 / B7 — kerf-d9f): when the
			// cleanup row is `empty` and the sampler produced an
			// unambiguous proposal, append the `try:` hint inline so it
			// stays adjacent to the reason. The hint replaces the
			// indented action line: agents only need one suggested next
			// step. For non-empty rows or no-hint cases, the original
			// two-line shape is preserved.
			hint := ""
			if label == "empty" && wc != "" {
				hint = nearMatchHints[wc]
			}
			if hint != "" {
				fmt.Fprintf(out, "%d. %-7s %s   %s — %s\n", i+1, label, wc, reason, hint)
			} else {
				fmt.Fprintf(out, "%d. %-7s %s   %s\n", i+1, label, wc, reason)
				if it.Action != "" {
					fmt.Fprintf(out, "          %s\n", it.Action)
				}
			}
		case feed.KindWarning:
			// Defensive: warnings normally render in the trailing stanza.
			fmt.Fprintf(out, "%d. warn   %s — %s\n", i+1, it.Title, it.Action)
		}
	}

	// --- Drift summary (single-line footer) ----------------------------------
	if headlineRenders {
		if len(main) > 0 {
			fmt.Fprintln(out)
		}
		fmt.Fprintln(out, summary.headline())
	}

	// --- Warning stanza (last) -----------------------------------------------
	if len(warnings) > 0 {
		if len(main) > 0 || headlineRenders {
			fmt.Fprintln(out)
		}
		for _, w := range warnings {
			fmt.Fprintf(out, "warning: %s — %s\n", w.Title, w.Action)
			if w.Reason != "" {
				fmt.Fprintf(out, "         %s\n", w.Reason)
			}
		}
	}

	// --- Storage-drift footer (Plan 017 / B11 — kerf-cgb) --------------------
	// Per specs/commands.md §"kerf next" → "Storage-drift footer": when
	// `kerf doctor` would report any non-green storage finding, append a
	// one-line footer below the warning stanza, above the tail-tip footer.
	// Silent when no drift is present.
	if storageDriftCount > 0 {
		noun := "findings"
		if storageDriftCount == 1 {
			noun = "finding"
		}
		fmt.Fprintln(out)
		fmt.Fprintf(out, "note: %d storage %s — run 'kerf doctor' for details\n", storageDriftCount, noun)
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, nextFooterTip)
	return nil
}

// computeStorageDriftCount queries the `storage-drift` detector and returns
// the number of non-green findings it reports. Returns 0 when the detector
// is unregistered, when the resolver is nil, or on any error — drift
// surfacing is advisory on the `kerf next` path and must not fail the
// command. The footer renders only when this returns a positive count
// (specs/commands.md §"kerf next" → "Storage-drift footer").
func computeStorageDriftCount(projectID string, r *storage.Resolver) int {
	if r == nil {
		return 0
	}
	det, ok := doctor.DefaultRegistry.Get("storage-drift")
	if !ok {
		return 0
	}
	ctx := &doctor.Context{
		ProjectID: projectID,
		Resolver:  r,
		BenchPath: r.BenchPath,
	}
	findings, err := det.Run(ctx)
	if err != nil {
		return 0
	}
	n := 0
	for _, f := range findings {
		if f.Severity != doctor.Green {
			n++
		}
	}
	return n
}

// renderNextJSON renders the full item stream including warnings. When a
// drift baseline is recorded, the output is an object with `drift_summary`
// alongside the `items` stream (Plan 009 / Bead 11b deliverable). When no
// baseline exists, the output is the bare item array for backwards
// compatibility — empty feed emits `[]`. WorkCodename / BeadID emit literal
// null for non-bead items per the spec (feed.Item enforces this via
// pointer types and no omitempty).
func renderNextJSON(out io.Writer, main, warnings []feed.Item, summary driftSummaryCounts, hasBaseline bool) error {
	// Payload-first ordering (Plan 019 / B3 — kerf-c1c): ranked items
	// stream first, warning items trail. Matches the text renderer's
	// payload-first order per specs/commands.md §"kerf next".
	combined := make([]feed.Item, 0, len(main)+len(warnings))
	combined = append(combined, main...)
	combined = append(combined, warnings...)
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	if !hasBaseline {
		return enc.Encode(combined)
	}
	return enc.Encode(struct {
		DriftSummary driftSummaryCounts `json:"drift_summary"`
		Items        []feed.Item        `json:"items"`
	}{
		DriftSummary: summary,
		Items:        combined,
	})
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
