"""Contract for README.md (DOC-1, DOC-2)."""

import json
import os
import re
import subprocess
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "tools"))
import skillkit  # noqa: E402

README = ROOT / "README.md"
MANIFEST = ROOT / ".claude-plugin" / "plugin.json"
STE_CHECK = ROOT / "skills" / "voice" / "scripts" / "ste_check.py"
SECTIONS = ["Install", "First run", "Skills", "Configuration", "Update and uninstall", "Docs"]


def read():
    return README.read_text(encoding="utf-8")


def section(name):
    m = re.search(rf"(?ms)^## {re.escape(name)}\n(.*?)(?=^## |\Z)", read())
    return m.group(1) if m else None


def fences(text):
    return re.findall(r"(?ms)^```[^\n]*\n(.*?)^```", text)


class Readme(unittest.TestCase):
    def test_at_most_150_lines(self):
        self.assertLessEqual(len(read().splitlines()), 150)

    def test_sections_in_order(self):
        text = read()
        self.assertRegex(text, r"\A# monet\n\n[^#\n][^\n]+\n\n## Install\n")
        self.assertEqual(re.findall(r"(?m)^## (.+)$", text), SECTIONS)

    def test_install_commands(self):
        body = section("Install")
        blocks = fences(body)
        self.assertEqual(len(blocks), 1)
        self.assertEqual(blocks[0].splitlines(),
                         ["/plugin marketplace add initor/monet", "/plugin install monet@monet"])
        self.assertRegex(body, r"plugin support")
        self.assertRegex(body, r"gang\b.*Go, Node, (and )?`?gh`?")

    def test_first_run_skills_exist(self):
        body = section("First run")
        examples = re.findall(r"(?m)^- `([^`]+)`: \S.*$", body)
        self.assertEqual(len(examples), 3)
        self.assertIn("/monet:help", examples)
        for example in examples:
            with self.subTest(example=example):
                if example == "claude --agent monet:monet":
                    continue
                m = re.fullmatch(r"/monet:([a-z0-9-]+)( .*)?", example)
                self.assertIsNotNone(m, "use `claude --agent monet:monet` or `/monet:<name>`")
                self.assertTrue((ROOT / "skills" / m.group(1) / "SKILL.md").is_file())

    def test_configuration_matches_manifest(self):
        body = section("Configuration")
        config = json.loads(MANIFEST.read_text(encoding="utf-8"))["userConfig"]
        rows = dict(re.findall(r"(?m)^\| `([a-z_]+)` \| ([^|\n]*?) \|", body))
        self.assertEqual(sorted(rows), ["branch_prefix", "data_dir"])
        for key, cell in rows.items():
            with self.subTest(key=key):
                default = config[key]["default"]
                if default:
                    self.assertEqual(cell, f"`{default}`")
                else:
                    self.assertEqual(cell, "empty, so no prefix")
        for command in ("/plugin configure monet@monet", "claude plugin configure monet@monet",
                        "claude plugin install monet@monet --config data_dir=<path>"):
            with self.subTest(command=command):
                self.assertIn(command, body)
        flat = " ".join(body.split())
        self.assertIn("literal placeholder", flat)
        self.assertRegex(flat, r"falls back to the default itself\. It uses no prefix for "
                               r"`branch_prefix`, and it uses `~/\.config/monet` for `data_dir`\.")

    def test_update_and_uninstall(self):
        body = section("Update and uninstall")
        update = body.find("claude plugin update monet@monet")
        self.assertGreaterEqual(update, 0)
        self.assertGreater(body.find("/reload-plugins"), update)
        self.assertIn("claude plugin uninstall monet@monet", body)
        self.assertRegex(" ".join(body.split()), r"(?i)\buninstall keeps your data dir\b")
        help_text = subprocess.run(["claude", "plugin", "--help"], capture_output=True, text=True,
                                   check=True, timeout=60).stdout
        for sub in sorted(set(re.findall(r"claude plugin ([a-z]+)", read()))):
            with self.subTest(sub=sub):
                self.assertRegex(help_text, rf"(?m)^  {sub}\b")

    def test_links_resolve(self):
        text = read()
        targets = [t.split("#")[0] for t in re.findall(r"\]\(([^)\s]+)\)", text)
                   if not re.match(r"[a-z]+:", t) and not t.startswith("#")]
        docs = section("Docs")
        for doc in ("docs/development.md", "docs/authoring.md", "docs/architecture.md",
                    "docs/design/gang.md", "docs/design/roadmap-handoff.md",
                    "CONTRIBUTING.md", "CHANGELOG.md"):
            with self.subTest(doc=doc):
                self.assertIn(f"]({doc})", docs)
        tracked = set(subprocess.run(["git", "-C", str(ROOT), "ls-files", "-z", "--cached"],
                                     capture_output=True, text=True, check=True, timeout=60,
                                     env={k: v for k, v in os.environ.items()
                                          if not k.startswith("GIT_")}).stdout.split("\0"))
        for target in targets:
            with self.subTest(target=target):
                self.assertFalse(target.startswith("/"), "use a relative link")
                self.assertIn(target, tracked)

    def test_catalog_is_current(self):
        result = subprocess.run([sys.executable, str(ROOT / "tools" / "catalog.py"), "--check"],
                                capture_output=True, text=True, check=False, timeout=60)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        rows = re.findall(r"(?m)^\| `/?monet:([a-z0-9-]+)` \|", read())
        self.assertEqual(rows, sorted(p.parent.name for p in skillkit.skills()))

    def test_names_no_dotfiles_path(self):
        self.assertEqual(skillkit.dotfiles_paths(read()), [])

    def test_prose_has_no_em_dash_or_semicolon(self):
        result = subprocess.run([sys.executable, str(STE_CHECK), str(README)],
                                capture_output=True, text=True, check=False, timeout=60)
        self.assertIn("em-dashes 0, semicolons 0", result.stdout)
        self.assertNotIn(chr(0x2014), read())


if __name__ == "__main__":
    unittest.main()
