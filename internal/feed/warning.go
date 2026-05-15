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
//
// Both detectors are project-level: WorkCodename and BeadID are nil and
// Score is 0. Warnings are never excluded by work state.
func NewWarningDetectors(projectFilter *beads.Filter) []Detector {
	return []Detector{
		DetectorFunc(unmatchedBeadsDetector),
		DetectorFunc(filterCaseMismatchDetector(projectFilter)),
	}
}

// unmatchedBeadsDetector emits a single warning when a meaningful fraction
// of the bead store matches no work via any resolved filter. It surfaces
// the most common label-prefix (everything up to and including the first
// ':') among unmatched beads, to point the user at the misconfiguration.
func unmatchedBeadsDetector(in Input) []Item {
	if len(in.AllBeads) == 0 {
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

	// Walk every bead; count those that match no work.
	unmatchedCount := 0
	prefixCounts := map[string]int{}
	total := len(in.AllBeads)
	for _, b := range in.AllBeads {
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
