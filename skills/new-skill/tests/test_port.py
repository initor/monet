"""Port contract for the new-skill skill (CORE-3): scaffold a plugin skill and register its route."""

import os
import re
import subprocess
import unittest
from pathlib import Path

import skillkit

SKILL = Path(__file__).resolve().parent.parent
REPO = SKILL.parent.parent
SKILL_MD = SKILL / "SKILL.md"
TEMPLATE = SKILL / "reference" / "skill-template.md"
NAME = "new-skill"
WRAPPER_TOOLS = ["Read", "Write", "Edit", "Bash", "Glob", "Grep", "AskUserQuestion"]
PREFIX = "${user_config." + "branch_prefix}"
# The dotfiles tooling and personal paths that no file of these three skills may hold.
# Built from parts, so this file does not hold them either.
FORBIDDEN = (
    "w" + "awen",
    "~/" + ".config",
    "~/" + ".claude",
    "bin/" + "check-monet-skill.py",
    "check-" + "portability.sh",
)
PORTED = ("help", "status", "new-skill")


def git(*args):
    env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}
    return subprocess.run(["git", "-C", str(REPO), *args], check=True, capture_output=True,
                          text=True, env=env, timeout=30).stdout


def section(body, heading):
    match = re.search(r"^## " + re.escape(heading) + r"\n(.*?)(?=^## |\Z)", body, re.S | re.M)
    if match is None:
        raise AssertionError(f"no section {heading!r}")
    return match.group(1)


class NewSkillPort(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.skill = skillkit.read_skill(SKILL_MD)
        cls.text = SKILL_MD.read_text(encoding="utf-8")

    def test_frontmatter_folds_command(self):
        meta = self.skill.frontmatter
        self.assertEqual(meta["name"], NAME)
        self.assertEqual(self.skill.name, NAME)
        self.assertEqual(meta["argument-hint"], "[skill-name]")
        self.assertEqual(meta["allowed-tools"], WRAPPER_TOOLS)
        self.assertIn("$ARGUMENTS", self.skill.body)
        self.assertEqual(git("ls-files", "import/commands/monet/new-skill.md", "import/templates"), "")

    def test_template_in_skill_dir(self):
        self.assertIn("${CLAUDE_SKILL_DIR}/reference/skill-template.md", self.skill.body)
        template = TEMPLATE.read_text(encoding="utf-8")
        for slot in ("{{SKILL_NAME}}", "{{SKILL_DESCRIPTION}}"):
            self.assertIn(slot, template)
        self.assertIn("skills/<name>/SKILL.md", self.skill.body)

    def test_resolves_monet_checkout(self):
        step = section(self.skill.body, "Step 0: Resolve ROOT")
        self.assertIn(".claude-plugin/plugin.json", step)
        self.assertIn('"name": "monet"', step)
        # ROOT is the checkout the user names, else the one that holds the cwd, never the plugin dir.
        self.assertIn("Take the path the user named, else the cwd.", step)
        self.assertIn("Set ROOT to what `git -C <path> rev-parse --show-toplevel` prints.", step)
        self.assertNotIn("${CLAUDE_PLUGIN_ROOT}", step)
        self.assertIn("plugin cache", step)
        # The worktree branch takes the configured prefix, with the R1 fallback right after.
        self.assertIn(PREFIX + "<name>", step)
        self.assertEqual(skillkit.missing_fallbacks(self.text), [])
        self.assertNotIn("${user_config." + "data_dir}", self.text)

    def test_registers_monet_route(self):
        body = self.skill.body
        register = section(body, "Step 3: Register the skill in the monet agent")
        self.assertIn("agents/monet.md", register)
        self.assertIn("`monet:<name>`", register)
        # One routing row and one dispatch line, the new step placed before Safety defaults.
        self.assertIn("Add one row to the Quick-reference routing table", register)
        self.assertIn("Add one dispatch line.", register)
        self.assertIn("immediately before the step labelled Safety defaults", register)
        self.assertNotIn("after the step labelled Safety defaults", register)
        self.assertIn("by its text, never by its number", register)
        # The skill runs make check itself and fixes each failure, not only reports it.
        verify = section(body, "Step 4: Verify and report")
        self.assertIn("Run `make -C ROOT check`.", verify)
        self.assertIn("Fix each failure at its cause, then run it again until it exits 0.", verify)
        self.assertLess(body.index("## Step 3"), body.index("## Step 4"))
        # Commands fold into skills: the skill writes no wrapper.
        self.assertNotIn("commands/", self.text)

    def test_no_dotfiles_tooling(self):
        for name in PORTED:
            for path in sorted((REPO / "skills" / name).rglob("*")):
                if not path.is_file() or "__pycache__" in path.parts:
                    continue
                text = path.read_text(encoding="utf-8")
                for needle in FORBIDDEN:
                    self.assertNotIn(needle, text, str(path.relative_to(REPO)))
                self.assertEqual(skillkit.dotfiles_paths(text), [], str(path.relative_to(REPO)))
                self.assertEqual(skillkit.missing_fallbacks(text), [], str(path.relative_to(REPO)))

    def test_description_use_when(self):
        desc = self.skill.frontmatter["description"].strip()
        self.assertTrue(desc.startswith("Use when"), desc)
        self.assertLessEqual(len(desc), 500)
        self.assertNotIn("command", desc.lower())

    def test_names_no_other_skill_md(self):
        self.assertEqual(skillkit.skill_md_refs(self.text, NAME), [])
        self.assertEqual(skillkit.skill_md_refs(TEMPLATE.read_text(encoding="utf-8"), NAME), [])


if __name__ == "__main__":
    unittest.main()
