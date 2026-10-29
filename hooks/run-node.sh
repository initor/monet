#!/bin/sh
# Resolve a node binary at run time and exec it with the given arguments.
#
# An absolute node path names one user and one nvm version, so a hook that
# pins one fails with exit 127 on any other machine. hooks.json runs the
# relay hook through this shim instead.
#
# Fail open. A hook that cannot find node must not block a tool use.
set -u

run() {
  candidate="$1"
  shift
  [ -n "$candidate" ] && [ -x "$candidate" ] && exec "$candidate" "$@"
}

# 1. PATH, the common case for a terminal-launched session.
run "$(command -v node 2>/dev/null || true)" "$@"

# 2. nvm without its shell function loaded (GUI launch, or a lazy-loading
#    zshrc where `node` is a function until first use). Newest version wins.
nvm_dir="${NVM_DIR:-$HOME/.nvm}"
if [ -d "$nvm_dir/versions/node" ]; then
  newest="$(ls -d "$nvm_dir"/versions/node/*/bin/node 2>/dev/null | sort -V | tail -1)"
  run "$newest" "$@"
fi

# 3. Homebrew on Apple Silicon, then Intel Homebrew or a plain /usr/local install.
for candidate in /opt/homebrew/bin/node /usr/local/bin/node; do
  run "$candidate" "$@"
done

echo "run-node.sh: node not found, skipping ${1:-hook}" >&2
exit 0
