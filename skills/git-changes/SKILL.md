---
name: git-changes
description: Use when drafting a commit or PR, or asked "what changed?". Produces a bounded digest (files, themes, risks, scope) of the current uncommitted changes, with untracked files listed separately. Also the canonical source of branch-policy state for the git skills.
---

## Preconditions

This skill only applies inside a git repo.

If `git rev-parse --is-inside-work-tree` is false:
- Say you're not in a git repo.
- Recommend running `monet:cwd-context` or `cd` into the repo.
- Stop.

## Collect change facts (bounded)

Run (use `|| true` to avoid failing the skill):

- `git symbolic-ref --short HEAD 2>/dev/null || echo DETACHED` (for `changes_digest.branch`)
  - If result is `DETACHED`: set `changes_digest.branch` to `DETACHED` and `branch_policy.violation: "detached_head"`. Include in digest but do NOT block: this skill is observational.
- Fallback: `git branch --show-current` (for `changes_digest.branch`)
- `git status --porcelain=v1 -uno`
- `git status -sb`
- `git diff --name-status`
- `git diff --cached --name-status`
- `git diff --stat`
- `git diff --cached --stat`
- `git ls-files --others --exclude-standard | sed -n '1,50p'` (untracked files, for `changes_digest.untracked_files`; if the list hits 50, say it was cut)

If you need a little more detail, sample hunks **bounded**:
- For top 3–5 changed files only:
  - `git diff -U3 -- <file> | sed -n '1,200p'`
  - `git diff --cached -U3 -- <file> | sed -n '1,200p'`
  - For an untracked file: `sed -n '1,200p' <file>`

Untracked files count toward themes, risks, commit type, and scope, but stay in their own list: they are not staged, and the caller decides whether they belong in the commit.

## Check branch policy

The branch prefix is `${user_config.branch_prefix}`. If the prefix above is empty or starts with `${user_config`, it isn't configured, so use no prefix.

After collecting the branch name:
- If branch is `DETACHED`: keep `violation: "detached_head"` (set above).
- Else if branch is `main` or `master`: set `branch_policy.violation: "on_main"`.
- Else if a prefix is set and the branch does NOT start with it: set `violation: "missing_branch_prefix"`. This check applies only when a prefix is set.
- Otherwise: set `violation: "none"`.

Set `has_branch_prefix` to true when the branch starts with the prefix, or when no prefix is set.

Include this in the digest. Do NOT block the digest -- this skill is observational.
If there is a violation, append to the "Suggested next step" bullets:
- "**Branch policy violation**: you're on `<branch>`. Create a feature branch first: `git checkout -b ${user_config.branch_prefix}<slug>`" If the prefix above is empty or starts with `${user_config`, it isn't configured, so use no prefix.

## Produce a Changes Digest (YAML + short bullets)

Return:

1) A YAML block:

```yaml
changes_digest:
  branch: "<branch>"
  staged_files: ["..."]
  unstaged_files: ["..."]
  untracked_files: ["..."]  # new files git doesn't track yet; never folded into the two lists above
  themes:
    - "<theme 1>"
    - "<theme 2>"
  risks:
    - "<risk 1>"
  suggested_commit_type: "feat|fix|docs|refactor|chore|test|ci|build|perf"
  scope_candidates:
    - "<candidate scope 1>"  # may be empty string meaning "no scope"
    - "<candidate scope 2>"
    - "<candidate scope 3>"
  branch_policy:
    on_main: true|false
    has_branch_prefix: true|false
    violation: "none|on_main|missing_branch_prefix|detached_head"
```

2) Then a short bullet list:
- **What changed** (2–5 bullets; name any untracked files as new files)
- **Why** (1–3 bullets; label as hypothesis if inferred)
- **Suggested next step** (tests vs commit vs PR)

## Heuristics (keep simple)

### Commit type
- Only docs changed → `docs`
- Tests only → `test`
- CI/build files → `ci` or `build`
- Bugfix patterns (guard clauses, error handling) → `fix`
- New endpoints/features/flags → `feat`
- Otherwise: `refactor` or `chore` depending on nature

### Scope candidates (the scope depends on the change)

There is no single fixed scope rule: the scope depends on the change.

Generate up to 3 candidates:

1) **Directory / package scope** (best when changes cluster)
   - If most changed files share a clear top-level folder (e.g., `pkg/foo`, `service/bar`, `cmd/baz`), add that folder name.

2) **Repo-level scope**
   - If changes span many folders but are still about one product area, add a repo-ish scope (repo name or primary service name).

3) **No scope**
   - Always include an empty-string candidate (`""`) as "omit scope" when changes are cross-cutting.

## Constraints

- Don't invent "why". If not obvious from diffs, label as hypothesis.
- Keep output bounded and skimmable.
- Don't stage/commit/push.
