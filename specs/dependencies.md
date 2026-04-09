# Dependencies

> How works declare and resolve dependencies on other works.

## Declaring Dependencies

Works declare dependencies in the `depends_on` field of [spec.yaml](works.md). Each entry identifies a target work and the nature of the relationship.

### Schema

```yaml
depends_on:
  - codename: database-migration        # string, required — codename of the dependency work
    project: acme-webapp                 # string, optional — project ID of the dependency
    relationship: must-complete-first    # string, required — relationship type
  - codename: auth-service-spec
    relationship: inform
```

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `codename` | string | yes | Codename of the dependency work. |
| `project` | string | no | Project ID of the dependency. Omit for same-project dependencies. |
| `relationship` | string | yes | One of the defined relationship types. |

The `depends_on` list is mutable. Dependencies can be added or removed at any time during the work's lifecycle.

## Relationship Types

### `must-complete-first`

The dependency must reach a completed status before the dependent work should be finalized. This relationship:

- Causes [finalization](finalization.md) to warn if the dependency is not complete
- Causes [verification](verification.md) to check dependency status
- Signals to orchestrators that the dependency should be implemented first

### `inform`

The dependency provides relevant context but does not block progress. This relationship:

- Does not trigger warnings at finalization or verification
- Signals that the dependency's artifacts should be read for context, not that it must be finished first

## Same-Project Dependencies

When the `project` field is omitted, the dependency is within the same project as the dependent work. kerf resolves same-project dependencies by looking up the codename under the current project's directory on the [bench](architecture.md):

```
~/.kerf/projects/{current-project-id}/{codename}/spec.yaml
```

## Cross-Project Dependencies

When the `project` field is present, it contains the [project ID](architecture.md) of the dependency work's project. kerf resolves cross-project dependencies by looking up the codename under the specified project's directory:

```
~/.kerf/projects/{project-id}/{codename}/spec.yaml
```

Cross-project dependencies require that the target project's works exist on the local bench. If the target project directory does not exist on the bench, the dependency is **unresolvable** — kerf records the reference but cannot check the dependency's status.

## Dependency Resolution

When kerf needs to check a dependency's status (during [verification](verification.md), [finalization](finalization.md), or context loading), it performs the following resolution:

1. **Locate the dependency.** Determine the bench path using the `codename` and `project` fields (same-project if `project` is omitted, cross-project otherwise).
2. **Read the dependency's `spec.yaml`.** Extract its `status` field.
3. **Determine completeness.** A dependency is **complete** if its status is at or past the last value in its `status_values` list (typically `ready`). "At or past" means the status either equals the terminal value, or does not appear in `status_values` at all (e.g., `finalized`, `implementing`). A dependency whose status appears in `status_values` at a position before the terminal value is **incomplete**.

### Unresolvable Dependencies

A dependency is unresolvable when:

- The target work directory does not exist on the bench
- The target work's `spec.yaml` cannot be read

Unresolvable dependencies are reported but do not cause errors. Commands that check dependencies note them as unresolvable and continue.

## What Dependencies Enable

Dependencies are informational and advisory. kerf does not enforce ordering or prevent work on items with incomplete dependencies. Dependencies enable:

- **Visibility.** [Commands](commands.md) that list works display dependency status inline, so agents and users can see the state of related works at a glance.
- **Context loading.** When resuming a work, kerf can load dependent works' current state so the agent knows what is decided vs. still in flux. See [sessions](sessions.md).
- **Ordering.** External orchestrators can read the dependency graph to build a DAG and determine implementation order. See [future.md](future.md) for orchestrator integration.
- **Blocking warnings.** [Finalization](finalization.md) warns when `must-complete-first` dependencies are incomplete. [Verification](verification.md) checks dependency status as part of its structural checks.
