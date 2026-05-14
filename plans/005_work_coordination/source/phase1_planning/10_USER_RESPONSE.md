
## Beads loading and integration

* With very large specs, theres a big challenge getting tasks loaded into beads. I had the system generate a yaml file and then that could be loaded into beads - if there were dependency issues - then it could fix those up. It seemed like the yaml was easier for an agent to scan and think through as opposed to beads. I think beads might have a way to load yaml - but we need to look into how to do this. Lets assume were using the 'br' command for beads.

I dont want to force someone into using beads. But I also want complete support for them. When defining a kerf project's settings, I'd like to be able to say its going to use beads - which triggers the system to generate yaml for the tasks. Then that yaml can be loaded into the beads system.


---

# Tier 1: Minimum Viable Coordination

## A. `areas` field in spec.yaml + overlap warnings

This is pretty interesting, and seems to sync with my thinking. 
When defining the system, kerf should be definng those system areas and build that map. Then the areas that agents define need to fit into that set - or the agent needs to define the system more accutately (add areas). We need to ensure coherence over many sessions - all agents agree on the parts of the system.

The areas are kind of the system map - in a way a simplified architectural diagram. I wonder if we could have a more sophisticated graph that could represent what parts talk to each other. 

## B. `kerf map` — computed portfolio view

> all active works for the current project
How do we know whats draft/active/complete? That information is in beads. (We may want to think about "Beads loading and integration" above)

Ah - right - I forgot about the status of the work items.

Generally I like the idea. The question is how to we tie the idea of work items to groups of beads to execute.
Another challenge - in the harmonik system, we designed a ton of tasks in like 6 groups. 

This is where all the tasks were defined - its a huge set.
/Users/gb/github/harmonik/docs/decompose-to-tasks
This is an example of the yaml that was created.
/Users/gb/github/harmonik/docs/decompose-to-tasks/cp-pilot-data.yaml

Whats interesting was that the yaml (line 904) defined the edges - which not only defined the relationship between one 'work item' (we didnt use kerf here) - but there was also relationships defined BETWEEN work items. We probably could leverage that to understand what work items need to be focused on first.

```
edges:
  # ───────────────────────── Intra-CP edges ─────────────────────────

  # §4.1 unified primitive — schema dependencies + intra-spec term-uses
  - {from: cp-001, to: cp-schema.control-point}
  - {from: cp-001, to: cp-schema.kind}
  - {from: cp-002, to: cp-schema.control-point}
```


#### C. Enhanced `kerf resume` with dependency and area context

We can do this, but... 

There are times when you have an idea and you just want to get it documented. So lets say you just want to create a work item and walk away.

'resume' is useful here.

But a lot of the time once I've gone through and planned it out, I just have the agent go and build out all the tasks.
Then later I start working on another idea which has over lap with an existing work item - where theres significant overlap.

I guess I'm saying that 'resume' I dont think is used a ton.

If this is small then lets do it - I'm just not sure how much we'll get out of it - but worth trying.

Related - if an agent wants a quick view into the details of the work - what if there was a summary to give a quick overview? Then it could understand the relations and purpose and understand the overlap more quickly.

## ADDITIONAL IDEA

Another idea. We've got the session-handoff and session-resume skills which use the HANDOFF document.
That document drifts over time. It also had multiple uses - it had operating instructions like "Do as much work as possible. Work independently.", but also had details about what was done and what needed to do next.

Some of those operating instructions won't be needed if the user is using harmonik for execution.
The main concern are all the small details that a next agent might need to know about.

That document also really grew very large over time and I think the bloat actually caused the agent to have issues knowing what was critical. It also drifted - it was playing the 'telephone game' and something that was minor was later interpreted as a blocker.


---

### Tier 2: Solid Foundation

#### D. `kerf next` — computed work selection

Generally I like this idea a lot. I think the challenge was discussed above - once a block of work is completed, how is the priority adjusted? What happens if X is done, while working on Y1 you find an issue Y2 and it needs to be done before Z. What is the process of adding Y2 higher in the queue to be done?

Again this is really important - I'd love to have this information (`next` output) feed into harmoniks system (via an agent probably). The idea in harmonik would be to have a large task set, then an agent add items from that set into a queue to be executed. When more work needed to be adde, `next` could be called and that information could help determine what beads to push into the queue and how dependencies were arranged.

> It ranks by graph structure, not business value.
AH! Interesting differentiation. There may be a couple ways to think about ordering (also see `## Prioritization` below, some items above I think, and maybe `STRUCTURAL PROCESS AND PROTOCOLS`):
* task X comes before Y because there is a technical dependency
* Task Y2 should be done before task Z because the user just did Y1, found an issue, and wants Y1 done so they can get that area of code working fully
* Task M and N could be done in any arbitrary order - but M is 'more valuable' to the system - based on some 'business value' or 'system value' distinction defined by the user.

This seems to be a thread we should 'pull on' further. 


#### E. `co-designs` relationship type

This is a promising idea. Once a relationship is generated, we'd want to figure out how to signal to an agent that the relationship has been investgated/addressed.
Example: work item 1 is tasked. work item 2 comes in relationship is addressed. Agent session is restarted. Next agent: How do they know that the two works should be investigated - AND, separate question, HOW/WHAT is done to syncronize those two work items? I want REALLY good guidance for the agents on how to address. There should be like a protocol or whatever so we can push agents to resolve issues like that as autonomously as possible.


#### F. Late-requirement handling via existing commands + guidance

Lol - came up with the below before I read this. I like this as a starting place.
We'd need to figure out how we get agents to read then follow a protocol, and need to give them very good rules around it.
Also, the state of the work needs to be identified. If its actively being worked, then probably dont want to stop it - might need a modification - maybe a work could be 'extended'?? Or instead theres a new work which is an extension of the previous work item. 
Lots of things to think through here.


RELATED IDEA
STRUCTURAL PROCESS AND PROTOCOLS:
When an Agent comes in fresh - what is it supposed to do? Just pull of the next items in the queue to work (or push into harmonik)? Or does it need to resolve depenencies? Or does it need to build out specs/tasks?
With the system were thinking about, its a 'manufacturing line' of work to be designed, tasked, and allocated.
We need to think about how that system works. Do we treat it like a kanban manufacturing line? If a dependency is identified, is that worked on first? What if the two dependencies aren't next in the queue? Does the first item of the queue get dispatched, then the dependencies get resolved?
If user completes testing and finds an issue, how is that tasked and prioritized ahead of other items? Especially between sessions - so user tasks out, restarts agent session, and thats the first task set to be prioritized.
It feels like we really could structure this process very well and generate strong processes/protocols around it.


### Tier 3: Ambitious

#### G. Area specs / shared design anchors

Interesting. So the idea would be to have a list of principles that each area would follow and when an agent pulled up a work with that area, those principles would be surfaced?

Not opposed to the idea. That structural pattern could be an intersting way to inject principles into the agent.

Devils Advocate
* seems like the planning process should be able to handle abiding by these principles
* principles are separated from the project code
* how do we get the agents to embed the principles initiailly? how do we prevent those file from growing too large? stagnating?

Interesting idea - but there may be some more promising pathways to focus on first.


#### H. `kerf audit` — graph invariant checks

We haven't talked about how over time old work items are archived. But that seems out of scope.

`dependency cycles` that is an interesting challenge - beads get their dependencies checked, but we dont have a way to check work item deps.

This is something to put on the lsit for later - but I wouldn't prioritize that for now. I think we need more ise of kerf and better integration/full life cycle handling - like when work is actually implemented we mark it as done, then archive or something.

#### I. WIP limits

`STRUCTURAL PROCESS AND PROTOCOLS` hints at this. harmonic also is going to have a queue that will limit the amout of active work to go through.

I dont think I want to put WIP limits in place - way too early.


## 4. Key Decisions

### Decision 1: Command naming

`kerf map` seems fine - but above I mentioned some other ideas (system map) that we probably should think through before finalizing.

### Decision 2: Area tags — freeform vs. defined taxonomy

I'd lean toward 'defined taxonomy'. We can do a bunch of graph stuff if we put that in place.
Oh - if there aren't areas in place - maybe we default to throwing an error - then the agent can define them in tan areas file, then re-execute the command.

### Decision 3: How many relationship types?

I dont know. Seems like we should think through some of the above first 

### Decision 4: Should `kerf next` exist

I'd like to think through the process as a whole - but yes I like the idea of `kerf next`. Agents need a good way to say whats up next.

We could also think about `kerf last` - which would display what was done previously.
And some type of 'handoff' mechanism where the agent could finalize. That could be a structured process or something.
And we also need to figure out how to hook all this into beads or the task tracking system.
Lets say the agent finishes 5 things. What does `kerf next` display?? How does it know what the status of tasks are?


## 5. What NOT to Build

Lets focus on what we do and dont want first. I don't want to exclude anything until we know what were going to build out.


---

## Prioritization

In harmonik, the prioritization 'P0, P1, P2, P3' seemed to become a bit of an issue.
When all of the first set of work was completed, there were a bunch of P2s that became P1s because they needed to be done to make progress on the system.

So lets say at the beginning making a core loop to work is the prority. But later once thats implemented, then gettng parallelizms working is the priority. 

Another way to view it: Today its important to priotitize X (X is P0, Y is P1). Once complete, then Y is the priority (X is done, Y is P1).

So theres dependencies between what MUST be done, but then theres also priorities which is a chain - but as items are completed that chain obviously changes, and it feels like 'P0' type numbering causes issues because that constantly needs updating.
