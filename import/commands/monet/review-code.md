---
name: monet:review-code
description: Review a Go or Kubernetes codebase against official conventions (Effective Go, Go Code Review Comments, K8s best practices). Produces structured, convention-backed feedback with a friendly tone.
argument-hint: "[path/to/repo]"
allowed-tools:
  - Read
  - Bash
  - Glob
  - Grep
  - Write
---
<objective>
Run the review-code skill to review a Go or Kubernetes codebase against official conventions.

This command provides direct invocation of the same code review workflow that the monet agent dispatches. Output is written to `reviews.md` at the repository root.
</objective>

<execution_context>
@~/.claude/skills/review-code/SKILL.md
</execution_context>

<context>
Target repository: $ARGUMENTS

- If `$ARGUMENTS` contains a path, verify it exists and contains a Go project (`go.mod` present), then use it as the working directory for the review.
- If `$ARGUMENTS` is empty, review the current working directory (same behavior as agent-dispatched code review).
</context>

<process>
## Step 1: Determine target directory (wrapper precondition)

Parse `$ARGUMENTS`:
- If a path is provided, verify the path exists and contains `go.mod`. If the path does not exist or has no `go.mod`, report the issue and stop.
- If empty, use the current working directory.

## Step 2: Read & execute the skill

Read the review-code SKILL.md referenced in the execution_context above and execute it exactly as written -- it is the single source of truth. Follow all Preconditions, Steps, Tone rules, and Common mistakes. Do not add, skip, or modify any steps.

## Step 3: Confirm output

The skill writes its review to `reviews.md` in the target directory. Report that this artifact was produced.
</process>
