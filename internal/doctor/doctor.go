// Package doctor implements the `kerf doctor` health check: a detector
// registry, the Finding/Severity types each detector emits, and the
// text + JSON output formatters.
//
// Spec references:
//   - specs/commands.md §"kerf doctor" — purpose, syntax, flags, behavior,
//     output (text and JSON), exit codes, errors.
//   - plans/017_storage_reconciliation/_plan.md §B5 — bead scope.
//
// This package is the scaffold that downstream beads kerf-7b4 / kerf-9jh
// / kerf-47z / kerf-kqn / kerf-7lq (the five detector beads) plug into.
// Each of those beads adds a single file in this package whose init()
// calls Register with a Detector value. The cmd-side caller (cmd/doctor.go)
// only sees Registered() and Run().
package doctor

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/gregberns/kerf/internal/storage"
)

// Severity is the spec's three-level rank for a single finding.
//
//   - green  — detector ran cleanly; no issue.
//   - yellow — warning; nothing is broken right now but action is owed.
//   - red    — blocker; normal use is impaired until resolved.
type Severity string

const (
	Green  Severity = "green"
	Yellow Severity = "yellow"
	Red    Severity = "red"
)

// Item is the per-target detail under a Finding. `Target` is typically a
// codename or a path; `Detail` is a one-line human description.
type Item struct {
	Target string `json:"target"`
	Detail string `json:"detail"`
}

// Finding is what a Detector returns: one severity-tagged row that maps
// 1:1 to a row in the text report and an entry in the JSON `findings`
// array (specs/commands.md §"kerf doctor" §"Output").
type Finding struct {
	Detector string   `json:"detector"`
	Severity Severity `json:"severity"`
	Summary  string   `json:"summary"`
	Items    []Item   `json:"items"`
	Hint     string   `json:"hint"`
}

// Context is the per-run context handed to each detector. It carries
// the resolved project id, the storage resolver (which already knows
// the active mode and all canonical paths), and the bench path. The
// scaffold deliberately keeps this small; downstream detector beads
// extend it only when they need to.
type Context struct {
	ProjectID string
	Resolver  *storage.Resolver
	BenchPath string
}

// Detector is the contract every detector implements. `ID` is the
// stable identifier the spec exposes via `--detector <id>` and the JSON
// `detector` field. `Run` returns one or more findings; an empty slice
// is treated as a single implicit green and is rendered by the
// formatter as a one-line summary per spec.
//
// Detectors do not exit, do not write to stdout, and do not mutate
// state — they return findings and let the formatter render and the
// caller decide exit semantics.
type Detector interface {
	ID() string
	Run(ctx *Context) ([]Finding, error)
}

// Registry holds the set of detectors known to the binary. It is the
// extension point for downstream beads B6-B10: each bead adds one
// init() that calls Register(&myDetector{}).
//
// Lookup-by-id and ordered iteration both happen often (the --detector
// flag does the former; the report renders in registration order), so
// we keep both an id->detector map and an insertion-ordered slice of
// ids.
type Registry struct {
	byID  map[string]Detector
	order []string
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{byID: make(map[string]Detector)}
}

// Register adds d to the registry. Re-registering the same id is a
// programmer error: panic so the duplicate is caught at process start.
func (r *Registry) Register(d Detector) {
	id := d.ID()
	if _, dup := r.byID[id]; dup {
		panic(fmt.Sprintf("doctor: detector %q registered twice", id))
	}
	r.byID[id] = d
	r.order = append(r.order, id)
}

// IDs returns the registered detector ids in registration order.
func (r *Registry) IDs() []string {
	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
}

// Get returns the detector with id name, or (nil, false).
func (r *Registry) Get(name string) (Detector, bool) {
	d, ok := r.byID[name]
	return d, ok
}

// Select returns the detectors named by ids, in the order ids appear.
// An unknown id returns an error referencing the spec's exact wording.
// When ids is empty, returns every registered detector in registration
// order — i.e., the default "all detectors" behaviour.
func (r *Registry) Select(ids []string) ([]Detector, error) {
	if len(ids) == 0 {
		out := make([]Detector, 0, len(r.order))
		for _, id := range r.order {
			out = append(out, r.byID[id])
		}
		return out, nil
	}
	out := make([]Detector, 0, len(ids))
	for _, id := range ids {
		d, ok := r.byID[id]
		if !ok {
			known := r.IDs()
			sort.Strings(known)
			return nil, fmt.Errorf("unknown detector '%s'. Known detectors: %s", id, strings.Join(known, ", "))
		}
		out = append(out, d)
	}
	return out, nil
}

// DefaultRegistry is the process-wide registry. Detector beads register
// here via init() (see triage's approach to command self-registration).
var DefaultRegistry = NewRegistry()

// Register adds d to the DefaultRegistry. Sugar for the common case.
func Register(d Detector) { DefaultRegistry.Register(d) }

// Report is the aggregated output of one doctor run.
type Report struct {
	ProjectID   string    `json:"project_id"`
	StorageMode string    `json:"storage_mode"`
	Findings    []Finding `json:"findings"`
}

// HasRed reports whether report contains any red finding. The strict
// exit-code policy keys off this (specs/commands.md §"kerf doctor"
// §"Exit codes").
func (r *Report) HasRed() bool {
	for _, f := range r.Findings {
		if f.Severity == Red {
			return true
		}
	}
	return false
}

// Run executes detectors against ctx, aggregates findings into a
// Report, and returns it. Detector errors abort the run — they signal
// an infrastructure problem the spec maps to exit 1 (bead store
// unreadable, etc.).
func Run(reg *Registry, ctx *Context, detectors []Detector) (*Report, error) {
	mode := ""
	if ctx.Resolver != nil {
		mode = string(ctx.Resolver.Mode)
	}
	rpt := &Report{
		ProjectID:   ctx.ProjectID,
		StorageMode: mode,
		Findings:    []Finding{},
	}
	for _, d := range detectors {
		fs, err := d.Run(ctx)
		if err != nil {
			return nil, fmt.Errorf("detector %q: %w", d.ID(), err)
		}
		for i := range fs {
			// Backfill detector id when the detector didn't set it,
			// so consumers don't need to. The detector remains free
			// to override (e.g., when splitting findings across
			// logical sub-detectors).
			if fs[i].Detector == "" {
				fs[i].Detector = d.ID()
			}
			if fs[i].Items == nil {
				fs[i].Items = []Item{}
			}
		}
		rpt.Findings = append(rpt.Findings, fs...)
	}
	return rpt, nil
}

// --- Formatters ----------------------------------------------------------

// RenderText writes the compact human-readable report.
//
// Per specs/commands.md §"kerf doctor" §"Output (default: compact text)":
//   - Header line names the project and active storage mode.
//   - Each detector emits one row tagged with its severity and a summary.
//   - Non-green rows are followed by an indented item list and a
//     `hint:` line naming the fix command.
//   - When every detector is green, the body collapses to one
//     summary line per detector and a final `All checks green.` line.
//
// `quiet` suppresses green findings (matches `--quiet` semantics).
func RenderText(out io.Writer, rpt *Report, quiet bool) error {
	mode := rpt.StorageMode
	if mode == "" {
		mode = "unknown"
	}
	if _, err := fmt.Fprintf(out, "kerf doctor — project: %s (%s mode)\n", rpt.ProjectID, mode); err != nil {
		return err
	}

	if len(rpt.Findings) == 0 {
		// No detectors registered (or none selected) — emit a single
		// informational line so the output is never empty.
		if _, err := fmt.Fprintln(out, "\nNo detectors registered."); err != nil {
			return err
		}
		return nil
	}

	allGreen := true
	for _, f := range rpt.Findings {
		if f.Severity != Green {
			allGreen = false
			break
		}
	}

	if _, err := fmt.Fprintln(out); err != nil {
		return err
	}
	for _, f := range rpt.Findings {
		if quiet && f.Severity == Green {
			continue
		}
		// Severity tag padded so columns align: green / yellow / red.
		tag := fmt.Sprintf("[%s]", f.Severity)
		// Pad to width 8 (longest is "[yellow]" = 8 chars).
		if len(tag) < 8 {
			tag = tag + strings.Repeat(" ", 8-len(tag))
		}
		if _, err := fmt.Fprintf(out, "%s %s\n", tag, f.Summary); err != nil {
			return err
		}
		if f.Severity != Green {
			for _, it := range f.Items {
				if it.Target != "" {
					if _, err := fmt.Fprintf(out, "         - %s: %s\n", it.Target, it.Detail); err != nil {
						return err
					}
				} else {
					if _, err := fmt.Fprintf(out, "         - %s\n", it.Detail); err != nil {
						return err
					}
				}
			}
			if f.Hint != "" {
				if _, err := fmt.Fprintf(out, "           hint: %s\n", f.Hint); err != nil {
					return err
				}
			}
		}
	}

	if allGreen {
		if _, err := fmt.Fprintln(out, "\nAll checks green."); err != nil {
			return err
		}
	}
	return nil
}

// RenderJSON writes the stable JSON shape documented in
// specs/commands.md §"kerf doctor" §"Output (--format=json)".
//
// `quiet` filters out green findings before encoding so the JSON shape
// matches what a `--quiet` consumer would see in text.
func RenderJSON(out io.Writer, rpt *Report, quiet bool) error {
	view := *rpt
	if quiet {
		filtered := make([]Finding, 0, len(rpt.Findings))
		for _, f := range rpt.Findings {
			if f.Severity == Green {
				continue
			}
			filtered = append(filtered, f)
		}
		view.Findings = filtered
	}
	if view.Findings == nil {
		view.Findings = []Finding{}
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(&view)
}
