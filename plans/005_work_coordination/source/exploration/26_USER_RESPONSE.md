doc 25 looks AWESOME!

> ## 3. The Agents
> ### The Four Types
I think it'll be fine - but for now - or without harmonik then most likely the ALLOCATE and MERGE are done in the same agent. EXEC, TEST would be done by subagents within
that main agent.
This is important to differentiate.
Its also important because there may be instructions that would be useful to give it. For example, many agents have taken the beads and written a whole thing to pass to
the sub agents to start their task. That shouldn't be needed - should just need to pass instructions and a bead id.

> It reads `kerf next` and dispatches.
What is kerf next going to show? When something starts to execute the bead, we'll need to make sure that its set to in_progress I assume - then it wont show up in 'kerf
next'??
Something to think about: theoretically - with harmonic we could have it read kerf next and take the tasks and programatically start/allocate them, then set them to
in_progress. If an agent is doing it - meaning reading from kerf next then dispatching to harmonic to dispatch. Does next filter out in_progress? I guess if an agent knows
it just put the item in progress and it for some reason reads and its not in progress yet - the agent is probably smart enough to figure things out.
I just want to make sure we've thought this through.
Seems like it'll be up to the dispatcher/allocater to actually set the bead into in_progress though - I don't think the agents have been doing that.

> the hardest seam, because PLANNING is sporadic (user-driven) while findings may be urgent.
This is an interesting point and one I want to 'feel' to see whats going on. What I'm wondering is if there are issues in EXECUTE or MERGE/TEST and we have a mechanism to
feed that back in. There are probably a couple ways.
1) the merge/test could just create new tasks and send them to ALLOCATE/kerf next - then the next items taken into the queue will be those.
2) If there are larger issues to be dealt with/a design is needed, then maybe in the 'handoff' process - or something like it - merge/test would save something that could
then be revealed at a place where planning is being done.
Now that you say that " PLANNING is sporadic", you're right, there may be a very different workflow than I'm used to. But thats ok - I think we should figure out how those
issues will be raised in the planning stage and we can figure out how to smooth out any issues later on.

> But no one knows until they next look at the blackboard.
Yes that is an issue, but I want to see how we work through that. I think the most difficult thing is going to be related to the agents and making sure they can and do pick up work - but thats a totally different problem.
In all the systems I've seen so far, this will be an issue - the delay. If an agent is in the middle of things, then something may have to wait until its complete.


> 1. Batch: entity or ephemeral grouping?
  I dont really know. What I do know is that when there are 5 tasks that need to get done, I want to make sure nothing gets lost - so once the 4 is completed, how do we make sure the last one isn't just left stranded and something from another area is prioritized higher.

> 2. The ALLOCATE agent: agent or script?
yes and yes. lol - for now I dont want to build some complicated graph system - but over time we'll want something that'll help us to deterministically help arrange the graph. There will also likely be an agent that is used manage the work. Basically - we need the basics, then dont worry about it becuase its not in kerfs perview.

> 3. Bead lifecycle for verification.
Durring execution there will be review and test phases to ensure that bad changes dont get through.
The merge/test will probably mostly be merges. I'm kind of thinking that it could kick off subagents to do testing of a specific commit - lets say every 10 tasks, or after a batch. If there is an issue the commit will stay, but it can communcate to the allocator (indirectly) that there is an issue that needs to be resolved.

> 4. The fast path vs. spec-first principle.
Not concerned about this. The agent instructions can be tweaked to help enforce this.

> 5. Area reservation during concurrent planning.
  I dont think I'm too concerned with this.

> 6. How does "rework before new work" interact with heijunka?
Not sure how we should signal the priority - but downsteam issues/rework should be prioritized over accepting new tasks coming from upstream.
