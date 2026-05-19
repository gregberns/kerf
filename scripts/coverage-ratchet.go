// coverage-ratchet enforces per-package coverage floors.
//
// Plan 024, bead kerf-9vkt. Reads a Go coverage profile (default: coverage.out)
// and the checked-in floor file (.github/coverage.floor.json). Aggregates
// statements covered / total per Go package, compares against the floor, and
// exits non-zero on any regression.
//
// Usage:
//
//	go run ./scripts/coverage-ratchet.go [-profile coverage.out] [-floor .github/coverage.floor.json]
//
// Workflow:
//
//   - Authors who raise coverage MUST bump the floor in the same PR. The
//     ratchet does not auto-bump — bumping is an intentional commit so reviewers
//     see "this package can no longer regress past N%".
//   - Packages absent from the floor file are reported but never fail the build,
//     so new packages don't break CI before a follow-up PR adds their floor.
//   - Set KERF_RATCHET_WARN_ONLY=1 to downgrade failures to warnings during
//     stabilization (Plan 024 OQ2: default hard-fail, env override available).
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
)

// floorFile mirrors .github/coverage.floor.json. The "_comment" field in the
// JSON is ignored by this struct (json.Unmarshal silently skips unknown fields).
type floorFile struct {
	Floors map[string]float64 `json:"floors"`
}

// pkgStats accumulates statement counts for a single Go package.
type pkgStats struct {
	totalStmts   int64
	coveredStmts int64
}

func main() {
	var (
		profilePath = flag.String("profile", "coverage.out", "path to go test -coverprofile output")
		floorPath   = flag.String("floor", ".github/coverage.floor.json", "path to coverage floor JSON")
		tolerance   = flag.Float64("tolerance", 0.05, "percentage points allowed below floor before failing (absorbs rounding)")
	)
	flag.Parse()

	warnOnly := os.Getenv("KERF_RATCHET_WARN_ONLY") == "1"

	floors, err := loadFloors(*floorPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "coverage-ratchet: load floors: %v\n", err)
		os.Exit(2)
	}

	current, err := parseProfile(*profilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "coverage-ratchet: parse profile: %v\n", err)
		os.Exit(2)
	}

	pkgs := make([]string, 0, len(current))
	for p := range current {
		pkgs = append(pkgs, p)
	}
	sort.Strings(pkgs)

	type regression struct {
		pkg     string
		current float64
		floor   float64
	}
	var regressions []regression
	var unfloored []string

	fmt.Printf("coverage-ratchet: %d packages in profile, %d floors\n", len(current), len(floors))
	for _, p := range pkgs {
		s := current[p]
		if s.totalStmts == 0 {
			continue
		}
		pct := 100.0 * float64(s.coveredStmts) / float64(s.totalStmts)
		floor, ok := floors[p]
		if !ok {
			unfloored = append(unfloored, fmt.Sprintf("  %s: %.1f%% (no floor — add to coverage.floor.json)", p, pct))
			continue
		}
		marker := "OK "
		if pct+*tolerance < floor {
			marker = "FAIL"
			regressions = append(regressions, regression{p, pct, floor})
		}
		fmt.Printf("  [%s] %-60s %5.1f%%  (floor %.1f%%)\n", marker, p, pct, floor)
	}

	if len(unfloored) > 0 {
		fmt.Fprintln(os.Stderr, "\ncoverage-ratchet: packages with no floor (informational):")
		for _, line := range unfloored {
			fmt.Fprintln(os.Stderr, line)
		}
	}

	if len(regressions) == 0 {
		fmt.Println("\ncoverage-ratchet: all packages at or above floor.")
		return
	}

	fmt.Fprintf(os.Stderr, "\ncoverage-ratchet: %d package(s) below floor:\n", len(regressions))
	for _, r := range regressions {
		fmt.Fprintf(os.Stderr, "  %s: %.1f%% < floor %.1f%%\n", r.pkg, r.current, r.floor)
	}
	if warnOnly {
		fmt.Fprintln(os.Stderr, "\nKERF_RATCHET_WARN_ONLY=1 set; exiting 0 despite regressions.")
		return
	}
	fmt.Fprintln(os.Stderr, "\nFix coverage or, if the regression is intentional, lower the floor in .github/coverage.floor.json (with reviewer sign-off).")
	os.Exit(1)
}

func loadFloors(path string) (map[string]float64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f floorFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	if len(f.Floors) == 0 {
		return nil, fmt.Errorf("%s: no floors defined", path)
	}
	return f.Floors, nil
}

// parseProfile reads a Go coverage profile and returns statements covered /
// total per package import path. The profile format is documented in
// `go help testflag` and `cmd/cover/profile.go`:
//
//	mode: <mode>
//	<file>:<startLine>.<startCol>,<endLine>.<endCol> <numStmt> <count>
//
// Lines with count > 0 contribute to the covered total. The package is the
// directory portion of the file path (which is a full import path under
// modules).
func parseProfile(profilePath string) (map[string]*pkgStats, error) {
	f, err := os.Open(profilePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	out := make(map[string]*pkgStats)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		if lineNo == 1 && strings.HasPrefix(line, "mode:") {
			continue
		}
		if line == "" {
			continue
		}
		// Format: file:start.col,end.col numStmt count
		colon := strings.Index(line, ":")
		if colon < 0 {
			return nil, fmt.Errorf("line %d: missing ':'", lineNo)
		}
		file := line[:colon]
		rest := line[colon+1:]
		fields := strings.Fields(rest)
		if len(fields) < 3 {
			return nil, fmt.Errorf("line %d: expected 3 fields after file, got %q", lineNo, rest)
		}
		numStmt, err := strconv.ParseInt(fields[len(fields)-2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: numStmt: %w", lineNo, err)
		}
		count, err := strconv.ParseInt(fields[len(fields)-1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: count: %w", lineNo, err)
		}
		pkg := path.Dir(file)
		s, ok := out[pkg]
		if !ok {
			s = &pkgStats{}
			out[pkg] = s
		}
		s.totalStmts += numStmt
		if count > 0 {
			s.coveredStmts += numStmt
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
