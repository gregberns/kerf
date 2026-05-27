package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/gregberns/kerf/internal/cmdutil"
	"github.com/gregberns/kerf/internal/spec"
	"github.com/gregberns/kerf/internal/storage"
)

var (
	listStatusFilter string
	listAll          bool
	listCreatedBy    string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Show all works on the bench",
	Long: `Show all works on the bench for the current project.

Examples:
  kerf list                 List active works
  kerf list --status research  Filter by status
  kerf list --all           Include archived works
  kerf list --created-by self  Only works this session created`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runList()
	},
}

func init() {
	listCmd.Flags().StringVar(&listStatusFilter, "status", "", "Filter to works with this status")
	listCmd.Flags().BoolVar(&listAll, "all", false, "Include archived works")
	listCmd.Flags().StringVar(&listCreatedBy, "created-by", "all", "Filter by creator: 'self' (current session) or 'all' (default, with attribution markers)")
	rootCmd.AddCommand(listCmd)
}

type workEntry struct {
	codename string
	workType string
	status   string
	updated  time.Time
	archived bool
	deps     []spec.Dependency
	// creatorID is the `id` of the first session entry on spec.yaml.
	// nil when sessions is empty or sessions[0].ID is null. The first
	// entry is the creator session per specs/sessions.md §"Creator
	// Attribution" (kerf new is what appends it; see internal/session
	// and cmd/new.go).
	creatorID *string
}

// currentSessionID returns the active agent's session identity for the
// purpose of --created-by self filtering and attribution. Today the only
// source is the KERF_SESSION_ID environment variable; the empty string
// is the "anonymous" sentinel and matches works whose creator session
// has a null id. There is no other session-id wiring in cmd/new or
// cmd/resume yet (both pass "" to internal/session.StartSession), so
// most works have anonymous creators in practice. See specs/sessions.md
// §"Creator Attribution".
func currentSessionID() string {
	return os.Getenv("KERF_SESSION_ID")
}

// creatorMatches reports whether the given creator id (from sessions[0])
// equals the current session identity. Both nil-creator and unset-env
// resolve to anonymous, so an anonymous creator matches an anonymous
// current session.
func creatorMatches(creator *string, current string) bool {
	if creator == nil {
		return current == ""
	}
	return *creator == current
}

// attributionMarker renders the per-row marker shown next to a
// codename when --created-by all is in effect. Format:
//   - "(you)"        — creator matches the current session
//   - "(by <8-char>)" — creator session has an id that does not match
//   - "(by anon)"    — creator session has no id (null)
func attributionMarker(creator *string, current string) string {
	if creatorMatches(creator, current) {
		return "(you)"
	}
	if creator == nil {
		return "(by anon)"
	}
	id := *creator
	if len(id) > 8 {
		id = id[:8]
	}
	return "(by " + id + ")"
}

func runList() error {
	projectID, err := cmdutil.ResolveProject(projectFlag)
	if err != nil {
		return err
	}

	r, err := cmdutil.Resolver(projectID)
	if err != nil {
		return err
	}

	var entries []workEntry

	// Active works.
	codenames, err := r.ListWorks()
	if err != nil {
		return err
	}
	for _, cn := range codenames {
		if e, ok := readWorkEntry(r, cn, false); ok {
			entries = append(entries, e)
		}
	}

	// Archived works if --all.
	if listAll {
		archived, err := r.ListArchivedWorks()
		if err != nil {
			return err
		}
		for _, cn := range archived {
			dir := r.ArchiveDir(cn)
			specPath := filepath.Join(dir, "spec.yaml")
			s, err := spec.Read(specPath)
			if err != nil {
				// Same rationale as readWorkEntry; archived works are
				// surfaced via the same stderr channel.
				fmt.Fprintf(os.Stderr, "warning: corrupt spec for '%s': %v (excluded from list)\n", cn, err)
				continue
			}
			entries = append(entries, workEntry{
				codename:  s.Codename,
				workType:  s.Type,
				status:    s.Status,
				updated:   s.Updated,
				archived:  true,
				deps:      s.DependsOn,
				creatorID: creatorIDFromSpec(s),
			})
		}
	}

	// Filter by status.
	if listStatusFilter != "" {
		var filtered []workEntry
		for _, e := range entries {
			if e.status == listStatusFilter {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	// Filter by creator. The "self" identity comes from KERF_SESSION_ID
	// (see currentSessionID); empty env matches works whose creator
	// session has a null id. See specs/commands.md §`kerf list`.
	currentSession := currentSessionID()
	switch listCreatedBy {
	case "", "all":
		// keep all, attribution markers applied at render time
	case "self":
		var filtered []workEntry
		for _, e := range entries {
			if creatorMatches(e.creatorID, currentSession) {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	default:
		return fmt.Errorf("invalid --created-by value '%s': expected 'self' or 'all'", listCreatedBy)
	}

	// Sort by updated, most recent first.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].updated.After(entries[j].updated)
	})

	if len(entries) == 0 {
		fmt.Printf("No works found for project '%s'.\n", projectID)
		fmt.Println()
		fmt.Println("Get started:")
		fmt.Println("  kerf new    Create a new work")
		return nil
	}

	fmt.Printf("On the bench for %s:\n", projectID)

	// Find column widths for alignment.
	maxCN, maxType, maxStatus := 0, 0, 0
	for _, e := range entries {
		if len(e.codename) > maxCN {
			maxCN = len(e.codename)
		}
		if len(e.workType) > maxType {
			maxType = len(e.workType)
		}
		sl := len(e.status)
		if e.archived {
			sl += 11 // " [archived]"
		}
		if sl > maxStatus {
			maxStatus = sl
		}
	}

	// Attribution markers shown in 'all' mode only; in 'self' mode every
	// row is "(you)" so the marker is omitted as visual noise.
	showAttribution := listCreatedBy == "" || listCreatedBy == "all"

	for _, e := range entries {
		statusStr := e.status
		if e.archived {
			statusStr += " [archived]"
		}
		if showAttribution {
			fmt.Printf("  %-*s  %-*s  %-*s  %s  %s\n",
				maxCN, e.codename,
				maxType, e.workType,
				maxStatus, statusStr,
				relativeTime(e.updated),
				attributionMarker(e.creatorID, currentSession),
			)
		} else {
			fmt.Printf("  %-*s  %-*s  %-*s  %s\n",
				maxCN, e.codename,
				maxType, e.workType,
				maxStatus, statusStr,
				relativeTime(e.updated),
			)
		}
	}

	// Dependencies section.
	var depLines []string
	for _, e := range entries {
		for _, d := range e.deps {
			depStatus := lookupDepStatus(r, d)
			depLines = append(depLines, fmt.Sprintf("  %s -> %s [%s]", e.codename, d.Codename, depStatus))
		}
	}
	if len(depLines) > 0 {
		fmt.Println()
		fmt.Println("  Dependencies:")
		for _, l := range depLines {
			fmt.Println(l)
		}
	}

	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  kerf show <codename>      View work details")
	fmt.Println("  kerf resume <codename>    Resume working on a work")
	fmt.Println("  kerf new                  Start a new work")

	if hasActiveNonComplete(entries) {
		if cwd, err := os.Getwd(); err == nil {
			if hint := cmdutil.MaybeRetrofitHint(cwd); hint != "" {
				fmt.Println()
				fmt.Println(hint)
			}
		}
	}

	return nil
}

func hasActiveNonComplete(entries []workEntry) bool {
	for _, e := range entries {
		if e.archived {
			continue
		}
		if e.status == "complete" || e.status == "archived" {
			continue
		}
		return true
	}
	return false
}

func readWorkEntry(r *storage.Resolver, codename string, archived bool) (workEntry, bool) {
	dir := r.WorkDir(codename)
	specPath := filepath.Join(dir, "spec.yaml")
	s, err := spec.Read(specPath)
	if err != nil {
		// Surface rather than silently swallow — analogous to the
		// `corrupt_spec` warning kerf next emits (Plan 008 / B10-code;
		// specs/commands.md §"Warning kinds"). list has no warning
		// channel, so we route to stderr and continue.
		fmt.Fprintf(os.Stderr, "warning: corrupt spec for '%s': %v (excluded from list)\n", codename, err)
		return workEntry{}, false
	}
	return workEntry{
		codename:  s.Codename,
		workType:  s.Type,
		status:    s.Status,
		updated:   s.Updated,
		archived:  archived,
		deps:      s.DependsOn,
		creatorID: creatorIDFromSpec(s),
	}, true
}

// creatorIDFromSpec returns the id of the first session entry (the
// creator session, per specs/sessions.md §"Creator Attribution"), or
// nil if sessions is empty or sessions[0].ID is null.
func creatorIDFromSpec(s *spec.SpecYAML) *string {
	if len(s.Sessions) == 0 {
		return nil
	}
	return s.Sessions[0].ID
}

func lookupDepStatus(r *storage.Resolver, d spec.Dependency) string {
	depProject := r.ProjectID
	if d.Project != nil && *d.Project != "" {
		depProject = *d.Project
	}
	var depResolver *storage.Resolver
	if depProject == r.ProjectID {
		depResolver = r
	} else {
		dr, err := cmdutil.Resolver(depProject)
		if err != nil {
			return "unknown"
		}
		depResolver = dr
	}
	specPath := filepath.Join(depResolver.WorkDir(d.Codename), "spec.yaml")
	s, err := spec.Read(specPath)
	if err != nil {
		return "unknown"
	}
	return s.Status
}

func relativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1m ago"
		}
		return fmt.Sprintf("%dm ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1h ago"
		}
		return fmt.Sprintf("%dh ago", h)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1d ago"
		}
		return fmt.Sprintf("%dd ago", days)
	}
}
