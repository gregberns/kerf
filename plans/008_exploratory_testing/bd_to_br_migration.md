# bd → br Migration Audit

Audit date: 2026-05-15
Scope: `cmd/`, `internal/`, `specs/`, `plans/006*`, `plans/007*`, `CLAUDE.md`
Method: `grep -rnw 'bd'` (word-boundary; excludes substrings like `bdiff`).

## Summary
- Total `\bbd\b` references found: **55**
- **MUST CHANGE: 26 items** (production code, shipped jig template, test fixtures, normative specs)
- **SHOULD CHANGE: 2 items** (historical plan READMEs)
- **OK / Skipped: 0 items** — every hit is a real `bd` tool reference; none are unrelated tokens
- **Commands with syntax divergence between `bd` and `br`:** `list --json` flag, `ready` (semantic equivalent of `list --status open`), `dep <child> <parent>` (positional args reversed and now a subcommand), `update --status` (works) — see table below.

The canonical reference implementation in the codebase is `internal/beads/beads.go:48`:
```
exec.Command("br", "list", "--format", "json", "--all", "--limit", "0")
```
All proposed replacements should track this style.

## Command-Syntax Differences

| bd | br equivalent | Notes |
|---|---|---|
| `bd list` | `br list` | Compatible. Default output is text. |
| `bd list --json` | `br list --format json` (or `--json` flag) | **Divergence:** bd uses `--json`; br canonical form per `internal/beads/beads.go` is `--format json`. br *does* accept `--json` as a global flag too, so either works, but project convention is `--format json`. |
| `bd list --status open` | `br list --status open` | Compatible. |
| `bd ready` | `br ready` | Both exist. br's `ready` returns open + unblocked + not-deferred. |
| `bd create "Title" --desc "..."` | `br create "Title" --description "..."` (or `-d`, or alias `--body`) | **Divergence:** bd uses `--desc`; br uses `--description` / `-d` / `--body`. bd's `--desc` is not a br flag. |
| `bd dep <child> <parent>` | `br dep add <issue> <depends-on>` | **Major divergence:** br requires the `add` subcommand. Semantics differ: `br dep add A B` means "A depends on B" — verify direction matches bd's. |
| `bd update <id> --status in-progress` | `br update <id> --status in_progress` | **Divergence:** status value uses underscore (`in_progress`) in br, hyphen (`in-progress`) in bd. |
| `bd update <id> --status closed` | `br close <id>` (preferred) or `br update <id> --status closed` | br has a dedicated `close` subcommand. |
| `bd status` | `br status` (alias for `stats`) | Compatible. |

The migration is *not* a pure mechanical s/bd/br/g — the docs in `internal/jig/builtin/implementation.md` use `--desc`, `--status in-progress`, and `bd dep <child> <parent>`, all of which need flag/syntax adjustment.

## MUST CHANGE (production code / shipped templates / normative specs)

### Production Go code (ships in binary, runs at runtime)
- `cmd/show.go:101` — comment `// Bead status (best-effort via bd tool)` → `// Bead status (best-effort via br tool)`
- `cmd/show.go:265` — comment `// bdBead represents a single bead from bd list --json output.` → reflect br; consider renaming `bdBead` struct to `brBead` or just `bead` for clarity (used at line ~265+).
- `cmd/show.go:271-272` — comments `// getBeadSummary tries to run bd list --json …` / `// Returns empty string if bd is unavailable …` → swap `bd` → `br`.
- `cmd/show.go:278` — **CRITICAL BUG**: `exec.Command("bd", args...).Output()` → `exec.Command("br", args...)`. Also verify `args` (need to inspect — likely `--json` flag that must become `--format json`). The whole function should ideally delegate to `internal/beads` instead of shelling out independently.
- `cmd/square.go:212` — comment `// tryLoadBeadInfo attempts to read bead status via `bd list`. Fails silently if bd unavailable.` → swap.
- `cmd/square.go:214` — **CRITICAL BUG**: `exec.Command("bd", "list").Output()` → `exec.Command("br", "list").Output()` (text format is fine here since `parseBeadOutput` parses text).
- `cmd/square.go:227` — comment `// parseBeadOutput extracts bead counts from bd list output.` → swap.

### Tests (assert on stale strings — will break after jig fix)
- `cmd/jig_test.go:32` — `testutil.AssertStringContains(t, out, "Tools: bd, ntm, agent-mail")` → `"Tools: br, ntm, agent-mail"`
- `cmd/show_test.go:118` — comment `// bd is not installed in test env …` → swap.
- `cmd/show_test.go:121` — error message `"expected empty string when bd unavailable, got %q"` → swap.
- `internal/config/project_test.go:45` — fixture `tasks: bd` → `tasks: br`
- `internal/config/project_test.go:69-70` — assertion `cfg.Tools["tasks"] != "bd"` and error string → `"br"`
- `internal/jig/jig_test.go:923-924` — `reflect.DeepEqual(jig.Tools, []string{"bd", "ntm", "agent-mail"})` and error → `"br"`.
- `internal/jig/jig_test.go:938-939` — `reflect.DeepEqual(jig.Passes[0].Tools, []string{"bd"})` and error → `"br"`.

### Built-in jig template (shipped in binary; agents read this!)
`internal/jig/builtin/implementation.md` — the embedded jig that instructs agents on the implementation pass. Every `bd` reference here ships and is read by worker agents:
- Line 7: `- bd` (tools list) → `- br`
- Line 22: `tools: ["bd"]` → `["br"]`
- Line 75: `3. Create beads using `bd`:` → `using `br`:`
- Line 76: `bd create "Task title" --desc "Full description …"` → `br create "Task title" --description "Full description …"` (note `--desc` → `--description`)
- Line 78: `Set dependencies between beads: `bd dep <child> <parent>`` → `br dep add <child> <depends-on>` (subcommand + verify semantics)
- Line 90: `Beads exist in the `bd` database` → `br`
- Line 112: `Query ready beads: `bd list --status open` or `bd ready`` → `br list --status open` or `br ready`
- Line 119: `bd update <id> --status in-progress` → `br update <id> --status in_progress` (underscore)
- Line 124: `bd list` or `bd status` → `br list` or `br status`
- Line 146: `bead state is in `bd`` → `br`
- Line 186: `bd update <id> --status closed` → recommend `br close <id>`
- Line 203: `bd list` → `br list`
- Line 253: `bd list` → `br list`

### Normative specs (source of truth — code must match these)
- `specs/architecture.md:237` — fixture `tasks: bd` → `tasks: br`
- `specs/verification.md:50` — narrative: `kerf reads bead status via `bd list` if `bd` is available. If `bd` is not available …` → all three `bd` → `br`
- `specs/commands.md:640` — example output `Tools: bd, ntm` → `Tools: br, ntm`
- `specs/commands.md:662` — example output `Tools: bd, ntm` → `Tools: br, ntm`
- `specs/commands.md:1320` — `which external tools are needed (e.g., `bd` for bead management, …)` → `br`
- `specs/jig-system.md:62` — `External tools the jig depends on (e.g., `bd`, `ntm`, …)` → `br`
- `specs/coordination.md:190` — `The beads system (bd) tracks bead execution state` → `(br)`
- `specs/coordination.md:257` — diagram label `beads (bd)` → `beads (br)`
- `specs/jig-implementation.md` lines **30, 45, 115, 116, 118, 130, 154, 161, 166, 188, 230, 247, 301, 326** — **same content as the built-in jig template** (it appears `specs/jig-implementation.md` is the normative spec and `internal/jig/builtin/implementation.md` is the shipped copy). Every `bd` line here needs the same treatment as the built-in template, with the same flag/syntax adjustments (`--desc` → `--description`, `--status in-progress` → `--status in_progress`, `bd dep <child> <parent>` → `br dep add <child> <depends-on>`).

## SHOULD CHANGE (historical plans)

These are completed/archived plan documents. They reference `bd` as the tool that was assumed at the time of writing. Updating them is non-functional but useful for consistency / future readers:

- `plans/006_bead_attachment/_plan.md:183` — "Beads are tracked in `bd`; see [/plans/bead-id-map.md] for the bd ID mapping." → swap to `br`, or add a footnote noting the migration.
- `plans/007_simulator/_plan.md:156` — same construction. → same treatment.

Note: there may also be `bd-NNN` style bead IDs throughout `plans/006*` and `plans/007*` (e.g., `bd-1`, `bd-2`) that the `\bbd\b` regex did not surface because they have `-` directly after. These are historical artifact IDs — if a separate sweep is desired, run `grep -rn 'bd-[0-9]' plans/`. They are **not** in the MUST CHANGE bucket (they refer to past-state issue IDs, not instructions to agents).

## OK / Skipped

None. Every word-bounded `bd` hit in the audited paths is a real reference to the `bd` tool that should be migrated. The grep already excluded substring noise.

## Recommended migration order

1. **First** (correctness): `cmd/show.go:278` and `cmd/square.go:214` — these are the only places where the binary itself shells out to a non-existent (for the user) `bd` and silently fails. Fix immediately, but verify args translate (`show.go` may pass `--json` which needs to become `--format json`).
2. **Second** (agent guidance): `internal/jig/builtin/implementation.md` — this ships in the binary and is read by worker agents during implementation passes. Stale instructions actively misdirect agents.
3. **Third** (source of truth): `specs/jig-implementation.md` and other specs — by project convention (`CLAUDE.md` Prime Directive) specs must change *before* code, but in practice these mirror the built-in template, so update in lockstep.
4. **Fourth** (test fixtures): all the test files that assert on the literal string `"bd"` — these will fail once the above changes land.
5. **Fifth** (housekeeping): historical plans 006, 007.

## Cross-cutting concern

`specs/jig-implementation.md` and `internal/jig/builtin/implementation.md` appear to be duplicates (same line content, same `bd` issues at parallel locations). After this migration, consider whether one should generate the other (e.g., `go:embed`) to prevent future drift.
