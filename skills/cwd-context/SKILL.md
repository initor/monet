---
name: cwd-context
description: Use when a session or skill needs to know whether the current working directory is a git codebase or a non-git workspace (defaults to operation). Classifies it and emits a reusable session_context YAML block.
---

## What to do

You are running inside Claude Code with a current working directory (CWD).

Your job: classify the CWD and emit a small structured `session_context:` YAML block that future steps can rely on.

## Step 1: Detect whether this is a git repo (lightweight)

Run a single git check:

```bash
git rev-parse --is-inside-work-tree 2>/dev/null || echo false
```

- If it prints `true` → classify as `kind: codebase`, `confidence: high`
- Otherwise → classify as `kind: operation` (default)

Note: This check works from the repo root, from any subfolder, and from a linked worktree.

## Step 2: Classification rules

**If inside a git work tree** → `kind: codebase`

**If not inside a git work tree** → `kind: operation` (default)

Only classify as `docs` if the CWD is not inside a git work tree and the **user explicitly mentions a Google Doc**, e.g.:
- a `docs.google.com` link
- "Google Doc" + a doc link/ID
- "read this doc" + a doc link/ID

Otherwise, even if the directory contains markdown files, keep the default `operation` posture.

Inside a git work tree, codebase wins: a named Google Doc never changes `kind: codebase`, so the git skills and `repo_owner` stay available. Mention the doc in `notes`.

## Step 2.5: Detect repo owner (only when `kind: codebase`)

Skip this step if `kind` is not `codebase`.

Run one command to get the remote URL:

```bash
git remote get-url origin 2>/dev/null
```

Classify `repo_owner` from the remote URL:

- `work` when a loaded rule (for example a work profile's routing rule) says the remote, or a
  directory with no remote, belongs to a work repo.
- `personal` otherwise, including when no such rule is loaded.

## Step 3: Emit Session Context YAML (always)

Return a YAML block like this:

```yaml
session_context:
  cwd: "<current working directory>"
  kind: "codebase|operation|docs"
  confidence: "high|medium|low"
  notes: "<1 sentence on why>"
  is_git_repo: true|false
  repo_owner: "work|personal"  # only present when kind: codebase
```

Note: Branch, origin, and detailed git info are deferred to `monet:git-changes` when needed.

## Step 4: Recommend next best tool posture

After the YAML block, add 3–6 bullets:

- If `codebase`: recommend `monet:git-changes`, `monet:git-draft-commit`, `monet:git-draft-pr`; if the user also named a Google Doc, recommend the Google Docs tools for that doc too
- If `docs`: recommend the Google Docs tools this session has (a loaded rule may name one)
- If `operation`: recommend evidence-first capture (bounded commands, save outputs into files)

## Constraints

- Be fast: one git check (`git rev-parse --is-inside-work-tree`), plus `git remote get-url origin` only when `kind: codebase`.
- Run no other git commands here: defer to `monet:git-changes` for detailed info.
- If unsure about docs vs operation and the user hasn't mentioned a Google Doc, keep default `operation` and set `confidence: medium`.
