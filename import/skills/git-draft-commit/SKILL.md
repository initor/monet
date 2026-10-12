---
name: git-draft-commit
description: Use when the user asks to draft a commit message for the current uncommitted changes. Drafts only; to make the commit, use git-ship's commit-only path.
---

## Voice

The commit subject and body go out under Wayne's name, so they run through `voice`. Read that file in full before drafting rather than relying on what you remember of it, because holding a rule is not the same as applying it. Apply it to the subject line and every bullet.

Register for this surface: **Commit message**. It governs how the words read. This skill still owns the Conventional Commits format and the scope selection.

Read `~/.claude/skills/voice/SKILL.md` before drafting. Expand `~` to the current home directory and read its referenced files explicitly.

## Preconditions

Only run inside a git repo. If not, ask the user to `cd` into the repo or run `cwd-context`.

### Branch policy gate

Before proceeding, check the current branch (`git symbolic-ref --short HEAD 2>/dev/null || echo DETACHED`):

1. If result is `DETACHED`: **STOP. Do not draft a commit.**
   Tell the user: "You're on a detached HEAD. Create a branch first: `git checkout -b wawen/<descriptive_slug>`"
2. If on `main` or `master`: **STOP. Do not draft a commit.**
   Tell the user: "You're on `main`. Create a feature branch first: `git checkout -b wawen/<descriptive_slug>`"
3. If branch does not start with `wawen/`: **warn and continue**: "Branch `<name>` is off the `wawen/` convention (`git checkout -b wawen/<slug>` to fix)." Drafting a message is read-only, so don't block on it.

## Step 1: Build a Changes Digest

If a recent `changes_digest` exists in the conversation, reuse it.

Otherwise, run the `git-changes` workflow to produce a `changes_digest`.

## Step 2: Choose Conventional Commit fields

Conventional Commits subject format:
- `type(scope): description`
- If scope omitted: `type: description`

Rules:
- `type`: use `changes_digest.suggested_commit_type` when available.
- `scope`: choose best-effort (see below).
- `description`: imperative mood ("add", "fix", "remove"); keep the whole subject line under ~70 chars.

### Scope selection (your preference varies)

Pick one default scope:
- If `changes_digest.scope_candidates[0]` is non-empty, use it.
- Otherwise omit scope.

Then include up to 2 alternate subject lines using other candidates. Do **not** force a question. Just present options.

## Step 3: Output (copy/paste ready)

Return exactly:

1) **Primary commit message** in a code block:

```text
type(scope): short description

- bullet 1 (what)
- bullet 2 (why)
```

Notes:
- If scope omitted, use `type: short description`.
- Bullets should be focused and factual.

2) **Alternate subject lines** (optional)

If there are meaningful alternatives, provide 1–2 lines like:
- Alternative A: `type(repo): ...`
- Alternative B: `type: ...` (no scope)

3) **Rationale** (optional, ≤2 bullets): only when the type or scope choice is non-obvious.

4) **Suggested commands**: one line, e.g. `git add <files> && git commit -m "<subject>"` (do not run).

## Constraints

- Don't claim tests ran unless you have evidence.
- If changes are large/mixed, suggest splitting and draft 2–3 commit subjects.
- Never run `git commit` automatically.
