# Plan 013 — Self-Diagnostics from Claude Transcripts

> **Status: DRAFT (revised 2026-05-19).** Captures the user's 2026-05-15 idea: build the Claude-transcript analysis from Plan 012 INTO kerf, so the procedural issues we just discovered are continuously surfaced. Consumes Plan 012's extractor as a porting reference.

## 2026-05-19 revision (independent review applied)

The original draft scoped 6 detectors (D1–D6) shipping together. The independent review (`critiques/independent_review.md`) found this over-built for v1 and ambiguous on consumption surface. This revision applies its recommendations.

**Changes from previous draft:**

1. **Scope cut to D1 + D6 only.** D2/D3/D4/D5 deferred to a follow-on Plan 013b after D1+D6 run against real corpora. Both motivating incidents — abandoned dispatches and the harmonik reviewer regression — are still covered.
2. **Surface decided per-detector** (was: all on `kerf doctor`):
   - **D1 (abandoned dispatch)** → `kerf next` warning channel. Operators won't run `kerf doctor` every wave; abandoned dispatches must interrupt the pull loop to be acted on. The plan's own quote ("enforced through workflows, not optional") demands the always-on surface.
   - **D6 (reviewer-absent commit)** → also `kerf next` warning channel. Per-bead signal; the agent should see it the cycle after the missing-reviewer commit lands, not on a manual `kerf doctor` invocation. Lives next to D1 to share the warning footer rendering.
   `kerf doctor` stays a fast, snapshot, project-state surface; the new historical-corpus detectors do not join its default run. (If a later detector is genuinely batch / on-demand, it can land under `kerf doctor` — but D1 and D6 do not fit there.)
3. **Calibration bead added before spec lockdown.** B-CAL runs proposed thresholds against the existing `plans/012_real_corpus/data/` CSVs and records observed finding counts. B1 (spec draft) commits numbers only after calibration.
4. **Superseded `kerf diagnose` content removed.** The original architecture sketch, sample output for `kerf diagnose`, and `watch` mode are gone — they were already overruled by the in-thread reconciliation but lingered in the doc. Removed below.
5. **Parser duplication acknowledged.** Plan 012's Python parser at `plans/012_real_corpus/data/extract.py` is **archived as analysis-only** going forward. The Go parser landing in this plan (under `internal/kerftranscript/`) is **authoritative**. Plan 012 should re-route through it when it next needs to re-extract; no concurrent maintenance of two parsers.
6. **`5 vs 6 waves` mismatch fixed** in `beads.md` (now 3 waves / 7 beads).
7. **Transcript discovery rule made explicit, not hard-coded.** `~/.claude/projects/-Users-gb-github-harmonik/` is macOS- and user-specific. B1 commits the discovery rule (canonical path template + `KERF_TRANSCRIPT_DIR` override + a "v1 = Claude Code on macOS only" scoping sentence).

The deferred detectors are not lost — their spec sentences still land in `specs/diagnostics.md` (capture only, no implementation). Plan 013b picks them up after calibrated learning from D1+D6.

## Intent

While extracting duration data for the simulator (Plan 012), the investigation surfaced procedural issues in real workflows that no one had been actively monitoring:

- **123 abandoned sub-agent dispatches in harmonik** (~25% rate in recent sessions). Sub-agents ran but produced no commit — beads silently not closed, work invisible.
- **Reviewer phase missing from 100% of recent harmonik beads** vs. present in 46% of kerf beads. A workflow regression: the project once enforced reviewers, then stopped, and no one was notified.

These are the kinds of issues kerf is *meant* to catch. The extraction pipeline that found them is reusable — turn it into a built-in detector family that runs against any kerf-tracked project's Claude transcripts and surfaces the same findings on demand.

## Why

- The user's instinct: "we should be able to surface those procedural issues so they can be fixed up over time." Diagnostics are how that happens.
- Harmonik's "reviewer phase vanished" is the exact failure mode kerf should refuse to let happen silently. The user explicitly said: *"this isn't great — that's why I want harmonik so the system doesn't have a choice whether it performs actions or not — they are procedural/enforced through harmonik's workflows."* Kerf can be the enforcement layer, but only if the signal interrupts the loop (hence `kerf next` warnings, not `kerf doctor`).
- The signals are already programmatically detectable. No model needed for the first pass.

## Scope: what kerf detects (v1)

### D1 — Abandoned dispatch

**Signal:** sub-agent ran for >N seconds (proposed 60s default, calibrated by B-CAL), produced no `git commit` anywhere reachable from the bead's parent/sibling IDs, the matching bead has no terminal status update.

**Critical implementation note (from 2026-05-15 investigation):** the naive version of this detector throws ~75–80% false positives. Real-world commit messages reference parent bead IDs, sibling subtask IDs, or spec codenames — not always the exact dispatched bead ID. The detector MUST:
- Index commit messages by **all** bead IDs they reference (regex from `project.yaml: bead.id_pattern`), not just one.
- Follow parent/child bead links so subtask commits roll up.
- Scan worktree branch refs (not just `main`) before flagging.
- Distinguish "agent never produced output" from "agent committed but result didn't land on integration branch."

**Surface:** `kerf next` warning. Each abandoned dispatch becomes one warning entry with bead id, duration, last activity, suspected reason category.

**Reason categories (programmatic):**
- Ended with assistant text but no tool calls in last 5 events → "appears completed; no commit"
- Last event is a tool_result with `is_error: true` → "errored mid-task"
- Last event timestamp >24h ago and no continuation → "orphaned"
- Sub-agent finished but parent never received the result → "tool linkage broken"

### D6 — Reviewer-absent commit

**Signal:** a bead commit landed but no reviewer sub-agent was dispatched in the same session for that bead.

**Surface:** `kerf next` warning. One entry per recent reviewer-absent bead within the configured window.

**Cross-link:** `kerf review` (`specs/commands.md:529`) defines the canonical reviewer dispatch. D6's "reviewer dispatch" definition must align with whatever `kerf review` produces; B1 commits a sentence to this effect in `specs/diagnostics.md`.

## Detectors deferred to Plan 013b

Captured for context; **not** in scope for this plan. Spec sentences for these still land in `specs/diagnostics.md` under a "Future detectors" section (no normative content, no thresholds committed).

- **D2 — Workflow phase regression.** Rolling-window stat. Needs baseline; defer until D6 calibration teaches us cohort size.
- **D3 — Stalled conflict resolution.** Real-time `git push` rejection / `CONFLICT` marker tracker. The corpus shows conflicts persist 11–35 hours, so the 30-min threshold needs re-calibration; defer.
- **D4 — Outlier task-work duration.** Statistical, post-hoc. Fine on `kerf doctor` (manual surface) when it ships.
- **D5 — Silent retry.** Depends on D1's dispatch index.

Future-detector ideas D7–D13 from the previous draft remain in capture-only state.

## Threshold defaults

**No numbers are committed in this document.** The previous draft committed (60s dispatch floor, 30-bead baseline, etc.) by feel. B-CAL runs the proposed defaults against the existing `plans/012_real_corpus/data/` CSV extracts and reports observed finding counts. B1 commits thresholds informed by that calibration.

Proposed starting points for B-CAL to validate:
- D1 dispatch floor: 60s (suppress instant fan-outs).
- D6 lookback window: last 30 beads (matches the cohort definition in the source examples).

B-CAL exits with one of: "thresholds OK as proposed" (then B1 commits them verbatim) or "revise to X" (then B1 commits X with the calibration rationale next to each number).

## Parser duplication — resolved decision

Plan 012's parser at `plans/012_real_corpus/data/extract.py` is **archived as analysis-only**. No further iteration there. The Go parser that B2 lands at `internal/kerftranscript/` is **authoritative going forward**. When Plan 012 next needs to extract transcripts (e.g. for new duration distributions), it re-routes through the Go binary — not by re-running the Python.

This matches the kerf principle (`feedback_integrated_tests.md`) that two implementations diverge. B4 includes a fixture transcript and a golden findings file; if the Python parser disagrees on the same fixture it does not matter, because Python is no longer authoritative.

## Surface specification

- **`kerf next` warning kinds (new):**
  - `abandoned_dispatch` — fires for each D1 finding within the window. Non-fatal; feed still renders.
  - `reviewer_absent` — fires for each D6 finding within the window. Non-fatal.
- B1 writes these into `specs/commands.md` §"`kerf next`" §"Warning kinds", following the existing `corrupt_spec` / `no_project_yaml` pattern (title / action / reason fields).
- `kerf doctor` is not extended in this plan. Its existing 5 detectors stay fast and snapshot. The `internal/diagnose/` (or `internal/kerftranscript/diag/`) package and detector definitions still implement the existing `doctor.Detector` interface for future reuse, but registration into `kerf next`'s warning channel is the only surface point this plan ships.

## Specs touched

- New `specs/diagnostics.md` — defines D1 and D6 (signal, severity, calibrated thresholds, finding shape, suppression rules), transcript-discovery rule (canonical path + `KERF_TRANSCRIPT_DIR`), `bead.id_pattern` config key, the normative "reviewer dispatch" definition, and a capture-only "Future detectors" section for D2–D5.
- `specs/commands.md` — adds two warning kinds to `kerf next`.

## Open decisions (after revision)

1. **Project-config thresholds.** Hard-coded defaults vs `.kerf/diagnostics.yaml` per-project? Default: hard-coded for v1, config in v2.
2. **Cross-project diagnostics.** Should kerf in repo A be able to diagnose repo B's transcripts? Default: no; one project at a time.
3. **`--explain` flag** on each warning — show why something was flagged. Probably yes from day 1; the difference between alerts people fix and alerts people ignore.

(The previous draft's open decisions about surface command and daemon mode are now closed — `kerf next` warnings, no daemon.)

## Notes for future detectors (capture only)

- **D2** workflow phase regression.
- **D3** stalled conflict resolution (recalibrate threshold from 30 min after seeing real conflict durations).
- **D4** cross-bead duration anomaly.
- **D5** sub-agent retry without reconciliation.
- **D7** plan/bead ratio drift.
- **D8** area starvation.
- **D9** spec drift not triaged.
- **D10** two parallel sub-agents in same area.
- **D11** sub-agent ran longer than parent's last status update.
- **D12** bead closed but commit message doesn't reference it.
- **D13** reviewer-was-dispatched-but-didn't-emit-an-approval.
