# Detector Examples — Plan 013

Real incidents extracted from the kerf and harmonik transcript corpora and from the CSVs produced by Plan 012. Used to validate detector design and as fixture seeds.

Conventions: timestamps are UTC, durations are seconds unless noted, bead IDs and sub-agent IDs are quoted verbatim, sessions identified by their UUID.

## D1 — Abandoned dispatch

`harmonik_wasted_effort.csv` lists 123 candidates. 121 of 122 distinct bead IDs in that set were never committed under their own ID anywhere in the bead-CSV index — but spot-checking against `git log --all` in `/Users/gb/github/harmonik` shows several of them did land as commits the indexer missed (e.g. `hk-a0htu` → commit `93aeaae`, `hk-sx9r.1` → close commit `43fe576`, `hk-4goy3` → `423d78f`). So the raw list is a mix of true abandons and indexer false negatives.

Two examples that look like true D1 (no commit in any branch under the bead ID, dispatched in a session that did produce other commits):

- Session `fed61a3d-3aa9-4c8a-91e7-0b1acb4ec1e8`, sub-agent `aa848865eff923eae`, bead `hk-qo08q.15`, dispatched 2026-05-15T18:10:11Z. No commit anywhere on `hk-qo08q.15`; the bead later appears in a close-as-SUBSUMED commit (`4a3c217`), suggesting the dispatch was abandoned and the bead retired without code. Likely cause: orchestrator realized the bead was subsumed mid-flight but never cleaned up the dispatch.
- Session `fed61a3d-...`, sub-agent `a97ff03cb5db4f5d9`, bead `hk-2ubs8`, dispatched 2026-05-15T18:10:20Z (9s after the previous one). Same SUBSUMED close commit. Pattern: a burst of dispatches all aimed at beads that the orchestrator simultaneously decided to retire — classic abandoned-dispatch fan-out.

A third concrete one from a different session:

- Session `db8d1c56-2c60-4b55-83fe-c29ab3ff3eea`, sub-agent `abd9a6f51d095d196`, bead `hk-4goy3`, dispatched 2026-05-15T15:46:52Z. Eventually committed under a DIFFERENT bead context (`423d78f` mentions filing `hk-4goy3` as a follow-up, not implementing it). The original dispatch produced nothing.

## D2 — Workflow phase regression

Headline example (harmonik reviewer phase):

- `harmonik_reviewer_beads.csv` lists 34 beads with a reviewer phase. All 34 commits cluster between 2026-05-07T05:08Z and 2026-05-08T04:01Z, across sessions `196fb94b-...`, `14c78c5f-...`, and one or two neighbors.
- `harmonik_beads.csv` has 150 beads spanning 2026-05-10T16:27Z through 2026-05-15T20:23Z. Zero of those have a reviewer agent populated.
- Cutoff: reviewers vanish on or before 2026-05-10. Roughly 6/33 oldest sessions (those overlapping 05-07/05-08) emit reviewers consistently; 0/60 newer beads do.

Secondary D2 candidate worth flagging but weaker: kerf shows reviewer phases in Plan 008 / 009 commits (per recent git log: "reviewer approved", "kerf-mgg snapshot test") but the `kerf_beads.csv` reviewer column is sparsely populated, suggesting the reviewer phase is happening conversationally without the structured handoff the parser keys on. This is a parsing gap masquerading as a regression — flag during D2 implementation.

## D3 — Stalled conflict resolution

From `conflict_incidents.csv`, filtered to patterns 1 (push rejected) and 2 (CONFLICT marker):

- Session `4c89151e-f376-464a-9b96-b3a7f6522442` (harmonik), pattern 1 push-rejected, 2026-04-26T04:42:27Z → 2026-04-27T16:09:18Z. Duration 127,610s (~35.4 hours) between rejection and the session's last event. Same session also tagged "long session with conflict markers".
- Session `de30293f-66f5-4c4a-b29c-98607b0c4cb2` (harmonik), pattern 1 push-rejected, 2026-04-23T22:02:50Z → 2026-04-24T17:49:30Z. Duration 71,199s (~19.8 hours).
- Tertiary: session `55603e31-930e-4b1f-a03a-e31483641041` (kerf), pattern 2 CONFLICT marker around bead `kerf-v20`, 2026-05-15T06:24:54Z → 2026-05-15T17:30:19Z, ~11.1 hours. Bead ID is captured, so this is a clean fixture.

## D4 — Outlier task-work duration

Harmonik (n=150, p95=1219.3s, p99=5193.9s, max=6559.3s):

- Bead `hk-gql20.14`, session `12688a9a-00b5-4bd6-b7a2-8fa84c0d6da2`, task_work 6559.3s — 5.4x project p95.
- Bead `hk-8mup.11`, session `14c78c5f-11eb-43eb-9956-963abdecc7db`, task_work 5193.9s — 4.3x p95 (also the p99 tip).

Kerf (n=52, p95=555.0s, p99=696.4s):

- Bead `kerf-3b2`, task_work 696.4s — 1.25x p95, sits at p99 (only 52 beads so p99 ≈ max).
- Bead `kerf-6n4`, task_work 624.9s — 1.13x p95.

Caveat: kerf p95/p99 are very close because the sample is small. The detector should require both an absolute floor (e.g. >2x project p95 AND >300s above the p95) before flagging in low-volume projects, or it will fire constantly on kerf.

## D5 — Silent retry

- Session `196fb94b-5f6d-4ffb-b5b7-28a8c02c84ab`, bead `hk-pvcs.2`. First dispatch sub-agent `addabf48ec84e0dd0` at 2026-05-07T05:06:44Z — no commit. Second dispatch sub-agent `ab73f75174cfecfb2` at 2026-05-07T05:10:46Z (4 minutes later) — committed as `ad369e9` with reviewer `aece9d2c86f190d3c`. Classic silent retry with no orchestrator-visible acknowledgement that the first attempt was abandoned.
- Session `6e3ce55e-a509-4a24-b2b7-a399e3e7682b`, bead `hk-kqdpf.5`. Three back-to-back dispatches at 2026-05-13T17:21, 17:33, 18:08 by sub-agents `aaa96803fda144b10`, `ad2a919f72aecb929`, `af5c221c93e33c3b7`. All three actually committed (different SHAs), so this is the converse: the orchestrator retried a successful bead three times in 47 minutes. Either a re-dispatch loop or a stale-state bug — but still a "silent retry" pattern the detector should catch and warn on.

Cross-session retry exists too: `hk-8i31.37` was wasted in session `a1b6aa78-...` at 2026-05-11T22:31Z, then committed cleanly in the same session at 2026-05-12T16:14Z by a different sub-agent — 17.7 hours later. Same-session, but spanning the orchestrator's overnight gap.

## D6 — Reviewer-absent commit

Most recent harmonik commits with `reviewer_seconds` empty (top of `harmonik_beads.csv` sorted by `commit_ts` desc):

- Bead `hk-iuaed.6`, commit `dcd7f7e5d1a5eb4cf6dc4b292d86a5ea01562c4f`, committed 2026-05-15T20:23:40Z in session `801120b5-...`. No reviewer dispatched.
- Bead `hk-zixbp`, commit `cc3da5c1b255fd5bd2c94e859d4d653ae6d1e5c6`, committed 2026-05-15T19:03:48Z in session `4c818416-...`. No reviewer.
- Bead `hk-qo08q`, commit `76a55be161a5d0c071fd511c47c57f30688ac1ec`, committed 2026-05-15T18:53:31Z, same session. No reviewer.

All three are in the post-2026-05-10 "no-reviewer era" — D2 and D6 will fire on the same population. The detectors are not independent and should share a fixture set.

## Validation summary

- True D1 incidents across both corpora: roughly 20–30 once indexer false negatives are removed. The current 123-row `harmonik_wasted_effort.csv` is ~75–80% false positives (the bead got committed but the indexer missed it). The per-investigator's improved indexing (all `hk-*` IDs, worktree refs, close commits) should drop the count by that much. Kerf: only 2 wasted-effort rows, both worth manually re-validating before using as fixtures.
- Brittle detectors:
  - **D1**: heavily depends on indexer correctness. Until the indexer catches close-commits, worktree refs, and aliased bead IDs (e.g. `hk-a0htu` mentioned in a parent bead's commit), D1 will have a high FP rate. Detector should report confidence and link to the indexer's evidence, not assert "abandoned".
  - **D4**: in low-volume projects (kerf, n=52), p95 is too close to p99 to be a useful threshold alone. Add an absolute-duration floor or require an effect size (≥2x p95).
  - **D2/D6 overlap**: same incidents will trigger both. Either merge them or have D6 suppress when D2 already explains the cohort.
