# Workflow improvements — friction reductions for the agent loop

Each item: rationale + rough scope (small fix / medium / new plan).

---

## 1. Snapshot-test `kerf next --help` and bare `kerf` against the spec

**Rationale**: Issue #1 in `agent_confusion.md` exists because `kerf next --help` drifted from `commands.md`. The spec already declares the help text part of the agent contract — a snapshot test would catch drift on every commit.

**Scope**: small fix. Add a test that diffs known sections of the cobra help against expected strings derived from `specs/commands.md`.

---

## 2. Add `kerf where` (or `kerf doctor`) — answers "where am I in the loop?"

**Rationale**: Today an agent reconstructs state by chaining `kerf list` → `kerf show <codename>` → `kerf next`. A single command that reports "project: X, active work: Y (status: research), next action: <one-liner>" would replace 3 invocations and make context-recovery deterministic. Especially valuable when context was just cleared.

**Scope**: small fix — pure read over existing state. Could be aliased into the bare `kerf` output too.

---

## 3. Make `kerf init` print a copy-pasteable agent-instruction block by default

**Rationale**: Spec says `kerf init` outputs agent-setup instructions, but they're embedded in a longer narrative. An agent should be able to extract a delimited block (`<<<KERF-AGENT-INSTRUCTIONS>>> ... <<<END>>>`) for direct paste into CLAUDE.md / .cursorrules without judgment.

**Scope**: small fix — wrap the existing setup output in stable delimiters.

---

## 4. `kerf next --explain <rank>` — show why an item ranked where it did

**Rationale**: Spec (`coordination.md#computed-priority`) defines scoring inputs but there's no way to see them. An agent picking between near-tied items has no insight into the decision. An `--explain` mode that prints score breakdown per factor would build trust and let users tune the project filter intelligently.

**Scope**: medium. Requires structuring the scorer to emit traces.

---

## 5. Compose `kerf status` advance with auto-snapshot label

**Rationale**: Today `kerf status <work> research` snapshots with a timestamp. The new status value is itself a perfect label. Auto-naming the snapshot `before-research` / `after-research` would make `kerf history` immediately scannable.

**Scope**: small fix.

---

## 6. `kerf shelve --write` shortcut

**Rationale**: Spec's normal shelve outputs instructions telling the agent to write SESSION.md. A `--write` flag that takes a path or stdin and writes SESSION.md *as part of shelving* would make the operation atomic. Today there's a race window where the agent shelves, then context-clears, then was supposed to write SESSION.md but didn't.

**Scope**: medium. Need to decide if it pre-empts the "let the agent compose freely" philosophy. Could just be `kerf shelve --session-file path/to/draft.md`.

---

## 7. Reconcile pass terminology across specs and tooling

**Rationale**: Issue #16 in `agent_confusion.md` — "pass", "status", "stage" all appear. One audit pass and a glossary entry resolves it permanently. Worth doing before another command joins the mix.

**Scope**: small fix. Mostly documentation.

---

## 8. `kerf next` warning item when `project.yaml` missing

**Rationale**: Currently a never-initialized project produces the same "no actionable works" output as a properly-initialized empty project. Spec already supports warning-kind items; adding one for "no project.yaml — run `kerf init`" closes a confusing dead-end.

**Scope**: small fix. New warning detector, fits the existing model.

---

## 9. Auto-detect "what pass am I in?" from filesystem artifacts

**Rationale**: A work's pass should arguably be implied by which artifact files exist (e.g., `02-analysis.md` exists → past `analyze`). Today the agent must remember to run `kerf status <codename> <new-status>` after writing each artifact. A pre-commit-like hook or a `kerf status --auto` mode that infers from files would reduce one manual step per pass.

**Scope**: new plan. Requires deciding what to do when files exist out of order or are empty.

---

## 10. Self-test command: `kerf verify-tools`

**Rationale**: The implementation jig declares `bd, ntm, agent-mail` as required tools. Today an agent may discover one is missing only when it tries to use it mid-pass. A `kerf verify-tools` (or rolled into `kerf doctor`) that runs at init time and reports availability would surface this up front.

**Scope**: small fix. One-time exec probe per declared tool.
