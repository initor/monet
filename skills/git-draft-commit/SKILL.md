---
name: git-draft-commit
description: Use when the user asks to draft a commit message for the current uncommitted changes. Drafts only; to make the commit, use monet:ship's commit-only path.
allowed-tools:
  - Skill
---

## Voice

The commit subject and body go out under the user's name, so they run through `monet:voice`. Before drafting, load it with the Skill tool: skill `monet:voice`, args `--no-ste`. Load it fresh rather than relying on what you remember of it, because holding a rule is not the same as applying it. Treat what it loads as the rules for the message you draft here, not as a request to edit earlier prose. Apply it to the subject line and every bullet.

Register for this surface: **Commit message**. It governs how the words read. This skill still owns the Conventional Commits format and the scope selection.

## Preconditions

Only run inside a git repo. If not, ask the user to `cd` into the repo or run `monet:cwd-context`.

### Branch policy gate

Before proceeding, check the current branch (`git symbolic-ref --short HEAD 2>/dev/null || echo DETACHED`), or reuse `changes_digest.branch_policy.violation` when a digest is at hand.

1. If result is `DETACHED` (`detached_head`): **STOP. Do not draft a commit.**
   Tell the user: "You're on a detached HEAD. Create a branch first: `git checkout -b ${user_config.branch_prefix}<descriptive_slug>`" If the prefix above is empty or starts with `${user_config`, it isn't configured, so use no prefix.
2. If on `main` or `master` (`on_main`): **STOP. Do not draft a commit.**
   Tell the user: "You're on `main`. Create a feature branch first: `git checkout -b ${user_config.branch_prefix}<descriptive_slug>`" If the prefix above is empty or starts with `${user_config`, it isn't configured, so use no prefix.
3. If a prefix is set and the branch does not start with it (`missing_branch_prefix`): **warn and continue**: "Branch `<name>` is off the `${user_config.branch_prefix}` convention (`git checkout -b ${user_config.branch_prefix}<slug>` to fix)." If the prefix above is empty or starts with `${user_config`, it isn't configured, so use no prefix. With no prefix, skip this check. Drafting a message is read-only, so don't block on it.

## Step 1: Build a Changes Digest

If a recent `changes_digest` exists in the conversation, reuse it.

Otherwise, run `monet:git-changes` with the Skill tool to produce a `changes_digest`.

## Step 2: Choose Conventional Commit fields

Conventional Commits subject format:
- `type(scope): description`
- If scope omitted: `type: description`

Rules:
- `type`: use `changes_digest.suggested_commit_type` when available.
- `scope`: choose best-effort (see below).
- `description`: imperative mood ("add", "fix", "remove"); keep the whole subject line under ~70 chars.

### Scope selection (it depends on the change)

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
