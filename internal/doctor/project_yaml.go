// Package doctor — project-yaml detector.
//
// Plan 017 / B6 (bead kerf-7b4). Implements the `project-yaml`
// detector listed in specs/commands.md §"kerf doctor" §Behavior:
//
//	"project-yaml — checks project.yaml exists at the location
//	 dictated by the active storage mode, parses cleanly, declares
//	 at least one jig, and (when applicable) names a default_jig."
//
// The schema lives in specs/architecture.md and is in flight under
// Plan 016 (init-ux). Per the bead's open-question gate, this is a
// thin first-pass check against the minimal field set the rest of
// the codebase already reads: `jigs` (see internal/config/project.go
// `ProjectConfig.Jigs`).
package doctor

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// projectYAMLDetector implements Detector. The struct is empty: all
// inputs are supplied via the Context handed to Run.
type projectYAMLDetector struct{}

func (projectYAMLDetector) ID() string { return "project-yaml" }

// fixHint is the canonical fix command named in non-green findings per
// specs/commands.md §"kerf doctor" §"Behavior" ("Each finding names the
// canonical fix command in its hint line.").
const projectYAMLFixHint = "kerf init  (create project.yaml at the canonical path)"

// projectYAMLShape is a minimal local view of the project.yaml schema
// — only the fields this detector inspects. We unmarshal into this
// struct rather than internal/config.ProjectConfig so the detector
// stays decoupled from richer validators (e.g. bead_filter parsing)
// whose failure would otherwise be mis-reported as a project.yaml
// shape error. Round-trip fidelity is not needed here.
type projectYAMLShape struct {
	Jigs       []string `yaml:"jigs"`
	DefaultJig string   `yaml:"default_jig"`
}

// Run implements Detector. It emits exactly one Finding describing
// the project.yaml state.
func (d projectYAMLDetector) Run(ctx *Context) ([]Finding, error) {
	if ctx == nil || ctx.Resolver == nil {
		// No resolver means the scaffold couldn't compute the
		// canonical path. Treat as red; the upstream scaffold
		// should have surfaced a clearer error, but never panic.
		return []Finding{{
			Severity: Red,
			Summary:  "project.yaml: cannot determine canonical path (no resolver)",
			Hint:     projectYAMLFixHint,
		}}, nil
	}

	path := ctx.Resolver.ProjectConfigPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Finding{{
				Severity: Red,
				Summary:  "project.yaml: missing",
				Items: []Item{{
					Target: path,
					Detail: "file does not exist at the canonical location for the active storage mode",
				}},
				Hint: projectYAMLFixHint,
			}}, nil
		}
		return []Finding{{
			Severity: Red,
			Summary:  "project.yaml: unreadable",
			Items: []Item{{
				Target: path,
				Detail: err.Error(),
			}},
			Hint: projectYAMLFixHint,
		}}, nil
	}

	var shape projectYAMLShape
	if err := yaml.Unmarshal(data, &shape); err != nil {
		return []Finding{{
			Severity: Red,
			Summary:  "project.yaml: invalid YAML",
			Items: []Item{{
				Target: path,
				Detail: err.Error(),
			}},
			Hint: "fix the YAML syntax in project.yaml (see error detail)",
		}}, nil
	}

	if len(shape.Jigs) == 0 {
		return []Finding{{
			Severity: Red,
			Summary:  "project.yaml: declares no jigs",
			Items: []Item{{
				Target: path,
				Detail: "the `jigs` key is missing or empty; at least one jig must be declared",
			}},
			Hint: "add a `jigs:` list to project.yaml (see specs/architecture.md)",
		}}, nil
	}

	// Healthy: green. Summary mirrors the shape shown in
	// specs/commands.md §"kerf doctor" §"Output (default: compact text)".
	defaultJig := shape.DefaultJig
	if defaultJig == "" {
		defaultJig = "(unset)"
	}
	summary := fmt.Sprintf("project.yaml: present, default_jig=%s, %d jigs declared", defaultJig, len(shape.Jigs))
	return []Finding{{
		Severity: Green,
		Summary:  summary,
	}}, nil
}

func init() { Register(projectYAMLDetector{}) }
