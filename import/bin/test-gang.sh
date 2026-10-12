#!/usr/bin/env bash
# Run every gang test: go vet and go test for the CLI, then the engine simulator.
# Usage: bin/test-gang.sh (from any directory)
set -euo pipefail

ROOT="$(dirname "${BASH_SOURCE[0]}")/.."

go -C "$ROOT/.claude/skills/gang/cli" vet ./...
go -C "$ROOT/.claude/skills/gang/cli" test ./...
node "$ROOT/.claude/skills/gang/sim/sim.mjs" --template "$ROOT/.claude/skills/gang/engine/engine.template.js"
