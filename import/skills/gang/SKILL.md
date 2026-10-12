---
name: gang
description: Use when the user asks to "orchestrate" a build, says "run this plan with a fleet", or wants a "fleet" or "gang" of agents to turn specs into a task graph and land every task on main through wave PRs. After one plan approval, it pushes the integration branch and opens wave PRs without asking. For one local review or one commit and PR, use review-local or git-ship instead.
---

# Gang

Turn specs into a task graph, then run the graph with a fleet of agents until every task reaches `main` through wave PRs.

## Overview

The `gang` CLI owns the plan, the run state, the merges, the gates, and the GitHub calls. A generated Workflow script, `run/engine.js`, runs ready tasks through implement, review, fix, merge, and health. It stops at a checkpoint. This session is the coordinator. It reacts to events, opens wave PRs, edits the plan, and launches the engine again.

git is the source of truth. A task is done when a commit whose message has the line `Task: <id>` is on the integration branch. The plan lives on the private ref `refs/gang/<slug>/plan`, never on the integration branch. The design is in `~/.claude/docs/specs/2026-10-11-monet-gang.md`.

## When to use

- A spec folder must become many reviewed commits, built in parallel by agents.
- An existing task plan must reach `main` with no orchestrator code written by hand.
- Not for one review of local changes. Use `review-local` instead. Not for one commit and PR. Use `git-ship` instead.
- Not for a view of a work stream. Use `roadmap` instead.

## Preconditions

- Run in the main session, started at the root of the target repo. Workflow agents make their worktrees in the session's own repo.
- Never run this skill as a subagent. A run spans many turns, and a subagent returns one result.
- `go` 1.24 or later, `node` 22 or later, `git` 2.40 or later, and an authenticated `gh` are on PATH.

## Paths

| Name | Path |
|---|---|
| `<repo>` | The session cwd, which is the root of the target repo |
| `<slug>` | The `--slug` value, else the base name of `<repo>` in lowercase. Each run outside `[a-z0-9]` becomes `-`, with no `-` at either end. It matches `^[a-z0-9][a-z0-9-]*$` |
| `<dir>` | `<repo>/.planning/gang/<slug>`, the gang dir of one run, as an absolute path |
| `<gang>` | `<dir>/run/bin/gang`, the CLI build for this run |

The target repo ignores the whole `.planning/gang/` folder. The reference files start with the build command. For `status`, build only when `<gang>` is missing:

```bash
go -C ~/.claude/skills/gang/cli build -ldflags "-X main.skillDir=$HOME/.claude/skills/gang" -o <dir>/run/bin/gang ./cmd/gang
```

For `run` and `status`, stop when `<dir>/plan/plan.json` is missing. Tell Wayne to run `plan` first, or to pass the right `--slug`.

## Verbs

| Verb | What it does | Read first |
|---|---|---|
| `plan <spec-dir> [--slug <slug>]` | Drafts, lints, and schedules a plan from specs, then asks for the one approval | `reference/planner.md`, "From specs" |
| `plan --adopt <file> [--adopt-source <file>] [--slug <slug>]` | Transcribes an existing plan into `plan.json` and cross-checks it with lint | `reference/planner.md`, "Adopt" |
| `run [--slug <slug>] [--merge-on-green]` | Runs preflight, a one-task pilot, and then the event loop until the done predicate holds | `reference/coordinator.md` |
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

Before the first engine launch of a run, confirm each item. `reference/coordinator.md` has the commands.

1. `git -C <repo> rev-parse --show-toplevel` prints the session cwd.
2. Wayne is present. The first launch is a one-task pilot.
3. The permission mode allows the Workflow tool and `git push`. In auto mode, ask Wayne to switch to accept-edits mode.
4. `git -C <repo> check-ignore -q .planning/gang/` exits 0.
5. The CLI is built, and `<gang> preflight --holder <session>` prints no `FAIL` line.
6. `run/standing.md` holds the seed orders.
7. Wayne knows the expected duration from `<gang> schedule`.

## Constraints

- Do not claim results without evidence. Prove the done predicate on the real repo before you close a run.
- Keep output bounded and skimmable. Read only short outputs, such as `<gang> status` and event lines. Never read raw diffs or transcripts.
- Write posture: after the one plan approval, gang pushes the integration branch and opens wave PRs without asking (spec decision 5). Plan edits at a checkpoint need no approval.
- Standing exception: in a gang run, a PR from the integration branch into `main` is allowed (spec decision 2). The global rule against PRs from an integration branch holds for all other work.
- Merge a wave PR only when the run started with `--merge-on-green`. Otherwise Wayne merges it.
- Never force-push. Never push to `main`. Push only the integration branch, and only with `<gang> push`.
- No Slack in version 1 (spec decision 9). Status lives in `run/status.md` and in the wave PRs.
- Never delete a branch, a worktree, or a file. Give Wayne the exact command.
- Use model aliases only: `opus`, `sonnet`, `haiku`, `fable`. Refer to Wayne by name or with they or them.
