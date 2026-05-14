# Contrarian Analysis: Work Coordination

> Challenging assumptions, finding simpler framings, and asking whether the five claimed problems are even real.

---

## 1. These Are Not Five Problems -- They Are One Problem With Five Symptoms

**Assumption challenged:** That there are five distinct problems requiring distinct solutions.

**Alternative framing:** The actual problem is "agents start sessions without enough context." That is it. Every one of the five claimed problems -- no persistent map, island works, no intake queue, late requirements, no cross-work coherence -- is a consequence of an agent beginning a session without knowing what else exists. If `kerf resume` (or whatever session-start mechanism) dumped a one-page summary of all active works, their statuses, and their overlaps, problems 1-5 largely evaporate. The agent would see the map, see the islands, see the queue, see the in-flight work, and see the overlaps.

**Why this might be right:** kerf already has `kerf list`, `kerf show`, and dependency declarations. The information exists. The problem is that agents do not read it at the right moment, or the output is not formatted for the decision they need to make.

**Why this might be wrong:** Context alone does not create coherence. An agent can see 12 active works and still design the 13th in isolation because it lacks the judgment to synthesize constraints across them. Information availability is necessary but not sufficient.

---

## 2. WIP Limits Would Kill Most of These Problems Without Any New Features

**Assumption challenged:** That coordination tooling is needed to manage many concurrent works.

**Alternative framing:** If you WIP-limit to 3 active works per project, you do not need an intake queue (problem 3), cross-work coherence is trivial because there are only 3 things (problem 5), the "map" fits in a sentence (problem 1), and late-arriving requirements affect at most 3 things (problem 4). The real question is: why are there so many concurrent works? Is the user (or the orchestrator) starting too many things? The coordination problem might be entirely self-inflicted.

**Why this might be right:** Every agile methodology in history converges on "limit WIP" as the highest-leverage intervention. If 15 works are in flight, the problem is not "we need better coordination tooling" -- the problem is "we have 15 works in flight." Solving the coordination problem enables the bad habit of unbounded WIP.

**Why this might be wrong:** Spec-writing is different from implementation. Works in the "spec" phase are cheap to have open -- they are just documents. The WIP cost model might genuinely be different here. Also, some projects really do have 10+ areas that need speccing before any implementation starts.

---

## 3. The User Is the Map

**Assumption challenged:** That the persistent map needs to be a data structure managed by tooling.

**Alternative framing:** Greg is the orchestrator. Greg knows the big picture. The "map" is in Greg's head, and the problem is not that it does not exist but that there is no convenient place to write it down and no mechanism to inject it into agent sessions. A single manually-maintained markdown file -- `~/.kerf/projects/{id}/MAP.md` -- that Greg updates every few days would solve this. No graph algorithms, no automatic relationship detection, no new commands. Just a file that `kerf resume` includes in its output.

**Why this might be right:** Automatic coherence detection is an AI-complete problem. Having a human maintain a 20-line summary of "here is what matters and how things relate" is more accurate and cheaper than any algorithm kerf could run. The tooling just needs to surface that file at the right moment.

**Why this might be wrong:** It does not scale, and it breaks the "agents should be autonomous" aspiration. If the human has to maintain the map, the human becomes the bottleneck. Also, the human forgets to update it.

---

## 4. The Problem Is Not kerf -- It Is the kerf/beads/harmonik Split

**Assumption challenged:** That kerf should own work coordination.

**Alternative framing:** kerf specs individual works. beads tracks implementation tasks. harmonik (presumably) handles execution. The coordination gap exists precisely because these three tools were designed as separate concerns, and no one owns the cross-cutting view. Adding coordination to kerf means kerf is now doing three things: spec management, work lifecycle, AND portfolio coordination. That is scope creep. Maybe the answer is: do not add coordination to kerf. Instead, build a thin "portfolio" layer that reads from kerf and beads and provides the cross-cutting view. Or, more radically, merge kerf and beads into one tool.

**Why this might be right:** The Unix philosophy of small composable tools works when the tools share a common interface (text streams). kerf and beads share... a filesystem? That is a weaker contract. The coordination problem might exist because the tool boundaries are wrong, not because any individual tool is missing features.

**Why this might be wrong:** Merging tools creates complexity. kerf is already non-trivial. Adding beads' task tracking would double the surface area. A thin overlay might be the right call, but "thin overlays" have a way of becoming thick.

---

## 5. Works Might Be the Wrong Unit

**Assumption challenged:** That "work" is the right granularity for coordination.

**Alternative framing:** A "work" in kerf is a fairly heavyweight thing -- it has a jig, passes, artifacts, sessions, snapshots, dependencies. What if the coordination problem exists because the atomic unit is too big? If works were smaller and more composable -- say, a single pass could be a standalone artifact -- then overlap would be mechanically impossible because each piece would be smaller than the overlap boundary. Alternatively, what if works are too small? What if the right unit is a "theme" or "initiative" that groups related works? The five problems might be symptoms of a missing level in the hierarchy, not a missing feature at the current level.

**Why this might be right:** The SDLC patterns in works.md (spike -> plan -> implementation) already show that one logical effort spans multiple works. The coordination problem is literally "how do we manage the relationships between these works" -- which is exactly what happens when your atomic unit is smaller than your logical unit.

**Why this might be wrong:** Adding a grouping level (epic, theme, initiative) is the oldest trick in the project management book and it rarely works. It just moves the coordination problem up one level. You end up needing "initiative coordination" next.

---

## 6. Just Make `kerf list` Better

**Assumption challenged:** That new commands, data structures, or concepts are needed.

**Alternative framing:** What if the entire solution is: `kerf list` shows a richer view? Right now it presumably lists works with status. What if it also showed: (a) which works share modified spec files, (b) which works have `inform` dependencies on each other, (c) a one-line "cluster" label derived from overlapping artifact paths, and (d) a staleness indicator? No new concepts. No new data model. Just a better query over existing data. Maybe add `kerf list --related <codename>` to show everything connected to a specific work.

**Why this might be right:** The data already exists. spec.yaml has dependencies. Artifact files have paths. Jig passes reference spec areas. The coordination information is latent in the existing data -- it just needs to be surfaced. A view-layer solution is cheaper and less risky than a model-layer change.

**Why this might be wrong:** Latent data is not the same as explicit data. "These two works both touch auth" is only knowable if the works mention "auth" somewhere parseable. Relying on filename heuristics or keyword matching is fragile. Explicit relationship declarations are more reliable.

---

## 7. Agents Should Figure This Out Themselves

**Assumption challenged:** That tooling needs to solve the coherence problem.

**Alternative framing:** Give the agent a better prompt. Literally. When `kerf resume` fires, include a system instruction: "Before beginning work, run `kerf list` and review all active works. Identify any that overlap with your current work. If you find overlaps, read their artifacts before proceeding." This is a process fix, not a tooling fix. The agent has the tools (`kerf list`, `kerf show`). It just does not use them because nobody told it to.

**Why this might be right:** The problem statement says "agents lose big picture over sessions." But agents lose everything over sessions -- that is what session boundaries are. The solution for every other kind of context loss is SESSION.md and `kerf resume`. Cross-work context is just another thing to include in the session-start ritual. No new features needed.

**Why this might be wrong:** Agents are bad at self-directed exploration. "Review all active works" sounds simple but an agent with 12 works will either skim superficially or burn half its context window reading artifacts. The tooling needs to pre-digest the information, not punt to the agent's judgment.

---

## 8. The Filesystem-as-Database Constraint Is Creating This Problem

**Assumption challenged:** That the filesystem is still the right storage model at portfolio scale.

**Alternative framing:** Every one of the five problems is a query problem. "What is the big picture?" is a query. "Which works overlap?" is a query. "What is the priority order?" is a query. Filesystems are terrible at queries. You can list directories and read files, but you cannot ask "which works touch the same spec area?" without reading every spec.yaml and every artifact. A SQLite database (still a single file, still portable, still no daemon) would make all five problems trivial query exercises. kerf could keep the filesystem as the authoring interface while maintaining a query index.

**Why this might be right:** The "filesystem is the database" philosophy works beautifully for individual work management. It breaks down at the portfolio level because portfolio questions are inherently cross-cutting. A lightweight index (even just a JSON file rebuilt on each command) would enable the queries without abandoning the filesystem-first philosophy.

**Why this might be wrong:** Adding an index means maintaining consistency between the index and the filesystem. That is a new class of bugs. Also, "just add a database" is the solution engineers propose for every problem, and it usually brings more complexity than it solves.

---

## 9. Smaller, More Frequent Integration Instead of Better Planning

**Assumption challenged:** That better upfront coordination prevents problems.

**Alternative framing:** The five problems are all about preventing conflicts before they happen. But what if the answer is not prevention but rapid detection and resolution? Instead of building a system that ensures works do not overlap, build a system that detects overlap quickly and makes merging cheap. This is the Git model: do not prevent merge conflicts, make resolving them fast. In kerf terms: let works be islands, but run a periodic `kerf check-coherence` that diffs all active works' artifacts against each other and flags overlaps. Fix them when found, not before they occur.

**Why this might be right:** Prevention systems are expensive to build and fragile. Detection systems are cheap and robust. You only pay the coordination cost when there is an actual conflict, not on every work creation. In practice, most works probably do not overlap at all.

**Why this might be wrong:** Spec conflicts are not like code merge conflicts. Two works that specify contradictory behaviors for the same system area cannot be "merged" -- one has to yield. Late detection of spec conflicts means wasted work. The earlier you catch it, the cheaper the fix.

---

## 10. The Real Problem Is That kerf Does Not Know About Specs

**Assumption challenged:** That the coordination problem is about works relating to each other.

**Alternative framing:** Works do not actually overlap with each other. Works overlap because they modify the same underlying specs. The real missing concept is not "work relationships" but "spec ownership." If kerf tracked which spec files (in the repo's `specs/` directory) each work intends to modify, coordination becomes trivial: when you create a new work, kerf tells you "these 2 other works also plan to modify `specs/auth.md`." This is analogous to file locking in version control -- not locking the work, but locking the target. This requires almost no new infrastructure. Just add an optional `affects: [specs/auth.md, specs/sessions.md]` field to spec.yaml and a query command.

**Why this might be right:** It is concrete, minimal, and addresses the root cause rather than the symptoms. The reason works conflict is that they touch the same spec. Making that explicit at creation time -- and warning when there is contention -- solves problems 2, 4, and 5 directly. It also gives you the "map" for free (problem 1): the map is just "which specs have active works against them."

**Why this might be wrong:** Agents do not always know which specs they will affect at creation time. The discovery happens during the work, not before it. Also, spec files are not always the right granularity -- two works might modify different sections of the same spec file without conflicting.

---

## 11. You Are Solving a Coordination Problem That Does Not Exist Yet

**Assumption challenged:** That these problems are currently causing pain at a scale that justifies building solutions.

**Alternative framing:** How many times has cross-work incoherence actually caused a real problem? Not "could cause" -- has caused. If kerf is relatively new and most usage involves 1-3 concurrent works, the five problems are theoretical. Building coordination infrastructure for a problem that might emerge at scale is premature optimization. Ship what exists, use it hard, and solve the coordination problems as they actually manifest. The specific shape of the real problem will be different from (and simpler than) the theoretical problem.

**Why this might be right:** YAGNI is the most reliable principle in software development. Building for a future that has not arrived means guessing at requirements. The best time to solve the coordination problem is when you have 10 concrete examples of coordination failures, not when you have 0.

**Why this might be wrong:** kerf is a spec-writing tool. If the specs are incoherent, the implementations will be incoherent, and the cost of fixing that is much higher than the cost of prevention. Also, the user is clearly experiencing these problems or they would not be running a 9-agent brainstorming exercise about them.

---

## Summary of Challenges

| # | Core Challenge | If Right, Then... |
|---|---------------|-------------------|
| 1 | Same problem, five symptoms | Fix session-start context, not five features |
| 2 | WIP limits solve it | Add `max_active_works` to config.yaml, done |
| 3 | Human is the map | Add MAP.md convention, surface in resume |
| 4 | Tool boundaries are wrong | Rethink kerf/beads split, not kerf internals |
| 5 | Wrong unit of work | Add grouping (theme/initiative) or make works smaller |
| 6 | Better list output | Enhance `kerf list` with overlap/relationship info |
| 7 | Process, not tooling | Better agent prompts at session start |
| 8 | Filesystem hits its limits | Add a lightweight query index |
| 9 | Detect, do not prevent | Build `kerf check-coherence`, not upfront coordination |
| 10 | Track spec ownership | Add `affects` field to spec.yaml |
| 11 | Premature optimization | Wait for real failures before building |

The highest-leverage ideas, in my estimation: #1 (fix session-start context), #6 (better `kerf list`), and #10 (track which specs each work affects). These three together might solve 80% of the claimed problems with minimal new infrastructure.
