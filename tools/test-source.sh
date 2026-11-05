#!/usr/bin/env bash
# Run every monet-source test: go vet and go test for the CLI.
# Usage: tools/test-source.sh (from any directory)
set -euo pipefail

ROOT="$(dirname "${BASH_SOURCE[0]}")/.."

go -C "$ROOT/skills/source/cli" vet ./...
go -C "$ROOT/skills/source/cli" test ./...
