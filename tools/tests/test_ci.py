"""Contract tests for the CI workflow, .github/workflows/check.yml (FND-9).

The tests read the workflow text with the stdlib only, so they parse the
small block-style YAML subset that the workflow uses: keys, nested maps by
indent, and a list of steps.
"""

import re
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / ".github" / "workflows" / "check.yml"
PACKAGE = "@anthropic-ai/claude-code"
# The pin the gate's no-login proof installs (owner ruling G1). Bump this,
# check.yml, and the FND6 verify command together.
PIN = "2.1.296"


def lines():
    """Return the workflow's non-blank lines, with full-line comments dropped."""
    text = WORKFLOW.read_text(encoding="utf-8")
    return [l.rstrip() for l in text.splitlines() if l.strip() and not l.lstrip().startswith("#")]


def indent(line):
    return len(line) - len(line.lstrip(" "))


def block(body, key):
    """Return the lines nested under the first `key:` line in body."""
    for i, line in enumerate(body):
        if line.strip() == key + ":":
            rest = []
            for nested in body[i + 1:]:
                if indent(nested) <= indent(line):
                    break
                rest.append(nested)
            return rest
    raise AssertionError(f"no `{key}:` block in {WORKFLOW.name}")


def children(body):
    """Return the lines at body's shallowest indent, stripped."""
    return [l.strip() for l in body if indent(l) == indent(body[0])] if body else []


def steps():
    """Return each step of the one job as a dict of its stripped `key: value` lines."""
    body = block(block(lines(), "jobs")[1:], "steps")
    out = []
    for line in body:
        text = line.strip()
        if indent(line) == indent(body[0]) and text.startswith("- "):
            out.append({})
            text = text[2:]
        key, _, value = text.partition(":")
        out[-1][key] = value.strip()
    return out


class Workflow(unittest.TestCase):
    def test_triggers_on_pull_request_and_push_to_main(self):
        on = block(lines(), "on")
        self.assertEqual(sorted(children(on)), ["pull_request:", "push:"])
        self.assertEqual(block(on, "pull_request"), [], "pull_request has no filter")
        push = block(on, "push")
        self.assertEqual(len(push), 1)
        self.assertEqual(children(push), ["branches: [main]"])

    def test_runs_on_ubuntu_latest_with_read_only_permissions(self):
        jobs = block(lines(), "jobs")
        self.assertEqual(len(children(jobs)), 1, "the workflow has exactly one job")
        job = jobs[1:]
        self.assertIn("runs-on: ubuntu-latest", children(job))
        self.assertEqual(block(job, "permissions"), [" " * indent(job[0]) + "  contents: read"])
        self.assertNotIn("secrets.", WORKFLOW.read_text(encoding="utf-8"))

    def test_claude_cli_pinned_to_exact_version(self):
        pins = re.findall(re.escape(PACKAGE) + r"@(\S+)", WORKFLOW.read_text(encoding="utf-8"))
        self.assertEqual(len(pins), 1, f"exactly one {PACKAGE} install")
        self.assertEqual(pins[0], PIN, "the exact version the no-login proof checks")
        runs = [s.get("run") for s in steps()]
        self.assertIn(f"npm install -g {PACKAGE}@{PIN}", runs)

    def test_runs_make_test_unguarded(self):
        got = [s.get("uses") or s.get("run") for s in steps()]
        self.assertEqual(len(got), 6)
        self.assertEqual(got[:3], ["actions/checkout@v4", "actions/setup-node@v4", "actions/setup-go@v5"])
        self.assertRegex(got[3], r"^npm install -g " + re.escape(PACKAGE) + r"@\d+\.\d+\.\d+$")
        self.assertEqual(got[4:], ["claude --version", "make test"])
        self.assertEqual(steps()[1].get("node-version"), '"22"')
        self.assertEqual(steps()[2].get("go-version"), '"1.24.x"')
        text = WORKFLOW.read_text(encoding="utf-8")
        for guard in ("continue-on-error", "if:", "|| true"):
            self.assertNotIn(guard, text)


if __name__ == "__main__":
    unittest.main()
