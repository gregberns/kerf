# Scope + Design Critique — Plan 010

## (a)/(b)/(c): recommendation holds

Option (a) is correct for v1. The reasoning in the plan is sound: ownership data already exists in `spec.yaml` (`active_session`, `started`), so (a) is a pure projection — no new state, no write path, no cleanup paths. (b) requires a write-side lifecycle (claim, release, expiry, crash recovery) that bakes in a single concurrency model; rejecting it now keeps the door open to layer it on top of (a) later. (c) is strictly weaker than (a) — same render with no JSON contract.

One quiet caveat: (a) only works if the orchestrator can identify "mine." See `--session` below.

## Exit code 0 vs 2 — recommend 0

Plan currently proposes 0 when at least one bead/cleanup is *not* owned by another, and explicitly asks whether a mixed "one mine, others contested" case is 0 or 2. **Recommendation: 0.** The exit code answers "can I act?" — if the caller has *any* actionable, non-conflicted item (including its own work), the answer is yes. Code 2 should mean "every actionable item is held by someone else." Mixed → 0, owner reads `owned_by` per item to route. This matches the principle that kerf surfaces signals; the orchestrator decides. Reserving 2 for the strict all-contested case keeps it a useful trigger for "back off / wait / escalate" loops without forcing every mixed case down that branch.

Sub-question: anonymous owners (`"anonymous"`). Treating them as "owned by another" is correct for v1 — an anonymous owner is by definition not identifiable as the caller. Flag the dogfooding question as the plan already does.

## Topology agnosticism — clean

The plan does not assume agent count, dispatch model, or coordination shape. Three signals (who, since, stale) projected as item fields; the consumer decides. The deferred-(b) discussion explicitly avoids prescribing claim/release semantics. No violation.

One soft spot: the prose around exit code 2 ("orchestrator should wait, escalate, or pick a non-`kerf next` action") edges toward prescribing behavior. Trim to "no actionable item is unowned." Let the orchestrator decide what that means.

## Staleness — composes correctly

The plan reuses `sessions.stale_threshold_hours` (24h default) from `specs/sessions.md` verbatim — no new threshold, no new config key. The only change is *where* the check fires (now also during `kerf next` feed assembly) and *how* it surfaces (a `stale_session` warning item). This is the right move. Cross-reference in `specs/sessions.md` should be a one-liner; the plan says exactly that.

## Scope: 2–3 days is honest, leaning optimistic

What's actually in scope: 4 spec edits, 2 JSON fields, 1 warning detector, 1 flag, an exit-code table, help-text update, snapshot test. No new package, no new files, no new state. 2–3 days is plausible if the existing feed-assembly code already reads each work's `spec.yaml` (the plan claims it does — verify before estimating). Add half a day for the exit-code table being load-bearing for orchestrator loops: test matrix is 4 codes × {with/without --session} × {anonymous/uuid owner} = real coverage work.

Estimate: **3 days realistic, 2 if the spec.yaml read is already on the hot path.**

## `--session <id>` — flag is right, provenance is the real question

Flag name is fine; `--session` is the obvious choice and matches `active_session` terminology in `sessions.md`. The deeper question is provenance: **caller-supplied is correct, and not as gross as it looks.** kerf already documents (sessions.md §"Session ID Recording") that the ID is a UUID provided by the agent runtime (`claude --session-id <uuid>`). The orchestrator launching the agent *knows* the ID it passed in; threading it into `kerf next --session $SESSION_ID` is symmetric with how it lands in `active_session` in the first place. kerf-generating an ID would require a side channel for the agent to discover its own ID — strictly worse.

Worth adding to the spec: a one-liner that `--session` is expected to match the same UUID the runtime used to start the session. Otherwise an agent might pass an arbitrary string and silently fail to match.
