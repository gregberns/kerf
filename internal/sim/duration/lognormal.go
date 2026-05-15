package duration

import (
	"math"
	"math/rand"
)

// LogNormal is a log-normal distribution parameterised by (mu, sigma) in
// natural-log space. Sample returns exp(mu + sigma*N(0,1)).
type LogNormal struct {
	Mu    float64
	Sigma float64
}

// Sample draws one value.
func (d LogNormal) Sample(r *rand.Rand) float64 {
	x := math.Exp(d.Mu + d.Sigma*r.NormFloat64())
	if x < 0 {
		return 0
	}
	return x
}

// Mean returns the analytic mean exp(mu + sigma^2/2). Used by tests.
func (d LogNormal) Mean() float64 {
	return math.Exp(d.Mu + d.Sigma*d.Sigma/2.0)
}
