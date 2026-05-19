// Plan 023 / B5 (kerf-k117) — Contract: kerf show / kerf work show
// shared-field agreement.
//
// Invariant. For any single work, the fields that BOTH `kerf show` and
// `kerf work show` render must carry identical values in both outputs.
// The two commands display overlapping metadata: `kerf show` is the
// rich, human-readable detail view; `kerf work show` is the
// spec.yaml-shaped field-by-field dump. They MUST agree on the
// underlying record — divergence would mean one renderer reads the spec
// differently from the other, which is a future-bug surface the audit
// behind plan 023 named explicitly (Background §bullet 4).
//
// Pre-emptive contract. No live bug motivates this — the rendering
// helpers do not currently share code. If a shared helper is ever
// introduced, this contract guards against drift in either direction;
// until then, it guards against the renderers diverging via independent
// edits.
//
// CANONICAL SHARED FIELD SET (OQ5 resolution).
//
// The two commands format their output differently — `kerf show` uses
// `Title-case: value` lines with prose framing; `kerf work show` uses
// `lower_snake: value` lines in spec.yaml field order. To compare, we
// extract values from each output by line-prefix and compare normalized
// strings. The fields covered:
//
//   - codename        — `kerf show` "Work: <cn>"   vs `kerf work show` "codename: <cn>"
//   - type            — `kerf show` "Type: <t>"    vs `kerf work show` "type: <t>"
//   - status          — `kerf show` "Status: <s>"  vs `kerf work show` "status: <s>"
//   - project_id      — `kerf show` "Project: <p>" vs `kerf work show` "project_id: <p>"
//   - jig + version   — both render "jig: <name> (v<n>)" (verbatim)
//   - bead_filter     — both render the slot via renderBeadFilterSlot (verbatim)
//   - areas           — `kerf show` "Areas: a, b"   vs `kerf work show` "areas: a, b"
//
// Fields NOT in the shared set (and why):
//   - title is rendered by both, but only `kerf work show` emits
//     "title: (none)" for the absent case (kerf show omits the line).
//     We only fixture the present case, where both emit the value.
//   - depends_on: `kerf show` emits a multi-line block with resolution
//     status; `kerf work show` emits a single comma-joined line without
//     resolution. The codenames listed agree but the rendering is
//     fundamentally different — out of scope for this contract.
//   - sessions: similar story (multi-line vs collapsed).
//   - pinned_beads: only `kerf work show` renders; `kerf show` shows
//     pinned beads inside the attached-beads block instead.
//   - created/updated: both render but in identical RFC3339 format —
//     covered.
//
// Helper note (OQ5). The bead body offered the option of introducing a
// shared rendering helper if the two commands "really do agree
// field-for-field". They do not at the line level (the framings differ
// by design — `kerf show` is a human view, `kerf work show` is a
// dump). Introducing a helper would force one of them to change
// rendering, which is a spec change beyond this bead's scope. We
// resolve OQ5 here: shared field = same underlying spec value, NOT
// same surface line. Comparison normalizes by extracting values.
//
// Spec ref. specs/testing.md §"Property-Based Tests" §"Cross-command
// contracts" — recognised contract id `show-agreement`. Plan ref:
// plans/023_property_contracts/_plan.md §B5.
package contracttest

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gberns/kerf/cmd"
	"github.com/gberns/kerf/internal/beads"
	"github.com/gberns/kerf/internal/spec"
)

const showAgreementContractID = "show-agreement"

// sharedField names one field both renderers emit. Each entry knows how
// to pull the value from the two outputs. Both extractors return the
// value with surrounding whitespace trimmed; the comparison is a
// straight string equality after extraction.
type sharedField struct {
	// name identifies the field in failure messages.
	name string
	// showPrefix is the "Title-case key: " (or similar) anchor `kerf show`
	// uses on its line. Match is "the first line in output that begins
	// with this prefix".
	showPrefix string
	// workShowPrefix is the analogous anchor for `kerf work show`.
	workShowPrefix string
}

// canonicalSharedFields is the maintained list of fields covered by
// this contract. Adding a new field: append here, ensuring both
// renderers emit a stable, unique line prefix for it.
var canonicalSharedFields = []sharedField{
	{name: "codename", showPrefix: "Work: ", workShowPrefix: "codename:"},
	{name: "type", showPrefix: "Type: ", workShowPrefix: "type:"},
	{name: "status", showPrefix: "Status: ", workShowPrefix: "status:"},
	{name: "project_id", showPrefix: "Project: ", workShowPrefix: "project_id:"},
	{name: "jig", showPrefix: "Jig: ", workShowPrefix: "jig:"},
	{name: "bead_filter", showPrefix: "bead_filter:", workShowPrefix: "bead_filter:"},
	{name: "areas", showPrefix: "Areas: ", workShowPrefix: "areas:"},
	{name: "created", showPrefix: "Created: ", workShowPrefix: "created:"},
	{name: "updated", showPrefix: "Updated: ", workShowPrefix: "updated:"},
}

// TestContract_ShowAgreement seeds a fully-populated work fixture on a
// scratch HOME, runs both `kerf show <cn>` and `kerf work show <cn>`
// in-process via rootCmd.Execute, then asserts the extracted values
// agree for every entry in canonicalSharedFields.
func TestContract_ShowAgreement(t *testing.T) {
	if IsExempt("kerf.show", showAgreementContractID) || IsExempt("kerf.work.show", showAgreementContractID) {
		// Exemptions for this contract are theoretical; if either side
		// is marked exempt, the contract cannot meaningfully run.
		t.Skip("show or work.show exempt from show-agreement contract")
	}

	// Isolated HOME so cmdutil.Resolver builds inside this test.
	home := t.TempDir()
	t.Setenv("HOME", home)
	// PATH replaced with empty so any developer-local `bd`/`br` cannot
	// leak in and add lines to `kerf show` output that we don't expect.
	t.Setenv("PATH", t.TempDir())

	const (
		projectID = "kerf-k117-contract"
		codename  = "shared-fixture"
	)
	projectDir := filepath.Join(home, ".kerf", "projects", projectID)
	workDir := filepath.Join(projectDir, codename)
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("seeding work dir: %v", err)
	}

	// Build the fixture spec. Fields are chosen to exercise the shared
	// set listed at the top of this file: bead_filter set (so the
	// renderBeadFilterSlot path is hit), title present, areas
	// non-empty, depends_on/pinned_beads/sessions non-empty (so the
	// non-shared blocks are exercised but do not contaminate the
	// shared-field extraction).
	sid := "sess-k117"
	dep1Proj := "other-proj"
	depBranch := "feat/k117"
	title := "Shared fixture title"
	fixture := &spec.SpecYAML{
		Codename:     codename,
		Title:        &title,
		Type:         "feature",
		Project:      spec.Project{ID: projectID},
		Jig:          "feature",
		JigVersion:   1,
		Status:       "research",
		StatusValues: []string{"problem-space", "decomposition", "research", "detailed-spec", "review", "ready"},
		Created:      time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC),
		Updated:      time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC),
		Sessions: []spec.Session{
			{ID: &sid, Started: time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)},
		},
		Areas: []string{"area-one", "area-two"},
		DependsOn: []spec.Dependency{
			{Codename: "upstream-work", Project: &dep1Proj, Relationship: "must-complete-first"},
		},
		BeadFilter: &beads.Filter{Label: "codename:" + codename},
		PinnedBeads: []string{"hk-aa01"},
		Implementation: spec.Implementation{Branch: &depBranch},
	}

	specPath := filepath.Join(workDir, "spec.yaml")
	if err := spec.Write(specPath, fixture); err != nil {
		t.Fatalf("writing fixture spec.yaml: %v", err)
	}

	// Re-read so timestamps / canonicalization match what the
	// renderers will see; spec.Write rewrites Updated.
	reread, err := spec.Read(specPath)
	if err != nil {
		t.Fatalf("rereading fixture: %v", err)
	}
	_ = reread // only used to assert the fixture is readable; rendering goes through the cobra commands.

	showOut := runRoot(t, "show", codename, "--project", projectID)
	workShowOut := runRoot(t, "work", "show", codename, "--project", projectID)

	for _, f := range canonicalSharedFields {
		showVal, ok := extractByPrefix(showOut, f.showPrefix)
		if !ok {
			t.Errorf("field %q: `kerf show` output does not contain a line beginning with %q.\nfull show output:\n%s", f.name, f.showPrefix, showOut)
			continue
		}
		workVal, ok := extractByPrefix(workShowOut, f.workShowPrefix)
		if !ok {
			t.Errorf("field %q: `kerf work show` output does not contain a line beginning with %q.\nfull work-show output:\n%s", f.name, f.workShowPrefix, workShowOut)
			continue
		}
		if showVal != workVal {
			t.Errorf(
				"contract violated: field %q disagrees between renderers.\n"+
					"  kerf show     : %q\n"+
					"  kerf work show: %q\n"+
					"  fix: align the two renderers on the same underlying spec field, or — if the divergence is intentional — register an opt-out in internal/contracttest/opt_outs.go citing this bead (kerf-k117) and document the rationale in specs/commands.md.",
				f.name, showVal, workVal,
			)
		}
	}
}

// extractByPrefix finds the first line of out that begins with prefix
// and returns the remainder of that line with surrounding whitespace
// trimmed. Returns ok=false if no such line exists.
//
// `kerf show` uses a single space after the colon ("Status: foo");
// `kerf work show` pads with multiple spaces for column alignment
// ("status:         foo"). After trim, both yield "foo".
func extractByPrefix(out, prefix string) (string, bool) {
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix)), true
		}
	}
	return "", false
}

// runRoot invokes the assembled kerf cobra root with argv and returns
// the combined stdout (cobra.SetOut target) plus anything the command
// writes directly to os.Stdout. Both renderers in this contract write
// via fmt.Print* to os.Stdout, so we redirect os.Stdout into a pipe
// for the duration of the call — matching runConfig in the
// config-roundtrip contract.
func runRoot(t *testing.T, args ...string) string {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	root := cmd.Root()
	root.SetArgs(args)
	var outBuf, errBuf bytes.Buffer
	root.SetOut(w)
	root.SetErr(&errBuf)
	t.Cleanup(func() {
		root.SetArgs(nil)
		root.SetOut(nil)
		root.SetErr(nil)
	})

	execErr := root.Execute()

	_ = w.Close()
	os.Stdout = oldStdout
	if _, copyErr := io.Copy(&outBuf, r); copyErr != nil {
		t.Fatalf("draining stdout pipe: %v", copyErr)
	}
	if execErr != nil {
		t.Fatalf("`kerf %s` failed: %v\nstdout:\n%s\nstderr:\n%s", strings.Join(args, " "), execErr, outBuf.String(), errBuf.String())
	}
	return outBuf.String()
}
