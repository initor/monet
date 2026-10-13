---
name: prep
description: Use when the user wants to practice interviews -- system design, behavioral, or coding mocks -- or view their prep progress dashboard. Routes by domain + mode arguments to the matching mock skill (sd-mock, bq-mock, code-mock) or interview-progress; falls back to interactive selection when arguments are missing or invalid.
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

## Overview

A thin router: resolve `<domain> [mode] [topic]` to a downstream interview skill, then hand off. This skill never runs a mock itself -- it only dispatches.

## When to use

- The user asks to practice or run a mock and hasn't named a specific domain skill, or wants a single entry point.
- The user asks to check interview prep progress / view their dashboard.
- Not for running the actual interview -- that is owned by `monet:sd-mock`, `monet:bq-mock`, `monet:code-mock`, and `monet:interview-progress`. This skill only picks one and delegates.

## Arguments

Arguments: $ARGUMENTS

Read `$ARGUMENTS` as positional inputs, `<domain> [mode] [topic]`. There are no flags. Pass them through unchanged. Empty arguments are valid: this skill then selects interactively (Step 1).

- First argument: domain (`sd`, `bq`, `code`, `progress`, or an alias)
- Second argument (optional): mode (`mock`, `drill`, `story-bank`, or an alias)
- Third argument (optional): topic, passed through verbatim to the downstream skill

## Quick reference

| Domain arg (aliases) | Mode arg (aliases) | Downstream skill | Context passed |
|----------------------|--------------------|------------------|----------------|
| `sd` / `system-design` / `system` / `design` | `mock` / `practice` / `interview` | `monet:sd-mock` | "Run a system design mock" + topic if provided |
| `sd` … | `drill` | `monet:sd-mock` | "Drill {topic or 'a specific phase'}" |
| `bq` / `behavioral` / `behavior` | `mock` / `practice` / `interview` | `monet:bq-mock` | "Run a behavioral mock" + topic if provided |
| `bq` … | `story-bank` / `stories` / `bank` / `build` | `monet:bq-mock` | "Build a story for the bank" + topic if provided |
| `code` / `coding` | `mock` / `practice` / `interview` | `monet:code-mock` | "Run a coding mock" + topic if provided |
| `code` … | `drill` | `monet:code-mock` | "Drill {topic or 'a specific aspect'}" |
| `progress` / `dashboard` / `stats` | (none) | `monet:interview-progress` | (none) |

Aliases are case-insensitive.

## Preconditions

Parse `$ARGUMENTS` positionally: `<domain> [mode] [topic]`. Resolve `domain` and `mode` via the alias columns in Quick reference. `topic` (third positional) is passed through verbatim -- no validation.

### Routing logic

1. Domain **empty** -> Step 1 (interactive domain selection).
2. Domain **invalid** (not in alias map) -> show `Unknown domain: {input}. Valid domains: sd, bq, code, progress`, then fall back to Step 1.
3. Domain **valid** and `progress` -> Step 3 (delegate).
4. Domain **valid**, mode **empty**, domain not `progress` -> Step 2 (interactive mode selection).
5. Domain **valid**, mode **invalid** -> show `Unknown mode: {input} for {domain}.`, then fall back to Step 2.
6. Domain **valid** and mode **valid** -> Step 3 (delegate).

## Step 1: Interactive domain selection

Use AskUserQuestion: "What would you like to practice?" Options map to domains:

- "System Design" -> `sd`
- "Behavioral" -> `bq`
- "Coding" -> `code`
- "View progress dashboard" -> `progress`

If `progress`, go to Step 3. Otherwise, Step 2.

## Step 2: Interactive mode selection

Use AskUserQuestion with a domain-specific prompt. Map the choice to a mode value, then go to Step 3.

- **sd** -- "System design -- choose your mode:" -> "Full mock interview" = `mock`, "Drill a specific phase" = `drill`
- **bq** -- "Behavioral -- choose your mode:" -> "Full mock interview" = `mock`, "Build stories for your bank" = `story-bank`
- **code** -- "Coding -- choose your mode:" -> "Full mock interview" = `mock`, "Drill a specific aspect" = `drill`

## Step 3: Delegate to downstream skill

Look up the downstream skill and context string in the Quick reference table. Invoke that skill with the Skill tool, by its plugin name (`monet:sd-mock`, `monet:bq-mock`, `monet:code-mock`, or `monet:interview-progress`), passing the context string as its arguments. The Skill tool resolves the skill from any working directory; do not construct a filesystem path. The downstream skill handles all further interaction. Do not add steps after delegation.

## Constraints

- Never run a downstream mock inline -- always delegate to the named skill with the Skill tool.
- Invalid input never dead-ends; every error falls back to interactive selection.
- Do not remember domain/mode choices across sessions.
