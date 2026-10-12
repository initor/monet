---
name: monet:review-local
description: Review local changes before PR creation. Spawns parallel domain-expert sub-agents for unit test coverage, coding conventions, pattern adherence, module placement, best practices, and simplification. Visualizes report via monet:paint. Does NOT modify code until you ask.
argument-hint: "[--domain <expertise>] [--agents <N>] [--model <model>]"
allowed-tools:
  - Read
  - Bash
  - Glob
  - Grep
  - Agent
  - AskUserQuestion
---
<objective>
Run the review-local skill to thoroughly review all local changes before creating a PR.

This command spawns N parallel domain-expert sub-agents that analyze your local changes (committed on branch + uncommitted) against six dimensions: unit test coverage, coding conventions, pattern adherence, module placement, best practices, and simplification opportunities. The review is visualized as an HTML report via monet:paint. No code is modified until you explicitly ask.
</objective>

<execution_context>
@~/.claude/skills/review-local/SKILL.md
</execution_context>

<context>
Arguments: $ARGUMENTS

- If `$ARGUMENTS` contains `--domain <expertise>`, pass it as the domain expertise for sub-agents (e.g., `--domain "distributed systems"`).
- If `$ARGUMENTS` contains `--agents <N>`, spawn N review agents (default: 3).
- If `$ARGUMENTS` contains `--model <model>`, pass it to the skill as `agent_model`. Accepted values: `opus`, `sonnet`, `haiku`. Default: `opus`.
- If `$ARGUMENTS` is empty, the skill will ask interactively.
- Assumes the current working directory is the root of the repository with local changes.
</context>

<process>
## Step 1: Parse arguments and verify prerequisites

Parse any `--domain`, `--agents`, and `--model` flags from `$ARGUMENTS` (see `<context>`) and carry them into the skill. Verify the current working directory is a git repository with local changes to review (committed ahead of base branch, or uncommitted). If no changes exist, inform the user and stop.

## Step 2: Read and execute the SKILL.md

Read the review-local SKILL.md referenced in `<execution_context>` above and execute it exactly as written: it is the single source of truth for the workflow, tone, and constraints. Pass any parsed flags to its Step 2.

## Step 3: Report

The skill ends in interactive mode (fix findings, re-review, or ship). Stay in that conversation until the user is satisfied.
</process>
