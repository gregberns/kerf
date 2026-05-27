// Package store provides an in-memory bead/work store for the kerfsim
// simulator. It mirrors the shape that `br list --format json` produces and
// exposes adapter methods that feed `queue.Compute` directly, with no further
// transformation.
//
// See specs/simulator.md §Relationship to kerf and §Loop Mechanics (Dispatch).
//
// The store is the simulator's single source of truth for run-time bead state.
// The simulator loop mutates it via Dispatch / Complete / Arrive; both the
// kerf policy (via queue.Compute) and the baseline policies read from it via
// Works() and SummaryByWork().
package store

import (
	"time"

	"github.com/gregberns/kerf/internal/beads"
	"github.com/gregberns/kerf/internal/sim/generator"
	"github.com/gregberns/kerf/internal/spec"
)

// Bead status constants. The store tracks status as an enum so that mutations
// (Dispatch / Complete) are unambiguous and so that SummaryByWork can map back
// to the beads.EpicSummary buckets that queue.Compute consumes.
const (
	StatusOpen       = "open"
	StatusInProgress = "in-progress"
	StatusClosed     = "closed"
)

// BeadState is the per-bead run-time state held by the store. It mirrors the
// fields of internal/beads.Bead plus simulator-only bookkeeping (arrival_tick,
// dispatch/complete ticks, owning agent).
type BeadState struct {
	ID         string
	Title      string
	Epic       string
	Labels     []string
	DependsOn  []string
	Status     string
	WorkCode   string // work codename this bead belongs to (mirrors `work:<codename>` label)
	ArrivedAt  int64  // simulation tick at which the bead entered the store
	DispatchAt int64  // tick at which Dispatch was last called (0 if never)
	CompleteAt int64  // tick at which Complete was called (0 if never)
	AgentID    int    // dispatching agent (0 if never dispatched)
}

// toBead returns the beads.Bead view of this state used by br-list-shaped
// consumers (e.g. CountByEpic, ForWork).
func (b *BeadState) toBead() beads.Bead {
	labels := make([]string, len(b.Labels))
	copy(labels, b.Labels)
	deps := make([]string, len(b.DependsOn))
	copy(deps, b.DependsOn)
	return beads.Bead{
		ID:        b.ID,
		Title:     b.Title,
		Status:    b.Status,
		Epic:      b.Epic,
		Labels:    labels,
		DependsOn: deps,
	}
}

// Store holds the simulator's in-memory works + beads. Adapter methods
// (Works / SummaryByWork) read current state; mutation methods (Dispatch /
// Complete / Arrive) advance it. Stores returned from From are mutation-
// isolated from each other: B10 uses this to run multiple policies against the
// same generated world without cross-contamination.
type Store struct {
	works      []*spec.SpecYAML
	workOrder  []string         // canonical codename ordering (insertion order)
	beadsByID  map[string]*BeadState
	beadOrder  []string         // insertion order for deterministic iteration
	durations  map[string]int64 // pre-rolled durations per bead, in ticks
}

// New returns an empty store.
func New() *Store {
	return &Store{
		beadsByID: make(map[string]*BeadState),
		durations: make(map[string]int64),
	}
}

// AddWork inserts a work into the store. The first insertion of a given
// codename wins; subsequent inserts of the same codename update the entry in
// place (the canonical ordering does not change).
func (s *Store) AddWork(w *spec.SpecYAML) {
	if w == nil {
		return
	}
	// If this codename is already present, replace it in place to keep the
	// canonical work order stable.
	for i, existing := range s.works {
		if existing.Codename == w.Codename {
			s.works[i] = w
			return
		}
	}
	s.works = append(s.works, w)
	s.workOrder = append(s.workOrder, w.Codename)
}

// Works returns a snapshot of the current works in canonical order. The slice
// header is freshly allocated; the *spec.SpecYAML pointers are shared. Callers
// must not mutate the returned works.
//
// This is one half of the tuple consumed by queue.Compute.
func (s *Store) Works() []*spec.SpecYAML {
	out := make([]*spec.SpecYAML, len(s.works))
	copy(out, s.works)
	return out
}

// SummaryByWork returns a per-work bead summary keyed by work codename. The
// result has the same shape that beads.CountByEpic produces, except keyed by
// work codename (the simulator treats one work as one "epic"). Rework counts
// reflect beads.IsRework over the union of open/in-progress beads (closed
// beads' rework-ness is not re-counted, matching production behavior).
//
// This is the second half of the tuple consumed by queue.Compute.
func (s *Store) SummaryByWork() map[string]beads.EpicSummary {
	out := make(map[string]beads.EpicSummary, len(s.workOrder))
	// Seed every known work, so works with zero beads still appear.
	for _, code := range s.workOrder {
		out[code] = beads.EpicSummary{}
	}
	for _, id := range s.beadOrder {
		b := s.beadsByID[id]
		code := b.WorkCode
		if code == "" {
			continue
		}
		summary := out[code]
		summary.Total++
		switch b.Status {
		case StatusClosed:
			summary.Complete++
		case StatusInProgress:
			summary.InProgress++
		}
		// Rework: count beads matching beads.IsRework that are not closed.
		if b.Status != StatusClosed && beads.IsRework(b.toBead()) {
			summary.Rework++
		}
		out[code] = summary
	}
	return out
}

// Beads returns a snapshot of all current beads (in beads.Bead form) in
// insertion order. Useful for baseline policies that iterate beads directly
// rather than through queue.Compute. The returned slice and its inner slices
// (Labels, DependsOn) are freshly allocated.
func (s *Store) Beads() []beads.Bead {
	out := make([]beads.Bead, 0, len(s.beadOrder))
	for _, id := range s.beadOrder {
		out = append(out, s.beadsByID[id].toBead())
	}
	return out
}

// Lookup returns the current state of a bead by ID, or nil if absent. The
// returned pointer is owned by the store; callers must not mutate it.
func (s *Store) Lookup(beadID string) *BeadState {
	return s.beadsByID[beadID]
}

// Arrive inserts a new bead into the store at the given tick. If a bead with
// the same ID already exists, Arrive is a no-op (arrivals are idempotent;
// duplicate arrival events from the heap should not cause double-counting).
// The bead's WorkCode is inferred from a `work:<codename>` label if not
// already set.
func (s *Store) Arrive(b beads.Bead, tick int64) {
	if _, ok := s.beadsByID[b.ID]; ok {
		return
	}
	state := &BeadState{
		ID:        b.ID,
		Title:     b.Title,
		Epic:      b.Epic,
		Labels:    append([]string(nil), b.Labels...),
		DependsOn: append([]string(nil), b.DependsOn...),
		Status:    StatusOpen,
		WorkCode:  workCodeFromLabels(b.Labels),
		ArrivedAt: tick,
	}
	// If status was supplied non-empty in the input bead, honor it.
	if b.Status != "" {
		state.Status = b.Status
	}
	s.beadsByID[b.ID] = state
	s.beadOrder = append(s.beadOrder, b.ID)
}

// Dispatch marks a bead as in-progress, owned by agentID, at the given tick.
// It is a no-op (no error) if the bead is unknown — the loop is responsible
// for only dispatching beads the policy returned, which by construction must
// exist in this store.
func (s *Store) Dispatch(beadID string, agentID int, tick int64) {
	b, ok := s.beadsByID[beadID]
	if !ok {
		return
	}
	b.Status = StatusInProgress
	b.DispatchAt = tick
	b.AgentID = agentID
}

// Complete marks a bead as closed at the given tick.
func (s *Store) Complete(beadID string, tick int64) {
	b, ok := s.beadsByID[beadID]
	if !ok {
		return
	}
	b.Status = StatusClosed
	b.CompleteAt = tick
}

// SetDuration records the pre-rolled completion duration (in ticks) for a
// bead. Durations are pre-rolled at scenario creation (see specs/simulator.md
// §Pre-rolled Durations) and looked up by the loop when scheduling a
// completion event after dispatch.
func (s *Store) SetDuration(beadID string, ticks int64) {
	if s.durations == nil {
		s.durations = make(map[string]int64)
	}
	s.durations[beadID] = ticks
}

// Duration returns the pre-rolled duration for a bead in ticks. If no
// duration has been recorded, it returns 0 — the loop treats a zero
// duration as "completes on the same tick it was dispatched".
func (s *Store) Duration(beadID string) int64 {
	if s.durations == nil {
		return 0
	}
	return s.durations[beadID]
}

// AllClosed reports whether every bead in the store is in the closed state.
// Returns true vacuously when the store has no beads.
func (s *Store) AllClosed() bool {
	for _, id := range s.beadOrder {
		if s.beadsByID[id].Status != StatusClosed {
			return false
		}
	}
	return true
}


// From returns a fresh Store seeded from the given generator.GeneratedWorld.
// Each call returns a mutually-independent store: mutating one store returned
// from From(w) does not affect another store returned from From(w). This
// isolation is load-bearing for the run orchestrator (B10), which runs four
// policies against the same world.
//
// From synthesizes a *spec.SpecYAML for each GeneratedWork (the generator's
// world carries the structural fields — codename, areas, deps, epic — but not
// the spec scaffolding queue.Compute consumes). The synthesized specs share
// a fixed Created baseline derived from the work's WorkCreatedTick so two
// stores constructed from the same world have byte-identical work specs.
//
// Initial beads (those with ArrivalTick == 0 in the generated world) are
// inserted at tick 0; future arrivals (ArrivalTick > 0) are NOT inserted —
// the orchestrator drives them onto the loop's event heap. Durations for
// every bead (initial AND future) are recorded so the loop can look them up
// on dispatch regardless of arrival timing.
func From(world *generator.GeneratedWorld) *Store {
	s := New()
	if world == nil {
		return s
	}
	for _, gw := range world.Works {
		s.AddWork(workToSpec(gw))
	}
	for _, gb := range world.Beads {
		s.SetDuration(gb.BeadID, gb.Duration)
		if gb.ArrivalTick != 0 {
			continue
		}
		s.Arrive(generatedBeadToBead(gb), 0)
	}
	return s
}

// FromSpecs returns a fresh Store containing the given works and initial
// beads. It is the lower-level constructor used by tests that need precise
// control over spec fields (Created timestamps, status_values, etc.) without
// going through the synthetic generator. Initial beads are inserted at the
// supplied tick.
func FromSpecs(works []*spec.SpecYAML, initial []beads.Bead) *Store {
	s := New()
	for _, w := range works {
		s.AddWork(w)
	}
	for _, b := range initial {
		s.Arrive(b, 0)
	}
	return s
}

// generatorEpoch is the canonical Created timestamp used as the baseline for
// synthesized specs. It is fixed so two stores built from the same world have
// byte-identical work timestamps.
var generatorEpoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// workToSpec synthesizes a *spec.SpecYAML from a generator.GeneratedWork. The
// fields populated here are exactly what queue.Compute, baseline policies,
// and the metrics collector read off the spec: Codename, Type, Status,
// StatusValues, Created, Updated, Areas.
//
// Dependencies on the spec are recorded as DependsOn entries with empty
// status restriction (matching queue.Compute's must-complete-first
// semantics: an empty restriction means "any non-terminal status").
func workToSpec(w generator.GeneratedWork) *spec.SpecYAML {
	created := generatorEpoch.Add(time.Duration(w.WorkCreatedTick) * time.Second)
	deps := make([]spec.Dependency, 0, len(w.Deps))
	for _, d := range w.Deps {
		dep := d
		deps = append(deps, spec.Dependency{
			Codename:     dep,
			Relationship: "must-complete-first",
		})
	}
	return &spec.SpecYAML{
		Codename:     w.Codename,
		Type:         "feature",
		Status:       "in-progress",
		StatusValues: []string{"design", "in-progress", "review", "complete"},
		Created:      created,
		Updated:      created,
		Areas:        append([]string(nil), w.Areas...),
		DependsOn:    deps,
	}
}

// generatedBeadToBead lifts a generator.GeneratedBead into the beads.Bead
// form Arrive consumes. The bead is tagged with `work:<codename>` so the
// store's WorkCode inference works without any caller bookkeeping.
func generatedBeadToBead(gb generator.GeneratedBead) beads.Bead {
	labels := append([]string{"work:" + gb.Work}, gb.Labels...)
	return beads.Bead{
		ID:     gb.BeadID,
		Title:  gb.BeadID,
		Status: "open",
		Epic:   gb.Work,
		Labels: labels,
	}
}

// workCodeFromLabels returns the work codename encoded in a `work:<codename>`
// label, or "" if none is present. This is how br/beads identify work
// affiliation in production; the simulator follows the same convention so the
// store's shape matches `br list --format json`.
func workCodeFromLabels(labels []string) string {
	const prefix = "work:"
	for _, l := range labels {
		if len(l) > len(prefix) && l[:len(prefix)] == prefix {
			return l[len(prefix):]
		}
	}
	return ""
}

