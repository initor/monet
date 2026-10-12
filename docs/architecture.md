# Architecture

This page shows how the monet plugin is wired: the skills, the monet agent, the `ste-lite` output style, the `userConfig` values, and the data dir. For the list of skills, see the [skill catalog in the README](../README.md).

```mermaid
flowchart LR
  user(["User in Claude Code"])
  subgraph plugin["Plugin dir, replaced on each update"]
    manifest[".claude-plugin/plugin.json userConfig"]
    agent["agents/monet.md, runs as monet:monet"]
    skills["skills/, each typed as /monet:name"]
    style["output-styles/ste-lite.md, monet:ste-lite"]
    card["skills/voice/reference/ste-lite.md"]
  end
  data[("data dir: personas/, interview-prep/")]
  planning[("target repo: .planning/")]
  user -->|"/monet:name"| skills
  user -->|"claude --agent monet:monet"| agent
  agent -->|"routes to monet:name"| skills
  skills -->|"Skill tool, monet:name"| skills
  style -.->|"relative symlink"| card
  manifest -->|"user_config values"| skills
  manifest -->|"user_config values"| agent
  skills -->|"read and write user data"| data
  skills -->|"run scratch"| planning
```

## Skills

Each `skills/<name>/SKILL.md` is one skill, and a user types it as `/monet:<name>`. The plugin ships no `commands/` dir. The old command wrappers folded into their skills, so each `/monet:<name>` that worked before keeps its name.

A skill names its own files as `${CLAUDE_SKILL_DIR}/<path>`. `${CLAUDE_SKILL_DIR}` expands to the skill's dir in the installed plugin. A skill names a plain file of another skill as `${CLAUDE_PLUGIN_ROOT}/skills/<other>/<path>`. `${CLAUDE_PLUGIN_ROOT}` expands to the root of the installed plugin.

One skill calls another as `monet:<name>` through the Skill tool. For example, `pr` and `review-local` call `monet:paint` to draw their report. A skill never reads another skill's `SKILL.md` by path, because a raw read does not expand the placeholders in it.

## The agent

`agents/monet.md` runs as `monet:monet`. Start it with `claude --agent monet:monet`. It matches the request to a row of its routing table, and then it follows the numbered dispatch step for that skill. Each route names its skill as `monet:<name>`.

The agent sets no `permissionMode`, because plugins ignore `permissionMode`.

## The output style

The plugin ships one output style, `monet:ste-lite`. The file `output-styles/ste-lite.md` is a relative symlink to `../skills/voice/reference/ste-lite.md`. So the STE-lite card lives in one place. The `voice` skill and the output style read the same file. Edit the card in `skills/voice/reference/ste-lite.md` only.

## userConfig

`.claude-plugin/plugin.json` declares two string values:

| Key | Default | What it sets |
|---|---|---|
| `branch_prefix` | empty | The text in front of each branch name that monet makes, so a branch is `<prefix><slug>`. |
| `data_dir` | `~/.config/monet` | The dir that holds the user's own data. |

A skill or the agent reads a value as `${user_config.<key>}`. Set the values in one of these ways:

```
/plugin configure monet@monet
claude plugin configure monet@monet
claude plugin install monet@monet --config data_dir=<path>
```

An unset key stays as the literal placeholder text, even though `plugin.json` gives it a `default`. So each skill that uses a key falls back to the default itself. When the value is empty or still starts with `${user_config`, a skill uses no prefix for `branch_prefix`, and it uses `~/.config/monet` for `data_dir`. The defaults stay in `plugin.json` because `/plugin configure` shows them.

## The data dir

The data dir holds only the user's own data:

```
<data dir>/
  personas/          voice personas and the surface map personas.md
  interview-prep/    interview session records (*.json) and dashboard.html
    playgrounds/     generated practice pages
  handoffs/          handoff notes, only when the current dir is not writable
```

- `personas/`: `distill-voice` writes a persona and its row in the surface map. `voice` reads them first, and it falls back to the copies that it bundles.
- `interview-prep/`: `interview-tracker` writes the session records, and `bq-mock`, `sd-mock`, and `code-mock` record their sessions through it. `interview-progress` reads the records and writes the dashboard. `sd-explainer` reads the records to find weak areas. `sd-explainer`, `sd-playground`, `bq-playground`, and `code-playground` write their pages to `playgrounds/`.
- `handoffs/`: `handoff` writes here only as a fallback.

User data is not in the plugin dir, because Claude Code replaces the plugin cache on each update. It is not in `${CLAUDE_PLUGIN_DATA}` either, because Claude Code deletes that dir when the user uninstalls the plugin. A dir that the user owns keeps personas and interview history through an update, an uninstall, and a reinstall.

## Run scratch

Six skills write run scratch under the target repo's `.planning/`: `gang`, `handoff`, `pr`, `review-local`, `teach`, and `visual-design`. The target repo is the one that the user works in, not this repo. Other output, such as a `paint` page or the `reviews.md` of `review-code`, goes where the user or the skill names it. Only user data goes to the data dir, and nothing writes inside the plugin dir.
