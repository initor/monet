# Upstream

This folder is a copy of `skills/hairline-create` from github.com/lucasmarkes/hairline, at commit
bc782244216620434b14736df1d74daed2d05046 (2026-10-05). It is MIT licensed, and `LICENSE` holds the
notice.

Every file that the list below does not name is unchanged. Do not edit them. `validate.mjs` rejects
a page whose kernel differs from `kernel.js`, and paint's explainer inlines the same kernel.

## Local changes

- `SKILL.md`: the description names the command `/monet:hairline-create`, and the steps run `build.mjs`, `validate.mjs`, and `look.mjs` by their path under `${CLAUDE_SKILL_DIR}`. Nothing else changed.
- `UPSTREAM.md`: added by monet. Upstream has no such file. It holds the pin, this list, and the update steps.
- `tests/`: added by monet. Its contract test checks the license, the pin, this list, the `SKILL.md` paths, and the `kernel.js` hash line.

## Update

To update, re-copy every upstream file from a newer commit over this folder, and keep `tests/` and
this file. Put the new commit and its date in the first paragraph. Then reapply the local changes
listed above to `SKILL.md`, and run `make check` from the repo root.
