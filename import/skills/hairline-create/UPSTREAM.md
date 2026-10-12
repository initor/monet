# Upstream

This folder is a copy of `skills/hairline-create` from github.com/lucasmarkes/hairline, at commit
bc782244216620434b14736df1d74daed2d05046 (2026-10-05). It is MIT licensed, and `LICENSE` holds the
notice.

Every file except this one and `LICENSE` is unchanged. Do not edit them. `validate.mjs` rejects a
page whose kernel differs from `kernel.js`, and paint's explainer inlines the same kernel. To update,
copy the folder from a newer commit, update this file, and run `bin/test-paint-explainer.py`.
