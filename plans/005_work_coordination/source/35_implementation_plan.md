# Phase 1 Implementation Plan

> Code-level breakdown for: areas system, `kerf map`, `kerf next`, overlap detection, basic beads integration.

---

## 1. Areas Package — `internal/areas/areas.go`

**Create** `internal/areas/areas.go` and `internal/areas/areas_test.go`

**What it does:**
- Defines `AreasYAML` struct: `Areas []Area` where `Area` has `Name string` and `Description string`
- `Load(path string) (*AreasYAML, error)` — reads `areas.yaml`, returns empty struct if missing
- `Save(path string, a *AreasYAML) error` — writes `areas.yaml`
- `Add(a *AreasYAML, name, description string) error` — validates name (lowercase, alphanumeric+hyphens), checks for duplicates, appends
- `Remove(a *AreasYAML, name string) error` — removes by name, errors if not found
- `Validate(a *AreasYAML, names []string) error` — checks that all names in a list exist in the taxonomy
- `AreasPath(benchPath, projectID string) string` — returns `~/.kerf/projects/{projectID}/areas.yaml`

**Storage:** `~/.kerf/projects/{projectID}/areas.yaml` — one file per project, alongside work directories.

**Dependencies:** None. Standalone package.

**Complexity:** Small

---

## 2. Add `areas` Field to `spec.yaml`

**Modify** `internal/spec/spec.go`

**What changes:**
- Add `Areas []string \`yaml:"areas,omitempty"\`` field to `SpecYAML` struct (after DependsOn)

That's it. The field is optional. Existing spec.yaml files without it will unmarshal with `nil` Areas, which is correct.

**Dependencies:** None.

**Complexity:** Small

---

## 3. `kerf areas` Subcommand

**Create** `cmd/areas.go` and `cmd/areas_test.go`

**What it does:**
- `kerf areas list` — loads areas.yaml for current project, prints name + description table
- `kerf areas add <name> [--description "..."]` — adds an area to areas.yaml
- `kerf areas remove <name>` — removes an area from areas.yaml (warns if in-flight works reference it)

Uses cobra subcommand pattern (like `kerf jig list`). Resolves project via `cmdutil.ResolveProject`. Loads/saves via `internal/areas`.

**Dependencies:** Bead 1 (areas package), Bead 2 (areas field in spec.yaml)

**Complexity:** Medium

---

## 4. Overlap Detection in `kerf new`

**Modify** `cmd/new.go`

**What changes:**
- After work creation, if the work has areas (via a new `--areas` flag on `kerf new`), scan all other active works in the project for shared areas
- For each shared area, print a warning: `Warning: area "api" also active in work "blue-fox" (status: research)`
- Add `--areas` flag: comma-separated list of area names. Validated against areas.yaml.
- Set `s.Areas` on the new spec before writing

Helper function: `findOverlappingWorks(bp, projectID string, areas []string, excludeCodename string) []overlapEntry` — reusable by other commands. Lives in `internal/areas/overlap.go`.

**Dependencies:** Bead 1 (areas package), Bead 2 (areas field)

**Complexity:** Medium

---

## 5. Beads Integration Package — `internal/beads/beads.go`

**Create** `internal/beads/beads.go` and `internal/beads/beads_test.go`

**What it does:**
- Shells out to `br` CLI to get bead data. All `br` interaction in one place.
- `type Bead struct` — ID, Title, Status, Epic, Labels, DependsOn
- `List(projectID string) ([]Bead, error)` — runs `br list --json [--project X]`, parses JSON
- `CountByEpic(beads []Bead) map[string]EpicSummary` — groups beads by epic, counts open/closed per epic
- `Available(beads []Bead) []Bead` — filters to beads whose deps are all complete and status is pending/open
- `IsAvailable() bool` — checks if `br` binary is on PATH (for graceful degradation)

The existing `getBeadSummary` in `cmd/show.go` and `tryLoadBeadInfo` in `cmd/square.go` can be refactored to use this package later, but that is not in scope for Phase 1.

**Dependencies:** None. Standalone package.

**Complexity:** Medium

---

## 6. `kerf map` Command

**Create** `cmd/map.go` and `cmd/map_test.go`

**What it does:**
- Loads all works for current project (reuses pattern from `cmd/list.go`)
- Groups works by area. Works with no areas go under "(unassigned)".
- For each work: shows codename, status, title (if any), updated time
- If beads integration available: queries `br list --json` once, aggregates bead counts per work (by epic/label matching), shows "N/M beads complete" per work
- Output format:
  ```
  Map for project-name:

  api:
    blue-fox     research    "Auth rewrite"       3/10 beads
    red-elk      implementing "Rate limiting"      0/5 beads

  database:
    blue-fox     research    "Auth rewrite"       3/10 beads

  (unassigned):
    green-owl    spec        "Logging cleanup"
  ```
- A work touching multiple areas appears under each area

**Dependencies:** Bead 1 (areas package), Bead 2 (areas in spec), Bead 5 (beads integration)

**Complexity:** Medium

---

## 7. Queue Algorithm — `internal/queue/queue.go`

**Create** `internal/queue/queue.go` and `internal/queue/queue_test.go`

**What it does:**
- `type QueueEntry struct` — Codename, Title, Areas, Status, Score, Reason (why it ranked here)
- `Compute(works []*spec.SpecYAML, beads []beads.Bead) []QueueEntry`
  1. Filter out: works in terminal/blocked/shelved status
  2. Filter out: works with incomplete must-complete-first dependencies
  3. Score remaining works:
     - **Dependency depth** — works that unblock the most other works score higher. Computed by counting transitive dependents across all project works.
     - **Completion momentum** — works whose beads are partially done (in-progress epic with N/M complete) score higher as M approaches N. Ratio-based: `completed / total` weighted.
     - **Creation order** — tiebreaker, older first
  4. Sort by score descending
  5. Return ordered list with score explanation

The algorithm is deliberately simple and in one file. Parameters (weights for each factor) are constants at the top of the file with comments marking them as future config candidates.

**Dependencies:** Bead 2 (areas in spec), Bead 5 (beads integration)

**Complexity:** Medium

---

## 8. `kerf next` Command

**Create** `cmd/next.go` and `cmd/next_test.go`

**What it does:**
- Loads all works for current project
- Loads beads via `internal/beads` (graceful if unavailable)
- Calls `queue.Compute(works, beads)`
- Prints ordered list:
  ```
  Next up for project-name:

  1. blue-fox     research    [api, database]   "Auth rewrite"
     Unblocks 3 works. 3/10 beads complete.

  2. red-elk      implementing [api]             "Rate limiting"
     In-progress epic near completion (8/10 beads).

  3. green-owl    spec        []                 "Logging cleanup"
     No dependencies. Created 3d ago.
  ```
- Optional `--limit N` flag (default: show all)
- Optional `--area <name>` flag to filter to works touching a specific area

**Dependencies:** Bead 6 (map, for pattern), Bead 7 (queue algorithm)

**Complexity:** Medium

---

## 9. Update `kerf show` with Areas

**Modify** `cmd/show.go`

**What changes:**
- After printing Type/Status/Project, print `Areas: api, database` (or `Areas: (none)` if empty)
- After the bead summary section, if areas are set, show overlap info: other active works sharing areas (reuse `findOverlappingWorks` from Bead 4)

**Dependencies:** Bead 1 (areas package), Bead 2 (areas in spec), Bead 4 (overlap detection)

**Complexity:** Small

---

## 10. Tests and Spec Alignment Verification

**Create/modify** test files for all new packages.

Each bead above includes its own `_test.go`. This bead covers:
- Integration test in `cmd/e2e_test.go` or a new `cmd/coordination_e2e_test.go`: create project with areas, create two works in same area (verify overlap warning), run `kerf map`, run `kerf next`
- Verify the queue algorithm with known inputs produces expected ordering

**Dependencies:** All previous beads

**Complexity:** Medium

---

## Dependency Graph

```
Bead 1 (areas pkg) ─────┬──> Bead 3 (kerf areas cmd)
                         ├──> Bead 4 (overlap detection in kerf new)
                         ├──> Bead 6 (kerf map)
                         └──> Bead 9 (kerf show + areas)

Bead 2 (spec.yaml areas) ┬──> Bead 3
                          ├──> Bead 4
                          ├──> Bead 6
                          ├──> Bead 7 (queue algorithm)
                          └──> Bead 9

Bead 5 (beads pkg) ──────┬──> Bead 6
                          └──> Bead 7

Bead 7 (queue algorithm) ──> Bead 8 (kerf next)

Bead 6 (kerf map) ─────────> Bead 8 (pattern reference only, not hard dep)

Beads 1-9 ────────────────> Bead 10 (integration tests)
```

Parallelizable: Beads 1+2 can run in parallel. Bead 5 can run in parallel with 1+2. Beads 3, 4, 6, 7 can start once their deps complete.

---

## Beads YAML

```yaml
beads:
  - id: coord-001
    title: "Areas package — internal/areas"
    description: |
      Create internal/areas/areas.go and areas_test.go.
      Structs: AreasYAML, Area. Functions: Load, Save, Add, Remove, Validate, AreasPath.
      Storage at ~/.kerf/projects/{projectID}/areas.yaml.
      Validate area names: lowercase alphanumeric + hyphens.
    depends_on: []

  - id: coord-002
    title: "Add areas field to spec.yaml"
    description: |
      Modify internal/spec/spec.go: add Areas []string field to SpecYAML struct.
      Field is optional (omitempty). Update spec_test.go to cover round-trip with areas.
    depends_on: []

  - id: coord-003
    title: "kerf areas subcommand (list/add/remove)"
    description: |
      Create cmd/areas.go and cmd/areas_test.go.
      Subcommands: kerf areas list, kerf areas add <name> [--description], kerf areas remove <name>.
      Uses cobra subcommand pattern. Resolves project via cmdutil.ResolveProject.
      Remove warns if in-flight works reference the area.
    depends_on: [coord-001, coord-002]

  - id: coord-004
    title: "Overlap detection at kerf new time"
    description: |
      Create internal/areas/overlap.go with findOverlappingWorks(bp, projectID, areas, excludeCodename).
      Modify cmd/new.go: add --areas flag (comma-separated, validated against areas.yaml).
      Set spec.Areas before writing. After creation, print warnings for shared areas.
    depends_on: [coord-001, coord-002]

  - id: coord-005
    title: "Beads integration package — internal/beads"
    description: |
      Create internal/beads/beads.go and beads_test.go.
      Struct: Bead (ID, Title, Status, Epic, Labels, DependsOn).
      Functions: List (shells out to br list --json), CountByEpic, Available, IsAvailable.
      Graceful degradation when br is not on PATH.
    depends_on: []

  - id: coord-006
    title: "kerf map command"
    description: |
      Create cmd/map.go and cmd/map_test.go.
      Loads all works for current project, groups by area.
      Works with no areas go under "(unassigned)".
      If beads available, shows bead progress per work.
      A work touching multiple areas appears under each.
    depends_on: [coord-001, coord-002, coord-005]

  - id: coord-007
    title: "Queue algorithm — internal/queue"
    description: |
      Create internal/queue/queue.go and queue_test.go.
      QueueEntry struct with Codename, Title, Areas, Status, Score, Reason.
      Compute function: filter terminal/blocked/shelved, filter incomplete deps,
      score by dependency depth + completion momentum + creation order tiebreak.
      Weights as constants at top of file. Algorithm in one function.
    depends_on: [coord-002, coord-005]

  - id: coord-008
    title: "kerf next command"
    description: |
      Create cmd/next.go and cmd/next_test.go.
      Loads works + beads, calls queue.Compute, prints ordered list.
      Each entry shows codename, status, areas, title, and reason for ranking.
      Flags: --limit N, --area <name>.
    depends_on: [coord-007]

  - id: coord-009
    title: "Update kerf show with areas display"
    description: |
      Modify cmd/show.go: print Areas line after Type/Status/Project.
      Show overlap info (other active works sharing areas) using findOverlappingWorks.
    depends_on: [coord-001, coord-002, coord-004]

  - id: coord-010
    title: "Integration tests for coordination features"
    description: |
      Create cmd/coordination_e2e_test.go (or extend e2e_test.go).
      Test flow: create areas, create two works sharing an area (verify overlap warning),
      run kerf map (verify grouping), run kerf next (verify ordering).
      Queue algorithm unit tests with known inputs and expected ordering.
    depends_on: [coord-003, coord-004, coord-006, coord-008, coord-009]
```
