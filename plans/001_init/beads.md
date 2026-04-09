# Implementation Beads — Plan 001

> Ordered, parallelizable implementation tasks derived from specs.
> Revised after 3-agent review (spec coverage, Go architecture, parallelization).

## Dependency Graph

```
L0 (scaffold)
 └─► L1a (spec.yaml types)  ─┐
 └─► L1b (config types)      ├─► L2a (bench engine)    ─┐
 └─► L1c (jig + built-ins)   │   L2b (snapshot engine)  ├─► L2.5 (test infra) ─► L3a-3i (commands)
 └─► L1d (codename gen)      │   L2c (session engine)   │
 └─► L1e (project ID)       ─┘   L2d (dep resolver)    ─┘
```

## Inter-Package Import Map

```
cmd/*           → internal/{bench,jig,snapshot,session,config,codename,project,spec,dep}
internal/bench  → internal/spec (reads spec.yaml to list works)
internal/session → internal/spec (mutates SpecYAML session fields)
internal/snapshot → internal/spec (reads spec.yaml for status in snapshots)
internal/dep    → internal/spec (reads dependency status)
internal/jig    → (no internal deps — standalone parsing + embed)
internal/config → (no internal deps — standalone)
internal/codename → (no internal deps — standalone)
internal/project → (no internal deps — standalone, shells out to git)
internal/spec   → (no internal deps — leaf package)
```

No cycles. All arrows point toward `internal/spec` (leaf).

## Cross-Cutting Concerns

These behaviors span multiple commands and must be wired into every relevant command bead:

1. **Global `--project` flag** — persistent cobra flag on root command. Inference chain: `--project` flag → `.kerf/project-identifier` in nearest git root → `default_project` from config.yaml → error. Established in Bead 0, consumed by all command beads.

2. **Jig version mismatch warning** — on any command that loads a work's spec.yaml + resolves the jig, compare `spec.jig_version` vs `jig.version`. If different, emit warning. Must be wired into: show, status, resume, shelve, square, finalize. (Per jig-system.md §Versioning.)

3. **Stale session warning** — on every command that reads spec.yaml, check if `active_session` is non-null and `started` is older than threshold. Emit warning if stale. Must be wired into: show, status, list, square, snapshot, history, restore, archive, delete. (Per sessions.md §Behavior on Stale Detection.)

4. **Interval snapshot check** — on any command invocation for a work, if `snapshots.interval_enabled`, check elapsed time since last snapshot. Take snapshot if exceeded. Runs on ALL commands including read-only (show, history). (Per snapshots.md §Interval-Based Snapshots.)

5. **SESSION.md in finalize artifact copy** — finalization.md §Artifact Copying excludes `spec.yaml` and `.history/`. commands.md §finalize step 5 says "Excludes spec.yaml, SESSION.md, and .history/". **SESSION.md IS excluded** per commands.md. Bead 3e must exclude all three.

Each command bead is responsible for calling the relevant cross-cutting helpers. A shared `internal/cmdutil/` helper (created in Bead 0) provides `ResolveProject()`, `LoadWorkWithChecks()` (stale warning + jig version warning + interval snapshot) to avoid duplication.

---

## Layer 0: Scaffold

### Bead 0 — Project scaffold
**Specs:** cli.md (executable name), architecture.md (bench path), commands.md (global flags)
**Deliverables:**
- `go mod init github.com/your-org/kerf` with cobra, gopkg.in/yaml.v3 dependencies
- `main.go` — root cobra command entry point
- `cmd/root.go` — root command with `--project` persistent flag and placeholder output
- Stub command files (one per command, with empty `init()` registration):
  - `cmd/new.go`, `cmd/list.go`, `cmd/show.go`, `cmd/status.go`
  - `cmd/resume.go`, `cmd/shelve.go`, `cmd/square.go`, `cmd/finalize.go`
  - `cmd/snapshot.go`, `cmd/history.go`, `cmd/restore.go`
  - `cmd/archive.go`, `cmd/delete.go`, `cmd/config.go`, `cmd/jig.go`
- Package directories (empty `.go` files with package declarations):
  - `internal/bench/`, `internal/jig/`, `internal/snapshot/`, `internal/session/`
  - `internal/config/`, `internal/codename/`, `internal/project/`, `internal/spec/`
  - `internal/dep/`, `internal/cmdutil/`
- `internal/cmdutil/helpers.go` — stubs for `ResolveProject()`, `LoadWorkWithChecks()`
- `.gitignore` for Go binary
**Tests:** `go build ./...` succeeds, `kerf` runs and prints placeholder text

---

## Layer 1: Core Types

### Bead 1a — spec.yaml types and serialization
**Specs:** works.md (spec.yaml schema, field reference, immutability rules)
**Package:** `internal/spec/`
**Deliverables:**
- `SpecYAML` struct — all fields from works.md schema
- `Session` struct — id, started, ended, notes
- `Dependency` struct — codename, project, relationship
- `Implementation` struct — branch, pr, commits
  - Note: `pr` field exists but is never written by kerf (per finalization.md) — test this invariant
- `Read(path string) (*SpecYAML, error)` — parse spec.yaml from disk
- `Write(path string, spec *SpecYAML) error` — serialize to disk, auto-sets `updated` timestamp
- RFC 3339 timestamp handling (time.Time with YAML marshaling)
- All function signatures return `error` for filesystem operations
**Tests:** Round-trip read/write, field validation, immutability enforcement, malformed YAML error handling

### Bead 1b — config.yaml types and parsing
**Specs:** architecture.md (config schema, defaults, semantics, `default_project` field)
**Package:** `internal/config/`
**Deliverables:**
- `Config` struct — all fields from architecture.md schema, including `default_project`
- `Load(path string) (*Config, error)` — parse with defaults for missing fields/file
- `Save(path string, cfg *Config) error` — serialize
- `Get(key string) (string, error)` — dot-notation key lookup, returns string representation
- `Set(key string, value string) error` — dot-notation key write, parses string to typed value
- `ValidKeys() []string` — enumerate known keys (for unknown-key error in `kerf config`)
- Default values as constants/var for all fields
- Unknown keys ignored on YAML read (forward compatibility)
**Tests:** Missing file returns defaults, round-trip, dot-notation get/set, unknown keys, `default_project`

### Bead 1c — Jig system (parsing + built-ins + resolution)
**Specs:** jig-system.md (file format, frontmatter, markdown body, resolution order, versioning), jig-feature.md, jig-bug.md
**Package:** `internal/jig/`
**Deliverables:**
- `JigDefinition` struct — name, description, version, status_values, passes, file_structure, raw markdown body
- `Pass` struct — name, status, output list
- `Parse(content []byte) (*JigDefinition, error)` — parse YAML frontmatter + markdown body
- `PassForStatus(status string) *Pass` — look up pass by status value (nil if not found)
- `TerminalStatus() string` — last value in status_values
- `IsAtOrPastTerminal(status string) bool` — for verification/dependency checks
- `ExpandComponents(fileStructure []string, components []string) []string` — expand `{component}` placeholders
- `InstructionsForPass(passName string) string` — extract markdown section for a pass
- `VersionMismatch(specVersion int) bool` — compare jig version vs recorded version
- Built-in jig files at `internal/jig/builtin/feature.md` and `internal/jig/builtin/bug.md` (//go:embed)
- `Resolve(name string, userJigsDir string) (*JigDefinition, string, error)` — resolution order: user-level → built-in. Returns (jig, source, error) where source is "user" or "built-in"
- `ListAll(userJigsDir string) ([]JigSummary, error)` — enumerate available jigs with source, user overrides built-in
- `SaveToUser(name string, content []byte, userJigsDir string) error` — write to ~/.kerf/jigs/
**Tests:** Parse feature/bug jig, component expansion, status lookup, malformed input, resolution order, user override, list with mixed sources, version mismatch detection

### Bead 1d — Codename generation and validation
**Specs:** works.md (codename format, generation, immutability)
**Package:** `internal/codename/`
**Deliverables:**
- Adjective and noun word lists (embedded via //go:embed, files at `internal/codename/adjectives.txt` and `internal/codename/nouns.txt`)
- `Generate() string` — random adjective-noun slug
- `Validate(name string) error` — check against `[a-z0-9]+(-[a-z0-9]+)*`
**Tests:** Generated names match format, validation accepts/rejects correctly, no duplicates in small runs

### Bead 1e — Project ID derivation
**Specs:** architecture.md (project identity, derivation, collision handling, monorepos)
**Package:** `internal/project/`
**Deliverables:**
- `Resolve(cwd string, benchPath string) (string, error)` — find .kerf/project-identifier or derive from git remote. Checks for collision with existing project on bench.
- `DeriveFromRemote(repoPath string) (string, error)` — parse origin URL (SSH and HTTPS), slugify owner/repo
- `DeriveFromDirectory(repoPath string) string` — fallback to directory name
- `WriteIdentifier(repoPath string, projectID string) error` — write .kerf/project-identifier
- `ReadIdentifier(repoPath string) (string, error)` — read existing identifier
- `FindGitRoot(cwd string) (string, error)` — walk up to find .git directory
**Tests:** SSH URL, HTTPS URL, no remote fallback, existing identifier, collision detection, not-in-git-repo error

---

## Layer 2: Engines

### Bead 2a — Bench engine
**Specs:** architecture.md (bench layout, bench vs repo boundary, archive directory)
**Package:** `internal/bench/`
**Deliverables:**
- `BenchPath() (string, error)` — resolve `~/.kerf/`
- `EnsureBench() error` — create bench + projects/ if missing
- `WorkDir(projectID, codename string) string` — path to work directory
- `ArchiveDir(projectID, codename string) string` — path to archive directory
- `CreateWork(projectID, codename string) error` — create work directory
- `ListWorks(projectID string) ([]string, error)` — list codenames in a project
- `ListArchivedWorks(projectID string) ([]string, error)` — list archived codenames
- `MoveToArchive(projectID, codename string) error` — move work to archive
- `DeleteWork(projectID, codename string) error` — remove work directory
- `WorkExists(projectID, codename string) bool`
- `IsArchived(projectID, codename string) bool`
**Tests:** Create/list/archive/delete lifecycle, missing bench auto-created, archive dir creation

### Bead 2b — Snapshot engine
**Specs:** snapshots.md (triggers, structure, pruning, restore sequence), architecture.md (snapshot config)
**Package:** `internal/snapshot/`
**Deliverables:**
- `Take(workDir string, label string) (string, error)` — create timestamped snapshot in .history/
- `List(workDir string) ([]SnapshotEntry, error)` — list snapshots (name, timestamp, status from spec.yaml)
- `Restore(workDir string, snapshotName string) (string, error)` — restore + preserve session data, returns pre-restore snapshot path
- `Prune(workDir string, maxSnapshots int) error` — remove oldest beyond limit
- `CheckInterval(workDir string, intervalSeconds int) (bool, error)` — should interval snapshot fire?
- `SnapshotEntry` struct — name, timestamp, label (optional), status
- Copies exclude `.history/` itself
- Full recursive directory copy preserving subdirectory structure
**Tests:** Take/list/restore round-trip, pruning, session preservation on restore, interval logic, named vs unnamed, byte-level correctness of restored files

### Bead 2c — Session engine
**Specs:** sessions.md (tracking, shelving, resuming, stale detection)
**Package:** `internal/session/`
**Deliverables:**
- `StartSession(spec *spec.SpecYAML, sessionID string)` — append session entry, set active_session (or "anonymous" if empty)
- `EndSession(spec *spec.SpecYAML)` — set ended timestamp on active entry, clear active_session
- `IsStale(spec *spec.SpecYAML, thresholdHours int) bool` — stale session detection
- `FindActiveWork(projectDir string) (string, error)` — scan for active session. Error on 0 or >1 matches.
- `StaleWarning(spec *spec.SpecYAML, thresholdHours int) string` — warning message if stale, empty string if not
**Tests:** Start/end lifecycle, stale detection at boundary, anonymous sessions, multiple active error, zero active error

### Bead 2d — Dependency resolver
**Specs:** dependencies.md (resolution, relationship types, completeness)
**Package:** `internal/dep/`
**Deliverables:**
- `Resolve(d spec.Dependency, benchPath string) (*DepResult, error)` — look up dependency status
- `IsComplete(status string, statusValues []string) bool` — status at-or-past terminal (uses jig.IsAtOrPastTerminal logic)
- `CheckBlocking(deps []spec.Dependency, benchPath string) []DepResult` — check all must-complete-first
- `DepResult` struct — codename, project, relationship, status, complete bool, unresolvable bool
**Tests:** Same-project, cross-project, unresolvable, complete/incomplete status logic, inform relationship skipped

---

## Layer 2.5: Test Infrastructure

### Bead 2.5 — Shared test helpers
**Specs:** testing.md (testing layers, strategy)
**Package:** `internal/testutil/`
**Deliverables:**
- `SetupBench(t *testing.T) string` — create temp bench directory, return path, auto-cleanup
- `SetupGitRepo(t *testing.T) string` — create temp git repo with initial commit, return path
- `SetupWork(t *testing.T, benchPath, projectID, codename string, opts ...WorkOpt) string` — create a work directory with valid spec.yaml
- `WorkOpt` functional options — `WithStatus()`, `WithJig()`, `WithSessions()`, `WithDeps()`
- `FixtureJig(name string) []byte` — return built-in jig content for test use
- `AssertFileExists(t *testing.T, path string)`
- `AssertFileContains(t *testing.T, path, substr string)`
- `AssertYAMLField(t *testing.T, path, field string, expected any)`
**Tests:** Self-testing: helpers produce valid structures

---

## Layer 3: Commands

All command beads must:
- Use `cmdutil.ResolveProject()` for project inference (respects `--project` flag)
- Use `cmdutil.LoadWorkWithChecks()` which handles stale session warning, jig version mismatch warning, and interval snapshot check
- Own their specific `cmd/{command}.go` file (stubbed in Bead 0)

### Bead 3a — `kerf` (root) + `kerf list`
**Specs:** commands.md (root command, list command), cli.md (zero-context usability, agent-first output)
**Deliverables:**
- Root command: one-line description, available commands with examples, standard workflow, bench summary (active works count), getting-started if no bench
- `kerf list`: resolve project, read all works, filter by --status, sort by updated, --all includes archived works, show dependency lines, suggest next commands. No-works message with `kerf new` suggestion.
**Tests:** No-bench output, with-bench output, list with --status filter, list with --all, empty project, dependency display

### Bead 3b — `kerf new`
**Specs:** commands.md (new command), works.md (creation), sessions.md (initial session), architecture.md (project identity first-use)
**Deliverables:**
- Resolve project identity (create .kerf/project-identifier if first use in repo, print derived ID)
- Create bench if `~/.kerf/` missing
- Resolve codename (generate or validate user-provided, check uniqueness)
- Resolve jig (from --jig flag or config default_jig)
- Create work directory + initialize spec.yaml (all fields per works.md schema)
- Record initial session
- Take snapshot
- Output: confirmation, jig process overview (all passes), first-pass agent instructions, next steps
- Error messages per commands.md error table
**Tests:** Auto-codename, user codename, duplicate codename error, invalid codename error, jig not found, no-repo error, first-use project derivation

### Bead 3c — `kerf show` + `kerf status`
**Specs:** commands.md (show command, status command), sessions.md (SESSION.md format)
**Deliverables:**
- `show`: metadata block, jig context + current pass instructions, file tree (excluding .history/), session history with active highlighted, dependencies with status, SESSION.md full contents (if present), contextual command suggestions
- `status` (read): current status + progression display with position marker
- `status` (write): update status in spec.yaml, warn on non-recommended value (list recommended), take snapshot, emit jig instructions for new pass, update `updated` timestamp
**Tests:** Show with all fields, show with missing SESSION.md, status read, status write, status write with unknown value warning

### Bead 3d — `kerf resume` + `kerf shelve`
**Specs:** commands.md (resume, shelve), sessions.md (resume sequence, shelve sequence, degraded mode, force shelve)
**Deliverables:**
- `resume`: error if active_session non-null, record new session, snapshot, emit full context block (metadata, SESSION.md contents or degraded notice, current pass + jig instructions, session history, dependency status, file listing, next steps)
- `shelve`: resolve target (codename arg or scan for active session), snapshot, end session, emit SESSION.md writing instructions with path
- `shelve --force`: codename required, end session, clear active_session, snapshot, no SESSION.md instructions
**Tests:** Resume happy path, resume with active session error, resume degraded mode, shelve with codename, shelve without codename (infer), shelve no active session error, shelve multiple active error, force shelve

### Bead 3e — `kerf square` + `kerf finalize`
**Specs:** commands.md (square, finalize), verification.md, finalization.md, sessions.md (active session state)
**Deliverables:**
- `square`: status check (at-or-past terminal), file check (expand {component} from directory structure), dependency check (must-complete-first only, report unresolvable), formatted SQUARE/NOT SQUARE output with details
- `finalize`: --branch required flag, pre-flight (run square, check uncommitted changes via `git status`, check branch doesn't exist), snapshot, `git checkout -b {branch}`, copy artifacts to repo_spec_path (exclude spec.yaml, SESSION.md, .history/), `git add` + `git commit` with "kerf: finalize {codename}" message, update implementation.branch and implementation.commits in spec.yaml, set status to "finalized", emit next steps
**Tests:** Square pass/fail for each check type, square with unresolvable deps, finalize happy path, finalize with failing square, finalize with dirty repo, finalize with existing branch, finalize with missing --branch

### Bead 3f — `kerf snapshot` + `kerf history` + `kerf restore`
**Specs:** commands.md (snapshot, history, restore), snapshots.md
**Deliverables:**
- `snapshot`: manual snapshot with optional --name label (validate slug format), always taken regardless of snapshots.enabled config
- `history`: list snapshots newest-first, show status from each snapshot's spec.yaml, suggest `kerf restore` command
- `restore`: verify snapshot exists, take pre-restore snapshot, copy snapshot files over current, preserve sessions + active_session from current state, emit confirmation with pre-restore snapshot path, emit active session warning if applicable
**Tests:** Named/unnamed snapshots, history ordering, restore round-trip, session preservation on restore, active session warning, invalid snapshot name

### Bead 3g — `kerf archive` + `kerf delete`
**Specs:** commands.md (archive, delete), architecture.md (archive directory structure)
**Deliverables:**
- `archive`: move work dir to `~/.kerf/archive/{project-id}/{codename}/`, create archive project dir if needed, output un-archive instructions (mv command), error if already archived
- `delete`: read spec.yaml for summary (codename, title, status, created, snapshot count), confirmation prompt (skip with --yes), remove directory, also handle deletion of archived works
**Tests:** Archive path, already-archived error, delete with confirmation, delete with --yes, delete archived work

### Bead 3h — `kerf config`
**Specs:** commands.md (config command), architecture.md (config schema, all fields)
**Deliverables:**
- No args: display all config keys with current values and defaults
- Key only: display single value (show default if not set)
- Key + value: write value, create config.yaml file if missing
- Dot-notation for nested keys (e.g., `snapshots.enabled`, `finalize.repo_spec_path`)
- Unknown key error, invalid value error (e.g., non-bool for `snapshots.enabled`)
**Tests:** Read all, read single, write, missing file creation, unknown key error, invalid value error, sessions.stale_threshold_hours

### Bead 3i — `kerf jig` subcommands
**Specs:** commands.md (jig list/show/save/load/sync)
**Deliverables:**
- `jig list`: enumerate from user-level + built-in, user overrides same-name built-in, show name/description/version/source
- `jig show`: resolve jig, display full definition (metadata, status values, passes with output files, file structure, agent instructions)
- `jig save`: without --from: copy resolved jig to user dir for customization. With --from: validate file as jig, copy to user dir. Create ~/.kerf/jigs/ if needed. Overwrite existing.
- `jig load`: fetch from local path or URL, validate as jig definition, save to user dir. Overwrite existing.
- `jig sync`: emit "Jig sync is not yet available."
**Tests:** List with mixed sources, show built-in, save from built-in, save from --from, load from file, load validation failure, sync stub

---

## Layer 4: Advanced Testing (after all commands work)

### Bead 4a — Property-based and fuzz tests
**Specs:** testing.md (property-based tests, fuzz tests)
**Deliverables:**
- Property-based tests (using `testing/quick` or `gopter`):
  - Codename: special characters, unicode, length limits, path traversal attempts
  - YAML round-tripping: write then read produces identical structure for spec.yaml and config.yaml
  - Snapshot integrity: snapshot then restore produces identical files (byte-level)
  - Config merging: defaults + partial config produces correct result
- Fuzz tests (using `testing.F`):
  - Malformed YAML (spec.yaml, config.yaml, jig files)
  - Invalid codenames: empty, very long, path separators, null bytes
  - Corrupted snapshot directories
  - Missing files in expected locations
  - Filesystem permission issues (where testable)

### Bead 4b — Integration and E2E tests
**Specs:** testing.md (integration tests, E2E tests)
**Deliverables:**
- Integration tests (using testutil from Bead 2.5):
  - Full lifecycle: new → write files → shelve → resume → finalize
  - Each command in isolation with real filesystem
  - Multi-work in same project simultaneously
  - Config interactions (bench-level defaults, overrides)
- E2E tests:
  - Real git repo: create work, work through passes, finalize to branch
  - Works with dependencies: warning at finalize
  - Multiple projects on bench
  - Jig loading from file

---

## Parallelization Plan

| Phase | Beads | Workers | Depends On |
|-------|-------|---------|------------|
| 1 | 0 | 1 | — |
| 2 | 1a, 1b, 1c, 1d, 1e | 3 | Phase 1 |
| 3 | 2a, 2b, 2c, 2d, 2.5 | 3 | Phase 2 |
| 4 | 3a-3i | 3 | Phase 3 |
| 5 | 4a, 4b | 2 | Phase 4 |

Notes:
- 1c (jig system) is the largest L1 bead — pair 1d+1e on one worker to balance load
- 2.5 (test infra) must complete before any L3 bead starts
- L3 has 9 beads for 3 workers. Suggested batching:
  - Worker A: 3a (root+list), 3h (config), 3i (jig subcommands) — read-heavy, config/jig themed
  - Worker B: 3b (new), 3d (resume+shelve), 3g (archive+delete) — lifecycle themed
  - Worker C: 3c (show+status), 3e (square+finalize), 3f (snapshot+history+restore) — verification/snapshot themed
- Workers use agent-mail file reservations: each worker owns their `cmd/{command}.go` file(s) and their assigned `internal/` packages
- No worker should modify another worker's files without coordination
