# Flow Dynamics — The System as a Graph with Cycles

> Thinking document. Analyzing the coordination system as a dynamical system with feedback loops, concurrent flows, and information propagation.

---

## 1. The Flow Graph

The previous factory-line analysis (doc 11) laid out seven stations in a linear chain. Greg's pushback is correct: the system is not a pipeline. It is a directed graph with cycles. Let me redraw it that way.

### Nodes (Activities)

These are not "stations a work item sits at." They are activities that the system performs. Multiple activities can be happening concurrently on different units of work.

```
  PLAN  ──  Think about what needs to change and why
  SPEC  ──  Define precisely what to build
  TASK  ──  Decompose the spec into atomic beads with dependencies
  EXEC  ──  An agent implements a bead
  TEST  ──  Verify that implementation matches intent (per-bead or holistic)
```

Five nodes. Not seven. Intake and Archive are boundary events (entry/exit), not sustained activities.

### The Graph

```
                          ┌─────────────────────────────────────────┐
                          │              WIDE LOOP                  │
                          │                                         │
                     ┌────┼───────────────────────┐                 │
                     │    │      MEDIUM LOOP       │                 │
                     │    │                        │                 │
                ┌────┼────┼──────────┐             │                 │
                │    │    │  TIGHT   │             │                 │
                │    │    │  LOOP    │             │                 │
                v    v    v          │             │                 │
 ──IN──>  PLAN ──> SPEC ──> TASK ──> EXEC ──> TEST ──> OUT──>
                                      ^  ^         │
                                      │  │         │
                                      │  └─────────┘
                                      │   REWORK LOOP
                                      │   (test fails,
                                      │    fix specific beads)
                                      │
                                      └── FAST TRACK
                                          (known bug enters
                                           directly as bead)
```

### Edge table (every transition in the graph)

```
Edge               Direction    Condition                           Speed
─────────────────  ──────────   ──────────────────────────────────  ──────
PLAN  → SPEC       forward      problem understood, approach chosen slow
SPEC  → TASK       forward      spec finalized                     medium
TASK  → EXEC       forward      bead unblocked, agent available    fast
EXEC  → TEST       forward      bead implemented                   fast
TEST  → OUT        forward      all tests pass, holistic check ok  medium

TEST  → EXEC       rework       test fails, fix is localized       fast
TEST  → TASK       medium loop  test reveals missing/wrong tasks    medium
TEST  → SPEC       medium loop  test reveals spec gap               medium
TEST  → PLAN       wide loop    test reveals architectural issue   slow

EXEC  → TASK       feedback     impl reveals task was mis-scoped   fast
EXEC  → SPEC       feedback     impl reveals spec error            medium
EXEC  → PLAN       feedback     impl reveals design flaw           slow

any   → PLAN       lateral      new work discovered at any point   varies

IN    → PLAN       intake       new idea, raw                      slow
IN    → SPEC       intake       well-formed requirement            medium
IN    → TASK       intake       known bug, clear fix               fast
IN    → EXEC       intake       trivial fix, bead created directly fast
```

The last four rows are the **intake paths** — work enters the system at different points depending on how well-formed it is. A vague idea enters at PLAN. A known bug with a clear fix enters at TASK or even EXEC directly.

---

## 2. Cycle Types

Not all feedback loops are equivalent. They differ in how far back the signal travels, how much rework they cause, and how fast they resolve.

### Tight Loop: EXEC <-> TEST

```
  EXEC ──> TEST
    ^        │
    └────────┘
     fix bead
```

**Trigger:** Test fails. The cause is a localized implementation error — wrong logic, missed edge case, typo.

**Scope:** One bead. No spec change. No new tasks.

**Resolution time:** Minutes. The same agent (or the next one) fixes the bead and re-tests.

**Distinguishing signal:** The failure maps clearly to a specific bead's acceptance criteria. The spec is correct; the code just doesn't match it yet.

### Rework Loop: TEST -> TASK -> EXEC -> TEST

```
  TASK ──> EXEC ──> TEST
    ^                 │
    └─────────────────┘
     new/modified beads
```

**Trigger:** Test reveals that tasks were incomplete or mis-scoped. Maybe a bead was too coarse and needs splitting. Maybe a prerequisite bead was missing.

**Scope:** New beads are created within the existing spec. The spec itself is fine.

**Resolution time:** An hour or less. Create beads, implement, re-test.

**Distinguishing signal:** The spec covers the behavior, but the task decomposition missed something. "The spec says X, but there's no bead for the glue code between A and B."

### Medium Loop: TEST -> SPEC -> TASK -> EXEC -> TEST

```
  SPEC ──> TASK ──> EXEC ──> TEST
    ^                          │
    └──────────────────────────┘
     spec amendment
```

**Trigger:** Testing reveals a gap in the spec itself. The behavior isn't just unimplemented — it was never specified. Or the spec assumed something false about the system.

**Scope:** Spec amendment. Possibly new beads, possibly modified beads, possibly invalidated beads.

**Resolution time:** Hours. The spec needs thought, review, then new tasks generated and executed.

**Distinguishing signal:** "The spec doesn't say what to do when X happens" or "The spec says Y, but Y is impossible because Z."

### Wide Loop: TEST -> PLAN -> SPEC -> TASK -> EXEC -> TEST

```
  PLAN ──> SPEC ──> TASK ──> EXEC ──> TEST
    ^                                   │
    └───────────────────────────────────┘
     re-plan
```

**Trigger:** Testing reveals an architectural problem. The approach is wrong, not just the details. The interaction between components doesn't work. The design needs rethinking.

**Scope:** Full re-plan. New spec sections or replaced spec sections. Many new beads.

**Resolution time:** A day or more. This is expensive.

**Distinguishing signal:** "This entire approach doesn't work" or "We need to restructure how A and B interact." The fix can't be localized to one spec section.

### How the system distinguishes them

The determining factor is: **how far upstream is the root cause?**

```
  Root cause is in...   →   Loop type
  ─────────────────────────────────────
  code (wrong impl)     →   tight
  tasks (wrong decomp)  →   rework
  spec (wrong/missing)  →   medium
  plan (wrong approach) →   wide
```

The agent (or human) at TEST makes this judgment. The signal is: "Can I fix this by changing code? Changing tasks? Changing the spec? Or does the whole approach need rethinking?"

---

## 3. Flow Rate and Bottlenecks

Each activity has a characteristic throughput rate:

```
Activity    Throughput         Bottleneck?   Who
────────    ──────────         ───────────   ────────────
PLAN        ~1/day             YES           human thinking
SPEC        ~2-3/day           sometimes     agent + human review
TASK        ~5-10/day          rarely        agent (mostly automated)
EXEC        ~20-50 beads/day   rarely        agents (parallelizable)
TEST        ~5-10/day          YES           agent + human review
```

### The bottleneck pattern

```
                PLAN          SPEC         TASK          EXEC           TEST
throughput:     ████          ████████     ████████████  ████████████   ████████
                slow          medium       fast          fast           medium


    Work accumulates HERE              Work accumulates HERE
    (waiting for planning)             (waiting for testing)
         v                                    v
 ──> [PLAN ····] ──> SPEC ──> TASK ──> EXEC ──> [TEST ····] ──>
```

PLAN is the narrowest pipe. It requires human cognition. You can't parallelize thinking about architecture.

TEST is the second bottleneck — especially for holistic testing. Individual bead tests are fast, but "does this all work together?" requires running the system, evaluating behavior, thinking about edge cases.

EXEC is the widest pipe. You can throw N agents at it and get N-fold throughput (modulo coordination costs).

This creates a characteristic **sawtooth pattern:**

```
Work in system
  ^
  │    ╱╲        ╱╲        ╱╲
  │   ╱  ╲      ╱  ╲      ╱  ╲
  │  ╱    ╲    ╱    ╲    ╱    ╲
  │ ╱      ╲  ╱      ╲  ╱      ╲
  │╱        ╲╱        ╲╱        ╲
  └─────────────────────────────── time
   plan  exec  plan  exec  plan
   builds  drains  builds  drains
   up    quickly  up    quickly
```

Planning builds up a batch of work. Execution drains it quickly. Then execution starves while planning builds the next batch. Testing creates a secondary accumulation point before work exits the system.

### Feedback processing — the unknown rate

Greg flagged this. When TEST produces feedback (failures, gaps, bugs), how fast does that feedback get processed?

Today: **it depends on session boundaries.** If the testing agent finds an issue, it gets written down somewhere (HANDOFF, a note, a message). The next agent session picks it up... maybe. Maybe it gets lost. Maybe it gets distorted (telephone game).

The feedback processing rate is currently **uncontrolled and variable.** This is the biggest dynamic problem in the system. Feedback that sits unprocessed is waste — it lets more work proceed on a flawed foundation.

---

## 4. Concurrent Flows

The system doesn't process one item at a time. Multiple flows move through it simultaneously.

### Flow types with different velocities

```
                    PLAN    SPEC    TASK    EXEC    TEST
                    ─────   ─────   ─────   ─────   ─────
Feature (new):      days    hours   hours   hours   hours     ← slow, wide
Enhancement:        hours   hours   hour    hours   hours     ← medium
Bug fix:            mins    mins    mins    hour    mins      ← fast, narrow
Test feedback:      --      mins    mins    hour    mins      ← fastest
```

These flows coexist:

```
Time ──────────────────────────────────────────────────────────>

Feature A:  [PLAN ··········][SPEC ····][TASK][EXEC ········][TEST ···]
Feature B:       [PLAN ·······][SPEC ··][TASK][EXEC ····][TEST ·]
Bug X:                              [TASK][EXEC][TEST]
Bug Y:                                     [TASK][EXEC][TEST]
Feedback Z:                                       [TASK][EXEC][TEST]
```

### Where concurrent flows interact

**Competition for EXEC capacity.** All flows need agents to implement beads. Bug fixes and feature beads compete for the same pool. Without prioritization, features starve bug fixes (there are more feature beads) or bug fixes starve features (if bugs are always higher priority).

**Competition for TEST capacity.** Holistic testing of Feature A blocks while Bug X's test is being run on the same system. Or worse: Bug X's fix changes code that Feature A's test depends on.

**Competition for human attention.** PLAN requires human thinking. If the human is reviewing Feature A's spec, Bug X's fix sits waiting for review. Human attention is the scarcest resource.

**File-level conflicts.** Feature A and Bug X both modify the same file. Their EXEC flows can't safely run in parallel without coordination (file reservation).

### Coordination points between concurrent flows

```
              PLAN     SPEC     TASK      EXEC       TEST
              ─────    ─────    ─────     ─────      ─────
Cross-work    area     spec     dep       file       integration
signal:       overlap  review   graph     reserv.    conflicts
              check    for      merge     + area
                       coher.             awareness
```

At each activity, concurrent flows need different types of coordination:
- PLAN: "Does this overlap with something already planned?"
- SPEC: "Is this consistent with related specs?"
- TASK: "Do these beads depend on beads from another flow?"
- EXEC: "Is anyone else modifying these files?"
- TEST: "Does this still work with the other recent changes?"

---

## 5. Information Flows

Work flows through the system. But information flows DIFFERENTLY from work — it propagates laterally, backwards, and asynchronously.

### The information channels

```
Channel                 Carries                          Direction
──────────────────────  ───────────────────────────────   ─────────────
Area awareness          "this area is being modified"     broadcast
Dependency signal       "X blocks Y"                     backward
Completion signal       "X is done"                      forward + lateral
Feedback signal         "testing revealed X"             backward
Priority signal         "focus on Y next"                broadcast
Conflict signal         "A and B touch same files"       lateral (peer)
Status signal           "here's what's in flight"        broadcast
```

### Information flow vs. work flow

```
WORK flows forward (with feedback loops):

    PLAN ───> SPEC ───> TASK ───> EXEC ───> TEST ───>


INFORMATION flows in ALL directions simultaneously:

    PLAN <──> SPEC <──> TASK <──> EXEC <──> TEST
      │         │         │         │         │
      └─────────┴─────────┴─────────┴─────────┘
                    broadcast layer
              (status, areas, priorities)
```

Key difference: work has **position** (it's at a specific activity). Information has **scope** (it's relevant to specific activities but doesn't "live" at any one of them).

### Critical information flows

**1. Feedback signal (TEST -> upstream).**
The most important information flow. When testing reveals a problem, this signal must propagate back to the right activity level (code? task? spec? plan?) FAST. Today this signal is lossy — it goes through HANDOFF documents, session boundaries, and human interpretation.

```
TEST finds bug ──> signal must reach ──> right upstream activity
                                          │
                                          ├── code bug?  → EXEC (tight loop)
                                          ├── task gap?  → TASK (rework loop)
                                          ├── spec gap?  → SPEC (medium loop)
                                          └── arch issue? → PLAN (wide loop)
```

**2. Completion signal (forward propagation).**
When a bead completes, downstream beads become unblocked. This signal needs to propagate through the dependency graph immediately. Today this happens within a single bead database. Cross-work, it doesn't propagate at all.

**3. Area awareness (broadcast).**
"The adapter layer is being modified by Feature A, Bug X, and Enhancement Q."
Every activity touching the adapter needs this information. It's not directional — it's ambient context.

**4. Priority signal (human -> system -> agents).**
"Stop working on features. Focus on getting the test suite green." This signal needs to reach every EXEC agent and change what TASK outputs next. Today it's communicated through HANDOFF or direct instruction — neither scales.

---

## 6. Steady State vs. Transient Behavior

### Cold start (beginning of project)

```
                    PLAN    SPEC    TASK    EXEC    TEST
Active work:        many    none    none    none    none
Flow:               ████    ░░░░    ░░░░    ░░░░    ░░░░

Characteristic: All work is upstream. No feedback loops exist yet
because nothing has been tested. Planning dominates.
The system is FILLING the pipeline.
```

Risk: Over-planning. Building a huge backlog of specs before any execution feedback. You're planning in a vacuum.

### Warm (steady state)

```
                    PLAN    SPEC    TASK    EXEC    TEST
Active work:        some    some    some    many    some
Flow:               ████    ████    ████    ████    ████

Characteristic: Work at every stage. Feedback loops are active.
New planning incorporates lessons from testing. The system is
CYCLING — work flows forward and feedback flows backward
continuously.
```

This is the healthy state. Every activity has work. Feedback from downstream informs upstream. The pipeline is full and flowing.

### Crisis (major bug or architectural issue)

```
                    PLAN    SPEC    TASK    EXEC    TEST
Active work:        surge   surge   surge   drain   blocked
Flow:               ████    ████    ████    ░░░░    ████

Characteristic: Wide feedback loop activated. Testing found
something big. Normal feature flow STOPS. All attention goes
to the feedback loop: plan fix → spec fix → task fix → exec →
re-test. The system is in RECOVERY mode.
```

The key question: how fast can the system shift from steady-state to crisis mode and back? This depends on:
- How fast the feedback signal propagates (TEST -> PLAN)
- How fast new beads can be created and prioritized (TASK)
- How fast in-flight work can be paused (EXEC)
- How fast the fix can be validated (TEST)

### Post-crisis return to steady state

```
Crisis fix validated ──> paused feature beads resume ──>
normal flow re-establishes ──> but now with UPDATED specs
incorporating the lessons from the crisis.
```

The system should be BETTER after a crisis — the specs now reflect reality more accurately. If the crisis lessons aren't captured in specs, the system is worse (same bug class will recur).

---

## 7. What kerf Needs to Model

Given these dynamics, here is what kerf — as the shared state layer — must make available.

### State that must be visible

```
What                         To whom              When
──────────────────────────   ──────────────────   ──────────────────────
Current activity graph       any agent, human     on demand (kerf map)
 (what's at each stage)

Area overlap map             PLAN, SPEC agents    when creating/modifying
 (what areas are being                            work
 touched by what)

Dependency graph             TASK, EXEC agents    when selecting next
 (what blocks what,                               work/bead
  cross-work)

Feedback queue               PLAN, TASK agents    continuously — this is
 (unprocessed signals                             the PULL signal that
  from TEST/EXEC)                                 drives re-planning

Work status                  all                  on demand (kerf next,
 (what's done, in-flight,                         kerf map)
  blocked, needs-attention)

Priority context             EXEC agents          when pulling next bead
 (what the human cares                            (kerf next)
  about right now)
```

### Transitions that kerf must support

```
Transition                    Mechanism
────────────────────────────  ──────────────────────────────────
New work enters system        kerf new (at appropriate entry point)
Work moves forward            automatic (spec finalized → tasks exist)
Feedback signal recorded      kerf <something> — a way to record
                              "TEST found X, it affects Y"
Feedback processed            PLAN/SPEC/TASK activity on the feedback
                              item, closing the loop
Priority override             kerf pin / kerf prioritize / human input
Cross-work dependency added   kerf link / automatic from bead deps
Work paused (crisis mode)     kerf pause — pull beads from queue
Work resumed                  kerf resume — re-enter beads to queue
```

### The critical missing piece: feedback ingestion

The single most important thing kerf needs that it doesn't have: **a way to record feedback from downstream activities and route it to the right upstream activity.**

Today: an agent finds a bug during testing. The agent... writes it in HANDOFF? Creates a new work item? Tells the human? There's no structured path.

What's needed:

```
TEST agent finds issue ──> kerf records it with:
                            - what failed
                            - which work/bead it relates to
                            - severity estimate (code fix? spec fix? arch fix?)

                        ──> kerf surfaces it via:
                            - kerf next (feedback items appear at top)
                            - kerf map (affected work shows status change)

                        ──> next agent session picks it up:
                            - sees prioritized feedback item
                            - processes it through the appropriate loop
                            - closes the loop when fix is verified
```

This is the "fast track" Greg described: "we almost immediately need to get those beads created, implemented and to go through another test." The fast track is a feedback loop with minimal latency.

### The graph kerf maintains

At its core, kerf maintains a graph with three types of nodes and several edge types:

```
Node types:
  WORK   ── a unit of planned change (has spec, has beads)
  AREA   ── a part of the system (defined taxonomy)
  SIGNAL ── a feedback item (finding from TEST/EXEC)

Edge types:
  WORK ──touches──> AREA      (area tagging)
  WORK ──depends──> WORK      (ordering constraint)
  WORK ──co-designs──> WORK   (awareness link, not blocking)
  SIGNAL ──affects──> WORK    (feedback routing)
  SIGNAL ──targets──> AREA    (feedback about an area, not a specific work)
```

The graph is the shared state. Every agent reads from it to orient. Every agent writes to it when they discover something. kerf computes over it (next, map, overlap detection).

---

## Summary: The System's Shape

It's a directed graph with five activity nodes and four cycle types. Work enters at different points depending on maturity. Feedback loops of varying radius are the primary mechanism for convergence on correctness. The bottlenecks are PLAN (human cognition) and TEST (validation). The critical missing infrastructure is structured feedback ingestion — getting signals from downstream back to upstream fast, through kerf as the shared state layer, without relying on lossy narrative handoffs.

The system's health is measured by:
1. **Feedback latency** — how fast do TEST findings become TASK items?
2. **Pipeline balance** — is work distributed across activities or clumped?
3. **Loop radius** — are most feedback loops tight (good) or wide (indicates spec quality issues)?
4. **Throughput** — how many beads exit TEST per unit time?
