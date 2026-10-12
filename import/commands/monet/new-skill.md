---
name: monet:new-skill
description: Generate a new monet skill from the standard template. Gathers skill details interactively, writes the skill and its /monet command, and registers it in the monet agent.
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
<objective>
Generate a new monet skill from the standard template and wire it the way `.claude/README.md` ("Authoring a skill") describes: the skill, its `/monet:<name>` command, a routing row and a dispatch line in the monet agent, and the README tables. Finish with `bin/check-monet-skill.py`, which checks every one of those surfaces.
</objective>

<execution_context>
@~/.claude/templates/skill-template.md
</execution_context>

<context>
Skill name argument: $ARGUMENTS

- If `$ARGUMENTS` holds a skill name (kebab-case: lowercase letters, digits, and hyphens only), use it and skip the name question in Step 1, but still stop on a collision as that question describes.
- Otherwise ask for the name in Step 1.

ROOT is the checkout of the config repo you write into: `~/.config` by default, or a worktree of it when the work happens on a branch (`git -C ~/.config worktree list` shows them). Every path below is relative to ROOT, never to the current directory. Inside another repo, `.claude/` is that repo's own folder.
</context>

<process>
## Step 0: Resolve ROOT

1. Take the worktree the user named, else `~/.config`. Check that `git -C ROOT rev-parse --show-toplevel` prints ROOT and that `ROOT/.claude/agents/monet.md` exists. If either check fails, stop and ask for the path.
2. If ROOT is the live checkout (`~/.config`), its files go live as soon as they are written, and its branch may hold unpushed work (`git -C ROOT status -sb`). Offer a worktree first: `git -C ~/.config fetch origin main && git -C ~/.config worktree add -b wawen/<name> .claude/worktrees/<name> origin/main`.

## Step 1: Gather skill details

Ask one question at a time with AskUserQuestion; each builds on the answers before it.

1. **Name** (unless given): kebab-case, matching `^[a-z][a-z0-9]*(-[a-z0-9]+)*$`, named for the artifact it produces (README, "Name it"). Stop on a collision: `ROOT/.claude/skills/<name>/` or `ROOT/.claude/commands/monet/<name>.md` already exists.
2. **Purpose**: 1-2 sentences, starting with a verb.
3. **Trigger phrases**: 2-4 phrases. Grep the Quick-reference routing table in `ROOT/.claude/agents/monet.md` for each. A phrase another skill already owns needs a disambiguator, not a duplicate.
4. **Siblings**: the skills it could be confused with. Each gets a disambiguator line in the new description, and a mirror line in its own description (README, step 2), except a vendored skill whose `UPSTREAM.md` forbids edits.
5. **Shape**: a typeable `/monet:<name>` command plus routing (the common case), or routing only.
6. **Preconditions**, then **output format**: conversation, file, YAML block, or other.

## Step 2: Write the skill

Read `ROOT/.claude/templates/skill-template.md` and fill it in:

- `{{SKILL_NAME}}` and `{{SKILL_DESCRIPTION}}`. The description follows the template's comment: `Use when ...`, the literal trigger phrases, the disambiguator lines, under ~400 characters. It must parse as YAML, because a strict loader drops a skill whose frontmatter does not, so quote it if it holds `: `.
- Preconditions from the answer. For "none", remove the section.
- Steps: keep the three starter steps and fit them to the skill: context, core action, and output in the chosen format. Budget 70-100 lines; push checklists and domain detail into `reference/`, read lazily from a step.
- Constraints: keep the two starters and add one specific to the skill.
- Tone rules: only for a skill that gives feedback or critique. Otherwise remove the section.

Write `ROOT/.claude/skills/<name>/SKILL.md`.

## Step 3: Write the command wrapper (command shape only)

Write `ROOT/.claude/commands/monet/<name>.md` in the shape of `commands/monet/paint.md`: frontmatter `name: monet:<name>`, `description`, `argument-hint` when it takes arguments, and `allowed-tools` as a block list. Then `<objective>`, `<execution_context>` holding `@~/.claude/skills/<name>/SKILL.md`, `<context>` documenting `$ARGUMENTS`, and a `<process>` that reads the SKILL.md and runs it exactly as written. About 35 lines; never restate the skill's steps.

Write `~` paths only. A machine path such as `/Users/<you>/...` fails the repo's portability tests.

## Step 4: Register the skill in the monet agent

In `ROOT/.claude/agents/monet.md`:

1. Add one row to the Quick-reference routing table (`Trigger phrases | Skill | Required posture | Write-gated?`), next to the rows of its siblings.
2. Add the dispatch line. When a numbered step already covers the skill's domain, add a bullet there: `- If I ask to "<trigger 1>" / "<trigger 2>", apply \`<name>\`.`, then one line on what it does. Otherwise insert a new numbered step immediately before the step labelled **Safety defaults**. Find that step by its text, never by its number, because the list may have grown or been reordered, and renumber the steps after it.

## Step 5: Update the README

In `ROOT/.claude/README.md`, add the command row to the table between `<!-- BEGIN GENERATED:commands -->` and `<!-- END GENERATED:commands -->`: `` | `/monet:<name>` | <first sentence of the description> | ``, in alphabetical order. Add `<name>` to the matching row of the shape table: "Command **and** routing row" or "Routing row only".

## Step 6: Verify and report

1. Run from ROOT: `python3 bin/check-monet-skill.py <name>` (add `--no-command` for the routing-only shape) and `bash .claude/bin/check-portability.sh`. Fix every failure, then run them again.
2. Report the files written, the trigger phrases, and the check results. Do not commit: shipping goes through `/monet:ship`.
</process>
