# Plan 008 — Summary

## What this plan is

A cleanup pass after the plan 008 exploration cycle, where six independent
agents probed kerf end-to-end against its specs. The exploration produced a
long list of findings, three critique passes, and a recheck against the
current main branch. This plan turns the validated findings into work.

It is not a redesign. Most items are code that drifted away from what the
spec already says. A smaller set needs a spec line written first, because
kerf's rule is spec-first.

## What's broken that we know how to fix (the "Now" items)

These ship without any spec change. About four engineer-days of work in
total.

- Two commands (`kerf show`, `kerf square`) try to run a tool called `bd`
  that does not exist on user machines (the real binary is `br`). Bead
  counts silently come back as zero.
- `kerf next` resolves the right bead-to-work filter but then loses the
  result on the way to display, producing wrong `work_codename` values
  and spurious "no beads attached" warnings. This is the most
  agent-visible bug in the bunch.
- `kerf next` hides works whose status it does not recognize; the spec
  says they should remain visible.
- Two files (`cmd/show.go`, `cmd/map.go`) still use an older, case-
  insensitive label match; the spec is case-sensitive.
- The "unmatched beads" count in the `kerf next` header sometimes
  disagrees with the list it prints (recomputed at the wrong step).
- The root command's help text omits six commands that exist.
- Snapshot tests for `kerf next --help` and bare `kerf` need to be added
  so help-text drift is caught the next time it happens.

## What needs spec work first (the "Next" items)

Each of these is a one-paragraph spec edit that unblocks a small code
change. Total: maybe two or three engineer-days of code, plus the writing
itself.

- `kerf init` on an existing project currently overwrites the user's
  config. The spec does not say what it should do; we need to decide
  (preserve, merge, prompt).
- When a work's spec file is corrupt, the work silently disappears from
  every command. The fix is a new warning type, but the spec needs to
  list the warning type first.
- "Relabel drift" (someone changed a bead's labels out of band) is
  invisible today. The spec needs to say what bytes the drift detector
  hashes.
- A small ambiguity in the simulator spec about churn-counting when there
  is only one candidate.
- Several specs still mention the old `bd` tool name; a pure text sweep.

## The investigation gate

The simulator (added in plan 007) reports zero priority inversions and
zero rework wait times across every test scenario. Read of the metrics
code looks correct, so the bug is in runtime conditions — probably the
warmup window swallowing dispatches, or the rework generator wiring
dependencies incorrectly.

This matters because every weight-tuning hypothesis from the exploration
relies on those metrics. Tuning scoring weights against a simulator that
reports zeros for the things we care about would burn weeks producing
contaminated baselines. So we diagnose first, then fix in a follow-on
plan. One to two engineer-days.

All scoring/weight-tuning work is explicitly blocked behind this gate.

## What is NOT in plan 008

- Triage workflow (`kerf show` rendering attached beads, a new `kerf
  triage` command, drift state file, attach/edit commands) is its own
  effort — promoted to plan 009. Honest sizing on it is roughly a
  sprint-and-a-half.
- Concurrency / multi-agent session signalling (the biggest agent-UX gap
  the exploration found: two agents both picking the top-ranked work)
  is promoted to plan 010.
- Scoring weight changes are deferred to a future plan that starts after
  the investigation gate clears.

## Rough cost

- Now items: ~4 engineer-days of code.
- Investigation gate: 1–2 engineer-days plus a written diagnosis.
- Next items: ~2–3 engineer-days of code, plus spec writing time.

Total wall clock with a small team in parallel: roughly one calendar week
for everything except the spec authoring.
