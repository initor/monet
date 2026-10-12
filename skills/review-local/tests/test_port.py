"""Contract tests for the review-local port to the plugin layout (CORE-8)."""
import os
import re
import subprocess
import unittest
from pathlib import Path

import skillkit

SKILL = Path(__file__).resolve().parent.parent
REPO = SKILL.parent.parent
SKILL_MD = SKILL / 'SKILL.md'
# Built so that this file does not hold the words it scans for.
PRIVATE = re.compile(r'git-[s]hip|[W]ayne|[w]awen|~/[.]claude')


def git(*args):
    env = {k: v for k, v in os.environ.items() if not k.startswith('GIT_')}
    return subprocess.run(['git', '-C', str(REPO), *args], check=True, capture_output=True,
                          text=True, env=env, timeout=30).stdout


def section(body, heading):
    """Return the text under `## heading`, up to the next `## ` heading, with whitespace collapsed."""
    return ' '.join(body.split(f'## {heading}\n', 1)[1].split('\n## ', 1)[0].split())


class ReviewLocalPort(unittest.TestCase):
    def setUp(self):
        self.skill = skillkit.read_skill(SKILL_MD)
        self.text = SKILL_MD.read_text(encoding='utf-8')

    def test_frontmatter_folds_command(self):
        meta = self.skill.frontmatter
        self.assertEqual(meta['name'], 'review-local')
        self.assertEqual(meta['argument-hint'], '[--domain <expertise>] [--agents <N>] [--model <model>]')
        self.assertEqual(meta['allowed-tools'], ['Read', 'Bash', 'Glob', 'Grep', 'Agent', 'AskUserQuestion'])
        args = section(self.skill.body, 'Arguments')
        for needle in ('$ARGUMENTS', '`--domain <expertise>`', '`--agents <N>`', '`--model <model>`',
                       'Default: `opus`.', '`agent_model`', '(default: 3)'):
            self.assertIn(needle, args)
        self.assertEqual(git('ls-files', 'import/skills/review-local',
                             'import/commands/monet/review-local.md'), '')

    def test_paints_by_name(self):
        step = section(self.skill.body, 'Step 6: Visualize with monet:paint')
        self.assertIn('Invoke `monet:paint` via the Skill tool', step)
        self.assertNotRegex(self.text, r'paint/SKILL\.md')

    def test_suggests_ship(self):
        step = section(self.skill.body, 'Step 8: Iterate (interactive)')
        self.assertIn('suggest using `monet:ship` to commit and create the PR', step)
        self.assertNotIn('git-' + 'ship', self.text)

    def test_names_no_other_skill_md(self):
        self.assertEqual(skillkit.skill_md_refs(self.text, 'review-local'), [])
        self.assertNotIn('${user_config', self.text)
        self.assertEqual(skillkit.missing_fallbacks(self.skill.body), [])

    def test_generic_text(self):
        for path in sorted(SKILL.rglob('*')):
            if path.is_file() and '__pycache__' not in path.parts:
                for n, line in enumerate(path.read_text(encoding='utf-8').splitlines(), 1):
                    self.assertIsNone(PRIVATE.search(line), f'{path}:{n}')


if __name__ == '__main__':
    unittest.main()
