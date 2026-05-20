# kerf — Agent Configuration

Spec-writing CLI for AI agents. Go. Single binary.
"Measure twice, cut once."

> Cross-project working style (keep moving, delegate, plain English, compact, review gate) lives in `~/.claude/CLAUDE.md`. This file adds kerf-specific bits only.

## Prime Directive

All changes are spec-driven:
1. Plan created in `plans/{name}/`
2. Plan modifies specs in `specs/`
3. Code is made consistent with specs

Never write code not backed by a spec. If the spec is wrong, fix the spec first.

## Directory Layout

```
specs/              # Source of truth. Code MUST match these.
  _index.md         # Start here. System overview, glossary, spec map.
plans/              # Change proposals. Each is a folder.
  {name}/
    _plan.md        # Intent, rationale, spec changes
    source/         # Supporting material (optional)
```

## Working With Specs

- Specs are normative: "the system does X", not "we chose X because Y"
- Organized by domain — see `specs/_index.md` for the map
- Read relevant spec(s) before implementing anything
- If a spec is ambiguous or incomplete: stop, flag it, update the spec
- Cross-references between specs use relative links

## Working With Plans

- Every spec change requires a plan
- Plans describe: what's changing, why, which specs are affected, and how
- Plans may include source material in `source/`
- Plan names are sequential: `001_init`, `002_add_foo`, etc.
- **`plans/_backlog/` is dormant.** Do not read, surface, suggest, or reference anything under `plans/_backlog/` unless the user names it explicitly. Treat it as if it were not in the tree.

## Implementation Rules

1. Read the spec before writing code. Implement what it says — no more, no less.
2. If a spec gap blocks you: update the spec first, then code. The spec wins.
3. Tests verify spec compliance, not just that code runs.

## Parallel Implementation — Worktrees

When more than one implementer agent runs concurrently, **each agent works in its own git worktree** off `main`. The orchestrator merges into `main` serially (or in independent batches).

- Worktree path convention: `/tmp/kerf-wt-<bead-id>/` (or similar one-per-bead path).
- Agents commit inside their worktree; they do not push, and they do not commit to the shared `main` working tree.
- Orchestrator merges (`git merge --ff-only` when possible) after the reviewer has approved, then removes the worktree.
- Dirty trees and pre-commit hooks that re-stage tracked files have produced commit-attribution incidents on `main` (work landing under the wrong bead's commit). Worktrees prevent this.

## Review Gate — Functional Verification

A reviewer must verify the feature works end-to-end against the spec sentence the bead claims to satisfy. Reading the diff alone is rubber-stamping.

For each bead, the reviewer:
1. Names the specific spec sentence the bead claims to satisfy.
2. Runs the feature CLI (or invokes the function) against a minimal repro that exercises that sentence.
3. Pastes the observed output.
4. Confirms the observed output matches the spec.
5. **Audits all call sites of the changed function/feature**, not just the bead's specific surface — the bead-tool config feature (commit `d965b9e`) shipped with 6 of 9 callers ignoring the config because the reviewer only checked the surface the bead named.
6. **When multiple parallel agents touch the same package, runs `go test ./...` against the integrated (merged) state**, not just the worktree-only state. Three parallel doctor-detector worktrees each landed a `newTestContext` test helper; each compiled in isolation but the merged package failed (`newTestContext redeclared`). Worktree-local green is necessary but not sufficient.

If any of those steps cannot be done, the bead is not approved.

## Orchestration

Three procedures in `.claude/commands/`:
- **`plan-implementation`** — break specs into beads, get 3 critique agents, build dep graph. Before any code.
- **`implement-beads`** — per-bead loop: dispatch (in worktree) → implementer commits → reviewer functionally verifies → orchestrator merges → clear → next.
- **`spawn-workers`** — ntm + agent-mail reference.
