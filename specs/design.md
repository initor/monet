# design

How monet moves into this repo. `goal.md` says what. This file says how, and it gives each requirement an id that a task names when it covers it.

## Source

- **Snapshot.** `initor/.config` at `3273e78995b8f4917b1986029f83965505f5b611`: the monet files under `.claude/` and five scripts under `bin/`. Read a file with `git -C ~/.config show 3273e78:<path>`.
- **Staging.** A bootstrap PR, merged before the run, copies the snapshot's monet files unchanged into `import/`, with the same relative layout as `.claude/` and the scripts under `import/bin/`.
- **Left out of `import/`.** Seven files, because they hold private or unlicensed content: `skills/voice/reference/personas/wayne.md`, the three scripts in `skills/distill-voice/scripts/`, `skills/distill-voice/reference/agent-content-filtering.md`, and the two specs in `docs/specs/`. A task that needs one reads it from the snapshot and writes new content.
- **`import/` is staging only.** No plugin component points at it, the gate skips it, and it's gone when the run ends.
- **History.** Only files come over. No dotfiles commit is carried into this repo.

## Decisions

1. **Packaging.** The repo root is a Claude Code plugin named `monet`, listed in a marketplace named `monet` in the same repo. Users install it with `/plugin marketplace add initor/monet` and `/plugin install monet@monet`.
2. **Commands fold into skills.** A plugin skill is typed as `/monet:<skill>`, so the 17 wrappers in `commands/monet/` become skills and no `commands/` dir ships. Every `/monet:<name>` that works today keeps its name.
3. **Personal content is generic and configurable.** Skills say "the user", not a name. The branch prefix and the data dir are plugin `userConfig` values. The voice persona and interview-prep data live in the user's data dir, never in the repo.
4. **The upstream-derived files are rewritten.** The repo that `distill-voice` borrowed three scripts and one reference from has no license, so those files are written new from the skill's own interface.
5. **The dotfiles cutover is out of scope for the run.** Removing monet from `initor/.config` is a separate PR in that repo, after the run, following `docs/migration.md`. The run itself uses the dotfiles copy of `gang`.
6. **Versioning.** `version` lives in `plugin.json` only, starting at `0.1.0`, with a `CHANGELOG.md`. A user gets an update when the version goes up.

## Port rules

Every skill port follows these rules. They aren't requirements on their own. The CORE and LAB requirements apply them.

- **P1. Move with `git mv`.** `import/skills/<name>/` goes to `skills/<name>/`, so history follows. Size a task by its edits with `git diff -M --stat`. A pure rename counts as 0 lines.
- **P2. Fold the command.** When `import/commands/monet/<cmd>.md` exists, its `argument-hint`, `allowed-tools`, `$ARGUMENTS` parsing, and precondition move into the skill's frontmatter and body, and the wrapper is deleted.
- **P3. Rewrite paths.** A skill's own file is `${CLAUDE_SKILL_DIR}/<path>`. Another skill's file is `${CLAUDE_PLUGIN_ROOT}/skills/<other>/<path>`. An `@~/.claude/...` include becomes a read instruction with the plugin path. A skill invoked by name is `monet:<name>`.
- **P4. Generic text.** "Wayne", "my", and "me" in instructions become "the user". `wawen/` becomes `${user_config.branch_prefix}`. Prose that refers to the user uses they or them.
- **P5. User data.** A skill that keeps user data reads and writes it under `${user_config.data_dir}`. Nothing writes inside the plugin dir.
- **P6. Tests move with their code.** A skill's tests live in `skills/<name>/tests/` and run from `make check` with `python3 -m unittest`, stdlib only.
- **P7. Prose.** Markdown has no em-dashes, and new prose follows the invariants in `skills/voice/SKILL.md`.
- **P8. Invocability.** Every skill stays typeable as `/monet:<name>`, unless it sets `user-invocable: false` today.

## FND: foundation

- **FND-1.** `.claude-plugin/plugin.json` names the plugin `monet`, with `description`, `version` `0.1.0`, `author`, `homepage`, `repository`, `license` `MIT`, and `keywords`. `.claude-plugin/marketplace.json` names the marketplace `monet`, owner `initor`, and one plugin with source `.` and no version. `claude plugin validate --strict` passes on `.claude-plugin/plugin.json` and on `.`.
- **FND-2.** `plugin.json` declares `userConfig` with two strings: `branch_prefix` (default empty, so a branch is `<prefix><slug>`) and `data_dir` (default `~/.config/monet`). Skills read them as `${user_config.<key>}`. An unset key stays as literal placeholder text, even with a `default`, so each use is followed by a sentence that falls back to the default when the value is empty or unexpanded.
- **FND-3.** `make check` is the run gate. It runs the manifest validation, the skill lint, the portability scan, the privacy scan, the prose scan, the catalog check, and every unittest suite. It skips `import/`, needs only `make`, `git`, `python3`, and `claude` on PATH, and exits nonzero on any failure.
- **FND-4.** The skill lint, ported from `import/bin/check-monet-skill.py` to the plugin layout, checks that each `skills/<name>/SKILL.md` has a `name` equal to its dir and a `description` that follows the authoring rules, that every skill the agent routes to exists, and that every `${CLAUDE_SKILL_DIR}` and `${CLAUDE_PLUGIN_ROOT}` path resolves to a tracked file. Its tests come from `import/bin/test-check-monet-skill.py`.
- **FND-5.** The portability scan fails when a tracked file outside `import/` holds `~/.claude/`, `$HOME/.claude`, `~/.config/.claude`, `/Users/<name>/`, or `/home/<name>/`. An allowlist file in the repo names the only files that may describe the dotfiles layout: `docs/migration.md`, the migration script, and the two specs in `specs/`.
- **FND-6.** The privacy scan fails on an email address other than a `noreply` or `example.com` address. It also reads extra patterns, one regex per line, from `$MONET_PRIVACY_PATTERNS` or `<data_dir>/privacy-patterns` when that file exists, so personal terms never sit in the repo. CI runs the built-in patterns only.
- **FND-7.** The prose scan fails when a tracked Markdown file outside `import/` holds an em-dash (U+2014).
- **FND-8.** `make test` runs `make check`, then `go vet ./...` and `go test ./...` for `skills/gang/cli`, then the gang simulator, ported from `import/bin/test-gang.sh`.
- **FND-9.** `.github/workflows/check.yml` runs `make test` on pull requests and on pushes to `main`, on `ubuntu-latest`, with the `claude` CLI installed from npm at a pinned version.
- **FND-10.** `.gitignore` ignores `.planning/`, gang build output, `__pycache__/`, and `.DS_Store`. The `LICENSE` stays MIT.

## CORE: the agent and the workflow skills

- **CORE-1.** `import/agents/monet.md` moves to `agents/monet.md` and runs as `monet:monet`. Its routing table and dispatch steps name skills as `monet:<name>`, and it drops `permissionMode`, which plugins ignore.
- **CORE-2.** `output-styles/ste-lite.md` ships as `monet:ste-lite`. It's a relative symlink to `../skills/voice/reference/ste-lite.md`, so the card lives in one place.
- **CORE-3.** `help`, `status`, and `new-skill` become skills built from their commands, `import/workflows/monet/help.md`, and `import/templates/skill-template.md`. `help` and `status` compute the inventory from the plugin dir. `new-skill` scaffolds `skills/<name>/`, adds the agent's routing row and dispatch step, and runs `make check`.
- **CORE-4.** `git-changes`, `git-draft-commit`, `git-draft-pr`, and `git-ship` are ported. `git-ship` becomes `ship`, in its dir and its `name`, so `/monet:ship` keeps its name, and every reference to `git-ship` follows.
- **CORE-5.** `voice` is ported. It resolves the persona map and persona files from `${user_config.data_dir}/personas/` first and falls back to the bundled map, which maps no surface, so a new user gets the invariants only. `ste_check.py` and its tests from `import/bin/test-ste-check.py` move with it.
- **CORE-6.** `voice` ships one new persona, `reference/personas/sample.md`, with the same sections as a distilled persona and synthetic samples only. `--persona sample` applies it.
- **CORE-7.** `distill-voice` is ported and writes personas to `${user_config.data_dir}/personas/`. Its three scripts and `reference/agent-content-filtering.md` are written new, from the inputs and outputs that its `SKILL.md` describes, with unittest tests. The task doesn't read the snapshot's copies of those four files.
- **CORE-8.** `review-code`, `review-local`, and `pr` are ported. Their calls to `monet:paint` still resolve.
- **CORE-9.** `handoff`, `teach`, `cwd-context`, `roadmap`, and `web-operator` are ported.
- **CORE-10.** `gang` is ported. Its build line uses `${CLAUDE_PLUGIN_ROOT}/skills/gang` for both `go -C` and `main.skillDir`. The task schema it reads at runtime moves into `skills/gang/reference/schema.md`. Its prompts refer to "the user". Test fixtures use a neutral repo name. `make test` passes.

## LAB: the visual and practice skills

- **LAB-1.** `paint` is ported, with its tests from `import/bin/test-paint-explainer.py`.
- **LAB-2.** `visual-design` is ported.
- **LAB-3.** `hairline-create` is ported with only the path rewrites. It keeps its `LICENSE`, and `UPSTREAM.md` lists the local changes.
- **LAB-4.** `sd-explainer` and `sd-playground` are ported.
- **LAB-5.** `prep`, `sd-mock`, `bq-mock`, `code-mock`, `bq-playground`, `code-playground`, `interview-tracker`, and `interview-progress` are ported. They keep sessions under `${user_config.data_dir}/interview-prep/` instead of `~/.config/interview-prep/`.

## DOC: docs and migration

- **DOC-1.** `README.md` is at most 150 lines. It holds a one-paragraph pitch, install in two commands, three first-run examples, the skill catalog, configuration, update and uninstall, and links into `docs/`.
- **DOC-2.** A script generates the skill catalog from each `SKILL.md`'s `name` and the first sentence of its `description`, between BEGIN and END markers in `README.md`. `make check` fails when the catalog is stale.
- **DOC-3.** `docs/development.md` covers the dev loop (the checkout added as a local-path marketplace, `/reload-plugins`), `make check` and `make test`, CI, and a release (bump the version, update `CHANGELOG.md`, `claude plugin tag`).
- **DOC-4.** `docs/authoring.md` covers adding or changing a skill in the plugin layout, from the dotfiles README's "Authoring a skill", with `new-skill` as the fast path.
- **DOC-5.** `docs/architecture.md` shows how monet is wired: skills, the agent, the output style, `userConfig`, and the data dir, with one Mermaid diagram.
- **DOC-6.** `docs/migration.md` takes a user who runs monet from dotfiles to the plugin: install, configure, move data, remove the old copies so nothing loads twice, and verify. It ends with the dev loop from DOC-3, so the author updates monet the same way other users do.
- **DOC-7.** `scripts/migrate-from-dotfiles.sh` is a dry run by default and prints each step. `--apply` copies personas and interview-prep data into the data dir, and moves the old copies (`agents/monet.md`, `commands/monet/`, the monet skill dirs, `output-styles/ste-lite.md`) into a timestamped backup dir. It never deletes. `--check` exits nonzero while an old copy would load next to the plugin. Its tests run against a temp `HOME`.
- **DOC-8.** `docs/design/` holds the gang and roadmap-handoff design specs, written from the snapshot with work paths, work-repo details, and `lessons.md` citations removed.
- **DOC-9.** `CHANGELOG.md` has the `0.1.0` entry, and `CONTRIBUTING.md` points to `docs/development.md` and `docs/authoring.md`.
- **DOC-10.** The last task removes `import/` and checks that every snapshot file is either ported or on the left-out list above.

## Traceability to goal.md

| goal.md line | Closed by |
|---|---|
| 3: move monet into this repo | DOC-10 |
| 3: world-class repo with a neat, simple, clear README and docs | DOC-1, DOC-5 |
| 5: new users install, explore, and use monet from the README | DOC-1 |
| 6: a clear migration path for the existing user | DOC-6, DOC-7 |
| 6: once migrated, use and update monet the same way as other users | DOC-6 |
| 14: refactor so the standalone repo makes sense | the CORE and LAB ports |
| 20: monet:voice shapes the writing | P7, and every DOC task |
| 21-22: monet:ship and monet:handoff | the coordinator: bootstrap PRs and context handoffs |
