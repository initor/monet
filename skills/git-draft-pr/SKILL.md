---
name: git-draft-pr
description: Use when the user asks to draft a pull request or write a PR description for the current branch's changes. Drafts only; to open the PR, use monet:ship.
allowed-tools:
  - Skill
---

## Overview

Draft a PR title (Conventional Commits) and a PR body that **leads with what changed and how it works**, short on purpose. This skill owns the canonical PR-body template; other skills (e.g. `monet:ship`) delegate here instead of re-specifying it.

## Voice

The PR body goes out under the user's name, so it runs through `monet:voice`. Before drafting, load it with the Skill tool: skill `monet:voice`, args `--no-ste`. Load it fresh rather than relying on what you remember of it, because holding a rule is not the same as applying it. Treat what it loads as the rules for the PR you draft here, not as a request to edit earlier prose. Apply its Step 2a invariants and its Step 2b flavor to every line of Summary, Details, and Testing Done.

Register for this surface: **PR description**. It governs how the words read. This skill still owns the template, the section caps, and the diagram rule.

## Preconditions

Only run inside a git repo. If not, ask the user to `cd` into the repo or run `monet:cwd-context`.

**Branch policy** (reuse `changes_digest.branch_policy` if present, else check `git symbolic-ref --short HEAD 2>/dev/null || git branch --show-current`):
- On `main`/`master` or detached HEAD (`on_main`, `detached_head`) → **STOP**: "PRs come from feature branches: `git checkout -b ${user_config.branch_prefix}<slug>`." If the prefix above is empty or starts with `${user_config`, it isn't configured, so use no prefix.
- Branch without the prefix (`missing_branch_prefix`) → **warn and continue** (convention, not a blocker). This check applies only when a prefix is set.
- Merge commits present (`git log --oneline --merges <trunk>..HEAD`, where `<trunk>` is `main`, or `master` when the repo has no `main`) → warn: "run `git rebase <trunk>` to linearize."

## Step 1: Build a changes digest

Reuse a recent `changes_digest` if one exists; otherwise run `monet:git-changes` with the Skill tool to produce one.

`monet:git-changes` sees only uncommitted work, so also read the commits ahead of the trunk (`git log --oneline <trunk>..HEAD`, `git diff --stat <trunk>..HEAD`). The PR covers both, and on a clean tree the commits are the whole change.

## Step 2: PR title

Conventional Commits: `type(scope): description`, under ~70 chars. Default scope = `changes_digest.scope_candidates[0]`, else omit. Offer 1 alternate line if useful. Don't ask a question.

## Step 3: PR body (lead with the change)

The reader scans the top first, so the **first line states what changed in plain, concrete terms**, never a why-hypothesis, never a file list (GitHub already shows files).

If `session_context.repo_owner` is unknown, derive it the way `monet:cwd-context` does; if still unknown, default to personal. A loaded rule may add a line to the template for work repos (for example a ticket field); apply it only when `repo_owner` is `work`.

```md
## Summary
<1–2 sentences: what changed, concretely. A short "why" clause only if non-obvious.>

## Details
- <how it works: one concrete mechanism>
- <…>

## Testing Done
- [x] <statement>
```

Hard rules (this brevity is the whole point of the skill):
- **Summary**: 1–2 sentences. Concrete WHAT. No bullet sprawl, no restating the diff.
- **Details**: ≤5 bullets, each a concrete mechanism of HOW it works. No risk/rollback prose unless a real rollback caveat exists, then one line.
- **Diagram** (optional, Step 4): at most one, at the end of Details, only when it saves words.
- **Testing Done**: a best-effort line; if unknown, `Manual/local testing not provided (please update)`.

## Step 4: Diagram only when it earns its place

A diagram should *replace* prose by showing how behavior changes. Include **at most one**, and **skip it entirely** for docs-only, config-only, or single-file mechanical changes: no diagram beats a trivial one.

When the diff changes a control flow, interaction, or state machine, draw the simplest truthful Mermaid that the file list alone wouldn't convey:
- **flowchart**: branching / retry / error handling / a new pipeline step
- **sequenceDiagram**: changed calls or ordering between components
- **stateDiagram**: new states or transitions

Keep it to 4–10 nodes; collapse everything unchanged into one node (e.g. `Existing auth…`). Back every node and edge with the diff. Use a title to label before/after if you show both. **Never** use ASCII or box-art diagrams.

## Constraints

- Don't claim tests were run without evidence.
- Don't invent ticket numbers or fill in a ticket field.
- If the change is too large for one PR, suggest splitting and draft separate titles/bodies.
