package duration

import "math/rand"

// PointMass is a degenerate "distribution" that always returns the same
// value. Useful as a mixture component (e.g. "95% chance of zero").
type PointMass struct {
	Value float64
}

// Sample returns Value verbatim.
func (d PointMass) Sample(_ *rand.Rand) float64 { return d.Value }

// Mean returns Value.
func (d PointMass) Mean() float64 { return d.Value }
