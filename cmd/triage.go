package cmd

// kerf triage — drift report + exit-code-driven convergence loop per
// Plan 009 / B8.
//
// Spec references:
//   - specs/commands.md §"kerf triage" — syntax, item kinds, behavior
//     steps 1-9, output (text and JSON), exit codes (0/1/2/3), help-text
//     contract, errors.
//   - specs/coordination.md §"Drift detection", §"Pin layer",
//     §"Baseline advancement", §"Drift categories".
//   - plans/009_triage/beads.md §B8 — deliverables and tests.
//
// Self-registers via init() (existing cmd/ pattern; see cmd/pin.go,
// cmd/next.go). The drift baseline at .kerf/sync-cache.json is the
// ONLY state mutated, and only on --ack (per
// specs/coordination.md §"Baseline advancement").

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gberns/kerf/internal/beads"
	"github.com/gberns/kerf/internal/cmdutil"
	"github.com/gberns/kerf/internal/config"
	"github.com/gberns/kerf/internal/doctor"
	"github.com/gberns/kerf/internal/drift"
	"github.com/gberns/kerf/internal/feed"
	"github.com/gberns/kerf/internal/spec"
	"github.com/gberns/kerf/internal/storage"
)

// Triage item kinds and external_drift sub-kinds (per
// specs/commands.md §"kerf triage" §"Item kinds").
const (
	triageKindUntriaged     = "untriaged"
	triageKindMultiMatched  = "multi_matched"
	triageKindExternalDrift = "external_drift"

	triageSubKindExternalClose  = "external_close"
	triageSubKindExternalReopen = "external_reopen"
	triageSubKindExternalDelete = "external_delete"
	triageSubKindExternalNew    = "external_new"
)

var (
	triageResolved bool
	triageAck      bool
	triageKinds    []string
	triageFormat   string
	triageTop      int
	triageGroupBy  string
)

// triageGroupByCodenameLabel is the only currently-accepted value for
// --group-by (per specs/commands.md §"kerf triage" §Flags). v1 groups the
// untriaged section by each bead's tier-1 cohort-defining label (see
// Suggester routing); beads without a tier-1 label fall into the
// '(ungrouped)' tail bucket. Group order is lexicographic by group key;
// the ungrouped bucket renders last.
const (
	triageGroupByCodenameLabel = "codename-label"
	triageGroupByUngroupedKey  = "(ungrouped)"
)

// triageTopUnlimited is the sentinel meaning "show all items" for --top.
// Per specs/commands.md §"kerf triage" §Flags: --top defaults to unlimited
// (flag absent). Passing --top 0 explicitly also means unlimited so an
// agent can opt out of a default-bounded recipe. Any positive N truncates
// each section to N items after sorting.
const triageTopUnlimited = 0

// Help-text contract per specs/commands.md §"kerf triage" §"Help text":
// fixed order — what triage returns, the three item kinds with one-line
// meanings, the --resolved exit-code semantics including the stuck-loop
// guidance, and --ack as the only baseline-advancement command.
// Changes require a spec change.
const triageLongHelp = `Returns a drift report for this project's bead store.

What triage returns:
  A single report reconciling kerf's last acknowledged view of the bead
  store with what is actually there: beads added, relabeled, closed,
  reopened, or deleted by other tools since the last '--ack'.

Item kinds:
  untriaged       — bead matches no work's filter and is not pinned.
  multi_matched   — bead matches >1 work's filter and is not pinned to disambiguate.
  external_drift  — bead's status changed externally since the baseline.
                    Sub-kinds: external_close / external_reopen / external_delete / external_new.

Exit codes (--resolved):
  0  — no untriaged, no multi_matched, no external_drift.
  1  — error (project not initialized, bead store unreadable, ...).
  2  — drift exists, no progress since the previous --resolved run.
  3  — drift exists, count decreased since the previous --resolved run.

  Loop pattern: 'until kerf triage --resolved; do <act>; done'.
  Two consecutive exit-3 runs with identical drift sets means an agent
  should stop and ask for help — exit 2 means the same drift set is being
  acted on without converging.

Baseline advancement:
  '--ack' is the only command that advances the drift baseline. It
  captures the current bead-store snapshot to .kerf/sync-cache.json.
  No other state is mutated.

Baseline lifecycle:
  - First run on a fresh project shows 'baseline: never' and the full
    current state — every untriaged or multi-matched bead is surfaced.
  - Subsequent runs without '--ack' show drift accumulating since the
    previous baseline (untriaged / multi_matched / external_drift).
  - After investigating and resolving items, 'kerf triage --ack'
    advances the baseline to the current bead-store snapshot.
  - The '--resolved' exit-code loop ('until kerf triage --resolved;
    do <act>; done') terminates when drift returns to zero.

  First run on a large project: 'kerf triage --top 20 --group-by codename-label'.`

var triageCmd = &cobra.Command{
	Use:   "triage",
	Short: "Drift report for the project's bead store",
	Long:  triageLongHelp,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTriage(cmd)
	},
}

func init() {
	triageCmd.Flags().BoolVar(&triageResolved, "resolved", false, "Exit-code mode; see help for the 0/1/2/3 matrix")
	triageCmd.Flags().BoolVar(&triageAck, "ack", false, "Acknowledge current snapshot as the new drift baseline")
	triageCmd.Flags().StringArrayVar(&triageKinds, "kind", nil, "Show only items of this kind (repeatable): untriaged, multi_matched, external_drift")
	triageCmd.Flags().StringVar(&triageFormat, "format", "text", "Output format: text or json")
	triageCmd.Flags().IntVar(&triageTop, "top", 0, "Truncate each section to the top N items after sorting (0 = unlimited; large-project recipe: --top 20)")
	triageCmd.Flags().StringVar(&triageGroupBy, "group-by", "", "Group untriaged items by a field. v1 accepts: codename-label")
	rootCmd.AddCommand(triageCmd)
}

// triageItem is the kind-tagged record emitted by the JSON output and
// rendered (per-kind) in the text output. Mirrors
// specs/commands.md §"kerf triage" §"Output (--format=json)".
type triageItem struct {
	Kind          string   `json:"kind"`
	SubKind       *string  `json:"sub_kind"`
	BeadID        string   `json:"bead_id"`
	Title         string   `json:"title"`
	Status        string   `json:"status"`
	Labels        []string `json:"labels"`
	WorkCodenames []string `json:"work_codenames"`
	Suggest       *string  `json:"suggest"`
	Reason        string   `json:"reason"`
}

type triageWorkHealth struct {
	Codename string `json:"codename"`
	Filter   string `json:"filter"`
	Open     int    `json:"open"`
	Closed   int    `json:"closed"`
}

type triageReport struct {
	BaselineCapturedAt string             `json:"baseline_captured_at"`
	BeadCounts         triageBeadCounts   `json:"bead_counts"`
	Works              []triageWorkHealth `json:"works"`
	Items              []triageItem       `json:"items"`
	// Summary is a kerf-internal field for machine consumers; not part of
	// the canonical snapshot shape. It mirrors the spec's per-section
	// counts (beads.md §B8 deliverables).
	Summary triageSummary `json:"summary"`
}

type triageSummary struct {
	Untriaged     int `json:"untriaged"`
	MultiMatched  int `json:"multi_matched"`
	ExternalDrift int `json:"external_drift"`
}

// triageBeadCounts is the canonical "N open · M total" pair surfaced in
// the report header per Plan 018 / B6 (count reconciliation). Both
// numbers are accurate; the previous bug was emitting them unlabeled in
// different places, which read as a discrepancy. `Open` excludes statuses
// in the closedSet (closed/done/complete/completed); `Total` is every
// bead in the store. `Ready` is the subset of `Open` that is also not
// blocked / in-progress — the population the untriaged / multi_matched
// classifications run over.
type triageBeadCounts struct {
	Open  int `json:"open"`
	Ready int `json:"ready"`
	Total int `json:"total"`
}

// notInitializedJSON is emitted on stdout for --format=json when
// project.yaml is absent (per specs/commands.md §"kerf triage" §"Errors"
// and beads.md §B8 not_initialized path).
type notInitializedJSON struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

func runTriage(cmd *cobra.Command) error {
	out := cmd.OutOrStdout()

	// --- Validate flags --------------------------------------------------
	if triageResolved && triageAck {
		return errors.New("--resolved and --ack are mutually exclusive")
	}
	format := strings.ToLower(strings.TrimSpace(triageFormat))
	if format == "" {
		format = "text"
	}
	if format != "text" && format != "json" {
		return fmt.Errorf("unknown format '%s'. Supported: text, json", triageFormat)
	}
	kindSet, err := parseTriageKinds(triageKinds)
	if err != nil {
		return err
	}
	// Per spec, --top defaults to unlimited and 0 is the explicit
	// "show all" sentinel; both map to the same render path. A positive N
	// truncates each section to N items after sorting.
	if triageTop < 0 {
		return fmt.Errorf("--top must be >= 0 (got %d)", triageTop)
	}
	groupBy := strings.TrimSpace(triageGroupBy)
	if groupBy != "" && groupBy != triageGroupByCodenameLabel {
		return fmt.Errorf("unknown --group-by value '%s'. Supported: %s", triageGroupBy, triageGroupByCodenameLabel)
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

	// --- not_initialized: project.yaml absent ----------------------------
	if _, statErr := os.Stat(r.ProjectConfigPath()); statErr != nil && os.IsNotExist(statErr) {
		const msg = "project not initialized. Run 'kerf init' first."
		if format == "json" {
			payload := notInitializedJSON{Kind: "not_initialized", Message: msg}
			b, _ := json.MarshalIndent(payload, "", "  ")
			fmt.Fprintln(out, string(b))
		} else {
			fmt.Fprintf(out, "Error: %s\n", msg)
			fmt.Fprintln(out, "run kerf init first")
		}
		// Exit 1 path — return an error so cobra surfaces non-zero exit.
		return errors.New(msg)
	}

	// --- Load works ------------------------------------------------------
	codenames, err := r.ListWorks()
	if err != nil {
		return err
	}
	// Archive scan (Plan 018 / B2 — archive-aware codename check). Performed
	// once per triage invocation and cached as a set so the suggester does
	// not re-emit `kerf new <archived>` for codenames already in
	// ~/.kerf/archive/<project-id>/. Failures (archive dir missing,
	// unreadable) degrade gracefully to "no known archive entries" rather
	// than failing the whole report — per the bench helper's contract.
	archivedSet := make(map[string]bool)
	if archived, aerr := r.ListArchivedWorks(); aerr == nil {
		for _, cn := range archived {
			archivedSet[cn] = true
		}
	}
	works := make([]*spec.SpecYAML, 0, len(codenames))
	for _, cn := range codenames {
		s, rerr := spec.Read(filepath.Join(r.WorkDir(cn), "spec.yaml"))
		if rerr != nil {
			// Per spec, corrupt spec.yaml files are not fatal here; they
			// are surfaced via `kerf next` as `corrupt_spec` warnings.
			// Triage simply skips them so the rest of the report renders.
			continue
		}
		works = append(works, s)
	}

	// --- Load project config (project-wide filter) -----------------------
	projCfg, _ := config.LoadProjectConfig(r.ProjectConfigPath())
	var projectFilter *beads.Filter
	if projCfg != nil {
		projectFilter = projCfg.BeadFilter
	}

	// --- Load beads ------------------------------------------------------
	// Honor project.yaml tools.tasks (default "br"). When the configured tool
	// is on PATH but invocation fails, surface the concrete diagnostic
	// (BEADS_TOOL_ERROR) — silent zero was the misconfiguration trap behind
	// plan 021.
	toolName := beads.DefaultToolName
	if projCfg != nil {
		toolName = beads.ResolveToolName(projCfg.Tools)
	}
	if !beads.IsAvailableNamed(toolName) {
		return fmt.Errorf("cannot read bead store: bead tool %q not on PATH", toolName)
	}
	allBeads, berr := beads.ListNamed(toolName)
	if berr != nil {
		return fmt.Errorf("cannot read bead store: %w", berr)
	}

	// --- Build BeadToWork (filter resolution) ----------------------------
	beadToWork := make(map[string][]string)
	pinAssignments := make(map[string]string)
	type wf struct {
		codename string
		filter   *beads.Filter
	}
	resolvedFilters := make([]wf, 0, len(works))
	for _, w := range works {
		f := beads.Resolve(w.BeadFilter, projectFilter)
		resolvedFilters = append(resolvedFilters, wf{codename: w.Codename, filter: f})
		matched := beads.ForWorkWithFilter(allBeads, w.Codename, f)
		for _, b := range matched {
			beadToWork[b.ID] = append(beadToWork[b.ID], w.Codename)
		}
		for _, pid := range w.PinnedBeads {
			// Single-owner: last writer wins (defense-in-depth; cmd/pin.go
			// enforces single-owner at write time).
			pinAssignments[pid] = w.Codename
		}
	}
	// Apply the pin layer on top of the filter-resolved attachment.
	postPin := feed.ResolvePins(beadToWork, pinAssignments)

	// --- Compute drift ---------------------------------------------------
	closedSet := map[string]bool{
		"closed":    true,
		"done":      true,
		"complete":  true,
		"completed": true,
	}
	cachePath := drift.CachePath(r.RepoRoot)
	baseline, hasBaseline, _ := drift.Read(cachePath)
	current := drift.Capture(allBeads, postPin)
	var diff drift.Diff
	if hasBaseline {
		diff = drift.Compute(baseline, current, closedSet)
	}

	// --- Canonical bead counts (Plan 018 / B6) ---------------------------
	// Compute the canonical open/ready/total triple once so the header,
	// summary, and any downstream consumer see the same numbers. open
	// excludes closedSet; ready additionally excludes blocked /
	// in-progress (the population classification runs over).
	beadCounts := triageBeadCounts{Total: len(allBeads)}
	for _, b := range allBeads {
		if !closedSet[strings.ToLower(b.Status)] {
			beadCounts.Open++
		}
		if triageIsReady(b) {
			beadCounts.Ready++
		}
	}

	// --- Classify beads --------------------------------------------------
	byID := make(map[string]beads.Bead, len(allBeads))
	for _, b := range allBeads {
		byID[b.ID] = b
	}

	var untriaged []triageItem
	var multiMatched []triageItem
	for _, b := range allBeads {
		if !triageIsReady(b) {
			continue
		}
		if _, pinned := pinAssignments[b.ID]; pinned {
			continue
		}
		matches := beadToWork[b.ID]
		switch {
		case len(matches) == 0:
			untriaged = append(untriaged, triageItem{
				Kind:          triageKindUntriaged,
				BeadID:        b.ID,
				Title:         b.Title,
				Status:        b.Status,
				Labels:        append([]string(nil), b.Labels...),
				WorkCodenames: []string{},
				Suggest:       strPtr(triageSuggestUntriaged(b, codenames, archivedSet)),
				Reason:        "matches no work's filter and is not pinned",
			})
		case len(matches) >= 2:
			sorted := append([]string(nil), matches...)
			sort.Strings(sorted)
			suggest := fmt.Sprintf("kerf pin %s %s", sorted[0], b.ID)
			multiMatched = append(multiMatched, triageItem{
				Kind:          triageKindMultiMatched,
				BeadID:        b.ID,
				Title:         b.Title,
				Status:        b.Status,
				Labels:        append([]string(nil), b.Labels...),
				WorkCodenames: sorted,
				Suggest:       strPtr(suggest),
				Reason:        fmt.Sprintf("matches %d works (%s); pin to disambiguate", len(sorted), strings.Join(sorted, ", ")),
			})
		}
	}
	sort.Slice(untriaged, func(i, j int) bool { return untriaged[i].BeadID < untriaged[j].BeadID })
	sort.Slice(multiMatched, func(i, j int) bool { return multiMatched[i].BeadID < multiMatched[j].BeadID })

	// --- External-drift items --------------------------------------------
	external := buildExternalDriftItems(diff, byID, baseline, postPin)

	// --- Filter by --kind ------------------------------------------------
	if kindSet != nil {
		if !kindSet[triageKindUntriaged] {
			untriaged = nil
		}
		if !kindSet[triageKindMultiMatched] {
			multiMatched = nil
		}
		// external_drift supports sub-kind selection too.
		filtered := external[:0]
		for _, it := range external {
			if kindSet[triageKindExternalDrift] {
				filtered = append(filtered, it)
				continue
			}
			if it.SubKind != nil && kindSet[*it.SubKind] {
				filtered = append(filtered, it)
			}
		}
		external = filtered
	}

	// --- Per-work bead health --------------------------------------------
	health := make([]triageWorkHealth, 0, len(works))
	for _, w := range works {
		f := beads.Resolve(w.BeadFilter, projectFilter)
		matched := beads.ForWorkWithFilter(allBeads, w.Codename, f)
		open, closed := 0, 0
		for _, b := range matched {
			if closedSet[strings.ToLower(b.Status)] {
				closed++
			} else {
				open++
			}
		}
		health = append(health, triageWorkHealth{
			Codename: w.Codename,
			Filter:   renderFilter(f),
			Open:     open,
			Closed:   closed,
		})
	}
	sort.Slice(health, func(i, j int) bool { return health[i].Codename < health[j].Codename })

	// --- Build report + summary ------------------------------------------
	report := triageReport{
		BeadCounts: beadCounts,
		Works:      health,
		Summary: triageSummary{
			Untriaged:     len(untriaged),
			MultiMatched:  len(multiMatched),
			ExternalDrift: len(external),
		},
	}
	if hasBaseline {
		report.BaselineCapturedAt = baseline.CapturedAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	report.Items = append(report.Items, untriaged...)
	report.Items = append(report.Items, multiMatched...)
	report.Items = append(report.Items, external...)

	// --- Render ----------------------------------------------------------
	// Per specs/commands.md §"kerf triage" step 7: with --ack, the render
	// step is skipped — stdout sees only the single-line baseline
	// confirmation (or summary record under --format=json) emitted in
	// step 8 below.
	//
	// Plan 018 / B7 (kerf-ee8): when --kind is given and the filtered set
	// is empty, skip the full report header and emit a single line. Single
	// --kind → `No {kind} items.`; multiple --kind flags → `No items in
	// selected kinds: {comma-separated list}.` JSON output is unchanged
	// (consumers see the empty `items` array on the canonical report).
	if !triageAck {
		emptyKindFilter := kindSet != nil &&
			len(untriaged) == 0 && len(multiMatched) == 0 && len(external) == 0
		switch {
		case emptyKindFilter && format == "text":
			fmt.Fprintln(out, emptyKindFilterLine(triageKinds))
		case format == "json":
			if rerr := renderTriageJSON(out, report); rerr != nil {
				return rerr
			}
		default:
			if rerr := renderTriageText(out, projectID, report, untriaged, multiMatched, external, triageTop, groupBy); rerr != nil {
				return rerr
			}
			// Plan 017 / B12 (kerf-pb4): append the one-line storage-drift
			// footer when `kerf doctor --detector storage-drift` would
			// surface any non-green finding. Spec: specs/commands.md
			// §"kerf triage" §"Storage-drift footer". Suppression
			// (kerf-bwd) follows in a separate bead; --ack already
			// short-circuits the render path, so the single-line baseline
			// confirmation stays clean.
			renderStorageDriftFooter(out, projectID, r)
		}
	}

	// --- --ack: advance the drift baseline -------------------------------
	// Per specs/commands.md §"kerf triage" step 8: capture the current
	// snapshot, write it to .kerf/sync-cache.json, and emit a single-line
	// confirmation (text) or a one-record summary (json).
	if triageAck {
		if cachePath == "" {
			return errors.New("cannot advance baseline: no repo root resolved (run inside the project's git repo)")
		}
		if werr := drift.Advance(cachePath, current); werr != nil {
			return fmt.Errorf("advancing baseline: %w", werr)
		}
		ts := current.CapturedAt.UTC().Format("2006-01-02T15:04:05Z")
		itemsCaptured := len(current.Beads)
		switch format {
		case "json":
			summary := struct {
				BaselineAdvancedAt string `json:"baseline_advanced_at"`
				ItemsCaptured      int    `json:"items_captured"`
			}{
				BaselineAdvancedAt: ts,
				ItemsCaptured:      itemsCaptured,
			}
			enc := json.NewEncoder(out)
			enc.SetIndent("", "  ")
			if eerr := enc.Encode(summary); eerr != nil {
				return eerr
			}
		default:
			fmt.Fprintf(out, "Baseline advanced to %s.\n", ts)
		}
	}

	// --- --resolved: exit codes 0/2/3 ------------------------------------
	if triageResolved {
		untCount := report.Summary.Untriaged
		mmCount := report.Summary.MultiMatched
		edCount := report.Summary.ExternalDrift
		total := untCount + mmCount + edCount
		if total == 0 {
			return nil // exit 0
		}
		// Compare against the previously-recorded resolved counts (kerf-
		// internal metadata stored alongside the cache file). Per
		// beads.md §B8 this is documented as kerf-internal metadata so
		// the canonical snapshot shape stays untouched.
		prev, hadPrev := readResolvedCounts(r.RepoRoot)
		writeResolvedCounts(r.RepoRoot, total) // record for next loop iteration
		switch {
		case !hadPrev:
			triageExit(2)
		case total < prev:
			triageExit(3)
		default:
			triageExit(2)
		}
		// triageExit normally calls os.Exit and never returns; tests
		// swap in a no-op hook, in which case fall through with a
		// typed error carrying the exit code so callers can assert it.
		return &triageExitError{Code: triageLastExitCode}
	}

	return nil
}

// triageIsReady mirrors feed.isReady (package-private). A bead is "ready"
// when its status is not blocked, in-progress, or closed/complete/done.
func triageIsReady(b beads.Bead) bool {
	switch strings.ToLower(b.Status) {
	case "blocked", "in_progress", "in-progress", "closed", "complete", "completed", "done":
		return false
	}
	return true
}

// tier1LabelPrefixes is the cohort-defining allow-list per Plan 018:
// only labels with these prefixes may seed a `kerf new` suggestion.
// Everything else (axis:, tag:, kind:, scope:, subsystem:, area:, …) is
// tier-2 / cross-cutting and falls back to a pin against the
// lexicographically-earliest active work.
var tier1LabelPrefixes = []string{"codename", "spec"}

// triagePickTier1Label returns the first label on a bead whose prefix is
// in the tier-1 allow-list, or "" when none match. Order of preference
// follows tier1LabelPrefixes; within a prefix, order follows bead label
// order so output is deterministic.
func triagePickTier1Label(labels []string) string {
	for _, prefix := range tier1LabelPrefixes {
		for _, lbl := range labels {
			pParts := strings.SplitN(lbl, ":", 2)
			if len(pParts) == 2 && pParts[0] == prefix && pParts[1] != "" {
				return lbl
			}
		}
	}
	return ""
}

// triageSuggestUntriaged renders a templated ready-to-paste command per
// specs/commands.md §"kerf triage" and Plan 018's tier-1 / tier-2 routing:
//   - Tier-1 label (codename: / spec:) → seed `kerf new` (or `kerf work
//     edit --bead-filter-add` when an existing work already has that
//     label in its filter — handled by the value-match below).
//   - All tier-2 labels → fall back to `kerf pin <codename> <bead-id>`
//     against the lexicographically-earliest active work.
//   - No active work to pin against → "no auto-suggestion".
//
// Plan 018 / B2 — archive-aware codename check. When the tier-1 route
// would emit `kerf new <value>` but <value> already exists in the
// project archive (archivedSet), the suggestion is replaced with a
// restore/pin hint per specs/commands.md §"Archive-aware suggestions".
// The bead-filter-add path is unaffected — it points at a live work.
// archivedSet may be nil/empty (no archive entries known); callers
// must not rely on identity, only on membership.
func triageSuggestUntriaged(b beads.Bead, existingCodenames []string, archivedSet map[string]bool) string {
	tier1 := triagePickTier1Label(b.Labels)
	if tier1 != "" {
		parts := strings.SplitN(tier1, ":", 2)
		value := parts[1]
		clause := "label=" + tier1
		// If an existing codename loosely matches the tier-1 label value,
		// suggest extending that work's filter rather than creating a
		// new one.
		for _, cn := range existingCodenames {
			if strings.Contains(cn, value) {
				return fmt.Sprintf("kerf work edit %s --bead-filter-add '%s'", cn, clause)
			}
		}
		// Archive-aware: avoid suggesting `kerf new <value>` when <value>
		// is already an archived codename for this project.
		if archivedSet[value] {
			return fmt.Sprintf("codename '%s' is archived — consider 'kerf restore %s' to unarchive, or 'kerf pin <codename> %s' to attach this bead to a different live work.",
				value, value, b.ID)
		}
		return fmt.Sprintf("kerf new %s --bead-filter '%s'", value, clause)
	}
	// Tier-2 fallback: pin against the lexicographically-earliest active
	// work. existingCodenames comes from r.ListWorks(), which returns
	// non-archived works.
	if len(existingCodenames) == 0 {
		return fmt.Sprintf("no auto-suggestion; investigate manually (bead %s)", b.ID)
	}
	sorted := append([]string(nil), existingCodenames...)
	sort.Strings(sorted)
	return fmt.Sprintf("kerf pin %s %s", sorted[0], b.ID)
}

// buildExternalDriftItems emits one triageItem per bead in each non-empty
// drift category (close → reopen → delete → new), in spec order. Pure
// over the supplied diff + bead lookup map.
func buildExternalDriftItems(d drift.Diff, byID map[string]beads.Bead, baseline drift.Snapshot, postPin map[string][]string) []triageItem {
	var out []triageItem
	emit := func(ids []string, subKind string, reason string) {
		for _, id := range ids {
			it := triageItem{
				Kind:          triageKindExternalDrift,
				SubKind:       strPtr(subKind),
				BeadID:        id,
				WorkCodenames: postPin[id],
				Reason:        reason,
				Suggest:       strPtr("kerf triage --ack"),
			}
			if it.WorkCodenames == nil {
				it.WorkCodenames = []string{}
			}
			if b, ok := byID[id]; ok {
				it.Title = b.Title
				it.Status = b.Status
				it.Labels = append([]string(nil), b.Labels...)
			} else if rec, ok := baseline.Beads[id]; ok {
				// Deleted: bead no longer in current store; fall back to
				// the baseline title/status so the report carries usable
				// context.
				it.Title = rec.Title
				it.Status = rec.Status
				it.Labels = append([]string(nil), rec.Labels...)
			}
			if it.Labels == nil {
				it.Labels = []string{}
			}
			out = append(out, it)
		}
	}
	emit(d.ClosedExternally, triageSubKindExternalClose, "closed externally since last acknowledged baseline")
	emit(d.ReopenedExternally, triageSubKindExternalReopen, "reopened externally since last acknowledged baseline")
	emit(d.Deleted, triageSubKindExternalDelete, "present at baseline, gone now")
	emit(d.New, triageSubKindExternalNew, "added externally since last acknowledged baseline")
	return out
}

// truncateSection returns (shown, hidden) for a section under --top
// semantics. n==0 (the unlimited sentinel) returns all items, zero hidden.
// Truncation never reorders — callers have already sorted.
func truncateSection(items []triageItem, n int) (shown []triageItem, hidden int) {
	if n <= 0 || len(items) <= n {
		return items, 0
	}
	return items[:n], len(items) - n
}

// renderSectionHeader chooses between the "(N):" and "(showing K of N):"
// header shapes per specs/commands.md §"Count reconciliation and --top
// rendering". The "showing K of N" form is only used when --top was given
// AND truncation actually applies (hidden > 0); a section that fits under
// the cap renders with the plain "(N):" header so a clean project does
// not gain noise from `--top 20`.
func renderSectionHeader(out io.Writer, label string, total, hidden int) {
	if hidden > 0 {
		fmt.Fprintf(out, "%s (showing %d of %d):\n", label, total-hidden, total)
	} else {
		fmt.Fprintf(out, "%s (%d):\n", label, total)
	}
}

// renderStorageDriftFooter appends the one-line storage-drift footer to
// `kerf triage` text output when the `storage-drift` doctor detector
// would surface any non-green finding for the current project. Per
// specs/commands.md §"kerf triage" §"Storage-drift footer", the footer
// mirrors the one rendered by `kerf next` and points the agent at
// `kerf doctor` for details.
//
// Failure modes inside the detector (e.g., bench unreadable) are
// intentionally silent here: a diagnostic surface should never gate the
// primary triage report on its own infra error. The footer simply
// elides when the detector cannot answer.
func renderStorageDriftFooter(out io.Writer, projectID string, r *storage.Resolver) {
	if r == nil {
		return
	}
	d, ok := doctor.DefaultRegistry.Get("storage-drift")
	if !ok {
		return
	}
	findings, err := d.Run(&doctor.Context{
		ProjectID: projectID,
		Resolver:  r,
		BenchPath: r.BenchPath,
	})
	if err != nil {
		return
	}
	nonGreen := 0
	for _, f := range findings {
		if f.Severity != doctor.Green {
			nonGreen++
		}
	}
	if nonGreen == 0 {
		return
	}
	plural := ""
	if nonGreen != 1 {
		plural = "s"
	}
	fmt.Fprintf(out, "note: %d storage finding%s — run 'kerf doctor' for details\n", nonGreen, plural)
}

// renderTruncationFooter prints the "... and X more — use --top 0 for
// full list" line when a section was truncated. Per the bead body's
// rendering contract: section header carries totals; footer carries the
// recovery hint so an agent never has to guess how to see the rest.
func renderTruncationFooter(out io.Writer, hidden int) {
	if hidden > 0 {
		fmt.Fprintf(out, "  ... and %d more — use --top 0 for full list\n", hidden)
	}
}

// renderTriageText writes the compact human-readable report. Sections
// with zero items are omitted, per spec. When top > 0 each section is
// truncated to that many items after sorting; top == 0 is the unlimited
// sentinel (matches the flag-absent default).
func renderTriageText(out io.Writer, projectID string, report triageReport, untriaged, multiMatched, external []triageItem, top int, groupBy string) error {
	// Header — `Triage for <project> (baseline: <ts>):` followed by the
	// canonical bead-count line. Per Plan 018 / B6, each count is labeled
	// with its status filter so 'open' and 'total' never appear ambiguous.
	if report.BaselineCapturedAt != "" {
		fmt.Fprintf(out, "Triage for %s (baseline: %s):\n", projectID, report.BaselineCapturedAt)
	} else {
		fmt.Fprintf(out, "Triage for %s (baseline: never):\n", projectID)
	}
	fmt.Fprintf(out, "  Beads: %d open · %d ready · %d total\n",
		report.BeadCounts.Open, report.BeadCounts.Ready, report.BeadCounts.Total)

	totalItems := len(untriaged) + len(multiMatched) + len(external)
	if totalItems == 0 {
		fmt.Fprintln(out, "  No untriaged, multi-matched, or externally-changed beads. Project is clean.")
		if len(report.Works) > 0 {
			fmt.Fprintln(out)
			renderWorkHealth(out, report.Works)
		}
		return nil
	}

	// Per the bead body / specs/commands.md §--top: a positive N truncates
	// each section after sorting. top == 0 is the unlimited sentinel (the
	// flag-absent default and the explicit "show all" override) and
	// short-circuits through truncateSection's n<=0 guard. external_drift
	// items are bounded by the same N when --top is given — the spec
	// exempts them from implicit defaults only, and an explicit --top is
	// always intentional.
	untriagedShown, untriagedHidden := truncateSection(untriaged, top)
	multiMatchedShown, multiMatchedHidden := truncateSection(multiMatched, top)
	externalShown, externalHidden := truncateSection(external, top)

	if len(untriaged) > 0 {
		fmt.Fprintln(out)
		if groupBy == triageGroupByCodenameLabel {
			renderUntriagedGrouped(out, untriaged, top)
		} else {
			renderSectionHeader(out, "Untriaged beads", len(untriaged), untriagedHidden)
			for _, it := range untriagedShown {
				fmt.Fprintf(out, "  %s  %s  %q  labels: %s\n",
					it.BeadID, it.Status, it.Title, joinOrDash(it.Labels))
				if it.Suggest != nil {
					fmt.Fprintf(out, "    suggest: %s\n", *it.Suggest)
				}
			}
			renderTruncationFooter(out, untriagedHidden)
		}
	}

	if len(multiMatched) > 0 {
		fmt.Fprintln(out)
		renderSectionHeader(out, "Multi-matched beads", len(multiMatched), multiMatchedHidden)
		for _, it := range multiMatchedShown {
			fmt.Fprintf(out, "  %s  %s  %q  matches: %s\n",
				it.BeadID, it.Status, it.Title, strings.Join(it.WorkCodenames, ", "))
			if it.Suggest != nil {
				fmt.Fprintf(out, "    suggest: %s\n", *it.Suggest)
			}
		}
		renderTruncationFooter(out, multiMatchedHidden)
	}

	if len(external) > 0 {
		fmt.Fprintln(out)
		renderSectionHeader(out, "External changes since last triage", len(external), externalHidden)
		for _, it := range externalShown {
			sk := ""
			if it.SubKind != nil {
				sk = *it.SubKind
			}
			fmt.Fprintf(out, "  %s  %s  %q  %s\n",
				it.BeadID, sk, it.Title, it.Reason)
		}
		renderTruncationFooter(out, externalHidden)
	}

	if len(report.Works) > 0 {
		fmt.Fprintln(out)
		renderWorkHealth(out, report.Works)
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "Next:")
	fmt.Fprintln(out, "  Address surfaced items, then run 'kerf triage --ack' to advance the baseline.")
	fmt.Fprintln(out, "  Re-run 'kerf triage --resolved' to confirm the project is clean.")
	return nil
}

// renderUntriagedGrouped renders the untriaged section grouped by tier-1
// label (currently only `--group-by codename-label`). Group key is the
// bead's first tier-1 label (e.g., `codename:foo`); beads without one
// land in a `(ungrouped)` bucket that renders last. Group order is
// lexicographic by key. Under `--top N`, truncation is applied per-group
// after sorting — each group's header reports `(showing K of N)` when
// the cap fires, and the overall section header carries the unmodified
// total so the reader can reconcile against the summary line. Items
// inside a group sort by bead ID, matching the flat rendering's contract.
func renderUntriagedGrouped(out io.Writer, items []triageItem, top int) {
	groups := make(map[string][]triageItem)
	for _, it := range items {
		key := triagePickTier1Label(it.Labels)
		if key == "" {
			key = triageGroupByUngroupedKey
		}
		groups[key] = append(groups[key], it)
	}
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	// Move the ungrouped bucket to the tail (sort.Strings places it first
	// because '(' < any letter — but spec-wise we want it last).
	if idx := indexOfString(keys, triageGroupByUngroupedKey); idx >= 0 {
		keys = append(append([]string{}, keys[:idx]...), keys[idx+1:]...)
		keys = append(keys, triageGroupByUngroupedKey)
	}
	fmt.Fprintf(out, "Untriaged beads (%d), grouped by codename-label:\n", len(items))
	for _, key := range keys {
		bucket := groups[key]
		sort.Slice(bucket, func(i, j int) bool { return bucket[i].BeadID < bucket[j].BeadID })
		shown, hidden := truncateSection(bucket, top)
		if hidden > 0 {
			fmt.Fprintf(out, "  %s (showing %d of %d):\n", key, len(shown), len(bucket))
		} else {
			fmt.Fprintf(out, "  %s (%d):\n", key, len(bucket))
		}
		for _, it := range shown {
			fmt.Fprintf(out, "    %s  %s  %q  labels: %s\n",
				it.BeadID, it.Status, it.Title, joinOrDash(it.Labels))
			if it.Suggest != nil {
				fmt.Fprintf(out, "      suggest: %s\n", *it.Suggest)
			}
		}
		if hidden > 0 {
			fmt.Fprintf(out, "    ... and %d more — use --top 0 for full list\n", hidden)
		}
	}
}

func indexOfString(ss []string, target string) int {
	for i, s := range ss {
		if s == target {
			return i
		}
	}
	return -1
}

func renderWorkHealth(out io.Writer, works []triageWorkHealth) {
	fmt.Fprintln(out, "Per-work bead health:")
	for _, w := range works {
		note := ""
		if w.Open == 0 && w.Closed == 0 {
			note = "   (no attached beads)"
		}
		fmt.Fprintf(out, "  %s  filter: %s  beads: %d open / %d closed%s\n",
			w.Codename, w.Filter, w.Open, w.Closed, note)
	}
}

// renderTriageJSON emits the spec's header object + item stream + summary.
func renderTriageJSON(out io.Writer, report triageReport) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// parseTriageKinds validates --kind values against the spec's allowed set
// (the three top-level kinds plus the four external_drift sub-kinds).
// Returns nil when no --kind was given (= all kinds).
func parseTriageKinds(in []string) (map[string]bool, error) {
	if len(in) == 0 {
		return nil, nil
	}
	allowed := map[string]bool{
		triageKindUntriaged:         true,
		triageKindMultiMatched:      true,
		triageKindExternalDrift:     true,
		triageSubKindExternalClose:  true,
		triageSubKindExternalReopen: true,
		triageSubKindExternalDelete: true,
		triageSubKindExternalNew:    true,
	}
	out := make(map[string]bool, len(in))
	for _, k := range in {
		k = strings.TrimSpace(k)
		if !allowed[k] {
			return nil, fmt.Errorf("unknown triage kind '%s'. Known kinds: untriaged, multi_matched, external_drift", k)
		}
		out[k] = true
	}
	return out, nil
}

// emptyKindFilterLine renders the one-line message per Plan 018 / B7
// (specs/commands.md §"kerf triage" step 6). Called only when --kind was
// given and the filtered item set is empty.
//
//   - Single --kind X → `No X items.`
//   - Multiple --kind flags → `No items in selected kinds: X, Y, Z.`
//
// The kinds list mirrors the user's flag order (de-duplicated) so the
// agent sees back exactly what it asked for.
func emptyKindFilterLine(kinds []string) string {
	seen := make(map[string]bool, len(kinds))
	uniq := make([]string, 0, len(kinds))
	for _, k := range kinds {
		k = strings.TrimSpace(k)
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		uniq = append(uniq, k)
	}
	if len(uniq) == 1 {
		return fmt.Sprintf("No %s items.", uniq[0])
	}
	return fmt.Sprintf("No items in selected kinds: %s.", strings.Join(uniq, ", "))
}

// renderFilter returns a single-line representation of a resolved filter
// suitable for the per-work bead-health line. Used only by triage; no
// existing helper renders Filter as text.
func renderFilter(f *beads.Filter) string {
	if f == nil {
		return "(default)"
	}
	if f.Label != "" {
		return "label=" + f.Label
	}
	if f.IDPrefix != "" {
		return "id_prefix=" + f.IDPrefix
	}
	if len(f.Any) > 0 {
		parts := make([]string, 0, len(f.Any))
		for i := range f.Any {
			parts = append(parts, renderFilter(&f.Any[i]))
		}
		return "any=[" + strings.Join(parts, ", ") + "]"
	}
	return "(empty)"
}

func joinOrDash(s []string) string {
	if len(s) == 0 {
		return "-"
	}
	return strings.Join(s, ", ")
}

func strPtr(s string) *string { return &s }

// --- Resolved-counts metadata (kerf-internal) ----------------------------
//
// Per beads.md §B8: progress tracking for exit code 3 stores the last
// `--resolved` invocation's unresolved counts. We persist this as
// `.kerf/resolved-counts` (a sibling of sync-cache.json) — a single
// integer line. This is explicitly kerf-internal metadata, NOT part of
// the canonical snapshot shape that B2 tests against (intentional
// separation so the snapshot file stays stable).
const resolvedCountsFile = "resolved-counts"

func resolvedCountsPath(repoRoot string) string {
	if repoRoot == "" {
		return ""
	}
	return filepath.Join(repoRoot, ".kerf", resolvedCountsFile)
}

func readResolvedCounts(repoRoot string) (int, bool) {
	p := resolvedCountsPath(repoRoot)
	if p == "" {
		return 0, false
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return 0, false
	}
	var n int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &n); err != nil {
		return 0, false
	}
	return n, true
}

func writeResolvedCounts(repoRoot string, total int) {
	p := resolvedCountsPath(repoRoot)
	if p == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(p, []byte(fmt.Sprintf("%d\n", total)), 0o644)
}

// --- Exit-code wiring ----------------------------------------------------
//
// cobra defaults err != nil → exit 1. To surface --resolved's exit codes
// 2 and 3 without touching cmd/root.go (per Plan 009 / B8: "no bead
// touches cmd/root.go"), triage invokes a process-exit hook directly.
// triageExit defaults to os.Exit; tests swap it for a no-op recorder so
// they can assert the requested exit code without terminating the test
// runner.

type triageExitError struct{ Code int }

func (e *triageExitError) Error() string { return fmt.Sprintf("triage: exit %d", e.Code) }

// triageExitFn is the process-exit hook. Indirected through a variable
// so tests can replace it. Default behavior is os.Exit.
var triageExitFn = func(code int) { os.Exit(code) }

// triageLastExitCode records the last code passed to triageExit. Tests
// inspect this after the RunE invocation to verify --resolved semantics.
var triageLastExitCode int

func triageExit(code int) {
	triageLastExitCode = code
	triageExitFn(code)
}

// TriageExitCode reports the exit code carried by err if it was produced
// by `kerf triage --resolved` running under a non-exiting test hook.
func TriageExitCode(err error) (int, bool) {
	if err == nil {
		return 0, false
	}
	var te *triageExitError
	if errors.As(err, &te) {
		return te.Code, true
	}
	return 0, false
}
