# Naming: kerf

## Decision

**`kerf`** — *Measure twice, cut once.*

The kerf is the width of material removed by a saw cut. If you don't account for it, nothing fits together. It's the gap between "measured" and "built" — the thing you must get right before anyone picks up a tool.

That's what this tool does: it makes sure the cut is clean before implementation starts.

### Why kerf works

- **4 characters**, one syllable, hard consonants — fast to type, distinct in a terminal
- Nobody confuses it with project management or ticket tracking
- The metaphor has *consequence*: skip the kerf and the project doesn't fit together
- It's a real word, uncommon enough to own, but immediately explainable
- Workshop word, not a corporate word. Devs will respect it.
- Works as a verb: "Did you kerf that?" → "Did you spec that out properly?"

### Availability

| Registry | Available |
|----------|-----------|
| npm | Yes |
| PyPI | Yes |
| crates.io | Yes |
| Homebrew | Yes |
| GitHub (exact) | Taken (unrelated — a columnar database by kevinlawler) |

GitHub namespace isn't a blocker — the org/repo would be `kerf-tool/kerf` or similar. Distribution channels are all clear.

## Vocabulary

The woodworking metaphor extends naturally to the tool's concepts without being forced.

| Concept | Term | Metaphor | Example |
|---------|------|----------|---------|
| The tool | **kerf** | The critical cut | `kerf new "auth redesign"` |
| Workspace | **bench** (`~/.kerf/`) | The workbench | "You've got 3 works on the bench" |
| A spec | **work** | The workpiece | "Resume that work" |
| Templates (feature, bug) | **jigs** | Repeatable guides for precise cuts | "Use the feature jig" |
| Phases within a jig | **passes** | Rough cut → fine cut | "You're on pass 2 of 4" |
| Verification / done-check | **square** | Hold a square to the piece — is it true? | "Check it's square" / "It's square" |

### Command feel

```
kerf new "user auth redesign"        # start a new work
kerf list                            # what's on the bench
kerf status auth-redesign            # where are we in the passes
kerf square auth-redesign            # check if it's square
kerf resume auth-redesign            # pick it back up
kerf archive auth-redesign           # off the bench, into storage
```

### Phrases that work naturally

- "You've got a work on the bench"
- "Run it through the feature jig"
- "You're on the rough cut pass"
- "Is it square?"
- "Measure twice, cut once"

## How we got here

Explored themes across blacksmithing, pottery, weaving, jewelry, nautical, gardening, bonsai, watchmaking, engraving, optics, archery, and woodworking. The criteria that narrowed it:

1. **"Fine work"** — small things at the beginning make huge differences in outcome
2. **Consequence** — skipping this step means things don't fit later
3. **CLI-friendly** — short, typeable, memorable
4. **Not precious** — a workshop word, not a design-school word

Other strong finalists:
- **jig** — became the term for templates instead
- **graft** — strong metaphor (joining things so they grow as one) but less CLI-natural
- **jin** — bonsai term for intentionally shaped deadwood; beautiful but too obscure
- **nock** — archery, the placement before the release; cool but niche
- **baste** — temporary stitches guiding final work; best direct metaphor but odd as a CLI name
- **square** — became the term for verification instead

## Tagline

*Measure twice, cut once.*
