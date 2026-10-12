# Contributing to monet

Thank you for your help. This page lists the rules for a change. The guides hold the details.

## Read the guides

- [docs/development.md](docs/development.md) shows the dev loop, the `make check` gate, CI, and a release.
- [docs/authoring.md](docs/authoring.md) shows how to add or change a skill.

## Before you open a pull request

1. Cut a new branch from the latest `main`.
2. Keep each pull request to one change.
3. Add or update the tests for your change.
4. Run `make check` and make sure that it passes.
5. Add a line to the `[Unreleased]` section of `CHANGELOG.md` when users see the change.

## Commits

Each commit subject follows [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/): `<type>[(<scope>)]: <description>`. Write it in lowercase and in the imperative, with no period at the end. For example, `fix(voice): count a quote as one word`. The pull request title matches the main commit subject.

## Prose

Do not use an em-dash in a file, a comment, or a commit message. Use a comma, a colon, parentheses, or a new sentence. Keep personal data out of the repo, because the repo is public.
