# Diagnostics

> Detector family that reads Claude session transcripts and surfaces procedural drift kerf is meant to catch. Detectors run during `kerf next` feed assembly and emit findings as `kerf next` warnings. They do not run under `kerf doctor` — `kerf doctor` stays a fast, project-state snapshot surface; diagnostics are historical-corpus checks taking seconds to minutes. See [plan 013](../plans/013_self_diagnostics/) for the motivating incidents.

## Scope and surface

The diagnostics family ships with two detectors in v1:

- **D1 — abandoned dispatch** → `kerf next` warning kind `abandoned_dispatch`.
- **D6 — reviewer-absent commit** → `kerf next` warning kind `reviewer_absent`.

Both are non-fatal warnings: the feed still renders. Each finding becomes one warning entry following the field shape defined under [`kerf next` Warning kinds](commands.md#warning-kinds).

Future detectors (D2–D5, D7–D13) are captured at the end of this document with no normative content; they are not implemented in v1.

## Transcript discovery and parsing

Diagnostics read **Claude Code session JSONL** from outside the repo.

- **Canonical path template:** `~/.claude/projects/<encoded-repo>/`, where `<encoded-repo>` is the absolute repo path with `/` replaced by `-` and a leading `-` (Claude Code's on-disk convention). Example: the repo at `/Users/gb/github/kerf` resolves to `~/.claude/projects/-Users-gb-github-kerf/`.
- **Override:** the environment variable `KERF_TRANSCRIPT_DIR`, when set and non-empty, replaces the canonical template wholesale. The override path is used as given; no further encoding is applied.
- **Scoping (v1):** diagnostics are scoped to **Claude Code on macOS only**. Other harnesses and other operating systems are not supported in v1 and produce a "transcript directory not found" no-op (no findings, no error). Cross-harness discovery is deferred to a future plan.
- **Discovery failure is silent.** If the resolved directory does not exist or contains no `*.jsonl` files, diagnostics emit zero findings. They do not surface a `kerf next` warning of their own for "no transcripts" — that condition is the normal state on a fresh checkout.

The Go parser at `internal/kerftranscript/` is **authoritative**. The Python parser at `plans/012_real_corpus/data/extract.py` is archived as analysis-only; it does not run from kerf and is not maintained in lockstep.

### Bead-ID resolution

The detectors need to know whether a given bead has a paired commit and a paired reviewer dispatch. Resolution rules:

- **Bead-ID regex:** the project's `project.yaml` declares `bead.id_pattern` (a regex string). The indexer applies this pattern to every commit message subject and body, and to every transcript event's bead-id field, to extract bead IDs. The pattern source is the project's own configuration — kerf does not hard-code a regex.
- **Aliasing:** commit messages routinely reference parent bead IDs, sibling subtask IDs, or spec codenames rather than the exact dispatched bead ID. The indexer keys commits by **every** bead ID referenced, not just one, and follows parent/child links so subtask commits roll up to their parent dispatch.
- **Branch scope:** the indexer scans `git log --all` (worktree branch refs included), not just `main`. A commit on a worktree branch that has not yet merged into `main` is still evidence that the dispatched bead produced code.

## Reviewer dispatch (normative definition)

For the purposes of D6, a **reviewer dispatch** is a sub-agent dispatch event in the transcript whose payload was rendered from `kerf review`'s output for the same bead's owning work.

`kerf review` (see [commands.md §`kerf review`](commands.md#kerf-review)) emits a reviewer prompt as stdout text (or JSON under `--format=json`). It does **not** dispatch the reviewer itself — the calling harness pipes the prompt into a sub-agent. The transcript-observable signal of "a reviewer was dispatched" is therefore a sub-agent dispatch event whose prompt content carries one of the canonical `kerf review` output markers:

- A line matching `Reviewer prompt for <codename> — pass: <pass-name>` (the text-format header), **or**
- A JSON object containing the `kerf review` keys `{ "codename", "pass", "artifacts", "criteria" }` (the `--format=json` shape).

A sub-agent dispatch is **not** a reviewer dispatch if neither marker is present, even if the dispatch description contains the word "review" — kerf workflows commonly use "review" informally for non-reviewer work. This tight definition is load-bearing: a naive "any sub-agent whose prompt mentions review" rule produces ~28 false positives against kerf's own bead history (see Calibration below). The definition is anchored on `kerf review`'s emission shape so the two specs move together; if `kerf review`'s output format changes, this definition follows in the same plan.

A reviewer dispatch is paired to a bead when it occurs in the same Claude session as the bead's implementer commit and the rendered `kerf review` codename resolves (via the project's work registry) to the work that owns the bead.

## Diagnostic input vocabulary

The parser exposes the following event types to detectors (the on-disk JSONL schema is the parser's contract; this list is the post-parse view detectors operate on):

| Event | Meaning |
|-------|---------|
| `dispatch` | Orchestrator launched a sub-agent against a bead. Marks the start of a dispatch interval. Carries `session_id`, `sub_agent_id`, `bead_id`, optional `role`. |
| `tool_result` | A tool invocation returned to a sub-agent. Carries `is_error` when applicable. Used to observe sub-agent activity and to classify abandon-reason categories. |
| `commit_ref` | A commit landed referencing one or more bead IDs (via the indexer described above). Carries `commit_sha`, `bead_id`, `committed_at`. |
| `bead_close` | A bead was closed (possibly as SUBSUMED). Carries `commit_sha`. Closing a bead does **not** by itself satisfy D1 — only a `commit_ref` against the dispatched bead's ID does. |

## D1 — abandoned dispatch

### Signal

A sub-agent dispatch whose wall-time exceeds the dispatch floor, for which no `commit_ref` event referencing the dispatched `bead_id` (or any aliased ID per the indexer) exists across `git log --all`, and whose owning bead has no terminal status update.

### Threshold

- **Dispatch floor:** `60s`. Sub-agent dispatches shorter than 60 seconds are suppressed (the floor exists to silence instant fan-outs and cheap successful dispatches). Calibrated against `plans/012_real_corpus/data/` (see Calibration). 60s preserves >96% of successful dispatches in both kerf and harmonik (sub-floor density: harmonik 3/150 = 2.0%; kerf 2/52 = 3.8%) while suppressing the noise band below it.

### Reason categories (programmatic)

Each finding is tagged with one reason category, derived from the events at the dispatch tail:

| `reason_category` | Trigger |
|-------------------|---------|
| `appears_completed_no_commit` | Final sub-agent event is assistant text with no tool calls in the last 5 events. |
| `errored_mid_task` | Final event is a `tool_result` with `is_error: true`. |
| `orphaned` | Last event timestamp >24h before the current invocation and no continuation event. |
| `tool_linkage_broken` | Sub-agent finished but no parent-side result event was recorded. |

### Finding `detail` schema

```jsonc
{
  "kind": "abandoned_dispatch",
  "bead_id": "<id>",
  "detail": {
    "session_id":      "<session uuid>",
    "sub_agent_id":    "<sub-agent id>",
    "dispatched_at":   "<RFC3339 UTC>",
    "last_activity_at":"<RFC3339 UTC>",
    "reason_category": "appears_completed_no_commit" | "errored_mid_task" | "orphaned" | "tool_linkage_broken",
    "close_commit":    "<optional commit sha that retired the bead without implementing it, e.g. a SUBSUMED close>"
  }
}
```

### Burst-dedup note (capture only)

Calibration observed that **~53% of abandoned dispatches arrive within 60 seconds of a sibling abandon** in the same session (46/87 inter-arrival gaps below 60s; see `plans/013_self_diagnostics/calibration.md` §"Inter-arrival of abandoned dispatches"). This is a known burst pattern: orchestrators fan out sub-agents in tight groups, and when one abandons, neighbours often abandon together.

**This is a capture-only observation. It does not change the D1 threshold for v1.** The 60s floor stays. A future detector (or a `--collapse-bursts` flag on the renderer) may fold sibling abandons in the same session into a single finding; that work is out of scope for plan 013 and is tracked for plan 013b. Downstream implementers (the D1 detector bead) should be aware of the pattern when interpreting finding counts but should not pre-emptively dedup in v1.

## D6 — reviewer-absent commit

### Signal

For each bead commit in the configured window, no reviewer dispatch (per the normative definition above) was made in the same Claude session for the bead's owning work.

### Threshold and window

- **Window:** the **last 30 beads** ordered by `dispatch_ts` descending (most recent dispatch first). The unit is beads, not days — a time-based window would silently produce zero findings during slow weeks even when every commit was reviewer-absent.
- **Minimum history guard:** D6 emits no findings until the project has at least 30 beads in its dispatch history. Below that, the detector suppresses with the rationale "insufficient history" (this prevents loud alerts on new or small projects).

### Finding `detail` schema

```jsonc
{
  "kind": "reviewer_absent",
  "bead_id": "<id>",
  "detail": {
    "session_id":              "<session uuid>",
    "commit_sha":              "<commit sha>",
    "committed_at":            "<RFC3339 UTC>",
    "implementer_sub_agent_id":"<optional implementer sub-agent id>"
  }
}
```

### Calibration figures

Against `plans/012_real_corpus/data/kerf_beads.csv` (n=52), sorted by `dispatch_ts_utc` descending:

| Window | Reviewer-absent | Rate |
|-------:|----------------:|-----:|
| last 30 | **28 / 30** | 93.3% |
| last 50 | **28 / 50** | 56.0% |

Against `plans/012_real_corpus/data/harmonik_beads.csv` (recent harmonik era, n=150), sorted by dispatch timestamp descending, last-30 = 30/30 (100%) — the motivating "reviewer phase vanished" regression.

These numbers were recomputed for this spec against the dispatch-timestamp-descending sort over the named CSV inputs. They are quoted as the calibrated baseline for the D6 surface: a fresh `kerf next` invocation against a project in the "kerf 2026-05-15" state will fire D6 28 times on the last-30 window. That is correct behaviour, not noise — those 28 beads were genuinely committed without a paired `kerf review` dispatch in their session.

### Multi-bead transcript fixtures (normative)

A single transcript file may carry events for more than one bead. When the diagnostic runs:

- **The detector's finding output is scoped to the bead named by the invocation's `--bead` argument** when one is given. Only findings for that bead appear in the result.
- **When `--bead` is not given**, the detector reports findings for **every** bead with at least one `commit_ref` in the scanned transcripts whose owning work is active in the project, subject to the window guard.

This rule binds the parser (`internal/kerftranscript/` parser bead) and the indexer (bead-ID resolution bead): both must support the per-bead query path so the detector can answer "findings for bead X" without re-scanning the entire transcript corpus, and both must support the all-beads query path for the `kerf next` warning-channel surface (which does not pass `--bead`).

The fixture `internal/kerftranscript/testdata/d6_reviewer_absent_b.jsonl` is the concrete test case: its JSONL contains a `commit_ref` for `hk-zixbp` and a `commit_ref` for `hk-qo08q` in the same session; the paired golden output `d6_reviewer_absent_b.golden.json` lists exactly one finding, for `hk-zixbp` only. That golden file therefore represents the `--bead=hk-zixbp` query result. The companion fixture `d6_reviewer_absent_c.jsonl` carries the per-bead query for `hk-qo08q`. The all-beads query (no `--bead`) against the union of those JSONLs would yield two findings; a separate golden may capture that case in the integration test bead (B-E2E).

## Severity and fatality

Both `abandoned_dispatch` and `reviewer_absent` are **non-fatal** warnings in `kerf next`. The ranked feed still renders. Severity is informational; agents are expected to act on findings within the same loop cycle but `kerf next` does not block.

The mirror table in [coordination.md §Feed-warning rules](coordination.md#feed-warning-rules) lists these kinds with their fatality and detector location.

## Future detectors (capture only)

The following detectors are deferred to a follow-on plan (Plan 013b) and ship no normative content here. Their names and one-line summaries are preserved so cross-references from existing plans remain valid:

- **D2 — Workflow phase regression.** Rolling-window stat that detects when a previously-present phase (e.g., reviewer) disappears from recent beads. Deferred until D6 calibration teaches cohort sizing.
- **D3 — Stalled conflict resolution.** Tracks `git push` rejection / `CONFLICT` markers that persist past a threshold. Threshold needs re-calibration against real conflict durations (corpus shows 11–35h conflicts) before shipping.
- **D4 — Outlier task-work duration.** Statistical, post-hoc analysis. Fine on `kerf doctor` (manual surface) when it ships.
- **D5 — Silent retry.** A bead re-dispatched after an abandon without an intervening reconciliation step. Depends on the D1 dispatch index.
- **D7 — Plan/bead ratio drift.**
- **D8 — Area starvation.**
- **D9 — Spec drift not triaged.**
- **D10 — Two parallel sub-agents in same area.**
- **D11 — Sub-agent ran longer than parent's last status update.**
- **D12 — Bead closed but commit message doesn't reference it.**
- **D13 — Reviewer was dispatched but didn't emit an approval.**

None of D2–D5 or D7–D13 are implemented in v1. Adding any of them requires its own plan and its own bead set.
