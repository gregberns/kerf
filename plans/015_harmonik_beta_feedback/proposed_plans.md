# Proposed Follow-On Plans

Stubs only — one paragraph of intent + scope bullets per plan. Each absorbs a subset of `triage.md` items routed to it.

Numbering: 014 (process-management reframe) and 015 (this triage) are taken. New plans start at 016.

---

## Plan 016 — Init UX overhaul

**Intent.** `kerf init` is the agent's first contact with kerf. Today it issues an interactive prompt with no escape hatch, lies about state it didn't persist, prints two overlapping AGENT SETUP INSTRUCTIONS blocks, and omits the current-generation command surface. Rework init so a fresh-context agent can run it once, read the output, and have an unambiguous, complete project setup with no manual fixes.

**Scope.**
- Non-interactive default; add `--yes` / `--no` flags; `--force` distinct from both.
- Stop printing `Set default_jig: spec` / `Created project.yaml` unless the artifact actually changed; emit a single state-change summary.
- Repair the label-prefix detector (sample current `.beads/issues.jsonl`, not stale data).
- Collapse the two AGENT SETUP INSTRUCTIONS blocks into one canonical source.
- Add `kerf next` / `kerf triage` / `kerf pin` / `kerf map` / `kerf areas` / `kerf work edit` to the instruction text.
- Mention the bench location (`~/.kerf/projects/<id>/`) and `kerf localize` explicitly.
- Ensure `default_jig` and pass-schedule fields actually land in `project.yaml` if init advertises them.

Absorbs triage items: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 1.8, 9.1.

---

## Plan 017 — Storage reconciliation + `kerf doctor`

**Intent.** The split between repo `.kerf/` and bench `~/.kerf/projects/<id>/` is a real architectural choice but agents can't tell which is canonical, drift accumulates silently, and there's no single "is my project healthy?" surface. Build the doctor / reconciliation primitives.

**Scope.**
- `kerf doctor` (or `kerf status --project`) — checks project.yaml completeness, repo-vs-bench sync, per-work bead_filter coverage, archive orphans.
- Drift surfacing on every `kerf next` / `kerf triage` (one-line footer when drift exists).
- `kerf new` ends with a clearly fenced "working directory:" line.
- Doc cleanup: every reference to `work.yaml` becomes `spec.yaml`; `kerf work edit --help` names the file path.
- Optional: `kerf localize --check` non-destructive preview of what reconcile would do.

Absorbs triage items: 2.1, 2.2, 2.3, 2.4, 9.5, 1.10.

---

## Plan 018 — Triage rework

**Intent.** `kerf triage` is the "what's wrong with this project today" surface, but today it dumps unbounded output, emits low-quality suggestions for cross-cutting label families, ignores archive state, and re-prints the full report on `--ack`. Tighten it so agents can lean on it for routing.

**Scope.**
- Suggester refuses to seed new works from `axis:`, `tag:`, `kind:`, `scope:` prefixes; prefers `codename:` and `spec:`.
- Archive-aware suggestions (emit `(archived)` instead of `kerf new <archived-name>`).
- `--ack` prints `Baseline advanced to <ts>` only (no full re-dump).
- `--top N` and `--group-by codename-label` flags.
- One canonical bead count (resolve 163-vs-168 discrepancy).
- `--help` documents the baseline / delta / ack lifecycle.
- `--kind=multi_matched` with zero items prints `no multi_matched items` and exits.

Absorbs triage items: 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.7.

---

## Plan 019 — Filter bootstrap + `kerf show` filter slot

**Intent.** Closing the 168-bead untriaged gap took 4 manual `kerf work edit` commands; `kerf show` doesn't display the `bead_filter` slot at all; the `clean` rank label conflates three different states (empty / unwired / broken); and bead-label conventions split between prefixed and bare. Build the filter-bootstrap primitive and clean up the surfaces around it.

**Scope.**
- `kerf bootstrap-filters` (or `kerf work edit --infer-from-labels`) — sample existing labels, propose filters for every work in one pass, apply with confirmation.
- `kerf show <codename>` prints `bead_filter:` slot (current value or `(none)`).
- Distinct rank labels in `kerf next`: `empty` / `unwired` / `broken` (cross-ref existing bead `hk-43ate`).
- `kerf work edit` count message disambiguates open / closed.
- `kerf work show <codename>` command.
- `kerf new` always emits a `bead_filter:` key (possibly empty).
- `kerf next` warns when a `clean` filter has a near-match under a different prefix (e.g. `codename:bridge-integration` filter + `bridge-integration` label both exist → suggest swap).
- `kerf list --created-by self` (or per-session attribution).

Absorbs triage items: 3.2, 3.3, 5.1, 5.2, 5.3, 5.4, 5.5, 6.1, 6.2, 6.3.

---

## Plan 020 — Jig review-gate + pass-loop fixes

**Intent.** The spec jig's pass loop hard-requires sub-agent primitives (Agent tool, sub-agent file writes) that aren't universally available, ships no output templates, and leaves file-naming conventions ambiguous. Make the jig harness-agnostic and template-driven.

**Scope.**
- Review-gate fallback paths documented in jig instructions (fresh-context re-read; parent-orchestrator review).
- Pass-3 instruction template tells the parent to collect inline returns and own the write step.
- `kerf review <codename>` command (or explicit acknowledgment that the harness owns the primitive).
- Ship per-pass output templates (`01-problem-space.md.template`, etc.).
- `kerf show` prints canonical pass filename in a stable location.
- Deduplicate "What done looks like" and "Review Criteria" blocks.
- Pass-N status advance creates the pass-N output directory.
- Declare the "one design decision per file" convention (or aggregate) in the jig.
- `kerf preview <next-status>` peek-at-next-pass command.
- `kerf show --compact` mode.
- `kerf status --quiet` for scripted transitions.

Absorbs triage items: 7.1, 7.2, 7.3, 7.4, 7.5, 7.6, 7.7, 7.8, 9.6, 9.7, 9.8.
