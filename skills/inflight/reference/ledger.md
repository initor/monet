# The in-flight ledger

The coordinator session keeps this file. `/monet:inflight` reads it and checks it against live state. The default path is `.planning/inflight.md` at the repo root.

## Template

```markdown
# In-flight items (the coordinator updates this at every dispatch, report, merge and ETA change)

Updated: 2026-01-05 15:07 PST

| # | Item | Round | Status | ETA (PST) | Waits for |
|---|---|---|---|---|---|
| 1 | Login page fix, PR #12 | 3 | Fix round from the review (2 fixes) | 16:00 | builder, then `make check` |
| 2 | Retry docs | 3 | Builder working | 15:40 | builder |

Merged this session: #10 cache key, #11 retry budget.
```

The Round column is optional. Drop it when the session has no rounds. Keep the zone of the `Updated:` line in the ETA header.

## When to rewrite it

Rewrite the ledger at each of these events, in the same turn:

- An agent is dispatched: add its row or set the row to the new step.
- An agent reports: set the status to what it reported, and the next ETA.
- A PR merges: remove its row and add `#N <short name>` to the merged line.
- An ETA changes, or the owner decides something that a row waits for.

## Rules

- **Time:** take the `Updated:` time from `date '+%Y-%m-%d %H:%M %Z'` in the same Bash call that prepares the write. Never type a time.
- **One row per item:** keep each row's `#` for the life of the item. Don't renumber the other rows when one leaves.
- **ETA:** the start time plus a measured duration of the same kind of step (the last gate of this kind, the last agent of this kind), plus any queue wait. Write `unknown` when nothing was measured. Never guess.
- **Status:** what is happening now, in plain words. Write each PR as `#N`, so the show mode can match it against the PR list.
- **Waits for:** the next thing that must happen and who does it: an agent, a gate, a merge, or the owner. Name a gate by its command (`make check`), so the show mode can find its process.
- **Edits:** change rows with Edit. Write the whole file only to create it.
- **Scope:** keep the ledger under `.planning/`, and never commit it.
