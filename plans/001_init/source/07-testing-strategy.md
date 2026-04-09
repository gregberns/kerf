# Testing Strategy

Testing is a first-class concern in this project. The tool manages a process that leads to code generation — if the tool's workflow breaks, the downstream impact is significant. A work that gets corrupted, a session that can't resume, or a finalization that silently drops files means wasted hours of human and agent time.

## Testing Layers

### Unit Tests
**What:** Test individual functions and components in isolation.  
**Coverage targets:**
- YAML parsing/serialization (spec.yaml, config.yaml)
- Status transitions and validation
- Project ID derivation (from git remote, directory name fallback, collision detection)
- Snapshot creation and management
- Jig file parsing
- Codename generation (adjective-noun slugs) and validation (lowercase alphanumeric and hyphens only)
- Dependency graph operations

**Approach:** Standard Go table-driven tests. These should be fast and comprehensive.

### Property-Based Tests
**What:** Generate random inputs to find edge cases.  
**Coverage targets:**
- Codename handling (special characters, unicode, length limits, path traversal attempts)
- YAML round-tripping (write then read produces identical structure)
- Snapshot integrity (snapshot then restore produces identical files)
- Concurrent file operations (multiple works being modified simultaneously)
- Config merging (bench config + project config + work config)

**Approach:** Go's `testing/quick` or a library like `gopter`. Focus on serialization boundaries and filesystem operations.

### Integration Tests
**What:** Test command sequences against a real filesystem.  
**Coverage targets:**
- Full lifecycle: `new` -> write files -> `shelve` -> `resume` -> `finalize`
- `new` creates correct directory structure for each jig type
- `shelve` preserves all state correctly
- `resume` with valid session ID
- `resume` with missing/stale session ID (fallback behavior)
- `finalize` copies files to correct repo location, creates branch
- `list` accurately reflects filesystem state
- `show` displays correct information
- `status` updates persist correctly
- `jig` subcommands (list, show, save, load)
- `square` verification checks
- `snapshot` creates correct point-in-time copy
- `history` shows correct timeline
- Config file interactions (bench, project level)

**Approach:** Create temp directories, run real commands, verify filesystem state. Each test sets up a fresh bench.

### End-to-End Tests
**What:** Test the complete user workflow, including interaction with git and Claude CLI.  
**Coverage targets:**
- Create a work in a real git repo, work through passes, finalize to a branch
- Multiple works in the same project simultaneously
- Works with dependencies — verify dependency warnings at finalize
- Bench with multiple projects
- Jig loading from file
- Config overrides at different levels

**Approach:** Shell scripts or Go test harness that sets up real git repos, runs the CLI, and verifies outcomes. May need to mock the Claude CLI for session-related tests.

### Agentic/Exploratory Tests
**What:** Have an AI agent actually use the tool and report on the experience.  
**Purpose:** This is critical — the tool is designed for agent interaction. We need to verify that:
- An agent can read the `kerf` root output and understand how to use the tool
- An agent can successfully work through a full feature jig process
- The `shelve` output gives the agent enough guidance to write a useful SESSION.md
- The `resume` context loading gives the agent enough to continue effectively
- The `finalize` instructions are followable
- The jig file is clear enough that the agent follows the process correctly
- Edge cases: what happens when the agent makes mistakes (wrong status, missing files, etc.)

**Approach:** Scripted agent sessions where an agent is given a task ("spec out a user authentication feature for this sample project") and uses the tool throughout. Capture where the agent gets confused, makes mistakes, or produces poor output. Iterate on CLI output and jig definitions based on findings.

**This is the most important testing layer.** Unit tests verify the code works. Agentic tests verify the *product* works. They should run as part of the development process — not just once at the end.

### Fuzz Testing
**What:** Throw malformed/unexpected input at the CLI.  
**Coverage targets:**
- Malformed YAML files (spec.yaml, config.yaml, jig files)
- Invalid codenames (empty, very long, path separators, null bytes)
- Corrupted snapshot directories
- Missing files in expected locations
- Concurrent CLI invocations on the same work
- Filesystem permission issues

**Approach:** Go's built-in fuzz testing (`testing.F`). Focus on input parsing and filesystem operations.

## Testing Principles

1. **Test the workflow, not just the code.** A function that works correctly is useless if the overall workflow breaks. Integration and E2E tests are as important as unit tests.

2. **Agentic tests are not optional.** The primary user of this tool's output is an AI agent. If an agent can't effectively use the tool, it doesn't matter that the unit tests pass.

3. **Test failure modes, not just happy paths.** What happens when a spec.yaml is corrupted? When a session ID doesn't exist? When the target repo has uncommitted changes at finalize time? The tool should fail gracefully and give useful error messages.

4. **Filesystem is the database.** Many bugs will be filesystem-related: permissions, path handling, concurrent access, disk full, symlinks. Test these scenarios explicitly.

5. **Snapshot correctness is critical.** If snapshots silently lose data, users lose work they thought was preserved. Snapshot tests should verify byte-level correctness.

## CI Strategy

- Unit + property tests: run on every commit
- Integration tests: run on every commit (they're fast if well-designed)
- Fuzz tests: run on PRs and nightly (they're slow)
- E2E tests: run on PRs (they're slower, need git setup)
- Agentic tests: run on significant changes to CLI output or jig definitions (they're expensive — require LLM calls)
