# Spec: `/monet:gang`

Date: 2026-10-11. Status: approved on 2026-10-11. The skill ships in `skills/gang/`, and every skill and tool path below is a repo path in the plugin layout.

> **Revised during planning (2026-10-11).** Cross-checking the implementation plan changed six details. The text below already reflects them.
> 1. A whole `Task: <id>` line in the commit message marks a task. A git trailer is not required, because adopted commits may hold that line outside a trailer.
> 2. The plan is committed to the private ref `refs/gang/<slug>/plan`, not to the integration branch. The target repo ignores `.planning/gang/`.
> 3. The planner uses Agent subagents, not a Workflow.
> 4. `preflight` takes `--holder`, and `release` takes no id.
> 5. Gate slots are per run, not machine-wide.
> 6. Wave PR titles come from `repo.wave_title`, and lint caps them at 72 characters.

## 1. Motivation

The user builds a new orchestrator by hand for each large task. Each one relearns the same harness lessons.

Two working systems exist today:

- **[pstack](https://github.com/cursor/plugins/tree/main/pstack)**, in Cursor's plugins repo, is the method gang is built on, and it's proven: briefs as the product, a ledger keyed by SHA, fresh agents, and independent verification.
- **The earlier engine**, hand-built on Claude Code for an earlier project, is proven: a compiled Workflow, resume from git, compare-and-swap merges, and a simulator.

`/monet:gang` makes the two into one reusable skill. It turns specs into a task graph, then runs the graph with a fleet of agents until every task reaches `main` through wave PRs.

## 2. Decisions

| # | Decision |
|---|---|
| 1 | The acceptance test for version 1 is the toy spec run (section 11.4). Adopting the rest of the earlier engine's plan is an optional later run (section 11.5). |
| 2 | Units merge into an integration branch. Waves reach `main` through wave PRs. A PR from the integration branch is a standing exception to the global PR rule for gang runs. |
| 3 | Scope is coordinator plus planner. |
| 4 | Correctness wins over human touches, wall clock, and token cost. |
| 5 | Agents may open PRs without approval. The user will revisit the global "nothing remote" rule. |
| 6 | Architecture C: a Workflow engine runs tasks to a checkpoint, and the main session coordinates. |
| 7 | Name `/monet:gang`. The CLI is Go, built at preflight. The engine and the simulator are JavaScript. |
| 8 | One roles table controls models. Every role defaults to `opus`. |
| 9 | No Slack in version 1. |

## 3. Scope

**In scope:** the planner (from specs, or adopting a plan), the engine, the coordinator, the `gang` CLI, the simulator, the tests, and the monet wiring. Wave PR merges happen only when the run starts with `--merge-on-green`.

**Not in scope for version 1:**

- A router with playbooks, or a principle index.
- Babysitting PRs that gang did not open. `correct` and `reflect`.
- Slack. Models from other vendors. Cloud or remote workers. Graphite.

## 4. Architecture

### 4.1 Components

| Component | Path (under `skills/gang/`) | Job |
|---|---|---|
| Skill | `SKILL.md` | Routes `plan`, `run`, and `status`. Holds the core rules and the launch checklist. About 120 lines. |
| References | `reference/planner.md`, `reference/coordinator.md`, `reference/roles.md`, `reference/schema.md` | Read only when that phase runs. `roles.md` holds the agent prompts. `schema.md` holds the plan schema of section 5. |
| Roles | `roles.json` | Default `{model, effort}` per role |
| Engine | `engine/engine.template.js` | One generic Workflow script. `gang gen` injects the plan data. Target: 800 lines or fewer. |
| CLI | `cli/` (Go module, `cmd/gang`) | Plan, ledger, merge, gate, lock, watch, and status commands |
| Simulator | `sim/sim.mjs` | Runs the generated engine against mock agents on a virtual clock |
| Command | `SKILL.md` | The monet wrapper. The `/monet:gang` command folded into `skills/gang/SKILL.md`, so no command file ships. |
| Tests | `tools/test-gang.sh` (repo root) | Runs `go vet ./...`, `go test ./...`, and the simulator. `make test` calls it. |

### 4.2 Flow

1. **Plan.** Specs, then a draft graph, then lint, then a schedule. The user approves the plan, and `gang plan commit` commits it to the private ref `refs/gang/<slug>/plan`. This is the only approval in a run.
2. **Preflight.** `go build` writes the CLI into the run folder. Then `gang preflight` checks the repo, the driver lock, the tools, the account, the trunk, the plan, and the disk.
3. **Pilot.** One task runs the full lifecycle, launched while the user is present.
4. **Loop.** An engine segment runs to a checkpoint. The coordinator handles events during the segment and decides at the checkpoint, then relaunches.
5. **Close.** The coordinator proves the done predicate on the real repo, writes the final status, and archives the run.

### 4.3 Run state, in the target repo

```
.planning/gang/<slug>/
  plan/                  committed to refs/gang/<slug>/plan
    plan.json            the task graph and run config
    plan.md              rendered from plan.json, never hand-edited
  run/                   local only, must be git-ignored
    bin/gang             built at preflight
    engine.js            generated by gang gen
    standing.md          standing orders, read at every spawn
    gates.md             questions parked with a default
    ledger.tsv           one verdict per (task, SHA)
    status.md            rendered by gang status
    driver.lock          the driver lease
    tasks/<id>/report.md agent memory across rounds
    tasks/<id>/owner.md  rulings that win over the plan
    runs/<tag>/          archived Workflow results and rulings
```

git is the source of truth for "done". A task is done when a commit whose message holds the line `Task: <id>` is reachable from the integration branch.

## 5. Plan schema (`plan.json`)

A task in the plan is its brief. Each of pstack's 9 brief fields is a required field, so lint catches a missing field before any spawn.

### 5.1 Run-wide fields

```json
{
  "version": 1,
  "slug": "widget-phase2",
  "repo": { "remote": "github.com/example/widget", "base": "main", "integration": "phase-2", "gh_account": "example", "wave_title": "chore(gang): land wave {k} of {slug}" },
  "commands": { "gate": ["make check"], "health": ["make check", "make test-e2e"], "wave_gate": ["make smoke"] },
  "env": { "KUBECONFIG": "./.kube/config" },
  "locks": ["gpu", "kind"],
  "limits": { "task_lines": 1200, "split_lines": 1600, "fix_rounds": 3, "repair_tries": 2, "blocked_stop": 3, "null_stop": 6, "retries": 2 },
  "models": {},
  "forbidden": ["git stash", "git rebase", "force-push", "push to main"],
  "done": "every task is reachable from origin/main and no wave PR is open",
  "requirements": [{ "id": "R-25", "source": "specs/engineering.md:670" }],
  "tasks": []
}
```

### 5.2 Task fields

`skills/gang/reference/schema.md` is the copy of the task schema that gang reads at run time. That copy also has fields added after this spec, such as `branch`. When the two differ, the copy wins.

| Field | Type | Brief field | Rule |
|---|---|---|---|
| `id` | string, `^[A-Z]+[0-9]+$` | GOAL | unique |
| `title`, `goal` | string | GOAL | `goal` is one sentence a stranger can run |
| `scope.write`, `scope.shared` | globs | SCOPE | `shared` means append-only edits |
| `peers` | ids | SCOPE | tasks allowed to write the same files |
| `depends_on`, `merge_after` | ids | CONTEXT | must exist, no cycles |
| `requirements` | ids | CONTEXT | each id exists in the run-wide list |
| `interfaces.consumes`, `interfaces.produces` | refs (`py:`, `cli:`, `make:`, `file:`, `test:`) | CONTEXT | a consumed ref is produced by a dependency, or already exists |
| `acceptance` | strings, 1 or more | ACCEPTANCE | one checkable line each |
| `verify.tests_first` | test ids, 1 or more | VERIFY | these fail before the change and pass after |
| `verify.commands` | strings | VERIFY | extra commands beyond the run gate |
| `size.lines`, `timebox_min` | ints | TIMEBOX | `size.lines` is at most `limits.task_lines` |
| `forbidden` | strings | FORBIDDEN | added to the run-wide list |
| `locks` | names | (scheduling) | each name is in the run-wide `locks` |
| `risk` | `low` or `high` | (review) | `high` adds an adversarial reviewer |
| `wave` | int | (scheduling) | set by `gang schedule`, or kept from an adopted plan |
| `model` | role overrides | (models) | optional, rare |
| `notes` | string | (any) | optional. Gives the reason when a requirement is covered twice. |

REPORT is a fixed return schema (section 7.3). STANDING is `run/standing.md`, pasted into every brief.

### 5.3 Lint (`gang lint`)

Lint fails with exit 1 and names every problem. It checks:

1. Schema shape and types. Unknown fields fail.
2. Every required brief field is present and not empty.
3. ids are unique. Every reference resolves. The graph has no cycle across `depends_on` and `merge_after`.
4. Each requirement is covered by at least one task. A requirement covered by two tasks needs a reason in the `notes` of both.
5. Two tasks whose `scope.write` globs overlap must list each other in `peers`.
6. `size.lines` is at most `limits.task_lines`.
7. Model values are aliases from the allowed set (`opus`, `sonnet`, `haiku`, `fable`). A raw model id fails.
8. With `--adopt-source <file>`, the task count and the edge set match the source plan.

### 5.4 Schedule (`gang schedule`)

Schedule simulates the graph with the measured agent cap (CPUs minus 2, or `--cap`), the locks, and per-task duration estimates from `size.lines`. Waves are PR checkpoints, not barriers. A task starts as soon as its dependencies merge. The output is the wave of each task, the critical chain, and a wall-clock range. After a pilot, `--calibrate <run>` prints a new estimate from the pilot's real durations. It does not edit the plan.

### 5.5 Planner

- **From specs:** `/monet:gang plan <spec-dir>` runs Agent subagents: one drafter per spec area, one reviewer per area, one global critic, then fixers. Then it runs lint and schedule.
- **Adopt:** `/monet:gang plan --adopt <file>` has agents transcribe an existing plan into the schema. Lint cross-checks counts and edges with `--adopt-source`.
- The planner renders `plan.md`, shows the user the schedule and the critical chain, and stops for approval. After approval, `gang plan commit` commits `plan.json` and `plan.md` to `refs/gang/<slug>/plan` with `commit-tree` and a compare-and-swap `update-ref`. The integration branch and the main checkout never change.

## 6. Roles and models

`roles.json` holds the defaults. A role resolves in this order: task `model`, then plan `models`, then `roles.json`. `gang status` prints the resolved table before launch.

| Role | Model | Effort |
|---|---|---|
| `implement` | `opus` | high |
| `implement_hard` (`risk: high`) | `opus` | xhigh |
| `review` | `opus` | xhigh |
| `review_adversarial` (`risk: high`) | `opus` | xhigh |
| `fix` | `opus` | high |
| `sync` | `opus` | high |
| `merge` | `opus` | low |
| `health` | `opus` | medium |
| `plan_draft`, `plan_review` | `opus` | high |

Review independence comes from three things: a fresh agent, a different lens, and the mutation check. The mutation check does not depend on the model.

## 7. Engine

### 7.1 Generation

`gang gen` fills the template with the task table, its checksum, the done set (from git), the resolved roles, and the run config. The script throws before any spawn if the checksum or the task count does not match. The script has no file system, so all data is injected at generation time. Agents read files at spawn through `gang brief`.

Every `agent()` call sets `label`, `phase`, `schema`, `model`, `effort`, and `isolation: "worktree"`. Every prompt names the absolute path of `run/bin/gang`.

The coordinator launches `run/engine.js` with the Workflow tool and `scriptPath`. The skill's instructions call Workflow, which is the opt-in the tool requires.

### 7.2 One task's lifecycle

1. **Implement.** The agent runs `gang claim <id>` and `gang brief <id>`. It writes the `verify.tests_first` tests and confirms they fail. It writes the code, runs `gang gate run <id>`, and commits once with the line `Task: <id>` in the message. It writes `tasks/<id>/report.md`.
2. **Review.** A fresh agent maps requirements to tests and re-runs the gate. It applies one-line mutants to each changed part. A surviving mutant is a blocker. A `risk: high` task also gets an adversarial reviewer.
3. **Ledger.** As its last step, the reviewer records its verdict with `gang ledger record <id> <sha> --lens review|adversarial --blockers N --majors N --survivors N [--gate-ran] [--mutation-checked] --evidence <findings_path>`. Clean means 0 blockers, 0 majors, the gate ran, and the mutation check ran. The engine also checks the review JSON. A check that did not run forces another review. `gang merge` trusts only the ledger.
4. **Fix.** A new agent per round gets the review, the report, and the expected tip SHA. The re-review covers the fix diff only. After `limits.fix_rounds` rounds, the task is BLOCKED.
5. **Merge.** A low-effort agent runs `gang merge <id> --sha <reviewed>`. An in-script mutex serializes merges.
   - On a conflict, a sync agent merges the trunk into the task branch. It never rebases.
   - Then a merge-only review runs. This repeats at most 3 times.
6. **Health.** After merges (coalesced), a health agent runs `gang gate run --health` on the trunk tip. A red trunk stops new merges and starts a repair task that owns the trunk, at most `limits.repair_tries` times.

### 7.3 Return schemas

| Role | Returns |
|---|---|
| implement, fix | `status`: DONE, DONE_WITH_CONCERNS, BLOCKED, TRUNK_RED, NO_WORKTREE, STOPPED. Plus `tip_sha` (40 hex), `gate_passed`, `rulings`, `summary`. |
| review | `blockers`, `majors`, `minors`, `gate_ran`, `mutation_checked`, `survivors`, `reviewed_sha`, `findings_path` |
| sync | `status`, `tip_sha` |
| merge | `status`: MERGED, CONFLICT, REFUSED, ALREADY. Plus `trunk_sha`, `reason`. |
| health | `status`: GREEN or RED. Plus `trunk_sha`, `failing`. |
| engine result | `stop`: DONE, BLOCKED, TRUNK_RED, INTERRUPTED, STOPPED. Plus `merged`, `blocked`, `interrupted`, `trunk_sha`, `agents`, `nulls`. |

### 7.4 Scheduling

- A task is ready when every `depends_on` task is merged.
- Ready tasks start in order of the longest remaining chain.
- The agent cap is CPUs minus 2 (measured at preflight). Implementer slots are `max(2, min(8, cap x 2/3, RAM GB / 4))`, the earlier engine's formula. The rest of the cap is for review, fix, merge, and health agents.
- A `merge_after` task can start early but merges only after the named tasks. A task that can never start gets a terminal status, so no waiter hangs.
- Agents take lease locks with `gang lock` around the commands that need them.

### 7.5 Stop conditions

The engine stops new spawns, drains in-flight work, and returns when:

1. Every task in scope is merged (DONE).
2. A BLOCKED task is on the critical chain, or `limits.blocked_stop` tasks are BLOCKED (BLOCKED).
3. The trunk stays red after `limits.repair_tries` repairs (TRUNK_RED).
4. `limits.null_stop` agents in a row return null (INTERRUPTED). A null result is never a pass.
5. An agent returns STOPPED because `standing.md` starts with a stop line (STOPPED).

The engine never pushes, opens a PR, or reads CI.

## 8. Coordinator

The coordinator is the main session, started in the target repo. It never writes code. It reads `gang status`, event lines, and task summaries, never raw diffs or transcripts.

### 8.1 Events

The coordinator runs the Monitor tool on `gang watch`, which prints one event per line. It restarts the Monitor at the deadline and keeps a self-paced wakeup as a fallback heartbeat.

| Event | Action |
|---|---|
| `WAVE_READY k` | Run the wave gate on the trunk tip. Push the integration branch with `gang push`. Open wave PR k with `gang wave open k`. At most one wave PR is open. |
| `PR_GREEN k` | With `--merge-on-green`, merge if every task in wave k has a clean ledger row. Otherwise mark the PR ready in `status.md`. |
| `PR_RED k` | Add a fix task to the plan. Write a stop line. Relaunch with the fix task first after the engine stops. |
| `ENGINE_STOPPED <reason>` | Handle the checkpoint (8.3). |
| `STALL` | No trunk move and no file activity for 2 times the median task time. Probe read-only. Never resume an agent to check on it. |

`gang watch` ports pstack's `watch-pr` policy. Blocker order is conflicts, then review threads, then CI, then gates. An empty check list is never green. A query error retries with backoff, then reports.

### 8.2 Files as the channel to a running engine

Agents read these files at spawn through `gang brief`:

- `standing.md`: numbered standing orders. A first line `STOP: <reason>` halts new spawns.
- `tasks/<id>/owner.md`: rulings that win over the plan and the spec.
- `gates.md`: questions an agent parked, each with a default. The agent continues on the default. The coordinator can overrule with an `owner.md` ruling.

### 8.3 Checkpoints

| Stop reason | Action |
|---|---|
| BLOCKED | Read the report and the review. Split the task, write a ruling, or edit the plan. Lint, regenerate, relaunch. |
| TRUNK_RED | Send read-only investigators. Add a repair task with a ruling. Relaunch. |
| INTERRUPTED | Archive the result. Schedule a wakeup for the usage reset. Relaunch. Resume comes from git. |
| STOPPED | Act on the stop line's reason. Clear the line. Relaunch. |
| DONE | Prove the predicate (8.4). Close. |

A plan edit at a checkpoint (a split, a fix task, a repair task) needs no approval from the user. The coordinator lints it, commits it with `gang plan commit`, and logs it in `status.md`.

Before every relaunch, three things hold. `gang done` agrees with the ledger. Every agent from the last segment has a terminal row. The Workflow result is archived in `run/runs/<tag>/`. A late zombie result is reconciled against the trunk and the ledger, never merged blind.

### 8.4 Done

The run is done when `git log origin/main..<integration>` prints nothing and `gh pr list` shows no open wave PR. The coordinator writes the final `status.md` with the metrics in section 11.5.

## 9. CLI reference (`gang`)

The CLI source in `skills/gang/cli/cmd/gang/` is the current reference for commands and flags. When this table and the source differ, the source wins.

Exit codes: 0 success. 1 user or lint error. 2 not found or refused (including NOT-VERIFIED). 3 conflict. 4 lock held. 64 usage error.

| Command | Effect |
|---|---|
| `preflight --holder <session>` | Runs the checks in section 10.1 |
| `lint <plan.json> [--adopt-source <file>]` | Section 5.3 |
| `schedule <plan.json> [--cap N] [--calibrate <run>]` | Section 5.4. Writes `wave` into the plan before approval. With `--calibrate`, it prints only. |
| `plan commit [--message <text>]` | Commits `plan/` to `refs/gang/<slug>/plan` without a checkout. Used at approval and for every later plan edit. |
| `plan assemble [--skeleton <file>] [--drafts <dir>]` | Joins the skeleton plan with the planner's task drafts into `plan/plan.json`. Fails on a duplicate task id. |
| `render <plan.json>` | Writes `plan.md` |
| `gen --tag <tag>` | Writes `run/engine.js` |
| `brief <id>` | Prints the task slice, the standing orders, and any `owner.md` rulings |
| `report <id> [--heading <text>] [--name <file.md>]` | Writes `tasks/<id>/report.md` from stdin, or appends a section under the heading |
| `gates park <id> --question <q> --default <d>`, `gates list`, `gates answer <G-id> --answer <a>` | Parks a question in `gates.md`, lists the open ones, or records the answer |
| `standing show\|add <line>\|stop <reason>\|clear-stop` | Prints or edits `run/standing.md` |
| `roles` | Prints the resolved model and effort for each role |
| `claim <id>`, `release` | Moves a task branch between worktrees by SHA. No stash. |
| `gate run <id> [--health \| --wave]`, `gate wait <job>` | Background gate job with per-run slots, pinned env, and a time limit |
| `lock acquire\|renew\|release <name>` | Lease locks. A dead holder frees in 5 minutes. |
| `ledger record\|check\|summary` | One row per (task, SHA). A new SHA voids the old row for that task. |
| `merge <id> --sha <sha>` | Refuses unless the tip is the reviewed SHA and the ledger is clean for it. Then `merge-tree`, `commit-tree`, and a compare-and-swap `update-ref`. |
| `done` | Lists task ids from `Task: <id>` lines in commits reachable from the integration branch |
| `status` | Writes `status.md` and prints at most 5 lines |
| `watch` | The event stream in section 8.1 |
| `push [--dry-run]` | Fast-forward push of the integration branch only, after a secret scan of added lines |
| `wave open <k> [--dry-run]` | Opens wave PR k with a body generated from the plan and the ledger. `--dry-run` prints the PR instead. |
| `wave merge <k>` | Merges wave PR k with a merge commit. Used only with `--merge-on-green`. |
| `prompts` | Prints one rendered prompt per role |
| `worktrees` | Lists task worktrees that are merged and safe to remove |

Types: task state and verdicts are closed Go enums. Every file write is atomic (write a temp file, then rename). Every TSV cell is sanitized against tabs, newlines, and formula prefixes.

## 10. Safety

### 10.1 Preflight (fails closed)

1. The session cwd is the repo root, and its remote matches `repo.remote`.
2. The driver lock is free, or is held by this session.
3. `go`, `node`, `git`, and `gh` are on PATH. `gh api user --jq .login` equals `repo.gh_account`.
4. The main checkout is clean. The integration branch exists, or is created from `base`.
5. `.planning/gang/` is git-ignored (`git check-ignore`).
6. `plan.json` lints clean, and it is identical to `plan.json` at `refs/gang/<slug>/plan`.
7. Free disk and the worktree count are under limits.

### 10.2 Launch

The first launch of a run is a one-task pilot, started while the user is present. Auto mode can deny the first write-capable launch. If the session cannot push, the skill asks the user to switch to accept-edits mode before the first push.

### 10.3 Agent rules (in every brief)

- Confirm the cwd is under `.claude/worktrees/`. Otherwise return NO-WORKTREE.
- Never use `git stash`, rebase, force-push, or push. Never delete branches or worktrees.
- Run heavy commands only through `gang gate run`.
- Write run state only through `gang` (`report`, `gates park`, `ledger record`). Claude Code blocks Edit and Write into the main checkout from a worktree agent, and one writer per file keeps the state consistent.
- Decide open choices and record each one as `Ruling: <what> - <why> - <cost if wrong>`.
- Spawn no subagents.

### 10.4 Retry by failure mode

| Failure | Action |
|---|---|
| Real diff above `limits.split_lines`, or over `timebox_min` | Split the task at the next checkpoint |
| Tool error | Retry with a fresh agent |
| Unknown | Retry once |
| `limits.retries` failed retries | BLOCKED |

## 11. Testing and acceptance

1. **CLI (`go test`).** Table tests pin each value against the thing it must agree with.
   - Lint: one fixture per failure class.
   - Merge, on temp repos: a moved tip, a missing verdict, a conflict, and a lost compare-and-swap.
   - Watch: a fake `gh` per blocker class, and an empty check list.
   - Schedule: a fixed graph gives fixed waves.
   - Ledger, lock, and done: one test per rule in section 9.
2. **Simulator (`node sim/sim.mjs`).** Runs the generated engine against mock agents.
   - Scenarios: the earlier engine's 27, plus 4 new ones.
     - A stop line.
     - PR_RED with a relaunch.
     - A resume after INTERRUPTED.
     - A BLOCKED task on the critical chain.
   - Each scenario runs twice with the same seed.
   - Invariants: no hang, and no merge before its dependencies. No merge without a clean ledger row for that SHA. The implementer cap holds. Every task ends terminal.
   - It exits 1 on any failure.
3. **Prompt review.** `gang prompts` prints one prompt per role.
4. **Planner test.** A toy spec of about 8 tasks, in a scratch repo with a local bare `origin`. The test runs plan, lint, schedule, then a full run up to a dry-run wave PR.
5. **Adopt acceptance (optional, later).**
   1. Stop the earlier engine and archive its state.
   2. Run `plan --adopt` on its remaining tasks. Lint cross-checks against its task graph.
   3. Run a one-task pilot, then the full run.
   4. Record the user's touches, wall clock, tokens, blockers found in review, and mutation survivors, beside its earlier runs.

Version 1 is done when tests 1 to 4 pass. The adopt run in test 5 is optional. It can come later, and it does not gate version 1.

## 12. Files

**Create:**

- `skills/gang/SKILL.md`, `roles.json`, `reference/planner.md`, `reference/coordinator.md`, `reference/roles.md`, `reference/schema.md`
- `skills/gang/engine/engine.template.js`
- `skills/gang/cli/` (Go module: `cmd/gang`, `internal/...`, tests, fixtures)
- `skills/gang/sim/sim.mjs` and its scenarios
- `tools/test-gang.sh`
- `docs/design/gang.md` (this file)

The `/monet:gang` command needs no file of its own. It folded into `skills/gang/SKILL.md`.

**Modify:**

- `agents/monet.md`: a routing row and a dispatch step
- `README.md`: the generated catalog row for the skill
- The `pr` and `review-local` skills: add `Agent` to `allowed-tools` (existing drift)
- Sibling descriptions that need a mirror line: `review-local`, `pr`, `roadmap`, `handoff`, `ship`

## 13. Build order

Each step ends in a check that can pass or fail.

1. CLI: plan schema, lint, render. Test: lint fixtures.
2. CLI: ledger, merge, done, claim, release. Test: temp-repo merge tests.
3. CLI: gate runner and locks. Test: slots, lease expiry.
4. CLI: schedule. Test: fixed graph.
5. Engine template and `gang gen`. Test: guards throw on a bad checksum.
6. Simulator with all scenarios. Test: green twice per seed.
7. CLI: watch, push, wave open, status. Test: fake `gh`.
8. Skill, references, roles, and the monet wiring. Test: `tools/check_skills.py gang` and the portability scan.
9. Planner workflow and the toy spec. Test: section 11.4.
10. Optional: adopt the earlier engine's plan, then a pilot and a full run. Test: section 11.5.

## 14. Risks

| Risk | Mitigation |
|---|---|
| Same-model reviewers share blind spots | The mutation check, and an adversarial lens for `risk: high` |
| The auto-mode classifier blocks a launch or a push | Pilot while the user is present. Accept-edits mode before the first push. |
| The agent cap differs per machine | Measure at preflight. Schedule with the measured cap. |
| Usage limits set the wall clock | Resume from git. Archive every segment. Schedule a wakeup for the reset. |
| The global PR and "nothing remote" rules conflict with decisions 2 and 5 | The user updates the global rules. The skill states its exception in its Constraints section. |
| "Propose a plan first" conflicts with "never block" | The plan approval is the one gate. The run never blocks after it. |
| The engine template grows past 800 lines | Move logic into the CLI. The simulator guards each move. |
| An earlier orchestrator drives the same repo at the same time | The driver lock, and stopping that orchestrator before adopt |
