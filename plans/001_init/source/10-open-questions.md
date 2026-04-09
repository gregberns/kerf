# Open Questions

Questions that surfaced during design but don't have answers yet. These need to be resolved before or during implementation.

---

## ~~Security: Jig Loading~~

**Deferred.** Not critical for v1. Revisit before external jig loading is implemented.

---

## ~~Session ID Discovery~~

**Resolved.** Session ID recording is best-effort. When the human launches Claude with `claude --session-id <uuid>`, kerf can record that UUID in spec.yaml. When the agent is already running and the session ID isn't discoverable, kerf records the session without an ID. The primary resumability mechanism is SESSION.md and work artifacts, not the session ID. Kerf does not launch or manage Claude sessions.

---

## ~~Project Identity & Repo Stability~~

**Resolved.**

- **Project ID format:** `user-repo` slug derived from git remote origin (e.g., `acme-webapp`)
- **Fallback:** Directory name if no remote is configured
- **User override:** User can define or change the project ID at any time
- **In-repo:** `.kerf/project-identifier` file in repo root (committed to git) contains the project ID
- **Bench structure:** `~/.kerf/projects/{project-id}/` holds all works for that project
- **Collisions:** If a derived ID matches an existing project with a different origin, warn the user and let them resolve
- **Worktrees:** Work automatically — `.kerf/project-identifier` is committed, so all checkouts of the same repo see the same project ID
- **Edge cases:** Don't over-engineer. Support manual fixing if things drift.
- **Monorepos:** Multiple logical projects in one repo derive the same project ID from the same git remote. For v1, monorepo users can manually set different project IDs by editing `.kerf/project-identifier`. A more ergonomic solution (e.g., subdirectory-scoped project IDs) can be added later if needed.

---

## ~~Concurrent Work Modification~~

**Deferred.** Don't handle in v1. Document that each work should have one active session at a time. The `active_session` field in spec.yaml serves as a soft lock. Future: CRDTs for tracking changes over time.

---

## Finalization: Where Do Works Land in the Repo?

**Resolved.** Default to `.kerf/{codename}/` in the repo root. Configurable.

---

## Finalization: Branch Strategy

**Resolved.** Branch from the default branch (main/master). The agent names the branch based on its context of the work (not the codename). Create an initial commit with the work artifacts. Refuse to finalize if the target repo has uncommitted changes.

---

## ~~Jig Versioning in Works~~

**Deferred.** Record something about the jig version in spec.yaml (name, hash, semver — TBD). Detect mismatches and alert the agent. Details to be refined during implementation.

---

## ~~Agent Instructions Delivery~~

**Deferred.** Natural language by default. Machine-readable output tension to be resolved during implementation when we have concrete examples.

---

## Cross-Project Work Discovery

**Question:** How does an agent working on work A discover and read work B in a different project?

**Resolved (basic approach).** Add a `--project` flag to relevant commands, defaulting to the current project. An agent can look up works in other projects via `kerf show <codename> --project <project-id>`.

**Key constraint:** The `~/.kerf/` bench must store projects separately by project ID so cross-project lookup works. See "Project Identity & Repo Stability" above.

---

## Work Codename Generation

**Question:** When creating new work, we may not know what to name it yet.

**Resolved.** Auto-generate a slug using an `adjective-noun` pattern (e.g., `blue-bear`, `swift-maple`). This becomes the codename used for the work directory and references. Codenames are immutable once created (they're used as directory names, dependency references, and session associations). A separate mutable `title` field provides human-friendly naming.

---

## ~~Naming~~

**Resolved.** See `09-naming.md`. The tool is called **kerf**. Jigs for templates, passes for phases, bench for workspace, square for verification.
