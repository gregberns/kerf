

Doc 19 has a structureal problem - you jump all over the place! We need to think of this as a systems problem first, then decompose the system into parts and discuss those.

IMPORTANT: Seems like we need to figure out what the domain's ideas are (without feeling too bound to existing ideas - like work items) - and how those flow through AND AROUND the system - because there ARE feedback loops.

Reminder: again - do not go modify the previous docs - lets generate new if there is something we need to discuss.

# New Context: Sessions

This additional context may be needed to understand some things. CAVEAT: I DO NOT want you planning this out - I'm telling you because the longer term vision is unclear - but there are some general ideas.

My guess is that a user may be working in one or more sessions planning items.
There may be another session where a user is testing functionality that has landed - issues need to be fed back through the system.
There MAY also be two other agents:
* ALLOCATE: One agent that is actually delegating work - so taking items that need to be done and dispatching them. This could be an agent, or it could be harmonik.
* MERGE: There also may be an agent that is actually merging and/or testing - so checking functionality and merging it in based on what is found.

These last two agents may be running in constant loops - specialized coding agents - but that is TBD.

There may be little or no direct communication between them. I think this is where kerf can come in.

Lets say the merge/test agent finds something that needs to get fed back into the system - lets say it was able to put something into the kerf system that would fast track a plan, task, implement process. 
Maybe then kerf could surface that and the ALLOCATE agent could take it and prioritize it through the process.

We need to be cautious about 'sessions'. I don't think we are aligned on what those are and what they mean. As I read through the docs, your 'sessions' and mine don't seem to align.



## 2. Resolved Contradictions

Why do I need to look at the resolved contradictions first? Don't I need to understand the full vision first? Then we can dig into why those decisions were made?- or alternatives??

When reading through these MASSIVE docs - I have to start at the top and read. As I read I need to think through what you are presenting AND write down those thoughts so you understand. If I dont do that, you wont have the context of why I'm thinking the way I am and you cant make as good decisions.


### 2.1 Session Records: per-work SESSION.md vs. per-session YAML files

> session records
I think there may be some disconnect between your and my definition of a session. When I was discussing sessions, they were execution sessions. So an agent is working through 10-20 beads in a session and runs into challenges, completes work, etc. We seem to be tying a session to a work item - which I'm not crazy about.

Lets say harmonik doesn't exist - instead we're working through the process of building it. You have specs, tasks, and beads. Lets assume your going to have an agent that is going to figure out whats next, implement a bunch of beads, then say what all has been done (and mark the beads as completed) - and then a new agent is started and the process repeats.

The unit of execution is the bead. You probably want to group those beads together to 1) think coherently about them and 2) ensure ordering and coherence and 3) focus on getting an area to get it fully working before moving on.

In what you discussed, it feels like you almost need to dedicate an agent session to a work item. But when you create a work item you're thinking in one context. An hour later you may be thinking in another context and thinking about work item #2.

Why does execution have or need to have anything to do with how those ideas were generated?

Bugs are a good example. You're implementing in a context and all of a sudden you find a bug but need to document, create a bead, and then restart the session. What happens when the bug has to do with multiple works??

The work items were just something we made up. It doesn't mean they should remain the center of the domain - or should be used when deciding the order of beads or group of beads to work on. (I think thats what I have an issue with.)

The process used in kerf (plan, spec, task) was primarily built because agents would just overwrite the same files over and over and there was no consistent structure. So the file structure kerf builds is supposed to be immutible - the output is a set of tasks/beads. KEY: That doesn't mean that 1) the work item is fully complete 2) that work should all be executed together. There are probably also many other reasons.


### 3.1 The Factory Line — Real Stations

> jig passes over a few sessions
There are two workflows, generally.
1) A large planning session is done either at the beginning of the project, or a large subsystem is being thought through. The first large planning session might just be one very large work item. The planning may take some time. Then the spec and task build out may take more sessions - especially if theres a lot of tasks to review.
2) When were iterating - meaning stuff is built but its not fully implemented, there are issues, or changes need to be made - then the "Think through whats wrong (plan), spec out changes, task creation" process is much smaller and might even just be sent to a sub agent to do most/all of that if we know what the issue is. Then that chunk of work needs to get deployed to agents to be implemented. This all might be done in a single session.

> but a single work doesn't move smoothly through them
Agreed

> Don't model "which station is this work at" as a first-class concept.
Agreed - as discussed above, the concept of a work item moving through the system seems a little problematic.

> When execution reveals a spec problem, the work goes back to design.
I'm finding that the spec may not have been defined properly, or previous implementors didn't actually wire critical things together. So we need to pass that back through the system to have the changes planned, tasked, and implemented - and I'm finding that after that there are still more problems.

Those problems: lets say once implemented, theres a bead to have an agent do a full test. If that test failes and/or there are fixes needed, then we almost immediately need to get those beads created, implemented and to go through another test.

> kerf needs to support these backward movements
I'd be cautious in saying thats 'backwards movement' - the system's graph has loop backs - where downstream findings need to be looped back to upstream processes so the tasks can flow through. (Feedback loops - probably)


### 3.3 Priority Model — Honest About Sparse Graphs

> three-dimension decomposition 
That terminology sounds pretty good.

> kerf next algorithm:
I think this is going to be really important. We don't need to focus on it yet - its an optimization - but will help make the system much more effective.

> 1. Filter: remove blocked, shelved, finalized, in-flight works
These are going to be key points to figure out.
* removing blocks is key to parallelization and total system throughput
* "in-flight works" - completing work or fixing issues that are found probably needs to be one of the highest priority - ex. if something isn't fully implemented, and an issue is discussed but not prioritized or processed, then the issue remains and may prevent the system from properly functioning. Generally - if there is a gap in functionality and the system is working on that area, those changes need to be captured and be made very high priority to get fixed up. The key is to 1) capture and 2) prioritize - and the system should identify those things and they should go through a high priority queue.

"high priority queue" - this is where we need to be thinking about feedback loops and how to get information from agents about these gaps pushed into beads and handled quickly.

This "in-flight" idea is like the 'pull' model of the Japanese assembly line - don't pull any work items forward unless there is no work.
This is really why I want to think about the work processing system, as a whole, were building. We need to focus on making sure those flows are well designed.

> `kerf pin <codename>`
Thats one way of solving.
>  Pins have a TTL: they expire after 3 sessions
I dont like this at all - kerf doesn't manage sessions or have anything to do with sessions. 

We should rethink the 'pins' idea. Look at the Japanese manufacturing practices - there will be something in there that helps. This is a prioritization issue in the broad sense.


### 3.4 Session Continuity — What Replaces HANDOFF

> Mechanism 1: Computed orientation (`kerf map` + `kerf resume`)
I'm not opposed to this - but we probably need to clarify how CORRECT information is going to feed into this.

> Mechanism 2: Per-work SESSION.md with structured append-only entries
As mentioned above - I see issues here.

It seems like we could provide a structured way for an agent(s) to provide the needed information - I just don't know what this looks like.

At this point, we may want to defer pursuing this too much until we have the broader shape of the system.


### 3.5 Agent Protocols — Essential vs. Nice-to-Have

We DO NOT want any more than 30-40 lines of instructions. Agents are MUCH better when they have a small set of instructions, then can fetch what they need from there.

You didn't define any of the information about the "full protocol stack" - so I can't comment on whatever.

But if were talking about an agent protocol for the agent dispatching work - pulling from kerf and updating as it goes - then that should be able to be pretty damn minimal. "Here's the work. If you need to look back do this. When your done do this."
All of the other instructions will be baked into the kerf cli's help menus and the return body's of the payloads.

Example: if there are high priority items that need to be done, then we tell it to 'kerf next' which will display the top 'n' items, and the first in the list will be those priority items.

> 3 escalate to user
I want to be really careful of adding what does/does not need to be raised up. This system shouldn't define that in any way - either we can provide a way for the user to define it or they can provide a prompt to control it.
Basically - I dont want to bake anything in.

### 3.6 Beads Integration — The Interface

> **The interface between kerf and beads:**
This aligns with what I'm thinking.

> **The cross-work edge problem:**
First this will be up to the task generating agent to clarify. Second the implementation agent/system will be responsible for execution.
"Task B3 depends on Task A7" - this will happen. This is the fundamental flaw of assuming a work item will be fully completed before other work can be done. The harmonik work ran into these issues and agents will be responsible for coordinating this and laying out the dependencies.

> the orchestrator handles this by working on other beads in Work B or switching to Work A.
Yes.

## 4. What to Build — Phased

I stopped here! (Didnt read it)

I don't want to talk about what to build - we haven't defined the flow through the system.
