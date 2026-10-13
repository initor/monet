# Planner

Read this file for `plan <spec-dir>` and for `plan --adopt <file>`. Both modes end in one approval from the user and one `<gang> plan commit`. `<skill>`, `<repo>`, and `<gang>` are as in `SKILL.md`.

## Build first

`<slug>` is the `--slug` value. By default, it is the base name of `<repo>` in lowercase. Each run of characters outside `[a-z0-9]` becomes `-`, and a `-` at either end is dropped, so `.config` becomes `config`. A slug must match `^[a-z0-9][a-z0-9-]*$`. Stop and tell the user when a `--slug` value does not match. `<dir>` is `<repo>/.planning/gang/<slug>`, as an absolute path.

Make the default slug with the first command. Run the build before any other step, so that `<gang>` exists for `lint`, `schedule`, and `render`:

```bash
basename <repo> | tr 'A-Z' 'a-z' | sed -E 's/[^a-z0-9]+/-/g; s/^-//; s/-$//'
go -C <skill>/cli build -ldflags "-X main.skillDir=<skill>" -o <dir>/run/bin/gang ./cmd/gang
```

## Rules for every planning agent

- Use the Agent tool. Take the model and the effort from the `plan_draft` or `plan_review` entry in `<skill>/roles.json`.
- Send independent agents in one message, 10 at most at a time.
- Give each agent a small input: file paths and line ranges, never a whole plan pasted into the prompt.
- Tell each agent to return only counts and ids. Its real output goes to a file.
- Paste these lines into each prompt: "Spawn no subagents. Use no browser. Refer to the user as the user, or with they or them."

## Set up

1. If `<dir>/plan/plan.json` exists, ask the user whether to replace it or to pass another `--slug`.
2. Make the folders `<dir>/plan/drafts/` and `<dir>/run/planning/`.
3. Run `git -C <repo> check-ignore -q .planning/gang/`. If it exits 0, skip steps 4 and 5.
4. Append `/.planning/gang/` to the local exclude file, which leaves the repo unchanged. `git -C <repo> rev-parse --path-format=absolute --git-path info/exclude` prints its path.
5. If the user wants the rule tracked, add it to `.gitignore` instead, and ship that edit through `/monet:ship`.

## Write the skeleton

Write `<dir>/run/planning/base.json`. It holds every run-wide field of `plan.json`, and `"tasks": []`. Fill each field from the repo, not from memory.

| Field | Value |
|---|---|
| `version`, `slug` | `1` and `<slug>` |
| `repo.remote` | `git -C <repo> remote get-url origin`, written as `github.com/<owner>/<name>`. For a local origin, its path |
| `repo.base` | The default branch, from `gh repo view --json defaultBranchRef` or `git -C <repo> symbolic-ref refs/remotes/origin/HEAD` |
| `repo.integration` | `gang/<slug>`. In adopt mode, the existing integration branch |
| `repo.gh_account` | `gh api user --jq .login`. Leave it empty for a local origin with no GitHub |
| `repo.wave_title` | A PR title template with `{k}` and `{slug}`, such as `chore(gang): land wave {k} of {slug}` |
| `commands.gate` | The repo's own check command, from its `Makefile`, `package.json`, or CI config |
| `commands.health` | The gate commands plus the slow suites, such as end-to-end tests |
| `commands.wave_gate` | The smoke test that the wave PR's CI also runs |
| `env` | Variables that every gate needs, such as a pinned `KUBECONFIG` |
| `locks` | Shared resources that one gate at a time can use, such as a GPU or a cluster |
| `limits`, `models` | `{}`, so that the defaults apply |
| `forbidden` | `"git stash"`, `"git rebase"`, `"force-push"`, `"push to main"`, and the spec's own rules |
| `done` | `"every task is reachable from origin/main and no wave PR is open"` |
| `requirements` | `[]` until the inventory fills it |

Match `repo.wave_title` to the repo's own PR title rules, such as a Conventional Commits check. Lint fails when the title for wave 99 is over 72 characters.

Ask the user once with AskUserQuestion to confirm `commands`, `locks`, `repo.integration`, and `repo.wave_title`. These fields decide what green means and what each wave PR is called.

## From specs

### 1. Inventory the requirements

1. Split the spec folder into areas. An area is one spec file, or one folder of files, with about 100 requirements at most.
2. Send one inventory agent per area, with the `plan_draft` role.
3. Each agent writes `<dir>/run/planning/req-<area>.json`: an array of `{"id", "source"}`, with `source` as `file:line`.
4. Tell each agent to keep the spec's own requirement ids. Where the spec has none, it uses `<AREA>-<n>`.
5. Merge the files and make the skeleton with the commands below. Never copy requirements by hand.

```bash
jq -s 'add | unique_by(.id)' <dir>/run/planning/req-*.json > <dir>/run/planning/requirements.json
jq --slurpfile r <dir>/run/planning/requirements.json '.requirements = $r[0]' <dir>/run/planning/base.json > <dir>/plan/skeleton.json
```

### 2. Draft by area

Send one drafter per area, with the `plan_draft` role. Each prompt holds:

- The area's spec files and `<dir>/run/planning/req-<area>.json`.
- The task id prefix of the area, in upper-case letters, such as `API`.
- The task schema: the "Task fields" table in `<skill>/reference/schema.md`.
- The output: a JSON object `{"tasks": [...]}` in `<dir>/plan/drafts/<area>.json`.

Tell each drafter these rules:

- One task is one commit of at most 1200 changed lines.
- `goal` is one sentence that a stranger can run.
- Each `acceptance` line is one check that passes or fails.
- `verify.tests_first` names tests that fail before the change and pass after it.
- Each requirement of the area is covered by a task.
- A need from another area goes in `interfaces.consumes`, never in `depends_on`.

### 3. Assemble and lint

```bash
<gang> plan assemble --skeleton <dir>/plan/skeleton.json --drafts <dir>/plan/drafts
<gang> lint
```

- When lint exits 0, go to "4. Review".
- When lint exits 1, give each problem line to the fixer of each area whose task id it names.
- Send one fixer per area, with the `plan_draft` role. A fixer gets only its draft file and its lines. Tell it the lines are pre-verified, and to make small, targeted edits.
- Assemble and lint again. After 3 rounds with problems left, show the user the remaining lines and stop.

### 4. Review

Run `<gang> render`. Then send all reviewers and the critic in one message, with the `plan_review` role.

- **One reviewer per area.** It reads the draft and the area's specs. It writes findings to `<dir>/run/planning/review-<area>.md`, each with a severity and a task id. It checks requirement coverage, concrete goals, checkable acceptance, honest sizes, a `scope.write` that covers each file, and tests that fail first.
- **One global critic.** It reads `<dir>/plan/plan.md` and the output of `<gang> schedule`. It writes `<dir>/run/planning/critic.md`. It checks cross-area `depends_on` edges from `interfaces`, `merge_after` where the trunk must stay green, peers, locks, `risk`, and the critical chain.

### 5. Fix

- Send one fixer per area that has findings, with the `plan_draft` role. It gets only its draft and the findings for its task ids.
- The critic's cross-area findings go to the fixer of the area that must change.
- Then repeat "3. Assemble and lint". If a reviewer found a blocker, run "4. Review" and "5. Fix" one more time.

### 6. Schedule

```bash
<gang> schedule --write
<gang> render
```

`schedule --write` sets the `wave` of each task. Keep its critical chain and range lines for the approval.

### 7. Approve

1. Show the user the task count, the wave count, the critical chain, the range, and the path of `<dir>/plan/plan.md`. Never paste the whole plan.
2. Also show `repo.remote`, `repo.gh_account`, `repo.integration`, and `repo.base` from `plan.json`. This approval is the only gate before the pushes and the PRs.
3. Ask with AskUserQuestion: "Approve the plan for <slug>? Gang then pushes <integration> to <remote> as <gh_account> and opens wave PRs into <base> without asking." The options are "Approve and start the pilot", "Approve only", and "Change it".
4. For "Change it", ask what to change if the user gave no words. Send a fixer with those words, then go back to "3. Assemble and lint".
5. For an approval, run `<gang> plan commit --message "plan: <slug>"`. It prints the new SHA of `refs/gang/<slug>/plan`.
6. If it exits 3, the ref moved. Run it one more time.
7. For "Approve and start the pilot", read `<skill>/reference/coordinator.md` and launch.
8. For "Approve only", tell the user to run `/monet:gang run` in a session started at `<repo>`.

## Adopt

`plan --adopt <file>` transcribes a plan that already exists. `<file>` is that plan, in Markdown or as a JSON task graph. `<source>` is the `--adopt-source` file, else `<file>` when it is JSON with a `tasks` array. Each element of that array has an `id`, deps under `depends_on`, `deps`, or `dependsOn`, and merge-after under `merge_after` or `mergeAfter`. If there is no `<source>`, ask the user for one, because lint cross-checks the counts and the edges against it.

The transcribers read the Markdown plan. That is `<file>`, or the path in the top-level `source` field of `<source>`, relative to `<repo>`. Where an element has `start` and `end` lines, pass them on, so that each transcriber reads only its tasks.

1. Do "Set up" and "Write the skeleton". Set `repo.integration` to the existing integration branch.
2. Take `commands`, `locks`, and `forbidden` from the global section of the source plan.
3. Count the source tasks: `jq '.tasks | length' <source>`.
4. Transcribe every task, the merged tasks too. `gang done` reads merged tasks from git, so the run skips them.
5. Cut the task list into slices of 5 tasks. Send one transcriber per slice, with the `plan_draft` role.
6. Give each transcriber its task ids, the line range of each task when known, and the "Task fields" table in `<skill>/reference/schema.md`.
7. Each transcriber writes `{"tasks": [...]}` to `<dir>/plan/drafts/<first id>.json`.
8. It also writes `<dir>/run/planning/req-<first id>.json`: each requirement id of its tasks, with the plan line as `source`.
9. Merge the requirement files and make the skeleton with the two `jq` commands in "1. Inventory the requirements".
10. Run `<gang> plan assemble` as in "3. Assemble and lint". Then run `<gang> lint --adopt-source <source>`.
11. Fix lint problems with one fixer per slice. Rule 8 must pass: the task count and the edge set match `<source>`.
12. Keep the source waves. Run `<gang> schedule` without `--write`, then `<gang> render`.
13. Go to "7. Approve".

Tell each transcriber these rules:

- Copy the plan. Do not redesign it.
- Keep each `id`, `branch`, `wave`, size, requirement id, and lock as the source has it.
- Write the source's deps as `depends_on`, and its merge-after list as `merge_after`.
- Where the source lacks a field, use its closest text, and name the gap in `notes`.
