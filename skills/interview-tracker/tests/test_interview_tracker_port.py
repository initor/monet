"""Contract tests for the interview-tracker port (LAB-5)."""

import json
import re
import unittest
from pathlib import Path

import skillkit

SKILL_DIR = Path(__file__).resolve().parent.parent
SKILL_MD = SKILL_DIR / "SKILL.md"
PLACEHOLDER = "${user_config.data_dir}"
# Built so that this file does not hold the text it bans.
BANNED = re.compile(r"~/\.config/interview-prep|~/\.claude|[W]ayne|[w]awen|\u2014")

# The session schema, byte for byte as the dotfiles snapshot wrote it.
SCHEMA_BLOCK = """{
  "version": 1,
  "id": "2026-02-22_sd_mock_001",
  "domain": "sd | bq | code",
  "mode": "mock | drill | story-bank",
  "topic": "string -- what was practiced (e.g., 'Design a URL shortener')",
  "started_at": "ISO 8601 timestamp",
  "ended_at": "ISO 8601 timestamp",
  "duration_seconds": 0,
  "timing_breakdown": {
    "phase_or_section_name": 0
  },
  "scores": {
    "dimension_name": {
      "score": 3,
      "label": "Average",
      "notes": "string -- brief evaluator notes for this dimension"
    }
  },
  "overall_score": 3.0,
  "takeaways": {
    "went_well": ["string -- things done well"],
    "to_improve": ["string -- areas needing improvement"],
    "tags": ["string -- machine-readable topic tags for weak area detection"]
  },
  "evaluator_summary": "string -- 2-3 sentence overall assessment"
}"""
KEYS = ["version", "id", "domain", "mode", "topic", "started_at", "ended_at", "duration_seconds",
        "timing_breakdown", "scores", "overall_score", "takeaways", "evaluator_summary"]
DIMENSIONS = {
    "sd": ["requirements_clarity", "architecture_design", "api_design", "scalability", "reliability",
           "tradeoff_articulation"],
    "bq": ["story_structure", "scope_and_impact", "decision_rationale", "leadership_signals",
           "communication_clarity"],
    "code": ["problem_understanding", "solution_design", "code_quality", "correctness",
             "complexity_analysis"],
}
LABELS = {1: "Poor", 2: "Below Average", 3: "Average", 4: "Good", 5: "Excellent"}


class InterviewTrackerPort(unittest.TestCase):
    def setUp(self):
        self.text = SKILL_MD.read_text(encoding="utf-8")
        self.skill = skillkit.read_skill(SKILL_MD)

    def test_sessions_under_data_dir(self):
        self.assertIn("mkdir -p <data dir>/interview-prep/\n", self.text)
        self.assertIn("ls -1 <data dir>/interview-prep/{date}_{domain}_{mode}_*.json", self.text)
        self.assertIn("ls -1 <data dir>/interview-prep/*.json", self.text)
        self.assertIn("`<data dir>/interview-prep/{filename}`", self.text)
        self.assertIn("`{YYYY-MM-DD}_{domain}_{mode}_{NNN}.json`", self.text)
        self.assertIn("sessions live in `<data dir>/interview-prep/`.", self.text)
        self.assertEqual(self.text.count("<data dir>/"), self.text.count("<data dir>/interview-prep/"))
        self.assertNotIn('"<data dir>', self.text)

    def test_session_schema_unchanged(self):
        m = re.search(r"### Required fields\n\n```json\n(.*?)\n```", self.text, re.S)
        self.assertIsNotNone(m, "no json block under ### Required fields")
        self.assertEqual(m.group(1), SCHEMA_BLOCK)
        schema = json.loads(m.group(1))
        self.assertEqual(list(schema), KEYS)
        self.assertEqual(schema["version"], 1)
        for domain, dims in DIMENSIONS.items():
            section = self.text.split(f"(`{domain}`)", 1)[1].split("####", 1)[0].split("## Step 3", 1)[0]
            self.assertEqual(re.findall(r"^\| `(\w+)` \|", section, re.M), dims, domain)
        for score, label in LABELS.items():
            self.assertIn(f"| {score} | {label} |", self.text)

    def test_no_dotfiles_paths(self):
        for path in sorted(p for p in SKILL_DIR.rglob("*") if p.is_file() and "__pycache__" not in p.parts):
            text = path.read_text(encoding="utf-8")
            self.assertEqual(skillkit.dotfiles_paths(text), [], path)
            self.assertIsNone(BANNED.search(text), path)
        self.assertEqual(sorted(p.name for p in SKILL_DIR.rglob("*.json")), [])

    def test_description_starts_use_when(self):
        desc = self.skill.frontmatter["description"]
        self.assertTrue(desc.startswith("Use when"), desc)
        self.assertLessEqual(len(desc), 500)
        for phrase in ("record", "read", "manage", "interview preparation practice sessions",
                       "rubric scores", "the interview-prep folder of the user's monet data dir"):
            self.assertIn(phrase, desc)
        self.assertNotIn("${user_config", desc)

    def test_data_dir_once_with_fallback(self):
        self.assertEqual(self.text.count(PLACEHOLDER), 1)
        line = next(s for s in self.text.splitlines() if PLACEHOLDER in s)
        self.assertIn("data dir is", line)
        after = self.text.split(PLACEHOLDER, 1)[1]
        rest = after.split(". ", 1)[1]
        self.assertTrue(rest.startswith(skillkit.USER_CONFIG_FALLBACKS["data_dir"]), rest[:200])
        self.assertEqual(skillkit.missing_fallbacks(self.text), [])

    def test_no_other_skill_md_by_path(self):
        self.assertEqual(skillkit.skill_md_refs(self.text, "interview-tracker"), [])


if __name__ == "__main__":
    unittest.main()
