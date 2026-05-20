# Agent-UX Critique — Plan 009 (Triage Workflow)

Lens: fresh autonomous agent enters a project, runs `kerf triage`, tries to converge.

## Cold start

`kerf triage` works only if the project is already `kerf init`'d. Plan's canonical
workflow includes `kerf init` but `triage` itself should detect uninitialized
state and emit a single-line directive (`run kerf init first`) rather than a
generic error. Otherwise the agent will treat the non-zero exit as "drift exists"
and loop on `kerf new`. **Fix:** spec must define behavior when `.kerf/` is
absent — exit 1 with `kind: not_initialized`, not exit 2.

## Mid-run comprehension

Section headers (`Untriaged`, `Multi-matched`, `External changes`) are jargon to
a fresh agent. The spec should require each section's first line to be a
one-sentence definition AND a suggested next action (`run kerf new <codename>
--bead-filter 'label=X'` for untriaged buckets). Agents copy-paste literally;
template the suggestion with the actual bead label/id so the agent has a working
command, not a recipe.

## Loop convergence — top risk

`kerf triage --resolved` exits 0 only when untriaged == 0 AND multi-matched == 0
AND unacked external changes == 0. Multi-matched has no guaranteed-progress
resolution: pinning to one work leaves the bead matching the other's filter, so
it stays multi-matched. Open Question #4 ("pin is additive") **guarantees an
infinite loop** for any bead whose pin lives inside a matching filter. Spec
must either (a) make pin override filter, or (b) let `triage --ack` mark a
specific multi-match as "intended ambiguity". Without one of these, an agent's
`until kerf triage --resolved` halts only by accident.

## Output shape / JSON contract

`--format=json` says "same shape as `kerf next --format=json` (kind-tagged
items)" but plan 008's critique flagged that `kerf next`'s JSON shape isn't
locked. Inheriting an unlocked contract propagates the problem. Spec should
enumerate kinds: `untriaged_bead`, `multi_matched_bead`, `external_close`,
`external_reopen`, `external_delete`, `external_new`, `work_health`. Stable
field set per kind. Version field at top level.

## Exit codes

0/1/2 match `kerf next` per plan 010, good. But 1 = "store unreadable"
collides with conventional "no items". Recommend 3 for unreadable; reserve 1
for "clean but empty / not initialized".

## Missing affordances

1. **`kerf triage --kind=untriaged`** — agent wants one bucket at a time to
   avoid re-parsing the whole report each loop iteration.
2. **Per-bead next-action hint in output** — see mid-run comprehension.
3. **`--since=<snapshot-id>`** — explicit baseline for scripted runs.
4. **Stuck-loop detector** — N consecutive `triage` runs with identical drift
   set should surface a `triage_stuck` warning. Parallels plan 008's
   stuck-loop gap.

## Rename

`kerf attach` collides with bead-attachment-to-work mental model already in
use across specs (filters *attach* beads to works). The pin semantics differ.
Suggest **`kerf pin <codename> <bead-id>`** — matches the `pinned_beads:`
schema field, distinguishes from filter-driven attachment.
