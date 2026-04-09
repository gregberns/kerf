# Feature Jig

> Built-in jig for specifying new features, subsystems, or significant enhancements.

This spec defines the `feature` jig that ships with kerf. It is the primary jig for new work that requires understanding the problem, breaking it into parts, researching approaches, and producing implementation-level guidance. See [jig-system.md](jig-system.md) for jig file format, resolution, and versioning. See [jig-bug.md](jig-bug.md) for the other built-in jig.

## When To Use

The `feature` jig applies when:

- The work involves designing something new
- Existing behavior is being substantially changed
- The problem must be understood before a solution can be specified
- Multiple components or subsystems are involved

It does not apply to defects (use the `bug` jig) or to changes where the solution is already fully understood and requires no decomposition.

## Frontmatter

The `feature` jig file contains this YAML frontmatter:

```yaml
---
name: feature
description: Full specification process for new features and subsystems
version: 1
status_values:
  - problem-space
  - decomposition
  - research
  - detailed-spec
  - review
  - ready
passes:
  - name: "Problem Space"
    status: problem-space
    output: ["01-problem-space.md"]
  - name: "Decomposition"
    status: decomposition
    output: ["02-components.md"]
  - name: "Research"
    status: research
    output: ["03-research/{component}/findings.md"]
  - name: "Detailed Spec"
    status: detailed-spec
    output: ["04-plans/{component}-spec.md"]
  - name: "Integration & Review"
    status: review
    output: ["05-integration.md", "06-checklist.md", "SPEC.md"]
file_structure:
  - spec.yaml
  - SESSION.md
  - 01-problem-space.md
  - 02-components.md
  - "03-research/{component}/findings.md"
  - "04-plans/{component}-spec.md"
  - 05-integration.md
  - 06-checklist.md
  - SPEC.md
---
```

## Status Progression

```
problem-space -> decomposition -> research -> detailed-spec -> review -> ready
```

The `ready` status indicates the work is complete and available for finalization. Statuses beyond `ready` (e.g., `implementing`, `done`) are orchestrator-defined and outside the jig's scope.

## Passes

### Pass 1: Problem Space (rough cut)

**Status:** `problem-space`
**Output:** `01-problem-space.md`

Pass 1 produces a clear articulation of the problem being solved, the boundaries of the work, and the criteria for success.

#### Agent Instructions

You are clarifying what the user wants to build and why. This is a conversation, not a questionnaire. Aim for 2-3 exchanges with the user.

**What to do:**

1. Read any source material the user has provided or referenced. Understand their starting point before asking questions.
2. Clarify the user's goals and motivations. Ask "what problem does this solve?" and "who benefits?" -- not "what should the API look like?"
3. Define scope boundaries explicitly. State what is in scope and what is explicitly out of scope. If the user hasn't thought about boundaries, propose them and ask for confirmation.
4. Identify constraints -- technical limitations, dependencies on other systems, performance requirements, backwards-compatibility needs, timeline pressure.
5. Capture success criteria. These are concrete, verifiable statements: "a user can do X" or "the system handles Y within Z milliseconds." Reject vague criteria like "it should be fast" or "it should be easy to use" -- push for specifics.

**What "done" looks like:**

`01-problem-space.md` contains:
- A one-paragraph summary of the problem
- Goals (what this work achieves)
- Non-goals (what this work explicitly does not attempt)
- Constraints (technical, business, or timeline)
- Success criteria (concrete, verifiable statements)

The user has confirmed the problem space is accurate. Advance status to `decomposition`.

### Pass 2: Decomposition

**Status:** `decomposition`
**Output:** `02-components.md`

Pass 2 breaks the problem into 3-7 components, each with concrete, testable requirements. Fewer components is better.

#### Agent Instructions

You are turning a problem statement into a structured breakdown. The output is a set of components with requirements -- not an implementation plan.

**What to do:**

1. Read `01-problem-space.md` to ground your decomposition in the agreed problem space.
2. Identify 3-7 components. A component is a cohesive unit of functionality that can be specified and implemented somewhat independently. If you have more than 7, you are decomposing too finely -- group related pieces. If you have fewer than 3, the work may not need the full feature jig.
3. For each component, write concrete, testable requirements. Requirements describe WHAT the component does, not HOW it does it. Each requirement is verifiable -- "returns a 404 with an error body when the resource is not found" not "handles errors gracefully."
4. Identify dependencies between components. Which components must be built before others? Which share interfaces?
5. Identify interfaces between components. Where does data flow from one component to another? What contracts exist at the boundaries?

**What "done" looks like:**

`02-components.md` contains:
- A list of 3-7 named components
- For each component: a one-line description, a list of concrete requirements, and its dependencies on other components
- An interface summary showing data flow and contracts between components

The decomposition is internally consistent and traceable back to the goals and success criteria in `01-problem-space.md`. Advance status to `research`.

### Pass 3: Research

**Status:** `research`
**Output:** `03-research/{component}/findings.md` (one file per component)

Pass 3 investigates technical approaches, existing patterns, and risks for each component. The `{component}` placeholder expands to one directory per component identified in Pass 2.

#### Agent Instructions

You are investigating how each component could be built, what already exists, and what risks are present. You are not making final decisions -- you are gathering information and presenting options.

**What to do:**

1. Read `02-components.md` to understand what each component needs to do.
2. For each component, identify 3-5 research questions. These are the things you need to know before you can write a detailed spec. Examples: "Does the codebase already have a pattern for X?", "What are the performance characteristics of approach Y?", "How does library Z handle edge case W?"
3. Explore the target codebase for existing patterns. Look for code that does something similar, conventions that should be followed, and constraints imposed by the existing architecture.
4. Investigate external dependencies -- APIs, libraries, services. Check version compatibility, licensing, maintenance status.
5. Identify technical risks. What could go wrong? What is uncertain? What requires a prototype or proof of concept?
6. Present findings with options and tradeoffs, not just "the answer." For non-trivial decisions, offer 2-3 approaches with pros and cons.

**What "done" looks like:**

For each component, `03-research/{component}/findings.md` contains:
- Research questions that were investigated
- Findings for each question, with evidence (code references, documentation links, benchmarks)
- Options and tradeoffs for key decisions
- Identified risks and unknowns

All research questions from the component requirements are addressed. No component has unresolved blockers that would prevent writing a detailed spec. Advance status to `detailed-spec`.

### Pass 4: Detailed Spec (fine cut)

**Status:** `detailed-spec`
**Output:** `04-plans/{component}-spec.md` (one file per component)

Pass 4 produces implementation-level specifications for each component, informed by the research findings. The `{component}` placeholder expands to one file per component identified in Pass 2.

#### Agent Instructions

You are writing the spec that an implementing agent will follow. Everything an implementer needs to know goes here. Everything they do not need goes elsewhere.

**What to do:**

1. Read `03-research/{component}/findings.md` for the component you are specifying. Your spec must be consistent with the research findings. If the research identified multiple options, make a decision and record it with rationale.
2. Write architecture decisions. State which approach was chosen and why. Reference the research findings.
3. Provide file-level guidance. Which files are created? Which existing files are modified? What is the purpose of each change?
4. Define interfaces precisely. API signatures, data shapes, type definitions, wire formats. Use the language and conventions of the target codebase.
5. Specify error handling and edge cases. What errors can occur? How is each handled? What are the boundary conditions?
6. Address migration and backwards compatibility. If this changes existing behavior, how does the transition work? Are there breaking changes?

**What "done" looks like:**

For each component, `04-plans/{component}-spec.md` contains:
- Architecture decisions with rationale
- File-level guidance (create, modify, delete)
- Interface definitions (signatures, data shapes, contracts)
- Error handling strategy
- Migration and backwards-compatibility plan (if applicable)

Each spec is concrete enough that an implementing agent can work from it without additional design decisions. Advance status to `review`.

### Pass 5: Integration & Review

**Status:** `review`
**Output:** `05-integration.md`, `06-checklist.md`, `SPEC.md`

Pass 5 assembles the component specs into a coherent whole, validates consistency, and produces the final deliverables.

#### Agent Instructions

You are assembling the work into its final form and checking it for completeness and consistency. This pass produces three files.

**What to do:**

1. Create the integration plan (`05-integration.md`). How do the components connect? What is the order of integration? Are there integration-specific concerns not covered in individual component specs (shared state, initialization order, cross-cutting concerns)?
2. Build the implementation checklist (`06-checklist.md`). This is an ordered task list an implementing agent follows. Each item is a concrete, completable action. Group by component, then order by dependency. Include integration tasks between components.
3. Identify follow-ups. What is blocked on external factors? What is independent and can be done in parallel? What has been deferred to future work? Record these in `06-checklist.md` as a separate section.
4. Assemble the final spec document (`SPEC.md`). This is the single document an implementing agent reads first. It contains:
   - A summary of the work (from `01-problem-space.md`)
   - The component breakdown (from `02-components.md`)
   - Key architecture decisions (from `04-plans/`)
   - The integration plan (from `05-integration.md`)
   - The implementation checklist (from `06-checklist.md`)
5. Review for completeness and consistency. Verify:
   - Every success criterion from `01-problem-space.md` is addressed by at least one component requirement
   - Every component requirement has a corresponding section in the detailed spec
   - Interface definitions between components are consistent (types match, contracts agree)
   - The checklist covers all work described in the specs
   - No contradictions exist between component specs

**What "done" looks like:**

- `05-integration.md` describes how components connect and the integration order
- `06-checklist.md` is an ordered, actionable task list with a follow-ups section
- `SPEC.md` is a self-contained summary that an implementing agent can use as its starting point
- All cross-references are consistent and no gaps remain

Run `kerf square <codename>` to verify structural completeness. When verification passes and the user approves, advance status to `ready`.

## File Structure

A work governed by the `feature` jig contains these files:

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

`spec.yaml` and `SESSION.md` are managed by kerf (see [works.md](works.md) and [sessions.md](sessions.md)). All other files are produced by the passes defined above.

The `{component}` placeholder expands to one entry per component identified during Pass 2. For example, a work with components `parser`, `resolver`, and `emitter` produces:

```
03-research/parser/findings.md
03-research/resolver/findings.md
03-research/emitter/findings.md
04-plans/parser-spec.md
04-plans/resolver-spec.md
04-plans/emitter-spec.md
```

## Finalization

When the work reaches `ready` status and `kerf square` passes, the work is eligible for [finalization](finalization.md). `kerf finalize <codename>` packages the work for implementation handoff.
