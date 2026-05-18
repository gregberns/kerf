// Package doctor — symlink-integrity detector.
//
// Spec reference:
//   - specs/commands.md §"kerf doctor" §"Behavior" item `symlink-integrity`
//     (line 1571): "(local mode only) — checks that
//     `~/.kerf/projects/<project-id>` is a symlink, that the target
//     exists, and that the target matches the resolver's expected path.
//     Reports broken, missing, or real-directory-where-symlink-expected
//     cases."
//   - plans/017_storage_reconciliation/_plan.md §B8 — bead scope; reuse
//     ensureLocalSymlink's semantics (path = bench/projects/<id>,
//     target = <repo>/.kerf/works).
//
// The detector is local-mode-only. Under bench mode it emits a single
// green Finding labelled "not applicable for bench storage" so the
// report row is informative rather than missing.

package doctor

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gberns/kerf/internal/storage"
)

type symlinkIntegrityDetector struct{}

func (symlinkIntegrityDetector) ID() string { return "symlink-integrity" }

func (symlinkIntegrityDetector) Run(ctx *Context) ([]Finding, error) {
	if ctx == nil || ctx.Resolver == nil {
		return nil, fmt.Errorf("symlink-integrity: nil resolver")
	}
	r := ctx.Resolver

	// Bench mode: skip cleanly with a green finding. The spec only
	// requires the check under local mode; in bench mode there is no
	// canonical symlink to check.
	if r.Mode != storage.ModeLocal {
		return []Finding{{
			Severity: Green,
			Summary:  "bench symlink: not applicable for bench storage",
		}}, nil
	}

	link := filepath.Join(r.BenchPath, "projects", r.ProjectID)
	expectedTarget := filepath.Join(r.RepoRoot, ".kerf", "works")

	info, err := os.Lstat(link)
	if err != nil {
		if os.IsNotExist(err) {
			return []Finding{{
				Severity: Red,
				Summary:  "bench symlink: missing",
				Items: []Item{{
					Target: link,
					Detail: "path does not exist",
				}},
				Hint: "kerf localize  (recreate the bench symlink)",
			}}, nil
		}
		return nil, fmt.Errorf("symlink-integrity: lstat %s: %w", link, err)
	}

	if info.Mode()&os.ModeSymlink == 0 {
		return []Finding{{
			Severity: Red,
			Summary:  "bench symlink: real directory where symlink expected",
			Items: []Item{{
				Target: link,
				Detail: "is a real directory, not a symlink",
			}},
			Hint: "remove or relocate the directory, then re-run 'kerf localize'",
		}}, nil
	}

	actualTarget, err := os.Readlink(link)
	if err != nil {
		return nil, fmt.Errorf("symlink-integrity: readlink %s: %w", link, err)
	}

	// Normalize for comparison: resolve symlink target relative to the
	// link's directory if not absolute.
	resolvedTarget := actualTarget
	if !filepath.IsAbs(resolvedTarget) {
		resolvedTarget = filepath.Join(filepath.Dir(link), resolvedTarget)
	}

	// Confirm the target exists and is a directory.
	tinfo, terr := os.Stat(link) // follows symlink
	if terr != nil {
		if os.IsNotExist(terr) {
			return []Finding{{
				Severity: Red,
				Summary:  "bench symlink: broken (target missing)",
				Items: []Item{
					{Target: link, Detail: fmt.Sprintf("points to %s, which does not exist", actualTarget)},
				},
				Hint: "kerf localize  (re-establish target directory)",
			}}, nil
		}
		return nil, fmt.Errorf("symlink-integrity: stat target of %s: %w", link, terr)
	}
	if !tinfo.IsDir() {
		return []Finding{{
			Severity: Red,
			Summary:  "bench symlink: target is not a directory",
			Items: []Item{
				{Target: link, Detail: fmt.Sprintf("points to %s (not a directory)", actualTarget)},
			},
			Hint: "remove the symlink and re-run 'kerf localize'",
		}}, nil
	}

	// Compare against the resolver's expected target. We compare both
	// raw and EvalSymlinks-resolved forms so a target reached through
	// an intermediate symlink (e.g., macOS /tmp -> /private/tmp) is
	// not flagged spuriously.
	if !samePath(resolvedTarget, expectedTarget) {
		return []Finding{{
			Severity: Red,
			Summary:  "bench symlink: target does not match resolver",
			Items: []Item{
				{Target: link, Detail: fmt.Sprintf("points to %s, expected %s", actualTarget, expectedTarget)},
			},
			Hint: "remove the symlink and re-run 'kerf localize'",
		}}, nil
	}

	return []Finding{{
		Severity: Green,
		Summary:  fmt.Sprintf("bench symlink: %s -> %s", link, expectedTarget),
	}}, nil
}

// samePath returns true when a and b refer to the same filesystem path,
// after resolving intermediate symlinks where possible. Falls back to a
// cleaned string compare when either path can't be resolved.
func samePath(a, b string) bool {
	if filepath.Clean(a) == filepath.Clean(b) {
		return true
	}
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	if errA == nil && errB == nil {
		return filepath.Clean(ra) == filepath.Clean(rb)
	}
	return false
}

func init() {
	Register(symlinkIntegrityDetector{})
}
