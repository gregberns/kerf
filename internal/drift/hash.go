// Package drift provides bead-store drift detection: snapshot capture,
// per-bead hashing, snapshot-to-snapshot diffing, and read/write of the
// project-local sync cache (`.kerf/sync-cache.json`).
//
// See specs/coordination.md §"Drift Detection" for the canonical hash
// scope, snapshot shape, and baseline-advancement rules.
package drift

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"

	"github.com/gberns/kerf/internal/beads"
)

// HashBead computes the canonical per-bead drift hash, per
// specs/coordination.md §"Hash scope".
//
// The hash is sha256, lowercase-hex, over the UTF-8 byte sequence formed
// by concatenating these five components, each terminated by a single
// `\n` (0x0A) — including the final line:
//
//  1. id=<bead_id>
//  2. status=<status>           lowercased
//  3. title=<title>             verbatim
//  4. labels=<l1>,<l2>,...,<ln> lowercased, deduplicated, lexicographically sorted, comma-joined
//  5. deps=<d1>,<d2>,...,<dn>   lexicographically sorted, comma-joined
//
// Empty labels/deps lists encode as `labels=` and `deps=` respectively.
// Fields the beads system carries that kerf does not consume (assignees,
// timestamps, body text, priority, etc.) are not part of the hash. This
// keeps the hash stable across changes kerf does not act on.
func HashBead(b beads.Bead) string {
	var sb strings.Builder
	sb.WriteString("id=")
	sb.WriteString(b.ID)
	sb.WriteByte('\n')

	sb.WriteString("status=")
	sb.WriteString(strings.ToLower(b.Status))
	sb.WriteByte('\n')

	sb.WriteString("title=")
	sb.WriteString(b.Title)
	sb.WriteByte('\n')

	sb.WriteString("labels=")
	sb.WriteString(strings.Join(normalizeLabels(b.Labels), ","))
	sb.WriteByte('\n')

	sb.WriteString("deps=")
	sb.WriteString(strings.Join(normalizeDeps(b.DependsOn), ","))
	sb.WriteByte('\n')

	sum := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(sum[:])
}

// normalizeLabels lowercases, deduplicates, and sorts a label slice.
// The result is a freshly-allocated slice; the input is not mutated.
func normalizeLabels(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, l := range in {
		lc := strings.ToLower(l)
		if _, ok := seen[lc]; ok {
			continue
		}
		seen[lc] = struct{}{}
		out = append(out, lc)
	}
	sort.Strings(out)
	return out
}

// normalizeDeps sorts a dependency ID slice. The result is a
// freshly-allocated slice; the input is not mutated.
func normalizeDeps(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	sort.Strings(out)
	return out
}
