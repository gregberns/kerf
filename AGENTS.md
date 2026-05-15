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

## Orchestration

Three procedures in `.claude/commands/`:
- **`plan-implementation`** — break specs into beads, get 3 critique agents, build dep graph. Before any code.
- **`implement-beads`** — per-bead loop: dispatch → implementer commits → reviewer approves → merge → clear → next.
- **`spawn-workers`** — ntm + agent-mail reference.
