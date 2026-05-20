// Package doctor — validation-section-coverage detector.
//
// Spec reference:
//   - specs/commands.md §"kerf doctor" §Detectors `validation-section-coverage`
//     (line 1601): the detector reports each active work using a plan / spec /
//     bug / implementation jig whose normative planning artifact does not list
//     both a scenario-test item ID and an exploratory-test item ID in its
//     "What done looks like" checklist. Severity yellow. Exit 0 regardless of
//     findings.
//   - specs/jig-system.md §"Validation-test requirement" (line ~346): canonical
//     sentence; lists the affected passes per jig and the retrofit/spike
//     exclusion.
//   - plans/025_jig_validation_section/_plan.md (B4): bead scope.
//
// Spec ambiguity reconciled in code (kerf-ystq):
//   The spec does not pin the affected-pass→artifact-filename mapping. The
//   mapping below is read from the built-in jig markdown bodies under
//   internal/jig/builtin/ (which the orchestrator/reviewer can re-check against
//   internal/jig/builtin/{plan,spec,bug,implementation}.md):
//
//     plan jig            Change Spec   05-specs/*-spec.md       (per-component, glob)
//     plan jig            Tasks         07-tasks.md
//     spec jig            Spec Draft    05-spec-drafts/*.md      (per-component, glob)
//     spec jig            Tasks         07-tasks.md
//     bug jig             Fix Spec      05-fix-spec.md
//     implementation jig  Breakdown     01-breakdown.md          (creation point)
//     implementation jig  Verify        03-verify.md             (closure-check)
//
// Per-jig artifact patterns are intentionally hard-coded in v1; if jig markdown
// bodies change the file_structure, this table must be updated in lockstep.

package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/gberns/kerf/internal/spec"
)

// validationSectionCoverageDetector is the registered Detector.
type validationSectionCoverageDetector struct{}

func (validationSectionCoverageDetector) ID() string { return "validation-section-coverage" }

// affectedArtifact names one artifact file (or glob) that the spec marks as
// "normative planning artifact" for a given (jig, pass) pair. A glob ending
// in `.md` whose stem contains `*` is expanded with filepath.Glob relative to
// the work directory; otherwise the path is taken literally.
type affectedArtifact struct {
	Pass string // human label, e.g. "Change Spec"
	Path string // path under the work directory; may contain `*`
}

// affectedByJig is the load-bearing mapping (see header comment for rationale
// and source).
var affectedByJig = map[string][]affectedArtifact{
	"plan": {
		{Pass: "Change Spec", Path: "05-specs/*-spec.md"},
		{Pass: "Tasks", Path: "07-tasks.md"},
	},
	"spec": {
		{Pass: "Spec Draft", Path: "05-spec-drafts/*.md"},
		{Pass: "Tasks", Path: "07-tasks.md"},
	},
	"bug": {
		{Pass: "Fix Spec", Path: "05-fix-spec.md"},
	},
	"implementation": {
		{Pass: "Breakdown", Path: "01-breakdown.md"},
		{Pass: "Verify", Path: "03-verify.md"},
	},
}

// excludedJigs are jigs the spec explicitly removes from this requirement
// (specs/jig-system.md §"Retrofit and spike exclusion").
var excludedJigs = map[string]bool{
	"retrofit": true,
	"spike":    true,
}

// archivedStatus is the spec.yaml `status` value that excludes a work. Archived
// works are also typically housed under the archive/ tree (and thus do not
// appear in Resolver.ListWorks), so this check is a belt-and-braces measure
// for any active-tree work whose status happens to read "archived".
const archivedStatus = "archived"

// What done looks like — section heading regex (case-insensitive, matches
// "What done looks like" with optional trailing punctuation, as a markdown
// heading or as a bold-emphasis sub-heading: both occur in built-in jigs).
var wdllHeadingRE = regexp.MustCompile(`(?im)^\s{0,3}(#{1,6}\s+|\*\*)\s*What done looks like`)

// scenarioItemRE / exploratoryItemRE match a checklist line under the
// "What done looks like" block. We are permissive on phrasing so that the
// "filed with ID" (creation point) and "with ID … is closed" (closure-check)
// shapes both match.
//
// The regexes require a list-item prefix (leading whitespace + `-` or `*`
// bullet + whitespace) so that free prose inside the WDLL block that merely
// mentions the marker phrase (e.g. "Some description of Scenario-test item
// flow") does NOT count as a checklist item. Without this anchoring, the
// detector flipped findings from `missing` to `emptyBoth` when prose
// happened to mention the phrase. See plan-025-ystq-fu2 (kerf-jka2).
var scenarioItemRE = regexp.MustCompile(`(?im)^\s*[-*]\s+Scenario-test item`)
var exploratoryItemRE = regexp.MustCompile(`(?im)^\s*[-*]\s+Exploratory-test item`)

// idBacktickRE captures the first backtick-quoted token on the line. The
// canonical shape is `<id>` (literal placeholder) or `kerf-xxx` (real ID).
var idBacktickRE = regexp.MustCompile("`([^`]*)`")

// missing kind for an artifact.
type missingKind int

const (
	missingBlock         missingKind = iota // "What done looks like" block absent
	missingScenarioOnly                     // checklist item missing
	missingExploratoryOnly
	missingBoth
	emptyScenarioOnly  // item present but `<id>` empty/placeholder
	emptyExploratoryOnly
	emptyBoth
	emptyScenarioMissingExploratory
	emptyExploratoryMissingScenario
)

func (k missingKind) detail() string {
	switch k {
	case missingBlock:
		return "no 'What done looks like' block found"
	case missingScenarioOnly:
		return "missing scenario-test item"
	case missingExploratoryOnly:
		return "missing exploratory-test item"
	case missingBoth:
		return "missing both scenario-test and exploratory-test items"
	case emptyScenarioOnly:
		return "scenario-test item has empty <id>"
	case emptyExploratoryOnly:
		return "exploratory-test item has empty <id>"
	case emptyBoth:
		return "both scenario-test and exploratory-test items have empty <id>"
	case emptyScenarioMissingExploratory:
		return "scenario-test item has empty <id>; exploratory-test item missing"
	case emptyExploratoryMissingScenario:
		return "exploratory-test item has empty <id>; scenario-test item missing"
	}
	return "validation items incomplete"
}

// analyzeArtifact reads a single artifact file and returns the finding kind,
// or (false, _) when the artifact is compliant.
func analyzeArtifact(path string) (bool, missingKind, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, 0, err
	}
	text := string(data)
	loc := wdllHeadingRE.FindStringIndex(text)
	if loc == nil {
		return true, missingBlock, nil
	}
	// Block body = from the heading to either EOF or the next heading of the
	// same-or-higher level. For simplicity (and to match how the built-in jigs
	// are written), we scan from the heading to the next `^## ` or `^---` or
	// EOF.
	rest := text[loc[1]:]
	if nextHead := regexp.MustCompile(`(?m)^(##\s|---\s*$)`).FindStringIndex(rest); nextHead != nil {
		rest = rest[:nextHead[0]]
	}
	scenLine := scenarioItemRE.FindString(rest)
	explLine := exploratoryItemRE.FindString(rest)
	// Locate the full line for each match so we can read its backtick id.
	scenID, scenFound := lineID(rest, scenarioItemRE)
	explID, explFound := lineID(rest, exploratoryItemRE)

	_ = scenLine
	_ = explLine

	switch {
	case !scenFound && !explFound:
		return true, missingBoth, nil
	case !scenFound && explFound:
		if isPlaceholderID(explID) {
			return true, emptyExploratoryMissingScenario, nil
		}
		return true, missingScenarioOnly, nil
	case scenFound && !explFound:
		if isPlaceholderID(scenID) {
			return true, emptyScenarioMissingExploratory, nil
		}
		return true, missingExploratoryOnly, nil
	}
	// Both lines found — check id validity.
	scenEmpty := isPlaceholderID(scenID)
	explEmpty := isPlaceholderID(explID)
	switch {
	case scenEmpty && explEmpty:
		return true, emptyBoth, nil
	case scenEmpty:
		return true, emptyScenarioOnly, nil
	case explEmpty:
		return true, emptyExploratoryOnly, nil
	}
	return false, 0, nil
}

// lineID finds the line in body containing the first match of itemRE and
// returns the first backtick-quoted token on that line (or "" if none).
func lineID(body string, itemRE *regexp.Regexp) (string, bool) {
	idx := itemRE.FindStringIndex(body)
	if idx == nil {
		return "", false
	}
	// Expand to the line containing the match.
	start := strings.LastIndexByte(body[:idx[0]], '\n') + 1
	end := idx[1]
	if nl := strings.IndexByte(body[end:], '\n'); nl >= 0 {
		end = end + nl
	} else {
		end = len(body)
	}
	line := body[start:end]
	m := idBacktickRE.FindStringSubmatch(line)
	if m == nil {
		return "", true
	}
	return m[1], true
}

// isPlaceholderID returns true when the captured id is the literal "<id>"
// placeholder, empty, or only whitespace. Any other token is treated as a
// real tracker id.
func isPlaceholderID(id string) bool {
	t := strings.TrimSpace(id)
	return t == "" || t == "<id>"
}

func (validationSectionCoverageDetector) Run(ctx *Context) ([]Finding, error) {
	if ctx == nil || ctx.Resolver == nil {
		return nil, fmt.Errorf("validation-section-coverage: nil context or resolver")
	}
	r := ctx.Resolver

	codenames, err := r.ListWorks()
	if err != nil {
		return nil, fmt.Errorf("validation-section-coverage: listing works: %w", err)
	}
	sort.Strings(codenames)

	type offender struct {
		codename string
		pass     string
		file     string // path relative to work directory
		kind     missingKind
	}
	var offenders []offender

	for _, codename := range codenames {
		workDir := r.WorkDir(codename)
		specPath := filepath.Join(workDir, "spec.yaml")
		if _, statErr := os.Stat(specPath); statErr != nil {
			continue // not this detector's concern; storage-drift covers missing spec.yaml
		}
		s, rerr := spec.Read(specPath)
		if rerr != nil {
			continue // malformed spec.yaml — other detectors surface this
		}
		if excludedJigs[s.Jig] {
			continue
		}
		if s.Status == archivedStatus {
			continue
		}
		artifacts, ok := affectedByJig[s.Jig]
		if !ok {
			continue
		}
		for _, art := range artifacts {
			matches, gerr := resolveArtifactPaths(workDir, art.Path)
			if gerr != nil {
				continue
			}
			if len(matches) == 0 {
				// Artifact not on disk yet (pass not run). Per spec: a
				// finding is emitted when an affected-pass artifact exists
				// and is missing items, etc. No artifact ⇒ no finding.
				continue
			}
			for _, abs := range matches {
				bad, kind, aerr := analyzeArtifact(abs)
				if aerr != nil {
					continue
				}
				if !bad {
					continue
				}
				rel, err := filepath.Rel(workDir, abs)
				if err != nil {
					rel = filepath.Base(abs)
				}
				offenders = append(offenders, offender{
					codename: codename,
					pass:     art.Pass,
					file:     filepath.ToSlash(rel),
					kind:     kind,
				})
			}
		}
	}

	if len(offenders) == 0 {
		return []Finding{{
			Severity: Green,
			Summary:  "validation-section-coverage: all affected-pass artifacts list scenario and exploratory IDs",
		}}, nil
	}

	items := make([]Item, 0, len(offenders))
	for _, o := range offenders {
		items = append(items, Item{
			Target: fmt.Sprintf("%s (%s)", o.codename, o.pass),
			Detail: fmt.Sprintf("%s — %s", o.file, o.kind.detail()),
		})
	}
	// Hint references the first offender's file as a canonical pointer; the
	// per-item Detail names every offending file so the operator has the full
	// list. Per spec: "add the two items to <file> § What done looks like".
	hint := fmt.Sprintf("add the two items to %s § What done looks like", offenders[0].file)

	return []Finding{{
		Severity: Yellow,
		Summary:  fmt.Sprintf("validation-section-coverage: %d artifact(s) missing scenario/exploratory IDs", len(offenders)),
		Items:    items,
		Hint:     hint,
	}}, nil
}

// resolveArtifactPaths expands a path that may contain `*` against workDir.
// A glob with zero matches returns ([], nil) — the caller treats that as
// "artifact not yet produced".
func resolveArtifactPaths(workDir, rel string) ([]string, error) {
	abs := filepath.Join(workDir, filepath.FromSlash(rel))
	if strings.ContainsAny(rel, "*?[") {
		matches, err := filepath.Glob(abs)
		if err != nil {
			return nil, err
		}
		sort.Strings(matches)
		return matches, nil
	}
	if _, err := os.Stat(abs); err != nil {
		return nil, nil
	}
	return []string{abs}, nil
}

func init() { Register(validationSectionCoverageDetector{}) }
