---
name: monet:roadmap
description: Regenerate a work stream's living roadmap.html from its plan markdown, with computed SVG geometry so lanes can't clip silently. Requires --plan. Not for general visualization. Use monet:paint for that.
argument-hint: "[stream-dir] --plan <path>"
allowed-tools:
  - Read
  - Bash
  - Write
  - AskUserQuestion
---
<objective>
Run the roadmap skill to regenerate one work stream's `roadmap.html` from its plan markdown plus live `gh` verification, with all SVG geometry computed rather than hand-placed.
</objective>

<execution_context>
@~/.claude/skills/roadmap/SKILL.md
</execution_context>

<context>
Arguments: $ARGUMENTS

- The first positional argument is `<stream-dir>`: the directory that holds, or will hold, `roadmap.html`. Defaults to the current working directory.
- `--plan <path>` is required and is never inferred; the existing streams place their plan files differently, so guessing fails silently on one of them. If it's absent, the skill asks for it.
- The page is undated and overwritten in place, which deliberately overrides paint's dated-filename convention. When `roadmap.html` already exists, the skill warns before every overwrite.
- There is no `--repo` argument: the skill infers `<owner/repo>` from the stream dir's git origin, and decides any item grouping itself.
</context>

<process>
1. Parse `$ARGUMENTS` for the optional `<stream-dir>` and the required `--plan <path>`.
2. Read the roadmap SKILL.md referenced above and execute it exactly as written: all Steps, Constraints, and Common mistakes. It is the single source of truth; don't restate or modify its steps here.
3. The skill computes the `viewBox` height and every box width from `reference/geometry.md`, and runs a per-lane capacity check. Do not hand-place a coordinate or shrink a box to make content fit. Overflow is meant to stop the run.
4. Report item count, box count, lane count, the computed `viewBox`, which items were grouped, and which PRs were re-verified.
</process>
