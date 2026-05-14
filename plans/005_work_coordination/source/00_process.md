# Brainstorming Process for Plan 005

## Phase 1: Divergent Thinking (6 agents, independent)

Each agent reads the problem definition and brainstorms from a specific perspective.
No cross-pollination — each works independently to maximize idea diversity.

| Agent | Perspective | File |
|-------|------------|------|
| A | **Systems Architect** — graph theory, data structures, information architecture | `01_systems_architect.md` |
| B | **Developer Experience** — what does the agent/user actually need moment-to-moment | `02_developer_experience.md` |
| C | **Process Designer** — workflow, lifecycle, state machines, process patterns | `03_process_designer.md` |
| D | **Prior Art** — how do other tools/domains solve portfolio coordination | `04_prior_art.md` |
| E | **Practitioner** — someone who's lived the pain, pragmatic solutions | `05_practitioner.md` |
| F | **Contrarian** — challenges assumptions, finds simpler framings, asks "do we even need this?" | `06_contrarian.md` |

## Phase 2: Synthesis (2 agents)

Two synthesis agents independently read ALL Phase 1 outputs and:
- Extract distinct ideas (dedup across agents)
- Group by which problem(s) they address
- Rate on: feasibility, leverage (bang for buck), complexity, risk
- Identify complementary ideas that work well together
- Flag contradictions worth resolving

| Agent | Focus | File |
|-------|-------|------|
| G | **Pattern Finder** — clusters ideas, finds themes, identifies the 80/20 | `07_pattern_synthesis.md` |
| H | **Critical Evaluator** — stress-tests ideas, finds gaps, ranks by leverage | `08_critical_evaluation.md` |

## Phase 3: Final Options Menu

One agent reads both synthesis outputs and produces a clean options document
organized for human decision-making.

| Agent | Focus | File |
|-------|-------|------|
| I | **Options Writer** — clean summary of top approaches for Greg to review | `09_options_menu.md` |

## Reading Order for Greg

1. `09_options_menu.md` — start here, this is the decision document
2. `07_pattern_synthesis.md` / `08_critical_evaluation.md` — if you want the reasoning
3. `01-06_*.md` — if you want the raw brainstorming
