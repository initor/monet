"""Contract tests for the code-mock port (LAB-5)."""

import json
import re
import unittest

import skillkit

SKILL = skillkit.REPO / "skills" / "code-mock" / "SKILL.md"
TRIGGERS = "Use when the user asks to practice coding, run a coding mock, or drill coding problems."
# The tracker's 13 keys minus `id`, which the tracker's write step sets from the filename.
KEYS = ["version", "domain", "mode", "topic", "started_at", "ended_at", "duration_seconds",
        "timing_breakdown", "scores", "overall_score", "takeaways", "evaluator_summary"]
DIMENSIONS = ["problem_understanding", "solution_design", "code_quality", "correctness",
              "complexity_analysis"]
# Quoted interviewer lines keep `me`, since `me` there is the interviewer and not the user.
INTERVIEWER_LINES = [
    "- **Edge case drill:** Present edge cases one at a time and ask the user to trace through their code: "
    '"What happens with an empty array? Walk me through line by line." Push for 4-5 edge cases.',
    '- "Walk me through your approach" not "Would you mind explaining your thought process?"',
]
# The acceptance grep, with each literal split by a char class so this file does not match it.
BANNED = re.compile(r"~/[.]config/interview-prep|~/[.]claude|W[a]yne|w[a]wen|\u2014")


def bare(text, name):
    return [s for s in text.splitlines() if re.search(rf"(?<![\w:-]){name}\b(?!-)", s)]


class CodeMockPort(unittest.TestCase):
    def setUp(self):
        self.text = SKILL.read_text(encoding="utf-8")
        self.skill = skillkit.read_skill(SKILL)

    def test_invokes_code_playground_as_monet_skill(self):
        section = self.text.split("### Playground generation", 1)[1].split("### During coding", 1)[0]
        self.assertIn("invoke `monet:code-playground` with the Skill tool, and pass it this problem "
                      "context as its arguments:", section)
        for item in ("- **Title:**", "- **Description:**", "- **Method signature:**", "- **Test cases:**"):
            self.assertIn(item, section)
        self.assertIn("never read its skill file by path", section)
        self.assertEqual(bare(self.skill.body, "code-playground"), [])
        self.assertNotIn("SKILL.md", self.text)

    def test_records_through_tracker_skill(self):
        self.assertIn("Load `monet:interview-tracker` with the Skill tool and follow its write mode", self.text)
        self.assertIn("A background sub-agent loads it with the Skill tool too.", self.text)
        self.assertIn("Run the `monet:interview-tracker` write mode, loaded with the Skill tool", self.text)
        self.assertEqual(bare(self.skill.body, "interview-tracker"), [])
        # The tracker resolves the data dir for code-mock's sessions.
        self.assertNotIn("${user_config", self.text)
        self.assertIsNone(re.search(r"interview-prep/", self.text))

    def test_description_starts_use_when(self):
        desc = self.skill.frontmatter["description"]
        self.assertTrue(desc.startswith(TRIGGERS), desc)
        self.assertLessEqual(len(desc), 500)
        for phrase in ("Java-focused coding mock interviews", "clarification, optimization, edge cases",
                       "golden solutions", "auto-records sessions"):
            self.assertIn(phrase, desc)

    def test_session_block_matches_tracker_keys(self):
        blocks = re.findall(r"```json\n(.*?)\n\s*```", self.text, re.S)
        self.assertEqual(len(blocks), 1)
        session = json.loads(blocks[0])
        self.assertEqual(list(session), KEYS)
        self.assertEqual(session["domain"], "code")
        self.assertEqual(list(session["scores"]), DIMENSIONS)
        self.assertEqual(list(session["takeaways"]), ["went_well", "to_improve", "tags"])

    def test_interviewer_lines_kept(self):
        lines = self.text.splitlines()
        for line in INTERVIEWER_LINES:
            self.assertIn(line, lines)

    def test_no_other_skill_md_by_path(self):
        self.assertEqual(skillkit.skill_md_refs(self.text, "code-mock"), [])

    def test_no_dotfiles_paths(self):
        # The acceptance grep runs over the whole skill dir, tests included.
        for path in [*SKILL.parent.rglob("*.md"), *SKILL.parent.rglob("*.py")]:
            lines = path.read_text(encoding="utf-8").splitlines()
            self.assertEqual([n for n, s in enumerate(lines, 1) if BANNED.search(s)], [], path)
        self.assertEqual(skillkit.dotfiles_paths(self.text), [])
        # A quoted `~` does not expand in Bash.
        self.assertNotIn('"<data dir>', self.text)
        self.assertFalse((skillkit.REPO / "import" / "skills" / "code-mock").exists())


if __name__ == "__main__":
    unittest.main()
