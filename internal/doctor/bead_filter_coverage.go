// Package doctor — bead-filter-coverage detector.
//
// Spec reference:
//   - specs/commands.md §"kerf doctor" §"Behavior" item `bead-filter-coverage`
//     (line 1572): "reports each active work whose `bead_filter`
//     resolves to zero beads, labelled by the rank-label vocabulary
//     documented under `kerf next` (`empty` or `unwired` today; `broken`
//     lands when parser support arrives — see plan 019 OQ5). The hint
//     for an unwired work names the filter-bootstrap entry point (see
//     [plan 019])."
//   - specs/coordination.md §"Rank Labels for Zero-Match Works" —
//     `unwired` / `empty` / `broken` vocabulary owned by Plan 019 / B2
//     (bead kerf-mgx).
//   - plans/017_storage_reconciliation/_plan.md §B10 — bead scope.
//
// The same rank-label classification rule lives in
// internal/feed/cleanup.go's detectWorkNoAttachedBeads (Plan 019 / B2
// landing site). Today the rule is three lines (BeadFilter == nil →
// unwired; zero matches → empty; broken is not yet observable — spec.Read
// rejects malformed filters at parse time per Plan 019 OQ5). Re-stating
// it here rather than calling into internal/feed keeps the doctor
// package's import graph narrow (it would otherwise have to depend on
// the feed/queue stack) and matches the precedent set by the other
// detectors in this package. If the vocabulary grows non-trivially,
// extract the classifier into internal/beads as a shared helper.

package doctor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/gberns/kerf/internal/beads"
	"github.com/gberns/kerf/internal/config"
	"github.com/gberns/kerf/internal/spec"
)

// beadFilterCoverageDetector is the registered Detector for
// "bead-filter-coverage".
type beadFilterCoverageDetector struct{}

func (beadFilterCoverageDetector) ID() string { return "bead-filter-coverage" }

// Hint text. Plan 019 (filter-bootstrap) owns `kerf bootstrap-filters`;
// `kerf work edit --bead-filter-add` is the per-work editor (already
// landed). Both are named so an agent can pick the appropriate one
// without re-reading the spec.
const beadFilterCoverageHint = "kerf bootstrap-filters  (project-wide suggestion) or kerf work edit <codename> --bead-filter-add '<clause>'"

// beadStoreUnavailableHint points operators at the configuration knob and
// the per-tool bootstrap step for the canonical "bead tool on PATH but
// failing" case (bead kerf-pq5).
const beadStoreUnavailableHint = "check kerf config tools.tasks; ensure the configured tool is initialized (e.g. 'bd init' or 'br init') and its JSON output is current"

func (beadFilterCoverageDetector) Run(ctx *Context) ([]Finding, error) {
	if ctx == nil || ctx.Resolver == nil {
		return nil, fmt.Errorf("bead-filter-coverage: nil context or resolver")
	}
	r := ctx.Resolver

	codenames, err := r.ListWorks()
	if err != nil {
		return nil, fmt.Errorf("bead-filter-coverage: listing works: %w", err)
	}
	sort.Strings(codenames)

	// Project-wide bead_filter + tool selection.
	var projectFilter *beads.Filter
	toolName := beads.DefaultToolName
	if cfg, cerr := config.LoadProjectConfig(r.ProjectConfigPath()); cerr == nil && cfg != nil {
		projectFilter = cfg.BeadFilter
		toolName = beads.ResolveToolName(cfg.Tools)
	}

	// Read the bead store once. A bead-tool subprocess failure
	// (beads.ToolError: configured binary is on PATH but exits non-zero,
	// emits malformed JSON, etc.) is degraded to a RED finding rather
	// than propagated up to Run() — propagating would kill the whole
	// `kerf doctor` command and prevent other detectors from running.
	// See bead kerf-pq5 (BLOCKER from dogfood test 2026-05-18): the
	// canonical repro is `br` returning `JSON_ERROR: missing field
	// jsonl_export`, which used to crash `kerf doctor` outright. Other
	// error categories (tool absent → (nil, nil); unexpected error
	// types) still short-circuit normally. The indirection through
	// beadFilterCoverageLoader is a test seam — production uses
	// beads.ListNamed; tests can substitute a stub returning a
	// beads.ToolError to exercise the degraded-finding path.
	allBeads, err := beadFilterCoverageLoader(toolName)
	if err != nil {
		var toolErr *beads.ToolError
		if errors.As(err, &toolErr) {
			return []Finding{{
				Severity: Red,
				Summary:  fmt.Sprintf("bead store unavailable: %s failed", toolErr.Tool),
				Items: []Item{{
					Target: toolErr.Tool,
					Detail: toolErr.Error(),
				}},
				Hint: beadStoreUnavailableHint,
			}}, nil
		}
		return nil, fmt.Errorf("bead-filter-coverage: reading bead store: %w", err)
	}

	totalWorks := len(codenames)
	var unwired []Item
	var empty []Item

	for _, codename := range codenames {
		specPath := filepath.Join(r.WorkDir(codename), "spec.yaml")
		// Missing spec.yaml is not this detector's concern (storage-drift
		// surfaces work-dir issues); skip silently.
		if _, statErr := os.Stat(specPath); statErr != nil {
			continue
		}
		s, rerr := spec.Read(specPath)
		if rerr != nil {
			// Malformed spec.yaml: a different detector's beat (and
			// spec.Read already rejects malformed bead_filters). Surface
			// as an item under "empty" so the operator knows the work
			// can't be filter-resolved cleanly, but don't fail the run.
			empty = append(empty, Item{
				Target: codename,
				Detail: fmt.Sprintf("spec.yaml unreadable: %v", rerr),
			})
			continue
		}

		// Classification — mirrors internal/feed/cleanup.go
		// detectWorkNoAttachedBeads (see header comment).
		if s.BeadFilter == nil {
			unwired = append(unwired, Item{
				Target: codename,
				Detail: "no bead_filter declared on spec.yaml",
			})
			continue
		}
		resolved := beads.Resolve(s.BeadFilter, projectFilter)
		matched := beads.ForWorkWithFilter(allBeads, codename, resolved)
		if len(matched) == 0 {
			empty = append(empty, Item{
				Target: codename,
				Detail: "bead_filter present but resolves to zero beads in the store",
			})
		}
	}

	// All works wired and matching at least one bead → single green.
	if totalWorks == 0 {
		return []Finding{{
			Severity: Green,
			Summary:  "bead_filter coverage: no active works",
		}}, nil
	}
	if len(unwired) == 0 && len(empty) == 0 {
		return []Finding{{
			Severity: Green,
			Summary:  fmt.Sprintf("bead_filter coverage: %d of %d works wired", totalWorks, totalWorks),
		}}, nil
	}

	var findings []Finding
	if len(unwired) > 0 {
		findings = append(findings, Finding{
			Severity: Red,
			Summary:  fmt.Sprintf("bead_filter coverage: %d of %d works unwired", len(unwired), totalWorks),
			Items:    unwired,
			Hint:     beadFilterCoverageHint,
		})
	}
	if len(empty) > 0 {
		findings = append(findings, Finding{
			Severity: Yellow,
			Summary:  fmt.Sprintf("bead_filter coverage: %d of %d works with empty filter (resolve to zero beads)", len(empty), totalWorks),
			Items:    empty,
			Hint:     beadFilterCoverageHint,
		})
	}
	return findings, nil
}

// beadFilterCoverageLoader is the bead-store loader the detector uses.
// Defaults to beads.ListNamed; overridable by tests to inject a fixed
// bead set without depending on a real bd binary on PATH.
var beadFilterCoverageLoader = beads.ListNamed

func init() { Register(beadFilterCoverageDetector{}) }
