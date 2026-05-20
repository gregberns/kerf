# Critique — Architecture & Spec Coherence

## Headline

Plan 013 is broadly coherent but its **architecture section is now obsolete**.
The detector registry it sketches (`Detector` interface, `Finding`, registry
file, "adding a detector = one new file + registry entry") was independently
built for `kerf doctor` and lives at `internal/doctor/doctor.go`. The same
`Detector` / `Finding` / `Registry` / `Report` shape is already in production,
spec'd in `specs/commands.md` §"kerf doctor", with five detectors registered
(`project-yaml`, `storage-drift`, `symlink-integrity`, `bead-filter-coverage`,
`archive-orphans`). Plan 013 should plug into that, not duplicate it.

## Specific architecture issues

1. **Surface conflict.** The plan proposes `kerf diagnose` (new) or extending
   `kerf triage` (existing). The freshness check correctly rerouted this to
   `kerf doctor` — but the body of the plan still describes a `kerf diagnose`
   surface (sample output, `--detector D1`, daemon `watch` mode). Body needs
   to be reconciled or the freshness note will be ignored by implementers.

2. **Transcript parser is a missing piece, not a "reuse."** Plan 012's parser
   is Python in `plans/012_real_corpus/data/extract.py`. The Go binary has
   no transcript parser. Plan 013's `internal/transcript/` package is **new
   code**, not "shared with simulator." This is the most significant scope
   underestimate in the plan: a fixture-tested JSONL parser that handles
   Claude transcript schema variations, sub-agent dispatch detection, phase
   labelling, and bead-ID extraction is at least a half-week of work on its
   own, before any detector lands.

3. **Detector vs. doctor-detector severity mismatch.** Doctor uses
   `green | yellow | red`; Plan 013 uses `info | warn | error`. Pick one.
   Doctor's vocabulary is normative in `commands.md`.

4. **Indexer correctness is load-bearing.** `source/detector_examples.md`
   reports ~75–80% false-positive rate on D1 from indexer false negatives
   (commits referenced parent bead IDs, sibling IDs, close-as-SUBSUMED
   commits, worktree refs). This means **D1 cannot ship before the
   commit-message indexer is implemented and tested**, and the indexer
   deserves its own bead, not a sub-bullet under D1. The plan currently
   buries this critical fact inside D1's "implementation note."

5. **D2 baseline window is unspecified.** "Phase appeared in ≥X% of beads
   over Y beads" — X and Y are free parameters with no default. Without
   them the detector is unrunnable. Source examples imply Y≈30 beads,
   X≈50%, but the spec must commit.

6. **D2 vs D6 redundancy is acknowledged but not resolved.** Source says
   "they fire on the same population." Spec must say which is canonical
   and how the other suppresses; otherwise users see duplicate findings.

7. **D4 brittleness rule is in plain English, not normative.** Plan says
   "default: floor + effect-size compound predicate" but doesn't pin the
   numbers (floor=600s? 2× median? per-area threshold?). The detector spec
   has to commit before code lands or the test will be the spec.

8. **Transcript discovery is unspecified.** Where does kerf find Claude
   transcripts? `~/.claude/projects/-Users-gb-github-<project>/*.jsonl`?
   `~/.config/claude/`? Cross-OS? The plan assumes the path is known.

## Spec touches — what's actually needed

The plan lists `specs/diagnostics.md` (new) and `specs/commands.md` (update).
Actual touches needed:

- **`specs/commands.md`** — `kerf doctor` gets new detector IDs in the table;
  no new command. Output examples already cover the shape.
- **`specs/diagnostics.md`** (new) — what each detector looks for, the
  thresholds, severity, and how findings link to evidence (session id,
  bead id, timestamp, transcript path). This is normative.
- **`specs/architecture.md`** — small addition under transcript discovery
  (where kerf reads JSONL from, how it handles missing/partial corpora).
- **No `specs/transcript.md` needed** — the parser is internal; spec only
  the discovery rule, not the JSONL schema.

## Coherence with other plans

- **Plan 014 (process-management reframe).** Explicitly names Plan 013 as
  "the feed for the T>0 adaptive layer." This is correct but adds a soft
  constraint: detector findings should be **machine-consumable as time
  series**, not just human-readable. JSON output (already in doctor) is
  fine; the schema for D4-style "observed duration" findings should carry
  the raw observation alongside the human summary, so 014's adaptive
  weight loop can feed on it without re-parsing transcripts.
- **Plan 012 (real corpus).** The fixture beads under
  `source/detector_examples.md` should become committed Go test fixtures
  under `internal/diagnose/testdata/` so future detector regressions
  catch the documented incidents.

## Recommendation

1. Replace the plan's "Architecture" section with "Plug into
   `internal/doctor/`: register diagnostic detectors against the existing
   registry. Detector IDs use the `dx-name` convention (`d1-abandoned`,
   `d2-phase-regression`, …)."
2. Promote "transcript parser" and "bead-ID indexer" to top-level beads
   with their own spec sentences. Today they're hidden assumptions.
3. Move D7–D13 (notes for future detectors) out of this plan into
   `specs/future.md` so they aren't accidentally pulled in.
