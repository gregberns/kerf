# Critique — Spec Coverage

## Headline

Plan 013 has **no canonical spec sentence yet**. It proposes a new
`specs/diagnostics.md` but does not draft the normative sentences each
detector implements. Without a concrete sentence per detector, the
review-gate procedure ("name the spec sentence the bead claims to satisfy")
cannot run. This must be drafted before beads are dispatched.

## What "canonical spec sentence" means here

Per kerf's review-gate notes (`CLAUDE.md` §"Review Gate — Functional
Verification"): each bead claims to satisfy one specific spec sentence,
the reviewer reads that sentence, runs the feature against the sentence,
and confirms the observed output matches. Without the sentence, the
reviewer is rubber-stamping the diff.

## Proposed sentence per detector

Drafted here; the implementer-of-spec bead will tighten them. All live in
`specs/diagnostics.md` §"Detectors".

### D1 — abandoned dispatch

> The `d1-abandoned` detector reports a finding for every sub-agent
> dispatch in the project's transcripts where (a) the sub-agent ran for at
> least 60 seconds, (b) no commit reachable from any local or worktree
> branch references the dispatched bead ID via the project's bead-ID
> regex (including parent/child rollup and SUBSUMED close-commits), and
> (c) the dispatched bead has no terminal status in the bead store. Each
> finding carries `sub_agent_id`, `bead_id`, `dispatched_at`, `duration_s`,
> `last_event_kind`, `reason_category` ∈ {`appears_completed_no_commit`,
> `errored_mid_task`, `orphaned`, `tool_linkage_broken`}.

### D2 — workflow phase regression

> The `d2-phase-regression` detector reports a finding when a workflow
> phase (e.g., `reviewer`) that appeared in at least 50% of the project's
> last 30 beads before a cutoff date now appears in fewer than 10% of the
> last 30 beads. Each finding carries `phase`, `historical_rate`,
> `current_rate`, `cutoff_date`, `last_bead_with_phase`. Projects may
> override the 50%/10%/30-bead defaults via
> `project.yaml: doctor.phase_regression.{historical_pct, current_pct, window}`.

### D3 — stalled conflict resolution

> The `d3-stalled-conflict` detector reports a finding when a session's
> transcript contains a `git push` rejection or `CONFLICT` marker and no
> successful commit in the same session within 30 minutes after the
> rejection. Each finding carries `session_id`, `bead_id` (when
> recoverable), `rejected_at`, `minutes_unresolved`.

### D4 — outlier task-work duration

> The `d4-outlier-duration` detector reports a finding for every bead
> whose task-work duration exceeds both (a) 600 seconds (absolute floor)
> and (b) twice the project's median task-work duration over the most
> recent 30 beads. When the project has fewer than 20 beads with
> measured task-work, the detector emits a single yellow finding stating
> the detector is dormant pending sufficient sample. Each fired finding
> carries `bead_id`, `task_work_s`, `project_median_s`, `multiplier`.

### D5 — silent retry

> The `d5-silent-retry` detector reports a finding when two or more
> sub-agent dispatches for the same bead ID appear within the same session
> and the first dispatch produced no commit (would satisfy D1's commit
> test). Each finding carries `bead_id`, `session_id`, `first_dispatch_id`,
> `second_dispatch_id`, `gap_minutes`. Cross-session retries are out of
> scope in v1.

### D6 — reviewer-absent commit

> The `d6-reviewer-absent` detector reports a finding for every commit
> attributed to a bead where no sub-agent dispatch in the same session
> carried the reviewer phase marker. When `d2-phase-regression` would
> already fire for the same cohort, `d6` suppresses its findings and
> emits a single info-level row linking to the d2 finding.

## Specs that need to exist

1. **`specs/diagnostics.md`** (new). Contains the six sentences above plus:
   - Project-config knobs (per-detector overrides under `doctor:` in
     `project.yaml`).
   - Severity mapping (D1 yellow, D2 red, D3 yellow, D4 yellow, D5 yellow,
     D6 info/yellow).
   - Transcript discovery rule (which paths kerf reads from).
   - Output schema fields per finding (`detail` map keys per detector).

2. **`specs/commands.md`** — `kerf doctor` detector table grows six rows;
   no command-level changes.

3. **`specs/architecture.md`** — small section under "Transcript ingestion"
   documenting the source path convention.

4. **`specs/cli.md`** — no change. The existing rule that diagnostic
   commands are read-only and "name the fix" already covers Plan 013.

## What's already covered elsewhere

- The `Detector` / `Finding` / `Registry` contract is already in
  `specs/commands.md` §"kerf doctor". Plan 013's `Detector` interface
  block can be deleted; just reference the existing spec.
- Severity vocabulary (`green | yellow | red`) is set; Plan 013's
  `info | warn | error` is non-canonical and should be retired.

## What's missing

- **Transcript discovery rule.** No spec sentence anywhere in the tree
  says where kerf finds Claude transcripts. This is a top-level gap.
- **Bead-ID regex.** Plan 013 mentions `hk-[a-z0-9]+(\.\d+)?` as an
  example. The actual regex must be a project-config value (different
  projects use different prefixes — `hk-`, `kerf-`, etc.). Spec sentence:
  "The bead-ID regex is read from `project.yaml: bead.id_pattern`,
  defaulting to `<project_id>-[a-z0-9]+(\.[0-9]+)?` when unset."
- **What counts as a "reviewer dispatch."** D2 and D6 both need this. Is
  it a sub-agent whose system prompt contains "reviewer"? Whose user-
  message names a phase marker? This needs a normative definition.
- **What counts as "in flight" for D3.** Session not ended? Last event
  within K hours? Sentence must pin this.

## Recommendation

1. The first bead is "draft `specs/diagnostics.md` with the six sentences
   above plus the transcript-discovery, bead-ID-regex, and reviewer-phase
   definitions." Everything else depends on this.
2. Each downstream detector bead's review gate names one of the sentences
   above (or its tightened replacement) explicitly.
3. Sentences must commit to numbers (`60s`, `30 beads`, `50%`, `600s`,
   `2× median`). The plan currently leaves these as "default:" lines,
   which is fine for the plan but not for the spec.
