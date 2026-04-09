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

## Implementation Rules

1. Read the spec before writing code
2. Implement what the spec says — nothing more, nothing less
3. If the spec doesn't cover an edge case, update the spec first
4. Tests verify spec compliance, not just code correctness
5. Do not add behaviors, features, or config not in a spec

## Agent Orchestration

- **Orchestrator agents** coordinate and review. They delegate implementation to worker agents. Orchestrators preserve context for critical decisions.
- **Worker agents** receive narrow, well-defined tasks with explicit spec references. They implement and report back.
- After implementation, verify code matches spec.
- If code and spec disagree, the spec wins.

### Procedures (in `.claude/commands/`)

1. **`plan-implementation`** — Break specs into beads, review the breakdown with 3 agents, create dependency graph. Do this before writing any code.
2. **`implement-beads`** — The per-bead execution loop: dispatch one bead, wait, review output against spec, give feedback if needed, clear context, send next bead. Never skip the review gate.
3. **`spawn-workers`** — ntm + agent-mail reference for spawning and managing parallel workers.
