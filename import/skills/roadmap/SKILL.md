---
name: roadmap
description: Use when the user asks to refresh, regenerate, or update a work stream's roadmap page, says "refresh the roadmap", or asks for a roadmap.html for a stream. Not for general visualization (that is paint). For building a plan's tasks with agents, use gang instead.
---

## Overview

Generate **one living `roadmap.html` per work stream** from data, not by hand-placing SVG. The page is
undated and overwritten in place. All geometry is computed, so growing a lane can never silently clip
content, the failure mode this skill exists to remove.

This skill **overrides paint's Step 5 filename convention** (`{topic}-{date}.html`, `-2` on collision).
Roadmap pages are deliberately living documents at a fixed path.

## When to use / not

- Use for: "refresh the roadmap", "regenerate the roadmap page", "update roadmap.html for <stream>".
- Not for: "visualize this", "make this visual", "paint me a page" → those are **paint**. This skill
  only ever emits `roadmap.html`.

## Step 1: Resolve inputs

```
/monet:roadmap [<stream-dir>] --plan <path>
```

- `<stream-dir>` is **the directory that holds, or will hold, `roadmap.html`**. Defaults to cwd.
- `--plan <path>` is **required**. Do not infer it. The two existing streams place the plan
  differently (envtest keeps it one level *above* the stream dir, PV/PVC keeps it in a `plan/`
  subdirectory), so any convention guess fails silently on one of them.

If `--plan` is missing, stop and ask for it. If the file does not exist, stop and say so.

## Step 2: Parse the plan into items

Read `reference/plan-parsing.md` and apply it: match the plan's table on column *names* (formats differ
across streams), build the `Item` array, and **fail loudly** if no table carries both a recognizable ID
column and a recognizable status column. Never render a half-empty page: it reads as authoritative.

Relationships are lane membership plus prose in the caption, **never drawn edges**.

## Step 3: Verify freshness

Verify **non-terminal PRs only**. Anything already rendered as merged is skipped, so a refresh stays cheap.

```
gh pr view <n> --repo <owner/repo> --json number,state,mergedAt
```

Infer `<owner/repo>` from the stream dir's git origin: `git -C <stream-dir> remote get-url origin`,
then take the `owner/repo` path from the URL, minus any `.git` suffix. There is no `--repo` argument.
If the stream dir has no origin, or the origin is not a GitHub remote, stop and say so rather than
guessing a repo.

A state change updates three things **together** (partial updates are the observed rot):

1. the visible PR label text
2. the status dot class (`.ok` merged, `.live` open)
3. the verified-on date

Emit **exactly one** freshness element with one class carrying the stamp. Do not scatter the date across
hero subtitle, footnote, and eyebrow the way the existing family does.

## Step 4: Compute geometry

Read `reference/geometry.md` and apply it. Nothing in it is optional; the `viewBox` height and every box
width are computed. Run the per-lane capacity check before emitting and stop on overflow rather than
shrinking or truncating.

## Step 5: Emit

- Start from `reference/template.html`. Substitute `{{TITLE}}` and, if the stream declares states beyond
  `done`/`now`/`later`, `{{EXTRA_STATE_RULES}}`.
- Generate the `aria-label` narration **from the same items array** that produces the SVG. Never
  hand-write it: the current 619-character hand-written one drifts the first time an item changes lane.
- Write `<stream-dir>/roadmap.html`, overwriting in place.
- If the file already exists, warn before every overwrite, whoever authored it: regeneration can change
  the layout, most visibly for PV/PVC, whose irregular lane pitch becomes uniform.

Report: item count, box count, lane count, computed `viewBox`, which items were grouped into one box,
and which PRs were re-verified.

## Constraints

- Never re-derive the preamble from the paint skill. `reference/template.html` encodes about a dozen
  deliberate deltas from the flavor defaults (`--max-width:960px`, `--section-pad-y:40px`,
  `--card-pad:16px`, dropped tokens) that a fresh derivation would lose.
- Every number on the page comes from the plan or from `gh`. Never invent state.
- No publishing to claude.ai. These pages live under a gitignored `.planning/` and are local-only.

## Common mistakes

- Hand-placing a coordinate "just this once": that is the clipping bug returning.
- Measuring bottom pad from the `pr` caption instead of the last rect (adds 46px of dead space).
- Re-verifying merged PRs, which makes a refresh needlessly slow.
- Writing a dated filename. This page is undated and lives at a fixed path.
