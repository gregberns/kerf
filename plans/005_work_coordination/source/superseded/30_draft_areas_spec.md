# Areas

> System topology graph: the defined taxonomy of system components, their hierarchy, and how works reference them.

## What Areas Are

An **area** is a named component or region of the system. Areas form the project's architectural topology — the map of what the system is made of. They are not freeform tags; they are a defined taxonomy maintained in a single file.

Areas serve three roles:

1. **Planning.** When creating a work, the agent declares which areas it touches. This enables overlap detection: if two works touch the same area, kerf warns about potential conflicts.
2. **Execution.** Tasks derived from works inherit area membership. The queue computation can group tasks by area for context efficiency.
3. **Visibility.** Area-based queries answer questions like "which parts of the system have the most active work?" and "which areas have no spec coverage?"

Areas describe the system, not the work. A work *touches* areas; it does not *own* them. Areas exist independently of any work — they are a property of the project.

## The Taxonomy File

### Location

The area taxonomy lives at:

```
~/.kerf/projects/{project-id}/areas.yaml
```

This is a project-level artifact, alongside `project.yaml`. It is kerf's working data, not source code. It may be finalized into the repository if desired.

### Schema

```yaml
# ~/.kerf/projects/{project-id}/areas.yaml

areas:
  auth:
    description: "Authentication, identity, token management"
  adapter:
    description: "External service integration layer"
  adapter.retry:
    description: "Retry logic for adapter calls"
    parent: adapter
  adapter.pool:
    description: "Connection pooling for adapters"
    parent: adapter
  database:
    description: "Persistence layer, migrations, query execution"
  api:
    description: "HTTP/REST interface, routing, middleware"
  core:
    description: "Domain logic, business rules"
```

### Field Reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `description` | string | yes | What this area is. One line. |
| `parent` | string | no | Name of the parent area. Must reference an existing area in the same file. |

Area names are the YAML keys under `areas:`. They must be lowercase alphanumeric characters, hyphens, and dots only (matching `[a-z0-9]+([.-][a-z0-9]+)*`). Dots are a naming convention for sub-areas but do not imply hierarchy on their own — the `parent` field is the explicit hierarchy mechanism.

### Missing File Behavior

If `areas.yaml` does not exist when a command requires area validation, kerf errors with guidance: "No areas defined. Define your system areas with `kerf areas init`."

kerf does not create `areas.yaml` automatically. The taxonomy is an intentional artifact that requires thought.

## Hierarchy

Areas form a tree through `parent` references. A child area has exactly one parent. The root areas (those with no `parent`) are the top-level components of the system.

### Properties

- **Depth limit.** The hierarchy has a maximum depth of 4 levels. kerf rejects area additions that would exceed this depth. Deeper hierarchies indicate the taxonomy is modeling implementation details rather than architectural components.
- **Rollup.** A work touching `adapter.retry` is implicitly associated with `adapter`. Overlap detection, heat mapping, and grouping all roll up through the hierarchy. A work explicitly tagged `adapter` and a work tagged `adapter.retry` are considered overlapping.
- **No orphans.** If a `parent` references an area that does not exist in `areas.yaml`, kerf rejects the file as invalid.

### Validation

kerf validates the hierarchy on any mutation to `areas.yaml`:

- No cycles (a parent chain must terminate at a root area).
- No orphan parent references.
- Maximum depth of 4.

## Area References in Works

Works declare which areas they touch in `spec.yaml`:

```yaml
# In spec.yaml, alongside existing fields
areas:                    # list<string>, optional, mutable
  - adapter
  - adapter.retry
```

### Validation

When a work's `areas` field is set or modified (via `kerf new`, `kerf set`, or any command that updates `spec.yaml`):

- Every area name must exist in `areas.yaml`.
- If any area is unknown, the command errors with the list of valid areas and instructions to add new ones.

This validation is the mechanism that ensures all agents agree on the system structure. No freeform strings, no drift.

### Inheritance Through Hierarchy

A work tagged with `adapter.retry` is implicitly associated with `adapter` (its parent) and any further ancestors. The `areas` field in `spec.yaml` lists only the directly specified areas; ancestor association is computed at query time.

Works should tag the most specific area they touch. Tagging a parent area means the work touches the area broadly, not a specific sub-area.

## Overlap Detection

kerf identifies works touching the same areas and warns about potential conflicts. Overlap detection runs at two points:

1. **At work creation** (`kerf new`). If the new work's areas overlap with any active work's areas, kerf emits a warning listing the overlapping works.
2. **On demand** (`kerf map`). The map view shows area-based grouping and highlights areas with multiple active works.

### What Counts as Overlap

Two works overlap when their area sets intersect after hierarchy rollup. Specifically:

- **Direct overlap.** Both works list the same area.
- **Parent-child overlap.** One work lists `adapter`, another lists `adapter.retry`. Because `adapter.retry` rolls up to `adapter`, these overlap.

Overlap is a **warning**, not a block. Two works may legitimately touch the same area. The warning ensures the agent is aware of potential conflicts and can coordinate.

### Overlap Output

When overlap is detected, kerf emits:

```
Warning: area overlap detected
  {codename-1} ({status}) — {area}
  {codename-2} ({status}) — {area}
```

The output includes the overlapping works' codenames, statuses, and the shared areas. This gives the agent enough context to decide whether coordination is needed.

## Area Queries

The area system answers these questions:

### Heat Map — "Where is the work concentrated?"

Count active works per area, rolled up through the hierarchy. An area with many active works is "hot" — it may need more coordination or may be a bottleneck.

Surfaced via `kerf map`:

```
Areas (6 defined, 4 with active work)

  adapter [3 works]
    adapter.retry [1 work: brave-falcon]
    adapter.pool [1 work: green-oak]
    (direct: bold-crane)
  auth [1 work: red-wave]
  database [1 work: calm-river]
  api [no active work]
  core [no active work]
```

### Overlap — "Are works competing for the same areas?"

Answered at `kerf new` time (see Overlap Detection above) and visible in `kerf map` output.

### Coverage — "What parts of the system have no active work?"

Areas with no active works are visible in the heat map. This is informational — not every area needs active work at all times.

### Area Listing — "What is the system made of?"

`kerf areas list` shows the full taxonomy with hierarchy, descriptions, and active work counts.

## Lifecycle

### Adding Areas

Areas are added via `kerf areas add`:

```
kerf areas add adapter.circuit-breaker --parent adapter --description "Circuit breaker pattern for adapter calls"
```

The command validates:

- The name matches the allowed pattern.
- No area with this name already exists.
- If `--parent` is specified, the parent area exists and the resulting depth does not exceed 4.

Agents may add areas when the existing taxonomy does not cover a region they need. The taxonomy grows over time as the system grows. Adding is non-destructive and does not affect existing works.

### Renaming Areas

Areas are renamed via `kerf areas rename`:

```
kerf areas rename old-name new-name
```

The command performs a coordinated update:

1. Renames the area in `areas.yaml`.
2. Updates all `parent` references pointing to the old name.
3. Updates all `spec.yaml` files in the project that reference the old name in their `areas` list.

Rename is the only way to change an area's name. Direct editing of `areas.yaml` is valid but risks orphaning references in `spec.yaml` files.

### Removing Areas

Areas are removed via `kerf areas remove`:

```
kerf areas remove old-area
```

The command checks:

- **No active works reference this area.** If any non-archived work lists the area in its `areas` field, the removal is rejected with a list of the referencing works.
- **No children.** If the area has children, the removal is rejected. Remove or reparent children first.

Removal is destructive. kerf does not soft-delete areas.

### Stale Areas

An area is stale when no work has referenced it for an extended period. kerf does not automatically remove stale areas. The taxonomy is a deliberate model of the system — an area with no current work is still a valid part of the architecture.

`kerf areas list` can surface staleness by showing areas with zero active works, letting the user decide whether to prune.

## Bootstrap

### New Projects

`kerf areas init` prompts for or accepts initial areas. This is typically run once, early in the project, when the major components are known.

```
kerf areas init
```

If `areas.yaml` already exists, `kerf areas init` errors. Use `kerf areas add` to extend the taxonomy.

### Existing Projects Without Areas

Projects that have been using kerf without areas continue to function. The `areas` field in `spec.yaml` is optional. Commands that depend on areas (overlap detection, heat mapping) are skipped when `areas.yaml` does not exist.

When the project is ready to adopt areas, the agent or user defines the taxonomy with `kerf areas init` and begins tagging works. Existing works are not required to be retroactively tagged.

## Edges (Deferred)

The area data model described here covers nodes (areas) and hierarchy (parent-child). Typed directed edges between areas (e.g., `api calls auth`, `adapter reads database`) are a planned extension. The current schema does not include edges. When edges are added, `areas.yaml` will gain an `edges:` section and commands like `kerf areas impact` will use them for adjacency-based overlap detection and blast radius computation.

This deferral is intentional. Hierarchy is immediately useful for grouping and rollup. Edges require more thought about maintenance burden and query design. The data model will support them when they are added without migrating existing `areas.yaml` files.

## Relationship to Specs

The `specs/` directory and the area graph are related but independent artifacts:

- **Specs** are normative documents describing what the system does.
- **Areas** are architectural topology describing what the system is made of.

A spec may cover one area, multiple areas, or cross-cutting concerns. An area may be covered by one spec, multiple specs, or no spec. The connection is informational, not structural — areas do not reference spec files, and specs do not reference areas.

## Key Invariants

1. Areas are a defined taxonomy, not freeform strings. Works can only reference areas that exist in `areas.yaml`.
2. The taxonomy is a project-level artifact, stored alongside `project.yaml`.
3. Agents may add areas but must use existing names when they exist.
4. Hierarchy rollup is computed at query time from the `parent` field. The `areas` field in `spec.yaml` stores only directly specified areas.
5. Overlap detection is a warning, not a gate. Works may legitimately touch the same areas.
6. The taxonomy is append-friendly: adding areas is cheap, removing requires no active references.
