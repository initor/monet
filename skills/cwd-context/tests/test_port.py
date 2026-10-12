"""Port contract for the cwd-context skill (CORE-9)."""

import re
import unittest
from pathlib import Path

import skillkit

SKILL_DIR = Path(__file__).resolve().parent.parent
SKILL = skillkit.read_skill(SKILL_DIR / "SKILL.md")
TEXT = (SKILL_DIR / "SKILL.md").read_text(encoding="utf-8")
# Bracketed so this file does not match the pattern it holds.
PERSONAL = re.compile(r"~/\.claude|W[a]yne|w[a]wen")
# A double-quoted span (one line break at most) quotes what a user types, so
# only text outside quotes must avoid I, me, and my, in any case.
QUOTED = re.compile(r'"[^"\n]*(?:\n[^"\n]*)?"')
FIRST_PERSON = re.compile(r"\bI\b|\b(?i:me|my)\b")


def skill_files():
    """Files under this skill dir that git tracks or would add (honors .gitignore)."""
    prefix = SKILL_DIR.relative_to(skillkit.REPO).as_posix() + "/"
    return [skillkit.REPO / p for p in skillkit.files() if p.startswith(prefix)]


def line_hits(files, pattern):
    """Return "path:line" for each line that matches pattern."""
    return [f"{p.relative_to(SKILL_DIR)}:{n}" for p in files
            for n, line in enumerate(p.read_text(encoding="utf-8").splitlines(), 1)
            if pattern.search(line)]


class CwdContextPort(unittest.TestCase):
    def test_name_and_invocable(self):
        self.assertEqual(SKILL.frontmatter["name"], "cwd-context")
        self.assertNotIn("user-invocable", SKILL.frontmatter)

    def test_description_use_when(self):
        desc = SKILL.frontmatter["description"].strip()
        self.assertTrue(desc.startswith("Use when"), desc)
        self.assertLessEqual(len(desc), 500)
        self.assertNotRegex(desc, FIRST_PERSON)
        self.assertNotRegex(TEXT, PERSONAL)

    def test_no_first_person(self):
        self.assertNotRegex(QUOTED.sub('""', TEXT), FIRST_PERSON)

    def test_skill_dir_generic(self):
        files = skill_files()
        self.assertIn(SKILL_DIR / "SKILL.md", files)
        self.assertEqual(line_hits(files, PERSONAL), [])

    def test_names_no_other_skill_md(self):
        self.assertEqual(skillkit.skill_md_refs(TEXT, "cwd-context"), [])


if __name__ == "__main__":
    unittest.main()
