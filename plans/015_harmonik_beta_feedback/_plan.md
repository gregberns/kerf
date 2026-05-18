# Plan 015 — Triage Harmonik Beta Feedback

> **Status: TRIAGE.** Ingests a snapshot of harmonik's live kerf-beta feedback log, normalizes each finding, and routes items to absorbing plans, spec edits, or quick fixes. Does not itself propose code or spec changes — the absorbing plans do.

## Intent

Harmonik is the user's dogfooding project for kerf. Between 2026-05-15 and 2026-05-18 the harmonik tester (Claude Opus 4.7) bootstrapping new-kerf on the harmonik repo logged ~50 distinct friction points against the kerf CLI surface — `kerf init`, `kerf next`, `kerf triage`, `kerf work edit`, the spec-jig pass loop, and the storage-layout split between the repo's `.kerf/` and the global bench `~/.kerf/projects/<id>/`. This plan reads the snapshot, groups the items by theme, normalizes each one (title, severity, problem, desired behavior, disposition), and routes them. The output is a working triage doc that the absorbing plans will reference when they get written.

## Process used

- Source: a single snapshot of the harmonik tester's running log, captured 2026-05-18 and stored at `plans/015_harmonik_beta_feedback/source/kerf-beta-feedback_2026-05-18.md` (464 lines). The live log at `/Users/gb/github/harmonik/docs/kerf-beta-feedback.md` may keep growing; this plan ignores anything beyond the snapshot.
- Read top-to-bottom; grouped the entries chronologically into themed buckets (~10 themes); deduped where one observation appeared in multiple sections.
- For each item: short imperative title, severity from the source, one-sentence problem statement, one-sentence "what good looks like," and a disposition (existing-plan, new-plan, spec-only, quick-fix, dropped).
- Cross-cutting "queue / momentum / prioritization" items route to Plan 014 (process-management reframe), which is being drafted in parallel.
- Items that may already be fixed by post-2026-05-15 commits are flagged "appears possibly fixed — verify before action" rather than verified by code-reading.

## Summary table

| Theme | Items | Severity mix | Likely disposition |
|---|---|---|---|
| Init / first-run UX | 10 | 1 BLOCKER, 5 MAJOR, 4 MINOR | new-plan 016 (init UX overhaul) |
| Storage layout (`.kerf/` ↔ bench) | 4 | 3 MAJOR, 1 MINOR | new-plan 017 (storage reconciliation + `kerf doctor`) |
| `kerf next` ranking + entry friction | 6 | 4 MAJOR, 2 MINOR | Plan 014 (process-management reframe) |
| `kerf triage` output + suggestions | 7 | 5 MAJOR, 2 MINOR | new-plan 018 (triage rework) + Plan 014 |
| Work bead-filter bootstrap | 5 | 4 MAJOR, 1 MINOR | new-plan 019 (filter bootstrap + `kerf show`) |
| Filter-syntax / convention drift | 3 | 2 MAJOR, 1 MINOR | new-plan 019 + spec-only |
| Spec-jig pass loop (review-gate / file conventions) | 8 | 4 MAJOR, 3 MINOR, 1 NIT | new-plan 020 (jig review-gate + template fixes) |
| Agent setup instructions / docs drift | 4 | 3 MAJOR, 1 MINOR | spec-only + new-plan 016 |
| Command-UX gaps (flags, JSON, `--quiet`) | 7 | 4 MAJOR, 2 MINOR, 1 NIT | quick-fix bundle + new-plan 018 |
| Harmonik-side bugs (out-of-scope for kerf) | 6 | 4 MAJOR, 1 MINOR, 1 NIT | dropped (not kerf — surfaced to harmonik) |
| **Total** | **~60** | | |

(Counts approximate; some items span multiple themes. See `triage.md` for the canonical normalized list.)

## Cross-references

- **Plan 014 (process-management reframe)** — absorbs `kerf next` placement of warnings vs. payload, the "ranked feed" framing, momentum signal, and the queue/prioritization items. Several `triage` ack/baseline items also belong here.
- **Plan 016 (proposed — init UX)** — absorbs the interactive-prompt blocker, the duplicate AGENT SETUP INSTRUCTIONS blocks, the stale `kerf:*`-label detector, and the `--yes` / `--no` flag gap.
- **Plan 017 (proposed — storage reconciliation)** — absorbs `.kerf/` vs. bench drift, orphan dirs, and the `kerf doctor` / `kerf status --project` health check.
- **Plan 018 (proposed — triage rework)** — absorbs the aggressive `kerf new <axis-label>` suggestions, the `--ack` re-printing the full report, archive-awareness, and `--top` / `--group-by` flags.
- **Plan 019 (proposed — filter bootstrap + `kerf show` slot)** — absorbs the per-work `bead_filter` bootstrap command, the `clean` vs. `empty` vs. `unwired` status confusion (`hk-43ate`), and `kerf show`'s missing `bead_filter` line.
- **Plan 020 (proposed — jig review-gate + pass-loop fixes)** — absorbs the reviewer-sub-agent assumption, pass-N file-naming conventions, output templates, and `04-design/` pre-creation.
- **Spec-only** — agent-setup instruction text, gitignore guidance, filter-syntax convention documentation.
- **Quick-fix bundle** — `clean` → `evaluated` wording, `kerf triage --kind=multi_matched` zero-handling, "Now matches: N beads" wording, `--quiet` flag on `kerf status` transitions.

## Open questions for the user

1. **Should new plans 016–020 collapse into fewer plans?** Five new plans is on the high end. A reasonable alternative: one combined plan ("Plan 016 — bootstrap UX cleanup") for 016 + 017 + 019, leaving 018 (triage) and 020 (jig) standalone. Default: split as proposed — each has distinct surface area.
2. **Plan 014 absorption fidelity.** Several `kerf next` items (entry friction, "clean" vs. "empty" rank labels) sit at the boundary between 014's process-management framing and 019's filter-bootstrap framing. If 014 doesn't take them, 019 should. Need to know 014's scope before final routing.
3. **Harmonik-side bugs.** Items in the "Phase-2 dogfood #2" section (br close 10s timeout, daemon orphan-sweep gap, daemon claim-path priority bypass) are harmonik bugs, not kerf bugs. Dropped here, but worth confirming they have tracking beads in harmonik's `.beads/` (the source notes `hk-rp48p`, `hk-jvzc2`, `hk-44w19`, `hk-sc3o4`).
4. **"Appears possibly fixed" markers.** ~3 items may have been addressed by intermediate kerf releases (the 2026-05-18 entry notes that repetitive triage suggestions weren't reproduced). Verification deferred to the absorbing plans.
