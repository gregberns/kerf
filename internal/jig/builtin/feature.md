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

# Feature Specification Jig

## Overview

This jig guides you through a structured process for specifying a new feature or subsystem. It produces implementation-ready specifications through five passes: understanding the problem, decomposing it into components, researching approaches, writing detailed specs, and integrating everything into a final deliverable.

Each pass produces one or more files. If work is not captured in a file, it is lost when the session ends.

## Pass 1: Problem Space (rough cut)

**Goal:** Clarify goals, scope, and constraints through 2-3 conversational exchanges with the user.

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

## Pass 2: Decomposition

**Goal:** Break the project into 3-7 components and define concrete, testable requirements for each.

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

## Pass 3: Research

**Goal:** Investigate technical approaches, existing patterns, and risks for each component.

You are investigating how each component could be built, what already exists, and what risks are present. You are not making final decisions -- you are gathering information and presenting options.

**What to do:**

1. Read `02-components.md` to understand what each component needs to do.
2. For each component, identify 3-5 research questions. These are the things you need to know before you can write a detailed spec.
3. Explore the target codebase for existing patterns. Look for code that does something similar, conventions that should be followed, and constraints imposed by the existing architecture.
4. Investigate external dependencies -- APIs, libraries, services. Check version compatibility, licensing, maintenance status.
5. Identify technical risks. What could go wrong? What is uncertain? What requires a prototype or proof of concept?
6. Present findings with options and tradeoffs, not just "the answer."

**What "done" looks like:**

For each component, `03-research/{component}/findings.md` contains:
- Research questions that were investigated
- Findings for each question, with evidence
- Options and tradeoffs for key decisions
- Identified risks and unknowns

All research questions are addressed. No component has unresolved blockers. Advance status to `detailed-spec`.

## Pass 4: Detailed Spec (fine cut)

**Goal:** Write implementation-level specifications for each component.

You are writing the spec that an implementing agent will follow. Everything an implementer needs to know goes here.

**What to do:**

1. Read `03-research/{component}/findings.md` for the component you are specifying.
2. Write architecture decisions. State which approach was chosen and why. Reference the research findings.
3. Provide file-level guidance. Which files are created? Which existing files are modified?
4. Define interfaces precisely. API signatures, data shapes, type definitions, wire formats.
5. Specify error handling and edge cases.
6. Address migration and backwards compatibility.

**What "done" looks like:**

For each component, `04-plans/{component}-spec.md` contains:
- Architecture decisions with rationale
- File-level guidance (create, modify, delete)
- Interface definitions (signatures, data shapes, contracts)
- Error handling strategy
- Migration and backwards-compatibility plan (if applicable)

Each spec is concrete enough that an implementing agent can work from it without additional design decisions. Advance status to `review`.

## Pass 5: Integration & Review

**Goal:** Assemble component specs into a coherent whole, validate consistency, produce final deliverables.

You are assembling the work into its final form and checking it for completeness and consistency.

**What to do:**

1. Create the integration plan (`05-integration.md`). How do the components connect? What is the order of integration?
2. Build the implementation checklist (`06-checklist.md`). An ordered task list an implementing agent follows.
3. Identify follow-ups and record them in `06-checklist.md` as a separate section.
4. Assemble the final spec document (`SPEC.md`). The single document an implementing agent reads first.
5. Review for completeness and consistency.

**What "done" looks like:**

- `05-integration.md` describes how components connect and the integration order
- `06-checklist.md` is an ordered, actionable task list with a follow-ups section
- `SPEC.md` is a self-contained summary that an implementing agent can use as its starting point
- All cross-references are consistent and no gaps remain

Run `kerf square <codename>` to verify structural completeness. When verification passes and the user approves, advance status to `ready`.

## Finalization

When this work moves to `ready`, run `kerf square <codename>` to verify, then `kerf finalize <codename>` to package it for implementation.
