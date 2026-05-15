package feed

// Warning detectors per Plan 006 / B5.
//
// Spec references:
//   - specs/commands.md §"kerf next" — Behavior step 3, Warning detectors:
//     "Unmatched beads" and "Filter literal yields zero matches".
//   - specs/coordination.md §"Unmatched beads" and §"Matching is case
//     sensitive" — surface a project-level warning when the project filter's
//     literal prefix matches nothing case-sensitively but would match
//     case-insensitively.
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

// Thresholds for the unmatched_beads detector. The warning fires when the
// unmatched count is at least UnmatchedAbsThreshold OR at least
// UnmatchedFracThreshold of the total bead population.
const (
	UnmatchedAbsThreshold  = 10
	UnmatchedFracThreshold = 0.05
)

// NewWarningDetectors returns the v1 warning detectors:
//   - unmatched_beads: surfaces beads that match no work via any resolved
//     filter, once a heuristic threshold is exceeded.
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
//     caller passes a zero-value DriftResult (no in-memory last-seen yet —
//     plan 009 wires the persisted cache), the detector emits nothing.
//
// All detectors are project-level: WorkCodename and BeadID are nil and
// Score is 0. Warnings are never excluded by work state.
func NewWarningDetectors(projectFilter *beads.Filter) []Detector {
	return []Detector{
		DetectorFunc(unmatchedBeadsDetector),
		DetectorFunc(filterCaseMismatchDetector(projectFilter)),
		DetectorFunc(corruptSpecDetector),
		DetectorFunc(noProjectYAMLDetector),
		DetectorFunc(relabelDriftDetector),
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

// unmatchedBeadsDetector emits a single warning when a meaningful fraction
// of the bead store matches no work via any resolved filter. It surfaces
// the most common label-prefix (everything up to and including the first
// ':') among unmatched beads, to point the user at the misconfiguration.
//
// The detector counts unmatched beads from the post-open-filter set — that
// is, only beads that would appear in the ranked feed (isReady true).
// Closed / done / blocked / in-progress beads are excluded from the count
// so the rendered header agrees with the listed items. See Plan 008 /
// Bead 6 (kerf-ohp).
func unmatchedBeadsDetector(in Input) []Item {
	if len(in.AllBeads) == 0 {
		return nil
	}

	// Restrict to the open / ready bead set — the same set the bead feed
	// renders. This makes the unmatched count consistent with what the
	// agent sees listed below the header.
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

	// Walk every open bead; count those that match no work.
	unmatchedCount := 0
	prefixCounts := map[string]int{}
	total := len(openBeads)
	for _, b := range openBeads {
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
		unmatchedCount++
		for _, lbl := range b.Labels {
			if p, ok := labelPrefix(lbl); ok {
				prefixCounts[p]++
			}
		}
	}

	if unmatchedCount == 0 {
		return nil
	}

	// Threshold: >= 10 OR >= 5% of total.
	frac := float64(unmatchedCount) / float64(total)
	if unmatchedCount < UnmatchedAbsThreshold && frac < UnmatchedFracThreshold {
		return nil
	}

	topPrefix := mostCommonPrefix(prefixCounts)

	action := "check project bead_filter"
	if topPrefix != "" {
		action = fmt.Sprintf("check project bead_filter — top unmatched prefix: '%s'", topPrefix)
	}

	return []Item{{
		Kind:         KindWarning,
		Score:        0,
		Title:        "unmatched beads",
		Action:       action,
		Reason:       fmt.Sprintf("%d beads match no work via current filter", unmatchedCount),
		WorkCodename: nil,
		BeadID:       nil,
	}}
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
// Returns ("", false) when no ':' is present. Used to group unmatched beads
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
