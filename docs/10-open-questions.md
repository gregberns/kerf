# Open Questions

Questions that surfaced during design but don't have answers yet. These need to be resolved before or during implementation.

---

## Security: Jig Loading

**Question:** How do we handle loading jigs from external sources without introducing security risks?

**Context:** Jigs contain agent instructions — prompts that tell an AI what to do. A malicious jig could instruct an agent to exfiltrate code, modify files destructively, or leak sensitive information. This is a real attack vector in the agent ecosystem, and users are rightly paranoid about it.

**Considerations:**
- Jigs should be inspectable (plain markdown, human-readable)
- Loading a jig should require explicit user action (never automatic/silent)
- Should there be a "trusted sources" concept?
- Should jigs be sandboxed somehow? (Probably not feasible — the whole point is that they instruct an agent)
- At minimum: show the user the jig content before activating it, require confirmation

**Deferred to:** Discussion before jig loading is implemented.

---

## Session ID Discovery

**Question:** How does the CLI discover the current Claude session ID to record in spec.yaml?

**Context:** Claude Code doesn't expose the session ID via environment variables (checked: no `CLAUDE_SESSION_ID` etc.). Sessions are stored as `{uuid}.jsonl` files in `~/.claude/projects/{path}/`. Options:
- Parse the Claude session directory to find the most recent session
- Use `claude --session-id <uuid>` to set a known ID when launching
- Use `claude --name <name>` and then look up by name
- Ask the agent to self-report (unreliable)

**Best approach so far:** When `kerf new` or `kerf resume` launches a Claude session, it generates or looks up a UUID and passes it via `--session-id <uuid>`. This way the CLI controls the session ID and can reliably record it.

**Needs:** Verification that `--session-id` works as expected with `--resume`.

---

## Repo Identifier Stability

**Question:** How do we handle repo moves/renames without losing work associations?

**Context:** Path-based identifiers (`github-myapp`) break when the repo directory is moved. Git-remote-based identifiers survive moves but require a remote to be configured and can change if the remote changes.

**Options:**
- Store the repo path in spec.yaml AND derive the project directory name from the path. If the path changes, provide a `kerf migrate` command to re-associate.
- Use git-remote-based identifiers as primary, path-based as fallback.
- Store both, use path for lookup but remote for identity.

**Deferred to:** Implementation. Path-based is fine for v1; add migration tooling if it becomes a real pain point.

---

## Concurrent Work Modification

**Question:** What happens if two agents/sessions modify the same work simultaneously?

**Context:** In the solo-dev use case this is unlikely but possible (e.g., you resume a work in one terminal while another terminal's agent is still writing to it). In the team case it's more likely.

**Options:**
- Lock file while a session is active (simple, may cause issues if sessions crash)
- Last-write-wins (simple, may lose data)
- Detect conflicts on write (compare against last-known state, warn if changed)
- Don't handle it in v1 (document the limitation)

**Recommendation:** For v1, don't handle it. Document that each work should have one active session at a time. The `active_session` field in spec.yaml serves as a soft lock — the CLI can warn if another session is recorded as active.

---

## Finalization: Where Do Works Land in the Repo?

**Question:** When finalizing, where exactly should work artifacts be placed in the target repo?

**Options:**
- `.specs/{codename}/` in repo root — consistent, discoverable, gitignore-able
- `docs/specs/{codename}/` — conventional docs location
- Configurable via `config.yaml` (with a sensible default)

**Recommendation:** Default to `.specs/{codename}/` in repo root. Configurable via `finalize.repo_spec_path` in config.yaml.

---

## Finalization: Branch Strategy

**Question:** What branch naming and strategy should finalization use?

**Context:** Finalization creates a git branch, copies works, generates tasks. Questions:
- Branch from main/master or from current branch?
- Branch name pattern? `spec/{codename}`? `feature/{codename}`?
- Should it also create an initial commit?
- What if there are uncommitted changes in the target repo?

**Recommendation:** Branch from the default branch (main/master). Pattern: `spec/{codename}`. Create an initial commit with the work artifacts. Refuse to finalize if the target repo has uncommitted changes (fail safe).

---

## Jig Versioning in Works

**Question:** When a work is created with jig v1, and the jig later gets updated to v2, which version governs the in-progress work?

**Context:** If a jig changes passes or file structure mid-work, the work could become inconsistent.

**Options:**
- Snapshot the jig into the work directory at creation time (the work always uses the version it started with)
- Always use the latest jig version (works must be forwards-compatible)
- Record the jig version in spec.yaml, warn if there's a mismatch

**Recommendation:** Snapshot the jig into the work directory. The work is self-contained. Jig updates affect new works, not existing ones.

---

## Agent Instructions Delivery

**Question:** When the CLI emits instructions for the agent (e.g., after `shelve`), what's the delivery mechanism?

**Context:** The CLI prints to stdout. The agent reads stdout. So the instructions are just printed text that the agent sees as command output. This works, but:
- The instructions need to be clearly delimited (agent needs to know what's instruction vs. status output)
- Should instructions be structured (YAML/JSON) or natural language?
- How verbose should they be?

**Recommendation:** Natural language, clearly delimited with a header like `## Agent Instructions`. The agent is an LLM — it's better at following natural language than parsing structured instruction formats. Keep instructions concise and actionable.

---

## Cross-Project Work Discovery

**Question:** How does an agent working on work A discover and read work B in a different project?

**Context:** Dependency references include a project identifier. But the agent is working in one project's context. To read a dependency in another project, it needs to know the path to that project's work directory.

**Options:**
- `kerf show <codename> --project <other-project>` — explicit project targeting
- `kerf deps <codename>` — show all dependencies with their locations, let the agent read them
- Auto-load dependency works into context when resuming

**Deferred to:** When cross-project dependencies are implemented.

---

## ~~Naming~~

**Resolved.** See `09-naming.md`. The tool is called **kerf**. Jigs for templates, passes for phases, bench for workspace, square for verification.
