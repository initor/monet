"""Tests for tools/skillkit.py, the helpers that skill contract tests use.

Fixtures live in temp dirs, and no test reads skills/ or import/. Dotfiles
paths are built at runtime, so skillkit.dotfiles_paths finds none in this file.
"""

import ast
import inspect
import os
import string
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

import skillkit

DOTFILES = ["~" + "/.claude/", "$HOME" + "/.claude", "${HOME}" + "/.claude",
            "~/.config" + "/.claude", "/Users/" + "alice/", "/home/" + "bob/"]
# Ruling R1, verbatim.
DATA = ("If the data dir above is empty or starts with `${user_config`, "
        "it isn't configured, so use `~/.config/monet`.")
PREFIX = ("If the prefix above is empty or starts with `${user_config`, "
          "it isn't configured, so use no prefix.")
# Each public name with the signature the module docstring must show.
SIGNATURES = {
    "REPO": "REPO -> Path",
    "skills": "skills(repo=REPO) -> list[Path]",
    "read_skill": "read_skill(path) -> Skill",
    "frontmatter": "frontmatter(text) -> dict",
    "path_refs": "path_refs(text) -> list[tuple[str, str, int]]",
    "DOTFILES_PATTERNS": "DOTFILES_PATTERNS -> tuple[re.Pattern, ...]",
    "dotfiles_paths": "dotfiles_paths(text) -> list[tuple[int, str]]",
    "files": "files(repo=REPO) -> list[str]",
    "USER_CONFIG_FALLBACKS": "USER_CONFIG_FALLBACKS -> dict[str, str]",
    "missing_fallbacks": "missing_fallbacks(text) -> list[tuple[int, str]]",
    "skill_md_refs": "skill_md_refs(text, own) -> list[tuple[int, str]]",
}


def fm(*lines):
    """Return a file text whose frontmatter block holds `lines`."""
    return "\n".join(["---", *lines, "---", "body", ""])


def write(path, text):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")


def run_git(cwd, *args):
    """Run git in `cwd`, without GIT_* variables that a git hook may export."""
    env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}
    subprocess.run(["git", "-C", str(cwd), *args], check=True, capture_output=True, env=env)


class Module(unittest.TestCase):
    def test_imports_only_the_standard_library(self):
        tree = ast.parse(Path(skillkit.__file__).read_text(encoding="utf-8"))
        names = {a.name.split(".")[0] for node in ast.walk(tree) if isinstance(node, ast.Import)
                 for a in node.names}
        names |= {node.module.split(".")[0] for node in ast.walk(tree)
                  if isinstance(node, ast.ImportFrom) and node.module and not node.level}
        self.assertIn("re", names)
        # -I -S leaves out PYTHONPATH, user site, and site-packages: only the stdlib imports.
        # Each named module imports before tools/ joins sys.path, so a file there cannot pass.
        code = "".join(f"import {name}\n" for name in sorted(names))
        code += "import sys; sys.path.insert(0, sys.argv[1]); import skillkit\n"
        run = subprocess.run([sys.executable, "-I", "-S", "-B", "-c", code,
                              str(Path(skillkit.__file__).parent)], capture_output=True, text=True)
        self.assertEqual(run.returncode, 0, run.stderr)

    def test_docstring_lists_each_public_name(self):
        doc = skillkit.__doc__
        self.assertIn("`import skillkit`", doc)
        self.assertIn("`PYTHONPATH=$(CURDIR)/tools` that `make check` sets", doc)
        for name, signature in SIGNATURES.items():
            with self.subTest(name):
                self.assertTrue(hasattr(skillkit, name))
                self.assertIn("\n" + signature + "\n", doc)
        functions = {name for name, obj in vars(skillkit).items()
                     if inspect.isfunction(obj) and obj.__module__ == "skillkit" and not name.startswith("_")}
        self.assertEqual(functions, set(SIGNATURES) - {"REPO", "DOTFILES_PATTERNS", "USER_CONFIG_FALLBACKS"})


class Frontmatter(unittest.TestCase):
    def test_scalars_and_quotes(self):
        text = fm(
            "name: demo",
            "description: Use when it's (really) needed, with \"inner\" quotes # a comment",
            "single: 'it''s # kept: yes'",
            'double: "tab\\t \\"q\\" \\u00e9 # kept"',
            'argument-hint: "[idea]"',
            "user-invocable: false",
            "disable-model-invocation: true",
            "model:",
            "agent: ~",
            'tab: "a\tb"',
            "none: null",
            "yes: True",
            "loud: TRUE",
            "version: 1.0.0",
            "digits: 3 modes, 2 tools",
            "wide:  demo",
            "tail: demo  ",
            "tailtab: Use when x.\t",
            "quiet: False",
            "shout: FALSE",
            "tabsep:\tdemo",
            "permissionMode: a",
            "when_to_use: b",
            "_x: c",
            "tabnote: Use when x\t# note",
            "runnote: Use when x   # note",
        )
        self.assertEqual(skillkit.frontmatter(text), {
            "name": "demo",
            "description": "Use when it's (really) needed, with \"inner\" quotes",
            "single": "it's # kept: yes",
            "double": "tab\t \"q\" é # kept",
            "argument-hint": "[idea]",
            "user-invocable": False,
            "disable-model-invocation": True,
            "model": None,
            "agent": None,
            "tab": "a\tb",
            "none": None,
            "yes": True,
            "loud": True,
            "version": "1.0.0",
            "digits": "3 modes, 2 tools",
            "wide": "demo",
            "tail": "demo",
            "tailtab": "Use when x.",
            "quiet": False,
            "shout": False,
            "tabsep": "demo",
            "permissionMode": "a",
            "when_to_use": "b",
            "_x": "c",
            "tabnote": "Use when x",
            "runnote": "Use when x",
        })

    def test_folded_block_scalar(self):
        text = fm(
            "description: >-", "  Use when one", "  two three.", "", "  Second paragraph.",
            "folded: >", "  a", "  b",
            "literal: |", "  line one", "    indented", "",
            "strip: |-", "  x", "  y",
            "spaced: >", "  para one", "  ", "  para two",
            "kept: |", "  a", "    ", "  b",
            "empty: |",
            "trail: |", "  a  ",
            "trailf: >-", "  a ",
            "lead: |", "  ", "  a",
            "leadf: >", "  ", "  a",
            "wide:  >-", "  a",
            "dash: |", "  ---", "  x",
            "name: demo",
        )
        self.assertEqual(skillkit.frontmatter(text), {
            "description": "Use when one two three.\nSecond paragraph.",
            "folded": "a b\n",
            "literal": "line one\n  indented\n",
            "strip": "x\ny",
            "spaced": "para one\npara two\n",
            "kept": "a\n  \nb\n",
            "empty": "",
            "trail": "a  \n",
            "trailf": "a ",
            "lead": "\na\n",
            "leadf": "\na\n",
            "wide": "a",
            "dash": "---\nx\n",
            "name": "demo",
        })

    def test_block_and_flow_sequences(self):
        text = fm(
            "allowed-tools:", "  - Read", '  - "Bash(git status *)"',
            "  - Bash(${CLAUDE_SKILL_DIR}/scripts/x.py *)",
            "paths:", "- docs/*.md", "", "- 'notes/*.md'",
            "flow: [Read, 'Grep', \"Bash(ls *)\", Bash(git log *),]",
            "empty: []",
            "pair: [a, b]",
            "q: ['a #b', \"Bash(gh pr view #*)\", Read]",
            "spaces: [ ]", "trail: [a, ]", "gaps: [a , b ]",
            "hash: [Bash(gh issue view#*), C#]",
            'esc: ["Bash(echo \\"x\\")"]',
            "ws:", "  - Read  ", "  - Grep\t",
            "wide:", "  -  Read",
            "q2: ['it''s']",
            "num: [10x faster, 2 ways]",
            "wideflow:  [a, b]",
        )
        self.assertEqual(skillkit.frontmatter(text), {
            "allowed-tools": ["Read", "Bash(git status *)", "Bash(${CLAUDE_SKILL_DIR}/scripts/x.py *)"],
            "paths": ["docs/*.md", "notes/*.md"],
            "flow": ["Read", "Grep", "Bash(ls *)", "Bash(git log *)"],
            "empty": [],
            "pair": ["a", "b"],
            "q": ["a #b", "Bash(gh pr view #*)", "Read"],
            "spaces": [], "trail": ["a"], "gaps": ["a", "b"],
            "hash": ["Bash(gh issue view#*)", "C#"],
            "esc": ['Bash(echo "x")'],
            "ws": ["Read", "Grep"],
            "wide": ["Read"],
            "q2": ["it's"],
            "num": ["10x faster", "2 ways"],
            "wideflow": ["a", "b"],
        })

    def test_comments_crlf_and_typed_scalars(self):
        # Ruled extras: YAML comments are skipped, CRLF reads as LF, and a plain scalar
        # that YAML reads as a number or a date is rejected, not read as a str.
        text = fm(
            "# a full-line comment",
            "d: > # c", "  a",
            "tools: # c", "  - x", "  # between items", "  - y",
            "flow: [a] # c",
            "  # an indented comment",
            'quoted: "q" # c',
            "glued: [a]#c",
            "nulls:", "  -", "  - a",
        )
        self.assertEqual(skillkit.frontmatter(text), {"d": "a\n", "tools": ["x", "y"], "flow": ["a"],
                                                      "quoted": "q", "glued": ["a"], "nulls": [None, "a"]})
        self.assertEqual(skillkit.frontmatter("---\r\nd: |\r\n  a\r\n---\r\n"), {"d": "a\n"})
        self.assertRejects({
            "date": (["date: 2026-10-11"], 2),
            "hex": (["v: 0x1F"], 2),
            "octal": (["v: 0o17"], 2),
            "float": (["v: 1.5"], 2),
            "negative": (["v: -1"], 2),
            "infinity": (["v: .inf"], 2),
            "negative infinity": (["v: -.inf"], 2),
            "leading-dot float": (["v: .5"], 2),
            "datetime": (["v: 2026-10-11T10:00:00Z"], 2),
            "plus sign": (["v: +1"], 2),
            "trailing-dot float": (["v: 1."], 2),
            "exponent": (["v: 1e5"], 2),
            "signed exponent": (["v: 1e-5"], 2),
            "lowercase hex": (["v: 0x1f"], 2),
            "nan": (["v: .nan"], 2),
            "capital infinity": (["v: .Inf"], 2),
            "space datetime": (["v: 2026-10-11 10:00:00"], 2),
            # One fixture for each alternative and class member of the typed-scalar regex.
            "underscore int": (["v: 1_000"], 2),
            "multi-digit int": (["v: 42"], 2),
            "capital exponent": (["v: 1.5E+3"], 2),
            "plus exponent": (["v: 1.5e+3"], 2),
            "multi-digit exponent": (["v: 1e10"], 2),
            "multi-digit leading-dot float": (["v: .25"], 2),
            "underscore hex": (["v: 0x1_f"], 2),
            "plus infinity": (["v: +.inf"], 2),
            "upper infinity": (["v: .INF"], 2),
            "camel nan": (["v: .NaN"], 2),
            "upper nan": (["v: .NAN"], 2),
            "one-digit month": (["v: 2026-1-11"], 2),
            "one-digit day": (["v: 2026-10-1"], 2),
            "lowercase t datetime": (["v: 2026-10-11t10:00:00"], 2),
        })

    def assertRejects(self, cases):
        for label, (lines, line) in cases.items():
            with self.subTest(label), self.assertRaisesRegex(ValueError, rf"^line {line}: "):
                skillkit.frontmatter(fm(*lines))

    def test_rejects_nested_map(self):
        self.assertRejects({
            "nested map": (["name: demo", "hooks:", "  PreToolUse: x"], 4),
            "inline map": (["meta: a: b"], 2),
            "flow map": (["meta: {a: b}"], 2),
            "empty flow map": (["metadata: {}"], 2),
            "flow map without a colon": (["d: {a}"], 2),
            "map in a sequence": (["tools:", "  - Bash: x"], 3),
            "anchor": (["name: &n demo"], 2),
            "alias": (["name: *n"], 2),
        })

    def test_rejects_other_yaml(self):
        self.assertRejects({
            "tag": (["name: !str demo"], 2),
            "multi-line plain": (["description: one", "  two"], 3),
            "unclosed quote": (['name: "demo'], 2),
            "unknown escape": (['name: "\\x41"'], 2),
            "unclosed flow": (["tools: [a, b"], 2),
            "nested flow": (["tools: [a, [b]]"], 2),
            "text after flow": (["tools: [a, b] c"], 2),
            "second flow": (["tools: [a] [b]"], 2),
            "comment inside flow": (["tools: [a #c, b]"], 2),
            "tab then comment inside flow": (["allowed-tools: [Read\t#Grep, Bash]"], 2),
            "comment glued to a closing double quote": (['d: "x"#c'], 2),
            "comment glued to a closing single quote": (["d: 'x'#c"], 2),
            "number": (["version: 1"], 2),
            "duplicate key": (["name: a", "name: b"], 3),
            "more-indented folded line": (["d: >", "  a", "    b"], 4),
            "keep chomping": (["d: |+", "  a"], 2),
            "tab indent": (["tools:", "\t- a"], 3),
            "uneven sequence indent": (["tools:", "  - a", "- b"], 4),
            "more-indented sequence item": (["tools:", "- a", "  - b"], 4),
            "bare dash value": (["d: -"], 2),
            "bare question mark value": (["d: ?"], 2),
            "blank flow item": (["t: [ , a]"], 2),
            "open bracket inside a plain flow item": (["t: [a[b, c]"], 2),
            "top-level sequence": (["- a"], 2),
            "less-indented block line": (["d: |", "   a", "  b"], 4),
            "code span first": (["description: `monet:voice` helper"], 2),
            "percent first": (["description: %x"], 2),
            "at sign first": (["description: @x"], 2),
            "trailing colon": (["description: Use when you need this:"], 2),
            "commented-out flow item": (["allowed-tools: [Read, #Grep]"], 2),
            "flow in a sequence": (["tools:", "  - [Read, Grep]"], 3),
            "mapping key": (["d: ? a"], 2),
            "sequence entry as a value": (["description: - Use when x"], 2),
            "comma first": (["d: ,a"], 2),
            "tab-indented block line": (["d: |", "\ta"], 3),
            "more-indented >- line": (["d: >-", "  a", "    b"], 4),
            "tab-led folded line": (["f: >", "  a", "  \tb"], 4),
            "blank-looking folded line wider than the indent": (["f: >", "  a", "    ", "  b"], 4),
            "no space after the colon": (["name:demo"], 2),
            "no space after the dash": (["tools:", "  -a"], 3),
            "text after a closing double quote": (['name: "demo" x'], 2),
            "text glued to a closing single quote": (["name: 'demo'x"], 2),
            "unquoted brace in a flow item": (["allowed-tools: [Bash(${CLAUDE_SKILL_DIR}/scripts/x.py *)]"], 2),
            "brace inside a plain flow item": (["t: [a{b}]"], 2),
            "literal indicator in a sequence": (["tools:", "  - |"], 3),
            "folded indicator in a sequence": (["tools:", "  - >"], 3),
            "literal indicator in a flow": (["t: [|]"], 2),
            "folded indicator in a flow": (["t: [>]"], 2),
            "close bracket first": (["d: ]x"], 2),
            "close brace first": (["d: }x"], 2),
            "dash then tab": (["d: -\tx"], 2),
            "question mark then tab": (["d: ?\tx"], 2),
            "colon then tab": (["d: a:\tb"], 2),
        })
        for text in ("name: demo\n", " ---\nname: a\n---\n", "----\nname: a\n---\n", "---x\nname: a\n---\n"):
            with self.subTest(text), self.assertRaisesRegex(ValueError, "^line 1: "):
                skillkit.frontmatter(text)
        for text in ("---\nname: demo\n", "---\nname: a\n----\nbody\n"):
            with self.subTest(text), self.assertRaisesRegex(ValueError, "closing ---"):
                skillkit.frontmatter(text)

    def test_empty_block(self):
        # A `---` line right after the opening one closes an empty frontmatter block.
        self.assertEqual(skillkit.frontmatter("---\n---\nbody\n"), {})
        self.assertEqual(skillkit.frontmatter("---\n---\nbody\n---\nname: x\n"), {})

    def test_rejects_tab_in_block_indent(self):
        # A tab inside a block scalar's indent is not YAML, even on a line that holds only
        # whitespace, so it raises at that line instead of reading as an empty line.
        self.assertRejects({
            "tab then spaces": (["d: |", "  a", "\t   ", "  b"], 4),
            "tab only": (["d: |", "  a", "\t", "  b"], 4),
            "space then tab": (["d: |-", "  a", " \t", "  b"], 4),
            "folded": (["d: >", "  a", "\t", "  b"], 4),
            "before the first line": (["d: |", "  \t", "  a"], 3),
            "last line": (["d: |", "  a", "\t", "name: x"], 4),
            "no-break space": (["d: |", "  a", " ", "  b"], 4),
        })
        # Spaces short of the indent, and a tab past the indent, are still YAML.
        text = fm("lit: |", "  a", " ", "  b", "fold: >", "  a", " ", "  b", "tab: |", "  a", "  \t", "  b")
        self.assertEqual(skillkit.frontmatter(text), {"lit": "a\n\nb\n", "fold": "a\nb\n", "tab": "a\n\t\nb\n"})


class ReadSkill(unittest.TestCase):
    def test_name_frontmatter_and_body(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "skills" / "demo" / "SKILL.md"
            write(path, "---\nname: demo\ndescription: Use when testing.\n---\n# Demo\n\nBody.\n")
            skill = skillkit.read_skill(path)
            self.assertEqual(skill.name, "demo")
            self.assertEqual(skill.path, path)
            self.assertEqual(skill.frontmatter, {"name": "demo", "description": "Use when testing."})
            self.assertEqual(skill.body, "# Demo\n\nBody.\n")
            # A relative path still gives the dir name.
            cwd = os.getcwd()
            os.chdir(path.parent)
            try:
                relative = skillkit.read_skill("SKILL.md")
                self.assertEqual((relative.name, relative.path), ("demo", Path("SKILL.md")))
            finally:
                os.chdir(cwd)
            for bad in ("# Demo\n", "---\nname: [a\n---\n"):
                write(path, bad)
                with self.assertRaises(ValueError) as caught:
                    skillkit.read_skill(path)
                self.assertIn(str(path), str(caught.exception))
            write(path, "---\n---\n# Demo\n")
            skill = skillkit.read_skill(path)
            self.assertEqual((skill.frontmatter, skill.body), ({}, "# Demo\n"))
            path.write_bytes(b"---\nname: d\xe9mo\n---\n")
            with self.assertRaises(ValueError) as caught:
                skillkit.read_skill(path)
            self.assertIn(str(path), str(caught.exception))

    def test_skills_are_sorted_absolute_paths(self):
        with tempfile.TemporaryDirectory() as tmp:
            for rel in ("skills/zeta/SKILL.md", "skills/alpha/SKILL.md", "skills/beta/README.md",
                        "skills/SKILL.md", "skills/alpha/templates/SKILL.md", "skills/.hidden/SKILL.md",
                        "skills/lower/skill.md"):
                write(Path(tmp) / rel, "x\n")
            root = Path(tmp).resolve()
            self.assertEqual(skillkit.skills(tmp),
                             [root / "skills/alpha/SKILL.md", root / "skills/zeta/SKILL.md"])
        # Called with no argument, it lists the repo's skills; the test pins the default
        # without reading skills/.
        self.assertIs(inspect.signature(skillkit.skills).parameters["repo"].default, skillkit.REPO)

    def test_repo_is_the_parent_of_tools(self):
        self.assertTrue(skillkit.REPO.is_absolute())
        self.assertEqual(skillkit.REPO, Path(__file__).resolve().parents[2])
        self.assertTrue((skillkit.REPO / "tools" / "skillkit.py").is_file())


class Skills(unittest.TestCase):
    def test_no_skills_dir_returns_empty(self):
        # The repo has no skills/ dir until the first port lands, and the skill lint runs first.
        with tempfile.TemporaryDirectory() as tmp:
            write(Path(tmp) / "README.md", "x\n")
            self.assertEqual(skillkit.skills(tmp), [])
            (Path(tmp) / "skills").mkdir()
            self.assertEqual(skillkit.skills(tmp), [])
        with tempfile.TemporaryDirectory() as tmp:
            # A regular file named skills is not a skills/ dir either.
            write(Path(tmp) / "skills", "x\n")
            self.assertEqual(skillkit.skills(tmp), [])


class PathRefs(unittest.TestCase):
    def test_lists_skill_dir_and_plugin_root_refs(self):
        text = ("Run `${CLAUDE_SKILL_DIR}/scripts/check.py`.\n"
                "Read ${CLAUDE_PLUGIN_ROOT}/skills/voice/reference/ste-lite.md.\n"
                "See ${CLAUDE_SKILL_DIR}/reference/<name>.md, then ${CLAUDE_PLUGIN_ROOT}/skills/gang/.\n"
                "Not ${OTHER}/x, $CLAUDE_SKILL_DIR/y, or ${CLAUDE_SKILL_DIR}/ alone.\n")
        self.assertEqual(skillkit.path_refs(text), [
            ("CLAUDE_SKILL_DIR", "scripts/check.py", 1),
            ("CLAUDE_PLUGIN_ROOT", "skills/voice/reference/ste-lite.md", 2),
            ("CLAUDE_SKILL_DIR", "reference/<name>.md", 3),
            ("CLAUDE_PLUGIN_ROOT", "skills/gang", 3),
        ])

    def test_path_stops_outside_the_class(self):
        text = ("${CLAUDE_PLUGIN_ROOT}/skills/voice/scripts/ste_check.py\n"
                "${CLAUDE_SKILL_DIR}/scripts/*.py and ${CLAUDE_SKILL_DIR}/templates/{name}.md\n"
                "${CLAUDE_SKILL_DIR}/reference/v2.md, ${CLAUDE_SKILL_DIR}/README.md\n"
                "Bash(${CLAUDE_SKILL_DIR}/scripts/x.py) '${CLAUDE_SKILL_DIR}/scripts/y.py'\n"
                "${CLAUDE_SKILL_DIR}/a,b ${CLAUDE_SKILL_DIR}/c:d ${CLAUDE_SKILL_DIR}/e\"f\n")
        self.assertEqual([(path, line) for _, path, line in skillkit.path_refs(text)], [
            ("skills/voice/scripts/ste_check.py", 1),
            ("scripts/*.py", 2),
            ("templates/{name}.md", 2),
            ("reference/v2.md", 3),
            ("README.md", 3),
            ("scripts/x.py", 4),
            ("scripts/y.py", 4),
            ("a", 5),
            ("c", 5),
            ("e", 5),
        ])
        # Every ASCII punctuation char outside the class stops the path, and each one
        # inside it does not.
        inside = "._/<>{}*-"
        outside = [c for c in map(chr, range(33, 127)) if not c.isalnum() and c not in inside]
        self.assertEqual(len(outside), 23)
        for c in outside + list(inside):
            with self.subTest(c):
                want = "a" if c in outside else "a" + c + "b"
                self.assertEqual(skillkit.path_refs("${CLAUDE_SKILL_DIR}/a" + c + "b\n"),
                                 [("CLAUDE_SKILL_DIR", want, 1)])
        # The class is ASCII: a letter or digit past ASCII stops the path too.
        for c in ("é", "ß", "٣"):
            with self.subTest(c):
                self.assertEqual(skillkit.path_refs("${CLAUDE_SKILL_DIR}/a" + c + "b\n"),
                                 [("CLAUDE_SKILL_DIR", "a", 1)])

    def test_ref_inside_allowed_tools(self):
        text = fm("allowed-tools: Bash(${CLAUDE_SKILL_DIR}/scripts/x.py *), Read",
                  "more:", "  - Bash(sh ${CLAUDE_PLUGIN_ROOT}/skills/gang/cli/run.sh:*)")
        self.assertEqual(skillkit.path_refs(text), [
            ("CLAUDE_SKILL_DIR", "scripts/x.py", 2),
            ("CLAUDE_PLUGIN_ROOT", "skills/gang/cli/run.sh", 4),
        ])


class DotfilesPaths(unittest.TestCase):
    def test_finds_each_pattern(self):
        self.assertEqual(len(skillkit.DOTFILES_PATTERNS), 6)
        text = "\n".join(f"see `{p}` here" for p in DOTFILES)
        self.assertEqual(skillkit.dotfiles_paths(text), list(enumerate(DOTFILES, 1)))
        # Two hits on one line, in reverse pattern order, come back in text order.
        both = "x\n" + DOTFILES[5] + " then " + DOTFILES[0]
        self.assertEqual(skillkit.dotfiles_paths(both), [(2, DOTFILES[5]), (2, DOTFILES[0])])
        clean = ["~/.config/monet", "/Users/", "~/.claude.json", "$HOME/.config", "/var/home",
                 "/Users/" + "$USER/", "/Users/" + "<name>/", "/home/" + "a b/", "/home/" + "~x/",
                 "/Users/" + "/x", "/home/" + "/x",
                 # A near miss on each literal dot, and a name past the ASCII class.
                 "~/_claude" + "/", "$HOME" + "/_claude", "${HOME}" + "/_claude",
                 "~/_config" + "/.claude", "~/.config" + "/_claude",
                 "/Users/" + "José/", "/home/" + "åsa/"]
        self.assertEqual(skillkit.dotfiles_paths(", ".join(clean)), [])
        # Each kind of char in the name class `[A-Za-z0-9_.-]`, under both home roots.
        names = ["first.last/", "first_last/", "ci-runner/", "Alice/", "dev01/"]
        homes = [root + name for root in ("/Users/", "/home/") for name in names]
        self.assertEqual(skillkit.dotfiles_paths(" ".join(homes)), [(1, home) for home in homes])

    def test_own_sources_hold_none(self):
        for path in (Path(skillkit.__file__), Path(__file__)):
            with self.subTest(path.name):
                self.assertEqual(skillkit.dotfiles_paths(path.read_text(encoding="utf-8")), [])


class Files(unittest.TestCase):
    def test_lists_untracked_and_skips_import(self):
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp)

            run_git(repo, "init", "-q")
            tracked = [".gitignore", "tracked.md", "sub/dir/b.py", "import/old.md", "gone.md", "import.md"]
            for rel in tracked + ["new.md", "debug.log", "import/new.md", "importer/x.md", "imports/y.md"]:
                write(repo / rel, "*.log\n" if rel == ".gitignore" else "x\n")
            run_git(repo, "add", *tracked)
            # A tracked file that an ignore rule matches comes only from the repo's own index.
            write(repo / "keep.log", "x\n")
            run_git(repo, "add", "-f", "keep.log")
            (repo / "gone.md").unlink()
            (repo / "dangling.md").symlink_to("missing.md")
            run_git(repo, "init", "-q", "nested")
            want = [".gitignore", "dangling.md", "import.md", "importer/x.md", "imports/y.md",
                    "keep.log", "new.md", "sub/dir/b.py", "tracked.md"]
            self.assertEqual(skillkit.files(repo), want)
            # A git hook exports GIT_DIR and GIT_INDEX_FILE, and files() must ignore them.
            # With the hook's empty index, keep.log would drop out.
            with mock.patch.dict(os.environ, {"GIT_DIR": str(repo / "missing"),
                                              "GIT_INDEX_FILE": str(repo / "missing-index")}):
                self.assertEqual(skillkit.files(repo), want)
        # A dir that is not a git repo raises, so a scan never passes on zero files.
        with tempfile.TemporaryDirectory() as tmp, self.assertRaises(subprocess.CalledProcessError):
            # A .git file that names no repo stops git from walking up to an outer repo.
            write(Path(tmp) / ".git", "gitdir: missing\n")
            skillkit.files(Path(tmp))
        # Called with no argument, it lists the repo's files.
        self.assertIs(inspect.signature(skillkit.files).parameters["repo"].default, skillkit.REPO)


class UserConfigFallbacks(unittest.TestCase):
    def test_map_holds_the_r1_sentences(self):
        self.assertEqual(skillkit.USER_CONFIG_FALLBACKS, {"data_dir": DATA, "branch_prefix": PREFIX})

    def test_use_followed_by_fallback_passes(self):
        wrapped = DATA.replace(", it isn't", ",\n  it isn't")
        text = ("# Demo\n\n"
                "Data lives in `${user_config.data_dir}/demo/`, logs in\n"
                "`${user_config.data_dir}/logs/`. " + wrapped + "\n\n"
                "Branches are `${user_config.branch_prefix}<slug>`. " + PREFIX + "\n")
        self.assertEqual(skillkit.missing_fallbacks(text), [])

    def test_use_without_fallback_is_reported(self):
        text = ("Use `${user_config.data_dir}`.\n\n" + DATA + "\n\n"
                "Prefix `${user_config.branch_prefix}`. " + DATA + "\n")
        self.assertEqual(skillkit.missing_fallbacks(text), [(1, "data_dir"), (5, "branch_prefix")])
        # A line that holds only whitespace is a blank line too.
        for blank in ("  ", "\t", " \r"):
            with self.subTest(repr(blank)):
                text = "Use `${user_config.data_dir}`.\n" + blank + "\n" + DATA + "\n"
                self.assertEqual(skillkit.missing_fallbacks(text), [(1, "data_dir")])

    def test_fallback_holding_a_full_placeholder_is_reported(self):
        bad = DATA.replace("`${user_config`", "`${user_config.data_dir}`")
        text = "Use `${user_config.data_dir}`. " + bad + " " + DATA + "\n"
        self.assertEqual(skillkit.missing_fallbacks(text), [(1, "data_dir")])

    def test_sentence_must_match_verbatim(self):
        # Each char of the R1 sentence counts, its final period and the dots of its path too.
        cases = {
            "prefix ends in !": ("${user_config.branch_prefix}", PREFIX[:-1] + "!", "branch_prefix"),
            "data path with X": ("${user_config.data_dir}", DATA.replace("~/.config", "~/Xconfig"), "data_dir"),
            "data ends in !": ("${user_config.data_dir}", DATA[:-1] + "!", "data_dir"),
            # The head of each sentence counts too, not only the part from `${user_config`.
            "data head altered": ("${user_config.data_dir}", DATA.replace("data dir", "data directory"),
                                  "data_dir"),
            "prefix head altered": ("${user_config.branch_prefix}",
                                    PREFIX.replace("the prefix", "the branch prefix"), "branch_prefix"),
            "data head reworded": ("${user_config.data_dir}",
                                   DATA.replace("If the data dir above is empty or", "If unset, or if it"),
                                   "data_dir"),
            "data head without empty": ("${user_config.data_dir}", DATA.replace("empty or", "unset or"),
                                        "data_dir"),
        }
        for label, (use, sentence, key) in cases.items():
            with self.subTest(label):
                self.assertEqual(skillkit.missing_fallbacks("Use `" + use + "`. " + sentence + "\n"), [(1, key)])
        # A placeholder without its `$` is not a use.
        text = "The key {user_config.data_dir} is set by Claude Code.\n"
        self.assertEqual(skillkit.missing_fallbacks(text), [])

    def test_unknown_key_is_reported(self):
        text = "Use `${user_config.theme}`. " + DATA + "\n"
        self.assertEqual(skillkit.missing_fallbacks(text), [(1, "theme")])
        # A key with a typo in case or a digit is unknown too.
        text = "Use ${user_config.dataDir}. " + DATA + "\n\nUse ${user_config.data_dir2}. " + DATA + "\n"
        self.assertEqual(skillkit.missing_fallbacks(text), [(1, "dataDir"), (3, "data_dir2")])


class MissingFallbacks(unittest.TestCase):
    def test_unknown_key_is_reported(self):
        # A key with a dash, a dot, or no name at all is unknown, so each use is reported.
        for key in ("branch-prefix", "data.dir", ""):
            with self.subTest(key=key):
                text = "Use `${user_config." + key + "}`. " + DATA + " " + PREFIX + "\n"
                self.assertEqual(skillkit.missing_fallbacks(text), [(1, key)])
        text = ("Use ${user_config.branch-prefix}. " + PREFIX + "\n\n"
                "Use ${user_config.data.dir}. " + DATA + "\n\n"
                "Use ${user_config.}.\n")
        self.assertEqual(skillkit.missing_fallbacks(text),
                         [(1, "branch-prefix"), (3, "data.dir"), (5, "")])


class SkillMdRefs(unittest.TestCase):
    def test_reports_other_skill_skill_md_paths(self):
        dotfiles = "~" + "/.claude/skills/teach/SKILL.md"
        text = "\n".join([
            "Read ${CLAUDE_PLUGIN_ROOT}/skills/voice/SKILL.md first.",
            "Or skills/git-changes/SKILL.md, then ../paint/SKILL.md.",
            "Old: " + dotfiles,
            "See [voice](../voice/SKILL.md).",
            "Joined: skills/voice/SKILL.md,skills/demo/SKILL.md;../gang/SKILL.md|x/<name>/SKILL.md",
            "Then |../teach/SKILL.md|, href=skills/paint/SKILL.md, <skills/ship/SKILL.md>.",
            "Colon: ../prep/SKILL.md:skills/demo/SKILL.md",
            "Tag: <b>skills/voice/SKILL.md</b>",
        ])
        self.assertEqual(skillkit.skill_md_refs(text, "demo"), [
            (1, "${CLAUDE_PLUGIN_ROOT}/skills/voice/SKILL.md"),
            (2, "skills/git-changes/SKILL.md"),
            (2, "../paint/SKILL.md"),
            (3, dotfiles),
            (4, "../voice/SKILL.md"),
            (5, "skills/voice/SKILL.md"),
            (5, "../gang/SKILL.md"),
            (6, "../teach/SKILL.md"),
            (6, "skills/paint/SKILL.md"),
            (6, "skills/ship/SKILL.md"),
            (7, "../prep/SKILL.md"),
            (8, "skills/voice/SKILL.md"),
        ])

    def test_ignores_own_bare_placeholder_and_non_skill_md_refs(self):
        text = "\n".join([
            "This SKILL.md and ${CLAUDE_SKILL_DIR}/SKILL.md stay.",
            "Own: ${CLAUDE_PLUGIN_ROOT}/skills/demo/SKILL.md and skills/demo/SKILL.md.",
            "Placeholders: skills/<name>/SKILL.md, skills/{name}/SKILL.md, skills/*/SKILL.md.",
            "Load monet:voice with the Skill tool.",
            "Run ${CLAUDE_PLUGIN_ROOT}/skills/voice/scripts/ste_check.py on skills/voice/reference/a.md.",
            "Not skills/voice/SKILL.mdx or skills/voice/SKILL.md.bak.",
            "Mid-segment: skills/git-<kind>/SKILL.md, skills/git-{kind}/SKILL.md, skills/git-*/SKILL.md.",
            "Not a name: skills/Voice/SKILL.md, skills/9lives/SKILL.md, skills/voice-/SKILL.md.",
            "Not a name: skills/my_skill/SKILL.md, skills/git_ship/SKILL.md, skills/v2.0/SKILL.md.",
            "Not a name: skills/gitShip/SKILL.md, skills/git-Ship/SKILL.md.",
            "Not SKILL.md: skills/voice/SKILL.md-old, skills/voice/SKILL.md_old.",
            "Not a dot: skills/voice/SKILLxmd. Not the end: skills/voice/SKILL.md/x.",
        ])
        self.assertEqual(skillkit.skill_md_refs(text, "demo"), [])

    def test_reports_angle_bracket_ref(self):
        # A `<` or `>` right before the name is markup, not part of the name.
        self.assertEqual(skillkit.skill_md_refs("See <voice/SKILL.md>.", "paint"), [(1, "voice/SKILL.md")])
        text = "See <code>voice/SKILL.md</code>.\nOr a>voice/SKILL.md."
        self.assertEqual(skillkit.skill_md_refs(text, "paint"), [(1, "voice/SKILL.md"), (2, "voice/SKILL.md")])

    def test_stop_chars_end_the_ref(self):
        # The usual Markdown and shell forms.
        cases = {
            "See voice/SKILL.md.": "voice/SKILL.md",
            "Read `voice/SKILL.md`.": "voice/SKILL.md",
            "cat 'voice/SKILL.md'": "voice/SKILL.md",
            'cat "voice/SKILL.md"': "voice/SKILL.md",
            "[x](voice/SKILL.md)": "voice/SKILL.md",
            "[voice/SKILL.md](x)": "voice/SKILL.md",
            "Read `skills/voice/SKILL.md`.": "skills/voice/SKILL.md",
            "cat 'skills/voice/SKILL.md'": "skills/voice/SKILL.md",
            'cat "skills/voice/SKILL.md"': "skills/voice/SKILL.md",
            "[skills/voice/SKILL.md](x)": "skills/voice/SKILL.md",
            "[x]../voice/SKILL.md": "../voice/SKILL.md",
            "x)../voice/SKILL.md": "../voice/SKILL.md",
            "a,skills/voice/SKILL.md": "skills/voice/SKILL.md",
        }
        for text, ref in cases.items():
            with self.subTest(text):
                self.assertEqual(skillkit.skill_md_refs(text, "paint"), [(1, ref)])
        # Each stop char ends the text before a bare name and before a longer path.
        for c in [" ", "\t", "\n", "`", "'", '"', "(", ")", "[", "]", ",", ";", "|", "=", "<", ">"]:
            line = 1 + c.count("\n")
            for ref in ("voice/SKILL.md", "skills/voice/SKILL.md", "../voice/SKILL.md"):
                with self.subTest(c=c, ref=ref):
                    self.assertEqual(skillkit.skill_md_refs("a" + c + ref, "paint"), [(line, ref)])
        # Any other char after SKILL.md ends the path, as in `SKILL.md#usage` or `**SKILL.md**`.
        for c in sorted(set(string.punctuation + " \t\n") - set("_/-.")):
            with self.subTest(after=c):
                self.assertEqual(skillkit.skill_md_refs("../voice/SKILL.md" + c + " x", "paint"),
                                 [(1, "../voice/SKILL.md")])

    def test_digit_names_and_extension_case(self):
        # A skill name may hold a digit, in its first segment or a later one.
        text = "Read skills/oauth2/SKILL.md, then ../sd-v2/SKILL.md and x/a1-b2c3/SKILL.md."
        self.assertEqual(skillkit.skill_md_refs(text, "demo"), [
            (1, "skills/oauth2/SKILL.md"),
            (1, "../sd-v2/SKILL.md"),
            (1, "x/a1-b2c3/SKILL.md"),
        ])
        # A suffix after SKILL.md, digit or uppercase, makes it another file.
        for suffix in (".1", ".BAK", ".Bak", ".2x"):
            with self.subTest(suffix=suffix):
                text = "See skills/voice/SKILL.md" + suffix + " now."
                self.assertEqual(skillkit.skill_md_refs(text, "demo"), [])


if __name__ == "__main__":
    unittest.main()
