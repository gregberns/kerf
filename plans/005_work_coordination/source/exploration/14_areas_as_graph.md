# Areas as Architectural Graph

> Deep analysis of Greg's feedback: areas should be a defined graph with relationships, not freeform tags on work items.

---

## The Shift

The brainstorming produced a good idea — area tags on works — but treated it as a coordination mechanism: "these works touch the same area, so they might conflict." Greg's feedback reframes areas as something more fundamental: **the areas ARE the system architecture, and the graph of their relationships IS the system map.**

This is a different thing. Tags answer "what does this work touch?" A graph answers "what is this system made of, and how do the parts connect?" The first is a property of work items. The second is a property of the project itself.

---

## What the Graph Represents

### Nodes: System Areas

A node is a named component/region of the system. Not a file, not a package — a conceptual area that an agent or human would recognize as "a part of the system."

Examples for a web application:
- `auth` — authentication and identity
- `adapter` — external service integration layer
- `database` — persistence and migrations
- `api` — HTTP/REST interface
- `core` — domain logic

Examples for kerf itself:
- `bench` — workspace storage and layout
- `jig-system` — jig resolution, passes, composability
- `works` — work lifecycle, spec.yaml, status
- `cli` — command parsing, output formatting
- `finalization` — bench-to-repo transfer
- `sessions` — session tracking, shelving, resuming
- `snapshots` — versioning, history
- `dependencies` — work dependency graph

### Edges: How Areas Relate

Edges are typed and directed. The minimum useful set:

| Edge type | Meaning | Example |
|-----------|---------|---------|
| `calls` | A invokes B at runtime or build time | `api` → `auth` (API calls auth to validate tokens) |
| `reads` | A reads data owned by B | `adapter` → `database` (adapter reads connection config) |
| `owns` | A contains B (hierarchical) | `cli` → `cli.commands`, `cli` → `cli.output` |

The `owns` edge gives hierarchy without requiring a separate hierarchy mechanism. `adapter` owns `adapter.retry` and `adapter.pool` — but those are still nodes in the graph, not a naming convention.

Why directed? Because "A calls B" and "B calls A" are different architectural facts. If the adapter calls the database but the database also calls the adapter, that's a cycle — and that's meaningful information.

### What Edges Are NOT

Edges are not dependency declarations between work items. That already exists in `depends_on`. Edges are structural facts about the system: "the API layer talks to the auth layer." This is true regardless of what work items exist.

---

## Where the Graph Lives

### Location: `areas.yaml` in project config

The graph is a project-level artifact, not a work-level one. It lives alongside `project.yaml`:

```
~/.kerf/projects/{project-id}/
  project.yaml           # existing — jig config, tool declarations
  areas.yaml             # NEW — the system area graph
```

Why here and not in the repo? Same reasoning as the rest of the bench — it's kerf's working data, not source code. It could be finalized into the repo if desired (like work artifacts), but its primary home is the bench.

### Schema

```yaml
# ~/.kerf/projects/{project-id}/areas.yaml

# Areas are the nodes of the system graph.
# Each key is an area name (the canonical identifier).
areas:
  auth:
    description: "Authentication, identity, token management"
  adapter:
    description: "External service integration layer"
  adapter.retry:
    description: "Retry logic for adapter calls"
    parent: adapter                          # sugar for an 'owns' edge
  adapter.pool:
    description: "Connection pooling for adapters"
    parent: adapter
  database:
    description: "Persistence layer, migrations, query execution"
  api:
    description: "HTTP/REST interface, routing, middleware"
  core:
    description: "Domain logic, business rules"

# Edges are the relationships between areas.
# Each edge is directed: from → to.
edges:
  - from: api
    to: auth
    type: calls
    note: "API middleware validates tokens via auth"
  - from: api
    to: core
    type: calls
  - from: core
    to: database
    type: calls
  - from: adapter
    to: database
    type: reads
    note: "Reads connection config"
```

### Why YAML, Not Derived from Specs

The area graph could theoretically be derived from the `specs/` directory structure. But the mapping is lossy in both directions:

- Not every spec file corresponds to an area (e.g., `_index.md`, `future.md`).
- Not every area corresponds to a spec file (areas can be more granular or more abstract).
- The relationships between areas aren't encoded in the spec directory structure at all.

The graph is its own artifact. It may be informed by specs, but it's authored and maintained independently.

---

## How the Graph Is Created and Maintained

### Bootstrap: `kerf init` or `kerf areas init`

When a project is initialized, the agent (or user) defines the initial area graph. This could happen:

1. **At `kerf init` time.** The init process already sets up `project.yaml`. It could also prompt for initial areas and create `areas.yaml`.
2. **On first `kerf new` that wants areas.** If `areas.yaml` doesn't exist when an area tag is needed, kerf errors with guidance: "Define your system areas first with `kerf areas init`."

Option 2 is better — it follows kerf's pattern of erroring with guidance rather than gating behind a setup wizard. Greg's feedback aligns: "if there aren't areas in place, maybe we default to throwing an error, then the agent can define them."

### Evolution: Adding Areas

An agent creating a work needs to tag it with areas from the defined set. If the needed area doesn't exist:

1. The agent proposes a new area (name + description + parent if hierarchical + edges).
2. kerf adds it to `areas.yaml`.
3. The new area is immediately available for all works.

This could be a command: `kerf areas add adapter.circuit-breaker --parent adapter --description "Circuit breaker pattern for adapter calls"` and `kerf areas link adapter.circuit-breaker adapter.retry --type calls --note "Circuit breaker wraps retry logic"`.

Or it could be direct YAML editing. For agents, a command is better — it validates the name, checks for duplicates, and ensures the graph stays consistent.

### Evolution: Renaming and Merging

This is the hard problem. If agent A uses `auth-adapter` and agent B uses `authentication-adapter`, we have drift. The defined taxonomy prevents this — you can't use an area that doesn't exist in `areas.yaml`. But what if the taxonomy itself needs fixing?

Rename is a graph mutation: change the node name, update all edges referencing it, update all work items tagged with it. This is a `kerf areas rename old-name new-name` command that does a coordinated update across `areas.yaml` and all `spec.yaml` files in the project.

### Coherence Enforcement

The key design decision: **works can only use areas that exist in `areas.yaml`.** When an agent runs `kerf new` and specifies areas:

- If all areas exist in `areas.yaml`: proceed.
- If any area is unknown: error with the list of valid areas and instructions to add new ones.

This is the mechanism that ensures all agents agree on the system structure. The taxonomy is the single source of truth. No freeform strings, no drift.

---

## How the Graph Connects to Work Items

Works reference areas by name in their `spec.yaml`:

```yaml
# in spec.yaml, alongside existing fields
areas:                    # list<string>, optional, mutable
  - adapter
  - adapter.retry
```

The area names must exist in `areas.yaml`. This is validated at `kerf new` time and at `kerf square` time.

### What This Enables That Tags Don't

With a flat tag model, "adapter" and "database" are just strings. You can check for set intersection (these works share an area) but nothing else.

With the graph, you can compute:

**Adjacency-based overlap.** Two works don't share an area, but one touches `api` and the other touches `auth`. The graph knows `api` calls `auth`. These works are adjacent — not overlapping, but potentially interacting. The overlap warning can include: "These works touch areas that interact."

**Blast radius.** "If I change `adapter`, what else might be affected?" Walk the graph: anything that `calls` or `reads` adapter. That's `api` (calls) and `core` (calls through api). A work touching `adapter` should know about works touching `api` and `core`.

**Hierarchy-aware grouping.** A work tagged `adapter.retry` is implicitly in the `adapter` area. `kerf map` can group by parent area, showing all adapter-related works together even if they're tagged with different sub-areas.

**Heat mapping.** "Which area of the system has the most active work?" Count works per area, roll up to parents. Three works touching `adapter.*` sub-areas means the adapter is hot — more coordination needed there.

---

## Queries the Graph Enables

### For `kerf map`

```
## System Areas (6 areas, 4 with active work)

  adapter [3 works]
    ├─ adapter.retry [1 work: brave-falcon]
    └─ adapter.pool [1 work: green-oak]
    └─ (area-level: bold-crane)
  auth [1 work: red-wave]
  database [1 work: calm-river]
  api [no active work]
  core [no active work]

## Area Interactions with Active Work

  adapter ← api (calls) — no work on api, but changes to adapter may affect api consumers
  adapter → database (reads) — calm-river touches database, coordinate with adapter works
```

### For `kerf new` (overlap + adjacency warnings)

```
$ kerf new --type plan --title "Circuit breaker" --areas adapter

Created: bold-crane (plan)

⚠ Area overlap:
  brave-falcon (implementing) — adapter.retry
  green-oak (research) — adapter.pool

⚠ Adjacent areas with active work:
  calm-river (ready) — database ← adapter reads database
```

### For impact analysis

```
$ kerf areas impact adapter

Direct: adapter, adapter.retry, adapter.pool (3 sub-areas)
Upstream (calls adapter): api
Downstream (adapter calls): database

Works in blast radius:
  brave-falcon — adapter.retry (direct)
  green-oak — adapter.pool (direct)
  bold-crane — adapter (direct)
  calm-river — database (downstream)
```

### For "what has no spec coverage?"

If specs are mapped to areas (even loosely), the graph can show areas with no corresponding spec. This is a gap-finder: "the adapter.pool area has no spec but has an active work — the work is designing something with no architectural anchor."

---

## How Sophisticated for v1?

The spectrum Greg identified:

| Level | What it is | What it enables |
|-------|-----------|-----------------|
| Simple | Flat list of area names | Set-intersection overlap detection |
| Medium | Hierarchy (parent/child) | Grouped display, roll-up counts |
| Rich | Directed graph with typed edges | Adjacency warnings, blast radius, impact analysis |

**Recommendation: start with Medium-plus.**

Define `areas.yaml` with:
- Named areas with descriptions
- Optional `parent` field for hierarchy
- A single edge list with `from`, `to`, `type`

But only implement hierarchy-aware features in v1 commands. Edge-based queries (adjacency, blast radius) are computed from the same data but surfaced later — the data model supports them from day one, the commands grow into them.

The reasoning: hierarchy is immediately useful (grouping in `kerf map`, parent-area rollup for overlap). Edges require more thought about what queries are valuable and how to present them without overwhelming the agent. But if the data model includes edges from the start, we don't have to migrate later.

### What "Medium-plus" Means Concretely

**v1 commands use:**
- Area validation against `areas.yaml` (coherence enforcement)
- Hierarchy for grouping and rollup in `kerf map`
- Direct overlap detection (same area or parent-child) in `kerf new`

**v1 data model supports but commands don't yet use:**
- Typed directed edges between areas
- Adjacency-based overlap detection
- Blast radius computation

**v2 adds:**
- `kerf areas impact <area>` — walks edges to show blast radius
- Adjacency warnings in `kerf new` — "these works touch areas that interact"
- Edge-aware grouping in `kerf map`

---

## Relationship to Specs

The `specs/` directory and the area graph are related but independent:

- **Specs are normative documents.** They describe what the system does.
- **Areas are architectural topology.** They describe what the system is made of and how the parts connect.

A spec may cover one area, multiple areas, or cross-cutting concerns that don't map cleanly to areas. An area may be covered by one spec, multiple specs, or no spec yet.

The connection is informational, not structural. An optional `spec` field on an area node could point to the relevant spec file:

```yaml
areas:
  auth:
    description: "Authentication, identity, token management"
    spec: "specs/auth.md"                    # optional, informational
```

This enables the "what areas have no spec coverage?" query without coupling the two systems tightly.

---

## Prior Art and Influences

**C4 model.** C4 defines four levels: context, containers, components, code. The area graph is most like C4's component level — named parts of the system with relationships. We don't need the full C4 hierarchy (system context, container boundaries) because kerf operates within a single project.

**DDD context maps.** Domain-Driven Design maps bounded contexts and their relationships (shared kernel, customer-supplier, conformist, etc.). The area graph is simpler — it doesn't model the contractual nature of relationships, just the structural fact that they exist. DDD's insight that boundaries and relationships should be explicit is directly applicable.

**Module dependency graphs (Go, Rust, etc.).** Build systems compute dependency graphs from imports. The area graph is similar in spirit but at a higher level of abstraction — it's the architecture the developer has in mind, not the architecture the compiler infers.

**Architecture Decision Records.** ADRs capture why decisions were made. The area graph captures what was decided (the structure). They're complementary — an ADR might explain why `adapter` and `auth` are separate areas.

The key lesson from all of these: **an explicit, maintained model of system structure is more valuable than an implicit one, even if it requires curation effort.** The curation cost is the price of coherence.

---

## Open Questions

1. **Granularity guidance.** How granular should areas be? "adapter" is coarse. "adapter.retry.exponential-backoff" is too fine. The right granularity is probably "the level at which different works would make independent design decisions." But this is hard to define precisely. Practical rule: if two works in the same area would never conflict, the area is too coarse. If two areas always change together, they should be one area.

2. **Edge maintenance burden.** Edges are the most powerful part of the graph but also the most maintenance-intensive. Areas change slowly (the system's major components are relatively stable). Edges change more often as the architecture evolves. Is the maintenance cost worth the query power? The medium-plus strategy hedges this: capture edges in the data model, but don't build commands that depend on them until we know they're being maintained.

3. **Multiple projects.** The area graph is per-project. But cross-project dependencies exist. If project A's `api` calls project B's `auth-service`, that's a cross-project edge. Is this in scope? Probably not for v1 — but the schema should not preclude it.

4. **Who curates?** Greg said agents should be able to add areas. But should agents also be able to add edges? Remove areas? The risk is an agent "helpfully" restructuring the graph in ways the user didn't intend. A possible rule: agents can add areas and edges (additive), but only humans can remove or rename (destructive). Or: all mutations go through commands that log what changed, so the user can review.

5. **Bootstrap for existing projects.** A new project can define areas as part of `kerf init`. But what about a project that's been using kerf without areas? The agent reads the codebase and specs, proposes an initial area graph, the user reviews and approves. This is a one-time cost that pays for itself in every subsequent session.
