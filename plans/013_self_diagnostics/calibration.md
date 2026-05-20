# Plan 013 — Threshold Calibration (Bead B-CAL / kerf-dq39)

**Date:** 2026-05-19
**Purpose:** Validate proposed D1 (abandoned dispatch) and D6 (reviewer-absent) thresholds against the Plan 012 corpus before bead B1 commits numbers in `specs/diagnostics.md`. Independent reviewer flagged that the previous draft committed by feel; this bead replaces feel with measurement.

## Method

- **Corpus:** `plans/012_real_corpus/data/` (read-only). Specifically:
  - `harmonik_wasted_effort.csv` (n=123) — abandoned-dispatch candidates from harmonik.
  - `kerf_wasted_effort.csv` (n=2) — abandoned-dispatch candidates from kerf.
  - `harmonik_beads.csv` (n=150) — harmonik beads 2026-05-10 → 2026-05-15 (post-reviewer-era).
  - `harmonik_reviewer_beads.csv` (n=34) — harmonik beads 2026-05-07 → 2026-05-08 (reviewer-era).
  - `kerf_beads.csv` (n=52) — kerf beads 2026-05-14 → 2026-05-15.
- **Extraction:** ad-hoc Python script at `/tmp/calib_d1_d6.py` (not committed). Reads CSVs only; no transcript re-parse. The Plan 012 Python parser at `plans/012_real_corpus/data/extract.py` was used as-is for the underlying CSVs; no Go parser exists yet (bead B2 ships it).
- **Caveat:** the Plan 012 Python parser is the source of truth for these CSVs. `source/detector_examples.md` flags that ~75-80% of the 123 harmonik wasted-effort rows are indexer false-negatives (commit landed but the indexer missed aliased bead IDs / worktree refs). The numbers below take this into account where it matters; the Go parser (bead B2) is expected to bring the FP rate down before D1 ships.

## D1 — abandoned dispatch

### Proposed threshold

60-second floor on dispatch wall-time below which the candidate is suppressed (intent: avoid flagging instant fan-outs / cheap successful dispatches).

### What the data show

**Naive abandoned-dispatch counts (no commit found for the bead ID, any branch):**

| Project   | Candidate dispatches |
|-----------|---------------------|
| harmonik  | 123                 |
| kerf      | 2                   |

After accounting for the source-documented ~75-80% indexer false-negative rate, the true D1 population in harmonik is roughly **20-30 incidents**; kerf is 2 candidates that must be hand-verified.

**Wall-time distribution of *successful* dispatches (these are what a too-high floor would silently suppress):**

| Percentile          | harmonik task_work (s) | kerf task_work (s) |
|---------------------|-----------------------:|-------------------:|
| min                 | 16.2                   | 41.5               |
| p10                 | 108.3                  | 90.5               |
| p25                 | 151.0                  | 139.1              |
| median              | 267.2                  | 225.8              |
| p75                 | 478.9                  | 321.9              |
| p90                 | 900.7                  | 517.2              |
| p99                 | 5193.9                 | 696.4              |
| max                 | 6559.3                 | 696.4              |

**Sub-floor density (would-be-suppressed by candidate floor, against successful dispatches):**

| Floor | harmonik below (of 150) | kerf below (of 52) |
|------:|------------------------:|-------------------:|
|  10s  | 0  (0.0%)               | 0  (0.0%)          |
|  30s  | 3  (2.0%)               | 0  (0.0%)          |
|  60s  | 3  (2.0%)               | 2  (3.8%)          |
|  90s  | 10 (6.7%)               | 4  (7.7%)          |
| 120s  | 23 (15.3%)              | 11 (21.2%)         |

**Read:** 60s preserves >96% of real work in both projects; 90s starts cutting into legitimate fast spec/scaffolding beads (~7-8%); 120s clearly over-aggressive.

**Inter-arrival of abandoned dispatches within a session** (proxy for "burst fan-out": the plan-013 source notes 9-second sibling dispatches as a real pattern):

| Window  | gaps below window (of 87) |
|--------:|--------------------------:|
|   5s    | 6   (6.9%)                |
|  10s    | 22  (25.3%)               |
|  30s    | 40  (46.0%)               |
|  60s    | 46  (52.9%)               |
| 120s    | 51  (58.6%)               |

Roughly half of abandoned dispatches arrive within 60s of a sibling abandon. A floor that fires on each individually will produce noisy bursts; deduping the burst is a separate concern (B7 / future detector) but should be noted as a follow-on.

### Recommendation

**Keep the 60s floor.** It suppresses essentially no real work (~2% harmonik, ~4% kerf), it aligns with p10 of harmonik's successful dispatches (108s) with comfortable margin, and dropping it lower buys nothing because there are no abandons below 60s the floor would have rescued (the abandon set has no measurable duration; the floor exists to spare successful dispatches, and 60s does that job at 30s as well as at 60s). Going *higher* (90s+) starts hiding legitimate sub-100s beads, which kerf has.

## D6 — reviewer-absent commit

### Proposed threshold

Window: last 30 beads. Each bead in window with no paired reviewer sub-agent dispatch produces one finding.

### What the data show

**Overall reviewer presence:**

| Project           | Beads | with reviewer | absent | absent rate |
|-------------------|------:|--------------:|-------:|------------:|
| harmonik (recent) | 150   | 0             | 150    | 100.0%      |
| harmonik (older)  | 34    | 34            | 0      | 0.0%        |
| harmonik (total)  | 184   | 34            | 150    | 81.5%       |
| kerf              | 52    | 24            | 28     | 53.8%       |

The cutoff in harmonik is sharp: reviewer phase vanishes on/after 2026-05-10 and never returns. This is the motivating incident.

**Rolling-window behaviour (last N beads, harmonik):**

| Window N | absent / N | absent rate |
|---------:|-----------:|------------:|
| 10       | 10 / 10    | 100.0%      |
| 20       | 20 / 20    | 100.0%      |
| 30       | 30 / 30    | 100.0%      |
| 50       | 50 / 50    | 100.0%      |
| 100      | 100 / 100  | 100.0%      |

**Rolling-window behaviour (last N beads, kerf):**

| Window N | absent / N | absent rate |
|---------:|-----------:|------------:|
| 10       | 10 / 10    | 100.0%      |
| 20       | 20 / 20    | 100.0%      |
| 30       | 28 / 30    | 93.3%       |
| 50       | 28 / 50    | 56.0%       |

**Time-window alternative** (last K days from corpus max, harmonik):

| K (days) | absent / total |
|---------:|---------------:|
| 1        | 61 / 61        |
| 3        | 127 / 127      |
| 7        | 150 / 150      |

### False-positive risk

- **harmonik:** all 30 D6 findings in the last-30 window are in the source-confirmed "no-reviewer era." Zero false positives expected. D6 will fire 30 times on first run — large but correct.
- **kerf:** 28/30 findings in the last-30 window. Source `detector_examples.md` line 27-28 explicitly flags this as a **parsing artifact**, not a regression: kerf reviewers happen conversationally without the structured sub-agent handoff the Python parser keys on. If the Go parser (B2) inherits the same handoff-only definition, **D6 will produce ~28 kerf false positives on day one.** B1 must either:
  - tighten the "reviewer dispatch" definition to align with whatever `kerf review` actually emits (the plan already calls out this cross-reference), or
  - ship D6 with an explicit "parsing-confidence" flag so kerf's findings are tagged and not loud-alarmed.

### Window unit — beads vs time

A bead window is the right unit for D6. Time-based windows (e.g., "last 7 days") couple the detector to project velocity: a slow week produces zero findings even if every commit was reviewer-absent. The bead window stays robust against velocity. Keep 30 beads.

### Recommendation

**Keep the 30-bead window, with one tightening.** Add a "no findings if window has <10 beads" guard for new/small projects (kerf had 22 reviewer-era beads followed by 28 absent — at the 10-bead minimum the detector remains meaningful and avoids screaming on a 3-bead repo). Specifically: D6 reports findings iff the lookback window has reached its full size (30 beads). Below that, suppress with reason "insufficient history."

## Verdict

**Verdict: thresholds OK as proposed — D1=60s, D6=last 30 beads — with two B1 follow-on items.**

- **D1 floor: 60s** (no change). Suppresses ≤4% of legitimate work; aligns with p10 of successful dispatch durations.
- **D6 window: 30 beads** (no change in number), plus add `min_window_beads=30` guard so D6 doesn't fire on projects with <30 historical beads.

### B1 follow-on items (not threshold changes; spec text changes)

1. **B1 must commit a sentence aligning D6's "reviewer dispatch" definition with `kerf review`'s output** (the plan already names this; calibration confirms it's load-bearing — without it, D6 produces ~28 kerf false positives).
2. **B1 should document the burst-dedup concern for D1:** ~53% of abandoned dispatches arrive within 60s of a sibling. A future detector or a `--collapse-bursts` flag should fold sibling abandons in the same session into one finding. Not a v1 blocker; capture only.

## Caveats

- The Python parser at `plans/012_real_corpus/data/extract.py` was used as-is. The Go parser landing in bead B2 is authoritative going forward, and may produce different counts. If B2 changes the harmonik wasted-effort count by more than ~25%, re-run this calibration before B1 lockdown.
- `kerf_wasted_effort.csv` has only 2 rows; both should be manually re-validated as true abandons before they are used as D1 fixtures.
- The reviewer presence in `kerf_beads.csv` is read from the `reviewer_seconds` column only (the CSV lacks a `reviewer_agent_id` column). Any change to that semantic in B2 (e.g., requiring an explicit reviewer agent ID) will shift kerf's 24/52 reviewer-present count downward.
- All analysis used `/tmp/calib_d1_d6.py` (not committed). The CSV inputs were not modified.
