# Coordination

> How work moves through the system, how kerf supports multi-agent workflows, and how priority is computed.

## The Coordination Layer

kerf coordinates work across independent agents through shared state. Agents do not communicate directly. Each reads kerf's state, acts, and writes results back. Coordination emerges from the state — from what is visible on the board — not from messages between agents. kerf is the blackboard: it stores facts, computes views over those facts, and lets any agent read or write. It does not dispatch work, manage agents, or enforce protocols.

## How Work Flows

### The Five Activities

Work moves through five activities:

```
PLAN    think about what needs to change and why
SPEC    define precisely what to build
TASK    decompose the spec into atomic beads with dependencies
EXEC    implement a bead
TEST    verify that implementation matches intent
```

These are activities the system performs, not stations where work sits. Multiple activities happen concurrently on different units of work. In kerf terms: PLAN and SPEC correspond to a work item's jig passes during spec-writing. TASK corresponds to bead breakdown (e.g., the implementation jig's breakdown pass). EXEC and TEST correspond to bead execution and verification.

### Forward Flow and Feedback Loops

```
                      +-------------------------------------+
                      |            WIDE LOOP                |
                      |                                     |
                 +----+-------------------+                 |
                 |    |    MEDIUM LOOP     |                 |
                 |    |                    |                 |
            +----+----+--------+          |                 |
            |    |    | TIGHT  |          |                 |
            |    |    | LOOP   |          |                 |
            v    v    v        |          |                 |
 --IN-> PLAN --> SPEC --> TASK --> EXEC --> TEST --> OUT-->
                                   ^  ^      |
                                   |  |      |
                                   |  +------+
                                   |  REWORK LOOP
                                   |
                                   +-- FAST TRACK
                                       (known fix enters
                                        directly as bead)
```

Work flows forward through the activities. When testing or execution surfaces a problem, the signal re-enters the system at the point where the root cause lives, then flows forward through the remaining activities. This is not backward movement — it is new information entering at the appropriate point.

The four feedback cycles differ by how far upstream the root cause lives:

| Root cause in | Cycle | Typical resolution |
|---|---|---|
| Code (wrong implementation) | Tight loop: EXEC <-> TEST | Minutes |
| Tasks (wrong decomposition) | Rework loop: TEST -> TASK -> EXEC -> TEST | An hour or less |
| Spec (wrong or missing) | Medium loop: TEST -> SPEC -> ... -> TEST | Hours |
| Plan (wrong approach) | Wide loop: TEST -> PLAN -> ... -> TEST | A day or more |

### Entry Points

Work enters the system at different points depending on how well-formed it is:

| Entry point | Condition | Example |
|---|---|---|
| PLAN | Vague idea, needs full cycle | "We should rethink auth" |
| SPEC | Well-formed requirement, approach known | "Add OAuth2 support per RFC 6749" |
| TASK | Known issue, clear fix needed | "Null check missing in handler.go:42" |
| EXEC | Trivial fix, bead created directly | "Fix typo in error message" |

This is one graph with multiple entry points, not separate pipelines. The entry point reflects how much upstream thinking has already been done.

### Findings Flow Through Beads

When execution or testing surfaces a problem — a bug, a gap, a contradiction — that signal is a **finding**. Findings are not a separate entity in kerf. They are tagged beads: a bead whose metadata indicates it surfaced an issue that needs attention. kerf can surface findings via `kerf next` by querying bead status and tags.

#### Tag Conventions

kerf treats a bead as **rework** if any of its labels match either of these forms (case-insensitive):

- `rework:true` — explicitly marks the bead as corrective work.
- `finding:<origin>` — marks the bead as a finding from another work; the suffix carries attribution (e.g. `finding:work-a`). Any label starting with `finding:` counts.

These two forms are equivalent for ordering purposes — both flag the bead as rework. The `finding:` form is preferred when attribution to an originating work item is useful; `rework:true` is a generic fallback. Beads are otherwise normal beads — same statuses, same dependencies, same lifecycle.

The classification of a finding determines what happens next:

- **Code-level fix.** Corrective beads are created within the existing work, tagged as rework. They enter the queue with a score bonus (see below). This is the tight or rework loop.
- **Implementation gap.** A new work item is created (e.g., with the bug jig) covering the missing piece. Compressed planning cycle, then new beads.
- **Spec deficiency.** A new work item is created with the affected area tags. This requires full planning attention. `kerf map` surfaces it prominently so it is addressed when a planning agent is next active.

In all cases, findings flow through the existing work item and bead infrastructure. They carry traceability — which bead surfaced them, which areas they affect — but they do not require their own storage or lifecycle outside of beads and work items.

## kerf's Role: The Shared State Layer

kerf maintains a graph of work items, jig artifacts, areas, dependencies, and status. Every agent reads a projection of this graph relevant to its current task. Some agents write new nodes and edges.

### What kerf Maintains

- **Work items** — the unit of planning. Each has a codename, type, jig, status, and artifacts. See [works.md](works.md).
- **Jig artifacts** — the specification documents produced by a work's jig passes. These are the designs.
- **Areas** — named regions of the system (e.g., `cli`, `jig-system`, `bench-storage`). Areas serve two roles: planning coherence (multiple work items touching the same area are made visible during design) and execution grouping (beads in the same area can be worked together for context efficiency).
- **Dependencies** — between work items and between beads. These constrain ordering.
- **Status** — where each work item and bead stands in its lifecycle.

### What kerf Computes

The graph is the thing. Commands are projections of it:

- **`kerf map`** — the portfolio view. All work items, their statuses, areas, dependencies, in-flight beads. Used by any agent to orient: what exists, what is in progress, what is blocked, what needs attention.
- **`kerf next`** — the work queue. Given the current graph, what should be worked on? Returns an ordered list of available beads considering dependencies, area focus, and priority signals. This is the pull signal that drives execution.
- **`kerf resume`** — the work context. For a specific work item, everything an agent needs: spec artifacts, dependency status, session history, related area peers.

### What kerf Is Not

- **Not a message queue.** Items are not consumed by reading. `kerf next` is idempotent — running it ten times with no state changes produces the same result.
- **Not an orchestrator.** kerf does not dispatch work or manage agents. An agent (or a human, or a script) reads `kerf next` and acts on it.
- **Not a lock manager.** Conflicts are resolved by convention and detection, not prevention.
- **Not a notification system.** Agents poll kerf when ready. Coordination is polling-based, consistent with the filesystem-as-database architecture.

### Agent-Agnostic Design

kerf does not prescribe agent topology. One agent might do everything — plan, execute, test. Or there might be a planning agent, an allocation agent, several execution agents, and a testing agent. kerf provides the same operations either way: `kerf map` to see the state, `kerf next` to find available work, `kerf resume` to load context for a specific work item.

As an example, a team might use kerf with: a planning agent that creates work items and specs during interactive sessions, a coordinator script that polls `kerf next` and dispatches beads to worker agents, and a testing agent that verifies completed beads. But this topology is a choice made by the user, not something kerf imposes. kerf sees reads and writes to shared state — it does not care who is making them.

## Priority and Ordering

### The Pull Model

Execution is pull-based. Agents pull work when ready via `kerf next`. Everything upstream of the queue — PLAN, SPEC — is push: human thinking and planning happen regardless of downstream capacity. The queue absorbs the impedance mismatch.

```
      PUSH                              PULL
 ideas enter regardless          agents pull when ready
 of downstream capacity

 PLAN ----> SPEC                 TASK ----> EXEC ----> TEST
 (human thinking                 (kerf next is the pull signal;
  cannot be throttled)            execution pulls from queue)
```

### Computed Priority

Priority is computed from graph structure, not assigned as static labels. The factors that compose into the ranking:

1. **Rework before new work.** `kerf next` adds a per-bead score bonus for each rework-tagged bead in a work, so works with active rework typically rank ahead of new-work-only works. The bonus is governed by the configurable `rework` weight (see below); rework beads are identified by the tag conventions described above.

2. **Completion momentum.** When most beads from an epic or work item are complete, the remaining beads get priority. This prevents orphaned work — when four of five beads are done, the fifth should not be stranded while beads from another area are dispatched.

3. **Dependency fan-out.** Beads that unblock the most downstream work rank higher. Computed from the dependency graph.

4. **Area focus.** Prefer to finish work in an area before starting work in a new area. This reduces context switching and avoids leaving areas in a partially-modified state.

These factors compose into a ranking that `kerf next` computes fresh on each invocation. No stored priority field. The ranking reflects the current state of the graph.

The scoring above applies to `bead` items. `cleanup` items are not mixed into the bead ranking: they sort after all beads, ordered by their parent work's would-be bead score (descending). Cleanup items with equal parent-work scores are ordered by the work's `created` timestamp ascending. `warning` items are not ranked at all — they render as a header block in `kerf next` output. See [commands.md](commands.md#kerf-next).

### The Ordering Algorithm

The ordering algorithm lives in one place in the codebase — the `kerf next` computation. The weights and parameters are expected to be configurable over time as real-world usage reveals optimal patterns. Some factors may initially be hardcoded; when they are, they should be obvious and localized so they can be extracted into configuration later.

#### Configurable Weights

Scoring weights are read from `project.yaml` under a `queue:` section:

- `fan_out` — multiplier applied per transitive downstream dependent a work unblocks. Default: `10.0`.
- `momentum` — multiplier applied to the completed/total bead ratio (a work at 100% completion gets the full value added). Default: `5.0`.
- `creation` — small tiebreaker added per position from newest, favoring older works. Default: `0.1`.
- `rework` — multiplier applied per rework-tagged bead in a work (see Tag Conventions). Default: `15.0`.

```yaml
queue:
  fan_out: 10.0
  momentum: 5.0
  creation: 0.1
  rework: 15.0
```

When the `queue:` section is absent, or any individual field is unset, the defaults above are used. Each field is independent — specifying `fan_out` alone leaves `momentum` and `creation` at their defaults.

### Batches Are Ephemeral

When an agent pulls multiple beads to work on together (e.g., beads in the same area), that grouping is a batch. Batches are ephemeral — assembled, worked, done. kerf does not store dispatch history or batch records. The beads themselves carry all necessary traceability; the batch is just a transient convenience.

## Integration Points

### kerf and Beads

kerf generates bead definitions during task breakdown (the TASK activity). The beads system (br) tracks bead execution state — who claimed it, whether it is complete, whether it failed. kerf queries bead status to compute its views:

- `kerf next` needs to know which beads are available (not blocked, not in-progress, not complete).
- `kerf map` needs to know how many beads are done vs. remaining for each work item.
- Completion momentum requires knowing how close an epic is to done.

kerf reads bead status but does not own it. The beads system is the source of truth for bead lifecycle. kerf is the source of truth for work items, specs, areas, and the relationships between them.

### Bead Attachment

A bead attaches to a work when it matches that work's **bead filter**. The filter is configurable so that real projects with their own labeling and ID conventions can drive bead attachment without renaming anything in the bead store.

#### Filter shape

A bead filter is an object with one of two forms:

```yaml
# Direct clause
bead_filter:
  label: "subsystem:{codename}"
```

```yaml
# Union of clauses
bead_filter:
  any:
    - label: "subsystem:bridge"
    - label: "codename:claude-hook-bridge"
    - id_prefix: "hk-cb"
```

Clause types:

- `label: <string>` — matches when the bead carries the named label.
- `id_prefix: <string>` — matches when the bead's ID starts with the given string.

The `any:` form is a union: a bead matches the filter if it matches any clause. There is no `all:` form in v1.

#### Template variables

One template variable is supported in clause values: `{codename}`. It is substituted at match time with the codename of the work whose filter is being evaluated. This keeps filters language-neutral and lets a single project-wide filter cover every work without per-work duplication.

Matching is case sensitive. When the project-wide filter's literal prefix yields zero matches anywhere in the bead store, kerf surfaces a `warning` item in `kerf next` suggesting the user check for a case-mismatch (e.g., `Subsystem:` vs `subsystem:`). See [commands.md](commands.md#kerf-next).

#### Resolution order

When attaching beads for a given work, kerf resolves the filter in this order — first hit wins, filters do not merge:

1. The per-work `bead_filter` from the work's `spec.yaml` (see [works.md](works.md)).
2. The project-wide `bead_filter` from `project.yaml` (see [architecture.md](architecture.md#project-configuration)).
3. The built-in default: `label: "work:{codename}"`.

"First hit" means "first filter defined at that level," not "first filter that produces a match." If a work has no per-work `bead_filter`, kerf uses the project-wide filter; if that project-wide filter exists but matches zero beads for the work, kerf does not fall through to the built-in default. The zero-match outcome is real signal — it surfaces as a `work_no_attached_beads` cleanup item (see [commands.md](commands.md#kerf-next)), prompting the user to fix the filter or accept that the work has no beads yet.

A filter resolution is per-call; no caching.

Pins (see [Pin Layer](#pin-layer) below) compose with filter resolution as a separate layer applied after the filter resolves. A bead pinned to work A is excluded from the resolved filter match of every other work, so it appears under A only. The "first hit wins, filters do not merge" rule above is unaffected — pins are not a filter.

#### Multiple matches

A bead may match the filters of more than one work. In that case it counts for each — bead attachment is a many-to-many relation, and downstream computations (queue scoring, bead progress in `kerf map`) see the bead under every work it matches.

#### Unmatched beads

A bead that matches no work's filter is **unmatched**. Unmatched beads are not an error — they may belong to other tooling, may be in flight, or may indicate a misconfigured filter. kerf surfaces them as a project-level warning item in `kerf next` (see [commands.md](commands.md#kerf-next)) so the user can decide whether to adjust the filter or ignore them.

An unmatched bead is a candidate for triage: `kerf triage` reports it as `untriaged` and offers ready-to-paste remediation commands (see [commands.md](commands.md#kerf-triage)). The `untriaged_beads` triage category and the project-wide unmatched-beads warning describe the same set of beads viewed from two angles — the warning surfaces existence; triage surfaces remediation. They do not double-fire: when triage output is rendered, the warning's count line is omitted from the same `kerf next` invocation.

The `work_no_attached_beads` cleanup detector (a work whose filter matches zero beads) and `untriaged_beads` (a bead matching no work) are complementary — they describe inverse zero-match conditions. They do not double-fire on the same work: a work with a zero-match filter surfaces only as `work_no_attached_beads`; the beads that fail to match are surfaced as `untriaged` items attributed to no specific work.

#### Bead-filter rank labels

A work whose resolved `bead_filter` matches zero open beads is classified by rank label. Today the vocabulary is two labels; a third (`broken`) is reserved for when parser support lands. This replaces the earlier single `clean` label, which conflated distinct conditions:

- **`empty`** — `bead_filter` is declared and syntactically valid, but matches zero open beads in the current store. Likely benign: the work is wired, its beads simply have not been created yet. A malformed filter that the parser rejects up front also surfaces here today, because `spec.Read` refuses to load a spec whose `bead_filter` clause does not parse — the work never reaches the zero-match detector with a malformed clause intact.
- **`unwired`** — no `bead_filter` key on the work's `spec.yaml`, or the key is present with an empty value. The work needs a filter authored (or bootstrapped from existing labels) before it can attach beads.

<!-- TBD: open question 5 from plan 019 — `broken` lands when the parser can distinguish a malformed clause from a valid clause that matches nothing. Until then, malformed filters are rejected at parse time and the surface is two-state. -->

The two labels share the surface previously occupied by `work_no_attached_beads`: a zero-match work surfaces as exactly one of `empty` or `unwired`, not as the generic cleanup item. The detector logic and `kerf next` rendering are specified in [commands.md](commands.md#kerf-next).

#### Prefix routing for label-driven suggestions

kerf commands that propose new works or bead-filter clauses from observed labels (notably `kerf triage` suggestions and `kerf bootstrap-filters`) classify label prefixes into two tiers:

- **Tier 1 — cohort-defining prefixes.** Prefixes that identify a work cohort: `codename:`, `spec:`. A label with a tier-1 prefix may seed a `kerf new` suggestion or anchor a per-work `bead_filter` clause.
- **Tier 2 — cross-cutting prefixes.** Prefixes that group beads orthogonally to work cohorts: `axis:`, `tag:`, `kind:`, `scope:`, plus any prefix not on the tier-1 list. A tier-2 label never seeds a `kerf new` suggestion; it may still appear inside a clause when paired with a tier-1 anchor.

Tier 1 is a small, explicit allow-list rather than a tier-2 deny-list, so an unfamiliar prefix (`subsystem:`, `area:`, etc.) falls to tier 2 by default. This keeps suggestions conservative when a project introduces a prefix kerf has not seen.

When a bead's labels are all tier 2, the suggester falls back to pinning the bead against the lexicographically-earliest active work (see [Pin Layer](#pin-layer)), or to "no auto-suggestion; investigate manually" when no active work exists. Suggesters also consult archive state before proposing `kerf new <codename>` — an archived codename produces a re-pin / unarchive hint instead.

The tier-1 list is currently fixed in code. A project-level override (e.g., a `cohort_prefixes:` slot in `project.yaml`) is contemplated but not specified here; see plan 018.

### Pin Layer

The **pin layer** attaches specific bead IDs to specific works regardless of filter outcome. Each work's `spec.yaml` carries a `pinned_beads:` list (see [works.md](works.md)). Pins exist for the case where a bead cannot reasonably be caught by any filter clause and editing the filter would over-broaden it.

#### Composition with filter resolution

Pins apply *after* filter resolution. For a given bead:

1. The filter for each work is resolved per [Resolution order](#resolution-order) above and produces a candidate set of matching works.
2. If the bead ID appears in any work's `pinned_beads`, the bead's attachment is restricted to that single pinning work. The bead is removed from every other work's filter-resolved match.
3. If the bead ID is not pinned, the filter-resolved candidate set stands. The bead may attach to multiple works (see [Multiple matches](#multiple-matches)).

The filter rule is unchanged: filters do not merge. The pin layer rides on top.

#### Single-owner invariant

A bead ID appears in at most one work's `pinned_beads` list across the entire project. Pinning bead B to work A removes B from any other work's `pinned_beads` list as part of the same operation — pins are *not* additive. This is what lets `kerf triage --resolved` converge: an additive pin layer would loop forever on a bead whose filter overlap cannot be narrowed.

A pinned bead is never reported as multi-matched, even if its filter matches multiple works. The pin is the answer.

See [commands.md](commands.md#kerf-pin) for the `kerf pin` command that mutates pin state.

### Drift Detection

kerf reads bead state from the beads system on every invocation, but a project's bead store evolves independently of kerf: beads are added, relabeled, closed, reopened, deleted by other tools. **Drift detection** records what kerf has previously acknowledged about the bead store and surfaces changes since that baseline.

Drift detection is the data layer that powers `kerf triage` (see [commands.md](commands.md#kerf-triage)) and the drift summary line at the top of `kerf next`.

#### Sync cache

Drift state lives in the project-local file `.kerf/sync-cache.json` (registered in [architecture.md](architecture.md#in-the-repo-inside-git)). The file holds a single snapshot — the **baseline** — representing the last bead-store state the project acknowledged via `kerf triage --ack`.

A missing or empty `.kerf/sync-cache.json` is treated as an empty baseline: every current bead reads as "new since baseline," and the first `kerf triage` run becomes a full inventory pass.

#### Snapshot shape

```json
{
  "snapshot_id": "<sha256 of sorted per-bead hashes>",
  "captured_at": "2026-05-15T12:34:56Z",
  "beads": {
    "hk-cb-042": {
      "status": "open",
      "labels": ["subsystem:bridge", "priority:p1"],
      "title": "wire retry into adapter",
      "deps": ["hk-cb-040"],
      "hash": "<sha256 of the fields above>"
    }
  },
  "filter_assignments": {
    "hk-cb-042": ["bridge"]
  }
}
```

- `snapshot_id` — sha256 of the per-bead hashes concatenated in sorted-by-id order. Cheap to recompute, lets the cache itself be a content-addressed signature.
- `captured_at` — RFC 3339 timestamp of when the baseline was written.
- `beads` — keyed by bead ID. The recorded fields are the only ones kerf consumes.
- `filter_assignments` — for each bead in the snapshot, the list of work codenames it was attached to at baseline time. Used to detect that a bead's attached works changed (e.g., a label edit moved it between works).

#### Hash scope

The per-bead `hash` is the sha256 of a deterministic byte encoding of the kerf-consumed fields. The encoding is a UTF-8 string formed by concatenating the following components, each terminated by a single `\n` (line feed, `0x0A`):

1. `id=<bead_id>` — the bead's own ID. Included so that two beads with otherwise identical fields produce distinct hashes, and so that an ID rename surfaces as drift.
2. `status=<status>` — the bead's status string, lowercased (e.g., `open`, `closed`, `in-progress`).
3. `title=<title>` — the bead's title, verbatim. Titles are taken as a single line; embedded line feeds in titles are not permitted by the beads system and are not specially escaped here.
4. `labels=<l1>,<l2>,...,<ln>` — the bead's labels, lowercased, deduplicated, sorted lexicographically (Go `sort.Strings` on the lowercased slice), then joined with a single `,` (comma). Empty label list encodes as `labels=`.
5. `deps=<d1>,<d2>,...,<dn>` — the bead's dependency IDs (the IDs of beads this bead depends on), sorted lexicographically, joined with a single `,`. Empty dependency list encodes as `deps=`.

The sha256 is computed over the resulting byte sequence and rendered as lowercase hex. Other fields the beads system may carry (assignees, custom metadata, timestamps not visible to kerf, body text, priority) are **not** part of the hash. This keeps the hash stable across changes kerf does not act on, and avoids a forced re-triage every time the bead system adds a field.

Worked example. A bead with `id=kerf-2sl`, `status=Open`, `title=Relabel drift hash scope`, `labels=["plan-008", "Phase-1", "plan-008"]`, `deps=["kerf-n4h", "kerf-0kx"]` encodes as the literal byte sequence:

```
id=kerf-2sl
status=open
title=Relabel drift hash scope
labels=phase-1,plan-008
deps=kerf-0kx,kerf-n4h
```

(five lines, each terminated by `\n`, including the final line). The per-bead `hash` is `sha256` of those bytes, hex-encoded. Note that `Phase-1` was lowercased and the duplicate `plan-008` was collapsed before sorting; status `Open` was lowercased; dependency IDs were sorted lexicographically.

#### Drift categories

When kerf compares the current bead store to the baseline, it classifies each difference into one of:

- **`untriaged`** — a bead in the current store that matches no work's filter and is not pinned. The bead may be new since baseline, or may have existed at baseline with a label that matched a filter that has since been edited away.
- **`multi_matched`** — a bead in the current store that matches more than one work's filter and is not pinned to resolve the ambiguity.
- **`external_close`** — a bead present at baseline and at current with status differing such that it closed externally.
- **`external_reopen`** — a bead present at baseline as closed, now open again at current.
- **`external_delete`** — a bead present at baseline, absent at current.
- **`external_new`** — a bead absent at baseline, present at current.

`untriaged` and `multi_matched` are computed from the current store alone (no baseline needed). The four `external_*` categories require the baseline to compute.

#### Baseline advancement

The baseline advances only on `kerf triage --ack` (see [commands.md](commands.md#kerf-triage)). On `--ack`, kerf captures the current snapshot and writes it to `.kerf/sync-cache.json`, replacing whatever was there.

The baseline does **not** advance implicitly. Commands that change kerf-side state — `kerf new`, `kerf pin`, `kerf work edit`, `kerf status` — leave the baseline untouched. Drift surfaces stay sticky until the agent acknowledges them. Implicit advancement would let drift silently rebaseline on routine commands, hiding external changes the agent never saw.

#### Composition with other detectors

The drift categories listed above coexist with the existing `kerf next` warning detectors (see [commands.md](commands.md#kerf-next)):

- The `untriaged_beads` triage category and the project-wide unmatched-beads warning describe the same set; the drift-summary line at the top of `kerf next` reports the count, and the lower warning block is omitted when the drift-summary line is present.
- The `multi_matched` triage category drives a new `multi_matched` warning kind on `kerf next`.
- The four `external_*` categories drive a single `external_drift` warning kind on `kerf next` (counts aggregated; details belong in `kerf triage`).

These compose with `work_no_attached_beads` per the rule above: a zero-match work surfaces only as a cleanup item, and the inverse zero-match beads surface only as `untriaged`. The two detectors do not double-fire on the same work.

### Information Flow

```
kerf                              beads (br)
 |                                   |
 |-- work items, specs, areas ------>|  (bead definitions reference
 |                                   |   backing specs and areas)
 |                                   |
 |<-- bead status, completion -------|  (kerf queries to compute
 |                                   |   next, map, momentum)
```

The boundary is clean: kerf owns planning artifacts, beads owns execution state. The `kerf next` computation bridges both — it composes kerf's work-level information (areas, dependencies, priority signals) with the beads system's task-level information (readiness, completion state) to produce the ordered queue.

## Feed-warning rules

`kerf next` may surface warnings during feed assembly instead of silently skipping the offending input. Each warning kind is defined normatively in [commands.md](commands.md#warning-kinds) under `kerf next`. This table is the cross-cutting index — it names every kind, marks whether it is fatal to the feed, and points to its detector.

| Kind | Fatal? | Detector lives in | Trigger summary |
|------|--------|-------------------|-----------------|
| `corrupt_spec` | No — the offending work is excluded, the feed is still emitted for the remaining works. | Spec loader inside `kerf next` feed assembly (per-work `spec.yaml` read). | A per-work `spec.yaml` cannot be parsed (malformed YAML, invalid timestamp, schema violation). Replaces the legacy silent drop. |
| `no_project_yaml` | Yes — no feed is produced; the command exits non-zero. | Project resolver inside `kerf next`, before feed assembly. | The project id resolves but `project.yaml` is absent from both the local-storage and bench paths. Suggests `kerf init`. |

Rules that apply to every warning kind:

1. **Single source of truth.** Field shapes (`title`, `action`, `reason`) and message templates are specified once in [commands.md](commands.md#warning-kinds). This table only catalogues kinds; it does not redefine their fields.
2. **Order.** Warnings are emitted in detection order, before the feed listing.
3. **Fatality.** A fatal warning suppresses the feed listing for that invocation and sets a non-zero exit status. A non-fatal warning is informational; the feed listing still prints with the offending entries excluded.
4. **No silent drops.** When feed assembly excludes a work for a structural reason (parse failure, missing required file), it must emit a warning of a documented kind. Adding a new exclusion path requires adding a new warning kind here and in commands.md.
5. **Stability.** Kind names are stable strings — agents and downstream tools may match on them. Renaming a kind is a breaking change.
