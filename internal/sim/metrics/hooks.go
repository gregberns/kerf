// Hooks adapter: forwards loop.Hooks callbacks into a Collector.
//
// This is the only file in the kerfsim packages that imports both
// internal/sim/loop and internal/sim/metrics, so the loop ↔ metrics
// coupling stays isolated here (see plans/007_simulator/beads.md, B9b).
//
// The adapter is a thin pass-through. Where the loop.Hooks interface
// does not carry enough data for a Collector record (e.g. arrival
// tick, rework status, area, work-completion, older-rework
// eligibility for priority inversions), the adapter looks the value
// up in the store at hook-call time.
//
// See specs/simulator.md §Loop Mechanics and §Metric Definitions.

package metrics

import (
	"github.com/gberns/kerf/internal/beads"
	"github.com/gberns/kerf/internal/sim/event"
	"github.com/gberns/kerf/internal/sim/loop"
	"github.com/gberns/kerf/internal/sim/store"
)

// Compile-time check: *LoopHooks implements loop.Hooks.
var _ loop.Hooks = (*LoopHooks)(nil)

// LoopHooks adapts a *Collector to the loop.Hooks interface.
//
// Construct one with NewLoopHooks; pass it to loop.Loop.Hooks.
//
// The adapter is stateless beyond its references — every per-event
// fact it needs is read from C and Store at the moment of the
// callback. This keeps the run-time cost a small constant per hook.
type LoopHooks struct {
	C     *Collector
	Store *store.Store
	// Debug is an optional sink for the structured Dispatch/Arrival
	// streams. When non-nil, the adapter forwards every arrival and
	// dispatch record to Debug in addition to the Collector. Used by
	// `kerfsim run --debug-dispatch` for B14 diagnostics.
	Debug DebugSink
	// warmupCutoffHint is the conservative cutoff used to populate
	// in_warmup in the debug stream. Set by SetWarmupCutoffHint before
	// the run starts; zero disables the in_warmup tag.
	warmupCutoffHint int64
}

// SetWarmupCutoffHint installs a conservative warmup-cutoff used to flag
// debug-dispatch records with in_warmup. The hint is typically the
// `floor(0.1 * ticks_cap)` upper bound; the spec's true cutoff is the
// minimum of that and `floor(0.1 * wall_ticks)`, which the runtime cannot
// know until the run ends. Callers that want the true window can recompute
// it post-hoc from the JSONL header.
func (h *LoopHooks) SetWarmupCutoffHint(cut int64) { h.warmupCutoffHint = cut }

// NewLoopHooks constructs a LoopHooks adapter. Both arguments are required.
func NewLoopHooks(c *Collector, s *store.Store) *LoopHooks {
	return &LoopHooks{C: c, Store: s}
}

// OnEvent forwards a raw event into the Collector for wall-tick tracking.
func (h *LoopHooks) OnEvent(e event.Event) {
	h.C.Observe(e)
}

// OnArrival fires when the loop has inserted a new bead. We look up the
// freshly-arrived bead in the store to determine its rework status; the
// loop carries only the bead ID and work codename across the seam.
func (h *LoopHooks) OnArrival(beadID string, work string, tick int64) {
	isRework := h.lookupIsRework(beadID)
	var deps []string
	if st := h.Store.Lookup(beadID); st != nil {
		deps = append([]string(nil), st.DependsOn...)
	}
	info := ArrivalInfo{
		Tick:      tick,
		BeadID:    beadID,
		WorkCode:  work,
		IsRework:  isRework,
		DependsOn: deps,
	}
	h.C.RecordArrival(info)
	if h.Debug != nil {
		h.Debug.Arrival(info)
	}
}

// OnDispatch fires when the loop has just marked a bead in-progress for
// an agent. We look up the bead's arrival tick, rework status, area, and
// whether any older rework bead was queue-eligible at this tick (which
// drives the priority_inversions metric).
func (h *LoopHooks) OnDispatch(agentID int, beadID string, tick int64) {
	st := h.Store.Lookup(beadID)
	if st == nil {
		// Defensive: the loop only dispatches beads the policy returned,
		// which must exist in the store. Record what we can.
		h.C.RecordDispatch(DispatchInfo{
			Tick:    tick,
			AgentID: agentID,
			BeadID:  beadID,
		})
		return
	}
	bead := beadStateToBead(st)
	isRework := beads.IsRework(bead)
	area := h.areaFor(st.WorkCode)

	hadOlderRework := h.reworkEligibleAtDispatch(beadID, st.ArrivedAt)
	unmet := h.unmetDeps(st.DependsOn)

	info := DispatchInfo{
		Tick:           tick,
		AgentID:        agentID,
		BeadID:         beadID,
		Area:           area,
		WorkCode:       st.WorkCode,
		IsRework:       isRework,
		IsNewWork:      !isRework,
		ArrivalTick:    st.ArrivedAt,
		HadOlderRework: hadOlderRework,
		UnmetDeps:      unmet,
	}
	h.C.RecordDispatch(info)
	if h.Debug != nil {
		// in_warmup uses 0.1*TicksCap as a conservative upper bound (the
		// true cutoff = min(0.1*ticks, 0.1*wall_ticks) which can only
		// be smaller). This is the right direction for B14 diagnosis:
		// if a dispatch is NOT in this conservative window, it is
		// definitely not in the actual warmup window either.
		inWarmup := h.inConservativeWarmup(tick)
		h.Debug.Dispatch(info, inWarmup)
	}
}

// unmetDeps returns the subset of deps whose corresponding bead is not
// closed in the store. An unknown dep is treated as unmet — this mirrors
// production queue semantics and matches what reworkEligibleAtDispatch / the
// kerf policy use to gate eligibility.
func (h *LoopHooks) unmetDeps(deps []string) []string {
	if len(deps) == 0 {
		return nil
	}
	out := make([]string, 0)
	for _, d := range deps {
		st := h.Store.Lookup(d)
		if st == nil || st.Status != store.StatusClosed {
			out = append(out, d)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// inConservativeWarmup returns true if tick falls within the warmup-window
// upper bound derived from TicksCap alone (the runtime wall_ticks is
// unknown until run-end; using TicksCap yields the widest possible window
// per spec's min() definition).
//
// The cutoff is read off the Debug sink's last Header() call. If no header
// was emitted yet, the function returns false.
func (h *LoopHooks) inConservativeWarmup(tick int64) bool {
	if h.warmupCutoffHint <= 0 {
		return false
	}
	return tick <= h.warmupCutoffHint
}

// OnComplete fires when the loop has marked a bead closed. We look up the
// bead's work codename and decide whether this completion closed the
// final open bead of that work (WorkCompleted).
func (h *LoopHooks) OnComplete(beadID string, tick int64) {
	st := h.Store.Lookup(beadID)
	if st == nil {
		h.C.RecordComplete(CompleteInfo{
			Tick:   tick,
			BeadID: beadID,
		})
		return
	}
	area := h.areaFor(st.WorkCode)
	workCompleted := h.workTerminallyClosed(st.WorkCode)

	h.C.RecordComplete(CompleteInfo{
		Tick:          tick,
		AgentID:       st.AgentID,
		BeadID:        beadID,
		Area:          area,
		WorkID:        st.WorkCode,
		WorkCompleted: workCompleted,
	})
}

// OnSnapshot forwards the current top-of-queue bead ID into the
// Collector's churn ledger.
func (h *LoopHooks) OnSnapshot(top string) {
	h.C.RecordSnapshot(top)
}

// lookupIsRework returns the rework status of the named bead by inspecting
// its labels via beads.IsRework. Returns false if the bead is unknown.
func (h *LoopHooks) lookupIsRework(beadID string) bool {
	st := h.Store.Lookup(beadID)
	if st == nil {
		return false
	}
	return beads.IsRework(beadStateToBead(st))
}

// areaFor returns the dispatch-time "area" for the bead's work, which the
// collector uses to detect area_collisions. A work may carry zero or more
// areas in spec; for Phase-1 metric purposes we take the first area as the
// canonical key (consistent with one-area-per-collision counting). Returns
// "" if the work is unknown or carries no areas.
func (h *LoopHooks) areaFor(workCode string) string {
	if workCode == "" {
		return ""
	}
	for _, w := range h.Store.Works() {
		if w == nil {
			continue
		}
		if w.Codename != workCode {
			continue
		}
		if len(w.Areas) == 0 {
			return ""
		}
		return w.Areas[0]
	}
	return ""
}

// workTerminallyClosed reports whether every bead currently in the store
// that belongs to workCode is in the closed state. The loop calls this
// adapter's OnComplete *after* mutating the store, so the just-completed
// bead is already closed when this runs.
func (h *LoopHooks) workTerminallyClosed(workCode string) bool {
	if workCode == "" {
		return false
	}
	any := false
	for _, b := range h.Store.Beads() {
		if workCodeFromBead(b) != workCode {
			continue
		}
		any = true
		if b.Status != store.StatusClosed {
			return false
		}
	}
	return any
}

// reworkEligibleAtDispatch reports whether any rework bead other than
// `beadID` is queue-eligible (open, all deps closed) at the moment of
// dispatch.
//
// This drives priority_inversions: per specs/simulator.md §Metric
// Definitions, the metric counts dispatches of a new-work bead while
// ANY rework bead was queue-eligible — no arrival-tick comparison is
// applied. Earlier drafts required the rework bead to predate the
// dispatched new-work bead, but that comparison was structurally
// unreachable under the synthetic generator (initial new-work all
// arrives at tick 0, rework only at tick >= 1) and produced a metric
// stuck at zero. See plans/008_exploratory_testing/sim_scenarios/diagnosis.md
// and plans/011 pillar E.
//
// The `arrived` parameter is retained on the signature for compatibility
// with the previous semantics but is intentionally unused.
func (h *LoopHooks) reworkEligibleAtDispatch(beadID string, _ int64) bool {
	closed := make(map[string]bool)
	all := make([]*store.BeadState, 0)
	for _, b := range h.Store.Beads() {
		st := h.Store.Lookup(b.ID)
		if st == nil {
			continue
		}
		all = append(all, st)
		if st.Status == store.StatusClosed {
			closed[st.ID] = true
		}
	}
	for _, st := range all {
		if st.ID == beadID {
			continue
		}
		if st.Status != store.StatusOpen {
			continue
		}
		bead := beadStateToBead(st)
		if !beads.IsRework(bead) {
			continue
		}
		if !depsAllClosed(st.DependsOn, closed) {
			continue
		}
		return true
	}
	return false
}

// depsAllClosed reports whether every dependency in deps maps to a
// closed bead in the store. An unknown dep is treated as not closed
// (the bead is therefore not eligible) — matching production queue
// behavior where missing dependencies leave a bead blocked.
func depsAllClosed(deps []string, closed map[string]bool) bool {
	for _, d := range deps {
		if !closed[d] {
			return false
		}
	}
	return true
}

// beadStateToBead converts a store.BeadState pointer to the public
// beads.Bead view that beads.IsRework consumes. We re-derive this here
// rather than depend on a store accessor so the adapter stays additive
// to store.Store.
func beadStateToBead(st *store.BeadState) beads.Bead {
	return beads.Bead{
		ID:        st.ID,
		Title:     st.Title,
		Status:    st.Status,
		Epic:      st.Epic,
		Labels:    st.Labels,
		DependsOn: st.DependsOn,
	}
}

// workCodeFromBead extracts a work codename from a bead's `work:<codename>`
// label, or returns "" if none is present. Mirrors the helper of the same
// name in package loop; duplicated here to avoid a package-cycle on loop.
func workCodeFromBead(b beads.Bead) string {
	const prefix = "work:"
	for _, l := range b.Labels {
		if len(l) > len(prefix) && l[:len(prefix)] == prefix {
			return l[len(prefix):]
		}
	}
	return ""
}
