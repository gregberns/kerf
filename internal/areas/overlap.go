package areas

import (
	"path/filepath"
	"sort"

	"github.com/gberns/kerf/internal/spec"
	"github.com/gberns/kerf/internal/storage"
)

// OverlapEntry represents a work that shares areas with another work.
type OverlapEntry struct {
	Codename    string
	Status      string
	SharedAreas []string
}

// FindOverlappingWorks finds active works that share areas with the given area list.
// excludeCodename is omitted from results (to avoid self-matching).
func FindOverlappingWorks(r *storage.Resolver, targetAreas []string, excludeCodename string) ([]OverlapEntry, error) {
	if len(targetAreas) == 0 {
		return nil, nil
	}

	// Build a set of target areas for fast lookup.
	targetSet := make(map[string]bool, len(targetAreas))
	for _, a := range targetAreas {
		targetSet[a] = true
	}

	// List all active works in the project.
	works, err := r.ListWorks()
	if err != nil {
		return nil, err
	}

	var entries []OverlapEntry
	for _, codename := range works {
		if codename == excludeCodename {
			continue
		}

		workDir := r.WorkDir(codename)
		specPath := filepath.Join(workDir, "spec.yaml")

		s, err := spec.Read(specPath)
		if err != nil {
			// Skip works with unreadable specs.
			continue
		}

		// Find shared areas.
		var shared []string
		for _, area := range s.Areas {
			if targetSet[area] {
				shared = append(shared, area)
			}
		}

		if len(shared) > 0 {
			sort.Strings(shared)
			entries = append(entries, OverlapEntry{
				Codename:    codename,
				Status:      s.Status,
				SharedAreas: shared,
			})
		}
	}

	// Sort entries by codename for stable output.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Codename < entries[j].Codename
	})

	return entries, nil
}
