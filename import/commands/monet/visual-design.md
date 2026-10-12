---
name: monet:visual-design
description: Design something visual in rounds, such as a logo, a figure, or a page, from options I pick by eye to a verified, shipped result.
argument-hint: "[what to design, for whom] [work folder to resume]"
allowed-tools:
  - Read
  - Write
  - Edit
  - Bash
  - Glob
  - Grep
  - Agent
  - Skill
  - AskUserQuestion
---
<objective>
Run the visual-design skill to design one visual artifact with me in rounds: options by ID on one page, shown where they will be used, my pick by eye, refinement, a real-browser check, and a ship only when I say go.
</objective>

<execution_context>
@~/.claude/skills/visual-design/SKILL.md
</execution_context>

<context>
Arguments: $ARGUMENTS

- The arguments name what to design and for whom: a logo for a team, a figure for a page, an overview page for a system. If they are empty, the skill asks.
- A work folder named in the arguments resumes that design instead of starting a new one.
</context>

<process>
1. Parse `$ARGUMENTS` for what to design, and for a work folder to resume.
2. Read the visual-design SKILL.md referenced above and execute it exactly as written, all steps, gates, and constraints. It is the single source of truth; don't restate its steps here.
3. Report the work folder, the round and its option IDs, and what waits on me.
</process>
