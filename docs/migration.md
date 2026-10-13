# Migrate from dotfiles

This guide is for a user who runs monet from a dotfiles copy under `~/.claude/`. It moves you to the monet plugin. After it, you install, use, and update monet the same way as every other user.

## Before you start

1. Finish or stop each session on the dotfiles copies, such as a gang build on the old skill dir. That dir moves in this guide.
2. Check that your dotfiles tree is committed, so `git status` in that repo is clean. Then each change that this guide makes is easy to see and easy to undo.
3. The script copies your data into `~/.config/monet/` and puts its backup there too. When `~/.config` is a git checkout, add `monet/` to its `.gitignore` before `--apply`, or pass `--backup-dir` outside it. Otherwise the backup shows up in that repo as new files to commit. The `.gitignore` entry also keeps your copied personas out of that repo.

## Install

Run these two commands in Claude Code:

```
/plugin marketplace add initor/monet
/plugin install monet@monet
```

The skills load in a new session, or after `/reload-plugins`. Each skill is typed as `/monet:<skill>`.

## Configure

monet has two options. Set them with `/plugin configure monet@monet` in Claude Code. From a shell, `claude plugin configure monet@monet` shows the options and which ones are unset, and `claude plugin configure monet@monet --values-stdin` saves values from a JSON object. You can also set a value at install time with `claude plugin install monet@monet --config data_dir=<path>`.

- `branch_prefix`: the text in front of each branch name that monet creates. Set it to the prefix that the old skills hard-coded. For example, the prefix `alice/` gives the branch `alice/fix-login`.
- `data_dir`: the dir where monet keeps your own data. The default in `plugin.json` is `~/.config/monet`.

An unset key stays as the literal placeholder text in each skill, even though `plugin.json` gives it a `default`. So each skill falls back to the default itself. An unset `branch_prefix` means no prefix, and an unset `data_dir` means `~/.config/monet`.

## Move your data

The migration script is in the monet repo. Get it with a clone into a temp dir, or `cd` into a checkout that you already have:

```
dir=$(mktemp -d)
git clone https://github.com/initor/monet "$dir/monet"
cd "$dir/monet"
```

Run every command below from that checkout. The clone is only for the script. The marketplace install from GitHub stays the copy that Claude Code loads.

First, run the script with no option. This is a dry run: it prints each copy and each move, and it changes nothing.

```
scripts/migrate-from-dotfiles.sh
```

Then run it with `--apply`:

```
scripts/migrate-from-dotfiles.sh --apply
```

When you set `data_dir` to a value other than the default, pass the same dir with `--data-dir <dir>` on each run. The script copies these, and leaves each source in place:

- the persona files from `~/.claude/skills/voice/reference/personas/` and the persona map `personas.md`, to `<data dir>/personas/`
- the interview-prep data in `~/.config/interview-prep/`, to `<data dir>/interview-prep/`

When a destination exists with other content, the script names each conflict, changes nothing, and exits 1.

## Remove the old copies

The `--apply` run also moves the old copies out of `~/.claude/`, so nothing loads twice. It moves `agents/monet.md`, `commands/monet/`, `output-styles/ste-lite.md`, and these skill dirs under `skills/`:

```
bq-mock            bq-playground      code-mock          code-playground
cwd-context        distill-voice      gang               git-changes
git-draft-commit   git-draft-pr       git-ship           hairline-create
handoff            interview-progress interview-tracker  paint
pr                 prep               review-code        review-local
roadmap            sd-explainer       sd-mock            sd-playground
teach              visual-design      voice              web-operator
```

The script never deletes a file. Each old copy goes to the backup dir at the same relative path. The default backup dir is `<data dir>/backup/dotfiles-<YYYYmmdd-HHMMSS>`, and the last line of the run names it. To restore the old copies, first run `claude plugin uninstall monet@monet`, so nothing loads twice. Then move each copy back:

```
backup=<the backup dir that the run printed>
mv "$backup/agents/monet.md" ~/.claude/agents/monet.md
mv "$backup/commands/monet" ~/.claude/commands/monet
mv "$backup/output-styles/ste-lite.md" ~/.claude/output-styles/ste-lite.md
mv "$backup"/skills/* ~/.claude/skills/
```

When `~/.claude` is a git checkout (your dotfiles), the moves show up in that repo as deletions. Commit them there. The script leaves the monet-only files that do not load. Remove them in the same commit: the templates, the workflow bodies, and the old check scripts.

If you start the agent with an alias such as `claude --agent monet`, change it to `claude --agent monet:monet`.

## Verify

1. Run `scripts/migrate-from-dotfiles.sh --check` from the checkout. It exits 0 when no old copy is left. Otherwise it names each old copy and exits 1.
2. Start a new session, or run `/reload-plugins`.
3. Run `/monet:status skills`. It lists the skills of the plugin.

## Update and develop

The GitHub install from Install is the default, and it stays your update path. To update, run `claude plugin update monet@monet`, then restart Claude Code or run `/reload-plugins`.

To edit monet itself, use the dev loop in [Development](development.md). It is optional. It loads your checkout as the `monet` marketplace in place of GitHub. When you are done, follow its steps to [switch back to the GitHub marketplace](development.md#the-dev-loop).
