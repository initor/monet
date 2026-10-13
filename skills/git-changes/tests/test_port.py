"""Port contract for the git-changes skill (CORE-4)."""

import re
import unittest
from pathlib import Path

import skillkit

SKILL_MD = Path(__file__).resolve().parent.parent / "SKILL.md"
NAME = "git-changes"
SIBLINGS = ("git-draft-commit", "git-draft-pr", "cwd-context")
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


class GitChangesPort(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.skill = skillkit.read_skill(SKILL_MD)
        cls.text = SKILL_MD.read_text(encoding="utf-8")

    def test_violation_names_are_generic(self):
        body = self.skill.body
        self.assertIn('violation: "none|on_main|missing_branch_prefix|detached_head"', body)
        self.assertRegex(body, r"\bhas_branch_prefix: true\|false")
        # The prefix check applies only when a prefix is set, on the line that sets the violation.
        rule = rule_line(body, 'set `violation: "missing_branch_prefix"`')
        self.assertIn("if a prefix is set and the branch does NOT start with it", rule)
        self.assertIn("This check applies only when a prefix is set.", rule)
        branches = CHECKOUT.findall(body)
        self.assertTrue(branches, "no branch suggestion")
        for branch in branches:
            self.assertTrue(branch.startswith(PREFIX + "<"), f"branch suggestion {branch!r} skips the prefix")
        templates = [(line_of(body, m.start()), m.group(1)) for m in TEMPLATE.finditer(body)]
        self.assertEqual([t for t in templates if t[1] != PREFIX], [])
        self.assertEqual(skillkit.dotfiles_paths(self.text), [])

    def test_user_config_fallbacks(self):
        self.assertIn(PREFIX, self.skill.body)
        self.assertEqual(skillkit.missing_fallbacks(self.text), [])
        self.assertNotIn("${user_config.data_dir}", self.text)

    def test_siblings_named_with_monet_prefix(self):
        bare = [(line_of(self.text, m.start()), m.group()) for m in BARE.finditer(self.text)]
        self.assertEqual(bare, [])
        self.assertIn("monet:cwd-context", self.skill.body)

    def test_names_no_other_skill_md(self):
        self.assertEqual(skillkit.skill_md_refs(self.text, NAME), [])


if __name__ == "__main__":
    unittest.main()
