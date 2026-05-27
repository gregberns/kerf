package output

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gregberns/kerf/internal/sim/metrics"
)

// fixtureResult builds a deterministic Result that covers all four event
// kinds and both summary blocks.
func fixtureResult() Result {
	agent0 := 0
	agent1 := 1
	return Result{
		ScenarioBytes: []byte("seed: 42\nticks: 100\nagents: 2\n"),
		WeightsBytes:  []byte("rework_weight: 1.0\n"),
		ScenarioSHA:   "deadbeef",
		WeightsSHA:    "cafef00d",
		Full: metrics.Block{
			WorkCompleted: 2, WorkTotal: 3, WallTicks: 80,
			AgentIdlePct: 0.25, AgentTicksTotal: 120,
			ReworkP50Wait: 5, ReworkP95Wait: 17,
			TopOfQueueChurn: 0.4,
			GoalCompletion1d: 1, GoalCompletion3d: 2, GoalCompletion7d: 2,
			PriorityInversions: 1, AreaCollisions: 0,
		},
		Warmup: metrics.Block{
			WorkCompleted: 1, WorkTotal: 3, WallTicks: 60,
			AgentIdlePct: 0.30, AgentTicksTotal: 90,
			TopOfQueueChurn: 0.5,
			GoalCompletion1d: 1, GoalCompletion3d: 2, GoalCompletion7d: 2,
		},
		WarmupSkipped: false,
		WallTicks:     80,
		Agents:        2,
		StopReason:    "all-terminal",
		Events: []EventEntry{
			{T: 10, Kind: "arrival", Bead: "hk-1", Work: "auth"},
			{T: 11, Kind: "dispatch", Agent: &agent0, Bead: "hk-1", Work: "auth"},
			{T: 15, Kind: "queue_snapshot", Top: "hk-2"},
			{T: 20, Kind: "complete", Bead: "hk-1"},
			{T: 21, Kind: "dispatch", Agent: &agent1, Bead: "hk-2", Work: "auth"},
		},
	}
}

func TestWriteRunCreatesAllFiles(t *testing.T) {
	dir := t.TempDir()
	if err := WriteRun(dir, fixtureResult()); err != nil {
		t.Fatalf("WriteRun: %v", err)
	}
	for _, name := range []string{"summary.json", "summary.txt", "events.jsonl", "scenario.yaml", "weights.yaml"} {
		p := filepath.Join(dir, name)
		info, err := os.Stat(p)
		if err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
		if info.Size() == 0 {
			t.Fatalf("empty %s", name)
		}
	}
}

func TestSummaryJSONShape(t *testing.T) {
	dir := t.TempDir()
	r := fixtureResult()
	if err := WriteRun(dir, r); err != nil {
		t.Fatalf("WriteRun: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "summary.json"))
	if err != nil {
		t.Fatalf("read summary.json: %v", err)
	}
	var doc summaryDoc
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("unmarshal summary.json: %v", err)
	}
	if !reflect.DeepEqual(doc.Full, r.Full) {
		t.Errorf("full mismatch: got %+v want %+v", doc.Full, r.Full)
	}
	if !reflect.DeepEqual(doc.Warmup, r.Warmup) {
		t.Errorf("warmup mismatch: got %+v want %+v", doc.Warmup, r.Warmup)
	}
	if doc.ScenarioSHA256 != r.ScenarioSHA || doc.WeightsSHA256 != r.WeightsSHA {
		t.Errorf("sha mismatch: got %s/%s", doc.ScenarioSHA256, doc.WeightsSHA256)
	}
	if doc.StopReason != "all-terminal" || doc.Agents != 2 || doc.WallTicks != 80 {
		t.Errorf("scalar mismatch: %+v", doc)
	}
}

func TestEventsJSONLRoundTrip(t *testing.T) {
	dir := t.TempDir()
	r := fixtureResult()
	if err := WriteRun(dir, r); err != nil {
		t.Fatalf("WriteRun: %v", err)
	}
	f, err := os.Open(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		t.Fatalf("open events.jsonl: %v", err)
	}
	defer f.Close()

	type line struct {
		T     int64  `json:"t"`
		Kind  string `json:"kind"`
		Agent *int   `json:"agent,omitempty"`
		Bead  string `json:"bead,omitempty"`
		Work  string `json:"work,omitempty"`
		Top   string `json:"top,omitempty"`
	}
	var got []line
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var l line
		if err := json.Unmarshal(sc.Bytes(), &l); err != nil {
			t.Fatalf("unmarshal line %q: %v", sc.Text(), err)
		}
		got = append(got, l)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(got) != len(r.Events) {
		t.Fatalf("line count: got %d want %d", len(got), len(r.Events))
	}

	// Build the expected set keyed by (T, Kind, Bead, Top) so we can verify
	// every event round-trips regardless of canonical-order shuffles.
	want := map[[4]string]EventEntry{}
	for _, e := range r.Events {
		key := [4]string{itoa(e.T), e.Kind, e.Bead, e.Top}
		want[key] = e
	}
	for _, g := range got {
		key := [4]string{itoa(g.T), g.Kind, g.Bead, g.Top}
		e, ok := want[key]
		if !ok {
			t.Fatalf("unexpected event: %+v", g)
		}
		if g.Work != e.Work {
			t.Errorf("work mismatch for %v: got %q want %q", key, g.Work, e.Work)
		}
		if (g.Agent == nil) != (e.Agent == nil) {
			t.Errorf("agent presence mismatch for %v", key)
		} else if g.Agent != nil && *g.Agent != *e.Agent {
			t.Errorf("agent value mismatch for %v: got %d want %d", key, *g.Agent, *e.Agent)
		}
	}
}

func TestDeterminism(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	if err := WriteRun(dirA, fixtureResult()); err != nil {
		t.Fatalf("WriteRun A: %v", err)
	}
	if err := WriteRun(dirB, fixtureResult()); err != nil {
		t.Fatalf("WriteRun B: %v", err)
	}
	for _, name := range []string{"summary.json", "summary.txt", "events.jsonl", "scenario.yaml", "weights.yaml"} {
		a, err := os.ReadFile(filepath.Join(dirA, name))
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(dirB, name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(a, b) {
			t.Errorf("non-deterministic %s", name)
		}
	}
}

func TestEventsCanonicalOrder(t *testing.T) {
	// Feed events in reversed order; written file must still be canonical.
	r := fixtureResult()
	rev := make([]EventEntry, len(r.Events))
	for i, e := range r.Events {
		rev[len(r.Events)-1-i] = e
	}
	r2 := r
	r2.Events = rev

	dirA := t.TempDir()
	dirB := t.TempDir()
	if err := WriteRun(dirA, r); err != nil {
		t.Fatalf("WriteRun: %v", err)
	}
	if err := WriteRun(dirB, r2); err != nil {
		t.Fatalf("WriteRun: %v", err)
	}
	a, _ := os.ReadFile(filepath.Join(dirA, "events.jsonl"))
	b, _ := os.ReadFile(filepath.Join(dirB, "events.jsonl"))
	if !bytes.Equal(a, b) {
		t.Errorf("events.jsonl not canonical-ordered:\nA:\n%s\nB:\n%s", a, b)
	}
}

func TestInputCopies(t *testing.T) {
	dir := t.TempDir()
	r := fixtureResult()
	if err := WriteRun(dir, r); err != nil {
		t.Fatalf("WriteRun: %v", err)
	}
	gotScen, err := os.ReadFile(filepath.Join(dir, "scenario.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotScen, r.ScenarioBytes) {
		t.Errorf("scenario.yaml mismatch")
	}
	gotW, err := os.ReadFile(filepath.Join(dir, "weights.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotW, r.WeightsBytes) {
		t.Errorf("weights.yaml mismatch")
	}
}

func TestWriteRunCreatesMissingDir(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "nested", "run-001")
	if err := WriteRun(target, fixtureResult()); err != nil {
		t.Fatalf("WriteRun: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "summary.json")); err != nil {
		t.Fatalf("expected nested dir created: %v", err)
	}
}

func TestWriteRunRejectsNonDir(t *testing.T) {
	root := t.TempDir()
	bogus := filepath.Join(root, "imafile")
	if err := os.WriteFile(bogus, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteRun(bogus, fixtureResult()); err == nil {
		t.Fatalf("expected error when path is a file")
	}
}

func TestSummaryJSONStandalone(t *testing.T) {
	b, err := SummaryJSON(fixtureResult())
	if err != nil {
		t.Fatalf("SummaryJSON: %v", err)
	}
	var doc summaryDoc
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
}

func itoa(n int64) string {
	// Tiny helper to avoid pulling strconv into the keying logic above.
	return string(jsonNumber(n))
}

func jsonNumber(n int64) []byte {
	b, _ := json.Marshal(n)
	return b
}
