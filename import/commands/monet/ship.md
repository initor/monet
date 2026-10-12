---
name: monet:ship
description: Draft a commit and pull request in one go (branch management, commit message, PR title/body), then, after one confirmation, commit, push, and open the PR end to end.
allowed-tools:
  - Read
  - Bash
  - Glob
  - Grep
  - AskUserQuestion
---
<objective>
Run the git-ship skill to draft a commit and PR in one workflow, with one confirmation gate before any write.
</objective>

<execution_context>
@~/.claude/skills/git-ship/SKILL.md
</execution_context>

<context>
No arguments. Operates on the current working directory's git state.
</context>

<process>
1. Verify the cwd is a git repo (`git rev-parse --is-inside-work-tree`). If not, tell the user and stop.
2. Read the git-ship SKILL.md referenced above and execute it exactly as written: all preconditions, steps, and the write-confirmation gate. It is the single source of truth; don't restate or modify its steps here.
3. Report what was drafted and which write actions (if any) ran. If the user chose manual handling, print the suggested commands.
</process>
