# Design: gang 0.5.0, small PRs and self-drive

Date: 2026-10-29. Status: approved by the owner on 2026-10-29. This document changes the 0.4.0 spec, `docs/design/gang.md`. Where the two differ, this document wins for 0.5.0. Section 3 cites monet 0.4.0 at e6c3d59.

## 1. Intent

Three problems drive this release.

- **Wave PRs are too big.** In an earlier run on a private repo, one wave PR held 27 commits and +5,907 lines. The next one held 47 commits and +6,345/-503 lines. The owner wrote: "we should always keep pr lean (a few commits at most, e.g,. 3), and as less changes (lines) as possible." The owner then set the budget: "100 lines", with an exception "if very necessary" (translated from Chinese).
- **A run still needs the owner.** The owner wants a run to drive itself from start to end, with the automatic handoff. Today the relay starts only at launch, and several steps wait for the owner.
- **Two coordinators can both act.** A relay can leave two sessions alive for a short time. The outward commands do not check the driver lease, so both could push or merge (0.4.0 relay review, round 1, finding 2).

## 2. Goals, non-goals, and anti-goals

Goals:

1. One task lands as one PR. Its branch starts at the fresh base and carries only that task's change.
2. A PR holds at most 3 commits.
3. A task changes at most 100 lines. A bigger task needs a written reason and stays under a hard ceiling.
4. From 0.6.0, after one kickoff with the owner present, the run needs no one until it ends.
5. From 0.6.0, the automatic handoff works from the first step of `/monet:gang plan` to the close.
6. Every outward command refuses to act unless its caller holds the driver lease.
7. A repo with no CI gets a green signal from a real gate run on the PR head.

Non-goals:

- More than one open PR at a time. This can come later, if the PR queue becomes the bottleneck.
- Stacked PRs, Graphite, or a merge queue.
- A parent session that drives several runs.
- Changes to review, the mutation check, or the ledger.
- Gang adding CI workflows to a target repo.

Anti-goals, which are failures even when the code works:

- A PR with more than 3 commits, or a size above the budget with no written reason.
- A force-push, an amend, or a rebase of a published commit.
- A question to the owner after kickoff that a standing order or a default can answer.
- A green check that did not come from a gate run on that exact PR head.
- Two sessions that push, open, or merge for the same run.

## 3. Facts from 0.4.0

Paths are under `skills/gang/` unless they start with another top folder.

| Area | Fact |
|---|---|
| Size | `limits.task_lines` defaults to 1200 and `limits.split_lines` to 1600 (`cli/internal/plan/plan.go:148-149`). |
| Size | Lint rejects a `size.lines` above `task_lines` (`cli/internal/lint/lint.go:364-370`). No code acts on `split_lines`. |
| Size | `reference/planner.md:84` hardcodes 1200 lines. No code measures the real diff of a task. |
| Merge | `gang merge` writes a merge commit with two parents, the trunk and the reviewed tip, titled `merge <id>: <title>` (`cli/internal/merge/merge.go:130`). |
| Merge | The implementer's own commit carries the `Task: <id>` line (`reference/roles.md:70`). The merge commit does not. |
| Done | `gang done` reads `Task:` lines in commits reachable from the local integration branch (`cli/cmd/gang/cmd_done.go:24`). |
| Push | `gang push` pushes the integration tip to `origin`, fast-forward only, after a secret scan (`cli/internal/wave/wave.go:110-138`). |
| Wave PR | `gang wave open` runs `gh pr create --head <integration>` (`cli/internal/wave/open.go:136`). |
| Wave PR | `gang wave merge` runs `gh pr merge --merge --match-head-commit` (`cli/internal/wave/merge.go:71`). |
| Waves | `gang schedule` cuts the tasks into equal-count buckets, by default one wave per 15 tasks (`cli/internal/schedule/schedule.go:309,325`). |
| Events | WAVE_READY k fires when no wave PR is open and every task with a wave of k or less is merged (`cli/internal/watch/watch.go:108-117`). |
| CI | With zero checks, `gh.Classify` returns neither green nor red (`cli/internal/gh/gh.go:88-93`). So no PR_GREEN fires, and `wave merge` refuses. |
| CI | The target repos of the last two runs have no `.github` folder on main. |
| Relay | The hook does nothing when `.claude/handoff-relay.json` is missing (`hooks/handoff-relay.js:83-85`). |
| Relay | Launch step 10 writes that file, and Launch step 6 first takes the driver lease (`reference/coordinator.md:36,40`). |
| Relay | `scripts/relay.mjs` always writes a `run --slug` kickoff (`scripts/relay.mjs:249`). `relay.md` assumes a held lease and a run. |
| Orders | `relay.mjs words` skips slash-command messages (`scripts/relay.mjs:158`). An order in the command line survives a relay only through `gang standing add`. |

Other constraints:

- Only the owner can start Docker Desktop or another local daemon, answer a permission prompt, change the permission mode, or do an interactive login.
- The auto-mode classifier can block a push, a merge, or `claude --bg` when it finds no explicit order from the owner.
- The owner's repos merge PRs by squash. For example, initor/monet #20 landed its 2 commits as one commit, 401ecb4.
- The owner's global PR rule: each PR gets a new branch cut from the fresh upstream main, with only that PR's commits. Decision 2 of the 0.4.0 spec made gang an exception. This design removes the exception.

## 4. Decisions of the owner

| # | Decision (2026-10-29) |
|---|---|
| O1 | Many small PRs. Each one has a few commits at most, about 3, and as few lines as possible. |
| O2 | 100 changed lines per PR. More only when truly necessary. |
| O3 | From 0.6.0, a run drives itself with no one in the loop, with the automatic handoff. |
| O4 | 0.5.0 also carries the lease checks. |
| O5 | From 0.5.x, with the cleanup grant at kickoff, gang removes the worktree of each shipped task at each checkpoint. It removes only a worktree whose task is on main and that has no uncommitted changes. |
| O6 | Claude squash-merges its own 0.5.0 PRs in monet, after CI is green and its own review is clean. |

## 5. Design

0.5.0 ships the PR queue only: each task becomes one small PR through `gang pr open`, `pr gate`, `pr fix`, and `pr merge`. The trunk keeps its 0.4.0 shape, and the wave commands stay. These wait for 0.5.x:

- One-parent trunk merges and `gang trunk rebuild`.
- `gang pr close` with a republish.
- `gang worktrees prune`.
- Plan version 2 with the 0.4.0 refusal.
- The removal of the wave commands.

Self-drive comes in 0.6.0: `gang plan init`, `--holder` on `plan commit`, the planning relay, the kickoff batch with self-approval, and no questions after kickoff. In 0.5.0 the 0.4.0 kickoff and approval question stay.

### 5.1 One task, one PR

The trunk (`repo.integration`) stays a local branch. Gang never pushes it.

From 0.5.x, `gang merge` writes one commit per task on the trunk, with one parent. Its tree is the merge result, as today. Its message is the task title, then the lines `Task: <id>` and `Reviewed: <sha>`. So the trunk is a straight line with one commit per task.

A PR queue publishes the trunk commits in trunk order. At most one gang PR is open at a time.

1. `gang pr open <id>` fetches the base and replays the task commit onto it.
2. The replay uses `merge-tree` with the commit's parent as the merge base, then `commit-tree`. It needs no checkout.
3. It runs the secret scan on the replayed patch.
4. It pushes the result to a new branch, `gang/<slug>/<id>`. It never moves an existing remote branch to another SHA.
5. It opens the PR with the task title and a body from the plan and the ledger.
6. When the PR is green (5.4), `gang pr merge <id>` merges it.

The merge is a squash merge with `--match-head-commit`. Its subject is the task title with the PR number. Its body holds the `Task: <id>` lines of every trunk commit in the PR. Gang never deletes a branch. With the repo setting "Automatically delete head branches" on, GitHub deletes it. Otherwise the close lists the branches for the owner.

The task title becomes the PR title and the squash subject. So lint checks that each title is a Conventional Commits subject of at most 72 characters.

The trunk is the private staging ref of the run. Only the gang CLI moves it, always with a compare-and-swap. Each move keeps the old tip under `refs/gang/<slug>/trunk-backup/<n>`.

From 0.5.x, `gang trunk rebuild` runs after each merged PR. It resets the trunk to the new `origin/<base>`. Then it replays the unshipped trunk commits onto it, in order. It leaves out each commit whose change already shipped, such as a fix that went out in a PR. So the trunk is always the base plus the commits still to ship.

From 0.5.x, a replay can still conflict, when the base changed outside the run or when a fix overlaps a later task. Then the queue stops, and a sync task resolves the conflict on the trunk. Agents still never rebase: the rebuild is a CLI step, like `gang merge`.

A red PR gets a fix. The fix task merges into the trunk first, as its own commit with `Task: <fix id>` and `Fixes: <id>`. Then `gang pr fix <id> <fix id>` replays that commit onto the open PR branch as a new commit. This push is a fast-forward.

A PR takes at most 2 fix commits. In 0.5.0 a PR that is still red stops the queue. From 0.5.x, `gang pr close <id>` closes it, and gang publishes the task and its fixes as one commit in a new PR, at most twice.

After that, the queue stops, and the checkpoint decides on a repair task or a split. From 0.5.x, for a split, the rebuild drops the task commit and the commits of the tasks that depend on it. Those tasks run again after the split. A drop happens only at a checkpoint, so the next `gang gen` sees the new done set.

A task is done when a commit on `origin/<base>` holds its `Task: <id>` line. The run is done when every task is done and no gang PR is open. The engine still starts a task when its dependencies are on the trunk, so the engine does not wait for the PR queue. A task branch that started from an older trunk tip still merges, because `gang merge` uses `merge-tree` against the current trunk.

`wave` stays in the schema for the schedule estimate and for `gen --max-wave`. No command opens a PR per wave. `gang push`, `gang wave open`, `gang wave merge`, and WAVE_READY go away in 0.5.x. A queue run does not call them.

### 5.2 Size budget

- `limits.task_lines` defaults to 100. `limits.split_lines` becomes the hard ceiling, default 300.
- A task with `size.lines` above `task_lines` must carry `size.reason`, which says why it cannot be split. Lint fails without it, and lint always fails above `split_lines`.
- `gang size <id> [--sha <sha>]` prints the real size of a task: the added plus deleted lines of the commit that `gang merge` would write. The PR replays that commit, so the PR shows the same size.
- The count includes tests. It leaves out binary files and the globs in `limits.size_exclude`. That default lists lock files: `go.sum`, `package-lock.json`, `pnpm-lock.yaml`, and `*.lock`.
- `gang merge` refuses a task whose real size breaks the rule, with exit 2 and the reason `size`. The engine then marks the task BLOCKED, and the checkpoint splits it.
- The implement brief states the budget. An implementer whose work cannot fit returns BLOCKED with a split proposal.
- `reference/planner.md` reads the limit from the plan, not from a hardcoded number.
- A plan may lower `task_lines` and `split_lines`. Lint refuses a plan that raises them above the defaults.
- `plan --adopt` stays. An adopted task above `task_lines` is split into parts, and each part records `source: <id>`. Lint rule 8 then compares the source graph with the parts grouped by `source`.

The budget holds for the whole PR: its diff against the base, fixes included. `gang pr fix` measures the PR after the fix. A fix that takes the PR over `task_lines` needs a written reason, which the coordinator records as a ruling. A PR over `split_lines` is closed, and the checkpoint splits its task.

### 5.3 Self-drive

This section is 0.6.0 behavior, except the worktree cleanup.

The kickoff of `/monet:gang plan <spec-dir>` asks once, while the owner is present. It uses one batch of questions, at most two AskUserQuestion calls.

1. The run settings that `planner.md` asks for today: the commands, the locks, and the trunk name.
2. The tracker page, a private Notion link or none.
3. The self-drive grant: approve a clean plan, open and merge each green PR, and relay at the threshold.
4. The cleanup grant (from 0.5.x): remove the worktree of each task that has shipped.
5. The permission setup: the allow rules for the gang binary, `claude --bg`, `claude agents`, and the Notion tools.
6. A check list: Docker running when the gate needs it, and the Mac on AC power with the lid open.

A clean plan means a clean lint and a plan review with no blocker. Each grant goes into `run/standing.md` with `gang standing add`, so that a relay carries it. Kickoff gets each permission settled while the owner is there. Where a harmless call exists, it makes that call once. For `claude --bg`, it asks the owner to add the allow rule.

With the grant, the planner approves its own plan and logs the approval in `status.md` and on the tracker. Then it launches the pilot and the loop with no question. Without the grant, the 0.4.0 approval question stays.

After kickoff, gang asks nothing that a standing order or a default can answer. Some blocks need the owner, such as an expired login or a stopped Docker. Gang writes each one to the tracker and goes on with the work that does not need it. At each heartbeat, the coordinator checks the power source and the lid, and it writes a warning to the tracker when the Mac can sleep.

Worktrees pile up: an earlier run left 116 of them for 17 tasks, and preflight fails at 400 (`limits.max_worktrees`). From 0.5.x, with the cleanup grant, `gang worktrees prune` removes the worktree of each task whose `Task:` line is on `origin/<base>`. It skips a worktree with uncommitted changes. The coordinator runs it at each checkpoint. Without the grant, the coordinator stops for the owner before the run reaches the cap.

### 5.4 Green signal

The plan gets `repo.checks`, with the value `github` or `gate`. Kickoff sets `gate` when the base has no `.github/workflows` folder, and `github` otherwise.

- With `gate`, gang runs `commands.gate` at the PR head SHA through `gang gate run`, in a temporary worktree.
- It then posts a commit status named `gang/gate` on that SHA, success or failure, with `gh api`.
- `gh.Classify` then sees one check, so PR_GREEN and PR_RED work as before.
- With `github`, zero checks after `limits.checks_grace_min` (default 10) count as red, with the reason `no checks`.

### 5.5 Relay from the first step

This section is 0.6.0 behavior.

- A new command, `gang plan init --holder <session>`, makes the run folder, takes the driver lease, and writes the relay config. Kickoff runs it before the first drafter starts.
- `reference/relay.md` gains a planning branch. `relay.mjs prompt --phase plan` writes a kickoff that resumes the planner, not a run.
- `reference/planner.md` gains "Resume planning in a new session". The new session reads the handoff and the drafts on disk, then goes on from the recorded step.
- A handoff in planning waits for running drafter and reviewer subagents to return, because their output lands in files.

### 5.6 Lease checks

- Every outward command takes `--holder <session>`: `gang pr open`, `pr fix`, `pr merge`, and the `gang/gate` status post. `pr close` takes it from 0.5.x, and `plan commit` from 0.6.0.
- Each one renews the driver lease for that holder first, with `lock.Renew`. It exits 4 when another session holds the lease. The renew runs under the lease mutex.
- `reference/coordinator.md` and `reference/relay.md` pass `--holder <session>` on every such call.

### 5.7 Compatibility

0.5.0 keeps plan version 1 and the trunk shape of 0.4.0. A run that 0.4.0 started can switch to the PR queue, because the queue reads its two-parent gang merges as landings. The steps are in `reference/coordinator.md`, under "Switch a 0.4.0 run to the queue". The plan version becomes 2 in a later release, when gang rebuilds the trunk on the base after each merged PR. From that release on, gang refuses a run folder from 0.4.0 and tells the user to finish that run with 0.4.0 first.

## 6. Risks

| Risk | Mitigation |
|---|---|
| One open PR at a time makes the queue slow when CI is slow. | The engine keeps working on the trunk meanwhile. Parallel PRs are a later step. |
| A fix commit on the trunk does not replay cleanly onto the open PR. | The queue stops, a sync task rebuilds the PR, and the checkpoint decides. |
| From 0.6.0, the classifier in a relayed session does not honor a grant that the relay carried. | The pilot forces one relay. A block goes to the tracker. |
| Tasks of 100 lines multiply the agent count and the token cost. | The owner accepted this cost for small PRs. Review stays per task. |
| The base moves outside the run. | The replay conflict path in 5.1. |
| Small tasks multiply the worktrees, about 7 per task in an earlier run. | From 0.5.x, `gang worktrees prune` at each checkpoint, under the cleanup grant. |

## 7. Left to the builder

- Command names and flags beyond those named here.
- The exact PR body. It stays short, in the shape that `monet:ship` uses.
- How the engine reports a size refusal, and how watch names the new events.
- The test design. Each rule gets a case that fails without it.
- The listing rule of `gang worktrees`. Today it checks ancestry of the trunk (`cli/cmd/gang/cmd_worktrees.go:48`), which the one-parent trunk commits of 0.5.x break.
- The split of the 0.5.0 work into PRs of at most 100 lines where possible. This document is one PR above 100 lines, because a design split across PRs is hard to review.

## 8. Calls made for the owner

These are Claude's calls, not the owner's. The owner checks them in review.

1. One task per PR, not several small tasks per PR.
2. Tests count toward the 100 lines. Lock files and binary files do not.
3. A hard ceiling of 300 lines, even with a reason. A plan may lower the limits but never raise them.
4. One open PR at a time.
5. Squash merges, because the owner's repos use them, with the `Task:` lines in the squash body.
6. A `gang/gate` commit status for a repo with no CI.
7. The trunk stays local, and only the CLI moves it. From 0.5.x it is rebuilt on the base after each merged PR. PR branches are `gang/<slug>/<id>`.
8. From 0.6.0, the kickoff asks everything in one batch, and the planner approves a clean plan on the owner's grant.
9. A red PR gets 2 fix commits, then the checkpoint. From 0.5.x, 2 republishes as one commit come between.
10. `plan --adopt` splits a big source task into parts that keep the source id.
