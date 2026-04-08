# Default Jigs

The tool ships with opinionated, built-in jigs that give users a working process out of the box. These are the "this is how you walk through a feature/bug/etc." definitions.

Jigs are replaceable — users can modify or create their own. But the defaults should be good enough that most users never need to.

## Feature Jig

**Purpose:** Full specification process for new features, subsystems, or significant enhancements.

**When to use:** The work involves designing something new or substantially changing existing behavior. Requires understanding the problem, breaking it into parts, researching approaches, and producing implementation-level guidance.

### Status Progression
```
problem-space -> decomposition -> research -> detailed-spec -> review -> ready
```

### Passes

#### Pass 1: Problem Space — rough cut (`problem-space`)
**Goal:** Understand what we're solving and why.
- Clarify the user's goals and motivations
- Define scope boundaries (what's in, what's explicitly out)
- Identify constraints (technical, business, timeline)
- Capture success criteria — how do we know this worked?
- 2-3 conversational exchanges, not a questionnaire

**Output:** `01-problem-space.md`

#### Pass 2: Decomposition (`decomposition`)
**Goal:** Break the problem into manageable components with concrete requirements.
- Identify 3-7 components (fewer is better)
- For each component, define concrete, testable requirements
- Requirements describe WHAT, not HOW
- Each requirement should be verifiable — "supports X" not "is good at X"
- Identify component dependencies and interfaces

**Output:** `02-components.md`

#### Pass 3: Research (`research`)
**Goal:** Investigate technical approaches, constraints, and existing patterns.
- For each component, identify 3-5 research questions
- Explore existing code patterns in the target codebase
- Investigate external APIs, libraries, or services needed
- Identify technical risks and constraints
- Present findings with options and tradeoffs, not just "the answer"

**Output:** `03-research/{component}/findings.md`

#### Pass 4: Detailed Spec — fine cut (`detailed-spec`)
**Goal:** Write implementation-level specifications informed by research.
- Architecture decisions with rationale
- File-level guidance (which files to create/modify)
- Interface definitions (APIs, data shapes, contracts)
- Error handling and edge case strategies
- Migration/backwards-compatibility considerations

**Output:** `04-plans/{component}-spec.md`

#### Pass 5: Review (`review`)
**Goal:** Assemble, validate, and prepare for handoff.
- Create integration plan (how components connect)
- Build implementation checklist (ordered task list)
- Identify follow-ups (blocked, independent, deferred)
- Assemble the final spec document
- Review for completeness, consistency, and testability

**Output:** `05-integration.md`, `06-checklist.md`, `SPEC.md`

### File Structure
```
{codename}/
  spec.yaml
  SESSION.md
  01-problem-space.md
  02-components.md
  03-research/{component}/findings.md
  04-plans/{component}-spec.md
  05-integration.md
  06-checklist.md
  SPEC.md
```

---

## Bug Jig

**Purpose:** Structured investigation and resolution of defects.

**When to use:** Something is broken. The goal is to understand what's wrong, prove it, find the cause, and define the fix — before any code is written.

### Status Progression
```
triaging -> reproducing -> locating -> specifying-fix -> ready
```

### Passes

#### Pass 1: Triage (`triaging`)
**Goal:** Understand the bug report and assess severity/scope.
- Capture the reported behavior vs. expected behavior
- Identify affected systems/users/environments
- Assess severity and urgency
- Gather any existing evidence (logs, screenshots, error messages)
- Determine if this is actually a bug vs. a feature gap or misunderstanding

**Output:** `01-triage.md`

#### Pass 2: Reproduce (`reproducing`)
**Goal:** Create a reliable reproduction case.
- Define exact steps to reproduce
- Identify the minimal reproduction (smallest case that triggers the bug)
- Document environment requirements
- If it can't be reproduced, document what was tried and escalate

**Output:** `02-reproduction.md`

#### Pass 3: Locate (`locating`)
**Goal:** Find the root cause in the codebase.
- Trace the reproduction through the code
- Identify the specific code path that fails
- Understand why it fails (not just where)
- Identify any related issues or similar patterns elsewhere
- Determine blast radius of the fix

**Output:** `03-root-cause.md`

#### Pass 4: Specify Fix (`specifying-fix`)
**Goal:** Define what the fix should look like, without implementing it.
- Propose the fix approach with rationale
- Define test cases that will verify the fix
- Define regression tests to prevent recurrence
- Identify any risks or side effects of the fix
- Estimate scope of changes (files, lines, complexity)

**Output:** `04-fix-spec.md`, `05-test-cases.md`

### File Structure
```
{codename}/
  spec.yaml
  SESSION.md
  01-triage.md
  02-reproduction.md
  03-root-cause.md
  04-fix-spec.md
  05-test-cases.md
```

---

## Future Jigs (not in v1)

### Migration Jig
For database migrations, API version bumps, framework upgrades. Emphasizes backwards compatibility, rollback plans, and staged rollout.

### Refactor Jig
For structural improvements that don't change behavior. Emphasizes before/after architecture, incremental steps, and verification that behavior is preserved.

### Exploration Jig
For open-ended technical investigation. Less structured than feature — more about capturing findings, options, and recommendations. Good for "should we use X?" or "how does Y work in our codebase?"

---

## Jig Design Principles

1. **Opinionated but not rigid.** The passes are guidance, not gates. An agent can skip a pass if the user says "we already know the root cause, skip to fix spec."

2. **Each pass produces a file.** This is critical for persistence and resumability. If it's not in a file, it's lost when the session ends.

3. **Requirements (WHAT) before implementation (HOW).** The decomposition pass captures what's needed. The detailed spec pass captures how to build it. This separation prevents premature implementation decisions.

4. **Concrete over vague.** "Supports up to 10,000 concurrent sessions" not "is scalable." "Returns 404 with error body when resource not found" not "handles errors gracefully."

5. **The jig teaches the agent the process.** A new agent with no context should be able to read the jig file and know exactly what to do at each pass, what questions to ask, what files to produce, and what "done" looks like.
