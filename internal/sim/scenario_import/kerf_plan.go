package scenario_import

import "fmt"

// ImportKerfPlan is a stub for converting a kerf plan directory
// (plans/NNN_name/{_plan.md,beads.md}) into a scenario document.
//
// TODO(plan-012/A): implement once the harmonik path stabilises. The
// `beads.md` markdown format is not yet machine-readable in a stable
// schema, and the immediate Plan 012 goal is harmonik-pilot ingestion;
// this stub keeps the CLI surface honest until then.
func ImportKerfPlan(source string) (*ImportResult, error) {
	return nil, fmt.Errorf("scenario_import: kerf-plan import not implemented (Plan 012 / A follow-up); source=%s", source)
}
