# Problem Statement

## The Shift

Software development is undergoing a paradigm shift. Engineers are increasingly working with AI agents to produce code. The emerging workflow is: humans design and specify, agents implement. This changes what engineers spend their time on — away from writing code, toward writing specifications.

But the tooling hasn't caught up.

## The Core Problem

**Managing the spec-writing process is painful.**

Writing a thorough specification for a non-trivial feature can take 1-2 days of focused work with an agent. During this time, the engineer needs to:

1. **Have a long-running conversation** with an agent about the codebase, iterating through problem definition, decomposition, research, and detailed planning.
2. **Persist evolving artifacts** — the spec documents being produced — without losing work.
3. **Juggle multiple specs in parallel** — a solo dev might have 4 spec-writing sessions active at once, plus 6 implementation agents running.
4. **Shelve and resume** — interrupt a spec conversation, come back hours or days later, and pick up where they left off.
5. **Hand off completed specs** to an implementation process (human or automated).

## Why Existing Tools Fail

### Git is too heavy for draft specs

Specs evolve rapidly during the writing process. Git requires commits, branches, merges, and PRs — ceremony that makes no sense for work-in-progress documents. You don't want to commit every paragraph change. You don't want a branch for "I'm thinking about auth."

Worse: if specs live in git branches, you need worktrees to work on multiple specs simultaneously. Each worktree needs its own editor. Managing 4 worktrees for 4 parallel specs is operationally painful.

And if a spec is on a branch, it needs to be merged to main before an implementation agent can work from it — adding another layer of process to what should be a fluid creative activity.

### Jira/Linear are structured wrong

Project management tools manage *tickets*, not *structured knowledge*. A Jira ticket has a title, description (free text), status, and assignee. This is not a spec. Specs are multi-document, phased, structured artifacts with internal cross-references, research findings, architecture decisions, and detailed implementation guidance.

Jira's "description" field cannot hold a spec. Confluence is closer but is disconnected from the workflow — there's no status-driven progression, no agent integration, no structured phases.

More fundamentally: these tools were designed for humans to read. Agents need structured, machine-consumable context to produce good implementations. A Jira ticket gives an agent almost nothing to work with.

### The worktree problem

When using Claude Code (or similar tools) to write specs, the agent needs access to the codebase to understand what exists. If the spec lives in the codebase (on a branch), you need a worktree. But:

- Each worktree = another editor instance
- Specs in one worktree aren't visible from another
- Multiple engineers can't easily see each other's in-progress specs
- The spec is tied to a branch that may conflict with other work

### No resumability

Long-running spec conversations get interrupted. Meetings, end of day, context switches. When you come back, you either:
- Try to continue a stale conversation (if the tool supports it)
- Start fresh and lose all the nuanced discussion that led to decisions
- Manually re-read all artifacts and try to reconstruct context

None of these are good.

## Who Has This Problem

### Solo developer
A single engineer juggling 4 spec-writing conversations and 6 implementation agents. They need to context-switch between specs efficiently, persist work automatically, and hand off completed specs to implementation without manual file shuffling.

### Small team (3-10 engineers)
Engineers primarily writing specs, with implementation handled by agents (possibly on remote infrastructure, not their dev machines). They need:
- Visibility into what specs are in progress and who's working on them
- The ability to read each other's in-progress specs (especially when specs have dependencies)
- A shared understanding of what's ready for implementation vs. still being designed
- Backup and sync of spec artifacts

### The emerging role split
There's a strong signal that team dynamics are changing. Some engineers may focus on the "front side" — spec writing, design, research. Others may focus on the "back side" — validation, review, orchestrating implementation. The tooling should support both without forcing a particular organizational structure.

## What Success Looks Like

An engineer can:
1. Start a new spec conversation with an agent, with the agent having full access to the codebase
2. Work through a structured process (problem space, decomposition, research, detailed spec)
3. Walk away at any point, come back later, and resume seamlessly
4. Have multiple specs in flight simultaneously without managing branches or worktrees
5. Mark a spec as complete and trigger a clean handoff to implementation
6. See all their specs (and their team's specs) with current status at a glance

All spec artifacts are automatically persisted and versioned as they're written — no manual saves, no commits, no ceremony.
