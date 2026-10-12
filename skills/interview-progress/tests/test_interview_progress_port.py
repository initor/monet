"""Contract tests for the interview-progress port (LAB-5)."""

import re
import unittest
from pathlib import Path

import skillkit

SKILL_DIR = Path(__file__).resolve().parent.parent
SKILL_MD = SKILL_DIR / "SKILL.md"
PLACEHOLDER = "${user_config.data_dir}"
# Built so that this file does not hold the text it bans.
BANNED = re.compile(r"~/\.config/interview-prep|~/\.claude|[W]ayne|[w]awen|\u2014")


class InterviewProgressPort(unittest.TestCase):
    def setUp(self):
        self.text = SKILL_MD.read_text(encoding="utf-8")
        self.skill = skillkit.read_skill(SKILL_MD)

    def test_reads_sessions_and_writes_dashboard_under_data_dir(self):
        self.assertIn("ls -1 <data dir>/interview-prep/*.json", self.text)
        dash = "<data dir>/interview-prep/dashboard.html"
        for line in ("Session data directory `<data dir>/interview-prep/` must exist",
                     "sessions live in `<data dir>/interview-prep/`.",
                     f"Generate a self-contained HTML file at `{dash}`.",
                     f"After writing the HTML to `{dash}` using the Write tool",
                     f"open {dash}\n", f"Dashboard: {dash}\n",
                     f"The dashboard file (`{dash}`) is overwritten"):
            self.assertIn(line, self.text)
        self.assertEqual(self.text.count("<data dir>/"), self.text.count("<data dir>/interview-prep/"))
        self.assertNotIn('"<data dir>', self.text)

    def test_no_dotfiles_paths(self):
        for path in sorted(p for p in SKILL_DIR.rglob("*") if p.is_file() and "__pycache__" not in p.parts):
            text = path.read_text(encoding="utf-8")
            self.assertEqual(skillkit.dotfiles_paths(text), [], path)
            self.assertIsNone(BANNED.search(text), path)
        self.assertEqual(sorted(p.name for p in SKILL_DIR.rglob("*.json")), [])

    def test_description_starts_use_when(self):
        desc = self.skill.frontmatter["description"]
        self.assertTrue(desc.startswith("Use when"), desc)
        self.assertLessEqual(len(desc), 500)
        for phrase in ("view progress", "check stats", "see their dashboard",
                       "track interview prep improvement over time", "weak area detection"):
            self.assertIn(phrase, desc)
        self.assertNotIn("${user_config", desc)

    def test_data_dir_once_with_fallback(self):
        self.assertEqual(self.text.count(PLACEHOLDER), 1)
        line = next(s for s in self.text.splitlines() if PLACEHOLDER in s)
        self.assertIn("data dir is", line)
        after = self.text.split(PLACEHOLDER, 1)[1]
        rest = after.split(". ", 1)[1]
        self.assertTrue(rest.startswith(skillkit.USER_CONFIG_FALLBACKS["data_dir"]), rest[:200])
        self.assertEqual(skillkit.missing_fallbacks(self.text), [])

    def test_no_other_skill_md_by_path(self):
        self.assertEqual(skillkit.skill_md_refs(self.text, "interview-progress"), [])


if __name__ == "__main__":
    unittest.main()
