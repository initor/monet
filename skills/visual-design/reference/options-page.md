# The options page (`iterate.html`)

One page holds every round. The owner reads it top to bottom and answers with an ID.

## Top to bottom

1. **Where we are**: the open round, what it explores, and the one question for the owner.
2. **Decided so far**: one line per decision in `brief.md`, with its values when it has any (a swatch, a font name, a size).
3. **Glance strip**: one small cell per option of the open round, each linking to its card.
4. **Cards**: the open round first, then the earlier rounds, newest first. Earlier rounds stay on the page.

## IDs

Every option gets a short ID that never changes and is never reused: a letter for the facet the round explores (C colors, W a word, M a mark, F a figure, P a page), a number in order, and a take letter for variants of one idea. C61B is colors, option 61, take B. W14D is the word, option 14, take D. To compare against an earlier option, show it again as a reference card under its own ID.

## The card

Each card shows, in this order:

1. The ID in bold, then the option's name.
2. The artifact, large.
3. The artifact in every real context from the brief, at real sizes. For an emoji, the chat client's light and dark themes at 16, 22, and 32 px. For a page, its desktop and 375 px views, light and dark. For a figure, its stage at rest and in motion. A context wider than the card, such as a desktop page, is shown scaled down and links to the real thing at full size.
4. One line: the idea behind it.
5. Chips: each value it sets, such as a color, a font, or a size, with the reason for that value as the chip's tooltip.
6. Flags: plain facts from the harness that the owner should weigh. Examples: a contrast under 4.5:1, two colors that merge for protanopia, a likeness to a stock icon.

A card holds facts only. Panel scores live in the round notes, never on the page, so no card carries a score, a rank, a star, or "recommended".

## Building the page

- Generate the page with a script in `tools/` from one data file, for example `rounds.json`: rounds, options, values, reasons, and harness results. Every round then renders the same way.
- Build it on `paint`'s base and its `minimal` flavor (`${CLAUDE_PLUGIN_ROOT}/skills/paint/reference/base.md`, `${CLAUDE_PLUGIN_ROOT}/skills/paint/reference/flavors/minimal.md`): self-contained, with light and dark themes.
- Card previews are the harness's real renders, never hand-drawn stand-ins. Scale each render to the size it shows at. A page of several MB is fine, since it stays local.
- Keep each build of the page in `history/`.
