# Spec Conformance Critique — Plan 010

Checks each new contract element against the exact spec location it must amend, and whether the change is additive for existing JSON/CLI consumers.

## 1. New Item fields: `owned_by`, `owned_since`

- **Spec location to amend:** `specs/commands.md` §`kerf next` → "Output (`--format=json`)" item shape block (lines 1509–1519).
- **Additive?** Yes. Existing six fields (`kind`, `score`, `title`, `action`, `reason`, `work_codename`, `bead_id`) are preserved. Spec already says "Future kinds … are additive. Consumers should treat unknown kinds as informational rather than erroring" (line 1521) — but that sentence is about unknown *kinds*, not unknown *fields*. The plan should add an explicit "Consumers MUST ignore unknown fields" sentence to the JSON section to discharge the "additive" claim it makes in §1.
- **Naming nit:** plan §1 uses `owned_since` in the field table but the design-options table at line 25 uses `since`. Spec must use one name; plan should self-correct to `owned_since` before spec update.
- **Inconsistency:** plan says `warning` items have both fields "always null", but the JSON shape block currently emits every field on every item. Spec amendment must explicitly state warning items render `owned_by: null, owned_since: null` (not omitted) to preserve schema uniformity.

## 2. New `--session <id>` flag

- **Spec location:** `specs/commands.md` §`kerf next` → Flags table (lines 1436–1443). Add a 6th row.
- **Help-text contract risk (the big one):** The §"Help text" block (lines 1526–1536) is a numbered six-element ordered contract: returns / kinds / loop / filter flags / machine output / scoring. Adding `--session` does NOT require a 7th bullet — it fits inside bullet 4 ("filter flags with concrete examples"). But the plan's Implementation Notes (line 143) says only "gains a line about the ownership fields and the exit-code table." That is ambiguous: a new exit-code table is not covered by any of the six bullets. The spec amendment must either (a) extend bullet 4 to cover `--session` and add a 7th bullet for exit codes, or (b) fold exit codes into bullet 3 (the loop) since codes drive loop branching. Option (b) is cleaner and preserves the six-bullet promise. Plan should pick one explicitly.
- **Additive?** Optional flag, no default behavior change → yes.

## 3. New `stale_session` warning kind

- **Spec location:** `specs/commands.md` §`kerf next` → Behavior step 3, "Warning detectors" sub-bullet (lines 1457–1459). Today lists two detectors (unmatched beads, filter literal zero-match). Add a third.
- **Cross-ref:** `specs/sessions.md` §"Stale Session Detection" (lines 62–86) currently scopes stale warnings to "every command invocation that reads `spec.yaml` for the affected work." `kerf next` reads every work's `spec.yaml` during feed assembly, so this is consistent — but the sessions.md spec should gain one sentence noting `kerf next` is one such command, surfacing staleness as a feed warning rather than a stderr line. Plan §"Specs Affected" calls for this cross-ref; good.
- **Additive?** Yes — new warning items only appear in conditions not previously surfaced.

## 4. Exit codes 0/1/2/3

- **Spec location:** No exit-code table currently exists in `specs/commands.md` §`kerf next`. Today the only error-row contract is the "Errors" table (lines 1540–1544) covering 2xx-ish messages. The plan must *create* a new "Exit codes" subsection between "Help text" and "Errors."
- **`specs/cli.md` check:** Plan §"Specs Affected" hedges ("If exit codes are documented globally"). This is a spec gap the plan must resolve before implementation — verify cli.md and either add a global table or document per-command.
- **Backward-compat concern:** Today `kerf next` returns 0 on any output. Scripts relying on `exit 0 == success` for "feed produced output" will now get 1 (empty) or 3 (warnings-only) where they previously got 0. **This is not strictly additive** — it is a behavior change for empty/warning-only cases. Plan should call this out as a breaking change in the exit-code contract and consider a `--legacy-exit` opt-out, OR document that pre-v1 callers were relying on undocumented behavior.

## 5. Cross-references to sessions.md

Plan §"Specs Affected" lists sessions.md as "Cross-reference only." Verify the sessions.md edit adds a forward link to `commands.md#kerf-next` under "Behavior on Stale Detection" so a reader of sessions.md learns kerf next surfaces it.

## Summary of contract touches

| Element | Spec section | Additive? |
|---|---|---|
| `owned_by` / `owned_since` fields | commands.md §kerf next JSON shape | Yes (with explicit unknown-field clause) |
| `--session` flag | commands.md §kerf next Flags table | Yes |
| `stale_session` warning kind | commands.md §kerf next warning detectors list; sessions.md cross-ref | Yes |
| Exit codes 0/1/2/3 | new subsection in commands.md §kerf next; possibly cli.md | **No — behavior change for empty/warning-only cases** |
| Help text | commands.md §kerf next Help text six-bullet contract | Requires either extending bullet 3 or adding a 7th bullet |

## Biggest risk

Exit-code semantics are a true contract break, and the six-bullet help-text contract is silently stressed by both `--session` and the new exit-code table. Plan should pick one of: (a) fold exit codes into bullet 3 (loop), or (b) extend the contract to seven bullets and update the "fixed order" promise.
