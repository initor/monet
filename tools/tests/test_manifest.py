"""Contract tests for the plugin manifests, the Makefile, and the repo's top-level files.

`make check` also runs `claude plugin validate --strict` on the manifests.
These tests pin the values that the validator doesn't check, and the
Makefile's structure and failure paths, which the gate's own green run
never exercises.
"""

import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
PLUGIN_JSON = ROOT / ".claude-plugin" / "plugin.json"
MARKETPLACE_JSON = ROOT / ".claude-plugin" / "marketplace.json"
REPO_URL = "https://github.com/initor/monet"

# One path per required .gitignore entry, each of which git must ignore (FND-10).
IGNORED_PATHS = (
    ".planning/x",
    ".claude/worktrees/x",
    "skills/gang/cli/gang",
    "tools/__pycache__/x",
    ".DS_Store",
)


def load_json(path):
    with path.open(encoding="utf-8") as f:
        return json.load(f)


def copyright_holder():
    """Return the holder named on the LICENSE line `Copyright (c) <year> <holder>`."""
    text = (ROOT / "LICENSE").read_text(encoding="utf-8")
    match = re.search(r"^Copyright \(c\) \d{4} (.+?)\s*$", text, re.MULTILINE)
    if match is None:
        raise AssertionError("LICENSE has no 'Copyright (c) <year> <holder>' line")
    return match.group(1)


def find_key(value, key):
    """Return True when `key` is a mapping key anywhere inside `value`."""
    if isinstance(value, dict):
        return key in value or any(find_key(v, key) for v in value.values())
    if isinstance(value, list):
        return any(find_key(v, key) for v in value)
    return False


class PluginManifest(unittest.TestCase):
    def setUp(self):
        self.manifest = load_json(PLUGIN_JSON)

    def test_required_fields(self):
        m = self.manifest
        self.assertEqual(m.get("name"), "monet")
        # The version lives here only; test_doc_changelog checks it against the newest release.
        self.assertRegex(m.get("version", ""), r"\A\d+\.\d+\.\d+\Z")
        self.assertEqual(m.get("license"), "MIT")
        self.assertEqual(m.get("homepage"), REPO_URL)
        self.assertEqual(m.get("repository"), REPO_URL)

        description = m.get("description")
        self.assertIsInstance(description, str)
        self.assertTrue(description.strip(), "description is empty")
        # One sentence: it ends with a period and holds no other sentence end.
        self.assertTrue(description.endswith("."), description)
        self.assertNotRegex(description[:-1], r"[.!?](\s|$)")

        keywords = m.get("keywords")
        self.assertIsInstance(keywords, list)
        self.assertTrue(keywords, "keywords is empty")
        for keyword in keywords:
            self.assertIsInstance(keyword, str)
            self.assertTrue(keyword.strip(), "a keyword is empty")

        author = m.get("author")
        self.assertIsInstance(author, dict)
        self.assertEqual(author.get("name"), copyright_holder())
        self.assertNotIn("email", author)

    def test_user_config_keys_and_defaults(self):
        config = self.manifest.get("userConfig")
        self.assertIsInstance(config, dict)
        self.assertEqual(sorted(config), ["branch_prefix", "data_dir"])

        expected_defaults = {"branch_prefix": "", "data_dir": "~/.config/monet"}
        for key, default in expected_defaults.items():
            with self.subTest(key=key):
                entry = config[key]
                self.assertEqual(entry.get("type"), "string")
                self.assertIn("default", entry)
                self.assertEqual(entry["default"], default)
                for field in ("title", "description"):
                    self.assertIsInstance(entry.get(field), str)
                    self.assertTrue(entry[field].strip(), f"{key}.{field} is empty")

        self.assertIn("<prefix><slug>", config["branch_prefix"]["description"])

    def test_user_config_entry_keys_exact(self):
        config = self.manifest.get("userConfig")
        self.assertIsInstance(config, dict)
        self.assertTrue(config, "userConfig is empty")
        for key, entry in config.items():
            with self.subTest(key=key):
                self.assertIsInstance(entry, dict)
                # Claude Code does not substitute a sensitive value into skill
                # content, so a skill would never see the value (FND-2).
                self.assertNotIn("sensitive", entry)
                self.assertEqual(sorted(entry), ["default", "description", "title", "type"])


class Marketplace(unittest.TestCase):
    def setUp(self):
        self.marketplace = load_json(MARKETPLACE_JSON)

    def test_single_plugin_entry_without_version(self):
        m = self.marketplace
        self.assertEqual(m.get("name"), "monet")
        self.assertEqual(m.get("owner", {}).get("name"), "initor")

        plugins = m.get("plugins")
        self.assertIsInstance(plugins, list)
        self.assertEqual(len(plugins), 1)
        self.assertEqual(plugins[0].get("name"), "monet")
        self.assertEqual(plugins[0].get("source"), ".")

        # The version lives in plugin.json only (design decision 6).
        self.assertFalse(find_key(m, "version"), "marketplace.json has a version key")


class RepoFiles(unittest.TestCase):
    def test_gitignore_entries(self):
        lines = (ROOT / ".gitignore").read_text(encoding="utf-8").splitlines()
        for entry in (
            ".planning/",
            ".claude/worktrees/",
            "skills/gang/cli/gang",
            "__pycache__/",
            ".DS_Store",
        ):
            with self.subTest(entry=entry):
                self.assertIn(entry, lines)

    def test_gitignore_ignores_each_entry(self):
        gitignore = ROOT / ".gitignore"
        lines = gitignore.read_text(encoding="utf-8").splitlines()
        # Git uses the last matching pattern, so a negation can cancel an entry.
        self.assertEqual([l for l in lines if l.startswith("!")], [])

        # Ask git in a scratch repo that holds only this .gitignore and reads
        # no global, system, or caller config, so the owner's global excludes
        # or the checkout's info/exclude cannot mask a missing entry.
        git = shutil.which("git")
        self.assertIsNotNone(git, "git is not on PATH")
        scratch = Path(tempfile.mkdtemp()).resolve()
        self.addCleanup(shutil.rmtree, scratch)
        env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}
        env.update(HOME=str(scratch), XDG_CONFIG_HOME=str(scratch), GIT_CONFIG_NOSYSTEM="1")
        repo = scratch / "repo"
        subprocess.run(
            [git, "init", "-q", "--template=", str(repo)], env=env, check=True, capture_output=True
        )
        shutil.copy(gitignore, repo / ".gitignore")
        for path in IGNORED_PATHS:
            with self.subTest(path=path):
                result = subprocess.run(
                    [git, "check-ignore", "-q", "--no-index", path],
                    cwd=repo, env=env, capture_output=True, text=True,
                )
                self.assertEqual(
                    result.returncode, 0, f"git does not ignore {path}: {result.stderr}"
                )

    def test_license_is_mit(self):
        lines = (ROOT / "LICENSE").read_text(encoding="utf-8").splitlines()
        self.assertTrue(lines, "LICENSE is empty")
        self.assertEqual(lines[0], "MIT License")
        self.assertTrue(copyright_holder())


def makefile_text():
    return (ROOT / "Makefile").read_text(encoding="utf-8")


def makefile_lines():
    """Return the Makefile's lines that are not comments."""
    return [l for l in makefile_text().splitlines() if not l.lstrip().startswith("#")]


class MakefileText(unittest.TestCase):
    STEPS = ("prereqs", "manifest", "catalog", "unittest")

    def test_check_and_test_structure(self):
        lines = makefile_lines()
        self.assertIn("check: " + " ".join(self.STEPS), lines)
        self.assertIn("test: check", lines)
        self.assertIn(".NOTPARALLEL:", lines)

        phony = set()
        for line in lines:
            if line.startswith(".PHONY:"):
                phony.update(line[len(".PHONY:"):].split())
        for step in ("check", "test") + self.STEPS:
            with self.subTest(step=step):
                self.assertIn(step, phony)

        # check and test only collect steps: no rule of theirs has a recipe.
        # Make attaches a tab line to the rule above it across blank and
        # comment lines, so skip those before looking for the tab.
        raw = makefile_text().splitlines()
        for i, line in enumerate(raw):
            if not re.match(r"(check|test)\s*:", line):
                continue
            with self.subTest(rule=line):
                self.assertNotIn(";", line, "inline recipe")
                j = i
                while raw[j].endswith("\\") and j + 1 < len(raw):
                    j += 1
                j += 1
                while j < len(raw) and (not raw[j].strip() or raw[j].startswith("#")):
                    j += 1
                following = raw[j] if j < len(raw) else ""
                self.assertFalse(following.startswith("\t"), f"recipe line {following!r}")

    def test_header_explains_appending_a_step(self):
        header = []
        for line in makefile_text().splitlines():
            if line and not line.startswith("#"):
                break
            header.append(line)
        header = "\n".join(header)
        for phrase in ("check: <step>", "test: <step>", ".PHONY: <step>", "end of this file"):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, header)

    def test_make_381_features_only(self):
        newer_anywhere = (
            r"\.ONESHELL|\.RECIPEPREFIX|\.SHELLFLAGS|\.EXTRA_PREREQS|\.WAIT\b"
            r"|\.NOTINTERMEDIATE|[$][({](file|let|intcmp|guile)\s"
        )
        # $(error) works under 3.81, but outside a recipe it runs at parse
        # time, and the claude check must be the prereqs step instead.
        newer_in_rules = (
            r"^\s*[A-Za-z_][A-Za-z0-9_.-]*\s*(!=|:::=|::=)"
            r"|^\s*(undefine|private|-?load)\b|:\s*(export\s+)?private\b"
            r"|^\s*define\s+\S+\s*(=|:=|::=|:::=|\+=|\?=|!=)\s*$"
            r"|&::?|[$][({]error\s"
        )
        for line in makefile_lines():
            with self.subTest(line=line):
                self.assertNotRegex(line, newer_anywhere)
                if not line.startswith("\t"):
                    self.assertNotRegex(line, newer_in_rules)


# A fixture suite that passes only when the unittest step sets the environment
# the brief names. fixture_helper is importable only through PYTHONPATH.
ENV_SUITE = """\
import os
import unittest
from pathlib import Path

import fixture_helper

class Env(unittest.TestCase):
    def test_env(self):
        self.assertEqual(fixture_helper.VALUE, 1)
        self.assertEqual(os.environ.get("PYTHONDONTWRITEBYTECODE"), "1")
        self.assertEqual(Path(os.environ["PYTHONPATH"]).resolve(), Path({tools!r}))
"""

# Scan input, not Python. Discovery must skip it, because its name does not
# match test_*.py. It is written as broken_fixture.py (caught by -p '*.py')
# and as testdata.py (caught by unittest's default pattern test*.py).
BROKEN_FIXTURE = "this is not python (\n"

FAILING_SUITE = """\
import unittest

class Fails(unittest.TestCase):
    def test_fails(self):
        self.fail("scratch failure")
"""

# Gives check and test a recipe from a second makefile. Make warns when it
# replaces a recipe and stops on a ":" and "::" mix, so this probe catches a
# recipe on check or test in any rule form, at parse time, running nothing.
RECIPE_PROBE = """\
check: ; @:
test: ; @:
recipe-probe: ; @:
"""

CATALOG_STUB = """\
import os
import sys
print("catalog stub ran with", sys.argv[1:])
print("catalog env", os.environ.get("PYTHONPATH"), os.environ.get("PYTHONDONTWRITEBYTECODE"))
sys.exit({code})
"""

# Logs each call. Exits 1, like a failed validation, when its last argument
# equals CLAUDE_STUB_FAIL_ON.
CLAUDE_STUB = """\
printf '%s\\n' "$*" >> "{log}"
for last; do :; done
if [ -n "$CLAUDE_STUB_FAIL_ON" ] && [ "$last" = "$CLAUDE_STUB_FAIL_ON" ]; then
  echo "stub claude: $last is not valid" >&2
  exit 1
fi
"""


class MakefileBehavior(unittest.TestCase):
    """Runs a copy of the Makefile in a scratch tree, with a stub claude.

    The scratch tree holds only the Makefile and fixture suites, so these
    tests never re-enter the repo's own gate. A test runs `make check` only
    where check stops before its last step, and names the steps or uses
    `make -n` otherwise, so a step that a later task appends after unittest
    never runs here. Under -n, make still runs a recipe line that holds
    $(MAKE) (the -n tests set MAKE to a no-op for it) or starts with `+`, so
    an appended step must not start a recipe line with `+`.
    """

    def setUp(self):
        # A space and an apostrophe in the path catch shell quoting bugs.
        scratch = Path(tempfile.mkdtemp(prefix="owner's dir ")).resolve()
        self.addCleanup(shutil.rmtree, scratch)
        self.tree = scratch / "tree"
        self.tree.mkdir()
        shutil.copy(ROOT / "Makefile", self.tree / "Makefile")
        tools = self.tree / "tools"
        suite = ENV_SUITE.format(tools=str(tools))
        self.write(tools / "fixture_helper.py", "VALUE = 1\n")
        self.suites = ["tools/tests", "skills/alpha/tests", "skills/beta/tests"]
        for tests in self.suites:
            self.write(self.tree / tests / "test_env.py", suite)
            for name in ("broken_fixture.py", "testdata.py"):
                self.write(self.tree / tests / name, BROKEN_FIXTURE)

        self.make = shutil.which("make")
        real = {"git": shutil.which("git"), "python3": sys.executable}
        self.assertIsNotNone(self.make, "make is not on PATH")
        self.assertIsNotNone(real["git"], "git is not on PATH")

        # Each PATH holds only these wrappers, so a real claude cannot leak in.
        self.claude_log = scratch / "claude.log"
        self.bins = {}
        for name, tools_on_path in (
            ("full", ("git", "python3", "claude")),
            ("no-claude", ("git", "python3")),
            ("no-git", ("python3", "claude")),
            ("no-python3", ("git", "claude")),
        ):
            bin_dir = scratch / ("bin-" + name)
            for tool in tools_on_path:
                if tool == "claude":
                    body = CLAUDE_STUB.format(log=self.claude_log)
                else:
                    body = f'exec "{real[tool]}" "$@"\n'
                self.write(bin_dir / tool, "#!/bin/sh\n" + body, mode=0o755)
            self.bins[name] = str(bin_dir)

    @staticmethod
    def write(path, text, mode=None):
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8")
        if mode is not None:
            path.chmod(mode)

    def run_make(self, *args, bin_name="full", extra_env=None):
        # Drop the outer make's flags and the outer gate's Python settings, so
        # the Makefile under test must set the Python environment itself.
        env = {
            k: v
            for k, v in os.environ.items()
            if k not in ("MAKEFLAGS", "MFLAGS", "MAKELEVEL", "MAKEFILES")
            and not k.startswith("PYTHON")
        }
        env.pop("CLAUDE_STUB_FAIL_ON", None)
        env.update(extra_env or {})
        env["PATH"] = self.bins[bin_name]
        return subprocess.run(
            [self.make, *args], cwd=self.tree, env=env, capture_output=True, text=True
        )

    def assert_in_order(self, text, markers):
        positions = [text.find(m) for m in markers]
        self.assertNotIn(-1, positions, f"a step is missing in:\n{text}")
        self.assertEqual(positions, sorted(positions), f"steps out of order in:\n{text}")

    def claude_calls(self):
        if not self.claude_log.exists():
            return []
        return self.claude_log.read_text(encoding="utf-8").splitlines()

    # A caller env that already sets both variables, which the steps must
    # override, not keep or extend.
    CALLER_PYTHON_ENV = {"PYTHONPATH": "/nonexistent/elsewhere", "PYTHONDONTWRITEBYTECODE": ""}

    def test_steps_pass_with_the_brief_environment(self):
        result = self.run_make(
            "prereqs", "manifest", "catalog", "unittest", extra_env=self.CALLER_PYTHON_ENV
        )
        out = result.stdout
        self.assertEqual(result.returncode, 0, out + result.stderr)
        self.assert_in_order(out, ["==> prereqs", "==> manifest", "==> catalog", "==> unittest"])
        self.assertEqual(
            self.claude_calls(),
            [
                "plugin validate --strict .claude-plugin/plugin.json",
                "plugin validate --strict .",
            ],
        )
        self.assertIn("==> catalog: skipped, tools/catalog.py is absent", out)
        self.assert_in_order(out, ["==> unittest: " + d + "\n" for d in self.suites])
        self.assertEqual(list(self.tree.rglob("__pycache__")), [])

    def test_unittest_passes_without_a_skills_dir(self):
        shutil.rmtree(self.tree / "skills")
        result = self.run_make("unittest")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("==> unittest: tools/tests\n", result.stdout)
        self.assertNotIn("skills", result.stdout + result.stderr)

    def test_check_and_test_have_no_recipe(self):
        self.write(self.tree / "probe.mk", RECIPE_PROBE)
        result = self.run_make("-f", "Makefile", "-f", "probe.mk", "recipe-probe")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertNotRegex(result.stderr, r"for target .(check|test)'")

    def test_check_and_test_run_every_step_in_order(self):
        for goal in ("check", "test"):
            with self.subTest(goal=goal):
                result = self.run_make("-n", goal, "MAKE=:")
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assert_in_order(
                    result.stdout,
                    [
                        "==> prereqs",
                        "claude plugin validate --strict .claude-plugin/plugin.json\n",
                        "claude plugin validate --strict .\n",
                        "==> catalog",
                        "==> unittest",
                    ],
                )

    def test_failing_suite_fails_check_after_every_suite_runs(self):
        for failing in ("tools/tests", "skills/alpha/tests"):
            self.write(self.tree / failing / "test_fails.py", FAILING_SUITE)
        result = self.run_make("check")
        self.assertNotEqual(result.returncode, 0)
        self.assert_in_order(result.stdout, ["==> unittest: " + d + "\n" for d in self.suites])
        self.assertIn(
            "error: unittest suites failed: tools/tests skills/alpha/tests\n", result.stderr
        )

    def test_catalog_runs_only_when_present(self):
        catalog = self.tree / "tools" / "catalog.py"
        self.write(catalog, CATALOG_STUB.format(code=0))
        result = self.run_make("catalog", extra_env=self.CALLER_PYTHON_ENV)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("catalog stub ran with ['--check']", result.stdout)
        self.assertIn(f"catalog env {self.tree / 'tools'} 1\n", result.stdout)

        self.write(catalog, CATALOG_STUB.format(code=1))
        result = self.run_make("check")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("catalog stub ran with ['--check']", result.stdout)
        self.assertNotIn("==> unittest", result.stdout)

    def test_missing_claude_fails_at_prereqs(self):
        for goal in ("prereqs", "check"):
            with self.subTest(goal=goal):
                result = self.run_make(goal, bin_name="no-claude")
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("the claude CLI", result.stderr)
                self.assertNotIn("==> manifest", result.stdout)

    def test_missing_git_or_python3_fails_at_prereqs(self):
        for tool in ("git", "python3"):
            with self.subTest(tool=tool):
                result = self.run_make("prereqs", bin_name="no-" + tool)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn(f"'{tool}' is not on PATH", result.stderr)

    def test_failing_validate_fails_check_at_manifest(self):
        for target in (".claude-plugin/plugin.json", "."):
            with self.subTest(target=target):
                result = self.run_make("check", extra_env={"CLAUDE_STUB_FAIL_ON": target})
                self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertIn(f"stub claude: {target} is not valid", result.stderr)
                self.assertIn("==> manifest", result.stdout)
                self.assertNotIn("==> catalog", result.stdout)


if __name__ == "__main__":
    unittest.main()
