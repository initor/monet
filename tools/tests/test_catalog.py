"""Contract for tools/catalog.py, the README skill catalog (DOC-2)."""

import subprocess
import sys
import tempfile
import textwrap
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "tools"))
import catalog  # noqa: E402

SCRIPT = ROOT / "tools" / "catalog.py"
BEGIN, END = "<!-- BEGIN GENERATED:skills -->", "<!-- END GENERATED:skills -->"


class Catalog(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.root = Path(tmp.name)
        self.readme = self.root / "README.md"
        self.add_skill("zeta", "Use when z. Then more.")
        self.add_skill("alpha", "Use when a | b is asked. Then more.")
        self.readme.write_text(f"# Demo\n\nIntro.\n\n{BEGIN}\nold\n{END}\n\n## After\n", encoding="utf-8")

    def add_skill(self, name, description, extra=""):
        path = self.root / "skills" / name / "SKILL.md"
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(f"---\nname: {name}\ndescription: {description}\n{extra}---\n\nBody.\n",
                        encoding="utf-8")

    def run_catalog(self, *args):
        return subprocess.run([sys.executable, str(SCRIPT), "--root", str(self.root), *args],
                              capture_output=True, text=True, check=False, timeout=60)

    def test_first_sentence(self):
        cases = {
            "Use when x. Then y.": "Use when x.",
            "Use when asked \"what changed?\". Produces a digest.":
                "Use when asked \"what changed?\".",
            "Use when a (go.mod present) dir, e.g. this one. Then y.":
                "Use when a (go.mod present) dir, e.g. this one.",
            "Use when\n  wrapped over lines. Next.": "Use when wrapped over lines.",
            "No end mark at all": "No end mark at all",
            "Ends here.": "Ends here.",
        }
        for text, want in cases.items():
            with self.subTest(text=text):
                self.assertEqual(catalog.first_sentence(text), want)

    def test_escapes_pipes(self):
        block = catalog.render(self.root)
        self.assertIn(r"Use when a \| b is asked.", block)
        rows = [line for line in block.splitlines() if line.startswith("| `")]
        self.assertEqual([r.split("`")[1] for r in rows], ["/monet:alpha", "/monet:zeta"])
        self.assertNotIn("Then more", block)

    def test_marks_non_invocable_skills(self):
        self.add_skill("beta", "Use when b.", "user-invocable: false\n")
        block = catalog.render(self.root)
        self.assertIn("| `monet:beta` | Use when b. |", block)
        self.assertNotIn("/monet:beta", block)

    def test_check_fails_on_stale_block(self):
        self.assertEqual(self.run_catalog("--write").returncode, 0)
        self.assertEqual(self.run_catalog("--check").returncode, 0)
        good = self.readme.read_text(encoding="utf-8")
        self.readme.write_text(good.replace("Use when z.", "Use when edited."), encoding="utf-8")
        result = self.run_catalog("--check")
        self.assertEqual(result.returncode, 1)
        self.assertIn("Use when edited.", result.stdout + result.stderr)
        self.readme.write_text(good, encoding="utf-8")
        self.add_skill("gamma", "Use when g.")
        result = self.run_catalog("--check")
        self.assertEqual(result.returncode, 1)
        self.assertIn("/monet:gamma", result.stdout + result.stderr)

    def test_check_fails_without_markers(self):
        for text in ("# Demo\n\nNo markers.\n", f"# Demo\n{BEGIN}\nonly begin\n",
                     f"# Demo\n{END}\n{BEGIN}\n"):
            with self.subTest(text=text):
                self.readme.write_text(text, encoding="utf-8")
                for mode in ("--check", "--write"):
                    result = self.run_catalog(mode)
                    self.assertEqual(result.returncode, 1)
                    self.assertIn("marker", result.stderr)
                self.assertEqual(self.readme.read_text(encoding="utf-8"), text)
        self.readme.unlink()
        result = self.run_catalog("--check")
        self.assertEqual(result.returncode, 1)
        self.assertIn("README.md", result.stderr)

    def test_write_is_idempotent(self):
        self.assertEqual(self.run_catalog("--write").returncode, 0)
        first = self.readme.read_text(encoding="utf-8")
        self.assertTrue(first.startswith("# Demo\n\nIntro.\n\n" + BEGIN + "\n"))
        self.assertTrue(first.endswith(END + "\n\n## After\n"))
        self.assertNotIn("old", first)
        self.assertEqual(self.run_catalog("--write").returncode, 0)
        self.assertEqual(self.readme.read_text(encoding="utf-8"), first)
        self.assertEqual(self.run_catalog("--check").returncode, 0)

    def test_uses_skillkit_and_stdlib_only(self):
        source = SCRIPT.read_text(encoding="utf-8")
        self.assertIn("import skillkit", source)
        self.assertIn("skillkit.read_skill", source)
        for module in ("yaml", "frontmatter"):
            with self.subTest(module=module):
                self.assertNotRegex(source, rf"(?m)^\s*(import|from) {module}\b")


if __name__ == "__main__":
    unittest.main()
