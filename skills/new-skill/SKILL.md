---
name: new-skill
description: Use when the user asks to create, scaffold, or add a new monet skill ("new monet skill", "scaffold a skill for X"). Writes skills/<name>/SKILL.md from the standard template in a git checkout of the monet repo, adds the agent's routing row and dispatch line, then runs make check. Never writes into the installed plugin, and never commits.
argument-hint: "[skill-name]"
allowed-tools:
  - Read
  - Write
  - Edit
  - Bash
  - Glob
  - Grep
  - AskUserQuestion
---

## Overview

Scaffold a new monet skill in a checkout of the monet repo and wire it in: the skill dir, a routing row and a dispatch line in the monet agent, and a clean `make check`. The skill is the slash entry point (`/monet:<name>`), so there is no separate wrapper file to write.

Skill name argument: $ARGUMENTS

- If `$ARGUMENTS` holds a skill name (kebab-case: lowercase letters, digits, and hyphens only), use it and skip the name question in Step 1, but still stop on a collision as that question describes.
- Otherwise ask for the name in Step 1.

ROOT is the git checkout of the monet repo that you write into. Every path below is relative to ROOT, never to the current directory.

## Step 0: Resolve ROOT

1. Take the path the user named, else the cwd. Set ROOT to what `git -C <path> rev-parse --show-toplevel` prints. If that fails, stop and ask for the path of a monet checkout.
2. Check that `ROOT/.claude-plugin/plugin.json` has `"name": "monet"` and that `ROOT/agents/monet.md` exists. If either check fails, ROOT isn't the monet repo: stop and ask for the path.
3. Never write under the installed plugin cache. If ROOT's real path (`cd ROOT && pwd -P`) has a `/.claude/plugins/` segment, it's the copy Claude Code installed, not a working checkout: stop and ask for the path of a clone.
4. If ROOT is on `main`, or `git -C ROOT status --porcelain` prints anything, offer a worktree first: `git -C ROOT fetch origin main && git -C ROOT worktree add -b ${user_config.branch_prefix}<name> .claude/worktrees/<name> origin/main`. If the prefix above is empty or starts with `${user_config`, it isn't configured, so use no prefix. When the user takes it, ROOT becomes that worktree.

## Step 1: Gather skill details

Ask one question at a time with AskUserQuestion. Each builds on the answers before it.

1. **Name** (unless given): kebab-case, matching `^[a-z][a-z0-9]*(-[a-z0-9]+)*$`, named for the artifact it produces. Stop on a collision: `ROOT/skills/<name>/` already exists, or `ROOT/agents/monet.md` already routes to `monet:<name>`.
2. **Purpose**: 1-2 sentences, starting with a verb.
3. **Trigger phrases**: 2-4 phrases. Grep the Quick-reference routing table in `ROOT/agents/monet.md` for each. A phrase another skill already owns needs a disambiguator, not a duplicate.
4. **Siblings**: the skills it could be confused with. Each gets a disambiguator line in the new description, and a mirror line in its own description, except a vendored skill whose `UPSTREAM.md` forbids edits.
5. **Shape**: typeable as `/monet:<name>` and routed by the agent (the common case), or routing only, which sets `user-invocable: false`.
6. **Preconditions**, then **output format**: conversation, file, YAML block, or other.

## Step 2: Write the skill

Read `${CLAUDE_SKILL_DIR}/reference/skill-template.md` and fill it in:

- `{{SKILL_NAME}}` and `{{SKILL_DESCRIPTION}}`. The description follows the template's comment: `Use when ...`, the literal trigger phrases, the disambiguator lines, under ~400 characters (the lint fails one over 500). It must parse as YAML, because a strict loader drops a skill whose frontmatter does not, so quote it if it holds `: `. Drop the template's guidance comments.
- Add `argument-hint` when the skill takes arguments, and `allowed-tools` as a block list. The body reads the arguments from `$ARGUMENTS`.
- Preconditions from the answer. For "none", remove the section.
- Steps: keep the three starter steps and fit them to the skill: context, core action, and output in the chosen format. Budget 70-100 lines. Push checklists and domain detail into `skills/<name>/reference/`, read lazily from a step as `${CLAUDE_SKILL_DIR}/reference/<file>`.
- Constraints: keep the two starters and add one specific to the skill.
- Tone rules: only for a skill that gives feedback or critique. Otherwise remove the section.

Path and reuse rules, which `make check` enforces:

- Name files inside the skill with `${CLAUDE_SKILL_DIR}/`, and other plugin files with `${CLAUDE_PLUGIN_ROOT}/`. Never write a machine path or a home-dir path.
- To reuse another skill's instructions, load it with the Skill tool as `monet:<other>`. Never read another skill's SKILL.md by path.
- For a userConfig value, write its placeholder once, and follow it in the next sentence with the fallback sentence that `USER_CONFIG_FALLBACKS` in `ROOT/tools/skillkit.py` gives for that key.

Write `ROOT/skills/<name>/SKILL.md`, plus any reference files.

## Step 3: Register the skill in the monet agent

In `ROOT/agents/monet.md`, write `monet:<name>` in both places:

1. Add one row to the Quick-reference routing table (`Trigger phrases | Skill | Required posture | Write-gated?`), with `` `monet:<name>` `` in the Skill column, next to the rows of its siblings.
2. Add one dispatch line. When a numbered step already covers the skill's domain, add a bullet there: `- If the user asks to "<trigger 1>" / "<trigger 2>", apply` `` `monet:<name>` ``, then one line on what it does. Otherwise insert a new numbered step immediately before the step labelled Safety defaults. Find that step by its text, never by its number, because the list may have grown or been reordered, and renumber the steps after it.

## Step 4: Verify and report

1. Run `make -C ROOT check`. Its lint checks the frontmatter, the description, every `${CLAUDE_SKILL_DIR}` and `${CLAUDE_PLUGIN_ROOT}` path, and that each `monet:<name>` the agent routes to exists. Its scans check portability, privacy, and prose. Fix each failure at its cause, then run it again until it exits 0. Never weaken a check to get green.
2. Report the files written, the trigger phrases, and the `make check` result. Don't commit: shipping goes through `monet:ship`.

## Constraints

- Write only under ROOT, and only `skills/<name>/` and `agents/monet.md`, plus a sibling's description line from Step 1.
- Don't claim a check passed without its output.
- Keep output bounded and skimmable.
