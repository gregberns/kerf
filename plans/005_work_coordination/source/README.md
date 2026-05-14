# Plan 005 Source Material

This directory holds the brainstorming, exploration, and decision documents that produced Plan 005 (Work Coordination). Files retain their original numeric prefixes so chronological cross-references (e.g. "see doc 19") still resolve. The top level contains only the load-bearing documents someone returning to this plan needs to understand the final design; earlier and superseded material has been moved into subdirectories.

## Layout

```
source/
  25_system_shape.md          # The accepted design — start here
  32_spec_review.md           # Review of draft specs against decisions
  33_USER_RESPONSE.md         # User direction after spec review
  34_phases.md                # Phasing of implementation
  35_implementation_plan.md   # Bead breakdown / implementation plan
  36_SESSION_SUMMARY.md       # End-of-session summary
  37_FORWARD_PLAN.md          # Next steps (Phase 2/3, Plan 004, hygiene)

  phase1_planning/            # Initial divergent brainstorm
    00_process.md             # Process overview
    01..06_*.md               # Six independent perspective agents
    07_pattern_synthesis.md   # Synthesis of the six
    08_critical_evaluation.md # First critique pass
    09_options_menu.md        # Decision-ready options output
    10_USER_RESPONSE.md       # User direction closing Phase 1

  exploration/                # Mid-session deep dives and second-round design
    11..16_*.md               # Deep dives (factory line, priority, beads, areas, protocols, continuity)
    17_critic_coherence.md    # Critic pass over 11-16
    18_critic_practitioner.md # Practitioner critic over 11-16
    19_final_synthesis.md     # First synthesis attempt (rejected by user — see 20)
    20_USER_RESPONSE.md       # User pushes for systems framing
    21..24_*.md               # Second-round dives (domain model, flow, TPS, multi-agent)
    26_USER_RESPONSE.md       # User accepts 25 as the system shape
    27_critical_decisions.md  # Pre-spec confirmation checklist
    28_spec_change_plan.md    # Plan for which specs change/are created

  superseded/                 # Draft specs replaced by the real specs in /specs
    29_draft_coordination_spec.md   # See specs/coordination.md
    30_draft_areas_spec.md          # See specs/areas.md
    31_draft_findings_spec.md       # Folded into specs/coordination.md
```

## Reading order for a new visitor

1. `25_system_shape.md` — the accepted design
2. `34_phases.md` and `35_implementation_plan.md` — how it gets built
3. `37_FORWARD_PLAN.md` — what's next
4. Dive into `exploration/` or `phase1_planning/` only if you need the rationale behind a specific decision.
