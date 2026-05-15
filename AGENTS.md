# kerf — Agent Configuration

Spec-writing CLI for AI agents. Go. Single binary.
"Measure twice, cut once."

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

## Working Style

- **Keep moving.** You have a task list — work through it. Don't stop to ask "should I continue?" — the answer is yes. Only ask the user when you are genuinely blocked on a decision they alone can make, and ask it in one sentence with concrete options.
- **Delegate by default.** For multi-file work, exploration, review, or anything parallelizable, spawn 5–10 sub-agents instead of doing it in the main thread. Orchestrator role > implementer role.
- **Plain English, always.** When you mention an internal name (bead ID, plan number, B-code, package path), translate it the first time: `kerf-mgg (B7 — the "next runs last" cleanup bead)`. The user shouldn't have to grep to follow you.
- **Compact output.** Default to <50 lines. Use bullets, not paragraphs. A menu of 4 options beats a 2,000-word essay. If you want to write more, ask first.
- **Review gate is not optional.** Before merging any bead, a separate reviewer agent must approve it. Skipping this has burned us. (See `.claude/commands/implement-beads.md`.)

## Implementation Rules

1. Read the spec before writing code. Implement what it says — no more, no less.
2. If a spec gap blocks you: update the spec first, then code. The spec wins.
3. Tests verify spec compliance, not just that code runs.

## Orchestration

Three procedures in `.claude/commands/`:
- **`plan-implementation`** — break specs into beads, get 3 critique agents, build dep graph. Before any code.
- **`implement-beads`** — per-bead loop: dispatch → implementer commits → reviewer approves → merge → clear → next.
- **`spawn-workers`** — ntm + agent-mail reference.
