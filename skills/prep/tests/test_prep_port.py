"""Contract tests for the prep port (LAB-5)."""

import re
import unittest

import skillkit

SKILL = skillkit.REPO / "skills" / "prep" / "SKILL.md"
DESCRIPTION = (
    "Use when the user wants to practice interviews -- system design, behavioral, or coding mocks -- "
    "or view their prep progress dashboard. Routes by domain + mode arguments to the matching mock "
    "skill (sd-mock, bq-mock, code-mock) or interview-progress; falls back to interactive selection "
    "when arguments are missing or invalid.")
TOOLS = ["Read", "Bash", "Glob", "Grep", "Write", "AskUserQuestion", "Skill"]
ROUTES = ("sd-mock", "bq-mock", "code-mock", "interview-progress")
# The acceptance grep, with each literal split by a char class so this file does not match it.
BANNED = re.compile(r"~/[.]config/interview-prep|~/[.]claude|W[a]yne|w[a]wen|\u2014")


class PrepPort(unittest.TestCase):
    def setUp(self):
        self.text = SKILL.read_text(encoding="utf-8")
        self.skill = skillkit.read_skill(SKILL)

    def test_command_folded(self):
        self.assertFalse((skillkit.REPO / "import" / "commands" / "monet" / "prep.md").exists())
        self.assertFalse((skillkit.REPO / "import" / "skills" / "prep").exists())
        meta = self.skill.frontmatter
        self.assertEqual(meta["name"], "prep")
        self.assertEqual(meta["description"], DESCRIPTION)
        self.assertEqual(meta["argument-hint"], "[sd|bq|code|progress] [mode] [topic]")
        self.assertEqual(meta["allowed-tools"], TOOLS)
        body = self.skill.body
        self.assertIn("Arguments: $ARGUMENTS", body)
        self.assertIn("`<domain> [mode] [topic]`", body)
        self.assertIn("Pass them through unchanged.", body)
        # Empty or invalid arguments fall back to interactive selection.
        self.assertIn("Empty arguments are valid", body)
        self.assertIn("Domain **empty** -> Step 1", body)
        self.assertIn("then fall back to Step 1", body)
        self.assertIn("then fall back to Step 2", body)
        self.assertEqual(body.count("Use AskUserQuestion"), 2)
        self.assertNotIn("execution_context", body)

    def test_routes_by_skill_tool_only(self):
        body = self.skill.body
        for name in ROUTES:
            self.assertIn(f"`monet:{name}`", body)
            bare = [s for s in body.splitlines() if re.search(rf"(?<![\w:-]){name}\b(?!-)", s)]
            self.assertEqual(bare, [], name)
        self.assertIn("Invoke that skill with the Skill tool", body)
        # The by-path fallback is gone, with no replacement.
        self.assertNotIn("{skill-name}", self.text)
        self.assertNotIn("SKILL.md", self.text)
        self.assertNotIn("If invoking via Skill is unavailable", self.text)
        # prep only routes, so it resolves no data dir.
        self.assertNotIn("${user_config", self.text)

    def test_no_other_skill_md_by_path(self):
        self.assertEqual(skillkit.skill_md_refs(self.text, "prep"), [])

    def test_no_dotfiles_paths(self):
        # The acceptance grep runs over the whole skill dir, tests included.
        for path in [*SKILL.parent.rglob("*.md"), *SKILL.parent.rglob("*.py")]:
            lines = path.read_text(encoding="utf-8").splitlines()
            self.assertEqual([n for n, s in enumerate(lines, 1) if BANNED.search(s)], [], path)
        self.assertEqual(skillkit.dotfiles_paths(self.text), [])
        # A quoted `~` does not expand in Bash.
        self.assertNotIn('"<data dir>', self.text)


if __name__ == "__main__":
    unittest.main()
