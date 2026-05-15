package scenario_import

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SourceKind identifies the input shape.
type SourceKind int

const (
	SourceUnknown SourceKind = iota
	SourceHarmonikPilot
	SourceKerfPlan
)

// Detect classifies `source` as a harmonik pilot YAML/dir or a kerf plan
// directory. The detection is structural: it looks at filename patterns
// and directory contents, not at YAML payloads.
func Detect(source string) (SourceKind, error) {
	st, err := os.Stat(source)
	if err != nil {
		return SourceUnknown, fmt.Errorf("scenario_import: stat %s: %w", source, err)
	}
	if !st.IsDir() {
		base := filepath.Base(source)
		if strings.HasSuffix(base, "-pilot-data.yaml") {
			return SourceHarmonikPilot, nil
		}
		return SourceUnknown, fmt.Errorf("scenario_import: %s: unrecognized source file (expected *-pilot-data.yaml)", source)
	}
	// Directory. Prefer harmonik pilot YAMLs if any are present; otherwise
	// fall back to kerf plan if `_plan.md` is at the directory root.
	entries, err := os.ReadDir(source)
	if err != nil {
		return SourceUnknown, fmt.Errorf("scenario_import: readdir %s: %w", source, err)
	}
	hasPilot := false
	hasPlan := false
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		name := ent.Name()
		if strings.HasSuffix(name, "-pilot-data.yaml") {
			hasPilot = true
		}
		if name == "_plan.md" {
			hasPlan = true
		}
	}
	switch {
	case hasPilot:
		return SourceHarmonikPilot, nil
	case hasPlan:
		return SourceKerfPlan, nil
	default:
		return SourceUnknown, fmt.Errorf("scenario_import: %s: no recognized inputs (expected *-pilot-data.yaml or _plan.md)", source)
	}
}
