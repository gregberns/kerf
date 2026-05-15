// Package output writes the on-disk artifacts of a single kerfsim run.
//
// One run directory contains:
//
//	summary.json    — canonical machine-readable summary (full + warmup blocks)
//	summary.txt     — compact human-readable rendering of summary.json
//	events.jsonl    — one JSON object per line, in canonical event order
//	scenario.yaml   — copy of the scenario bytes used
//	weights.yaml    — copy of the effective weights bytes used
//
// All writers in this package are pure: same `Result` in, byte-identical
// files out. No wall-clock data is ever embedded.
//
// Spec: specs/simulator.md §Run Output, §summary.json, §events.jsonl.
package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gberns/kerf/internal/sim/metrics"
)

// Result is the aggregate produced by a finished kerfsim run, projected to
// canonical on-disk form by WriteRun.
//
// ScenarioBytes/WeightsBytes are the exact bytes copied into the run dir;
// ScenarioSHA/WeightsSHA are the hex-encoded sha256 of those bytes and are
// embedded in summary.json. The caller computes them so this package stays
// free of hashing dependencies.
type Result struct {
	ScenarioBytes []byte
	WeightsBytes  []byte
	ScenarioSHA   string
	WeightsSHA    string

	Full          metrics.Block
	Warmup        metrics.Block
	WarmupSkipped bool

	WallTicks  int64
	Agents     int
	StopReason string

	// Events are written in the order supplied; callers are responsible for
	// sorting into canonical event order before calling WriteRun. WriteRun
	// applies a stable secondary sort so byte-identical inputs always yield
	// byte-identical events.jsonl.
	Events []EventEntry
}

// EventEntry is one row of events.jsonl. Kind-specific fields are zero when
// not applicable; encoding only emits the fields valid for the given Kind.
type EventEntry struct {
	T     int64
	Kind  string
	Agent *int
	Bead  string
	Work  string
	Top   string // queue_snapshot only
}

// summaryDoc is the on-disk shape of summary.json. Field order in this struct
// fixes JSON key order under encoding/json.
type summaryDoc struct {
	Full           metrics.Block `json:"full"`
	Warmup         metrics.Block `json:"warmup"`
	WarmupSkipped  bool          `json:"warmup_skipped"`
	WallTicks      int64         `json:"wall_ticks"`
	Agents         int           `json:"agents"`
	ScenarioSHA256 string        `json:"scenario_sha256"`
	WeightsSHA256  string        `json:"weights_sha256"`
	StopReason     string        `json:"stop_reason"`
}

// WriteRun materializes a Result into dir. dir is created if it does not
// exist; it is an error if dir exists as a non-directory.
//
// All four files (summary.json, summary.txt, events.jsonl, scenario.yaml,
// weights.yaml) are written under dir. WriteRun never embeds wall-clock data
// in its output.
func WriteRun(dir string, r Result) error {
	if err := ensureDir(dir); err != nil {
		return err
	}

	if err := writeSummaryJSON(dir, r); err != nil {
		return fmt.Errorf("write summary.json: %w", err)
	}
	if err := writeSummaryTxt(dir, r); err != nil {
		return fmt.Errorf("write summary.txt: %w", err)
	}
	if err := writeEventsJSONL(dir, r.Events); err != nil {
		return fmt.Errorf("write events.jsonl: %w", err)
	}
	if err := writeFile(filepath.Join(dir, "scenario.yaml"), r.ScenarioBytes); err != nil {
		return fmt.Errorf("copy scenario.yaml: %w", err)
	}
	if err := writeFile(filepath.Join(dir, "weights.yaml"), r.WeightsBytes); err != nil {
		return fmt.Errorf("copy weights.yaml: %w", err)
	}
	return nil
}

// SummaryJSON returns the canonical summary.json bytes for r. Exposed for
// callers that need to also write the summary to stdout (`--format=json`).
func SummaryJSON(r Result) ([]byte, error) {
	doc := summaryDoc{
		Full:           r.Full,
		Warmup:         r.Warmup,
		WarmupSkipped:  r.WarmupSkipped,
		WallTicks:      r.WallTicks,
		Agents:         r.Agents,
		ScenarioSHA256: r.ScenarioSHA,
		WeightsSHA256:  r.WeightsSHA,
		StopReason:     r.StopReason,
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(&doc); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeSummaryJSON(dir string, r Result) error {
	b, err := SummaryJSON(r)
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(dir, "summary.json"), b)
}

// writeSummaryTxt renders a flat, key=value view of the full block plus the
// run-level identifiers. Lines are emitted in a fixed order.
func writeSummaryTxt(dir string, r Result) error {
	var b bytes.Buffer
	f := r.Full
	w := r.Warmup
	fmt.Fprintf(&b, "stop_reason          %s\n", r.StopReason)
	fmt.Fprintf(&b, "agents               %d\n", r.Agents)
	fmt.Fprintf(&b, "wall_ticks           %d\n", r.WallTicks)
	fmt.Fprintf(&b, "warmup_skipped       %t\n", r.WarmupSkipped)
	fmt.Fprintf(&b, "scenario_sha256      %s\n", r.ScenarioSHA)
	fmt.Fprintf(&b, "weights_sha256       %s\n", r.WeightsSHA)
	b.WriteString("\n[full]\n")
	writeBlockTxt(&b, f)
	b.WriteString("\n[warmup]\n")
	writeBlockTxt(&b, w)
	return writeFile(filepath.Join(dir, "summary.txt"), b.Bytes())
}

func writeBlockTxt(b *bytes.Buffer, blk metrics.Block) {
	fmt.Fprintf(b, "work_completed       %d\n", blk.WorkCompleted)
	fmt.Fprintf(b, "work_total           %d\n", blk.WorkTotal)
	fmt.Fprintf(b, "wall_ticks           %d\n", blk.WallTicks)
	fmt.Fprintf(b, "agent_idle_pct       %.6f\n", blk.AgentIdlePct)
	fmt.Fprintf(b, "agent_ticks_total    %d\n", blk.AgentTicksTotal)
	fmt.Fprintf(b, "rework_p50_wait      %d\n", blk.ReworkP50Wait)
	fmt.Fprintf(b, "rework_p95_wait      %d\n", blk.ReworkP95Wait)
	fmt.Fprintf(b, "top_of_queue_churn   %.6f\n", blk.TopOfQueueChurn)
	fmt.Fprintf(b, "goal_completion_1d   %d\n", blk.GoalCompletion1d)
	fmt.Fprintf(b, "goal_completion_3d   %d\n", blk.GoalCompletion3d)
	fmt.Fprintf(b, "goal_completion_7d   %d\n", blk.GoalCompletion7d)
	fmt.Fprintf(b, "priority_inversions  %d\n", blk.PriorityInversions)
	fmt.Fprintf(b, "area_collisions      %d\n", blk.AreaCollisions)
}

// writeEventsJSONL emits one JSON object per line in canonical order.
//
// Inputs are stable-sorted by (T, Kind, Agent, Bead, Top) so that two equal
// Result values produce byte-identical files even if the caller's slice
// happened to be in a different but ordering-equivalent permutation.
func writeEventsJSONL(dir string, events []EventEntry) error {
	sorted := make([]EventEntry, len(events))
	copy(sorted, events)
	sort.SliceStable(sorted, func(i, j int) bool {
		return eventLess(sorted[i], sorted[j])
	})

	var buf bytes.Buffer
	for _, e := range sorted {
		line, err := encodeEvent(e)
		if err != nil {
			return err
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	return writeFile(filepath.Join(dir, "events.jsonl"), buf.Bytes())
}

func eventLess(a, b EventEntry) bool {
	if a.T != b.T {
		return a.T < b.T
	}
	if ka, kb := kindPriority(a.Kind), kindPriority(b.Kind); ka != kb {
		return ka < kb
	}
	ai, bi := -1, -1
	if a.Agent != nil {
		ai = *a.Agent
	}
	if b.Agent != nil {
		bi = *b.Agent
	}
	if ai != bi {
		return ai < bi
	}
	if a.Bead != b.Bead {
		return a.Bead < b.Bead
	}
	return a.Top < b.Top
}

func kindPriority(k string) int {
	switch k {
	case "complete":
		return 0
	case "arrival":
		return 1
	case "dispatch":
		return 2
	case "queue_snapshot":
		return 3
	default:
		return 4
	}
}

// encodeEvent renders one EventEntry as a single line of JSON with stable
// key order ("t", "kind", then kind-specific keys).
func encodeEvent(e EventEntry) ([]byte, error) {
	// Use json.RawMessage assembly to fix key order.
	var b bytes.Buffer
	b.WriteByte('{')
	if err := writeKV(&b, "t", e.T); err != nil {
		return nil, err
	}
	b.WriteByte(',')
	if err := writeKV(&b, "kind", e.Kind); err != nil {
		return nil, err
	}
	switch e.Kind {
	case "dispatch":
		b.WriteByte(',')
		if err := writeKV(&b, "agent", agentValue(e.Agent)); err != nil {
			return nil, err
		}
		b.WriteByte(',')
		if err := writeKV(&b, "bead", e.Bead); err != nil {
			return nil, err
		}
		if e.Work != "" {
			b.WriteByte(',')
			if err := writeKV(&b, "work", e.Work); err != nil {
				return nil, err
			}
		}
	case "complete":
		b.WriteByte(',')
		if err := writeKV(&b, "bead", e.Bead); err != nil {
			return nil, err
		}
	case "arrival":
		b.WriteByte(',')
		if err := writeKV(&b, "bead", e.Bead); err != nil {
			return nil, err
		}
		if e.Work != "" {
			b.WriteByte(',')
			if err := writeKV(&b, "work", e.Work); err != nil {
				return nil, err
			}
		}
	case "queue_snapshot":
		b.WriteByte(',')
		if err := writeKV(&b, "top", e.Top); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown event kind %q", e.Kind)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

func agentValue(p *int) any {
	if p == nil {
		return 0
	}
	return *p
}

func writeKV(b *bytes.Buffer, key string, value any) error {
	kb, err := json.Marshal(key)
	if err != nil {
		return err
	}
	b.Write(kb)
	b.WriteByte(':')
	vb, err := json.Marshal(value)
	if err != nil {
		return err
	}
	b.Write(vb)
	return nil
}

// ensureDir creates dir if missing and confirms it is a directory.
func ensureDir(dir string) error {
	info, err := os.Stat(dir)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("output path %q exists and is not a directory", dir)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	return os.MkdirAll(dir, 0o755)
}

// writeFile writes b to path with 0o644 perms. Any existing file is replaced.
func writeFile(path string, b []byte) error {
	// Guard against accidental empty paths from upstream bugs.
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("writeFile: empty path")
	}
	return os.WriteFile(path, b, 0o644)
}
