# Plan 006 — Implementation Beads

## Overview

Plan 006 splits into 8 beads across 4 phases. P0 lands the filter engine leaf (`internal/beads`). P1 lands the YAML schema fields (config + spec) that depend on the filter type. P2 lands the feed engine, the cleanup/warning detectors, and the `kerf init` auto-detect heuristic in parallel. P3 rewrites `cmd/next.go`. P4 wires end-to-end tests. Engines land before commands; each bead owns its own files and is independently testable.

## Dependency Graph

```
P0 — Filter engine (leaf)
  └── B2   internal/beads — Filter, Match, Resolve; ForWork wrapper preserved
            depends on: —

P1 — YAML schema (parallel after B2)
  ├── B1a  config.ProjectConfig.BeadFilter   (internal/config/project.go)
  └── B1b  spec.SpecYAML.BeadFilter          (internal/spec/spec.go)
            both depend on: B2 (use beads.Filter)

P2 — Feed engine + detectors + init heuristic (parallel)
  ├── B3   internal/feed — Item, Kind, Assemble, Score, Sort, Filter
  ├── B4   internal/feed/cleanup.go — cleanup detectors
  ├── B5   internal/feed/warning.go — warning detectors
  └── B7   cmd/init.go — bead_filter auto-detect heuristic + write project.yaml
            B3/B4/B5 depend on: B2
            B7 depends on: B2, B1a

P3 — CLI rewrite
  └── B6   cmd/next.go — flag parsing, render text + JSON, help-text contract
            depends on: B3, B4, B5

P4 — Integration
  └── B8   E2E coordination tests
            depends on: B6, B7
```

This graph and the Parallelization Plan table below are consistent: B2 is P0; the two schema beads are P1; B3/B4/B5/B7 are P2; B6 is P3; B8 is P4.

## Inter-Package Import Map

```
cmd/next.go      ─► internal/feed
                    internal/beads
                    internal/queue
                    internal/spec
                    internal/config
                    internal/cmdutil

cmd/init.go      ─► internal/beads        (NEW: filter heuristic uses ForWork/Match)
                    internal/config       (NEW: writes bead_filter)
                    internal/project, jig, storage, bench  (existing)

internal/feed    ─► internal/beads
                    internal/queue
                    internal/spec
                    internal/config
                    (cleanup.go and warning.go live inside this package)

internal/beads   ─► (none internal — leaf)
internal/config  ─► internal/beads        (NEW: BeadFilter field type)
internal/spec    ─► internal/beads        (NEW: BeadFilter field type)
```

Leaves: `internal/beads`. `internal/config` and `internal/spec` import `internal/beads` to reference the `Filter` type — no cycles, since `internal/beads` does not import either. `internal/feed` is the only new package and it imports only existing leaves plus `internal/queue` for the bead ranking input. `internal/queue` does NOT need to change; the feed package consumes its existing `Compute` output. `internal/queue.Compute` does NOT take a `Filter` in its signature — filter resolution happens in callers (`cmd/next.go`, `internal/feed`) before calling into queue.

## Cross-Cutting Concerns

| Concern | Beads | Spec section |
|---|---|---|
| `{codename}` template substitution | B2 (impl), B6 (E2E), B7 (heuristic uses it) | `coordination.md` §"Template variables" |
| Case-sensitive label/id matching | B2 (impl), B5 (case-mismatch detector), B8 (E2E) | `coordination.md` §"Template variables" / "Matching is case sensitive" |
| Filter resolution precedence (per-work → project → built-in default) | B2 (resolver), B4 (uses resolved filter), B6 (uses resolved filter) | `coordination.md` §"Resolution order" |
| `any:` union semantics, no `all:` in v1 | B1a/B1b (schema), B2 (match logic) | `coordination.md` §"Filter shape" |
| Item kind enum (`bead`, `cleanup`, `warning`) — additive in future | B3 (declares enum), B6 (validates flag values against it) | `commands.md` §"Item kinds" |
| snake_case JSON field names | B3 (Item JSON tags), B6 (render) | `commands.md` §"Output (--format=json)" |
| JSON null for absent `work_codename` / `bead_id` (not `omitempty`) | B3 | `commands.md` §"Output (--format=json)" |
| Exclusion rules (beads excluded when work blocked/archived/finalized; cleanups excluded only when archived/finalized; warnings unfiltered) | B3 (Assemble exclusion), B6 (passes flags) | `commands.md` §"Behavior" step 4 |
| Cross-kind ranking (beads first by coordination score; cleanups after, sorted by parent work's would-be score, tie-break by work `created` asc; warnings as header) | B3 (Sort) | `commands.md` §"Behavior" step 5 + `coordination.md` §"Computed Priority" |
| Flag precedence (`--kinds` base, `--only` intersect, `--include` union, idempotent repeats) | B6 (parse) | `commands.md` §"Flag precedence" |
| Six-element help text contract | B6 | `commands.md` §"Help text" |
| Mutual exclusivity of `work_no_attached_beads` vs `work_beads_done_status_open` | B4 | `commands.md` §"Cleanup detectors" |
| Non-interactive `kerf init` honors spec step 8 | B7 | `commands.md` §"`kerf init`" step 8 |

## Per-Bead Specification

### Bead 2 — `internal/beads`: Filter type, Match, Resolve, ForWork wrapper (P0)

**Specs:** `coordination.md` §"Bead Attachment" (filter shape, template variables, resolution order, multiple matches).
**Package:** `internal/beads` (files: `filter.go`, `filter_test.go`).
**Deliverables:**
- `type Filter struct { Label string; IDPrefix string; Any []Filter }` with YAML tags `label`, `id_prefix`, `any`.
- `func (f *Filter) Validate() error` — rejects `all:` (not in v1), mixed direct + `any:`, empty filter.
- `func (f *Filter) Match(b Bead, codename string) bool` — case-sensitive; supports `{codename}` substitution; handles `any:` union (a single match wins).
- `func Resolve(workFilter, projectFilter *Filter) *Filter` — first-non-nil-wins; returns built-in default `{Label: "work:{codename}"}` when both are nil.
- **Preserve the existing `ForWork(beads []Bead, codename string) []Bead` signature** as a thin wrapper that calls `Resolve(nil, nil)` and applies the result. This ensures existing callers (`cmd/next.go`, `cmd/show.go`, `cmd/square.go`, `cmd/map.go`, `internal/queue` test helpers) continue to compile and behave identically without any edits in this bead.
- New entry point: `func ForWorkWithFilter(all []Bead, codename string, resolved *Filter) []Bead` for callers (B6, B4, B5) that need to pass an explicitly resolved filter.
- Because B2 leaves the existing wrapper alone, this bead does NOT touch `cmd/next.go`, `cmd/show.go`, `cmd/square.go`, or `cmd/map.go`. B6 owns the real `cmd/next.go` rewrite; the other cmd files keep working unchanged.
- Queue integration note: `internal/queue.Compute` does NOT take a `Filter` argument. Filter resolution stays in callers; queue continues to consume bead summaries only.

**Tests:**
- Truth table for `Match`: label exact, label miss, id_prefix hit, id_prefix miss, case-sensitivity (`Subsystem:bridge` vs `subsystem:bridge`).
- `{codename}` substitution in label and id_prefix.
- `any:` union: matches when any clause matches; doesn't match when all clauses miss.
- `Resolve`: per-work wins over project; project wins over default; nil/nil → default; partial-form rejection via `Validate`.
- `ForWork` wrapper: behaves identically to today (default filter).
- `ForWorkWithFilter`: applies the supplied resolved filter; returns a bead once per matching work in a single call.

---

### Bead 1a — `project.yaml` schema: top-level `bead_filter` (P1)

**Specs:** `architecture.md` §"Project configuration"; `coordination.md` §"Resolution order".
**Package:** `internal/config` (file: `project.go`).
**Deliverables:**
- Import `internal/beads`.
- Add `BeadFilter *beads.Filter \`yaml:"bead_filter,omitempty"\`` to `ProjectConfig`.
- Round-trip test: load/save preserves `bead_filter` (direct `label:` clause and `any:` form).
- Backward-compat test: existing `project.yaml` without `bead_filter` loads cleanly (nil pointer).

**Tests:** YAML round-trip for both shapes; nil-safe load.

---

### Bead 1b — `spec.yaml` schema: per-work `bead_filter` (P1)

**Specs:** `works.md` §"spec.yaml schema" (`bead_filter` row); `coordination.md` §"Resolution order".
**Package:** `internal/spec` (file: `spec.go`).
**Deliverables:**
- Add `BeadFilter *beads.Filter \`yaml:"bead_filter,omitempty"\`` to `SpecYAML`.
- Round-trip test: load/save preserves both `label:` direct and `any:` union forms.
- Update `spec_property_test.go` / `spec_fuzz_test.go` to cover the new optional field.

**Tests:** YAML round-trip; nil-safe load; co-exists with existing fields (Codename, DependsOn, Areas, etc.).

---

### Bead 3 — `internal/feed`: Item, Kind, Assemble, Score, Sort, Filter (P2)

**Specs:** `commands.md` §"`kerf next`" (behavior steps 3–7, default kind selection, flag precedence, item shape); `coordination.md` §"Priority and Ordering" (cross-kind sort rule, cleanup tie-break).
**Package:** new `internal/feed` (files: `feed.go`, `feed_test.go`, `item.go`).
**Deliverables:**
- `type Kind string` with constants `KindBead`, `KindCleanup`, `KindWarning`; `func KnownKinds() []Kind`; `func ParseKind(string) (Kind, error)`.
- `type Item struct { Kind Kind; Score float64; Title string; Action string; Reason string; WorkCodename *string; BeadID *string }` with snake_case JSON tags.
  - **Hard requirement:** `WorkCodename` and `BeadID` are pointer types (or equivalent optional). When absent, JSON emits literal `null`, not omitted. Do NOT use `omitempty` on these fields. Spec §"Output (--format=json)" mandates `<id or null>`.
- `type Input struct { Works []*spec.SpecYAML; AllBeads []beads.Bead; ProjectFilter *beads.Filter; QueueEntries []queue.Entry; ProjectID string }` — caller pre-loads.
- `func Assemble(in Input, detectors ...Detector) []Item` — calls bead source + each detector; applies exclusion rules (bead items excluded when target work is blocked/archived/finalized; cleanup items only when archived/finalized; warnings unfiltered).
- Bead-source helper: emits one `Item{Kind: KindBead}` per `(ready bead, matching work)` pair. Pulls bead score from the work's `queue.Entry.Score`. Ready = not blocked, not in progress, not closed.
- `func Sort(items []Item)` — beads ranked by Score desc; cleanup items sorted after all beads by their parent work's would-be queue score (descending). **Tie-break:** cleanup items with equal parent-work scores order by the work's `created` timestamp ascending. Warnings retain insertion order (rendered as a header block by B6, not interleaved).
- `func ApplyKindFilter(items []Item, sel KindSelection) []Item` — sel computed from flags (B6 owns parsing; B3 owns the data type).
- `Detector` interface: `Detect(in Input) []Item`. B4 and B5 supply implementations.

**Tests:**
- Item assembly: mixed bead + cleanup + warning input → correct ordering.
- Exclusion: blocked work emits no bead items but still emits its cleanup items; archived/finalized work emits nothing.
- Cross-kind sort: cleanup sorts after every bead even when its parent work has higher score than some lower-ranked bead's parent.
- Cleanup tie-break: two cleanups with equal parent score order by work `created` ascending.
- ApplyKindFilter precedence: `--kinds` base, `--only` intersect, `--include` union, idempotent repeats.
- JSON encode of `Item`: snake_case fields; nil `work_codename` / `bead_id` serialize as literal `null` (not omitted); golden-string assertion confirms exact bytes.

---

### Bead 4 — `internal/feed/cleanup.go`: detectors (P2)

**Specs:** `commands.md` §"Behavior" step 3 (cleanup detectors); §"Cleanup item triggers (v1)".
**Package:** `internal/feed` (files: `cleanup.go`, `cleanup_test.go` — flat inside the feed package, not a subpackage).
**Deliverables:**
- `WorkNoAttachedBeads` detector: for each non-archived/non-finalized work, resolve its filter, count matching beads; if zero → emit `cleanup` item with `Action: "edit spec.yaml bead_filter or project filter"` and `Reason: "no beads match the resolved bead_filter"`.
- `WorkBeadsDoneStatusOpen` detector: for each work, resolve filter, gather matched beads; if attached_count > 0 AND every bead's status is terminal (closed/done/complete) AND work's jig status is non-terminal → emit `cleanup` item with action template `kerf status <codename> <next-stage>` OR `kerf shelve <codename>`.
- Mutual exclusivity guard: the attached_count > 0 condition on the second detector ensures these never both fire for the same work.

**Tests:**
- `WorkNoAttachedBeads` fires on zero-match, clears once a matching bead is created.
- `WorkBeadsDoneStatusOpen` fires when all attached beads closed + work status non-terminal; does not fire when work status is terminal; does not fire when attached_count == 0 (mutual exclusivity).
- Both detectors honor the resolved filter (per-work override beats project filter beats default).

---

### Bead 5 — `internal/feed/warning.go`: detectors (P2)

**Specs:** `commands.md` §"Behavior" step 3 (warning detectors); `coordination.md` §"Unmatched beads", §"Matching is case sensitive".
**Package:** `internal/feed` (files: `warning.go`, `warning_test.go` — flat inside the feed package, not a subpackage).
**Deliverables:**
- `UnmatchedBeads` detector: gather every bead; for each, check whether ANY work's resolved filter matches it; collect the unmatched set; if non-empty → emit a single `warning` item with `Title` containing the count and `Action: "check bead_filter in project.yaml"`.
- `FilterCaseMismatch` detector: **project-wide only** (not per-work). When a project-wide `bead_filter` is set and its literal prefix (everything before `{codename}` if templated, else the whole literal) matches zero beads in the store, but a case-insensitive variant of that prefix would match something → emit a single warning suggesting case-mismatch check. Per-work overrides are intentionally not inspected by this detector.
- Both detectors no-op silently when the bead tool is unavailable (bead list empty).

**Tests:**
- `UnmatchedBeads` fires when at least one bead matches no work; clears when all beads match.
- `FilterCaseMismatch` fires for `Subsystem:` configured project-wide vs `subsystem:` in store; does not fire when the configured prefix has any case-sensitive match; does not inspect per-work overrides.
- No warnings when bead tool unavailable.

---

### Bead 7 — `cmd/init.go`: bead_filter auto-detect heuristic (P2)

**Specs:** `commands.md` §"`kerf init`" steps 7–8; `coordination.md` §"Onboarding".
**Depends on:** B2 (uses `beads.Filter`, `Match`), B1a (writes into `ProjectConfig.BeadFilter`). Does NOT depend on B3/B4/B5/B6 — the heuristic is self-contained and runs at init time, not feed assembly time.
**Package:** `cmd` (modify `init.go`; new helper in `internal/beads/heuristic.go`). Add `init_bead_filter_test.go` for the heuristic in isolation.
**Deliverables:**
- New `internal/beads/heuristic.go`:
  - `func DetectFilterPrefix(all []Bead, codenames []string) (prefix string, matchScore float64, topByCount []prefixCount)` — implements the spec algorithm: collect label prefixes (`P:` where the label is `P:value`) appearing on ≥ 3 beads; for each prefix compute `match_score = (beads matching some codename via "P:{codename}") / (beads with prefix "P:")`; return the highest above 0.5; otherwise return zero score and top-5 by raw count.
- `cmd/init.go` changes:
  - After project resolution + before `createDefaultProjectConfig`, if `beads.IsAvailable()`: call `beads.List()`, gather existing codenames from `~/.kerf/projects/{id}/` (or the bench equivalent if local). If zero codenames exist, **skip silently** (no top-5 prompt — per spec).
  - Run `DetectFilterPrefix`; if `matchScore > 0.5`, prompt `Detected: X% of beads use \`P:*\` labels. Set project-wide bead_filter to \`P:{codename}\`? [Y/n]`. Accept Y/y/empty as confirm.
  - If no candidate qualifies but codenames exist, fall back to a numbered top-5 prompt + "type your own" + skip option.
  - Persist the choice into `ProjectConfig.BeadFilter` so `createDefaultProjectConfig` writes it.
  - If `beads.IsAvailable()` is false, skip silently (no prompt, no error).
  - **Non-interactive (stdin not a TTY):** auto-detect runs but does not prompt; if a confident candidate (`matchScore > 0.5`) exists it is written, otherwise no `bead_filter` is set. Per spec `commands.md` §"`kerf init`" step 8.

**Tests:**
- `DetectFilterPrefix` unit tests: zero codenames → no detection; <3 beads on every prefix → no detection; clean `subsystem:{cn}` data → that prefix wins above 0.5; ambiguous data → falls back to top-5 by count; case-sensitive prefix grouping (`Subsystem:` and `subsystem:` are separate prefixes).
- `cmd/init` integration test: with a stubbed `br` (or with bead list injected via test seam), runs init, accepts default `Y`, writes `bead_filter: {label: "subsystem:{codename}"}` to `project.yaml`.
- Non-interactive path: stdin not a TTY + confident candidate → writes filter without prompt; no candidate → completes without writing `bead_filter`.
- Skip path: when `br` unavailable, init still completes and writes a `project.yaml` without `bead_filter`.
- Fresh-project path: zero codenames + nonempty bead store → no detection, no top-5 prompt, no `bead_filter` set.

---

### Bead 6 — `cmd/next.go`: flag parsing, render, help-text contract (P3)

**Specs:** `commands.md` §"`kerf next`" (syntax, flags, behavior, output, help text, errors).
**Package:** `cmd` (files: `next.go` (rewrite), `next_test.go`).
**Deliverables:**
- **Drop `--area`**. The v1 spec for `kerf next` does not list `--area`; keeping it is a spec deviation. Remove the flag, its parsing, and any references to it. Update help-text golden to confirm `--area` is absent.
- Replace `--limit` with `--only`, `--include`, `--kinds`, `--format`, plus existing `--project`.
- Flag parser builds a `feed.KindSelection` per the precedence rules: `--kinds` base (default = `{bead, cleanup}`; warnings always rendered as header unless excluded by `--only`/`--kinds` not including `warning`); `--only` repeatable intersection; `--include` repeatable union; idempotent repeats.
- Validate kind values against `feed.KnownKinds()`; emit the spec's error message for unknown kinds.
- Validate `--format` against `{text, json}`; emit spec error for unknown.
- Load works + project config + per-work specs; build the `feed.Input`; run `queue.Compute` for bead-score input; call `feed.Assemble` with the three v1 detectors. Filter resolution happens here (in the caller) before invoking queue/feed — queue receives no filter argument.
- Render text per spec sample (rank, kind abbrev, target id, title/reason, work codename); warning items as a header block above the ranked list.
- Render JSON per spec: snake_case fields, full item stream including warnings, in rank order, `work_codename`/`bead_id` emit `null` when absent.
- **Empty feed deliverable:** when no beads, no cleanups, and no warnings exist, emit a specific one-liner:
  ```
  No items. Run 'kerf new' to start a work, or check 'kerf list' for in-progress works.
  ```
  In JSON output, an empty feed emits `[]`. Add a test covering this case.
- Help text: hard-coded to the six-element contract; changing the text requires a spec change (referenced by `next_test.go` golden file). Golden file must confirm `--area` is not mentioned.

**Tests:**
- Flag precedence matrix: `--kinds=bead,cleanup`, `--only=bead`, `--only=bead --only=cleanup`, `--include=warning`, idempotent repeats, `--only=warning` produces only the header.
- Unknown kind error message matches spec exactly.
- `--area` is rejected as an unknown flag (golden error or cobra default).
- JSON output: snake_case, all items present in rank order, warning items included, absent fields serialize as `null`.
- **Empty-feed test:** no beads/cleanups/warnings → text emits the specified one-liner; JSON emits `[]`.
- Help text golden file matches the six-element order from spec and does not reference `--area`.

---

### Bead 8 — E2E coordination tests (P4)

**Specs:** `commands.md` §"`kerf next`" output; full feed loop end-to-end.
**Package:** `cmd` (new file: `next_feed_e2e_test.go`; extend `coordination_e2e_test.go` if shape fits).
**Parallel-authoring note:** the test fixtures (project layout, seeded bead store, golden files) are independent of the implementation files and can be authored in parallel with P2/P3 by a separate worker. Wire them up once B6 and B7 land.
**Deliverables:**
- E2E flow:
  1. Create a project with two works (`bridge`, `gateway`), set `bead_filter: {label: "subsystem:{codename}"}` in `project.yaml`; give `gateway` a per-work override (`id_prefix: "gw-"`).
  2. Seed an in-memory `beads.List` source (test seam in `internal/beads`) with: matching beads for both works, one all-closed work, one zero-match work, and one unmatched bead.
  3. Run `kerf next` text output and JSON output; assert: beads ranked; cleanup item for the all-closed work; cleanup item for the zero-match work; warning header for unmatched bead; case-sensitivity (verify `Subsystem:` configured produces case-mismatch warning when only `subsystem:` is in store).
  4. Filter flag matrix: `--only=bead`, `--only=cleanup`, `--include=warning`, `--kinds=bead,cleanup`, `--format=json`.
  5. Empty-feed scenario: project with no beads/cleanups/warnings emits the empty-feed one-liner (text) and `[]` (JSON).
- Verify `kerf init` auto-detect heuristic end-to-end: seed bead store with codename-shaped labels, run init, assert the prompt fires and the chosen filter is written. Add a non-interactive variant that exercises the no-prompt path.

**Tests:** Above scenarios; golden-output file for the help text.

## Parallelization Plan

| Phase | Beads | Workers | Depends on |
|---|---|---|---|
| P0 | B2 (filter engine, leaf types + ForWork wrapper) | 1 | — |
| P1 | B1a (project.yaml schema), B1b (spec.yaml schema) | 2 | B2 (uses `beads.Filter`) |
| P2 | B3 (feed engine), B4 (cleanup detectors), B5 (warning detectors), B7 (init heuristic) | 4 | B2 (B3/B4/B5/B7); B1a (B7) |
| P3 | B6 (cmd/next rewrite) | 1 | B3, B4, B5 |
| P4 | B8 (E2E) | 1 | B6, B7 |

Total: 8 beads, 4 phases, peak concurrency 4 (P2). B7 moves into P2 because it only depends on B2 and B1a — not on the feed engine or `cmd/next` rewrite. B8's fixtures may be authored in parallel earlier and wired up at P4.

---

## Judgment Calls (Resolved)

1. **Owner of `Filter` type.** Placed in `internal/beads` because filtering is a property of bead data. `internal/config` and `internal/spec` import the type. No cycles.
2. **B2 wrapper preserves the existing `ForWork` signature.** New callers use `ForWorkWithFilter`. This removes the file-ownership collision that would otherwise put B2 and B6 in conflict on `cmd/next.go`, `cmd/show.go`, `cmd/square.go`, `cmd/map.go`.
3. **`--area` flag on `kerf next`: dropped.** Not in the v1 spec; removing it brings code in line with spec.
4. **JSON null contract:** `work_codename` / `bead_id` use pointer types so they emit literal `null`, never omitted. `omitempty` is forbidden on these fields. Enforced in B3.
5. **Cleanup tie-break:** equal parent-work scores order by work `created` timestamp ascending. Codified in `coordination.md`.
6. **`FilterCaseMismatch` scope:** project-wide only; per-work overrides are not inspected by the case-mismatch detector.
7. **Auto-detect on fresh project (zero codenames):** skip silently. No top-5 prompt for first-time users — they can edit `project.yaml` later.
8. **Non-interactive `kerf init`:** auto-detect runs without prompting; writes the filter if a confident candidate exists, otherwise leaves `bead_filter` unset. Spec updated.
9. **`internal/feed/cleanup` and `internal/feed/warning` flattened to files** (`cleanup.go`, `warning.go`) inside `internal/feed`. Avoids subpackage proliferation for two-file modules.
