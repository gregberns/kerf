package beads

import (
	"sort"
	"strings"
)

// PrefixCount holds the raw bead count for a single label prefix.
// "Prefix" here is the substring before the first ':' in a label
// (so the label "subsystem:auth" has prefix "subsystem"). Grouping is
// case-sensitive per the bead-attachment spec.
type PrefixCount struct {
	Prefix string
	Count  int
}

// DetectFilterPrefix implements the kerf init step 8 auto-detect algorithm
// (specs/commands.md §"kerf init", step 8).
//
// It returns:
//   - prefix: the chosen prefix P (without the trailing ':'), if any candidate
//     scored above the 0.5 confidence threshold. Empty otherwise.
//   - matchScore: the score for the chosen prefix (0 if none qualifies).
//   - topByCount: the top 5 prefixes by raw bead count, with ties broken
//     alphabetically. Always populated when there is at least one qualifying
//     prefix in the store (>= 3 beads).
//
// Algorithm:
//  1. Collect label prefixes that appear on at least 3 beads.
//  2. For each prefix P, match_score = (beads whose labels contain
//     "P:{codename}" for some codename) / (beads carrying any "P:*" label).
//  3. Pick the highest match_score above 0.5. Ties between equally-confident
//     candidates are broken by raw count (then alphabetically).
//
// A bead is counted once per prefix regardless of how many "P:*" labels it
// carries. Likewise it counts once toward the numerator if any of its
// "P:*" labels matches some codename.
//
// If codenames is empty the function still returns top-by-count data so the
// caller can decide how to present the fallback prompt; matchScore will be 0.
func DetectFilterPrefix(all []Bead, codenames []string) (prefix string, matchScore float64, topByCount []PrefixCount) {
	// Build a set of codenames for O(1) membership tests.
	codenameSet := make(map[string]struct{}, len(codenames))
	for _, cn := range codenames {
		if cn != "" {
			codenameSet[cn] = struct{}{}
		}
	}

	// For each prefix, count: total beads carrying any "P:*" label, and
	// beads carrying at least one "P:{codename}" label.
	type stats struct {
		total   int
		matched int
	}
	prefixStats := make(map[string]*stats)

	for _, b := range all {
		// Deduplicate prefixes per bead — a bead with multiple "subsystem:*"
		// labels should only bump the "subsystem" totals once.
		seenPrefix := make(map[string]bool)
		matchedPrefix := make(map[string]bool)

		for _, label := range b.Labels {
			idx := strings.IndexByte(label, ':')
			if idx <= 0 || idx == len(label)-1 {
				continue
			}
			p := label[:idx]
			rest := label[idx+1:]

			seenPrefix[p] = true
			if _, ok := codenameSet[rest]; ok {
				matchedPrefix[p] = true
			}
		}

		for p := range seenPrefix {
			s, ok := prefixStats[p]
			if !ok {
				s = &stats{}
				prefixStats[p] = s
			}
			s.total++
			if matchedPrefix[p] {
				s.matched++
			}
		}
	}

	// Filter to prefixes with at least 3 beads.
	type scored struct {
		prefix string
		total  int
		score  float64
	}
	var candidates []scored
	for p, s := range prefixStats {
		if s.total < 3 {
			continue
		}
		score := 0.0
		if s.total > 0 {
			score = float64(s.matched) / float64(s.total)
		}
		candidates = append(candidates, scored{prefix: p, total: s.total, score: score})
	}

	if len(candidates) == 0 {
		return "", 0, nil
	}

	// Build top-by-count (descending count, alphabetical tiebreak), limited to 5.
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].total != candidates[j].total {
			return candidates[i].total > candidates[j].total
		}
		return candidates[i].prefix < candidates[j].prefix
	})
	limit := len(candidates)
	if limit > 5 {
		limit = 5
	}
	topByCount = make([]PrefixCount, 0, limit)
	for i := 0; i < limit; i++ {
		topByCount = append(topByCount, PrefixCount{Prefix: candidates[i].prefix, Count: candidates[i].total})
	}

	// Pick the highest match_score above 0.5. Ties broken by raw count, then alphabetically.
	bestIdx := -1
	for i := range candidates {
		if candidates[i].score <= 0.5 {
			continue
		}
		if bestIdx == -1 {
			bestIdx = i
			continue
		}
		b := candidates[bestIdx]
		c := candidates[i]
		switch {
		case c.score > b.score:
			bestIdx = i
		case c.score == b.score && c.total > b.total:
			bestIdx = i
		case c.score == b.score && c.total == b.total && c.prefix < b.prefix:
			bestIdx = i
		}
	}

	if bestIdx == -1 {
		return "", 0, topByCount
	}
	return candidates[bestIdx].prefix, candidates[bestIdx].score, topByCount
}
