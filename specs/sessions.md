# Sessions

> Session tracking, shelving, resuming, and the SESSION.md format.

## Overview

A **session** links a [work](works.md) to an agent conversation. kerf records sessions in `spec.yaml` to maintain history and enable resumability across agent conversations. kerf does not launch or manage agent sessions — it records metadata and emits context.

## Session Tracking in spec.yaml

Each work's `spec.yaml` contains two session-related fields: `sessions` (a list of all sessions) and `active_session` (a quick-lookup pointer to the current session).

### `sessions` List

The `sessions` field is an ordered list of session entries. Each entry has the following schema:

```yaml
sessions:
  - id: <uuid | null>        # session ID, or null if not available
    started: <RFC 3339>       # timestamp when kerf recorded the session
    ended: <RFC 3339 | null>  # timestamp when the session ended, or null if active
    notes: <string | null>    # optional free-text summary
```

kerf appends a new entry to `sessions` when:

- `kerf new` creates a work (the first session)
- `kerf resume` loads context for a work

kerf sets the `ended` timestamp on the active session entry when:

- `kerf shelve` pauses the work
- `kerf shelve --force` clears a stale session

The `notes` field is not set by kerf. It is available for agents or orchestrators to annotate via direct `spec.yaml` edits.

### `active_session` Field

The `active_session` field holds the `id` value of the currently active session entry, or `null` when no session is active.

```yaml
active_session: 5829f3a1-357e-4ee7-92b6-fff4a0e93251
```

When the active session has no `id` (recorded without a session ID), `active_session` is set to the string `"anonymous"`.

`kerf shelve` sets `active_session` to `null`.

`kerf resume` sets `active_session` to the new session's `id` (or `"anonymous"`).

When `codename` is omitted from `kerf shelve`, kerf uses `active_session` to identify which work in the current project to shelve. If no work has a non-null `active_session`, kerf errors. If multiple works in the same project have a non-null `active_session`, kerf errors with a message listing the ambiguous works.

## Session ID Recording

Session ID recording is best-effort. The session ID is a UUID provided by the agent runtime (e.g., via `claude --session-id <uuid>`).

- When a session ID is available, kerf records it in the `id` field of the session entry and in `active_session`.
- When a session ID is not available (e.g., the agent is already running and the ID is not discoverable), kerf records the session with `id: null` and sets `active_session` to `"anonymous"`.

The session ID is not the primary resumability mechanism. SESSION.md and the work's artifacts provide the context needed to resume. The session ID exists for history and traceability.

## Stale Session Detection

A session is **stale** when it has a non-null `active_session`, a null `ended` timestamp, and the `started` timestamp is older than the configured threshold.

The stale threshold defaults to 24 hours. It is configurable in `~/.kerf/config.yaml`:

```yaml
# ~/.kerf/config.yaml
sessions:
  stale_threshold_hours: 24
```

### Behavior on Stale Detection

kerf checks for stale sessions on every command invocation that reads `spec.yaml` for the affected work. When a stale session is detected, kerf emits a warning:

```
Warning: active session started 2026-04-06T10:00:00Z appears stale
(threshold: 24h). The previous session may have ended without running
`kerf shelve`. Run `kerf shelve --force <codename>` to clear it.
```

kerf does not automatically clear stale sessions. The user or agent must run `kerf shelve --force` to mark the stale session as ended and clear `active_session`.

`kerf resume` refuses to create a new session when `active_session` is non-null. It directs the user to shelve or force-clear the existing session first.

## Shelving

`kerf shelve` pauses work on a session with state preservation. See [commands.md](commands.md) for argument syntax.

### Shelve Sequence

1. kerf identifies the target work (from the provided codename or inferred from `active_session` in the current project).
2. kerf takes a [snapshot](snapshots.md) of the current work state.
3. kerf sets the `ended` timestamp on the active session entry in `spec.yaml` to the current time.
4. kerf sets `active_session` to `null`.
5. kerf updates the `updated` timestamp in `spec.yaml`.
6. kerf emits instructions directing the agent to write SESSION.md.

### Agent Instructions on Shelve

After completing the mechanical steps, kerf emits:

```
Work <codename> shelved.

Before ending this session, write SESSION.md in the work directory with:
- Current pass and progress within it
- Decisions made during this session
- Open questions
- Suggested next steps
- Reading order for a new session picking this up

Path: ~/.kerf/projects/<project-id>/<codename>/SESSION.md
```

The agent is responsible for writing SESSION.md. kerf does not write it. See [SESSION.md Format](#sessionmd-format) below for the expected structure.

### Force Shelve

`kerf shelve --force <codename>` clears a stale or orphaned `active_session`:

1. kerf sets the `ended` timestamp on the active session entry to the current time.
2. kerf sets `active_session` to `null`.
3. kerf updates the `updated` timestamp in `spec.yaml`.
4. kerf takes a [snapshot](snapshots.md).
5. kerf does not emit SESSION.md instructions (the original agent is no longer present).

## Resuming

`kerf resume` loads context for continuing work. See [commands.md](commands.md) for argument syntax.

### Resume Sequence

1. kerf reads `spec.yaml` for the target work.
2. If `active_session` is non-null, kerf errors (see [Stale Session Detection](#stale-session-detection)).
3. kerf records a new session entry in `sessions` with the current timestamp and `ended: null`.
4. kerf sets `active_session` to the new session's `id` (or `"anonymous"`).
5. kerf updates the `updated` timestamp in `spec.yaml`.
6. kerf takes a [snapshot](snapshots.md) of the current state.
7. kerf emits the resume context (see below).

### Resume Context Output

kerf emits a context block containing:

- **Work metadata**: codename, title, type, status, project ID
- **SESSION.md contents**: the full text of SESSION.md, if present
- **Current pass**: the jig pass corresponding to the current status, with the jig's agent instructions for that pass (see [jig-system.md](jig-system.md))
- **Session history**: the `sessions` list from `spec.yaml` (previous sessions, not the newly created one)
- **Dependency status**: current status of each work listed in `depends_on` (see [dependencies.md](dependencies.md))
- **File listing**: the files present in the work directory
- **Next steps**: suggested actions based on the current pass and SESSION.md content

### Degraded Mode

When SESSION.md is missing (e.g., the previous session terminated without running `kerf shelve`, or the agent did not write SESSION.md), kerf operates in degraded mode:

- kerf emits a notice: `SESSION.md not found — resuming without interpreted session state.`
- kerf substitutes a context summary assembled from `spec.yaml` and the work's existing artifact files: status, session history, file listing, and current pass instructions.
- The agent can continue working, but lacks the interpreted state (decisions, open questions, next steps) that SESSION.md would have provided.

## SESSION.md Format

SESSION.md is a markdown file written by the agent. It lives at the root of the work directory alongside `spec.yaml`. kerf reads SESSION.md during `kerf resume` and `kerf show` but never writes it.

### Purpose

SESSION.md captures interpreted session state that cannot be derived from the raw artifacts alone: what the agent was doing, what decisions were made, what questions remain, and what a new session should do first.

### Template

```markdown
# Session State

## Current Pass
<pass name> — <progress description>

## Decisions Made
- <decision and reference to relevant artifact>
- <decision and reference to relevant artifact>

## Open Questions
- <question>
- <question>

## Next Steps
- <actionable next step>
- <actionable next step>

## Context for New Sessions
If starting a fresh session (not resuming), read these files first:
1. spec.yaml — current status and metadata
2. This file — SESSION.md
3. <artifact file> — <why>
4. <artifact file> — <why>
```

### Sections

| Section | Required | Description |
|---------|----------|-------------|
| **Current Pass** | Yes | The jig pass the agent was working on and progress within it. |
| **Decisions Made** | Yes | Key decisions from the session, each referencing the artifact where the decision is recorded. |
| **Open Questions** | No | Unresolved questions that the next session should address. |
| **Next Steps** | Yes | Concrete actions for the next session, in priority order. |
| **Context for New Sessions** | Yes | Ordered reading list of files a new agent should read to get oriented. Always starts with `spec.yaml` and `SESSION.md`. |

### Authorship and Timing

- The agent writes SESSION.md, prompted by `kerf shelve`'s output instructions.
- The agent may update SESSION.md at any time during a session (not only at shelve time).
- Each session overwrites the previous SESSION.md. Prior versions are preserved in [snapshots](snapshots.md).

## Example

A work `auth-rewrite` after two sessions, with the second session active:

```yaml
# spec.yaml (session-related fields only)
sessions:
  - id: 39142ac7-b54e-4726-bbb0-a6d41dfe9fba
    started: 2026-04-07T10:00:00Z
    ended: 2026-04-07T16:30:00Z
    notes: "Completed problem space and decomposition"
  - id: 5829f3a1-357e-4ee7-92b6-fff4a0e93251
    started: 2026-04-08T09:00:00Z
    ended: null
    notes: "Research pass, 3 of 5 components done"

active_session: 5829f3a1-357e-4ee7-92b6-fff4a0e93251
```

```markdown
<!-- SESSION.md -->
# Session State

## Current Pass
Research — component 3 of 5 (notification-service)

## Decisions Made
- Using event-driven architecture for auth events (see 04-plans/auth-events-spec.md)
- Rejected OAuth proxy approach due to latency concerns (see 03-research/auth-flow/findings.md)

## Open Questions
- Need to determine if existing session store can handle new token format
- Waiting on database-migration work for schema decisions

## Next Steps
- Complete research for notification-service and user-preferences components
- Begin detailed spec for auth-flow component (research complete)

## Context for New Sessions
If starting a fresh session (not resuming), read these files first:
1. spec.yaml — current status and metadata
2. This file — SESSION.md
3. 02-components.md — the decomposition
4. 03-research/ — completed research
```
