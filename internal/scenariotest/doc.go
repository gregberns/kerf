// Package scenariotest provides a real-binary scenario test harness for kerf.
//
// Per specs/testing.md ("Scenario tests" section), scenario tests compile the
// real kerf binary once per `go test` invocation and drive it as a subprocess
// against a real `bd`-shaped bead store and per-scenario tempdir + HOME.
// The harness scrubs KERF_* and BD_* env from the parent process, skips
// scenarios with a clear message when `bd` is not on PATH, and exposes
// subprocess + filesystem helpers for scenario authors.
//
// This package is internal-only. Scenarios live in sibling packages and use
// the Runner via the New function.
package scenariotest
