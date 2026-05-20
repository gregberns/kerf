package static

import (
	"math"
	"reflect"
	"testing"
)

// toyGraph returns a five-node graph used across the analyzer tests.
//
//	a (areas: cli)        e  (areas: db)
//	 \
//	  c (areas: cli,core)
//	 /
//	b (areas: cli)
//	 \
//	  d (areas: cli)
//
// Edges (dependency -> dependent):
//
//	a -> c
//	b -> c
//	b -> d
//
// "e" is an isolated node. Estimates are all 1.0 so CriticalPath returns
// the chain length in node-count units.
func toyGraph() Graph {
	return NewGraph([]Node{
		{ID: "a", Estimate: 1, Areas: []string{"cli"}},
		{ID: "b", Estimate: 1, Areas: []string{"cli"}},
		{ID: "c", Estimate: 1, Areas: []string{"cli", "core"}, DependsOn: []string{"a", "b"}},
		{ID: "d", Estimate: 1, Areas: []string{"cli"}, DependsOn: []string{"b"}},
		{ID: "e", Estimate: 1, Areas: []string{"db"}},
	})
}

func TestCriticalPath_toyGraph(t *testing.T) {
	chain, dur := CriticalPath(toyGraph())
	if dur != 2.0 {
		t.Fatalf("dur = %v, want 2.0", dur)
	}
	// Two chains are length 2: a->c and b->c (and b->d). Lex tie-break
	// on chain root means "a" wins over "b", so the chain is a->c.
	if !reflect.DeepEqual(chain, []string{"a", "c"}) {
		t.Fatalf("chain = %v, want [a c]", chain)
	}
}

func TestCriticalPath_singleNode(t *testing.T) {
	g := NewGraph([]Node{{ID: "only", Estimate: 3}})
	chain, dur := CriticalPath(g)
	if dur != 3 || !reflect.DeepEqual(chain, []string{"only"}) {
		t.Fatalf("got chain=%v dur=%v", chain, dur)
	}
}

func TestCriticalPath_weighted(t *testing.T) {
	// a (1) -> b (5): chain length is 6.
	g := NewGraph([]Node{
		{ID: "a", Estimate: 1},
		{ID: "b", Estimate: 5, DependsOn: []string{"a"}},
	})
	chain, dur := CriticalPath(g)
	if dur != 6 {
		t.Fatalf("dur = %v, want 6", dur)
	}
	if !reflect.DeepEqual(chain, []string{"a", "b"}) {
		t.Fatalf("chain = %v", chain)
	}
}

func TestCriticalPath_empty(t *testing.T) {
	chain, dur := CriticalPath(Graph{})
	if chain != nil || dur != 0 {
		t.Fatalf("got chain=%v dur=%v", chain, dur)
	}
}

func TestFanOut_toyGraph(t *testing.T) {
	g := toyGraph()
	cases := []struct {
		id   string
		want int
	}{
		{"a", 1}, // c
		{"b", 2}, // c, d
		{"c", 0},
		{"d", 0},
		{"e", 0},
		{"missing", 0},
	}
	for _, tc := range cases {
		got := FanOut(g, tc.id)
		if got != tc.want {
			t.Errorf("FanOut(%q) = %d, want %d", tc.id, got, tc.want)
		}
	}
}

func TestAreaOverlap_toyGraph(t *testing.T) {
	g := toyGraph()
	// {a,b,d}: all share "cli" — every pair overlaps. 3 pairs, 3 overlap → 1.0.
	got := AreaOverlap(g, []string{"a", "b", "d"})
	if math.Abs(got-1.0) > 1e-9 {
		t.Errorf("a/b/d overlap = %v, want 1.0", got)
	}
	// {a,e}: cli vs db — no overlap. 1 pair, 0 overlap → 0.0.
	got = AreaOverlap(g, []string{"a", "e"})
	if got != 0 {
		t.Errorf("a/e overlap = %v, want 0", got)
	}
	// {a,c,e}: 3 pairs. a/c share cli; a/e none; c/e none → 1/3.
	got = AreaOverlap(g, []string{"a", "c", "e"})
	want := 1.0 / 3.0
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("a/c/e overlap = %v, want %v", got, want)
	}
}

func TestAreaOverlap_degenerate(t *testing.T) {
	g := toyGraph()
	if got := AreaOverlap(g, nil); got != 0 {
		t.Errorf("nil set: got %v", got)
	}
	if got := AreaOverlap(g, []string{"a"}); got != 0 {
		t.Errorf("single node: got %v", got)
	}
	if got := AreaOverlap(g, []string{"missing-1", "missing-2"}); got != 0 {
		t.Errorf("unknown nodes: got %v", got)
	}
}
