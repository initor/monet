"""Contract for the gang design spec in docs/design/ (DOC-8)."""

import re
import subprocess
import sys
import unittest
from pathlib import Path

import skillkit

ROOT = Path(__file__).resolve().parents[2]
DOC = ROOT / "docs" / "design" / "gang.md"
STE_CHECK = ROOT / "skills" / "voice" / "scripts" / "ste_check.py"
SCHEMA = "skills/gang/reference/schema.md"
SKILL = "skills/gang/SKILL.md"

# Every numbered heading of the source spec, in order.
HEADINGS = (
    "## 1. Motivation",
    "## 2. Decisions",
    "## 3. Scope",
    "## 4. Architecture",
    "### 4.1 Components",
    "### 4.2 Flow",
    "### 4.3 Run state, in the target repo",
    "## 5. Plan schema (`plan.json`)",
    "### 5.1 Run-wide fields",
    "### 5.2 Task fields",
    "### 5.3 Lint (`gang lint`)",
    "### 5.4 Schedule (`gang schedule`)",
    "### 5.5 Planner",
    "## 6. Roles and models",
    "## 7. Engine",
    "### 7.1 Generation",
    "### 7.2 One task's lifecycle",
    "### 7.3 Return schemas",
    "### 7.4 Scheduling",
    "### 7.5 Stop conditions",
    "## 8. Coordinator",
    "### 8.1 Events",
    "### 8.2 Files as the channel to a running engine",
    "### 8.3 Checkpoints",
    "### 8.4 Done",
    "## 9. CLI reference (`gang`)",
    "## 10. Safety",
    "### 10.1 Preflight (fails closed)",
    "### 10.2 Launch",
    "### 10.3 Agent rules (in every brief)",
    "### 10.4 Retry by failure mode",
    "## 11. Testing and acceptance",
    "## 12. Files",
    "## 13. Build order",
    "## 14. Risks",
)
# A level 2 or 3 heading that starts with a section number, such as 4. or 4.1.
NUMBERED = re.compile(r"^#{2,3} [0-9]+\.[0-9]* .*$", re.M)
# Files that live only in the planning folder or the dotfiles install, and
# paths that only make sense there. skillkit.dotfiles_paths covers the rest of
# the dotfiles path forms.
PRIVATE_STRINGS = ("lessons.md", "decisions.md", "findings.md", "research/",
                   "~/.claude", "/Users/", "plans/00-overview.md")
# A ticket key or a ticket-style id (two or more capitals, a dash, then digits
# or nn), and a PR number with four or more digits.
TICKET = re.compile(r"\b[A-Z]{2,}-(?:[0-9]+|nn)[a-z]?\b|#[0-9]{4,}")
# A .claude dir path that is not a task worktree. The plugin keeps no file
# under .claude/, so any other such path is a leftover dotfiles path.
CLAUDE_DIR = re.compile(r"\.claude\b(?!/worktrees/)")
# The header names no owner and no approver, in any case or phrasing.
SIGN_OFF = re.compile(r"\b(?:owner|approver|approved by)\b", re.I)
# Where a path in the components table or the Files list may resolve: the
# skill dir, its Go module (for cmd/ and internal/), and the repo root.
PATH_BASES = (ROOT / "skills" / "gang", ROOT / "skills" / "gang" / "cli", ROOT)


def doc_text():
    return DOC.read_text(encoding="utf-8")


def section(text, heading):
    """The body under a heading line, up to the next heading of any level."""
    match = re.search(rf"^{re.escape(heading)}\n(.*?)(?=^#{{1,6}} |\Z)", text, re.M | re.S)
    return match.group(1) if match else ""


def header(text):
    """The lines between the title and the first section heading."""
    return text.split("\n## ", 1)[0]


def code_spans(line):
    """The backticked spans in a line."""
    return re.findall(r"`([^`]+)`", line)


def resolves(path):
    """True when the path names a file or dir under one of PATH_BASES."""
    path = path.removesuffix("...")
    return any((base / path).exists() for base in PATH_BASES)


def table_row(body, first_cell):
    """The table row in `body` whose first cell is `first_cell`."""
    for line in body.split("\n"):
        if line.startswith(f"| {first_cell} |"):
            return line
    return ""


class GangDesignDoc(unittest.TestCase):
    def test_keeps_section_headings(self):
        found = tuple(NUMBERED.findall(doc_text()))
        self.assertEqual(found, HEADINGS)

    def test_points_at_runtime_schema(self):
        body = section(doc_text(), "### 5.2 Task fields")
        self.assertTrue(body, "section 5.2 is missing")
        claim = f"`{SCHEMA}` is the copy of the task schema that gang reads at run time."
        sentences = [s for s in re.split(r"(?<=\.)\s+", body.strip()) if SCHEMA in s]
        self.assertIn(claim, sentences)
        self.assertTrue((ROOT / SCHEMA).is_file(), f"{SCHEMA} is not in the repo")

    def test_has_no_private_references(self):
        text = doc_text()
        for needle in PRIVATE_STRINGS:
            with self.subTest(needle=needle):
                self.assertNotIn(needle, text)
        self.assertEqual(skillkit.dotfiles_paths(text), [])
        self.assertEqual(TICKET.findall(text), [])
        self.assertEqual(CLAUDE_DIR.findall(text), [])
        self.assertEqual(SIGN_OFF.findall(header(text)), [])

    def test_uses_neutral_example_repo(self):
        body = section(doc_text(), "### 5.1 Run-wide fields")
        self.assertIn('"remote": "github.com/example/widget"', body)
        self.assertIn('"gh_account": "example"', body)

    def test_components_are_repo_paths(self):
        body = section(doc_text(), "### 4.1 Components")
        self.assertIn("Path (under `skills/gang/`)", table_row(body, "Component"))
        command = table_row(body, "Command")
        self.assertIn(SKILL, command)
        self.assertIn("folded", command)
        self.assertTrue((ROOT / SKILL).is_file())
        self.assertIn("`tools/test-gang.sh`", table_row(body, "Tests"))
        self.assertTrue((ROOT / "tools" / "test-gang.sh").is_file())

    def test_listed_paths_exist(self):
        text = doc_text()
        rows = section(text, "### 4.1 Components").split("\n")
        paths = [span for row in rows[2:] if row.startswith("| ")
                 for span in code_spans(row.split(" | ")[1])]
        create = section(text, "## 12. Files").split("**Modify:**")[0]
        paths += [span for line in create.split("\n") if line.startswith("- ")
                  for span in code_spans(line)]
        self.assertGreater(len(paths), 15)
        for path in paths:
            with self.subTest(path=path):
                self.assertTrue(resolves(path), f"{path} is not in the repo")

    def test_prose_has_no_banned_marks(self):
        result = subprocess.run([sys.executable, str(STE_CHECK), str(DOC)],
                                capture_output=True, text=True, check=False)
        self.assertIn("em-dashes 0, semicolons 0", result.stdout)


if __name__ == "__main__":
    unittest.main()
