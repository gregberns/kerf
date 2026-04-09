# Future Work

> Items explicitly out of v1 scope, preserved with enough context that future plans can build on them without re-deriving decisions.

Nothing in this document is spec'd. These are **deferred items** — captured intent, constraints, and likely approaches for work that will be planned and spec'd independently when the time comes. Each item here should eventually become a plan in `plans/` before any code is written.

---

## Sync & Sharing

### Intent

Allow multiple team members to work with a shared bench. Engineers see each other's in-progress works, read dependent works, and coordinate without manual file sharing.

### Design Constraints

- Must work with the existing file-based data model — no migration to a database.
- Must not require running a persistent server for the basic case (solo devs need backup before they need sharing).
- Conflict resolution strategy must be explicit and understandable. Two people modifying the same work simultaneously is the hard problem.

### Likely Approach

Start with `kerf sync --remote <url>` that does simple file sync (rsync-like). Add conflict detection that warns but does not auto-resolve. Graduate to CRDT or OT only if concurrent editing becomes a demonstrated need. Explored and rejected: git-based sync (poor fit for this data model), Dolt (overkill), last-write-wins (too lossy).

---

## Server Mode

### Intent

Run an HTTP server (`kerf serve`) that exposes the bench over an API. Enables: web UI for stakeholders, API access for orchestrators, remote agent access.

### Design Constraints

- The CLI remains fully functional without the server — server is an optional layer.
- API mirrors CLI commands: same operations, same semantics.
- Read-only initially; write API for remote agents comes later.
- Auth and access control become necessary once the server exists.

### Likely Approach

A Go HTTP server embedded in the same `kerf` binary. Serves a JSON API alongside a simple web UI. Reads from the same `~/.kerf/` directory. No separate database — the filesystem remains the source of truth.

---

## Web UI

### Intent

A browser-based read-through interface for non-engineer stakeholders (PMs, designers) to browse works, see status, and read documents without CLI access.

### Design Constraints

- Must not be required for the core workflow — purely additive.
- Must work with the existing file format; no special markup required in work documents.
- Should render markdown, show status progression, and display dependency graphs.

### Likely Approach

Served dynamically by server mode, or generated as a static site from work files. Depends on server mode being implemented first.

---

## External System Integration

### Intent

Link works to tickets in external systems (Jira, Linear, GitHub Issues) for organizations that need those systems for compliance, reporting, or process.

### Design Constraints

- The tool must be fully useful without any external integrations.
- `spec.yaml` already has an `external` field stubbed for this purpose.
- Bidirectional sync (work status to/from ticket status) is desirable but complex — defer full sync until linking is proven.

### Likely Approach

Start with reference storage: `kerf link <codename> --jira PROJ-1234` writes to the `external` field in `spec.yaml`. A future `kerf sync-external` command could push status updates. Build linking first, sync second.

---

## Orchestrator Integration

### Intent

Make it trivially easy for an external orchestrator to use kerf. The orchestrator picks up finalized works, manages implementation agents, handles failures, and updates work status.

### Design Constraints

- kerf is a passive tool — it never initiates agent sessions or manages execution.
- The orchestrator drives the workflow; kerf stores state and emits context.
- The open-ended status field exists specifically to support orchestrator-defined statuses.
- A short jig file should be sufficient to teach an orchestrator agent how to interact with kerf.

### Likely Approach

Document the orchestrator interaction pattern. Provide a sample jig. The existing CLI commands are the integration surface: `kerf list --status ready`, `kerf show <codename>`, `kerf status <codename> implementing`. The agent-first CLI output is already designed with this use case in mind.

---

## Jig Marketplace / Team Jigs

### Intent

Teams define shared jigs encoding their specific processes. A team lead creates a "team-feature" jig with team conventions, and all members use it via `kerf jig sync`.

### Design Constraints

- **Security**: loading jigs from untrusted sources could inject malicious agent instructions. Explicit user consent is required for jig loading.
- Jigs must be inspectable — plain markdown, not compiled or obfuscated.
- Jig versioning matters: a jig change must not break in-progress works. The jig version recorded in `spec.yaml` at creation time allows mismatch detection.

### Likely Approach

`kerf jig sync` pulls from a git repo, URL, or server. Jig version is pinned at work creation. kerf detects version mismatches between the jig used to start a work and the currently available version, alerting the agent.

---

## Work Dependencies Across Projects

### Intent

Works in different repositories can depend on each other — useful for microservice architectures where a feature spans multiple repos.

### Design Constraints

- Current `depends_on` uses codenames within a single project.
- Cross-project dependencies need a project identifier in the reference.
- Interacts with sync/sharing — you need access to the other project's works.
- Adds complexity to dependency resolution and status checking.

### Likely Approach

Extend `depends_on` with a `project` field. Cross-project dependencies are informational only (no blocking) unless both projects are on the same bench.

---

## Auto-Versioning Improvements

### Intent

Make snapshots smarter — deduplicated, diffable, named, tagged.

### Design Constraints

- v1 snapshots are full copies (simple and correct, but wasteful at scale).
- Any optimization must not break the existing snapshot format or make snapshots harder to read directly from the filesystem.

### Likely Approach

Add named snapshots ("before-research", "post-review") as a CLI feature alongside timestamp-based snapshots. Optimize storage later via hard links for unchanged files or diff-based storage — but only if size becomes a demonstrated problem (unlikely for text files). Keep full-copy as the default for simplicity and debuggability.

---

## Multi-Language Jig Instructions

### Intent

Jigs could include instructions optimized for different LLMs or agent frameworks, not just Claude Code.

### Design Constraints

- Claude Code is the primary target. Do not over-engineer for hypothetical agents.
- If other agents can read markdown and follow instructions, the same jigs may already work without modification.

### Likely Approach

If needed, the jig format could include multiple instruction sets keyed by agent type. Defer until there is concrete demand from a second agent framework. The markdown-based jig format is already relatively agent-agnostic.
