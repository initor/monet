"""Contract tests for the hairline-create port (LAB-3).

The skill is a vendored copy of upstream hairline-create. These tests check
that the port keeps the license and the pin, that UPSTREAM.md lists every
local change, that SKILL.md runs as /monet:hairline-create with its scripts
named under ${CLAUDE_SKILL_DIR}, and that kernel.js still matches the hash
line that validate.mjs checks.
"""

import hashlib
import re
import unittest
from pathlib import Path

import skillkit

SKILL_DIR = Path(__file__).resolve().parent.parent
PIN = "bc782244216620434b14736df1d74daed2d05046"


def read(name):
    return (SKILL_DIR / name).read_text(encoding="utf-8")


def section(text, heading):
    """The lines under the `## <heading>` line, up to the next `## ` line."""
    match = re.search(r"^## " + re.escape(heading) + r"[ \t]*\n(.*?)(?=^## |\Z)", text, re.M | re.S)
    return match.group(1) if match else None


class HairlineCreatePort(unittest.TestCase):
    def test_license_and_pin_kept(self):
        license_text = read("LICENSE")
        self.assertIn("MIT License", license_text)
        self.assertIn("Lucas Marques", license_text)
        self.assertRegex(read("UPSTREAM.md"), r"at commit\s+" + PIN)

    def test_upstream_lists_local_changes(self):
        upstream = read("UPSTREAM.md")
        changes = section(upstream, "Local changes")
        self.assertIsNotNone(changes, "UPSTREAM.md has no `## Local changes` section")
        lines = [line for line in changes.splitlines() if line.startswith("- ")]
        for name in ("SKILL.md", "UPSTREAM.md", "tests/"):
            named = [line for line in lines if line.startswith(f"- `{name}`")]
            self.assertEqual(len(named), 1, f"Local changes needs one line for {name}")
        self.assertNotIn("bin/test-paint-explainer.py", upstream)
        update = section(upstream, "Update")
        self.assertIsNotNone(update, "UPSTREAM.md has no `## Update` section")
        for phrase in ("re-copy", "reapply the local changes", "`make check`"):
            self.assertIn(phrase, update)

    def test_skill_invoked_as_monet_and_uses_skill_dir(self):
        skill = skillkit.read_skill(SKILL_DIR / "SKILL.md")
        meta = skill.frontmatter
        self.assertEqual(meta["name"], "hairline-create")
        self.assertEqual(meta["argument-hint"], "[idea]")
        self.assertIn("/monet:hairline-create", meta["description"])
        self.assertNotIn(" /hairline-create", meta["description"])
        for script in ("build.mjs", "validate.mjs", "look.mjs"):
            self.assertIn("node ${CLAUDE_SKILL_DIR}/" + script, skill.body)

    def test_kernel_matches_its_hash_line(self):
        # The same check as validate.mjs: the hash on line 1 covers the rest.
        kernel = read("kernel.js").replace("\r\n", "\n").rstrip()
        first, _, rest = kernel.partition("\n")
        match = re.search(r"sha256:([0-9a-f]{64})", first)
        self.assertIsNotNone(match, "kernel.js has no sha256 hash line")
        self.assertEqual(hashlib.sha256((rest + "\n").encode("utf-8")).hexdigest(), match.group(1))

    def test_no_other_skill_md_by_path(self):
        self.assertEqual(skillkit.skill_md_refs(read("SKILL.md"), "hairline-create"), [])


if __name__ == "__main__":
    unittest.main()
