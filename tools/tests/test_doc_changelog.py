"""Contract for CHANGELOG.md and CONTRIBUTING.md (DOC-9)."""

import json
import re
import subprocess
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
CHANGELOG = ROOT / "CHANGELOG.md"
CONTRIBUTING = ROOT / "CONTRIBUTING.md"
MANIFEST = ROOT / ".claude-plugin" / "plugin.json"
STE_CHECK = ROOT / "skills" / "voice" / "scripts" / "ste_check.py"
RELEASE = re.compile(r"^## \[(\d+\.\d+\.\d+)\] - \d{4}-\d{2}-\d{2}$", re.M)


def read(path):
    return path.read_text(encoding="utf-8")


def prose_is_clean(case, path):
    result = subprocess.run([sys.executable, str(STE_CHECK), str(path)],
                            capture_output=True, text=True, check=False, timeout=60)
    case.assertIn("em-dashes 0, semicolons 0", result.stdout)
    case.assertNotIn(chr(0x2014), read(path))


class Changelog(unittest.TestCase):
    def test_top_release_matches_manifest_version(self):
        text = read(CHANGELOG)
        top = RELEASE.search(text)
        self.assertIsNotNone(top, "no `## [x.y.z] - YYYY-MM-DD` heading")
        version = json.loads(read(MANIFEST))["version"]
        self.assertEqual(top.group(1), version)
        self.assertRegex(text, r"(?m)^## \[0\.1\.0\] - \d{4}-\d{2}-\d{2}$")
        unreleased = text.find("\n## [Unreleased]\n")
        self.assertGreaterEqual(unreleased, 0, "no `## [Unreleased]` heading")
        self.assertLess(unreleased, top.start())

    def test_first_release_lists_what_it_adds(self):
        text = read(CHANGELOG)
        entry = re.split(r"(?m)^## ", text[RELEASE.search(text).start():])[1]
        self.assertRegex(entry, r"(?m)^### Added$")
        for needle in ("marketplace", "`/monet:<name>`", "commands folded into skills",
                       "`git-ship`", "`ship`", "`branch_prefix`", "`data_dir`",
                       "Personas and interview-prep", "sample persona",
                       "Rewritten `distill-voice`", "`scripts/migrate-from-dotfiles.sh`",
                       "`make check`", "`make test`", "CI", "`docs/"):
            with self.subTest(needle=needle):
                self.assertIn(needle, entry)

    def test_prose_has_no_em_dash_or_semicolon(self):
        prose_is_clean(self, CHANGELOG)


class Contributing(unittest.TestCase):
    def test_links_resolve(self):
        text = read(CONTRIBUTING)
        targets = re.findall(r"\]\(([^)\s]+)\)", text)
        for doc in ("docs/development.md", "docs/authoring.md"):
            with self.subTest(doc=doc):
                self.assertIn(doc, targets)
        for target in targets:
            if re.match(r"[a-z]+:", target):
                continue
            with self.subTest(target=target):
                self.assertFalse(target.startswith("/"), "use a relative link")
                self.assertTrue((ROOT / target.split("#")[0]).exists())

    def test_short_and_names_the_rules(self):
        text = read(CONTRIBUTING)
        self.assertLessEqual(len(text.splitlines()), 40)
        flat = re.sub(r"\s+", " ", text)
        self.assertRegex(flat, r"commit subjects? follows? \[Conventional Commits\]")

    def test_make_check_passes_before_a_pull_request(self):
        text = read(CONTRIBUTING)
        section = re.search(r"(?ms)^## Before you open a pull request$(.*?)(?=^## |\Z)", text)
        self.assertIsNotNone(section, "no `## Before you open a pull request` section")
        self.assertRegex(re.sub(r"\s+", " ", section.group(1)), r"`make check`[^.]*\bpasses\b")

    def test_prose_has_no_em_dash_or_semicolon(self):
        prose_is_clean(self, CONTRIBUTING)


if __name__ == "__main__":
    unittest.main()
