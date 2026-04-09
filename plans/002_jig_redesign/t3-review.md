# T3 — Cross-Spec Consistency Review

> Final consistency pass across all specs after plan 002 jig redesign.

## Check 1: Internal Links

**Result: FAIL (1 broken link) — FIXED**

Every `[text](file.md)` link in every spec file was verified against the file listing in `specs/`.

| File | Link | Target | Status |
|------|------|--------|--------|
| testing.md:91 | `[jig-feature.md](jig-feature.md)` | `specs/jig-feature.md` | BROKEN — file deleted in T1a |

All other links across all 16 spec files resolve correctly (architecture.md, cli.md, commands.md, dependencies.md, finalization.md, future.md, jig-bug.md, jig-plan.md, jig-spec.md, jig-system.md, sessions.md, snapshots.md, verification.md, works.md, _index.md, testing.md).

**Fix applied:** testing.md:91 — changed `[jig-feature.md](jig-feature.md)` to `[jig-plan.md](jig-plan.md)`.

---

## Check 2: Status Values

**Result: PASS**

### Extracted from jig frontmatter:

**jig-plan.md:**
`problem-space -> analyze -> decompose -> research -> change-spec -> integration -> tasks -> ready`

**jig-spec.md:**
`problem-space -> decompose -> research -> change-design -> spec-draft -> integration -> tasks -> ready`

**jig-bug.md:**
`reported -> research -> reproducing -> root-cause -> fix-spec -> ready`

### Verified against works.md §Recommended Values (lines 80-95):

All three progressions match exactly. Plan jig, spec jig, and bug jig progressions are listed.

### Verified against commands.md:

`kerf status` (lines 506-568) references the jig's `status_values` list generically via template placeholders — correct behavior, no hardcoded status values to go stale.

---

## Check 3: Config Keys

**Result: FAIL (2 missing keys in commands.md) — FIXED**

### Config keys in architecture.md schema (lines 76-136):

1. `default_jig`
2. `spec_path`
3. `default_project`
4. `snapshots.enabled`
5. `snapshots.interval_enabled`
6. `snapshots.interval_seconds`
7. `snapshots.max_snapshots`
8. `sessions.stale_threshold_hours`
9. `finalize.repo_spec_path`

### commands.md `kerf config` known keys (lines 802-813):

**Before fix:** Listed 7 keys — missing `default_project` and `sessions.stale_threshold_hours`.

**Fix applied:** Added both missing keys to the `kerf config` output block in commands.md.

### finalization.md `spec_path` references:

Correctly references `spec_path` as a config value with default `specs/` (lines 107, 118). Consistent with architecture.md.

---

## Check 4: Alias Consistency

**Result: PASS**

The `feature` -> `plan` alias is consistently described in all required locations:

| Location | Evidence |
|----------|----------|
| jig-system.md §Resolution Order (line 176) | "Example: `kerf new --jig feature` resolves to the `plan` jig" |
| jig-plan.md frontmatter (line 30) | `aliases: [feature]` |
| commands.md `kerf jig list` output (line 600) | `plan (also: feature)` |
| works.md §Type (line 63) | "also accepts `feature` as an alias" |

---

## Check 5: File Structure vs Pass Details

**Result: PASS**

### jig-plan.md

| file_structure entry | Pass output | Match |
|---------------------|-------------|-------|
| spec.yaml | (managed by kerf) | exempt |
| SESSION.md | (managed by kerf) | exempt |
| 01-problem-space.md | Pass 1 output | yes |
| 02-analysis.md | Pass 2 output | yes |
| 03-components.md | Pass 3 output | yes |
| 04-research/{component}/findings.md | Pass 4 output | yes |
| 05-specs/{component}-spec.md | Pass 5 output | yes |
| 06-integration.md | Pass 6 output | yes |
| SPEC.md | Pass 6 output | yes |
| 07-tasks.md | Pass 7 output | yes |

No orphan file_structure entries. No missing pass outputs.

### jig-spec.md

| file_structure entry | Pass output | Match |
|---------------------|-------------|-------|
| spec.yaml | (managed by kerf) | exempt |
| SESSION.md | (managed by kerf) | exempt |
| 01-problem-space.md | Pass 1 output | yes |
| 02-components.md | Pass 2 output | yes |
| 03-research/{component}/findings.md | Pass 3 output | yes |
| 04-design/{component}-design.md | Pass 4 output | yes |
| 05-spec-drafts/{component}.md | Pass 5 output | yes |
| 05-changelog.md | Pass 5 output | yes |
| 06-integration.md | Pass 6 output | yes |
| 07-tasks.md | Pass 7 output | yes |

No orphan file_structure entries. No missing pass outputs.

### jig-bug.md

| file_structure entry | Pass output | Match |
|---------------------|-------------|-------|
| spec.yaml | (managed by kerf) | exempt |
| SESSION.md | (managed by kerf) | exempt |
| 01-report.md | Pass 1 output | yes |
| 02-research.md | Pass 2 output | yes |
| 03-reproduction.md | Pass 3 output | yes |
| 04-root-cause.md | Pass 4 output | yes |
| 05-fix-spec.md | Pass 5 output | yes |

No orphan file_structure entries. No missing pass outputs.

---

## Check 6: Finalization Exclusion List

**Result: PASS**

| File | Exclusions listed |
|------|-------------------|
| finalization.md (lines 57-59) | `spec.yaml`, `SESSION.md`, `.history/` |
| commands.md (line 400) | `spec.yaml`, `SESSION.md`, `.history/` |

Both files list the same three exclusions. For spec-first works, both also document excluding `05-spec-drafts/` from the standard artifact copy (finalization.md line 61, commands.md line 405).

---

## Check 7: Component Placeholder Compatibility

**Result: FAIL (stale references) — FIXED**

### {component} patterns per jig:

**jig-plan.md:**
- `04-research/{component}/findings.md` (directory-based)
- `05-specs/{component}-spec.md` (file-based)

**jig-spec.md:**
- `03-research/{component}/findings.md` (directory-based)
- `04-design/{component}-design.md` (file-based)
- `05-spec-drafts/{component}.md` (file-based)

**jig-bug.md:** No `{component}` patterns.

### verification.md expansion logic (line 30):

Before fix: Referenced `04-plans/` as an example directory for component detection. This directory does not exist in any current jig — it was from the deleted `jig-feature.md`.

**Fix applied:** Changed example from `04-plans/` to `04-research/` (plan jig) and `03-research/` (spec jig).

### verification.md output example (lines 73-74):

Before fix: Used `04-plans/auth-spec.md`, `04-plans/session-spec.md`, `05-integration.md`, `06-checklist.md` — all from the deleted feature jig.

**Fix applied:** Updated to `05-specs/auth-spec.md`, `05-specs/session-spec.md`, `06-integration.md`, `SPEC.md` — matching the current plan jig file structure.

### Spec jig compatibility:

The spec jig's `{component}` means "target spec filename" (e.g., `jig-system`, `commands`), which is different from other jigs where it means feature components. This is explicitly documented in jig-spec.md §Component Placeholder (lines 85-91). The detection logic in verification.md works correctly: components are discovered from subdirectories under `03-research/` (the first directory-based pattern), then applied to all template patterns.

### Additional stale reference:

sessions.md line 243 (inside a SESSION.md example): referenced `04-plans/auth-events-spec.md`. Updated to `05-specs/auth-events-spec.md` for consistency.

---

## Check 8: No Dangling References to jig-feature.md

**Result: FAIL (1 reference) — FIXED**

Grep across all spec files for `jig-feature`:

| File | Line | Reference | Status |
|------|------|-----------|--------|
| testing.md | 91 | `[jig-feature.md](jig-feature.md)` | FIXED (same as Check 1) |

No other references to `jig-feature` exist in any spec file.

---

## Summary

| Check | Result | Issues | Fixed |
|-------|--------|--------|-------|
| 1. Internal links | FAIL | testing.md broken link to jig-feature.md | yes |
| 2. Status values | PASS | — | — |
| 3. Config keys | FAIL | commands.md missing default_project, sessions.stale_threshold_hours | yes |
| 4. Alias consistency | PASS | — | — |
| 5. File structure vs passes | PASS | — | — |
| 6. Finalization exclusion list | PASS | — | — |
| 7. Component placeholder compatibility | FAIL | verification.md stale 04-plans/ references; sessions.md stale example | yes |
| 8. No dangling jig-feature.md refs | FAIL | testing.md:91 | yes |

### Files modified:
- `specs/testing.md` — fixed jig-feature.md reference to jig-plan.md
- `specs/commands.md` — added default_project and sessions.stale_threshold_hours to kerf config output
- `specs/verification.md` — updated stale 04-plans/ to current jig directory names
- `specs/sessions.md` — updated stale 04-plans/ to 05-specs/ in SESSION.md example
