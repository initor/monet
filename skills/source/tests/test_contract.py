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

TEXT_FILES = ["reference/rules.md", "reference/relay.md", "reference/developer.md", "reference/evaluator.md",
              "reference/reporter.md", "reference/kickoff.md", "reference/brain.md", "SKILL.md"]
BUILD = ('go -C ${CLAUDE_PLUGIN_ROOT}/skills/source/cli build -ldflags "-X main.skillDir=${CLAUDE_PLUGIN_ROOT}/skills/source" '
         '-o <run>/bin/monet-source ./cmd/monet-source')
ARGUMENT_HINT = "kickoff [<file>] [--slug <slug>] | resume [--slug <slug>] | status [--slug <slug>]"
# R4: the allowed-tools of gang without Workflow.
ALLOWED_TOOLS = ["Bash", "Read", "Write", "Edit", "Grep", "Glob", "Agent", "Monitor", "AskUserQuestion",
                 "ScheduleWakeup", "TaskCreate", "TaskUpdate", "TaskStop", "ListAgents", "SendMessage"]
# R7: the description is fixed copy, and its first sentence is the catalog row.
DESCRIPTION = ("Use when the user types /monet:source kickoff to start a long research, design, or report run with an "
               "outside panel of evaluator personas. The run works for hours with no one, and one private Notion page "
               "is its channel. It is typed only. For one PR, use ship instead, and to land a plan of code, use gang "
               "instead.")
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
        self.assertLessEqual({f"reference/{p.name}" for p in REF.glob("*.md")} | {"SKILL.md"}, set(TEXT_FILES))
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


class Relay(unittest.TestCase):
    def setUp(self):
        self.text = read("reference/relay.md")

    def holds(self, text, wants):
        self.assertEqual([want for want in wants if want not in text], [])

    def test_relay_md_layout(self):
        self.assertEqual(re.findall(r"(?m)^#{1,2} .*", self.text),
                         ["# Relay", "## Hand off", "## If the relay fails", "## After the handoff"])
        calls = re.findall(r"\S*monet-source", self.text)
        self.assertGreater(len(calls), 5)
        self.assertEqual(([c for c in calls if not c.endswith("`<run>/bin/monet-source")], re.findall(r"(?<!monet-source )\b(relay (?:words|prompt|mute)|report(?: done)?|lock (?:acquire|release|renew)|decision add|dev \w+|status|context --role)\b", self.text)), ([], ["lock acquire", "dev takeover"]))

    def test_relay_md_names_its_copies(self):
        self.holds(self.text, ("`<run>/relay/brain.md`", "`<run>/relay/dev.md`", "two folders above", "<skill>/reference/rules.md",
                               "`<role>` is `brain` for the Brain and `dev` for the developer.", "Before step 1, run `<run>/bin/monet-source status`. If its `lease <role>` line does not name `<session>`"))

    def test_relay_md_hand_off_order(self):
        hand = section(self.text, "Hand off")
        order = ["Wait for each evaluator subagent in flight to return.", "with TaskStop, such as the Monitor.", "TaskStop",
                 "Note the command of each task other than the Monitor.", "relay words", "note why under State, and go on.",
                 "<run>/handoff/<role>-<stamp>.md", "date -u +%Y%m%dT%H%M%SZ", "Goal, Done, Next, Blocked, Decisions, Corrections, Gotchas, and State",
                 "Under State, list each event line that this session did not handle, and each task that step 2 stopped.",
                 "relay prompt --role <role> --handoff <run>/handoff/<role>-<stamp>.md --holder <session>", "relay prompt --role <role> --handoff", "--mode <mode>`, the mode of this session.",
                 "fails in another way", "lock release brain", "Run the command that step 6 printed", "Every 30 seconds", "for 10 minutes at most", "10 minutes",
                 "run `<run>/bin/monet-source status` until its `lease <role>` line names another holder",
                 'or no other holder shows up, follow "If the relay fails"', "relay mute"]
        self.holds(hand, order)
        self.assertEqual(([hand.find(w) for w in order], *map(hand.count, ("Monitor", "stopped", "again"))), (sorted(map(hand.find, order)), 2, 1, 1))
        self.assertIn("The Brain releases its lease with `<run>/bin/monet-source lock release brain --holder <session>`, "
                      "because the new Brain starts with `lock acquire brain`.", hand)
        self.assertEqual(["The developer skips this step." in line for line in hand.splitlines() if "report done" in line], [True])

    def test_relay_md_keeps_dev_lease(self):
        self.assertIn("The developer keeps the `dev` lease, because the new session takes it over with `dev takeover`.", self.text)
        self.assertEqual(re.findall(r"lock release \S+", self.text), ["lock release brain"])

    def test_relay_md_failure_steps(self):
        self.holds(section(self.text, "If the relay fails"), (
            "The Brain runs `<run>/bin/monet-source lock acquire brain --holder <session>`. "
            'On exit 4, the new session holds the lease, so go on with step 10 of "Hand off".',
            "The developer runs `<run>/bin/monet-source status`. If its `lease dev` line names another holder, "
            'go on with step 10 of "Hand off".', "relay mute", "The Brain writes `relay failed: <reason>` on the page",
            'The developer runs `<run>/bin/monet-source decision add "relay failed: <reason>"', "Arm the Monitor and the heartbeat "
            "again, start again each task that step 2 stopped, and keep working. Do not try a relay again in this session."))

    def test_relay_md_forward_step(self):
        self.holds(section(self.text, "After the handoff"), ("relay words", "claude agents --json", "the session that step 8 started",
            "Skip a session in between, since it handed off too.", "SendMessage", "Read entry <id> of `<run>/owner-words.md`, and act on it."))


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


class Kickoff(unittest.TestCase):
    def test_kickoff_md_order(self):
        text = read("reference/kickoff.md")
        steps = ["init --slug <slug> --holder <session>", "## Questions", "charter lint", "charter render",
                 "charter seal --words <id>", "persona set <id>", "preflight", "dev start"]
        firsts = [text.find(step) for step in steps]
        self.assertNotIn(-1, firsts, dict(zip(steps, firsts)))
        self.assertEqual(firsts, sorted(firsts), dict(zip(steps, firsts)))
        for want in ("one question at a time", "relay words", "<run>/personas/<id>.in.md"):
            with self.subTest(want=want):
                self.assertIn(want, text)

    def test_kickoff_md_charter_fields(self):
        charter = section(read("reference/kickoff.md"), "Charter")
        for field in ("goal", "context", "invariants", "deliverable", "phases", "quality_bar", "personas", "page",
                      "grants", "id", "description", "weight", "sources", "research", "design"):
            with self.subTest(field=field):
                self.assertIn(f"`{field}`", charter)
        fields, cards = charter.split("Each persona card holds these fields.")
        rows = ((fields, ("goal", "context", "invariants", "deliverable", "phases", "quality_bar", "personas", "page",
                          "grants")),
                (cards, ("id", "description", "weight", "sources")))
        for part, names in rows:
            for name in names:
                with self.subTest(row=name):
                    self.assertRegex(part, rf"(?m)^\| `{name}` \|")
        self.assertNotRegex(read("reference/kickoff.md"), r"(?im)^\s*```\s*(json|ya?ml)\b")

    def test_kickoff_md_preflight(self):
        text = read("reference/kickoff.md")
        for heading, wants in (("Preflight", ("gh auth status", "[source <slug>] Page ready.", "in one list", "FAIL")),
                               ("Pilot", ("pwd -P", "10 minutes", "accept-edits"))):
            for want in wants:
                with self.subTest(heading=heading, want=want):
                    self.assertIn(want, section(text, heading))

    def test_kickoff_md_is_step_one(self):
        text = read("reference/kickoff.md")
        self.assertEqual(re.findall(r"(?m)^#{1,2} .*", text),
                         ["# Kickoff", "## Before the questions", "## Questions", "## Charter", "## Briefs",
                          "## Preflight", "## Pilot", "## Start the run"])
        self.assertIsNone(re.search(r"\bgang\b", text, re.IGNORECASE))
        for word in ("main_deploys", "Docker", "`implementation`"):
            with self.subTest(word=word):
                self.assertNotIn(word, text)
        start = section(text, "Start the run")
        for want in ("the seal, a clean preflight, and a clean pilot", "<skill>/reference/brain.md"):
            with self.subTest(want=want):
                self.assertIn(want, start)


def reference_names(src):
    """The reference files that engine source src names (R27, owner ruling): each reference/<name>.md, the last
    argument of filepath.Join(skill, "reference", ...), and the role files of relayKickoff, never a d.Path name."""
    role = re.search(r"(?ms)^func relayKickoff\(.*?^}", src)
    found = re.findall(r'reference/([a-z]+\.md)|"reference", "([a-z]+\.md)"', src)
    return {a or b for a, b in found} | set(re.findall(r'"([a-z]+\.md)"', role.group(0) if role else ""))


class Brain(unittest.TestCase):
    def in_order(self, text, wants):
        firsts = [text.find(want) for want in wants]
        self.assertNotIn(-1, firsts, dict(zip(wants, firsts)))
        self.assertEqual(firsts, sorted(firsts), dict(zip(wants, firsts)))

    def test_brain_md_round(self):
        part = section(read("reference/brain.md"), "A round")
        self.in_order(part, ["round open", "one Agent call per `check` line, all in one message", "round close", "item open",
                             "report > <run>/packet.json", "report done --at <time> --row <log_next>", "context --role brain"])
        for want in ("`--pick <id,...>`", "<skill>/reference/rules.md", "<skill>/reference/evaluator.md", "<skill>/reference/reporter.md"):
            with self.subTest(want=want):
                self.assertIn(want, part)
        prompts = [line for line in part.splitlines() if "Its prompt names" in line]
        self.assertEqual([("<skill>/reference/rules.md" in line) for line in prompts], [True, True])
        self.assertIn("and the packet path `<run>/packet.json`.", prompts[-1])
        self.assertRegex(part, r"(?m)^\d+\. Only a `wrote <time>` result leads to `<run>/bin/monet-source report done --at <time> "
                               r"--row <log_next> --holder <session>`\. Take `log_next` from that packet\.$")
        self.assertRegex(part, r"(?m)^\d+\. On `ACCEPT` of the final round, follow \"Delivery\"\. On any other `ACCEPT`, "
                               r"run `<run>/bin/monet-source phase next --holder <session>`\.")
        self.assertRegex(part, r"(?m)^\d+\. On `FIX` of the final round, run `<run>/bin/monet-source phase back <phase> "
                               r"--reason <reason> --holder <session>`")

    def test_brain_md_events(self):
        loop = section(read("reference/brain.md"), "Event loop")
        routes = re.findall(r'"([A-Z_]+)": \{([^}]*)\}', (SKILL / "cli" / "internal" / "events" / "events.go").read_text(encoding="utf-8"))
        brain = sorted(name for name, roles in routes if '"brain"' in roles)
        self.assertEqual((len(brain), "VERSION" in brain, "FINDINGS" in brain), (10, True, False))
        self.assertEqual(sorted(re.findall(r"(?m)^\| `([A-Z_]+)", loop)), brain)
        for want in ("watch --role brain", "FINDINGS", "claude agents --json", *brain):
            with self.subTest(want=want):
                self.assertRegex(loop, rf"\b{re.escape(want)}\b")
        self.assertRegex(loop, r"(?m)^\| `RESPONSE <n>` \|.* Start nothing,")
        self.assertRegex(loop, r"(?m)^\| `PHASE <moves> <phase>` \| At `acceptance`, run `<run>/bin/monet-source accept --holder")

    def test_brain_md_owner_channel(self):
        text = read("reference/brain.md")
        for want in ("[source <slug>]", "Never resolve a discussion.", "relay words --comment <id> --at <time>", "item rule",
                     "--words <id>", "wall clear", "charter amend", "The Brain never does the work.",
                     "the Brain talks with the owner only at a wall, an item, or delivery."):
            with self.subTest(want=want):
                self.assertIn(want, text)
        self.assertEqual(re.findall(r"(?<!<skill>/reference/)\b(?:evaluator|reporter)\.md", text), [])
        for want in ("- Each reply on the page starts with `[source <slug>]`.",
                     "relay words --comment <id> --at <time> < <run>/comment.in.txt`"):
            with self.subTest(want=want):
                self.assertIn(want, section(text, "The owner and the page"))
        walls = section(text, "Walls and items")
        for want in ("- On the `WALL W<n>` event, send the owner one short message.",
                     "run `<run>/bin/monet-source wall clear W<n> --words <id> --holder <session>`",
                     "run `<run>/bin/monet-source item rule D<n> --pick default|other --words <id> --holder <session>`",
                     "run `<run>/bin/monet-source charter amend --words <id> --holder <session>`"):
            with self.subTest(want=want):
                self.assertIn(want, walls)
        self.assertEqual(walls.lower().count("send the owner"), 1)

    def test_brain_md_heartbeat(self):
        beat = section(read("reference/brain.md"), "Heartbeat")
        for want in ("ScheduleWakeup", "at most 45 minutes", "preflight", "power", "lid"):
            with self.subTest(want=want):
                self.assertIn(want, beat)
        wakes = [int(n) for n in re.findall(r"(?m)^\d+\. Call ScheduleWakeup for (\d+) minutes", beat)]
        self.assertTrue(wakes and max(wakes) <= 45, wakes)
        self.assertRegex(beat, r"(?m)^\d+\. A `FAIL power` or `FAIL lid` line .*Post that line on the page in a comment "
                               r"that starts with `\[source <slug>\]`\.")

    def test_brain_md_delivery(self):
        for text in ("b a", "b"):
            with self.subTest(control=text), self.assertRaises(AssertionError):
                self.in_order(text, ["a", "b"])
        self.in_order(section(read("reference/brain.md"), "Delivery"),
                      ["bin/monet-source deliver --holder", "claude stop", "lock release dev", "lock release brain"])

    def test_brain_md_resume(self):
        self.in_order(section(read("reference/brain.md"), "Resume"),
                      ["lock acquire brain", "relay words", "phase back <phase> --reason <reason>", "item rule"])

    def test_brain_md_is_step_one(self):
        text = read("reference/brain.md")
        self.assertEqual(re.findall(r"(?m)^#{1,2} .*", text),
                         ["# Brain", "## Event loop", "## A round", "## The owner and the page", "## Walls and items",
                          "## Heartbeat", "## Delivery", "## Resume"])
        self.assertIsNone(re.search(r"\bgang\b", text, re.IGNORECASE))
        for banned in ("item hold", "ship wait", "SHIP_OK"):
            with self.subTest(banned=banned):
                self.assertNotIn(banned, text)

    def test_engine_names_existing_reference_files(self):
        sample = ('d.Path("relay", "dev.md")\nround.Path(run, n, "response.md")\n'
                  'filepath.Join(skill, "reference", "relay.md") // <skill>/reference/rules.md\n'
                  'func relayKickoff() {\n\tfile := "brain.md"\n}\nfunc other() { x := "x.md" }\n')
        self.assertEqual(reference_names(sample), {"relay.md", "rules.md", "brain.md"})
        names = set()
        for path in sorted((SKILL / "cli" / "cmd" / "monet-source").glob("*.go")):
            if not path.name.endswith("_test.go"):
                names |= reference_names(path.read_text(encoding="utf-8"))
        self.assertLessEqual({"brain.md", "developer.md", "relay.md", "rules.md"}, names)
        for name in sorted(names):
            with self.subTest(name=name):
                self.assertTrue((REF / name).is_file(), name)


class Skill(unittest.TestCase):
    def setUp(self):
        self.text = read("SKILL.md")

    def test_skill_md_frontmatter(self):
        meta = skillkit.read_skill(SKILL / "SKILL.md").frontmatter
        self.assertEqual(sorted(meta), ["allowed-tools", "argument-hint", "description", "disable-model-invocation", "name"])
        self.assertEqual((meta["name"], meta["argument-hint"], meta["allowed-tools"]), ("source", ARGUMENT_HINT, ALLOWED_TOOLS))
        self.assertIs(meta["disable-model-invocation"], True)
        self.assertTrue(meta["description"].startswith("Use when the user types /monet:source kickoff"))
        self.assertEqual(meta["description"], DESCRIPTION)
        self.assertEqual(re.findall(r"(?m)^#{1,2} .*", self.text.split("\n---\n", 1)[1]),
                         ["# Source", "## Overview", "## When to use", "## Arguments", "## Preconditions", "## Paths",
                          "## Verbs", "## Core rules", "## Constraints"])

    def test_skill_md_build_line(self):
        self.assertEqual(build_lines(self.text), [BUILD])

    def test_skill_md_paths(self):
        paths = section(self.text, "Paths")
        self.assertRegex(paths, r"(?m)^\| `<skill>` \| `\$\{CLAUDE_PLUGIN_ROOT\}/skills/source`")
        self.assertRegex(paths, r"(?m)^\| `<run>` \| `<home>/\.planning/source/<slug>`")
        for name in ("<home>", "<slug>"):
            with self.subTest(name=name):
                self.assertRegex(paths, rf"(?m)^\| `{name}` \| ")
        mkdir = paths.find("mkdir -p <run>/bin")
        self.assertTrue(-1 < mkdir < paths.find(BUILD), (mkdir, paths.find(BUILD)))

    def test_skill_md_arguments(self):
        args, pre = section(self.text, "Arguments"), section(self.text, "Preconditions")
        for part, want in ((args, "$ARGUMENTS"), (args, "ask which verb with AskUserQuestion"),
                           (args, "When a `--slug` value does not match `^[a-z0-9][a-z0-9-]*$`, tell the owner, and stop."),
                           (pre, "Never run this skill as a subagent."), (pre, "`go` 1.24 or later"), (pre, "`node` 22 or later")):
            with self.subTest(want=want):
                self.assertIn(want, part)

    def test_skill_md_verbs(self):
        verbs = section(self.text, "Verbs")
        for want in ("${CLAUDE_SKILL_DIR}/reference/kickoff.md", "${CLAUDE_SKILL_DIR}/reference/brain.md", '"Resume"',
                     "status", "8 at most"):
            with self.subTest(want=want):
                self.assertIn(want, verbs)
        self.assertEqual(re.findall(r"(?m)^\| `([a-z]+) ", verbs), ["kickoff", "resume", "status"])

    def test_skill_md_constraints(self):
        rules = section(self.text, "Constraints")
        for want in ("Refer to the owner as the owner, or with they or them.",
                     "Use model aliases only: `opus`, `sonnet`, `haiku`, `fable`."):
            with self.subTest(want=want):
                self.assertIn(want, rules)

    def test_readme_door_row(self):
        lines = (ROOT / "README.md").read_text().splitlines()
        row = "| Run a long research or design job with an outside review panel | `/monet:source kickoff` |"
        self.assertEqual(lines.count(row), 1)
        self.assertEqual(lines[lines.index(row) - 1], "| Land a whole plan with a fleet of agents | `/monet:gang plan <spec-dir>` |")


if __name__ == "__main__":
    unittest.main()
