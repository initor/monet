"""Contract for the development guide, docs/development.md (DOC-3)."""

import json
import re
import subprocess
import sys
import unittest
from pathlib import Path

import skillkit

ROOT = Path(__file__).resolve().parents[2]
DOC = ROOT / "docs" / "development.md"
STE_CHECK = ROOT / "skills" / "voice" / "scripts" / "ste_check.py"
MAKEFILE = ROOT / "Makefile"
WORKFLOW = ROOT / ".github" / "workflows" / "check.yml"
GANG_SKILL = ROOT / "skills" / "gang" / "SKILL.md"
PLUGIN = ROOT / ".claude-plugin" / "plugin.json"

# The four level 2 headings, in order, each found by a word it must hold.
SECTIONS = ("dev loop", "gate", "CI", "release")

FENCE = re.compile(r"^```[^\n]*\n(.*?)^```$", re.M | re.S)


def doc_text():
    return DOC.read_text(encoding="utf-8")


def headings(text):
    return re.findall(r"^## (.+)$", text, re.M)


def section(text, word):
    """The body of the level 2 section whose heading holds `word`."""
    parts = re.split(r"^## ", text, flags=re.M)[1:]
    for part in parts:
        title, _, body = part.partition("\n")
        if word in title:
            return body
    return ""


def check_steps():
    """The prerequisites of `check`, in the order the Makefile lists them."""
    steps = []
    for line in MAKEFILE.read_text(encoding="utf-8").split("\n"):
        match = re.match(r"^check:\s*(.*)$", line)
        if match:
            steps += match.group(1).split()
    return steps


def flat(body):
    """The body with line breaks folded into spaces."""
    return re.sub(r"\s+", " ", body)


def prose(body):
    """The body without its fenced code blocks, folded to one line."""
    return flat(FENCE.sub(" ", body))


def sentences(body):
    """The prose sentences of a body."""
    return re.split(r"(?<=\.) ", prose(body))


def code(body):
    """The fenced code blocks and the inline code spans of a body."""
    spans = re.findall(r"`([^`\n]+)`", FENCE.sub(" ", body))
    return "\n".join(FENCE.findall(body) + spans)


def github_source():
    """The owner/repo of the GitHub marketplace, from plugin.json `repository`."""
    repo = json.loads(PLUGIN.read_text(encoding="utf-8"))["repository"]
    return re.search(r"github\.com/([^/]+/[^/.]+)", repo).group(1)


class DevelopmentDoc(unittest.TestCase):
    def test_sections_in_order(self):
        found = headings(doc_text())
        self.assertEqual(len(found), len(SECTIONS), found)
        for title, word in zip(found, SECTIONS):
            with self.subTest(heading=title):
                self.assertIn(word, title)

    def test_covers_dev_loop(self):
        raw = section(doc_text(), "dev loop")
        body = flat(raw)
        for needle in ("/plugin marketplace add <path to the checkout>",
                       "/plugin install monet@monet", "/reload-plugins",
                       "no version bump", "next session",
                       "claude --plugin-dir <path>", "one session",
                       "silently replaces", "named `monet`"):
            with self.subTest(needle=needle):
                self.assertIn(needle, body)
        # The prose, not only the code block, says /reload-plugins applies an edit.
        self.assertTrue(
            any("/reload-plugins" in s and "no version bump" in s
                for s in sentences(raw)),
            "no prose sentence says /reload-plugins applies an edit with no version bump")

    def test_dev_loop_says_how_to_switch(self):
        raw = section(doc_text(), "dev loop")
        recipe = code(raw)
        named = re.findall(r"marketplace ([a-z]+)", recipe)
        for name in ("remove", "add", "list"):
            with self.subTest(subcommand=name):
                self.assertIn(name, named)
        # The recipe removes the old `monet` before it adds the other source.
        remove = recipe.find("claude plugin marketplace remove monet")
        self.assertNotEqual(remove, -1, "the switch recipe does not remove monet")
        self.assertLess(remove, recipe.find("claude plugin marketplace add"))
        body = prose(raw)
        self.assertIn("switch back", body)
        self.assertIn(f"`{github_source()}`", body)

    def test_dev_loop_marketplace_subcommands_exist(self):
        raw = section(doc_text(), "dev loop")
        # `marketplace <name>` in code, plus `plugin marketplace <name>` in prose.
        named = set(re.findall(r"marketplace ([a-z]+)", code(raw)))
        named |= set(re.findall(r"plugin marketplace ([a-z]+)", flat(raw)))
        self.assertTrue(named, "the dev loop names no marketplace subcommand")
        result = subprocess.run(["claude", "plugin", "marketplace", "--help"],
                                capture_output=True, text=True, check=False,
                                timeout=60)
        listed = set(re.findall(r"^  ([a-z]+)(?:\|[a-z]+)? ", result.stdout, re.M))
        for name in sorted(named):
            with self.subTest(subcommand=name):
                self.assertIn(name, listed)

    def test_covers_gate(self):
        body = flat(section(doc_text(), "gate"))
        steps = check_steps()
        self.assertGreaterEqual(len(steps), 6, steps)
        positions = [body.find(f"`{step}`") for step in steps]
        for step, pos in zip(steps, positions):
            with self.subTest(step=step):
                self.assertNotEqual(pos, -1, f"`{step}` is not named")
        self.assertEqual(positions, sorted(positions), "steps are out of Makefile order")
        for needle in ("needs only `make`, `git`, `python3`, and `claude`.",
                       "skills/gang/cli", "$MONET_PRIVACY_PATTERNS",
                       "~/.config/monet/privacy-patterns", "one per line",
                       "never sit in the repo", "built-in patterns only"):
            with self.subTest(needle=needle):
                self.assertIn(needle, body)
        # make test: make check, then go vet, then go test, then the simulator.
        start = body.find("`make test` runs `make check`")
        self.assertNotEqual(start, -1, "the doc does not say make test runs make check")
        order = [body.find(needle, start) for needle in
                 ("`make check`", "`go vet ./...`", "`go test ./...`", "simulator")]
        self.assertNotIn(-1, order, order)
        self.assertEqual(order, sorted(order), "make test steps are out of order")
        gang = GANG_SKILL.read_text(encoding="utf-8")
        go = re.search(r"`go` ([0-9.]+) or later", gang).group(1)
        node = re.search(r"`node` ([0-9.]+) or later", gang).group(1)
        self.assertIn(f"Go {go}", body)
        self.assertIn(f"Node {node}", body)

    def test_covers_ci(self):
        raw = section(doc_text(), "CI")
        body = flat(raw)
        for needle in ("`.github/workflows/check.yml`", "pull request",
                       "push to `main`", "pinned"):
            with self.subTest(needle=needle):
                self.assertIn(needle, body)
        # The paragraph that names the workflow says that it runs make test.
        first = flat(raw.strip().split("\n\n")[0])
        self.assertIn("`.github/workflows/check.yml`", first)
        self.assertIn("runs `make test`", first)
        pinned = re.search(r"claude-code@([0-9][0-9.]*)",
                           WORKFLOW.read_text(encoding="utf-8")).group(1)
        self.assertNotIn(pinned, doc_text())

    def test_covers_release(self):
        raw = section(doc_text(), "release")
        body = flat(raw)
        for needle in ("`version`", "`.claude-plugin/plugin.json`", "`CHANGELOG.md`",
                       "claude plugin tag --dry-run", "claude plugin tag --push",
                       "`monet--v<version>`", "only after",
                       "Do not add a version anywhere else"):
            with self.subTest(needle=needle):
                self.assertIn(needle, body)
        self.assertLess(body.find("--dry-run"), body.find("--push"))
        # The version bump names plugin.json and no other manifest.
        bump = [line for line in raw.split("\n") if "Bump `version`" in line]
        self.assertEqual(len(bump), 1, bump)
        self.assertEqual(re.findall(r"[\w./-]+\.json", bump[0]),
                         [".claude-plugin/plugin.json"])

    def test_has_no_dotfiles_paths(self):
        self.assertEqual(skillkit.dotfiles_paths(doc_text()), [])

    def test_prose_has_no_em_dash_or_semicolon(self):
        result = subprocess.run([sys.executable, str(STE_CHECK), str(DOC)],
                                capture_output=True, text=True, check=False,
                                timeout=60)
        self.assertIn("em-dashes 0, semicolons 0", result.stdout)
        self.assertNotIn(chr(0x2014), doc_text())


if __name__ == "__main__":
    unittest.main()
