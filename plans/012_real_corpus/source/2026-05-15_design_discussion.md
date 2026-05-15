# Design Discussion — 2026-05-15

Captured so the next session can pick this up without re-deriving it. Translates internal jargon, summarizes prior-art search, records every decision.

## What kicked this off

Plan 008 (exploratory testing) and Plan 009 (triage workflow) shipped. Backlog was nominally empty. But three deferred items from the simulator track were sitting on disk without follow-up beads:

1. Adversarial scenarios designed in Plan 008 (`adversarial.md`) — never run.
2. Saturated re-runs recommended in Plan 008 findings — never done.
3. Weight tuning hypotheses in Plan 008 findings — never tested.
4. `priority_inversions` semantic fix diagnosed in bead `kerf-3b2` — deferred without a follow-up bead.

Plus three not-yet-discussed gaps:

5. No scenario varies agent count on the same workload. Existing scenarios all hard-code agents ∈ {3, 4, 5}. We want sweeps across {1, 2, 3, 5, 7, 10}.
6. All scenarios are synthetic. No real workload data has been loaded.
7. All durations are placeholder lognormal. No real timing data has been measured.

## What we already knew but had forgotten

Four background investigation agents recovered the prior-art picture:

**Two May 14, 2026 sessions** in kerf's Claude transcripts had laid out a "Tier 1 / Tier 2 / Tier 3" framework:

- **Tier 1 (executed May 14):** Point kerf at harmonik's real `bd` database (~1,267 beads), run `kerf next`, watch scoring. Found and fixed a bug — kerf's JSON parser expected a bare array but `br list --format json` returned `{"issues":[...]}`, silently zeroing all scoring.
- **Tier 2 (executed May 14):** Load harmonik's pilot YAMLs (`docs/decompose-to-tasks/*.yaml`) into a scratch `bd` database via a one-off Python loader. Wire them up as kerf works. Walk the chain to confirm each scoring signal fires. Loaded 234 beads across four pilots; rework weight dominated by +225 points when 15 rework beads were injected on the hc work.
- **Tier 3 (became Plan 007):** Full simulator with tick loop, baseline policies, metrics.

The May 14 sessions also produced a `SIM-PROPOSALS-2026-05-14.md` design doc from two background agents (workflow/UX angle + data/loop/analysis angle). **That doc was never committed.** Its substance is reconstructed in `sim_proposals_reconstruction.md`.

Key items from the reconstruction that Plan 007 did NOT ship:

- Fourth canned scenario `real-export.yaml` (real data).
- `kerfsim import <project>` subcommand.
- Claude-log-derived duration distributions.
- Merge/integration cost model.
- Per-task startup latency model.
- Stochastic-duration mode.

This plan (012) covers all of those. Plan 011 covers the validation quick wins that don't need new data.

## What the methodology investigation found

Two methodology agents investigated how to extract durations from Claude transcripts. Independent verdicts, same answer: **hybrid pipeline.**

**Programmatic extractor (~80% of the data we need):**
- Sub-agent JSONLs live at `~/.claude/projects/<proj>/<sessionId>/subagents/*.jsonl`. Each has its own `meta.json` with a description string — de facto bead label.
- `<usage><duration_ms>` is already in the transcripts as ground truth.
- Trace verified end-to-end on bead `kerf-665` (the `kerf triage` command):
  - Spin-up (dispatch → first sub-agent tool call): 3.9s
  - Reconnaissance (`bd show`, `grep`): 3.6 min
  - Write → commit: 3.3 min
  - Merge delta: 0 (direct-to-main)
  - Total: 7m28s — matches the JSONL `duration_ms` within a second.
- Engineering: 6–10 hours. Wall clock to run across both corpora: <10 min.

**LLM clean-up (~$50 one-time, Sonnet 4.6, cache-friendly):**
- Resolves the cases the programmatic pass cannot: spin-up vs task-work boundary inside reconnaissance, orchestrator-vs-worker time on interleaved sessions, bead-attribution when text doesn't carry bead IDs.
- Pure-LLM cost would be ~$400 — wasteful for cases regex already nails.

**Hard data gap:** merge-under-conflict. Both kerf and harmonik commit direct-to-main from worktrees. No PR step, no merge commit, no conflict resolution events in either corpus. **Decision: synthesize the conflict behavior in the simulator** (Plan 012 pillar C) rather than wait for real data.

**Bonus phase user wants captured:** reviewer round-trip. Programmatic agent flagged this as moderate-reliability (90%) because reviewers sometimes run in separate top-level sessions and require text-matching on bead ID to link. **Decision: capture where reliable, flag missing rows.**

## Definitional calls the user made

Required for extraction to be meaningful:

1. **Reconnaissance is spin-up,** not task work. (Default proposed; user accepted.)
2. **Reviewer round-trip is its own phase,** captured separately. (User wants this captured.)
3. **Abandoned `worktree-agent-*` branches** are filtered out of duration stats but counted in a "wasted-effort" counter. (Default proposed; user accepted.)

## What was decided this session

- **Plan 010 (concurrency)** moved to `plans/_backlog/010_concurrency/` with a CLAUDE.md rule that backlog plans are dormant. Don't surface unless user names them.
- **Plan 011** (this session's first draft, rewritten after prior-art search): simulator validation quick wins. Concurrency sweep, adversarial scenarios, saturated re-runs, weight tuning, priority_inversions fix.
- **Plan 012** (this doc's plan): real-workload corpus. Bead-shape importer, transcript-derived duration distributions, synthetic conflict model, real-corpus scenarios.
- **kerf binary reinstalled** at `/Users/gb/go/bin/kerf` (HEAD `92075d3`, includes Plan 009 triage + pin + drift summary).

## What was NOT decided (still open)

- Weight-tuning decision rule (placeholder: dominates ≥60% of scenarios, no scenario loses >5% on any metric). Plan 011 pillar D.
- Whether Plan 011 D should consume Plan 012's scenarios opportunistically if both finish in time.
- Sample size for duration extraction. Initial proposal: enough to fit a distribution per phase, not the full corpus. Probably ~50–100 beads with full phase data is plenty per pilot.

## Corrections from follow-up investigation (same day)

A targeted investigation agent verified two findings that affect downstream pipelines:

### `duration_ms` lives on the orchestrator side, not the sub-agent

The field `<usage><duration_ms>` is emitted by the orchestrator runtime inside `<task-notification>` queue-operation events when a sub-agent completes — **not** on sub-agent assistant messages. Verified on bead `kerf-665`: orchestrator reports `duration_ms=447870`; sub-agent JSONL wall clock = 447723 ms; delta ~150 ms (within event-write jitter). Pipeline implication: source of truth is the orchestrator JSONL (regex `<duration_ms>(\d+)</duration_ms>`); fallback to wall clock is fine within ~200 ms.

### The 123 "abandoned harmonik dispatches" was overstated

A 5-sample audit found:
- **4 of 5 were extractor false positives** — bead-ID indexing mismatch. Sub-agent committed in its worktree, the work reached `main`, but the `git log` grep didn't match because the commit message uses a different bead identifier (parent ID, sibling subtask, or spec codename like `EM-031a` instead of `hk-b3f.40`).
- **1 of 5 was a real truncation** — sub-agent stopped after 13s with `last_event.type == "user"` (parent cancel/inject), zero git activity.

The actual abandonment rate is likely much lower (back-of-envelope ~5–10×) than the raw CSV row count suggests.

**Fixes needed in the extractor (rolls into Plan 013 D1):**
- Index `main` commits by **all** bead IDs found in the message body (regex `hk-[a-z0-9]+(\.\d+)?`), not just the dispatch's primary ID.
- Follow parent/child bead links so subtask commits roll up to their parents.
- Scan worktree branch refs (not just `main`) before flagging "no commit."
- Truncation detector: `last_event.type == "user"` AND `duration < 60s` AND `tool_uses < 5` → "early termination."

The data files on disk are still accurate (they record what was found); only the **interpretation** of `wasted_effort.csv` row count was off.

## How this connects to the user's goal

The user said: *"I want to figure out distributions on each part of the process so we can model it and so our ordering algo can be improved. So there's the spin up time, how long the tasks take, merge time, merge time changing when there are conflicts, etc."*

Plan 012 maps directly:

| User's phrase | Plan 012 pillar | Status |
|---|---|---|
| Spin-up time | B (spin-up phase) | Extractable; reconnaissance counts as spin-up. |
| Task time | B (task-work phase) | Extractable. |
| Merge time | B (merge phase) | Near-zero in this corpus; document and move on. |
| Merge time under conflict | C (synthetic model) | No real data; simulator injects. |
| Reviewer time | B (reviewer phase) | Extractable where reliable. |
| Improved ordering algo | Plan 011 D (weight tuning) | Consumes Plan 012's scenarios + distributions. |
