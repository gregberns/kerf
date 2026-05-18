# Harmonik Beta Feedback — Normalized Triage

Source: `source/kerf-beta-feedback_2026-05-18.md` (464 lines, dated 2026-05-15 → 2026-05-18).

Translation glossary (used throughout):
- **kerf** — the spec-writing CLI under triage here.
- **harmonik** — the user's other project, dogfooding kerf.
- **bench** — the global state directory at `~/.kerf/projects/<project-id>/` where works, spec.yaml, and per-pass artifacts live (not in the repo).
- **bead** — an issue / task tracked in `.beads/` (the harmonik issue store).
- **work** — a kerf work item, codename-addressed, with a `bead_filter` that attaches beads to it.
- **jig** — a workflow template (e.g. `spec` jig = 8-pass spec authoring loop).
- **pass** — one step of a jig (pass-1 = problem-space, pass-2 = decompose, etc.).

Each item:
- **Title** (imperative)
- **Severity** (from source)
- **Problem** — one sentence
- **Desired** — one sentence
- **Disposition** — existing-plan / new-plan / spec-only / quick-fix / dropped

---

## Theme 1 — Init / first-run UX

### 1.1 Don't prompt interactively during `kerf init`
- **Severity:** BLOCKER
- **Problem:** `kerf init` issues a y/N prompt for the project-wide bead_filter, but agent harnesses can't answer prompts; behavior on no-input is opaque (no echo, no resulting field).
- **Desired:** `kerf init` runs non-interactively by default (or accepts `--yes` / `--no` flags), with explicit echoed defaults so the agent can read what was decided.
- **Disposition:** new-plan 016 (init UX overhaul).

### 1.2 `kerf init` is partially idempotent and lies about state
- **Severity:** MAJOR
- **Problem:** Init prints `Project already initialized` and also `Created project.yaml` in the same run; the agent can't tell what actually changed.
- **Desired:** Init emits a single state-change summary (created / updated / unchanged for each artifact) so the agent has an unambiguous after-state.
- **Disposition:** new-plan 016.

### 1.3 Stale `Detected: 100% of beads use 'kerf:*' labels` claim
- **Severity:** MAJOR
- **Problem:** The label-prefix detector reports `kerf:*` even when zero beads carry that prefix — misfires or runs on stale data.
- **Desired:** Detector samples the current `.beads/issues.jsonl` and reports the actual top label prefix with a count, or stays silent if confidence is low.
- **Disposition:** new-plan 016. Flag: appears possibly fixed — verify.

  **Verdict:** still-live.
  **Evidence:** `internal/beads/heuristic.go:41-155` (`DetectFilterPrefix`) and `cmd/init.go:262-321` (`detectBeadFilter`) last touched by commit `88542e0` (2026-05-15 10:29), which predates the 12:36 user observation. No commits to either file since. Detector still scores prefixes by `(beads with P:<known-codename>) / (beads with any P:*)`, prints `Detected: N% of beads use \`P:*\` labels.` whenever a prefix scores above 0.5 — there is no silence path when the count is low or the corpus contradicts the prior. Code unchanged since 2026-05-15.
  **Action:** keep existing routing — new-plan 016 (init UX overhaul) remains the home for the fix. "Possibly fixed" marker removed.

### 1.4 `kerf init` claims `Set default_jig: spec` but project.yaml doesn't carry it
- **Severity:** MAJOR
- **Problem:** Init's stdout asserts a setting that doesn't land in the on-disk file.
- **Desired:** Either persist the field or stop claiming it was set; symptom and state agree.
- **Disposition:** new-plan 016 (or quick-fix if the bug is a single missing write).

### 1.5 Two `AGENT SETUP INSTRUCTIONS` blocks print back-to-back
- **Severity:** MAJOR
- **Problem:** A single `kerf init` invocation prints two overlapping-but-different instruction blocks (one hardcoded, one apparently from `kerf setup`), unlabeled.
- **Desired:** One canonical instruction block per run, or two clearly labeled blocks with a single source-of-truth pointer.
- **Disposition:** new-plan 016.

### 1.6 Instruction block omits the new command surface
- **Severity:** MAJOR
- **Problem:** Instructions don't mention `kerf next`, `kerf triage`, `kerf pin`, `kerf map`, `kerf areas`, `kerf work edit` — exactly the daily-driver commands.
- **Desired:** Instruction block lists current-generation commands prominently.
- **Disposition:** spec-only (agent-setup template text) + new-plan 016.

### 1.7 Ambiguous gitignore guidance
- **Severity:** MINOR
- **Problem:** Instructions say `gitignore .kerf/` but `commit .kerf/project-identifier` without spelling out the `!.kerf/project-identifier` re-include pattern.
- **Desired:** Instruction names the exact two-line gitignore pattern.
- **Disposition:** spec-only.

### 1.8 `project.yaml` shape doesn't match what init said it created
- **Severity:** BLOCKER (for beta goal)
- **Problem:** project.yaml lacks `default_jig`, `bead_filter`, and pass schedules for non-`implementation` jigs — yet init claims all three.
- **Desired:** project.yaml is the complete self-describing config init advertises, or init's claims match the actual schema.
- **Disposition:** new-plan 016.

### 1.9 project.yaml is a thin manifest with no `project.id`
- **Severity:** MINOR
- **Problem:** Identifier lives in `.kerf/project-identifier` (text file) and not in project.yaml; readers can't tell project.yaml is incomplete by design.
- **Desired:** project.yaml either includes `project.id` or has a documented "manifest only, see `.kerf/project-identifier`" header.
- **Disposition:** spec-only.

### 1.10 `kerf init` instruction block doesn't mention `kerf localize` or the bench
- **Severity:** MAJOR
- **Problem:** New agents don't learn that the authoritative state lives at `~/.kerf/projects/<id>/`, not in the repo's `.kerf/`.
- **Desired:** Instruction block names the bench path and points to `kerf localize` for reconciliation.
- **Disposition:** spec-only + new-plan 017.

---

## Theme 2 — Storage layout (`.kerf/` ↔ bench)

### 2.1 `.kerf/` and bench drift silently
- **Severity:** MAJOR
- **Problem:** Repo `.kerf/` contains different work dirs than `~/.kerf/projects/<id>/`; agents have no way to know which is canonical.
- **Desired:** A `kerf doctor` (or `kerf status --project`) command reports drift between repo `.kerf/` and bench, and proposes a fix.
- **Disposition:** new-plan 017 (storage reconciliation + `kerf doctor`).

### 2.2 No reconciliation tool surfaced from `kerf init` or `kerf next`
- **Severity:** MAJOR
- **Problem:** When drift exists, neither command flags it or routes the agent to `kerf localize`.
- **Desired:** Drift is detected on every `kerf next` / `kerf triage` run and surfaced as a one-line hint.
- **Disposition:** new-plan 017.

### 2.3 `kerf new` doesn't make the bench path obvious
- **Severity:** MINOR
- **Problem:** The bench path is printed once, mid-output; agents writing files relative to the repo silently produce orphan files.
- **Desired:** `kerf new` ends with a clearly fenced "working directory:" line.
- **Disposition:** new-plan 017 (or spec-only).

### 2.4 Doc drift: `work.yaml` vs. `spec.yaml`
- **Severity:** MAJOR
- **Problem:** Brief / docs reference `work.yaml`; actual file is `spec.yaml`. `kerf work edit --help` doesn't name the file at all.
- **Desired:** All docs and help text name `spec.yaml`; `kerf work edit --help` prints the file path it edits.
- **Disposition:** spec-only + quick-fix (help text).

---

## Theme 3 — `kerf next` ranking + entry friction

### 3.1 Drift wall before payload
- **Severity:** MAJOR
- **Problem:** `kerf next` leads with multi-line drift warnings (untriaged count + external_close + external_new) before any ranked item.
- **Desired:** Ranked items first; drift becomes a single-line footer with a `kerf triage` hint.
- **Disposition:** Plan 014 (process-management reframe).

### 3.2 `clean` is the wrong rank label for zero-bead works
- **Severity:** MAJOR
- **Problem:** Rank column reads `clean` for works whose filters resolve to zero beads — sounds positive when the state needs attention.
- **Desired:** Distinct labels: `empty` (filter right, no beads yet), `unwired` (no filter declared), `broken` (filter wrong); see Theme 6 too.
- **Disposition:** new-plan 019 (filter bootstrap + `kerf show` slot). Cross-ref `hk-43ate`.

### 3.3 No beads in feed when no work has a filter
- **Severity:** MAJOR
- **Problem:** `kerf next` returns zero ranked beads (only work-level diagnostics) until at least one work has a non-empty filter — but the path from "no filter" to "filter" isn't surfaced.
- **Desired:** When no work has a filter, `kerf next` calls out the bootstrap step explicitly (one command to run).
- **Disposition:** Plan 014 + new-plan 019.

### 3.4 `--kinds` / `--only` / `--include` enumerate against unlisted kinds
- **Severity:** MINOR
- **Problem:** Help mentions kind-filters but doesn't enumerate the valid kind values.
- **Desired:** `--help` lists the kinds.
- **Disposition:** quick-fix.

### 3.5 `untriaged_beads` is hygiene, not routing
- **Severity:** MINOR
- **Problem:** Untriaged count appears alongside ranked items, conflating hygiene cue with routing answer.
- **Desired:** Untriaged moves to footer alongside drift.
- **Disposition:** Plan 014.

### 3.6 `--format=json` shape undocumented
- **Severity:** MINOR
- **Problem:** Flag exists in `--help` but JSON schema isn't documented.
- **Desired:** `--help` links to schema doc, or `kerf next --format=json --schema` prints it.
- **Disposition:** quick-fix + spec-only.

---

## Theme 4 — `kerf triage` output + suggestions

### 4.1 Suggester proposes `kerf new <axis-label>` for cross-cutting tags
- **Severity:** MAJOR
- **Problem:** Triage suggests creating new works seeded from `axis:`, `tag:`, `kind:`, `scope:` labels — which are cross-cutting tags, not work cohorts. Following naively creates dozens of phantom works.
- **Desired:** Suggester prefers `codename:` and `spec:` prefixes; refuses to seed new works from cross-cutting label families.
- **Disposition:** new-plan 018 (triage rework).

  **Verdict:** still-live.
  **Evidence:** `cmd/triage.go:451-481` (`triageSuggestUntriaged`) picks the first label containing `:` (no prefix-family ranking) and emits `kerf new <value> --bead-filter 'label=...'` whenever no codename loosely matches. No commits to `cmd/triage.go` since `a78df33` (2026-05-15 11:34). Source feedback line 463 ("did not encounter repetitive triage suggestions") was about repetition / dedup, not about the cross-cutting-tag seeding bug; the underlying suggester logic is unchanged.
  **Action:** keep existing routing — new-plan 018 remains the home for the fix.

### 4.2 Suggester ignores archive state
- **Severity:** MAJOR
- **Problem:** `kerf new imrest` suggested even though `imrest` is archived under `~/.kerf/archive/`.
- **Desired:** Suggester checks archive; emits "(archived — consider unarchive or re-pin)" instead.
- **Disposition:** new-plan 018.

  **Verdict:** still-live.
  **Evidence:** `cmd/triage.go:451-481` does not consult archive state at all. `internal/feed/cleanup.go:41-51,84` shows archive awareness exists for cleanup items via `feed.Inputs.ArchivedOrFinalized`, but that map is never threaded into `triageSuggestUntriaged`. Code unchanged since 2026-05-15.
  **Action:** keep existing routing — new-plan 018 remains the home for the fix.

### 4.3 `kerf triage --ack` re-prints the full triage report
- **Severity:** MAJOR
- **Problem:** `--ack` dumps 137+ lines before advancing baseline; piping to logs is N× noisy.
- **Desired:** `--ack` prints only `Baseline advanced to <ts>` on success.
- **Disposition:** quick-fix.

### 4.4 No `--limit` / `--top` / `--group-by` on triage
- **Severity:** MAJOR (for scale)
- **Problem:** Triage output is unbounded; 168 entries today, would explode on a 10k-bead project.
- **Desired:** `--top N` and `--group-by codename-label` flags bound output.
- **Disposition:** new-plan 018.

### 4.5 Bead count discrepancy (163 pre-init vs. 168 post-init)
- **Severity:** MINOR
- **Problem:** Pre-init warning and triage count disagree (probably status-filter difference).
- **Desired:** One canonical count, or each header states its filter explicitly.
- **Disposition:** quick-fix.

### 4.6 `baseline: never` semantics not documented
- **Severity:** MINOR
- **Problem:** Triage shows `baseline: never` on first run, but `--help` doesn't explain that subsequent runs after `--ack` show deltas.
- **Desired:** `--help` documents baseline / delta / ack lifecycle.
- **Disposition:** spec-only.

### 4.7 `--kind=multi_matched` with zero items prints full report header
- **Severity:** NIT
- **Problem:** Flag ignored when count is zero; report header still prints.
- **Desired:** Prints `no multi_matched items` and exits.
- **Disposition:** quick-fix.

---

## Theme 5 — Work bead-filter bootstrap

### 5.1 No one-shot bootstrap from existing labels
- **Severity:** MAJOR
- **Problem:** Closing the 168-bead untriaged gap took 4 manual `kerf work edit` commands; there's no `kerf bootstrap --infer-filters-from-labels`.
- **Desired:** A single command infers per-work filters from existing `codename:` labels (or sampled top-label prefix).
- **Disposition:** new-plan 019.

### 5.2 `kerf show` doesn't display `bead_filter`
- **Severity:** MAJOR
- **Problem:** `kerf show <codename>` omits the `bead_filter` slot entirely — agent can't see the missing filter without `cat`-ing `spec.yaml`.
- **Desired:** `kerf show` prints `bead_filter: (none)` or the current value.
- **Disposition:** new-plan 019 (or quick-fix).

### 5.3 `kerf work edit` count includes closed beads (43 vs. 31 open)
- **Severity:** MAJOR
- **Problem:** Confirmation message says `Now matches: 43 beads` while `br list --status=open` says 31 — the delta is closed beads, not disambiguated.
- **Desired:** Confirmation reads `Now matches: N (M open / K closed). Previously: 0.`
- **Disposition:** quick-fix.

### 5.4 No `kerf work show <codename>` command
- **Severity:** MINOR
- **Problem:** No dedicated command to dump a single work's bead_filter without parsing yaml.
- **Desired:** `kerf work show <codename>` exists, complementing `kerf work edit`.
- **Disposition:** new-plan 019.

### 5.5 Multi-agent works appear unannounced in `kerf list`
- **Severity:** MAJOR
- **Problem:** A 5th work `phase-3-dot` showed up in per-work health created by another session; no flag to scope `kerf list` to "mine vs. others."
- **Desired:** `kerf list --created-by self` or per-session attribution shown.
- **Disposition:** new-plan 019.

---

## Theme 6 — Filter-syntax / convention drift

### 6.1 Bead-label convention split (`codename:foo` vs. bare `foo`)
- **Severity:** MAJOR
- **Problem:** Bead authors use both prefixed and bare conventions; kerf seeds works uniformly with `codename:*`, so half the filters silently resolve to zero.
- **Desired:** `kerf init` / `kerf new` samples existing label distributions and suggests the most-likely filter clause; `kerf next` warns when a `clean` filter has a near-match under a different prefix.
- **Disposition:** new-plan 019.

### 6.2 `clean` conflates three different states
- **Severity:** MAJOR
- **Problem:** `clean: bead_filter matches zero beads` shows up for "filter right but no matches yet," "filter wrong," and "no filter declared" — three different operator actions, one message.
- **Desired:** Distinct status lines: `empty`, `broken`, `unwired`. (Same as 3.2; tracked under existing bead `hk-43ate`.)
- **Disposition:** new-plan 019.

### 6.3 `phase-3-dot` had no `bead_filter` field at all
- **Severity:** MINOR
- **Problem:** Some works (created via `kerf new`) carry no `bead_filter:` key — only `pinned_beads: []`. Rendered as `clean` (see 6.2).
- **Desired:** `kerf new` always emits a `bead_filter:` key (possibly empty), and `kerf next` reports `unwired` for empty.
- **Disposition:** new-plan 019.

---

## Theme 7 — Spec-jig pass loop (review-gate / file conventions)

**Section verdict (re: "possibly-not-re-exercised" marker):** still-live.
**Evidence:** `internal/jig/builtin/spec.md` last touched by commit `dd89e34` (2026-05-14) — unchanged since the feedback was captured. Lines 111, 171, 219, 261, 303 still say "spawn a review sub-agent" with no fallback path (7.1, 7.3). Pass-3 instructions at line 135 still direct sub-agents to author files (7.2). No `kerf review` command exists in `cmd/` (7.3). No per-pass `*.md.template` files under `internal/jig/builtin/` (7.4). `cmd/show.go` was not touched since 2026-05-15 — review-criteria duplication (7.6) and pass output-filename surfacing (7.5) unchanged. `04-design/` directory pre-creation on status advance: no commits to `cmd/status.go` after 2026-05-15 (7.7). No new commits to `internal/jig/` since 2026-05-14.
**Action:** keep existing routing — all 7.x sub-items continue to point at new-plan 020.


### 7.1 Reviewer-sub-agent assumes Agent tool availability
- **Severity:** MAJOR
- **Problem:** Pass-2 and pass-4 instructions hard-require dispatching a reviewer via the Agent tool; in deferred-tool harnesses the Agent tool isn't loaded, so the gate falls back to self-re-read (structurally weaker).
- **Desired:** Jig instructions name fallback paths (fresh-context re-read, parent-orchestrator review) and don't assume any single harness primitive.
- **Disposition:** new-plan 020 (jig review-gate + pass-loop fixes).

### 7.2 Pass-3 research instructions tell sub-agents to write files
- **Severity:** MAJOR
- **Problem:** 2 of 5 research sub-agents hit harness rules that block `.md` writes from sub-agents; orchestrator had to persist on their behalf, with content-fidelity loss on the finalizer's heredoc fallback.
- **Desired:** Jig instructions tell the parent to collect inline returns and own the write step — sub-agents return text.
- **Disposition:** new-plan 020.

### 7.3 No `kerf review <codename>` command
- **Severity:** MAJOR
- **Problem:** Pass instructions reference "spawn a review sub-agent" but kerf doesn't ship the primitive.
- **Desired:** Either `kerf review` exists and emits a canned reviewer prompt, or instructions acknowledge the gap and route to whatever the harness exposes.
- **Disposition:** new-plan 020.

### 7.4 Pass-1 instructions ship no output template
- **Severity:** MINOR
- **Problem:** "Save to `01-problem-space.md`" with bullets on coverage but no skeleton; two agents will produce wildly different layouts.
- **Desired:** Jig ships `01-problem-space.md.template` (and per-pass siblings).
- **Disposition:** new-plan 020.

### 7.5 Pass output file-naming convention not surfaced by `kerf show`
- **Severity:** MAJOR
- **Problem:** Pass-2 output is `02-components.md` (abbreviated) while pass-1 is `01-problem-space.md` (full); `kerf show` doesn't print the canonical filename in a fixed location.
- **Desired:** `kerf show` prints `Pass N: <name> → Output: NN-<filename>.md` in a stable place.
- **Disposition:** new-plan 020.

### 7.6 Review-criteria duplicated in `kerf show`
- **Severity:** MINOR
- **Problem:** Review-criteria checklist appears twice in slightly different framings ("What done looks like:" and "Review Criteria").
- **Desired:** One normative source: `Done = reviewer APPROVE on these criteria: ...`.
- **Disposition:** new-plan 020 (or spec-only).

### 7.7 `04-design/` not pre-created on pass-4 entry
- **Severity:** MINOR
- **Problem:** `kerf status … change-design` advances status but doesn't create `04-design/`; sub-agent has to `mkdir -p` first.
- **Desired:** Pass-N status advance creates the pass-N output directory.
- **Disposition:** new-plan 020 (or quick-fix).

### 7.8 No convention for "one design decision per file" vs. monolithic
- **Severity:** MINOR
- **Problem:** Pass-4 is silent on whether each design decision is one file or `04-design/design.md` aggregates them; impacts parallel sub-agents.
- **Desired:** Jig declares the convention; per-decision-file is better for parallelism.
- **Disposition:** new-plan 020 (or spec-only).

---

## Theme 8 — Agent setup instructions / docs drift

### 8.1 Two AGENT SETUP INSTRUCTIONS blocks (cross-listed with 1.5)
- See 1.5.

### 8.2 Instructions omit `kerf next` / `kerf triage` / `kerf pin` / `kerf map` / `kerf areas` / `kerf work edit` (cross-listed with 1.6)
- See 1.6.

### 8.3 Instructions don't surface that the project's authoritative state lives on the bench
- **Severity:** MAJOR
- **Problem:** Agents writing files relative to the repo silently produce orphan files; the bench location isn't called out.
- **Desired:** Instruction block dedicates a section to "where state lives."
- **Disposition:** spec-only.

### 8.4 `--help` for `kerf work edit` doesn't name the file it edits
- **Severity:** MINOR
- **Problem:** Help says "edit a work's bead-attachment configuration" without naming `spec.yaml`.
- **Desired:** Help text names the file and bench path.
- **Disposition:** quick-fix.

---

## Theme 9 — Command-UX gaps

### 9.1 `kerf init --yes` / `--no` flags
- **Severity:** MAJOR (blocker upstream as 1.1)
- **Problem:** No way to suppress the interactive prompt from automation.
- **Desired:** `--yes` and `--no` flags exist; `--force` is distinct.
- **Disposition:** new-plan 016.

### 9.2 `kerf next --top N`
- **Severity:** MAJOR
- **Problem:** No way to bound the ranked-feed output.
- **Desired:** `--top N` flag.
- **Disposition:** quick-fix.

### 9.3 `kerf next --kinds` enumeration in `--help`
- See 3.4.

### 9.4 `kerf triage --top N` / `--group-by`
- See 4.4.

### 9.5 `kerf doctor` / `kerf status --project`
- **Severity:** MAJOR
- **Problem:** No single command answers "is my project healthy?"
- **Desired:** One command checks project.yaml completeness, `.kerf/` ↔ bench sync, work-filter coverage; prints a green/yellow/red summary.
- **Disposition:** new-plan 017.

### 9.6 `kerf status` `--quiet` flag
- **Severity:** NIT
- **Problem:** Every `kerf status <codename> <pass>` transition prints the full next-pass block; noisy when scripted.
- **Desired:** `--quiet` suppresses the instruction dump.
- **Disposition:** quick-fix.

### 9.7 `kerf preview <next-status>` to peek at next-pass instructions
- **Severity:** MINOR
- **Problem:** No way to view the next-pass instructions before advancing status.
- **Desired:** `kerf preview` (or `kerf show --pass=N+1`) shows next-pass instructions read-only.
- **Disposition:** quick-fix (or new-plan 020).

### 9.8 `kerf show --compact`
- **Severity:** NIT
- **Problem:** `kerf show <codename>` prints full pass instructions + file tree + session ledger + command palette every time.
- **Desired:** `--compact` mode prints just status, next-pass name, file count, last-session marker.
- **Disposition:** quick-fix.

---

## Theme 10 — Harmonik-side bugs (out-of-scope for kerf)

### 10.1 Daemon claim-path ignores `br ready --priority` ordering
- **Severity:** MAJOR
- **Disposition:** dropped — harmonik bug `hk-rp48p` already filed.

### 10.2 `br close` 10s timeout fails successful runs
- **Severity:** MAJOR
- **Disposition:** dropped — harmonik bug `hk-yjsk8` already filed.

### 10.3 Orphan-sweep observes stale intents, resets zero
- **Severity:** MAJOR
- **Disposition:** dropped — harmonik bug `hk-sc3o4` already filed.

### 10.4 Daemon dirties parent repo's working tree on launch
- **Severity:** MAJOR
- **Disposition:** dropped — harmonik bug `hk-jvzc2` already filed.

### 10.5 SIGTERM doesn't propagate to tmux children
- **Severity:** MINOR
- **Disposition:** dropped — harmonik bug `hk-44w19` already filed.

### 10.6 No documented `hk --project <dir> --max-concurrent N` invocation in HANDOFF.md
- **Severity:** NIT
- **Disposition:** dropped — harmonik doc issue; bead `hk-icecw` covers the subcommand path.

---

## Possibly-already-fixed markers — resolved 2026-05-18

All three "verify before action" flags below were investigated against the current `main` (HEAD `d814f46`). Findings:

- **1.3** (stale `kerf:*` detector) — **still-live.** `cmd/init.go` + `internal/beads/heuristic.go` last touched 2026-05-15 10:29 (commit `88542e0`), before the 12:36 user observation. No silence path for low-confidence detections. See inline verdict block under 1.3.
- **4.1** (cross-cutting-tag suggestion) — **still-live.** `cmd/triage.go:451-481` unchanged since 2026-05-15. The 2026-05-18 source note (`Did not encounter: repetitive triage suggestions`) was about repetition / dedup, not the suggester's prefix-family selection. See inline verdict block under 4.1.
- **4.2** (archive-aware suggestions) — **still-live.** `cmd/triage.go` does not consult archive state. See inline verdict block under 4.2.
- **7.x** (spec-jig pass loop) — **still-live.** `internal/jig/builtin/spec.md` last touched 2026-05-14 (commit `dd89e34`). All six sub-items verified unchanged. See section verdict at the top of Theme 7.

No items were demoted to `dropped (fixed in <commit>)`. All "verify" flags above have been resolved; the inline `Flag: appears possibly fixed — verify.` line on 1.3 remains as historical record alongside the verdict.
