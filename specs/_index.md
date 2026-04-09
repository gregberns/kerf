# kerf — Specification Index

> Spec-writing CLI for AI agents. Go. Single binary. "Measure twice, cut once."

## System Overview

kerf manages a **bench** — a persistent, auto-versioned store of specification documents that lives outside of git. Agents read the codebase for context but write spec artifacts to the bench. The bench handles persistence, versioning, session tracking, and handoff to implementation.

The tool provides opinionated, built-in processes (**jigs**) that guide agents through structured spec-writing workflows.

## Glossary

| Term | Meaning |
|------|---------|
| **archive** | Storage for completed/inactive works. Archived works are hidden from `kerf list` but retain their structure under `~/.kerf/archive/`. |
| **kerf** | The tool itself. "The critical cut before implementation." |
| **bench** | Root workspace directory (`~/.kerf/`). Where all works live. |
| **work** | A unit of specification — a collection of structured documents in its own directory. |
| **jig** | A process template defining how an agent walks through a work. Like a woodworking jig — a repeatable guide. |
| **pass** | A phase within a jig (rough cut → fine cut). Each pass produces artifact files. |
| **square** | Verification. "Is the work true?" Structural checks against jig requirements. |
| **codename** | Immutable identifier for a work (`adjective-noun` slug or user-chosen). |
| **session** | A link between a work and an agent conversation. Tracked for history and resumability. |
| **finalization** | The process of moving a completed work from the bench into the git repo. |

## Spec Map

### Architecture
- [architecture.md](architecture.md) — Bench layout, project identity, global configuration

### Works
- [works.md](works.md) — Work lifecycle, spec.yaml schema, codenames, status, types
- [sessions.md](sessions.md) — Session tracking, shelving, resuming, SESSION.md format
- [snapshots.md](snapshots.md) — Versioning via `.history/`, snapshot triggers and structure
- [dependencies.md](dependencies.md) — Work dependencies, cross-project references

### Jigs
- [jig-system.md](jig-system.md) — Jig format, resolution order, versioning
- [jig-feature.md](jig-feature.md) — Built-in feature jig definition
- [jig-bug.md](jig-bug.md) — Built-in bug jig definition

### CLI
- [cli.md](cli.md) — CLI design principles, output philosophy, agent-first design
- [commands.md](commands.md) — All command specifications
- [finalization.md](finalization.md) — Finalization process and git operations
- [verification.md](verification.md) — Square verification checks

### Engineering
- [testing.md](testing.md) — Testing strategy, layers, CI approach

### Scope
- [future.md](future.md) — Explicitly out of scope for v1, with preserved context

## Implementation Language

Go — compiles to a single binary, cross-platform, good CLI ecosystem (cobra, etc.).

## Key Invariants

These hold across the entire system:

1. Works live at `~/.kerf/`, not in the git repo. They enter git only at finalization.
2. The filesystem is the database. Files are the source of truth.
3. kerf never launches or manages agent sessions. It reads/writes data and emits context.
4. Codenames are immutable once created.
5. Status is an open string. Jigs recommend values; the CLI warns on unrecognized values but does not enforce.
6. Jigs are guidance, not gates. Passes can be skipped.
7. CLI output is agent-first. Every state-changing command emits next steps.
8. Snapshots happen on command invocation, not via filesystem watchers.
