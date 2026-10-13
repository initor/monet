"""Port contract for the help skill (CORE-3): the command, its workflow, and a live inventory."""

import os
import re
import subprocess
import unittest
from pathlib import Path

import skillkit

SKILL = Path(__file__).resolve().parent.parent
REPO = SKILL.parent.parent
SKILL_MD = SKILL / "SKILL.md"
REFERENCE = SKILL / "reference" / "help.md"
NAME = "help"
WRAPPER_TOOLS = ["Read", "Bash", "Glob", "Grep"]
SKILLS_GLOB = "${CLAUDE_PLUGIN_ROOT}/skills/*/SKILL.md"
AGENT = "${CLAUDE_PLUGIN_ROOT}/agents/monet.md"
# Built from parts, so this file does not hold the paths it scans for.
OLD_COMMANDS = "/".join(("commands", "monet")) + "/"
INCLUDE = re.compile(r"^\s*@\S", re.M)


def git(*args):
    env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}
    return subprocess.run(["git", "-C", str(REPO), *args], check=True, capture_output=True,
                          text=True, env=env, timeout=30).stdout


class HelpPort(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.skill = skillkit.read_skill(SKILL_MD)
        cls.text = SKILL_MD.read_text(encoding="utf-8")
        cls.reference = REFERENCE.read_text(encoding="utf-8")

    def test_frontmatter_folds_command(self):
        meta = self.skill.frontmatter
        self.assertEqual(meta["name"], NAME)
        self.assertEqual(self.skill.name, NAME)
        self.assertEqual(meta["allowed-tools"], WRAPPER_TOOLS)
        self.assertNotIn("argument-hint", meta)
        self.assertEqual(git("ls-files", "import/commands/monet/help.md", "import/workflows"), "")

    def test_inventory_from_plugin_root(self):
        body = self.skill.body
        self.assertIn(SKILLS_GLOB, body)
        self.assertIn(AGENT, body)
        self.assertIn("${CLAUDE_SKILL_DIR}/reference/help.md", body)
        # Only the frontmatter of each listed skill is read, never its instructions.
        self.assertIn("frontmatter", body)
        self.assertIn("`name`", body)
        self.assertIn("`description`", body)
        self.assertIn("/monet:<name>", body)
        for text in (self.text, self.reference):
            self.assertNotIn(OLD_COMMANDS, text)
            self.assertEqual(skillkit.dotfiles_paths(text), [])
            self.assertIsNone(INCLUDE.search(text))
        self.assertNotIn("~/" + ".config", self.text)

    def test_reference_describes_plugin_skills(self):
        self.assertIn("/monet:<name>", self.reference)
        self.assertIn("`skills/`", self.reference)
        self.assertNotIn(".claude/", self.reference)

    def test_description_use_when(self):
        desc = self.skill.frontmatter["description"].strip()
        self.assertTrue(desc.startswith("Use when"), desc)
        self.assertLessEqual(len(desc), 500)

    def test_names_no_other_skill_md(self):
        self.assertEqual(skillkit.skill_md_refs(self.text, NAME), [])
        self.assertEqual(skillkit.skill_md_refs(self.reference, NAME), [])
        self.assertEqual(skillkit.missing_fallbacks(self.text), [])


if __name__ == "__main__":
    unittest.main()
