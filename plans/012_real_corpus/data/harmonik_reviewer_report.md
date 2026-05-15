# Harmonik reviewer-phase corpus

Extension of `harmonik_beads.csv` focused on beads with a non-null `reviewer_seconds`.
The prior pass (newest-first, 60 sessions) yielded zero reviewer phases; this pass
walks oldest-first using a per-session bead-grouped reviewer-linker.

## Window scanned

- Sessions processed: 33 (oldest-first; stopped after exceeding 30 reviewer rows)
- Oldest session mtime: 2026-04-19 23:42 UTC
- Newest session mtime: 2026-05-08 04:18 UTC
- Sessions with at least one reviewer-linked bead: 4 (`196fb94b`, `e3ab8a4d`, `5d26a493`, `7e362669`, `e96e6732`)

The first 24 oldest sessions yielded no reviewer rows — they either had no per-bead
reviewer dispatch (single-implementer workflow) or no successful implementer+reviewer
pair on the same bead. The reviewer pattern materializes mid-corpus.

## Reviewer-phase distribution (seconds)

| stat | reviewer_seconds |
|---|---|
| n | 34 |
| min | 68.6 |
| median | 125.5 |
| p95 | 247.9 |
| max | 799.5 |

Companion phases on the same 34 rows: spin_up p50=3.7s/max=6.0s; task_work p50=138.8s/max=561.5s; merge p50≈-0.7s (clock-skew floor) with one 215.7s outlier; total p50=263.7s/max=1046.6s.

## Pattern uniformity

The reviewer phase is bimodal:

- Main cluster (n≈32): 70-250s. Tight band suggesting a standard approve-or-comment
  flow — reviewer reads the diff, runs a couple of greps/tests, emits a verdict.
- Tail (n=2): 564s and 800s. These look like full re-execution rounds: the reviewer
  dispatched a fresh test run or requested re-work. Worth treating as a separate
  regime if modeling.

Per-session bead clustering is striking — `196fb94b` contributes 3 rows, `5d26a493`
14 rows, `7e362669` 4 rows, `e96e6732` 10 rows. Reviewer-dispatch is a per-session
habit, not a per-bead policy: orchestrators either use it for every bead or for none.

## Schema

Matches `harmonik_beads.csv` exactly. All 34 rows have all five phase columns filled
(`spin_up_seconds`, `task_work_seconds`, `merge_seconds`, `reviewer_seconds`,
`total_seconds`). `merge_seconds` lookups for worktree-only commits fell back to
`git show -s` against dangling objects in the harmonik repo.
