package queue

import (
	"testing"

	"github.com/gberns/kerf/internal/config"
)

// newCollisionFloorCase is a per-tier expectation used by the table
// tests below. Named specifically (not the generic `testCase`) to avoid
// the collision-name incident referenced in CLAUDE.md (three sibling
// worktrees previously each landed a `newTestContext` that merged to a
// redeclaration failure).
type newCollisionFloorCase struct {
	name     string
	maturity string
	want     float64
}

// TestCollisionFloor_perMaturityTier asserts the three spec-pinned
// values from specs/coordination.md §"Collision Tolerance (driven by
// `maturity`)". `stable` MUST preserve today's 5% — this is the
// non-regression baseline named in the bead's definition-of-done.
func TestCollisionFloor_perMaturityTier(t *testing.T) {
	cases := []newCollisionFloorCase{
		{name: "experimental loosens to 0.20", maturity: config.MaturityExperimental, want: 0.20},
		{name: "stable preserves 0.05 (today's value)", maturity: config.MaturityStable, want: 0.05},
		{name: "frozen tightens to 0.02", maturity: config.MaturityFrozen, want: 0.02},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CollisionFloor(tc.maturity)
			if got != tc.want {
				t.Fatalf("CollisionFloor(%q) = %v, want %v", tc.maturity, got, tc.want)
			}
		})
	}
}

// TestCollisionFloor_emptyDefaultsToStable mirrors the empty-string
// default-application rule in ProjectConfig.ResolvedMaturity().
// Callers should be able to pass cfg.Maturity directly without
// first resolving the default.
func TestCollisionFloor_emptyDefaultsToStable(t *testing.T) {
	got := CollisionFloor("")
	if got != CollisionFloorStable {
		t.Fatalf("CollisionFloor(\"\") = %v, want %v (stable default)", got, CollisionFloorStable)
	}
}

// TestCollisionFloor_unknownDefaultsToStable guards the closed-enum
// boundary. The config loader rejects unknown maturities at load time
// (see internal/config/project.go validateMaturity), but the queue
// helper is defensive: any value outside the enum resolves to the
// stable default rather than panicking or returning a sentinel.
func TestCollisionFloor_unknownDefaultsToStable(t *testing.T) {
	for _, v := range []string{"unknown", "FROZEN", "Stable", "experiemental"} {
		if got := CollisionFloor(v); got != DefaultCollisionFloor {
			t.Errorf("CollisionFloor(%q) = %v, want %v", v, got, DefaultCollisionFloor)
		}
	}
}

// TestCollisionFloor_orderingInvariant guards the directional spec
// language ("frozen tightens; experimental loosens") at the value
// level. If a future tuner updates the constants, the relative
// ordering MUST hold or the spec sentence is wrong.
func TestCollisionFloor_orderingInvariant(t *testing.T) {
	frozen := CollisionFloor(config.MaturityFrozen)
	stable := CollisionFloor(config.MaturityStable)
	experimental := CollisionFloor(config.MaturityExperimental)
	if !(frozen < stable && stable < experimental) {
		t.Fatalf("ordering broken: frozen=%v stable=%v experimental=%v; want frozen < stable < experimental",
			frozen, stable, experimental)
	}
}

// TestCollisionFloor_consumesResolvedMaturity verifies the helper
// composes with the project-config layer: a freshly-loaded
// ProjectConfig with no `maturity:` field yields the stable floor when
// its ResolvedMaturity() is fed to CollisionFloor. This is the
// integration path the eventual sweep-decision and advise consumers
// will follow.
func TestCollisionFloor_consumesResolvedMaturity(t *testing.T) {
	var cfg *config.ProjectConfig // nil → ResolvedMaturity returns default
	if got := CollisionFloor(cfg.ResolvedMaturity()); got != CollisionFloorStable {
		t.Fatalf("nil cfg path: CollisionFloor(ResolvedMaturity()) = %v, want %v", got, CollisionFloorStable)
	}
	cfg = &config.ProjectConfig{} // empty Maturity → default
	if got := CollisionFloor(cfg.ResolvedMaturity()); got != CollisionFloorStable {
		t.Fatalf("empty cfg path: CollisionFloor(ResolvedMaturity()) = %v, want %v", got, CollisionFloorStable)
	}
	cfg.Maturity = config.MaturityFrozen
	if got := CollisionFloor(cfg.ResolvedMaturity()); got != CollisionFloorFrozen {
		t.Fatalf("frozen cfg path: CollisionFloor(ResolvedMaturity()) = %v, want %v", got, CollisionFloorFrozen)
	}
}
