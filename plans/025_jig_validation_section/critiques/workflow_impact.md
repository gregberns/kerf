# Critique B — Impact on existing plans / beads workflow

Angle: what happens to existing in-flight kerf works (and existing beads downstream of them) once this change lands?

## Findings

1. **Existing works won't have the section.** Works currently in `change-spec` / `tasks` / `fix-spec` / `breakdown` status produced files before this requirement existed. `kerf square` should not retroactively fail on them. Plan's OQ1 (warn-only) already handles this, but the warning text should be informative ("filed before Validation requirement was added — file two test items now or document why not"), not just "missing section".

2. **Existing implementation jigs already in `dispatch` or `implement` status.** Pass 1 (Breakdown) for in-flight implementation works has already advanced. The Validation requirement on Pass 1 cannot be retroactively enforced. Recommend: spec the requirement as "must be added before the work's terminal status", letting in-flight works backfill the section without re-doing Pass 1.

3. **Bead dependency graph implications.** The harmonik override says test beads must be dependents of the implementation beads they validate, and downstream beads cannot close until test beads close. kerf does not currently coordinate bead-dependency graphs — it produces task lists in markdown. The requirement should be expressed as a structural one ("list the test bead IDs in the artifact, declare dependency in whichever tracker"), not as something kerf itself enforces. Recommend: spec language must be "the artifact records the test-item IDs and the agent files the dependency in the tracker"; kerf does not query the tracker.

4. **Plan 022 (scenario_harness) and plan 023 (property_contracts) overlap.** Both plans (status unknown — directory exists with only `_plan.md`) touch testing strategy. Cross-check: if 022 introduces a scenario harness that conflicts with "scenario-test bead" terminology used here, we get terminology drift. Recommend: scan plan 022's intent at landing time, align terminology.

5. **`implementation` jig Pass 4 (Verify) gets the Validation section in harmonik's override.** That feels like the right pass: Verify is where the work is supposed to be confirmed end-to-end. But the wording in harmonik's Pass 4 says "test beads must already exist from Pass 1" — meaning Pass 4 is verifying compliance, not creating items. Spec should be explicit that the **creation** point is Pass 1 (Breakdown) and Pass 4 is **verification of closure**.

6. **No effect on existing CLI surfaces.** No new commands, no new flags in v1. `kerf square` gains one check. `kerf show` could optionally surface test-bead IDs if the artifact contains them, but that's a follow-up, not part of this plan.

## Verdict

Backfill path needs to be specced explicitly so in-flight works don't get spurious failures. Coordination-graph language must be tool-agnostic (kerf does not own the dep graph). Otherwise low blast radius.
