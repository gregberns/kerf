package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gregberns/kerf/internal/sim/metrics"
	"github.com/gregberns/kerf/internal/sim/output"
)

// writeRun materializes a Result with the given Full block into dir.
// Test helper: only the fields used by diff need to be populated.
func writeRun(t *testing.T, dir string, full metrics.Block) {
	t.Helper()
	r := output.Result{
		ScenarioBytes: []byte("seed: 1\n"),
		WeightsBytes:  []byte("k: v\n"),
		ScenarioSHA:   "aa",
		WeightsSHA:    "bb",
		Full:          full,
		Warmup:        full,
		WarmupSkipped: true,
		WallTicks:     full.WallTicks,
		Agents:        2,
		StopReason:    "done",
	}
	if err := output.WriteRun(dir, r); err != nil {
		t.Fatalf("WriteRun(%s): %v", dir, err)
	}
}

// TestDiff_SingleRunDeltas verifies that diffing two single-run directories
// produces the correct per-metric delta (b - a) and that count==1 hides the
// percentile range.
func TestDiff_SingleRunDeltas(t *testing.T) {
	tmp := t.TempDir()
	aDir := filepath.Join(tmp, "a")
	bDir := filepath.Join(tmp, "b")
	writeRun(t, aDir, metrics.Block{
		WorkCompleted: 10, WorkTotal: 12, WallTicks: 100,
		AgentIdlePct: 0.25, AgentTicksTotal: 50,
	})
	writeRun(t, bDir, metrics.Block{
		WorkCompleted: 14, WorkTotal: 12, WallTicks: 90,
		AgentIdlePct: 0.10, AgentTicksTotal: 80,
	})

	var buf bytes.Buffer
	if err := runDiff(&buf, aDir, bDir, "json"); err != nil {
		t.Fatalf("runDiff: %v", err)
	}
	var doc diffOutput
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if doc.ARuns != 1 || doc.BRuns != 1 {
		t.Fatalf("expected 1 run per side, got A=%d B=%d", doc.ARuns, doc.BRuns)
	}

	want := map[string]float64{
		"work_completed":    4,    // 14-10
		"work_total":        0,    // 12-12
		"wall_ticks":        -10,  // 90-100
		"agent_idle_pct":    -0.15,
		"agent_ticks_total": 30, // 80-50
	}
	got := map[string]float64{}
	for _, r := range doc.Metrics {
		got[r.Metric] = r.Delta
		if r.A.Count != 1 || r.B.Count != 1 {
			t.Errorf("metric %s: expected count==1, got A=%d B=%d", r.Metric, r.A.Count, r.B.Count)
		}
		if r.A.P10 != r.A.P50 || r.A.P50 != r.A.P90 {
			t.Errorf("metric %s: single-run side A has spread: %+v", r.Metric, r.A)
		}
	}
	for k, v := range want {
		if !approxEqual(got[k], v) {
			t.Errorf("delta[%s]=%v, want %v", k, got[k], v)
		}
	}
}

// TestDiff_AggregatePercentiles verifies median + p10/p90 across multi-run
// aggregate directories.
func TestDiff_AggregatePercentiles(t *testing.T) {
	tmp := t.TempDir()
	aDir := filepath.Join(tmp, "a")
	bDir := filepath.Join(tmp, "b")

	// A: work_completed values {10, 20, 30, 40, 50}; nearest-rank
	// p10=10 (ceil(0.5)=1), p50=30 (ceil(2.5)=3), p90=50 (ceil(4.5)=5).
	for i, v := range []int{30, 10, 50, 20, 40} { // unsorted on disk
		writeRun(t, filepath.Join(aDir, "seed_"+strconvI(i)), metrics.Block{WorkCompleted: v})
	}
	// B: shifted by +5; medians shift by 5.
	for i, v := range []int{35, 15, 55, 25, 45} {
		writeRun(t, filepath.Join(bDir, "seed_"+strconvI(i)), metrics.Block{WorkCompleted: v})
	}

	var buf bytes.Buffer
	if err := runDiff(&buf, aDir, bDir, "json"); err != nil {
		t.Fatalf("runDiff: %v", err)
	}
	var doc diffOutput
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if doc.ARuns != 5 || doc.BRuns != 5 {
		t.Fatalf("expected 5 runs per side, got A=%d B=%d", doc.ARuns, doc.BRuns)
	}

	var row *diffRow
	for i := range doc.Metrics {
		if doc.Metrics[i].Metric == "work_completed" {
			row = &doc.Metrics[i]
			break
		}
	}
	if row == nil {
		t.Fatal("work_completed not in output")
	}
	if !approxEqual(row.A.P10, 10) || !approxEqual(row.A.P50, 30) || !approxEqual(row.A.P90, 50) {
		t.Errorf("A stats wrong: %+v", row.A)
	}
	if !approxEqual(row.B.P10, 15) || !approxEqual(row.B.P50, 35) || !approxEqual(row.B.P90, 55) {
		t.Errorf("B stats wrong: %+v", row.B)
	}
	if !approxEqual(row.Delta, 5) {
		t.Errorf("delta=%v, want 5", row.Delta)
	}
}

// TestDiff_MismatchedRunCounts verifies that an unequal number of runs per
// side yields an explicit error rather than silently truncating.
func TestDiff_MismatchedRunCounts(t *testing.T) {
	tmp := t.TempDir()
	aDir := filepath.Join(tmp, "a")
	bDir := filepath.Join(tmp, "b")
	for i := 0; i < 3; i++ {
		writeRun(t, filepath.Join(aDir, "seed_"+strconvI(i)), metrics.Block{WorkCompleted: i})
	}
	for i := 0; i < 2; i++ {
		writeRun(t, filepath.Join(bDir, "seed_"+strconvI(i)), metrics.Block{WorkCompleted: i})
	}
	err := runDiff(&bytes.Buffer{}, aDir, bDir, "json")
	if err == nil {
		t.Fatal("expected error for mismatched run counts, got nil")
	}
	if !strings.Contains(err.Error(), "mismatched run counts") {
		t.Fatalf("expected mismatched-count error, got: %v", err)
	}
}

// TestDiff_TextOutputStable verifies that the text rendering is deterministic
// across repeated invocations on the same inputs.
func TestDiff_TextOutputStable(t *testing.T) {
	tmp := t.TempDir()
	aDir := filepath.Join(tmp, "a")
	bDir := filepath.Join(tmp, "b")
	writeRun(t, aDir, metrics.Block{WorkCompleted: 10, WorkTotal: 12, WallTicks: 100, AgentIdlePct: 0.25})
	writeRun(t, bDir, metrics.Block{WorkCompleted: 14, WorkTotal: 12, WallTicks: 90, AgentIdlePct: 0.10})

	var b1, b2 bytes.Buffer
	if err := runDiff(&b1, aDir, bDir, "text"); err != nil {
		t.Fatal(err)
	}
	if err := runDiff(&b2, aDir, bDir, "text"); err != nil {
		t.Fatal(err)
	}
	if b1.String() != b2.String() {
		t.Errorf("text output not stable:\n--- run1 ---\n%s\n--- run2 ---\n%s", b1.String(), b2.String())
	}
	// Sanity: text output should mention metric names and a delta column.
	out := b1.String()
	for _, want := range []string{"work_completed", "delta", "+4"} {
		if !strings.Contains(out, want) {
			t.Errorf("text output missing %q:\n%s", want, out)
		}
	}
}

// TestDiff_JSONOutputStable verifies byte-stability of --format=json across
// repeat invocations.
func TestDiff_JSONOutputStable(t *testing.T) {
	tmp := t.TempDir()
	aDir := filepath.Join(tmp, "a")
	bDir := filepath.Join(tmp, "b")
	for i, v := range []int{10, 20, 30} {
		writeRun(t, filepath.Join(aDir, "seed_"+strconvI(i)), metrics.Block{WorkCompleted: v})
		writeRun(t, filepath.Join(bDir, "seed_"+strconvI(i)), metrics.Block{WorkCompleted: v + 1})
	}

	var b1, b2 bytes.Buffer
	if err := runDiff(&b1, aDir, bDir, "json"); err != nil {
		t.Fatal(err)
	}
	if err := runDiff(&b2, aDir, bDir, "json"); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b1.Bytes(), b2.Bytes()) {
		t.Errorf("json output not byte-stable")
	}
}

// TestDiff_SelfIsZero verifies the spec property: diffing a run against
// itself produces zero delta on every metric.
func TestDiff_SelfIsZero(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "run")
	writeRun(t, dir, metrics.Block{
		WorkCompleted: 7, WorkTotal: 9, WallTicks: 42,
		AgentIdlePct: 0.33, AgentTicksTotal: 100,
		ReworkP50Wait: 3, ReworkP95Wait: 11,
		TopOfQueueChurn: 0.2, GoalCompletion1d: 2,
		PriorityInversions: 1, AreaCollisions: 4,
	})
	var buf bytes.Buffer
	if err := runDiff(&buf, dir, dir, "json"); err != nil {
		t.Fatal(err)
	}
	var doc diffOutput
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	for _, r := range doc.Metrics {
		if !approxEqual(r.Delta, 0) {
			t.Errorf("self-diff %s: delta=%v, want 0", r.Metric, r.Delta)
		}
	}
}

// TestDiff_UnknownFormat verifies the format guard.
func TestDiff_UnknownFormat(t *testing.T) {
	tmp := t.TempDir()
	aDir := filepath.Join(tmp, "a")
	bDir := filepath.Join(tmp, "b")
	writeRun(t, aDir, metrics.Block{})
	writeRun(t, bDir, metrics.Block{})
	err := runDiff(&bytes.Buffer{}, aDir, bDir, "yaml")
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
}

// TestDiff_MissingSummary verifies a clean error for a directory that has
// neither summary.json nor child runs.
func TestDiff_MissingSummary(t *testing.T) {
	tmp := t.TempDir()
	empty := filepath.Join(tmp, "empty")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	good := filepath.Join(tmp, "good")
	writeRun(t, good, metrics.Block{})
	if err := runDiff(&bytes.Buffer{}, empty, good, "json"); err == nil {
		t.Fatal("expected error when side A has no summary.json")
	}
}

func approxEqual(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 1e-9
}

func strconvI(i int) string {
	// Local helper to avoid importing strconv just for tests.
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	digits := []byte{}
	for i > 0 {
		digits = append([]byte{byte('0' + i%10)}, digits...)
		i /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}
