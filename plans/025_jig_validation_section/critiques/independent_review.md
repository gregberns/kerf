# Independent Review — Plan 025 (Jig Validation Section)

**Reviewer brief:** fresh-eyes review against the user worry that kerf is growing parallel features without enough UX thought. Author has not read prior critiques except to skim for terminology consistency; verdict drawn from independent reading of the plan, beads, harmonik feedback, current upstream jigs, and existing kerf specs (`verification.md`, `jig-plan.md`, `jig-implementation.md`).

## Verdict

**Proceed-with-changes.** The underlying *requirement* (force integration-surface tests at planning time) is load-bearing — the three harmonik incidents are real and the existing "Acceptance criteria + Verification" subsections in the plan jig demonstrably did not catch them. But the *shape* the plan currently proposes (14 beads, new glossary terms, new square check, four per-jig deltas, a brand-new "Validation / Acceptance Tests" heading parallel to the existing "Acceptance criteria" / "Verification" subsections) is over-built for what it accomplishes, and it does duplicate surface that already exists. Specifically:

1. The plan jig already has, inside Pass 5 § Change Spec, two sibling subsections called **"Acceptance criteria"** and **"Verification"** (specs/jig-plan.md L268-269 / internal/jig/builtin/plan.md `change-spec` pass). Pass 7 already mandates "test tasks" with "every acceptance criterion from every component spec appears in at least one task". The implementation jig Pass 4 (Verify) already mandates "run the full test suite" and "walk every acceptance criterion." Adding a third heading named "Validation / Acceptance Tests" — which is just two more checklist items asserting that two of those test tasks must be scenario-flavored and exploratory-flavored — is a parallel mechanism, not a missing one.

2. The motivating incidents are real but the diagnosis pinned in the plan is incomplete. They are not "no test was specified"; they are "no test exercised the integration surface against a runnable substrate." `hk-37zy8` had unit tests *and* reviewer approval, which means the existing acceptance-criteria machinery wasn't the gap — the *kind* of test was. A new heading does not by itself force agents to write a *twin-substrate* test rather than another unit test. See finding #4 below.

3. Critique reconciliation has already inflated this from a one-paragraph requirement into 14 beads with new glossary entries, a new spec section, a new square check, and four per-jig deltas. That smell is exactly what the user is worried about.

## Top findings

### 1. Feature duplication with existing per-pass subsections

The plan jig Pass 5 already requires every component spec to contain:
- **Acceptance criteria** — "concrete, testable criteria. Each must be verifiable by running a test, executing a command, or observing specific behavior."
- **Verification** — "how to confirm the component works. Commands to run, tests to execute, manual checks to perform."

Pass 7 already requires that every acceptance criterion lands in at least one task, and that test tasks are explicitly enumerated. The Validation / Acceptance Tests section is *another* place to repeat the same content in a slightly different shape — IDs of two tracker items with naming convention `scenario:` / `explore:`.

A planning agent reading the pass top-to-bottom will encounter, by the end of Pass 5: "Acceptance criteria" (1 list), "Verification" (1 list), "Validation / Acceptance Tests" (2 mandatory items), and "Review Criteria" (~7 reviewer checks). The user's worry about UX coherence is well-founded here. Ergonomics critique #5 already flagged this and proposed merging Validation INTO "What done looks like" — but only as two checkbox items, not as a heading. I think that proposal goes about 70% of the way; the plan should adopt it fully and **drop the standalone heading entirely**.

### 2. The motivating incidents do not all trace cleanly through the mechanism

Walking `hk-37zy8` (handler-pause goroutine, unit-tested + reviewer-LGTM, never wired into composition root) through the proposed mechanism:

> Pass 1 (Breakdown) requires a `scenario:` bead and an `explore:` bead.
> An agent files: `scenario: handler-pause — handler_fatal bead transitions to paused`.
> The bead exists. Does it specify a twin-substrate test? Not necessarily — the title says "transitions to paused", which a unit test against a fake also satisfies.
> The bead gets closed when the test passes.
> The test was: a unit test against a fake. The composition root is still un-wired.

So: yes, an agent forced to file the bead would write *something*. Whether that something exercises the integration surface depends on whether the agent understands the term "scenario-test bead" — and the harmonik override defines it as "twin-substrate or real-claude," but kerf is tracker-agnostic and substrate-agnostic. In the upstream-promotion version (per OQ2 default), the definition is even more abstract. The risk that this becomes boilerplate ceremony — two `br create` calls with no real integration-test discipline — is high.

This is not a fatal objection; even a 30% chance of catching such bugs is worth two checkbox items. But the *plan* should be more honest about it: the mechanism's power is "name and shame the missing integration test at planning time," not "structurally force a working integration test." That distinction should land in `specs/jig-system.md` so future readers don't assume the section is a guarantee.

### 3. Could a cheaper intervention do the same work?

Yes. Two alternatives the plan does not weigh:

**(a) A `kerf doctor` detector**, not a `kerf square` check. The plan adds a square-time warn rule. Square is the finalization gate; warnings there are likely to be ignored ("file came late, fix in next PR"). `kerf doctor` is the per-project health check that already surfaces issues like empty bead filters and storage drift, and the existing detector pattern is more discoverable than a new heading-match in square. A `validation-section-coverage` detector on `kerf doctor` that surfaces every active plan/spec/bug/impl work missing scenario+explore beads in its tracker would deliver the same warn-only signal, integrate with the existing `kerf next` warning footer, and require **one** spec edit and **one** code change instead of seven spec edits and five code changes.

**(b) A default bead-label policy** on the implementation jig. The bead-filter-coverage detector already exists; extending it to surface "this work has tasks but no bead labeled `scenario-test` and no bead labeled `exploratory-test`" is a much smaller surface change. Plan/spec/bug jigs would then be unaffected — the integration-test discipline lives where the integration happens (the implementation jig), not in the upstream plan/spec writing.

The plan should at minimum write down why (a) and (b) are rejected. Right now the plan presents one design without weighing alternatives.

### 4. Square check is the wrong host

Even if the standalone heading survives, putting the detection in `kerf square` is awkward:

- Square's invariant (#6) is "jigs are guidance, not gates" — so the check must be warn-only.
- Square is documented as **structural**, not content-quality (specs/verification.md L7). Detecting a *heading* inside a markdown file is content-quality detection, and `verification.md` explicitly says "Whether a spec is well-written, complete in substance, or technically sound is not assessed."
- A heading-match check that warns-not-fails is exactly the kind of soft surface that gets ignored in practice.

`kerf doctor` is the correct host for warn-only, content-quality findings (see L1571-1604 of commands.md). The plan should move the check there. Bead 7 + bead 12 collapse into one bead.

### 5. Scope creep / right-sizing

14 beads is too many. Walking through the list:

- Beads 1, 3, 4, 5, 6 (5 beads) — five spec edits to add the same conceptual delta in five files. If the per-jig specs delegate to `jig-system.md` (as critique architecture #4 recommends), beads 3-6 are near-trivial one-paragraph edits; collapse them into a single bead "spec: per-jig Validation delegation across plan/spec/bug/implementation."
- Bead 2 — glossary entries. Fold into bead 1.
- Bead 7 — square check (should be doctor; see finding #4).
- Beads 8-11 — four code edits to four jig markdown bodies. Same delegation logic: these are near-trivial after bead 1 lands. Could be one bead "code: jig body Validation blocks." Probably worth two for code-review reasons (one for plan/spec/bug, one for implementation because the impl jig has two passes affected).
- Bead 12 — code for square check (collapses with bead 7).
- Beads 13-14 — self-dogfood tests. Keep as-is; this is the requirement eating its own tail.

Reasonable right-sized decomposition: **5-6 beads**, not 14.

- B1: spec — canonical Validation requirement in `jig-system.md` + glossary in `_index.md` + per-jig delegations + retrofit/spike exclusion rationale (one PR, four files).
- B2: spec — `kerf doctor` (not square) gets a `validation-section-coverage` detector entry in `commands.md`. Decide warn level vs. existing detectors.
- B3: code — built-in jig markdown bodies (`internal/jig/builtin/{plan,spec,bug,implementation}.md`) get the Validation block per the spec.
- B4: code — `kerf doctor` detector implementation.
- B5: scenario-test (self-dogfood).
- B6: explore-test (self-dogfood).

### 6. UX coherence at the agent's edit moment

When the planning agent is sitting in Pass 5 of the plan jig and reaches "Validation / Acceptance Tests" three lines below "Acceptance criteria," they have to context-switch from "design the requirements" to "file two tracker tickets" mid-write. The ergonomics critique #3 already noticed this and proposes positioning the block adjacent to "Save to disk." That's a partial fix.

A better fix: the Validation surface should not be a *new section in the jig body* at all. It should be **two extra entries in the existing "What done looks like" checklist**, exactly as ergonomics critique #5 says — and then the canonical text about why and what kinds of tests to file lives ONCE in `jig-system.md`, referenced by `_index.md` glossary. This is consistent with how kerf treats other cross-jig requirements (e.g., session tracking). No new heading. Three lines added to four files instead of a new ~20-line block in each.

This change alone resolves about 80% of the duplication / sprawl concern.

## Concrete recommended next action

Restructure the plan along these axes before filing any beads:

1. **Drop the standalone "Validation / Acceptance Tests" heading.** Make it two checklist items inside each affected pass's "What done looks like." The prose definition lives once in `specs/jig-system.md` § Validation requirement (one paragraph, ~6 lines), referenced from the glossary.
2. **Move the warn-rule from `kerf square` to `kerf doctor`** as a detector. Justify: doctor is the canonical host for warn-only content-quality findings (commands.md L1571-1604); square is explicitly structural (verification.md L7).
3. **Collapse the bead graph from 14 to 5-6** as enumerated in finding #5 above.
4. **Add an honest "What this mechanism does NOT do" subsection** to the canonical spec text. Specifically: it does not structurally guarantee the test is integration-surface vs. unit; it relies on the agent understanding the test-kind distinction. List the three motivating incidents as examples of what kinds of tests *would* have caught them, so the agent has concrete patterns to copy.
5. **Briefly weigh alternatives (a) doctor detector instead of square check, (b) implementation-jig-only label policy** in the plan, and write down why the chosen shape is preferred. The user's worry is precisely that alternatives are not being weighed.

If the orchestrator wants a smaller experiment: ship change #1 + #2 alone (one spec PR + one code PR, ~2-3 beads total), dogfood for a session, and only add the doctor detector + spec extensions if the lightweight version proves insufficient. That is the "measure twice, cut once" path.

## What is solid

- The motivating incidents are real and the root-cause analysis is accurate at the level of "unit-tested + reviewer-LGTM + no integration test." This is a genuine gap.
- The decision to keep `kerf square` (or doctor) warn-only is correct and consistent with invariant #6.
- The decision to exclude `retrofit` and `spike` is well-reasoned.
- Tracker-agnostic phrasing (OQ2 default) is the right call given the "kerf serves users" memory.
- The self-dogfood beads (13 and 14) are a good touch — the plan eating its own tail.
- Critique reconciliation captured most of the duplication and ergonomics concerns; my objection is that it did not push hard enough to consolidate them.
