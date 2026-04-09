# Code Implementation Beads — Plan 002

> Code changes to implement the jig redesign specs.
> The specs are already updated (spec beads T0-T3 complete). These beads make the code match.
> Revised after sub-agent review (13 findings applied).

## Dependency Graph

```
C0 (jig files + resolve + jig commands)
 └─► C1a (config changes)  ─┐
 └─► C1b (new command)      ├─► C2 (finalize) ─► C3 (test suite update)
                            ─┘
```

## Beads

### C0 — Update built-in jigs, jig resolution, and jig commands

**Specs:** jig-system.md (aliases, resolution), jig-plan.md, jig-spec.md, jig-bug.md
**Packages:** `internal/jig/`, `internal/jig/builtin/`, `cmd/jig.go`

**Deliverables:**

1. **Replace built-in jig files:**
   - Delete `internal/jig/builtin/feature.md`
   - Create `internal/jig/builtin/plan.md` — copy content from `specs/jig-plan.md`
   - Create `internal/jig/builtin/spec.md` — copy content from `specs/jig-spec.md`
   - Replace `internal/jig/builtin/bug.md` — copy content from `specs/jig-bug.md`
   - Note: the `//go:embed builtin/*.md` glob picks up all .md files automatically — no embed directive change needed, just the files on disk

2. **Add `Aliases` to jig structs:**
   - Add `Aliases []string` field to `JigDefinition` struct (parsed from frontmatter `aliases:` field)
   - Add `Aliases []string` field to `JigSummary` struct

3. **Update `Resolve()` — add alias resolution (step 3):**
   - Current: (1) user-level by filename, (2) built-in by filename
   - New: (1) user-level by filename, (2) built-in by filename, (3) scan built-in jigs' `Aliases` for match
   - When resolved via alias, return the jig with its canonical `Name` (e.g., resolve "feature" → return jig with Name "plan")

4. **Update `ReadBuiltinRaw()` to handle aliases:**
   - `ReadBuiltinRaw("feature")` must resolve the alias to "plan" and read `plan.md`
   - This is needed for `kerf jig save feature` to work after `feature.md` is removed

5. **Update `ListAll()` to populate `JigSummary.Aliases`**

6. **Update `cmd/jig.go` jig list display:**
   - Show aliases in output: `plan (also: feature)` format
   - Read aliases from `JigSummary.Aliases`

**Tests:**
- Alias resolution: `Resolve("feature", "")` returns plan jig, source "built-in"
- Canonical name: resolved jig's Name is "plan" not "feature"
- `ReadBuiltinRaw("feature")` returns plan jig content
- All three built-in jigs parse correctly
- ListAll returns plan, spec, bug with correct aliases
- User-level jig named "feature" takes priority over alias
- `jig list` shows "plan (also: feature)", "spec", "bug"
- `jig show feature` displays plan jig
- `jig save feature` copies plan jig content

---

### C1a — Config changes

**Specs:** architecture.md (spec_path, default_jig default)
**Package:** `internal/config/`

**Deliverables:**

1. Add `SpecPath` field to `Config` struct — `yaml:"spec_path,omitempty"`, default `"specs/"`
2. Add `EffectiveSpecPath() string` method (returns SpecPath if set, else default)
3. Change `DefaultJig` constant from `"feature"` to `""` (empty string = unset)
4. Note: `EffectiveDefaultJig()` will now return `""` when unconfigured — this is correct, callers must handle it
5. Add `"spec_path"` to `ValidKeys()` list
6. Add `spec_path` to Get/Set dot-notation handlers

**Update existing tests that assert `DefaultJig == "feature"`:**
- `internal/config/config_test.go` — update assertions for default jig
- `internal/config/config_property_test.go` — update property test assertions

**Tests:**
- Missing config returns spec_path default `"specs/"`
- Missing config returns default_jig `""` (empty, not "feature")
- Get/Set spec_path round-trip
- ValidKeys includes spec_path

---

### C1b — kerf new onboarding flow + canonical name recording

**Specs:** commands.md (kerf new first-run error), jig-system.md (canonical name recording)
**Package:** `cmd/new.go`

**Deliverables:**

1. **Onboarding check:** Before resolving the jig, check if effective jig name is empty (default_jig unset AND no --jig flag). If empty, print the onboarding error message from commands.md §kerf new and exit with error code. If --jig is provided, skip this check.

2. **Canonical name recording:** After `jig.Resolve()` returns, use `j.Name` (the canonical jig name) for:
   - `spec.yaml` `jig:` field (currently uses raw `jigName` input — must use `j.Name`)
   - `--type` default when `--type` is not provided (currently uses `jigName` — must use `j.Name`)
   - This ensures `kerf new --jig feature` records `jig: plan` and `type: plan` in spec.yaml

3. The error message must match the spec exactly:
   ```
   Error: No default workflow configured.

   How do you want to use kerf?

     kerf config default_jig plan
       Write a plan before changing code. Best for existing projects.
       ...

     kerf config default_jig spec
       Maintain a living spec that defines your system. Best for new projects.
       ...

   Or specify for just this work:  kerf new <codename> --jig plan
   ```

**Tests:**
- No config, no --jig flag → error with "No default workflow configured"
- No config, --jig plan → succeeds, spec.yaml has `jig: plan`
- No config, --jig feature → succeeds, spec.yaml has `jig: plan` (canonical, not alias)
- Config set to plan, no --jig → succeeds
- Error output contains both `kerf config default_jig plan` and `kerf config default_jig spec`

---

### C2 — Finalize spec-first behavior

**Specs:** finalization.md (spec-first finalization), commands.md (kerf finalize)
**Package:** `cmd/finalize.go`
**Depends on:** C1a (needs EffectiveSpecPath)

**Deliverables:**

1. **Detect spec-first work:** After loading spec.yaml, check if `Jig` field equals `"spec"`.

2. **Conditional exclusion in `copyArtifacts`:** Add a parameter or flag to `copyArtifacts` to conditionally exclude `05-spec-drafts/`. For spec-first works, pass `excludeSpecDrafts: true`. For other works, `false` (they can still have that directory and it should be copied normally).

3. **Copy spec drafts to spec_path:** If spec-first, after standard artifact copy:
   - Read `spec_path` from config via `cfg.EffectiveSpecPath()`
   - If `05-spec-drafts/` doesn't exist or is empty in work dir: print warning, continue
   - Create `{repo_root}/{spec_path}/` if it doesn't exist
   - Copy all files from `05-spec-drafts/` to `{repo_root}/{spec_path}/`

4. **Stage spec_path files:** Add `git add {spec_path}` after copying spec drafts (in addition to existing `git add repoSpecPath`).

5. **Dual-destination output:** Print both "Artifacts copied to {repo_spec_path}" and "Spec drafts applied to {spec_path}" for spec-first works.

**Tests:**
- Spec-first work: `05-spec-drafts/` files copied to `spec_path/`
- Spec-first work: `05-spec-drafts/` excluded from standard artifact copy at `repo_spec_path/`
- Plan-first work: no spec_path behavior, `05-spec-drafts/` copied normally if present
- Missing `05-spec-drafts/`: warning printed, no error
- `spec_path` directory created if missing
- Git commit includes files from both destinations

---

### C3 — Update scenario tests + run full suite

**Depends on:** C0, C1a, C1b, C2
**Package:** `tests/scenarios/`

**Deliverables:**

1. **Update `01_onboarding_and_new.sh`:** Add Phase 0 testing onboarding flow — `kerf new` without config or --jig flag should error with "No default workflow configured". Then set `default_jig plan` and proceed.

2. **Update `03_alias_and_config.sh`:** Test alias resolution — `kerf new --jig feature` should succeed, and spec.yaml should have `jig: plan` (canonical name). Update jig list assertions from `"feature"` to `"plan"` (with alias display).

3. **Add `06_spec_first_finalize.sh`:** Create work with `--jig spec`, create `05-spec-drafts/` with test files, set status to ready, create all required artifacts, finalize, verify: spec drafts land in `spec_path/` (not in `repo_spec_path/`), process artifacts land in `repo_spec_path/`, commit includes both.

4. **Run `tests/scenarios/run_all.sh`** — all scenarios pass including existing ones (01-05) and new one (06).

5. **Run `go test ./...`** — all unit tests pass.

---

## Parallelization Plan

| Phase | Beads | Workers | Depends On |
|-------|-------|---------|------------|
| 1 | C0 | 1 | — |
| 2 | C1a, C1b | 2 | Phase 1 |
| 3 | C2 | 1 | Phase 2 (needs C1a for spec_path) |
| 4 | C3 | 1 | Phase 3 |

**Notes:**
- C0 is the largest bead — jig files, resolution, alias handling, jig commands. One worker.
- C1a and C1b are independent (different packages) and can run in parallel.
- C2 needs C1a's `EffectiveSpecPath()`. Sequential after Phase 2.
- C3 validates everything end-to-end. Must be last.
- C2b (jig list aliases) merged into C0 since it depends only on C0's JigSummary changes and touches the same `cmd/jig.go` file.
