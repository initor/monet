---
name: paint
description: Use when the user asks to visualize, "paint", or turn session research/analysis/data into a shareable self-contained HTML page, or wants a visual summary of findings. With --sheet, it makes one dense image instead, as HTML plus a PNG. With --explainer, it teaches one concept to a newcomer, with one Hairline figure and plain words. For a visual the owner picks by eye in rounds, use visual-design.
---

## Overview

Turn the session's findings into **one self-contained, honest, theme-aware HTML page**. The structure and components are fixed; the visual *voice* is a **flavor** chosen to fit the content: engineering write-ups shouldn't look like product launches. Every number on the page comes from the session. Never invent data.

## Prose voice

Distinct from the **flavor voice** in Step 2, which is visual. This one is about the words.

A painted page gets shared, so its text goes out under Wayne's name and runs through `voice`. Read that file in full, as the instruction below says, rather than taking it as a mere reference, because holding a rule is not the same as applying it. It covers section headings, narrative copy, callouts, metric labels, and chart captions.

Register for this surface: **paint page copy**. An explainer's words follow the STE-lite card instead, as `reference/explainer.md` says. Proper capitalization, fully formed sentences in narrative sections. Not chat. Fragments are fine in labels and metric captions, where they are the convention rather than a shortcut.

Two invariants carry most of the weight here:

- **Ground every claim.** The Constraints below already forbid fabricating numbers. The same rule applies to prose assertions, which slip in more easily because nothing about them looks numeric.
- **No cute generated phrasing.** A visual page is exactly where a model reaches for punchy headline copy. A section heading states what the section shows. It is not a tagline.

Read `../voice/SKILL.md` relative to this skill directory before writing prose.
Resolve all `reference/` paths against this skill directory, not the task's
working directory. If the client does not expand `@` includes, read referenced
files explicitly using its available file-reading tool.

## When to use / not

- Use when asked to visualize, paint, or make a page/report out of what's in the session.
- Use `--explainer` to teach one concept to a newcomer, for a getting-started guide or an example.
- Not for a single fact or number: suggest a sentence instead and confirm before generating a full page.

## Step 1: Harvest the session

Scan the conversation and pull out: key insights (3–5), quantifiable data points, relationships (comparisons, hierarchies, flows, timelines), categories, and supporting detail (quotes, code). Organize into an outline before designing. If there's nothing visualizable, ask what to paint.

## Step 2: Choose the flavor (automatic)

**Layout.** The default is a **page** of scrolling sections. Switch to a **sheet** only when the request asks for one: `--sheet`, "one sheet", "sheet this", or "one image I can paste". A sheet is one fixed canvas of lettered panels with a PNG export, and `reference/sheet.md` holds its rules. Switch to an **explainer** only when the request asks for one: `--explainer`, "explain this to a newcomer", or "a picture explainer". An explainer is one Hairline figure with plain words and numbered steps, and `reference/explainer.md` holds its rules. The flavor below still sets the tokens for every layout.

Pick the flavor that fits the content and intent. Don't ask. Honor an explicit `--flavor <name>` or a natural-language cue ("keep it minimal", "make it pop" → showcase) when present.

| Signals in the content / request | Flavor |
|---|---|
| RCA / postmortem, debugging writeup, runbook, getting-started or how-to guide, architecture/design explainer, educational material | **technical** |
| Exec summary, results/benchmarks to present, comparison or recommendation you're "selling", launch recap, "make it impressive" | **showcase** |
| Plain notes, single-topic summary/reference, "keep it simple/clean", **or anything ambiguous** | **minimal** (default) |

Read `reference/flavors/<flavor>.md` for that flavor's tokens, rules, and *visual* voice.

## Step 3: Design the page

For a sheet, choose 4 to 6 panels instead, per `reference/sheet.md`. For an explainer, follow the order of work in `reference/explainer.md`: the words, then the figure, then the page. For a page, choose 4–8 sections, one idea each, building a story (overview → detail → conclusion). Alternate data-heavy and narrative sections. Pick the visualization per section from the palette below, and honor the chosen flavor's *visual* voice (e.g. technical favors tables/code/flow diagrams over decorative charts). For the words in those sections, see Prose voice above.

| Pattern | Use for |
|---|---|
| Metric cards | 3–6 headline numbers |
| Horizontal bar chart | ranked comparison (5–15 items) |
| Donut | part-of-whole (2–6 segments) |
| Comparison table | option/feature matrix |
| Flow diagram (inline SVG) | process, pipeline, architecture, RCA causal chain |
| Timeline | chronology (incident timeline, evolution) |
| Code block | code, config, commands |
| Callout | key takeaway, warning, or root cause |
| Card grid | categorized items |
| Copy panel | copy-paste payloads (drafted review comments, prompts, commands): body stays raw text with a byte-for-byte copy button, never rendered as markdown |
| Explorer | a what-if over session data: inputs in the base `.explorer` component and a result that the page computes with inline JS, with no network |

## Step 4: Assemble the HTML

Read `reference/base.md` for the skeleton, component CSS, and theme/reveal JS. Build a single self-contained file: inline the base CSS, then the chosen flavor's token block **after** it (so flavor wins), then the content. Set `<html data-anim>` to the flavor's default. No external dependencies: it must work offline. For a sheet, build from the skeleton in `reference/sheet.md` instead of sections, with `data-anim="off"`. For an explainer, build from the skeleton in `reference/explainer.md`, with `data-anim="off"`, then fill its scripts with `scripts/explainer.mjs`.

## Step 5: Write and open

1. Use the output directory already specified by the user, otherwise the current
   working directory. If a location decision is needed, ask with the client's
   question tool or plain text.
2. Filename: `{topic-slug}-{YYYY-MM-DD}.html` (slug <=40 chars; if that file exists,
   append `-2`, `-3`, ...).
3. Write with the client's available file-writing tool. For a page on macOS, open it
   with `open "<path>"` after writing it, unless the user asked you not to or the
   session has no display. If opening is unavailable, return the absolute path.
   Follow the host's delivery and publication requirements using tools actually
   available there; generating HTML does not require an Artifact tool or remote
   hosting.
4. For a sheet, do not open the HTML. Export and check the PNG as `reference/sheet.md`
   describes. Then open the PNG once, with the same exceptions as a page.
   For an explainer, run the check and export in `reference/explainer.md`, then open the HTML.
5. Confirm tersely. A sheet uses the second form, and an explainer the third. An explainer adds a
   line for anything it could not verify, such as a figure not looked at in a browser:
   ```
   Painted: {filename}  ·  flavor: {flavor}
   {N} sections, {M} visualizations · light + dark theme

   Painted: {name}.html + {name}.png  ·  flavor: {flavor}
   {N} panels · PNG 3200 by 1800, light theme

   Painted: {name}.html + {name}.png  ·  flavor: {flavor}
   one figure, {N} steps · PNG 2400 wide, light theme
   ```

## Constraints

- Fully self-contained: inline CSS/JS, no external requests, no analytics/tracking/cookies. Works offline.
- Every data point comes from the session. Never fabricate numbers, stats, or findings.
- All colour/typography via the CSS custom properties defined by base + flavor. No hardcoded colours outside those token blocks. An explainer's figure is the exception: it takes its colours from the Hairline kernel, which the page inlines unchanged.
- <3 data points for a chart → fall back to metric cards or a card grid.
- Visuals use inline SVG, CSS, or tables. **Never ASCII/box-art**, and no emoji in visual elements (renders inconsistently).
- Responsive at 1440 / 1024 / 375px. A sheet meets this by scaling its fixed canvas to the window. Keep generated HTML under ~3000 lines.

## Common mistakes

- Painting an RCA or runbook in the **showcase** flavor. Match the flavor to the content (Step 2).
- Fabricating or rounding numbers the session didn't establish.
- ASCII/box-art diagrams, or a chart built from fewer than 3 data points.
- A 70vh marketing hero on technical content (technical/minimal use a compact title block).
- Prettifying a copy panel's payload: rendering its markdown, styling `​```suggestion` fences into diff boxes, or linkifying bare URLs. The payload is verbatim text; what the user sees is exactly what the copy button puts on the clipboard.
