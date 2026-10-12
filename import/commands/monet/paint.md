---
name: monet:paint
description: Visualize session research, analysis, or data as a self-contained, theme-aware HTML page, with the design flavor (showcase / minimal / technical) auto-selected to fit the content. Pass --flavor to override. Pass --sheet for one dense image with a PNG, or --explainer to teach one concept with a Hairline figure.
argument-hint: "[topic or focus] [--flavor showcase|minimal|technical] [--sheet | --explainer]"
allowed-tools:
  - Read
  - Bash
  - Write
  - AskUserQuestion
---
<objective>
Run the paint skill to turn the session's research/analysis/data into one self-contained HTML page, in the design flavor that best fits the content.
</objective>

<execution_context>
@~/.claude/skills/paint/SKILL.md
</execution_context>

<context>
Arguments: $ARGUMENTS

- A topic/focus guides which session content to prioritize and how to title the page; if empty, infer the dominant content.
- `--flavor showcase|minimal|technical` forces a flavor; otherwise the skill auto-selects from the content and intent.
- `--sheet` switches to the sheet layout: one fixed canvas of lettered panels, written as HTML and PNG (the skill's `reference/sheet.md`).
- `--explainer` switches to the explainer layout: one Hairline figure with plain words and numbered steps, written as HTML and PNG (the skill's `reference/explainer.md`).
- If there's no visualizable content, the skill asks what to paint.
</context>

<process>
1. Parse `$ARGUMENTS` for an optional topic/focus, an optional `--flavor` override, and an optional `--sheet` or `--explainer`.
2. Read the paint SKILL.md referenced above and execute it exactly as written: all steps and constraints. It is the single source of truth; don't restate its steps here.
3. Report the filename (and the PNG for a sheet or an explainer), the section, panel, or step count, and the flavor that was used.
</process>
