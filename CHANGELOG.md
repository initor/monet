# Changelog

This file lists the notable changes to monet in each release. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow [Semantic Versioning](https://semver.org/). The version lives in `.claude-plugin/plugin.json` only.

## [Unreleased]

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
