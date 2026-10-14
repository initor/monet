# Development

This guide is for a contributor who changes monet from a checkout of this repo.

## The dev loop

Load your checkout as a local-path marketplace. Run these commands in Claude Code:

```
/plugin marketplace add <path to the checkout>
/plugin install monet@monet
/reload-plugins
```

Claude Code then loads the skills from your checkout. An edit applies on the next session, or after `/reload-plugins`, with no version bump.

For a quick test, `claude --plugin-dir <path>` loads a checkout for one session only. In that session, it silently replaces an installed plugin named monet. So make sure that you know which copy you test.

The GitHub marketplace and your checkout are both named `monet`, so only one of them can be configured at a time. To switch from GitHub to your checkout, run:

```
claude plugin marketplace remove monet
claude plugin marketplace add <path to the checkout>
claude plugin install monet@monet
```

To switch back, run the same three commands with `initor/monet` in place of the path. Then start a new session. Run `claude plugin marketplace list` to see which source `monet` points at now.

## The gate: `make check` and `make test`

`make check` is the gate. It runs its steps one at a time, in this order, and it stops at the first step that fails:

1. `prereqs`: checks that `git`, `python3`, and `claude` are on PATH.
2. `manifest`: runs `claude plugin validate --strict` on `.claude-plugin/plugin.json`, then on the repo root, which checks `.claude-plugin/marketplace.json`.
3. `catalog`: runs `python3 tools/catalog.py --check` when that file exists, and skips otherwise.
4. `unittest`: runs `tools/tests` and each `skills/*/tests` dir as its own suite. It fails at the end when any suite fails.
5. `scan`: runs `python3 tools/scan.py`, which holds the portability, privacy, and prose scans.
6. `lint`: runs `python3 tools/check_skills.py`, the skill lint.

`make check` needs only `make`, `git`, `python3`, and `claude`.

`make test` runs `make check` first. Then `tools/test-gang.sh` runs `go vet ./...` and `go test ./...` for `skills/gang/cli`. Last, it runs the gang simulator. These steps need Go 1.24 or later and Node 22 or later, as `skills/gang/SKILL.md` requires.

The privacy scan flags an email address, except for a short list of built-in safe addresses. It also reads extra regexes, one per line, from a file outside the repo. That file is the one that `$MONET_PRIVACY_PATTERNS` names, else `~/.config/monet/privacy-patterns`. So your personal terms never sit in the repo. CI sets neither, so CI runs the built-in patterns only.

## CI

`.github/workflows/check.yml` runs on each pull request and on each push to `main`. It installs Node, Go, and the claude CLI, and then it runs `make test`.

The claude CLI version is pinned in the workflow, so a CLI release cannot change what `claude plugin validate` accepts. To move to a new CLI version, change the pin in the workflow and run `make test` locally with that version. The job reads no secret.

## A release

1. Bump `version` in `.claude-plugin/plugin.json`. Do not add a version anywhere else.
2. Add a `CHANGELOG.md` entry for the new version. Move what `## [Unreleased]` holds into it, and date it the day you tag.
3. Commit both changes and merge them to `main`.
4. From a clean checkout of `main`, run `claude plugin tag --dry-run`. It checks that `plugin.json` and the marketplace entry agree, and prints the tag.
5. Run `claude plugin tag --push`. It makes the tag `monet--v<version>` and pushes it to `origin`.
6. Publish the GitHub release from that tag, with the new `CHANGELOG.md` entry as its notes: `gh release create monet--v<version> --verify-tag --title "monet <version>" --notes-file <notes.md>`.

Users see the update only after the bump. A change that keeps the old version does not reach an installed copy.
