---
name: pr
description: Use when a GitHub PR URL is shared and the user asks to review a pull request or do a code review on it before merge. Drafts copy-paste-ready comments with GitHub permalinks. Posts nothing until the user says "post all", then posts every comment as one COMMENT review. For your own un-pushed local changes, use review-local instead. For the CI and merge of a gang run's wave PRs, use gang instead.
---

## Overview

Review a published PR by fanning out domain-expert sub-agents over its diff, then hand back drafted comments. Nothing is posted to GitHub until the user says "post all". Step 9 then posts every comment as one `COMMENT` review, after re-checking the PR head and thread, and never approves or requests changes.

## Voice

Every `draft_comment` is prose that gets posted under Wayne's name, so it runs through `voice`. That file is read in full rather than merely referenced, because holding a rule is not the same as applying it.

Apply it in **Step 6**, where comment prose is actually written. The Step 5 sub-agents return structured findings rather than finished prose, so they don't need it.

Register for this surface: **PR review, someone else's code**. That is a fast surface, so contractions always and no semicolons. The Tone rules at the end of this file add the review-specific layer on top of those mechanics.

Read `~/.claude/skills/voice/SKILL.md` before drafting. Expand `~` to the current home directory and read its referenced files explicitly.

## When to use

- A GitHub PR URL was shared and the user wants a code review of it.
- The user asks to review, critique, or "look over" a pull request before merge.
- Not for your own un-pushed local working-tree changes: use **review-local** instead (it spawns experts over `git diff`, no PR URL needed).

## Common mistakes

- Posting anything before the user says "post all", posting comments one by one instead of as one review, or approving or requesting changes. Hard constraint: never.
- Commenting on test files (unless the test logic itself is buggy) or generated code (protobuf, mocks) unless its generation config is wrong.
- Nitpicking formatting when the project has a formatter (gofmt, prettier).
- Inventing issues not backed by evidence in the diff.
- Restating location, category, or severity inside `draft_comment`: GitHub already anchors a review comment to its line; that's UI chrome for the panel header, never text in the comment itself.
- Writing long, wordy comments: a comment is the finding, stated once, ideally in 1 sentence under 30 words (hard ceiling 40 words). A stacked-clause 45-word sentence is a long comment even when the sentence count says otherwise.
- Recommending a change without a `​```suggestion` block when the fix is a concrete line edit (code or docs): give the author one-click text.
- Anchoring a comment at a line that merely *mentions* the problem (a `t.Run` header, a doc comment) rather than the line the suggestion must replace.
- Wrapping a repo permalink reference in `[text](url)` markdown: paste it bare so GitHub unfurls it as a code embed.

## Preconditions

Requires `gh` CLI authenticated (`gh auth status`). If not, tell the user to run `gh auth login` and stop. Being inside the target repo is NOT required: Step 3 checks out the PR.

## Step 1: Parse PR input

Accept a full GitHub PR URL (e.g. `https://github.com/org/repo/pull/123`). Parse `owner`, `repo`, `number` with `github.com/([^/]+)/([^/]+)/pull/(\d+)`. Reject anything not matching the `github.com` host or this pattern. If input is not a valid PR URL, use AskUserQuestion: "Paste the full GitHub PR URL (e.g. https://github.com/org/repo/pull/123)". Construct the repo slug `owner/repo`.

## Step 2: Gather review configuration

- **Domain expertise**: if `--domain` not provided, AskUserQuestion: "What domain expertise should reviewers have? (e.g. distributed systems, kubernetes controllers, java concurrency)". Allow a domain or "general".
- **Agent count**: if `--agents` not provided, AskUserQuestion: "How many parallel review agents? (default: 3)". Accept a number or use 3.
- **Model**: use `--model` / `agent_model` if provided, else `opus`. Accepted: `opus`, `sonnet`, `haiku` (map directly to the Agent tool's `model`).

Record `domain`, `agent_count`, `agent_model`.

## Step 3: Checkout and gather PR metadata

Check if already inside the correct repo:

```bash
CURRENT_REPO=$(gh repo view --json nameWithOwner -q .nameWithOwner 2>/dev/null || echo "")
```

If `CURRENT_REPO` equals `owner/repo`, stay put. Otherwise clone into a temp dir keyed by owner+repo+number so concurrent reviews don't collide, reuse that clone when an earlier review of the same PR left it there (git won't clone into a non-empty directory), and stop on clone failure rather than silently continuing:

```bash
CLONE_DIR="${TMPDIR:-/tmp}/monet-pr-<owner>-<repo>-<number>"
[ -d "$CLONE_DIR/.git" ] || gh repo clone owner/repo "$CLONE_DIR" || { echo "clone failed for owner/repo"; exit 1; }
cd "$CLONE_DIR"
```

Checkout the PR (`gh pr checkout <number>`); if checkout fails, report the error and stop. Then gather metadata:

```bash
gh pr view <number> --json title,body,author,baseRefName,headRefName,url,headRefOid,files,additions,deletions,commits
```

Extract title, author, base/head branch, `headRefOid` (head SHA), changed files, additions/deletions, and commits. Construct the base blob URL `https://github.com/owner/repo/blob/<headRefOid>/`.

## Step 4: Collect the diff and assign files to agents

Collect with `gh pr diff <number>`. If very large (>2000 lines), sample strategically and read full context per file.

Split changed files into `agent_count` roughly equal groups. **Prioritization (applies to both sampling and assignment): non-test/non-generated files first, logic-heavy files over config/docs, balance groups by diff size rather than file count.** Each group receives its files, their full diff, the PR metadata, the `domain` context, and the review dimensions.

## Step 5: Spawn N parallel review sub-agents

Dispatch `agent_count` sub-agents in parallel via the **Agent tool**, passing `model: agent_model` explicitly. Each prompt MUST begin with this calibration:

> "Apply maximum analytical depth. Examine every edge case, trace every code path, and question every assumption. Do not skim. Read each line as if a production incident depends on it."

Each sub-agent acts as a `domain` expert and reviews its files against:

1. **Correctness**: logic errors, edge cases, nil/null handling, off-by-one, race conditions.
2. **Safety**: concurrency (mutexes, channels, atomics), idempotency, error propagation, resource leaks.
3. **Craftsmanship**: API/schema design, naming clarity, abstraction level, code organization.

It returns findings as a JSON array (see Step 6 for the shape). Dispatch one additional cross-file agent concurrently: do the changes tell a coherent story, are there inconsistencies, are there missing changes (updated a function but not its callers)? Merge all findings into a unified list.

## Step 6: Format review comments

`draft_comment` is the single source of truth for what gets pasted to GitHub: location, category, and severity are separate structured fields, never restated inside it. Each finding is compact:

| Field | Purpose |
|---|---|
| `draft_comment` | The GitHub-ready markdown to paste (see format below). Self-contained. |
| `location` | File path and line on the PR head, or on the base for a removed line (`start-end` for a span). Anchors the inline comment in Step 9. |
| `category` | `[correctness]`, `[safety]`, or `[craftsmanship]` (used for grouping/stats). |
| `severity` | `blocker`, `suggestion`, or `nit` (drives ordering). |
| `suggested_code` | Optional fenced snippet of the recommended change. |

The comment body IS the finding: there is no separate bold title plus explanation, and no footer. State it directly, as one thing, then references if any:

````
<the finding itself, framed directly as the comment. at most 2 sentences, ideally 1. simple words.>

```suggestion
<exact replacement for the commented line(s), see rule 2>
```

https://github.com/owner/repo/blob/<sha>/other/file.go#L42-L58
````

Use `#L<start>-L<end>` for multi-line spans. Tone (see Tone rules) applies to every `draft_comment`.

**Comment style: hard rules (apply to every comment):**

1. **One direct sentence, not a headline plus an essay.** No bold title line, no "finding + separate explanation" structure: the comment is the finding, stated once, plainly. **Target 1 sentence under 30 words; hard ceiling 40 words of prose across at most 2 sentences. Count the words.** The budget covers prose only: the suggestion block and reference lines are free. An over-budget draft is fixed by *deleting detail* (the permalink and the suggestion block carry the evidence and the mechanism), never by fusing clauses with "since/so/which" to sneak under the sentence cap. A 45-word sentence is long no matter how it's punctuated.
2. **Suggest the change.** When the fix is a concrete edit to the changed lines, include a GitHub `​```suggestion` block with the exact replacement so the author applies it in one click. This works for code **and** for docs/markdown/config. If the fix is structural (not a clean line-for-line swap), give a one-line recommended snippet or state it in words instead. Never leave a suggestion block that won't apply cleanly, and never let two suggestions target overlapping lines (consolidate into one).
3. **No footer, no location line, no category/severity restated in the comment.** The comment being posted is already anchored to its file and line by GitHub's own review-comment UI (you comment by clicking the line). Restating `file.go:42 · [category] · severity` inside the comment body is noise that doesn't belong in a PR conversation. Track file, line, category, and severity as the finding's structured fields (for grouping, ordering, and the visualizer). Never write them into `draft_comment` itself.
4. **A reference to code in this repo is a bare permalink, not a markdown link.** `https://github.com/owner/repo/blob/<sha>/path#L42` pasted on its own line auto-expands into a rendered code-snippet embed on GitHub. Wrapping it as `​[text](url)` suppresses that and leaves a plain clickable link instead. Never markdown-link a repo permalink; paste it raw, one per line, with no leading label. Reserve `[text](url)` markdown links for genuinely external references (official docs, RFCs, blog posts) that GitHub can't unfurl as code.
5. **Say it the way you'd say it out loud, not the way a model writes it.** The Voice section at the top carries the mechanics, so don't restate them here. On top of those, specific to review comments: no "not because X, but because Y" antithesis chains, lowercase sentence starts (code review is chat register, not written-down prose), and a 2-sentence budget, so spending a second sentence to land the fix is fine. A comment that needs re-reading to parse is a comment that needs rewriting. Skim the batch for stacked-clause sentences before finalizing and cut them down: splitting alone keeps the words; the word budget in rule 1 still applies after the split.

**Severity ordering**: present comments grouped: **Blockers** (must fix: correctness/safety), then **Suggestions** (craftsmanship, patterns), then **Nits** (style/naming, optional). Label each section. If there are no blockers, say so explicitly. That is valuable signal.

**Bounding**: max 15 comments; if more, keep the highest-severity and note "N additional minor items omitted". Skip things that are clearly intentional style choices unless they violate team conventions.

**Pre-render check: run before Step 7, fix violations in place.** Sweep every `draft_comment` against this list; a violation here is exactly what forces a re-render later, so catch it now:

- [ ] prose word count is inside the budget. Actually count it: target 1 sentence under 30 words, hard ceiling 40; over means cut detail, not merge clauses
- [ ] lowercase throughout, except code identifiers and proper nouns
- [ ] fix is a concrete line edit ⇒ has a `​```suggestion` block that applies cleanly to the commented lines
- [ ] repo code references are bare permalinks on their own line; external references (docs, RFCs, blogs) are `[text](url)` markdown links
- [ ] no file/line, category, or severity text inside the body

## Step 7: Visualize with monet:paint

Invoke `monet:paint` via the Skill tool; it harvests the review data already in session context (comments, PR metadata, summary). When paint asks where to save, prefer `.planning/tmp/review/<number>/`; if `.planning` does not exist in this repo, fall back to a session temp dir:

```bash
if [ -d .planning ]; then OUT=".planning/tmp/review/<number>"; else OUT="${TMPDIR:-/tmp}/monet-review-owner-repo-<number>"; fi
mkdir -p "$OUT"
```

Pass `$OUT` to paint. Do NOT invoke the playground skill.

**Comment panels are non-negotiable in the rendered page: raw, not rendered.** Instruct paint to render every finding with the **Copy panel** component from its `reference/base.md` (`.copy-panel`: markup, CSS, and copy-button JS are all defined there; use them as-is, don't improvise a panel). Per that component's contract: the header carries file/line permalink, category, and severity as UI chrome only (pills and a link, never pasted text), and the `<pre>` body holds the literal `draft_comment` verbatim as raw markdown: `​```suggestion` fences visible as text, repo permalinks as bare URLs, nothing parsed or prettified into title/prose/code-box elements. GitHub renders it after paste; this page must not. The copy button then yields the body byte for byte, so what the user sees is exactly what lands in the GitHub comment box.

Build the panels from the raw `draft_comment` strings gathered in Step 6, grouped by severity per Step 6's ordering.

## Step 8: Present summary and wait for "post all"

Present in chat a short summary: PR `#<number>: <title>`, author, a verdict (Approve / Approve with Comments / Request Changes / Comment Only), the PR's strengths (always acknowledge good work), the top 1-3 concerns if any, and stats (`<N>` comments: blockers/suggestions/nits). Also show the `review_body`: one short sentence, written under the Voice rules, that Step 9 posts as the review's top-level text.

Then state: "Review complete. No comments have been posted to the PR. Say 'post all' to post every comment above as one review, or tell me which to edit or drop first."

Post only when the user's reply is an explicit "post all", and only through Step 9. Anything else ("looks good", approval of the summary) posts nothing. A request to post only some comments means dropping the rest from the set, re-presenting, and waiting for "post all". Hard constraint.

## Step 9: Post on "post all"

Run this step only after the user says "post all". Every comment from Step 6 goes into one review with `event: COMMENT`, so the author gets one notification and nothing lands half-posted. Never `APPROVE` or `REQUEST_CHANGES`, even when the verdict was Request Changes, and never post through `gh pr comment` or `gh pr review`.

1. **Re-check the head.** Run `gh pr view <number> --repo owner/repo --json state,headRefOid`. If the PR is no longer `OPEN`, or `headRefOid` differs from the one recorded in Step 3, post nothing: say the PR moved, and offer to re-run the review against the new head (the anchors and permalinks point at the old commit).
2. **Re-check the thread.** Read what was said since the review started: `gh api repos/owner/repo/pulls/<number>/comments --paginate` for inline comments, and `gh pr view <number> --repo owner/repo --json reviews,comments` for reviews and the conversation. If a draft repeats a point already raised on the same lines, or the thread changed what a draft should say, post nothing: list those drafts, edit or drop them as the user directs, and wait for a fresh "post all".
3. **Post one review.** Build the payload with `python3` and `json.dumps` (or `jq`) so newlines and backticks in comment bodies survive, and write it to `$OUT/review.json` in Step 7's directory. Set `commit_id` to the Step 3 `headRefOid` and `body` to the Step 8 `review_body`. Add one entry per comment: `path` and `line` from its `location`, `start_line` as well for a span, `side` (and `start_side` for a span) `RIGHT` for added or context lines and `LEFT` for removed ones, and `body` set to the `draft_comment` byte for byte.

   ```json
   {
     "commit_id": "<headRefOid>",
     "event": "COMMENT",
     "body": "<review_body>",
     "comments": [
       { "path": "pkg/foo/bar.go", "line": 58, "side": "RIGHT", "body": "<draft_comment>" },
       { "path": "pkg/foo/baz.go", "start_line": 42, "start_side": "RIGHT", "line": 47, "side": "RIGHT", "body": "<draft_comment>" }
     ]
   }
   ```

   ```bash
   gh api --method POST repos/owner/repo/pulls/<number>/reviews --input "$OUT/review.json"
   ```

   GitHub accepts or rejects the review whole. On an error (a `422` usually means a line outside the diff), nothing was posted: report the error and the comment it names, fix the anchor with the user, and retry only on a fresh "post all". Never fall back to posting comments one by one.
4. **Report.** Give the review's `html_url` from the response and the number of comments posted.

## Constraints

- NEVER post before an explicit "post all", and never post any way other than Step 9's single `COMMENT` review. NEVER approve or request changes, whatever the verdict.
- Don't comment on test files unless the test logic itself is buggy, or on generated code (protobuf, mocks) unless its generation config is wrong.
- Don't nitpick formatting when the project has a formatter.
- If the PR is too large to review thoroughly, say so and suggest the author split it.
- Don't invent issues: only flag what the diff evidences. Quality over quantity.

## Tone rules

- All-lowercase, friendly, approachable, never demanding or shouting.
- Frame findings as suggestions: "consider…", "what about…", "have you thought about…".
- Acknowledge good patterns: "nice use of…", "good call on…".
- Explain the WHY behind every suggestion, with reference links (official guidelines like Effective Go or Java Concurrency in Practice, best-practice docs, relevant blogs/RFCs).
- Never use "wrong"/"bad"/"incorrect" alone: pair with the specific issue and a constructive suggestion.
- When disagreeing with an approach, present the alternative with reasoning, and assume the author had good reasons: ask "was this intentional?" before assuming a mistake.
- Keep it short. Prefer the fewest words that are still clear; lead with the point, skip the preamble.
