# Monet Command Reference

## Commands

Monet commands are invoked as `/monet:<name>` in Claude Code. Each command is a `.md` file in `.claude/commands/monet/` with frontmatter defining its name, description, and allowed tools.

Available commands are discovered at runtime by scanning the commands directory.

## Skills

Monet dispatches to skills based on user intent. Skills are defined in `.claude/skills/<name>/SKILL.md` and contain domain-specific workflows (code review, git workflow, visualization, etc.).

The monet agent matches user requests to the appropriate skill automatically. You do not need to invoke skills directly -- monet routes for you.

## Usage

**Direct command invocation:**
- `/monet:help` -- show this reference
- `/monet:status` -- show agent status and skill inventory
- `/monet:status skills` -- show skill inventory (argument passed directly)

**Agent dispatch (via monet agent):**
- Ask monet to "review this code" -- routes to the review-code skill
- Ask monet to "review my changes" -- routes to the review-local skill
- Ask monet to "paint this" -- routes to the paint skill

## Command Features

Monet commands support four capabilities:

1. **Slash invocation:** Commands live in `.claude/commands/monet/` and are invoked as `/monet:<name>`.
2. **Interactive input:** Commands can use `AskUserQuestion` in their allowed-tools to gather input interactively when arguments are not provided.
3. **Argument passing:** Commands declare `argument-hint` in frontmatter and receive arguments via `$ARGUMENTS` in their process block.
4. **Workflow references:** Commands reference shared workflow files via `@` paths in their `execution_context` block, enabling content reuse across commands.
