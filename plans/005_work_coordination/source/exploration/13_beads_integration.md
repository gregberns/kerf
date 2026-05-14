# Beads Integration — Data Flow Architecture

> Source: analysis of harmonik's pilot-data YAML schema, `br`/`bv` CLI capabilities,
> Greg's feedback in `10_USER_RESPONSE.md`, and the five problems in `_plan.md`.

---

## 1. The kerf-to-beads pipeline

The pipeline has four stages. Each stage has a clear owner and a clear artifact.

```
  kerf jig passes          agent + kerf           loader           beads (br/bv)
  ───────────────         ────────────────       ────────────     ───────────────
  specs + plans   ──►   task YAML file(s)  ──►  br create/dep  ──►  SQLite + JSONL
       ▲                                                                 │
       └─────────────────── status queries ◄─────────────────────────────┘
```

**Stage 1: Spec production.** kerf's jig passes produce specs and plans. This is kerf's core job. The output is markdown files in `specs/` and `plans/`.

**Stage 2: Task decomposition.** An agent reads the specs and produces a task YAML file. This is the creative/analytical step — the agent decides how to break a spec into implementable beads, what the dependency edges are, and how they relate to beads from other specs/works. The harmonik project proved this works: the agent authored `cp-pilot-data.yaml` (85 beads, 280 edges) directly from the spec.

**Stage 3: Loading.** A loader script reads the YAML and issues `br create` and `br dep add` commands. This is mechanical. The harmonik loader (`scripts/load-pilot.py`) handles: idempotent re-runs via mnem-map CSV, cross-spec edge resolution via mnem-map lookups, forward-deferred edge logging, cycle detection at load time.

**Stage 4: Execution and status.** Agents work beads via `br` commands. `bv` provides the analytical layer (triage, priority, dependency analysis). kerf queries bv/br to understand work status.

### What kerf owns vs. what it delegates

kerf should own Stage 2's *trigger* (knowing when decomposition is needed) and Stage 4's *query* (asking beads for status). kerf should NOT own Stage 2's *content* (the actual decomposition logic — that's an agent skill, not a CLI feature) or Stage 3's *mechanics* (the loader is a beads-ecosystem tool).

kerf's role is:
- Produce the spec artifacts that feed decomposition
- Provide the YAML schema definition so agents know the target format
- Trigger decomposition when a work is ready for tasking
- Query beads to compute work-level status from task-level status
- Surface that status in `kerf map` and `kerf next`

---

## 2. The YAML intermediate representation

### Why YAML and not direct br calls

Three reasons emerged from the harmonik experience:

1. **Agent reasoning.** Agents can scan a YAML file and reason about the whole decomposition — all beads and all edges — before anything is loaded. They can check their own work, find missing edges, spot decomposition gaps. With direct `br create` calls, the decomposition is committed incrementally and hard to review as a whole.

2. **Validation before commit.** The YAML can be validated (schema conformance, dependency cycle detection, cross-spec reference resolution) before any beads exist. This catches errors cheaply.

3. **Reviewability.** A human or review-agent can read the YAML, compare it against the spec, and flag problems. The harmonik project ran formal review rounds on YAML files (see `pilot-review-protocol.md`), producing findings like "cp-008 missing AR role-taxonomy edges." This is much harder against a live beads database.

### Proposed schema

The harmonik schema is proven at scale (8 specs, hundreds of beads, thousands of edges). The kerf version should be a subset that drops harmonik-specific conventions while preserving the core structure.

```yaml
# kerf task YAML — produced by decomposition agent, consumed by beads loader

work:
  id: "adapter-retry"                    # kerf work identifier
  title: "Adapter retry logic"
  spec_paths:                            # specs this work implements
    - specs/adapter.md
  areas:                                 # kerf area tags
    - adapter
    - resilience

beads_config:
  prefix: ar                             # mnemonic prefix for all beads
  default_labels:
    - "work:adapter-retry"               # ties beads back to kerf work
    - "area:adapter"

epic:
  mnem: ar
  title: "Adapter retry logic — implementation"
  description: |
    Implements adapter retry per specs/adapter.md ...

beads:
  - mnem: ar-001
    title: "RetryPolicy configuration struct"
    description: |
      Per spec §3.1: ...
    labels: []                           # additional labels beyond defaults

  - mnem: ar-002
    title: "Exponential backoff with jitter"
    description: |
      Per spec §3.2: ...

edges:
  # Intra-work edges
  - {from: ar-002, to: ar-001}          # backoff uses RetryPolicy config

  # Cross-work edges — reference beads from other works
  - {from: ar-003, to: conn-007}        # retry interacts with connection pool
  - {from: ar-005, to: metrics-002}     # retry emits metrics via metrics subsystem

cross_works:
  conn: works/connection-pooling/tasks.yaml    # where to resolve conn-* mnemonics
  metrics: works/metrics/tasks.yaml
```

### Key schema decisions

**`work` block links to kerf.** Every task YAML declares which kerf work it belongs to, which specs it implements, and which areas it touches. This is how kerf can find the YAML for a given work and how the beads-to-work mapping is maintained.

**`default_labels` include `work:<id>`.** This is the primary mechanism for querying "all beads belonging to this work." `br list -l work:adapter-retry` returns exactly the beads from this decomposition. `bv --label work:adapter-retry` scopes all analysis to this work.

**Cross-work edges use prefix-based resolution.** Same pattern as harmonik's `cross_specs`. The `cross_works` block maps prefixes to YAML files (or mnem-map CSVs). The loader resolves `conn-007` by reading the connection-pooling work's mnem-map. This is exactly how harmonik handled cross-spec dependencies.

**Forward-deferred edges.** When work A needs to reference a bead in work B, but work B hasn't been decomposed yet, the edge uses `forward:<prefix>-<mnem>` syntax. The loader logs it but doesn't load it. When work B is later decomposed, the edge can be materialized.

---

## 3. The status feedback loop

### The query model (kerf reads from beads)

kerf should query beads, not the other way around. Reasons:

- beads already has rich query capabilities (`br list`, `br ready`, `bv --robot-triage`, `bv --robot-plan`)
- Pushing state from beads to kerf requires hooks/callbacks — fragile, another thing to configure and break
- The YAML file is not a good place for runtime state — it's an input artifact, not a live document

### How kerf computes work status

```
kerf map  →  for each work:
               1. Find task YAML (from work metadata)
               2. Read work:<id> label
               3. Query: br list -l work:<id> --json
               4. Count: total, open, in_progress, closed
               5. Derive: not_started | in_progress | blocked | done
```

Concretely:

| Bead state | Meaning |
|---|---|
| All beads closed | Work is done |
| Some beads in_progress or open with ready deps | Work is in progress |
| All open beads have unresolved deps (from other works) | Work is blocked |
| No beads exist yet (no YAML or YAML not loaded) | Work is planned but not tasked |

The `work:<id>` label is the join key. It's cheap — `br list -l work:X --json` is a single indexed query against SQLite.

### What about works without beads?

Works that haven't been decomposed into tasks yet have no beads. kerf knows about them from its own work metadata (spec.yaml or equivalent). Their status is "planned" or "specced" — kerf tracks this from its own state (jig pass completion). Once decomposition happens and the YAML is loaded, beads takes over status tracking for the implementation phase.

Work lifecycle from kerf's perspective:

```
draft → specced → tasked → in_progress → done → archived
  ↑        ↑         ↑          ↑            ↑
  kerf    kerf    kerf+beads   beads       kerf
  owns    owns    handoff      owns        owns
```

---

## 4. Large task sets

### The harmonik scale problem

harmonik had 8 specs producing ~400+ beads with ~2000+ edges across 8 YAML files. Key lessons:

**Decomposition is spec-scoped.** Each YAML file corresponds to one spec. The agent decomposes one spec at a time. Cross-spec edges reference other specs' beads by mnemonic prefix. This keeps each decomposition manageable.

In kerf terms: each work produces one task YAML. Cross-work edges reference other works' beads. The decomposition agent needs to see the current work's spec plus the mnem-maps of already-decomposed works (to know what to reference).

**Validation is critical at scale.** The harmonik loader validated:
- Mnemonic uniqueness within a file
- Dependency cycle detection (via `br dep cycles`)
- Cross-spec reference resolution (is `em-040` actually a real bead?)
- Forward-deferred edge logging (acknowledged missing references)

kerf should define a `kerf validate-tasks` command (or fold this into the load process) that checks:
- Schema conformance of the YAML
- Intra-work cycle detection
- Cross-work reference resolution against mnem-maps
- Work/area tag consistency with kerf's registry

**Incremental updates are append-only.** In harmonik, YAML files were versioned (v0.1.0 → v0.1.1 → v0.1.2). New beads were added, edges were added or removed. The loader used the mnem-map as a ledger — beads already in the map were skipped. This made re-runs safe.

For kerf: when a work's spec changes mid-implementation (Problem 4 — late-arriving requirements), the task YAML gets a new version. New beads are appended. New edges are added. The loader only creates what's missing. Existing beads are untouched (their status in beads is preserved).

### Generating good YAML from specs

The decomposition agent needs:
1. The spec being decomposed (the primary input)
2. The decomposition discipline/rules (what kinds of beads to mint, how to handle edge cases)
3. Mnem-maps from already-decomposed works (for cross-work edges)
4. The kerf area registry (to tag beads correctly)

kerf's job is to assemble this context and hand it to the agent. The agent produces the YAML. kerf validates it. The loader ingests it.

This is a natural `kerf` command:

```
kerf decompose <work-id>
  → assembles context (spec, discipline, mnem-maps, areas)
  → agent produces YAML
  → kerf validates YAML
  → kerf invokes loader (or prints load command)
```

### Inter-work edges are the hard part

Intra-work edges are straightforward — the agent can see all the beads and reason about their relationships. Inter-work edges require knowing what beads exist in OTHER works.

Three cases:

1. **Other work already decomposed.** Mnem-map exists. Agent references beads by mnemonic. Loader resolves them. This is the happy path.

2. **Other work not yet decomposed.** Use `forward:<prefix>-<description>` edges. These are logged but not loaded. When the other work is decomposed, a reconciliation pass materializes the edges. This is exactly what harmonik did.

3. **Bidirectional dependency discovered during decomposition.** Work A references work B, and decomposing B reveals it also needs something from A. This surfaces as a cycle or as forward-deferred edges in both directions. The resolution is architectural — it means the two works need to be co-designed (Problem 2). kerf's `co-designs` relationship type captures this.

---

## 5. Optional beads integration

### Project-level setting

```yaml
# ~/.kerf/projects/{project-id}/project.yaml
jigs:
  - plan
  - spec
  - bug
tools:
  task_tracker: beads        # or: none, github-issues, linear, ...
beads:
  label_prefix: "work"       # label scheme for work→bead mapping
  mnem_map_dir: ".beads/mnem-maps"
```

### Behavior when enabled vs. disabled

| Capability | beads enabled | beads disabled |
|---|---|---|
| `kerf map` shows work status | Queries `br list` for task-level status | Shows kerf-only status (draft/specced/planned) |
| `kerf next` considers task deps | Reads `bv --robot-triage` for unblocked work | Uses only work-level `depends_on` |
| `kerf decompose` | Produces task YAML, validates, loads | Not available (or produces YAML without loading) |
| Work completion | Derived from all beads closed | Manual `kerf close` |
| Cross-work dependencies | Modeled as inter-work bead edges | Modeled as work-level `depends_on` only |

### The interface boundary

kerf talks to beads through exactly three channels:

1. **Write: the task YAML + loader.** kerf produces YAML; the loader creates beads and edges. This is the only direction kerf writes to beads.

2. **Read: `br list`/`bv` queries.** kerf reads bead status by label. The `work:<id>` label is the join key. kerf never reads individual bead details — it only needs counts and aggregate status.

3. **Read: mnem-maps.** kerf reads mnem-map CSVs to resolve cross-work references during decomposition. These are filesystem artifacts, not API calls.

This keeps the coupling loose. beads doesn't know kerf exists. kerf knows beads exists only through the project config, and all beads-specific code is behind the `tools.task_tracker == "beads"` gate.

---

## 6. Harmonik queue feeding

### What harmonik needs from kerf

harmonik's execution model: a queue of tasks, dispatched to agents. An orchestrator agent reads the queue and assigns work. The question is: what feeds the queue?

```
kerf next  →  orchestrator agent  →  harmonik queue  →  worker agents
```

`kerf next` provides:
- Which work to focus on (work-level selection)
- Why this work is next (dependency chain, blocking relationships)
- What specs to read (spec paths from work metadata)

The orchestrator agent then:
1. Reads the spec
2. Checks if task YAML exists for this work
3. If not: runs decomposition (or asks kerf to)
4. If yes: queries `bv --robot-triage -l work:<id>` for ready beads
5. Pushes ready beads into harmonik's execution queue

### The `kerf next` output format

```json
{
  "work_id": "adapter-retry",
  "title": "Adapter retry logic",
  "reason": "unblocks: [metrics-integration, load-testing]",
  "specs": ["specs/adapter.md"],
  "areas": ["adapter", "resilience"],
  "task_status": {
    "total": 12,
    "closed": 3,
    "open": 7,
    "blocked": 2,
    "ready": 5
  },
  "task_yaml": "works/adapter-retry/tasks.yaml",
  "suggested_action": "continue_implementation"
}
```

`suggested_action` is one of:
- `needs_decomposition` — spec exists but no task YAML yet
- `continue_implementation` — tasks exist, some are ready
- `blocked` — all remaining tasks depend on other works
- `needs_review` — all tasks done, work needs verification
- `co_design_needed` — overlapping work detected, resolve first

This gives the orchestrator agent enough to decide what to do without reading every spec and querying every bead itself.

---

## 7. Failure modes and mitigations

**Stale mnem-maps.** If work A's mnem-map is outdated when work B is being decomposed, cross-work edges may reference stale or missing beads. Mitigation: the loader validates cross-work references at load time and rejects unresolvable edges (same as harmonik).

**Decomposition drift.** The spec changes after decomposition. The task YAML no longer matches the spec. Mitigation: kerf tracks spec version in the YAML's `work` block. When the spec is modified, kerf can flag "tasks may be stale" in `kerf map`. Re-decomposition produces a new YAML version; the loader only adds new beads.

**Label pollution.** If `work:<id>` labels aren't consistently applied, the join between kerf and beads breaks. Mitigation: the loader enforces `default_labels` — every bead gets the work label automatically. Manual `br create` calls bypass this, but that's an agent discipline issue, not a tooling issue.

**Scale limits.** `br list -l work:X --json` is O(beads in work), not O(all beads). The `work:` label scoping keeps queries fast even with thousands of total beads across dozens of works. `bv` indexes by label, so analytical queries are also scoped.

**Cycle detection across works.** Intra-work cycles are caught by `br dep cycles` at load time. Cross-work cycles (work A bead depends on work B bead which depends on work A bead) are caught when both works are loaded — `br dep cycles` operates on the full graph. The risk is that the cycle isn't detected until the second work is loaded. Mitigation: the YAML validation step can detect potential cross-work cycles by analyzing forward-deferred edges across all YAML files before loading.

---

## 8. Summary: what to build

Ordered by dependency:

1. **Task YAML schema.** Define the schema (based on harmonik's proven format). This is a spec artifact — it tells agents what to produce.

2. **`work:<id>` label convention.** Document the convention that every bead from a kerf work carries this label. This is the fundamental join key.

3. **`tools.task_tracker` project setting.** Add to project.yaml. Gates all beads-specific behavior.

4. **Status query in `kerf map`.** When beads enabled: query `br list -l work:<id> --json` for each work, compute aggregate status.

5. **`kerf next` with beads awareness.** When beads enabled: factor task-level readiness (via `bv`) into work selection.

6. **`kerf decompose` command.** Assembles context, triggers agent decomposition, validates output YAML, optionally invokes loader.

7. **Cross-work edge tooling.** Mnem-map management, forward-deferred edge reconciliation, cross-work cycle detection.

Items 1-3 are foundational. Items 4-5 deliver the status feedback loop. Item 6 completes the pipeline. Item 7 handles scale.
