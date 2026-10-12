---
name: monet:pr
description: Review a pull request -- checkout from URL, spawn domain-expert sub-agents for parallel analysis, produce copy-paste-ready comments with GitHub permalinks and reference links, and visualize via monet:paint. Does NOT post comments until user confirms with "post all", then posts them as one COMMENT review.
argument-hint: "<pr-url> [--domain <expertise>] [--agents <N>] [--model <model>]"
allowed-tools:
  - Read
  - Bash
  - Glob
  - Grep
  - Agent
  - AskUserQuestion
---
<objective>
Run the pr skill to review a pull request from its GitHub URL.

This command accepts a full PR URL (e.g., https://github.com/org/repo/pull/123), spawns N parallel domain-expert sub-agents to analyze the diff for correctness, safety, and craftsmanship, and produces structured review comments with exact GitHub permalinks in copy-paste-ready format. Invokes monet:paint for a visual review report. The user retains full control -- no comments are posted until the user says "post all", and then the skill posts them all as one COMMENT review after re-checking the PR head and thread.
</objective>

<execution_context>
@~/.claude/skills/pr/SKILL.md
</execution_context>

<context>
PR URL: $ARGUMENTS

Parse $ARGUMENTS for:
- A full GitHub PR URL (required, either positional or the first URL-like argument)
- Optional `--domain <expertise>` flag (e.g., --domain "distributed systems")
- Optional `--agents <N>` flag (e.g., --agents 5)
- Optional `--model <model>` flag (e.g., --model opus). Accepted values: `opus`, `sonnet`, `haiku`. Default: `opus`.

If $ARGUMENTS contains a valid PR URL, pass it to the skill.
If $ARGUMENTS is empty or contains no valid URL, the skill will ask interactively.
If --domain is provided, pass it to the skill (skip the domain prompt).
If --agents is provided, pass it to the skill (skip the agent count prompt).
If --model is provided, pass it to the skill as `agent_model`. If omitted, default to `opus`.
</context>

<process>
## Step 1: Wrapper preconditions

Parse `$ARGUMENTS` for the PR URL and the `--domain` / `--agents` / `--model` flags as described in `<context>`. Verify `gh` CLI is authenticated (`gh auth status`); if not, tell the user to run `gh auth login` and stop. Being inside the target repo is NOT required.

## Step 2: Execute the skill

Read and execute the pr SKILL.md from `<execution_context>` exactly as written -- it is the single source of truth. Pass the parsed PR URL and flags to the appropriate skill steps.

## Step 3: Report

After the skill completes, the user edits or drops drafts as they like. Posting happens only on an explicit "post all", through the skill's Step 9: one COMMENT review carrying every inline comment, after re-checking the PR head and thread. Nothing approves or requests changes.
</process>
