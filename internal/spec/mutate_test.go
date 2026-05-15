package spec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFixture writes content to a fresh tempdir + file and returns the path.
func writeFixture(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(b)
}

// --- AddPinnedBead / RemovePinnedBead ---------------------------------------

func TestAddPinnedBead_CreatesKey(t *testing.T) {
	src := "codename: bridge\ntype: feature\n"
	p := writeFixture(t, "spec.yaml", src)
	if err := AddPinnedBead(p, "hk-cb-001"); err != nil {
		t.Fatalf("AddPinnedBead: %v", err)
	}
	got := readFile(t, p)
	if !strings.Contains(got, "pinned_beads:") || !strings.Contains(got, "hk-cb-001") {
		t.Fatalf("expected pinned_beads with hk-cb-001, got:\n%s", got)
	}
}

func TestAddPinnedBead_Idempotent(t *testing.T) {
	src := "codename: bridge\npinned_beads:\n  - hk-cb-001\n"
	p := writeFixture(t, "spec.yaml", src)
	if err := AddPinnedBead(p, "hk-cb-001"); err != nil {
		t.Fatalf("AddPinnedBead: %v", err)
	}
	got := readFile(t, p)
	// Should still contain exactly one occurrence of hk-cb-001.
	if strings.Count(got, "hk-cb-001") != 1 {
		t.Fatalf("expected idempotent (1 occurrence), got:\n%s", got)
	}
}

func TestAddPinnedBead_PreservesCommentAbovePinnedBeads(t *testing.T) {
	src := `codename: bridge
type: feature
# A heartfelt note about pins.
pinned_beads:
  - hk-cb-001
`
	p := writeFixture(t, "spec.yaml", src)
	if err := AddPinnedBead(p, "hk-cb-002"); err != nil {
		t.Fatalf("AddPinnedBead: %v", err)
	}
	got := readFile(t, p)
	if !strings.Contains(got, "# A heartfelt note about pins.") {
		t.Fatalf("comment lost:\n%s", got)
	}
	if !strings.Contains(got, "hk-cb-002") {
		t.Fatalf("new pin missing:\n%s", got)
	}
}

func TestRemovePinnedBead_Removes(t *testing.T) {
	src := "codename: bridge\npinned_beads:\n  - hk-cb-001\n  - hk-cb-002\n"
	p := writeFixture(t, "spec.yaml", src)
	if err := RemovePinnedBead(p, "hk-cb-001"); err != nil {
		t.Fatalf("RemovePinnedBead: %v", err)
	}
	got := readFile(t, p)
	if strings.Contains(got, "hk-cb-001") {
		t.Fatalf("expected hk-cb-001 removed:\n%s", got)
	}
	if !strings.Contains(got, "hk-cb-002") {
		t.Fatalf("expected hk-cb-002 retained:\n%s", got)
	}
}

func TestRemovePinnedBead_AbsentNoOp(t *testing.T) {
	src := "codename: bridge\n"
	p := writeFixture(t, "spec.yaml", src)
	before := readFile(t, p)
	if err := RemovePinnedBead(p, "hk-cb-001"); err != nil {
		t.Fatalf("RemovePinnedBead: %v", err)
	}
	if readFile(t, p) != before {
		t.Fatalf("expected unchanged content")
	}
}

func TestRemovePinnedBead_EmptyListRendersFlow(t *testing.T) {
	src := "codename: bridge\npinned_beads:\n  - hk-cb-001\n"
	p := writeFixture(t, "spec.yaml", src)
	if err := RemovePinnedBead(p, "hk-cb-001"); err != nil {
		t.Fatalf("RemovePinnedBead: %v", err)
	}
	got := readFile(t, p)
	if !strings.Contains(got, "pinned_beads: []") {
		t.Fatalf("expected flow-style empty list, got:\n%s", got)
	}
}

func TestPin_RoundTripIdempotent_RemoveThenAdd(t *testing.T) {
	src := `codename: bridge
# user-added comment
pinned_beads:
  - hk-cb-001
type: feature
`
	p := writeFixture(t, "spec.yaml", src)
	if err := RemovePinnedBead(p, "hk-cb-001"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if err := AddPinnedBead(p, "hk-cb-001"); err != nil {
		t.Fatalf("add: %v", err)
	}
	got := readFile(t, p)
	if !strings.Contains(got, "# user-added comment") {
		t.Fatalf("comment lost across remove+add round-trip:\n%s", got)
	}
	if !strings.Contains(got, "hk-cb-001") {
		t.Fatalf("bead lost across round-trip:\n%s", got)
	}
}

// --- AddBeadFilterClause / RemoveBeadFilterClause ---------------------------

func TestAddBeadFilterClause_CreatesKeyAsDirect(t *testing.T) {
	src := "codename: bridge\n"
	p := writeFixture(t, "spec.yaml", src)
	if err := AddBeadFilterClause(p, "label=subsystem:bridge"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	got := readFile(t, p)
	if !strings.Contains(got, "bead_filter:") || !strings.Contains(got, "label: subsystem:bridge") {
		t.Fatalf("expected direct clause, got:\n%s", got)
	}
	if strings.Contains(got, "any:") {
		t.Fatalf("expected direct form (no any:), got:\n%s", got)
	}
}

func TestAddBeadFilterClause_LiftsDirectToAny(t *testing.T) {
	src := "codename: bridge\nbead_filter:\n  label: \"x:y\"\n"
	p := writeFixture(t, "spec.yaml", src)
	if err := AddBeadFilterClause(p, "label=z:w"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	got := readFile(t, p)
	if !strings.Contains(got, "any:") {
		t.Fatalf("expected any: lift, got:\n%s", got)
	}
	if !strings.Contains(got, "x:y") || !strings.Contains(got, "z:w") {
		t.Fatalf("expected both clauses present:\n%s", got)
	}
}

func TestAddBeadFilterClause_AppendsToAny(t *testing.T) {
	src := `codename: bridge
bead_filter:
  any:
    - label: "a:b"
    - label: "c:d"
`
	p := writeFixture(t, "spec.yaml", src)
	if err := AddBeadFilterClause(p, "id_prefix=hk-"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	got := readFile(t, p)
	if !strings.Contains(got, "id_prefix: hk-") {
		t.Fatalf("expected new clause appended:\n%s", got)
	}
	if strings.Count(got, "label:") != 2 {
		t.Fatalf("expected 2 label clauses, got:\n%s", got)
	}
}

func TestAddBeadFilterClause_Idempotent(t *testing.T) {
	src := "codename: bridge\nbead_filter:\n  label: \"x:y\"\n"
	p := writeFixture(t, "spec.yaml", src)
	if err := AddBeadFilterClause(p, "label=x:y"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	got := readFile(t, p)
	if strings.Contains(got, "any:") {
		t.Fatalf("idempotent add should not lift to any:, got:\n%s", got)
	}
	if strings.Count(got, "label:") != 1 {
		t.Fatalf("expected exactly one label clause:\n%s", got)
	}
}

func TestRemoveBeadFilterClause_CollapsesTwoToOne(t *testing.T) {
	src := `codename: bridge
bead_filter:
  any:
    - label: "a:b"
    - label: "c:d"
`
	p := writeFixture(t, "spec.yaml", src)
	if err := RemoveBeadFilterClause(p, "label=a:b"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	got := readFile(t, p)
	if strings.Contains(got, "any:") {
		t.Fatalf("expected collapse to direct form, got:\n%s", got)
	}
	if !strings.Contains(got, "label: c:d") {
		t.Fatalf("expected remaining clause as direct, got:\n%s", got)
	}
}

func TestRemoveBeadFilterClause_RemovesKeyWhenLast(t *testing.T) {
	src := "codename: bridge\nbead_filter:\n  label: \"x:y\"\ntype: feature\n"
	p := writeFixture(t, "spec.yaml", src)
	if err := RemoveBeadFilterClause(p, "label=x:y"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	got := readFile(t, p)
	if strings.Contains(got, "bead_filter") {
		t.Fatalf("expected bead_filter key removed entirely, got:\n%s", got)
	}
	if !strings.Contains(got, "type: feature") {
		t.Fatalf("expected surrounding keys retained:\n%s", got)
	}
}

func TestRemoveBeadFilterClause_AbsentNoOp(t *testing.T) {
	src := "codename: bridge\n"
	p := writeFixture(t, "spec.yaml", src)
	before := readFile(t, p)
	if err := RemoveBeadFilterClause(p, "label=x:y"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if readFile(t, p) != before {
		t.Fatalf("expected unchanged content")
	}
}

func TestRemoveBeadFilterClause_NoMatchInAnyNoOp(t *testing.T) {
	src := `codename: bridge
bead_filter:
  any:
    - label: "a:b"
    - label: "c:d"
`
	p := writeFixture(t, "spec.yaml", src)
	before := readFile(t, p)
	if err := RemoveBeadFilterClause(p, "label=does:notexist"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if readFile(t, p) != before {
		t.Fatalf("expected no-op when clause not present")
	}
}

// --- Comment-preservation: round-trip on a richer fixture -------------------

func TestRoundTrip_PreservesHeadAndInlineComments(t *testing.T) {
	src := `# Top-of-file note.
codename: bridge          # inline comment on codename
type: feature

# Comment above pinned_beads.
pinned_beads:
  - hk-cb-001             # inline on a bead

bead_filter:
  label: "subsystem:bridge"  # inline on filter
`
	p := writeFixture(t, "spec.yaml", src)
	// Idempotent op: remove a nonexistent pin, then add an existing one.
	if err := RemovePinnedBead(p, "nope"); err != nil {
		t.Fatalf("remove nope: %v", err)
	}
	if err := AddPinnedBead(p, "hk-cb-001"); err != nil {
		t.Fatalf("add existing: %v", err)
	}
	got := readFile(t, p)
	for _, want := range []string{
		"# Top-of-file note.",
		"# inline comment on codename",
		"# Comment above pinned_beads.",
		"# inline on a bead",
		"# inline on filter",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("comment lost: %q\nfile:\n%s", want, got)
		}
	}
}

func TestAddBeadFilterClause_RejectsBadClause(t *testing.T) {
	src := "codename: bridge\n"
	p := writeFixture(t, "spec.yaml", src)
	if err := AddBeadFilterClause(p, "all=foo"); err == nil {
		t.Fatal("expected parse error for unknown key")
	}
}
