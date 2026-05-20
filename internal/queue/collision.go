// Collision-tolerance floor, driven by project `maturity`.
//
// Spec reference: specs/coordination.md §"Collision Tolerance (driven by
// `maturity`)". The floor is a default that downstream callers (sweep
// decision rules, eventual advisory surfaces) compare against the
// observed area-collision rate of a candidate set. It is NOT applied
// universally by `queue.Compute`; v1 does not gate `kerf next` ranking
// on the floor.
//
// Plan 014 / Bead B3 (kerf-aajh). The B1 sibling (kerf-3ka1) landed the
// `maturity` field on `project.yaml`; this file is the first consumer.
//
// The numeric defaults are kerf-internal — see the table in
// specs/coordination.md §"Collision Tolerance" for the canonical values.
// Consumers must call CollisionFloor(maturity) rather than hard-coding
// these numbers so the spec-driven values can be tuned without changing
// every call site.

package queue

import "github.com/gberns/kerf/internal/config"

// Default collision-tolerance floors per project maturity.
//
// `stable` preserves today's 5% (the value that lived in the v2 sweep
// decision rule discussed in Plan 014's reframe). `frozen` tightens to
// 2%; `experimental` loosens to 20%. See specs/coordination.md
// §"Collision Tolerance (driven by `maturity`)" for the canonical table
// and rationale.
const (
	CollisionFloorExperimental = 0.20
	CollisionFloorStable       = 0.05
	CollisionFloorFrozen       = 0.02
)

// DefaultCollisionFloor is the floor returned when maturity is unset,
// empty, or unrecognised. Matches the project-config default of
// `stable` (see config.DefaultMaturity).
const DefaultCollisionFloor = CollisionFloorStable

// CollisionFloor returns the maturity-driven collision-tolerance floor.
//
// The empty string and any value not in the closed enum
// {experimental, stable, frozen} resolve to DefaultCollisionFloor; this
// mirrors ProjectConfig.ResolvedMaturity()'s default-application
// behaviour and means callers can safely pass cfg.Maturity directly
// (without first calling ResolvedMaturity).
//
// The floor is a fraction in [0.0, 1.0] interpreted as the maximum
// share of work pairs in a candidate set that may share an `areas:`
// label before downstream callers treat the set as collision-saturated.
// Compare against `static.AreaOverlap` (or the area_collisions rate
// reported by the simulator) on the same set.
func CollisionFloor(maturity string) float64 {
	switch maturity {
	case config.MaturityExperimental:
		return CollisionFloorExperimental
	case config.MaturityStable:
		return CollisionFloorStable
	case config.MaturityFrozen:
		return CollisionFloorFrozen
	default:
		return DefaultCollisionFloor
	}
}
