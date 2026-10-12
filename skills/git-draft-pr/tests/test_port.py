"""Port contract for the git-draft-pr skill (CORE-4)."""

import re
import unittest
from pathlib import Path

import skillkit

SKILL_MD = Path(__file__).resolve().parent.parent / "SKILL.md"
NAME = "git-draft-pr"
SIBLINGS = ("git-changes", "git-draft-commit", "cwd-context")
BARE = re.compile(r"(?<![\w:/-])(" + "|".join(map(re.escape, SIBLINGS)) + r")(?![\w-])|`(ship|voice)`")
CHECKOUT = re.compile(r"git checkout -b (\S+)")
PREFIX = "${user_config.branch_prefix}"
# A branch template in prose: whatever stands before a slug placeholder must be the prefix.
# `<name>` is left to CHECKOUT, since in prose it can name the current branch.
TEMPLATE = re.compile(r"([^\s`]*)<(?:slug|suggested-slug|descriptive_slug)>")


def line_of(text, pos):
    return text.count("\n", 0, pos) + 1


def rule_line(body, marker):
    """The one line of body that holds marker, so an assert on it checks that rule and no other."""
    lines = [line for line in body.splitlines() if marker in line]
    if len(lines) != 1:
        raise AssertionError(f"{len(lines)} lines hold {marker!r}, want 1")
    return lines[0]


class GitDraftPrPort(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.skill = skillkit.read_skill(SKILL_MD)
        cls.text = SKILL_MD.read_text(encoding="utf-8")

    def test_voice_loaded_by_skill_tool(self):
        self.assertIn("Skill", self.skill.frontmatter["allowed-tools"])
        voice = re.search(r"^## Voice\n(.*?)^## ", self.skill.body, re.S | re.M)
        self.assertIsNotNone(voice, "no Voice section")
        self.assertRegex(voice.group(1), r"Skill tool")
        self.assertRegex(voice.group(1), r"`monet:voice`")
        self.assertRegex(voice.group(1), r"`--no-ste`")
        self.assertLess(self.skill.body.index("## Voice"), self.skill.body.index("## Step 2"))
        self.assertNotRegex(self.text, r"voice/SKILL\.md|\$\{CLAUDE_(SKILL_DIR|PLUGIN_ROOT)\}")

    def test_user_config_fallbacks(self):
        self.assertIn(PREFIX, self.skill.body)
        self.assertEqual(skillkit.missing_fallbacks(self.text), [])
        self.assertNotIn("${user_config.data_dir}", self.text)

    def test_branch_prefix_from_user_config(self):
        branches = CHECKOUT.findall(self.skill.body)
        self.assertTrue(branches, "no branch suggestion")
        for branch in branches:
            self.assertTrue(branch.startswith(PREFIX + "<"), f"branch suggestion {branch!r} skips the prefix")
        body = self.skill.body
        templates = [(line_of(body, m.start()), m.group(1)) for m in TEMPLATE.finditer(body)]
        self.assertEqual([t for t in templates if t[1] != PREFIX], [])
        # The prefix check applies only when a prefix is set, on the line that holds the check.
        rule = rule_line(body, "(`missing_branch_prefix`)")
        self.assertIn("This check applies only when a prefix is set.", rule)
        self.assertEqual(skillkit.dotfiles_paths(self.text), [])

    def test_description_points_to_ship(self):
        self.assertTrue(self.skill.frontmatter["description"].startswith("Use when"))
        self.assertIn("monet:ship", self.skill.frontmatter["description"])

    def test_siblings_named_with_monet_prefix(self):
        bare = [(line_of(self.text, m.start()), m.group()) for m in BARE.finditer(self.text)]
        self.assertEqual(bare, [])
        for name in ("git-changes", "cwd-context", "ship"):
            self.assertIn(f"monet:{name}", self.skill.body)

    def test_names_no_other_skill_md(self):
        self.assertEqual(skillkit.skill_md_refs(self.text, NAME), [])


if __name__ == "__main__":
    unittest.main()
