package scenario_import

import "github.com/gregberns/kerf/internal/sim/scenario"

// loadAndValidate is a thin test helper that re-loads a scenario YAML
// from disk through the production scenario.Load path. Kept here so the
// tests don't import the production package twice with different alias
// idioms.
func loadAndValidate(path string) (*scenario.Scenario, error) {
	return scenario.Load(path)
}
