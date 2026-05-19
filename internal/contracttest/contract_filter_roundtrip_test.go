// Contract: filter-clause round-trip between labelsample and ParseFilterClause.
//
// Plan 023 / B3 (kerf-hl13). Locks MAJOR #8 (the `any:` clause asymmetry
// closed by kerf-2km) shut: every filter shape that labelsample.ProposeFilter
// can emit must be accepted by internal/beads.ParseFilterClause, so an agent
// can copy a bootstrap-filters proposal text and feed it back through
// `kerf work edit --bead-filter-add` without translation.
//
// Direction. This contract is one-way only: producer → parser. The reverse
// direction (parser may accept inputs the producer would never emit, e.g.
// "id_prefix=hk-") is intentionally not asserted. See plan 023 OQ4: the
// parser is the looser surface by design — it must accept anything an agent
// or human types as well as anything the sampler emits. Asserting symmetric
// containment would over-constrain the parser.
//
// Generator. testing/quick (Go stdlib) drives the corpora. We do not use
// gopter (OQ3) because the implementation here lands well under the
// ~50-line threshold the plan named — quick.Check + a custom Generator on
// our corpus type is enough and avoids a new dependency.
package contracttest

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"testing/quick"

	"github.com/gberns/kerf/internal/beads"
	"github.com/gberns/kerf/internal/labelsample"
)

// TestContract_FilterClauseRoundTrip asserts that for every (bead corpus,
// codename) pair the random generator produces, any non-nil clause emitted
// by labelsample.ProposeFilter is parsed back to a structurally equivalent
// filter by beads.ParseFilterClause.
//
// Acceptance budget: the bead requires 1000+ generated clauses pass through.
// quick.Config.MaxCount of 2000 with codenames per corpus typically produces
// several thousand proposal evaluations; we count non-nil proposals
// separately and t.Fatal if we somehow under-shoot the budget so the
// contract cannot silently degrade into a no-op.
func TestContract_FilterClauseRoundTrip(t *testing.T) {
	var nonNilProposals int

	property := func(c roundTripCorpus) bool {
		for _, cn := range c.Codenames {
			prop := labelsample.ProposeFilter(c.Beads, cn)
			if prop.Filter == nil {
				continue
			}
			nonNilProposals++

			clause := stringifyFilter(*prop.Filter)
			if clause == "" {
				t.Errorf("stringifyFilter returned empty for proposal %+v (codename=%q)", *prop.Filter, cn)
				return false
			}

			parsed, err := beads.ParseFilterClause(clause)
			if err != nil {
				t.Errorf("ParseFilterClause rejected sampler output %q (codename=%q): %v", clause, cn, err)
				return false
			}

			if !filtersEquivalent(prop.Filter, parsed) {
				t.Errorf("round-trip drift: codename=%q clause=%q\n  produced: %+v\n  parsed:   %+v",
					cn, clause, *prop.Filter, *parsed)
				return false
			}
		}
		return true
	}

	cfg := &quick.Config{
		MaxCount: 2000,
		Rand:     rand.New(rand.NewSource(1)),
	}
	if err := quick.Check(property, cfg); err != nil {
		t.Fatalf("filter round-trip property failed: %v", err)
	}

	const minProposals = 1000
	if nonNilProposals < minProposals {
		t.Fatalf("contract budget under-shot: only %d non-nil proposals exercised (want >= %d); "+
			"strengthen the generator before trusting this contract", nonNilProposals, minProposals)
	}
}

// roundTripCorpus is one quick.Check input: a synthetic bead slice plus the
// codenames to query against it. Implements quick.Generator so we control
// the label distribution — the default reflective generator would produce
// random strings that almost never match the sampler's candidate shapes,
// leaving the property vacuously true.
type roundTripCorpus struct {
	Beads     []beads.Bead
	Codenames []string
}

// Generate builds a corpus biased toward producing non-nil proposals:
//   - A pool of 3-6 codename slugs is chosen.
//   - Each bead carries 1-3 labels drawn from a mix of prefixed
//     ({codename,subsystem,area,kind}:<slug>) and bare-slug shapes for those
//     codenames, plus a small amount of unrelated noise.
//   - The codenames-to-query list always includes the pooled slugs (so the
//     sampler has something to find) plus an unrelated slug (so we exercise
//     the no-match / below-floor branches too — those return nil filters,
//     which the test skips, but verifying the skip path is hit costs us
//     nothing and guards against regressions where every proposal becomes
//     non-nil).
func (roundTripCorpus) Generate(r *rand.Rand, _ int) reflect.Value {
	slugPool := []string{"bridge", "gama", "harmonik", "kerf", "queue", "triage", "labelsample"}
	r.Shuffle(len(slugPool), func(i, j int) { slugPool[i], slugPool[j] = slugPool[j], slugPool[i] })
	nSlugs := 3 + r.Intn(4) // 3..6
	if nSlugs > len(slugPool) {
		nSlugs = len(slugPool)
	}
	slugs := slugPool[:nSlugs]

	prefixes := []string{"codename", "subsystem", "area", "kind"}
	noiseLabels := []string{"P0", "P1", "kind:work", "kind:bug", "spec:testing"}

	nBeads := 5 + r.Intn(20) // 5..24 beads per corpus
	bs := make([]beads.Bead, nBeads)
	for i := range bs {
		nLabels := 1 + r.Intn(3) // 1..3 labels per bead
		labels := make([]string, 0, nLabels)
		for k := 0; k < nLabels; k++ {
			// 80% chance: a candidate-shape label for some slug; 20%: noise.
			if r.Intn(5) != 0 {
				slug := slugs[r.Intn(len(slugs))]
				// Half the time prefixed, half bare-slug.
				if r.Intn(2) == 0 {
					labels = append(labels, prefixes[r.Intn(len(prefixes))]+":"+slug)
				} else {
					labels = append(labels, slug)
				}
			} else {
				labels = append(labels, noiseLabels[r.Intn(len(noiseLabels))])
			}
		}
		bs[i] = beads.Bead{
			ID:     fmt.Sprintf("kerf-%04x", r.Uint32()&0xFFFF),
			Title:  "synthetic bead",
			Status: "open",
			Labels: labels,
		}
	}

	cns := make([]string, 0, len(slugs)+1)
	cns = append(cns, slugs...)
	cns = append(cns, "unmatched-slug-noise") // forces a no-match / below-floor sweep

	return reflect.ValueOf(roundTripCorpus{Beads: bs, Codenames: cns})
}

// stringifyFilter renders a labelsample-emitted beads.Filter back to the
// canonical CLI clause form accepted by beads.ParseFilterClause.
//
// Leaf shapes mirror cmd/next.go formatFilterClause: "label=<v>" /
// "id_prefix=<v>" (the sampler only emits Label leaves today, but IDPrefix
// support is here defensively in case a future sampler variant emits one).
//
// `any:` unions are joined as "any:<leaf>,<leaf>..." — the form
// ParseFilterClause's CutPrefix("any:") branch accepts (kerf-2km).
// bootstrap_filters takes a different path for its mutator, calling
// AddBeadFilterClause once per leaf and letting the mutator lift them into
// the union shape; this contract pins the single-string union form, which
// is what an agent would type after reading a dry-run preview.
func stringifyFilter(f beads.Filter) string {
	if len(f.Any) > 0 {
		parts := make([]string, 0, len(f.Any))
		for _, m := range f.Any {
			leaf := stringifyLeaf(m)
			if leaf == "" {
				return ""
			}
			parts = append(parts, leaf)
		}
		return "any:" + strings.Join(parts, ",")
	}
	return stringifyLeaf(f)
}

func stringifyLeaf(f beads.Filter) string {
	switch {
	case f.Label != "":
		return "label=" + f.Label
	case f.IDPrefix != "":
		return "id_prefix=" + f.IDPrefix
	}
	return ""
}

// filtersEquivalent reports structural equivalence between a produced and a
// parsed Filter. Leaf-level: Label and IDPrefix match. Union-level: same
// number of members in the same order, each pairwise-equivalent. (Order
// matters because both the sampler and the parser preserve it.)
//
// We do NOT use reflect.DeepEqual directly because the sampler may leave
// Any nil while a unioned proposal supplies a non-nil slice; this helper is
// explicit about what we care about.
func filtersEquivalent(a, b *beads.Filter) bool {
	if a == nil || b == nil {
		return a == b
	}
	if len(a.Any) != len(b.Any) {
		return false
	}
	if len(a.Any) > 0 {
		for i := range a.Any {
			if !filtersEquivalent(&a.Any[i], &b.Any[i]) {
				return false
			}
		}
		return true
	}
	return a.Label == b.Label && a.IDPrefix == b.IDPrefix
}

