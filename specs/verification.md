# Verification

> Structural verification of works against jig requirements — the "square" check.

## Overview

kerf's verification system determines whether a [work](works.md) is **square**: structurally complete and consistent with its [jig](jig-system.md) requirements. Square is a structural check. It verifies that expected artifacts exist, the workflow reached completion, and blocking dependencies are satisfied. It does not verify content quality — that is the responsibility of the human or a review agent.

Square checks work-directory artifacts only. All checks (status, files, dependencies) operate on the work directory on the bench. There is no special behavior for spec-first works — drafted spec changes in `05-spec-drafts/` live in the work directory and are checked like any other artifact file.

## Checks

`kerf square` runs three checks against a work. All three must pass for the work to be square.

### Status Check

kerf reads the work's `status` from [spec.yaml](works.md) and compares it against the jig's `status_values` list. The check **passes** if the work's status is at or past the jig's terminal status (the last entry in `status_values`, typically `ready`).

"At or past" means the status either:

- Equals the terminal status, or
- Does not appear in `status_values` at all (indicating a status beyond the jig's progression, e.g., `finalized`, `implementing`, or `done`)

The check **fails** if the status appears in `status_values` at a position before the terminal entry.

### File Check

kerf reads the jig's `file_structure` list and checks that each expected file exists on disk in the work directory.

Paths containing `{component}` placeholders are expanded. kerf determines the set of components from the work's existing directory structure (e.g., subdirectories under `04-research/` or `03-research/`). Each component produces one expected file per template path. If no components are detected, template paths with `{component}` placeholders are skipped — they produce no expected files.

The check **passes** if every expected file exists. The check **fails** if one or more expected files are missing.

`spec.yaml` and `SESSION.md` are included in `file_structure` and are checked like any other expected file.

### Process Pass Check

For jigs that include **process passes** (see [jig-system.md](jig-system.md) §Process Passes vs. Artifact Passes), kerf checks process completion status in addition to file existence.

Process passes do not produce primary output files — their state lives in external tools (e.g., the bead database for implementation tasks). kerf checks process pass completion by examining the work's status progression:

- A process pass is **complete** if the work's status has advanced past the pass's status value
- A process pass is **incomplete** if the work's status matches the pass's status value (still in progress) or precedes it (not yet started)

For **implementation works** specifically, kerf also reports bead status when available:

- Total beads, open beads, closed beads
- Whether any beads have unresolved review feedback

This is informational — bead counts are reported alongside the process pass status to give visibility into implementation progress. kerf reads bead status via `bd list` if `bd` is available. If `bd` is not available, the process pass check relies solely on the work's status value.

Process pass checks apply only to jigs that have process passes. Spec-writing jigs (plan, spec, bug) have only artifact passes and are unaffected.

### Dependency Check

kerf reads the work's `depends_on` list from [spec.yaml](works.md) and checks each dependency with a `must-complete-first` [relationship](dependencies.md).

A dependency is **complete** if its status is at or past the last value in its own `status_values` list, using the same "at or past" logic as the Status Check (the status either equals the terminal value, or does not appear in `status_values` at all). See [dependencies](dependencies.md) for the full resolution process.

Dependencies with an `inform` relationship are not checked. They do not affect whether the work is square.

**Unresolvable dependencies** (target work not found on the bench, or `spec.yaml` unreadable) are reported but do not cause the dependency check to fail. They appear in the output as unresolvable so the user can investigate.

The check **passes** if all resolvable `must-complete-first` dependencies are complete. The check **fails** if one or more resolvable `must-complete-first` dependencies are incomplete.

## Result

The overall result is **SQUARE** if all three checks pass. The result is **NOT SQUARE** if any check fails.

## Output

`kerf square` reports the result of each check with enough detail for the agent or user to act on failures.

```
Square check for auth-rewrite:

  Status:        pass — ready (expected: ready or later)
  Files:         pass — 9/9 expected files present
  Dependencies:  pass — 1/1 blocking dependencies complete

Result: SQUARE
```

When checks fail, the output includes specifics:

```
Square check for auth-rewrite:

  Status:        fail — research (expected: ready or later)
  Files:         fail — 5/9 expected files present
    Missing:     05-specs/auth-spec.md
                 05-specs/session-spec.md
                 06-integration.md
                 SPEC.md
  Dependencies:  fail — 0/1 blocking dependencies complete
    Incomplete:  database-migration [decomposition]

Result: NOT SQUARE
```

When unresolvable dependencies exist:

```
  Dependencies:  pass — 1/1 blocking dependencies complete
    Unresolvable: payment-service (project: billing-api — not found on bench)
```

See [commands.md](commands.md) for the full command syntax and error messages.

When process passes are present (implementation works):

```
Square check for api-refactor:

  Status:        pass — complete (expected: complete or later)
  Files:         pass — 3/3 expected files present
  Process:       pass — 4/4 process passes complete
    Beads:       37 total, 37 closed, 0 open
  Dependencies:  pass — 1/1 blocking dependencies complete

Result: SQUARE
```

When an implementation work is still in progress:

```
Square check for api-refactor:

  Status:        fail — implementing (expected: complete or later)
  Files:         fail — 2/3 expected files present
    Missing:     03-verify.md
  Process:       fail — 2/4 process passes complete
    Beads:       37 total, 22 closed, 15 open (3 with unresolved review feedback)
  Dependencies:  pass — 1/1 blocking dependencies complete

Result: NOT SQUARE
```

## What Square Does Not Check

Square is deliberately limited to structural verification:

- **Content quality.** Whether a spec is well-written, complete in substance, or technically sound is not assessed. That is the job of a human reviewer or a review agent.
- **Semantic consistency.** Whether artifacts contradict each other or contain stale information is not assessed.
- **Pass ordering.** Whether the agent followed the jig's passes in order is not checked. Passes are guidance, not gates.

## Use During Finalization

[Finalization](finalization.md) runs square as a pre-flight check before proceeding with any git operations. If the work is not square, finalization reports the issues and aborts. The user must resolve the failing checks and run `kerf finalize` again.

This is the only place where square is run automatically. At all other times, the agent or user runs `kerf square` explicitly.
