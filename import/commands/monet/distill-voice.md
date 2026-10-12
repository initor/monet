---
name: monet:distill-voice
description: Distill a piece of writing I hand you into a named persona that the voice skill can apply. Measures the source, extracts its moves with verbatim samples, and writes one persona file.
argument-hint: "<source: path | PR #/URL | pasted text> --as <persona> [--surface <surface>] | --corpus <transcripts | github:<login> orgs:...|repos:... | dir> --as <persona>"
allowed-tools:
  - Read
  - Write
  - Edit
  - Bash
  - Glob
  - Grep
  - WebFetch
  - AskUserQuestion
---
<objective>
Distill a source I hand you into a persona file that `/monet:voice` can apply. Measure it, quote it, and never invent a rule I cannot see the evidence for.
</objective>

<execution_context>
@~/.claude/skills/distill-voice/SKILL.md
</execution_context>

<context>
Target: $ARGUMENTS

- `--as <persona>` names the author. Required. Lowercase single word.
- `--surface <surface>` names the row from the surface map this source evidences. Ask me if absent.
- `--corpus <scope>` switches to corpus mode: gather the scope I named with the bundled scripts,
  apply the agent-content filter, and confirm the register-to-surface mapping with me before
  distilling. No `--surface` in this mode.
- Everything else is the source: a file path, a PR number or URL, or pasted text.
</context>

<process>
1. Parse `$ARGUMENTS` into source, persona, and surface. Ask for the persona if it is missing rather than guessing an author.
2. Read the distill-voice SKILL.md referenced above and execute it exactly as written, every Step, hard rule, and Constraint. It is the single source of truth, so do not restate or modify its steps here.
3. If the named persona already exists, this run replaces it: show me which old sources, moves, and register rows the new file drops, and get my confirmation before writing.
4. Write exactly one persona file, then update its roster row in `~/.claude/skills/voice/reference/personas.md`. Never touch another persona.
5. Report the metrics you measured, the moves you extracted with a sample count each, and which register rows filled. Name any move you considered and dropped for lack of a sample. Print the surface map row you suggest for each evidenced surface, and never write the surface map yourself.
</process>
