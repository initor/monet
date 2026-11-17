---
name: inflight
description: Use when a coordinator session with agents and PRs running is asked "what's in flight", "show the in-flight table", "where are the ETAs", or "update the in-flight ledger". Reads the ledger, checks it against live PRs and gates, and prints one table. Read-only unless asked to update. For a gang run's own state, use gang status instead.
argument-hint: "[update] [<ledger path>]"
allowed-tools:
  - Read
  - Bash
  - Edit
  - Write
---

## Overview

Show the in-flight items of a coordinator session as one table with ETAs, checked against live state. The ledger is a Markdown file that the coordinator keeps. With `update`, follow the rules that keep the ledger true.

Arguments: $ARGUMENTS

## Preconditions

- Run inside a git repo. The PR checks need the `gh` CLI, authenticated for the repo's GitHub remote. Without it, skip the PR checks and say so in one line.

## Step 1: Find the mode and the ledger

Read `$ARGUMENTS`. When its first word is `update`, the mode is update, else the mode is show. The other word, if any, is the ledger path.

Without a path, find the ledger in one Bash call:

```bash
top=$(git rev-parse --show-toplevel)
ls "$top/.planning/inflight.md" 2>/dev/null || find "$top/.planning" -name inflight.md -not -path '*/node_modules/*' 2>/dev/null
```

- One file: use it.
- No file: in show mode, say that there is no ledger and that `/monet:inflight update` starts one at `.planning/inflight.md`, then stop. In update mode, start one there.
- More than one file: list them and say to pass the path, then stop.

In update mode, go to Step 4.

## Step 2: Check the ledger against live state

Read the ledger: its `Updated:` line, the table (`#`, Item, Round, Status, ETA, Waits for), and the merged line. Then check live state, read-only, in one Bash call:

```bash
date '+%Y-%m-%d %H:%M %Z'
gh pr list --state open --limit 50 --json number,title,headRefName
gh pr list --state merged --limit 5 --json number,title,mergedAt
ps -Ao pid,etime,comm | awk 'NR == 1 || $3 ~ /(^|\/)(make|go|pytest|bazel|cargo)$|\.test$|gate/'
```

The process list shows the program name only, never its arguments, because arguments can hold tokens. When a row names a gate (for example `make check`), count its processes in the same call with `pgrep -f -- '<the gate command>' | wc -l`, which prints a count and no command line.

Compare each row with live state:

- **late**: the row has a clock-time ETA, and now is more than 15 minutes past it. A row whose ETA is not a time ("after #3", "unknown") is never late.
- **stale**: live state contradicts the row. For example, the row calls PR `#N` open or in review and `#N` is in the merged list, or the row says a gate runs and no process of that gate runs.
- The merges since the ledger update: merged PRs whose `mergedAt` (UTC) is later than the `Updated:` time, compared in one zone.

## Step 3: Print the table

Reply in the language the user writes in. Print one table with the columns item, round, status, ETA, waits for, one row for each ledger row. Add "late" or "stale (<reason>)" to the status cell where Step 2 found it.

Under the table:

- The time of the ledger update.
- The merges since then, as `#N title`, or "none".
- One line when the ledger is more than 30 minutes old.

Ask nothing and change nothing. Show mode writes no file.

## Step 4: Update the ledger

Read `${CLAUDE_SKILL_DIR}/reference/ledger.md`. Start the ledger from its template when there is none, or rewrite the existing one from what this session knows now. Follow its rules at every later dispatch, report, merge, and ETA change in this session.

## Constraints

- Don't claim results without evidence.
- Keep output bounded and skimmable.
- An empty or failed live check means unknown, never merged or stopped. Mark a row stale only on a positive contradiction, and write only the ledger, and only in update mode.
