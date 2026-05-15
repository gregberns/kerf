package feed

// Warning detectors. Originally introduced by Plan 006 / B5; extended by
// Plan 008 (corrupt_spec, no_project_yaml, relabel_drift) and Plan 009 /
// B4 (untriaged_beads — renamed from the plan-006 untriaged surface,
// multi_matched, external_drift, pin_conflict factory).
//
// Spec references:
//   - specs/commands.md §"kerf next" — Behavior step 3, Warning detectors.
//   - specs/commands.md §"Warning kinds" — canonical kind catalog.
//   - specs/coordination.md §"Composition with other detectors" — defines
//     untriaged_beads / multi_matched / external_drift / pin_conflict.
//   - specs/coordination.md §"Matching is case sensitive" — case-mismatch
//     surface for the project bead_filter.
//
// These detectors are pure over Input. They produce KindWarning items with
// no WorkCodename / BeadID (project-level) and Score == 0 (warnings are not
// ranked among items — they render as a header block, see commands.md).

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gberns/kerf/internal/beads"
)

// Warning-kind title constants. These are the rendered Title strings on
// emitted Items; they double as the spec-defined "kind" tokens documented
// in specs/commands.md §"Warning kinds" and specs/coordination.md
// §"Composition with other detectors".
//
// `pin_conflict` is owned by this file (Plan 009 / Bead 4) per the
// kerf-nn8 coordination note: B5 documented the contract on feed.Input
// but the kind constant + factory live with the other warning kinds.
// Callers construct conflict warnings via PinConflictWarning when they
// detect a bead pinned to two works while building PinAssignments.
const (
	WarningKindUntriagedBeads    = "untriaged_beads"
	WarningKindMultiMatchedBead  = "multi_matched"
	WarningKindExternalDrift     = "external_drift"
	WarningKindPinConflict       = "pin_conflict"
	WarningKindFilterCaseMismatch = "bead_filter case-mismatch"
)

// NewWarningDetectors returns the v1 warning detectors:
//   - untriaged_beads: surfaces beads matching no work's resolved filter
//     AND not pinned to any work (Plan 009 / Bead 4; renamed from the
//     plan-006 unmatched-beads detector). Emits a single project-level
//     warning with the count; remediation is `kerf triage`.
//   - multi_matched: emits one warning per bead matching ≥2 works'
//     resolved filters and not pinned (Plan 009 / Bead 4). Pin overrides
//     multi-match per specs/coordination.md §"Pin layer".
//   - external_drift: emits one warning per non-empty drift category in
//     Input.DriftResult (New, Deleted, ClosedExternally, ReopenedExternally).
//     Plan 009 / Bead 4. The `Changed` category is surfaced by the
//     pre-existing relabel_drift detector (Plan 008 / B11-code).
//   - filter_case_mismatch: project-wide check — when the project filter's
//     literal prefix has zero case-sensitive matches but a case-insensitive
//     variant would match, suggests a case-mismatch.
//   - corrupt_spec: emits one warning per work whose spec.yaml could not be
//     parsed during feed assembly (Plan 008 / B10-code; specs/commands.md
//     §"Warning kinds" → `corrupt_spec`).
//   - no_project_yaml: emits a single fatal warning when project.yaml is
//     absent from both local-storage and bench paths
//     (specs/commands.md §"Warning kinds" → `no_project_yaml`).
//   - relabel_drift: emits one warning per bead whose per-bead drift hash
//     changed between the cached baseline and the current store, i.e. any
//     bead listed in Input.DriftResult.Changed (Plan 008 / B11-code; hash
//     scope per specs/coordination.md §"Hash scope"). Hash-only: when the
//     caller passes a zero-value DriftResult (no in-memory last-seen yet),
//     the detector emits nothing.
//
// All detectors are project-level: WorkCodename and BeadID are nil and
// Score is 0. Warnings are never excluded by work state.
func NewWarningDetectors(projectFilter *beads.Filter) []Detector {
	return []Detector{
		DetectorFunc(untriagedBeadsDetector),
		DetectorFunc(multiMatchedBeadDetector),
		DetectorFunc(externalDriftDetector),
		DetectorFunc(filterCaseMismatchDetector(projectFilter)),
		DetectorFunc(corruptSpecDetector),
		DetectorFunc(noProjectYAMLDetector),
		DetectorFunc(relabelDriftDetector),
	}
}

// PinConflictWarning constructs a project-level pin_conflict warning Item
// for callers that detect a bead pinned to two works while collapsing the
// per-work PinnedBeads lists into the PinAssignments map (single-owner
// invariant violation; specs/coordination.md §"Pin layer"). The kind
// constant lives here (Plan 009 / Bead 4); B9's `kerf pin` is the
// primary enforcement, this is defense-in-depth against manual edits.
func PinConflictWarning(beadID, winner, loser string) Item {
	return Item{
		Kind:         KindWarning,
		Score:        0,
		Title:        fmt.Sprintf("%s: %s", WarningKindPinConflict, beadID),
		Action:       fmt.Sprintf("kerf pin %s %s", winner, beadID),
		Reason:       fmt.Sprintf("Bead '%s' is pinned to both '%s' and '%s'; using '%s' (lexicographically earliest). Re-pin to disambiguate.", beadID, winner, loser, winner),
		WorkCodename: nil,
		BeadID:       nil,
	}
}

// relabelDriftDetector emits one warning per bead ID in
// Input.DriftResult.Changed. A bead lands in Changed when it is present at
// both baseline and current with the same open/closed polarity but its
// canonical per-bead hash differs (see drift.Compute). The hash scope is
// id+status+labels+title+deps (specs/coordination.md §"Hash scope"), so a
// label edit, retitle, or dependency change all surface here.
//
// This bead (Plan 008 / B11-code) is contract-only: it consumes whatever
// DriftResult the caller supplies. Persisted last-seen state — i.e.
// actually wiring `.kerf/sync-cache.json` into the next-command Input — is
// plan 009 scope (kerf-8o9 cache + kerf-nn8 wiring). When the caller
// passes a zero-value DriftResult (empty Changed slice), the detector is
// silent: no baseline, no warning.
//
// Warnings are project-level (no WorkCodename / BeadID) so they render in
// the header block of `kerf next` next to the other drift signals.
func relabelDriftDetector(in Input) []Item {
	if len(in.DriftResult.Changed) == 0 {
		return nil
	}
	out := make([]Item, 0, len(in.DriftResult.Changed))
	for _, id := range in.DriftResult.Changed {
		out = append(out, Item{
			Kind:         KindWarning,
			Score:        0,
			Title:        fmt.Sprintf("Relabel drift: %s", id),
			Action:       "kerf triage",
			Reason:       fmt.Sprintf("Bead '%s' content (labels, title, or deps) changed since the last acknowledged baseline. Run 'kerf triage' to review and 'kerf triage --ack' to acknowledge.", id),
			WorkCodename: nil,
			BeadID:       nil,
		})
	}
	return out
}

// corruptSpecDetector emits one warning per entry in in.CorruptSpecs.
// Field shapes follow specs/commands.md §"Warning kinds" → `corrupt_spec`.
func corruptSpecDetector(in Input) []Item {
	if len(in.CorruptSpecs) == 0 {
		return nil
	}
	out := make([]Item, 0, len(in.CorruptSpecs))
	for _, cs := range in.CorruptSpecs {
		cn := cs.Codename
		out = append(out, Item{
			Kind:         KindWarning,
			Score:        0,
			Title:        fmt.Sprintf("Corrupt spec: %s", cn),
			Action:       fmt.Sprintf("kerf show %s", cn),
			Reason:       fmt.Sprintf("Could not parse spec.yaml for '%s': %s. Work excluded from this feed.", cn, cs.ParseError),
			WorkCodename: nil,
			BeadID:       nil,
		})
	}
	return out
}

// noProjectYAMLDetector emits a single warning when in.NoProjectYAML is
// true. Field shapes follow specs/commands.md §"Warning kinds"
// → `no_project_yaml`. This warning is fatal: the caller suppresses the
// feed listing and sets a non-zero exit status.
func noProjectYAMLDetector(in Input) []Item {
	if !in.NoProjectYAML {
		return nil
	}
	pid := in.ProjectID
	return []Item{{
		Kind:         KindWarning,
		Score:        0,
		Title:        fmt.Sprintf("No project.yaml for '%s'", pid),
		Action:       "kerf init",
		Reason:       fmt.Sprintf("Project '%s' has no project.yaml. Run 'kerf init' to create one before using 'kerf next'.", pid),
		WorkCodename: nil,
		BeadID:       nil,
	}}
}

// untriagedBeadsDetector emits a single project-level warning summarizing
// beads that match no work's resolved filter AND are not pinned to any
// work via PinAssignments. A bead in either set is "triaged"; only the
// intersection of "matches nothing" and "pinned nowhere" is untriaged
// (specs/coordination.md §"Drift categories" → `untriaged`).
//
// Counts are taken over the open / ready bead set — the same set the
// bead feed renders — so the rendered count agrees with the listed items
// (Plan 008 / Bead 6 invariant). Closed / done / blocked beads are
// excluded from the count.
//
// Surfaces the most common label-prefix among untriaged beads as the
// suggested-action hint, pointing the user at the misconfiguration.
//
// Plan 009 / Bead 4 — renamed from the plan-006 detector; the kind
// string and behavior now match specs/commands.md §"Warning kinds" →
// `untriaged`. The plan-006 heuristic abs/frac thresholds are gone:
// the spec emits the warning whenever the count is non-zero and lets
// the drift-summary line in `kerf next` do the rate-limiting.
func untriagedBeadsDetector(in Input) []Item {
	if len(in.AllBeads) == 0 {
		return nil
	}

	// Restrict to the open / ready bead set — the same set the bead feed
	// renders. This makes the untriaged count consistent with the listed
	// items.
	openBeads := make([]beads.Bead, 0, len(in.AllBeads))
	for _, b := range in.AllBeads {
		if isReady(b) {
			openBeads = append(openBeads, b)
		}
	}
	if len(openBeads) == 0 {
		return nil
	}

	// Build the set of resolved per-work filters.
	type wf struct {
		codename string
		filter   *beads.Filter
	}
	resolved := make([]wf, 0, len(in.Works))
	for _, w := range in.Works {
		if w == nil {
			continue
		}
		f := beads.Resolve(w.BeadFilter, in.ProjectFilter)
		resolved = append(resolved, wf{codename: w.Codename, filter: f})
	}

	// Walk every open bead; count those that match no work AND are not
	// pinned. A pinned bead is triaged-by-pin even if its labels match no
	// filter (specs/coordination.md §"Pin layer").
	untriagedCount := 0
	prefixCounts := map[string]int{}
	for _, b := range openBeads {
		if _, pinned := in.PinAssignments[b.ID]; pinned {
			continue
		}
		matched := false
		for _, wf := range resolved {
			if wf.filter != nil && wf.filter.Match(b, wf.codename) {
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		untriagedCount++
		for _, lbl := range b.Labels {
			if p, ok := labelPrefix(lbl); ok {
				prefixCounts[p]++
			}
		}
	}

	if untriagedCount == 0 {
		return nil
	}

	topPrefix := mostCommonPrefix(prefixCounts)

	action := "kerf triage"
	if topPrefix != "" {
		action = fmt.Sprintf("kerf triage — top untriaged prefix: '%s'", topPrefix)
	}

	return []Item{{
		Kind:         KindWarning,
		Score:        0,
		Title:        WarningKindUntriagedBeads,
		Action:       action,
		Reason:       fmt.Sprintf("%d beads match no work via current filter and are not pinned", untriagedCount),
		WorkCodename: nil,
		BeadID:       nil,
	}}
}

// multiMatchedBeadDetector emits one warning per bead that matches ≥2
// works' resolved filters AND is not pinned to any work via
// PinAssignments (specs/coordination.md §"Drift categories" →
// `multi_matched`; §"Pin layer" — pin overrides multi-match).
//
// Counts are taken over the open / ready bead set (same invariant as
// untriagedBeadsDetector). Output is sorted by bead ID for deterministic
// rendering.
func multiMatchedBeadDetector(in Input) []Item {
	if len(in.AllBeads) == 0 || len(in.Works) < 2 {
		return nil
	}

	type wf struct {
		codename string
		filter   *beads.Filter
	}
	resolved := make([]wf, 0, len(in.Works))
	for _, w := range in.Works {
		if w == nil {
			continue
		}
		f := beads.Resolve(w.BeadFilter, in.ProjectFilter)
		resolved = append(resolved, wf{codename: w.Codename, filter: f})
	}

	type hit struct {
		id    string
		works []string
	}
	hits := make([]hit, 0)
	for _, b := range in.AllBeads {
		if !isReady(b) {
			continue
		}
		if _, pinned := in.PinAssignments[b.ID]; pinned {
			continue
		}
		matched := make([]string, 0, 2)
		for _, wf := range resolved {
			if wf.filter != nil && wf.filter.Match(b, wf.codename) {
				matched = append(matched, wf.codename)
			}
		}
		if len(matched) >= 2 {
			sort.Strings(matched)
			hits = append(hits, hit{id: b.ID, works: matched})
		}
	}
	if len(hits) == 0 {
		return nil
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].id < hits[j].id })

	out := make([]Item, 0, len(hits))
	for _, h := range hits {
		// Suggested action: pin to the lexicographically-earliest match
		// (specs/commands.md §`kerf triage` — multi_matched suggestion).
		winner := h.works[0]
		out = append(out, Item{
			Kind:         KindWarning,
			Score:        0,
			Title:        fmt.Sprintf("%s: %s", WarningKindMultiMatchedBead, h.id),
			Action:       fmt.Sprintf("kerf pin %s %s", winner, h.id),
			Reason:       fmt.Sprintf("Bead '%s' matches %d works (%s); pin to disambiguate.", h.id, len(h.works), strings.Join(h.works, ", ")),
			WorkCodename: nil,
			BeadID:       nil,
		})
	}
	return out
}

// externalDriftDetector emits one warning per non-empty external-drift
// category in Input.DriftResult. The four `external_*` sub-kinds
// (specs/commands.md §"Warning kinds" → `external_drift`) are:
//
//   - external_new:    Input.DriftResult.New
//   - external_close:  Input.DriftResult.ClosedExternally
//   - external_reopen: Input.DriftResult.ReopenedExternally
//   - external_delete: Input.DriftResult.Deleted
//
// The `Changed` category is surfaced by the pre-existing
// relabelDriftDetector (Plan 008 / B11-code) and is intentionally not
// duplicated here.
//
// When DriftResult is the zero value (cache absent or read failed —
// caller responsibility), all category slices are empty and the
// detector is silent: the empty baseline is interpreted as "no drift
// known yet", not "everything is new", to avoid spamming a first-run
// inventory through the warning channel (the drift-summary line and
// `kerf triage` are the right surfaces for that).
//
// Output is ordered close → reopen → delete → new for deterministic
// rendering.
func externalDriftDetector(in Input) []Item {
	type cat struct {
		subKind string
		ids     []string
	}
	cats := []cat{
		{"external_close", in.DriftResult.ClosedExternally},
		{"external_reopen", in.DriftResult.ReopenedExternally},
		{"external_delete", in.DriftResult.Deleted},
		{"external_new", in.DriftResult.New},
	}
	out := make([]Item, 0, len(cats))
	for _, c := range cats {
		if len(c.ids) == 0 {
			continue
		}
		out = append(out, Item{
			Kind:         KindWarning,
			Score:        0,
			Title:        fmt.Sprintf("%s/%s", WarningKindExternalDrift, c.subKind),
			Action:       "kerf triage",
			Reason:       fmt.Sprintf("%d bead(s) %s since the last acknowledged baseline.", len(c.ids), c.subKind),
			WorkCodename: nil,
			BeadID:       nil,
		})
	}
	return out
}

// filterCaseMismatchDetector returns a detector closure bound to the
// project-wide filter. Per-work overrides are intentionally not inspected
// (B5 task spec, coordination.md §"Matching is case sensitive").
//
// The detector emits a warning when:
//   - the project filter's literal prefix matches zero beads in the store
//     case-sensitively, AND
//   - lower-casing that prefix would match at least one bead.
func filterCaseMismatchDetector(projectFilter *beads.Filter) func(Input) []Item {
	return func(in Input) []Item {
		if projectFilter == nil || len(in.AllBeads) == 0 {
			return nil
		}
		prefixes := literalPrefixes(projectFilter)
		if len(prefixes) == 0 {
			return nil
		}
		for _, p := range prefixes {
			if p == "" {
				continue
			}
			if countWithPrefix(in.AllBeads, p, true) > 0 {
				// Case-sensitive match exists — no mismatch.
				return nil
			}
		}
		// No case-sensitive prefix matched. See if any lower-cased variant
		// would match.
		var suggest string
		for _, p := range prefixes {
			if p == "" {
				continue
			}
			lower := strings.ToLower(p)
			if lower == p {
				continue
			}
			if countWithPrefix(in.AllBeads, lower, false) > 0 {
				suggest = lower
				break
			}
		}
		if suggest == "" {
			return nil
		}
		return []Item{{
			Kind:         KindWarning,
			Score:        0,
			Title:        "bead_filter case-mismatch",
			Action:       "check case of project bead_filter — beads use lower case",
			Reason:       fmt.Sprintf("project bead_filter prefix has zero case-sensitive matches; try '%s'", suggest),
			WorkCodename: nil,
			BeadID:       nil,
		}}
	}
}

// labelPrefix returns the substring up to and including the first ':' in s.
// Returns ("", false) when no ':' is present. Used to group untriaged beads
// by convention prefix (e.g., "subsystem:", "work:").
func labelPrefix(s string) (string, bool) {
	i := strings.IndexByte(s, ':')
	if i < 0 {
		return "", false
	}
	return s[:i+1], true
}

// mostCommonPrefix returns the prefix with the highest count. Ties broken by
// alphabetical order for determinism. Returns "" when the map is empty.
func mostCommonPrefix(counts map[string]int) string {
	if len(counts) == 0 {
		return ""
	}
	type kv struct {
		k string
		v int
	}
	pairs := make([]kv, 0, len(counts))
	for k, v := range counts {
		pairs = append(pairs, kv{k, v})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].v != pairs[j].v {
			return pairs[i].v > pairs[j].v
		}
		return pairs[i].k < pairs[j].k
	})
	return pairs[0].k
}

// literalPrefixes walks a filter (including any: unions) and returns the
// list of literal prefixes from its Label / IDPrefix clauses. For label
// clauses containing "{codename}", the prefix is the substring before the
// template. For label clauses without "{codename}", the entire literal is
// the prefix. IDPrefix clauses use the literal value as-is (after stripping
// any trailing {codename}).
func literalPrefixes(f *beads.Filter) []string {
	if f == nil {
		return nil
	}
	var out []string
	if len(f.Any) > 0 {
		for i := range f.Any {
			out = append(out, literalPrefixes(&f.Any[i])...)
		}
		return out
	}
	if f.Label != "" {
		out = append(out, literalBefore(f.Label))
	}
	if f.IDPrefix != "" {
		out = append(out, literalBefore(f.IDPrefix))
	}
	return out
}

// literalBefore returns the substring before "{codename}". When the template
// variable is absent, the whole string is returned.
func literalBefore(s string) string {
	if i := strings.Index(s, "{codename}"); i >= 0 {
		return s[:i]
	}
	return s
}

// countWithPrefix counts beads whose label OR ID starts with the given
// prefix. caseSensitive controls comparison mode. A bead counts at most
// once regardless of how many of its labels match.
func countWithPrefix(bds []beads.Bead, prefix string, caseSensitive bool) int {
	if prefix == "" {
		return 0
	}
	cmpPrefix := prefix
	if !caseSensitive {
		cmpPrefix = strings.ToLower(prefix)
	}
	n := 0
	for _, b := range bds {
		if matchPrefix(b.ID, cmpPrefix, caseSensitive) {
			n++
			continue
		}
		for _, l := range b.Labels {
			if matchPrefix(l, cmpPrefix, caseSensitive) {
				n++
				break
			}
		}
	}
	return n
}

func matchPrefix(s, prefix string, caseSensitive bool) bool {
	if caseSensitive {
		return strings.HasPrefix(s, prefix)
	}
	return strings.HasPrefix(strings.ToLower(s), prefix)
}
