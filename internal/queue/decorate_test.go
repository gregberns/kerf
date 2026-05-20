package queue

import (
	"strings"
	"testing"

	"github.com/gberns/kerf/internal/spec"
)

// TestDecorateGraphSignals_toyGraph exercises the T=0 analyzer wired
// through the queue decorator on a 5-work toy graph:
//
//	alpha (cli)        epsilon (db)
//	    \
//	     gamma (cli, core)
//	    /
//	beta (cli)
//	    \
//	     delta (cli)
//
// Edges: alpha,beta -> gamma; beta -> delta. Critical paths of length 2
// include alpha->gamma and beta->gamma/delta. Lex tie-break makes "alpha"
// the chain root and "gamma" the tail.
func TestDecorateGraphSignals_toyGraph(t *testing.T) {
	dep := func(codename string) spec.Dependency {
		return spec.Dependency{Codename: codename, Relationship: "must-complete-first"}
	}
	works := []*spec.SpecYAML{
		{Codename: "alpha", Areas: []string{"cli"}},
		{Codename: "beta", Areas: []string{"cli"}},
		{Codename: "gamma", Areas: []string{"cli", "core"}, DependsOn: []spec.Dependency{dep("alpha"), dep("beta")}},
		{Codename: "delta", Areas: []string{"cli"}, DependsOn: []spec.Dependency{dep("beta")}},
		{Codename: "epsilon", Areas: []string{"db"}},
	}
	entries := []Entry{
		{Codename: "alpha", Areas: []string{"cli"}},
		{Codename: "beta", Areas: []string{"cli"}},
		{Codename: "gamma", Areas: []string{"cli", "core"}},
		{Codename: "delta", Areas: []string{"cli"}},
		{Codename: "epsilon", Areas: []string{"db"}},
	}

	added := DecorateGraphSignals(entries, works)

	// alpha is on the critical path and has fan-out 1 (gamma).
	if !containsPrefix(added["alpha"], "on critical path") {
		t.Errorf("alpha expected on-critical-path signal, got %v", added["alpha"])
	}
	if !containsPrefix(added["alpha"], "graph fan-out 1") {
		t.Errorf("alpha expected fan-out 1, got %v", added["alpha"])
	}

	// beta has fan-out 2 (gamma + delta), and is NOT on the chain
	// (alpha wins lex tie-break).
	if containsPrefix(added["beta"], "on critical path") {
		t.Errorf("beta should not be on critical path (lex tie-break to alpha): %v", added["beta"])
	}
	if !containsPrefix(added["beta"], "graph fan-out 2") {
		t.Errorf("beta expected fan-out 2, got %v", added["beta"])
	}

	// gamma is the chain tail; on critical path but no fan-out.
	if !containsPrefix(added["gamma"], "on critical path") {
		t.Errorf("gamma expected on-critical-path signal, got %v", added["gamma"])
	}
	if containsPrefix(added["gamma"], "graph fan-out") {
		t.Errorf("gamma should not surface fan-out (zero dependents): %v", added["gamma"])
	}

	// Every entry gets area-overlap (>=2 candidates with declared areas).
	for _, cn := range []string{"alpha", "beta", "gamma", "delta", "epsilon"} {
		if !containsPrefix(added[cn], "area-overlap density") {
			t.Errorf("%s missing area-overlap signal, got %v", cn, added[cn])
		}
	}

	// Decorator must have actually appended to Entry.Reasons (read-only
	// elsewhere — append-only here).
	for _, e := range entries {
		if len(e.Reasons) == 0 {
			t.Errorf("entry %s: Reasons not populated", e.Codename)
		}
	}
}

func TestDecorateGraphSignals_emptyInputs(t *testing.T) {
	if got := DecorateGraphSignals(nil, nil); got != nil {
		t.Errorf("expected nil for empty inputs, got %v", got)
	}
	if got := DecorateGraphSignals(nil, []*spec.SpecYAML{{Codename: "a"}}); got != nil {
		t.Errorf("expected nil when no entries, got %v", got)
	}
}

func containsPrefix(s []string, prefix string) bool {
	for _, x := range s {
		if strings.HasPrefix(x, prefix) {
			return true
		}
	}
	return false
}
