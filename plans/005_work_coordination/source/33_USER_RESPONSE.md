Responding to doc 27


> ### 1. Intent replaces "work" as the planning unit
First I've been calling them 'work item' - instead of 'work' cause it makes more sense.
It kind of feels like were going to rename a bunch of stuff so we can change work to intent. I'd rather not do that. Also since a very large work item could be literally
hundreds of tasks - it seems odd to call it one 'intent'.
I dont want to change the name just to change the name - thats dumb.
If its the same thing just a different name...

> ### 2. Area is a first-class entity kerf manages
I dont understand the question. "Does kerf own and maintain a system map of Areas" - who else would?
The idea this planning process came up with was to use areas to name different parts of the code base to figure out overlap. If the idea exists in here - how would another system own it??

> ### 3. Finding is a first-class entity with a structured ingestion path
> Does kerf provide a structured way for downstream agents (EXEC, MERGE/TEST) to record findings that flow back into the planning cycle?

Beads I assume. Maybe those beads get tagged a particular way or something so the agent picks them up or something.

> ### 4. Queue is a computed view, not stored state
> Is `kerf next` a live computation over current task states, dependencies, and priority signals — rather than a stored, manually-ordered list?

Seems like it will need to be a live computed list. It'll most likely come from beads ordered a particular way.

> ### 5. Design freeze is the commitment boundary
> Once tasks are generated from a design, is the design frozen

I dont really want to go modifying documents too much - but 
> immutability guarantee
Don't take it that strongly - it was so that an agent wouldnt just modify the same thing over and over and fuckup the data. The intent was to be forward progressing - leaving a footprint in the sand - that could be inspected later and understood.

Maybe I'd say - if the work has been completed - then it probably shouldn't be modified. If none of the work has been completed - then I'm not sure what to say. If theres just an issue in the tasks then just fix that. If theres a fundamental issue that requires going back to the planning or something - then I dunno what to say - maybe you do that? I haven't really needed to - we've usually just said heres a new requirement.

Please stop trying to generate laws. You say these things that might need to change and then later on some agent parrots that shit and wont do its fucking job because of some stupid fucking policy it made up. 
I don't know what this looks like exactly. 


> ### 6. kerf reads bead status but does not own it
> Does kerf query the beads system (bd) for task completion status

Yes thats what I'm thinking right now.


> ### 7. ALLOCATE and MERGE/TEST collapse into one agent (for now)
> This affects what kerf needs to surface.

IMPORTANT: SCRUB THIS ASSUMPTION FROM WHERE EVER IT CAME FROM!!!! (Some agent interpreted my statement very incorrectly - I dont want to have this discussion again.)

> most likely the ALLOCATE and MERGE are done in the same agent.
GOD DAMN IT - THAT IS TAKEN OUT OF CONTEXT. I DID NOT SAY THAT.

I said that if harmonik was not used by the user - then all the work would be done by one agent.
This tool should not be concerned with whether its one agent or many - the interface should be the same. If we need to add more things, then lets do that.


> ### 8. Rework priority is structural, not labeled
> does rework win by default

Yes very likely it needs to. WE'll probably need to be able to configure those rules - whatever they are /are going to be.

This is actually pretty important - when we actually build things out - were going to want to be able to change the parameters/way that the ordering is determined.
There may be some things that will be harder than others to parameterize. Thats fine - then hard code it, but try and make it very obvious, and we probably want the whole algo in one place.


> ### 9. Batch is ephemeral — kerf does not track dispatch history

FOR NOW - lets make it "batches are ephemeral". 
AH! Idea: there are two tasks each in different epics (a beads thing). Bead A is in an epic where 10 beads are complete. Bead B is in an epic where there are 8 beads to deploy. Bead A is deployed first because the other 10 beads are 'pulling' that bead through - meaning theres some mechanism that exherts pressure on dispatching that bead first essentially so that the whole epic could be completed.
Seems like the dependencies will be filters (what can and cant be done), a bead that blocks a bunch of work has a high weight so is ordered sooner, in progress epics generally have higher priority (but blockers need to be considered).

All this ordering will need to be figured out - but we'll do that later. We can use real world examples of work to find the optimal queueing patterns.
