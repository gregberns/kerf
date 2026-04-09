# CLI

> Design philosophy, output conventions, agent discovery, and the human-agent handoff protocol.

## The Executable

- **Name:** `kerf`
- **Tagline:** *Measure twice, cut once.*

kerf is a single Go binary. It is a data and workflow management tool. It never launches or manages agent sessions, and it never orchestrates implementation. See [commands.md](commands.md) for individual command specifications.

## Design Principles

### 1. Zero-Context Usability

An agent encountering kerf for the first time — with no documentation, no prior conversation, no CLAUDE.md — is able to use it effectively from `kerf --help` alone. Help text is complete, discoverable, and includes examples. Every command's `--help` states what it does, what it outputs, and what to do next.

### 2. Agent-First Output

Every command's output is structured for consumption by an AI agent. Output includes context, current state, and suggested next actions. Output is also human-readable, but agent consumability is the priority.

### 3. No-Arg Root Command

Running `kerf` with no arguments prints a quick-start guide that gives an agent everything it needs to begin using the tool. This eliminates the need to crawl through `--help` subcommands.

The root command output includes:

- A one-line description of what kerf does
- Available commands with brief descriptions and examples
- The standard workflow: `kerf new` → work through passes → `kerf shelve` / `kerf finalize`
- A bench summary: number of active works, current project (if inside a repo)
- If no bench exists yet, instructions for getting started

This output is the primary agent onboarding surface. It is self-contained — an agent never needs external documentation to use kerf.

### 4. Commands Emit Next Steps

State-changing commands do not just perform mechanical operations — they also emit what to do next.

- `kerf new` emits the jig overview and instructions for the first pass.
- `kerf shelve` emits instructions to write SESSION.md.
- `kerf resume` emits the work's current state and where to pick up.
- `kerf finalize` emits follow-up steps (create PR, notify team).
- `kerf status` (on change) emits the new status and the jig's guidance for the current pass.

The CLI guides the workflow, not just executes it.

### 5. Contextual Output

Read-only commands include not just data but also actionable context.

- `kerf list` shows works and the commands likely needed next (e.g., `kerf resume <codename>`).
- `kerf show` displays metadata, the file tree, jig context, and the current pass description.

## Output Format

CLI output follows these conventions:

### Structure

Output is plain text, organized into labeled sections. Each section has a clear heading or label. Structured data (file trees, status progressions, dependency lists) uses indentation and alignment for readability by both agents and humans.

### Next Steps Block

State-changing commands append a `Next steps:` block at the end of their output. Each step is a concrete instruction — either a kerf command to run or an action to take (e.g., "Write SESSION.md with current state, decisions, and open questions").

### Example: `kerf list` Output

```
On the bench for acme-webapp:
  auth-rewrite     feature   research                     2h ago
  login-timeout    bug       reproducing                  1d ago

  Dependencies: auth-rewrite -> database-migration [decomposition]

Commands:
  kerf show <codename>      View work details
  kerf resume <codename>    Resume working on a work
  kerf new                  Start a new work
```

### Warnings

Warnings are non-fatal. They are emitted inline with output, prefixed with `Warning:`. Examples:

- Setting a status not in the jig's recommended list
- A stale `active_session` detected on a work
- A jig version mismatch between the work and the current jig definition

Warnings never block command execution.

## Agent Discovery

Agents learn about kerf through two mechanisms:

### CLAUDE.md Snippet

Projects using kerf add a snippet to their CLAUDE.md (or equivalent agent configuration file):

```markdown
This project uses `kerf` for spec management. Run `kerf` with no arguments
for a quick-start guide. Use `kerf list` to see active works, `kerf resume
<codename>` to load context for an in-progress work.
```

### Root Command

If no CLAUDE.md exists, the human tells the agent to use kerf. The agent runs `kerf` with no arguments and reads the quick-start output. This is sufficient for full tool discovery — no external documentation is required.

## Human-Agent Handoff Protocol

kerf mediates the handoff between humans and agents across sessions. The protocol has six phases:

### 1. Human Initiates

The human runs `kerf new` (or tells the agent to). kerf creates the work, outputs the jig overview and first-pass instructions.

### 2. Agent Works

The agent follows jig instructions, writing artifacts directly to the work directory on the bench. It uses `kerf status` to advance through passes. The agent reads the jig's guidance for each pass from kerf's output.

### 3. Human Steers

The human can redirect at any time — "skip the research pass, we already know the approach." The agent uses `kerf status <codename> <new-status>` to jump ahead. Passes are guidance, not gates.

### 4. Shelving

When a session ends (planned or unplanned), `kerf shelve` performs bookkeeping (snapshot, session end marker) and emits instructions for the agent to write SESSION.md summarizing current state, decisions made, open questions, and next steps.

If a session terminates unexpectedly, `kerf shelve` may never run and SESSION.md may not be written. The raw artifacts and spec.yaml still provide enough context for a future resume, but without the agent's interpreted state summary.

### 5. Resuming

The human starts a new agent session and runs `kerf resume <codename>` (or tells the agent to). kerf outputs the work's full context: SESSION.md contents, current pass, jig instructions, session history, and open questions. The agent reads this output and orients.

kerf does not launch an agent session. The human starts the session; the agent (or human) runs `kerf resume` to load context.

### 6. Finalizing

The human reviews the spec (via `kerf show` or by reading files directly), then tells the agent to run `kerf finalize`. Finalization is not unilateral — the human decides when a spec is ready. See [finalization.md](finalization.md) for the finalization process.

## What kerf Is Not

- **Not a project management tool.** No sprints, story points, or burndown charts.
- **Not an orchestrator.** kerf does not schedule or manage implementation agents.
- **Not a database.** The filesystem is the source of truth.
- **Not a git replacement.** Works enter git only at finalization.
- **Not prescriptive about methodology.** Jigs are opinionated defaults, not enforced processes.
