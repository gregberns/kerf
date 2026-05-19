// Plan 022 / Scenario C (kerf-c0v7) — bootstrap-filters round-trip.
//
// Verifies the kerf-2km invariant end-to-end: when `kerf bootstrap-filters`
// proposes an `any:` union (mixed labels match more than one candidate prefix
// without any single one dominating), the exact wire-form clause it writes
// must round-trip through `kerf work edit --bead-filter-add`. Regression here
// would mean the proposer and the parser have drifted apart.
package scenariotest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readWorkSpec returns the contents of $HOME/.kerf/projects/<id>/<cn>/spec.yaml.
// The scenario reads through HOME rather than the project root because kerf
// stores work artefacts under the bench, not in the user's repo.
func readWorkSpec(t testing.TB, r *Runner, projectID, codename string) string {
	t.Helper()
	path := filepath.Join(r.HomeDir(), ".kerf", "projects", projectID, codename, "spec.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// bootstrapJSON is the decoded shape of `kerf bootstrap-filters --format json`.
// Only the fields the scenario inspects are modelled.
type bootstrapJSON struct {
	Reports []struct {
		Codename string   `json:"codename"`
		Reason   string   `json:"reason"`
		Clauses  []string `json:"clauses"`
	} `json:"reports"`
}

// TestScenario_BootstrapRoundtrip drives the kerf-c0v7 scenario: seed beads
// with two label families (codename:foo and subsystem:foo) so the sampler
// produces an `any:` union proposal; capture the clauses; assert the proposal
// reaches spec.yaml as an `any:` union; then feed the same wire-form string
// into `kerf work edit --bead-filter-add` against a sibling work and assert
// the resulting bead_filter is structurally identical.
func TestScenario_BootstrapRoundtrip(t *testing.T) {
	r := New(t)

	// 1. Init the project. kerf init resolves the git root that `bd init`
	//    created inside the project root, so this Just Works without any
	//    extra setup.
	if stdout, stderr, code, err := r.Run("init", "--no"); err != nil || code != 0 {
		t.Fatalf("kerf init: code=%d err=%v\nstdout:\n%s\nstderr:\n%s", code, err, stdout, stderr)
	}

	// Point kerf at `bd` for task reads. Default is `br`, but on hosts where
	// the local `br` build is older than the `bd` JSON schema (a common
	// environment skew), `br` cannot decode `bd`'s export and the bootstrap
	// command fails before it gets to do anything interesting. Configuring
	// tools.tasks=bd routes all reads through the same binary the harness
	// uses to seed beads — no schema mismatch.
	if stdout, stderr, code, err := r.Run("config", "tools.tasks", "bd"); err != nil || code != 0 {
		t.Fatalf("kerf config tools.tasks bd: code=%d err=%v\nstdout:\n%s\nstderr:\n%s",
			code, err, stdout, stderr)
	}

	// 2. Seed beads with mixed labels. Each label family carries three open
	//    beads — equal counts mean no single candidate dominates (the 80%
	//    threshold cannot be cleared), but each clears the union-member floor
	//    of 2, so the sampler emits an `any:` proposal.
	beads := []BeadSpec{
		{Title: "a1", Priority: "p2", Labels: []string{"codename:foo"}},
		{Title: "a2", Priority: "p2", Labels: []string{"codename:foo"}},
		{Title: "a3", Priority: "p2", Labels: []string{"codename:foo"}},
		{Title: "b1", Priority: "p2", Labels: []string{"subsystem:foo"}},
		{Title: "b2", Priority: "p2", Labels: []string{"subsystem:foo"}},
		{Title: "b3", Priority: "p2", Labels: []string{"subsystem:foo"}},
	}
	r.SeedBeads(beads)

	// 3. `kerf new foo`. --no-auto-filter is set so the per-work auto-filter
	//    path (which only fires on a dominant single-leaf proposal anyway)
	//    cannot accidentally pre-populate bead_filter and short-circuit the
	//    bootstrap path we want to exercise.
	if stdout, stderr, code, err := r.Run("new", "foo", "--jig", "plan", "--no-auto-filter"); err != nil || code != 0 {
		t.Fatalf("kerf new foo: code=%d err=%v\nstdout:\n%s\nstderr:\n%s", code, err, stdout, stderr)
	}

	// 4. `kerf bootstrap-filters --yes` — dry-mode preview would suffice to
	//    capture the clause, but --yes is the documented apply path and the
	//    one the scenario is asserting works end-to-end.
	bootstrapOut, bootstrapErr, code, err := r.Run("bootstrap-filters", "--yes", "--codename", "foo", "--format", "json")
	if err != nil || code != 0 {
		t.Fatalf("kerf bootstrap-filters: code=%d err=%v\nstdout:\n%s\nstderr:\n%s",
			code, err, bootstrapOut, bootstrapErr)
	}

	// Find the JSON object in stdout. The command may emit a confirmation
	// preamble even under --yes; locate the first `{` to be tolerant.
	jsonStart := strings.Index(bootstrapOut, "{")
	if jsonStart < 0 {
		t.Fatalf("bootstrap-filters: no JSON object in output:\n%s", bootstrapOut)
	}
	var report bootstrapJSON
	if err := json.Unmarshal([]byte(bootstrapOut[jsonStart:]), &report); err != nil {
		t.Fatalf("decode bootstrap json: %v\nbody:\n%s", err, bootstrapOut[jsonStart:])
	}

	// Locate the foo report and assert it produced a union.
	var fooClauses []string
	var fooReason string
	for _, rpt := range report.Reports {
		if rpt.Codename == "foo" {
			fooClauses = rpt.Clauses
			fooReason = rpt.Reason
			break
		}
	}
	if fooReason != "union" {
		t.Fatalf("expected foo proposal reason 'union', got %q (clauses=%v)", fooReason, fooClauses)
	}
	if len(fooClauses) < 2 {
		t.Fatalf("expected union proposal with >=2 clauses, got %v", fooClauses)
	}

	// 5. Assert foo's spec.yaml carries an `any:` union shape. The mutator
	//    is comment-preserving but the structural shape is fixed: an
	//    `any:` key under `bead_filter:`.
	//
	//    Per-work specs live under $HOME/.kerf/projects/<id>/<cn>/spec.yaml,
	//    where <id> is the basename of the project root (see
	//    internal/project.Resolve). The scenario root is r.ProjectRoot().
	projectID := filepath.Base(r.ProjectRoot())
	fooSpec := readWorkSpec(t, r, projectID, "foo")
	if !strings.Contains(fooSpec, "bead_filter:") || !strings.Contains(fooSpec, "any:") {
		t.Fatalf("foo spec.yaml missing bead_filter / any:\n%s", fooSpec)
	}
	for _, c := range fooClauses {
		// Each leaf clause is of the form `label=<value>` — assert the value
		// half landed in the spec. (We don't anchor on `label=` because the
		// YAML rendering may use the key/value form `label: subsystem:foo`.)
		val := c
		if eq := strings.IndexByte(c, '='); eq >= 0 {
			val = c[eq+1:]
		}
		if !strings.Contains(fooSpec, val) {
			t.Errorf("foo spec.yaml missing clause value %q\n%s", val, fooSpec)
		}
	}

	// 6. `kerf new bar` — a sibling work with the same dominant label shapes.
	//    --no-auto-filter again so the per-work auto-filter path doesn't
	//    pre-populate bar's bead_filter from the dominant-leaf code path.
	if stdout, stderr, code, err := r.Run("new", "bar", "--jig", "plan", "--no-auto-filter"); err != nil || code != 0 {
		t.Fatalf("kerf new bar: code=%d err=%v\nstdout:\n%s\nstderr:\n%s", code, err, stdout, stderr)
	}

	// 7. Round-trip the captured proposal through `kerf work edit`. The
	//    `any:<leaf>,<leaf>...` wire form is the exact string the parser
	//    accepts (see internal/beads/filter.go ParseFilterClause), so this
	//    is the strictest possible round-trip: the proposer emits clauses,
	//    we re-serialise them into the wire form, and the parser must agree.
	unionWire := "any:" + strings.Join(fooClauses, ",")
	if stdout, stderr, code, err := r.Run("work", "edit", "bar", "--bead-filter-add", unionWire); err != nil || code != 0 {
		t.Fatalf("kerf work edit bar --bead-filter-add %q: code=%d err=%v\nstdout:\n%s\nstderr:\n%s",
			unionWire, code, err, stdout, stderr)
	}

	// 8. Assert bar's resulting spec.yaml carries the same structural
	//    `any:` union as foo's. We don't byte-compare (comments, ordering,
	//    timestamps differ) — we assert the `any:` key plus every leaf
	//    value is present, which is the kerf-2km invariant.
	barSpec := readWorkSpec(t, r, projectID, "bar")
	if !strings.Contains(barSpec, "bead_filter:") || !strings.Contains(barSpec, "any:") {
		t.Fatalf("bar spec.yaml missing bead_filter / any: after round-trip:\n%s", barSpec)
	}
	for _, c := range fooClauses {
		val := c
		if eq := strings.IndexByte(c, '='); eq >= 0 {
			val = c[eq+1:]
		}
		if !strings.Contains(barSpec, val) {
			t.Errorf("bar spec.yaml missing clause value %q after round-trip\n%s", val, barSpec)
		}
	}
}

