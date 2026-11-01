# Changelog

This file lists the notable changes to monet in each release. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow [Semantic Versioning](https://semver.org/). The version lives in `.claude-plugin/plugin.json` only.

## [Unreleased]

## [0.5.1] - 2026-11-01

### Fixed

- The PR queue no longer skips trunk landings after `gang pr fix` puts a fix task from the trunk tip on an earlier task's PR. The queue had counted every landing up to that fix task as shipped, so a later PR could fail on code from a skipped landing (#77).
- A fix task that ships on its own PR counts again as the last shipped landing, so an earlier fix that a hand squash left without its `Task:` line does not come back (#78).
- A run picks up both fixes after it builds `<gang>` again.

## [0.5.0] - 2026-10-31

### Added

- `/monet:gang run` ships each trunk task as its own small PR, one at a time, in trunk order. A PR holds at most 100 changed lines by default. A plan may lower that limit with `limits.task_lines`. A task that cannot be split may go above it, up to `limits.split_lines` (300 by default), when its `size.reason` says why.
- `gang pr open <id>` pushes the branch `gang/<slug>/<id>` and opens the PR of that task. `gang pr gate <id> <job>` posts the result of a gate job as the commit status `gang/gate`, for a repo with no CI (`repo.checks: gate`). `gang pr fix <id> <fix id>` pushes a CI fix onto the PR as a new commit, and a PR takes at most 2 fix commits. `gang pr merge <id>` squash-merges the PR and keeps each `Task:` line of its commits. Each of these commands takes the driver lease with `--holder <session>`.
- `gang watch` prints `PR_NEXT <id>` for the next task to open, and `PR_GREEN` and `PR_RED` for the open PR. `PR_RED` names `no-checks` when the CI of the repo gives no check within `limits.checks_grace_min` minutes (10 by default). In gate mode, a PR is green only when `gang/gate` has passed.
- `gang size <id>` prints how many lines a task would change on the base. Each implementer runs it on its commit, and reports a split when the commit is over the budget.

### Changed

- `plan/plan.json` gains `repo.checks`, which is `github` (the default) or `gate`, and the size limits `limits.task_lines`, `limits.split_lines` and `limits.checks_grace_min`.
- Wave PRs are no longer how a run ships. The wave commands stay for a run that still has an open wave PR.
- A run that 0.4.0 started switches to the queue with the steps under "Switch a 0.4.0 run to the queue" in `skills/gang/reference/coordinator.md`. Build `<gang>` again first, because a 0.4.0 binary refuses the new `checks` field.

### Known issues

- A branch that `gang pr open` pushed before it failed stays on origin. Once the base moves, it blocks a retry with `exists at ... with another change or base`. Delete that branch on GitHub, then run the command again.
- A task id must not repeat a `Task:` id that is already on the base from an earlier run. The queue counts such a task as shipped.
- A failed push is always exit 2 with `push rejected`, even when the cause is the network. Retry the command.
- The wave commands still count any green check as green in gate mode. Use them only to finish a wave PR that is already open.

## [0.4.0] - 2026-10-29

### Added

- `/monet:gang run` keeps a run going across sessions. When the context of the coordinator reaches 80% of its window, it writes a handoff and starts a new background session with `claude --bg`. The new session takes the driver lease and resumes from git and the run files, with no prompt from the user. Each kickoff prompt carries the standing orders and the words of the user, copied by `skills/gang/scripts/relay.mjs`.
- A plugin hook, `hooks/handoff-relay.js`, gives the relay notice. It acts only in a project that has `.claude/handoff-relay.json`, and only on tool calls of the session itself, not of its subagents.
- `/monet:gang run` can keep a private Notion page as the tracker of the run, with an executive summary on top and a dated log below it. The coordinator reads the comments of the user on that page and acts on them.

## [0.3.0] - 2026-10-16

### Added

- `/monet:think [--higher | --deeper | --wider] <task>` rewrites a task into a more ambitious prompt, after Felix Rieseberg's [How to be more ambitious](https://felixrieseberg.com/how-to-be-more-ambitious-a-tutorial-for-humans-with-models/). It checks which raw data, tools, and judges the session can reach, and the prompt it writes carries its own check and guardrails. With `--wider`, it can run a small crew and a red team as subagents. It runs only when typed.

### Changed

- The README intro names the work monet takes over and the first command to type, `/monet:review-local`. It no longer promises prose that reads like you, since that needs a voice persona.

## [0.2.0] - 2026-10-14

This release breaks one thing: the interview-prep commands, such as `/monet:prep`, are gone.

### Changed

- The README's How it works diagram is now an animated isometric figure, in a light and a dark version. `tools/how-it-works.mjs` builds it from `docs/assets/how-it-works.json`. It goes still for readers who turn on reduced motion.

### Removed

- The interview-prep skills: `prep`, `sd-mock`, `bq-mock`, `code-mock`, `sd-explainer`, `sd-playground`, `bq-playground`, `code-playground`, `interview-tracker`, and `interview-progress`. They were personal tools, not engineering workflow. The monet agent no longer routes interview requests. Session records that they wrote stay in `<data dir>/interview-prep/` until you remove them.

### Fixed

- `review-local` lists the Skill tool in `allowed-tools`, so the call that draws its report through `paint` no longer depends on the session's permission rules.
- `visual-design` loads `voice` in plain mode, like every other caller. Its page copy no longer gets the STE-lite rules by default.

## [0.1.0] - 2026-10-13

The first release of monet as a Claude Code plugin.

### Added

- The `monet` plugin and its `monet` marketplace, so you install it with `/plugin marketplace add initor/monet` and `/plugin install monet@monet`.
- The monet skills, the `monet` agent, and the `ste-lite` output style. The old wrappers are commands folded into skills, so every old `/monet:<name>` keeps its name.
- The rename of `git-ship` to `ship`, so `/monet:ship` keeps its name.
- The `branch_prefix` and `data_dir` userConfig keys. Set them with `/plugin configure monet@monet`.
- Personas and interview-prep data in the data dir, which defaults to `~/.config/monet`. Nothing writes inside the plugin dir.
- A synthetic sample persona for `voice`, which `--persona sample` applies.
- Rewritten `distill-voice` scripts, with unit tests. They write personas to the data dir.
- The docs: the README, `docs/development.md`, `docs/authoring.md`, `docs/architecture.md`, and the gang and roadmap-handoff design specs in `docs/design/`.
- The `make check` gate (manifests, catalog, unit tests, scans, and the skill lint) and `make test`, which adds the gang tests.
- CI that runs `make test` on each pull request and each push to `main`, with a pinned claude CLI.
