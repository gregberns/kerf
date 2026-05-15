package duration

import (
	"math"
	"math/rand"
)

// Gamma is a Gamma distribution with the standard (shape k, scale theta)
// parameterisation. Mean = k * theta, Variance = k * theta^2.
type Gamma struct {
	Shape float64
	Scale float64
}

// Sample draws one value using Marsaglia & Tsang's method for shape >= 1
// and the boost trick for shape < 1. Result is always >= 0.
func (d Gamma) Sample(r *rand.Rand) float64 {
	k := d.Shape
	if k <= 0 || d.Scale <= 0 {
		return 0
	}
	if k < 1 {
		// Boost: X = Y * U^(1/k) where Y ~ Gamma(k+1).
		boosted := Gamma{Shape: k + 1, Scale: d.Scale}.Sample(r)
		u := r.Float64()
		return boosted * math.Pow(u, 1.0/k)
	}
	d3 := k - 1.0/3.0
	c := 1.0 / math.Sqrt(9.0*d3)
	for {
		var x, v float64
		for {
			x = r.NormFloat64()
			v = 1.0 + c*x
			if v > 0 {
				break
			}
		}
		v = v * v * v
		u := r.Float64()
		if u < 1.0-0.0331*x*x*x*x {
			return d3 * v * d.Scale
		}
		if math.Log(u) < 0.5*x*x+d3*(1.0-v+math.Log(v)) {
			return d3 * v * d.Scale
		}
	}
}

// Mean returns k * theta.
func (d Gamma) Mean() float64 { return d.Shape * d.Scale }
