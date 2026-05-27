// Package scenario_import converts external bead/work definitions into
// kerfsim scenario documents. Phase 1 supports harmonik-pilot YAML files;
// other source flavors are stubbed for follow-up.
//
// See specs/sim_corpus.md for the source-format contract.
package scenario_import

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/gregberns/kerf/internal/sim/scenario"
)

// harmonikDoc is the subset of a harmonik *-pilot-data.yaml we read.
//
// Fields outside this struct (description, cite tags, forward-deferred
// metadata, etc.) are dropped on import per the Plan 012 / A scope note.
type harmonikDoc struct {
	Epic  harmonikEpic   `yaml:"epic"`
	Beads []harmonikBead `yaml:"beads"`
	Edges []harmonikEdge `yaml:"edges"`
}

type harmonikEpic struct {
	Mnem  string `yaml:"mnem"`
	Title string `yaml:"title"`
}

// harmonikBead carries only the structural fields the simulator needs. The
// `kind` field is taken as the bead's area tag (one of: req, invariant,
// schema, error-taxonomy, test-infra, etc.).
type harmonikBead struct {
	Mnem string `yaml:"mnem"`
	Kind string `yaml:"kind"`
}

// harmonikEdge is one `{from, to}` tuple from the YAML `edges:` block.
//
// discipline §2.5: `from` cites `to` (i.e. `from` depends on `to`). YAML
// edges may target `forward:*` placeholders or sub-bead mnems like
// `cp-schema.attach-point`; we ignore those and key on the leading mnem
// prefix only.
type harmonikEdge struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

// PilotSummary is the per-pilot import report.
type PilotSummary struct {
	Mnem      string   // top-level mnem (e.g. "cp")
	Path      string   // source YAML path
	BeadCount int      // beads after dropping non-{req,...} entries
	Areas     []string // sorted unique kinds observed
	Deps      []string // sorted unique cross-pilot prereq prefixes
	EdgeCount int      // total YAML edges (informational)
}

// ImportResult is the full output of an import call.
type ImportResult struct {
	Scenario *scenario.Scenario
	Pilots   []PilotSummary
	// Notes carries human-readable lines (dropped entries, forward-deferred
	// counts, etc.) that the CLI surfaces after writing the scenario.
	Notes []string
}

// ImportHarmonik reads either a single harmonik pilot YAML file or a
// directory containing one or more `*-pilot-data.yaml` files and produces
// a scenario document. Each pilot YAML becomes exactly one `work` whose
// codename is the pilot's top-level `epic.mnem` and whose `bead_count` is
// the number of structural beads in the YAML.
//
// Cross-pilot deps are discovered by scanning the `edges:` block for `to`
// targets whose mnem prefix differs from the citing pilot's mnem.
func ImportHarmonik(source string) (*ImportResult, error) {
	paths, err := collectPilotPaths(source)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("scenario_import: no harmonik pilot YAMLs found at %s", source)
	}

	docs := make(map[string]*harmonikDoc, len(paths))
	order := make([]string, 0, len(paths))
	for _, p := range paths {
		doc, err := readHarmonikYAML(p)
		if err != nil {
			return nil, err
		}
		mnem := strings.TrimSpace(doc.Epic.Mnem)
		if mnem == "" {
			return nil, fmt.Errorf("scenario_import: %s: epic.mnem is empty", p)
		}
		if _, dup := docs[mnem]; dup {
			return nil, fmt.Errorf("scenario_import: duplicate pilot mnem %q across files", mnem)
		}
		docs[mnem] = doc
		order = append(order, mnem)
	}
	sort.Strings(order)

	// Build a set of known pilot mnems so we can classify edges as intra-
	// pilot vs cross-pilot vs unknown-prefix (forward-deferred etc.).
	known := make(map[string]struct{}, len(docs))
	for m := range docs {
		known[m] = struct{}{}
	}

	works := make([]scenario.Work, 0, len(order))
	summaries := make([]PilotSummary, 0, len(order))
	notes := []string{}

	for _, mnem := range order {
		doc := docs[mnem]
		// Areas: sorted unique kinds, dropping empties.
		areaSet := map[string]struct{}{}
		beadCount := 0
		for _, b := range doc.Beads {
			if strings.TrimSpace(b.Mnem) == "" {
				continue
			}
			beadCount++
			if k := strings.TrimSpace(b.Kind); k != "" {
				areaSet[k] = struct{}{}
			}
		}
		areas := setToSorted(areaSet)
		if len(areas) == 0 {
			areas = []string{mnem}
		}

		// Cross-pilot deps from `edges:`.
		depSet := map[string]struct{}{}
		droppedForward := 0
		droppedUnknown := 0
		for _, e := range doc.Edges {
			target := strings.TrimSpace(e.To)
			if target == "" {
				continue
			}
			if strings.HasPrefix(target, "forward:") {
				droppedForward++
				continue
			}
			prefix := mnemPrefix(target)
			if prefix == "" {
				continue
			}
			if prefix == mnem {
				continue // intra-pilot
			}
			if _, ok := known[prefix]; !ok {
				droppedUnknown++
				continue
			}
			depSet[prefix] = struct{}{}
		}
		deps := setToSorted(depSet)

		// Always emit an explicit deps pointer (possibly to an empty
		// slice). The harmonik importer is authoritative about the
		// dep graph it just computed, so the kerfsim generator must
		// not later inject random older-sibling edges into dep-less
		// works (which would re-introduce cycles after breakCycles
		// runs below).
		depPtr := append([]string(nil), deps...)
		works = append(works, scenario.Work{
			Codename:  mnem,
			Areas:     areas,
			Deps:      &depPtr,
			BeadCount: beadCount,
		})

		summaries = append(summaries, PilotSummary{
			Mnem:      mnem,
			Path:      pathFor(paths, mnem, docs),
			BeadCount: beadCount,
			Areas:     areas,
			Deps:      deps,
			EdgeCount: len(doc.Edges),
		})
		if droppedForward > 0 {
			notes = append(notes, fmt.Sprintf("pilot %s: dropped %d forward-deferred edge(s)", mnem, droppedForward))
		}
		if droppedUnknown > 0 {
			notes = append(notes, fmt.Sprintf("pilot %s: dropped %d edge(s) to unknown pilot prefix", mnem, droppedUnknown))
		}
	}

	// Drop self-deps and deps to non-included works (defensive — already
	// filtered above, but keeps the scenario provably valid).
	codes := map[string]struct{}{}
	for _, w := range works {
		codes[w.Codename] = struct{}{}
	}
	for i := range works {
		cur := works[i].DepsSlice()
		filtered := cur[:0]
		for _, d := range cur {
			if d == works[i].Codename {
				continue
			}
			if _, ok := codes[d]; !ok {
				continue
			}
			filtered = append(filtered, d)
		}
		works[i].Deps = &filtered
	}

	// Harmonik pilots routinely declare bidirectional inter-spec citations
	// at the bead level, which collapse to cyclic work-level deps. The
	// simulator requires a DAG, so we greedily drop edges that close a
	// cycle. Order is deterministic (works are already sorted by codename)
	// so the scenario remains reproducible.
	droppedCycle := breakCycles(works)
	if droppedCycle > 0 {
		notes = append(notes, fmt.Sprintf("dropped %d work-level dep edge(s) to keep the scenario acyclic", droppedCycle))
	}

	sc := defaultScenario(works)
	res := &ImportResult{
		Scenario: sc,
		Pilots:   summaries,
		Notes:    notes,
	}
	return res, nil
}

// collectPilotPaths normalizes `source` (file or directory) into a list of
// candidate pilot YAML paths. Files named `meta-pilot-data.yaml` are
// included; the caller may filter if desired.
func collectPilotPaths(source string) ([]string, error) {
	st, err := os.Stat(source)
	if err != nil {
		return nil, fmt.Errorf("scenario_import: stat %s: %w", source, err)
	}
	if !st.IsDir() {
		return []string{source}, nil
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		return nil, fmt.Errorf("scenario_import: readdir %s: %w", source, err)
	}
	var out []string
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		name := ent.Name()
		if !strings.HasSuffix(name, "-pilot-data.yaml") {
			continue
		}
		out = append(out, filepath.Join(source, name))
	}
	sort.Strings(out)
	return out, nil
}

func readHarmonikYAML(path string) (*harmonikDoc, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("scenario_import: read %s: %w", path, err)
	}
	var doc harmonikDoc
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("scenario_import: parse %s: %w", path, err)
	}
	return &doc, nil
}

// mnemPrefix returns the leading dash-separated component of a mnem string.
// Examples:
//
//	"cp-001"             -> "cp"
//	"cp-schema.kind"     -> "cp"
//	"ev-events.foo"      -> "ev"
//	"hk-ahvq.46"         -> "hk"
//	"forward:wm-040"     -> ""   (filtered upstream)
func mnemPrefix(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Strip any leading `forward:` prefix defensively.
	if i := strings.Index(s, ":"); i >= 0 {
		s = s[i+1:]
	}
	if i := strings.Index(s, "-"); i >= 0 {
		return s[:i]
	}
	if i := strings.Index(s, "."); i >= 0 {
		return s[:i]
	}
	return s
}

func setToSorted(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func pathFor(paths []string, mnem string, docs map[string]*harmonikDoc) string {
	// Best-effort: match the path whose basename starts with `<mnem>-pilot-data`.
	want := mnem + "-pilot-data.yaml"
	for _, p := range paths {
		if filepath.Base(p) == want {
			return p
		}
	}
	if len(paths) == 1 {
		return paths[0]
	}
	return ""
}

// breakCycles greedily removes dep edges from `works` until the deps graph
// is acyclic. Candidate edges are evaluated in (codename, dep)
// lexicographic order; an edge is kept iff adding it does not create a
// cycle in the already-accepted subgraph. Returns the number of edges
// dropped.
func breakCycles(works []scenario.Work) int {
	type edge struct{ from, to string }

	// Snapshot the original edge list, then reset Deps to an explicit
	// empty slice on every work so the kerfsim generator treats this
	// importer as authoritative (no random older-sibling injection).
	var edges []edge
	for i := range works {
		for _, d := range works[i].DepsSlice() {
			edges = append(edges, edge{from: works[i].Codename, to: d})
		}
		empty := []string{}
		works[i].Deps = &empty
	}
	// Edges already arrive in deterministic order because `works` is
	// sorted and each Deps slice was sorted at construction time.

	// adjacency: from -> set(to). We add edges one at a time, dropping
	// any that would close a cycle.
	adj := map[string]map[string]struct{}{}
	addEdge := func(from, to string) bool {
		// Reachable(to -> from) would close a cycle.
		if pathExists(adj, to, from) {
			return false
		}
		if adj[from] == nil {
			adj[from] = map[string]struct{}{}
		}
		adj[from][to] = struct{}{}
		return true
	}

	dropped := 0
	for _, e := range edges {
		if !addEdge(e.from, e.to) {
			dropped++
		}
	}

	// Materialise accepted adjacency back onto works.
	idx := map[string]int{}
	for i, w := range works {
		idx[w.Codename] = i
	}
	for from, tos := range adj {
		i, ok := idx[from]
		if !ok {
			continue
		}
		deps := make([]string, 0, len(tos))
		for to := range tos {
			deps = append(deps, to)
		}
		sort.Strings(deps)
		works[i].Deps = &deps
	}
	return dropped
}

// pathExists reports whether `dst` is reachable from `src` via `adj`.
func pathExists(adj map[string]map[string]struct{}, src, dst string) bool {
	if src == dst {
		return true
	}
	seen := map[string]struct{}{src: {}}
	stack := []string{src}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for next := range adj[n] {
			if next == dst {
				return true
			}
			if _, ok := seen[next]; ok {
				continue
			}
			seen[next] = struct{}{}
			stack = append(stack, next)
		}
	}
	return false
}

// defaultScenario fills in the non-import fields with placeholder values
// the simulator will accept. Callers are expected to overwrite the
// duration model and arrival generator before relying on the run output.
func defaultScenario(works []scenario.Work) *scenario.Scenario {
	median := 25.0
	totalBeads := 0
	for _, w := range works {
		totalBeads += w.BeadCount
	}
	// Scale ticks roughly to workload size, keeping a generous ceiling so
	// the canonical scenarios still terminate cleanly.
	ticks := int64(5000)
	if totalBeads > 200 {
		ticks = int64(totalBeads) * 25
	}
	// `target_works` defaults to the first half of the pilots so the rework
	// generator has somewhere to land. Empty target_works is legal.
	var targets []string
	for i, w := range works {
		if i >= (len(works)+1)/2 {
			break
		}
		targets = append(targets, w.Codename)
	}
	return &scenario.Scenario{
		Seed:   42,
		Ticks:  ticks,
		Agents: 4,
		Works:  works,
		BeadArrivals: scenario.BeadArrivals{
			Generator: &scenario.Generator{
				ReworkRatePerTick: 0.001,
				TargetWorks:       targets,
			},
		},
		AgentModel: scenario.AgentModel{
			Duration: scenario.Duration{
				Kind:        scenario.DurationKindLogNormal,
				MedianTicks: scenario.Float64Ptr(median),
				Sigma:       0.9,
			},
		},
	}
}
