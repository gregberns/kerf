# Spec Review — Draft Coordination, Areas, and Findings Specs

> Review of docs 29, 30, 31 against each other, Greg's decisions (docs 20, 26), existing specs (architecture.md, works.md, commands.md), and the spec change plan (doc 28).

---

## 1. Coordination Spec (doc 29) — Per-Spec Findings

### 1.1 Internal Consistency Issues

**Finding categories vs. severity terminology.** The coordination spec uses "Category A / B / C" throughout (e.g., "Category A findings may bypass the full work creation path"). The findings spec (doc 31) uses `code | task | spec | design` severity levels. The coordination spec never uses the word "severity" for its categories. These are meant to map to each other but use different language, which will cause confusion.

Relevant passages:
- Doc 29: "Three categories of findings determine routing: A — simple fix, B — implementation gap, C — spec deficiency"
- Doc 31: "severity: code | task | spec | design"

The coordination spec has **three** categories; the findings spec has **four** severity levels (splitting "design" out from "spec"). The coordination spec's Category C covers both spec deficiency and architectural problems, while the findings spec separates those into `spec` and `design`. This is a genuine inconsistency — the two specs disagree on the taxonomy.

**Design lifecycle states vs. work status values.** The coordination spec defines a design lifecycle:
```
drafting → coherent → sufficient → frozen
```
But the existing works.md defines status values per jig (e.g., Plan jig: `problem-space -> analyze -> decompose -> research -> change-spec -> integration -> tasks -> ready`). The coordination spec does not explain how these two status progressions relate. Is "drafting" a new status? Does "coherent" map to an existing jig status? This is a gap.

**Intent lifecycle "absorbed" is undefined operationally.** The coordination spec defines:
```
captured → designed → tasked → absorbed
```
"Absorbed" means "all derived tasks complete." But the existing work status system has `ready` as the terminal status for spec-writing jigs and `complete` for implementation jigs. How does "absorbed" relate? Is it a new status value? Is it computed? The spec does not say.

### 1.2 Consistency with Greg's Decisions

**ALLOCATE + MERGE as one agent: GOOD.** The coordination spec correctly models this (Section "Simplified Topology"):
> "In practice, ALLOCATE and MERGE/TEST run as a single coordinating agent."

This matches doc 26: "for now... the ALLOCATE and MERGE are done in the same agent."

**Agent instructions 30-40 lines: NOT ADDRESSED.** Doc 20 says: "We DO NOT want any more than 30-40 lines of instructions." The coordination spec's agent role descriptions are informational (for the spec reader), not agent-facing instructions, so this is not violated — but the spec does not mention this constraint anywhere. It should, since the coordination spec is the natural home for "how agents receive instructions."

**No baking in escalation rules: GOOD.** The coordination spec describes feedback routing structurally (by root cause location) rather than prescribing escalation rules. Doc 20: "I don't want to bake anything in."

**Work items not the center of execution: GOOD.** The coordination spec is explicit:
> "Tasks are the unit of execution, but not the unit of planning."

This matches doc 20's concern about works being the center of execution.

**Sessions not tied to works: PARTIAL.** The coordination spec barely mentions sessions. It references "execution sessions" in passing. It does not contradict doc 20, but it also does not address how sessions relate to the coordination model at all.

### 1.3 Consistency with Existing Specs

**`kerf map` and `kerf next` and `kerf resume` are referenced but not yet specified in commands.md.** The coordination spec relies heavily on these commands. `kerf resume` exists in commands.md already. `kerf map` and `kerf next` do not. This is expected (they are planned additions per doc 28) but should be noted — the coordination spec references behavior that is not yet specified elsewhere.

**The flow graph's "FAST TRACK" path.** The diagram shows a "FAST TRACK" where a known fix enters directly as a task. This path is not mentioned in the text beyond the diagram label. It is unclear whether this is distinct from the "tight loop" or a separate entry point. Underspecified.

### 1.4 Normative Quality

**Mostly normative, with some rationale leakage.** A few passages slip into "why" language:
- "This preserves the historical record and prevents retroactive coherence problems." (rationale)
- "This is the separation between planning structure (intents group by problem) and execution structure (batches group by availability)." (explanatory, borderline)

These are minor. The spec is predominantly normative.

### 1.5 Overreach

**The consistency model section goes further than needed.** Statements like "The system does not need millisecond consistency — it needs cycle-level consistency" and "Stale reads lead to suboptimal but not incorrect decisions" are architectural assertions that constrain implementation without clear benefit. They could be shortened to "Consistency is eventual, at polling-cycle granularity."

---

## 2. Areas Spec (doc 30) — Per-Spec Findings

### 2.1 Internal Consistency Issues

**No significant internal contradictions found.** The spec is internally consistent. Terms are used consistently throughout.

### 2.2 Consistency with Greg's Decisions

**Defined taxonomy, not freeform: GOOD.** Doc 20: Greg wants a "defined taxonomy for areas, not freeform." The areas spec is explicit:
> "Areas are not freeform tags; they are a defined taxonomy maintained in a single file."

This is well-aligned.

### 2.3 Consistency with Existing Specs

**Storage location.** The areas spec places `areas.yaml` at `~/.kerf/projects/{project-id}/areas.yaml`. This location is not yet documented in architecture.md's bench directory structure. Doc 28 identifies this as a needed change to architecture.md. Consistent with the plan.

**The `spec.yaml` extension.** The areas spec adds an `areas` field to `spec.yaml`:
```yaml
areas:
  - adapter
  - adapter.retry
```
This field is not in the current `spec.yaml` schema (works.md). Doc 28 calls this out as a needed change. Consistent with the plan.

**The `connections` property listed in doc 29 but absent from doc 30.** The coordination spec (doc 29) lists area properties as:
> "Name, Connections, Description"

But the areas spec (doc 30) defines area fields as only `description` and `parent`. There is no `connections` field. The "Edges (Deferred)" section in doc 30 explicitly defers typed edges between areas. This is a cross-spec inconsistency — doc 29 claims areas have "connections" but doc 30 does not model them.

### 2.4 Normative Quality

**Strong.** The spec is consistently normative. It specifies exact file formats, validation rules, error messages, and CLI commands. Implementable.

### 2.5 Completeness

**`kerf areas init` is underspecified.** The spec says it "prompts for or accepts initial areas" but does not specify the interface — CLI flags? Interactive prompts? A file path to import from? Given that kerf is agent-first, interactive prompts may not be appropriate.

**`kerf areas list` output format not specified.** The heat map example in the "Area Queries" section shows output for `kerf map`, not `kerf areas list`. What does `kerf areas list` output look like?

### 2.6 Overreach

**The depth limit of 4 is arbitrary.** The spec states: "The hierarchy has a maximum depth of 4 levels." This is a reasonable default but is stated as an absolute rule. Should this be configurable? If not, the spec should justify the choice or at least acknowledge it is a design choice.

---

## 3. Findings Spec (doc 31) — Per-Spec Findings

### 3.1 Internal Consistency Issues

**`kerf next` integration is described twice with different detail.** The "How Findings Surface in `kerf next`" section (lines 226-233) says findings appear "above new work" and within findings "severity determines order: `code` findings surface first... then `task`, then `spec`, then `design`." But the coordination spec (doc 29) says priority is "computed from graph structure, not assigned as static labels" and uses factors like completion momentum and dependency fan-out. These are compatible but the findings spec's absolute rule ("any finding in surfaced or triaged state ranks higher than any new-work bead, regardless of area or dependency ordering") may conflict with the coordination spec's more nuanced priority computation. What if a `spec`-severity finding that "requires planning review" blocks the queue above a critical dependency-unblocking new-work bead?

**The "Why Findings Are First-Class" section is rationale, not normative.** Lines 16-20 explain *why* the design choice was made. This should either be removed or moved to a non-normative note. Specs should say what the system does, not why.

### 3.2 Consistency with Greg's Decisions

**Findings as first-class: GOOD.** Doc 26: "downstream issues/rework should be prioritized over accepting new tasks coming from upstream." The findings spec makes this a system invariant:
> "Rework (from findings) has structural priority over new work in the queue."

**Completion momentum: NOT ADDRESSED.** Doc 26 raised completion momentum ("once the 4 is completed, how do we make sure the last one isn't just left stranded"). The coordination spec covers this (Section "Computed Priority"), but the findings spec's absolute priority rule ("any finding... ranks higher than any new-work bead, regardless") could undermine completion momentum. If a finding arrives when 4 of 5 tasks are done, does the remaining task get pre-empted by the finding? The interaction is not specified.

**Rework > new work: GOOD.** Both specs align on this.

### 3.3 Consistency with Existing Specs

**Storage location.** Findings live at `~/.kerf/projects/{project-id}/.findings/{finding-id}/`. The dot-prefix is a convention not seen elsewhere in kerf — works don't use dot-prefixed directories. This is a new pattern that architecture.md would need to document. The dot-prefix is explained (to exclude from work listings) but it introduces a new convention.

**Finding IDs vs. codenames.** Works use codenames (`adjective-noun`). Findings use `f-{date}-{sequence}`. These are different ID schemes, which is fine, but it means `kerf next` will need to handle two types of identifiers. The spec should clarify how findings appear in `kerf next` output — by finding ID? By title?

**`kerf finding create` vs. `kerf new`.** Doc 28 raised this as "Key Decision #4": should findings use `kerf new --source merge-test` or a dedicated `kerf finding` command? The findings spec chose the dedicated command route (`kerf finding create`). This is a decision — but it was flagged as needing Greg's input, and the spec makes the call without noting that the decision was made.

### 3.4 Normative Quality

**Mostly good, with rationale leakage.** The "Why Findings Are First-Class" section (lines 14-20) and the rationale paragraph under "Rework Before New Work" (lines 222-223: "The rationale: continuing to build on a flawed foundation creates compound waste") are explanatory, not normative. These should be trimmed or clearly marked as non-normative notes.

### 3.5 Completeness

**Missing: how does a finding become a work?** The findings spec says findings follow a "large path" where PLANNING "creates a new work (bug jig or plan jig) to address it." But the mechanics are not specified. Does PLANNING run `kerf new --related-to f-2026-05-08-001`? Does the finding get linked automatically? The `resolved_by` field exists but the creation-of-work step is hand-waved.

**Missing: how does ALLOCATE dispatch a finding on the small path?** The spec says ALLOCATE "dispatches it (or the bead it generates)" but does not specify how a finding generates a bead. Does the coordinator create a bead via `bd`? Does it create a work first? The small path needs more operational detail.

### 3.6 Overreach

**The `addressed → surfaced` transition.** The spec allows a failed fix to re-surface a finding. This is reasonable but introduces a cycle in the state machine. The spec should clarify: does this reset the finding's triage or just re-surface it for attention? The state machine text says "Fix attempt failed; finding re-surfaces for re-triage" — so it goes back to needing triage. This is clear enough.

---

## 4. Cross-Spec Findings

### 4.1 Category vs. Severity Taxonomy Mismatch

**This is the most significant cross-spec issue.**

The coordination spec defines three categories:
| Category | Root cause |
|---|---|
| A | Code (wrong implementation) |
| B | Tasks (wrong decomposition) |
| C | Spec (wrong or missing) |

The findings spec defines four severity levels:
| Severity | Root cause |
|---|---|
| `code` | Implementation error |
| `task` | Task decomposition gap |
| `spec` | Specification deficiency |
| `design` | Architectural problem |

These do not align. Category C in the coordination spec covers what the findings spec splits into `spec` and `design`. The coordination spec's flow graph shows a "wide loop" (TEST → PLAN → SPEC → TASK → EXEC → TEST) for approach-level problems, which maps to `design` severity — but Category C is described as "spec deficiency" not "design deficiency."

Either the coordination spec needs a Category D, or the findings spec should collapse `spec` and `design` into a single category, or both specs need to be updated to use the same taxonomy.

### 4.2 Findings Storage: Two Descriptions

The coordination spec says findings are stored "as new works" — it uses phrases like "The coordinator writes findings as new works in kerf" and "creates a new work via `kerf new`."

The findings spec gives findings their own storage: `~/.kerf/projects/{project-id}/.findings/{finding-id}/` with `finding.yaml` and `description.md`. Findings are explicitly NOT works — they are separate entities that may eventually *become* works.

These are contradictory. The coordination spec treats findings as works from the start. The findings spec treats findings as their own entity type that, once triaged, may spawn a work.

This needs resolution. The findings spec's model (separate entity) is more detailed and seems more considered. The coordination spec should be updated to match.

### 4.3 Area "Connections" Property

As noted above, the coordination spec lists "Connections — which other areas this area interfaces with" as an area property. The areas spec explicitly defers connections/edges to a future version. The coordination spec should remove the "Connections" property from its area description or mark it as deferred.

### 4.4 `kerf next` Priority Rules

The coordination spec describes priority as computed from four factors: rework before new work, completion momentum, dependency fan-out, area focus. It states these "compose into a ranking."

The findings spec adds an absolute rule: "Any finding in surfaced or triaged state ranks higher than any new-work bead, **regardless of area or dependency ordering**." (emphasis mine)

The "regardless" clause is in tension with the coordination spec's composition model. If findings always rank above everything else regardless of other factors, then completion momentum and dependency fan-out only matter within the set of non-finding items. This may be intentional but it is not stated consistently across the two specs.

### 4.5 Who Creates Findings — Coordinator vs. Agent Type

The coordination spec says: "the coordinator writes findings as new works."
The findings spec says: "MERGE/TEST agent — the primary source" and "EXECUTE agent — secondary source."

In the simplified topology (doc 29), MERGE/TEST runs as part of the coordinator. So both specs agree on the actor but use different names. This is confusing but not contradictory. Recommend standardizing: use "coordinator (performing MERGE/TEST)" or similar.

---

## 5. Consistency with Spec Change Plan (doc 28)

### 5.1 Missing Specs

Doc 28 calls for these new spec files:
- `specs/domain-model.md` — entity definitions, lifecycle state machines, relationship graph
- `specs/queue.md` — `kerf next`, priority computation, pull model
- `specs/coordination.md` — the coordination spec (doc 29 covers this)
- `specs/areas.md` — the areas spec (doc 30 covers this)
- `specs/findings.md` — the findings spec (doc 31 covers this)

The coordination spec (doc 29) effectively merges `domain-model.md` and `queue.md` into itself. It defines all entities, lifecycles, the flow graph, and priority computation in one document. Doc 28 envisioned these as separate specs. This is a structural decision — either approach works, but it means the coordination spec is very large and covers multiple concerns. If Greg wants the entity model and queue logic factored out into separate specs, the coordination spec would need to be split.

### 5.2 Spec Changes Not Yet Drafted

Doc 28 calls for modifications to:
- `specs/works.md` — add `areas`, `urgency`, `source`, `related_to` fields to `spec.yaml`
- `specs/commands.md` — `kerf next`, `kerf map`, `kerf finding create`, etc.
- `specs/architecture.md` — bench directory structure updates
- `specs/_index.md` — glossary and spec map updates
- `specs/dependencies.md` — cross-intent task dependencies
- `specs/verification.md` — area overlap checks
- `specs/cli.md` — principles updates

These are expected to be drafted later. The three current drafts are the new-spec portion of the work.

### 5.3 Key Decision #1 — "Work" as User-Facing Term

Doc 28 flagged: "Does 'work' remain the user-facing term?" The three drafts sidestep this. The coordination spec introduces "intent" and "design" as concepts but still says "An intent **is** a work." The areas spec uses "works" throughout. The findings spec uses both "works" and "findings" as separate entities. This decision remains open.

### 5.4 Key Decision #4 — Finding Ingestion

Doc 28 flagged: "`kerf new` with flags or a dedicated command?" The findings spec chose the dedicated `kerf finding create` command without noting this was a decision point. Greg should confirm this choice.

---

## 6. Summary

### Issues Requiring Resolution Before Greg Review

1. **Category/severity taxonomy mismatch** (cross-spec, Section 4.1). The coordination spec has 3 categories; the findings spec has 4 severity levels. These must align.

2. **Finding storage model contradiction** (cross-spec, Section 4.2). The coordination spec says findings ARE works. The findings spec says findings are their own entity type. Pick one.

3. **Area "Connections" property** (cross-spec, Section 4.3). The coordination spec claims areas have connections; the areas spec defers this. Remove from coordination spec.

### Issues for Greg to Decide

4. **Dedicated `kerf finding create` vs. `kerf new` with flags.** The findings spec made this call. Greg should confirm.

5. **Absolute priority of findings vs. composed priority.** The findings spec says findings always rank above new work "regardless." The coordination spec describes a nuanced composition. Which model?

6. **Spec structure: one large coordination spec or split into domain-model + queue + coordination?** Doc 28 envisioned three separate specs. Doc 29 combined them.

### Issues That Are Minor / Can Be Fixed During Finalization

7. Rationale leakage in findings spec ("Why Findings Are First-Class" section, "Rework Before New Work" rationale paragraph).
8. `kerf areas init` interface underspecified.
9. Design lifecycle states not mapped to existing jig status values.
10. "Intent lifecycle absorbed" not operationally defined relative to existing status system.
11. Finding → work creation mechanics not specified (how does PLANNING link a finding to the work it creates?).
12. Small-path bead creation mechanics not specified (how does ALLOCATE turn a finding into a bead?).
13. FAST TRACK path in flow diagram not described in text.
14. Depth limit of 4 for area hierarchy — arbitrary, should be acknowledged as a design choice.

### Verdict

**The drafts need one more pass to resolve the three cross-spec contradictions (items 1-3) before they are ready for Greg's review.** The category/severity mismatch and the finding-storage contradiction are substantive — Greg will be confused reading specs that disagree on these fundamentals. The connections property mismatch is easy to fix.

The remaining issues (items 4-14) are appropriate for Greg to see and decide on. The specs are well-written, predominantly normative, and cover the planned scope. They are close to ready.
