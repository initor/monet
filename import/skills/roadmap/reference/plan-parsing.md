# Plan parsing

How a stream's plan markdown becomes the items array that drives the SVG. Read this from Step 2.

## Minimum table shape

The plan must contain a markdown table with, at minimum, **a named ID column and a named status
column**. Everything else is optional and stream-specific.

Formats genuinely differ across streams (envtest carries
`| ID | Item | Status | Ticket | Origin |`, PV/PVC does not use the same columns), so match on column
*names*, case-insensitively, rather than on position:

| Role | Accepted header names |
|---|---|
| ID | `ID`, `Item ID`, `Key` |
| Status | `Status`, `State` |
| Label | `Item`, `Title`, `Description`, `Summary` |
| Caption source | `Ticket`, `PR`, `Notes`, `Origin` |

No column sets the repo or the grouping. The repo for `gh` comes from the stream dir's git origin
(SKILL.md Step 3), and `groupedIds` is the agent's call (see Grouping below).

## Fail loudly

If no table carries both a recognizable ID column and a recognizable status column:

**Stop.** Report which tables were found and which columns each one had. Do not render a half-empty
page: a roadmap that silently omits half a stream is worse than no roadmap, because it reads as
authoritative.

If parsing proves brittle for a particular stream, the documented fallback is to add an explicit
items-block argument rather than to loosen the matching. That argument is not built yet: today the
only arguments are `[<stream-dir>]` and `--plan <path>`.

## The Item entity

One entity, built per plan row:

| Field | Meaning |
|---|---|
| `id` | Stable identifier, never reused or renumbered (`EG-nn`, `PVC-nn`) |
| `label` | Short text for the box |
| `lane` | Which status lane the item sits in |
| `state` | Drives the `.bx` class, muted text, and dot |
| `caption` | Free-form string. Deliberately polymorphic: `#10034 merged`, `blocked on EG-12`, `5 items`, `review P1 #7` |
| `groupedIds` | Optional. Collapses N items into one box. Set by the agent's judgment, not read from the plan |

## Relationships

Expressed as **lane membership plus prose in the caption**, never as drawn edges between items. Edge
routing is not worth the complexity at this scale, and the constraint is what keeps the layout a pure
function of lane and label.

## Grouping

`groupedIds` collapses N items into one box. The envtest page renders 19 items as 14 boxes this way.

The agent decides the groups while building the items array: collapse items that share a lane and are
plainly one piece of work (one PR, one ticket, sibling sub-tasks), and give the box a caption such as
`5 items`. No plan column drives this, so name every group formed in the run's report.

Grouping is also the intended remedy when a lane fails the capacity check in `geometry.md`: collapse
related items rather than shrinking boxes or truncating labels.
