"""Contract for the roadmap and handoff design spec in docs/design/ (DOC-8)."""

import re
import subprocess
import sys
import unittest
from pathlib import Path

import skillkit

ROOT = Path(__file__).resolve().parents[2]
DOC = ROOT / "docs" / "design" / "roadmap-handoff.md"
STE_CHECK = ROOT / "skills" / "voice" / "scripts" / "ste_check.py"
HANDOFF_SKILL = "skills/handoff/SKILL.md"
BANNER_START = "> **Superseded in part"
# Paths that only make sense in a dotfiles install or in another repo.
PRIVATE_STRINGS = ("~/.claude", ".claude/skills/", "/Users/", "STATE.md")
# A ticket key or a ticket-style id (two or more capitals, a dash, then digits
# or nn), and a PR number with four or more digits.
TICKET = re.compile(r"\b[A-Z]{2,}-(?:[0-9]+|nn)[a-z]?\b|#[0-9]{4,}")
FENCE = re.compile(r"^```.*?^```", re.M | re.S)


def doc_text():
    return DOC.read_text(encoding="utf-8")


def banner(text):
    """The block quote that opens with the superseded notice, without its > marks."""
    lines = text.split("\n")
    start = next((i for i, line in enumerate(lines) if line.startswith(BANNER_START)), None)
    if start is None:
        return ""
    quote = []
    for line in lines[start:]:
        if not line.startswith(">"):
            break
        quote.append(line[1:].strip())
    return " ".join(quote)


def section(text, heading):
    """The body of the `## heading` section, up to the next `## ` heading."""
    match = re.search(rf"^## {re.escape(heading)}\n(.*?)(?=^## |\Z)", text, re.M | re.S)
    return match.group(1) if match else ""


def listed_paths(body):
    """(path, rest of line) for each line inside the code fences of a section."""
    entries = []
    for block in FENCE.findall(body):
        for line in block.split("\n")[1:-1]:
            if line.strip():
                path, _, rest = line.strip().partition(" ")
                entries.append((path, rest))
    return entries


class RoadmapDesignDoc(unittest.TestCase):
    def test_keeps_superseded_banner(self):
        text = doc_text()
        quote = banner(text)
        self.assertTrue(quote, "the superseded-in-part banner is missing")
        self.assertLess(text.index(BANNER_START), text.index("\n## "),
                        "the banner must come before the first section")
        self.assertIn("`roadmap`", quote)
        self.assertIn("`resume`", quote)
        # The banner sends the reader to the repo path of the current skill.
        self.assertIn(f"`{HANDOFF_SKILL}`", quote)
        self.assertTrue((ROOT / HANDOFF_SKILL).is_file())

    def test_has_no_private_references(self):
        text = doc_text()
        for needle in PRIVATE_STRINGS:
            with self.subTest(needle=needle):
                self.assertNotIn(needle, text)
        self.assertEqual(skillkit.dotfiles_paths(text), [])
        self.assertEqual(TICKET.findall(text), [])

    def test_files_section_lists_repo_paths(self):
        entries = listed_paths(section(doc_text(), "Files"))
        self.assertTrue(entries, "the Files section lists no paths")
        for path, rest in entries:
            with self.subTest(path=path):
                self.assertRegex(path, r"^(skills/[a-z-]+/|agents/monet\.md$)")
                # A planned skill file that shipped must still exist in the repo.
                if path.startswith("skills/") and "not shipped" not in rest:
                    self.assertTrue((ROOT / path).is_file(), f"{path} is not in the repo")
        self.assertIn("skills/handoff/SKILL.md", [path for path, _ in entries])

    def test_prose_has_no_banned_marks(self):
        result = subprocess.run([sys.executable, str(STE_CHECK), str(DOC)],
                                capture_output=True, text=True, check=False)
        self.assertIn("em-dashes 0, semicolons 0", result.stdout)


if __name__ == "__main__":
    unittest.main()
