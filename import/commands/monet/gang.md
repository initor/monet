---
name: monet:gang
description: Turn specs into a task graph, then run it with a fleet of agents until every task reaches main through wave PRs. After the one plan approval, it pushes the integration branch and opens wave PRs without asking. It merges a wave PR only with --merge-on-green, and it never force-pushes or pushes to main.
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
<objective>
Run the gang skill: plan a task graph from specs or adopt an existing plan, run the graph with a fleet of agents until every task reaches `main` through wave PRs, or print the run's status.
</objective>

<execution_context>
@~/.claude/skills/gang/SKILL.md
</execution_context>

<context>
Arguments: $ARGUMENTS

- `plan <spec-dir>`: draft, lint, and schedule a plan from the specs in `<spec-dir>`, then ask for the one approval.
- `plan --adopt <file>`: transcribe the existing plan in `<file>`, and cross-check it with lint. `--adopt-source <file>` names the JSON task graph that lint checks against. Without it, `<file>` must be that graph.
- `run`: preflight, a one-task pilot, then the event loop until every task is on `main`. Add `--merge-on-green` to merge each wave PR when its checks pass and its tasks are verified.
- `status`: print the run's status in at most 5 lines.
- `--slug <slug>` works with every verb and names the run. It must match `^[a-z0-9][a-z0-9-]*$`. It defaults to the base name of the repo root in lowercase, with each run of characters outside `[a-z0-9]` turned into `-` and no `-` at either end, so `.config` becomes `config`.
- Empty or unknown arguments: ask which verb with AskUserQuestion.
- The session cwd must be the root of the target repo, because Workflow agents make their worktrees in the session's own repo.
</context>

<process>
1. Parse `$ARGUMENTS` for the verb, its argument, and its flags, as `<context>` describes.
2. Check that `git rev-parse --show-toplevel` prints the current directory. If not, tell the user to start a session at the target repo root, and stop.
3. Read the gang SKILL.md referenced above and execute it exactly as written: the verb's reference file, the core rules, the launch checklist, and the constraints. It is the single source of truth. Do not restate or modify its steps here.
4. Report what the verb produced: the plan path and its schedule, the run's stop reason and next action, or the status lines.
</process>
