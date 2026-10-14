# Changelog

This file lists the notable changes to monet in each release. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow [Semantic Versioning](https://semver.org/). The version lives in `.claude-plugin/plugin.json` only.

## [Unreleased]

### Removed

- The interview-prep skills: `prep`, `sd-mock`, `bq-mock`, `code-mock`, `sd-explainer`, `sd-playground`, `bq-playground`, `code-playground`, `interview-tracker`, and `interview-progress`. They were personal tools, not engineering workflow. The monet agent no longer routes interview requests. Session records that they wrote stay in `<data dir>/interview-prep/` until you remove them.

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
