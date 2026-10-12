"""Contract for the architecture overview, docs/architecture.md (DOC-5)."""

import json
import os
import re
import subprocess
import sys
import unittest
from pathlib import Path

import skillkit

ROOT = Path(__file__).resolve().parents[2]
DOC = ROOT / "docs" / "architecture.md"
PLUGIN = ROOT / ".claude-plugin" / "plugin.json"
STYLE = ROOT / "output-styles" / "ste-lite.md"
SKILLS = ROOT / "skills"
STE_CHECK = ROOT / "skills" / "voice" / "scripts" / "ste_check.py"

MERMAID = re.compile(r"^```mermaid\n(.*?)^```$", re.M | re.S)
FENCE = re.compile(r"^```[^\n]*\n(.*?)^```$", re.M | re.S)
# A diagram node: id["label"], id(["label"]), or id[("label")].
NODE = re.compile(r"^\s*(\w+)[\[(]+\"([^\"]+)\"[\])]+\s*$")
EDGE = re.compile(r"^\s*(\w+) -\.?->(?:\|\"[^\"]*\"\|)? (\w+)\s*$")
GROUP = re.compile(r"^\s*(flowchart \w+|subgraph \w+\[\"[^\"]+\"\]|end)\s*$")
# The nodes the diagram must hold, each as a pattern that its label starts with.
NODES = (r"skills/,", r"agents/monet\.md,", r"output-styles/ste-lite\.md,",
         r"\.claude-plugin/plugin\.json userConfig", r"data dir:")
# A userConfig table row: | `key` | default cell | ... |
ROW = re.compile(r"^\| `([a-z_]+)` \| ([^|]*) \|", re.M)
# No prose sentence names more skills than this, so no sentence is a catalog.
MAX_SKILLS_PER_SENTENCE = 6


def doc_text():
    return DOC.read_text(encoding="utf-8")


def section(text, word):
    """The body of the level 2 section whose heading holds `word`."""
    for part in re.split(r"^## ", text, flags=re.M)[1:]:
        title, _, body = part.partition("\n")
        if word in title:
            return body
    return ""


def sentences(body):
    """The prose sentences of a body, with fences dropped and lines folded."""
    return re.split(r"(?<=\.) ", re.sub(r"\s+", " ", FENCE.sub(" ", body)))


def manifest_defaults():
    config = json.loads(PLUGIN.read_text(encoding="utf-8"))["userConfig"]
    return {key: spec["default"] for key, spec in config.items()}


def skill_names():
    return sorted(p.parent.name for p in SKILLS.glob("*/SKILL.md"))


def named_skills(text):
    """The skill dir names that a text holds as code spans."""
    spans = set(re.findall(r"`([a-z-]+)`", text))
    return [name for name in skill_names() if name in spans]


class ArchitectureDoc(unittest.TestCase):
    def assertSentence(self, body, pattern):
        """Assert that one sentence of `body` matches every regex in `pattern`."""
        patterns = [pattern] if isinstance(pattern, str) else pattern
        hits = [s for s in sentences(body)
                if all(re.search(p, s) for p in patterns)]
        self.assertTrue(hits, f"no sentence matches {patterns}")

    def test_one_mermaid_diagram(self):
        text = doc_text()
        self.assertEqual(text.count("```mermaid"), 1)
        diagram = MERMAID.search(text)
        self.assertIsNotNone(diagram, "the mermaid fence is not closed")
        labels, edges = {}, []
        for line in diagram.group(1).splitlines():
            node, edge = NODE.match(line), EDGE.match(line)
            if node:
                labels[node.group(1)] = node.group(2)
            elif edge:
                edges.append(edge.groups())
            else:
                self.assertRegex(line, GROUP, "a diagram line is not a node, "
                                 "an edge, or a group line")
        for start, end in edges:
            with self.subTest(edge=f"{start} -> {end}"):
                self.assertIn(start, labels)
                self.assertIn(end, labels)
        for node in NODES:
            with self.subTest(node=node):
                self.assertTrue(any(re.match(node, label)
                                    for label in labels.values()),
                                f"no node label starts with {node}")

    def test_user_config_defaults_match_manifest(self):
        rows = {key: cell.strip() for key, cell in ROW.findall(doc_text())}
        defaults = manifest_defaults()
        self.assertEqual(sorted(rows), sorted(defaults))
        for key, default in defaults.items():
            with self.subTest(key=key):
                # An empty default is written as the word "empty".
                self.assertEqual(rows[key], f"`{default}`" if default else "empty")

    def test_skills_section(self):
        body = section(doc_text(), "Skills")
        self.assertSentence(body, r"`skills/<name>/SKILL\.md`.*`/monet:<name>`")
        self.assertSentence(body, r"old command wrappers folded into their skills")
        self.assertSentence(body, r"`\$\{CLAUDE_SKILL_DIR\}` expands to the "
                                  r"skill's dir in the installed plugin")
        self.assertSentence(body, r"`\$\{CLAUDE_PLUGIN_ROOT\}` expands to the "
                                  r"root of the installed plugin")
        self.assertSentence(body, r"One skill calls another as `monet:<name>`")
        self.assertSentence(body, r"`pr` and `review-local` call `monet:paint`")

    def test_agent_section(self):
        body = section(doc_text(), "agent")
        self.assertSentence(body, r"`agents/monet\.md` runs as `monet:monet`")
        self.assertSentence(body, r"Start it with `claude --agent monet:monet`")
        self.assertSentence(body, [r"routing table", r"dispatch step"])
        self.assertSentence(body, r"plugins ignore `permissionMode`")

    def test_output_style_section(self):
        body = section(doc_text(), "output style")
        self.assertSentence(body, r"`monet:ste-lite`")
        target = os.readlink(STYLE)
        self.assertFalse(os.path.isabs(target), "the style symlink is absolute")
        self.assertSentence(body, r"`output-styles/ste-lite\.md` is a relative "
                                  r"symlink to `" + re.escape(target) + "`")
        self.assertSentence(body, r"the STE-lite card lives in one place")

    def test_user_config_section(self):
        body = section(doc_text(), "userConfig")
        defaults = manifest_defaults()
        self.assertSentence(body, r"`\$\{user_config\.<key>\}`")
        for command in ("/plugin configure monet@monet",
                        "claude plugin configure monet@monet",
                        "claude plugin install monet@monet --config data_dir=<path>"):
            with self.subTest(command=command):
                self.assertIn(command, FENCE.search(body).group(1))
        self.assertSentence(body, [r"unset key stays as the literal placeholder",
                                   r"`default`"])
        self.assertEqual(defaults["branch_prefix"], "")
        self.assertSentence(body, [r"uses no prefix for `branch_prefix`",
                                   r"uses `" + re.escape(defaults["data_dir"])
                                   + r"` for `data_dir`"])

    def test_data_dir_section(self):
        body = section(doc_text(), "data dir")
        layout = FENCE.search(body).group(1)
        for dir_name in ("personas/", "interview-prep/"):
            with self.subTest(layout=dir_name):
                self.assertRegex(layout, r"(?m)^\s+" + re.escape(dir_name))
        items = dict(re.findall(r"^- `([a-z-]+/)`: (.*)$", body, re.M))
        for dir_name, writes, reads in (
                ("personas/", "distill-voice", "voice"),
                ("interview-prep/", "interview-tracker", "interview-progress")):
            with self.subTest(item=dir_name):
                self.assertIn(dir_name, items)
                self.assertIn(f"`{writes}` writes", items[dir_name])
                self.assertIn(f"`{reads}` reads", items[dir_name])
        self.assertSentence(body, [r"not in the plugin dir",
                                   r"replaces the plugin cache on each update"])
        self.assertSentence(body, [r"not in `\$\{CLAUDE_PLUGIN_DATA\}`",
                                   r"deletes that dir when the user uninstalls"])

    def test_names_the_planning_writers(self):
        body = section(doc_text(), "Run scratch")
        line = [s for s in sentences(body) if "`.planning/`" in s]
        self.assertTrue(line, "no sentence names `.planning/`")
        for name in ("gang", "handoff", "pr", "review-local", "teach",
                     "visual-design"):
            with self.subTest(skill=name):
                self.assertIn(f"`{name}`", line[0])
        self.assertSentence(body, r"nothing writes inside the plugin dir")

    def test_links_the_catalog_and_does_not_repeat_it(self):
        text = doc_text()
        self.assertIn("](../README.md", text)
        names = skill_names()
        self.assertLess(len(named_skills(text)), len(names),
                        "the doc names every skill")
        for sentence in sentences(text):
            with self.subTest(sentence=sentence[:60]):
                self.assertLessEqual(len(named_skills(sentence)),
                                     MAX_SKILLS_PER_SENTENCE)
        row = re.compile(r"^\s*[-*|]\s*`/?(?:monet:)?(" + "|".join(
            map(re.escape, names)) + r")`", re.M)
        self.assertEqual(row.findall(text), [], "a list or table row per skill")

    def test_has_no_dotfiles_paths(self):
        self.assertEqual(skillkit.dotfiles_paths(doc_text()), [])

    def test_prose_has_no_em_dash_or_semicolon(self):
        result = subprocess.run([sys.executable, str(STE_CHECK), str(DOC)],
                                capture_output=True, text=True, check=False,
                                timeout=60)
        self.assertIn("em-dashes 0, semicolons 0", result.stdout)
        self.assertNotIn(chr(0x2014), doc_text())


if __name__ == "__main__":
    unittest.main()
