---
name: gang
description: Use when the user asks to "orchestrate" a build, says "run this plan with a fleet", or wants a "fleet" or "gang" of agents to turn specs into a task graph and land every task on main through wave PRs. After one plan approval, it pushes the integration branch and opens wave PRs without asking. For one local review or one commit and PR, use review-local or ship instead.
argument-hint: "plan <spec-dir> [--slug <slug>] | plan --adopt <file> [--adopt-source <file>] [--slug <slug>] | run [--slug <slug>] [--merge-on-green] | status [--slug <slug>]"
allowed-tools:
  - Bash
  - Read
  - Write
  - Edit
  - Grep
  - Glob
  - Agent
  - Workflow
  - Monitor
  - AskUserQuestion
  - ScheduleWakeup
  - TaskCreate
  - TaskUpdate
---

# Gang

Turn specs into a task graph, then run the graph with a fleet of agents until every task reaches `main` through wave PRs.

## Overview

The `gang` CLI owns the plan, the run state, the merges, the gates, and the GitHub calls. A generated Workflow script, `run/engine.js`, runs ready tasks through implement, review, fix, merge, and health. It stops at a checkpoint. This session is the coordinator. It reacts to events, opens wave PRs, edits the plan, and launches the engine again.

git is the source of truth. A task is done when a commit whose message has the line `Task: <id>` is on the integration branch. The plan lives on the private ref `refs/gang/<slug>/plan`, never on the integration branch. Its schema is in `${CLAUDE_SKILL_DIR}/reference/schema.md`.

## When to use

- A spec folder must become many reviewed commits, built in parallel by agents.
- An existing task plan must reach `main` with no orchestrator code written by hand.
- Not for one review of local changes. Use `review-local` instead. Not for one commit and PR. Use `ship` instead.
- Not for a view of a work stream. Use `roadmap` instead.

## Arguments

Arguments: $ARGUMENTS

Parse the arguments into the verb, its argument, and its flags:

- `plan <spec-dir>`: draft, lint, and schedule a plan from the specs in `<spec-dir>`, then ask for the one approval.
- `plan --adopt <file>`: transcribe the existing plan in `<file>`, and cross-check it with lint. `--adopt-source <file>` names the JSON task graph that lint checks against. Without it, `<file>` must be that graph.
- `run`: preflight, a one-task pilot, then the event loop until every task is on `main`. Add `--merge-on-green` to merge each wave PR when its checks pass and its tasks are verified.
- `status`: print the run's status in at most 5 lines.
- `--slug <slug>` works with every verb and names the run. See `<slug>` in "Paths". When a `--slug` value does not match `^[a-z0-9][a-z0-9-]*$`, tell the user, and stop.
- Empty or unknown arguments: ask which verb with AskUserQuestion.

When the verb ends, report what it produced: the plan path and its schedule, the run's stop reason and next action, or the status lines.

## Preconditions

- Check that `git rev-parse --show-toplevel` prints the current directory. If it does not, tell the user to start a session at the target repo root, and stop. Compare it with `pwd -P`, which resolves symlinks as git does. Workflow agents make their worktrees in the session's own repo.
- Never run this skill as a subagent. A run spans many turns, and a subagent returns one result.
- `go` 1.24 or later, `node` 22 or later, `git` 2.40 or later, and an authenticated `gh` are on PATH.

## Paths

| Name | Path |
|---|---|
| `<skill>` | `${CLAUDE_PLUGIN_ROOT}/skills/gang`, this skill's folder. The reference files write `<skill>`, because a reference file is read without variable expansion |
| `<repo>` | The session cwd, which is the root of the target repo |
| `<slug>` | The `--slug` value, else the base name of `<repo>` in lowercase. Each run outside `[a-z0-9]` becomes `-`, with no `-` at either end. It matches `^[a-z0-9][a-z0-9-]*$` |
| `<dir>` | `<repo>/.planning/gang/<slug>`, the gang dir of one run, as an absolute path |
| `<gang>` | `<dir>/run/bin/gang`, the CLI build for this run |

The target repo ignores the whole `.planning/gang/` folder. The reference files start with the build command. For `status`, build only when `<gang>` is missing:

```bash
go -C ${CLAUDE_PLUGIN_ROOT}/skills/gang/cli build -ldflags "-X main.skillDir=${CLAUDE_PLUGIN_ROOT}/skills/gang" -o <dir>/run/bin/gang ./cmd/gang
```

For `run` and `status`, stop when `<dir>/plan/plan.json` is missing. Tell the user to run `plan` first, or to pass the right `--slug`.

## Verbs

| Verb | What it does | Read first |
|---|---|---|
| `plan <spec-dir> [--slug <slug>]` | Drafts, lints, and schedules a plan from specs, then asks for the one approval | `${CLAUDE_SKILL_DIR}/reference/planner.md`, "From specs" |
| `plan --adopt <file> [--adopt-source <file>] [--slug <slug>]` | Transcribes an existing plan into `plan.json` and cross-checks it with lint | `${CLAUDE_SKILL_DIR}/reference/planner.md`, "Adopt" |
| `run [--slug <slug>] [--merge-on-green]` | Runs preflight, a one-task pilot, and then the event loop until the done predicate holds | `${CLAUDE_SKILL_DIR}/reference/coordinator.md` |
| `status [--slug <slug>]` | Runs `<gang> status` and prints its lines, 5 at most | nothing |

Read only the reference file for the verb. Read it again after a compaction.

## Core rules

1. **The coordinator never writes code.** It writes the plan, rulings, and standing orders. It runs `<gang>`. Agents write all code.
2. **The brief is the product.** A task in `plan.json` is its brief. Agents cannot ask, so a vague brief fails quietly. Lint must pass before any spawn.
3. **Standing orders go into every spawn and resume.** `gang brief` pastes `run/standing.md` into each brief. Change direction with `<gang> standing add`, never with a message to one agent.
4. **Fresh agents, not resumes.** Each round gets a new agent with the brief and `report.md`. Never resume an agent to check on it.
5. **Completions are queue events.** Act on a `gang watch` line or on the Workflow completion notification. Do not poll in a loop. Do not read transcripts.
6. **Verdicts are keyed by PR or task plus head SHA.** A new SHA voids the old verdict. `gang merge` trusts only the ledger. A null agent result is never a pass.
7. **Land only the contiguous verified run.** Open wave PR k only when waves 1 to k are merged. Merge wave PRs in order, and only when each task in them is CLEAN.

## Launch checklist

Before the first engine launch of a run, confirm each item. `${CLAUDE_SKILL_DIR}/reference/coordinator.md` has the commands.

1. `git -C <repo> rev-parse --show-toplevel` prints the session cwd.
2. The user is present. The first launch is a one-task pilot.
3. The permission mode allows the Workflow tool and `git push`. In auto mode, ask the user to switch to accept-edits mode.
4. `git -C <repo> check-ignore -q .planning/gang/` exits 0.
5. The CLI is built, and `<gang> preflight --holder <session>` prints no `FAIL` line.
6. `run/standing.md` holds the seed orders.
7. The user knows the expected duration from `<gang> schedule`.

## Constraints

- Do not claim results without evidence. Prove the done predicate on the real repo before you close a run.
- Keep output bounded and skimmable. Read only short outputs, such as `<gang> status` and event lines. Never read raw diffs or transcripts.
- Write posture: after the one plan approval, gang pushes the integration branch and opens wave PRs without asking. Plan edits at a checkpoint need no approval.
- Standing exception: in a gang run, a PR from the integration branch into `main` is allowed, even where the user's own rules forbid PRs from an integration branch. Those rules hold for all other work.
- Merge a wave PR only when the run started with `--merge-on-green`. Otherwise the user merges it.
- Never force-push. Never push to `main`. Push only the integration branch, and only with `<gang> push`.
- No Slack in version 1. Status lives in `run/status.md` and in the wave PRs.
- Never delete a branch, a worktree, or a file. Give the user the exact command.
- Use model aliases only: `opus`, `sonnet`, `haiku`, `fable`. Refer to the user as the user, or with they or them.
