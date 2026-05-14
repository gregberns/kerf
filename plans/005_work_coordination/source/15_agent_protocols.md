# Agent Protocols for Work Coordination

> Design analysis: what makes protocols work for AI agents, and concrete protocol
> designs for session lifecycle, co-design synchronization, late requirements,
> disruptions, and protocol evolution.

---

## 1. What Makes a Protocol Agent-Executable

Agents are not humans. They follow instructions literally, lose state between
sessions, have finite context windows, and cannot "use judgment" without a
decision tree to follow. Protocols designed for agents need different properties
than protocols designed for people.

### Required Properties

**Observable conditions, not assessments.** Every decision point must reference
something the agent can check mechanically. "Check if file X exists" works.
"Assess whether the design is compatible" does not — unless you decompose it
into checks like "do both specs reference the same interface? do they define
conflicting types for any shared field?"

**Bounded scope per step.** Each protocol step should have a clearly delimited
action. "Read these 2 files and compare field X" is bounded. "Make sure
everything is consistent" is unbounded — the agent doesn't know when it has
checked enough.

**Explicit stop and escalation conditions.** The protocol must say when the
agent is done and when it should stop and ask the human. Without this, agents
either gold-plate (keep checking forever) or under-check (declare victory after
one step). The key distinction: stop conditions should be structural ("all items
in the list have been processed") not qualitative ("when you're satisfied").

**Decision trees, not heuristics.** "If X then Y, else Z" works. "Use your best
judgment" does not. When judgment is genuinely required, the protocol should
escalate to the human rather than asking the agent to simulate judgment.

**Idempotent steps.** Because agents lose state between sessions and may re-run
protocols from the beginning, each step should be safe to repeat. Checking a
file that was already checked is fine. Appending a duplicate entry to a log is
not.

**Artifacts as state, not memory.** Agents can't remember what they did last
session. Every protocol outcome must be written to a file or field that the next
session can read. If the protocol produces a decision, that decision must be
recorded somewhere durable — not held in the agent's context.

### Properties That Make Protocols Fragile

**Narrative instructions.** Long paragraphs describing what to do. Agents parse
these inconsistently. Numbered steps with clear verbs work better.

**Implicit ordering.** If step 3 depends on step 2's output, say so explicitly.
Don't assume the agent will infer the dependency.

**Unbounded reads.** "Read all relevant specs" is dangerous because "relevant"
is undefined. Specify which files to read, or provide a mechanical way to
determine the set (e.g., "read the spec.yaml of every work listed in
`depends_on`").

**Context window exhaustion.** Protocols that require the agent to hold too much
information simultaneously will degrade. Design for streaming: process one item,
record the result, move to the next.

**Silent failures.** If a protocol step can fail (file doesn't exist, field is
missing), the protocol must say what to do. The default should be "record the
failure and continue" or "stop and escalate," never "silently skip."

### The Format Question

Protocols should be structured as numbered steps with:
- **Precondition**: what must be true before this step runs
- **Action**: what to do (a single verb phrase)
- **Output**: what artifact or field is produced
- **Branch**: if/then for different outcomes
- **Escalation**: when to stop and ask the human

This format is verbose but unambiguous. Agents execute it consistently because
there is no room for interpretation.

---

## 2. Session-Start Protocol

When an agent begins a new session, it needs to orient itself in the work
landscape. The current approach — reading HANDOFF.md — fails because HANDOFF
accumulates narrative, loses structure, and plays telephone across sessions.

### The Protocol

```
SESSION-START PROTOCOL

Trigger: Agent begins a new session with no prior context.

Step 1: IDENTIFY PROJECT
  Action: Read `.kerf/project-identifier` in the current repo.
  Output: project_id
  Failure: If file missing, ask user which project to work on. STOP.

Step 2: LOAD PORTFOLIO
  Action: Run `kerf map`.
  Output: Structured portfolio view (statuses, dependencies, area clusters,
          actionable/blocked lists).
  Record: Save the output for reference during this session.

Step 3: CHECK FOR ASSIGNED WORK
  Action: Check if the user provided a specific work codename or task.
  Branch:
    - If yes → go to Step 6 (LOAD SPECIFIC WORK).
    - If no → go to Step 4 (DETERMINE NEXT WORK).

Step 4: DETERMINE NEXT WORK
  Action: Run `kerf next` (or read the "Actionable Now" section of kerf map).
  Output: Ranked list of actionable works.
  Branch:
    - If exactly one actionable work → go to Step 6.
    - If multiple actionable works → go to Step 5.
    - If zero actionable works → report "all works are blocked or complete."
      Ask user what to do. STOP.

Step 5: SELECT WORK (requires user input)
  Action: Present the ranked list to the user with one-line summaries.
  Ask: "Which work should I focus on?"
  Output: Selected codename.
  Note: Do NOT auto-select. The ranking is a suggestion, not a decision.
        The exception: if the agent is running in an automated pipeline
        (e.g., harmonik dispatch), take the top-ranked item.

Step 6: LOAD SPECIFIC WORK
  Action: Run `kerf resume {codename}`.
  Output: Work state, jig position, dependency status, area peers.
  Record: Note which dependencies are incomplete and which area peers exist.

Step 7: CHECK CO-DESIGNS
  Action: Read the `depends_on` field of the work's spec.yaml.
          Filter for entries with `relationship: co-designs`.
  Branch:
    - If co-designs exist → run CO-DESIGN PROTOCOL (Section 3) before
      proceeding with work.
    - If no co-designs → continue.

Step 8: CHECK SESSION HISTORY
  Action: Read SESSION.md in the work's directory.
  Output: What the previous session accomplished, what it intended next,
          any warnings or surprises it recorded.
  Branch:
    - If SESSION.md contains unresolved blockers → present to user. Ask
      whether to proceed or address blockers first. STOP until answered.
    - If SESSION.md is clean → proceed with work.

Step 9: BEGIN WORK
  Action: Identify the current jig pass from spec.yaml status.
          Read the jig's guidance for that pass.
          Execute the pass.
  Output: Work artifacts as defined by the jig.

DONE when: Step 9 is reached and work begins.
```

### Design Decisions

**Why `kerf map` before `kerf resume`?** The agent needs portfolio context
before diving into a specific work. Without the map, it doesn't know about area
peers, blocked items, or the overall state. This is the "look at the whiteboard
before picking up a task" step.

**Why require user selection for multiple actionable items?** Automated
selection is tempting but wrong for most contexts. The ranking algorithm
(fan-out, priority, age) captures structural importance but not business
importance. The user might know that the third-ranked item is actually urgent
because of an external deadline. The exception is automated pipelines where no
user is available.

**Why check co-designs as a separate step?** Co-design relationships require
specific actions (reading peer specs, checking for contradictions) that are
distinct from just knowing that area peers exist. Area peers are informational;
co-designs are obligatory.

---

## 3. Co-Design Protocol

Two works are flagged as co-designs when they touch overlapping areas and their
design decisions must be mutually aware. This protocol defines what an agent
does when it encounters a co-design relationship.

### When Co-Designs Are Created

Co-design relationships are created:
1. Manually by the user via `kerf link A B --rel co-designs`.
2. Suggested by `kerf new` when area overlap is detected with an in-flight work.
3. Discovered during a session when an agent realizes its work's design
   decisions affect another work.

### The Protocol

```
CO-DESIGN PROTOCOL

Trigger: A work has one or more `co-designs` entries in its depends_on list.

Step 1: ENUMERATE CO-DESIGN PEERS
  Action: Read spec.yaml for the current work.
          Extract all depends_on entries where relationship = "co-designs".
  Output: List of (codename, project) pairs.

Step 2: LOAD PEER STATE (for each peer)
  Action: Read the peer's spec.yaml.
  Extract: status, areas, jig, current pass.
  Output: A status record for each peer.

Step 3: CLASSIFY PEER STAGES (for each peer)
  Action: Compare peer's status against its status_values list.
  Branch:
    - AHEAD: Peer is at a later stage than current work (e.g., peer is
      implementing, current work is in research). The peer's design
      decisions are more settled.
    - SAME: Both works are at comparable stages. Design decisions are
      still in flux for both.
    - BEHIND: Peer is at an earlier stage. Its design decisions are less
      settled than ours.

Step 4: READ PEER ARTIFACTS (for each peer)
  Action: Based on peer stage, read the appropriate artifacts:
    - AHEAD peer: Read all design artifacts (spec drafts, component
      decompositions, interface definitions). These are constraints
      the current work should respect.
    - SAME peer: Read whatever artifacts exist. Note areas of potential
      conflict.
    - BEHIND peer: Read problem-space artifacts. Note areas where the
      current work's decisions will constrain the peer later.
  Output: For each peer, a list of:
    - Shared interfaces (types, APIs, data structures both works reference)
    - Design decisions already made by the peer
    - Potential conflicts (places where both works make claims about
      the same thing)

Step 5: RECONCILE
  Branch:
    A. NO CONFLICTS found:
       - Record in SESSION.md: "Co-design check with {peer}: no conflicts
         found. Peer status: {status}. Date: {now}."
       - Proceed with work.

    B. CONFLICTS found, current work is BEHIND or SAME:
       - The peer's decisions take precedence if the peer is ahead.
       - Adjust the current work's design to be compatible.
       - Record in SESSION.md what was adjusted and why.
       - If adjustment requires changing already-completed artifacts,
         update them and note the change.
       - Proceed with work.

    C. CONFLICTS found, current work is AHEAD:
       - The current work's decisions are more settled.
       - Record the conflict in SESSION.md.
       - Add a note to the PEER's SESSION.md (or a shared coordination
         artifact): "Work {current} has made design decisions about
         {interface}. When {peer} reaches design phase, it must
         conform to: {list of decisions}."
       - Proceed with work.

    D. CONFLICTS found, DEEP INCOMPATIBILITY:
       - The designs are fundamentally incompatible (e.g., one assumes
         synchronous calls, the other assumes event-driven).
       - STOP. Do not proceed.
       - Report to user: "Co-design conflict between {current} and
         {peer}: {description}. These cannot both proceed with current
         designs. Options: (1) revise current work, (2) revise peer,
         (3) create a shared interface spec that both conform to."
       - Wait for user decision.

DONE when: All peers have been checked and either reconciled (A/B/C)
or escalated (D).
```

### What "Synchronize" Means Concretely

Synchronization is not a vague "make sure they're compatible." It is:

1. **Identify shared surfaces.** What interfaces, data structures, APIs, or
   system areas do both works reference? This is a mechanical check: look for
   the same type names, function signatures, or area tags.

2. **Check for contradictions.** Do both works define the same interface
   differently? Does one assume a dependency that the other removes? This
   requires reading the design artifacts and comparing claims.

3. **Establish precedence.** When there is a conflict, the work that is further
   along in its lifecycle has precedence. Its decisions are more costly to
   change. The behind work adapts.

4. **Record the outcome.** Write down what was checked, what was found, and what
   was decided. This is the artifact that prevents the next session from
   re-checking everything.

### Signaling That Co-Design Has Been Addressed

The protocol records outcomes in SESSION.md with structured entries:

```markdown
## Co-Design Check: {date}
- Peer: {codename} (status: {status})
- Shared surfaces: {list}
- Conflicts: {none | list}
- Resolution: {compatible | adjusted {what} | escalated}
```

The next session can read these entries and skip re-checking peers whose status
hasn't changed since the last check. This is the idempotency mechanism: check
the peer's `updated` timestamp against the co-design check date.

---

## 4. Late-Requirement Protocol

A new requirement arrives that overlaps with in-flight work. This is Problem 4
from the plan: the requirement is architecturally entangled with work that's
already partially designed or implemented.

### Detection

Late requirements are detected at two points:
1. **At creation time.** `kerf new` checks area tags and warns about overlap
   with active works. This is the primary detection mechanism.
2. **During a session.** An agent working on one thing realizes it needs
   something that overlaps with in-flight work. The agent runs `kerf map` to
   check for area peers.

### The Protocol

```
LATE-REQUIREMENT PROTOCOL

Trigger: A new requirement is identified that overlaps with an in-flight
work (detected via area overlap warning or agent observation).

Step 1: IDENTIFY THE OVERLAP
  Action: Determine which in-flight work(s) the new requirement overlaps with.
  For each overlapping work, record:
    - codename
    - current status
    - which areas overlap
    - how deep the overlap is (shared interface? same component?
      same function?)

Step 2: ASSESS IN-FLIGHT WORK STAGE
  Action: For each overlapping work, classify its stage:
    A. PRE-DESIGN: Status is before design phases (problem-space, research).
       The work hasn't made binding design decisions yet.
    B. IN-DESIGN: Status is in a design phase (change-spec, spec-draft,
       decompose). Design decisions are being made but aren't final.
    C. POST-DESIGN: Status is past design (tasks, implementing, ready).
       Design decisions are made and possibly partially implemented.

Step 3: ASSESS OVERLAP DEPTH
  Action: Classify the overlap:
    A. SURFACE: Both works reference the same area but don't share
       interfaces or data structures. They can proceed independently
       with awareness.
    B. INTERFACE: Both works touch the same interface or API boundary.
       They need a shared definition but can design internals
       independently.
    C. DEEP: Both works make claims about the same internal structures,
       algorithms, or state. They cannot be designed independently.

Step 4: DECIDE PATH
  Action: Use the stage x depth matrix:

  | Depth \ Stage | PRE-DESIGN        | IN-DESIGN          | POST-DESIGN         |
  |---------------|-------------------|--------------------|--------------------|
  | SURFACE       | Proceed. Add      | Proceed. Add       | Proceed. Add       |
  |               | co-designs link.  | co-designs link.   | inform link.       |
  | INTERFACE     | Proceed. Add      | Pause new work.    | Create new work    |
  |               | co-designs link.  | Define shared      | with co-designs    |
  |               |                   | interface first.   | link. Inherit      |
  |               |                   | Then both proceed. | interface from     |
  |               |                   |                    | existing work.     |
  | DEEP          | Merge. Fold new   | STOP. Escalate     | STOP. Escalate     |
  |               | requirement into  | to user. Options:  | to user. Options:  |
  |               | existing work.    | (1) merge works,   | (1) extend existing|
  |               |                   | (2) pause both and | work, (2) pause    |
  |               |                   | replan.            | and replan.        |

Step 5: EXECUTE DECISION
  Branch by decision from Step 4:

  PROCEED:
    - Create the new work: `kerf new`.
    - Add the relationship: `kerf link {new} {existing} --rel {type}`.
    - Continue with normal jig workflow.

  PAUSE AND DEFINE SHARED INTERFACE:
    - Create a minimal interface spec that both works must conform to.
      This could be an area spec file or an artifact in one of the works
      that the other references.
    - Record the shared interface location in both works' SESSION.md.
    - Both works proceed with the constraint that their designs must
      conform to the shared interface.

  MERGE:
    - Add the new requirement to the existing work's problem-space
      or spec artifacts.
    - If the existing work has already passed the relevant design phase,
      update its status back to that phase for the merged portion.
    - Record the merge in SESSION.md with rationale.

  ESCALATE:
    - Report to user with the overlap analysis and options.
    - Wait for user decision.
    - Execute whatever the user decides.

Step 6: RECORD DECISION
  Action: Regardless of path taken, record in SESSION.md:
    - What requirement arrived
    - Which work(s) it overlapped with
    - What path was chosen and why
    - What links/artifacts were created

DONE when: Decision is executed and recorded.
```

### The Key Insight: Stage x Depth

The decision is not one-dimensional. "How far along is the in-flight work?"
matters, but so does "how deep is the overlap?" A surface overlap with a
post-design work is fine — just add a link and keep going. A deep overlap with a
post-design work is a crisis — you might need to replan.

The matrix above is a decision tree disguised as a table. An agent can look up
its situation and get a concrete action. No judgment required for 6 of the 9
cells. Only the 3 DEEP cases require escalation, and those are genuinely hard
problems where human judgment is warranted.

---

## 5. Session-End Protocol

The session-end problem: HANDOFF documents drift, bloat, and play telephone.
Details get distorted. Minor observations become "blockers" three sessions later.
The structured alternative replaces narrative with fields.

### The Protocol

```
SESSION-END PROTOCOL

Trigger: Agent is ending its session (context filling up, natural stopping
point, user requests handoff).

Step 1: UPDATE WORK STATUS
  Action: If the work's status should change based on what was accomplished,
          update spec.yaml status field.
  Output: Updated spec.yaml (or no change if status didn't advance).

Step 2: WRITE STRUCTURED SESSION ENTRY
  Action: Append a structured entry to SESSION.md using the format below.
          Do NOT rewrite previous entries. Do NOT summarize previous entries.
          Append only.

  Format:
  ```
  ## Session: {date}

  ### Completed
  - {Concrete deliverable 1: "Wrote 02-components.md with 4 components"}
  - {Concrete deliverable 2: "Updated spec.yaml status to change-spec"}

  ### Next
  - {Specific next action 1: "Write 03-change-spec.md for the adapter interface"}
  - {Specific next action 2: "Resolve co-design conflict with brave-falcon on retry types"}

  ### Discovered
  - {Unexpected finding 1: "The adapter interface has 3 callers, not 2 as assumed"}
  - {Unexpected finding 2: "brave-falcon's retry logic assumes synchronous calls"}

  ### Blocked
  - {Blocker, if any: "Cannot proceed with X until Y is resolved"}
  - {Or: "None"}
  ```

  Rules:
  - COMPLETED: Only things actually done this session. Not aspirations.
  - NEXT: Specific enough that a fresh agent can execute without guessing.
    Bad: "Continue implementation." Good: "Implement the RetryPolicy
    struct in adapter/retry.go per the interface in 02-components.md."
  - DISCOVERED: Things that surprised you. Things the next session needs
    to know that aren't obvious from the artifacts. This is the ONLY
    place for "soft" information. Keep it to genuine surprises, not
    routine observations.
  - BLOCKED: Only genuine blockers. Something that prevents the next
    step from executing. NOT "this might be tricky" — that goes in
    Discovered.

Step 3: TRIM SESSION HISTORY (if needed)
  Action: If SESSION.md has more than 10 session entries, archive older
          entries.
  How: Move entries older than the most recent 5 to a file called
       SESSION-ARCHIVE.md in the work directory. Keep only the 5 most
       recent entries in SESSION.md.
  Why: Prevents context window exhaustion. The most recent 5 sessions
       contain the relevant operational context. Older sessions are
       available if needed but don't consume context by default.

Step 4: UPDATE SPEC.YAML SESSION RECORD
  Action: Add a session entry to spec.yaml's sessions list with:
    - started: session start time
    - ended: now
    - notes: one-line summary of what was accomplished

Step 5: VERIFY ARTIFACTS
  Action: Run `kerf square {codename}`.
  Branch:
    - If square passes → session end is clean.
    - If square fails → record failures in the NEXT section of
      SESSION.md so the next session addresses them.

DONE when: SESSION.md is updated, spec.yaml session record is added,
and square has been run.
```

### Why This Prevents Telephone

The key anti-telephone mechanisms:

1. **Append-only.** Each session adds its own entry. It never rewrites previous
   entries. Previous sessions' words are preserved exactly. No paraphrasing, no
   summarization, no drift.

2. **Structured fields prevent category drift.** A "discovered" item stays in
   the Discovered section. It can't migrate to Blocked through successive
   rewrites. The next session reads it as a discovery, not a blocker.

3. **Archiving prevents bloat.** Only 5 recent entries are in the active file.
   This bounds the context an agent needs to consume at session start.

4. **Artifacts are the source of truth, not SESSION.md.** SESSION.md is
   operational metadata — what happened, what's next. The actual design decisions
   live in the jig artifacts. If SESSION.md and an artifact disagree, the
   artifact wins.

---

## 6. Disruption Protocol

During implementation, something unexpected happens: a test failure reveals a
design flaw, a dependency turns out to be harder than expected, a new piece of
information invalidates an assumption.

### The Protocol

```
DISRUPTION PROTOCOL

Trigger: An unexpected event occurs during work execution that may affect
the work's design, scope, or dependencies.

Step 1: CLASSIFY THE DISRUPTION
  Action: Determine the disruption type:
    A. LOCAL: Affects only the current work. Example: a test failure in
       the current component, an implementation detail that's harder
       than expected.
    B. INTERFACE: Affects the boundary between the current work and
       another work. Example: an API that the current work consumes
       doesn't behave as documented.
    C. DESIGN: Invalidates a design decision made in an earlier pass.
       Example: the assumed data model can't support a required query
       pattern.
    D. SCOPE: Reveals that the work's scope is wrong — either too large,
       too small, or missing a critical piece.

Step 2: ASSESS SEVERITY
  Action: Can the work continue with a local fix, or does it need to
          stop and re-plan?
  Test: "Can I fix this by changing only implementation details within
        the current work, without changing any spec artifacts or
        affecting any other work?"
  Branch:
    - YES → this is a LOCAL disruption. Go to Step 3A.
    - NO → go to Step 3B.

Step 3A: HANDLE LOCALLY
  Action: Fix the issue within the current work.
  Record: Add a DISCOVERED entry in SESSION.md noting what happened
          and how it was resolved.
  Continue: Resume normal work execution.
  DONE.

Step 3B: ASSESS IMPACT SCOPE
  Action: Determine what is affected beyond the current work.
  Check:
    - Does this affect any works in the depends_on list?
    - Does this affect any works that depend on the current work?
    - Does this affect any area peers?
    - Does this invalidate completed artifacts in the current work?
  Output: List of affected works and artifacts.

Step 4: DECIDE RESPONSE
  Branch by disruption type:

    INTERFACE disruption:
      - Identify which work "owns" the interface.
      - If current work owns it: update the interface spec, notify
        dependent works via SESSION.md entries.
      - If another work owns it: record the incompatibility. Check if
        the other work has a co-designs relationship. If not, add one.
        Escalate to user: "Interface {X} defined by {work} doesn't
        match what {current work} needs. Options: (1) adapt current
        work, (2) request change to {work}."

    DESIGN disruption:
      - If the design flaw is in a completed artifact: update the
        artifact. Roll status back to the design phase if necessary.
        Record the rollback in SESSION.md with rationale.
      - If the design flaw affects other works (via co-designs or
        dependencies): STOP. Escalate to user. This is a cross-work
        design change that needs human judgment.

    SCOPE disruption:
      - If scope is too large: propose splitting the work. Record the
        proposal in SESSION.md. Ask user to approve before creating
        new works.
      - If scope is too small or missing a piece: propose extending.
        If the extension is minor (< 20% of original scope), extend
        and record in SESSION.md. If major, escalate to user.

Step 5: UPDATE WORK GRAPH
  Action: If the disruption changed dependencies or relationships:
    - Add/update depends_on entries in spec.yaml.
    - If new works were created, add appropriate links.
    - Run `kerf map` to verify the graph is consistent (no cycles,
      no orphaned dependencies).

Step 6: SIGNAL TO OTHER AGENTS
  Action: For each affected work that has an active session or is
          likely to be resumed soon:
    - Add a note to that work's SESSION.md under a "## External
      Update" heading: "{work} encountered {disruption type} that
      affects this work: {description}. Action needed: {what}."
    - If using agent-mail or similar coordination: send a message
      to the agent working on the affected work.

DONE when: Disruption is classified, response is executed, affected
works are notified, and SESSION.md is updated.
```

### The Keep-Going vs. Stop Boundary

The critical question: when does an agent keep going vs. stop and escalate?

The rule is simple: **if the fix stays inside the current work's implementation
(no spec changes, no impact on other works), keep going. Otherwise, stop.**

This is conservative by design. Agents are bad at predicting second-order
effects of design changes. A "small" interface change can cascade through
multiple works. The cost of stopping and asking is one round-trip with the user.
The cost of an agent silently making a cross-work design change that turns out
to be wrong is re-doing multiple works.

The exception: if the agent is running in an automated pipeline with no user
available, it should record the disruption, shelve the current work, and move
to the next actionable work. The disruption gets addressed in the next human
review cycle.

---

## 7. Protocol Evolution

Protocols will be wrong initially. They need to be refined based on real usage.
This section addresses where protocols live, how they get updated, and how
agents pick up changes.

### Where Protocols Live

Protocols live in **jig pass guidance**. Each jig already defines passes with
guidance text that tells the agent what to do. Protocols are an extension of
this: structured instructions embedded in the pass guidance.

Specifically:
- **Session-start protocol** → embedded in `kerf resume` output. When an agent
  resumes a work, kerf emits the protocol steps as part of its output. The agent
  doesn't need to find the protocol — it's delivered to them.
- **Co-design protocol** → embedded in the `kerf resume` output when co-design
  relationships are detected. The protocol steps appear alongside the co-design
  warning.
- **Late-requirement protocol** → embedded in `kerf new` output when overlap is
  detected. The decision matrix appears alongside the overlap warning.
- **Session-end protocol** → embedded in a `kerf handoff` (or similar) command
  that the agent runs at session end.
- **Disruption protocol** → part of the jig's implementation pass guidance.
  Surfaced when the agent is in an implementing status.

### Why Embed in Command Output

This is the key design decision. Protocols could live in:

1. **A separate file the agent reads.** Problem: agents forget to read it, or
   read it once and then ignore it as context fills up.
2. **The jig definition.** Problem: agents read jig guidance at the start of a
   pass, not mid-pass when disruptions happen.
3. **Command output.** The protocol is delivered at the exact moment it's
   relevant, in the exact context where the agent will act on it.

Option 3 is the most robust because it requires no agent initiative. The agent
runs `kerf resume`, and the protocol is right there in the output. The agent
doesn't need to remember to look for it.

The downside: command output becomes longer. This is mitigated by only including
protocol steps that are relevant to the current situation (e.g., co-design
protocol only appears when co-designs exist).

### How Protocols Get Updated

1. **Protocol text lives in kerf's codebase** as part of command output
   templates and jig definitions.
2. **Updates ship with kerf releases.** When a protocol is refined, it's a code
   change to kerf (updating the output template or jig guidance).
3. **Agents automatically pick up changes** because they read the command output
   fresh each session. There is no cached copy of the protocol that can go stale.

This means protocol updates have the same lifecycle as code changes:
spec the change, implement it in kerf, release it. The spec-driven process
applies to protocols too.

### How Protocols Get Refined

The refinement loop:

1. **Observe.** Watch agents follow the protocol across several sessions.
   Identify where they deviate, get stuck, or produce bad outcomes.
2. **Diagnose.** Is the problem in the protocol (ambiguous step, missing branch,
   wrong escalation threshold) or in the agent (misreading clear instructions)?
3. **Fix.** If protocol problem: update the protocol text in kerf. If agent
   problem: make the protocol more explicit (agents misreading clear
   instructions usually means the instructions weren't as clear as you thought).
4. **Test.** Run the updated protocol through a few sessions and verify the
   deviation is fixed.

The feedback mechanism: SESSION.md entries capture what agents actually did.
Comparing SESSION.md entries against protocol steps reveals where agents diverge.
This is a manual review process — but it only needs to happen during the
protocol stabilization period, not forever.

---

## 8. Graceful Degradation

Protocols will not be followed perfectly. Agents will skip steps, misclassify
situations, and produce incomplete records. The system must degrade gracefully
when this happens.

### Design for Partial Compliance

**Every protocol should produce value even if only half the steps are
followed.** The session-end protocol is useful even if the agent only writes the
COMPLETED and NEXT sections and skips DISCOVERED and BLOCKED. The co-design
protocol is useful even if the agent reads peer artifacts but doesn't write a
formal reconciliation record.

**No protocol should create a broken state if abandoned mid-way.** If an agent
starts the late-requirement protocol, classifies the overlap as DEEP, and then
runs out of context before escalating — the work should still be in a valid
state. The next session can pick up from wherever the agent left off.

**Detection of skipped protocols should be mechanical.** kerf can check: "Does
this work have co-design relationships? Is there a co-design check entry in
SESSION.md dated after the peer's last update?" If not, the protocol was skipped.
`kerf square` can include this check and surface it as a warning.

### Recovery When Protocols Fail

The recovery mechanism is always the same: **the next session runs the
session-start protocol, which re-evaluates the current state from artifacts
(not from memory), and picks up wherever things were left.**

This is why artifacts-as-state is so important. If an agent crashed mid-session
and wrote nothing to SESSION.md, the next agent can still orient itself by:
1. Reading `kerf map` (portfolio state from spec.yaml files)
2. Reading the work's artifacts (what design decisions exist)
3. Running `kerf square` (what's missing or invalid)

The worst case of protocol failure is lost operational context (what the
previous agent was thinking, why it made a choice). But the structural state
(what exists, what's complete, what's blocked) is always recoverable from the
filesystem.

---

## 9. Summary: The Protocol Stack

From bottom to top:

| Layer | Protocol | Trigger | Key Output |
|-------|----------|---------|------------|
| Session lifecycle | Session-Start | New session begins | Work selection, context load |
| Session lifecycle | Session-End | Session ending | Structured SESSION.md entry |
| Coordination | Co-Design | co-designs relationship detected | Reconciliation record |
| Coordination | Late-Requirement | Area overlap with in-flight work | Path decision (proceed/merge/escalate) |
| Exception | Disruption | Unexpected event during work | Classification + response |
| Meta | Evolution | Protocol observed to fail | Updated protocol text in kerf |

These protocols are layered: session protocols run every session, coordination
protocols run when relationships exist, exception protocols run when things go
wrong, and the evolution protocol runs when the protocols themselves need fixing.

The design principle throughout: **make the right thing easy and the wrong thing
visible.** Protocols are delivered in command output (easy). Skipped protocols
are detected by `kerf square` (visible). No protocol blocks work or creates
hard gates — but every protocol leaves an artifact that the next session can
check.
