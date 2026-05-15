# Agent-UX Critique — Plan 010 (Concurrency Signals)

Lens: two autonomous agents, fresh contexts, polling `kerf next` in a loop.

## Does option (a) actually prevent collisions?

**No — and the plan never claims it does, but the framing buries that.** Option
(a) is a *signal*, not a *gate*. The only thing preventing two agents from both
picking bead #1 is whichever one calls `kerf resume` first; `kerf next` itself
is pure read. The 50ms-apart scenario:

- A runs `kerf next` → sees item #1 with `owned_by: null`. Picks it.
- B runs `kerf next` 50ms later, before A has called `resume` → sees the same
  item, same null ownership. Picks it.
- A runs `resume`, succeeds. B runs `resume`, gets `active session exists`.

The collision window is "time between `kerf next` and `kerf resume`," which is
exactly the gap an unsupervised orchestrator widens (LLM latency, tool
roundtrips). Plan 010 narrows the *post*-resume collision window to zero but
does nothing for the pre-resume one. SUMMARY.md says "See, before picking up a
work, that someone else is already on it" — true only if the other agent has
already committed via `resume`. Two agents in parallel `next` see clean slates.

## Caller session identity

`--session <id>` requires the agent to *know its own id*. Nothing in the plan
says how. `specs/sessions.md` records session ids best-effort and may use
`"anonymous"`. The agent has no `kerf whoami`. Two failure modes:

1. Agent omits `--session`, sees its own in-flight work as "owned by another,"
   exits 2, orchestrator thinks the queue is contended and backs off → stuck.
2. Agent fabricates an id ("agent-A"), `resume` records a UUID instead, the
   field never matches, exit code is permanently 2. Silent stall.

**Fix needed in plan:** define how an agent obtains its session id (stdout of
`resume`? env var? `kerf whoami`?). Without this, `--session` is unusable and
exit code 2 becomes a footgun.

## Staleness, A-crashed-mid-flight

Plan reuses the 24h threshold. For an agent that crashes at minute 3, B waits
24h before the warning fires. In practice B sees `owned_by: <A>`,
`owned_since: 2m ago`, exit code 2, and has no way to distinguish "A is
working" from "A is a corpse." No liveness signal (no heartbeat, no pid). The
orchestrator either waits 24h or guesses. **Top agent-stuck risk.**

## Discoverability

Plan says "`kerf next --help` gains a line." Two fields, four exit codes, one
new flag, one new warning kind — one line is not enough. An agent reading help
won't learn that exit 2 means contention rather than failure.

## A-holds, B-holds convention

Unaddressed. Plan is silent on priority/stealing/FIFO. Default is whoever called
`resume` first wins; the loser sees exit 2 forever (until 24h staleness).

## (a) vs (b) from the agent's seat

(b) `--claim` is *less* likely to cause silent collisions because the claim
happens at pick time, not at `resume` time, closing the pre-resume window. (a)
is cheaper and more honest about being advisory, but only wins if orchestrators
gate on `owned_by` *and* serialize their `next`→`resume` calls. The plan should
say this out loud.
