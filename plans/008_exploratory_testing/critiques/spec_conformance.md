# Spec-Conformance Critique

## Spec-already-covered bugs
- **P0.1 work-level `bead_filter` attachment** — `coordination.md` §Filter resolution L238-242. Spec is explicit; `ForWorkWithFilter` join key has drifted.
- **P0.3 / P0.4 `bd list --project`** — `architecture.md:237` / `commands.md` show+map already presume the `internal/beads` path. Shelling `bd` violates configurable tool spec. Pure code drift.
- **P1.5 `EqualFold` in `ForWork`** — `coordination.md:232` "Matching is case sensitive." Code drift, no spec edit.
- **P1.6 unknown status excludes works** — Invariant 5 (`_index.md`) + `commands.md:559` ("warn but proceed"). Code drift.
- **P2 `filter_case_mismatch` detector** — `commands.md:1459` mandates the case-mismatch warning. Spec covers; only generic unmatched fires.
- **P2 Available-commands omissions** — `commands.md:42` requires the list; current root prints a subset. Drift.
- **P0.2 corrupt `spec.yaml`** — Partial: `_index.md` Invariant 2 ("filesystem is the database") implies surfacing, but no spec text mandates a warning row. Borderline — see Spec-update-needed.

## Spec-update-needed (CLAUDE.md says spec first)
- **P1.1 `kerf init` re-run idempotency** — `commands.md` §`kerf init` (L1144-1186) is silent on re-invocation against existing `project.yaml`. **Biggest gap**: needs an explicit "if `project.yaml` exists, merge / skip / abort" subsection before code can be fixed.
- **P1.7 orphan-dir reconciliation** — neither `commands.md` `kerf new` nor `kerf list` defines orphan handling. New spec text needed (`commands.md` §new, §list).
- **P1.8 bare `kerf` cwd scoping** — `commands.md:44` says "current project (if inside a repo)". Ambiguous: needs explicit "scope active-work count to resolved project" line.
- **P1.9 no-`project.yaml` warning** — needs a `warning` kind entry in `commands.md` §`kerf next` and a feed-warning rule in `coordination.md`.
- **P0.2 corrupt-spec warning** — add `corrupt_spec` warning kind to the `kerf next` warning table.
- **Design: `area_diversity` penalty, effort-weighted fan-out, rework cap, staleness, per-work concurrency cap, external priority pin** — `coordination.md` §Computed priority (L167-180) currently enumerates only `fan_out / momentum / creation / rework`. All six require new signals/weights → spec edit in coordination.md + `simulator.md` weights table (L178-181, L222-228).
- **Triage workflow items 3–7 (`--bead-filter`, `work edit`, `attach`, `triage`, drift-state)** — wholly new commands; need sections in `commands.md`, with drift-state persistence touched in `architecture.md`.
- **Workflow: `kerf where`, `kerf doctor`, `kerf verify-tools`, `kerf next --explain`, `kerf status --auto`, `kerf shelve --session-file`** — all net-new surface; `commands.md` additions required.
- **`top_of_queue_churn` ambiguity** — `simulator.md:290` defines numerator/denominator but is silent on "single-candidate" case. Tighten in place.

## Spec conflicts
- **Design: cut `momentum` to 2.0/0.0** — `coordination.md:149` calls momentum a first-class principle ("prevents orphaned work"). Dropping it materially conflicts with that named principle; spec rationale needs revision, not just a default change.
- **Design: drop creation-order weight** — `coordination.md:180` treats `creation` as an independent field; demoting to tiebreaker conflicts with current "each field is independent" framing.

## Underspecified
- **P1.3 relabel drift detection** — "hash labels-per-bead" not in `coordination.md` drift section; two implementers would pick different hashing scopes.
- **P1.4 unmatched counter recompute** — `commands.md:1458` says "surfaced once"; recompute-timing unspecified.
- **Triage `kerf triage --resolved` exit code** — semantics of "resolved" (zero warnings? zero cleanups?) undefined.
- **Sim integrity (rework metrics = 0)** — `simulator.md` L277-281 defines metrics, but `BeadSource` ↔ `br list --format json` schema mapping is not pinned anywhere; needs a fidelity-layer note.

## Spec-debt blockers
- **All Design/Scoring hypotheses except "cap rework"** — must not ship before `coordination.md` queue-weights section is extended; otherwise reviewers cannot check code against spec.
- **All Triage Workflow new commands** — block on `commands.md` additions; per CLAUDE.md spec-first.
- **bd→br spec sweep (`jig-implementation.md`, `architecture.md:237`, `verification.md:50`, `commands.md:640/662/1320`, `jig-system.md:62`, `coordination.md:190/257`)** — must land before any further code review touching jig-impl / verification, because reviewer will otherwise hit spec/code divergence on every read. Also: `jig-implementation.md` ↔ `internal/jig/builtin/implementation.md` duplication needs the proposed `go:embed` decision documented in `jig-system.md` first.
- **P0.2 corrupt-spec** — needs the warning-kind spec line before the feed/warning code change is reviewable.
