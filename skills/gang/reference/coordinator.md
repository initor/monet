# Coordinator

Read this file for `run`. This session is the coordinator from launch to close. `<skill>`, `<repo>`, and `<gang>` are as in `SKILL.md`. This skill's instructions call the Workflow tool, and that call is the opt-in the tool requires.

## Build first

`<slug>` is the `--slug` value. By default, it is the base name of `<repo>` in lowercase. Each run of characters outside `[a-z0-9]` becomes `-`, and a `-` at either end is dropped, so `.config` becomes `config`. A slug must match `^[a-z0-9][a-z0-9-]*$`. Stop and tell the user when a `--slug` value does not match. `<dir>` is `<repo>/.planning/gang/<slug>`, as an absolute path.

Make the default slug with the first command. Run the build before any other step, so that `<gang>` exists for `lint`, `schedule`, and `render`:

```bash
basename <repo> | tr 'A-Z' 'a-z' | sed -E 's/[^a-z0-9]+/-/g; s/^-//; s/-$//'
go -C <skill>/cli build -ldflags "-X main.skillDir=<skill>" -o <dir>/run/bin/gang ./cmd/gang
```

## Context rules

- Read only short outputs: `<gang> status`, event lines, the tail that `<gang> gate wait` prints, and the counts in a Workflow result.
- Never read raw diffs, agent transcripts, or whole logs. To learn what a long file says, send a read-only Agent subagent. It returns 10 lines or fewer.
- Never resume an agent to check on it. Probe git, `<gang> status`, and the run files instead.
- Keep one run card, made with TaskCreate: `gang <slug>: session <session>, tag <tag>, merge-on-green <yes or no>`. Update it with TaskUpdate. It survives a compaction.
- After a compaction, read this file and the run card again before the next action.
- Log each checkpoint decision, relaunch, and wave PR as one dated line under `## Log` in `<dir>/run/status.md`. Write it as `- <YYYY-MM-DD HH:MM> <note>`. `<gang> status` keeps that section when it rewrites the file.

## Launch

1. Make sure that `git -C <repo> rev-parse --show-toplevel` prints the session cwd.
2. If it does not, stop. Ask the user to start a session at the target repo root.
3. If `<dir>/plan/plan.json` is missing, stop. Tell the user to run `plan` first, or to pass the right `--slug`.
4. Run `git -C <repo> check-ignore -q .planning/gang/`. If it exits 1, do "Set up" steps 4 and 5 in `<skill>/reference/planner.md`.
5. Pick `<session>`: `coord-` and the output of `date +%Y%m%d-%H%M%S`. Make the run card.
6. Run `<gang> preflight --holder <session>`. It takes the driver lease for 12 hours.
7. A `FAIL` line for the driver lease means that another session drives this run. Stop and tell the user. If that session is gone, see "Resume in a new session".
8. Fix each other `FAIL` line, or show it to the user and stop. `SKIP gh` means a local origin with no GitHub.
9. Seed the standing orders with one `<gang> standing add "<order>"` per order. The list follows these steps.
10. In auto mode, ask the user to switch to accept-edits mode now. The first push can come when the user is away.
11. Run `<gang> schedule`. The pilot is the first task of the critical chain. The chain leaves out merged tasks, and `chain: none` means that every task merged.
12. Run `<gang> gen --tag pilot --only <pilot id>`.
13. Tell the user that the pilot takes 1 to 2 hours.
14. Launch with the Workflow tool: `Workflow({scriptPath: "<dir>/run/engine.js"})`.
15. Arm the event loop. Then wait for the Workflow completion notification.

The seed orders:

- "Only the coordinator pushes, opens PRs, or posts. Agents never do."
- "Refer to the user as the user, or with they or them."
- Each standing rule that the user gave in this session, word for word.

When the pilot segment stops, do the relaunch checklist. If the pilot task merged, run `<gang> schedule --calibrate <dir>/run/durations.tsv`, and tell the user the new range. Then launch the full run with tag `r1` and no `--only`. If the pilot did not merge, handle its stop reason, then pilot again.

## Resume in a new session

Use this when the session that drove the run is gone, for example after a crash or a closed terminal.

1. Do "Launch" steps 1 to 5. Then run `<gang> status`. Its `driver` line names the old holder.
2. Ask the user to confirm that the old session is closed. Never take over a live driver.
3. Run `<gang> lock release driver --holder <old holder>`. Skip this step when the line reads `driver free`.
4. Read `## Log` in `<dir>/run/status.md` and the newest folder in `<dir>/run/runs/`. The next tag is one above the last.
5. A segment that left no result has nothing to archive. Log that under `## Log`, and skip item 1 of the relaunch checklist.
6. Do the relaunch checklist. Its `preflight` step takes the driver lease for this session.
7. Arm the event loop.

## Event loop

- Run the Monitor tool on `<gang> watch --interval 60s`, with `timeout_ms` 1800000 and the description `gang <slug> events`.
- When the Monitor reaches its deadline, arm it again.
- Also call ScheduleWakeup for 45 minutes as a fallback heartbeat. At each wakeup, run `<gang> status` and make sure that the Monitor is armed.
- At each event, wakeup, and checkpoint, first renew the lease: `<gang> lock renew driver --holder <session> --ttl 12h`.
- At each wake, run `<gang> gates list`. Accept a default, or overrule it with a ruling in `tasks/<id>/owner.md`. Then run `<gang> gates answer <G-id> --answer <a>`.
- Ask the user only for a product or preference call. Decide all other questions, and record each one as a ruling.

| Event | Action |
|---|---|
| `WAVE_READY <k>` | Run the wave gate on the trunk tip, push the integration branch, and open wave PR k. See "Wave ready". |
| `PR_GREEN <k> <pr>` | With merge-on-green, run `<gang> wave merge <k>`. If it exits 2, tell the user the reason. Without merge-on-green, tell the user that wave PR k is ready to merge, with its URL. Ask for a merge commit, never a squash or a rebase. |
| `PR_RED <k> <pr> <checks>` | Add a CI fix task, write a stop line, and run the fix task first after the engine stops. See "Red wave PR". |
| `STALL <minutes>` | Probe read-only. Never resume an agent. See "Stall". |
| Workflow completion notification | Act on the `stop` field of the result. See "Checkpoints". |

### Wave ready

1. Run `<gang> gate run --wave`. It prints a job id.
2. Run `<gang> gate wait <job> --timeout 540s`. Run it again while it exits 75.
3. If it exits 2, the gate could not run, for example because its runner died. Run the wave gate one more time.
4. If it exits 1, the gate failed. Do not push. Handle the failure as in "Red wave PR", with the gate log in place of the PR checks.
5. If it exits 0, run `<gang> push --dry-run`, then `<gang> push`.
6. If a push exits 2, stop. Show the user the reason. Never bypass the secret scan, and never force a push.
7. If auto mode blocks the push, do not try again. Ask the user to switch to accept-edits mode.
8. Run `<gang> wave open <k>`. It prints the PR URL.

An empty `repo.gh_account` means a local origin with no GitHub. Then run `<gang> wave open <k> --dry-run` instead. It prints the PR, and no PR event follows.

### Red wave PR

1. Send one read-only Agent subagent to read the failure: `gh pr checks <pr>` and its logs, or the wave gate log.
2. It returns the cause and the files to change, in 10 lines or fewer.
3. Add a requirement `CI-<n>` to `plan/plan.json`. Its `source` is the PR URL, or the wave gate job id.
4. Add a fix task `CI<n>` with every brief field, `requirements` set to `CI-<n>`, and `wave` set to `<k>`. The fields are in `<skill>/reference/schema.md`.
5. Run `<gang> lint`, then `<gang> plan commit --message "plan: add CI<n> for wave <k>"`.
6. If an engine runs, run `<gang> standing stop "wave <k> is red. CI<n> runs next."`. Wait for the engine to stop.
7. Run `<gang> standing clear-stop`. Relaunch with `--only CI<n>`.
8. When `CI<n>` merges and no wave PR is open, do "Wave ready" again for wave `<k>`.
9. When `CI<n>` merges and wave PR `<k>` is open, push as in "Wave ready" steps 5 to 7.
10. Then relaunch with no `--only`.

`events.tsv` keeps each `WAVE_READY` key, so that line does not print twice. The `PR_GREEN` and `PR_RED` keys hold the PR head SHA, so a new push can raise them again.

### Stall

1. Run `<gang> status` and `<gang> gates list`.
2. Run `git -C <repo> log -1 --format='%h %cr' <integration>`.
3. If a gate job runs, wait. A gate can run for `limits.gate_timeout_min` minutes.
4. If `<gang> status` shows the last wave PR as `closed`, ask the user why. To open it again, run `<gang> wave open <k>`.
5. If a wave PR is open, run `gh pr view <pr> --json mergeable`. `CONFLICTING` means that `<base>` moved under the wave. Tell the user, and merge nothing.
6. If a second `STALL` comes and nothing moved, write a stop line: `<gang> standing stop "stall: <what you found>"`.
7. Wait for the engine to stop. Then handle the checkpoint.

## Checkpoints

A segment can stop with the trunk red, whatever its stop reason. So at every checkpoint, check the trunk first. `result.health` lists the segment's health runs, and each `sha` is a short SHA.

If the list is empty, or its last `sha` is not the start of `result.trunkTip`, run `<gang> gate run --health` and `<gang> gate wait <job> --timeout 540s`. Run the wait again while it exits 75. Exit 1 means `RED`. Exit 2 means that the gate could not run, so run it one more time. If the last health result is `RED`, add a repair task `R<n>` as in the `TRUNK_RED` row, before any relaunch.

| Stop reason | Action |
|---|---|
| `DONE` | Every task in scope merged. After the pilot, launch `r1`. After a full segment, keep the event loop on until the done predicate holds, then close. |
| `BLOCKED` | Send a read-only subagent to read the task's `report.md` and its newest review. Then split the task, write a ruling, or edit the plan. Lint, commit the plan, and relaunch. |
| `TRUNK_RED` | Send read-only investigators for the failing health checks. Add a repair task `R<n>`, with requirement `R-<n>` and a ruling in its `owner.md`. Relaunch with `--only R<n>`. |
| `INTERRUPTED` | A usage limit or an outage ended the segment. Archive the result. Call ScheduleWakeup until the usage reset, 1 hour at most per call. Then relaunch. Resume comes from git. |
| `STOPPED` | Act on the reason in the stop line. Run `<gang> standing clear-stop`, then relaunch. |

A ruling is one line in `<dir>/run/tasks/<id>/owner.md`: `Ruling: <what> - <why> - <cost if wrong>`. A ruling wins over the plan and the spec.

To split a task, give each part a new id `R<n>`, and move the task's requirements to the parts. Make its dependents depend on the last part, and remove the old task from `plan.json`.

The engine numbers its own repair tasks `F<nn>`. The coordinator never creates an `F<nn>` id. It adds only `CI<n>` and `R<n>` tasks.

To change the model or the effort of a role for the rest of the run, set it under `models` in `plan.json`. Never edit `<skill>/roles.json`, which holds the defaults and is replaced when the plugin updates. `<gang> roles` prints the model and the effort that each role resolves to.

A plan edit at a checkpoint needs no approval from the user. Lint it, commit it with `<gang> plan commit`, and log it under `## Log` in `<dir>/run/status.md`.

## Relaunch checklist

Before each relaunch, confirm each item:

1. The Workflow result is in `<dir>/run/runs/<tag>/result.json`, and its `rulings` are in `rulings.md` beside it.
2. Each task in scope is in one `byStatus` list with a terminal status. The lists also hold tasks merged before this segment and the engine's `F<nn>` repairs, so do not compare totals.
3. For each task in the result's `mergeLog`, `<gang> ledger check <id> <sha>` prints `CLEAN`.
4. The trunk check in "Checkpoints" ended `GREEN`. If it did not, a repair task `R<n>` is in the plan and runs next.
5. A late result from an older segment is checked against `<gang> done` and the ledger. Never merge it by hand.
6. `<gang> lint` exits 0, and `<gang> plan commit` ran after each plan edit.
7. `<gang> standing show` has no `STOP:` line.
8. `<gang> preflight --holder <session>` prints no `FAIL` line.
9. The next tag is `r<n>`, and the run card shows it. A tag matches `^[a-z0-9._-]+$`, such as `pilot`, `r1`, or `r2`.
10. `<gang> gen --tag <tag>` ran, with `--only <ids>` for a fix or a repair task.
11. Launch `Workflow({scriptPath: "<dir>/run/engine.js"})`, and make sure that the Monitor is armed.

## Close

1. Run `git -C <repo> fetch origin`.
2. Run `git -C <repo> log --oneline origin/<base>..<integration>`. It must print nothing. If every wave PR merged and it still prints commits, a wave PR was likely squashed. Tell the user.
3. Run `gh pr list --head <integration> --state open`. It must print nothing.
4. Run `jq -r '.tasks[].id' <dir>/plan/plan.json | LC_ALL=C sort | LC_ALL=C comm -23 - <(<gang> done)`. It must print nothing.
5. With an empty `repo.gh_account`, skip steps 2 and 3. Wave PRs ran as dry runs, so nothing reaches `main`.
6. If one of these commands prints a line, the run is not done. Keep the event loop on.
7. Run `<gang> status`. Then add a `## Metrics` section to `<dir>/run/status.md` by hand.
8. The metrics are the user's touches, wall clock, tokens, blockers found in review, and mutation survivors.
9. Take each number from the archived results, `<gang> ledger summary`, and the `survivors` column of `run/ledger.tsv`. Write `unknown` for a number with no source.
10. Release the lease: `<gang> lock release driver --holder <session>`.
11. Mark the run card done. Leave the run files as they are. They are the postmortem.

## Cleanup

Clean up after the close, and when `<gang> preflight` fails on the worktree count.

1. Run `<gang> worktrees > <dir>/run/cleanup.sh`. Each line is one `git worktree remove <path>` command.
2. Auto mode blocks deletions, so do not run the file. Give the user one command: `! bash <dir>/run/cleanup.sh`.
3. Never delete a branch. Leave remote branches to the user.
