# monet: repo checks.
#
# `make check` is the gate. It runs its steps one at a time (.NOTPARALLEL holds
# even under make -j), in the order they are listed as prerequisites of
# `check`, and it stops at the first step that fails.
# `make test` runs `make check`, then any step appended to `test`.
#
# To add a step, append these lines at the end of this file, and edit nothing
# above them:
#
#   check: <step>          (or `test: <step>` for a step that runs after check)
#   .PHONY: <step>
#   <step>:
#       <recipe>       (each recipe line starts with a tab)
#
# The step names scan, lint, and gang are reserved for the tasks that add them.
#
# This file must run under GNU Make 3.81 (macOS /usr/bin/make): no .ONESHELL,
# no $(file ...), no != or ::= assignments, and no .RECIPEPREFIX.

SHELL := /bin/sh
.NOTPARALLEL:

# Python runs with tools/ importable and writes no bytecode into the tree.
# Make exports these, so no shell quoting can break on the checkout path.
catalog unittest: export PYTHONPATH := $(CURDIR)/tools
catalog unittest: export PYTHONDONTWRITEBYTECODE := 1

.PHONY: check
check: prereqs manifest catalog unittest

.PHONY: test
test: check

# Fail early, with a clear message, when a tool that check needs is missing.
.PHONY: prereqs
prereqs:
	@echo "==> prereqs: git, python3, claude"
	@missing=0; \
	for tool in git python3; do \
	  if ! command -v "$$tool" >/dev/null 2>&1; then \
	    echo "error: '$$tool' is not on PATH, and make check needs it." >&2; \
	    missing=1; \
	  fi; \
	done; \
	if ! command -v claude >/dev/null 2>&1; then \
	  echo "error: the claude CLI (Claude Code) is not on PATH, and make check needs it to validate the plugin manifests. Install it with: npm install -g @anthropic-ai/claude-code" >&2; \
	  missing=1; \
	fi; \
	exit $$missing

# Validate plugin.json, then the repo root, which checks marketplace.json.
# Neither run catches a SKILL.md whose frontmatter does not parse.
.PHONY: manifest
manifest:
	@echo "==> manifest: claude plugin validate --strict"
	claude plugin validate --strict .claude-plugin/plugin.json
	claude plugin validate --strict .

# The README catalog check, once tools/catalog.py exists.
.PHONY: catalog
catalog:
	@if [ -f tools/catalog.py ]; then \
	  echo "==> catalog: python3 tools/catalog.py --check"; \
	  python3 tools/catalog.py --check; \
	else \
	  echo "==> catalog: skipped, tools/catalog.py is absent"; \
	fi

# Run each test dir as its own suite, keep going after a failing suite, and
# fail at the end when any suite failed.
.PHONY: unittest
unittest:
	@failed=""; \
	for dir in tools/tests skills/*/tests; do \
	  [ -d "$$dir" ] || continue; \
	  echo "==> unittest: $$dir"; \
	  if ! python3 -m unittest discover -s "$$dir" -t "$$dir" -p 'test_*.py'; then \
	    failed="$$failed $$dir"; \
	  fi; \
	done; \
	if [ -n "$$failed" ]; then \
	  echo "error: unittest suites failed:$$failed" >&2; \
	  exit 1; \
	fi
