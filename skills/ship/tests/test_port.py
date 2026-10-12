"""Port contract for the ship skill (CORE-4): renamed from its old git- name, with its command folded in."""

import re
import unittest
from pathlib import Path

import skillkit

SKILL_MD = Path(__file__).resolve().parent.parent / "SKILL.md"
NAME = "ship"
# The skills ship invokes or dispatches by name. Each must read monet:<name>. A hyphenated name
# is never a plain word, so any bare use counts. `gang` counts when it stands alone in backticks,
# so prose such as "a gang run" stays plain. A later mention of `voice`, once the Voice section
# has loaded it as monet:voice, names it and invokes nothing, so it may stay bare.
SIBLINGS = ("git-changes", "git-draft-commit", "git-draft-pr", "cwd-context", "review-local")
BARE = re.compile(r"(?<![\w:/-])(" + "|".join(map(re.escape, SIBLINGS)) + r")(?![\w-])|`gang`")
CHECKOUT = re.compile(r"git checkout -b (\S+)")
PREFIX = "${user_config.branch_prefix}"
# A branch template in prose: whatever stands before a slug placeholder must be the prefix.
# `<name>` is left to CHECKOUT, since in prose it can name the current branch.
TEMPLATE = re.compile(r"([^\s`]*)<(?:slug|suggested-slug|descriptive_slug)>")
# Built from parts, so a grep of skills/ship for the old name finds nothing, this file included.
OLD_NAME = "-".join(("git", NAME))


def line_of(text, pos):
    return text.count("\n", 0, pos) + 1


def section(body, heading):
    match = re.search(r"^## " + re.escape(heading) + r"\n(.*?)(?=^## |\Z)", body, re.S | re.M)
    if match is None:
        raise AssertionError(f"no section {heading!r}")
    return match.group(1)


class ShipPort(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.skill = skillkit.read_skill(SKILL_MD)
        cls.text = SKILL_MD.read_text(encoding="utf-8")

    def test_name_is_ship(self):
        self.assertEqual(self.skill.name, NAME)
        self.assertEqual(self.skill.frontmatter["name"], NAME)
        self.assertNotIn(OLD_NAME, self.text)

    def test_frontmatter_folds_command(self):
        meta = self.skill.frontmatter
        self.assertNotIn("argument-hint", meta)
        self.assertEqual(meta["allowed-tools"], ["Read", "Bash", "Glob", "Grep", "AskUserQuestion", "Skill"])
        self.assertTrue(meta["description"].startswith("Use when"))
        # The wrapper's precondition and its closing report step live in the body now.
        preconditions = section(self.skill.body, "Preconditions")
        self.assertIn("git rev-parse --is-inside-work-tree", preconditions)
        self.assertIn("If that first check fails, the cwd isn't a git repo: tell the user and stop.", preconditions)
        self.assertIn("Report what was drafted and which write actions (if any) ran", self.skill.body)

    def test_branch_prefix_from_user_config(self):
        body = self.skill.body
        branches = CHECKOUT.findall(body)
        self.assertTrue(branches, "ship suggests no new branch")
        for branch in branches:
            self.assertTrue(branch.startswith(PREFIX + "<"), f"branch suggestion {branch!r} skips the prefix")
        # Every prose suggestion too: Step 1 (detached HEAD), Step 2 (AskUserQuestion), and Constraints.
        templates = [(line_of(body, m.start()), m.group(1)) for m in TEMPLATE.finditer(body)]
        self.assertEqual([t for t in templates if t[1] != PREFIX], [])
        for heading in ("Step 1: Detect environment", "Step 2: Branch management", "Constraints"):
            self.assertRegex(section(body, heading), re.escape(PREFIX) + r"<(slug|suggested-slug)>", heading)
        # The prefix check applies only when a prefix is set.
        step2 = section(body, "Step 2: Branch management")
        self.assertIn("The prefix check applies only when a prefix is set.", step2)
        self.assertIn("missing_branch_prefix", step2)
        self.assertEqual(skillkit.dotfiles_paths(self.text), [])

    def test_user_config_fallbacks(self):
        self.assertIn(PREFIX, self.skill.body)
        self.assertEqual(skillkit.missing_fallbacks(self.text), [])
        self.assertNotIn("${user_config.data_dir}", self.text)

    def test_siblings_named_with_monet_prefix(self):
        bare = [(line_of(self.text, m.start()), m.group()) for m in BARE.finditer(self.text)]
        self.assertEqual(bare, [])
        for name in ("git-changes", "git-draft-commit", "git-draft-pr", "voice", "gang"):
            self.assertIn(f"monet:{name}", self.text)

    def test_names_no_other_skill_md(self):
        self.assertEqual(skillkit.skill_md_refs(self.text, NAME), [])

    def test_voice_loaded_by_skill_tool(self):
        self.assertIn("Skill", self.skill.frontmatter["allowed-tools"])
        voice = re.search(r"^## Voice\n(.*?)^## ", self.skill.body, re.S | re.M)
        self.assertIsNotNone(voice, "no Voice section")
        self.assertRegex(voice.group(1), r"Skill tool")
        self.assertRegex(voice.group(1), r"`monet:voice`")
        self.assertRegex(voice.group(1), r"`--no-ste`")
        self.assertLess(self.skill.body.index("## Voice"), self.skill.body.index("## Step 4"))
        self.assertNotRegex(self.text, r"voice/SKILL\.md|\$\{CLAUDE_(SKILL_DIR|PLUGIN_ROOT)\}")


if __name__ == "__main__":
    unittest.main()
