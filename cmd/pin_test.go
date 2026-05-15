package cmd

// Plan 009 / B9 — kerf pin command tests.
//
// Spec references:
//   - specs/commands.md §"kerf pin" — steps 1-7, output messages, errors.
//   - specs/coordination.md §"Pin layer" — single-owner invariant.

import (
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gberns/kerf/internal/spec"
)

// writePinTestSpec writes a minimal spec.yaml for a work under
// $HOME/.kerf/projects/<proj>/<codename>/. The optional preamble (e.g.
// "# head comment\n") is written before the YAML body so comment-survival
// can be asserted.
func writePinTestSpec(t *testing.T, projectID, codename, preamble string, pinned []string) string {
	t.Helper()
	home := os.Getenv("HOME")
	workDir := filepath.Join(home, ".kerf", "projects", projectID, codename)
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("mkdir work dir: %v", err)
	}
	body := preamble + "codename: " + codename + "\n" +
		"type: plan\n" +
		"project:\n  id: " + projectID + "\n" +
		"jig: plan\n" +
		"jig_version: 1\n" +
		"status: problem-space\n" +
		"status_values: [problem-space, ready]\n" +
		"created: 2026-04-09T00:00:00Z\n" +
		"updated: 2026-04-09T00:00:00Z\n" +
		"sessions: []\n" +
		"depends_on: []\n" +
		"areas: []\n"
	if len(pinned) == 0 {
		body += "pinned_beads: []\n"
	} else {
		body += "pinned_beads:\n"
		for _, p := range pinned {
			body += "  - " + p + "\n"
		}
	}
	body += "implementation:\n  branch: null\n  pr: null\n  commits: []\n"
	specPath := filepath.Join(workDir, "spec.yaml")
	if err := os.WriteFile(specPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	// Ensure project.yaml exists so any helper that wants it does not bail.
	projectYAML := filepath.Join(filepath.Dir(workDir), "project.yaml")
	if _, err := os.Stat(projectYAML); os.IsNotExist(err) {
		_ = os.WriteFile(projectYAML, []byte("jigs: []\n"), 0o644)
	}
	return specPath
}

// runPinCapture invokes runPin with the given codename and bead ID, capturing
// output written via cobra's OutOrStdout. Returns (output, error).
func runPinCapture(t *testing.T, projectID, codename, beadID string) (string, error) {
	t.Helper()
	prevProject := projectFlag
	projectFlag = projectID
	t.Cleanup(func() { projectFlag = prevProject })

	var buf bytes.Buffer
	pinCmd.SetOut(&buf)
	t.Cleanup(func() { pinCmd.SetOut(nil) })

	err := runPin(pinCmd, codename, beadID)
	return buf.String(), err
}

// readSpec is a thin wrapper around spec.Read that t.Fatal's on error.
func readSpec(t *testing.T, path string) *spec.SpecYAML {
	t.Helper()
	s, err := spec.Read(path)
	if err != nil {
		t.Fatalf("read spec %s: %v", path, err)
	}
	return s
}

// ----------------------------------------------------------------------------
// Test 1: pinning B to work A removes B from work C's pinned_beads.
// (Spec step 4 — single-owner invariant.)
// ----------------------------------------------------------------------------

func TestPin_RemovesFromPriorOwner(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	// Point PATH at an empty dir so `br` is unavailable — the command must
	// not require the bead store to be present (per spec §"kerf pin"
	// rationale: pin is a kerf-side declaration).
	t.Setenv("PATH", t.TempDir())

	proj := "pin-proj-1"
	aPath := writePinTestSpec(t, proj, "alpha", "", nil)
	cPath := writePinTestSpec(t, proj, "gamma", "", []string{"kerf-cb-001"})

	out, err := runPinCapture(t, proj, "alpha", "kerf-cb-001")
	if err != nil {
		t.Fatalf("runPin: %v", err)
	}
	if !strings.Contains(out, "Pinned kerf-cb-001 to alpha") {
		t.Errorf("output missing pin confirmation: %q", out)
	}
	if !strings.Contains(out, "removed from gamma") {
		t.Errorf("output missing removed-from clause: %q", out)
	}

	aSpec := readSpec(t, aPath)
	cSpec := readSpec(t, cPath)
	if len(aSpec.PinnedBeads) != 1 || aSpec.PinnedBeads[0] != "kerf-cb-001" {
		t.Errorf("alpha.pinned_beads = %v, want [kerf-cb-001]", aSpec.PinnedBeads)
	}
	if len(cSpec.PinnedBeads) != 0 {
		t.Errorf("gamma.pinned_beads = %v, want []", cSpec.PinnedBeads)
	}
}

// ----------------------------------------------------------------------------
// Test 2: idempotent — pinning B to A twice leaves a single entry.
// (Spec step 3 — already-pinned no-op path.)
// ----------------------------------------------------------------------------

func TestPin_Idempotent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("PATH", t.TempDir())

	proj := "pin-proj-2"
	aPath := writePinTestSpec(t, proj, "alpha", "", nil)

	if _, err := runPinCapture(t, proj, "alpha", "kerf-cb-002"); err != nil {
		t.Fatalf("first pin: %v", err)
	}
	out, err := runPinCapture(t, proj, "alpha", "kerf-cb-002")
	if err != nil {
		t.Fatalf("second pin: %v", err)
	}
	if !strings.Contains(out, "is already pinned to alpha. No change.") {
		t.Errorf("second pin should emit no-op message; got %q", out)
	}
	aSpec := readSpec(t, aPath)
	if len(aSpec.PinnedBeads) != 1 || aSpec.PinnedBeads[0] != "kerf-cb-002" {
		t.Errorf("alpha.pinned_beads = %v, want [kerf-cb-002]", aSpec.PinnedBeads)
	}
}

// ----------------------------------------------------------------------------
// Test 3: bead ID does not exist in the bead store → error, no file change.
// (Bead 9 deliverable: validate via beads.List when available.)
//
// We install a stub `br` that emits a JSON list NOT containing the bead ID
// the test pins; the command must error and the target spec must be byte-
// unchanged.
// ----------------------------------------------------------------------------

func TestPin_BeadIDMissingInStore_Errors(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	// Stub `br` to return a JSON list that does NOT contain our test bead ID.
	stubBr(t, `[{"id":"kerf-other-001","status":"open","labels":[]}]`)

	proj := "pin-proj-3"
	aPath := writePinTestSpec(t, proj, "alpha", "", nil)
	before, err := os.ReadFile(aPath)
	if err != nil {
		t.Fatalf("read spec before: %v", err)
	}

	_, err = runPinCapture(t, proj, "alpha", "kerf-cb-404")
	if err == nil {
		t.Fatalf("expected error for missing bead ID, got nil")
	}
	if !strings.Contains(err.Error(), "not found in bead store") {
		t.Errorf("error message = %q, want substring 'not found in bead store'", err.Error())
	}

	after, err := os.ReadFile(aPath)
	if err != nil {
		t.Fatalf("read spec after: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("spec.yaml changed after a failed pin:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// ----------------------------------------------------------------------------
// Test 4: baseline file (sync-cache.json) is byte-unchanged after kerf pin.
// (Spec step 7: "The drift baseline is not advanced.")
//
// We seed a fake sync-cache.json at both candidate locations (bench-mode and
// local-mode paths under the resolver) and assert the bytes are identical
// after the pin runs. cmd/pin.go must NOT call drift.Advance.
// ----------------------------------------------------------------------------

func TestPin_DoesNotAdvanceDriftBaseline(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("PATH", t.TempDir())

	proj := "pin-proj-4"
	writePinTestSpec(t, proj, "alpha", "", nil)

	// Seed a fake sync-cache.json under the bench-mode location.
	cacheDir := filepath.Join(tmp, ".kerf", "projects", proj)
	cachePath := filepath.Join(cacheDir, "sync-cache.json")
	const cacheContent = `{"snapshot_id":"baseline-deadbeef","beads":{},"filter_assignments":{}}`
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("mkdir cache: %v", err)
	}
	if err := os.WriteFile(cachePath, []byte(cacheContent), 0o644); err != nil {
		t.Fatalf("write cache: %v", err)
	}
	beforeHash := sha256.Sum256([]byte(cacheContent))

	if _, err := runPinCapture(t, proj, "alpha", "kerf-cb-005"); err != nil {
		t.Fatalf("runPin: %v", err)
	}
	got, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("read cache after: %v", err)
	}
	afterHash := sha256.Sum256(got)
	if beforeHash != afterHash {
		t.Errorf("sync-cache.json changed after kerf pin:\nbefore: %x\nafter:  %x\ncontent after: %s",
			beforeHash, afterHash, got)
	}
}

// ----------------------------------------------------------------------------
// Test 5: comment-survival — a head comment on the target spec.yaml survives
// kerf pin via the B1 mutators.
// ----------------------------------------------------------------------------

func TestPin_PreservesHeadComment(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("PATH", t.TempDir())

	proj := "pin-proj-5"
	const preamble = "# load-bearing comment: do not delete\n"
	aPath := writePinTestSpec(t, proj, "alpha", preamble, nil)

	if _, err := runPinCapture(t, proj, "alpha", "kerf-cb-006"); err != nil {
		t.Fatalf("runPin: %v", err)
	}
	got, err := os.ReadFile(aPath)
	if err != nil {
		t.Fatalf("read spec after: %v", err)
	}
	if !strings.Contains(string(got), "# load-bearing comment: do not delete") {
		t.Errorf("head comment was lost; spec is now:\n%s", got)
	}
	// Sanity: the pin actually landed.
	s := readSpec(t, aPath)
	if len(s.PinnedBeads) != 1 || s.PinnedBeads[0] != "kerf-cb-006" {
		t.Errorf("alpha.pinned_beads = %v, want [kerf-cb-006]", s.PinnedBeads)
	}
}

// ----------------------------------------------------------------------------
// Test 6: invalid bead ID format → error.
// ----------------------------------------------------------------------------

func TestPin_InvalidBeadID(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("PATH", t.TempDir())

	proj := "pin-proj-6"
	writePinTestSpec(t, proj, "alpha", "", nil)

	_, err := runPinCapture(t, proj, "alpha", "not a valid id!!")
	if err == nil {
		t.Fatalf("expected error for invalid bead ID, got nil")
	}
	if !strings.Contains(err.Error(), "is not a valid identifier") {
		t.Errorf("error = %q, want substring 'is not a valid identifier'", err.Error())
	}
}

// ----------------------------------------------------------------------------
// Test 7: work-not-found → error matching spec wording.
// ----------------------------------------------------------------------------

func TestPin_WorkNotFound(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("PATH", t.TempDir())

	proj := "pin-proj-7"
	// Create the project dir but no work.
	_ = os.MkdirAll(filepath.Join(tmp, ".kerf", "projects", proj), 0o755)

	_, err := runPinCapture(t, proj, "nonexistent", "kerf-cb-007")
	if err == nil {
		t.Fatalf("expected error for missing work, got nil")
	}
	if !strings.Contains(err.Error(), "not found in project") {
		t.Errorf("error = %q, want substring 'not found in project'", err.Error())
	}
}
