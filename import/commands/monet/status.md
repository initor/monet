---
name: monet:status
description: Show monet agent status, skill inventory, and configuration. Use when you want to inspect what monet can do or how it is wired.
argument-hint: "[skills|agent|all]"
allowed-tools:
  - Read
  - Bash
  - Glob
  - Grep
  - AskUserQuestion
---
<objective>
Show monet status information with optional scope selection. Displays skill inventory, agent configuration, or both depending on the argument or interactive selection.
</objective>

<context>
Scope argument: $ARGUMENTS

If $ARGUMENTS is provided, use it directly as the scope. If empty, ask the user interactively.
</context>

<process>
## Step 1: Parse arguments

Check `$ARGUMENTS` for the requested scope:
- If `$ARGUMENTS` is "skills" -- proceed to Step 3 with scope = skills
- If `$ARGUMENTS` is "agent" -- proceed to Step 3 with scope = agent
- If `$ARGUMENTS` is "all" -- proceed to Step 3 with scope = all
- If `$ARGUMENTS` is empty or not one of the above -- proceed to Step 2

## Step 2: Gather input interactively

Use AskUserQuestion to ask the user:

"What status would you like to see?"

Options:
- **skills** -- List all monet skills with descriptions and paths
- **agent** -- Show monet agent configuration and dispatch routing
- **all** -- Show everything

Use the response as the scope for Step 3.

## Step 3: Execute based on scope

### If scope includes "skills" or "all":

Read each `~/.claude/skills/*/SKILL.md` file:
- Extract `name` and `description` from frontmatter
- List each skill with:
  - Skill name
  - Description (first line of description field)
  - File path (`.claude/skills/<name>/SKILL.md`)

For the `all` scope, there can be many skills. Keep it scannable. Group skills by family (e.g. git, review, interview prep, docs, visualization) and give each one line. Trim each description to a short clause; do not paste the full frontmatter description.

### If scope includes "agent" or "all":

Read `~/.claude/agents/monet.md`:
- Extract agent name and description from frontmatter
- Extract model configuration
- Extract tools list
- Count the numbered dispatch steps in the body
- Show agent dispatch routing summary

## Step 4: Output

Present the findings from Step 3 as a clean status report: a top-level heading, one section per scope, headers and bullet points throughout. Keep output concise and scannable.
</process>
