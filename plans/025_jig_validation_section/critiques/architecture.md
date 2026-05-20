# Critique A — Architecture / spec coherence

Angle: does this change fit kerf's spec architecture cleanly, or does it introduce new concepts that contradict existing invariants?

## Findings

1. **Invariant #6 conflict if we go too far.** `specs/_index.md` lists "Jigs are guidance, not gates. Passes can be skipped." A Validation section that hard-blocks `kerf square` would violate this. The plan's OQ1 already names the conflict and recommends warn-not-fail. Accept: ship warn-only.

2. **No new YAML field needed in v1, but a future-proofing seam is missing.** The plan correctly avoids adding a `validation:` field to the jig YAML frontmatter — markdown-body instructions are how kerf jigs encode requirements today. However, `specs/verification.md` will need a way to detect the section. Two options: (a) literal heading match `## Validation / Acceptance Tests` or `### Validation / Acceptance Tests`; (b) a structured marker like an HTML comment `<!-- kerf-validation -->`. Option (a) is simpler, matches the harmonik override pattern verbatim, and avoids ad-hoc structured markers. Recommend (a).

3. **"Normative planning artifact" is a new term in the glossary.** The plan introduces this term in `specs/jig-system.md` to scope which passes need a Validation section. Coherence concern: the glossary in `specs/_index.md` should get an entry, or the term should be inlined into the spec without coining a new label. Recommend: add to glossary in `_index.md` for discoverability.

4. **Cross-jig duplication.** The four built-in jig markdown bodies will each carry near-identical Validation prose. Plan 020 already had to collapse a similar duplication ("What done looks like" vs "Review Criteria"). Recommend: each per-jig body says "See `specs/jig-system.md` § Validation / Acceptance Tests for the canonical shape" and then names only the pass-specific particulars (artifact file path, codename style). Avoids drift across the four jigs.

5. **`retrofit` and `spike` exclusion is defensible but should be auditable.** The plan excludes both; that's the right call. Add a one-line justification in `specs/jig-system.md` so the omission is intentional, not an oversight.

6. **Test-item terminology.** Harmonik says "test beads" — kerf's spec must abstract this. Recommend: in `specs/jig-system.md` use "test items" (the jig-system spec is tool-agnostic); in the per-jig markdown bodies use "bead" as the example tracker label since `br` is what every shipping kerf user runs today. The lesson "Kerf serves users" supports examples being concrete.

## Verdict

Architecturally clean if (a) warn-not-fail, (b) literal heading match for square, (c) shared canonical definition in `jig-system.md` referenced by each per-jig body, (d) glossary entry added. No invariants are broken.
