# Plan 010 — Agent Concurrency Signals + Opt-In Claim (Summary)

> **Status: BACKLOG.** Planning is complete; implementation deferred. The collision class this prevents is theoretical for today's usage pattern (one orchestrator dispatching subagents into separate worktrees). Revisit when multiple independent agents start operating on the same project at once and a real collision is reported.

## Why this matters today

When two AI agents run `kerf next` 50ms apart, both see the same top item with `owned_by: null` and both pick it. Read-side advisory fields alone don't fix this — there's a window between `next` and `resume` where neither agent has committed yet. Plan 010 closes the window with an opt-in claim primitive while keeping the advisory signals for cooperative diagnostics.

## What changes

1. **Two new fields per item.** `bead` and `cleanup` items carry `owned_by` (which session is on this work) and `owned_since` (when they started). Existing tools ignore the fields and keep working.

2. **A `stale_session` warning.** Works with active sessions older than 24h surface a warning suggesting `kerf shelve --force`. No auto-cleanup.

3. **Exit codes that mean something.** 0 = caller has actionable work, 1 = empty feed, 2 = all-contested, 3 = warnings-only. This is a contract change (today is always 0); a `--legacy-exit` flag opts back into the old behavior during transition.

4. **`kerf whoami`.** Prints the agent's runtime-provided session id. Agents pipe it into `--session <id>` so their own in-flight work doesn't read as "owned by another."

5. **`kerf claim <codename>` (opt-in).** Atomically takes a work via an `O_CREAT|O_EXCL` marker. Success → proceed to `kerf resume`. Contested → exit 2, loop. Claims expire after a `claim_ttl` (default 30 minutes — much shorter than the 24h staleness rule). `kerf shelve` and `kerf release` clear the marker.

## The intended two-step flow

```
kerf next --format=json     # see what's available + who owns it
kerf claim <codename>       # atomically take it (exit 2 if lost the race)
kerf resume <codename>      # proceed as normal
```

`kerf whoami` provides the session id for the `--session` flag — no guessing.

## Honest framing

The advisory fields alone do **not** prevent collisions; they're a signal. The claim primitive is what does. Agents that don't care about collisions never call `kerf claim`. Agents that do, opt in. kerf still doesn't assume agent count or coordination model.

## What we deliberately won't do

- No FIFO release queues, no OS-level locks, no auto-cleanup, no cross-work session registry, no heartbeat/liveness (deferred — A-crashed-at-minute-3 falls back to `claim_ttl`, not a finer signal).

## Rough cost

**4–5 days.** 3 new commands (`claim`, `release`, `whoami`), 2 JSON fields, 1 warning, 1 exit-code table, 1 new config key, 1 on-disk marker, race-test coverage.
