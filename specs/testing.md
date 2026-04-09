# Testing

> Testing strategy, layers, coverage targets, and CI approach for kerf.

## Testing Layers

kerf uses six testing layers. Each layer targets a different category of defect.

| Layer | Scope | Speed |
|-------|-------|-------|
| Unit | Individual functions and components | Fast |
| Property-based | Serialization boundaries, invariants | Fast |
| Integration | Command sequences against real filesystem | Fast |
| End-to-end | Complete workflows including git | Slow |
| Agentic/exploratory | Agent interaction with CLI output | Expensive (LLM) |
| Fuzz | Malformed and unexpected input | Slow |

### Unit Tests

Unit tests use Go table-driven tests.

**Coverage targets:**

- YAML parsing and serialization (`spec.yaml`, `config.yaml`)
- Status transitions and validation
- Project ID derivation (git remote, directory name fallback, collision detection) — see [architecture.md](architecture.md)
- Snapshot creation and management — see [snapshots.md](snapshots.md)
- Jig file parsing — see [jig-system.md](jig-system.md)
- Codename generation (adjective-noun slugs) and validation (lowercase alphanumeric and hyphens only) — see [works.md](works.md)
- Dependency graph operations — see [dependencies.md](dependencies.md)

### Property-Based Tests

Property-based tests use Go's `testing/quick` or a compatible library (e.g., `gopter`). They focus on serialization boundaries and filesystem operations.

**Coverage targets:**

- Codename handling: special characters, unicode, length limits, path traversal attempts
- YAML round-tripping: write then read produces identical structure
- Snapshot integrity: snapshot then restore produces identical files
- Concurrent file operations: multiple works modified simultaneously
- Config merging: bench config + project config + work config — see [architecture.md](architecture.md)

### Integration Tests

Integration tests create temporary directories, run real commands, and verify filesystem state. Each test sets up a fresh bench.

**Coverage targets:**

| Area | What is verified |
|------|-----------------|
| Full lifecycle | `new` -> write files -> `shelve` -> `resume` -> `finalize` |
| `new` | Correct directory structure created for each jig type |
| `shelve` | All state preserved correctly |
| `resume` (valid) | Resumes with valid session ID |
| `resume` (invalid) | Fallback behavior with missing or stale session ID |
| `finalize` | Files copied to correct repo location, branch created |
| `list` | Output accurately reflects filesystem state |
| `show` | Displays correct information for a work |
| `status` | Updates persist correctly |
| `jig` subcommands | `list`, `show`, `save`, `load` |
| `square` | Verification checks pass and fail correctly — see [verification.md](verification.md) |
| `snapshot` | Correct point-in-time copy created — see [snapshots.md](snapshots.md) |
| `history` | Correct timeline displayed |
| Config | Bench-level and project-level config interactions — see [architecture.md](architecture.md) |

See [commands.md](commands.md) for command specifications.

### End-to-End Tests

E2E tests use a Go test harness (or shell scripts) that sets up real git repos, runs the CLI binary, and verifies outcomes. The Claude CLI is mocked for session-related tests.

**Coverage targets:**

- Create a work in a real git repo, work through passes, finalize to a branch
- Multiple works in the same project simultaneously
- Works with dependencies — dependency warnings at finalize time — see [dependencies.md](dependencies.md)
- Bench with multiple projects
- Jig loading from file — see [jig-system.md](jig-system.md)
- Config overrides at bench, project, and work levels

### Agentic / Exploratory Tests

Agentic tests are scripted agent sessions where an agent is given a task (e.g., "spec out a user authentication feature for this sample project") and uses kerf throughout. The tests capture where the agent gets confused, makes mistakes, or produces poor output. CLI output and jig definitions are iterated based on findings.

**This is the most important testing layer.** Unit tests verify the code works. Agentic tests verify the *product* works.

**Coverage targets:**

- Agent reads `kerf` root output and understands how to use the tool
- Agent works through a full plan jig process — see [jig-plan.md](jig-plan.md)
- `shelve` output gives the agent enough guidance to write a useful `SESSION.md` — see [sessions.md](sessions.md)
- `resume` context loading gives the agent enough to continue effectively
- `finalize` instructions are followable — see [finalization.md](finalization.md)
- Jig file is clear enough that the agent follows the process correctly
- Edge cases: agent makes mistakes (wrong status, missing files, etc.)

### Fuzz Tests

Fuzz tests use Go's built-in fuzz testing (`testing.F`). They focus on input parsing and filesystem operations.

**Coverage targets:**

- Malformed YAML files (`spec.yaml`, `config.yaml`, jig files)
- Invalid codenames: empty, very long, path separators, null bytes
- Corrupted snapshot directories
- Missing files in expected locations
- Concurrent CLI invocations on the same work
- Filesystem permission issues

## Testing Principles

1. **Test the workflow, not just the code.** A function that works correctly is useless if the overall workflow breaks. Integration and E2E tests carry equal weight to unit tests.

2. **Agentic tests are not optional.** The primary consumer of kerf's output is an AI agent. If an agent cannot effectively use the tool, passing unit tests are irrelevant.

3. **Test failure modes, not just happy paths.** Corrupted `spec.yaml`, nonexistent session IDs, uncommitted changes at finalize time — the tool fails gracefully with useful error messages in all cases.

4. **Filesystem is the database.** Many defects are filesystem-related: permissions, path handling, concurrent access, disk full, symlinks. These scenarios are tested explicitly.

5. **Snapshot correctness is critical.** Snapshots that silently lose data cause unrecoverable work loss. Snapshot tests verify byte-level correctness.

## CI Strategy

| Trigger | Layers run |
|---------|-----------|
| Every commit | Unit, property-based, integration |
| Pull request | Unit, property-based, integration, fuzz, E2E |
| Nightly | Unit, property-based, integration, fuzz |
| Significant CLI output or jig changes | Agentic/exploratory |

Integration tests are fast when well-designed and run on every commit alongside unit and property-based tests. Fuzz tests and E2E tests are slower and run on PRs. Agentic tests require LLM calls and run only when CLI output format or jig definitions change significantly.
