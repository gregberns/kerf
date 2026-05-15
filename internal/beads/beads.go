// Package beads provides integration with the beads task tracking system (br CLI).
//
// All interaction with the br binary is isolated in this package.
// No other part of kerf should shell out to br directly.
// When br is not available, functions degrade gracefully.
package beads

import (
	"encoding/json"
	"os/exec"
	"strings"
)

// Bead represents a single bead (task) from the br CLI.
type Bead struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Status    string   `json:"status"`
	Epic      string   `json:"epic"`
	Labels    []string `json:"labels"`
	DependsOn []string `json:"depends_on"`
}

// EpicSummary holds aggregated counts for beads within an epic.
type EpicSummary struct {
	Total      int
	Complete   int
	InProgress int
	Blocked    int
	Rework     int
}

// IsAvailable checks whether the br binary is on PATH.
func IsAvailable() bool {
	_, err := exec.LookPath("br")
	return err == nil
}

// List shells out to `br list --format json --all --limit 0` and parses the
// JSON output into a slice of Bead. Returns an empty slice (no error) if br
// is not available, keeping callers from needing to handle the missing-tool case.
func List() ([]Bead, error) {
	if !IsAvailable() {
		return nil, nil
	}

	out, err := exec.Command("br", "list", "--format", "json", "--all", "--limit", "0").Output()
	if err != nil {
		// br failed (bad config, no DB, etc.) -- degrade gracefully
		return nil, nil
	}

	return ParseJSON(out)
}

// ParseJSON parses br's JSON output into a slice of Bead.
// Exported so callers (and tests) can parse cached or piped output.
func ParseJSON(data []byte) ([]Bead, error) {
	var beads []Bead
	if err := json.Unmarshal(data, &beads); err != nil {
		return nil, err
	}
	return beads, nil
}

// CountByStatus groups beads by their Status field and returns a count map.
func CountByStatus(beads []Bead) map[string]int {
	counts := make(map[string]int)
	for _, b := range beads {
		counts[b.Status]++
	}
	return counts
}

// CountByEpic groups beads by their Epic field and returns an EpicSummary per epic.
func CountByEpic(beads []Bead) map[string]EpicSummary {
	epics := make(map[string]EpicSummary)
	for _, b := range beads {
		s := epics[b.Epic]
		s.Total++
		switch {
		case isComplete(b.Status):
			s.Complete++
		case isBlocked(b.Status):
			s.Blocked++
		case isInProgress(b.Status):
			s.InProgress++
		}
		epics[b.Epic] = s
	}
	return epics
}

// Available returns beads that are neither complete nor blocked.
// A bead is blocked if its status is "blocked".
// A bead is complete if its status is one of the recognized terminal statuses.
func Available(beads []Bead) []Bead {
	var result []Bead
	for _, b := range beads {
		if !isComplete(b.Status) && !isBlocked(b.Status) {
			result = append(result, b)
		}
	}
	return result
}

// IsRework reports whether the bead is tagged as rework. A bead is rework if
// any of its labels equals "rework:true" (case-insensitive) or begins with the
// "finding:" prefix (case-insensitive). The finding: prefix typically carries
// an attribution to the originating work (e.g. "finding:work-a").
func IsRework(b Bead) bool {
	for _, label := range b.Labels {
		if strings.EqualFold(label, "rework:true") {
			return true
		}
		if len(label) >= len("finding:") && strings.EqualFold(label[:len("finding:")], "finding:") {
			return true
		}
	}
	return false
}

// ReworkCount returns the number of beads in the slice that are tagged as rework.
func ReworkCount(beads []Bead) int {
	n := 0
	for _, b := range beads {
		if IsRework(b) {
			n++
		}
	}
	return n
}

// ForWork filters beads whose labels contain "work:<codename>".
//
// This is the back-compat entry point preserved from before Plan 006. It is
// intentionally case-insensitive to keep existing callers (cmd/next.go,
// cmd/show.go, cmd/square.go, cmd/map.go, internal/queue test helpers) and
// the historical TestForWork_CaseInsensitive contract behaving identically.
//
// Conceptually this is equivalent to applying Resolve(nil, nil) — the
// default filter "work:{codename}" — except that Filter.Match is
// case-sensitive per spec. Callers that need spec-conformant case-sensitive
// matching or a configured filter should use ForWorkWithFilter with an
// explicitly resolved *Filter.
//
// TODO(reviewer): confirm the case-sensitivity divergence between
// Filter.Match (case-sensitive per spec) and this wrapper (case-insensitive
// for back-compat) is acceptable until callers are migrated in later beads.
func ForWork(beads []Bead, workCodename string) []Bead {
	target := "work:" + workCodename
	var result []Bead
	for _, b := range beads {
		for _, label := range b.Labels {
			if strings.EqualFold(label, target) {
				result = append(result, b)
				break
			}
		}
	}
	return result
}

// --- status helpers ---

func isComplete(status string) bool {
	s := strings.ToLower(status)
	return s == "closed" || s == "done" || s == "complete"
}

func isBlocked(status string) bool {
	return strings.ToLower(status) == "blocked"
}

func isInProgress(status string) bool {
	s := strings.ToLower(status)
	return s == "in-progress" || s == "in_progress" || s == "active" || s == "wip"
}
