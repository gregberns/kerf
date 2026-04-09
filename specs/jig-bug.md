# Bug Jig

> Built-in jig for structured investigation and resolution of defects.

Something is broken. This jig guides an agent through understanding what is wrong, proving it, finding the cause, and defining the fix — before any code is written. See [jig-system.md](jig-system.md) for file format, resolution, and versioning. See [jig-feature.md](jig-feature.md) for the other built-in jig.

## When to Use

Use the `bug` jig when the work involves a defect in existing behavior. The reported behavior does not match the expected behavior, and the goal is to investigate, locate, and specify a fix. If the work involves designing something new or substantially changing existing behavior, use the [`feature`](jig-feature.md) jig instead.

## Status Progression

```
triaging -> reproducing -> locating -> specifying-fix -> ready
```

## Frontmatter

The `bug` jig file contains this YAML frontmatter:

```yaml
---
name: bug
description: Structured investigation and resolution of defects
version: 1
status_values:
  - triaging
  - reproducing
  - locating
  - specifying-fix
  - ready
passes:
  - name: "Triage"
    status: triaging
    output: ["01-triage.md"]
  - name: "Reproduce"
    status: reproducing
    output: ["02-reproduction.md"]
  - name: "Locate"
    status: locating
    output: ["03-root-cause.md"]
  - name: "Specify Fix"
    status: specifying-fix
    output: ["04-fix-spec.md", "05-test-cases.md"]
file_structure:
  - spec.yaml
  - SESSION.md
  - 01-triage.md
  - 02-reproduction.md
  - 03-root-cause.md
  - 04-fix-spec.md
  - 05-test-cases.md
---
```

## Passes

### Pass 1: Triage

**Status:** `triaging`
**Output:** `01-triage.md`

Pass 1 produces a clear understanding of the bug report, its severity, and its scope.

#### Agent Instructions

You are assessing a bug report to understand what is broken and how severe it is. This pass establishes the facts before any investigation begins.

**What to do:**

1. Read the bug report or user description carefully. Identify what behavior was observed and what behavior was expected.
2. Capture the reported behavior and expected behavior as precise, observable statements. "The CLI returns exit code 0 when validation fails" not "it doesn't work right."
3. Identify affected systems, components, users, or environments. Be specific — name the command, endpoint, module, or subsystem.
4. Assess severity:
   - **Critical** — data loss, security vulnerability, or complete feature unavailability
   - **High** — major functionality broken, no workaround
   - **Medium** — functionality impaired, workaround exists
   - **Low** — cosmetic, minor inconvenience, or edge case
5. Gather any existing evidence: error messages, log output, stack traces, screenshots, or links to related issues.
6. Determine whether this is actually a bug, a feature gap, or a misunderstanding. If it is not a bug, document that conclusion and stop.

**What "done" looks like:**

`01-triage.md` contains:
- Reported behavior — what was observed
- Expected behavior — what should have happened
- Affected area — systems, components, environments
- Severity — critical / high / medium / low, with justification
- Evidence — any logs, errors, or artifacts collected
- Assessment — confirmed bug, feature gap, or needs more information

Advance status to `reproducing`.

### Pass 2: Reproduce

**Status:** `reproducing`
**Output:** `02-reproduction.md`

Pass 2 creates a reliable reproduction case that proves the bug exists and isolates the triggering conditions.

#### Agent Instructions

You are proving the bug exists and narrowing it to the minimal reproduction. A bug that cannot be reliably reproduced cannot be reliably fixed.

**What to do:**

1. Define the exact steps to reproduce the bug. Number each step. Include specific inputs, commands, or actions.
2. Execute the steps and confirm the bug occurs. Record the actual output.
3. Narrow to the minimal reproduction — the smallest set of steps, inputs, and configuration that triggers the bug. Remove anything unnecessary.
4. Document environment requirements: OS, versions, configuration, dependencies, or state that must be present for the bug to manifest.
5. If the bug cannot be reproduced:
   - Document every approach attempted and why each failed.
   - Note whether the bug is intermittent, environment-specific, or dependent on state that is difficult to recreate.
   - Escalate to the user with a clear summary of what was tried. Do not proceed to Pass 3 without either a reproduction or an explicit decision from the user to continue.

**What "done" looks like:**

`02-reproduction.md` contains:
- Steps to reproduce — numbered, exact steps
- Minimal reproduction — the smallest case that triggers the bug
- Environment — OS, versions, config, dependencies
- Observed output — what happens when the steps are followed
- Reproduction status — reliably reproduced / intermittent / not reproduced

Advance status to `locating`.

### Pass 3: Locate

**Status:** `locating`
**Output:** `03-root-cause.md`

Pass 3 traces the reproduction through the codebase to find the root cause — not just where the code fails, but why.

#### Agent Instructions

You are tracing the bug from its entry point to the root cause. The goal is to understand the defect deeply enough to specify a fix, not just to find a line number.

**What to do:**

1. Trace the reproduction steps through the code. Start at the entry point (CLI command, API endpoint, event handler) and follow the execution path.
2. Identify the specific code path that fails. Name the file, function, and line range.
3. Determine *why* the code fails, not just *where*. Explain the logical error, incorrect assumption, missing check, or race condition.
4. Search for related issues — similar patterns elsewhere in the codebase that may have the same flaw. Note any found.
5. Determine the blast radius of a fix: what other code paths, features, or behaviors depend on or are affected by the code that must change.

**What "done" looks like:**

`03-root-cause.md` contains:
- Entry point — where execution begins for the reproduction case
- Execution trace — the code path from entry to failure, with file and function references
- Root cause — the specific defect: what the code does wrong and why
- Related patterns — other locations in the codebase with the same or similar issue
- Blast radius — what else is affected by the code that must change

Advance status to `specifying-fix`.

### Pass 4: Specify Fix

**Status:** `specifying-fix`
**Output:** `04-fix-spec.md`, `05-test-cases.md`

Pass 4 defines what the fix should look like — the approach, risks, and test cases — without writing implementation code.

#### Agent Instructions

You are specifying the fix so that an implementing agent knows exactly what to change. Everything an implementer needs goes here. Do not write implementation code — describe the change at the level of logic and structure.

**What to do:**

1. Propose the fix approach. Describe the change at the level of logic and structure — what condition to add, what function to modify, what data flow to correct. Do not write implementation code.
2. If there are multiple viable approaches, list them with tradeoffs and recommend one.
3. Identify risks or side effects of the fix. Will it change any public API? Affect performance? Alter behavior in cases that currently work correctly?
4. Define test cases that verify the fix works:
   - The original reproduction case passes after the fix.
   - Any boundary conditions around the fix are covered.
5. Define regression tests that prevent recurrence:
   - Tests that would have caught this bug before it shipped.
   - Tests for the related patterns identified in Pass 3, if applicable.
6. Estimate the scope of changes: which files change, approximate number of changes, and complexity (trivial / straightforward / involved).

**What "done" looks like:**

`04-fix-spec.md` contains:
- Proposed fix — the approach, described structurally
- Alternatives considered — other approaches and why they were not chosen (omit if only one viable approach)
- Risks and side effects — what could go wrong or change unexpectedly
- Scope estimate — files affected, approximate size, complexity

`05-test-cases.md` contains:
- Verification tests — tests that confirm the fix resolves the bug
- Regression tests — tests that prevent recurrence
- Edge cases — boundary conditions worth covering

Run `kerf square <codename>` to verify structural completeness. When verification passes and the user approves, advance status to `ready`.

## Finalization

When the work reaches `ready` status and `kerf square` passes, the work is eligible for [finalization](finalization.md). `kerf finalize <codename>` packages the work for implementation handoff.

## File Structure

A work governed by the `bug` jig contains the following files:

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

`spec.yaml` and `SESSION.md` are defined in [works.md](works.md) and [sessions.md](sessions.md) respectively. All other files are pass outputs defined by this jig.
