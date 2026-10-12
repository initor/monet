"""Port contract for the status skill (CORE-3): the command folded in, with a live inventory."""

import os
import re
import subprocess
import unittest
from pathlib import Path

import skillkit

SKILL = Path(__file__).resolve().parent.parent
REPO = SKILL.parent.parent
SKILL_MD = SKILL / "SKILL.md"
NAME = "status"
WRAPPER_TOOLS = ["Read", "Bash", "Glob", "Grep", "AskUserQuestion"]
SKILLS_GLOB = "${CLAUDE_PLUGIN_ROOT}/skills/*/SKILL.md"
AGENT = "${CLAUDE_PLUGIN_ROOT}/agents/monet.md"
# Built from parts, so this file does not hold the paths it scans for.
OLD_COMMANDS = "/".join(("commands", "monet")) + "/"
INCLUDE = re.compile(r"^\s*@\S", re.M)


def git(*args):
    env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}
    return subprocess.run(["git", "-C", str(REPO), *args], check=True, capture_output=True,
                          text=True, env=env, timeout=30).stdout


class StatusPort(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.skill = skillkit.read_skill(SKILL_MD)
        cls.text = SKILL_MD.read_text(encoding="utf-8")

    def test_scope_argument(self):
        meta = self.skill.frontmatter
        self.assertEqual(meta["name"], NAME)
        self.assertEqual(self.skill.name, NAME)
        self.assertEqual(meta["argument-hint"], "[skills|agent|all]")
        self.assertEqual(meta["allowed-tools"], WRAPPER_TOOLS)
        body = self.skill.body
        self.assertIn("$ARGUMENTS", body)
        for scope in ("`skills`", "`agent`", "`all`"):
            self.assertIn(scope, body)
        # An empty or unknown scope asks, rather than guessing.
        self.assertIn("AskUserQuestion", body)
        self.assertEqual(git("ls-files", "import/commands/monet/status.md"), "")

    def test_inventory_from_plugin_root(self):
        body = self.skill.body
        self.assertIn(SKILLS_GLOB, body)
        self.assertIn(AGENT, body)
        self.assertIn("frontmatter", body)
        self.assertIn("`name`", body)
        self.assertIn("`description`", body)
        self.assertIn("Safety defaults", body)
        self.assertNotIn(OLD_COMMANDS, self.text)
        self.assertEqual(skillkit.dotfiles_paths(self.text), [])
        self.assertIsNone(INCLUDE.search(self.text))
        self.assertNotIn("~/" + ".config", self.text)

    def test_description_use_when(self):
        desc = self.skill.frontmatter["description"].strip()
        self.assertTrue(desc.startswith("Use when"), desc)
        self.assertLessEqual(len(desc), 500)

    def test_names_no_other_skill_md(self):
        self.assertEqual(skillkit.skill_md_refs(self.text, NAME), [])
        self.assertEqual(skillkit.missing_fallbacks(self.text), [])


if __name__ == "__main__":
    unittest.main()
