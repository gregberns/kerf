# Plan 019 — Filter Bootstrap and `kerf next` Payload-First Surface

> **Status: baked.** Spawned from Plan 015 (harmonik beta-feedback triage). Expanded per the `plan-implementation` flow.

## Intent

A kerf work attaches to its beads via a `bead_filter` declared on the work's `spec.yaml` (and resolved through the project-wide filter in `project.yaml`). When that wiring is missing or wrong, the work is silently disconnected from the bead store: `kerf next` reports it as `clean`, agents read "clean" as "good," and the daily-driver loop stops surfacing real work. During the harmonik dogfood session (2026-05-15 → 2026-05-18), closing the 168-bead untriaged gap took four manual `kerf work edit` invocations, only one of the four works actually picked up its beads (label-convention drift between `codename:foo` and bare `foo`), and `kerf next` opened with 70+ lines of drift output before any actionable item. This plan ships the bootstrap primitive that closes the gap in one command, makes the filter slot visible everywhere it ought to be, and reorders the `kerf next` payload so the agent sees the work first and the diagnostics second.

## Background

Items come from `plans/015_harmonik_beta_feedback/triage.md`:
- Theme 3 (`kerf next` ranking + entry friction) — items 3.2, 3.3.
- Theme 5 (work bead-filter bootstrap) — items 5.1, 5.2, 5.3, 5.4, 5.5.
- Theme 6 (filter-syntax / convention drift) — items 6.1, 6.2, 6.3.

Harmonik bead `hk-43ate` already tracks the `clean` vs. `empty` vs. `unwired` distinction from the consumer side; this plan is the kerf-side fix that closes it out.

## Scope

In scope:
- A one-shot bootstrap primitive that samples existing bead labels, proposes a per-work filter for every work that lacks one (or has an empty resolved set), and applies the proposals after agent confirmation.
- A convention-aware label sampler that recognizes both prefixed (`codename:foo`) and bare (`foo`) label families and proposes the dominant pattern per work — not a single project-wide assumption.
- `kerf next` reordered to lead with ranked payload items; drift and untriaged counts collapse into a single-line footer with a hint pointing at the remediation command; work-level warning rows (with rank labels below) render as a short stanza after that footer, not as a header block above the payload.
- Distinct rank labels for works whose filters resolve to zero beads: `empty` (filter present, no current matches), `unwired` (no `bead_filter` declared), `broken` (filter present but malformed or referencing a label family that exists nowhere).
- A near-match advisor on `kerf next`: when a work has a `clean`/`empty` filter that looks like a one-character or one-prefix swap from a heavily-populated label family, surface the suggested edit inline (one line per work, suppressed when the swap isn't unambiguous).
- `kerf show <codename>` always displays the `bead_filter:` slot — current literal value or `(none)`.
- `kerf work show <codename>` — a single-work dump that prints `spec.yaml` field-by-field without forcing agents to parse YAML.
- `kerf work edit` confirmation message disambiguates open vs. closed: `Now matches: N (M open / K closed). Previously: P (Q open / R closed).`
- `kerf new` always emits a `bead_filter:` key in the new work's `spec.yaml`, even when its value is empty — eliminating the silent-`unwired` case where the key is absent entirely.
- A `--created-by self` filter on `kerf list` (or per-session attribution on each row) so multi-agent works don't appear in another agent's `kerf list` output without an attribution marker.

Out of scope:
- The underlying bead-filter query language (the `label=`, `id_prefix=`, `any:` grammar is a separate surface; this plan reads and writes existing grammar, doesn't extend it).
- The `kerf next` ranking algorithm itself (Plan 014 owns the algorithm; this plan only owns the output ordering and rank-label vocabulary).
- Project-wide bootstrap that writes `project.yaml`'s `bead_filter` slot — that wiring overlaps Plan 016's init surface (see Conflicts below).
- Bead-store drift detection beyond what already exists; this plan consumes the existing baseline.

## Design notes

**Filter syntax surface.** The plan does not invent grammar. Bootstrap reads the existing bead store, proposes clauses in current `label=<value>` / `id_prefix=<value>` form, and applies them through the existing `kerf work edit --bead-filter-add` path. The only new surface is the `bootstrap` verb (or `--infer-from-labels` flag — see open questions) and the sampler that produces proposals.

**Convention handling (prefixed vs. bare).** The harmonik dogfood failure was that kerf assumed `codename:*` labels uniformly while the project's existing beads mixed prefixed and bare conventions. The sampler walks each work in turn:
1. Build the candidate label set: codename, codename with common prefixes (`codename:`, `subsystem:`, `area:`, `kind:`), and the bare codename slug.
2. Count bead matches for each candidate against the open-bead set.
3. If exactly one candidate dominates (≥ 80% of matches across the candidates, with a minimum absolute count threshold), propose that clause.
4. If two or more candidates tie or both have non-trivial counts, propose `any:` of both (the multi-clause path the spec already supports) so the work picks up beads under both conventions.
5. If no candidate yields any matches, propose nothing for that work and surface it under `unwired` with a note that no label resembles its codename.

The sampler is per-work, not project-wide: harmonik's failure was treating prefix choice as a global setting when in practice each work's authors had picked independently.

**`kerf next` payload-first output.** The current command leads with drift warnings (untriaged count, external_close, external_new, multi_matched), then ranked items. After this plan, the layout is:
1. Ranked items (beads + cleanups), one per line.
2. Empty-feed fallback text only when there are zero ranked items.
3. Footer line: `drift: N untriaged · M external · K multi-matched — run 'kerf triage'` rendered only when any counter is non-zero.
4. Warning block (work-level configuration issues) compressed into a single short stanza after the footer, with rank labels (`empty`, `unwired`, `broken`) instead of the current `clean`.

The reordering is presentation-only; the underlying feed assembly is unchanged.

**Rank-label vocabulary.** Replace `clean` (which was ambiguous) with:
- `empty` — `bead_filter` declared, syntactically valid, matches zero open beads. Likely benign; the work is wired but its beads haven't been created yet.
- `unwired` — no `bead_filter` key on `spec.yaml`, or key present but value is empty. The agent needs to bootstrap or edit.
- `broken` — `bead_filter` declared but malformed (parse error or references a non-existent clause shape).

Cleanup items already keyed on `work_no_attached_beads` continue to fire; only the surface label changes.

**Near-match advisor.** For each work in the `empty` state, the sampler runs against the actual bead store. If exactly one alternate clause would lift the work out of `empty` (e.g., dropping a `codename:` prefix), the warning row appends `— try: kerf work edit <codename> --bead-filter '<proposed>'`. Suppressed when the swap isn't unambiguous (multiple candidates, or none).

**Bootstrap interaction model.** Two surfaces considered:
- *Top-level verb* (`kerf bootstrap-filters` or `kerf work bootstrap`) — discoverable from `kerf --help`, scoped to "do this once per project."
- *Flag on `kerf work edit`* (`--infer-from-labels`) — composes with existing edit semantics, but the bulk-across-all-works case becomes awkward (a loop in the caller).

The plan recommends the top-level verb path and prints a dry-run preview by default; `--apply` actually mutates. Non-interactive mode (`--yes`) skips the confirmation. This question stays open because Plan 016 owns init's interactive-vs-not convention, and bootstrap arguably belongs in the same family.

**Alternatives considered.**
- *Always-on auto-bootstrap inside `kerf init`.* Rejected: init runs once, before most works exist; bootstrap is a recurring operation as new works appear.
- *Strict per-work confirmation prompts in bootstrap.* Rejected: the harmonik pain point was four manual edits; a four-prompt bootstrap doesn't move the needle. Default is "show diff, ask once, apply all."
- *Static rank-label rename without the bootstrap verb.* Rejected on its own — renames `clean` to `unwired` but doesn't close the friction that produced the rename request.

## Spec changes proposed (prose)

- `specs/commands.md`
  - Add `kerf bootstrap-filters` (or merge the surface under `kerf work edit` per open question 1): grammar, flags, interaction model, exit codes, output shape.
  - Add `kerf work show <codename>`: a single-work dump command.
  - Update `kerf show`: include the `bead_filter:` slot in the per-work output unconditionally.
  - Update `kerf work edit`: confirmation message format `Now matches: N (M open / K closed). Previously: ...`.
  - Update `kerf new`: a `bead_filter:` key is always emitted on `spec.yaml`, default value empty.
  - Update `kerf next`: payload-first output ordering; new rank-label vocabulary (`empty` / `unwired` / `broken`); near-match advisor lines on the warning block.
  - Update `kerf list`: `--created-by self` flag (or per-session attribution column).
- `specs/coordination.md`
  - Define `empty` / `unwired` / `broken` rank labels for the cleanup / warning surface. Replace the current `work_no_attached_beads` single-state with the tri-state classification.
- `specs/works.md`
  - Tighten `spec.yaml` schema: `bead_filter` is always present as a key on `kerf new` output (value may be empty / null). Make explicit that "absent key" and "present-but-empty key" are equivalent for filter resolution but only the latter is canonical for new works.
- `specs/sessions.md`
  - Add per-session work attribution if `kerf list --created-by self` lands here (open question 4).

No `MUST` language. No spec edits in this plan — the spec changes are proposed in prose for the implementation phase.

## Beads outline (no `bd`)

Grouped by layer; intended as a sketch, not a contract.

- **B1. Filter slot visible everywhere.** `kerf show` and (new) `kerf work show` render the `bead_filter` slot. `kerf new` emits an always-present key. (5.2, 5.4, 6.3.)
- **B2. Rank-label vocabulary.** Replace `clean` with `empty` / `unwired` / `broken` across cleanup classification and `kerf next` rendering. Depends on resolving open question 5 — if the filter parser cannot distinguish a malformed clause from a zero-match clause, the `broken` label collapses into `empty` and B2 ships as a two-state change. (3.2, 6.2; addresses harmonik bead `hk-43ate`.)
- **B3. `kerf next` payload-first reorder.** Move drift summary to footer; compress warning block. (3.3 partial; remainder belongs to Plan 014.)
- **B4. Label-convention sampler.** Per-work candidate enumeration, dominance check, multi-clause fallback. Standalone library; unit-testable without the CLI surface. (6.1.)
- **B5. Bootstrap command surface.** `kerf bootstrap-filters` (or `kerf work edit --infer-from-labels`) wiring B4 to apply via existing edit semantics; dry-run by default. (5.1.)
- **B6. Edit-confirmation disambiguation.** `kerf work edit` message format change. (5.3.)
- **B7. Near-match advisor.** Inline suggestion rendering on `kerf next` warning rows when the alternate clause is unambiguous. (6.1 extension.)
- **B8. Session attribution on `kerf list`.** Either `--created-by self` flag or per-row marker. (5.5.)

Sequencing intuition: B1 + B2 land first (smallest surface, biggest immediate clarity gain). B4 is the standalone analyzer. B5 depends on B4. B3 is independent. B6, B7, B8 each parallelizable.

## Items absorbed from Plan 015

- 3.2 — `clean` is the wrong rank label for zero-bead works.
- 3.3 — no beads in feed when no work has a filter (the bootstrap-call-to-action half; the drift-relocation half is shared with Plan 014).
- 5.1 — no one-shot bootstrap from existing labels.
- 5.2 — `kerf show` doesn't display `bead_filter`.
- 5.3 — `kerf work edit` count includes closed beads.
- 5.4 — no `kerf work show <codename>` command.
- 5.5 — multi-agent works appear unannounced in `kerf list`.
- 6.1 — bead-label convention split (`codename:foo` vs bare `foo`).
- 6.2 — `clean` conflates three different states.
- 6.3 — `phase-3-dot` had no `bead_filter` field at all.

Harmonik bead `hk-43ate` is the consumer-side tracking issue; the kerf-side fix is B2.

## Open questions

1. Is the bootstrap surface a new top-level verb (`kerf bootstrap-filters` or `kerf work bootstrap`) or a flag on `kerf work edit` (`--infer-from-labels`)? Plan tentatively recommends the verb path because the common case is "do every unwired work at once," which composes poorly as a per-work flag.
2. Interaction model for confirmation: single yes/no for all proposed filters, per-work confirmation, or non-interactive dry-run-then-apply? Tentative default: dry-run by default, single confirmation, `--yes` to skip. Defers to Plan 016's convention.
3. What counts as a "near-match" for the advisor on `kerf next`? Exact label minus a known prefix is unambiguous; broader matching (Levenshtein, fuzzy) risks noisy suggestions. Tentative default: prefix swaps only (`codename:foo` ↔ `foo`, `subsystem:foo` ↔ `foo`, etc.), nothing fuzzier.
4. Per-session attribution on `kerf list` — is the data already on `spec.yaml` (via `sessions:` list, with the first entry as creator)? If so, B8 is rendering-only; if not, it's a schema addition and belongs partly in `specs/sessions.md`.
5. The `broken` rank label assumes the filter parser can distinguish "malformed clause" from "valid clause that happens to match nothing." Does the existing parser surface that distinction, or does it silently accept and return zero matches?

## Conflicts flagged for the orchestrator

- **Plan 016 (init UX overhaul) overlaps on `project.yaml` schema and the interactive-vs-not convention.** Plan 019 needs `kerf new` to emit a `bead_filter:` key and likely wants `project.yaml` to carry a project-wide default filter slot in canonical form. Plan 016 is reworking init's `project.yaml` shape (1.4, 1.8) and the `--yes` / `--no` convention (1.1, 9.1). These plans need to agree on the schema and the prompt convention before either implementation phase begins. **Do not resolve here.**
- **Plan 017 (storage reconciliation) overlaps on `project.yaml` health.** Plan 017's `kerf doctor` reports `bead_filter` coverage; Plan 019 mutates it. Order of operations matters (doctor reads what bootstrap writes). Minor relative to the 016 conflict, but worth surfacing.
