# Bead ID Map

Mapping from plan/bead reference → bd issue ID. Generated 2026-05-14.

Use these IDs with `bd show <id>`, `bd update <id>`, etc. The `implement-beads` procedure dispatches one bead at a time using these IDs.

## Plan 006 — Flexible Bead Attachment & Actionable `kerf next`

| Plan / Bead | Title | bd ID | Layer / Phase | Blocked by |
|---|---|---|---|---|
| 006 / B2  | internal/beads Filter, Match, Resolve, ForWork wrapper | `kerf-ddi` | L0 / P0 | — (READY) |
| 006 / B1a | project.yaml schema: top-level bead_filter | `kerf-1pb` | L1 / P1 | B2 |
| 006 / B1b | spec.yaml schema: per-work bead_filter | `kerf-c30` | L1 / P1 | B2 |
| 006 / B3  | internal/feed Item, Kind, Assemble, Score, Sort, Filter | `kerf-o5y` | L2 / P2 | B2 |
| 006 / B4  | internal/feed/cleanup.go detectors | `kerf-aka` | L2 / P2 | B2 |
| 006 / B5  | internal/feed/warning.go detectors | `kerf-w4z` | L2 / P2 | B2 |
| 006 / B7  | cmd/init.go bead_filter auto-detect heuristic | `kerf-lxr` | L2 / P2 | B2, B1a |
| 006 / B6  | cmd/next.go flag parsing, render, help-text | `kerf-nm3` | L3 / P3 | B3, B4, B5 |
| 006 / B8  | E2E coordination tests | `kerf-frb` | L4 / P4 | B6, B7 |

**L0 ready to dispatch first:** `kerf-ddi` (B2 — filter engine leaf).

## Plan 007 — Queue Simulator (`kerfsim`) — Phase 1

| Plan / Bead | Title | bd ID | Layer / Phase | Blocked by |
|---|---|---|---|---|
| 007 / B1  | Sub-seed derivation utility | `kerf-7ll` | L0 / Phase 1 | — (READY) |
| 007 / B2  | Scenario types, YAML loader, validator | `kerf-jpk` | L0 / Phase 1 | — (READY) |
| 007 / B3  | Event heap and ordering | `kerf-een` | L0 / Phase 1 | — (READY) |
| 007 / B12 | Canned scenario YAML files | `kerf-ted` | L0 / Phase 1 | B2 |
| 007 / B5  | Synthetic scenario generator | `kerf-2ne` | L1 / Phase 2 | B1, B2 |
| 007 / B6  | In-memory bead store + queue adapter | `kerf-wkb` | L1 / Phase 2 | B2 |
| 007 / B9a | Metric collector (pure) | `kerf-aa2` | L1 / Phase 2 | B3 |
| 007 / B7  | Event-driven tick loop, Policy + Hooks interfaces | `kerf-v20` | L2 / Phase 3 | B1, B3, B6 |
| 007 / B8  | Baseline policies | `kerf-dwh` | L2 / Phase 3 | B1, B6, B7 |
| 007 / B9b | Loop hook wiring (thin) | `kerf-nl3` | L2 / Phase 3 | B7, B9a |
| 007 / B10 | Run orchestrator | `kerf-6un` | L3 / Phase 3 | B5, B6, B7, B8, B9a, B9b |
| 007 / B11 | Output writers | `kerf-vmg` | L3 / Phase 3 | B9a |
| 007 / B13 | cmd/kerfsim run command | `kerf-4m4` | L4 / Phase 3 | B10, B11, B12 |
| 007 / B14 | cmd/kerfsim diff command | `kerf-z75` | L4 / Phase 3 | B11 |
| 007 / B15 | E2E determinism test | `kerf-e53` | L4 / Phase 3 | B12, B13 |

> **B4 is intentionally skipped** (resolved during planning, no implementation work — see plan 007 beads.md). Numbering is preserved for cross-references.

**L0 ready to dispatch first:** `kerf-7ll` (B1), `kerf-jpk` (B2), `kerf-een` (B3). These three can run in parallel.

## Quick commands

```
bd show kerf-ddi          # view bead detail
bd ready                  # list unblocked beads
bd dep tree kerf-frb      # see full dep chain for plan 006 E2E
bd list --label plan-006  # plan 006 only
bd list --label plan-007  # plan 007 only
```
