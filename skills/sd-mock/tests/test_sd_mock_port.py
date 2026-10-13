"""Contract tests for the sd-mock port (LAB-5)."""

import json
import re
import unittest

import skillkit

SKILL = skillkit.REPO / "skills" / "sd-mock" / "SKILL.md"
TRIGGERS = ("Use when the user asks to practice system design, run a system design mock, or drill a "
            "system design topic.")
# The tracker's 13 keys minus `id`, which the tracker's write step sets from the filename.
KEYS = ["version", "domain", "mode", "topic", "started_at", "ended_at", "duration_seconds",
        "timing_breakdown", "scores", "overall_score", "takeaways", "evaluator_summary"]
DIMENSIONS = ["requirements_clarity", "architecture_design", "api_design", "scalability", "reliability",
              "tradeoff_articulation"]
# Quoted interviewer lines keep `me`, since `me` there is the interviewer and not the user.
INTERVIEWER_LINES = [
    '| 6 | Detailed Workflows | Key user flows end-to-end, sequence diagrams | "Walk me through the write path", '
    '"What happens if step 3 fails?" |',
    "How would you design the API for this system? Walk me through your endpoints.",
    '- "Walk me through..." not "Would you mind explaining..."',
]
# The acceptance grep, with each literal split by a char class so this file does not match it.
BANNED = re.compile(r"~/[.]config/interview-prep|~/[.]claude|W[a]yne|w[a]wen|\u2014")


def bare(text, name):
    return [s for s in text.splitlines() if re.search(rf"(?<![\w:-]){name}\b(?!-)", s)]


class SdMockPort(unittest.TestCase):
    def setUp(self):
        self.text = SKILL.read_text(encoding="utf-8")
        self.skill = skillkit.read_skill(SKILL)

    def test_names_skills_as_monet(self):
        for name in ("interview-tracker", "sd-playground", "interview-progress"):
            self.assertIn(f"`monet:{name}`", self.skill.body)
            self.assertEqual(bare(self.skill.body, name), [], name)
        self.assertIn("Invoke `monet:sd-playground` with the Skill tool", self.text)
        self.assertIn("Invoke `monet:interview-progress` with the Skill tool", self.text)
        # The tracker's write mode runs through the Skill tool, from a background sub-agent too.
        self.assertIn("Load `monet:interview-tracker` with the Skill tool and follow its write mode", self.text)
        self.assertIn("A background sub-agent loads it with the Skill tool too.", self.text)
        self.assertIn("Run the `monet:interview-tracker` write mode, loaded with the Skill tool", self.text)
        # The tracker resolves the data dir for sd-mock's sessions.
        self.assertNotIn("${user_config", self.text)
        self.assertIsNone(re.search(r"interview-prep/", self.text))

    def test_description_starts_use_when(self):
        desc = self.skill.frontmatter["description"]
        self.assertTrue(desc.startswith(TRIGGERS), desc)
        self.assertLessEqual(len(desc), 500)
        for phrase in ("mock interviews", "drills specific phases", "Staff+", "golden answer outlines",
                       "auto-records sessions"):
            self.assertIn(phrase, desc)

    def test_session_block_matches_tracker_keys(self):
        blocks = re.findall(r"```json\n(.*?)\n\s*```", self.text, re.S)
        self.assertEqual(len(blocks), 1)
        session = json.loads(blocks[0])
        self.assertEqual(list(session), KEYS)
        self.assertEqual(session["domain"], "sd")
        self.assertEqual(list(session["scores"]), DIMENSIONS)
        self.assertEqual(list(session["takeaways"]), ["went_well", "to_improve", "tags"])

    def test_interviewer_lines_kept(self):
        lines = self.text.splitlines()
        for line in INTERVIEWER_LINES:
            self.assertIn(line, lines)

    def test_no_other_skill_md_by_path(self):
        self.assertEqual(skillkit.skill_md_refs(self.text, "sd-mock"), [])

    def test_no_dotfiles_paths(self):
        # The acceptance grep runs over the whole skill dir, tests included.
        for path in [*SKILL.parent.rglob("*.md"), *SKILL.parent.rglob("*.py")]:
            lines = path.read_text(encoding="utf-8").splitlines()
            self.assertEqual([n for n, s in enumerate(lines, 1) if BANNED.search(s)], [], path)
        self.assertEqual(skillkit.dotfiles_paths(self.text), [])
        # A quoted `~` does not expand in Bash.
        self.assertNotIn('"<data dir>', self.text)
        self.assertFalse((skillkit.REPO / "import" / "skills" / "sd-mock").exists())


if __name__ == "__main__":
    unittest.main()
