# Tasks

## Phase 1: Scaffolding
- [x] Create AGENTS.md + CLAUDE.md symlink
- [x] Create plans/001_init/ structure
- [x] Move docs/ to plans/001_init/source/
- [x] Write plans/001_init/_plan.md
- [x] Write specs/_index.md
- [x] Create .claude/commands/spawn-workers.md (with agent-mail process)

## Phase 2: Spec Generation (ntm workers)
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

## Phase 3: Review
- [ ] Cross-reference consistency check across all specs
- [ ] Verify no gaps between source docs and generated specs
- [ ] Commit everything
- [ ] Remove docs/ directory (after commit)
