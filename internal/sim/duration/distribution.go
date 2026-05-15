// Package duration defines the simulator's pluggable per-phase duration
// distributions. A Distribution draws a non-negative duration in ticks
// from one of several supported families (lognormal, gamma, weibull,
// point-mass, mixture).
//
// Distributions are constructed from one of two sources:
//
//   - The fitted-distributions registry (see registry.go) — loads
//     `plans/012_real_corpus/data/fitted_distributions.yaml` once at
//     sim start and exposes the per-phase fits by name.
//   - Inline scenario spec — a scenario may declare a distribution
//     literally (kind=lognormal/gamma/weibull/point_mass/mixture) so
//     it doesn't depend on the corpus file.
//
// All implementations are pure: given the same *rand.Rand, two Sample()
// calls return the same value. The package never reaches for math/rand's
// global state.
package duration

import "math/rand"

// Distribution is the common contract for a per-phase duration model.
//
// Sample returns the next draw in ticks (or seconds — the unit is
// caller-defined; the package itself is unit-agnostic). The result is
// always >= 0.
type Distribution interface {
	Sample(r *rand.Rand) float64
}

// Family is the YAML-facing "family" tag for a distribution spec.
type Family string

const (
	FamilyLogNormal Family = "lognormal"
	FamilyGamma     Family = "gamma"
	FamilyWeibull   Family = "weibull"
	FamilyPointMass Family = "point_mass"
	FamilyMixture   Family = "mixture"
)
