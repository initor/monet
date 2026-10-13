# Roadmap geometry

Every number below is **computed**, never authored. Hand-placing coordinates is the failure mode this
skill exists to remove: the previous pages clipped content silently when a lane grew.

Values verified against a reference stream's `roadmap.html` (5 lanes, `viewBox="0 0 900 590"`).

## Canvas

```
width          = 900          # fixed
first_box_x    = 110          # trunk occupies 0..110
right_margin   = 58           # observed: last box right edge 842
lane_capacity  = 900 - 110 - 58 = 732
```

## Lane geometry

Uniform pitch. A hand-made page may use an irregular hand-tuned pitch (160/120/114/102); that is **not**
reproduced (see "Accepted visual break" below).

```
pitch          = 104
lane_y(i)      = 92 + 104*i        # text baseline reference
rect_y(i)      = lane_y(i) - 22    # i=0 -> 70, i=1 -> 174, i=2 -> 278  (verified)
rect_height    = 44
rect_rx        = 8
```

## viewBox height

```
H = 104 * lanes + 70
```

Derivation: last rect bottom = `70 + 104*(lanes-1) + 44`; bottom pad = 60 measured **from that edge**.

Worked example, 5 lanes: last rect bottom = 70 + 416 + 44 = 530 → H = 590. Matches the reference page exactly.

> **Trap.** The `pr` caption baseline for the last lane sits at 544, *inside* the 60px pad, 14px below
> the rect and 46px clear of the bottom. Measuring the pad from the caption instead of from the rect
> adds 46px of dead space to every page.

## Text baselines, relative to `lane_y(i)`

```
id text y      = lane_y(i) - 2
sub text y     = lane_y(i) + 13
pr text y      = lane_y(i) + 36
dot cx         = rect_right - 12
dot cy         = lane_y(i) - 10
text pad       = 12             # each side
```

## Box width

Computed from label length. Hand-fitted widths in the existing pages range 140–230px and carry slack;
they are not reproduced, because the first reuse with a longer label overflows silently (no
`textLength`, no `foreignObject`, no wrapping).

Font advances, from the `.dg` CSS:

| Class | Font | Size | Advance factor |
|---|---|---|---|
| `.id` | `--font-mono` | 13px | 0.62 |
| `.sub` | `--font-text` | 10px | 0.58 |
| `.pr` | `--font-mono` | 9.5px | 0.62 |

```
w_id   = len(id)    * 13   * 0.62 + 20     # +20 clears the status dot at rect_right-12
w_sub  = len(label) * 10   * 0.58
w_pr   = len(caption) * 9.5 * 0.62
width  = max(w_id, w_sub, w_pr, 140) + 24  # 24 = text pad both sides; 140 = floor
```

The factors are deliberate upper bounds for mixed-case text, so computed widths land at or just above
the hand-fitted ones and never under. Validation against lane 0 of the reference page:

| Label | chars | computed | actual | Δ |
|---|---|---|---|---|
| `cache eviction / warm start` | 27 | 181 | 178 | +3 |
| `Request Retry budget rules` | 26 | 175 | 178 | −3 |
| `de-flake removal test` | 21 | 146 | 158 | −12 |
| `paused reconcile path` | 21 | 146 | 158 | −12 |

## Horizontal packing

Gap between boxes in a lane is **defined, not recovered**: the reference page uses 20/22/12/12 across
its four lanes, hand-tuned per lane, so there is no formula to recover.

```
gap = 20
x(0) = 110
x(n) = x(n-1) + width(n-1) + 20
```

**Capacity check, before emitting.** For each lane:

```
sum(widths) + 20 * (count - 1) <= 732
```

On failure, **stop and report** the offending lane, its computed total, and the overflow in px. Do not
shrink boxes below the 140 floor and do not silently truncate labels. The intended remedy is the
`groupedIds` field, which the agent sets by judgment (`plan-parsing.md`, Grouping) and which collapses
N items into one box: the reference page renders 19 items as 14 boxes this way.

This check is the horizontal counterpart of the computed `viewBox`: together they make both clipping
axes loud instead of silent.

## Status enum

Declared per stream, not hardcoded. Shape:

```
{ className, fill, stroke, strokeWidth, dash?, mutedText: bool, dot?: 'ok' | 'live' | none }
```

`done`, `now`, `later` ship in the template. Extra states (e.g. a `gate` state) are emitted into the
`{{EXTRA_STATE_RULES}}` placeholder as `.dg .bx.<name>{...}` rules.

Dot semantics are fixed and not overridable: `.ok` green = merged, `.live` blue = open.

## Accepted visual break

Regenerating an existing page changes how it looks. Most visible for a hand-tuned page, whose irregular
lane pitch becomes uniform 104. This is intended and was accepted in the spec. Flag it to the user before
every overwrite of a page that already exists, whether it was hand-authored or generated.
