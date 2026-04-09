# kerf — Documentation Index

*Measure twice, cut once.*

## Problem
- [01 - Problem Statement](01-problem-statement.md) — Why this tool needs to exist

## Solution
- [02 - Proposed Solution](02-proposed-solution.md) — High-level approach and design principles
- [03 - Core Concepts](03-core-concepts.md) — Works, jigs, sessions, status, dependencies, bench
- [04 - CLI Design](04-cli-design.md) — Commands, arguments, output philosophy
- [05 - Data Model](05-data-model.md) — Directory structure, YAML schemas, file formats
- [06 - Default Jigs](06-default-jigs.md) — Built-in feature and bug jigs

## Engineering
- [07 - Testing Strategy](07-testing-strategy.md) — Unit, property, integration, E2E, agentic, fuzz
- [09 - Naming](09-naming.md) — Name decision: kerf, and supporting vocabulary

## Scope Management
- [08 - Future Work](08-future-work.md) — Out of v1 scope, with intent and context preserved
- [10 - Open Questions](10-open-questions.md) — Unresolved decisions that need answers

## Key Decisions Made
- **Works live at `~/.kerf/` (the bench), not in the repo** — solves the worktree problem
- **Project identity via `.kerf/project-identifier`** — committed to repo, stable across moves and worktrees
- **Jigs define the process** — opinionated defaults, user-replaceable; machine-readable data in YAML frontmatter
- **Status is an open string** — jigs recommend values, CLI warns on unrecognized values but doesn't enforce
- **Git ceremony only at finalization** — drafts are just files with snapshot versioning
- **Kerf never launches or manages agent sessions** — it reads/writes data and emits context
- **Codenames are immutable** — auto-generated `adjective-noun` slugs, plus optional mutable `title`
- **Snapshots on command invocation** — no filesystem watchers, honest about the trade-off
- **Finalization is hybrid** — kerf does mechanical git ops, emits instructions for agent follow-up
- **Go as implementation language** — single binary, cross-platform
- **No orchestrator** — this tool manages works, not implementation
- **Testing includes agentic testing** — the tool must work when an agent uses it
- **Naming: kerf** — "Measure twice, cut once." Jigs for templates, passes for phases, bench for workspace, square for verification.

## Implementation Language
Go — compiles to a single binary, runs anywhere, good CLI ecosystem (cobra, etc.)

## Status
Design phase complete. All major design decisions resolved. Open questions resolved or explicitly deferred. Next step: implementation.
