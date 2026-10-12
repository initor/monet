"""Port contract for the roadmap skill (CORE-9)."""

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
REFERENCES = {"reference/geometry.md", "reference/plan-parsing.md", "reference/template.html"}


def skill_files():
    """Files under this skill dir that git tracks or would add (honors .gitignore)."""
    prefix = SKILL_DIR.relative_to(skillkit.REPO).as_posix() + "/"
    return [skillkit.REPO / p for p in skillkit.files() if p.startswith(prefix)]


def line_hits(files, pattern):
    """Return "path:line" for each line that matches pattern."""
    return [f"{p.relative_to(SKILL_DIR)}:{n}" for p in files
            for n, line in enumerate(p.read_text(encoding="utf-8").splitlines(), 1)
            if pattern.search(line)]


class RoadmapPort(unittest.TestCase):
    def test_reference_paths_use_skill_dir(self):
        refs = {path for var, path, _ in skillkit.path_refs(TEXT) if var == "CLAUDE_SKILL_DIR"}
        self.assertTrue(REFERENCES <= refs, REFERENCES - refs)
        for path in refs:
            self.assertTrue((SKILL_DIR / path).is_file(), path)
        # Every reference/ path in the body goes through ${CLAUDE_SKILL_DIR}.
        bare = re.findall(r"(?<!\$\{CLAUDE_SKILL_DIR\}/)\breference/", SKILL.body)
        self.assertEqual(bare, [])

    def test_frontmatter_folds_command(self):
        meta = SKILL.frontmatter
        self.assertEqual(meta["argument-hint"], "[stream-dir] --plan <path>")
        self.assertEqual(meta["allowed-tools"], ["Read", "Bash", "Write", "AskUserQuestion"])
        self.assertIn("$ARGUMENTS", SKILL.body)

    def test_stops_without_plan(self):
        self.assertIn("If `--plan` is missing, stop", SKILL.body)

    def test_generic_text(self):
        self.assertNotRegex(TEXT, PERSONAL)

    def test_no_first_person(self):
        self.assertNotRegex(QUOTED.sub('""', TEXT), FIRST_PERSON)

    def test_skill_dir_generic(self):
        files = skill_files()
        self.assertIn(SKILL_DIR / "SKILL.md", files)
        self.assertEqual(line_hits(files, PERSONAL), [])

    def test_reference_docs_no_first_person(self):
        docs = [p for p in skill_files() if p.suffix == ".md"
                and p.relative_to(SKILL_DIR).parts[0] == "reference"]
        self.assertTrue(docs)
        for doc in docs:
            text = QUOTED.sub('""', doc.read_text(encoding="utf-8"))
            self.assertNotRegex(text, FIRST_PERSON, str(doc))

    def test_names_no_other_skill_md(self):
        self.assertEqual(skillkit.skill_md_refs(TEXT, "roadmap"), [])


if __name__ == "__main__":
    unittest.main()
