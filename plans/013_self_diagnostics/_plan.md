# Plan 013 — Self-Diagnostics from Claude Transcripts

> **Status: DRAFT.** Captures the user's 2026-05-15 idea: build the Claude-transcript analysis from Plan 012 INTO kerf, so the procedural issues we just discovered are continuously surfaced and can be fixed over time. Independent of Plans 011 and 012; consumes their extractor infrastructure.

## Intent

While extracting duration data for the simulator (Plan 012), the investigation surfaced procedural issues in real workflows that no one had been actively monitoring:

- **123 abandoned sub-agent dispatches in harmonik** (~25% rate in recent sessions). Sub-agents ran but produced no commit — beads silently not closed, work invisible.
- **Reviewer phase missing from 100% of recent harmonik beads** vs. present in 46% of kerf beads. A workflow regression: the project once enforced reviewers, then stopped, and no one was notified.
- **Wasted-effort signals** scattered across abandoned `worktree-agent-*` branches, unmatched orchestrator dispatches, and `git push` rejections that took 20 minutes to resolve.

These are the kinds of issues kerf is *meant* to catch. The extraction pipeline that found them is reusable — turn it into a built-in `kerf diagnose` (or similar) command that runs against any kerf-tracked project's Claude transcripts and surfaces the same findings on demand.

## Why

- The user's instinct: "we should be able to surface those procedural issues so they can be fixed up over time." Diagnostics are how that happens.
- Harmonik's "reviewer phase vanished" is the exact failure mode kerf should refuse to let happen silently. The user explicitly said: *"this isn't great — that's why I want harmonik so the system doesn't have a choice whether it performs actions or not — they are procedural/enforced through harmonik's workflows."* Kerf can be the enforcement layer.
- The signals are already programmatically detectable. No model needed for the first pass.
- Same data we collect for the simulator's duration distributions can power diagnostics — single extraction pipeline, two consumers.

## Scope: what kerf detects

Initial detector set (each is independently shippable):

### D1 — Abandoned dispatch

**Signal:** sub-agent ran for >N seconds (suggest 60s default), produced no `git commit` anywhere reachable from the bead's parent/sibling IDs, the matching bead has no terminal status update.

**Critical implementation note (from 2026-05-15 investigation):** the naive version of this detector throws ~80% false positives. Real-world commit messages reference parent bead IDs, sibling subtask IDs, or spec codenames — not always the exact dispatched bead ID. The detector MUST:
- Index commit messages by **all** bead IDs they reference (regex `hk-[a-z0-9]+(\.\d+)?` or kerf's equivalent), not just one.
- Follow parent/child bead links so subtask commits roll up.
- Scan worktree branch refs (not just `main`) before flagging.
- Distinguish "agent never produced output" from "agent committed but result didn't land on integration branch."

**Surfacing:** `kerf diagnose` lists each abandoned dispatch with sub-agent id, bead id, duration, last activity, suspected reason category.

**Reason categories (programmatic):**
- Ended with assistant text but no tool calls in last 5 events → "appears completed; no commit"
- Last event is a tool_result with `is_error: true` → "errored mid-task"
- Last event timestamp >24h ago and no continuation → "orphaned"
- Sub-agent finished but parent never received the result → "tool linkage broken"

### D2 — Workflow phase regression

**Signal:** a phase that historically appeared in ≥X% of beads is missing in the last Y beads. Example: reviewer phase was 100% during week N-2, 80% during N-1, 0% during N.

**Surfacing:** "Reviewer phase dropped from 78% to 0% over the last 30 beads. Last bead with a reviewer: hk-XXXX, 12 days ago."

**Optionally:** allow projects to declare expected phases (in `project.yaml` or a sibling diagnostics config) and alert on any drop.

### D3 — Stalled conflict resolution

**Signal:** `git push` rejected or `CONFLICT` marker, no successful commit within K minutes after.

**Surfacing:** "Active conflict on session ABCD for hk-XXXX; rejected push at HH:MM, no resolution in 18 minutes."

### D4 — Cross-bead time-since-last-commit anomaly

**Signal:** a sub-agent's task-work duration is >Pp95 of the project's recent distribution.

**Surfacing:** "kerf-XXX took 47 min vs project p95 of 12 min — review for stuck patterns."

Plan 012's per-phase distributions feed this directly.

### D5 — Sub-agent retry without reconciliation

**Signal:** same bead id dispatched as two consecutive sub-agents in the same session, where the first produced no commit and the second produced one. Tells the project "we silently re-tried — was the first attempt's work lost?"

### D6 — Reviewer-absent commits

**Signal:** a bead commit landed but no reviewer sub-agent was dispatched. Counts toward D2 trend but also useful per-bead.

## Surface design (sketch)

Two commands:

```
kerf diagnose                  # current state — emit findings
kerf diagnose --since 7d       # detector run scoped to a window
kerf diagnose --detector D1    # one detector
kerf diagnose --format json    # for tools/CI
```

```
kerf diagnose watch            # daemon mode — emit notification per new finding
```

Or fold into the existing `kerf triage` (Plan 009 — drift + outstanding work surface) as a new section. Probably the cleanest path; triage is already "what's wrong with this project today."

### Output shape

```
kerf diagnose — project: harmonik, window: last 30 days

D1 Abandoned dispatches: 28
  Most recent: hk-iuaed.2 (5h ago) — appears completed; no commit
  Top reason: appears completed; no commit (19/28)
  Run `kerf diagnose --detector D1 --verbose` for full list.

D2 Workflow phase regressions: 1
  Reviewer phase: 78% (30d ago) → 0% (last 30 beads)
  Last reviewer-tagged bead: hk-fzc1 (12 days ago)

D3 Stalled conflicts: 0
D4 Outlier task durations: 3
D5 Silent retries: 4
D6 Reviewer-absent commits: 29 of 30 last beads
```

## Architecture

```
internal/
  transcript/        # JSONL parser shared with simulator (Plan 012 extractor)
    parser.go
    sub_agent.go
    phase.go         # phase detection from events
  diagnose/
    d1_abandoned.go
    d2_phase_regression.go
    d3_stalled_conflict.go
    d4_outlier_duration.go
    d5_silent_retry.go
    d6_reviewer_absent.go
    registry.go      # detector registry; new ones plug in here
  cmd/
    diagnose.go      # CLI surface
```

Each detector implements:

```go
type Detector interface {
  ID() string
  Description() string
  Run(corpus Corpus, window Window) []Finding
}

type Finding struct {
  DetectorID  string
  Severity    Severity   // info, warn, error
  BeadID      string     // optional
  SessionID   string     // optional
  Timestamp   time.Time
  Summary     string
  Detail      map[string]any
}
```

Adding a detector = one new file + registry entry.

## Sequencing

- **L0:** D1, D6 (single-snapshot detectors, simplest). Ship one detector first to validate the surface.
- **L1:** D3, D5 (event-pattern detectors). Reuse the conflict-hunter logic from Plan 012's extraction.
- **L2:** D2, D4 (require historical baseline). Depends on Plan 012's duration distributions for D4.

## Specs touched

- New `specs/diagnostics.md` — defines what each detector looks for, severity, output format.
- `specs/commands.md` — `kerf diagnose` (or extension of `kerf triage`).

## Open decisions

1. **Surface command.** New `kerf diagnose` or extension of `kerf triage`? Default: extend triage; it's already the "what's wrong today" surface.
2. **Project config for detector thresholds.** Hard-coded defaults vs `.kerf/diagnostics.yaml` per-project? Default: hard-coded for v1, config in v2.
3. **Notifications / daemon mode.** Out of v1; just on-demand CLI.
4. **Cross-project diagnostics.** Should kerf in repo A be able to diagnose repo B's transcripts? Default: no; one project at a time.
5. **`kerf diagnose --explain D1`** to show *why* something was flagged. Probably yes from day 1 — it's the difference between alerts people fix and alerts people ignore.

## Notes for future detectors (capture as ideas land)

- **D7 — Plan/bead ratio drift.** Beads being created without matching plan updates (or vice versa).
- **D8 — Area starvation.** A declared area has no recent bead activity but other areas do.
- **D9 — Spec drift not triaged.** `kerf triage` reports drift that's been outstanding for N days.
- **D10 — Two parallel sub-agents in same area.** Plan 008 already showed area_collisions hurt throughput; flag it live.
- **D11 — Sub-agent ran for longer than its parent's last status update.** Orchestrator stopped waiting before worker finished — likely a context-bleed bug.
- **D12 — Bead closed but commit message doesn't reference it.** Audit trail broken.
- **D13 — Reviewer-was-dispatched-but-didn't-emit-an-approval.** Maps to the user's specific concern about enforcement.
