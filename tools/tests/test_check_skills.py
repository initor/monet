"""Tests for tools/check_skills.py, the skill lint (FND-4).

Each test builds a fixture repo in a temp dir (`git init`, no commit), copies
tools/check_skills.py and tools/skillkit.py into its tools/, and runs the lint
there. No test reads the real skills/, agents/, or import/.
"""

import os
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

TOOLS = Path(__file__).resolve().parent.parent
MAKEFILE = TOOLS.parent / "Makefile"

SKILL = ("---\nname: paint\ndescription: Use when the user asks for a page.\n---\n"
         "Read ${CLAUDE_SKILL_DIR}/reference/style.md first.\n")
AGENT = "---\nname: monet\n---\n| a page | `monet:paint` |\n\n- To make a page, load monet:paint.\n"
WIRED = {
    "skills/paint/SKILL.md": SKILL,
    "skills/paint/reference/style.md": "Run ${CLAUDE_PLUGIN_ROOT}/skills/paint/scripts/render.py.\n",
    "skills/paint/scripts/render.py": "print('page')\n",
    "agents/monet.md": AGENT,
    "output-styles/card.md": "See ${CLAUDE_PLUGIN_ROOT}/skills/paint.\n",
}


def env():
    """The caller's environment without GIT_*, PYTHONPATH, or an outer make's flags.

    So only the fixture's tools/ load, and a nested make reads only the fixture's Makefile.
    """
    drop = ("PYTHONPATH", "MAKEFLAGS", "MFLAGS", "MAKELEVEL", "MAKEFILES")
    keep = {k: v for k, v in os.environ.items() if not k.startswith("GIT_") and k not in drop}
    return dict(keep, PYTHONDONTWRITEBYTECODE="1")


class CheckSkills(unittest.TestCase):
    def repo(self, files=None, **changes):
        """Build a fixture repo from WIRED (or `files`), updated by `changes`; None drops a path."""
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        root = Path(temp.name).resolve()
        subprocess.run(["git", "init", "-q", str(root)], check=True, env=env())
        tree = dict(WIRED if files is None else files)
        tree.update(changes)
        for name in ("check_skills.py", "skillkit.py"):
            tree["tools/" + name] = (TOOLS / name).read_text(encoding="utf-8")
        for rel, text in tree.items():
            if text is not None:
                (root / rel).parent.mkdir(parents=True, exist_ok=True)
                (root / rel).write_text(text, encoding="utf-8")
        return root

    def lint(self, root, *names):
        return subprocess.run([sys.executable, str(root / "tools" / "check_skills.py"), *names],
                              cwd=root, capture_output=True, text=True, env=env())

    def fails(self, result):
        return [line for line in result.stdout.splitlines() if line.startswith("FAIL")]

    def assert_one_fail(self, result, *parts):
        fails = self.fails(result)
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertEqual(len(fails), 1, result.stdout)
        for part in parts:
            self.assertIn(part, fails[0])

    def test_wired_skill_passes(self):
        root = self.repo()
        for names in ((), ("paint",)):
            result = self.lint(root, *names)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertEqual(self.fails(result), [])
            for label in ("frontmatter", "name", "keys", "description", "paths", "routing", "commands"):
                self.assertRegex(result.stdout, rf"(?m)^ok +{label} ")

    def test_missing_skill_fails(self):
        result = self.lint(self.repo(), "ghost")
        self.assert_one_fail(result, "skill", "skills/ghost/SKILL.md")

    def test_bad_name_prints_usage(self):
        root = self.repo()
        for bad in ("Not_A_Name", "--all", "paint-"):
            result = self.lint(root, "paint", bad)
            self.assertEqual(result.returncode, 2, bad)
            self.assertIn("usage: python3 tools/check_skills.py [<name> ...]", result.stderr)
            self.assertEqual(result.stdout, "")

    def test_unparsable_frontmatter_fails(self):
        body = "Read ${CLAUDE_SKILL_DIR}/reference/style.md first.\n"
        for text, why in ((body, "no --- frontmatter"),
                          ("---\nname: paint\n" + body, "no closing ---"),
                          ("---\nname: paint\nname: paint\n---\n" + body, "duplicate key")):
            with self.subTest(why=why):
                result = self.lint(self.repo(**{"skills/paint/SKILL.md": text}))
                self.assert_one_fail(result, "frontmatter", "skills/paint/SKILL.md", why)

    def test_no_names_checks_every_skill_and_names_check_only_those(self):
        sketch = ("---\nname: sketch\ndescription: Draws a sketch.\n---\n"
                  "Read ${CLAUDE_SKILL_DIR}/reference/missing.md first.\n")
        root = self.repo(**{"skills/sketch/SKILL.md": sketch})
        result = self.lint(root)
        self.assertEqual(result.returncode, 1, result.stdout)
        fails = self.fails(result)
        self.assertEqual(len(fails), 2, result.stdout)
        self.assertRegex(fails[0], r"^FAIL +description +skills/sketch/SKILL.md")
        self.assertRegex(fails[1], r"^FAIL +paths +skills/sketch/SKILL.md:5")
        self.assertRegex(result.stdout, r"(?m)^ok +description +skills/paint/SKILL.md")
        result = self.lint(root, "paint")
        self.assertEqual(result.returncode, 0, result.stdout)
        self.assertNotIn("sketch", result.stdout)
        self.assertRegex(result.stdout, r"(?m)^ok +description +skills/paint/SKILL.md")
        result = self.lint(root, "sketch")
        self.assertEqual(len(self.fails(result)), 2, result.stdout)
        self.assertNotIn("skills/paint/SKILL.md", result.stdout)

    def test_name_mismatch_fails(self):
        text = SKILL.replace("name: paint", "name: painter")
        self.assert_one_fail(self.lint(self.repo(**{"skills/paint/SKILL.md": text})), "name", "painter")

    def test_unknown_frontmatter_key_fails(self):
        text = SKILL.replace("---\nRead", "color: blue\n---\nRead")
        self.assert_one_fail(self.lint(self.repo(**{"skills/paint/SKILL.md": text})), "keys", "color")
        known = SKILL.replace("---\nRead", "argument-hint: <topic>\nallowed-tools: [Read, Write]\n"
                              "model: sonnet\ndisable-model-invocation: false\nuser-invocable: true\n---\nRead")
        self.assertEqual(self.lint(self.repo(**{"skills/paint/SKILL.md": known})).returncode, 0)

    def test_description_without_use_when_fails(self):
        for desc in ("Paints a page.", "", "Use this skill when the user asks for a page.",
                     "use when the user asks for a page.", "Use when " + "x" * 492):
            text = SKILL.replace("Use when the user asks for a page.", desc)
            with self.subTest(desc=desc[:20]):
                self.assert_one_fail(self.lint(self.repo(**{"skills/paint/SKILL.md": text})), "description")
        text = SKILL.replace("Use when the user asks for a page.", "Use when " + "x" * 491)
        self.assertEqual(self.lint(self.repo(**{"skills/paint/SKILL.md": text})).returncode, 0)

    def test_agent_routes_to_missing_skill_fails_only_routing(self):
        agent = AGENT + ("- To ghost, load monet:ghost. I am monet:monet, styled by monet:card.\n"
                         "- To ship, load monet:git-ship, styled by monet:dark-card.\n")
        hyphens = {"skills/git-ship/SKILL.md": "---\nname: git-ship\ndescription: Use when shipping.\n---\n",
                   "output-styles/dark-card.md": "A dark card.\n"}
        result = self.lint(self.repo(**{"agents/monet.md": agent}, **hyphens))
        self.assert_one_fail(result, "routing", "agents/monet.md:7", "monet:ghost")
        self.assertRegex(result.stdout, r"(?m)^ok +name +skills/git-ship/SKILL.md")
        result = self.lint(self.repo(**{"agents/monet.md": agent,
                                        "output-styles/dark-card.md": hyphens["output-styles/dark-card.md"]}))
        self.assertEqual(len(self.fails(result)), 2, result.stdout)
        self.assertIn("agents/monet.md:8: monet:git-ship names no", result.stdout)
        result = self.lint(self.repo(**{"agents/monet.md": None}))
        self.assertEqual(result.returncode, 0, result.stdout)
        self.assertRegex(result.stdout, r"(?m)^skip +routing ")

    def test_unresolved_plugin_root_path_fails(self):
        result = self.lint(self.repo(**{"skills/paint/scripts/render.py": None}))
        self.assert_one_fail(result, "paths", "skills/paint/reference/style.md:1",
                             "${CLAUDE_PLUGIN_ROOT}/skills/paint/scripts/render.py")
        result = self.lint(self.repo(**{"output-styles/card.md": "See ${CLAUDE_PLUGIN_ROOT}/docs/x.md\n"}))
        self.assert_one_fail(result, "paths", "output-styles/card.md:1", "${CLAUDE_PLUGIN_ROOT}/docs/x.md")
        result = self.lint(self.repo(**{"skills/paint/reference/style.md": None}))
        self.assert_one_fail(result, "paths", "skills/paint/SKILL.md:5", "${CLAUDE_SKILL_DIR}/reference/style.md")
        agent = AGENT + "Run ${CLAUDE_PLUGIN_ROOT}/skills/paint/scripts/missing.py.\n"
        result = self.lint(self.repo(**{"agents/monet.md": agent}))
        self.assert_one_fail(result, "paths", "agents/monet.md:7",
                             "${CLAUDE_PLUGIN_ROOT}/skills/paint/scripts/missing.py")
        # On disk, but not in skillkit.files(): git-ignored, or under import/.
        untracked = {".gitignore": "build/\n", "build/gen.py": "print('gen')\n",
                     "import/bin/old.py": "print('old')\n"}
        for ref in ("build/gen.py", "build", "import/bin/old.py", "import"):
            with self.subTest(ref=ref):
                card = "See ${CLAUDE_PLUGIN_ROOT}/" + ref + "\n"
                result = self.lint(self.repo(**{"output-styles/card.md": card}, **untracked))
                self.assert_one_fail(result, "paths", "output-styles/card.md:1", "${CLAUDE_PLUGIN_ROOT}/" + ref)

    def test_placeholder_path_checks_parent_dir(self):
        ok = ("Read ${CLAUDE_SKILL_DIR}/reference/<topic>.md, ${CLAUDE_SKILL_DIR}/reference/{topic}.md,\n"
              "and ${CLAUDE_PLUGIN_ROOT}/skills/*/SKILL.md.\n")
        result = self.lint(self.repo(**{"skills/paint/reference/style.md": ok}))
        self.assertEqual(result.returncode, 0, result.stdout)
        bad = "Read ${CLAUDE_SKILL_DIR}/notes/{topic}.md.\n"
        result = self.lint(self.repo(**{"skills/paint/reference/style.md": bad}))
        self.assert_one_fail(result, "paths", "skills/paint/reference/style.md:1",
                             "${CLAUDE_SKILL_DIR}/notes/{topic}.md")

    def test_commands_dir_fails(self):
        result = self.lint(self.repo(**{"commands/monet/paint.md": "---\nname: monet:paint\n---\n"}))
        self.assert_one_fail(result, "commands", "commands/")

    def test_no_skills_dir_passes(self):
        result = self.lint(self.repo(files={}))
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(self.fails(result), [])
        self.assertRegex(result.stdout, r"(?m)^skip +skills ")

    def test_make_check_runs_the_lint(self):
        make = shutil.which("make")
        self.assertIsNotNone(make, "make is not on PATH")
        # A file named lint at the root: without .PHONY: lint, make would call
        # the step up to date and run nothing.
        root = self.repo(**{"Makefile": MAKEFILE.read_text(encoding="utf-8"), "lint": ""})
        # Under -n, make only prints the recipes; MAKE=: keeps a $(MAKE) line inert.
        result = subprocess.run([make, "-n", "check", "MAKE=:"], cwd=root, capture_output=True,
                                text=True, env=env())
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertRegex(result.stdout, r"(?m)^python3 tools/check_skills.py$")
        bad = SKILL.replace("Use when the user asks for a page.", "Paints a page.")
        for skill, code, line in ((SKILL, 0, r"(?m)^ok +description "), (bad, 2, r"(?m)^FAIL +description ")):
            with self.subTest(code=code):
                (root / "skills/paint/SKILL.md").write_text(skill, encoding="utf-8")
                result = subprocess.run([make, "lint"], cwd=root, capture_output=True, text=True, env=env())
                self.assertEqual(result.returncode, code, result.stdout + result.stderr)
                self.assertRegex(result.stdout, line)


if __name__ == "__main__":
    unittest.main()
