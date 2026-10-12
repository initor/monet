"""Port contract for the monet agent, agents/monet.md (CORE-1, CORE-4)."""

import os
import re
import subprocess
import unittest
from pathlib import Path

import skillkit

ROOT = Path(__file__).resolve().parents[2]
AGENT = ROOT / "agents" / "monet.md"
PREFIX = "${user_config.branch_prefix}"
DATA_DIR = "${user_config.data_dir}"

# Each routing row: its trigger cell, verbatim from the snapshot, and its Skill cell.
TABLE = (
    ('"what changed", before commit/PR', "`monet:git-changes`"),
    ('"draft a commit message"', "`monet:git-draft-commit`"),
    ('"draft a PR", "write a PR description"', "`monet:git-draft-pr`"),
    ('"ship it", "commit and PR", "send it", "push it", "open a PR", "make a commit" (commit only)',
     "`monet:ship`"),
    ('"write this in my voice", "does this sound like me?", "make this not sound like AI"', "`monet:voice`"),
    ('"STE-lite", "write this runbook in STE", "put these test steps in STE-lite"',
     "`monet:voice` (STE-lite is the default)"),
    ('"distill this writing into a persona", "learn this writing style", "extract the mechanics of this doc"',
     "`monet:distill-voice`"),
    ('"review this code", "code review" (no PR #)', "`monet:review-code`"),
    ('"review this pull request" (with a full PR URL)', "`monet:pr`"),
    ('"review my changes", "pre-PR review", "check my work"', "`monet:review-local`"),
    ('"practice interviews", "run a mock", "prep", "show my progress"', "`monet:prep`"),
    ('"explain a concept", "what is X"', "`monet:sd-explainer`"),
    ('"architecture diagram", "design playground"', "`monet:sd-playground`"),
    ('"paint this", "visualize this", "make this visual"', "`monet:paint`"),
    ('"one sheet", "sheet this", "one image I can paste"', "`monet:paint` with `--sheet`"),
    ('"explain this to a newcomer", "a picture explainer", "an explainer for the getting-started guide"',
     "`monet:paint` with `--explainer`"),
    ('"a Hairline figure", "an isometric figure", "an empty-state figure"', "`monet:hairline-create`"),
    ('"design a logo", "visual design for X", "give me options for the look of X", '
     '"design an overview page for X"', "`monet:visual-design`"),
    ('"refresh the roadmap", "regenerate roadmap.html"', "`monet:roadmap`"),
    ('"orchestrate", "run this plan with a fleet", "fleet", "gang"', "`monet:gang`"),
    ('"I\'m at 80% context", "starting a new session", "draft a prompt to pick this up"', "`monet:handoff`"),
    ('"teach me this", "help me understand what we did", "walk me through this PR/file/change"',
     "`monet:teach`"),
    ('"drive a browser", "operate Slack/Docs for me", "take over a Chrome window"', "`monet:web-operator`"),
)

ROUTED = {"git-changes", "git-draft-commit", "git-draft-pr", "ship", "voice", "distill-voice",
          "review-code", "pr", "review-local", "prep", "sd-explainer", "sd-playground", "paint",
          "hairline-create", "visual-design", "roadmap", "gang", "handoff", "teach", "web-operator"}

# Quoted phrases in the dispatch steps that hold I, me, or my, verbatim from the snapshot.
STEP_PHRASES = (
    '"review my changes" / "review local" / "review before PR" / "check my work" / "pre-PR review" / '
    '"review what I\'ve done" / "review my local changes"',
    '"view prep progress" / "check my progress" / "show dashboard" / "prep stats"',
    '"I want to practice"',
    '"create an architecture diagram" / "draw my system design" / "design playground"',
    '"one sheet" / "sheet this" / "one image I can paste"',
    '"Practice a mock and show my progress"',
    '"Review my changes before the PR"',
    '"drive a browser" / "operate Slack/Docs/a dashboard for me" / "take over a Chrome window"',
    '"teach me this" / "help me understand what we just did" / "walk me through this change/PR/file" / '
    '"make sure I actually understand this" / "quiz me until I get it"',
    '"write this in my voice" / "does this sound like me?" / "make this not sound like AI" / '
    '"rewrite this so it sounds like me"',
)

ROW = re.compile(r"^\| (.+?) \| (.+?) \| (.+?) \| (.+?) \|$", re.M)
# Every skill dir name, so a bare `<name>` in backticks shows up wherever it sits.
ALL_SKILLS = sorted({p.parent.name for p in skillkit.skills(ROOT)} | {"git" + "-ship"}, key=len, reverse=True)
BARE = re.compile(r"(?<![\w:/-])`(" + "|".join(map(re.escape, ALL_SKILLS)) + r")[`\s]")
ROUTE = re.compile(r"(?<![\w-])monet:([a-z][a-z0-9]*(?:-[a-z0-9]+)*)")
FIRST_PERSON = re.compile(r"\bI\b|\b(?:me|my|mine|myself)\b", re.I)
QUOTED = re.compile(r'"[^"\n]*"|`[^`\n]*`')
CHECKOUT = re.compile(r"git checkout -b (\S+)")
USE = re.compile(r"\$\{user_config\.([a-z_]+)\}")
# The end of a sentence: a stop, any closing marks, then whitespace or the end.
SENTENCE_END = re.compile(r"[.!?][`\"')\]]*(?=\s|$)")


def line_of(text, pos):
    return text.count("\n", 0, pos) + 1


def tracked(*paths):
    """The paths, out of the ones given, that the git index tracks."""
    env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}
    out = subprocess.run(["git", "-C", str(ROOT), "ls-files", "-z", "--cached", "--", *paths],
                         check=True, capture_output=True, encoding="utf-8", env=env).stdout
    return {p for p in out.split("\0") if p}


def step(body, n):
    """The text of dispatch step n, from its `n) ` line to the next step or heading."""
    m = re.search(rf"^{n}\) .*?(?=^\d+\) |^## )", body, re.M | re.S)
    if not m:
        raise AssertionError(f"no dispatch step {n}")
    return m.group()


class AgentPort(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.text = AGENT.read_text(encoding="utf-8")
        cls.meta = skillkit.frontmatter(cls.text)
        cls.body = cls.text.split("\n---\n", 1)[1]
        cls.rows = [m.groups() for m in ROW.finditer(cls.body) if m.group(1) != "Trigger phrases"]

    def test_no_permission_mode(self):
        self.assertEqual(self.meta.get("name"), "monet")
        self.assertTrue(self.meta.get("tools"))
        self.assertEqual(self.meta.get("model"), "opus")
        self.assertNotIn("permissionMode", self.meta)
        self.assertNotIn("permissionMode", self.text)

    def test_routes_use_monet_prefix(self):
        self.assertEqual([(r[0], r[1]) for r in self.rows], list(TABLE))
        routed = {m.group(1) for row in self.rows for m in ROUTE.finditer(row[1])}
        self.assertEqual(routed, ROUTED)
        bare = [(line_of(self.body, m.start()), m.group()) for m in BARE.finditer(self.body)]
        self.assertEqual(bare, [])
        self.assertIn("`monet:cwd-context`", step(self.body, 0))
        self.assertIn("`monet:interview-progress`", step(self.body, 9))
        self.assertIn("`monet:interview-tracker`", self.body)
        self.assertIn("type `/monet:teach` in the main session", step(self.body, 11))
        self.assertIn("type `/monet:gang` in a session", step(self.body, 15))

    def test_moved_out_of_import(self):
        self.assertEqual(tracked("import/agents"), set())
        self.assertEqual(tracked("agents/monet.md"), {"agents/monet.md"})

    def test_every_route_has_a_skill(self):
        names = {m.group(1) for m in ROUTE.finditer(self.text)}
        self.assertTrue(ROUTED | {"cwd-context", "interview-progress", "interview-tracker"} <= names)
        wanted = {f"skills/{n}/SKILL.md" for n in names}
        self.assertEqual(sorted(wanted - tracked(*wanted)), [])

    def test_no_git_ship(self):
        self.assertNotIn("git" + "-ship", self.text)
        self.assertIn("use `monet:ship`", step(self.body, 3))

    def test_branch_prefix_from_user_config(self):
        policy = step(self.body, 3)
        self.assertIn(f"`git checkout -b {PREFIX}<descriptive_slug>`", policy)
        branches = CHECKOUT.findall(self.body)
        self.assertTrue(branches)
        self.assertEqual([b for b in branches if not b.startswith(PREFIX + "<")], [])
        rule = [line for line in policy.splitlines() if "missing_branch_prefix" in line]
        self.assertEqual(len(rule), 1, "one policy line names the missing_branch_prefix violation")
        self.assertIn(f"Only when `{PREFIX}` is set", rule[0])
        self.assertIn("monet:git-changes", rule[0])

    def test_generic_user(self):
        self.assertNotRegex(self.text, re.compile(r"wayne|wawen", re.I))
        self.assertRegex(self.meta["description"], re.compile(r"\bthe user\b", re.I))
        self.assertNotRegex(self.meta["description"], FIRST_PERSON)
        opening = next(line for line in self.body.splitlines() if line.strip())
        self.assertIn("the user", opening)
        prose = QUOTED.sub("", self.body)
        hits = [(line_of(prose, m.start()), m.group()) for m in FIRST_PERSON.finditer(prose)]
        self.assertEqual(hits, [])

    def test_trigger_phrases_verbatim(self):
        for phrases in STEP_PHRASES:
            self.assertIn(phrases, self.body)

    def test_personas_from_data_dir(self):
        voice = step(self.body, 14)
        first = voice.find(f"`{DATA_DIR}/personas/` first")
        bundled = voice.find("the bundled map in the voice skill")
        self.assertGreaterEqual(first, 0)
        self.assertGreater(bundled, first)

    def test_user_config_fallbacks(self):
        self.assertIn(PREFIX, self.text)
        self.assertIn(DATA_DIR, self.text)
        self.assertEqual(skillkit.missing_fallbacks(self.text), [])
        self.assertEqual(skillkit.dotfiles_paths(self.text), [])
        # missing_fallbacks looks up to the next blank line. R1 wants the very next sentence.
        uses = list(USE.finditer(self.text))
        self.assertEqual({m.group(1) for m in uses}, {"branch_prefix", "data_dir"})
        for m in uses:
            with self.subTest(line=line_of(self.text, m.start()), key=m.group(1)):
                end = SENTENCE_END.search(self.text, m.end())
                self.assertIsNotNone(end, "the sentence that holds the use has an end")
                following = " ".join(self.text[end.end():].split())
                fallback = " ".join(skillkit.USER_CONFIG_FALLBACKS[m.group(1)].split())
                self.assertTrue(following.startswith(fallback),
                                f"the next sentence is the R1 fallback, not: {following[:80]!r}")


if __name__ == "__main__":
    unittest.main()
