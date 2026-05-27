// debug_dispatch.go — `--debug-dispatch` sink for the kerfsim run subcommand.
//
// Implements metrics.DebugSink as a file-backed JSONL writer. One record per
// line, ordered by arrival/dispatch as observed by the LoopHooks adapter.
// Used by Plan 008 / B14 (sim-integrity investigation gate) to diagnose
// rework-metric anomalies (priority_inversions, rework_p50_wait).
//
// Record schema:
//
//	{"kind":"header","scenario_sha":"…","warmup_cutoff":N,"ticks_cap":N,"agents":N}
//	{"kind":"arrival","tick":N,"bead":"…","work":"…","is_rework":bool,"depends_on":[…]}
//	{"kind":"dispatch","tick":N,"agent":N,"bead":"…","work":"…",
//	 "is_rework":bool,"arrival_tick":N,"older_rework_eligible":bool,
//	 "unmet_deps":[…],"in_warmup":bool}
//
// "in_warmup" reflects a conservative warmup-window upper bound derived from
// ticks_cap alone (the runtime wall_ticks is unknown until run-end; the
// spec's true cutoff is min(0.1*ticks_cap, 0.1*wall_ticks), always smaller).
// A dispatch flagged in_warmup=false is definitively outside the window;
// a dispatch flagged in_warmup=true may or may not be — re-derive post-hoc
// from the run's wall_ticks if needed.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/gregberns/kerf/internal/sim/metrics"
)

// jsonlDebugSink is the file-backed implementation of metrics.DebugSink
// used by `--debug-dispatch`.
type jsonlDebugSink struct {
	f *os.File
	w *bufio.Writer
}

// newJSONLDebugSink opens (truncating) the file at path for write and
// returns a buffered JSONL sink.
func newJSONLDebugSink(path string) (*jsonlDebugSink, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	return &jsonlDebugSink{f: f, w: bufio.NewWriter(f)}, nil
}

// Close flushes pending writes and closes the underlying file.
func (s *jsonlDebugSink) Close() error {
	if s == nil {
		return nil
	}
	if err := s.w.Flush(); err != nil {
		s.f.Close()
		return err
	}
	return s.f.Close()
}

// Header writes the run-level header record.
func (s *jsonlDebugSink) Header(scenarioSHA string, warmupCutoff int64, ticksCap int64, agents int) {
	s.write(map[string]any{
		"kind":          "header",
		"scenario_sha":  scenarioSHA,
		"warmup_cutoff": warmupCutoff,
		"ticks_cap":     ticksCap,
		"agents":        agents,
	})
}

// Arrival writes one arrival record.
func (s *jsonlDebugSink) Arrival(a metrics.ArrivalInfo) {
	s.write(map[string]any{
		"kind":       "arrival",
		"tick":       a.Tick,
		"bead":       a.BeadID,
		"work":       a.WorkCode,
		"is_rework":  a.IsRework,
		"depends_on": depsOrEmpty(a.DependsOn),
	})
}

// Dispatch writes one dispatch record.
func (s *jsonlDebugSink) Dispatch(d metrics.DispatchInfo, inWarmup bool) {
	s.write(map[string]any{
		"kind":                  "dispatch",
		"tick":                  d.Tick,
		"agent":                 d.AgentID,
		"bead":                  d.BeadID,
		"work":                  d.WorkCode,
		"is_rework":             d.IsRework,
		"arrival_tick":          d.ArrivalTick,
		"older_rework_eligible": d.HadOlderRework,
		"unmet_deps":            depsOrEmpty(d.UnmetDeps),
		"in_warmup":             inWarmup,
	})
}

// write serializes one record as a single JSON object followed by a
// newline. Errors are silenced: the sink is a best-effort diagnostic.
func (s *jsonlDebugSink) write(rec map[string]any) {
	b, err := json.Marshal(rec)
	if err != nil {
		fmt.Fprintf(os.Stderr, "debug-dispatch marshal: %v\n", err)
		return
	}
	_, _ = s.w.Write(b)
	_, _ = s.w.WriteString("\n")
}

// depsOrEmpty returns an empty slice (so JSON serializes as `[]`, not
// `null`) when the input is nil. This keeps the JSONL line shape stable
// for grep-based diagnostics.
func depsOrEmpty(ds []string) []string {
	if ds == nil {
		return []string{}
	}
	return ds
}
