"""Contract tests for the gang port (CORE-10).

They read the real files under skills/gang/ and check what the port promises:
the plugin-root build line, the `<skill>` placeholder in the reference files,
the plan schema in reference/schema.md, prompts that name the user, neutral
fixtures, the folded /monet:gang command, and no read of another skill's
SKILL.md by path (ruling R8). Run with tools/ on PYTHONPATH, as `make check`
does.
"""

import json
import os
import re
import subprocess
import unittest
from pathlib import Path

import skillkit

SKILL = Path(__file__).resolve().parent.parent
REPO = SKILL.parent.parent
REF = SKILL / "reference"
PLAN_GO = SKILL / "cli" / "internal" / "plan" / "plan.go"
LINT_GO = SKILL / "cli" / "internal" / "lint" / "lint.go"

BUILD = ('go -C ${CLAUDE_PLUGIN_ROOT}/skills/gang/cli build '
         '-ldflags "-X main.skillDir=${CLAUDE_PLUGIN_ROOT}/skills/gang" -o <dir>/run/bin/gang ./cmd/gang')
REF_BUILD = 'go -C <skill>/cli build -ldflags "-X main.skillDir=<skill>" -o <dir>/run/bin/gang ./cmd/gang'
USER_LINE = "Refer to the user as the user, or with they or them."
ARGUMENT_HINT = ("plan <spec-dir> [--slug <slug>] | plan --adopt <file> [--adopt-source <file>] [--slug <slug>]"
                 " | run [--slug <slug>] [--merge-on-green] | status [--slug <slug>]")
ALLOWED_TOOLS = ["Bash", "Read", "Write", "Edit", "Grep", "Glob", "Agent", "Workflow", "Monitor",
                 "AskUserQuestion", "ScheduleWakeup", "TaskCreate", "TaskUpdate", "TaskStop", "ListAgents",
                 "SendMessage"]
# The acceptance grep, built from parts so that this file does not match it.
PERSONAL = re.compile("|".join(["way" + "ne", "pot" + "ato", "wa" + "wen",
                                r"~/\.cl" + "aude", r"\$HOME/\.cl" + "aude"]), re.IGNORECASE)
OLD_SPEC = "docs/specs/2026-10-11-monet-" + "gang.md"


def read(path):
    return Path(path).read_text(encoding="utf-8")


def build_lines(text):
    return [line.strip() for line in text.splitlines() if "go -C" in line or "go build" in line]


def section(text, heading):
    """The text under `## <heading>`, up to the next `## ` heading."""
    m = re.search(rf"^## {re.escape(heading)}\n(.*?)(?=^## |\Z)", text, re.MULTILINE | re.DOTALL)
    if m is None:
        raise AssertionError(f"no '## {heading}' section")
    return m.group(1)


def go_fields(struct):
    """(Go type, JSON key) of each field of a struct in plan.go, in field order."""
    body = re.search(rf"^type {struct} struct \{{\n(.*?)^\}}", read(PLAN_GO), re.MULTILINE | re.DOTALL)
    if body is None:
        raise AssertionError(f"plan.go has no struct {struct}")
    fields = re.findall(r'^\s*\w+\s+(\S+)\s+`json:"([^"]+)"`', body.group(1), re.MULTILINE)
    return [(typ, tag.split(",")[0]) for typ, tag in fields]


def go_json_tags(struct):
    """The JSON keys of a struct in plan.go, in field order."""
    return [tag for _, tag in go_fields(struct)]


def check_decode(test, value, typ, where):
    """Assert that value decodes into Go type typ as plan.Load does: strict keys, exact JSON types."""
    if typ.startswith("[]"):
        test.assertIsInstance(value, list, where)
        for i, v in enumerate(value):
            check_decode(test, v, typ[2:], f"{where}[{i}]")
    elif typ.startswith("map[string]"):
        test.assertIsInstance(value, dict, where)
        for k, v in value.items():
            check_decode(test, v, typ[len("map[string]"):], f"{where}.{k}")
    elif typ in ("string", "int"):
        test.assertIs(type(value), {"string": str, "int": int}[typ], where)
    else:
        test.assertIsInstance(value, dict, where)
        fields = {tag: t for t, tag in go_fields(typ)}
        test.assertLessEqual(set(value), set(fields), where)
        for k, v in value.items():
            check_decode(test, v, fields[k], f"{where}.{k}")


def dig(obj, dotted):
    for key in dotted.split("."):
        obj = obj[key]
    return obj


def go_strings(text, var):
    """The quoted strings of a Go `<var> = ...` line."""
    return re.findall(r'"([^"]+)"', re.search(rf"\b{var}\s*=.*", text).group())


def tracked_files(*paths):
    env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}
    out = subprocess.run(["git", "-C", str(REPO), "ls-files", "--", *paths],
                         check=True, capture_output=True, text=True, env=env).stdout
    return out.split()


def skill_files():
    for root, dirs, names in os.walk(SKILL):
        dirs[:] = [d for d in dirs if d != "__pycache__"]
        for name in names:
            yield Path(root) / name


def skill_texts():
    """(path, text) for every file under skills/gang that decodes as UTF-8."""
    for path in skill_files():
        try:
            yield path, read(path)
        except UnicodeDecodeError:
            continue


class GangPort(unittest.TestCase):
    def setUp(self):
        self.skill = skillkit.read_skill(SKILL / "SKILL.md")
        self.text = read(SKILL / "SKILL.md")
        self.coordinator = read(REF / "coordinator.md")
        self.planner = read(REF / "planner.md")

    def test_build_line_uses_plugin_root(self):
        self.assertEqual(build_lines(self.text), [BUILD])
        self.assertNotIn("The design is in", self.text)
        for path, text in skill_texts():
            self.assertNotIn(OLD_SPEC, text, path.relative_to(REPO))

    def test_reference_files_use_skill_placeholder(self):
        paths = section(self.text, "Paths")
        self.assertRegex(paths, r"(?m)^\| `<skill>` \| `\$\{CLAUDE_PLUGIN_ROOT\}/skills/gang`")
        for name, text in (("coordinator.md", self.coordinator), ("planner.md", self.planner)):
            with self.subTest(file=name):
                self.assertEqual(build_lines(text), [REF_BUILD])
                self.assertIn("<skill>/reference/schema.md", text)
                # Every roles.json and schema.md the file names sits under <skill>.
                for m in re.finditer(r"\S*(roles\.json|schema\.md)", text):
                    self.assertTrue(m.group().lstrip("`").startswith("<skill>/"), f"{name}: {m.group()}")
        self.assertIn("<skill>/roles.json", self.planner)
        self.assertIn("<skill>/roles.json", self.coordinator)
        # A reference file is read without expansion, so no ${...} variable may appear in one.
        for path in sorted(REF.glob("*.md")):
            with self.subTest(file=path.name):
                self.assertNotIn("${", read(path))

    def test_schema_in_reference(self):
        schema = read(REF / "schema.md")
        # No section citation in any form: "section(s)", "spec", the section sign, or a bare N.N.
        for word in (r"\bsections?\b", r"\bspec\b", "§", r"\b\d+\.\d+\b", "p" + "stack", "ENG-25", "ini" + "tor"):
            self.assertIsNone(re.search(word, schema, re.IGNORECASE), word)

        run_wide = section(schema, "Run-wide fields")
        example = json.loads(re.search(r"```json\n(.*?)```", run_wide, re.DOTALL).group(1))
        self.assertEqual(list(example), go_json_tags("Plan"))
        self.assertEqual(example["repo"]["remote"], "github.com/example/demo")
        self.assertEqual(example["repo"]["gh_account"], "example")
        self.assertEqual(example["requirements"], [{"id": "API-1", "source": "specs/api.md:12"}])

        # The example passes the strict decode (keys and JSON types of plan.go) and lint rules 1, 2, and 7.
        go = read(PLAN_GO)
        lint = read(LINT_GO)
        check_decode(self, example, "Plan", "plan")
        version = int(re.search(r"p\.Version != (\d+)", lint).group(1))
        self.assertEqual(example["version"], version)
        slug_re = re.search(r"slugRE\s*= regexp\.MustCompile\(`([^`]+)`", lint).group(1)
        self.assertRegex(example["slug"], slug_re)
        repo = example["repo"]
        self.assertNotEqual(repo["integration"], repo["base"])
        wave_title = repo.get("wave_title") or re.search(r'DefaultWaveTitle = "([^"]+)"', go).group(1)
        self.assertIn("{k}", wave_title)
        max_title = int(re.search(r"const maxWaveTitle = (\d+)", lint).group(1))
        wave = re.search(r"plan\.WaveTitle\(p, (\d+)\)", lint).group(1)
        self.assertLessEqual(len(wave_title.replace("{k}", wave).replace("{slug}", example["slug"])), max_title)
        lock_re = re.search(r"nameRE = regexp\.MustCompile\(`([^`]+)`",
                            read(SKILL / "cli" / "internal" / "lock" / "lock.go")).group(1)
        for name in example["locks"]:
            self.assertRegex(name, lock_re)
        required = re.search(r"(?ms)^func \(l \*linter\) required\(\) \{\n(.*?)^\}", lint).group(1)
        for name in re.findall(r'\{"([\w.]+)", p\.', required):
            self.assertTrue(dig(example, name).strip(), name)
        for name in re.findall(r'l\.list\("-", "([\w.]+)"', required):
            self.assertTrue(dig(example, name), name)
            self.assertTrue(all(v.strip() for v in dig(example, name)), name)
        for r in example["requirements"]:
            self.assertTrue(r["id"] and r["source"], r)
        shape = json.loads(re.search(r"`models` maps a role to `(\{[^`]*\})`", run_wide).group(1))
        self.assertEqual(list(shape), go_json_tags("Role"))
        for role, override in example["models"].items():
            self.assertIn(role, go_strings(go, "Roles"))
            self.assertLessEqual(set(override), set(go_json_tags("Role")))
            self.assertIn(override.get("model", "opus"), go_strings(go, "Aliases"))
            self.assertIn(override.get("effort", "low"), go_strings(go, "Efforts"))

        # The defaults and closed sets that schema.md states match plan.go.
        tags = dict(re.findall(r'^\s*(\w+)\s+int\s+`json:"(\w+)"`', go, re.MULTILINE))
        defaults = re.findall(r"orDefault\(&l\.(\w+), (\d+)\)", go)
        self.assertEqual(len(defaults), len(go_json_tags("Limits")))
        for field, n in defaults:
            self.assertRegex(run_wide, rf"`{tags[field]}` {n}(?!\d)")
        title = re.search(r'DefaultWaveTitle = "([^"]+)"', go).group(1)
        self.assertIn(f"An empty `repo.wave_title` takes `{title}`", run_wide)
        for var in ("Roles", "Efforts"):
            listed = run_wide.split(f"The {var.lower()} are ")[1].split(".")[0]
            self.assertEqual(re.findall(r"`(\w+)`", listed), go_strings(go, var))

        # The rows name every Task key, and every key of a sub-struct as `<key>.<sub>`.
        rows = {re.findall(r"`([^`]+)`", row.split("|")[1])[0]: row
                for row in section(schema, "Task fields").splitlines() if row.startswith("| `")}
        names = {name for row in rows.values() for name in re.findall(r"`([^`]+)`", row.split("|")[1])}
        want = set()
        for typ, tag in go_fields("Task"):
            subs = go_json_tags(typ) if typ in ("Scope", "Interfaces", "Verify", "Size") else []
            want |= {f"{tag}.{sub}" for sub in subs} or {tag}
        self.assertEqual(names, want)
        prefix = re.search(r'return "([^"]+)" \+ t\.ID', go).group(1)
        self.assertIn(f"Defaults to `{prefix}<id>`", rows["branch"])
        self.assertEqual(re.findall(r"`(\w+)`", rows["risk"].split("|")[2]), re.findall(r't\.Risk != "(\w+)"', lint))
        id_re = re.search(r"idRE\s*= regexp\.MustCompile\(`([^`]+)`", lint).group(1)
        self.assertIn(f"string, `{id_re}`", rows["id"])
        self.assertEqual(re.findall(r"`(\w+:)`", rows["interfaces.consumes"]), go_strings(lint, "refPrefixes"))

        lint_rules = section(schema, "Lint")
        rules = re.findall(r"(?m)^(\d+)\. ", lint_rules)
        self.assertEqual(rules, [str(n) for n in range(1, 9)])
        self.assertIn(f"a `version` other than {version},", lint_rules)
        self.assertIn(f"more than {max_title} characters for wave {wave}.", lint_rules)
        # Rule 2 names every field that lint.go required() checks.
        rule2 = re.search(r"(?m)^2\. .*", lint_rules).group()
        for name in re.findall(r'"([a-z_.]*[a-z])"', required) + re.findall(r'"([\w.]+) is 0"', required):
            self.assertIn(f"`{name}`", rule2)
        aliases = re.search(r"(?m)^7\. .*?allowed set \(([^)]*)\)", lint_rules).group(1)
        self.assertEqual(re.findall(r"`(\w+)`", aliases), go_strings(go, "Aliases"))

        self.assertIn("<skill>/reference/schema.md", section(self.planner, "From specs"))
        self.assertIn("<skill>/reference/schema.md", section(self.planner, "Adopt"))
        self.assertNotIn("section 5.2", self.planner)

    def test_prompts_name_the_user(self):
        self.assertIn(f'"Spawn no subagents. Use no browser. {USER_LINE}"', self.planner)
        self.assertIn(f'- "{USER_LINE}"', section(self.coordinator, "Launch"))
        for name, text in (("SKILL.md", self.text), ("coordinator.md", self.coordinator),
                           ("planner.md", self.planner)):
            with self.subTest(file=name):
                self.assertNotRegex(text, r"(?i)\brefer to (?!the user as the user, or with they or them\.)")
                self.assertIsNone(PERSONAL.search(text))

    def test_fixtures_use_neutral_repo(self):
        hits = []
        for path, text in skill_texts():
            for n, line in enumerate(text.splitlines(), 1):
                if PERSONAL.search(line):
                    hits.append(f"{path.relative_to(REPO)}:{n}")
        self.assertEqual(hits, [])
        self.assertIn('"demo-phase2"', read(PLAN_GO.with_name("plan_test.go")))
        self.assertIn('"demo-phase2"', read(SKILL / "cli" / "internal" / "lint" / "lint_test.go"))

    def test_arguments_folded(self):
        meta = self.skill.frontmatter
        self.assertEqual(meta["name"], "gang")
        self.assertTrue(meta["description"].startswith("Use when"))
        self.assertIn("use review-local or ship instead", meta["description"])
        self.assertEqual(meta["argument-hint"], ARGUMENT_HINT)
        self.assertEqual(meta["allowed-tools"], ALLOWED_TOOLS)
        self.assertNotIn("git-ship", self.text)
        self.assertIn("Use `ship` instead", section(self.text, "When to use"))

        args = section(self.text, "Arguments")
        self.assertIn("$ARGUMENTS", args)
        self.assertIn("When a `--slug` value does not match `^[a-z0-9][a-z0-9-]*$`, tell the user, and stop.", args)
        for verb in ("`plan <spec-dir>`", "`plan --adopt <file>`", "`run`", "`status`",
                     "`--adopt-source <file>`", "`--merge-on-green`", "`--slug <slug>`"):
            self.assertIn(verb, args)
        self.assertRegex(args, r"Empty or unknown arguments: ask which verb with AskUserQuestion")
        pre = section(self.text, "Preconditions")
        self.assertRegex(pre, r"`git rev-parse --show-toplevel` prints the current directory\. "
                              r"If it does not, tell the user .*, and stop\.")
        self.assertIn("Compare it with `pwd -P`, which resolves symlinks as git does.", pre)
        self.assertEqual(tracked_files("import/skills/gang", "import/commands/monet/gang.md"), [])

    def test_names_no_other_skill_md(self):
        self.assertEqual(skillkit.skill_md_refs(self.text, "gang"), [])
        for path in REF.glob("*.md"):
            with self.subTest(file=path.name):
                self.assertEqual(skillkit.skill_md_refs(read(path), "gang"), [])


if __name__ == "__main__":
    unittest.main()
