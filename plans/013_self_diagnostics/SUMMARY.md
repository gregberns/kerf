# Plan 013 — Self-Diagnostics from Claude Transcripts (Planning Summary)

**Date:** 2026-05-19 (revised after independent review)
**Status:** planning phase complete; ready for bead creation.

## Scope (revised 2026-05-19)

Two detectors surfaced as `kerf next` warnings, fed by a Go transcript parser + bead-ID indexer:
- **D1** abandoned dispatch -> `kerf next` warning (`abandoned_dispatch`)
- **D6** reviewer-absent commit -> `kerf next` warning (`reviewer_absent`)

**Deferred to Plan 013b** (after D1+D6 run against real corpora): D2 phase regression, D3 stalled conflict, D4 outlier duration, D5 silent retry. Their spec sentences still ship as a capture-only "Future detectors" section in `specs/diagnostics.md`.

Plan 014's T>0 adaptive scheduler is the natural downstream consumer; D1+D6 is sufficient to prove the feed exists.

## What changed in the 2026-05-19 revision

The independent review (`critiques/independent_review.md`) found the previous 6-detector / 12-bead / 6-wave draft over-built and ambiguous on surface. Applied changes:

- **Scope cut to D1 + D6** for v1. Both motivating incidents (abandoned dispatches + harmonik reviewer regression) still covered.
- **Surface is `kerf next` warnings**, not `kerf doctor`. `kerf doctor` is a fast snapshot surface; historical-corpus detectors don't belong there. Operators won't run `kerf doctor` every wave, but they call `kerf next` every cycle.
- **Calibration bead (B-CAL) added before spec lockdown.** Validates proposed thresholds against `plans/012_real_corpus/data/` extracts before B1 commits numbers.
- **Parser duplication resolved.** Go parser at `internal/kerftranscript/` is authoritative; Python parser at `plans/012_real_corpus/data/extract.py` is archived as analysis-only.
- **`kerf diagnose` content fully removed** from `_plan.md` (it was already superseded; lingering text removed).
- **Wave arithmetic fixed** in `beads.md` (now 4 waves / 7 beads -- prior draft mismatched "5 waves" header against a 6-wave body).
- **Transcript discovery rule made explicit, not hard-coded** -- B1 commits canonical path + `KERF_TRANSCRIPT_DIR` + "Claude Code on macOS v1" scoping. Removes the hard-coded `/Users/gb/.claude/projects/-Users-gb-github-harmonik/` from `source/detector_examples.md` as a normative path.

## Critique headlines (from independent review)

- **Surface sprawl risk** -- `kerf doctor` is millisecond-snapshot; new detectors are seconds-to-minutes historical. Don't mix. Use `kerf next` warning channel.
- **Parser duplication** -- Python and Go parsers will diverge. Pick one as authoritative (Go) and retire the other.
- **Threshold calibration missing** -- original draft committed numbers by feel; review showed the deferred D3 30-min threshold would fire on 11-35 h conflicts. Same risk in D1/D6 until measured.
- **MVP cut** -- 2 detectors first, learn, then 4 more.

## Beads (7, in 4 waves)

B-CAL calibration -> B1 spec -> B2 parser + B3 indexer + B4 fixtures (parallel) -> B-D6 first-ship + B-D1 (parallel) -> B-E2E integration. Critical path: 4 hops. See `beads.md`.

## User decisions needed

1. Confirm surface is **`kerf next` warning channel** for D1+D6 (not `kerf doctor`).
2. Approve calibration-driven thresholds (B-CAL runs before B1 commits numbers; no defaults pre-approved here).
3. Confirm Python parser archived as analysis-only; Go parser at `internal/kerftranscript/` becomes authoritative for any future Plan 012 re-extraction.
4. Approve B-D6 first-ship validation milestone (lands before B-D1 to validate the warning surface end-to-end on the cheaper detector).

## Process note

This is the second planning pass for Plan 013. The first pass (12 beads, 6 detectors) was reviewed by an independent fresh-context agent (`critiques/independent_review.md`) and scoped down. Three earlier critiques informed the first pass but did not catch the scope and surface problems the independent review surfaced.
