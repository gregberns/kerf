// Package doctor — archive-orphans detector.
//
// Spec reference:
//   - specs/commands.md §"kerf doctor" §"Behavior" item `archive-orphans`
//     (line 1573): "reports any
//     `~/.kerf/archive/<project-id>/<codename>/` entry whose codename
//     also appears as a live work in the project."
//   - specs/commands.md §"kerf doctor" §"Output" (line 1592): the green
//     summary form is `archive: N entry, no live collisions`.
//   - plans/017_storage_reconciliation/_plan.md beads outline item 9 —
//     bead scope; the storage-drift detector (kerf-9jh) already includes
//     archive/live collisions among its five finding classes, but the
//     dedicated `archive-orphans` row exists so users can `--detector
//     archive-orphans` and get a focused green/yellow signal on archive
//     hygiene without the noise of the broader drift scan.
//
// The detector enumerates `~/.kerf/archive/<project-id>/`; for each
// archived work, it checks whether the same codename exists as a live
// work under the canonical works directory. When a collision is found
// the finding is yellow (matching storage-drift class 5 severity): the
// archive is intact, but the next attempt to restore the archived work
// will clash, and `kerf new <codename>` against the live work is
// already ambiguous.
//
// When no archive exists, the detector emits a single green finding
// rather than skipping silently — so the doctor report always has a
// row for archive hygiene.

package doctor

import (
	"fmt"
	"sort"
)

type archiveOrphansDetector struct{}

func (archiveOrphansDetector) ID() string { return "archive-orphans" }

func (archiveOrphansDetector) Run(ctx *Context) ([]Finding, error) {
	if ctx == nil || ctx.Resolver == nil {
		return nil, fmt.Errorf("archive-orphans: nil context or resolver")
	}
	r := ctx.Resolver

	archived, err := r.ListArchivedWorks()
	if err != nil {
		return nil, fmt.Errorf("archive-orphans: listing archive: %w", err)
	}
	if len(archived) == 0 {
		return []Finding{{
			Severity: Green,
			Summary:  "archive: no entries",
		}}, nil
	}

	// Enumerate live works for the project. A missing works directory
	// (e.g., fresh project with no active works) is not an error — it
	// simply means there can be no collisions.
	live, err := r.ListWorks()
	if err != nil {
		// Distinguish a truly unreadable works directory from a
		// not-yet-created one: ListWorks already swallows IsNotExist
		// via listDirs, so any error here is genuine I/O failure.
		return nil, fmt.Errorf("archive-orphans: listing live works: %w", err)
	}
	liveSet := make(map[string]bool, len(live))
	for _, n := range live {
		liveSet[n] = true
	}

	var collisions []Item
	for _, name := range archived {
		if !liveSet[name] {
			continue
		}
		collisions = append(collisions, Item{
			Target: name,
			Detail: fmt.Sprintf("archive %s shares codename with live work %s",
				r.ArchiveDir(name), r.WorkDir(name)),
		})
	}
	if len(collisions) == 0 {
		return []Finding{{
			Severity: Green,
			Summary: fmt.Sprintf("archive: %d %s, no live collisions",
				len(archived), entryWord(len(archived))),
		}}, nil
	}

	sort.Slice(collisions, func(i, j int) bool {
		return collisions[i].Target < collisions[j].Target
	})
	return []Finding{{
		Severity: Yellow,
		Summary: fmt.Sprintf("archive: %d codename collision%s with live works",
			len(collisions), pluralS(len(collisions))),
		Items: collisions,
		Hint:  "rename the archive entry, or finalize/delete the live work; codenames must be unique within a project",
	}}, nil
}

// entryWord returns "entry" / "entries" for the count.
func entryWord(n int) string {
	if n == 1 {
		return "entry"
	}
	return "entries"
}

// pluralS returns "" for n==1, else "s". Local to this file to avoid
// colliding with storage_drift's plural() helper at the package level.
func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func init() {
	Register(archiveOrphansDetector{})
}
