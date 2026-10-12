---
name: monet:help
description: Show available monet commands and skill inventory for the monet agent
allowed-tools:
  - Read
  - Bash
  - Glob
  - Grep
---
<objective>
Display the monet command reference and skill inventory.

Output a formatted reference showing all available /monet:* commands and all registered monet skills. Use the shared workflow file for static reference content, then augment with live discovery of commands and skills on disk.
</objective>

<execution_context>
@~/.claude/workflows/monet/help.md
</execution_context>

<process>
1. Read the shared help workflow at `~/.claude/workflows/monet/help.md` for the static command reference content.

2. Scan `~/.claude/commands/monet/` to discover all available `/monet:*` commands:
   - For each `.md` file, take `<name>` from the file name without `.md`, and read the frontmatter to extract `description`.
   - List each command as `/monet:<name>` with its description.

3. Scan `~/.claude/skills/` to discover all registered skills:
   - For each `SKILL.md` in `~/.claude/skills/*/`, read the frontmatter to extract `name` and `description`.
   - List each skill with its name, description, and directory path.

4. Output a formatted reference combining:
   - The static content from the help workflow (command features, usage patterns)
   - The live-discovered commands list
   - The live-discovered skills inventory

Format cleanly with headers and bullet points. Do NOT add project-specific analysis, git status, or next-step suggestions beyond the reference content.
</process>
