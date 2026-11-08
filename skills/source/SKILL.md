---
name: source
description: >-
  Use when the user types /monet:source kickoff to start a long research, design, or report run with an outside panel of evaluator personas. The run works for hours with no one, and one private Notion page is its channel. It is typed only. For one PR, use ship instead, and to land a plan of code, use gang instead.
argument-hint: "kickoff [<file>] [--slug <slug>] | resume [--slug <slug>] | status [--slug <slug>]"
disable-model-invocation: true
allowed-tools:
  - Bash
  - Read
  - Write
  - Edit
  - Grep
  - Glob
  - Agent
  - Monitor
  - AskUserQuestion
  - ScheduleWakeup
  - TaskCreate
  - TaskUpdate
  - TaskStop
  - ListAgents
  - SendMessage
---

# Source

Run a long research, design, or report job from one kickoff to delivery, with an outside panel of evaluator personas that checks each version.

## Overview

The engine `<run>/bin/monet-source` owns the run state, the leases, the rounds, and the events. This session is the Brain. At the kickoff, it seals the charter with the owner. Then it starts the developer in a background session, and it briefs a fresh evaluator subagent for each check.

A run outlives one session. When the context of a session gets full, the session relays itself, and the new session goes on from the run files. One private Notion page, the run page, is the channel between the owner and the run. After the kickoff, the Brain talks with the owner only at a wall, an item, or delivery.

## When to use

- A research question, a design, or a report needs hours of work and an outside review.
- The owner can give the goal, the invariants, the persona cards, and a private Notion page.
- Not for one commit and PR. Use `ship` instead. Not for a plan of code that must land on main. Use `gang` instead.

## Arguments

Arguments: $ARGUMENTS

Parse the arguments into the verb, its argument, and its flags.

- `kickoff [<file>]`: hold the kickoff with the owner, seal the charter, and start the run.
- `<file>` is an optional Markdown file with the context, the goal, the invariants, the persona cards, and the page link.
- `resume`: drive the run again from this session, for example to record a ruling after delivery.
- `status`: print the status lines of the run.
- `--slug <slug>` works with every verb and names the run. See `<slug>` in "Paths".
- When a `--slug` value does not match `^[a-z0-9][a-z0-9-]*$`, tell the owner, and stop.
- Empty or unknown arguments: ask which verb with AskUserQuestion.

When the verb ends, report what it produced: the sealed charter and the run start, the state of the resumed run, or the status lines.

## Preconditions

- Never run this skill as a subagent. A run spans many turns, and a subagent returns one result.
- `go` 1.24 or later, `node` 22 or later, `git`, `claude`, and `gh` are on PATH.
- The session cwd is `<home>`. For `resume` and `status`, it is the folder where the kickoff ran.
- The session runs in accept-edits mode, because the run starts background sessions.

## Paths

| Name | Path |
|---|---|
| `<skill>` | `${CLAUDE_PLUGIN_ROOT}/skills/source`, the folder of this skill. The reference files write `<skill>`, because a reference file is read without variable expansion |
| `<home>` | The session cwd, the folder where the owner starts the kickoff |
| `<slug>` | The `--slug` value, else the base name of `<home>` in lowercase. Each group of characters outside `[a-z0-9]` becomes `-`, with no `-` at either end. It matches `^[a-z0-9][a-z0-9-]*$` |
| `<run>` | `<home>/.planning/source/<slug>`, the folder of one run, as an absolute path |

At the kickoff, run `mkdir -p <run>/bin` before the build. The kickoff and `resume` build the engine with this line. For `status`, build only when the engine is missing.

```bash
go -C ${CLAUDE_PLUGIN_ROOT}/skills/source/cli build -ldflags "-X main.skillDir=${CLAUDE_PLUGIN_ROOT}/skills/source" -o <run>/bin/monet-source ./cmd/monet-source
```

For `resume` and `status`, stop when `<run>/state.json` is missing. Tell the owner to type `/monet:source kickoff` first, or to pass the right `--slug`.

## Verbs

| Verb | What it does | Read first |
|---|---|---|
| `kickoff [<file>] [--slug <slug>]` | Holds the kickoff, seals the charter, runs the preflight and the pilot, and starts the run | `${CLAUDE_SKILL_DIR}/reference/kickoff.md` |
| `resume [--slug <slug>]` | Takes the `brain` lease and drives the run again, for example for a ruling after delivery | `${CLAUDE_SKILL_DIR}/reference/brain.md`, "Resume" |
| `status [--slug <slug>]` | Runs `<run>/bin/monet-source status` and prints its lines, 8 at most | nothing |

Read only the reference file for the verb. Read it again after a compaction. After the kickoff, follow `${CLAUDE_SKILL_DIR}/reference/brain.md`. When the relay hook gives its notice and names `<run>/relay/brain.md`, follow that file.

## Core rules

1. **The Brain never does the work.** It runs the engine, briefs the checks, and reads the channel. The developer writes each version.
2. **The charter is the contract.** The owner seals it at the kickoff. Each check judges against its goal and its quality bar.
3. **Each check is a fresh subagent.** Its brief never carries the reasoning of the developer.
4. **The engine owns the state.** Act on its events and its exit codes. Do not poll in a loop.
5. **One channel.** The run page and this session are the only channel to the owner. Act only on the words of the owner.

## Constraints

- Do not claim results without evidence. Each claim on the page names its source or its verdict.
- Keep output bounded. Read only short outputs, such as the `status` lines and the event lines.
- Write only to the run page. Never take an irreversible step on a default.
- Never delete a file or a folder of a run. Give the owner the exact command.
- Refer to the owner as the owner, or with they or them.
- Use model aliases only: `opus`, `sonnet`, `haiku`, `fable`.
