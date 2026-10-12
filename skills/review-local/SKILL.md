---
name: review-local
description: Use when the user wants to review uncommitted or branch changes before opening a PR ("review my changes", "check before I push", "pre-PR review"). Spawns parallel domain-expert sub-agents across six dimensions. For an already-posted GitHub PR use the pr skill instead. For a task graph that agents build and review end to end, use gang instead. Read-only until asked to fix.
argument-hint: "[--domain <expertise>] [--agents <N>] [--model <model>]"
allowed-tools:
  - Read
  - Bash
  - Glob
  - Grep
  - Agent
  - AskUserQuestion
---

## Overview

Catch issues in local changes before they reach a PR by reviewing the branch diff through several focused expert lenses in parallel, then reporting a deduplicated, severity-sorted set of findings.

## When to use

- The user has uncommitted or branch-local changes and wants a review before pushing or opening a PR.
- Symptoms: "review my changes", "check this before I push", "pre-PR review", "look over my diff".
- Not for an already-posted GitHub PR: use the **pr** skill instead (it pulls the PR from GitHub and drafts review comments).

## Quick reference

Each finding is tagged with one of these review dimensions (plus the cross-file coherence pass):

| Dimension | Focus |
|---|---|
| Unit test coverage | New/changed public behavior is covered; edge and error paths tested. |
| Coding conventions | Matches project style and the language's idioms. |
| Pattern adherence | Follows existing package patterns; reuses helpers instead of duplicating. |
| Module placement | Code lives in the right package/layer; dependencies flow correctly. |
| Best practices | Security, resource cleanup, concurrency safety, error handling, performance. |
| Simplification | Removable complexity, dead code, premature abstraction (the /simplify lens). |
| Cross-file coherence | Changes tell a consistent story; no missed callers or drifting contracts. |

## Common mistakes

- Modifying code during the review phase: Steps 1–7 are strictly read-only; only fix in Step 8 when asked.
- Inventing findings: every finding must cite specific code from the diff.
- Nitpicking formatting when the project has a formatter (gofmt, prettier, black).
- Flagging generated code (protobuf, mocks, vendored deps) unless the generation config is wrong.
- Routing to the **pr** skill's job: this skill reviews local changes and never posts to GitHub.

## Arguments

Arguments: $ARGUMENTS

Parse these flags from `$ARGUMENTS` and carry them into Step 2:

- `--domain <expertise>`: the domain expertise for the sub-agents (for example, `--domain "distributed systems"`).
- `--agents <N>`: the number of review agents to spawn (default: 3).
- `--model <model>`: the sub-agent model, recorded as `agent_model`. Accepted values: `opus`, `sonnet`, `haiku`. Default: `opus`.

If `$ARGUMENTS` is empty, Step 2 asks for the domain and the agent count. Run from the root of the repository that holds the local changes.

## Preconditions

Only run inside a git repo. If `git rev-parse --is-inside-work-tree` fails, tell the user and stop.

There must be local changes to review. Check:

```bash
# Uncommitted changes (staged + unstaged)
git diff --stat HEAD
# Untracked files (git diff skips them)
git ls-files --others --exclude-standard
# Commits ahead of base branch (origin/HEAD, else main, as in Step 1)
git log --oneline origin/HEAD..HEAD 2>/dev/null || git log --oneline origin/main..HEAD 2>/dev/null
```

If all are empty (no uncommitted changes, no untracked files, and no commits ahead), tell the user and stop.

## Step 1: Detect changes and base branch

### Determine the base

```bash
BASE_BRANCH=$(git symbolic-ref refs/remotes/origin/HEAD 2>/dev/null | sed 's@^refs/remotes/origin/@@')
BASE_BRANCH="${BASE_BRANCH:-main}"
MERGE_BASE=$(git merge-base HEAD "origin/$BASE_BRANCH" 2>/dev/null || echo "HEAD~1")
CURRENT_BRANCH=$(git branch --show-current)
COMMITS_AHEAD=$(git rev-list --count "$MERGE_BASE"..HEAD 2>/dev/null || echo "0")
```

### Collect the full diff

The review scope is all changes on this branch (committed + uncommitted) relative to the base:

```bash
git diff "$MERGE_BASE" --stat
git diff "$MERGE_BASE"
```

If the diff is empty (can happen on main with only uncommitted changes), fall back to:

```bash
git diff HEAD --stat
git diff HEAD
```

`git diff` skips untracked files, so list them too and review each one's full content as an added file:

```bash
git ls-files --others --exclude-standard
```

Record:
- **changed_files**: list of files with change type (A/M/D)
- **branch**: current branch name
- **commits_ahead**: number of commits ahead of base
- **additions / deletions**: total line counts

## Step 2: Gather review configuration

### Domain expertise

If `--domain` flag was provided, use it. Otherwise use AskUserQuestion:
- "What domain expertise should reviewers have? (e.g., distributed systems, kubernetes controllers, java concurrency, or 'general')"

### Agent count

If `--agents` flag was provided, use it. Otherwise use AskUserQuestion:
- "How many parallel review agents? (default: 3)"

### Model

If `--model` flag (or `agent_model`) was provided, use it. Otherwise default to `opus`.

Accepted values: `opus`, `sonnet`, `haiku`. These map directly to the Agent tool's `model` parameter.

Record `domain`, `agent_count`, and `agent_model`.

## Step 3: Map codebase context

Before spawning reviewers, gather context that all agents need. Do this quickly (read, don't analyze):

1. **Project conventions**: Read CLAUDE.md, .editorconfig, linter configs (e.g., .golangci.yml, .eslintrc, .prettierrc) from the repo root.
2. **Language/framework**: Detect from go.mod, package.json, pom.xml, Cargo.toml, etc.
3. **Neighbor patterns**: For each changed file, read 1-2 sibling files in the same directory to capture local conventions.
4. **Test patterns**: Find test files adjacent to changed files. Note the test framework, naming convention, and typical test structure.

Package as a `codebase_context` block (conventions, language, patterns, test style) that every agent receives.

**Parallelism:** Read neighbor files and test patterns concurrently.

## Step 4: Spawn N parallel review sub-agents

### File assignment

Distribute changed files across `agent_count` groups:
- Balance by diff size (line count), not just file count
- Keep related files together when possible (e.g., `foo.go` and `foo_test.go` in the same group)
- Non-test, logic-heavy files get priority in smaller groups for deeper analysis

### Agent dispatch

Dispatch all `agent_count` agents in parallel using the **Agent tool**. Pass `model: agent_model` explicitly to each Agent call. Each agent's prompt MUST begin with this thoroughness calibration:

> "Apply maximum analytical depth. Examine every edge case, trace every code path, and question every assumption. Do not skim. Read each line as if a production incident depends on it."

Each agent:
- **Persona**: "You are a distinguished {domain} engineer conducting a pre-PR review. You are thorough, principled, and constructive."
- **Receives**: its assigned files, the full diff for those files, the `codebase_context` block
- **Reviews against ALL six dimensions** for its assigned files. You are the {domain} expert. Apply the full senior-engineer checklist each dimension implies; the focus statement is the lens, not the limit:

1. **Unit test coverage**: new/changed public behavior, edge cases, and error paths are tested; flag untested public APIs by name and line.
2. **Coding conventions**: matches the project's established style and the language's idioms (Effective Go, idiomatic Java, Pythonic patterns).
3. **Pattern adherence**: follows existing package/module patterns and reuses helpers rather than duplicating; new patterns need a justified reason.
4. **Module placement**: code lives in the right package/layer with dependencies flowing the right way (no circular deps).
5. **Best practices**: security, resource cleanup, concurrency safety, error handling, and performance (the standard senior-review hazards: OWASP, leaks, races, swallowed errors, N+1).
6. **Simplification** (the /simplify lens): removable complexity, dead code, over-engineering, or premature abstraction that can go without losing clarity.

### Agent output format

Each agent returns a JSON array of findings:

```json
[{
  "file": "path/to/file.go",
  "line": 42,
  "dimension": "test-coverage|conventions|patterns|placement|best-practices|simplification",
  "severity": "blocker|suggestion|nit",
  "title": "Short description",
  "rationale": "WHY this matters: the underlying principle, not just the rule",
  "suggestion": "What to do instead (with optional code snippet)"
}]
```

### Cross-file coherence agent

Dispatch one additional agent concurrently that reviews ALL changed files together:

- Do changes across files tell a coherent story?
- Are there inconsistencies between files? (e.g., different error handling styles in the same PR)
- Are there missing changes? (updated a function signature but not its callers)
- Are API contracts between changed files consistent?
- Do the changes introduce any architectural drift?

This agent returns findings in the same JSON format with `dimension: "coherence"`.

## Step 5: Merge and deduplicate findings

Collect all agent results. Merge into a unified review:

1. **Deduplicate** findings that overlap between agents (same file + line + same concern).
2. **Sort** by severity: blockers first, then suggestions, then nits.
3. **Group by dimension** for the visual report (7 groups: the 6 dimensions + coherence).
4. **Cap** at 20 findings total. If more exist, keep top 20 by severity and note "N additional minor items omitted".
5. **Compute per-dimension counts** for the summary.

## Step 6: Visualize with monet:paint

Invoke `monet:paint` via the Skill tool. The paint skill harvests the structured review findings from the current session context.

**Output path:** Prefer `.planning/tmp/review-local/` in the repo root. If `.planning/` does not exist (e.g. a repo that doesn't use it), fall back to a session/temp dir so the run still succeeds:

```bash
if [ -d .planning ]; then
  OUT_DIR=.planning/tmp/review-local
else
  OUT_DIR="${TMPDIR:-/tmp}/review-local"
fi
mkdir -p "$OUT_DIR"
```

Paint writes the HTML into `$OUT_DIR`. Do NOT invoke the playground skill.

## Step 7: Present summary

Present a chat summary headed "Local Review Summary" containing:

- **Branch** and commits-ahead-of-base; **files changed** with `+`/`-` line totals; a one-word **verdict** (Ready for PR / Needs Work / Major Issues).
- A per-dimension breakdown listing the finding count (and blocker count) for each of the seven groups: unit test coverage, coding conventions, pattern adherence, module placement, best practices, simplification, cross-file coherence.
- **Top concerns:** the three highest-severity findings, each as `file:line` + title + one-line rationale.
- **Stats:** total findings split into blockers / suggestions / nits.

Then state: **"Review complete. No code has been modified. Visual report saved to `$OUT_DIR`. Let me know which findings to address, or say 'fix all' to work through them."** (substitute the actual path from Step 6).

## Step 8: Iterate (interactive)

After presenting the review, the user may ask to fix findings:

- **"Fix all"**: work through findings by severity (blockers first, then suggestions). Skip nits unless the user asks.
- **"Fix #3"** or **"Fix the test coverage issues"**: address specific findings or a specific dimension.
- **"Re-review"**: after fixes, re-run Steps 4-7 to verify the fixes and catch any new issues.
- **"Ship it"**: user is satisfied; suggest using `monet:ship` to commit and create the PR.

When implementing fixes:
- Follow the craftsmanship bar: existing patterns, clean abstractions, minimal diff.
- Explain each fix briefly (what changed, why).
- Re-run affected tests after fixes.
- Do NOT batch-commit fixes without user confirmation.

## Constraints

- **NEVER modify code during Steps 1-7.** The review phase is strictly read-only; only modify code in Step 8 when the user explicitly asks.
- Don't comment on unchanged code unless it's directly impacted by the changes.
- Keep findings bounded (max 20) and actionable. Quality over quantity.

(See **Common mistakes** above for the don't-cite, don't-nitpick-formatting, and don't-flag-generated-code rules.)

## Tone rules

- Distinguished engineer voice: authoritative, thorough, constructive.
- Frame findings as observations: "this introduces...", "consider whether...", "the existing pattern here is..."
- Acknowledge good patterns: "good use of...", "this aligns well with...", "solid approach to..."
- Explain WHY behind every finding: the principle, not just the rule.
- Never use "wrong" or "bad" alone: pair with the specific issue and a constructive alternative.
- When a finding is borderline, acknowledge the trade-off: "this could go either way, but leaning toward X because..."
