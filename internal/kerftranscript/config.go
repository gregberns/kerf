package kerftranscript

// Shared config-load helpers for the diagnostics family.
//
// Implements specs/commands.md §"Warning kinds" → `corrupt_project_config`
// and specs/coordination.md §"Feed-warning rules" row for
// `corrupt_project_config`: a single emission per `kerf next` invocation,
// shared by D1 (`abandoned_dispatch`) and D6 (`reviewer_absent`). Both
// detectors must consume the same compiled result so they can be uniformly
// disabled when the pattern is malformed.

import (
	"regexp"
	"strings"
)

// BeadIDPatternResult is the outcome of compiling `bead.id_pattern` from
// project.yaml. Exactly one of (Pattern != nil) or (CompileError != "")
// will be set when Configured is true; when Configured is false both are
// zero (the project simply did not declare a pattern — not a corruption).
type BeadIDPatternResult struct {
	// Configured is true iff project.yaml set a non-empty `bead.id_pattern`.
	// When false, D1 silently no-ops per its existing contract; this is
	// NOT a `corrupt_project_config` condition.
	Configured bool
	// Pattern is the compiled regex when compilation succeeded; nil
	// otherwise.
	Pattern *regexp.Regexp
	// CompileError is the verbatim regex compile error message when
	// compilation failed; empty otherwise. The `corrupt_project_config`
	// warning's `reason` field embeds this verbatim per
	// specs/commands.md §`corrupt_project_config`.
	CompileError string
}

// Corrupt reports whether the configured pattern failed to compile.
// When true, both D1 and D6 must be disabled for this invocation per
// specs/coordination.md §"Feed-warning rules" → `corrupt_project_config`.
func (r BeadIDPatternResult) Corrupt() bool {
	return r.Configured && r.Pattern == nil && r.CompileError != ""
}

// CompileBeadIDPattern is the single shared entry point that both D1 and
// D6 consume to obtain the project's `bead.id_pattern` regex. The input
// is the raw string from project.yaml (typically obtained via
// (*config.ProjectConfig).BeadIDPattern()); empty strings are reported
// as "not configured" and are not a corruption.
//
// Per specs/commands.md §`corrupt_project_config`: this is the
// shared config-load layer; D1 and D6 must NOT compile the pattern
// independently. A single emission of the `corrupt_project_config`
// warning per `kerf next` invocation covers both detectors.
func CompileBeadIDPattern(patternStr string) BeadIDPatternResult {
	patternStr = strings.TrimSpace(patternStr)
	if patternStr == "" {
		return BeadIDPatternResult{Configured: false}
	}
	re, err := regexp.Compile(patternStr)
	if err != nil {
		return BeadIDPatternResult{
			Configured:   true,
			CompileError: err.Error(),
		}
	}
	return BeadIDPatternResult{
		Configured: true,
		Pattern:    re,
	}
}
