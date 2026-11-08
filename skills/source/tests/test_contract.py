"""Contract tests for the text of the source skill (PLAN-D2).

The SkillText checks run over each file of TEXT_FILES, and each later task
appends its file there and adds one class for it. The suite puts tools/ on
sys.path itself, so a plain `python3 -m unittest` runs it.
"""

import re
import subprocess
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
SKILL = ROOT / "skills" / "source"
REF = SKILL / "reference"
COMMANDS = SKILL / "cli" / "testdata" / "commands.txt"
STE_CHECK = ROOT / "skills" / "voice" / "scripts" / "ste_check.py"
sys.path.insert(0, str(ROOT / "tools"))
import skillkit  # noqa: E402

TEXT_FILES = ["reference/rules.md", "reference/developer.md", "reference/evaluator.md", "reference/reporter.md"]
ENGINE = re.compile(r"monet-source((?: [a-z]+){1,2})")
# A persona card (R25): a JSON weight field, or a YAML weight line.
CARD = re.compile(r'"weight"\s*:\s*"(blocking|advisory)"|^\s*weight:\s*(blocking|advisory)\s*$', re.MULTILINE)
# The personal terms of the gang port test, built from parts so that this file does not match them.
PERSONAL = re.compile("|".join(["way" + "ne", "pot" + "ato", "wa" + "wen",
                                r"~/\.cl" + "aude", r"\$HOME/\.cl" + "aude"]), re.IGNORECASE)
PRONOUN = re.compile(r"\b(he|him|his|himself|she|her|hers|herself)\b", re.I)


def read(rel):
    return (SKILL / rel).read_text(encoding="utf-8")


def section(text, heading):
    """The text under `## <heading>`, up to the next `## ` heading."""
    m = re.search(rf"^## {re.escape(heading)}\n(.*?)(?=^## |\Z)", text, re.MULTILINE | re.DOTALL)
    if m is None:
        raise AssertionError(f"no '## {heading}' section")
    return m.group(1)


def build_lines(text):
    return [line.strip() for line in text.splitlines() if "go -C" in line or "go build" in line]


def engine_commands(text):
    """(command, known) per engine call: the two-word form when commands.txt lists it, else the first word."""
    known = set(COMMANDS.read_text(encoding="utf-8").splitlines())
    calls = []
    for m in ENGINE.finditer(text):
        words = m.group(1).strip()
        name = words if words in known else words.split()[0]
        calls.append((name, name in known))
    return calls


class SkillText(unittest.TestCase):
    def test_text_names_only_engine_commands(self):
        sample = "Run `<run>/bin/monet-source round open --holder h`, then monet-source status and monet-source frobnicate."
        self.assertEqual(engine_commands(sample), [("round open", True), ("status", True), ("frobnicate", False)])
        self.assertEqual(engine_commands("monet-source round opne"), [("round", False)])
        for rel in TEXT_FILES:
            with self.subTest(file=rel):
                self.assertEqual([c for c, ok in engine_commands(read(rel)) if not ok], [])

    def test_text_passes_ste_check(self):
        for rel in TEXT_FILES:
            with self.subTest(file=rel):
                r = subprocess.run([sys.executable, str(STE_CHECK), str(SKILL / rel)],
                                   capture_output=True, text=True, check=False, timeout=60)
                self.assertEqual(r.returncode, 0, r.stdout + r.stderr)

    def test_text_names_no_other_skill_md(self):
        for rel in TEXT_FILES:
            with self.subTest(file=rel):
                self.assertEqual(skillkit.skill_md_refs(read(rel), "source"), [])

    def test_text_holds_no_persona_card(self):
        self.assertIsNotNone(CARD.search('{"id": "x", "weight": "blocking"}'))
        self.assertIsNotNone(CARD.search("  weight: advisory\n"))
        self.assertIsNotNone(CARD.search('"weight":"advisory"'))
        self.assertIsNotNone(CARD.search("x\nweight: blocking  \ny"))
        for rel in TEXT_FILES:
            with self.subTest(file=rel):
                self.assertIsNone(CARD.search(read(rel)))

    def test_reference_files_hold_no_variable(self):
        self.assertEqual(build_lines("a\n  go -C x build ./y\ngo build ./z\nb"), ["go -C x build ./y", "go build ./z"])
        self.assertLessEqual({f"reference/{p.name}" for p in REF.glob("*.md")}, set(TEXT_FILES))
        for rel in TEXT_FILES:
            if rel.startswith("reference/"):
                with self.subTest(file=rel):
                    self.assertNotIn("${", read(rel))
                    self.assertEqual(build_lines(read(rel)), [])
                    self.assertEqual(REF / Path(rel).name, SKILL / rel)

    def test_text_names_no_person(self):
        pronouns = ["He", "him", "his", "himself", "She", "her", "hers", "herself"]
        self.assertEqual(PRONOUN.findall(" ".join(pronouns) + " they other heroes"), pronouns)
        for term in ("Way" + "ne", "pot" + "ato", "wa" + "wen", "~/.cl" + "aude", "$HOME/.cl" + "aude"):
            self.assertIsNotNone(PERSONAL.search(term))
        for rel in TEXT_FILES:
            with self.subTest(file=rel):
                self.assertIsNone(PERSONAL.search(read(rel)))
                self.assertIsNone(PRONOUN.search(read(rel)))


class Rules(unittest.TestCase):
    def test_rules_md_names_the_shared_rules(self):
        self.assertEqual(section("## A\nx\n## B\ny\n", "A"), "x\n")
        self.assertEqual(section("## A\nx\n## B\ny\n", "B"), "y\n")
        self.assertRaises(AssertionError, section, "## A\nx\n", "B")
        text = read("reference/rules.md")
        self.assertEqual(re.findall(r"(?m)^#{1,2} .*", text),
                         ["# Rules", "## Every role", "## Engine calls", "## Exit codes", "## The owner"])
        for want in ("The owner is the user who started the kickoff of this run.", 'echo "$CLAUDE_CODE_SESSION_ID"',
                     "--holder", "`brain`", "`dev`", "Act only on the words of the owner.", "`<run>/owner-words.md`",
                     "Write only to the run page.", "Never take an irreversible step on a default.",
                     "run `<run>/bin/monet-source status`, and stop driving the run"):
            with self.subTest(want=want):
                self.assertIn(want, text)
        self.assertIsNone(re.search(r"\bgang\b", text, re.IGNORECASE))
        codes = section(text, "Exit codes")
        for code in (0, 1, 2, 3, 4, 64, 75):
            with self.subTest(code=code):
                self.assertEqual(len([row for row in codes.splitlines() if row.startswith(f"| {code} |")]), 1)


class Developer(unittest.TestCase):
    def test_developer_md_headings(self):
        self.assertEqual(re.findall(r"(?m)^#{1,2} .*", read("reference/developer.md")),
                         ["# Developer", "## Start", "## Pilot", "## A version", "## Findings",
                          "## Decisions and walls", "## Heartbeat", "## Relay"])

    def test_developer_md_loop(self):
        text = read("reference/developer.md")
        for want in ("dev hello", "watch --role dev", "dev beat", "at most 45 minutes", "`<run>/dev`", "`<run>/notes/`",
                     "submit --path", "respond", "decision add", "--reason", "wall open",
                     "<run>/rounds/<n>/findings.md", "RULING", "context --role dev", "`<run>/relay/dev.md`"):
            with self.subTest(want=want):
                self.assertIn(want, text)

    def test_developer_md_pilot(self):
        pilot = section(read("reference/developer.md"), "Pilot")
        for want in ("pwd -P", "decision add"):
            with self.subTest(want=want):
                self.assertIn(want, pilot)

    def test_developer_md_is_step_one(self):
        text = read("reference/developer.md")
        self.assertIsNone(re.search(r"\bgang\b", text, re.IGNORECASE))
        for banned in ("dev handover", "ship wait", "item hold", "submit --sha"):
            with self.subTest(banned=banned):
                self.assertNotIn(banned, text)


class Evaluator(unittest.TestCase):
    def test_evaluator_md_writes_one_verdict(self):
        text = read("reference/evaluator.md")
        self.assertEqual(re.findall(r"(?m)^#{1,2} .*", text), ["# Evaluator", "## Read", "## Judge", "## Write"])
        for want in ("Read only your brief, the version files, and the sources that the brief lists.",
                     "Judge against the goal and the quality bar of the charter, not against the reading of the developer.",
                     "`accept`", "`block`", "`blocker`", "`major`", "`minor`",
                     "A `block` needs at least one `blocker` finding.", "Write only the verdict file.",
                     "Return one line: the path of the verdict file."):
            with self.subTest(want=want):
                self.assertIn(want, text)


class Reporter(unittest.TestCase):
    def test_reporter_md_writes_the_page(self):
        text = read("reference/reporter.md")
        self.assertEqual(re.findall(r"(?m)^#{1,2} .*", text),
                         ["# Reporter", "## Read", "## The page", "## The write", "## Delivery"])
        # The Brain names the packet path in the prompt, so the file names no fixed path (R17).
        self.assertNotIn("packet.json", text)
        for want in ("the packet path",
                     "the executive summary, the items that wait for the owner, the decisions, and a dated log",
                     "Fetch the page right before the write.", "Use only targeted edits.", "Never replace the whole page",
                     "Write on no other page.", "STE-lite", "mermaid", "<at> <line>", "wrote <time>",
                     "no write: <reason>"):
            with self.subTest(want=want):
                self.assertIn(want, text)
        delivery = section(text, "Delivery")
        for want in ("verdicts", "costs of invariants", "link"):
            with self.subTest(delivery=want):
                self.assertIn(want, delivery)


if __name__ == "__main__":
    unittest.main()
