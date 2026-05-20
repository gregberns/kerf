# Plan 013 — Implementation Beads (revised 2026-05-19)

## Overview

Plan 013 (self-diagnostics from Claude transcripts) lands as **7 beads
across 3 waves** after the 2026-05-19 scope cut. Critical path is **4
hops long**: B-CAL → B1 → B2/B3 → B-D1/B-D6 → B-E2E.

Detectors plug into `kerf next` warnings (not `kerf doctor`), per the
independent review's surface map. D2–D5 are deferred to Plan 013b; their
spec sentences still ship as a "Future detectors" capture-only section
in `specs/diagnostics.md`.

First-ship validation target: **B-D6 (reviewer-absent)** as soon as B2
(parser) lands. It needs only the parser, not the indexer, and exercises
the full warning-channel surface for cheap validation.

## Wave model

```
Wave 1 (1 implementer, before spec lockdown)
  B-CAL  Threshold calibration against plans/012_real_corpus/data/

Wave 2 (parallel, ~3 implementers, after B-CAL)
  B1     specs/diagnostics.md + commands.md warning kinds  (spec)
  B2     internal/kerftranscript: JSONL parser             (foundation)
  B3     internal/kerftranscript: bead-ID indexer          (foundation)
  B4     internal/kerftranscript/testdata: fixtures        (test infra)

Wave 3 (parallel, ~2 implementers, after Wave 2)
  B-D6   D6 reviewer-absent warning  (needs B1 + B2)
  B-D1   D1 abandoned dispatch       (needs B1 + B2 + B3)

  --- First-ship validation: B-D6 lands and renders in kerf next before B-D1 ships ---

Wave 4 (1 implementer)
  B-E2E  End-to-end integration test + commands.md closeout
```

## Dependency graph

```
B-CAL ──> B1 ──┐
B2 ────────────┤
B3 ────────────┼─> B-D6
B4 ────────────┤    │
                    └─> B-D1
                          │
                          └─> B-E2E
```

## Beads

### B-CAL — Threshold calibration against existing corpus

**Spec sentence claimed:** N/A — produces input to B1.

**Scope:** Run the proposed D1 and D6 thresholds (60s dispatch floor;
30-bead D6 window) against `plans/012_real_corpus/data/*.csv` and
`harmonik_wasted_effort.csv` / `harmonik_beads.csv`. Report observed
finding counts per project. Exit with either "thresholds OK as proposed"
or "revise to X (rationale)." Output a one-page note inside
`plans/013_self_diagnostics/calibration.md` that B1 consumes.

This bead exists because the original draft committed thresholds by
feel and the independent review flagged the 30-min D3 floor as wrong
against a 19–35 h corpus reality. Same risk applies to D1/D6 numbers
until checked.

**Depends on:** —

### B1 — Draft `specs/diagnostics.md` + new `kerf next` warning kinds

**Spec sentence claimed:** N/A — this bead writes the sentences others claim.

**Scope:**
- Create `specs/diagnostics.md` with D1 and D6 entries: signal,
  severity, calibrated thresholds (from B-CAL), finding `detail`
  schema, suppression rule between D6 and the future D2.
- Add a capture-only "Future detectors" section listing D2–D5
  (and D7–D13) without normative content.
- Commit the transcript-discovery rule: canonical path template
  (`~/.claude/projects/<encoded-repo>/`), `KERF_TRANSCRIPT_DIR`
  override, and a "v1 = Claude Code on macOS only" scoping sentence.
- Commit `bead.id_pattern` config key location.
- Commit the normative "reviewer dispatch" definition; cross-link
  `specs/commands.md` §"`kerf review`".
- Add `abandoned_dispatch` and `reviewer_absent` rows to
  `specs/commands.md` §"`kerf next`" §"Warning kinds", following the
  existing `corrupt_spec` / `no_project_yaml` shape (title / action /
  reason fields).

**Depends on:** B-CAL

### B2 — `internal/kerftranscript`: JSONL parser

**Spec sentence claimed:** `specs/diagnostics.md` §"Transcript discovery
and parsing" (written in B1).

**Scope:** Parse Claude session JSONL into typed events. Identify
sub-agent dispatches (Task tool invocations + result events), session
boundaries, tool calls, phase markers, reviewer-dispatch markers per
B1's definition. Port from `plans/012_real_corpus/data/extract.py` as a
reference, not a runtime dependency. The Go parser is authoritative
going forward; the Python parser is archived as analysis-only.

**Depends on:** B1

### B3 — `internal/kerftranscript`: bead-ID indexer

**Spec sentence claimed:** `specs/diagnostics.md` §"Bead-ID resolution"
(written in B1).

**Scope:** Build an index over `git log --all` commit messages keyed
by **every** bead ID referenced (regex from `project.yaml:
bead.id_pattern`; includes parent/child rollup, sibling sub-task IDs,
SUBSUMED close-commits, worktree-branch refs). Provides
`HasCommitFor(beadID) bool` plus an evidence trail. Load-bearing for
D1's false-positive rate.

**Depends on:** B1

### B4 — Fixture transcripts in `internal/kerftranscript/testdata/`

**Spec sentence claimed:** N/A (test infrastructure).

**Scope:** Promote the concrete D1 and D6 incidents in
`source/detector_examples.md` (two D1 abandons, three D6 reviewer-absent
commits) to committed Go test fixtures. Each fixture is a minimal JSONL
+ a golden findings file.

**Depends on:** —

### B-D6 — D6 reviewer-absent warning in `kerf next`

**Spec sentence claimed:** `specs/diagnostics.md` §"D6 — reviewer-absent
commit" (drafted in B1).

**Scope:** For every bead commit in the configured window, flag the
bead when no reviewer sub-agent was dispatched in the same session per
B1's definition. Emit one `reviewer_absent` warning per bead in
`kerf next` output, following the warning-kind contract in
`specs/commands.md`. Detector is internally a `doctor.Detector` impl so
future `kerf doctor` integration is one switch flip; this plan only
wires it to the `kerf next` warning channel.

First-ship validation: run against the harmonik corpus; expect ~3 fresh
warnings on recent beads.

**Depends on:** B1, B2

### B-D1 — D1 abandoned dispatch warning in `kerf next`

**Spec sentence claimed:** `specs/diagnostics.md` §"D1 — abandoned
dispatch" (drafted in B1).

**Scope:** For every sub-agent dispatch lasting ≥(calibrated floor),
query the B3 indexer for any commit referencing the dispatched bead ID
(with parent/child rollup and worktree refs). When none found and the
bead has no terminal status, emit one `abandoned_dispatch` warning per
finding with the four reason categories. Same internal Detector impl
shape as B-D6.

**Depends on:** B1, B2, B3

### B-E2E — End-to-end integration + `commands.md` closeout

**Spec sentence claimed:** `specs/commands.md` §"`kerf next`" §"Warning
kinds" — verify `abandoned_dispatch` and `reviewer_absent` render
correctly in text and JSON.

**Scope:** Run `kerf next` against the B4 fixture project; golden-test
the warning output (text and JSON). Confirm fatal-vs-non-fatal
classification matches B1's spec. Update `commands.md` example output
if needed.

**Depends on:** B-D6, B-D1

## Spec sentences each bead claims

| Bead   | Spec touchpoint |
|--------|------------------|
| B-CAL  | (produces calibration.md, no spec) |
| B1     | (writes the spec) |
| B2     | `specs/diagnostics.md` §"Transcript discovery and parsing" |
| B3     | `specs/diagnostics.md` §"Bead-ID resolution" |
| B4     | (test infrastructure) |
| B-D6   | `specs/diagnostics.md` §"D6 — reviewer-absent commit" |
| B-D1   | `specs/diagnostics.md` §"D1 — abandoned dispatch" |
| B-E2E  | `specs/commands.md` §"`kerf next`" §"Warning kinds" |

## Parallelism summary

Peak concurrent implementers: 3 (Wave 2). Steady-state: 2 (Wave 3).
Total work-equivalent: ~7 bead-units. With 3-worker parallelism the
wall clock is ~4 waves; with 1-worker serial it is ~7 waves.
