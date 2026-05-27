package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/gregberns/kerf/internal/areas"
	"github.com/gregberns/kerf/internal/beads"
	"github.com/gregberns/kerf/internal/bench"
	"github.com/gregberns/kerf/internal/cmdutil"
	"github.com/gregberns/kerf/internal/config"
	"github.com/gregberns/kerf/internal/dep"
	"github.com/gregberns/kerf/internal/drift"
	"github.com/gregberns/kerf/internal/jig"
	"github.com/gregberns/kerf/internal/spec"
)

var showCompactFlag bool

var showCmd = &cobra.Command{
	Use:   "show <codename>",
	Short: "Display full details for a work",
	Args:  cobra.ExactArgs(1),
	// SilenceUsage: errors returned by runShow (including BEADS_TOOL_ERROR
	// from the configured `tools.tasks` subprocess) are user-facing
	// diagnostics, not flag-misuse signals. Suppress cobra's default usage
	// dump so scripts see only the single-line error before exit 1 — mirrors
	// kerf-1d6 / kerf-jy2i on next / triage / doctor.
	SilenceUsage: true,
	RunE:         runShow,
}

func init() {
	showCmd.Flags().BoolVar(&showCompactFlag, "compact", false, "One-line-per-section summary: status, next-pass name, file count, last-session marker")
	rootCmd.AddCommand(showCmd)
}

func runShow(cmd *cobra.Command, args []string) error {
	codename := args[0]

	projectID, err := cmdutil.ResolveProject(projectFlag)
	if err != nil {
		return err
	}

	s, workDir, err := cmdutil.LoadWorkWithChecks(projectID, codename)
	if err != nil {
		return fmt.Errorf("work '%s' not found in project '%s'", codename, projectID)
	}

	// Load jig
	bp, _ := bench.BenchPath()
	jigsDir := filepath.Join(bp, "jigs")
	jigDef, _, _ := jig.Resolve(s.Jig, jigsDir)

	if showCompactFlag {
		return runShowCompact(s, workDir, jigDef)
	}

	// Metadata
	fmt.Printf("Work: %s\n", s.Codename)
	if s.Title != nil {
		fmt.Printf("Title: %s\n", *s.Title)
	}
	fmt.Printf("Type: %s\n", s.Type)
	fmt.Printf("Status: %s\n", s.Status)
	fmt.Printf("Project: %s\n", s.Project.ID)
	if len(s.Areas) > 0 {
		fmt.Printf("Areas: %s\n", strings.Join(s.Areas, ", "))
	} else {
		fmt.Println("Areas: (none)")
	}
	fmt.Printf("Jig: %s (v%d)\n", s.Jig, s.JigVersion)
	// Bead filter slot — always rendered per specs/commands.md §`kerf show`
	// ("Bead filter ... always rendered as a single line"). Literal value
	// when present, `(none)` when absent or present-but-empty.
	fmt.Printf("bead_filter: %s\n", renderBeadFilterSlot(s.BeadFilter))
	fmt.Printf("Created: %s\n", s.Created.Format(time.RFC3339))
	fmt.Printf("Updated: %s\n", s.Updated.Format(time.RFC3339))
	fmt.Println()

	// Area overlap
	printAreaOverlap(bp, projectID, s.Areas, s.Codename)

	// Jig context — current pass instructions
	if jigDef != nil {
		pass := jigDef.PassForStatus(s.Status)
		if pass != nil {
			fmt.Printf("Current pass: %s (status: %s)\n", pass.Name, pass.Status)
			fmt.Println()
			instructions := jigDef.InstructionsForPass(pass.Name)
			if instructions != "" {
				fmt.Println(instructions)
				fmt.Println()
			}
		}

		// Pass-output list — one stable line per pass identifying the canonical
		// output filename(s) per specs/jig-system.md §"Surfacing Pass Filenames"
		// and specs/commands.md §`kerf show` ("Pass N: <name> → Output: ...").
		if lines := renderPassOutputLines(jigDef); len(lines) > 0 {
			fmt.Println("Passes:")
			for _, ln := range lines {
				fmt.Println("  " + ln)
			}
			fmt.Println()
		}

		// Pass status for composable jigs
		if jigDef.Composable {
			fmt.Println("Pass status:")
			maxLen := 0
			for _, p := range jigDef.Passes {
				if len(p.Status) > maxLen {
					maxLen = len(p.Status)
				}
			}
			for _, p := range jigDef.Passes {
				status := computePassStatus(jigDef.StatusValues, s.Status, p.Status)
				fmt.Printf("  %-*s  %s\n", maxLen, p.Status+":", status)
			}
			fmt.Println()
		}
	}

	// Bead status (best-effort via configured beads tool; default "br").
	// Per Plan 008 / B5: filter beads via the resolved per-work + project
	// bead_filter (spec-conformant, case-sensitive matching) rather than
	// counting every bead in the project. Tool-failure errors are surfaced
	// (kerf-cz2t / kerf-1d6 invariant): "not on PATH" is silent degrade,
	// "on PATH but failed" is exit-non-zero.
	beadSummary, err := getBeadSummary(s.Project.ID, s.Codename, s.BeadFilter)
	if err != nil {
		return err
	}
	if beadSummary != "" {
		fmt.Println(beadSummary)
		fmt.Println()
	}

	// Attached beads block (Plan 009 / B7).
	// Lists beads attached to this work via resolved filter, plus any
	// pinned_beads, annotated with drift markers from the cached baseline.
	// Silent no-op when the bead store is unavailable; tool-failure errors
	// are surfaced (kerf-cz2t / kerf-1d6 invariant).
	block, err := getAttachedBeadsBlock(s)
	if err != nil {
		return err
	}
	if block != "" {
		fmt.Println(block)
		fmt.Println()
	}

	// File tree (excluding .history/)
	fmt.Println("Files:")
	printFileTree(workDir, workDir, "  ")
	fmt.Println()

	// Session history
	if len(s.Sessions) > 0 {
		fmt.Println("Sessions:")
		for _, sess := range s.Sessions {
			id := "anonymous"
			if sess.ID != nil {
				id = *sess.ID
			}
			active := ""
			if s.ActiveSession != nil && ((sess.ID != nil && *sess.ID == *s.ActiveSession) || (sess.ID == nil && *s.ActiveSession == "anonymous")) {
				active = " [active]"
			}
			started := sess.Started.Format(time.RFC3339)
			ended := "(active)"
			if sess.Ended != nil {
				ended = sess.Ended.Format(time.RFC3339)
			}
			fmt.Printf("  %s  started: %s  ended: %s%s\n", id, started, ended, active)
		}
		fmt.Println()
	}

	// Dependencies
	if len(s.DependsOn) > 0 {
		fmt.Println("Dependencies:")
		for _, d := range s.DependsOn {
			result := dep.Resolve(d, bp, projectID)
			status := result.Status
			if result.Unresolvable {
				status = "unresolvable"
			}
			project := projectID
			if d.Project != nil {
				project = *d.Project
			}
			fmt.Printf("  %s (project: %s, relationship: %s) — %s\n", d.Codename, project, d.Relationship, status)
		}
		fmt.Println()
	}

	// SESSION.md contents
	sessionMDPath := filepath.Join(workDir, "SESSION.md")
	if data, err := os.ReadFile(sessionMDPath); err == nil {
		fmt.Println("SESSION.md:")
		fmt.Println(string(data))
		fmt.Println()
	}

	// Commands block
	fmt.Println("Commands:")
	fmt.Printf("  kerf resume %s                 Resume working\n", codename)
	fmt.Printf("  kerf status %s <next-status>   Advance status\n", codename)
	fmt.Printf("  kerf square %s                 Verify completeness\n", codename)
	fmt.Printf("  kerf shelve %s                 Pause work\n", codename)

	return nil
}

// printAreaOverlap emits the "Area overlap:" block if other active works share
// any of the given areas. Used by both `kerf show` and `kerf resume`.
func printAreaOverlap(benchPath, projectID string, workAreas []string, excludeCodename string) {
	if len(workAreas) == 0 {
		return
	}
	r, err := cmdutil.Resolver(projectID)
	if err != nil {
		return
	}
	overlaps, err := areas.FindOverlappingWorks(r, workAreas, excludeCodename)
	if err != nil || len(overlaps) == 0 {
		return
	}
	areaWorks := make(map[string][]string)
	for _, o := range overlaps {
		for _, a := range o.SharedAreas {
			areaWorks[a] = append(areaWorks[a], fmt.Sprintf("%s (%s)", o.Codename, o.Status))
		}
	}
	fmt.Println("Area overlap:")
	for _, a := range workAreas {
		if works, ok := areaWorks[a]; ok {
			fmt.Printf("  %s — also active in: %s\n", a, strings.Join(works, ", "))
		}
	}
	fmt.Println()
}

func printFileTree(root, dir, indent string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.Name() == ".history" {
			continue
		}
		rel, _ := filepath.Rel(root, filepath.Join(dir, e.Name()))
		if e.IsDir() {
			fmt.Printf("%s%s/\n", indent, rel)
			printFileTree(root, filepath.Join(dir, e.Name()), indent)
		} else {
			fmt.Printf("%s%s\n", indent, rel)
		}
	}
}

// formatRelativeTime produces a human-friendly relative time string.
func formatRelativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		return fmt.Sprintf("%dm ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		return fmt.Sprintf("%dh ago", h)
	default:
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	}
}

// computePassStatus determines the display status of a pass based on the work's
// current status and the ordered status_values list.
func computePassStatus(statusValues []string, currentStatus, passStatus string) string {
	currentIdx := -1
	passIdx := -1
	for i, sv := range statusValues {
		if sv == currentStatus {
			currentIdx = i
		}
		if sv == passStatus {
			passIdx = i
		}
	}
	if passIdx < 0 {
		return "unknown"
	}
	if currentIdx < 0 {
		// Current status not in list — past terminal
		return "done"
	}
	if currentIdx > passIdx {
		return "done"
	}
	if currentIdx == passIdx {
		return "active"
	}
	return "pending"
}

// getBeadSummary tries to read bead status via the configured beads CLI
// (project.yaml `tools.tasks`, default `br`) and returns a summary string
// for the beads attached to the given work via the resolved bead_filter.
// Returns empty string if the tool is unavailable, the project config is
// unreadable, or no beads match the filter.
//
// The argv shape used is the canonical `br` form
// (`br list --format json --all --limit 0`); if the user has configured a
// different binary, it must accept compatible flags.
//
// Bead attachment uses beads.ForWorkWithFilter with the spec-conformant
// case-sensitive matcher (Plan 008 / B5; specs/coordination.md §"Bead
// Attachment"). The filter is resolved per work via beads.Resolve(perWork,
// project) — first-defined-wins, no merge.
func getBeadSummary(projectID, codename string, perWork *beads.Filter) (string, error) {
	toolName := beads.DefaultToolName
	var projectFilter *beads.Filter
	if r, err := cmdutil.Resolver(projectID); err == nil {
		if cfg, err := config.LoadProjectConfig(r.ProjectConfigPath()); err == nil && cfg != nil {
			toolName = beads.ResolveToolName(cfg.Tools)
			projectFilter = cfg.BeadFilter
		}
	}

	// "Tool not on PATH" is a silent degrade (kerf works without a bead
	// store); "tool on PATH but failed" is a hard error so scripts see the
	// misconfiguration (kerf-cz2t / kerf-1d6 invariant).
	if !beads.IsAvailableNamed(toolName) {
		return "", nil
	}
	bs, err := beads.ListNamed(toolName)
	if err != nil {
		return "", err
	}
	if len(bs) == 0 {
		return "", nil
	}

	resolved := beads.Resolve(perWork, projectFilter)
	matched := beads.ForWorkWithFilter(bs, codename, resolved)
	if len(matched) == 0 {
		return "", nil
	}

	total := len(matched)
	closed := 0
	open := 0
	for _, b := range matched {
		switch strings.ToLower(b.Status) {
		case "closed", "done", "complete":
			closed++
		default:
			open++
		}
	}

	return fmt.Sprintf("Beads: %d total, %d closed, %d open", total, closed, open), nil
}

// getAttachedBeadsBlock renders the "Attached beads (N open / M closed)" block
// per specs/commands.md §`kerf show`. It includes:
//   - Beads matching the work's resolved bead_filter.
//   - Beads listed in spec.PinnedBeads (annotated "(pinned)" — surface even
//     when they would not match the filter).
//   - Drift markers (closed externally / reopened externally / new /
//     deleted / changed) computed against the cached baseline snapshot.
//
// Returns "" when:
//   - The bead store is unavailable (silent degrade).
//   - The work has zero attached and zero pinned beads (the higher
//     "Bead status" line above already covers the empty case).
//
// This function never advances the drift baseline.
func getAttachedBeadsBlock(s *spec.SpecYAML) (string, error) {
	toolName := beads.DefaultToolName
	var projectFilter *beads.Filter
	r, err := cmdutil.Resolver(s.Project.ID)
	if err == nil {
		if cfg, cerr := config.LoadProjectConfig(r.ProjectConfigPath()); cerr == nil && cfg != nil {
			toolName = beads.ResolveToolName(cfg.Tools)
			projectFilter = cfg.BeadFilter
		}
	}

	// Silent no-op if the bead store is unavailable.
	if !beads.IsAvailableNamed(toolName) {
		return "", nil
	}

	// Tool on PATH but failed → surface error (kerf-cz2t / kerf-1d6).
	all, err := beads.ListNamed(toolName)
	if err != nil {
		return "", err
	}

	resolved := beads.Resolve(s.BeadFilter, projectFilter)
	matched := beads.ForWorkWithFilter(all, s.Codename, resolved)

	// Index current beads by ID for pin-overlay and drift lookup.
	currentByID := make(map[string]beads.Bead, len(all))
	for _, b := range all {
		currentByID[b.ID] = b
	}

	// Build the dedup'd set, marking pinned membership.
	pinned := make(map[string]bool, len(s.PinnedBeads))
	for _, id := range s.PinnedBeads {
		pinned[id] = true
	}

	seen := make(map[string]bool, len(matched)+len(s.PinnedBeads))
	type row struct {
		ID     string
		Status string
		Title  string
		Pinned bool
		Drift  string // empty if no marker
	}
	var rows []row

	addRow := func(b beads.Bead) {
		if seen[b.ID] {
			return
		}
		seen[b.ID] = true
		rows = append(rows, row{
			ID:     b.ID,
			Status: b.Status,
			Title:  b.Title,
			Pinned: pinned[b.ID],
		})
	}

	for _, b := range matched {
		addRow(b)
	}
	// Pinned beads that did not match the filter — surface them too.
	for _, id := range s.PinnedBeads {
		if b, ok := currentByID[id]; ok {
			addRow(b)
		}
	}

	// Read baseline + compute drift. Baseline absence is non-fatal:
	// drift markers are simply omitted in that case (per spec).
	closedSet := map[string]bool{
		"closed":    true,
		"done":      true,
		"complete":  true,
		"completed": true,
	}
	repoRoot := ""
	if r != nil {
		repoRoot = r.RepoRoot
	}
	cachePath := drift.CachePath(repoRoot)
	baseline, hasBaseline, _ := drift.Read(cachePath)

	if hasBaseline {
		current := drift.Capture(all, nil)
		diff := drift.Compute(baseline, current, closedSet)

		markerFor := make(map[string]string)
		for _, id := range diff.ClosedExternally {
			markerFor[id] = "closed externally since last triage"
		}
		for _, id := range diff.ReopenedExternally {
			markerFor[id] = "reopened externally since last triage"
		}
		for _, id := range diff.New {
			markerFor[id] = "new since last triage"
		}
		for _, id := range diff.Changed {
			// Distinguish relabel vs retitle vs dep-change by comparing
			// the baseline record to the current bead. "relabeled" wins
			// if labels differ; else "retitled" if title differs; else
			// fall back to "changed".
			b, present := currentByID[id]
			base, basePresent := baseline.Beads[id]
			switch {
			case present && basePresent && !sameLabels(base.Labels, b.Labels):
				markerFor[id] = "relabeled since last triage"
			case present && basePresent && base.Title != b.Title:
				markerFor[id] = "retitled since last triage"
			default:
				markerFor[id] = "changed since last triage"
			}
		}

		// Apply markers to existing rows.
		for i := range rows {
			if m, ok := markerFor[rows[i].ID]; ok {
				rows[i].Drift = m
			}
		}

		// Deleted beads: present in baseline, absent from current. They
		// still appear in the block (with their baseline title) and a
		// "! deleted" marker — they only fall out when --ack advances
		// the baseline.
		//
		// Per spec the deleted bead is shown only when it had been
		// attached to this work (via the baseline's filter assignments)
		// or is currently pinned. PinnedBeads handles the pin case
		// (skipped above when the bead isn't in currentByID — restore
		// that here).
		for _, id := range diff.Deleted {
			if seen[id] {
				continue
			}
			// Was this deleted bead attached to this work in the baseline?
			attached := false
			if works, ok := baseline.FilterAssignments[id]; ok {
				for _, w := range works {
					if w == s.Codename {
						attached = true
						break
					}
				}
			}
			if !attached && !pinned[id] {
				continue
			}
			baseRec := baseline.Beads[id]
			rows = append(rows, row{
				ID:     id,
				Status: baseRec.Status,
				Title:  baseRec.Title,
				Pinned: pinned[id],
				Drift:  "deleted since last triage",
			})
			seen[id] = true
		}
	}

	if len(rows) == 0 {
		return "", nil
	}

	// Sort: open beads first, then closed; within each group, by ID.
	open := rows[:0:0]
	closed := rows[:0:0]
	for _, r := range rows {
		if isClosedShowStatus(r.Status) {
			closed = append(closed, r)
		} else {
			open = append(open, r)
		}
	}
	sortRowsByID := func(rs []row) {
		for i := 1; i < len(rs); i++ {
			for j := i; j > 0 && rs[j-1].ID > rs[j].ID; j-- {
				rs[j-1], rs[j] = rs[j], rs[j-1]
			}
		}
	}
	sortRowsByID(open)
	sortRowsByID(closed)

	// Compute column widths for stable alignment.
	idW, statW, titleW := 0, 0, 0
	all2 := append(append([]row{}, open...), closed...)
	for _, r := range all2 {
		if len(r.ID) > idW {
			idW = len(r.ID)
		}
		if len(r.Status) > statW {
			statW = len(r.Status)
		}
		if len(r.Title) > titleW {
			titleW = len(r.Title)
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Attached beads (%d open / %d closed):\n", len(open), len(closed))
	for _, r := range all2 {
		fmt.Fprintf(&b, "  %-*s  %-*s  %-*s", idW, r.ID, statW, r.Status, titleW, r.Title)
		if r.Pinned {
			b.WriteString("  (pinned)")
		}
		if r.Drift != "" {
			b.WriteString("  ! ")
			b.WriteString(r.Drift)
		}
		b.WriteByte('\n')
	}
	// Trim trailing newline so the caller's fmt.Println() does not double-blank.
	return strings.TrimRight(b.String(), "\n"), nil
}

// renderBeadFilterSlot returns the single-line literal for a work's
// bead_filter slot, per specs/commands.md §`kerf show` and `kerf work show`.
//   - nil / empty filter → "(none)"
//   - single leaf (label / id_prefix) → "label=<v>" or "id_prefix=<v>"
//   - any: union → comma-joined leaves in source order
func renderBeadFilterSlot(f *beads.Filter) string {
	if f == nil {
		return "(none)"
	}
	if len(f.Any) > 0 {
		parts := make([]string, 0, len(f.Any))
		for i := range f.Any {
			if leaf := renderLeafClause(&f.Any[i]); leaf != "" {
				parts = append(parts, leaf)
			}
		}
		if len(parts) == 0 {
			return "(none)"
		}
		return strings.Join(parts, ", ")
	}
	if leaf := renderLeafClause(f); leaf != "" {
		return leaf
	}
	return "(none)"
}

func renderLeafClause(f *beads.Filter) string {
	if f == nil {
		return ""
	}
	if f.Label != "" {
		return "label=" + f.Label
	}
	if f.IDPrefix != "" {
		return "id_prefix=" + f.IDPrefix
	}
	return ""
}

// isClosedShowStatus is the closed-status predicate used by the attached
// beads block. Mirrors internal/feed/cleanup.go isClosedStatus for the
// open/closed split rule per specs/commands.md §`kerf show`.
func isClosedShowStatus(s string) bool {
	switch strings.ToLower(s) {
	case "closed", "done", "complete", "completed":
		return true
	}
	return false
}

// sameLabels returns true iff a and b contain the same elements in the
// same order. The drift package normalizes label slices before hashing,
// so baseline.Beads[id].Labels and normalize(current.Labels) are
// directly comparable; we re-normalize current here for parity.
func sameLabels(a, b []string) bool {
	an := normalizeForCompare(a)
	bn := normalizeForCompare(b)
	if len(an) != len(bn) {
		return false
	}
	for i := range an {
		if an[i] != bn[i] {
			return false
		}
	}
	return true
}

// normalizeForCompare returns a lower-cased, deduplicated, sorted copy of
// labels for comparison parity with drift.Capture's normalization.
func normalizeForCompare(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	// simple sort
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

// renderPassOutputLines builds one stable line per declared pass of the form
//
//	Pass N: <name> → Output: <path>
//
// per specs/jig-system.md §"Surfacing Pass Filenames" and specs/commands.md
// §`kerf show`. Passes with multiple outputs render comma-joined. Passes that
// declare no output (process passes like Dispatch/Implement/Complete) render
// with `Output: (none)` so the line is always emitted — agents can see the
// full pass list at a glance.
func renderPassOutputLines(j *jig.JigDefinition) []string {
	if j == nil || len(j.Passes) == 0 {
		return nil
	}
	out := make([]string, 0, len(j.Passes))
	for i, p := range j.Passes {
		var outputs string
		if len(p.Output) == 0 {
			outputs = "(none)"
		} else {
			outputs = strings.Join(p.Output, ", ")
		}
		out = append(out, fmt.Sprintf("Pass %d: %s → Output: %s", i+1, p.Name, outputs))
	}
	return out
}

// runShowCompact prints the four-line compact summary plus the bead-filter slot
// per specs/commands.md §`kerf show` "--compact output":
//
//	{codename}  status: {current-status} → next: {next-pass-name}
//	bead_filter: {value or (none)}
//	files:       {n} in work directory
//	last session: {relative-time} ({active|ended})
//
// Errors and command-line resolution behave identically to the full form;
// only the rendering is collapsed.
func runShowCompact(s *spec.SpecYAML, workDir string, jigDef *jig.JigDefinition) error {
	// Next-pass name: the pass *after* the current status's pass. When the
	// work is at or past the terminal status, fall back to "(terminal)".
	next := "(terminal)"
	if jigDef != nil {
		for i, p := range jigDef.Passes {
			if p.Status == s.Status {
				if i+1 < len(jigDef.Passes) {
					next = jigDef.Passes[i+1].Name
				}
				break
			}
		}
	}
	fmt.Printf("%s  status: %s → next: %s\n", s.Codename, s.Status, next)
	fmt.Printf("bead_filter: %s\n", renderBeadFilterSlot(s.BeadFilter))

	// File count: non-recursive count of entries in the work directory,
	// excluding .history/ to mirror the verbose file tree's exclusion.
	n := 0
	if entries, err := os.ReadDir(workDir); err == nil {
		for _, e := range entries {
			if e.Name() == ".history" {
				continue
			}
			n++
		}
	}
	fmt.Printf("files:       %d in work directory\n", n)

	// Last-session marker: most recent entry in s.Sessions, with active/ended
	// derived from s.ActiveSession.
	if len(s.Sessions) == 0 {
		fmt.Println("last session: (none)")
	} else {
		last := s.Sessions[len(s.Sessions)-1]
		rel := formatRelativeTime(last.Started)
		state := "ended"
		if s.ActiveSession != nil {
			if (last.ID != nil && *last.ID == *s.ActiveSession) || (last.ID == nil && *s.ActiveSession == "anonymous") {
				state = "active"
			}
		}
		fmt.Printf("last session: %s (%s)\n", rel, state)
	}
	return nil
}

// statusProgression renders the status progression with a pointer to current.
func statusProgression(statusValues []string, current string) string {
	parts := make([]string, len(statusValues))
	pointer := -1
	for i, sv := range statusValues {
		parts[i] = sv
		if sv == current {
			pointer = i
		}
	}
	line := "  " + strings.Join(parts, " -> ")
	if pointer >= 0 {
		// Build pointer line
		pos := 2 // initial indent
		for i := 0; i < pointer; i++ {
			pos += len(parts[i]) + 4 // " -> " is 4 chars
		}
		mid := pos + len(parts[pointer])/2
		pointerLine := strings.Repeat(" ", mid) + "^^ current"
		return line + "\n" + pointerLine
	}
	// current status not in list
	return line + "\n  (current status '" + current + "' is not in the jig's list)"
}
