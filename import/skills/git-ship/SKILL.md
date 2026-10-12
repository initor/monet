---
name: git-ship
description: Use when the user wants to commit and open a PR in one step (asks to "ship", "land", "open a PR", or push-and-PR the current branch's uncommitted changes or commits ahead of main), or asks to "make a commit", which takes the commit-only path. For a gang run's wave PRs, use gang instead.
---

## Overview

One workflow: detect the git state, manage the branch, draft the commit and PR, show the drafts once, and on one explicit yes stage, commit, push, and open the PR end to end. Commit and PR drafting are delegated to `git-draft-commit` and `git-draft-pr` (the single sources of truth for those formats); this skill orchestrates and gates the writes.

## Voice

Steps 4 and 5 dispatch `git-draft-commit` and `git-draft-pr` as sub-agents, and each of those reads `voice` itself. The drafted commit and PR are therefore already voice-governed before they reach Step 6, and nothing here needs to re-apply them.

Here, `voice` covers only the prose *this* skill writes directly: the Step 6 rationale bullets, the text around the drafts, and the Step 7 question. Register for those: **Chat, instructing an agent**. With the `ste-lite` output style on, they follow the STE-lite card instead, because they are replies to Wayne. The commit message, PR title, and PR body keep the registers that `git-draft-commit` and `git-draft-pr` set. The style does not change them, even in a revision during Step 6 or 7.

Read `~/.claude/skills/voice/SKILL.md` before drafting. Expand `~` to the current home directory and read its referenced files explicitly.

## When to use

- "ship this", "land it", "commit and open a PR", "push and PR", "open a PR".
- "make a commit": the **commit-only path**. Skip Step 5, present only the commit draft in Step 6, and in Step 7 offer "Commit" / "I'll handle it." A yes stages and commits; nothing is pushed.
- There are uncommitted changes (staged, unstaged, or untracked) **or** commits ahead of `main`.
- For *just* a commit message use `git-draft-commit`; for *just* a PR body use `git-draft-pr`; to review before shipping use `monet:review-local`.

## Preconditions

Only run inside a git repo (`git rev-parse --is-inside-work-tree`). Below, `<trunk>` is `main`, or `master` when the repo has no `main` (`git rev-parse --verify --quiet main`). If `git status --porcelain` is empty **and** `git log --oneline <trunk>..HEAD` is empty, there's nothing to ship: say so and stop.

## Step 1: Detect environment

Run these bounded, read-only queries in parallel:

```bash
git symbolic-ref --short HEAD 2>/dev/null || echo DETACHED
git status --porcelain=v1
git status -sb
git remote -v
git log --oneline -5
```

Extract: current **branch** (if `DETACHED`, prompt to create `wawen/<slug>` and stop), whether **staged/unstaged** changes exist, any **untracked** files (`??` lines), the **remote**, and recent commits for style.

## Step 2: Branch management

- **On `main`/`master`** → STOP drafting. AskUserQuestion: create `wawen/<suggested-slug>` (suggest from the changes) or let the user name it (`wawen/` prefix). Then `git fetch origin`. If `git log --oneline origin/<trunk>..HEAD` is empty, run `git checkout -b wawen/<name> origin/<trunk>`, so the branch is cut from the freshly fetched upstream trunk (uncommitted changes carry over unless they conflict with it). If it lists commits, local `<trunk>` holds unpushed work: say so, and cut with `git checkout -b wawen/<name>` from HEAD so those commits come along.
- **On a non-`wawen/` branch** → warn it's off-convention, but allow proceeding.
- **On a valid `wawen/` branch** → AskUserQuestion: stay, or create a new branch (cut the same way, from a freshly fetched `origin/<trunk>`).
- **Merge commits** (`git log --oneline --merges <trunk>..HEAD`) → warn to `git rebase <trunk>` before the PR.

## Step 3: Analyze changes

Produce a `changes_digest` by running the `git-changes` skill (or, inline, the same bounded reads: `git diff [--cached] --name-status`/`--stat`, plus `git diff -U3 -- <file> | head -200` for the top 3–5 files, `git ls-files --others --exclude-standard | head -50` for new files; and `git log --oneline <trunk>..HEAD` / `git diff --stat <trunk>..HEAD` for PR context). The digest yields themes, risks, `suggested_commit_type`, and `scope_candidates`, and lists new files under `untracked_files`. The drafts cover the untracked files, but none is staged until Step 7 says so.

## Step 4: Draft the commit message

Run `git-draft-commit` against the digest. It owns the Conventional-Commits format, scope selection, and alternates. Don't re-specify them here.

## Step 5: Draft the PR title and body

Run `git-draft-pr` against the digest. It owns the canonical PR template (lead-with-what, capped Summary/Details, optional single diagram) and the work-vs-personal branch; don't re-specify them here.

**Parallelism:** Steps 4 and 5 are independent; dispatch `git-draft-commit` and `git-draft-pr` as concurrent sub-agents against the shared digest, then collect both.

## Step 6: Present output

Copy-paste-ready:
- **Commit message**: primary in a `text` code block; alternates below.
- **Pull request**: title bolded on its own line; body in an `md` code block (no extra indentation; Mermaid fenced with `mermaid`), exactly what pastes into GitHub's description field.
- **Rationale**: 1–3 bullets, only if a non-obvious type/scope/risk decision was made.
- **New files**: if `untracked_files` is non-empty, list them, so the Step 7 question can ask which to stage.

Show the drafts once. Revise them if Wayne asks, but don't re-present them as a second gate.

## Step 7: One confirmation, then run end to end

This is the one gate for stage, commit, push, and PR: none of them runs before it. Ask once, with AskUserQuestion, based on state:
- Uncommitted: "Ship: stage, commit, push, and open the PR" / "Commit only" / "I'll handle it."
- Committed, unpushed: "Push and open the PR" / "Push only" / "I'll handle it."
- Commit-only path: "Commit" / "I'll handle it."

If `untracked_files` is non-empty, the same AskUserQuestion call carries a second, multi-select question: which new files to stage (AskUserQuestion takes at most four options, so group them by folder when there are more). Stage none it doesn't name.

A yes runs every write it covers, in order, with no further confirmation between them:
1. Stage → `git add <specific-files>`: the tracked changes plus only the new files Wayne picked (prefer specific files over `-A`).
2. Commit → `git commit -m "<message>"`.
3. Push → `git push -u origin <branch>`.
4. PR → `gh pr create --title "<title>" --body "<body>"`, then return the PR URL.

If a write fails, stop there, report what ran, and print the remaining commands. If the user picks "I'll handle it", stop and print the suggested commands.

## Common mistakes

- Drafting on `main` instead of creating a `wawen/` branch first.
- An over-long PR body: the lede is *what changed*, carried by `git-draft-pr`; don't re-expand it here.
- Re-specifying the commit/PR formats inline (they live in `git-draft-commit` / `git-draft-pr`).
- Claiming tests ran without evidence, or inventing a ticket number.
- Asking again between commit, push, and PR after the Step 7 yes.
- Staging an untracked file Wayne didn't pick.

## Constraints

- Never stage, commit, push, or open a PR before the Step 7 yes. One yes covers every write it names. Never stage an untracked file without asking. Never force-push.
- Prefer staging specific files over `git add -A`.
- Clean markdown output: no stray indentation or formatting artifacts.
- `wawen/` prefix for all new branches (the owner's convention).
- If the changes are too large for one commit/PR, suggest splitting before drafting.
