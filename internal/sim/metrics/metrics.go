// Package metrics is the pure metric collector for kerfsim.
//
// A Collector consumes a stream of simulator events and structured per-event
// records (dispatch, arrival, complete, snapshot) and emits a Summary
// containing both a `full` and a `warmup` block.
//
// The collector has no dependency on the loop package. The thin adapter that
// forwards loop hooks into a Collector lives in bead 9b (`hooks.go`).
//
// All metric definitions and formulas trace back to specs/simulator.md
// §Metrics, §Metric Definitions, §Warmup Window.
//
// Design note: the warmup-window cutoff is a function of the final
// `wall_ticks`, so per-event metrics cannot be partitioned into full vs
// post-warmup blocks online. The collector records a small per-event ledger
// during the run and computes both blocks at Result() time.
package metrics

import (
	"math"
	"sort"

	"github.com/gregberns/kerf/internal/sim/event"
)

// Config configures a Collector for one run.
//
// NumAgents is the agent count of the run (≥1).
// TicksCap is the scenario `ticks` cap, used in the warmup-skipped predicate
// per spec (`wall_ticks < ticks * 0.2`).
// WorkTotal is the total number of works in the scenario; used as the
// denominator of `work_completed`.
// Deadline1d/3d/7d are the scenario tick deadlines for goal_completion_1d/3d/7d.
type Config struct {
	NumAgents  int
	TicksCap   int64
	WorkTotal  int
	Deadline1d int64
	Deadline3d int64
	Deadline7d int64
}

// DispatchInfo carries the structured payload for a dispatch record.
//
// IsNewWork is true when the selected bead is new-work (not rework).
// HadOlderRework is true when at least one rework bead was queue-eligible
// (dependencies met, not in-progress) with a lower arrival tick than the
// selected bead. Ties on arrival tick are broken by bead_id ascending — the
// caller is responsible for applying that tie-break before setting this flag.
//
// WorkCode and UnmetDeps are populated by the LoopHooks adapter for the
// benefit of the optional DebugSink stream (B14). They are not consumed by
// the Collector itself.
type DispatchInfo struct {
	Tick           int64
	AgentID        int
	BeadID         string
	Area           string
	WorkCode       string
	IsRework       bool
	IsNewWork      bool
	ArrivalTick    int64
	HadOlderRework bool
	UnmetDeps      []string
}

// CompleteInfo carries the structured payload for a completion record.
//
// WorkCompleted is true when this completion makes the bead's work terminal
// (drives work_completed and goal_completion_1d/3d/7d).
type CompleteInfo struct {
	Tick          int64
	AgentID       int
	BeadID        string
	Area          string
	WorkID        string
	WorkCompleted bool
}

// ArrivalInfo carries the structured payload for an arrival record.
//
// WorkCode and DependsOn are populated by the LoopHooks adapter for the
// benefit of the optional DebugSink stream (B14). They are not consumed by
// the Collector itself.
type ArrivalInfo struct {
	Tick      int64
	BeadID    string
	WorkCode  string
	IsRework  bool
	DependsOn []string
}

// DebugSink is an optional observer for the dispatch + arrival streams,
// used by `kerfsim run --debug-dispatch` to externalize per-event JSONL for
// diagnosing rework-metric anomalies (Plan 008 B14).
//
// All three methods are called from the same goroutine that drives the
// metrics collector. The Collector itself does not depend on DebugSink;
// the LoopHooks adapter routes events to it when non-nil.
type DebugSink interface {
	Header(scenarioSHA string, warmupCutoff int64, ticksCap int64, agents int)
	Arrival(a ArrivalInfo)
	Dispatch(d DispatchInfo, inWarmup bool)
}

// Block holds one view (full or warmup) of the computed metrics.
type Block struct {
	WorkCompleted      int     `json:"work_completed"`
	WorkTotal          int     `json:"work_total"`
	WallTicks          int64   `json:"wall_ticks"`
	AgentIdlePct       float64 `json:"agent_idle_pct"`
	AgentTicksTotal    int64   `json:"agent_ticks_total"`
	ReworkP50Wait      int64   `json:"rework_p50_wait"`
	ReworkP95Wait      int64   `json:"rework_p95_wait"`
	TopOfQueueChurn    float64 `json:"top_of_queue_churn"`
	GoalCompletion1d   int     `json:"goal_completion_1d"`
	GoalCompletion3d   int     `json:"goal_completion_3d"`
	GoalCompletion7d   int     `json:"goal_completion_7d"`
	PriorityInversions int     `json:"priority_inversions"`
	AreaCollisions     int     `json:"area_collisions"`
}

// Summary is the result of a Collector run.
//
// Per spec, summary.json carries both `full` and `warmup` blocks; when
// WarmupSkipped is true, Warmup == Full because no separation was possible.
type Summary struct {
	Full          Block `json:"full"`
	Warmup        Block `json:"warmup"`
	WarmupTicks   int64 `json:"warmup_ticks"`
	WarmupSkipped bool  `json:"warmup_skipped"`
}

// recordKind identifies an entry in the collector ledger.
type recordKind int

const (
	recArrival recordKind = iota
	recDispatch
	recComplete
	recSnapshot
	recObserve
)

// ledgerEntry is one row of the collector's internal event log. Stored in
// arrival order; replayed at Result() time once the warmup cutoff is known.
type ledgerEntry struct {
	kind     recordKind
	tick     int64
	dispatch DispatchInfo
	complete CompleteInfo
	arrival  ArrivalInfo
	top      string
}

// Collector is a pure consumer of the simulator's event stream.
//
// Use NewCollector to build one, then drive it with Observe and the Record*
// methods; call Result when the run ends to obtain the Summary.
type Collector struct {
	cfg      Config
	entries  []ledgerEntry
	lastTick int64
}

// NewCollector returns a fresh Collector configured for one run.
func NewCollector(cfg Config) *Collector {
	if cfg.NumAgents < 1 {
		cfg.NumAgents = 1
	}
	return &Collector{cfg: cfg}
}

// Observe is the generic event ingest. It advances wall_ticks and records a
// time marker in the ledger.
//
// Observe is safe to call with any event.Event regardless of kind.
func (c *Collector) Observe(e event.Event) {
	if e.Tick > c.lastTick {
		c.lastTick = e.Tick
	}
	c.entries = append(c.entries, ledgerEntry{kind: recObserve, tick: e.Tick})
}

// RecordArrival logs a bead arrival.
func (c *Collector) RecordArrival(a ArrivalInfo) {
	if a.Tick > c.lastTick {
		c.lastTick = a.Tick
	}
	c.entries = append(c.entries, ledgerEntry{kind: recArrival, tick: a.Tick, arrival: a})
}

// RecordDispatch logs a bead-dispatch decision.
func (c *Collector) RecordDispatch(d DispatchInfo) {
	if d.Tick > c.lastTick {
		c.lastTick = d.Tick
	}
	c.entries = append(c.entries, ledgerEntry{kind: recDispatch, tick: d.Tick, dispatch: d})
}

// RecordComplete logs a bead completion.
func (c *Collector) RecordComplete(co CompleteInfo) {
	if co.Tick > c.lastTick {
		c.lastTick = co.Tick
	}
	c.entries = append(c.entries, ledgerEntry{kind: recComplete, tick: co.Tick, complete: co})
}

// RecordSnapshot logs the top-of-queue bead after a mutating event.
//
// The snapshot is timestamped at the most recently observed tick; the
// caller is responsible for invoking RecordSnapshot after the matching
// arrival/dispatch/complete record on the same tick.
func (c *Collector) RecordSnapshot(top string) {
	c.entries = append(c.entries, ledgerEntry{kind: recSnapshot, tick: c.lastTick, top: top})
}

// Result computes the final Summary. Safe to call once at run-end.
func (c *Collector) Result() Summary {
	wall := c.lastTick
	skipped := c.warmupSkipped(wall)
	cut := c.warmupCutoff(wall)

	full := c.compute(false, wall, cut)

	var warmup Block
	if skipped {
		warmup = full
	} else {
		warmup = c.compute(true, wall, cut)
	}

	return Summary{
		Full:          full,
		Warmup:        warmup,
		WarmupTicks:   cut,
		WarmupSkipped: skipped,
	}
}

// warmupCutoff returns the inclusive last-tick of the warmup window per
// spec: `min(0.1 * ticks, 0.1 * wall_ticks)`. Events at tick > cutoff are
// post-warmup.
func (c *Collector) warmupCutoff(wall int64) int64 {
	a := float64(c.cfg.TicksCap) * 0.1
	b := float64(wall) * 0.1
	cut := a
	if b < a {
		cut = b
	}
	return int64(math.Floor(cut))
}

// warmupSkipped reports the fall-through condition from spec:
// `wall_ticks < ticks * 0.2`.
func (c *Collector) warmupSkipped(wall int64) bool {
	return float64(wall) < float64(c.cfg.TicksCap)*0.2
}

// compute walks the ledger and produces one block.
//
// When postOnly is true, the block is computed over the post-warmup window
// only; otherwise over the full run.
func (c *Collector) compute(postOnly bool, wall, cut int64) Block {
	// In-window predicate for a tick.
	inWindow := func(t int64) bool {
		if !postOnly {
			return true
		}
		return t > cut
	}

	// Per-bead state.
	reworkArrival := make(map[string]int64)
	var reworkWaits []int64

	// Time-segment accumulators: integrate (Δt * idle_count) and
	// (Δt * busy_count) over event boundaries inside the window.
	var idleAgentTicks int64
	var busyAgentTicks int64
	busy := 0
	prevTick := int64(-1)
	prevInit := false

	// Area state.
	areaActive := make(map[string]map[int]bool)
	overlapping := make(map[areaPair]bool)

	// Counters.
	workCompleted := 0
	goal1d, goal3d, goal7d := 0, 0, 0
	priorityInversions := 0
	areaCollisions := 0

	// Churn — per spec: first mutating event sets baseline, second-onward
	// changes count. Mutating events are arrival/dispatch/complete; the
	// caller drives RecordSnapshot after each. Use the snapshot stream.
	churnSeen := 0
	churnChange := 0
	churnPrev := ""

	// advanceTime integrates an interval [prev, t] for the current busy
	// count. The window-clipped overlap is integrated into the accumulators.
	advanceTime := func(t int64) {
		if !prevInit {
			prevTick = t
			prevInit = true
			return
		}
		if t < prevTick {
			return
		}
		delta := t - prevTick
		idleCnt := int64(c.cfg.NumAgents - busy)
		busyCnt := int64(busy)

		// Clip (prevTick, t] to the active window.
		lo, hi := prevTick, t
		if postOnly {
			if lo < cut {
				lo = cut
			}
		}
		if hi > lo {
			pd := hi - lo
			idleAgentTicks += pd * idleCnt
			busyAgentTicks += pd * busyCnt
		}
		_ = delta
		prevTick = t
	}

	for _, e := range c.entries {
		switch e.kind {
		case recObserve:
			advanceTime(e.tick)
		case recArrival:
			advanceTime(e.tick)
			if e.arrival.IsRework {
				reworkArrival[e.arrival.BeadID] = e.arrival.Tick
			}
		case recDispatch:
			advanceTime(e.tick)
			d := e.dispatch
			// Rework wait time.
			if d.IsRework {
				if arr, ok := reworkArrival[d.BeadID]; ok {
					wait := d.Tick - arr
					if inWindow(d.Tick) {
						reworkWaits = append(reworkWaits, wait)
					}
					delete(reworkArrival, d.BeadID)
				}
			}
			// Priority inversion.
			if d.IsNewWork && d.HadOlderRework && inWindow(d.Tick) {
				priorityInversions++
			}
			// Agent becomes busy.
			busy++
			// Area collision.
			if d.Area != "" {
				set := areaActive[d.Area]
				if set == nil {
					set = make(map[int]bool)
					areaActive[d.Area] = set
				}
				// Stable iteration: sort the other-agent IDs so collision
				// counts are deterministic across map-iteration order.
				others := make([]int, 0, len(set))
				for o := range set {
					others = append(others, o)
				}
				sort.Ints(others)
				for _, other := range others {
					a, b := d.AgentID, other
					if a > b {
						a, b = b, a
					}
					key := areaPair{Area: d.Area, A: a, B: b}
					if !overlapping[key] {
						overlapping[key] = true
						if inWindow(d.Tick) {
							areaCollisions++
						}
					}
				}
				set[d.AgentID] = true
			}
		case recComplete:
			advanceTime(e.tick)
			co := e.complete
			if busy > 0 {
				busy--
			}
			if co.Area != "" {
				if set, ok := areaActive[co.Area]; ok {
					delete(set, co.AgentID)
					// Forget overlapping pairs containing this agent on
					// this area; a future re-overlap counts as a new
					// collision per spec.
					for key := range overlapping {
						if key.Area != co.Area {
							continue
						}
						if key.A == co.AgentID || key.B == co.AgentID {
							delete(overlapping, key)
						}
					}
					if len(set) == 0 {
						delete(areaActive, co.Area)
					}
				}
			}
			if co.WorkCompleted {
				if inWindow(co.Tick) {
					workCompleted++
				}
				// goal_completion_* is reported on the full run; per spec
				// the deadlines are scenario-tick deadlines, independent
				// of the warmup window. We count on full-run only and
				// mirror into the warmup block via the same value.
				if co.Tick <= c.cfg.Deadline1d {
					goal1d++
				}
				if co.Tick <= c.cfg.Deadline3d {
					goal3d++
				}
				if co.Tick <= c.cfg.Deadline7d {
					goal7d++
				}
			}
		case recSnapshot:
			if !inWindow(e.tick) {
				continue
			}
			churnSeen++
			if churnSeen > 1 && e.top != churnPrev {
				churnChange++
			}
			churnPrev = e.top
		}
	}

	// Denominator wall: full uses `wall`; post uses `wall - cut`.
	var blockWall int64 = wall
	if postOnly {
		blockWall = wall - cut
		if blockWall < 0 {
			blockWall = 0
		}
	}
	denom := blockWall * int64(c.cfg.NumAgents)
	idlePct := 0.0
	if denom > 0 {
		idlePct = float64(idleAgentTicks) / float64(denom)
	}
	churn := 0.0
	if churnSeen > 1 {
		churn = float64(churnChange) / float64(churnSeen)
	}

	return Block{
		WorkCompleted:      workCompleted,
		WorkTotal:          c.cfg.WorkTotal,
		WallTicks:          blockWall,
		AgentIdlePct:       idlePct,
		AgentTicksTotal:    busyAgentTicks,
		ReworkP50Wait:      percentile(reworkWaits, 50),
		ReworkP95Wait:      percentile(reworkWaits, 95),
		TopOfQueueChurn:    churn,
		GoalCompletion1d:   goal1d,
		GoalCompletion3d:   goal3d,
		GoalCompletion7d:   goal7d,
		PriorityInversions: priorityInversions,
		AreaCollisions:     areaCollisions,
	}
}

// areaPair is the keyed form of a concurrent-overlap interval. The agent IDs
// are stored normalized (A < B) so the same unordered pair maps to one key.
type areaPair struct {
	Area string
	A, B int
}

// percentile returns the nearest-rank percentile of an int64 sample. p is in
// [0, 100]. Returns 0 on an empty sample. The function sorts a defensive
// copy so the caller's slice is not mutated, preserving determinism across
// callers that may hold references.
func percentile(xs []int64, p int) int64 {
	n := len(xs)
	if n == 0 {
		return 0
	}
	sorted := make([]int64, n)
	copy(sorted, xs)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	// nearest-rank: idx = ceil(p/100 * n) - 1
	rank := int(math.Ceil(float64(p)/100.0*float64(n))) - 1
	if rank < 0 {
		rank = 0
	}
	if rank >= n {
		rank = n - 1
	}
	return sorted[rank]
}
