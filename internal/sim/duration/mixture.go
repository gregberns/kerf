package duration

import "math/rand"

// MixtureComponent is one weighted component of a Mixture.
type MixtureComponent struct {
	Weight       float64
	Distribution Distribution
}

// Mixture is a weighted union of sub-distributions. Each Sample call
// picks a component with probability proportional to its weight and
// then draws from that component.
//
// Weights need not sum to 1.0 — the constructor normalises them on
// first use. Negative weights are clamped to zero.
type Mixture struct {
	Components []MixtureComponent

	cumWeights []float64
	total      float64
}

// NewMixture builds a Mixture and precomputes the cumulative weight
// table used at Sample time. Returns the zero-mean PointMass when no
// component has positive weight (defensive fallback).
func NewMixture(components []MixtureComponent) Mixture {
	cum := make([]float64, len(components))
	total := 0.0
	for i, c := range components {
		w := c.Weight
		if w < 0 {
			w = 0
		}
		total += w
		cum[i] = total
	}
	return Mixture{
		Components: components,
		cumWeights: cum,
		total:      total,
	}
}

// Sample picks a component by weight and draws from it.
func (m Mixture) Sample(r *rand.Rand) float64 {
	if m.total <= 0 || len(m.Components) == 0 {
		return 0
	}
	x := r.Float64() * m.total
	for i, c := range m.cumWeights {
		if x < c {
			return m.Components[i].Distribution.Sample(r)
		}
	}
	return m.Components[len(m.Components)-1].Distribution.Sample(r)
}

// Mean returns the weighted mean of the components, assuming each
// component implements the optional Mean() float64 method. Components
// without a Mean fall back to 0.
func (m Mixture) Mean() float64 {
	if m.total <= 0 {
		return 0
	}
	type meaner interface{ Mean() float64 }
	sum := 0.0
	for _, c := range m.Components {
		w := c.Weight
		if w < 0 {
			w = 0
		}
		mv := 0.0
		if mm, ok := c.Distribution.(meaner); ok {
			mv = mm.Mean()
		}
		sum += (w / m.total) * mv
	}
	return sum
}
