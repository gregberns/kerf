package duration

import (
	"math"
	"math/rand"
)

// Weibull is a Weibull distribution with (shape k, scale lambda).
// Sample is computed via the inverse CDF: lambda * (-ln(1-U))^(1/k).
type Weibull struct {
	Shape float64
	Scale float64
}

// Sample draws one value.
func (d Weibull) Sample(r *rand.Rand) float64 {
	if d.Shape <= 0 || d.Scale <= 0 {
		return 0
	}
	// Use 1 - r.Float64() to avoid log(0) when r returns 0.
	u := 1.0 - r.Float64()
	return d.Scale * math.Pow(-math.Log(u), 1.0/d.Shape)
}

// Mean returns lambda * Gamma(1 + 1/k).
func (d Weibull) Mean() float64 {
	return d.Scale * math.Gamma(1.0+1.0/d.Shape)
}
