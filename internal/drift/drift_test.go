package drift

import (
	"reflect"
	"testing"

	"github.com/gberns/kerf/internal/beads"
)

// closedSet is the closed-status vocabulary used by drift tests. Mirrors
// what the bead tool exposes; the production caller passes its own set.
var closedSet = map[string]bool{
	"closed":   true,
	"complete": true,
	"done":     true,
}

func TestHashBead_WorkedExample(t *testing.T) {
	// Mirrors the worked example in specs/coordination.md §"Hash scope".
	b := beads.Bead{
		ID:        "kerf-2sl",
		Title:     "Relabel drift hash scope",
		Status:    "Open",
		Labels:    []string{"plan-008", "Phase-1", "plan-008"},
		DependsOn: []string{"kerf-n4h", "kerf-0kx"},
	}
	got := HashBead(b)
	if len(got) != 64 {
		t.Fatalf("hash length = %d, want 64 (hex sha256)", len(got))
	}
	// Determinism: a second call returns the same value.
	if HashBead(b) != got {
		t.Fatal("HashBead is not deterministic across calls")
	}
}

func TestHashBead_LabelOrderIrrelevant(t *testing.T) {
	a := beads.Bead{ID: "x", Status: "open", Title: "t", Labels: []string{"a", "b", "c"}}
	b := beads.Bead{ID: "x", Status: "open", Title: "t", Labels: []string{"c", "a", "b"}}
	if HashBead(a) != HashBead(b) {
		t.Fatal("label reorder changed the hash")
	}
}

func TestHashBead_LabelCaseAndDup(t *testing.T) {
	a := beads.Bead{ID: "x", Status: "open", Title: "t", Labels: []string{"Plan-008", "plan-008"}}
	b := beads.Bead{ID: "x", Status: "open", Title: "t", Labels: []string{"plan-008"}}
	if HashBead(a) != HashBead(b) {
		t.Fatal("label case+dup not normalized")
	}
}

func TestHashBead_DepOrderIrrelevant(t *testing.T) {
	a := beads.Bead{ID: "x", Status: "open", Title: "t", DependsOn: []string{"a", "b", "c"}}
	b := beads.Bead{ID: "x", Status: "open", Title: "t", DependsOn: []string{"c", "b", "a"}}
	if HashBead(a) != HashBead(b) {
		t.Fatal("dep reorder changed the hash")
	}
}

func TestHashBead_StatusCaseFolded(t *testing.T) {
	a := beads.Bead{ID: "x", Status: "OPEN", Title: "t"}
	b := beads.Bead{ID: "x", Status: "open", Title: "t"}
	if HashBead(a) != HashBead(b) {
		t.Fatal("status case folding not applied")
	}
}

func TestHashBead_TitleSensitive(t *testing.T) {
	a := beads.Bead{ID: "x", Status: "open", Title: "alpha"}
	b := beads.Bead{ID: "x", Status: "open", Title: "beta"}
	if HashBead(a) == HashBead(b) {
		t.Fatal("title change did not move the hash")
	}
}

func TestHashBead_IDSensitive(t *testing.T) {
	a := beads.Bead{ID: "x", Status: "open", Title: "t"}
	b := beads.Bead{ID: "y", Status: "open", Title: "t"}
	if HashBead(a) == HashBead(b) {
		t.Fatal("id change did not move the hash")
	}
}

func TestHashBead_EmptyLabelsAndDeps(t *testing.T) {
	// Confirm `labels=` / `deps=` empty-line encoding hashes stably.
	a := beads.Bead{ID: "x", Status: "open", Title: "t"}
	b := beads.Bead{ID: "x", Status: "open", Title: "t", Labels: []string{}, DependsOn: []string{}}
	if HashBead(a) != HashBead(b) {
		t.Fatal("empty vs nil slices produce different hashes")
	}
}

func TestCapture_Empty(t *testing.T) {
	snap := Capture(nil, nil)
	if len(snap.Beads) != 0 {
		t.Fatalf("Beads = %d, want 0", len(snap.Beads))
	}
	if snap.SnapshotID == "" || len(snap.SnapshotID) != 64 {
		t.Fatalf("SnapshotID = %q, want 64-hex string (sha256 of empty)", snap.SnapshotID)
	}
}

func TestCapture_SnapshotIDDeterministicAndOrderIndependent(t *testing.T) {
	bs1 := []beads.Bead{
		{ID: "b", Status: "open", Title: "two"},
		{ID: "a", Status: "open", Title: "one"},
	}
	bs2 := []beads.Bead{
		{ID: "a", Status: "open", Title: "one"},
		{ID: "b", Status: "open", Title: "two"},
	}
	s1 := Capture(bs1, nil)
	s2 := Capture(bs2, nil)
	if s1.SnapshotID != s2.SnapshotID {
		t.Fatal("SnapshotID depends on input order")
	}
}

func TestCompute_EmptyBaselineEverythingNew(t *testing.T) {
	cur := Capture([]beads.Bead{
		{ID: "a", Status: "open", Title: "x"},
		{ID: "b", Status: "open", Title: "y"},
	}, nil)
	d := Compute(Snapshot{}, cur, closedSet)
	if !reflect.DeepEqual(d.New, []string{"a", "b"}) {
		t.Fatalf("New = %v, want [a b]", d.New)
	}
	if len(d.Deleted)+len(d.ClosedExternally)+len(d.ReopenedExternally)+len(d.Changed) != 0 {
		t.Fatalf("other categories non-empty: %+v", d)
	}
}

func TestCompute_IdenticalEmpty(t *testing.T) {
	bs := []beads.Bead{{ID: "a", Status: "open", Title: "x"}}
	base := Capture(bs, nil)
	cur := Capture(bs, nil)
	d := Compute(base, cur, closedSet)
	if !d.Empty() {
		t.Fatalf("expected empty diff, got %+v", d)
	}
}

func TestCompute_TruthTable(t *testing.T) {
	// Five-bead combined run, each illustrating one category:
	//   keep:    unchanged           (no entry in diff)
	//   gone:    deleted             (Deleted)
	//   closed:  external close      (ClosedExternally)
	//   reopen:  external reopen     (ReopenedExternally)
	//   relabel: same id+status,
	//            different hash      (Changed)
	//   new:     not in baseline     (New)
	baseline := Capture([]beads.Bead{
		{ID: "keep", Status: "open", Title: "k"},
		{ID: "gone", Status: "open", Title: "g"},
		{ID: "closed", Status: "open", Title: "c"},
		{ID: "reopen", Status: "closed", Title: "r"},
		{ID: "relabel", Status: "open", Title: "rl", Labels: []string{"old"}},
	}, nil)
	current := Capture([]beads.Bead{
		{ID: "keep", Status: "open", Title: "k"},
		// "gone" missing
		{ID: "closed", Status: "closed", Title: "c"},
		{ID: "reopen", Status: "open", Title: "r"},
		{ID: "relabel", Status: "open", Title: "rl", Labels: []string{"new"}},
		{ID: "new", Status: "open", Title: "n"},
	}, nil)

	d := Compute(baseline, current, closedSet)
	if !reflect.DeepEqual(d.New, []string{"new"}) {
		t.Fatalf("New = %v, want [new]", d.New)
	}
	if !reflect.DeepEqual(d.Deleted, []string{"gone"}) {
		t.Fatalf("Deleted = %v, want [gone]", d.Deleted)
	}
	if !reflect.DeepEqual(d.ClosedExternally, []string{"closed"}) {
		t.Fatalf("ClosedExternally = %v, want [closed]", d.ClosedExternally)
	}
	if !reflect.DeepEqual(d.ReopenedExternally, []string{"reopen"}) {
		t.Fatalf("ReopenedExternally = %v, want [reopen]", d.ReopenedExternally)
	}
	if !reflect.DeepEqual(d.Changed, []string{"relabel"}) {
		t.Fatalf("Changed = %v, want [relabel]", d.Changed)
	}
}

func TestCompute_ChangedCoversRelabelRetitleAndDeps(t *testing.T) {
	cases := []struct {
		name string
		base beads.Bead
		cur  beads.Bead
	}{
		{
			name: "relabel",
			base: beads.Bead{ID: "x", Status: "open", Title: "t", Labels: []string{"a"}},
			cur:  beads.Bead{ID: "x", Status: "open", Title: "t", Labels: []string{"b"}},
		},
		{
			name: "retitle",
			base: beads.Bead{ID: "x", Status: "open", Title: "old"},
			cur:  beads.Bead{ID: "x", Status: "open", Title: "new"},
		},
		{
			name: "depchange",
			base: beads.Bead{ID: "x", Status: "open", Title: "t", DependsOn: []string{"a"}},
			cur:  beads.Bead{ID: "x", Status: "open", Title: "t", DependsOn: []string{"a", "b"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base := Capture([]beads.Bead{tc.base}, nil)
			cur := Capture([]beads.Bead{tc.cur}, nil)
			d := Compute(base, cur, closedSet)
			if !reflect.DeepEqual(d.Changed, []string{"x"}) {
				t.Fatalf("Changed = %v, want [x]; full diff = %+v", d.Changed, d)
			}
		})
	}
}
