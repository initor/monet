"""Tests for tools/scan.py, the portability, privacy, and prose scans.

Each scan test scans a scratch git repo, with HOME set to a temp dir and
MONET_PRIVACY_PATTERNS cleared, so no test reads the owner's patterns file.
Dotfiles paths, emails, and em-dashes are built at runtime, so this file
passes the scans with no exemption.
"""

import io
import os
import re
import shutil
import subprocess
import sys
import tempfile
import unittest
from contextlib import redirect_stderr, redirect_stdout
from pathlib import Path
from unittest import mock

import scan
import skillkit

ROOT = Path(__file__).resolve().parents[2]
ALLOW = "tools/portability-allow.txt"
# The literal name, so a typo in scan.PATTERNS_ENV fails the tests.
ENV = "MONET_PRIVACY_PATTERNS"
DASH = "\u2014"
DOTFILES = ["~" + "/.claude/", "$HOME" + "/.claude", "${HOME}" + "/.claude",
            "~/.config" + "/.claude", "/Users/" + "alice/", "/home/" + "bob/"]
PRIVATE = "pat" + "@private-mail.test"
EMAIL_HIT = "email address not on the allowed list"
# Ruling R3: the lookbehind, then the address.
LOOKBEHIND = "(?<![A-Za-z0-9._%+/@-])"
R3 = r"[A-Za-z0-9._%+-]+@[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)*\.[A-Za-z]{2,}"


def write(path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    if isinstance(data, bytes):
        path.write_bytes(data)
    else:
        path.write_text(data, encoding="utf-8")


def git(repo, *args):
    env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}
    subprocess.run(["git", "-C", str(repo), *args], check=True, capture_output=True, env=env)


def pattern_hit(n):
    return f"matches line {n} of the privacy patterns file"


def dotfiles_hit(i):
    return "dotfiles path matching " + skillkit.DOTFILES_PATTERNS[i].pattern


class ScanCase(unittest.TestCase):
    def setUp(self):
        self.tmp = Path(tempfile.mkdtemp()).resolve()
        self.addCleanup(shutil.rmtree, self.tmp)
        self.home = self.tmp / "home"
        self.home.mkdir()
        env = mock.patch.dict(os.environ, {"HOME": str(self.home)})
        env.start()
        self.addCleanup(env.stop)
        os.environ.pop(ENV, None)
        self.repo = self.tmp / "repo"
        self.repo.mkdir()
        git(self.repo, "init", "-q")
        write(self.repo / ALLOW, (ROOT / ALLOW).read_text(encoding="utf-8"))

    def scan(self, *names):
        out, err = io.StringIO(), io.StringIO()
        with redirect_stdout(out), redirect_stderr(err):
            code = scan.main(list(names), repo=self.repo)
        return code, out.getvalue().splitlines(), err.getvalue()

    def assertHits(self, name, want):
        code, out, err = self.scan(name)
        self.assertEqual((out, err), ([f"FAIL {name} {h}" for h in want] or [f"ok {name}"], ""))
        self.assertEqual(code, 1 if want else 0)


class PortabilityScan(ScanCase):
    def test_flags_each_dotfiles_pattern(self):
        # Each path sits mid-line, as it does in prose and scripts.
        write(self.repo / "notes.txt", "\n".join(["clean"] + ["cp x " + d for d in DOTFILES]) + "\n")
        write(self.repo / "clean.txt", "~/.config/monet and $HOME/.config\n")
        self.assertEqual(len(DOTFILES), len(skillkit.DOTFILES_PATTERNS))
        self.assertHits("portability", [f"notes.txt:{i + 2}: {dotfiles_hit(i)}"
                                        for i in range(len(DOTFILES))])

    def test_allowlisted_file_passes(self):
        # Not listed: a sibling, a longer name, another dir, and a listed path under another dir.
        near = ["docs/other.md", "old/specs/design.md", "specs/goal.md.bak", "tools/x.py", "x/specs/goal.md"]
        for rel in ["specs/goal.md", "specs/design.md"] + near:
            write(self.repo / rel, "intro\n" + DOTFILES[0] + "\n")
        self.assertHits("portability", [f"{rel}:2: {dotfiles_hit(0)}" for rel in near])
        # A listed path in another case is not listed (a rename, so a case-insensitive disk works too).
        (self.repo / "specs" / "goal.md").rename(self.repo / "specs" / "GOAL.md")
        self.assertHits("portability", [f"{rel}:2: {dotfiles_hit(0)}" for rel in sorted(["specs/GOAL.md"] + near)])

    def test_skips_import_and_checks_untracked(self):
        write(self.repo / ".gitignore", "*.log\n")
        for rel in ("import/old.md", "tracked.txt", "ignored.log", "new.txt"):
            write(self.repo / rel, DOTFILES[4] + "\n")
        git(self.repo, "add", ".gitignore", "import/old.md", "tracked.txt")
        self.assertHits("portability", [f"{rel}:1: {dotfiles_hit(4)}" for rel in ("new.txt", "tracked.txt")])

    def test_flags_absolute_symlink(self):
        outside = self.tmp / "outside.md"
        write(outside, f"{DOTFILES[0]} {PRIVATE} {DASH}\n")
        os.symlink(str(outside), self.repo / "abs.md")
        os.symlink("/nonexistent/target", self.repo / "dangling")
        os.symlink("../outside.md", self.repo / "rel.md")
        # The allowlist exempts text, not an absolute symlink.
        (self.repo / "specs").mkdir()
        os.symlink(str(outside), self.repo / "specs" / "goal.md")
        self.assertHits("portability", [f"{rel}:1: symlink to an absolute path"
                                        for rel in ("abs.md", "dangling", "specs/goal.md")])
        # No scan reads a symlink's content.
        self.assertHits("privacy", [])
        self.assertHits("prose", [])


class PrivacyScan(ScanCase):
    def test_flags_private_email(self):
        lines = ["clean", "Mail " + PRIVATE + " today", "<" + "first.last+tag" + "@sub.corp-mail.test>",
                 PRIVATE + ", " + "sam" + "@private-mail.test", "noreply-team" + "@corp.test",
                 "alice" + "@notexample.com", "git" + "@gitlab.com", "bob" + "@example.com.evil.test",
                 "x" + "@example.co", "git" + "@github.com, " + PRIVATE, "octocat" + "@github.com",
                 "pat.noreply" + "@corp.test"]
        write(self.repo / "notes.txt", "\n".join(lines) + "\n")
        self.assertEqual(scan.EMAIL.pattern, LOOKBEHIND + R3)
        self.assertHits("privacy", [f"notes.txt:{n}: {EMAIL_HIT}" for n in range(2, 13)])

    def test_allows_noreply_and_example(self):
        allowed = ["noreply" + "@anthropic.com", "NoReply" + "@corp.test",
                   "12345+someone" + "@users.noreply.github.com", "alice" + "@example.com",
                   "bob" + "@example.org", "carol" + "@Example.NET", "git" + "@github.com"]
        write(self.repo / "notes.md", "\n".join(allowed) + "\n")
        self.assertHits("privacy", [])

    def test_ignores_url_userinfo_and_scp_remote(self):
        lines = ["https://user" + "@github.com/initor/monet", "ssh://git" + "@github.com/initor/monet.git",
                 "git" + "@github.com:initor/monet.git", "remote " + "git" + "@github.com",
                 "https://x-access-token" + "@github.com/o/r.git"]
        write(self.repo / "notes.md", "\n".join(lines) + "\n")
        self.assertHits("privacy", [])

    def test_ignores_package_version_specs(self):
        lines = ["npm install -g @anthropic-ai/claude-code@2.1.296", "@anthropic-ai/claude-code@2.1.296",
                 "/plugin install monet@monet", "monet@monet", "npx playwright-core@1",
                 "playwright-core@1.48.2"]
        write(self.repo / "notes.md", "\n".join(lines) + "\n")
        self.assertHits("privacy", [])

    def test_extra_patterns_from_env_file(self):
        write(self.repo / "notes.txt", "Project Zephyrine\nclean\nQuillion and Zephyrine\nzephyrine\n")
        env_file = self.tmp / "patterns"
        # The comment (BOM first, no space after `#`) would match line 3 if read as a regex.
        # Regexes are case-sensitive, so line 4 is clean.
        write(env_file, "\ufeff  #terms: Zephyr|Quill\n\nZephyr\\w+\n  \n  Quill(ion)?  \n")
        default = self.home / ".config" / "monet" / "privacy-patterns"
        write(default, "Quill\n")
        os.environ[ENV] = str(env_file)
        self.assertHits("privacy", ["notes.txt:1: " + pattern_hit(3), "notes.txt:3: " + pattern_hit(3),
                                    "notes.txt:3: " + pattern_hit(5)])
        # Empty, then unset, the default file under HOME applies.
        os.environ[ENV] = ""
        self.assertHits("privacy", ["notes.txt:3: " + pattern_hit(1)])
        del os.environ[ENV]
        self.assertHits("privacy", ["notes.txt:3: " + pattern_hit(1)])
        default.unlink()
        self.assertHits("privacy", [])
        # A missing $MONET_PRIVACY_PATTERNS file or an invalid regex exits 2; only privacy loads them.
        deep = "(" * 1000 + "a" + ")" * 1000
        for name, text in (("missing", None), ("huge", "ok\nx{4294967296}\n"), ("deep", deep + "\n"),
                           ("bad", "ok\n(unclosed\n")):
            if text is not None:
                write(self.tmp / name, text)
            os.environ[ENV] = str(self.tmp / name)
            code, out, err = self.scan()
            self.assertEqual((code, out), (2, []))
            self.assertIn(str(self.tmp / name), err)
            self.assertEqual(self.scan("prose")[0], 0)
        self.assertIn("line 2", err)
        os.environ.pop(ENV)
        write(default, "(unclosed\n")
        self.assertEqual(self.scan("privacy")[0], 2)

    def test_hit_does_not_print_term(self):
        write(self.repo / "notes.txt", "clean\nProject Zephyrine with " + PRIVATE + "\n")
        patterns = self.tmp / "patterns"
        write(patterns, "Zephyr\\w+\n")
        os.environ[ENV] = str(patterns)
        code, out, err = self.scan("privacy")
        self.assertEqual((code, out, err), (1, ["FAIL privacy notes.txt:2: " + EMAIL_HIT,
                                                "FAIL privacy notes.txt:2: " + pattern_hit(1)], ""))
        # An error, or a re warning (a late flag warns on Python 3.9 and 3.10, a nested set
        # warns on all), exits 2 and never quotes the regex.
        for bad in ("Zephyr\\w+(", "Zephyrine(?i)", "Zephyr[[x]"):
            write(patterns, bad + "\n")
            code, out, err = self.scan("privacy")
            self.assertEqual((code, out), (2, []))
            for term in ("Zephyr", "private-mail", "pat@"):
                self.assertNotIn(term, err)


class ProseScan(ScanCase):
    def test_flags_em_dash_in_markdown(self):
        for rel in ("a.md", "B.MD", "docs/c.Md"):
            write(self.repo / rel, f"clean -- and \u2013\nnow {DASH} here {DASH}\n")
        self.assertHits("prose", [f"{rel}:2: em-dash (U+2014)" for rel in ("B.MD", "a.md", "docs/c.Md")])

    def test_ignores_non_markdown(self):
        for rel in ("a.txt", "b.py", "c.mdx", "d.md.txt", "e.markdown", "md", "READMEmd"):
            write(self.repo / rel, f"x {DASH} y\n")
        self.assertHits("prose", [])


class Cli(ScanCase):
    def test_runs_all_or_named_scans(self):
        write(self.repo / "a.md", "clean\n")
        self.assertEqual(self.scan(), (0, ["ok portability", "ok privacy", "ok prose"], ""))
        self.assertEqual(self.scan("prose", "portability", "prose"), (0, ["ok prose", "ok portability"], ""))
        write(self.repo / "b.md", DASH + "\n")
        # A failing scan before a clean one still exits 1, and the clean one still prints ok.
        self.assertEqual(self.scan("prose", "portability"),
                         (1, ["FAIL prose b.md:1: em-dash (U+2014)", "ok portability"], ""))
        self.assertEqual(self.scan()[:2],
                         (1, ["ok portability", "ok privacy", "FAIL prose b.md:1: em-dash (U+2014)"]))
        code, out, err = self.scan("prose", "spelling")
        self.assertEqual((code, out), (2, []))
        self.assertIn("unknown scan 'spelling'", err)

    def test_subprocess_runs_only_named_scans(self):
        # Through the real sys.argv, in a scratch copy of the tools: a privacy hit shows if privacy runs.
        for rel in ("tools/scan.py", "tools/skillkit.py"):
            write(self.repo / rel, (ROOT / rel).read_text(encoding="utf-8"))
        write(self.repo / "notes.txt", PRIVATE + "\n")
        write(self.repo / "b.md", DASH + "\n")
        env = {k: v for k, v in os.environ.items() if not k.startswith(("GIT_", "PYTHON"))}

        def cli(*names):
            result = subprocess.run([sys.executable, "tools/scan.py", *names], cwd=self.repo, env=env,
                                    capture_output=True, text=True)
            return result.returncode, result.stdout.splitlines(), result.stderr

        self.assertEqual(cli("prose", "portability"),
                         (1, ["FAIL prose b.md:1: em-dash (U+2014)", "ok portability"], ""))
        self.assertEqual(cli("portability"), (0, ["ok portability"], ""))

    def test_skips_binary_files(self):
        hit = f"{DOTFILES[0]} {PRIVATE} {DASH}".encode("utf-8")
        write(self.repo / "blob.md", b"\0" + hit)
        write(self.repo / "edge.md", b"x" * 8191 + b"\0\n" + hit)
        write(self.repo / "late.md", b"x" * 8192 + b"\0\n" + hit)
        write(self.repo / "latin.md", b"caf\xe9\n" + hit)
        for name, reason in (("portability", dotfiles_hit(0)), ("privacy", EMAIL_HIT),
                             ("prose", "em-dash (U+2014)")):
            self.assertHits(name, [f"late.md:2: {reason}", f"latin.md:2: {reason}"])


class Sources(unittest.TestCase):
    def test_allowlist_holds_the_two_specs(self):
        self.assertEqual(scan.allowlist(ROOT), {"specs/goal.md", "specs/design.md"})
        lines = (ROOT / ALLOW).read_text(encoding="utf-8").splitlines()
        for i, line in enumerate(lines):
            if line.strip() and not line.startswith("#"):
                self.assertTrue(lines[i - 1].startswith("# "), f"no reason above {line}")

    def test_own_sources_hold_no_literal_hits(self):
        loose = re.compile(R3)
        for path in (ROOT / "tools" / "scan.py", Path(__file__)):
            text = path.read_text(encoding="utf-8")
            self.assertEqual(skillkit.dotfiles_paths(text), [], path)
            self.assertIsNone(loose.search(text), path)
            self.assertNotIn(DASH, text, path)


class MakeScan(unittest.TestCase):
    def test_make_scan_names_each_hit(self):
        # A space and an apostrophe in the path catch shell quoting bugs.
        tmp = Path(tempfile.mkdtemp(prefix="owner's dir ")).resolve()
        self.addCleanup(shutil.rmtree, tmp)
        tree = tmp / "tree"
        for rel in ("Makefile", "tools/scan.py", "tools/skillkit.py", ALLOW):
            write(tree / rel, (ROOT / rel).read_text(encoding="utf-8"))
        git(tree, "init", "-q")
        write(tree / "scan", "")  # A file named like the target: only .PHONY keeps the rule running.
        write(tree / "notes.md", "\n".join([DOTFILES[4], PRIVATE, DASH]) + "\n")
        env = {k: v for k, v in os.environ.items() if not k.startswith(("GIT_", "PYTHON", "MAKE", "MFLAGS"))
               and k not in ("GNUMAKEFLAGS", ENV)}
        env["HOME"] = str(tmp)

        def make(*args):
            return subprocess.run(["make", *args], cwd=tree, env=env, capture_output=True, text=True)

        result = make("scan")
        self.assertNotEqual(result.returncode, 0)
        for hit in ("FAIL portability notes.md:1: ", "FAIL privacy notes.md:2: ", "FAIL prose notes.md:3: "):
            self.assertIn(hit, result.stdout)
        (tree / "notes.md").unlink()
        result = make("scan")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("ok portability\nok privacy\nok prose\n", result.stdout)
        self.assertEqual(list(tree.rglob("__pycache__")), [])
        dry = make("-n", "check", "MAKE=:").stdout
        self.assertGreater(dry.find("python3 tools/scan.py"), dry.find("==> unittest"), dry)
        self.assertGreater(dry.find("==> unittest"), -1, dry)


if __name__ == "__main__":
    unittest.main()
