# Cross-cutting exploratory: full agent bootstrap flow

**Date:** 2026-05-18
**Tester:** Claude (Opus 4.7, fresh-agent dogfood mode)
**Binaries:** `kerf` at `/Users/gb/go/bin/kerf`, `bd` at `/Users/gb/.local/bin/bd`, `br` at `/Users/gb/.local/bin/br` (v0.1.45)
**Sandbox:** `/tmp/kerf-dogfood-test/` (fresh git repo + `bd init` + 13 seeded mixed-label beads)

## Narrative

I started in `/tmp/kerf-dogfood-test/`, a freshly `git init`'d repo with `bd init` already run and 13 issues seeded across five codenames (`auth`, `billing`, `search`, `ui`, `queue`) and a label-free infra chore. The intent was to walk the steps a brand-new agent would walk: `kerf next` cold, then `kerf init`, then follow the printed AGENT INSTRUCTIONS block all the way to a review prompt.

**Pre-init.** `kerf triage` did the right thing — it printed "project not initialized. Run 'kerf init' first." That is the gold-standard error: it names the missing precondition and the exact remediation command. `kerf next` did NOT do this; it tried to read the bead store, the `br` shellout failed with a raw JSON-decode error, and `kerf` surfaced the stack trace verbatim and exited 1 (with usage spam underneath). A fresh agent reading that output would have no idea that running `kerf init` is the fix. The cold-start error path is uneven across commands.

**`kerf init`.** This step went well. The instructions block is comprehensive: it lists the jigs in SDLC order, prints the full pass list for each jig, names the kerf commands, and explains the `.gitignore` rule. The `State changes` footer ("project.yaml created at /Users/gb/.kerf/projects/...") is accurate — it does NOT lie about where things went. The bench-vs-repo split is named explicitly. Good.

**Following the `.gitignore` instructions.** First snag. The block tells you to add `.kerf/` and `!.kerf/project-identifier` to `.gitignore` and then commit `.kerf/project-identifier`. Done literally, `git add .kerf/project-identifier` is refused: git's gitignore semantics require the parent directory not be excluded for negation to work. The agent has to know to use `git add -f` or know that the correct pattern is `.kerf/*` + `!.kerf/project-identifier`. The instructions don't say. A fresh agent will either fail the commit and panic, or commit `project-identifier` with `-f` and wonder if they did something wrong. The actual `.gitignore` pattern needs revisiting.

**`kerf list` / `kerf new auth`.** List was clean (empty, with a "get started" hint). `kerf new auth` worked and printed the entire plan-jig pass list. BUT: it picked `plan` as the default jig silently — there's no line in the output that says "(jig: plan, set as default_jig)". You only see `Jig: plan (v1)` mid-block. And critically: even though `auth` matches `codename:auth` on 3 open beads, `kerf new` did NOT pre-populate `bead_filter`. `kerf show auth` confirms: `bead_filter: (none)`. The test brief explicitly checks "did the `bead_filter:` slot land?" — no, it did not. The user has to discover `kerf work edit --bead-filter-add` separately. (More on that flag below.)

**`kerf next` after `kerf new`.** Still fails with the `br` JSON error. So at this point in the flow, an agent has a work but cannot see a ranked feed of what to do next on it. This is the central capability the brief asks about ("step 6: does it suggest something?") and it does not work at all in a `bd`-managed repo. Same blocker hits `kerf bootstrap-filters` (step 7), `kerf doctor` (step 12), and `kerf triage` (step 13). Four out of thirteen scripted steps are gated on `br` working against the seeded store, and it doesn't. The exit codes are inconsistent: `kerf next` returned 0 even after printing an Error: line, while `kerf bootstrap-filters` returned 1. An agent piping these into a script can't trust exit status.

**`kerf work edit`.** The init block advertises `kerf work edit <codename>  Mutate a work's bead-filter`. The natural flag guess is `--bead-filter <expr>`. That flag does not exist; the real flags are `--bead-filter-add label=codename:auth` and `--bead-filter-remove ...`. The help text on error is fine, but the AGENT INSTRUCTIONS block doesn't teach this and there's no `--bead-filter` shortcut. Once invoked correctly, the confirmation is terse: `Updated bead_filter for auth:  + label=codename:auth`. It does NOT say "this now matches N open beads" or "run `kerf next` to see them" — which is the natural follow-up. A fresh agent has to know to re-run `kerf next` on their own.

**`kerf status auth analyze`.** Advanced cleanly; printed the next pass's instruction block. BUT: the brief asks "does each create the pass dir + template?" The answer is **no**. After advancing through `problem-space → analyze → decompose`, the work directory still contains only `spec.yaml`. The pass numbering implies output files (`01-problem-space.md`, `02-analysis.md`, ...) but `kerf` doesn't seed them. The agent has to know to `touch` or write them from scratch. This is a real friction point: a fresh agent will probably read "Output: 02-analysis.md" and look for an existing file to edit, find none, and second-guess whether they're in the right directory.

**`kerf review auth`.** Returned `Error: jig 'plan' declares no review criteria for pass 'Analyze'` and exit 0. This is a bug or a documentation gap: either the `plan` jig should declare review criteria, or `kerf review` should print "this pass has no review gate; advance directly" instead of an Error: line. As an agent, I cannot tell whether `plan` lacks review intentionally or whether I have a corrupt jig file.

**`kerf preview` vs `kerf review` CLI shape.** `kerf review --pass <name>` works. `kerf preview --pass <name>` does NOT — preview requires `<status>` as a positional second arg. Two adjacent commands that take a pass identifier have different shapes. A fresh agent will guess wrong on at least one of them.

**Did I become productive?** Partially. I got as far as creating a `plan`-jig work, attaching a bead filter manually, and walking three passes. I could not validate that the queue surfaced the right beads, could not run `doctor`, could not run `triage`, could not run `bootstrap-filters`, could not commit `.kerf/project-identifier` without `-f`, and got no template files to write into. A fresh agent in this environment would burn at least one turn diagnosing `br`, then accept that several headline kerf commands simply do not work here, then proceed with the subset that does. The narrative arc is "init is great, then the wheels fall off intermittently for the rest of the flow."

## Rough edges (severity)

**Red (blocks the flow):**

1. **`br` shellout incompatible with `bd`-managed `.beads/`.** `kerf next`, `kerf triage`, `kerf doctor`, `kerf bootstrap-filters` all fail with `JSON error: missing field jsonl_export at line 7 column 1`. `br` reads `.beads/config.yaml` and expects its own schema; `bd init` writes a different schema. Every `bd`-backed repo (i.e. ~all kerf consumers per the docs) hits this. The agent instructions list `br` as a hard tool requirement, so this is a contradiction with reality. Either kerf must adapt its shellout to detect `bd` and use `bd export`, or the docs must state `bd` is unsupported.

2. **`.gitignore` instructions cause `git add` to fail.** Telling agents to write `.kerf/` + `!.kerf/project-identifier` then `git add .kerf/project-identifier` produces "paths are ignored by one of your .gitignore files". The correct pattern is `.kerf/*` + `!.kerf/project-identifier` (or `git add -f`). The instructions are technically wrong.

**Yellow (degrades the experience):**

3. **`kerf new <codename>` does not pre-populate `bead_filter` from matching labels.** `kerf new auth` on a store with three `codename:auth` beads should propose attaching them; instead `bead_filter: (none)` and the agent has to discover `kerf work edit --bead-filter-add` independently. Most of the value prop of "kerf attaches works to beads" is lost on first contact.

4. **`kerf next` exits 0 on error.** Same `br` failure, two exit codes: 0 from `next`, 1 from `bootstrap-filters` / `doctor` / `triage`. Scripting against `kerf next` is unsafe.

5. **Status advance does not seed pass-output template files.** Walking `problem-space → analyze → decompose` leaves the work dir with only `spec.yaml`. Pass output paths (`01-problem-space.md`, `02-analysis.md`, ...) are named everywhere but never created. Agent has to invent them from scratch.

6. **`kerf review` errors instead of degrading gracefully when a jig has no review criteria for a pass.** "Error: jig 'plan' declares no review criteria for pass 'Analyze'" should be "(no review gate for this pass — advance directly)".

7. **`kerf next` cold-start error path is bad.** Compare `kerf triage` cold-start ("project not initialized. Run 'kerf init' first.") with `kerf next` cold-start (raw JSON-decode trace). `next` should detect missing `project.yaml` and print the same message.

8. **`kerf work edit` confirmation is too terse.** "+ label=codename:auth" with no "this matches 3 open beads; run `kerf next` to see them". A fresh agent has no signal that anything useful happened.

9. **`kerf preview` vs `kerf review` flag-shape inconsistency.** `review --pass <name>` is the flag form; `preview <codename> <status>` is positional. Pick one.

**Green (cosmetic):**

10. **AGENT INSTRUCTIONS block places `### .gitignore` after the kerf-commands cheat-sheet.** Easy to miss. Move it up before commands, since you must do it before committing `project-identifier`.

11. **`kerf new` does not print the chosen jig as a separate "decision" line.** `Jig: plan (v1)` is buried in the work-created block, not announced as "I'm picking `plan` because it's `default_jig`; override with `--jig <name>`."

12. **No `kerf next` warning when work has empty bead_filter.** A work with no filter silently contributes nothing to the queue. Worth a yellow line in `kerf doctor` (assuming doctor worked).

## Critical-question answers

- **Wrong-path hints?** Yes — `.gitignore` instructions are wrong. Also `bead_filter` silently empty after `kerf new`.
- **Stale instructions?** The `kerf work edit <codename>  Mutate a work's bead-filter` line implies a `--bead-filter` flag that doesn't exist (real flags are `--bead-filter-add` / `--bead-filter-remove`).
- **Moments without a clear next step?** Yes, after `kerf next` failed with the `br` error — no recovery path is suggested. Also after `kerf status auth analyze` advanced with no template file created.
- **Help vs output vs spec agreement?** Help text on `kerf work edit` correctly lists the real flags; the AGENT INSTRUCTIONS block hints at a simpler flag that doesn't exist. Divergence between init-block prose and `--help` reality.
- **Need to read source to interpret CLI output?** Yes, twice: once to figure out why `kerf review` errored on missing review criteria (is it my bug or kerf's bug?), once to figure out why `kerf next` exits 0 on a hard failure (is it a warning or a real error?).
