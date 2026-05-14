# Findings

> First-class feedback entities that capture issues discovered during execution or testing and route them back into the planning/tasking pipeline.

## What Is a Finding

A finding is a signal from downstream activity (EXEC or TEST) that something requires action. Findings are not annotations on existing works — they are independent, first-class entities stored on the bench. A finding represents the system's self-correction mechanism: the moment where execution reality diverges from planned intent.

Findings enter the system the same way any other intent does — they flow through the pipeline from their appropriate entry point. The difference is provenance: findings originate from agents doing work, not from humans having ideas.

### Why Findings Are First-Class

Without durable, structured findings:

- Issues discovered during testing get lost between sessions
- Small fixes accumulate as tribal knowledge in HANDOFF documents
- Design-level problems sit unaddressed because PLANNING is sporadic
- The feedback loop — the mechanism that makes the whole system converge on correctness — is lossy

Making findings first-class means they persist, they have lifecycle state, they surface in `kerf next`, and they cannot be silently dropped.

## Finding Severity

A finding's severity determines how far upstream the correction must travel. Severity is not a priority label assigned by a human — it is a structural property of where the root cause lives.

| Severity | Root Cause Location | Resolution Path | Typical Resolution Time |
|----------|-------------------|-----------------|------------------------|
| `code` | Implementation error — wrong logic, missed edge case | EXEC (tight loop: fix the code, re-test) | Minutes |
| `task` | Task decomposition gap — missing bead, wrong scope | TASK → EXEC → TEST (rework loop) | Hour |
| `spec` | Specification deficiency — behavior unspecified or incorrectly specified | SPEC → TASK → EXEC → TEST (medium loop) | Hours |
| `design` | Architectural problem — the approach is wrong | PLAN → SPEC → TASK → EXEC → TEST (wide loop) | Day+ |

### How Severity Is Determined

The agent at TEST (or EXEC) classifies the finding by answering: "Can I fix this by changing code? Changing tasks? Changing the spec? Or does the whole approach need rethinking?"

The classification does not need to be perfect. If a `code`-severity finding turns out to require a spec change, the agent working on it escalates by updating the finding's severity. The system self-corrects.

### Severity Is Not Priority

Severity describes the loop radius. Priority describes execution order. A `code`-severity finding and a `design`-severity finding can both be high priority. The difference: `code` severity can be addressed immediately by ALLOCATE dispatching a fix bead, while `design` severity must wait for PLANNING attention.

## How Findings Are Created

### Who Creates Findings

- **MERGE/TEST agent** — the primary source. When testing reveals a failure, gap, or inconsistency, MERGE/TEST creates a finding.
- **EXECUTE agent** — secondary source. When implementation reveals that a task is mis-scoped or a spec is wrong, EXECUTE creates a finding before or after completing its current bead.

PLANNING agents do not create findings. If PLANNING discovers an issue during spec review, it addresses it directly within the planning session (it is already at the right upstream level).

### What a Finding Contains

A finding is stored as a directory on the bench, parallel to works:

```
~/.kerf/projects/{project-id}/.findings/
  {finding-id}/
    finding.yaml          # metadata and classification
    description.md        # agent-written description of the issue
```

#### `finding.yaml` Schema

```yaml
# Identity
id: f-2026-05-08-001                    # string, required, auto-generated
title: "Adapter fails on empty input"   # string, required

# Classification
severity: code                          # string, required — code | task | spec | design
status: surfaced                        # string, required — see lifecycle below

# Provenance
source_agent: merge-test                # string, required — which agent type created this
source_work: auth-rewrite               # string, optional — codename of the work being tested
source_bead: "b-042"                    # string, optional — bead ID that exposed the issue

# Relationships
affects_works:                          # list of strings, optional
  - auth-rewrite
  - session-handler
affects_areas:                          # list of strings, optional
  - adapter
  - auth

# Resolution
resolved_by: null                       # string or null — codename of work/bead that fixed it
resolved_at: null                       # RFC 3339 or null

# Timestamps
created: 2026-05-08T14:30:00Z          # RFC 3339, required
updated: 2026-05-08T14:30:00Z          # RFC 3339, required
```

#### `description.md`

Free-form markdown written by the agent that discovered the issue. Contains:

- What was being tested or implemented
- What failed or was discovered
- Reproduction steps or evidence (test output, error messages)
- The agent's assessment of root cause (this informs severity)

This file is the finding's "body." It provides enough context for the next agent — whether that is ALLOCATE dispatching a fix or PLANNING doing a re-spec — to act without re-discovering the issue.

### Creation Command

Findings are created via `kerf finding create`:

```
kerf finding create --severity code --title "Adapter fails on empty input" \
  --work auth-rewrite --bead b-042 --area adapter
```

The command:

1. Creates the finding directory and files
2. Sets status to `surfaced`
3. Sets timestamps
4. Emits the finding ID and next steps

Agents may also create findings by writing the directory structure directly, as with any kerf artifact. The CLI command is the preferred path because it validates fields and emits guidance.

## The Two Paths

Findings follow one of two paths based on severity. The system does not have a separate routing mechanism — the severity classification determines which path applies, and `kerf next` surfaces findings on the appropriate path.

### Small Path: Code and Task Severity → Tasks Directly

Findings with severity `code` or `task` do not require PLANNING. They are actionable by ALLOCATE and EXECUTE immediately.

**Flow:**

```
MERGE/TEST creates finding (severity: code or task)
  → finding appears in kerf next with rework priority
  → ALLOCATE dispatches it (or the bead it generates)
  → EXECUTE implements the fix
  → MERGE/TEST verifies
  → finding status → resolved
```

For `code` severity, MERGE/TEST may attach the finding directly to the existing work as a corrective bead, bypassing work creation entirely. The finding still exists as a record.

For `task` severity, a new bead (or small set of beads) is created within the existing work's task structure. The finding references the new beads.

**No PLANNING session is needed.** The fix is scoped within existing specs and task structures.

### Large Path: Spec and Design Severity → Needs Planning

Findings with severity `spec` or `design` require PLANNING attention because the specification itself must change.

**Flow:**

```
MERGE/TEST creates finding (severity: spec or design)
  → finding appears in kerf next as "requires planning review"
  → ALLOCATE skips it (cannot dispatch without spec work)
  → finding persists on the bench, visible in kerf map
  → PLANNING session starts
  → kerf map / kerf next surfaces the finding first
  → PLANNING triages: creates a new work (bug jig or plan jig) to address it
  → finding status → triaged, linked to the new work
  → new work flows through normal pipeline
  → when the corrective work completes, finding status → resolved
```

**The critical property:** spec and design findings do not get lost when PLANNING is offline. They persist on the bench. They appear prominently in `kerf map`. When a PLANNING session eventually starts, the first thing the agent sees is unresolved findings requiring attention.

### Routing Summary

```
Severity    Path        Needs PLANNING?   Entry Point
────────    ────        ───────────────   ───────────
code        small       no                EXEC (tight loop)
task        small       no                TASK (rework loop)
spec        large       yes               SPEC (medium loop)
design      large       yes               PLAN (wide loop)
```

## Finding Lifecycle

```
surfaced ──→ triaged ──→ addressed ──→ resolved
               │
               └──→ dismissed
```

### States

| State | Meaning | Who Transitions |
|-------|---------|----------------|
| `surfaced` | Finding has been recorded. No agent has evaluated it yet. | Set at creation. |
| `triaged` | An agent or human has reviewed the finding, confirmed it is valid, and determined the resolution path. For small-path findings, triage may be automatic. | ALLOCATE (small path) or PLANNING (large path). |
| `addressed` | Corrective action is underway — a fix bead is in progress or a corrective work has been created. | Set when the corrective bead/work begins execution. |
| `resolved` | The fix has been verified. The issue no longer exists. | MERGE/TEST, after verifying the fix. |
| `dismissed` | The finding was reviewed and determined to be invalid, a duplicate, or not actionable. | PLANNING or ALLOCATE. |

### State Transitions

```
surfaced → triaged      Agent reviews and confirms the finding
surfaced → dismissed    Agent reviews and rejects (invalid, duplicate)
triaged  → addressed    Corrective work/bead begins execution
addressed → resolved   Fix is verified by MERGE/TEST
addressed → surfaced   Fix attempt failed; finding re-surfaces for re-triage
triaged  → dismissed   On further review, finding is not actionable
```

### Automatic Triage for Small-Path Findings

For `code`-severity findings, triage can be implicit. When ALLOCATE picks up a `code`-severity finding from `kerf next` and dispatches a fix bead, the finding transitions from `surfaced` to `triaged` to `addressed` in rapid succession. The agent does not need to perform a separate triage step — the act of dispatching the fix is the triage.

For `task`-severity findings, triage is similarly lightweight: confirm the finding, create the missing bead(s), dispatch.

## Priority Integration

### Rework Before New Work

Findings produce rework. Rework has structural priority over new work in the queue. This is not a configurable preference — it is a system invariant.

The rationale: continuing to build on a flawed foundation creates compound waste. Every new bead implemented while a known defect exists risks being invalidated by the eventual fix. Addressing findings first minimizes total rework.

### How Findings Surface in `kerf next`

`kerf next` computes the queue by composing work-level and bead-level information. Findings integrate as follows:

1. **Unresolved findings appear above new work.** Any finding in `surfaced` or `triaged` state ranks higher than any new-work bead, regardless of area or dependency ordering.
2. **Within findings, severity determines order.** `code` findings surface first (fastest to resolve), then `task`, then `spec`, then `design`. This minimizes time-to-resolution for the tightest loops.
3. **`spec` and `design` findings are tagged "requires planning review"** in `kerf next` output. ALLOCATE skips these. They are visible so that the state is transparent, but they are not dispatched until PLANNING has triaged them.
4. **`addressed` findings do not appear in `kerf next`.** They are in progress — their corrective beads appear in the queue like any other bead.

### Priority Is Origin-Based

A bead's priority is determined by its origin, not by a human-assigned label:

- Beads born from findings → rework priority
- Beads born from intents → new-work priority

`kerf next` distinguishes them by tracing provenance: does this bead trace back to a finding or to an intent? The provenance chain is: finding → corrective work/bead → execution.

## Persistence

### Storage Location

Findings live at `~/.kerf/projects/{project-id}/.findings/{finding-id}/` on the bench.

The `.findings/` directory is a sibling to work directories under the project, prefixed with a dot to distinguish it from works. This placement ensures:

- Findings are project-scoped (they belong to a specific codebase)
- Findings persist across sessions (they are files on disk)
- Findings are visible to any agent that reads the bench
- Findings do not clutter work listings (`kerf list` ignores `.findings/`)

### Finding ID Format

Finding IDs are auto-generated with the format `f-{date}-{sequence}` (e.g., `f-2026-05-08-001`). The date prefix provides chronological ordering. The sequence number prevents collisions within a day.

### Durability Guarantees

Findings follow the same durability model as works:

- The filesystem is the database. Files are the source of truth.
- A finding, once created, is never silently deleted. It must be explicitly dismissed or resolved.
- Dismissed and resolved findings remain on disk for audit purposes. They are filtered out of active views (`kerf next`, `kerf map`) but can be queried.

### Archival

Resolved and dismissed findings accumulate over time. kerf does not automatically delete them. A future `kerf finding archive` command may move old findings to `~/.kerf/archive/{project-id}/.findings/`, mirroring the work archival pattern. For v1, findings remain in place indefinitely.

## Cross-Work Findings

A finding may affect multiple works or areas. This is represented through the `affects_works` and `affects_areas` fields in `finding.yaml`.

### Multiple Works

When a finding affects multiple works:

```yaml
affects_works:
  - auth-rewrite
  - session-handler
```

Both works are flagged. `kerf resume` for either work surfaces the finding. `kerf map` shows the finding connected to both.

The corrective action may be a single fix that addresses both, or separate fixes per work. The finding is resolved when all affected works have been corrected.

### Multiple Areas

When a finding affects an area rather than a specific work:

```yaml
affects_areas:
  - adapter
  - auth
```

This is common for integration-level findings ("the adapter and auth modules don't compose correctly"). The finding is visible when working in any of the listed areas.

### Area-Only Findings

Some findings do not trace to a specific work — they are discovered during holistic testing and affect an area of the system. These findings have `affects_areas` populated but `source_work` and `affects_works` may be empty. They represent systemic issues rather than work-specific bugs.

## CLI Commands

### `kerf finding create`

Create a new finding.

```
kerf finding create --severity <code|task|spec|design> --title "<title>" \
  [--work <codename>] [--bead <bead-id>] [--area <area>...]
```

Writes `finding.yaml` and an empty `description.md` scaffold. Emits the finding ID and instructions for the agent to populate `description.md`.

### `kerf finding list`

List findings for the current project.

```
kerf finding list [--status <status>] [--severity <severity>] [--area <area>]
```

Default: shows `surfaced` and `triaged` findings (the active ones). Use `--status all` to include `addressed`, `resolved`, and `dismissed`.

### `kerf finding show`

Show details of a specific finding.

```
kerf finding show <finding-id>
```

Emits the full `finding.yaml` and `description.md` content.

### `kerf finding update`

Update a finding's status or classification.

```
kerf finding update <finding-id> --status <status> [--severity <severity>] \
  [--resolved-by <codename>]
```

Validates state transitions. Warns on invalid transitions (e.g., `resolved` → `surfaced`) but does not block them — agents may need to reopen findings.

### Integration with Existing Commands

- **`kerf next`** — includes unresolved findings in the queue output, ranked by the priority rules above.
- **`kerf map`** — shows findings as a separate section, grouped by severity and status. Findings with `affects_areas` appear in the relevant area's section.
- **`kerf resume <codename>`** — when resuming a work, shows any findings that reference that work via `affects_works` or `source_work`.

## Invariants

1. A finding, once created, persists until explicitly resolved or dismissed. It is never silently dropped.
2. Rework (from findings) has structural priority over new work in `kerf next`.
3. Findings with severity `spec` or `design` are not dispatched by ALLOCATE. They require PLANNING triage.
4. Every finding has a severity. Severity determines the resolution path.
5. Findings are project-scoped. They live on the bench alongside works.
6. The filesystem is the database. Finding state is determined by files on disk, not in-memory state.
