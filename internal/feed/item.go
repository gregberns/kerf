// Package feed assembles the heterogeneous item list shown by `kerf next`.
//
// Spec references:
//   - specs/commands.md §"kerf next" — Behavior, item shape, flag precedence,
//     JSON output ("<id or null>").
//   - specs/coordination.md §"Priority and Ordering" — cross-kind sort and
//     cleanup tie-break by parent-work created timestamp.
//
// The feed package is pure: it composes inputs from internal/beads,
// internal/queue, and internal/spec into a single ordered slice of Items.
// Rendering and flag parsing live in cmd/next.go (Bead B6).
package feed

import "fmt"

// Kind enumerates the categories of feed items. The string value is the
// canonical lowercase token used in JSON output and on the `--kinds`,
// `--only`, and `--include` flags.
type Kind string

const (
	KindBead    Kind = "bead"
	KindCleanup Kind = "cleanup"
	KindWarning Kind = "warning"
)

// String returns the canonical lowercase token for the kind.
func (k Kind) String() string { return string(k) }

// KnownKinds returns the full set of feed item kinds in declaration order.
func KnownKinds() []Kind {
	return []Kind{KindBead, KindCleanup, KindWarning}
}

// ParseKind parses a kind token (case-insensitive in tolerance, but the
// canonical representation is lowercase).
func ParseKind(s string) (Kind, error) {
	for _, k := range KnownKinds() {
		if string(k) == s {
			return k, nil
		}
	}
	return "", fmt.Errorf("unknown kind %q (valid: bead, cleanup, warning)", s)
}

// Item is one entry in the feed produced by `kerf next`.
//
// JSON tags use snake_case. WorkCodename and BeadID are pointer types so that
// an absent value serializes as literal `null` (per the `kerf next` spec's
// "<id or null>" requirement). They must NOT carry `omitempty` — the keys
// are always present.
type Item struct {
	Kind         Kind    `json:"kind"`
	Score        float64 `json:"score"`
	Title        string  `json:"title"`
	Action       string  `json:"action"`
	WorkCodename *string `json:"work_codename"`
	BeadID       *string `json:"bead_id"`
	Reason       string  `json:"reason"`

	// RankLabel carries the rank-label vocabulary defined in
	// specs/coordination.md §"Rank Labels for Zero-Match Works":
	// "empty", "unwired", or "broken". Populated only for cleanup items
	// emitted by the work_no_attached_beads detector; empty string for all
	// other items. Per Plan 019 / B2 and Plan 019 open question 5, the
	// "broken" state collapses into "empty" while spec.Read still rejects
	// malformed bead_filter values at parse time — the field is preserved
	// so the third label becomes available when parser support lands.
	RankLabel string `json:"rank_label,omitempty"`
}
