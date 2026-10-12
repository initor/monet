"""Contract tests for the sd-explainer port (LAB-4)."""

import re
import unittest

import skillkit

SKILL = skillkit.REPO / "skills" / "sd-explainer" / "SKILL.md"
DATA_DIR = '<data dir>'
PLAYGROUND = DATA_DIR + '/interview-prep/playgrounds/sd-concept-{concept-slug}-{date}.html'
EM_DASH = chr(0x2014)
DESCRIPTION = (
    "Use when the user asks to explain a concept, learn about a distributed systems topic, or wants "
    "a concept playground. Generates interactive playgrounds for distributed systems concepts, "
    "self-contained HTML simulations where users manipulate inputs and see results. For a quiz-gated "
    "lesson in chat, use teach instead. For a newcomer explainer with one Hairline figure, use paint "
    "--explainer instead.")


class SdExplainerPort(unittest.TestCase):
    def setUp(self):
        self.text = SKILL.read_text(encoding="utf-8")
        self.lines = self.text.splitlines()

    def test_playgrounds_under_data_dir(self):
        self.assertIn(f"mkdir -p {DATA_DIR}/interview-prep/playgrounds/", self.text)
        self.assertIn(f"Path: `{PLAYGROUND}`", self.text)
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
        self.assertFalse((skillkit.REPO / "import" / "skills" / "sd-explainer").exists())

    def test_data_dir_once_with_fallback(self):
        self.assertEqual(self.text.count("${user_config.data_dir}"), 1)
        # The line states the data dir, and the next sentence is exactly the R1 fallback.
        self.assertIn("The data dir is `${user_config.data_dir}`. "
                      + skillkit.USER_CONFIG_FALLBACKS["data_dir"], self.text)
        self.assertEqual(skillkit.missing_fallbacks(self.text), [])

    def test_no_other_skill_md_by_path(self):
        self.assertEqual(skillkit.skill_md_refs(self.text, "sd-explainer"), [])

    def test_frontmatter_unchanged(self):
        meta = skillkit.read_skill(SKILL).frontmatter
        self.assertEqual(meta["name"], "sd-explainer")
        self.assertEqual(meta["description"], DESCRIPTION)

    def test_recommendation_reads_sessions(self):
        step = next(s for s in self.lines if "asks for a recommendation" in s)
        self.assertIn("`monet:interview-progress`", step)
        self.assertIn("`<data dir>/interview-prep/*.json`", step)


if __name__ == "__main__":
    unittest.main()
