"""Contract tests for the bq-mock port (LAB-5)."""

import json
import re
import unittest

import skillkit

SKILL = skillkit.REPO / "skills" / "bq-mock" / "SKILL.md"
PLACEHOLDER = "${user_config.data_dir}"
DATA_DIR = '<data dir>'
TRIGGERS = ("Use when the user asks to practice behavioral questions, build stories, run a behavioral "
            "mock, or add stories to their story bank.")
# The tracker's 13 keys minus `id`, which the tracker's write step sets from the filename.
KEYS = ["version", "domain", "mode", "topic", "started_at", "ended_at", "duration_seconds",
        "timing_breakdown", "scores", "overall_score", "takeaways", "evaluator_summary"]
DIMENSIONS = ["story_structure", "scope_and_impact", "decision_rationale", "leadership_signals",
              "communication_clarity"]
# Quoted interviewer lines keep `me`, since `me` there is the interviewer and not the user.
INTERVIEWER_LINES = [
    '   | **Action** | "Walk me through exactly what you did. Step by step. What decisions did you make and '
    'why? What alternatives did you consider and reject?" |',
    '   - "Walk me through the decision-making process. What were the alternatives?"',
    '- "Walk me through..." not "Would you mind explaining..."',
]
# The acceptance grep, with each literal split by a char class so this file does not match it.
BANNED = re.compile(r"~/[.]config/interview-prep|~/[.]claude|W[a]yne|w[a]wen|\u2014")


class BqMockPort(unittest.TestCase):
    def setUp(self):
        self.text = SKILL.read_text(encoding="utf-8")
        self.skill = skillkit.read_skill(SKILL)

    def test_sessions_under_data_dir(self):
        # bq-mock keeps its inline write steps (snapshot Step 4, item 7), under its own data dir.
        self.assertIn("   - Ensure storage directory: `mkdir -p <data dir>/interview-prep/`\n", self.text)
        self.assertIn("   - Generate filename: `{YYYY-MM-DD}_bq_{mode}_{NNN}.json`\n", self.text)
        self.assertIn("   - Validate all required fields\n", self.text)
        self.assertIn("`<data dir>/interview-prep/{filename}` with 2-space indented formatting", self.text)
        # A background sub-agent does not see the parent's expanded body, so it gets the data dir.
        self.assertIn("Give the sub-agent the data dir resolved above.", self.text)
        self.assertEqual(self.text.count(DATA_DIR + "/"), self.text.count(DATA_DIR + "/interview-prep/"))
        # A quoted `~` does not expand in Bash.
        self.assertNotIn('"' + DATA_DIR, self.text)
        # The tracker is named by its plugin name.
        self.assertIn("`monet:interview-tracker` write protocol", self.text)
        bare = [s for s in self.skill.body.splitlines() if re.search(r"(?<![\w:-])interview-tracker\b", s)]
        self.assertEqual(bare, [])

    def test_description_starts_use_when(self):
        desc = self.skill.frontmatter["description"]
        self.assertTrue(desc.startswith(TRIGGERS), desc)
        self.assertLessEqual(len(desc), 500)
        for phrase in ("STAR+ stories", "behavioral mocks", "Staff+ evaluation"):
            self.assertIn(phrase, desc)

    def test_session_block_matches_tracker_keys(self):
        blocks = re.findall(r"```json\n(.*?)\n\s*```", self.text, re.S)
        self.assertEqual(len(blocks), 1)
        session = json.loads(blocks[0])
        self.assertEqual(list(session), KEYS)
        self.assertEqual(session["domain"], "bq")
        self.assertEqual(list(session["scores"]), DIMENSIONS)
        self.assertEqual(list(session["takeaways"]), ["went_well", "to_improve", "tags"])

    def test_data_dir_once_with_fallback(self):
        self.assertEqual(self.text.count(PLACEHOLDER), 1)
        self.assertIn(f"The data dir is `{PLACEHOLDER}`. " + skillkit.USER_CONFIG_FALLBACKS["data_dir"],
                      self.text)
        self.assertEqual(skillkit.missing_fallbacks(self.text), [])

    def test_interviewer_lines_kept(self):
        lines = self.text.splitlines()
        for line in INTERVIEWER_LINES:
            self.assertIn(line, lines)

    def test_no_other_skill_md_by_path(self):
        self.assertEqual(skillkit.skill_md_refs(self.text, "bq-mock"), [])

    def test_no_dotfiles_paths(self):
        # The acceptance grep runs over the whole skill dir, tests included.
        for path in [*SKILL.parent.rglob("*.md"), *SKILL.parent.rglob("*.py")]:
            lines = path.read_text(encoding="utf-8").splitlines()
            self.assertEqual([n for n, s in enumerate(lines, 1) if BANNED.search(s)], [], path)
        self.assertEqual(skillkit.dotfiles_paths(self.text), [])
        self.assertFalse((skillkit.REPO / "import" / "skills" / "bq-mock").exists())


if __name__ == "__main__":
    unittest.main()
