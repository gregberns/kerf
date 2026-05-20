// Package static is the T=0 static analyzer for kerf's process-management
// layer. It computes graph-shape signals on a dependency graph — critical
// path, fan-out, and area overlap — that surface as additional `reason`
// strings on `kerf next --format=json` output.
//
// Spec reference: specs/coordination.md §"Graph signals (T=0 static
// analyzer)".
//
// The package is graph-generic: it operates on a plain Graph struct rather
// than reaching into kerf's spec/queue types. The queue decorator adapts
// `*spec.SpecYAML` slices into a Graph before calling these functions.
//
// All functions are pure — same input, same output, no I/O.
package static

import (
	"sort"
)

// Node is a single graph node. Estimate is the duration weight used by
// CriticalPath (units are caller-defined; today kerf passes 1.0 per node so
// CriticalPath returns the longest-chain length in node count). Areas is the
// set of named regions the node touches; AreaOverlap reads this slice.
type Node struct {
	ID       string
	Estimate float64
	Areas    []string
	// DependsOn is the list of node IDs this node depends on (i.e., this
	// node is a *dependent* of each listed ID — the edge points from
	// dependency to dependent). Matches the orientation of
	// spec.SpecYAML.DependsOn.
	DependsOn []string
}

// Graph is a directed acyclic graph of work nodes keyed by ID. The caller is
// responsible for ensuring no cycles; CriticalPath assumes acyclicity and
// would loop on a cyclic graph.
type Graph struct {
	Nodes map[string]Node
}

// NewGraph constructs a Graph from a slice of nodes, dropping later
// duplicates by ID.
func NewGraph(nodes []Node) Graph {
	m := make(map[string]Node, len(nodes))
	for _, n := range nodes {
		if _, ok := m[n.ID]; ok {
			continue
		}
		m[n.ID] = n
	}
	return Graph{Nodes: m}
}

// CriticalPath returns the longest weighted chain through the graph,
// measured by summed node Estimate. The first return is the chain of node
// IDs from chain-root to chain-tail; the second is the summed estimate. A
// graph with zero nodes returns (nil, 0).
//
// Implementation: standard DAG longest-path via memoized DFS over the
// reverse dependency graph. Complexity O(V+E).
func CriticalPath(g Graph) ([]string, float64) {
	if len(g.Nodes) == 0 {
		return nil, 0
	}
	// memo[id] = (length, nextID-in-chain-after-id-or-"")
	type memo struct {
		length float64
		next   string
	}
	cache := make(map[string]memo, len(g.Nodes))

	// Build forward edges: dependency -> dependents.
	dependents := make(map[string][]string, len(g.Nodes))
	for _, n := range g.Nodes {
		for _, dep := range n.DependsOn {
			dependents[dep] = append(dependents[dep], n.ID)
		}
	}
	// Sort dependents for deterministic tie-break.
	for k := range dependents {
		sort.Strings(dependents[k])
	}

	var visit func(id string) memo
	visit = func(id string) memo {
		if m, ok := cache[id]; ok {
			return m
		}
		n, ok := g.Nodes[id]
		if !ok {
			cache[id] = memo{}
			return cache[id]
		}
		best := memo{length: n.Estimate, next: ""}
		for _, d := range dependents[id] {
			sub := visit(d)
			candidate := n.Estimate + sub.length
			if candidate > best.length {
				best = memo{length: candidate, next: d}
			}
		}
		cache[id] = best
		return best
	}

	// Walk every node so isolated nodes are considered as single-node
	// chains. Pick the chain with greatest length; deterministic
	// tie-break by lexicographic root ID.
	var rootIDs []string
	for id := range g.Nodes {
		rootIDs = append(rootIDs, id)
	}
	sort.Strings(rootIDs)

	var bestRoot string
	var bestLen float64
	for _, id := range rootIDs {
		m := visit(id)
		if m.length > bestLen {
			bestLen = m.length
			bestRoot = id
		}
	}
	if bestRoot == "" {
		return nil, 0
	}

	// Reconstruct chain.
	var chain []string
	curr := bestRoot
	for curr != "" {
		chain = append(chain, curr)
		curr = cache[curr].next
	}
	return chain, bestLen
}

// FanOut returns the count of direct dependents of nodeID — that is, how
// many nodes in the graph list nodeID in their DependsOn. Returns 0 when
// the node does not exist or has no dependents.
func FanOut(g Graph, nodeID string) int {
	if _, ok := g.Nodes[nodeID]; !ok {
		return 0
	}
	count := 0
	for _, n := range g.Nodes {
		for _, dep := range n.DependsOn {
			if dep == nodeID {
				count++
				break
			}
		}
	}
	return count
}

// AreaOverlap returns the area-overlap density across the given candidate
// set: the fraction of unordered node pairs that share at least one area
// label. The result is in [0.0, 1.0]. A set of fewer than two nodes (or
// IDs not present in the graph) returns 0.
//
// Definition matches specs/coordination.md §"Graph signals" — overlap
// density is the share of candidate pairs that touch the same area, a
// proxy for expected merge-conflict pressure when running the set in
// parallel.
func AreaOverlap(g Graph, nodeIDs []string) float64 {
	// Collect node area-sets in the order given, skipping unknowns.
	areas := make([]map[string]struct{}, 0, len(nodeIDs))
	for _, id := range nodeIDs {
		n, ok := g.Nodes[id]
		if !ok {
			continue
		}
		s := make(map[string]struct{}, len(n.Areas))
		for _, a := range n.Areas {
			s[a] = struct{}{}
		}
		areas = append(areas, s)
	}
	if len(areas) < 2 {
		return 0
	}
	totalPairs := 0
	overlapPairs := 0
	for i := 0; i < len(areas); i++ {
		for j := i + 1; j < len(areas); j++ {
			totalPairs++
			if shareAny(areas[i], areas[j]) {
				overlapPairs++
			}
		}
	}
	if totalPairs == 0 {
		return 0
	}
	return float64(overlapPairs) / float64(totalPairs)
}

func shareAny(a, b map[string]struct{}) bool {
	// Iterate the smaller set for cheap intersection check.
	if len(a) > len(b) {
		a, b = b, a
	}
	for k := range a {
		if _, ok := b[k]; ok {
			return true
		}
	}
	return false
}
