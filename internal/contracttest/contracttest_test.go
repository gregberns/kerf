package contracttest

import (
	"sort"
	"testing"

	"github.com/spf13/cobra"
)

// TestEveryCommandHasContractCoverage is the harness's own sanity test:
// it verifies the walker finds the real kerf command tree, returns
// deterministic output, and only enumerates non-hidden leaves. The five
// real contracts are landed by downstream beads (plan 023 / B2..B6); this
// test does NOT assert any of those contracts, only that the scaffolding
// they will register against works.
func TestEveryCommandHasContractCoverage(t *testing.T) {
	leaves := Walk(t)
	if len(leaves) == 0 {
		t.Fatal("Walk returned zero leaves; cmd.Root() is empty or the walker is broken")
	}

	// Sanity lower bound: kerf has well over ten leaf commands today
	// (init, new, list, show, status, resume, shelve, finalize, square,
	// next, triage, pin, map, snapshot, history, restore, archive,
	// delete, config, setup, localize, doctor, bootstrap-filters,
	// review, preview, jig.list, jig.show, jig.save, jig.load, jig.sync,
	// areas.init, areas.list, areas.add, areas.remove, work.edit,
	// work.show ...). If this count plummets, something is wrong.
	const minLeaves = 10
	if len(leaves) < minLeaves {
		t.Errorf("Walk returned %d leaves; expected at least %d. leaves=%v",
			len(leaves), minLeaves, leafPaths(leaves))
	}

	// Determinism: a second walk must produce identical output.
	again := Walk(t)
	if len(leaves) != len(again) {
		t.Fatalf("Walk is non-deterministic: first run %d, second run %d", len(leaves), len(again))
	}
	for i := range leaves {
		if leaves[i].Path != again[i].Path {
			t.Errorf("Walk is non-deterministic at index %d: %q vs %q",
				i, leaves[i].Path, again[i].Path)
		}
	}

	// Sorted by Path.
	if !sort.SliceIsSorted(leaves, func(i, j int) bool { return leaves[i].Path < leaves[j].Path }) {
		t.Error("Walk output is not sorted by Path")
	}

	// Every entry must be Runnable and have a non-empty path.
	for _, l := range leaves {
		if l.Path == "" {
			t.Errorf("leaf with empty path: %+v", l)
		}
		if l.Cmd == nil {
			t.Errorf("leaf %q has nil Cmd", l.Path)
			continue
		}
		if !l.Cmd.Runnable() {
			t.Errorf("leaf %q is not Runnable", l.Path)
		}
		if l.Cmd.Hidden {
			t.Errorf("leaf %q is Hidden but was returned by Walk", l.Path)
		}
	}
}

// TestOptOutRegistry verifies the opt-out registry filters as expected.
// It builds a synthetic cobra tree, registers one leaf as exempt from a
// fake contract, and confirms IsExempt distinguishes registered from
// unregistered (path, contract) pairs.
func TestOptOutRegistry(t *testing.T) {
	const (
		contractA = "fake-contract-a"
		contractB = "fake-contract-b"
	)

	// Save and restore the package-level optOuts map.
	saved := optOuts
	defer func() { optOuts = saved }()

	optOuts = map[string]string{
		exemptKey("kerf.foo", contractA): "synthetic; tracked by kerf-test-0001",
	}

	if !IsExempt("kerf.foo", contractA) {
		t.Error("expected kerf.foo to be exempt from fake-contract-a")
	}
	if IsExempt("kerf.foo", contractB) {
		t.Error("kerf.foo should NOT be exempt from fake-contract-b (different contract)")
	}
	if IsExempt("kerf.bar", contractA) {
		t.Error("kerf.bar should NOT be exempt from fake-contract-a (different command)")
	}
	if got := ExemptionReason("kerf.foo", contractA); got == "" {
		t.Error("expected a non-empty rationale for registered opt-out")
	}
	if got := ExemptionReason("kerf.bar", contractA); got != "" {
		t.Errorf("expected empty rationale for unregistered opt-out, got %q", got)
	}
}

// TestWalkerFiltersHiddenAndOptOuts verifies the walker on a synthetic
// tree: hidden leaves are excluded, opt-outs (applied by the caller via
// IsExempt) filter the slice down.
func TestWalkerFiltersHiddenAndOptOuts(t *testing.T) {
	root := &cobra.Command{Use: "synth"}
	leafA := &cobra.Command{Use: "alpha", Run: func(*cobra.Command, []string) {}}
	leafB := &cobra.Command{Use: "bravo <arg>", Run: func(*cobra.Command, []string) {}}
	leafHidden := &cobra.Command{Use: "ghost", Hidden: true, Run: func(*cobra.Command, []string) {}}
	parent := &cobra.Command{Use: "group"}
	leafC := &cobra.Command{Use: "charlie", Run: func(*cobra.Command, []string) {}}
	parent.AddCommand(leafC)
	root.AddCommand(leafA, leafB, leafHidden, parent)

	got := WalkRoot(root)
	gotPaths := leafPaths(got)
	wantPaths := []string{"synth.alpha", "synth.bravo", "synth.group.charlie"}
	if !equalStrings(gotPaths, wantPaths) {
		t.Errorf("WalkRoot leaves = %v; want %v", gotPaths, wantPaths)
	}

	// Apply a synthetic opt-out and confirm callers can filter.
	saved := optOuts
	defer func() { optOuts = saved }()
	optOuts = map[string]string{
		exemptKey("synth.alpha", "fake"): "synthetic; tracked by kerf-test-0002",
	}

	var kept []string
	for _, l := range got {
		if IsExempt(l.Path, "fake") {
			continue
		}
		kept = append(kept, l.Path)
	}
	want := []string{"synth.bravo", "synth.group.charlie"}
	if !equalStrings(kept, want) {
		t.Errorf("after opt-out filtering: %v; want %v", kept, want)
	}
}

func leafPaths(leaves []CommandDef) []string {
	out := make([]string, len(leaves))
	for i, l := range leaves {
		out[i] = l.Path
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
