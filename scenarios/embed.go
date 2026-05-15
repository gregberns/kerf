// Package scenarios ships the canned scenario YAML files used by kerfsim.
//
// The files are compiled into the binary via embed.FS so that
// `kerfsim run small-linear` works without a file path on disk. See
// specs/simulator.md §Synthetic Generator (three canned scenarios) and
// plans/007_simulator/beads.md (B12).
package scenarios

import "embed"

//go:embed *.yaml
var fs embed.FS

// FS returns the embedded filesystem containing the canned scenario YAML
// files (small-linear.yaml, wide-fanout.yaml, rework-heavy.yaml).
func FS() embed.FS { return fs }
