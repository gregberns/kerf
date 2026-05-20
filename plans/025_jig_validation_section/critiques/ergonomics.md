# Critique C — Ergonomics for the planning agent filling out the Validation section

Angle: when a planning agent is at, say, Pass 5 of the plan jig and has to fill in the Validation section, can it actually do so without thrashing?

## Findings

1. **"Scenario test" vs "exploratory test" definitions need to live in the jig body itself, not be assumed knowledge.** Without inline definitions, a fresh agent will guess. The harmonik override does include the definitions in each pass — kerf's upstream should too, possibly as a shared block referenced from `jig-system.md`. Recommend: the per-jig body says "see jig-system § Validation" and the jig-system spec carries a ~6-line definitional block. The agent loads the jig markdown anyway, so if both blocks live there, it's one read.

2. **The agent needs the project's tracker command, not a generic instruction.** "File a scenario-test bead" is too abstract; "run `br create '...' --type task --label scenario-test`" is concrete but tool-locked. The compromise: the per-jig body shows a `br` example with a "If your project uses a different tracker, substitute the equivalent" note. This matches the "kerf serves users" memory.

3. **At what point in Pass 5 does the agent file these?** Currently the harmonik override puts the Validation block AFTER "What done looks like" but BEFORE "Review Criteria". The agent reading top-down hits the bead-creation instructions after the artifact is already written. Recommend: position the Validation block right next to "Save to disk" — the agent files the test items at the same time it saves the artifact, then the artifact's "test bead IDs" section can be populated in the same edit.

4. **Title conventions friction.** `scenario: <codename> — <brief>` works for harmonik because harmonik works have codenames. kerf works do too, so this is fine. But for the `bug` jig, the codename naming pattern is `regression-<noun>` per the harmonik override; we should check what kerf's bug jig codename pattern actually is. (Reviewed `internal/jig/builtin/bug.md` — kerf bug jig uses generic codenames, so `scenario: <codename> — regression: <brief>` works.)

5. **Cognitive load: a plan jig now has 4 review-style blocks per pass (What done, Validation, Review Criteria, Save instructions).** This compounds the "duplicate blocks" issue plan 020 was trying to fix. Recommend: merge "Validation / Acceptance Tests" content INTO the "What done looks like" checklist as two additional items ("Test-item IDs listed for scenario and exploratory" + "Bead-tracker dependency filed"), and keep the prose explanation only in `jig-system.md`. This single-source-of-truths the requirement and avoids per-pass bloat.

6. **Empty-checkbox fatigue.** If the agent files two test items at Pass 5 of plan, then again at Pass 7, then again at Pass 1 of implementation, they may be filing the SAME logical items three times. Recommend: spec language is "file or reference" — Pass 7 of plan and Pass 1 of implementation may reference the same items filed at Pass 5, as long as the IDs are recorded.

## Verdict

The biggest ergonomic risk is duplication-across-passes. Fix it by allowing reuse of test items across passes (cite IDs, don't re-file) and by collapsing the Validation block into the existing "What done looks like" checklist plus a single canonical definition in `jig-system.md`. Otherwise the change is straightforward for agents to comply with.
