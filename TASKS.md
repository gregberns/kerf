# Tasks

## Phase 1: Scaffolding — COMPLETE
- [x] Create AGENTS.md + CLAUDE.md symlink
- [x] Create plans/001_init/ structure
- [x] Move docs/ to plans/001_init/source/
- [x] Write plans/001_init/_plan.md
- [x] Write specs/_index.md
- [x] Create .claude/commands/spawn-workers.md (with agent-mail process)

## Phase 2: Spec Generation — COMPLETE
- [x] specs/architecture.md — bench layout, project identity, config.yaml
- [x] specs/works.md — work lifecycle, spec.yaml schema, codenames, status
- [x] specs/sessions.md — session tracking, shelving, resuming, SESSION.md
- [x] specs/snapshots.md — .history/ structure, snapshot triggers
- [x] specs/dependencies.md — work dependencies, cross-project refs
- [x] specs/jig-system.md — jig format, resolution order, versioning
- [x] specs/jig-feature.md — feature jig definition
- [x] specs/jig-bug.md — bug jig definition
- [x] specs/cli.md — CLI principles, output philosophy, agent-first design
- [x] specs/commands.md — all command specifications
- [x] specs/finalization.md — finalization process, git operations
- [x] specs/verification.md — square checks
- [x] specs/testing.md — testing strategy and requirements
- [x] specs/future.md — out of scope, preserved context

## Phase 3: Review — COMPLETE
- [x] Cross-reference consistency check across all specs (3 overlapping review clusters)
- [x] Apply fixes from review findings (12 issues resolved)
- [x] Verify no gaps between source docs and generated specs
- [x] Commit everything
- [x] Remove docs/ directory

## Phase 4: Implementation Planning — COMPLETE
- [x] Break specs into implementation beads (tasks via `bd`)
- [x] Create beads.md in plans/001_init/ with full breakdown
- [x] Review beads with 3 agents (spec coverage, Go architecture, parallelization)
- [x] Revise beads based on review feedback
- [x] Initialize bd, create 22 beads with dependency chains in bd

## Next: Implementation

### Beads are in bd (`bd list --limit 0` to see all, `bd ready` for what's unblocked)

### Execution order:
1. **Bead 0** (kerf-5b9): Project scaffold — Go module, cobra, stub files. Must go first.
2. **Layer 1** (5 beads): Core types — spec.yaml, config, jig parsing, codename, project ID. All parallel after Bead 0.
3. **Layer 2** (4 beads + test infra): Engines — bench, snapshot, session, deps. Parallel after L1.
4. **Layer 3** (9 beads): Commands — all CLI commands. Parallel after L2 + test infra.
5. **Layer 4** (2 beads): Advanced testing — property-based, fuzz, integration, E2E.

### Worker plan (3 panes via ntm):
- Worker 1: Bead 0 first, then Bead 1a (spec.yaml types)
- Worker 2: Bead 1c (jig system — largest L1 bead) after Bead 0 lands
- Worker 3: Bead 1d + 1e (codename + project ID — small, pair them) after Bead 0 lands
- Then rotate workers through L2, L3 beads as deps clear

### Key files:
- `plans/001_init/beads.md` — full bead specs with deliverables, specs refs, test requirements
- `bd list`, `bd ready`, `bd show <id>` — bead status and details
- Cross-cutting concerns (global --project flag, stale session check, jig version warning, interval snapshots) documented in beads.md
