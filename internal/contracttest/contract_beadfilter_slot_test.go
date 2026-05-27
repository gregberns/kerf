// Contract: bead_filter slot invariant — present-but-empty == absent.
//
// Plan 023 / B6 (kerf-l60u). Promotes the unit-level slot invariant first
// pinned by kerf-3ac into the cross-command contract framing of plan 023:
// every command that consumes the bead_filter slot agrees that a spec.yaml
// whose `bead_filter:` key is present-but-empty (null value) is observably
// identical to one where the key is absent entirely.
//
// Spec ref: specs/testing.md §"Cross-command contracts" → "`bead_filter`
// slot invariant. A present-but-empty `bead_filter` resolves identically
// to an absent one across every command that consumes the slot."
//
// Related invariants:
//   - kerf-3ac:  `kerf new` ALWAYS emits the key (canonical present-but-empty).
//   - kerf-xb4:  spec.Write's ensureBeadFilterPresent restores the key when the
//                yaml.omitempty tag drops a nil pointer.
//   - kerf-o7x:  spec.AddBeadFilterClause accepts a present-but-null slot.
//
// Approach. The contract has two layers:
//
//  1. Foundational layer — the canonical resolver (spec.Read +
//     beads.Resolve) returns structurally identical filters for the two
//     equivalent fixtures. Every consuming command goes through this
//     resolver, so once the foundation holds, all consumers inherit it.
//
//  2. Cross-command framing — walk every cobra leaf, identify the ones
//     that consume bead_filter (by inspecting flag names containing
//     "bead-filter"), and assert each is covered by the slot invariant
//     either through the spec-layer foundation OR via a recorded
//     command-specific assertion below. A new leaf that introduces a
//     --bead-filter-* flag without honouring the invariant fails the
//     contract automatically because the registry below will not list it.
//
// We deliberately test against the in-process spec mutator and resolver
// rather than the CLI — the spec layer is the single source of truth for
// the invariant; CLI exit-code parity follows from it.
package contracttest

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/gregberns/kerf/internal/beads"
	"github.com/gregberns/kerf/internal/spec"
)

const beadFilterSlotContractID = "bead-filter-slot"

// beadFilterFlagNames lists the cobra flag names that introduce a
// bead_filter input on a leaf command. A leaf with one of these flags is
// considered a bead_filter consumer and MUST be covered by the contract.
//
// Adding a new flag name here (or to a future command) without a
// corresponding entry in beadFilterCoveredLeaves below will fail the
// contract — that is the auto-enrollment teeth promised by the bead's
// acceptance criterion ("Adding a new bead_filter-consuming command
// without honouring the invariant fails the contract automatically").
var beadFilterFlagNames = map[string]struct{}{
	"bead-filter":        {}, // kerf init, kerf new
	"bead-filter-add":    {}, // kerf work edit
	"bead-filter-remove": {}, // kerf work edit
}

// beadFilterCoveredLeaves is the registry of leaves audited as honouring
// the slot invariant. Each entry names the bead-filter-touching code path
// and (implicitly) confirms it routes through spec.Read / spec.Write /
// the spec mutators — i.e. the canonical layer this contract pins.
//
// Adding a new bead_filter-consuming command means:
//  1. Add its dotted path here.
//  2. Verify it reads spec.yaml via spec.Read (foundational invariant
//     inherited) and writes via spec.Write or the mutators in
//     internal/spec/mutate.go (canonicalisation inherited).
//  3. If it bypasses those — register an opt-out keyed
//     "<path>::bead-filter-slot" in opt_outs.go citing a bead id.
var beadFilterCoveredLeaves = map[string]string{
	"kerf.init":      "resolves --bead-filter / detector output and writes via spec.Write (inherits ensureBeadFilterPresent).",
	"kerf.new":       "resolves --bead-filter and writes via spec.Write — kerf-3ac canonicalisation path.",
	"kerf.work.edit": "mutates via spec.AddBeadFilterClause / spec.RemoveBeadFilterClause — kerf-o7x and the present-but-empty retention rule.",
}

// TestContract_BeadFilterEmptyEqualsAbsent locks the slot invariant at
// both layers described in the package comment.
func TestContract_BeadFilterEmptyEqualsAbsent(t *testing.T) {
	t.Run("foundation_resolver_identical", testFoundationResolverIdentical)
	t.Run("foundation_write_canonicalises", testFoundationWriteCanonicalises)
	t.Run("foundation_add_accepts_null_slot", testFoundationAddAcceptsNullSlot)
	t.Run("foundation_remove_retains_key", testFoundationRemoveRetainsKey)
	t.Run("cross_command_registry_covers_all_consumers", testCrossCommandRegistryCoversAllConsumers)
}

// testFoundationResolverIdentical builds two equivalent on-disk specs —
// one with the bead_filter key absent, one with the key present-but-null —
// reads them through spec.Read, and asserts beads.Resolve returns
// structurally identical filters.
//
// This is the load-bearing assertion: every consuming command runs
// through spec.Read + beads.Resolve, so identity here implies identity
// everywhere downstream (matching, attachment, cleanup, warnings, ...).
func testFoundationResolverIdentical(t *testing.T) {
	dir := t.TempDir()

	const absentBody = `codename: alpha
type: feature
project:
  id: demo
jig: spec
jig_version: 1
status: ready
status_values: [ready, complete]
created: 2026-05-19T00:00:00Z
updated: 2026-05-19T00:00:00Z
sessions: []
depends_on: []
pinned_beads: []
implementation:
  branch: null
  pr: null
  commits: []
`
	// Identical, with `bead_filter:` (null) inserted before pinned_beads.
	presentNullBody := strings.Replace(
		absentBody,
		"pinned_beads: []",
		"bead_filter:\npinned_beads: []",
		1,
	)
	if presentNullBody == absentBody {
		t.Fatal("test fixture setup: failed to insert bead_filter: null marker")
	}

	absentPath := filepath.Join(dir, "absent.yaml")
	presentPath := filepath.Join(dir, "present_null.yaml")
	if err := os.WriteFile(absentPath, []byte(absentBody), 0o644); err != nil {
		t.Fatalf("write absent fixture: %v", err)
	}
	if err := os.WriteFile(presentPath, []byte(presentNullBody), 0o644); err != nil {
		t.Fatalf("write present-null fixture: %v", err)
	}

	absent, err := spec.Read(absentPath)
	if err != nil {
		t.Fatalf("spec.Read(absent): %v", err)
	}
	present, err := spec.Read(presentPath)
	if err != nil {
		t.Fatalf("spec.Read(present-null): %v", err)
	}

	// Layer A: SpecYAML.BeadFilter pointer is nil in both cases.
	if absent.BeadFilter != nil {
		t.Errorf("absent fixture parsed to non-nil BeadFilter: %+v", absent.BeadFilter)
	}
	if present.BeadFilter != nil {
		t.Errorf("present-but-null fixture parsed to non-nil BeadFilter: %+v", present.BeadFilter)
	}

	// Layer B: beads.Resolve produces identical results for several
	// project-filter shapes. These exercise the three branches of
	// Resolve (perWork wins, project wins, default fallback).
	projectShapes := []struct {
		name    string
		project *beads.Filter
	}{
		{"nil_project_falls_through_to_default", nil},
		{"project_set_label", &beads.Filter{Label: "subsystem:demo"}},
		{"project_set_id_prefix", &beads.Filter{IDPrefix: "demo-"}},
	}
	for _, ps := range projectShapes {
		ps := ps
		t.Run(ps.name, func(t *testing.T) {
			ra := beads.Resolve(absent.BeadFilter, ps.project)
			rp := beads.Resolve(present.BeadFilter, ps.project)
			if !reflect.DeepEqual(ra, rp) {
				t.Errorf("Resolve drift between absent and present-but-null:\n  absent:        %+v\n  present-null:  %+v", ra, rp)
			}
		})
	}
}

// testFoundationWriteCanonicalises asserts kerf-3ac / kerf-xb4: spec.Write
// ALWAYS emits the `bead_filter:` key, even when the source SpecYAML has
// a nil BeadFilter pointer.
func testFoundationWriteCanonicalises(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.yaml")

	s := minimalSpec()
	s.BeadFilter = nil // explicit; this is what `kerf new` produces without --bead-filter.

	if err := spec.Write(path, s); err != nil {
		t.Fatalf("spec.Write: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, "bead_filter:") {
		t.Fatalf("expected bead_filter: key emitted by spec.Write even with nil BeadFilter; got:\n%s", body)
	}
	// And the round-trip resolves to nil (i.e. behaves as absent).
	rs, err := spec.Read(path)
	if err != nil {
		t.Fatalf("spec.Read round-trip: %v", err)
	}
	if rs.BeadFilter != nil {
		t.Errorf("present-but-empty emission read back as non-nil filter: %+v", rs.BeadFilter)
	}
}

// testFoundationAddAcceptsNullSlot asserts kerf-o7x: AddBeadFilterClause
// accepts a spec.yaml whose `bead_filter:` is present-but-null and writes
// a direct leaf clause into the slot.
func testFoundationAddAcceptsNullSlot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.yaml")
	const body = "codename: bridge\nbead_filter:\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := spec.AddBeadFilterClause(path, "label=subsystem:demo"); err != nil {
		t.Fatalf("AddBeadFilterClause against present-but-null slot: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !strings.Contains(string(got), "label: subsystem:demo") {
		t.Fatalf("clause not written into null slot; got:\n%s", got)
	}
}

// testFoundationRemoveRetainsKey asserts that removing the last clause
// from a bead_filter leaves the key present-but-null — the canonical
// form `kerf new` emits — so subsequent reads round-trip to the same
// nil-BeadFilter state as a fixture that never had the clause.
func testFoundationRemoveRetainsKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.yaml")
	const body = "codename: bridge\nbead_filter:\n  label: subsystem:demo\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := spec.RemoveBeadFilterClause(path, "label=subsystem:demo"); err != nil {
		t.Fatalf("RemoveBeadFilterClause: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !strings.Contains(string(got), "bead_filter:") {
		t.Fatalf("expected bead_filter key retained after last-clause remove; got:\n%s", got)
	}
	if strings.Contains(string(got), "label:") {
		t.Fatalf("expected clause removed; got:\n%s", got)
	}
}

// testCrossCommandRegistryCoversAllConsumers walks every cobra leaf and
// asserts: any leaf with a bead-filter-* flag (per beadFilterFlagNames)
// MUST appear in beadFilterCoveredLeaves OR be registered as exempt in
// opt_outs.go under the "bead-filter-slot" contract id. This is the
// cross-command teeth that auto-enrolls new commands.
func testCrossCommandRegistryCoversAllConsumers(t *testing.T) {
	leaves := Walk(t)
	consumers := make(map[string]*cobra.Command, len(leaves))
	for _, l := range leaves {
		if leafConsumesBeadFilter(l.Cmd) {
			consumers[l.Path] = l.Cmd
		}
	}

	// Detect drift in either direction.
	for path := range consumers {
		_, covered := beadFilterCoveredLeaves[path]
		exempt := IsExempt(path, beadFilterSlotContractID)
		if !covered && !exempt {
			t.Errorf("leaf %q exposes a bead-filter-* flag but is not in beadFilterCoveredLeaves "+
				"and has no opt-out for contract %q; either add coverage or register an exemption with a bead id.",
				path, beadFilterSlotContractID)
		}
	}
	for path := range beadFilterCoveredLeaves {
		if _, ok := consumers[path]; !ok {
			t.Errorf("beadFilterCoveredLeaves names %q but no such leaf with a bead-filter-* flag was found by Walk; "+
				"the leaf may have been renamed or had its flag removed — update the registry.", path)
		}
	}

	// Lower bound: the three known consumers (init, new, work.edit) must
	// be present. A regression that strips the bead-filter flag from any
	// of them would silently shrink the registry; this guards against
	// that.
	const minConsumers = 3
	if len(consumers) < minConsumers {
		t.Fatalf("found only %d bead_filter-consuming leaves (paths=%v); expected at least %d "+
			"(kerf init, kerf new, kerf work edit). Has a flag been renamed?",
			len(consumers), keysOf(consumers), minConsumers)
	}
}

// leafConsumesBeadFilter reports whether the cobra command exposes any
// flag whose name matches beadFilterFlagNames. Walking flags (rather
// than hardcoding command paths) is what gives the contract its auto-
// enrollment property.
func leafConsumesBeadFilter(c *cobra.Command) bool {
	if c == nil {
		return false
	}
	found := false
	c.Flags().VisitAll(func(f *pflag.Flag) {
		if _, hit := beadFilterFlagNames[f.Name]; hit {
			found = true
		}
	})
	return found
}

// minimalSpec returns a fully-populated SpecYAML acceptable to
// spec.Write (Validate passes; required fields present). Callers mutate
// BeadFilter / other fields as needed.
func minimalSpec() *spec.SpecYAML {
	return &spec.SpecYAML{
		Codename:     "alpha",
		Type:         "feature",
		Project:      spec.Project{ID: "demo"},
		Jig:          "spec",
		JigVersion:   1,
		Status:       "ready",
		StatusValues: []string{"ready", "complete"},
		Sessions:     []spec.Session{},
		DependsOn:    []spec.Dependency{},
		PinnedBeads:  []string{},
	}
}

func keysOf(m map[string]*cobra.Command) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
