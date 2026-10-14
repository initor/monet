# Authoring a skill

This guide shows how to add or change a monet skill in the plugin layout. Each skill is one dir under `skills/`. Work in a git checkout of this repo, never in the copy that Claude Code installed.

## Fast path: `/monet:new-skill`

Run `/monet:new-skill <name>` from a checkout of the repo. It asks about the purpose, the trigger phrases, and the sibling skills. Then it does these steps:

1. It writes `skills/<name>/SKILL.md` from its template, plus any reference files.
2. It registers the skill in `agents/monet.md` with a routing row and a dispatch line.
3. It runs `make check` and fixes each failure at its cause.

It never commits. It does not write tests or refresh the README catalog, so do steps 4 and 5 below yourself. Ship the change with `/monet:ship`.

## Manual steps

Do these steps in this order when you write a skill by hand. To change a skill, do the steps that the change touches.

### 1. Name the skill

Use kebab-case: lowercase letters, digits, and hyphens. Name the skill for what it produces. The dir name, the frontmatter `name`, and the text after `/monet:` are the same string. For example, `skills/git-changes/` holds `name: git-changes`, and users type `/monet:git-changes`.

Some skills form a prefix family, such as `git-*` for the git skills and `review-*` for the review skills. A new skill joins a family only when it belongs there. Do not add a prefix to group a skill by topic alone.

### 2. Write `skills/<name>/SKILL.md`

The file starts with a YAML frontmatter block. `tools/check_skills.py` lints it. To start from the same layout that `/monet:new-skill` uses, copy `skills/new-skill/reference/skill-template.md`.

The lint accepts these frontmatter keys:
`name`, `description`, `argument-hint`, `allowed-tools`, `model`,
`disable-model-invocation`, and `user-invocable`.

Any other key fails `make check`. The `description` must start with `Use when`, and it must be at most 500 characters. Aim for about 400. Put the literal trigger phrases in it, and one line for each sibling that a reader can confuse it with. Add a mirror line to that sibling's description too, so routing works in both directions. Quote the description if it holds `: `, because a strict loader drops a skill whose frontmatter does not parse.

The skill is its own slash command, `/monet:<name>`. No `commands/` dir ships, because commands folded into skills. Set `user-invocable: false` for a skill that only the agent routes to.

Write each path in the body so it works on any machine:

- `${CLAUDE_SKILL_DIR}/<path>` names a file in the skill's own dir, such as `${CLAUDE_SKILL_DIR}/reference/checklist.md`.
- `${CLAUDE_PLUGIN_ROOT}/skills/<other>/<path>` names a plain file in another skill's dir, such as a script.
- `monet:<name>` loads another skill with the Skill tool. Never read another skill's `SKILL.md` by path.

The lint checks that each `${CLAUDE_SKILL_DIR}` and `${CLAUDE_PLUGIN_ROOT}` path resolves to a file in the repo.

Two user settings come from `plugin.json`: `${user_config.branch_prefix}` and `${user_config.data_dir}`. User data goes under the data dir. Nothing writes inside the plugin dir, because an update replaces it.

Claude Code puts a configured value in place of its placeholder. An unset key stays as the literal placeholder text, even though `plugin.json` gives it a `default`. So each skill falls back to the default itself. It uses no prefix for `branch_prefix`, and it uses `~/.config/monet` for `data_dir`.

A skill writes each placeholder once and follows it with a fallback sentence. The fallback sentence never repeats the full placeholder, because a configured value would replace it there too. Copy the sentence for each key from `USER_CONFIG_FALLBACKS` in `tools/skillkit.py`:

```markdown
Save the file under `${user_config.data_dir}/notes/`. If the data dir above is empty or starts with `${user_config`, it isn't configured, so use `~/.config/monet`.

Create the branch `${user_config.branch_prefix}<slug>`. If the prefix above is empty or starts with `${user_config`, it isn't configured, so use no prefix.
```

A contract test can call `skillkit.missing_fallbacks` on the skill text to check this rule.

### 3. Register it in `agents/monet.md`

The monet agent routes requests to skills. Add the skill in two places:

1. Add a routing row to the Quick-reference routing table. Put `` `monet:<name>` `` in the Skill column, next to the rows of its siblings.
2. Add a numbered dispatch step immediately before the `Safety defaults` step. Find that step by name, not by number, because the list grows and changes order. Then renumber the steps after it. When an existing step already covers the skill's domain, add a bullet to that step instead.

The lint checks that each `monet:<name>` in the agent names a skill, an agent, or an output style.

### 4. Add tests

Tests live in `skills/<name>/tests/`. Name each test file `test_<x>.py`. A tests dir holds at least one test, because `unittest discover` exits 5 on an empty dir and fails the gate. Use stdlib `unittest` only. A test may `import skillkit` to use `tools/skillkit.py`, which reads frontmatter, path refs, and fallback sentences.

`make check` runs each `skills/*/tests` dir as its own suite, so a new dir needs no Makefile edit. Run one suite while you work:

```sh
PYTHONPATH=tools python3 -m unittest discover -s skills/<name>/tests -t skills/<name>/tests -p 'test_*.py'
```

### 5. Refresh the README catalog

The skill catalog in `README.md` comes from each skill's `name` and the first sentence of its `description`. Run `python3 tools/catalog.py --write` after adding or renaming a skill, or after a change to the first sentence of its description. Commit the README change with the skill. `make check` fails on a stale catalog.

### 6. Run `make check`

Run `make check` from the repo root. It validates the plugin manifests, checks the catalog, runs every test suite, scans for machine paths, private data, and em-dashes, and lints each skill. Fix each failure at its cause, and run it again until it passes. Never weaken a check to get green.

To try the skill in a live session, see the dev loop in [development.md](development.md).
