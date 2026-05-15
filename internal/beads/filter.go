package beads

import (
	"errors"
	"strings"
)

// Filter represents a resolved bead_filter from project.yaml or a work's
// spec.yaml. It supports two leaf clause kinds — Label and IDPrefix — and a
// union of sub-filters via Any. The literal strings may contain the template
// variable "{codename}", which is substituted at match time with the work's
// codename.
//
// Per Plan 006 / coordination spec §"Bead Attachment":
//   - Matching is case-sensitive.
//   - `any:` is a union: a single matching clause wins.
//   - `all:` is not supported in v1.
//   - A clause cannot mix a direct leaf (Label or IDPrefix) with `any:`.
//   - An empty filter (no Label, no IDPrefix, no Any entries) is invalid; if
//     somehow propagated to Match it matches nothing.
type Filter struct {
	Label    string   `yaml:"label,omitempty"`
	IDPrefix string   `yaml:"id_prefix,omitempty"`
	Any      []Filter `yaml:"any,omitempty"`
}

// DefaultFilter returns the built-in default filter used when neither a
// per-work nor a project-level filter is configured. It matches beads whose
// labels contain "work:{codename}".
func DefaultFilter() *Filter {
	return &Filter{Label: "work:{codename}"}
}

// Validate reports whether the filter is well-formed per the spec.
//   - An empty filter (no clauses) is rejected.
//   - Mixing a direct leaf (Label or IDPrefix) with `any:` is rejected.
//   - `any:` entries are validated recursively; nested empty/invalid clauses
//     surface here too.
func (f *Filter) Validate() error {
	if f == nil {
		return errors.New("bead_filter: nil filter")
	}
	hasLabel := f.Label != ""
	hasPrefix := f.IDPrefix != ""
	hasAny := len(f.Any) > 0

	if !hasLabel && !hasPrefix && !hasAny {
		return errors.New("bead_filter: empty filter (need label, id_prefix, or any)")
	}
	if hasAny && (hasLabel || hasPrefix) {
		return errors.New("bead_filter: cannot mix direct clause (label/id_prefix) with any:")
	}
	if hasLabel && hasPrefix {
		return errors.New("bead_filter: cannot mix label and id_prefix in the same clause; use any:")
	}
	for i := range f.Any {
		if err := f.Any[i].Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Match reports whether the bead matches the filter for the given codename.
// The "{codename}" template variable is substituted into Label and IDPrefix
// at match time. Matching is case-sensitive.
//
// An empty/zero-value filter matches nothing (callers should Validate first).
func (f *Filter) Match(b Bead, codename string) bool {
	if f == nil {
		return false
	}
	if len(f.Any) > 0 {
		for i := range f.Any {
			if f.Any[i].Match(b, codename) {
				return true
			}
		}
		return false
	}
	if f.Label != "" {
		want := substitute(f.Label, codename)
		for _, l := range b.Labels {
			if l == want {
				return true
			}
		}
		return false
	}
	if f.IDPrefix != "" {
		want := substitute(f.IDPrefix, codename)
		return strings.HasPrefix(b.ID, want)
	}
	// Empty filter: matches nothing.
	return false
}

// Resolve picks the effective filter for a work using first-defined-wins
// precedence: per-work → project → built-in default. Filters do not merge.
//
// Note: if a project filter is set but returns zero matches for a work
// without a per-work override, that is NOT a fall-through to the default;
// the project filter is returned unchanged. Zero matches surface as a
// cleanup item in later beads.
func Resolve(perWork, project *Filter) *Filter {
	if perWork != nil {
		return perWork
	}
	if project != nil {
		return project
	}
	return DefaultFilter()
}

// ForWorkWithFilter returns beads matching the supplied resolved filter for
// the given codename. Case-sensitive per the spec. A nil filter matches no
// beads.
func ForWorkWithFilter(all []Bead, codename string, resolved *Filter) []Bead {
	if resolved == nil {
		return nil
	}
	var result []Bead
	for _, b := range all {
		if resolved.Match(b, codename) {
			result = append(result, b)
		}
	}
	return result
}

func substitute(s, codename string) string {
	if !strings.Contains(s, "{codename}") {
		return s
	}
	return strings.ReplaceAll(s, "{codename}", codename)
}
