// Package scenario: resolve.go — converts a scenario.Duration spec into a
// runtime duration.Distribution. The registry is consulted only for
// kind=from_distribution entries; inline kinds (lognormal/gamma/weibull/
// point_mass/mixture) build a Distribution directly from the YAML fields.

package scenario

import (
	"fmt"
	"math"

	"github.com/gregberns/kerf/internal/sim/duration"
)

// Resolve returns the runtime Distribution implied by d. reg may be nil
// — in which case kind=from_distribution is a hard error. Inline kinds
// work fine without the registry.
func (d Duration) Resolve(reg *duration.Registry) (duration.Distribution, error) {
	switch d.Kind {
	case DurationKindLogNormal:
		mu, err := d.Mu()
		if err != nil {
			return nil, err
		}
		return duration.LogNormal{Mu: mu, Sigma: d.Sigma}, nil
	case DurationKindFromDistribution:
		if reg == nil {
			return nil, fmt.Errorf("scenario: kind=from_distribution requires a loaded distribution registry (looked up %q)", d.Distribution)
		}
		dist, ok := reg.Lookup(d.Distribution)
		if !ok {
			return nil, fmt.Errorf("scenario: distribution %q not found in registry", d.Distribution)
		}
		return dist, nil
	case DurationKindGamma:
		return duration.Gamma{Shape: d.Shape, Scale: d.Scale}, nil
	case DurationKindWeibull:
		return duration.Weibull{Shape: d.Shape, Scale: d.Scale}, nil
	case DurationKindPointMass:
		return duration.PointMass{Value: d.Value}, nil
	case DurationKindMixture:
		comps := make([]duration.MixtureComponent, 0, len(d.Components))
		for i, c := range d.Components {
			sub, err := c.resolveComponent(reg)
			if err != nil {
				return nil, fmt.Errorf("component[%d]: %w", i, err)
			}
			comps = append(comps, duration.MixtureComponent{Weight: c.Weight, Distribution: sub})
		}
		return duration.NewMixture(comps), nil
	default:
		return nil, fmt.Errorf("scenario: unsupported duration kind %q", d.Kind)
	}
}

// resolveComponent maps a mixture-component leaf to a Distribution. The
// component schema accepts both YAML shapes the project speaks: the
// scenario-native (kind/sigma/...) and the registry-native
// (family/params:{...}).
func (c DurationComponent) resolveComponent(reg *duration.Registry) (duration.Distribution, error) {
	kind := c.Kind
	if kind == "" {
		kind = c.Family
	}
	// Promote params{} into named fields if present (fitted_distributions.yaml style).
	shape := c.Shape
	scale := c.Scale
	val := c.Value
	mu := c.Mu
	sigma := c.Sigma
	if c.Params != nil {
		if v, ok := c.Params["shape"]; ok {
			shape = v
		}
		if v, ok := c.Params["scale"]; ok {
			scale = v
		}
		if v, ok := c.Params["value"]; ok {
			val = v
		}
		if v, ok := c.Params["mu"]; ok {
			mu = v
		}
		if v, ok := c.Params["sigma"]; ok {
			sigma = v
		}
	}
	switch kind {
	case DurationKindLogNormal:
		if mu == 0 {
			// Compute from mean/median if provided.
			d := Duration{Kind: DurationKindLogNormal, Sigma: sigma, MeanTicks: c.MeanTicks, MedianTicks: c.MedianTicks}
			if d.MeanTicks != nil || d.MedianTicks != nil {
				m, err := d.Mu()
				if err != nil {
					return nil, err
				}
				mu = m
			}
		}
		if sigma == 0 {
			return nil, fmt.Errorf("lognormal: sigma must be > 0")
		}
		if math.IsNaN(mu) {
			return nil, fmt.Errorf("lognormal: mu is NaN")
		}
		return duration.LogNormal{Mu: mu, Sigma: sigma}, nil
	case DurationKindGamma:
		return duration.Gamma{Shape: shape, Scale: scale}, nil
	case DurationKindWeibull:
		return duration.Weibull{Shape: shape, Scale: scale}, nil
	case DurationKindPointMass:
		return duration.PointMass{Value: val}, nil
	case DurationKindFromDistribution:
		if reg == nil {
			return nil, fmt.Errorf("kind=from_distribution requires a registry")
		}
		dist, ok := reg.Lookup(c.Distribution)
		if !ok {
			return nil, fmt.Errorf("distribution %q not found", c.Distribution)
		}
		return dist, nil
	default:
		return nil, fmt.Errorf("unsupported mixture component kind %q", kind)
	}
}
