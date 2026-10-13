"""Contract tests for the bq-playground port (LAB-5)."""

import re
import unittest

import skillkit

SKILL = skillkit.REPO / "skills" / "bq-playground" / "SKILL.md"
PLACEHOLDER = "${user_config.data_dir}"
DATA_DIR = '<data dir>'
PLAYGROUND = DATA_DIR + "/interview-prep/playgrounds/bq-storymap-{YYYY-MM-DD}.html"
TRIGGERS = ("Use when the user asks to visualize stories, see their story bank, generate a concept map, "
            "or view story coverage.")
# The acceptance grep, with each literal split by a char class so this file does not match it.
BANNED = re.compile(r"~/[.]config/interview-prep|~/[.]claude|W[a]yne|w[a]wen|\u2014")


class BqPlaygroundPort(unittest.TestCase):
    def setUp(self):
        self.text = SKILL.read_text(encoding="utf-8")
        self.lines = self.text.splitlines()
        self.skill = skillkit.read_skill(SKILL)

    def test_playgrounds_under_data_dir(self):
        self.assertIn(f"mkdir -p {DATA_DIR}/interview-prep/playgrounds/", self.text)
        # The write path sits alone in a fenced block, so it is a whole line.
        self.assertIn(PLAYGROUND, [s.strip() for s in self.lines])
        self.assertIn(f"open {PLAYGROUND}", self.text)
        # The filename patterns are unchanged.
        self.assertIn("`bq-storymap-{YYYY-MM-DD}.html`", self.text)
        self.assertIn("`bq-storymap-{YYYY-MM-DD}-002.html`", self.text)
        # A quoted `~` does not expand in Bash.
        self.assertNotIn('"' + DATA_DIR, self.text)
        self.assertIn("Use `monet:bq-mock` in story-bank mode", self.text)

    def test_description_starts_use_when(self):
        desc = self.skill.frontmatter["description"]
        self.assertTrue(desc.startswith(TRIGGERS), desc)
        self.assertLessEqual(len(desc), 500)
        for phrase in ("concept map playground", "behavioral story bank", "competency themes",
                       "Requires 2+ stories"):
            self.assertIn(phrase, desc)

    def test_data_dir_once_with_fallback(self):
        self.assertEqual(self.text.count(PLACEHOLDER), 1)
        self.assertIn(f"The data dir is `{PLACEHOLDER}`. " + skillkit.USER_CONFIG_FALLBACKS["data_dir"],
                      self.text)
        self.assertEqual(skillkit.missing_fallbacks(self.text), [])

    def test_no_other_skill_md_by_path(self):
        self.assertEqual(skillkit.skill_md_refs(self.text, "bq-playground"), [])

    def test_no_dotfiles_paths(self):
        # The acceptance grep runs over the whole skill dir, tests included.
        for path in [*SKILL.parent.rglob("*.md"), *SKILL.parent.rglob("*.py")]:
            lines = path.read_text(encoding="utf-8").splitlines()
            self.assertEqual([n for n, s in enumerate(lines, 1) if BANNED.search(s)], [], path)
        self.assertEqual(skillkit.dotfiles_paths(self.text), [])
        self.assertFalse((skillkit.REPO / "import" / "skills" / "bq-playground").exists())


if __name__ == "__main__":
    unittest.main()
