// Package queue computes work ordering for kerf next.
//
// The entire ordering algorithm lives in this file. Weights are named
// constants at the top so they are obvious and easy to tune.
package queue

import (
	"fmt"
	"sort"

	"github.com/gberns/kerf/internal/beads"
	"github.com/gberns/kerf/internal/dep"
	"github.com/gberns/kerf/internal/spec"
)

// Priority weights. These are initial values; expected to become configurable.
const (
	// WeightFanOut rewards works that unblock many downstream works.
	// Each direct or transitive dependent adds this much to the score.
	WeightFanOut = 10.0

	// WeightMomentum rewards works that are close to completion (high
	// completed/total bead ratio). A work at 100% momentum gets this
	// full value added to its score.
	WeightMomentum = 5.0

	// WeightCreation is a small tiebreaker that favors older works.
	// Each position from newest gets this added (index 0 = oldest in
	// the creation-sorted list).
	WeightCreation = 0.1
)

// Entry is a single item in the computed queue.
type Entry struct {
	Codename string
	Title    string
	Areas    []string
	Status   string
	Score    float64
	Reasons  []string // why it ranked here (human-readable)
}

// Compute returns an ordered list of actionable works, highest priority first.
//
// Algorithm:
//  1. Filter out works in terminal status or shelved.
//  2. Filter out works whose must-complete-first dependencies are not met.
//  3. Score remaining works by fan-out, momentum, and creation order.
//  4. Sort by score descending.
func Compute(works []*spec.SpecYAML, beadsByWork map[string]beads.EpicSummary) []Entry {
	if len(works) == 0 {
		return nil
	}

	// Build a lookup of all works by codename for dependency checks.
	workByName := make(map[string]*spec.SpecYAML, len(works))
	for _, w := range works {
		workByName[w.Codename] = w
	}

	// Step 1: Filter out terminal and shelved works.
	var active []*spec.SpecYAML
	for _, w := range works {
		if isTerminal(w) || isShelved(w) {
			continue
		}
		active = append(active, w)
	}

	// Step 2: Filter out works with unmet must-complete-first dependencies.
	var actionable []*spec.SpecYAML
	for _, w := range active {
		if hasUnmetDeps(w, workByName) {
			continue
		}
		actionable = append(actionable, w)
	}

	if len(actionable) == 0 {
		return nil
	}

	// Precompute the transitive fan-out for every work (how many other works
	// depend on it, directly or transitively).
	fanOut := computeFanOut(works)

	// Step 3 & 4: Score and sort.
	entries := make([]Entry, len(actionable))
	for i, w := range actionable {
		var score float64
		var reasons []string

		// Fan-out score.
		fo := fanOut[w.Codename]
		foScore := float64(fo) * WeightFanOut
		if fo > 0 {
			reasons = append(reasons, fmt.Sprintf("unblocks %d work(s) (+%.1f)", fo, foScore))
		}
		score += foScore

		// Momentum score.
		if summary, ok := beadsByWork[w.Codename]; ok && summary.Total > 0 {
			ratio := float64(summary.Complete) / float64(summary.Total)
			mScore := ratio * WeightMomentum
			score += mScore
			reasons = append(reasons, fmt.Sprintf("completion %d/%d (+%.1f)", summary.Complete, summary.Total, mScore))
		}

		// Creation order score: older works get a slight boost.
		// We sort actionable by creation time ascending, so index 0 = oldest.
		// The boost is (len - 1 - reverseIndex) * weight, meaning the oldest
		// gets the highest tiebreaker. We'll compute this after sorting by
		// creation time below.

		title := ""
		if w.Title != nil {
			title = *w.Title
		}

		entries[i] = Entry{
			Codename: w.Codename,
			Title:    title,
			Areas:    w.Areas,
			Status:   w.Status,
			Score:    score,
			Reasons:  reasons,
		}
	}

	// Sort actionable by creation time ascending to assign creation order scores.
	// We need a parallel sort of entries and actionable.
	type indexed struct {
		entry Entry
		work  *spec.SpecYAML
	}
	pairs := make([]indexed, len(entries))
	for i := range entries {
		pairs[i] = indexed{entry: entries[i], work: actionable[i]}
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		return pairs[i].work.Created.Before(pairs[j].work.Created)
	})

	// Apply creation order score: oldest (index 0) gets the highest boost.
	n := len(pairs)
	for i := range pairs {
		creationScore := float64(n-1-i) * WeightCreation
		pairs[i].entry.Score += creationScore
		if creationScore > 0 {
			pairs[i].entry.Reasons = append(pairs[i].entry.Reasons,
				fmt.Sprintf("creation order (+%.1f)", creationScore))
		}
	}

	// Extract entries and sort by score descending (stable to preserve creation order ties).
	for i := range pairs {
		entries[i] = pairs[i].entry
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Score > entries[j].Score
	})

	return entries
}

// isTerminal returns true if the work's status is at or past its terminal status.
func isTerminal(w *spec.SpecYAML) bool {
	return dep.IsComplete(w.Status, w.StatusValues)
}

// isShelved returns true if the work's status is "shelved".
func isShelved(w *spec.SpecYAML) bool {
	return w.Status == "shelved"
}

// hasUnmetDeps returns true if any must-complete-first dependency is not in
// terminal status. Only checks dependencies that can be resolved (i.e., exist
// in the provided works map).
func hasUnmetDeps(w *spec.SpecYAML, workByName map[string]*spec.SpecYAML) bool {
	for _, d := range w.DependsOn {
		if d.Relationship != "must-complete-first" {
			continue
		}
		depWork, ok := workByName[d.Codename]
		if !ok {
			// Unresolvable dependency — skip per spec (does not block).
			continue
		}
		if !dep.IsComplete(depWork.Status, depWork.StatusValues) {
			return true
		}
	}
	return false
}

// computeFanOut returns, for each codename, how many other works depend on it
// (directly or transitively via must-complete-first).
func computeFanOut(works []*spec.SpecYAML) map[string]int {
	// Build a reverse adjacency list: for each codename, which works depend on it?
	directDeps := make(map[string][]string) // depCodename -> []dependentCodename
	for _, w := range works {
		for _, d := range w.DependsOn {
			if d.Relationship == "must-complete-first" {
				directDeps[d.Codename] = append(directDeps[d.Codename], w.Codename)
			}
		}
	}

	// For each work, compute transitive dependents via BFS.
	result := make(map[string]int, len(works))
	for _, w := range works {
		visited := make(map[string]bool)
		queue := []string{w.Codename}
		for len(queue) > 0 {
			curr := queue[0]
			queue = queue[1:]
			for _, dependent := range directDeps[curr] {
				if !visited[dependent] {
					visited[dependent] = true
					queue = append(queue, dependent)
				}
			}
		}
		result[w.Codename] = len(visited)
	}

	return result
}
