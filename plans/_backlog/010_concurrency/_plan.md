# Plan 010 — Agent Concurrency Signals + Opt-In Claim

> **Status: BACKLOG.** Design and critique passes are complete; implementation is deferred. The collision class this prevents (two parallel agents picking the same top item simultaneously) is theoretical for today's single-orchestrator-with-subagents pattern. Pull off the backlog when multiple independent agents start operating on the same project concurrently and a real "both grabbed item 1" incident is reported.

## Intent

Two parallel agents running `kerf next` today both pick item #1 with no warning. There is no session-ownership signal in the item feed, no exit-code distinction for "all top items are owned", no JSON-contract field that surfaces contention, and — critically — no atomic primitive for an agent to *take* an item before the slower `kerf resume` step.

This plan adds **concurrency signals** to `kerf next` AND a small **opt-in claim primitive** (`kerf claim`). Signals alone are advisory and do not prevent the pre-`resume` collision window; the claim primitive is what closes it. kerf still does not enforce locks at the `next` layer, does not prescribe an agent count, and does not assume how agents are spawned. Orchestrators that want collision-prevention opt in to `kerf claim`.

## Why

The agent-UX critique on plan 008 identified `kerf next` ownership signals as the biggest gap missing from the action list. The agent-UX critique on this plan (010) then identified the 50ms-window flaw: two agents both running `kerf next` before either has called `kerf resume` see identical `owned_by: null` feeds and both pick #1. Read-side advisory is a *signal*, not a *gate*.

Today's state, per `specs/sessions.md`:

- A work tracks `active_session` and a `sessions` list per `spec.yaml`.
- `kerf resume` refuses if `active_session` is non-null, but `kerf next` does not consult it.
- Stale-session detection (24h default) exists but only surfaces on commands that touch the work's `spec.yaml`.

The bead/cleanup/warning items emitted by `kerf next` carry no ownership information, and an agent has no way to atomically *take* an item between picking it and calling `kerf resume` (which is where the existing single-session-per-work rule finally bites).

## Design Pivot

The original plan recommended **option (a)** only — read-side advisory fields. The agent-UX critique correctly forced a pivot: option (a) alone leaves a collision window between `kerf next` and `kerf resume` that unsupervised orchestrators widen (LLM latency, tool roundtrips). Revised v1 = **option (a) + an opt-in claim primitive**:

- **Keep** option (a)'s advisory fields (`owned_by`, `owned_since`) — they remain useful for cooperative agents and for diagnostics.
- **Add** an opt-in `kerf claim <work-codename>` (also exposed as `kerf next --claim` for the one-shot path) that atomically writes a claim marker to the work's session state. Only the act of claiming creates the gate.
- **Add** `kerf whoami` so an agent can discover its runtime-provided session id without guessing.

The intended agent flow:

1. `kerf next --format=json` — see what is available and who currently owns it.
2. `kerf claim <codename>` — atomically take it. On success, proceed to `kerf resume`. On failure (someone else got there first), exit 2 and loop back to step 1.

Atomic write is via `O_CREAT|O_EXCL` (or equivalent) on a claim marker file. No new infra, no lock daemon, no out-of-process coordination — just basic filesystem semantics.

### Why this is still topology-agnostic

Agents that don't care about collisions never call `kerf claim`. Agents that do, opt in. kerf does not prescribe claim-then-release as the One True flow; it provides the primitive and the surfaces it requires (whoami, ttl), and orchestrators compose.

### Options (b) and (c) revisited

- (b) full claim+release lifecycle: **partially adopted.** We take the atomic-claim part; we do *not* bake in FIFO release semantics. Claims expire by TTL or by `kerf shelve` clearing `active_session`.
- (c) pure visualization: **still rejected** — strictly weaker than (a).

## What Changes

### 1. JSON contract — two new optional fields on Item

`bead` and `cleanup` items gain:

| field | type | meaning |
|-------|------|---------|
| `owned_by` | string or null | The `active_session` value of the item's target work (a UUID, the literal `"anonymous"`, or null). |
| `owned_since` | RFC 3339 string or null | The `started` timestamp of that session entry. |

For `warning` items, both fields are emitted as `null` (schema uniformity, not omission). Additive to the shape in `specs/commands.md` §`kerf next`; existing consumers ignore unknown fields. The spec amendment adds an explicit "Consumers MUST ignore unknown fields" clause to discharge the additive claim.

### 2. New warning kind — `stale_session`

When a work has a stale `active_session` (per the 24h-default rule in `specs/sessions.md`), kerf emits a `warning` item:

```
warning: work `bridge` has stale active session (started 2026-04-06T10:00:00Z, threshold 24h). Run `kerf shelve --force bridge` to clear.
```

Stale-session staleness uses the existing 24h BENCH threshold. **Claim staleness uses a separate, shorter `claim_ttl` — see §5 below.**

### 3. Exit codes for `kerf next` (contract change — see Transition)

| code | meaning |
|------|---------|
| 0 | At least one `bead` or `cleanup` item is actionable by the caller (not owned by another session, OR owned by the caller's own session per `--session`). Mixed cases (some mine, some contested) return 0. |
| 1 | No `bead` or `cleanup` items at all (empty feed; warnings may be present). |
| 2 | `bead`/`cleanup` items exist but **every one** is owned by another session (strict all-contested). Also returned by `kerf claim` when the claim is contested. |
| 3 | Warnings present and no actionable items (e.g., stale sessions block the only ready work). |

The default text output prints a one-line footer indicating which case fired.

**Transition:** this is **not purely additive** — today `kerf next` returns 0 on any output. Callers relying on `exit 0 == feed-produced-output` will see 1 or 3 where they previously saw 0. Two mitigations:

1. The new contract is documented prominently in `commands.md` and the CHANGELOG.
2. A `--legacy-exit` flag opts back into "always 0 on output" for the transition; removed in a later plan after dogfooding.

### 4. Caller session identity: `--session` and `kerf whoami`

- `kerf next --session <id>` and `kerf claim --session <id>` accept the caller's session id. Items owned by `<id>` are treated as "mine" for exit-code purposes.
- `kerf whoami` prints the runtime-provided session id (per the `sessions.md` UUID convention — the same UUID the agent runtime passed to `claude --session-id <uuid>`). Agents call `kerf whoami` to discover the id to pass to `--session`.
- Spec note in `commands.md`: `--session` **must** match the runtime UUID, otherwise it silently won't match `owned_by` and the agent will see permanent exit 2. `kerf whoami` is the canonical source.
- Anonymous owners (`"anonymous"`) are treated as "owned by another" — flagged for dogfooding review.

### 5. `kerf claim` — the opt-in gate

`kerf claim <codename>` (and `kerf next --claim` for the one-shot path):

- Writes an atomic claim marker via `O_CREAT|O_EXCL` at a path inside the work's session state (exact path defined in spec amendment; lives in the bench, not the repo, per plan 004 storage rules).
- On success: exits 0, prints the claim id and ttl. The agent then calls `kerf resume` normally.
- On failure (marker already exists and is fresh): exits 2 with the current owner. Caller loops.
- On failure (marker exists but is older than `claim_ttl`): the call **succeeds** — the stale claim is overwritten. `claim_ttl` defaults to **30 minutes** (configurable in `~/.kerf/config.yaml` under `sessions.claim_ttl_minutes`).
- `kerf shelve` clears the claim marker as part of its existing teardown.
- A new `kerf release <codename>` clears a claim without shelving (for orchestrators that grabbed a claim and decided not to proceed).

`claim_ttl` is deliberately shorter than the 24h `stale_threshold_hours`: claims protect a small picking window; stale sessions protect long-running open work. The two thresholds serve different needs.

### 6. Help-text six-bullet contract

`specs/commands.md` §`kerf next` Help text is a numbered six-element contract: returns / kinds / loop / filter flags / machine output / scoring. Mapping:

- `--session` slots into **bullet 4** (filter flags) — additive.
- New exit-code table slots into **bullet 3** (action loop) — exit codes drive loop branching, so this is the natural home and preserves the fixed-order six-bullet promise.
- No 7th bullet is added.

`kerf claim`, `kerf release`, and `kerf whoami` get their own help-text sections in `commands.md` (new command entries, not inside `kerf next`).

### 7. Storage-mode interaction (plan 004)

Ownership data already lives in each work's `spec.yaml` (bench or repo per `storage` setting). The new claim marker file lives **in the bench**, not the repo, regardless of storage mode — the repo is shared across machines and a per-machine claim file would create false ownership signals on teammates' clones.

## Specs Affected

| Spec file | Change |
|-----------|--------|
| `specs/commands.md` | `kerf next` JSON shape gains `owned_by` and `owned_since`. New exit-code table in bullet 3. New `--session` flag and `--legacy-exit` flag in bullet 4. New `stale_session` warning kind. New command entries: `kerf claim`, `kerf release`, `kerf whoami`. Explicit "unknown fields ignored" clause. |
| `specs/sessions.md` | Cross-reference: stale-session detection is now surfaced by `kerf next`. New `claim_ttl_minutes` config (default 30) and claim-marker lifecycle (write on claim, clear on shelve/release/ttl). `--session` provenance note pointing at the UUID convention and `kerf whoami`. |
| `specs/coordination.md` | `kerf next` reads each work's `active_session` during feed assembly; `kerf claim` is the opt-in gate that closes the pre-resume window. Ownership/claim do not affect scoring; metadata only. |
| `specs/cli.md` | Document the new exit-code contract for `kerf next` and `kerf claim`. |
| `specs/architecture.md` | No change. Claim writes go through existing storage layer; no new package. |

## Out of Scope

- A FIFO or priority-based release lifecycle. Claims expire by TTL or are cleared by `shelve`/`release`; no queue semantics.
- OS-level advisory locks (`flock`, etc.). The `O_CREAT|O_EXCL` approach is sufficient and portable.
- Automatic stale-session cleanup. Surfacing only.
- A cross-work `.kerf/sessions.json` registry.
- Per-bead ownership independent of the parent work.
- Changes to `kerf resume` / `kerf shelve` semantics beyond clearing the claim marker on shelve.
- Heartbeat / liveness signals for active sessions (flagged by agent-UX critique; deferred — would need a separate plan).

## Open Questions

1. **Exit code 2 vs. 0 in mixed cases:** resolved — exit 0 if any item is actionable by the caller. Exit 2 only for strict all-contested.
2. **Anonymous sessions:** v1 treats anonymous owners as "owned by another." Revisit after dogfooding.
3. **Heartbeats for liveness:** A-crashed-mid-flight scenarios (B sees `owned_since: 2m ago`, no liveness signal) wait for the 30-minute `claim_ttl` rather than the 24h stale threshold. Whether a finer-grained heartbeat is needed is a v2 question.
4. **`--legacy-exit` removal:** when can we drop it? Tied to dogfooding signal.

## Implementation Notes

- **Pivot acknowledgment:** the original "option (a) only" framing is replaced. Advisory fields ship together with the claim primitive in this plan; shipping (a) alone would leave the documented collision window open.
- New surface area: 3 new commands (`claim`, `release`, `whoami`), 1 new `--claim` flag on `next`, 1 new `--session` flag on `next`/`claim`, 1 new `--legacy-exit` flag, 2 new JSON fields, 1 new warning kind, 1 exit-code table, 1 new config key (`claim_ttl_minutes`), 1 new on-disk artifact (claim marker file).
- `internal/feed` reads each work's `spec.yaml` already — no extra I/O for ownership projection.
- Claim marker write path is `~/.kerf/projects/<id>/<codename>/.claim` (exact path finalized in spec amendment). Bench-only.
- Tests: ownership-field projection; exit-code matrix (4 codes × {with/without `--session`} × {anonymous/uuid owner}); `O_CREAT|O_EXCL` race test (two concurrent claims, exactly one wins); `claim_ttl` expiry; `kerf shelve` clears the marker; `kerf whoami` returns the runtime id; `--legacy-exit` opt-out.
- Help-text: six-bullet contract preserved; `--session` in bullet 4, exit codes in bullet 3.

## Sequencing

1. Spec updates (`commands.md`, `sessions.md`, `coordination.md`, `cli.md`) — one PR.
2. JSON shape + populate `owned_by`/`owned_since` in `internal/feed`.
3. `kerf whoami` (small, unblocks `--session` testing).
4. `kerf claim` / `kerf release` with `O_CREAT|O_EXCL` and `claim_ttl`.
5. `--session` flag + new exit-code rule + `--legacy-exit` in `cmd/next.go`.
6. `stale_session` warning detector.
7. Help text + snapshot tests + race test.

## Scope Estimate

The original plan estimated **2–3 days** for option (a) alone. With the claim primitive, `kerf whoami`, the TTL rules, the `--legacy-exit` transition flag, and the race-test coverage, revised estimate is **4–5 days**. Roughly: 1 day specs, 1 day claim/whoami/release, 1 day next-side exit codes + `--session` + legacy-exit, 0.5 day stale-session warning, 1–1.5 days tests (race coverage is real work).
