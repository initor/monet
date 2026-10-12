---
name: monet:teach
description: Deeply understand the work just done, or any PR, file, diff, or concept you name, through a guided, incremental teaching loop with a running checklist and active quizzing, gated on demonstrated mastery.
argument-hint: "[target: session (default) | PR #/URL | path | diff range | concept]"
allowed-tools:
  - Read
  - Bash
  - Glob
  - Grep
  - Write
  - AskUserQuestion
  - TaskCreate
  - TaskUpdate
  - Skill
---
<objective>
Run the teach skill: act as a wise teacher that makes sure you deeply understand the work in front of you: incrementally, and gated on demonstrated mastery, not a one-shot explanation at the end.
</objective>

<execution_context>
@~/.claude/skills/teach/SKILL.md
</execution_context>

<context>
Target: $ARGUMENTS

- Empty → teach the work from the current session (what we just did).
- A PR number or URL → read the PR's diff and description as the subject.
- A file or directory path → read it as the subject.
- A diff range or commit (e.g., `main..HEAD`, a SHA) → read that diff as the subject.
- Anything else → treat as a concept/topic to teach (research if needed).
</context>

<process>
1. Parse `$ARGUMENTS` to resolve the subject (see context above); if empty, use the current session's work.
2. Read the teach SKILL.md referenced above and execute it exactly as written: all Steps, Constraints, and Tone rules. It is the single source of truth; do not restate or skip its steps here.
3. The skill maintains a running checklist at `.planning/tmp/teach/<slug>-understanding.md` and does not consider the session complete until every item is demonstrated. Do not short-circuit that gate.
</process>
