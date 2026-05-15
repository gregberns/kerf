// Command kerfsim — `diff` subcommand.
//
// Compares two run directories produced by `kerfsim run`. Reports each
// numeric metric from the `full` block of `summary.json` side by side with
// a delta column. Does not re-execute scenarios.
//
// When either side is a `--runs N` aggregate directory (a directory whose
// immediate children are themselves run directories containing
// `summary.json`), `diff` reports median plus p10/p90 across the included
// runs. Both sides must have the same shape; mixing a single run with an
// aggregate is an error, as is comparing two aggregates of different sizes.
//
// Spec: specs/simulator.md §`kerfsim diff`, §Confidence Intervals.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gberns/kerf/internal/sim/metrics"
	"github.com/gberns/kerf/internal/sim/output"
)

// canonicalMetrics fixes the metric ordering used in diff output. Sorted
// alphabetically by JSON tag for determinism.
var canonicalMetrics = []metricSpec{
	{key: "agent_idle_pct", isFloat: true, extract: func(b metrics.Block) float64 { return b.AgentIdlePct }},
	{key: "agent_ticks_total", extract: func(b metrics.Block) float64 { return float64(b.AgentTicksTotal) }},
	{key: "area_collisions", extract: func(b metrics.Block) float64 { return float64(b.AreaCollisions) }},
	{key: "goal_completion_1d", extract: func(b metrics.Block) float64 { return float64(b.GoalCompletion1d) }},
	{key: "goal_completion_3d", extract: func(b metrics.Block) float64 { return float64(b.GoalCompletion3d) }},
	{key: "goal_completion_7d", extract: func(b metrics.Block) float64 { return float64(b.GoalCompletion7d) }},
	{key: "priority_inversions", extract: func(b metrics.Block) float64 { return float64(b.PriorityInversions) }},
	{key: "rework_p50_wait", extract: func(b metrics.Block) float64 { return float64(b.ReworkP50Wait) }},
	{key: "rework_p95_wait", extract: func(b metrics.Block) float64 { return float64(b.ReworkP95Wait) }},
	{key: "top_of_queue_churn", isFloat: true, extract: func(b metrics.Block) float64 { return b.TopOfQueueChurn }},
	{key: "wall_ticks", extract: func(b metrics.Block) float64 { return float64(b.WallTicks) }},
	{key: "work_completed", extract: func(b metrics.Block) float64 { return float64(b.WorkCompleted) }},
	{key: "work_total", extract: func(b metrics.Block) float64 { return float64(b.WorkTotal) }},
}

type metricSpec struct {
	key     string
	isFloat bool
	extract func(metrics.Block) float64
}

// sideStats holds one side's view of a single metric. For single-run
// comparisons, P10==P50==P90 and Count==1.
type sideStats struct {
	Count int     `json:"count"`
	P10   float64 `json:"p10"`
	P50   float64 `json:"p50"`
	P90   float64 `json:"p90"`
}

// diffRow is the per-metric diff payload. Used both for text rendering
// (via the human formatter) and for `--format=json` output.
type diffRow struct {
	Metric string    `json:"metric"`
	A      sideStats `json:"a"`
	B      sideStats `json:"b"`
	Delta  float64   `json:"delta"` // B.P50 - A.P50
}

// diffOutput is the top-level shape emitted under `--format=json`.
type diffOutput struct {
	A       string    `json:"a"`
	B       string    `json:"b"`
	ARuns   int       `json:"a_runs"`
	BRuns   int       `json:"b_runs"`
	Metrics []diffRow `json:"metrics"`
}

func newDiffCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "diff <runA-dir> <runB-dir>",
		Short: "Compare two kerfsim run directories.",
		Long: `Compare two run directories produced by kerfsim run.

When either side is a --runs N aggregate, diff reports median plus p10/p90
across the included runs. Both sides must have the same number of runs.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDiff(cmd.OutOrStdout(), args[0], args[1], format)
		},
	}
	cmd.Flags().StringVar(&format, "format", "text", "Output format: text or json")
	return cmd
}

func runDiff(out io.Writer, aDir, bDir, format string) error {
	aSummaries, err := loadSide(aDir)
	if err != nil {
		return fmt.Errorf("side A (%s): %w", aDir, err)
	}
	bSummaries, err := loadSide(bDir)
	if err != nil {
		return fmt.Errorf("side B (%s): %w", bDir, err)
	}
	if len(aSummaries) != len(bSummaries) {
		return fmt.Errorf("mismatched run counts: A has %d, B has %d", len(aSummaries), len(bSummaries))
	}

	rows := make([]diffRow, 0, len(canonicalMetrics))
	for _, m := range canonicalMetrics {
		aVals := extractAll(aSummaries, m.extract)
		bVals := extractAll(bSummaries, m.extract)
		aStats := computeStats(aVals)
		bStats := computeStats(bVals)
		rows = append(rows, diffRow{
			Metric: m.key,
			A:      aStats,
			B:      bStats,
			Delta:  bStats.P50 - aStats.P50,
		})
	}

	doc := diffOutput{
		A:       aDir,
		B:       bDir,
		ARuns:   len(aSummaries),
		BRuns:   len(bSummaries),
		Metrics: rows,
	}

	switch format {
	case "text", "":
		return renderText(out, doc)
	case "json":
		return renderJSON(out, doc)
	default:
		return fmt.Errorf("unknown --format %q (want \"text\" or \"json\")", format)
	}
}

// loadSide returns one or more parsed summaries for dir.
//
// dir is treated as a single-run directory when it contains summary.json
// directly. Otherwise dir is treated as an aggregate: each immediate
// subdirectory containing summary.json is loaded. Subdirectories without a
// summary.json are ignored so that runs/<name>/aggregate.json style sibling
// files do not confuse the loader.
//
// Aggregate child ordering is by directory name ascending so output is
// deterministic. (The order does not affect percentiles, which sort.)
func loadSide(dir string) ([]*output.Summary, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory")
	}

	// Single-run case.
	if _, err := os.Stat(filepath.Join(dir, "summary.json")); err == nil {
		s, err := output.ReadSummary(dir)
		if err != nil {
			return nil, err
		}
		return []*output.Summary{s}, nil
	}

	// Aggregate case.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		child := filepath.Join(dir, e.Name())
		if _, err := os.Stat(filepath.Join(child, "summary.json")); err != nil {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	if len(names) == 0 {
		return nil, fmt.Errorf("no summary.json found (neither single-run nor aggregate)")
	}
	out := make([]*output.Summary, 0, len(names))
	for _, n := range names {
		s, err := output.ReadSummary(filepath.Join(dir, n))
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func extractAll(summaries []*output.Summary, get func(metrics.Block) float64) []float64 {
	vs := make([]float64, len(summaries))
	for i, s := range summaries {
		vs[i] = get(s.Full)
	}
	return vs
}

// computeStats returns p10/p50/p90 of vs using the nearest-rank percentile
// method over the ascending-sorted values. For a single value all three
// percentiles equal that value.
func computeStats(vs []float64) sideStats {
	sorted := make([]float64, len(vs))
	copy(sorted, vs)
	sort.Float64s(sorted)
	return sideStats{
		Count: len(sorted),
		P10:   nearestRank(sorted, 0.10),
		P50:   nearestRank(sorted, 0.50),
		P90:   nearestRank(sorted, 0.90),
	}
}

// nearestRank returns the nearest-rank percentile of an already-sorted
// slice. For p in (0,1], the rank is ceil(p*n) clamped to [1,n] and the
// returned value is sorted[rank-1]. Panics are avoided by returning 0 for
// empty input — callers always pass non-empty slices in practice.
func nearestRank(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	rank := int(p*float64(n) + 0.9999999999) // ceil for p in (0,1]
	if rank < 1 {
		rank = 1
	}
	if rank > n {
		rank = n
	}
	return sorted[rank-1]
}

// renderText prints a fixed-width table. Single-run sides show one value;
// multi-run sides show "p50 (p10..p90)". Delta is always p50(B)-p50(A).
func renderText(out io.Writer, doc diffOutput) error {
	fmt.Fprintf(out, "A: %s  (runs=%d)\n", doc.A, doc.ARuns)
	fmt.Fprintf(out, "B: %s  (runs=%d)\n", doc.B, doc.BRuns)
	fmt.Fprintln(out)

	rows := make([][3]string, 0, len(doc.Metrics))
	for _, r := range doc.Metrics {
		rows = append(rows, [3]string{
			r.Metric,
			formatSide(r.A),
			formatSide(r.B),
		})
	}

	metricW, aW, bW := len("metric"), len("A"), len("B")
	for _, r := range rows {
		if len(r[0]) > metricW {
			metricW = len(r[0])
		}
		if len(r[1]) > aW {
			aW = len(r[1])
		}
		if len(r[2]) > bW {
			bW = len(r[2])
		}
	}

	fmt.Fprintf(out, "%-*s  %-*s  %-*s  %s\n", metricW, "metric", aW, "A", bW, "B", "delta")
	fmt.Fprintf(out, "%s  %s  %s  %s\n",
		strings.Repeat("-", metricW),
		strings.Repeat("-", aW),
		strings.Repeat("-", bW),
		strings.Repeat("-", 8),
	)
	for i, row := range rows {
		fmt.Fprintf(out, "%-*s  %-*s  %-*s  %s\n",
			metricW, row[0],
			aW, row[1],
			bW, row[2],
			formatDelta(doc.Metrics[i].Delta),
		)
	}
	return nil
}

func formatSide(s sideStats) string {
	if s.Count <= 1 {
		return trimNum(s.P50)
	}
	return fmt.Sprintf("%s (%s..%s)", trimNum(s.P50), trimNum(s.P10), trimNum(s.P90))
}

func formatDelta(d float64) string {
	s := trimNum(d)
	if d > 0 && !strings.HasPrefix(s, "+") {
		return "+" + s
	}
	return s
}

// trimNum renders a float as an integer when it has no fractional part,
// otherwise as %g (which strips trailing zeros). Keeps integer metrics
// readable without sacrificing precision on floats like agent_idle_pct.
func trimNum(v float64) string {
	if v == float64(int64(v)) {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}

func renderJSON(out io.Writer, doc diffOutput) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(&doc)
}
