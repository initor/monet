---
name: think
description: >-
  Use when the user types /monet:think to make a task more ambitious ("be more ambitious", "think higher, deeper or wider", "an out-of-the-box take on X"). Rewrites the ask along one lens (--higher outcome, --deeper expert method, --wider other fields plus a red team), grounded in what the session can reach and checked by an outside judge. Typed only, never auto-invoked.
argument-hint: "[--higher | --deeper | --wider] <task or draft prompt>"
disable-model-invocation: true
allowed-tools:
  - Read
  - Bash
  - Glob
  - Grep
  - Agent
  - AskUserQuestion
---

## Overview

Models get stronger every few weeks, but the asks we give them don't. This skill takes one task and asks for higher, deeper, or wider work, grounded in what this session can actually reach and checked by something other than the model. The method comes from Felix Rieseberg, [How to be more ambitious: A tutorial for humans with models](https://felixrieseberg.com/how-to-be-more-ambitious-a-tutorial-for-humans-with-models/) (13 October 2026).

| Flag | Lens | Use when | Output |
|---|---|---|---|
| `--higher` | Altitude | an outcome you'll hold meetings about | 3 rungs, task to outcome to new process |
| `--deeper` | Depth | you want the task done really well | the 4 expert answers and a rewritten prompt |
| `--wider` | Range | you want angles you'd never see | verbs, a crew and a red team, and the crew runs on a yes |
| none | picked in Step 1 | | |

Not for brainstorming the requirements of a feature already chosen, and not for a lesson on finished work (use `monet:teach`).

## Step 1: Parse and route

1. Read the flag and the task from `$ARGUMENTS`. If the task is empty, ask for it once with AskUserQuestion. If more than one lens flag is given, use the first and say so in one line.
2. With no flag, pick the lens from the routing tells in `${CLAUDE_SKILL_DIR}/reference/lenses.md`, and state the pick and its tell in one line.

## Step 2: Ground

Look before you claim. With at most about ten cheap, read-only calls (`ls`, `git -C`, `rg`, the tool and connector names this session lists), fill three lists for the task:

- **Raw material**: logs, data, repos, and connectors that hold the source, not a summary of it.
- **Instruments**: the methods and tools a top practitioner would use that are installed or one install away.
- **Judges**: anything other than the model and the user that can say the result is right, such as a test, a benchmark, held-out data, CI, a real machine, or a metric.

Name each gap in one line. Never invent a source or a judge you didn't see.

## Step 3: Apply the lens

Read the chosen lens in `${CLAUDE_SKILL_DIR}/reference/lenses.md` and apply it to the task with the Step 2 lists. Across every lens:

- **Deeper, not bigger.** Ask for better work on the goal, never more deliverables. Only `--higher` changes the goal, and the user picks how far.
- **Methods, not voices.** Name each expert by the procedure and instrument they use, never by how they talk.
- **One stretch goal.** Name one specific target far past the obvious one (a 10s page load becomes 200ms, not 5s), and state it as reachable.
- **A judge for every direction.** Use a judge from Step 2, or write "untested" plus what it would take to get one.
- **No invented numbers.** Every number on the card, a stretch goal's baseline included, comes from a command or file read in Step 2, and a count reads the same everywhere. Otherwise write "unknown" and say how to measure it.

## Step 4: Print the card

Print one card of at most 35 lines, in this shape:

```
think --<lens>: <the task in one line>
Lens      <why this lens, one line>
Reach     raw: ... | instruments: ... | judges: ...
Gaps      <what's missing, one line each>
<the lens block from reference/lenses.md>
Stretch   <the one stretch goal>
Judge     <what shows the result is right>
Prompt    <the rewritten prompt, ready to paste or run>
Also      --<other lens>: <one line on what it would ask for> | --<third lens>: <one line>
```

The Prompt line must work alone, pasted into a fresh session, in at most about 150 words. Besides the ask, it carries the judge (how the result will be proven, and what evidence to report back) and the guardrails a careful engineer would add, such as no workaround that hides the problem and no merge, push, or other irreversible step without asking. A Judge line the Prompt line drops is a defect.

For `--higher`, the Prompt line serves the recommended rung: its first sentence states that rung's goal, so the judge and the guardrails never crowd the altitude out. The card says how to take another rung instead.

## Step 5: Run the crew (`--wider` only, on a yes)

Ask once with AskUserQuestion whether to run the crew. On a yes:

1. Pick the crew fields most likely to disagree with each other, since two fields that reach the same answer add cost and no range. Dispatch the field agents (2 by default, 3 at most) in one message, on Sonnet at medium effort unless the user names another model. Each brief holds the task, the verb restatement, the field, its procedure, the raw-material paths, the judge, a read-only rule, and a cap of 25 lines back: finding, evidence, direction, and how the judge would test it.
2. Then dispatch one red-team agent with the field results, using the attack procedure from the lens. It returns at most 15 lines of objections, each tied to evidence.
3. Synthesize 2 or 3 directions, each with its evidence, the red team's strongest objection, and its judge, in at most 30 lines.

## Constraints

- Don't claim results without evidence. A direction without a reachable judge is labeled untested.
- Keep output bounded and skimmable: the card fits one screen.
- Read-only: the skill writes and commits nothing, and running the rewritten prompt is the user's call.
