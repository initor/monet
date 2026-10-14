"""Contract for the skill authoring guide, docs/authoring.md (DOC-4)."""

import ast
import re
import subprocess
import sys
import unittest
from pathlib import Path

import skillkit

ROOT = Path(__file__).resolve().parents[2]
DOC = ROOT / "docs" / "authoring.md"
LINT = ROOT / "tools" / "check_skills.py"
NEW_SKILL = ROOT / "skills" / "new-skill" / "SKILL.md"
AGENT = ROOT / "agents" / "monet.md"
STE_CHECK = ROOT / "skills" / "voice" / "scripts" / "ste_check.py"

FENCE = re.compile(r"^```[^\n]*\n(.*?)^```$", re.M | re.S)
# The manual steps, in order, each found by a phrase its heading must hold.
STEPS = ("Name the skill", "Write `skills/<name>/SKILL.md`", "Register it in `agents/monet.md`",
         "Add tests", "Refresh the README catalog", "Run `make check`")


def doc_text():
    return DOC.read_text(encoding="utf-8")


def section(text, title, level=2):
    """The body of the first section at `level` whose heading holds `title`."""
    marks = "#" * level
    for part in re.split(rf"^{marks} ", text, flags=re.M)[1:]:
        head, _, body = part.partition("\n")
        if title in head:
            return re.split(rf"^#{{1,{level}}} ", body, flags=re.M)[0]
    return ""


def flat(body):
    """The body without fenced code, with line breaks folded into spaces."""
    return re.sub(r"\s+", " ", FENCE.sub(" ", body))


def lint_constants():
    """KEYS and MAX_DESCRIPTION, read from the lint's source, not run."""
    found = {}
    for node in ast.parse(LINT.read_text(encoding="utf-8")).body:
        if isinstance(node, ast.Assign) and isinstance(node.targets[0], ast.Name):
            if node.targets[0].id in ("KEYS", "MAX_DESCRIPTION"):
                found[node.targets[0].id] = ast.literal_eval(node.value)
    return found["KEYS"], found["MAX_DESCRIPTION"]


class AuthoringDoc(unittest.TestCase):
    def test_names_new_skill_fast_path(self):
        text = doc_text()
        first = re.findall(r"^## (.+)$", text, re.M)[0]
        self.assertIn("`/monet:new-skill`", first)
        body = flat(section(text, "/monet:new-skill"))
        for needle in ("`skills/<name>/SKILL.md`", "routing row", "dispatch",
                       "`make check`", "never commits"):
            with self.subTest(needle=needle):
                self.assertIn(needle, body)
        # What the doc says new-skill writes is what new-skill says it writes.
        skill = re.sub(r"\s+", " ", NEW_SKILL.read_text(encoding="utf-8"))
        for needle in ("skills/<name>/SKILL.md", "routing row", "dispatch line",
                       "make check", "never commits"):
            with self.subTest(source=needle):
                self.assertIn(needle, skill)

    def test_manual_steps_in_order(self):
        manual = section(doc_text(), "Manual steps")
        heads = re.findall(r"^### (.+)$", manual, re.M)
        self.assertEqual(len(heads), len(STEPS), heads)
        for n, (head, phrase) in enumerate(zip(heads, STEPS), 1):
            with self.subTest(step=n):
                self.assertEqual(head, f"{n}. {phrase}")

    def test_naming_step(self):
        body = flat(section(doc_text(), "Name the skill", 3))
        for needle in ("The dir name, the frontmatter `name`, and the text after `/monet:` are the same string",
                       "`git-*`", "`review-*`", "joins a family only when it belongs there"):
            with self.subTest(needle=needle):
                self.assertIn(needle, body)

    def test_frontmatter_keys_match_lint(self):
        keys, cap = lint_constants()
        body = section(doc_text(), "Write `skills/<name>/SKILL.md`", 3)
        para = re.search(r"^The lint accepts these frontmatter keys:.*?(?=\n\n)", body, re.M | re.S)
        self.assertIsNotNone(para, "no paragraph lists the frontmatter keys")
        self.assertEqual(re.findall(r"`([a-z-]+)`", para.group(0)), list(keys))
        prose = flat(body)
        for needle in ("must start with `Use when`", f"at most {cap} characters",
                       "No `commands/` dir ships, because commands folded into skills"):
            with self.subTest(needle=needle):
                self.assertIn(needle, prose)

    def test_covers_plugin_paths(self):
        raw = section(doc_text(), "Write `skills/<name>/SKILL.md`", 3)
        body = flat(raw)
        for needle in ("`${CLAUDE_SKILL_DIR}/<path>`", "`${CLAUDE_PLUGIN_ROOT}/skills/<other>/<path>`",
                       "`monet:<name>`", "`${user_config.branch_prefix}`",
                       "`${user_config.data_dir}`", "Nothing writes inside the plugin dir",
                       "An unset key stays as the literal placeholder text",
                       "even though `plugin.json` gives it a `default`",
                       "each skill falls back to the default itself",
                       "no prefix", "`~/.config/monet`",
                       "writes each placeholder once and follows it with a fallback sentence",
                       "never repeats the full placeholder", "`tools/skillkit.py`"):
            with self.subTest(needle=needle):
                self.assertIn(needle, body)
        # The worked example passes the same fallback check that skills pass.
        example = "\n".join(FENCE.findall(raw))
        for key in skillkit.USER_CONFIG_FALLBACKS:
            with self.subTest(key=key):
                self.assertIn(f"${{user_config.{key}}}", example)
        self.assertEqual(skillkit.missing_fallbacks(example), [])

    def test_register_step(self):
        body = flat(section(doc_text(), "Register it in `agents/monet.md`", 3))
        for needle in ("routing row", "numbered dispatch step immediately before the `Safety defaults` step",
                       "by name, not by number"):
            with self.subTest(needle=needle):
                self.assertIn(needle, body)
        self.assertRegex(AGENT.read_text(encoding="utf-8"), r"(?m)^\d+\) Safety defaults:")

    def test_tests_step(self):
        body = flat(section(doc_text(), "Add tests", 3))
        for needle in ("`skills/<name>/tests/`", "`test_<x>.py`", "at least one test",
                       "exits 5", "Use stdlib `unittest` only", "`tools/skillkit.py`",
                       "no Makefile edit"):
            with self.subTest(needle=needle):
                self.assertIn(needle, body)

    def test_names_catalog_refresh(self):
        body = flat(section(doc_text(), "Refresh the README catalog", 3))
        for needle in ("`python3 tools/catalog.py --write`", "adding or renaming",
                       "first sentence", "stale catalog"):
            with self.subTest(needle=needle):
                self.assertIn(needle, body)

    def test_has_no_dotfiles_paths(self):
        self.assertEqual(skillkit.dotfiles_paths(doc_text()), [])

    def test_prose_has_no_em_dash_or_semicolon(self):
        result = subprocess.run([sys.executable, str(STE_CHECK), str(DOC)],
                                capture_output=True, text=True, check=False, timeout=60)
        self.assertIn("em-dashes 0, semicolons 0", result.stdout)
        self.assertNotIn(chr(0x2014), doc_text())


if __name__ == "__main__":
    unittest.main()
