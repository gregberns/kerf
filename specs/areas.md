# Areas

> Named regions of the system forming the project's topology. Used for overlap detection and coordination.

## What Areas Are

An **area** is a named component or region of the system. Areas form the project's architectural topology — the map of what the system is made of. They are a defined taxonomy maintained in a single file, not freeform tags.

Areas describe the system, not the work. A work *touches* areas; it does not *own* them. Areas exist independently of any work — they are a property of the project.

Areas serve two roles:

1. **Coordination.** When creating a work, the agent declares which areas it touches. If two works touch the same area, kerf flags the overlap so the agent can decide whether coordination is needed.
2. **Visibility.** Area-based queries answer questions like "which parts of the system have active work?" and "where is work concentrated?"

## The Areas File

### Location

The area taxonomy lives at:

```
~/.kerf/projects/{project-id}/areas.yaml
```

This is a project-level artifact, alongside `project.yaml`. It is kerf's working data, not source code.

### Schema

```yaml
# ~/.kerf/projects/{project-id}/areas.yaml

areas:
  auth:
    description: "Authentication, identity, token management"
  adapter:
    description: "External service integration layer"
  database:
    description: "Persistence layer, migrations, query execution"
  api:
    description: "HTTP/REST interface, routing, middleware"
  core:
    description: "Domain logic, business rules"
```

Area names are the YAML keys under `areas:`. Names are lowercase alphanumeric characters and hyphens (matching `[a-z0-9]+(-[a-z0-9]+)*`).

Each area has a `description` field — a short line explaining what the area covers.

Phase 1 areas are flat. There is no hierarchy, no parent field, no nesting. Hierarchy is a future extension.

### Missing File

If `areas.yaml` does not exist, commands that use areas (overlap detection, area queries) are simply skipped. Works continue to function without areas.

When the project is ready to adopt areas, define the taxonomy with `kerf areas init` and begin tagging works. Existing works do not need to be retroactively tagged.

## Area References in Works

Works declare which areas they touch in `spec.yaml`:

```yaml
# In spec.yaml, alongside existing fields
areas:                    # list<string>, optional, mutable
  - adapter
  - database
```

When `areas.yaml` exists and a work's `areas` field is set or modified, kerf checks that each area name exists in `areas.yaml`. If an area name is unknown, the command warns with the list of valid areas.

The `areas` field is optional. Works without areas are valid and unaffected by area-related features.

## Overlap Detection

kerf identifies works that touch the same areas and flags potential conflicts.

Two works overlap when their area lists share at least one area name.

Overlap detection runs at several points:

1. **At work creation** (`kerf new`). If the new work's areas overlap with any active work's areas, kerf emits a warning listing the overlapping works.
2. **On demand** (`kerf map`). The map view shows area-based grouping and highlights areas with multiple active works.
3. **When inspecting a work** (`kerf show`, `kerf resume`). Both commands surface the same area-overlap information so an agent inspecting or reentering work sees adjacent active work in the same areas.

Overlap is a **warning**, not a block. Two works may legitimately touch the same area. The warning ensures the agent is aware and can coordinate if needed.

### Overlap Output

When overlap is detected, kerf emits:

```
Warning: area overlap detected
  {codename-1} ({status}) — {area}
  {codename-2} ({status}) — {area}
```

## Area Queries

### List Areas

`kerf areas list` shows all defined areas with descriptions and active work counts.

```
Areas (5 defined, 3 with active work)

  adapter [2 works: bold-crane, green-oak]
  auth [1 work: red-wave]
  database [1 work: calm-river]
  api [no active work]
  core [no active work]
```

### Map View

`kerf map` includes area information when `areas.yaml` exists. It shows which areas have active work and where overlap exists. Areas with no active work are listed for completeness.

### Works Per Area

Any area listing shows the active works touching that area, giving a quick view of where effort is concentrated.

## Lifecycle

### Creating the Taxonomy

`kerf areas init` creates `areas.yaml` with an initial set of areas. This is typically run once, early in the project.

If `areas.yaml` already exists, `kerf areas init` warns and does nothing.

### Adding Areas

`kerf areas add {name} --description "..."` adds a new area to `areas.yaml`. The name must match the allowed pattern and not already exist.

Adding areas is non-destructive and does not affect existing works.

### Removing Areas

`kerf areas remove {name}` removes an area from `areas.yaml`. If any active (non-archived) work references the area, kerf warns and lists them. The user can proceed or cancel.

### Renaming Areas

`kerf areas rename {old} {new}` renames an area in `areas.yaml` and updates all `spec.yaml` files in the project that reference the old name.

## Relationship to Specs

The `specs/` directory and areas are related but independent:

- **Specs** are normative documents describing what the system does.
- **Areas** are architectural topology describing what the system is made of.

A spec may cover one area, multiple areas, or cross-cutting concerns. An area may be covered by multiple specs or none. Areas do not reference spec files, and specs do not reference areas.

## Future Extensions

Hierarchy (parent-child relationships between areas) and typed edges between areas (e.g., "api calls auth") are planned extensions. Phase 1 is flat areas only. The schema will accommodate these additions without migrating existing `areas.yaml` files.
