"""Contract tests for the sd-playground port (LAB-4)."""

import re
import unittest

import skillkit

SKILL = skillkit.REPO / "skills" / "sd-playground" / "SKILL.md"
DATA_DIR = '<data dir>'
PLAYGROUND = DATA_DIR + '/interview-prep/playgrounds/sd-diagram-{topic-slug}-{date}.html'
EM_DASH = chr(0x2014)
DESCRIPTION = (
    "Use when the user asks for an architecture diagram, system design diagram, or design playground. "
    "Generates an interactive architecture diagram playground from a system design, as a "
    "self-contained HTML file with draggable components, connections, and annotations in a "
    "whiteboard aesthetic. For a designed overview page or visual the owner picks by eye, use "
    "visual-design instead.")


class SdPlaygroundPort(unittest.TestCase):
    def setUp(self):
        self.text = SKILL.read_text(encoding="utf-8")
        self.lines = self.text.splitlines()

    def test_playgrounds_under_data_dir(self):
        self.assertIn(f"mkdir -p {DATA_DIR}/interview-prep/playgrounds/", self.text)
        # The write path sits alone in a fenced block, so it is a whole line.
        self.assertIn(PLAYGROUND, [s.strip() for s in self.lines])
        self.assertIn(f"open {PLAYGROUND}", self.text)
        # A quoted `~` does not expand in Bash.
        self.assertNotIn('"' + DATA_DIR, self.text)

    def test_no_dotfiles_paths(self):
        # The acceptance grep, with each literal split by a char class so this file
        # does not match it.
        bad = re.compile(r"~/[.]config/interview-prep|~/[.]claude|W[a]yne|w[a]wen")
        self.assertEqual([n for n, line in enumerate(self.lines, 1) if bad.search(line)], [])
        self.assertEqual(skillkit.dotfiles_paths(self.text), [])
        self.assertNotIn(EM_DASH, self.text)
        self.assertFalse((skillkit.REPO / "import" / "skills" / "sd-playground").exists())

    def test_data_dir_once_with_fallback(self):
        self.assertEqual(self.text.count("${user_config.data_dir}"), 1)
        # The line states the data dir, and the next sentence is exactly the R1 fallback.
        self.assertIn("The data dir is `${user_config.data_dir}`. "
                      + skillkit.USER_CONFIG_FALLBACKS["data_dir"], self.text)
        self.assertEqual(skillkit.missing_fallbacks(self.text), [])

    def test_no_other_skill_md_by_path(self):
        self.assertEqual(skillkit.skill_md_refs(self.text, "sd-playground"), [])

    def test_frontmatter_unchanged(self):
        meta = skillkit.read_skill(SKILL).frontmatter
        self.assertEqual(meta["name"], "sd-playground")
        self.assertEqual(meta["description"], DESCRIPTION)

    def test_sd_mock_named_as_plugin_skill(self):
        mentions = [s for s in self.lines if re.search(r"(?<![\w:-])sd-mock\b", s)]
        self.assertEqual(mentions, [])
        self.assertGreaterEqual(self.text.count("`monet:sd-mock`"), 3)


if __name__ == "__main__":
    unittest.main()
