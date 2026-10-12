---
name: monet:handoff
description: Write this session's state to .planning/tmp/handoff/ and print a short, paste-ready prompt for a fresh session. Use when you're low on context and have to start over.
allowed-tools:
  - Bash
  - Write
---
<objective>
Run the handoff skill: write this conversation's state to a timestamped file, then print a ~6-line paste block pointing at it.
</objective>

<execution_context>
@~/.claude/skills/handoff/SKILL.md
</execution_context>

<context>
Invoked when context is nearly exhausted and the user is about to start a new session. The state comes from THIS conversation, not from re-reading files, which is exactly what there is no context left to do.
</context>

<process>
1. Read the handoff SKILL.md referenced above and execute it exactly as written. It is the single source of truth; don't restate or modify its steps here.
2. Honor its three rules without exception: run in the main session (never a subagent), never ask the user a question, never re-read files to rebuild state already in context.
3. End with the fenced paste block and the file path. Nothing after them.
</process>
