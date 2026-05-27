package doctor

// storage-drift detector — Plan 017 / B7 (bead kerf-9jh).
//
// Reports presence-level storage-layout drift between the bench
// (~/.kerf/projects/<id>/) and the repo (<repo>/.kerf/). Content-level
// (hash) comparison is out of scope for v1 — see the TBD marker in
// specs/architecture.md §"What counts as storage-layout drift (v1)".
//
// Spec references:
//   - specs/architecture.md §"Drift Detection (storage layout)" — the
//     five finding classes (lines 131-138).
//   - specs/commands.md §"kerf doctor" §"storage-drift" — presence-only
//     scope, output shape, exit semantics.
//
// The detector enumerates these five classes:
//
//  1. Wrong-location work directories — works present in the
//     non-canonical location for the active mode (work dir in bench
//     when local-mode active, or vice versa).
//  2. Double-presence — a work appears in BOTH the canonical and
//     non-canonical location simultaneously.
//  3. Broken bench symlink (local mode) — bench path exists but is a
//     real directory instead of a symlink, the symlink is stale, or it
//     points somewhere other than the resolver's expected target.
//  4. Double project.yaml / areas.yaml — the per-project config or
//     area definitions exist in BOTH the repo's .kerf/ and the bench's
//     ~/.kerf/projects/<id>/ at once.
//  5. Archive / live codename collision — an archive entry
//     (~/.kerf/archive/<id>/<codename>/) shares its codename with a
//     live work under the same project.
//
// Each finding names the canonical fix hint (commonly `kerf localize`
// for bench→local, a manual move for the inverse).

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gregberns/kerf/internal/storage"
)

// storageDriftDetector is the registered Detector for "storage-drift".
type storageDriftDetector struct{}

// ID implements Detector.
func (storageDriftDetector) ID() string { return "storage-drift" }

func init() { Register(storageDriftDetector{}) }

// Run implements Detector. It collects the five finding classes and
// returns one Finding per class triggered. When none triggers, a single
// green Finding summarising the canonical layout is returned, per the
// formatter contract in doctor.go.
func (storageDriftDetector) Run(ctx *Context) ([]Finding, error) {
	if ctx == nil || ctx.Resolver == nil {
		return nil, fmt.Errorf("storage-drift: nil context or resolver")
	}
	r := ctx.Resolver

	var findings []Finding

	// Canonical and non-canonical locations for works.
	canonicalWorks := r.WorksDir()
	nonCanonicalWorks := otherWorksDir(r, ctx.BenchPath, ctx.ProjectID)

	canonicalWorkSet, err := listDirSet(canonicalWorks)
	if err != nil {
		return nil, fmt.Errorf("storage-drift: reading %s: %w", canonicalWorks, err)
	}
	// Skip enumeration of the non-canonical bench path when it's actually
	// a symlink resolving back to the canonical works dir — that's the
	// healthy local-mode bench symlink, not drift. Its presence/integrity
	// is inspected separately below (class 3).
	nonCanonicalWorkSet := map[string]bool{}
	if !isSymlinkToSameTarget(nonCanonicalWorks, canonicalWorks) {
		nonCanonicalWorkSet, err = listDirSet(nonCanonicalWorks)
		if err != nil {
			return nil, fmt.Errorf("storage-drift: reading %s: %w", nonCanonicalWorks, err)
		}
	}

	// --- Class 2: double-presence (red — takes precedence over class 1) ---
	var doubles []Item
	for name := range nonCanonicalWorkSet {
		if canonicalWorkSet[name] {
			doubles = append(doubles, Item{
				Target: name,
				Detail: fmt.Sprintf("present in both %s and %s", canonicalWorks, nonCanonicalWorks),
			})
		}
	}
	if len(doubles) > 0 {
		findings = append(findings, Finding{
			Severity: Red,
			Summary:  fmt.Sprintf("%d work director%s present in both canonical and non-canonical locations", len(doubles), plural(len(doubles), "y", "ies")),
			Items:    sortItems(doubles),
			Hint:     "investigate manually; remove or merge the duplicate before running kerf commands",
		})
	}

	// --- Class 1: wrong-location work directories (yellow) ---
	var wrongLoc []Item
	for name := range nonCanonicalWorkSet {
		if canonicalWorkSet[name] {
			continue // already reported as double-presence
		}
		wrongLoc = append(wrongLoc, Item{
			Target: name,
			Detail: fmt.Sprintf("lives in %s (canonical for active mode is %s)", nonCanonicalWorks, canonicalWorks),
		})
	}
	if len(wrongLoc) > 0 {
		findings = append(findings, Finding{
			Severity: Yellow,
			Summary:  fmt.Sprintf("%d work director%s in non-canonical location for %s mode", len(wrongLoc), plural(len(wrongLoc), "y", "ies"), r.Mode),
			Items:    sortItems(wrongLoc),
			Hint:     hintWrongLocation(r.Mode),
		})
	}

	// --- Class 3: broken bench symlink (local mode only) ---
	if r.Mode == storage.ModeLocal && ctx.BenchPath != "" && ctx.ProjectID != "" {
		benchLink := filepath.Join(ctx.BenchPath, "projects", ctx.ProjectID)
		expected, _ := filepath.Abs(canonicalWorks)
		symFinding := inspectBenchSymlink(benchLink, expected)
		if symFinding != nil {
			findings = append(findings, *symFinding)
		}
	}

	// --- Class 4: double project.yaml / areas.yaml ---
	var doubleCfg []Item
	for _, pair := range configPairs(ctx.BenchPath, ctx.ProjectID, r.RepoRoot) {
		if fileExists(pair.bench) && fileExists(pair.repo) {
			doubleCfg = append(doubleCfg, Item{
				Target: pair.name,
				Detail: fmt.Sprintf("present in both %s and %s", pair.bench, pair.repo),
			})
		}
	}
	if len(doubleCfg) > 0 {
		findings = append(findings, Finding{
			Severity: Red,
			Summary:  fmt.Sprintf("%d config file%s present in both bench and repo", len(doubleCfg), plural(len(doubleCfg), "", "s")),
			Items:    sortItems(doubleCfg),
			Hint:     "remove the non-canonical copy; canonical home depends on the active storage mode (see specs/architecture.md 'Where state lives')",
		})
	}

	// --- Class 5: archive / live codename collisions ---
	archived, err := r.ListArchivedWorks()
	if err != nil {
		return nil, fmt.Errorf("storage-drift: listing archive: %w", err)
	}
	var collisions []Item
	for _, name := range archived {
		if canonicalWorkSet[name] || nonCanonicalWorkSet[name] {
			collisions = append(collisions, Item{
				Target: name,
				Detail: fmt.Sprintf("archive %s shares codename with a live work", r.ArchiveDir(name)),
			})
		}
	}
	if len(collisions) > 0 {
		findings = append(findings, Finding{
			Severity: Yellow,
			Summary:  fmt.Sprintf("%d archive/live codename collision%s", len(collisions), plural(len(collisions), "", "s")),
			Items:    sortItems(collisions),
			Hint:     "rename the archive entry or the live work; codenames must be unique within a project",
		})
	}

	if len(findings) == 0 {
		findings = append(findings, Finding{
			Severity: Green,
			Summary:  fmt.Sprintf("storage layout consistent with %s mode", r.Mode),
		})
	}
	return findings, nil
}

// otherWorksDir returns the works directory for the mode opposite to
// what the resolver is configured for — the non-canonical location
// where presence is evidence of drift.
func otherWorksDir(r *storage.Resolver, benchPath, projectID string) string {
	if r.Mode == storage.ModeLocal {
		// Non-canonical: bench projects dir.
		return filepath.Join(benchPath, "projects", projectID)
	}
	// Bench mode → non-canonical is repo .kerf/works.
	if r.RepoRoot == "" {
		return ""
	}
	return filepath.Join(r.RepoRoot, ".kerf", "works")
}

// configPairs enumerates the (project.yaml, areas.yaml) double-presence
// candidates: bench path and repo path for each file class.
type configPair struct {
	name        string
	bench, repo string
}

func configPairs(benchPath, projectID, repoRoot string) []configPair {
	if benchPath == "" || projectID == "" || repoRoot == "" {
		return nil
	}
	projDir := filepath.Join(benchPath, "projects", projectID)
	repoKerf := filepath.Join(repoRoot, ".kerf")
	return []configPair{
		{name: "project.yaml", bench: filepath.Join(projDir, "project.yaml"), repo: filepath.Join(repoKerf, "project.yaml")},
		{name: "areas.yaml", bench: filepath.Join(projDir, "areas.yaml"), repo: filepath.Join(repoKerf, "areas.yaml")},
	}
}

// inspectBenchSymlink validates the local-mode bench-side symlink.
// Returns nil when the link is healthy or simply absent (bench symlink
// is provisioned by `kerf localize`; absence is reported by the
// symlink-integrity detector, kerf-47z — we only flag positive
// breakage here so the two detectors don't double-report).
func inspectBenchSymlink(linkPath, expectedTarget string) *Finding {
	info, err := os.Lstat(linkPath)
	if err != nil {
		// Missing — left for symlink-integrity detector.
		return nil
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return &Finding{
			Severity: Red,
			Summary:  "bench path is a real directory, expected symlink (local mode)",
			Items: []Item{{
				Target: linkPath,
				Detail: "a real directory at this path shadows the repo works dir",
			}},
			Hint: "move or remove the directory and re-run `kerf localize` to recreate the symlink",
		}
	}
	target, err := os.Readlink(linkPath)
	if err != nil {
		return &Finding{
			Severity: Red,
			Summary:  "bench symlink unreadable",
			Items:    []Item{{Target: linkPath, Detail: err.Error()}},
			Hint:     "remove the symlink and re-run `kerf localize`",
		}
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(linkPath), target)
	}
	targetAbs, _ := filepath.Abs(target)
	if _, err := os.Stat(targetAbs); os.IsNotExist(err) {
		return &Finding{
			Severity: Red,
			Summary:  "bench symlink target missing",
			Items: []Item{{
				Target: linkPath,
				Detail: fmt.Sprintf("points to %s, which does not exist", target),
			}},
			Hint: "re-run `kerf localize` to recreate the works directory",
		}
	}
	if expectedTarget != "" && targetAbs != expectedTarget {
		return &Finding{
			Severity: Yellow,
			Summary:  "bench symlink points to unexpected target",
			Items: []Item{{
				Target: linkPath,
				Detail: fmt.Sprintf("points to %s, expected %s", target, expectedTarget),
			}},
			Hint: "re-run `kerf localize` to repoint the symlink at the active repo works dir",
		}
	}
	return nil
}

func hintWrongLocation(mode storage.Mode) string {
	if mode == storage.ModeLocal {
		return "move the directories from the bench into <repo>/.kerf/works/ (manual move; `kerf localize` only handles initial bench→local migration)"
	}
	return "run `kerf localize` to migrate works onto the bench, or remove the stale `.kerf/works/` directory if works are already on the bench"
}

// --- helpers -------------------------------------------------------------

func listDirSet(dir string) (map[string]bool, error) {
	out := map[string]bool{}
	if dir == "" {
		return out, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() {
			out[e.Name()] = true
		}
	}
	return out, nil
}

// isSymlinkToSameTarget returns true when linkPath is a symlink whose
// resolved (absolute) target equals the absolute form of targetPath.
func isSymlinkToSameTarget(linkPath, targetPath string) bool {
	if linkPath == "" || targetPath == "" {
		return false
	}
	info, err := os.Lstat(linkPath)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return false
	}
	resolved, err := os.Readlink(linkPath)
	if err != nil {
		return false
	}
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(filepath.Dir(linkPath), resolved)
	}
	resolvedAbs, _ := filepath.Abs(resolved)
	wantAbs, _ := filepath.Abs(targetPath)
	return resolvedAbs == wantAbs
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func sortItems(items []Item) []Item {
	// Stable insertion order; small N (handful per project) — a manual
	// bubble keeps imports minimal and the order deterministic for tests.
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j-1].Target > items[j].Target; j-- {
			items[j-1], items[j] = items[j], items[j-1]
		}
	}
	return items
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
