---
name: handoff
description: Use when the user is running out of context and needs to continue in a fresh session ("I'm at 80% context", "I have to start a new session", "draft me a prompt to pick up this work", "hand this off"). Writes the state to a file, then prints a short paste-ready prompt pointing at it. For resuming a gang run, use gang instead.
---

## Overview

The user is out of context and starting a new session. Produce exactly two things in one pass:

1. `.planning/tmp/handoff/<YYYY-MM-DD-HHMM>.md`: full state, written from **this conversation**.
2. A ~6-line paste block pointing at it.

Then stop. No questions, no confirmation, no offer of follow-up work.

## Three rules

These are what keep this skill from re-growing into a workflow:

- **Run in the main session.** Never delegate to a subagent: the conversation *is* the payload, and
  a subagent has none of it. It would write a hollow file confidently.
- **Never ask the user anything.** You are invoked at the moment context is scarce. Emit and stop.
- **Don't re-read files to rebuild state you already hold.** Step 1 is the only exception.

## Step 1: Refresh the volatile facts

One cheap block. These rot fastest, so take them live rather than from context:

```bash
date +%Y-%m-%d-%H%M
git rev-parse --abbrev-ref HEAD 2>/dev/null
git status --porcelain 2>/dev/null | head -20
gh pr view --json number,state,reviewDecision 2>/dev/null
```

Skip failures silently: not every stream is a repo, not every branch has a PR.

## Step 2: Write the detail file

`mkdir -p .planning/tmp/handoff`, then write `<YYYY-MM-DD-HHMM>.md` using the timestamp from Step 1
(minute-stamped, so two handoffs in one day never collide). If cwd isn't writable, fall back to
`~/.claude/handoffs/`.

Sections, in order. Omit any that is genuinely empty; don't pad.

| Section | What it holds |
|---|---|
| **Goal** | What we're trying to accomplish. One paragraph. |
| **Done** | Past tense, with paths. Phrase so it cannot be misread as still-pending. |
| **Next** | Open items, most-immediate first. |
| **Blocked** | On what, and on whom, by name. |
| **Decisions** | What we chose and why, so it isn't relitigated. |
| **Corrections** | What you got wrong and the user corrected, **and which artifact still holds the wrong version**. Most expensive content to lose: losing one reproduces the error. |
| **Gotchas** | Traps a fresh session would walk into. |
| **State** | Branch, tree, PR (from Step 1). |

Concrete paths and runnable commands throughout. A fresh session should never have to search for
something you already know the location of.

## Step 3: Print the paste block

Terminals render code fences with little or no visible border, so the fence alone does not show the
user where to stop selecting. **Delimit the prompt with explicit markers inside the fence:**

```
━━━━━━━━━━━━━━━━━━━━ BEGIN PROMPT ━━━━━━━━━━━━━━━━━━━━
Continue <one-line goal> in <repo> (branch <branch>, <PR state>).

Read <ABSOLUTE path from Step 2> first: full state, decisions, gotchas.

Then: <single most immediate next action, named not described>.
<Any standing agreement, e.g. "don't push or comment without my go-ahead".>
━━━━━━━━━━━━━━━━━━━━━ END PROMPT ━━━━━━━━━━━━━━━━━━━━━
```

- The path must be **absolute**: the new session may start from a different cwd.
- **Never put prose on the line directly above or below the fence.** Anything adjacent gets selected
  as part of the prompt. Anything you need to say goes before the fence with a blank line between,
  or after the file path.

After the fence, print the file path on one line. Nothing else.

## Common mistakes

- Delegating to a subagent, which has none of the conversation this file is made of.
- Asking a clarifying question instead of just emitting.
- Omitting the BEGIN/END markers, or butting prose against the fence. The fence is nearly invisible
  in a terminal, so whatever sits next to it gets selected and pasted along with the prompt.
- A relative path in the paste block, which breaks from another cwd.
- Describing the next action ("continue the refactor") instead of naming it ("wire the retry budget
  into `reconciler.go:88`").
- Writing Done and Next in the same voice, so finished work gets redone.
