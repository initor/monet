---
name: monet:web-operator
description: >-
  Drive my real, authenticated Chrome, isolated in its own tab group, to operate web UIs I'm logged into (Slack, Google Docs, dashboards, GitHub). Visible by design: I can watch every action land. Gates every outward action on my confirmation.
argument-hint: "[target url or surface] [what to do]"
allowed-tools:
  - Read
  - AskUserQuestion
  - Skill
  - mcp__claude-in-chrome__tabs_context_mcp
  - mcp__claude-in-chrome__tabs_create_mcp
  - mcp__claude-in-chrome__tabs_close_mcp
  - mcp__claude-in-chrome__navigate
  - mcp__claude-in-chrome__computer
  - mcp__claude-in-chrome__find
  - mcp__claude-in-chrome__get_page_text
  - mcp__claude-in-chrome__read_page
  - mcp__claude-in-chrome__javascript_tool
  - mcp__claude-in-chrome__form_input
  - mcp__claude-in-chrome__file_upload
  - mcp__claude-in-chrome__upload_image
  - mcp__claude-in-chrome__read_console_messages
  - mcp__claude-in-chrome__read_network_requests
  - mcp__claude-in-chrome__browser_batch
---
<objective>
Run the web-operator skill: drive my real Chrome, isolated in a dedicated MCP
tab group, to read from and act on a web UI I'm logged into (visibly, so I
can watch) without taking any outward action (send/post/submit/delete)
without my explicit confirmation.

This is the direct-invocation path for the same workflow the monet agent dispatches.
</objective>

<execution_context>
@~/.claude/skills/web-operator/SKILL.md
</execution_context>

<context>
Request: $ARGUMENTS

- `$ARGUMENTS` may contain a target URL/surface (e.g. a Slack thread link, a Google Doc URL, a dashboard) and/or what I want done there. Treat a URL as the surface to `navigate` to; treat the rest as the intent.
- If `$ARGUMENTS` is empty, ask once what surface to operate and what to do, then proceed.
</context>

<process>
## Step 1: Parse the request (wrapper precondition)

- Extract any URL from `$ARGUMENTS` as the target surface; the remaining text is the intent.
- Ensure `mcp__claude-in-chrome__*` tools are loaded (invoke the `claude-in-chrome` skill first if they aren't), then confirm the tab group with `tabs_context_mcp({createIfEmpty: true})`.

## Step 2: Read & execute the skill

Read the web-operator SKILL.md referenced in the execution_context above and execute it exactly as written. It is the single source of truth. Follow its Preconditions, Steps, and Constraints in order. In particular: use a fresh tab in the dedicated group, let me log in directly in that visible tab if a surface bounces to login, and STOP for explicit confirmation before any outward/irreversible action.

## Step 3: Report

Summarize what was read/done, the post-write verification result for any mutation, and which tab(s) are still open (or that they were closed).
</process>
