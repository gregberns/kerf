// Package duration: registry — loader for fitted_distributions.yaml.
//
// The registry maps phase names (`spin_up`, `task_work`, `merge`,
// `reviewer`, `conflict_resolution`) to constructed Distribution
// values. Used by the scenario layer to resolve
// `kind: from_distribution` references against the fitted corpus.
//
// The canonical location of the YAML in the repo is
// `plans/012_real_corpus/data/fitted_distributions.yaml`. Callers that
// want the default file should use LoadDefault, which resolves the
// path via an upward search rooted at the executable's directory and
// the process cwd. LoadRegistry(path) remains the escape hatch for
// explicit overrides (tests, alternate fixtures).

package duration

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"
)

// DefaultRegistryPath is the canonical relative path used by callers
// that have not been given an explicit override. It is relative to a
// repo root; ResolveDefaultPath walks up from the executable and cwd
// to find it.
const DefaultRegistryPath = "plans/012_real_corpus/data/fitted_distributions.yaml"

// ResolveDefaultPath searches for DefaultRegistryPath by walking up
// the directory tree from each of:
//
//  1. The directory containing the running executable.
//  2. The current working directory.
//  3. The directory of this source file (via runtime.Caller). This
//     handles `go run` and `go test`, where the binary lives in the
//     Go build cache outside the repo, but the source tree (and the
//     committed YAML data file) is reachable from the package path.
//
// The first hit wins. Returns "" if not found anywhere — callers
// should treat that as "no registry available" (the same fallback
// path as a missing file).
func ResolveDefaultPath() string {
	roots := make([]string, 0, 3)
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		roots = append(roots, filepath.Dir(exe))
	}
	if cwd, err := os.Getwd(); err == nil {
		roots = append(roots, cwd)
	}
	if _, file, _, ok := runtime.Caller(0); ok {
		roots = append(roots, filepath.Dir(file))
	}
	for _, root := range roots {
		if p := searchUpward(root, DefaultRegistryPath); p != "" {
			return p
		}
	}
	return ""
}

// searchUpward walks up from start, returning the first existing path
// at start/<rel>, parent/<rel>, etc. Empty string if none exists.
func searchUpward(start, rel string) string {
	dir := start
	for {
		candidate := filepath.Join(dir, rel)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// LoadDefault loads the canonical fitted-distribution registry,
// resolving its path via ResolveDefaultPath. If the file cannot be
// found anywhere up the tree, returns (nil, nil) — matching the
// "missing file is non-fatal" contract of LoadRegistry.
func LoadDefault() (*Registry, error) {
	p := ResolveDefaultPath()
	if p == "" {
		return nil, nil
	}
	return LoadRegistry(p)
}

// Registry is the resolved name -> Distribution map.
type Registry struct {
	entries map[string]Distribution
}

// Lookup returns the registered distribution for name, or (nil, false)
// if no entry exists. A nil Registry returns (nil, false) for every
// name — the typical "fallback to inline params" code path.
func (r *Registry) Lookup(name string) (Distribution, bool) {
	if r == nil {
		return nil, false
	}
	d, ok := r.entries[name]
	return d, ok
}

// Names returns the registered distribution names in YAML-declaration
// order is not preserved (Go maps); callers must sort if they need
// determinism. Mostly used by diagnostics.
func (r *Registry) Names() []string {
	if r == nil {
		return nil
	}
	out := make([]string, 0, len(r.entries))
	for k := range r.entries {
		out = append(out, k)
	}
	return out
}

// rawEntry is the on-disk shape of one phase entry. Fields that don't
// apply to a given family are silently ignored. `variants` is parsed
// but not used by the loader — variants are an annotation for the
// human reader; scenarios that want the kerf-only fit must reference
// it explicitly under a separate name (out of scope for B-step3).
type rawEntry struct {
	Family     string                 `yaml:"family"`
	Params     map[string]float64     `yaml:"params,omitempty"`
	Components []rawMixtureComponent  `yaml:"components,omitempty"`
	// Catch-all fields ignored by the loader.
	KSPValue float64                `yaml:"ks_p,omitempty"`
	N        int                    `yaml:"n,omitempty"`
	Source   string                 `yaml:"source,omitempty"`
	Notes    string                 `yaml:"notes,omitempty"`
	Variants map[string]rawEntry    `yaml:"variants,omitempty"`
}

type rawMixtureComponent struct {
	Weight float64            `yaml:"weight"`
	Family string             `yaml:"family"`
	Params map[string]float64 `yaml:"params,omitempty"`
}

// LoadRegistry reads and parses fitted_distributions.yaml from path.
// A missing file is NOT an error — it returns (nil, nil) so the
// caller can fall back to inline scenario params. Any other error
// (malformed YAML, unknown family) is returned.
func LoadRegistry(path string) (*Registry, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("duration: read %s: %w", path, err)
	}
	return LoadRegistryBytes(b)
}

// LoadRegistryBytes parses an in-memory YAML payload. Used by tests
// and by callers that have already read the file.
func LoadRegistryBytes(b []byte) (*Registry, error) {
	raw := map[string]rawEntry{}
	if err := yaml.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("duration: parse: %w", err)
	}
	reg := &Registry{entries: make(map[string]Distribution, len(raw))}
	for name, e := range raw {
		d, err := buildDistribution(e)
		if err != nil {
			return nil, fmt.Errorf("duration: %s: %w", name, err)
		}
		reg.entries[name] = d
	}
	return reg, nil
}

// buildDistribution materialises a Distribution from a parsed rawEntry.
func buildDistribution(e rawEntry) (Distribution, error) {
	switch Family(e.Family) {
	case FamilyLogNormal:
		return LogNormal{
			Mu:    e.Params["mu"],
			Sigma: e.Params["sigma"],
		}, nil
	case FamilyGamma:
		return Gamma{
			Shape: e.Params["shape"],
			Scale: e.Params["scale"],
		}, nil
	case FamilyWeibull:
		return Weibull{
			Shape: e.Params["shape"],
			Scale: e.Params["scale"],
		}, nil
	case FamilyPointMass:
		return PointMass{Value: e.Params["value"]}, nil
	case FamilyMixture:
		comps := make([]MixtureComponent, 0, len(e.Components))
		for i, c := range e.Components {
			sub, err := buildDistribution(rawEntry{Family: c.Family, Params: c.Params})
			if err != nil {
				return nil, fmt.Errorf("component[%d]: %w", i, err)
			}
			comps = append(comps, MixtureComponent{Weight: c.Weight, Distribution: sub})
		}
		return NewMixture(comps), nil
	default:
		return nil, fmt.Errorf("unknown family %q", e.Family)
	}
}
