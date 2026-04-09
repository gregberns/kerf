# Future Work

Items explicitly out of scope for v1, captured here with intent and context so future work can build on these decisions rather than re-deriving them.

---

## Sync & Sharing

### Intent
Allow multiple team members to work with a shared bench. Engineers can see each other's in-progress works, read dependent works, and coordinate without manual file sharing.

### Context
- Solo devs need backup even before they need sharing
- The simplest sync is just "copy files to a server"
- Conflict resolution is the hard part — two people modifying the same work simultaneously
- Options explored: git-based sync (ugly for this use case), CRDT (complex but correct), last-write-wins (simple but lossy), Dolt (versioned database, probably overkill)
- A `kerf sync` command could push/pull from a remote, or a file watcher could sync continuously

### Design Constraints
- Must work with the existing file-based data model (no migration to a database)
- Must not require running a persistent server for the basic case
- Conflict resolution strategy should be explicit and understandable

### Likely Approach
Start with `kerf sync --remote <url>` that does a simple file sync (rsync-like). Add conflict detection (warn, don't auto-resolve). Graduate to CRDT or OT if concurrent editing becomes a real need.

---

## Server Mode

### Intent
Run a server that serves the bench over HTTP. Enables: web UI for PMs to read works, API access for orchestrators, remote agent access.

### Context
- `kerf serve --port 8080` or similar
- Read-only initially — just serving the filesystem state
- Eventually: write API for remote agents to create/update works
- Auth and access control become necessary here

### Design Constraints
- The CLI should remain fully functional without the server
- Server is an optional layer, not a replacement for the filesystem
- API should mirror CLI commands (same operations, same semantics)

### Likely Approach
A Go HTTP server embedded in the same binary. Serves JSON API + simple web UI. Reads from the same `~/.kerf/` directory. No separate database.

---

## Web UI

### Intent
A browser-based read-through interface for non-engineer stakeholders (PMs, designers) to browse works, see status, read documents.

### Context
- PMs shouldn't need CLI access to know what's being specced
- Read-only is fine initially
- Could be a static site generated from the work files, or served dynamically by the server
- Should render markdown nicely, show status progression, display dependency graphs

### Design Constraints
- Must not be required for the core workflow
- Should work with the existing file format (no special markup)

---

## External System Integration

### Intent
Link works to tickets in Jira, Linear, GitHub Issues, etc. for organizations that need to maintain those systems for compliance, reporting, or process reasons.

### Context
- `spec.yaml` already has an `external` field stubbed out
- Integration could be bidirectional: create a Jira ticket from a work, or link an existing ticket
- Status sync (work status -> Jira status) is desirable but complex
- This is a "later" concern — the tool should be useful without any external integrations

### Likely Approach
`kerf link <codename> --jira PROJ-1234` stores the reference. A future `kerf sync-external` could push status updates. Start with just storing references, add sync later.

---

## Orchestrator Integration

### Intent
Make it trivially easy for an external orchestrator to use this tool. The orchestrator picks up finalized works, manages implementation agents, handles failures, and updates work status.

### Context
- Explicitly NOT building an orchestrator into this tool
- The orchestrator needs to: list ready works, read work contents, update status, report implementation results
- A short jig file should be enough to teach an orchestrator agent how to interact with `kerf`
- The CLI's agent-friendly output is already designed for this

### Design Constraints
- `kerf` is a passive tool — it doesn't initiate anything
- The orchestrator drives the workflow; `kerf` just stores state
- Status field is open-ended specifically to support orchestrator-defined statuses

### Likely Approach
Document the orchestrator interaction pattern. Provide a sample jig file. The CLI commands (`list --status ready`, `show <codename>`, `status <codename> implementing`) are the integration surface.

---

## Jig Marketplace / Team Jigs

### Intent
Teams define shared jigs that encode their specific processes. A team lead creates a "team-feature" jig with the team's conventions, and all team members use it.

### Context
- `kerf jig sync` is stubbed in the CLI design
- Could pull from a git repo, a URL, or the server
- Versioning of jigs matters — you don't want a jig change to break in-progress works
- The jig version is recorded in spec.yaml at creation time; kerf can detect mismatches and alert the agent

### Design Constraints
- Security concern: loading jigs from untrusted sources could inject malicious agent instructions
- Need explicit user consent for jig loading (addressed in security discussion)
- Jigs should be inspectable (plain markdown, not compiled/obfuscated)

---

## Work Dependencies Across Projects

### Intent
Works in different repositories can depend on each other. Useful for microservice architectures where a feature spans multiple repos.

### Context
- Current `depends_on` uses codename within a project
- Cross-project dependencies need a project identifier in the reference
- This interacts with sync/sharing — you need access to the other project's works
- Adds complexity to dependency resolution and status checking

### Likely Approach
Extend `depends_on` with a `project` field. Cross-project dependencies are informational only (no blocking) unless both projects are on the same bench.

---

## Auto-Versioning Improvements

### Intent
Make snapshots smarter — deduplicated, diffable, named, tagged.

### Context
- v1 snapshots are full copies (simple, correct, wasteful)
- Could use hard links for unchanged files
- Could store diffs instead of full copies
- Named snapshots ("before-research", "post-review") in addition to timestamps
- Integration with snapshot browsing / rollback

### Likely Approach
Keep full-copy snapshots for v1. Add named snapshots as a CLI feature. Optimize storage later if it becomes a real problem (it probably won't for text files).

---

## Multi-Language Jig Instructions

### Intent
Jigs could include instructions optimized for different LLMs or agent frameworks, not just Claude.

### Context
- Current design assumes Claude Code as the agent
- Other agent frameworks (Cursor, Copilot, custom) may have different interaction patterns
- The jig format could include multiple instruction sets keyed by agent type

### Design Constraints
- Don't over-engineer this. Claude Code is the primary target.
- If other agents can read markdown and follow instructions, the same jigs may work anyway.
