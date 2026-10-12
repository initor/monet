"""Contract for the migration guide, docs/migration.md (DOC-6)."""

import json
import os
import re
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
DOC = ROOT / "docs" / "migration.md"
SCRIPT = ROOT / "scripts" / "migrate-from-dotfiles.sh"
STE_CHECK = ROOT / "skills" / "voice" / "scripts" / "ste_check.py"
PLUGIN = ROOT / ".claude-plugin" / "plugin.json"
README = ROOT / "README.md"

SECTIONS = ("Before you start", "Install", "Configure", "Move your data",
            "Remove the old copies", "Verify", "Update and develop")
INSTALL = ("/plugin marketplace add initor/monet", "/plugin install monet@monet")
CLONE = "git clone https://github.com/initor/monet"
FALLBACK = "~/.config/monet"  # each skill's own data_dir fallback (ruling R1)
DEV_LOOP = "development.md#the-dev-loop"
FENCE = re.compile(r"^```[^\n]*\n(.*?)^```$", re.M | re.S)
SPAN = re.compile(r"`([^`\n]+)`")
LINK = re.compile(r"\]\(([^)\s]+)\)")


def doc_text():
    return DOC.read_text(encoding="utf-8")


def section(title):
    """The body of the level 2 section with this exact heading."""
    for part in re.split(r"^## ", doc_text(), flags=re.M)[1:]:
        head, _, body = part.partition("\n")
        if head.strip() == title:
            return body
    return ""


def flat(body):
    return re.sub(r"\s+", " ", body)


def sentences(body):
    """The prose sentences of a body, with fences dropped and lines folded."""
    return re.split(r"(?<=\.) ", flat(FENCE.sub(" ", body)))


def commands(body):
    """Each line of the fenced blocks, then each inline code span, in order."""
    lines = [line for block in FENCE.findall(body) for line in block.split("\n")]
    return lines + SPAN.findall(FENCE.sub(" ", body))


def script_list(var):
    match = re.search(rf'^{var}="([^"]*)"$', SCRIPT.read_text(encoding="utf-8"), re.M)
    return match.group(1).split()


def slug(heading):
    return re.sub(r"[^a-z0-9 -]", "", heading.lower()).replace(" ", "-")


class MigrationDoc(unittest.TestCase):
    def assertSentence(self, body, *needles):
        hits = [s for s in sentences(body) if all(n in s for n in needles)]
        self.assertTrue(hits, f"no sentence holds all of {needles}")

    def test_sections_in_order(self):
        found = re.findall(r"^## (.+)$", doc_text(), re.M)
        self.assertEqual(tuple(h.strip() for h in found), SECTIONS)

    def test_before_you_start_covers_gitignore(self):
        body = section("Before you start")
        self.assertSentence(body, "session", "gang")
        self.assertSentence(body, "committed")
        self.assertSentence(body, "`.gitignore`", "`monet/`", "`--apply`", "`--backup-dir`")

    def test_install_matches_readme(self):
        lines = [c.strip() for c in commands(section("Install"))]
        for command in INSTALL:
            with self.subTest(command=command):
                self.assertIn(command, lines)
                if README.exists():
                    self.assertIn(command, README.read_text(encoding="utf-8"))

    def test_configure(self):
        raw = section("Configure")
        body = flat(raw)
        default = json.loads(PLUGIN.read_text(encoding="utf-8"))["userConfig"]["data_dir"]["default"]
        for needle in ("/plugin configure monet@monet", "claude plugin configure monet@monet",
                       "claude plugin install monet@monet --config data_dir=<path>",
                       "`branch_prefix`", "`alice/`"):
            with self.subTest(needle=needle):
                self.assertIn(needle, body)
        self.assertSentence(raw, "prefix", "old skills hard-coded")
        self.assertSentence(raw, "default in `plugin.json`", f"`{default}`")
        self.assertSentence(raw, "unset key", "literal placeholder", "even though",
                            "`plugin.json`", "`default`")
        self.assertSentence(raw, "skill falls back to the default itself")
        self.assertSentence(raw, "unset `branch_prefix` means no prefix",
                            f"unset `data_dir` means `{FALLBACK}`")
        # The branch example uses the neutral prefix only.
        examples = [s for s in SPAN.findall(raw)
                    if re.match(r"[A-Za-z0-9._-]+/", s) and not re.search(r"[\s~]", s)]
        self.assertTrue(examples, "no branch example")
        for example in examples:
            with self.subTest(example=example):
                self.assertTrue(example.startswith("alice/"), example)

    def test_says_where_the_script_comes_from(self):
        # The clone is a command line of its own, and it comes before the
        # first mention of the script anywhere in the doc.
        text = doc_text()
        self.assertTrue(any(c.strip().startswith(CLONE) for c in commands(text)),
                        "no command line starts with the clone")
        first_run = text.find("scripts/migrate-from-dotfiles.sh")
        self.assertNotEqual(first_run, -1, "the doc never runs the script")
        self.assertLess(text.find(CLONE), first_run)
        body = section("Move your data")
        self.assertSentence(body, "only for the script")
        self.assertSentence(body, "or `cd` into a checkout")
        self.assertSentence(body, "every command below from that checkout")
        self.assertSentence(body, "marketplace install from GitHub", "Claude Code loads")
        self.assertSentence(body, "persona files", "persona map `personas.md`")
        runs = [c for c in commands(body) if "scripts/migrate-from-dotfiles.sh" in c]
        apply = [i for i, c in enumerate(runs) if "--apply" in c]
        self.assertTrue(apply and apply[0] > 0, "no dry run before --apply")
        for needle in ("`--data-dir", "personas", "`~/.config/interview-prep/`"):
            with self.subTest(needle=needle):
                self.assertIn(needle, flat(body))

    def test_names_every_moved_path(self):
        body = section("Remove the old copies")
        code = "\n".join(SPAN.findall(FENCE.sub(" ", body)) + FENCE.findall(body))
        words = set(re.findall(r"[\w.<>/-]+", code))
        for name in script_list("MONET_SKILL_DIRS"):
            with self.subTest(skill=name):
                self.assertIn(name, words)
        for entry in script_list("MONET_OTHER_ENTRIES"):
            with self.subTest(entry=entry):
                self.assertIn(entry, body)
        self.assertSentence(body, "never deletes")
        self.assertIn("dotfiles-<YYYYmmdd-HHMMSS>", body)
        # Each moved entry, and the skill dirs, go from the backup back to the
        # dir that the section says the copies move out of.
        home = re.search(r"out of `([^`]+/)`", body)
        self.assertTrue(home, "the section does not name the dir the copies leave")
        # Each mv has one source, and the source is exact: the skill dirs come
        # back through the "$backup"/skills/* glob, so every dir is restored.
        moves = [c.split() for c in commands(body) if c.strip().startswith("mv ")]
        sources = {entry: rf'"\$backup/{re.escape(entry)}"|"\$backup"/{re.escape(entry)}'
                   for entry in script_list("MONET_OTHER_ENTRIES")}
        sources["skills/"] = r'"\$backup/skills"/\*|"\$backup"/skills/\*'
        for entry, source in sources.items():
            with self.subTest(restore=entry):
                self.assertIn(True, [len(m) == 3 and bool(re.fullmatch(source, m[1]))
                                     and m[2] == home.group(1) + entry for m in moves], moves)
        self.assertSentence(body, "`~/.claude` is a git checkout", "deletions")
        self.assertSentence(body, "monet-only files", "do not load")
        self.assertSentence(body, "templates", "workflow bodies", "check scripts")
        self.assertIn("claude --agent monet:monet", body)

    def test_verify(self):
        body = flat(section("Verify"))
        for needle in ("scripts/migrate-from-dotfiles.sh --check", "exits 0",
                       "/reload-plugins", "/monet:status"):
            with self.subTest(needle=needle):
                self.assertIn(needle, body)

    def test_update_and_develop(self):
        body = section("Update and develop")
        self.assertIn("claude plugin update monet@monet", body)
        self.assertIn("/reload-plugins", body)
        self.assertSentence(body, "GitHub install", "default", "update path")
        self.assertSentence(body, "dev loop", "Development")
        self.assertSentence(body, "optional")
        # A link to DOC1's steps to switch back to the GitHub marketplace.
        links = re.findall(r"\[([^\]]*)\]\(([^)\s]+)\)", body)
        self.assertIn(DEV_LOOP, [target for text, target in links if "switch back" in text])
        dev_loop = (DOC.parent / "development.md").read_text(encoding="utf-8")
        dev_loop = dev_loop.partition("## The dev loop\n")[2].split("\n## ")[0]
        self.assertIn("To switch back", dev_loop)

    def test_links_resolve(self):
        links = [link for link in LINK.findall(doc_text()) if "://" not in link]
        self.assertTrue(links, "the doc has no relative link")
        for link in links:
            with self.subTest(link=link):
                path, _, anchor = link.partition("#")
                target = (DOC.parent / path).resolve() if path else DOC
                self.assertTrue(target.is_file(), target)
                if anchor:
                    heads = re.findall(r"^#+ (.+)$", target.read_text(encoding="utf-8"), re.M)
                    self.assertIn(anchor, [slug(h) for h in heads])

    def test_options_match_script_help(self):
        used = set()
        for c in commands(doc_text()):
            if "migrate-from-dotfiles.sh" in c or c.startswith("--"):
                used |= set(re.findall(r"(?<![\w-])--[a-z][a-z-]*", c))
        self.assertTrue({"--apply", "--check", "--data-dir", "--backup-dir"} <= used, used)
        with tempfile.TemporaryDirectory() as home:
            result = subprocess.run(["bash", str(SCRIPT), "--help"], capture_output=True,
                                    text=True, check=True, timeout=60,
                                    env=dict(os.environ, HOME=home))
        listed = set(re.findall(r"(?<![\w-])--[a-z][a-z-]*", result.stdout))
        self.assertEqual(used - listed, set())

    def test_no_personal_paths(self):
        text = doc_text()
        self.assertEqual(re.findall(r"/(?:Users|home)/[A-Za-z0-9_.-]+/", text), [])

    def test_prose_has_no_em_dash_or_semicolon(self):
        result = subprocess.run([sys.executable, str(STE_CHECK), str(DOC)],
                                capture_output=True, text=True, check=False, timeout=60)
        self.assertIn("em-dashes 0, semicolons 0", result.stdout)
        self.assertNotIn(chr(0x2014), doc_text())


if __name__ == "__main__":
    unittest.main()
