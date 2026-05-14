# Forward Plan — Next Steps

## Where We Are

- Plan 005 Phase 1: 9 of 10 beads implemented, all tests passing
- Specs written: `coordination.md`, `areas.md`; amendments to `architecture.md`, `works.md`, `commands.md`, `_index.md`
- Plan 004 (local storage): planned, not implemented
- All builds clean, all tests green

## Immediate (Phase 1 finish)

### Coord-010 — Integration Tests
End-to-end test covering the full coordination flow:
- Create project with areas via `kerf areas add`
- Create two works sharing an area, verify overlap warning
- Run `kerf map`, verify grouping
- Run `kerf next`, verify ordering
- Verify `kerf show` displays areas + overlap

**Location:** `cmd/coordination_e2e_test.go` (or extend `cmd/e2e_test.go`)
**Complexity:** Medium

### `kerf areas init` (gap)
The spec references `kerf areas init` for taxonomy bootstrap. Not yet implemented. Should create an empty `areas.yaml` with a helpful comment block.

**Location:** Extend `cmd/areas.go`
**Complexity:** Small

### Smoke Test the Binary
Build the binary and manually exercise the new commands in a real project (not just tests). Confirm output formatting reads well, no surprise behaviors.

```bash
go install ./...
cd /some/test/repo
kerf areas init
kerf areas add api --description "HTTP layer"
kerf new --jig plan --area api
kerf map
kerf next
```

## Phase 2 — Rework Priority and Queue Refinement

### 2.1 Findings via Tagged Beads
Make findings a first-class concept that flows through beads (NOT a separate kerf entity).
- Convention: beads tagged `finding:<work>` or `rework:true` are findings
- Document the convention in `specs/coordination.md` (already partially there)
- `kerf next` weights rework beads higher than new-work beads

**Spec work:** Extend `coordination.md` with the tagging convention
**Code:** Modify `internal/queue/queue.go` to consume bead tags

### 2.2 Configurable Queue Parameters
The weights are currently constants in `queue.go`. Greg explicitly wants these configurable over time.
- Add a `queue` section to `config.yaml` (global) or `project.yaml` (per-project)
- Load weights at runtime, fall back to defaults
- Document in `specs/coordination.md`

**Complexity:** Small

### 2.3 Cross-Work Bead Dependencies
Currently the queue only filters works by must-complete-first dependencies. It doesn't consider that a bead in Work A might depend on a bead in Work B.
- Decide whether kerf cares about this, or whether beads system handles it entirely
- If kerf cares: extend queue to query bead-level dependencies from `br`

**Open question:** Is this Phase 2 or deferred? Probably defer until pain emerges.

### 2.4 `kerf resume` Area Context
When resuming a work, surface the related/overlapping works in the same areas. Already partially done in `kerf show` via overlap detection — extend to `kerf resume`.

**Complexity:** Small (copy pattern from `kerf show`)

## Phase 3 — Area Hierarchy and Edges

### 3.1 Area Hierarchy
Add parent-child relationships between areas:
- `parent:` field in `areas.yaml`
- Rollup queries (works under an area or any of its descendants)
- `kerf map` groups hierarchically

### 3.2 Area Edges (talks-to relationships)
Per Greg's interest in "a more sophisticated graph that could represent what parts talk to each other":
- Typed edges between areas (`calls`, `reads`, `shares-data-with`)
- Adjacency-based overlap warnings ("this work touches `api`, but `api` calls `auth` which has 3 active works")
- Blast radius queries

**Decision:** Phase 3 is exploratory — only build when Phase 1+2 reveal the need.

## Parallel Track — Plan 004 (Local Storage)

Plan 004 is written (`plans/004_local_storage/_plan.md`) but not implemented. This is independent of Plan 005 and can proceed in parallel:
- Add `.kerf/config.yaml` with `storage: local|bench`
- Symlink bench project dir → repo's `.kerf/works/` when local
- `kerf localize` command

**Complexity:** Medium. Well-scoped. Could be done in one session.

## Spec Hygiene

### Source File Cleanup
The `source/` directory has 37 files. Many are historical drafts. After Phase 1 lands fully, consider:
- Move docs 01-08 (brainstorming) into a `phase1_planning/` subdirectory
- Keep 25 (system shape), 33-36 (final decisions + implementation plan) at the top level
- Drafts 29, 30, 31 are superseded by the actual specs — keep as historical or move to `superseded/`

### Spec Index
`specs/_index.md` was amended but should be reviewed to ensure the new specs (coordination, areas) are properly cross-linked from related specs.

### `specs/queue.md`?
The spec change plan (doc 28) proposed a separate `queue.md`. Currently the queue/priority content is in `coordination.md`. Decide: split it out for clarity, or keep consolidated. Probably keep consolidated until coordination.md grows too large.

## Process Improvements (from this session)

These came up as observations during the session — worth capturing.

### Memory Items to Maintain
The auto-memory captured two important rules this session:
- No rigid policies in specs
- Agent topology agnostic interface

Continue adding feedback memories as Greg gives correctional feedback. These prevent recurring mistakes across sessions.

### When to Use This Multi-Agent Pattern
The brainstorm → synthesis → critic → implementation pipeline used in this session worked well for a large, ambiguous design problem. Pattern to reuse:
1. Define problem clearly in plan
2. Spawn N agents from different perspectives (independent, parallel)
3. Synthesis agents cluster/evaluate
4. User feedback shapes direction
5. Deep dives on critical threads
6. Critics stress-test
7. Final synthesis with phasing
8. Implementation agents per bead with clear specs

Total: ~30 agents across this session. High value when the design space is genuinely unclear.

## Recommended Order for Next Session

1. **Coord-010 integration tests** — close out Phase 1 cleanly
2. **`kerf areas init`** — small gap, fast win
3. **Smoke test the binary** — confirm it actually works in practice
4. **Phase 2.2 configurable weights** — small change, unlocks tuning
5. **Phase 2.1 findings via tagged beads** — the most user-visible Phase 2 feature
6. **Plan 004 local storage** — independent, could slot in anywhere

Defer Phase 2.3, Phase 3, and source file cleanup until they're clearly needed.
