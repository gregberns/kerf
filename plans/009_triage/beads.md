# Plan 009 — Implementation Beads

## Overview

Plan 009 lands the triage workflow in **11 beads across 4 layers**. The
critical path is **4 hops long**: B2 (`internal/drift`, now including the
cache lifecycle that B6 used to own) → B5 (drift-aware feed Input + pin
layer wired into `BeadSource`) → B8 (`cmd/triage.go`) → B12 (E2E).
Specs are already merged on `main` (commands.md, coordination.md,
works.md, architecture.md), so there are no spec beads — every bead is
code + tests against an existing spec section.

The two load-bearing pieces (per the plan): `kerf show <codename>` rendering
attached beads (B7) and `kerf triage --resolved` (B8). B1 (comment-preserving
YAML mutators) and B2 (`internal/drift`, including cache lifecycle) are the
unblockers — both single-package leaves with no internal deps.

## Dependency Graph

```
L0 — Leaves (parallel)
  ├── B1  internal/spec/mutate.go — comment-preserving YAML mutators
  │       (pinned_beads add/remove; bead_filter clause add/remove).
  │       Also introduces `beads.ParseFilterClause` in
  │       internal/beads/filter.go (NOT in internal/spec — see
  │       "Pin-layer placement" / Judgment Call #1).
  │       depends on: —
  ├── B2  internal/drift — snapshot capture, hash, diff, AND cache
  │       lifecycle (read/write/--ack advance). (Folded the former
  │       B6 in here; cache.go is four small functions and splitting
  │       only added an inter-bead dep on the critical path.)
  │       depends on: —
  └── B3  internal/spec/spec.go — add PinnedBeads field to SpecYAML
          depends on: —  (pure SpecYAML field add; file-disjoint from B1)

L1 — Detectors, feed wiring, and command leaves (parallel after L0)
  ├── B4   internal/feed/warning.go — untriaged / multi-matched /
  │        external-drift detectors. Includes explicit rename of the
  │        existing plan-006 unmatchedBeadsDetector → untriagedBeadsDetector
  │        plus the matching rewire in NewWarningDetectors and a
  │        callers-audit across cmd/ and internal/feed/.
  │        depends on: B2 (consumes drift.Diff), B3 (reads PinnedBeads)
  ├── B5   internal/feed/feed.go — Input gains DriftResult + PinAssignments;
  │        pin layer wired into BeadSource (NOT Assemble). The caller
  │        applies PinAssignments to in.BeadToWork BEFORE invoking
  │        BeadSource — see "Pin-layer placement" below.
  │        depends on: B2, B3
  ├── B9   cmd/pin.go — new command; single-owner pin enforcement
  │        depends on: B1, B3
  ├── B10  cmd/work_edit.go — new `kerf work edit` cobra subcommand with
  │        --bead-filter-add/-remove
  │        depends on: B1
  └── B11a cmd/new.go — --bead-filter flag wiring at creation
           depends on: B1, B3

L2 — Show + drift-consuming commands (parallel after L1)
  ├── B7   cmd/show.go — render Attached beads block + drift markers
  │        depends on: B3, B5 (reads PinnedBeads + drift via feed.Input);
  │        also depends on plan 008 B1 (cmd/show.go internal/beads.List
  │        rewrite) having landed on main
  ├── B8   cmd/triage.go — new command; --resolved/--ack/--format/--kind
  │        depends on: B2 (cache lifecycle), B4, B5
  └── B11b cmd/next.go — drift-summary headline counters above ranking
           depends on: B2, B4, B5

L3 — Integration
  └── B12  E2E triage loop test
           depends on: B7, B8, B9, B10, B11a, B11b
```

**Pin-layer placement.** `feed.Assemble` operates on already-classified
`Item` slices; filter resolution happens earlier when the caller
(`cmd/next.go`) constructs `BeadToWork` and `feed.BeadSource` emits one
item per `(bead, matched-work)` pair from that map. The pin override must
mutate `BeadToWork` *before* `BeadSource` runs — i.e. the caller
(`cmd/next.go` / `cmd/triage.go`) applies `PinAssignments` to its
`BeadToWork` after the filter join and before constructing `feed.Input`,
OR `BeadSource` itself consults `PinAssignments` at emission time. Putting
the pin step in `Assemble` is too late — items are already emitted. B5
wires the override into the `BeadToWork`-construction step (or
equivalently inside `BeadSource`); the helper SHOULD be exposed as
`feed.ResolvePins(beadToWork, pinAssignments)` for the caller to invoke
before populating `feed.Input.BeadToWork`. Tests in B5 must exercise
`BeadSource` end-to-end, not just `Assemble`, to ensure the placement is
correct.

## Inter-Package Import Map

```
cmd/triage.go       ─► internal/drift
                       internal/feed
                       internal/beads
                       internal/spec
                       internal/config
                       internal/cmdutil

cmd/pin.go          ─► internal/spec        (NEW: mutate + read PinnedBeads)
                       internal/beads       (verify bead exists)
                       internal/cmdutil

cmd/work_edit.go    ─► internal/spec        (NEW: bead_filter clause mutators)
                       internal/beads       (NEW: ParseFilterClause)
                       internal/cmdutil

cmd/new.go          ─► internal/spec        (now writes PinnedBeads:[] + parsed filter)
                       internal/beads       (NEW: ParseFilterClause)
                       (existing imports unchanged)

cmd/next.go         ─► internal/drift       (NEW: read cache, compute drift)
                       internal/feed        (drift counters via feed.Input)
                       (existing imports unchanged)

cmd/show.go         ─► internal/drift       (NEW: drift markers per bead)
                       internal/beads       (List + filter resolve)
                       internal/spec        (read PinnedBeads)

internal/feed       ─► internal/drift       (NEW: consumes drift.Result)
                       internal/beads
                       internal/spec
                       internal/config
                       internal/queue

internal/drift      ─► internal/beads       (Bead type only)
                       (no other internal imports — near-leaf)

internal/spec       ─► internal/beads       (NEW: mutate.go calls
                                            beads.ParseFilterClause for
                                            the bead_filter clause path)
                       (mutate.go reuses go-yaml v3 node-based
                       decode/encode for comment preservation)

internal/beads      ─► (unchanged externally; gains a new
                       ParseFilterClause symbol that lives next to
                       Filter.Match — clause syntax stays beside the
                       matcher it parses for)
```

Leaves: `internal/beads` (gains `ParseFilterClause`, otherwise unchanged),
`internal/drift` (new, depends only on `internal/beads.Bead`),
`internal/spec` (gains mutators; takes a new internal dep on
`internal/beads` for clause parsing only — `beads` does not import
`spec`, so no cycle). No cycles.

## Cross-Cutting Concerns

| Concern | Beads | Spec section |
|---|---|---|
| Bead-record hash scope (id + status + sorted labels + title + sorted deps) | B2 (impl), B4 (consumes), B12 (E2E) | `coordination.md` §"Hash scope" |
| Snapshot shape (`snapshot_id`, `beads`, `filter_assignments`) | B2 (write + read + --ack advance), B8 (presents), B12 (asserts) | `coordination.md` §"Snapshot shape" |
| `--ack`-only baseline advancement (no implicit advance from `new`/`pin`/`work edit`) | B8 (only writer), B9/B10/B11a (must NOT write) | `coordination.md` §"Baseline advancement" |
| Single-owner pin semantics (pinning to A removes from B) | B1 (mutator helper), B9 (enforces), B5 (applies post-filter, pre-BeadSource) | `coordination.md` §"Pin layer"; `commands.md` §`kerf pin` step 4 |
| Pin layer applied AFTER filter resolution and BEFORE `BeadSource` emits items; pin wins over filter overlap | B5 (impl), B4 (multi-match detector excludes pinned), B12 (E2E) | `coordination.md` §"Pin layer" |
| Comment-preserving YAML round-trip on `spec.yaml` | B1 (impl), B9/B10/B11a (callers), B12 (asserts a comment survives) | `_plan.md` Implementation Notes |
| Empty-baseline first-run = full inventory | B2 (returns empty baseline when cache absent), B8 (does not crash), B12 | `coordination.md` §"Baseline advancement" (last paragraph) |
| `not_initialized` exit code from `kerf triage` | B8 | `commands.md` §`kerf triage` exit codes |
| `--resolved` exit code matrix (0/1/2/3) | B8 (impl), B12 (E2E asserts each path) | `_plan.md` `--resolved` table; `commands.md` §`kerf triage` |
| Drift markers in `kerf show` attached-beads block | B7 | `commands.md` §`kerf show` attached-beads block |

## Per-Bead Specification

### Bead 1 — `internal/spec/mutate.go`: comment-preserving YAML mutators (L0)

**Specs:** `commands.md` §`kerf pin` step 5 ("preserving comments and surrounding YAML formatting"), §`kerf work edit` step 3; `works.md` `pinned_beads:` row.
**Package / files:** `internal/spec/mutate.go` (new), `internal/spec/mutate_test.go` (new), `internal/beads/filter.go` (extend with `ParseFilterClause`), `internal/beads/filter_test.go` (extend). Does **not** modify `spec.go` (that's B3 — disjoint files).
**Deliverables:**
- `func AddPinnedBead(path, beadID string) error` — appends `beadID` to the `pinned_beads:` list in `spec.yaml`, creating the key with surrounding blank-line whitespace preserved if it does not yet exist. No-op if already present.
- `func RemovePinnedBead(path, beadID string) error` — removes `beadID` from `pinned_beads:` if present; leaves the list (possibly empty) and surrounding comments intact. No-op if absent.
- `func AddBeadFilterClause(path string, clause string) error` — parses `clause` (form `label=...` or `id_prefix=...`) via `beads.ParseFilterClause` (NEW symbol introduced by this bead in `internal/beads/filter.go` — see note below) and appends as a new `any:` entry, lifting a single direct clause to an `any:` list if needed. Idempotent.
- `func RemoveBeadFilterClause(path string, clause string) error` — removes a matching clause from the `any:` list, collapsing to a single direct clause if exactly one remains, or removing the `bead_filter` key entirely if zero remain.
- **`func beads.ParseFilterClause(s string) (*beads.Filter, error)`** — single-clause parser. Lives in `internal/beads/filter.go` (next to `Filter.Match`), NOT in `internal/spec`. Clause syntax is a property of the matcher; keeping the parser beside the matcher prevents `internal/spec` from reimplementing clause syntax. B1 owns introducing this symbol (in `internal/beads/filter.go` + matching test in `internal/beads/filter_test.go`); B9, B10, B11a all import it directly from `internal/beads`.
- All mutators round-trip through `yaml.Node` so head/foot/line comments survive.

**Tests:**
- Pin add/remove round-trips with a comment line above `pinned_beads:` — the comment survives.
- Filter-clause add lifts a single-label `bead_filter: {label: "x:y"}` into `any: [{label: "x:y"}, {label: "z:w"}]`.
- Filter-clause remove collapses two→one (back to direct form).
- Round-trip on a fixture `spec.yaml` containing inline + head comments produces byte-equal output for an idempotent operation (remove-then-add same value).
- `beads.ParseFilterClause` rejects unknown forms (`all=...`, malformed) with a stable error message; accepts `label=<v>` and `id_prefix=<v>`.

**YAML Node API budget.** `gopkg.in/yaml.v3` is already in `go.mod` (currently indirect; will become direct). Idempotent byte-equal re-encode requires careful handling of style flags (`FlowStyle`, indent, head/foot/line comment slots) — in particular, an empty `pinned_beads: []` must render as flow `[]` rather than block. Budget more than a "land cheaply" assumption (per architecture critique).

---

### Bead 2 — `internal/drift`: snapshot capture, hash, diff, and cache lifecycle (L0)

**Specs:** `coordination.md` §"Sync cache", §"Snapshot shape", §"Hash scope", §"Drift detection on every read", §"Baseline advancement"; `architecture.md` §"In the repo, inside git" sync-cache entry (bench vs local paths).
**Package / files:** `internal/drift/drift.go`, `internal/drift/hash.go`, `internal/drift/cache.go`, `internal/drift/drift_test.go`, `internal/drift/cache_test.go` (new package). Cache lifecycle is folded in here (was previously planned as a separate B6) — `cache.go` is four small functions and splitting only added an inter-bead dep on the critical path.
**Deliverables:**

*Snapshot + hash + diff:*
- `type Snapshot struct { SnapshotID string; CapturedAt time.Time; Beads map[string]BeadRecord; FilterAssignments map[string][]string }` with snake_case JSON tags matching the spec.
- `type BeadRecord struct { Status string; Labels []string; Hash string }`.
- `func HashBead(b beads.Bead) string` — sha256 over `id\x00status\x00<sorted labels joined>\x00title\x00<sorted dep ids joined>`. Stable; documented byte format.
- `func Capture(all []beads.Bead, assignments map[string][]string) Snapshot` — sorts labels, computes per-bead hashes, computes `SnapshotID` as sha256 over `<id>:<hash>` lines sorted by id.
- `type Diff struct { New, Deleted, ClosedExternally, ReopenedExternally, Changed []string }` (bead IDs).
- `func Compute(baseline, current Snapshot, closedStatuses map[string]bool) Diff` — categorizes per spec. `Changed` covers relabel / retitle / dependency change (same id, same status, different hash).
- Closed-status set is passed in (caller knows the bead tool's terminal statuses); no hard-coded list here.

*Cache lifecycle (folded in from former B6):*
- `func CachePath(projectID, repoRoot string, storageMode config.StorageMode) string` — returns `~/.kerf/projects/{id}/sync-cache.json` for bench mode or `{repo}/.kerf/sync-cache.json` for local mode (honors plan-004 storage modes).
- `func Read(path string) (Snapshot, bool, error)` — returns `(zero, false, nil)` if the file does not exist (first-run empty-baseline case); `(snapshot, true, nil)` on success; error on parse failure.
- `func Write(path string, snap Snapshot) error` — atomic write (temp file + rename), permission `0o644`. Creates parent dir if needed.
- `func Advance(path string, current Snapshot) error` — convenience wrapper used only by `cmd/triage.go --ack`. Equivalent to `Write` but documented as the single legitimate baseline-advance path.

**Tests:**
- Hash stability: reordering labels does not change the hash; reordering deps does not change the hash; changing title does.
- `Compute` truth table: each of the five categories with a one-bead fixture; combined-category run with five beads each illustrating one category.
- Empty baseline (first run) → every current bead in `Diff.New`.
- Identical snapshots → all categories empty.
- `Read` of nonexistent path returns `(_, false, nil)`.
- Round-trip: Capture → Write → Read produces an equal Snapshot.
- Atomic write: a crash mid-write (simulated by writing to the temp path then injecting a failure before rename) leaves the original cache file intact.
- Bench vs local path selection.

---

### Bead 3 — `internal/spec/spec.go`: PinnedBeads field on SpecYAML (L1)

**Specs:** `works.md` §"spec.yaml schema" `pinned_beads` row; `commands.md` §`kerf new` step 6 (empty `pinned_beads` list at creation).
**Package / files:** `internal/spec/spec.go` (modify), `internal/spec/spec_test.go` (modify).
**Deliverables:**
- Add `PinnedBeads []string \`yaml:"pinned_beads"\`` to `SpecYAML`. **No `omitempty`** — empty list must serialize as `pinned_beads: []` per spec (works.md row says yes-required, default `[]`).
- Update `Validate()` (if present) to reject duplicate bead IDs within the list.
- Update existing default-construction sites (used by `cmd/new.go` today) to initialize the field to a non-nil empty slice so YAML output renders `[]`.

**Tests:**
- Round-trip with `pinned_beads: [hk-cb-001]`.
- Round-trip with empty list produces `pinned_beads: []` (golden-byte assert).
- Duplicate-ID validation rejects.
- Extend `spec_property_test.go` / `spec_fuzz_test.go` with the new field.

---

### Bead 4 — `internal/feed/warning.go`: triage detectors (L1)

**Specs:** `coordination.md` §"Composition with other detectors" (the three new warning kinds: `untriaged_beads`, `multi_matched_bead`, `external_drift`); `commands.md` §`kerf next` warning header.
**Package / files:** `internal/feed/warning.go` (extend), `internal/feed/warning_test.go` (extend). Does **not** create new files — keeps the package flat.
**Deliverables:**

*Explicit rename (deliverable, not a side note):*
- Rename the existing plan-006 `unmatchedBeadsDetector` (currently at `internal/feed/warning.go:53`) to `untriagedBeadsDetector`. The spec wording is `untriaged_beads`; the surfaced warning kind constant moves with it.
- Rewire the registration in `NewWarningDetectors` (`internal/feed/warning.go:42`) accordingly.
- **Callers audit:** grep `cmd/` and `internal/feed/` for `unmatchedBeadsDetector`, `UnmatchedBeads`, and `unmatched_beads` (case variants), updating every reference — including test names, golden-file fixtures, and any kind-string constants. The bead is not done until the audit is clean. Document in PR description which call sites moved.
- Update any plan-006 test fixtures that asserted on the old kind string.

*New detectors:*
- `UntriagedBeads` detector (the renamed surface, with updated semantics): a bead is untriaged iff it matches no work's resolved filter AND is not in any work's `pinned_beads`. Emits a single warning item with the count.
- `MultiMatchedBead` detector: a bead is multi-matched iff it matches ≥2 works' resolved filters AND is not pinned to any work. (Pin overrides multi-match per coordination.md §Pin layer.) Emits one warning item per multi-matched bead.
- `ExternalDrift` detector: reads `feed.Input.DriftResult` (added by B5) and emits one warning per non-empty drift category, summarizing counts. No-ops when `DriftResult` is the zero value (cache absent or read failed → caller responsibility to handle).
- Mutual-exclusivity guard with `WorkNoAttachedBeads` (B4 from plan 006): `UntriagedBeads` and `WorkNoAttachedBeads` operate on disjoint sets — one is about beads, the other about works — but ensure the existing plan-006 detector's emission is not duplicated.

**Tests:**
- Rename audit: a unit test asserts the new `untriaged_beads` kind constant; no test still references the old `unmatched_beads` string.
- `UntriagedBeads` fires for a bead matching no filter and no pin; clears once pinned.
- `MultiMatchedBead` fires for a bead matching two works; does not fire once pinned to one.
- `ExternalDrift` fires for each non-empty `Diff` category; renders categorized count lines.

---

### Bead 5 — `internal/feed/feed.go`: drift + pin layer wired before BeadSource (L1)

**Specs:** `coordination.md` §"Pin layer" (resolution order: filter → pin → render), §"Drift detection".
**Package / files:** `internal/feed/feed.go` (modify), `internal/feed/item.go` (extend Item if needed), `internal/feed/feed_test.go` (extend).

**Pin-layer placement (load-bearing).** Filter resolution into `BeadToWork` happens in the **caller** (`cmd/next.go` constructs `BeadToWork` from each work's resolved filter and hands it to `feed.Input`). `BeadSource` then emits one item per `(bead, matched-work)` pair from `BeadToWork`. The pin override must therefore mutate `BeadToWork` BEFORE `BeadSource` runs — applying it inside `Assemble` is too late (items are already emitted). Mutate `BeadToWork` (in the caller, or inside `BeadSource`) — NOT inside `Assemble`.

**Deliverables:**
- Extend `feed.Input` with: `DriftResult drift.Diff`, `PinAssignments map[string]string` (bead ID → owning work codename).
- Add `func ResolvePins(beadToWork map[string][]string, pinAssignments map[string]string) map[string][]string` in `internal/feed/` — a pure helper that returns a new `BeadToWork` map where each pinned bead's slice is replaced with `[owningWork]`, and beads pinned but absent from `beadToWork` are added with `[owningWork]` (so pins surface beads that wouldn't otherwise attach). The caller (`cmd/next.go`, `cmd/triage.go`) invokes `ResolvePins` after building the filter-driven `BeadToWork` and before populating `feed.Input.BeadToWork`.
- `BeadSource` is unchanged in shape (still reads `in.BeadToWork`) but its godoc gains a note pointing at `ResolvePins` as the canonical pin-application step.
- Pre-condition check in `ResolvePins`: if a bead ID appears in two works' `PinnedBeads` lists (i.e. `PinAssignments` has been collapsed from multiple-owner input), that is a single-owner invariant violation. `ResolvePins` does not see the conflict directly because `PinAssignments` is already a `map[string]string`; the conflict surface lives in the caller, which builds `PinAssignments` by scanning every active work's `spec.yaml.PinnedBeads`. If the caller detects a bead pinned to two works, it MUST emit a `warning` item with kind `pin_conflict` (added to `internal/feed/warning.go` as part of B4 — coordinate constant placement) and use the lexicographically-earliest codename as winner. Defense-in-depth against a manual edit to `spec.yaml`; `cmd/pin.go` (B9) is the primary enforcement.

**Tests (must exercise `BeadSource` end-to-end, not just `Assemble`):**
- Pin override: bead B matches works A and C in `BeadToWork`; `ResolvePins` with `PinAssignments[B]=A` → `BeadSource` emits one item with `work_codename=A`.
- Pin without filter match: bead B absent from `BeadToWork` but `PinAssignments[B]=A` → `BeadSource` emits one item under A (this is the whole point of pins).
- Two-owner conflict: caller-side handler emits `pin_conflict` warning; assert the kind appears in the warning slice.
- DriftResult passthrough: a populated `DriftResult` does not affect bead/cleanup items, only drives B4's `ExternalDrift` detector.

---

### Bead 7 — `cmd/show.go`: Attached beads block + drift markers (L2)

**Specs:** `commands.md` §`kerf show` "Attached beads" block (line 251).
**Package / files:** `cmd/show.go` (modify), `cmd/show_test.go` (extend).
**Pre-req:** plan 008 B1 (rewrite of `cmd/show.go:278` to use `internal/beads.List`) has landed on main.
**Deliverables:**
- After existing show output, render an `Attached beads (N open / M closed)` block when the work has at least one attached or pinned bead. Each line: `<bead-id>  <status>  <title>  [(pinned)]  [! <drift-marker>]`.
- "Attached" = beads matching the work's resolved filter, **plus** beads in `PinnedBeads`, minus duplicates.
- Drift markers consulted from `internal/drift`: `closed externally`, `reopened externally`, `relabeled`, `retitled`, `deleted` (a deleted bead appears with its baseline title and a `! deleted` marker — beads only disappear from this block when `--ack` removes them from the baseline).
- Drift-marker rendering does **not** advance the baseline.
- When the bead store is unavailable (`internal/beads.IsAvailable() == false`), omit the block silently rather than erroring.

**Tests:**
- Block renders for a work with three attached beads (two open, one closed); counts in the header line match.
- A pinned bead that does not match the filter appears with `(pinned)`.
- A bead closed since the baseline shows `! closed externally`.
- A deleted bead still appears with `! deleted`.
- Bead store unavailable → no block, command succeeds.

---

### Bead 8 — `cmd/triage.go`: new command (L3)

**Specs:** `commands.md` §`kerf triage` (full section, including exit codes 0/1/2/3 and `--resolved`/`--ack`/`--format`/`--kind` flags); `_plan.md` `--resolved` exit-code table.
**Package / files:** `cmd/triage.go` (new), `cmd/triage_test.go` (new).
**Deliverables:**
- Cobra subcommand registered via `init()` in `cmd/triage.go` itself (matching the existing pattern at `cmd/new.go:66-71`, `cmd/next.go:81-86`). **No bead touches `cmd/root.go`** — each new command file self-registers via `init()`.
- Sections (in spec order):
  1. **Untriaged** — beads matching no filter and not pinned; grouped by their primary label; each bucket emits a templated `kerf new` / `kerf work edit --bead-filter-add` / `kerf pin` suggestion.
  2. **Multi-matched** — beads matching ≥2 works; template suggestion is `kerf pin <codename> <bead-id>` (per Open Question #2 resolution: pin is the convergence path — narrowing filters does not converge, pins do).
  3. **External changes since last triage** — from `drift.Diff` categories.
  4. **Per-work bead health** — open/closed counts, filter expression, pin count.
- Flags:
  - `--resolved` — exits per the table: 0 when all three convergence conditions hold (no untriaged, no multi-matched, no unacked drift); 1 when not initialized; 2 when drift exists and no progress since last `--resolved` run; 3 when drift exists but resolved-count strictly decreased.
  - `--ack` — writes current snapshot to the sync cache via `drift.Advance`; does **not** clear pins or filters, only the baseline.
  - `--format=json` — emits the kind-tagged item stream matching `kerf next --format=json` shape, plus a top-level `summary` object with counts.
  - `--kind=<kind>` — filter sections (`untriaged`, `multi_matched`, `external_drift`, `work_health`).
  - `--project=<id>` — standard project selector.
- Progress tracking for exit code 3: stores the last `--resolved` invocation's unresolved counts in `sync-cache.json` under a `last_resolved_counts` field (extension to the snapshot shape; documented as kerf-internal metadata, not part of the canonical snapshot shape that B2 tests against). This field is written via `drift.Write` (from B2's cache lifecycle); `drift.Advance` is the `--ack` advance path.
- `not_initialized` path: when `project.yaml` is missing, exit 1 with `kind: not_initialized` JSON and a one-liner `run kerf init first` (text mode).

**Tests:**
- Each section renders correctly against a fixture project with seeded untriaged, multi-matched, and externally-closed beads.
- `--resolved` exit codes: clean project → 0; uninitialized → 1; stuck → 2; progress → 3.
- `--ack` rewrites the cache file; subsequent `--resolved` returns 0.
- JSON shape matches the spec (kind-tagged stream + summary).
- `--kind` filters sections.

---

### Bead 9 — `cmd/pin.go`: new command (L3)

**Specs:** `commands.md` §`kerf pin` (steps 1–7); `coordination.md` §"Pin layer" single-owner rule.
**Package / files:** `cmd/pin.go` (new), `cmd/pin_test.go` (new). Self-registers via `init()` (existing `cmd/` pattern).
**Deliverables:**
- `kerf pin <codename> <bead-id> [--project <id>]`.
- Step 4 enforcement: scan every other active work's `spec.yaml`; for any other work with `bead-id` in `PinnedBeads`, call `spec.RemovePinnedBead` on that file. Single-owner invariant.
- Step 5: call `spec.AddPinnedBead` on the target work's `spec.yaml`.
- Step 7: do **not** call `drift.Advance`. The command must not advance the baseline.
- Exit 0 on success. Validate that the bead ID exists in the current bead-store snapshot (use `beads.List`); error if not. Validate that the codename resolves to an active work; error if not.

**Tests:**
- Pinning B to work A removes B from work C's `pinned_beads`.
- Idempotent: pinning B to A twice leaves a single entry.
- Bead ID does not exist in the store → error, no file change.
- Baseline file (`sync-cache.json`) is byte-unchanged after `kerf pin` runs.
- Comment-survival assert via B1 mutators (a head comment on the target `spec.yaml` survives).

---

### Bead 10 — `cmd/work_edit.go`: bead-filter mutators (L3)

**Specs:** `commands.md` §`kerf work edit` (full section, including step 7 "do not advance the drift baseline").
**Package / files:** `cmd/work_edit.go` (new — a cobra subcommand under a new `work` parent if one does not exist; otherwise inline under the existing `cmd/work.go`). **File ownership note:** there is no existing `cmd/work.go` in the tree today, so this bead creates the file fresh; no contention.
**Deliverables:**
- `kerf work edit <codename> [--bead-filter-add <clause>...] [--bead-filter-remove <clause>...] [--project <id>]`.
- Repeatable flags; each clause goes through `beads.ParseFilterClause` (from B1; lives in `internal/beads`) and `spec.AddBeadFilterClause` / `spec.RemoveBeadFilterClause`.
- Step 7: do not advance the baseline.
- Exit 0 on success; non-zero on parse error or codename miss.

**Tests:**
- Adding two clauses produces an `any:` list.
- Removing the last clause removes the `bead_filter` key entirely.
- Round-trip preserves comments on the target `spec.yaml`.
- Baseline unchanged after invocation.

---

### Bead 11a — `cmd/new.go`: `--bead-filter` flag (L3)

**Specs:** `commands.md` §`kerf new` step 6 (parse `--bead-filter` into the filter shape and write under `bead_filter`).
**Package / files:** `cmd/new.go` (modify), `cmd/new_test.go` (extend).
**Deliverables:**
- Add `--bead-filter <clause>` flag (one-shot; not repeatable per spec — multi-clause is what `kerf work edit` is for).
- Parse via `beads.ParseFilterClause` (introduced in B1, lives in `internal/beads`); write into `SpecYAML.BeadFilter` before the file is written for the first time.
- Initialize `PinnedBeads` to a non-nil empty slice (uses B3 field).
- Do not advance the baseline.

**Tests:**
- `--bead-filter 'label=subsystem:bridge'` writes the expected YAML shape.
- Omitting the flag leaves `bead_filter` absent (omitempty preserved).
- `PinnedBeads: []` always renders on first create.

---

### Bead 11b — `cmd/next.go`: drift-summary headline counters (L3)

**Specs:** `commands.md` §`kerf next` drift summary line (the `! 6 untriaged · ! 2 multi-matched · ! 1 deleted externally` example at line 1520).
**Package / files:** `cmd/next.go` (modify), `cmd/next_test.go` (extend).
**Deliverables:**
- Read sync cache (B2 cache lifecycle), compute drift (B2), build `feed.Input.DriftResult` and `PinAssignments`.
- Render a single drift-summary line above the existing warning block when any of (untriaged, multi-matched, external drift) is non-zero. Categories with zero counts are omitted; when all three are zero, the line is not rendered.
- JSON output gains a top-level `drift_summary` object alongside the existing item stream.
- This bead does **not** advance the baseline.

**Tests:**
- Drift summary line renders with three non-zero categories; matches the exact format from the spec example.
- Zero drift → no line.
- JSON `drift_summary` shape.
- Sync cache absent → empty baseline → first-run shows everything as new; text rendering still works.

---

### Bead 12 — E2E triage loop test (L4)

**Specs:** the full triage agent workflow in `_plan.md` "Triage agent workflow (canonical)".
**Package / files:** `cmd/triage_e2e_test.go` (new), reuses fixtures from `cmd/coordination_e2e_test.go` if shape fits.
**Deliverables:**
- E2E scenario:
  1. Seed a project with two works (`bridge`, `gateway`) and `project.yaml` `bead_filter: {label: "subsystem:{codename}"}`.
  2. Seed a stub bead store with: three matching beads per work, one unmatched bead, one bead matching both works.
  3. Run `kerf triage` → assert Untriaged section lists the unmatched bead; Multi-matched section lists the dual-match bead; External changes section is empty (first run, empty baseline → everything is "new"); per-work health renders.
  4. Run `kerf triage --resolved` → exit 2 (drift exists, no progress yet).
  5. Run `kerf new <new-work> --bead-filter 'label=subsystem:<label>'` for the unmatched bead's label.
  6. Run `kerf pin bridge <dual-match-bead-id>`.
  7. Run `kerf triage --ack`.
  8. Run `kerf triage --resolved` → exit 0.
- Loop convergence: a `until kerf triage --resolved` shell loop wrapping steps 5–7 must terminate.
- `kerf show bridge` renders the pinned bead with `(pinned)` marker.
- `kerf show` drift marker test: externally close a bead (mutate the stub store), do NOT `--ack`, assert `kerf show` shows `! closed externally`.
- Comment-survival assert: a head comment on one work's `spec.yaml` survives `kerf pin` + `kerf work edit --bead-filter-add`.

**Tests:** the above scenario as a single Go test plus sub-tests per assertion.

## Parallelization Plan

| Phase | Beads | Workers | Depends on |
|---|---|---|---|
| L0 | B1 (`internal/spec/mutate.go` + `internal/beads.ParseFilterClause`), B2 (`internal/drift` incl. cache lifecycle), B3 (`internal/spec/spec.go`) | 3 | — |
| L1 | B4 (warning detectors + rename), B5 (feed Input + pin layer pre-`BeadSource`), B9 (`cmd/pin.go`), B10 (`cmd/work_edit.go`), B11a (`cmd/new.go`) | up to 5 | B4: B2, B3; B5: B2, B3; B9: B1, B3; B10: B1; B11a: B1, B3 |
| L2 | B7 (`cmd/show.go`), B8 (`cmd/triage.go`), B11b (`cmd/next.go`) | up to 3 | B7: B3, B5, plus plan 008 B1; B8: B2, B4, B5; B11b: B2, B4, B5 |
| L3 | B12 (E2E) | 1 | B7, B8, B9, B10, B11a, B11b |

Total: **11 beads, 4 layers**, peak concurrency **6** (the maximum number of beads in flight at once across all phase boundaries — counting B2's L0 work overlapping with L1 fan-out, or equivalently the original critique's L1' fan-out before B6 was folded into B2). Critical path length **4 hops**: B2 → B5 → B8 → B12 (or equivalently B2 → B4 → B8 → B12). L1 is fully parallel because B4/B5 own distinct files in `internal/feed`, and B9/B10/B11a each own a distinct `cmd/*.go`. No bead touches `cmd/root.go`; each new command self-registers via `init()`.

---

## File-Ownership Contention (resolved)

Multiple beads touch `internal/spec` and `internal/feed`. Resolved by file-level split:

| File | Owning bead |
|------|-------------|
| `internal/spec/spec.go` | B3 only (adds `PinnedBeads` field) |
| `internal/spec/mutate.go` | B1 only (new file; mutators) |
| `internal/beads/filter.go` | B1 (extends with `ParseFilterClause`); existing matcher untouched |
| `internal/feed/feed.go` | B5 only (Input + `ResolvePins` helper; pin layer applied to `BeadToWork` before `BeadSource`) |
| `internal/feed/warning.go` | B4 only (rename + three new detectors) |
| `internal/feed/item.go` | B5 if needed; otherwise untouched |
| `internal/drift/drift.go` + `hash.go` + `cache.go` | B2 only |
| `cmd/show.go` | B7 only |
| `cmd/new.go` | B11a only |
| `cmd/next.go` | B11b only |
| `cmd/triage.go` | B8 only (self-registers via `init()`) |
| `cmd/pin.go` | B9 only (self-registers via `init()`) |
| `cmd/work_edit.go` | B10 only (self-registers via `init()`) |

`cmd/root.go` is **not** touched by any bead. Existing pattern (verified at `cmd/new.go:66-71`, `cmd/next.go:81-86`) is per-file `init()` + `rootCmd.AddCommand`; every new command in this plan follows it. There is no `root.go` serialization risk and no special sequencing required.

---

## Judgment Calls (Resolved)

1. **Owner of `Filter` parsing.** `beads.ParseFilterClause` lives in `internal/beads/filter.go` (B1 introduces it), NOT `internal/spec`. Rationale: clause syntax is a property of the matcher (`Filter.Match`); putting the parser beside the matcher prevents `internal/spec` from reimplementing clause syntax. `internal/spec/mutate.go` calls `beads.ParseFilterClause` for the bead-filter clause path. (Reversal of an earlier judgment; per architecture critique.)
2. **Pin-layer placement.** Pin layer is applied to `BeadToWork` BEFORE `BeadSource` emits items — via a new `feed.ResolvePins` helper invoked by the caller (`cmd/next.go`, `cmd/triage.go`) after the filter-driven join and before populating `feed.Input.BeadToWork`. NOT applied inside `feed.Assemble`, because by the time `Assemble` runs, items have already been emitted from `BeadSource`. Pins remain an attachment-layer concept (not a bead-matching one), so the helper lives in `internal/feed`; `internal/beads` stays a leaf.
3. **`UntriagedBeads` vs plan-006 `UnmatchedBeads`.** Rename plan 006's `UnmatchedBeads` to `UntriagedBeads`; the spec calls it `untriaged_beads`. B4 owns the rename and any caller updates.
4. **Snapshot extension for progress tracking.** `last_resolved_counts` is kerf-internal sync-cache metadata; not part of the canonical snapshot shape that external tooling might consume. B8 owns this field; B2's tests assert only the spec-canonical shape.
5. **Empty-baseline first run.** Per spec, the first read with no cache treats the baseline as empty — every bead is "new" until `--ack`. This is the desired UX (full inventory pass) and B2's `Read` returns `(zero, false, nil)` for missing files to make this explicit.
6. **Defense-in-depth pin conflict.** Two-owner pins should never happen because B9 enforces single-owner, but B5 emits a `pin_conflict` warning if it sees one — cheap insurance against a manual edit to `spec.yaml`.
7. **`cmd/work_edit.go` file naming.** No existing `cmd/work.go`. Create `cmd/work_edit.go`; if a future `kerf work <other-subcommand>` lands, the parent cobra group can be lifted into `cmd/work.go` then.
