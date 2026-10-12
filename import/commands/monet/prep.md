---
name: monet:prep
description: Practice interviews -- system design, behavioral, or coding mocks with Staff+ evaluation. Or view your progress dashboard.
argument-hint: "[sd|bq|code|progress] [mode] [topic]"
allowed-tools:
  - Read
  - Bash
  - Glob
  - Grep
  - Write
  - AskUserQuestion
  - Skill
---
<objective>
Run the prep skill to route to the appropriate interview practice skill based on domain and mode arguments.
</objective>

<execution_context>
@~/.claude/skills/prep/SKILL.md
</execution_context>

<context>
Arguments: $ARGUMENTS

Positional argument parsing:
- First argument: domain (sd, bq, code, progress)
- Second argument (optional): mode (mock, drill, story-bank, etc.)
- Third argument (optional): topic (passed through verbatim to downstream skill)

If no arguments are provided, the skill handles interactive selection via AskUserQuestion.
</context>

<process>
## Step 1: Parse arguments

Read `$ARGUMENTS` as positional inputs (`<domain> [mode] [topic]`). No flags or tool-auth to check. Pass them through unchanged; empty arguments are valid (the skill handles interactive selection).

## Step 2: Execute the prep skill

Read and execute the @-included SKILL.md exactly as written -- it is the single source of truth for routing, aliases, interactive selection, and delegation.

## Step 3: Report

The downstream skill handles all further interaction. Do not add steps after delegation.
</process>
