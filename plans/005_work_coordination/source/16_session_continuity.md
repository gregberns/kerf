# Session Continuity — Analysis and Design

> Deep analysis of why HANDOFF documents degrade and what should replace them.
> Written in response to Greg's feedback (10_USER_RESPONSE.md) and the five problems in _plan.md.

---

## 1. Why HANDOFF Documents Degrade

HANDOFF.md has five distinct failure modes. Each one is independently sufficient to corrupt session continuity over time; together they guarantee it.

### 1.1 Accumulation without pruning

Each session adds to the document but almost never removes anything. Information that was critical in session 2 ("the YAML parser has a bug with multiline strings") is still present in session 8 when the parser has been rewritten. The agent treats stale information as current because there's no expiry signal.

The incentive structure is wrong: removing a line risks losing something important; adding a line is always safe. So documents only grow.

### 1.2 Mixed concerns in a single document

A typical HANDOFF contains at least four distinct types of information:
- **Operating instructions** — "Work independently. Don't ask for permission."
- **Status** — "Beads cp-003 through cp-007 are complete."
- **Context** — "We chose the repository pattern because the adapter needs to be mockable."
- **Warnings** — "Don't use the `sync` package for this; it causes a deadlock."

These have completely different lifecycles. Operating instructions are static across sessions. Status changes every session. Context is relevant only to specific work. Warnings are relevant only until the underlying issue is resolved.

Mixing them in one document means the agent can't distinguish "what's changed since last time" from "what's always true" from "what used to matter but doesn't anymore."

### 1.3 The telephone game (recursive summarization)

Session N's HANDOFF summarizes session N-1's HANDOFF, which summarized session N-2's. Each pass through the summarization filter:
- Loses nuance ("be careful with the adapter" was originally "the adapter's Close() method must be called before Reconnect() or you get a file descriptor leak")
- Amplifies emphasis on whatever the summarizing agent thought was important
- Introduces interpretation errors (a note about a "possible issue" becomes a "known blocker")

After 5 rounds of summarization, the HANDOFF bears only a loose relationship to reality. The information hasn't been lost gradually — it's been *transformed* into something confidently wrong.

### 1.4 No structural distinction between current and historical

Narrative prose doesn't have a built-in mechanism for "this is true right now" vs. "this was true three sessions ago." An agent reading the HANDOFF can't tell which statements reflect current state and which are fossils from earlier sessions.

Some HANDOFF authors use section headers like "CURRENT STATE" and "HISTORY," but the boundary between them is itself subject to the telephone game — each session's "current state" becomes the next session's unsorted history.

### 1.5 No validation against actual state

Nothing checks whether the HANDOFF is accurate. If it says "bead cp-005 is in progress" but cp-005 was actually completed two sessions ago, no mechanism catches this. The HANDOFF is a claim about reality with no connection to reality.

This is the deepest problem. The other four could be mitigated with discipline. This one is structural: a prose document that describes system state will always drift from the system state it describes.

---

## 2. What Actually Needs to Persist Across Sessions

Not all information is equal. The key insight: **information that can be recomputed from authoritative sources should never be stored in a handoff document.** Storing it creates a stale cache that competes with the source of truth.

### 2.1 DO NOT STORE — compute fresh each session

| Information | Authoritative source |
|---|---|
| Bead/task status | Beads database (`br status`) |
| Work item status | spec.yaml `status` field |
| Dependency graph | spec.yaml `depends_on` fields |
| What's actionable | Computed from dependency graph + status |
| File structure / codebase state | `git status`, `git log`, filesystem |
| What was done last session | `git log --since`, bead completion records |
| Which specs exist and their content | The spec files themselves |

All of this is currently crammed into HANDOFF documents. All of it should be computed by `kerf map` / `kerf orient` / equivalent tooling at session start. Storing it in a handoff is creating a cache that will go stale.

### 2.2 MUST STORE — not derivable from any source

| Information | Why it's not derivable | Example |
|---|---|---|
| Design decisions and rationale | "Why" is never in the code or specs | "We chose append-only over overwrite because of the telephone game problem" |
| Discoveries about the environment | Learned through trial and error | "The CI runner has 2GB RAM limit; large test suites must be split" |
| Warnings / negative knowledge | Only exists in someone's experience | "Don't use `sqlx` query caching with this schema; it causes stale prepared statements" |
| Unfinished reasoning | Partial conclusions from exploration | "Investigated three approaches to X, ruled out A and B for reasons Y and Z, was exploring C when session ended" |
| Surprising behaviors | Things that violated expectations | "The `Flush()` call is synchronous despite the docs saying async" |

This is Greg's "small details that a next agent might need to know about." These details have no home other than session records. They're the irreducible core of what must persist.

### 2.3 SHOULD NOT STORE IN HANDOFF — belongs elsewhere

| Information | Where it belongs |
|---|---|
| Operating instructions | Project config, jig guidance, CLAUDE.md |
| Area/domain principles | Area specs (if/when built) |
| Process protocols | Documented in specs or commands |

---

## 3. The Right Abstraction: Immutable Session Records + Computed Orientation

The HANDOFF document tries to be a single mutable artifact that represents "everything the next session needs." This is the wrong abstraction. The replacement is two separate mechanisms:

### 3.1 Computed orientation (replaces the "status" and "what to do next" parts of HANDOFF)

At session start, the system computes a fresh view from authoritative sources:

```
$ kerf orient

# Orientation — acme-webapp
# Generated: 2026-05-08T14:32:00Z

## Portfolio State (from spec.yaml files)
  implementing:  brave-falcon "Adapter retry logic" (3/7 beads complete)
  research:      green-oak "Adapter observability"
  ...

## Recent Activity (from git log, last 48h)
  - 14 commits across 3 works
  - brave-falcon: beads cp-005, cp-006 completed
  - green-oak: spec draft created
  
## Dependency State (from spec.yaml depends_on)
  bold-crane blocked by brave-falcon
  
## Area Clusters (from spec.yaml areas)
  adapter: 3 works — review together before design decisions

## Actionable Now
  1. brave-falcon — continue implementation (4 beads remaining)
  2. green-oak — ready for next jig pass
```

This is never stored. It's computed every time. It can never go stale because it reads from the sources of truth.

### 3.2 Session log (replaces the "context, decisions, and discoveries" parts of HANDOFF)

An append-only sequence of immutable session records. Each session writes exactly one record at the end. Records are never edited after creation.

```yaml
# .kerf/sessions/2026-05-08T14-32.yaml
session_id: "2026-05-08T14-32"
works_touched:
  - brave-falcon
  - green-oak
duration_estimate: "~2 hours"

decisions:
  - context: "brave-falcon adapter retry logic"
    decision: "Use exponential backoff with jitter, not fixed intervals"
    rationale: "Fixed intervals cause thundering herd when multiple adapters retry simultaneously"
    
  - context: "green-oak observability approach"  
    decision: "Instrument at the adapter boundary, not inside individual methods"
    rationale: "Method-level instrumentation is too noisy; boundary instrumentation captures what callers care about"

discoveries:
  - context: "adapter connection pool"
    finding: "Pool.Get() returns a connection that may have been idle for up to 30 seconds"
    implication: "Must validate connection health before use; stale connections cause silent failures"

warnings:
  - context: "testing adapter retry"
    warning: "Don't use httptest.Server with TLS for retry tests"
    reason: "The TLS handshake timeout interacts with the retry timer in confusing ways. Use plain HTTP."

unfinished:
  - context: "brave-falcon bead cp-007"
    state: "Started investigating connection pool interaction with retry logic"
    thinking: "The pool may need to be retry-aware — if a connection fails and triggers retry, should it come from the same pool slot or a new one? Leaning toward new slot to avoid poison-connection loops."
```

Key properties of this design:

**Immutable.** Once written, a session record is never modified. No telephone game. Session 3's record says exactly what session 3 said, forever.

**Append-only.** Each session adds a new file. No overwriting, no summarizing previous sessions.

**Structured.** Decisions, discoveries, warnings, and unfinished work are in labeled fields, not buried in narrative prose. An agent can scan just the `warnings` across all sessions without reading everything.

**Bounded.** Each session writes its own record. There's no temptation to accumulate everything from all previous sessions into a single growing document.

**Typed.** Each entry has a `context` field that ties it to a specific work or area. This enables relevance filtering later.

---

## 4. Solving the Telephone Game

The telephone game happens when session N summarizes session N-1. The fix is simple: **don't summarize. Don't touch previous records. Write your own.**

At session start, the agent reads the session log — not a summary of it, the actual records. Each record is short (the structured format enforces brevity). The agent reads them directly.

The question is what happens at scale.

---

## 5. Scale: Handling Growing History

After 50 sessions, there are 50 session records. Even structured, this is too much for context. Three mechanisms handle this, applied in combination:

### 5.1 Relevance filtering (primary mechanism)

Most session records are irrelevant to current work. If you're working on `brave-falcon`, you don't need discoveries from sessions that only touched `red-wave`.

The `works_touched` field enables filtering:

```
$ kerf orient --work brave-falcon
# Shows only session records where works_touched includes brave-falcon
# Plus any records tagged with the same areas as brave-falcon
```

This is the first and most important scaling mechanism. It doesn't lose information — it just doesn't show information about unrelated work.

### 5.2 Tiered detail (secondary mechanism)

For the relevant records that remain after filtering:
- **Last 3 sessions:** Show in full
- **Sessions 4-10:** Show decisions and warnings only (skip discoveries and unfinished)
- **Sessions 11+:** Show warnings only

The rationale: decisions older than ~10 sessions have likely been incorporated into specs or code. Warnings are the longest-lived — "don't try X" remains relevant until the underlying issue is fixed.

### 5.3 Resolution marking (tertiary mechanism)

Some session record entries become obsolete when the underlying issue is resolved. Rather than editing the original record (which would break immutability), a later session can mark entries as resolved:

```yaml
# In a later session record
resolutions:
  - session: "2026-05-08T14-32"
    type: warning
    context: "testing adapter retry"
    resolution: "Fixed in brave-falcon bead cp-009 — TLS test helper now handles timeouts correctly"
```

The orientation system filters out resolved entries. The original record is untouched.

### 5.4 What about summarization?

Summarization is the obvious answer and the wrong one. It's exactly how the telephone game starts. The moment you summarize 10 session records into a paragraph, you've lost the nuance, introduced interpretation, and created a document that will itself be summarized later.

The three mechanisms above — relevance filtering, tiered detail, and resolution marking — achieve what summarization tries to achieve (reducing volume) without the information degradation that summarization causes.

If a project genuinely has 100+ sessions and the filtered/tiered view is still too large, the right response is probably: the decisions and discoveries from sessions 1-50 should have been incorporated into specs by now. If they haven't, that's a process problem, not a tooling problem. An explicit "incorporate into spec" step could be added to the session-end protocol.

---

## 6. Session Bookends — Start and End Protocols

### 6.1 Session start

```
1. Run `kerf orient` — get computed state (portfolio, dependencies, areas, actionable)
2. Read filtered session log — decisions, discoveries, warnings relevant to current work
3. Read relevant specs — the specs for the work(s) you're about to touch
```

This replaces reading HANDOFF.md. The first two steps could be combined into a single command (`kerf orient` that includes the session log output).

The total context loaded is:
- Computed orientation: ~50-100 lines (scales with portfolio size, not session count)
- Filtered session log: ~20-50 lines per relevant session, 3-5 recent sessions shown in full
- Relevant specs: however long the specs are (unchanged from today)

### 6.2 Session end

```
1. Run `kerf close` (or equivalent)
2. Agent writes structured session record:
   - works_touched (auto-populated from git/bead activity)
   - decisions (agent writes — "what did I decide and why?")
   - discoveries (agent writes — "what did I learn that isn't in specs?")
   - warnings (agent writes — "what should future sessions avoid?")
   - unfinished (agent writes — "what was I in the middle of?")
3. Record is written to .kerf/sessions/{timestamp}.yaml
4. No HANDOFF.md is generated
```

### 6.3 Enforcement without overhead

The risk is that agents skip the session-end protocol. Two mitigations:

**Make it cheap.** The structured format is faster to write than a narrative HANDOFF. The agent doesn't have to summarize everything — just decisions, discoveries, warnings, and unfinished items. If a session had no notable decisions or discoveries, the record is nearly empty, and that's fine.

**Auto-populate what you can.** `works_touched` can be derived from git commits and bead activity during the session. The agent only needs to write the parts that require judgment.

**Make the consequence visible.** If a session doesn't write a record, `kerf orient` for the next session will show a gap: "Session 2026-05-08: no record found." This makes the omission visible without blocking anything.

---

## 7. How Much of the HANDOFF Becomes Unnecessary?

Given `kerf orient` (computed) + session log (persistent), here's what happens to each piece of a typical HANDOFF:

| HANDOFF content | Replacement | Stored? |
|---|---|---|
| "Work independently, don't ask permission" | CLAUDE.md / project config | No (already exists elsewhere) |
| "Beads cp-003 through cp-007 are complete" | `kerf orient` computes from bead state | No (computed) |
| "Next: implement cp-008" | `kerf orient` / `kerf next` | No (computed) |
| "We chose repository pattern because..." | Session record: `decisions` | Yes (immutable) |
| "Don't use sync package, causes deadlock" | Session record: `warnings` | Yes (immutable) |
| "The CI has a 2GB RAM limit" | Session record: `discoveries` | Yes (immutable) |
| "Was in the middle of investigating X" | Session record: `unfinished` | Yes (immutable) |
| "brave-falcon depends on green-oak" | `kerf orient` computes from spec.yaml | No (computed) |
| "3 works touch the adapter area" | `kerf orient` computes from area tags | No (computed) |

Roughly 60-70% of a typical HANDOFF is computable state that should never have been stored. The remaining 30-40% is decisions, discoveries, and warnings — the irreducible core that the session log captures.

---

## 8. The Abstraction, Summarized

The right abstraction is not a document. It's a **computed view + append-only structured log.**

- **Computed view** (`kerf orient`): Assembles current state from authoritative sources. Never stored. Never stale. Replaces the "what's the status" and "what's next" parts of HANDOFF.

- **Session log** (`.kerf/sessions/*.yaml`): Immutable, append-only, structured records of decisions, discoveries, warnings, and unfinished work. Each session writes exactly one. Never edited. Never summarized. Filtered by relevance and recency at read time. Replaces the "context for the next agent" part of HANDOFF.

Together, they provide everything a HANDOFF tried to provide — without the degradation.

---

## 9. Relationship to Plan 005

This analysis is complementary to the Plan 005 proposals. Specifically:

- **`kerf map`** (Tier 1, option B) is the portfolio-level component of `kerf orient`
- **Enhanced `kerf resume`** (Tier 1, option C) is the work-level component of `kerf orient`
- **Session log** is a new mechanism not covered in the options menu (09_options_menu.md)

The session log fills a gap that `kerf map` and `kerf orient` cannot: capturing the non-computable knowledge that agents accumulate during work. `kerf map` tells you *what* the state of the portfolio is. The session log tells you *why* things are the way they are, what to watch out for, and what's in-flight at a conceptual level.

### Open questions for the plan

1. **Where do session records live?** `.kerf/sessions/` is one option. Inside the work directory is another (ties records to specific works but fragments the log). Recommendation: project-level `.kerf/sessions/` with `works_touched` field for filtering.

2. **Who writes the record — the agent or kerf?** The auto-populatable fields (works_touched, timestamp) should be computed by kerf. The judgment fields (decisions, discoveries, warnings, unfinished) must be written by the agent. A `kerf close` command could scaffold the record and prompt the agent to fill in the judgment fields.

3. **Integration with harmonik.** If harmonik manages agent sessions, the session bookend protocol (orient at start, close at end) should be part of harmonik's session lifecycle, not something the agent has to remember to do.

4. **Migration from HANDOFF.** Projects currently using HANDOFF.md need a transition path. The simplest: run `kerf orient` + read HANDOFF.md for the first session under the new system. Write a session record at the end. From session 2 onward, HANDOFF.md is no longer read.
