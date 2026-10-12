"""Contract tests for the pr port to the plugin layout (CORE-8)."""
import os
import re
import subprocess
import unittest
from pathlib import Path

import skillkit

SKILL = Path(__file__).resolve().parent.parent
REPO = SKILL.parent.parent
SKILL_MD = SKILL / 'SKILL.md'
WRAPPER_TOOLS = ['Read', 'Bash', 'Glob', 'Grep', 'Agent', 'AskUserQuestion']
# Built so that this file does not hold the words it scans for.
PRIVATE = re.compile(r'git-[s]hip|[W]ayne|[w]awen|~/[.]claude')


def git(*args):
    env = {k: v for k, v in os.environ.items() if not k.startswith('GIT_')}
    return subprocess.run(['git', '-C', str(REPO), *args], check=True, capture_output=True,
                          text=True, env=env, timeout=30).stdout


def section(body, heading):
    """Return the text under `## heading`, up to the next `## ` heading, with whitespace collapsed."""
    return ' '.join(body.split(f'## {heading}\n', 1)[1].split('\n## ', 1)[0].split())


class PrPort(unittest.TestCase):
    def setUp(self):
        self.skill = skillkit.read_skill(SKILL_MD)
        self.text = SKILL_MD.read_text(encoding='utf-8')

    def test_frontmatter_folds_command(self):
        meta = self.skill.frontmatter
        self.assertEqual(meta['name'], 'pr')
        self.assertEqual(meta['argument-hint'], '<pr-url> [--domain <expertise>] [--agents <N>] [--model <model>]')
        args = section(self.skill.body, 'Arguments')
        for needle in ('$ARGUMENTS', 'PR URL', '`--domain <expertise>`', '`--agents <N>`',
                       '`--model <model>`', '`agent_model`', 'Default: `opus`.'):
            self.assertIn(needle, args)
        self.assertEqual(git('ls-files', 'import/skills/pr', 'import/commands/monet/pr.md'), '')

    def test_voice_loaded_by_skill_tool(self):
        self.assertEqual(self.skill.frontmatter['allowed-tools'], WRAPPER_TOOLS + ['Skill'])
        voice = section(self.skill.body, 'Voice')
        self.assertIn('Before drafting in Step 6, load `monet:voice` with the Skill tool, '
                      'with `--no-ste` in the Skill args', voice)
        # Voice loads before the Step 6 drafting, and nothing names voice by path or maps its variables.
        self.assertLess(self.skill.body.index('## Voice'), self.skill.body.index('## Step 6'))
        self.assertNotRegex(self.text, r'voice/SKILL\.md|skills/voice|CLAUDE_SKILL_DIR')

    def test_paints_by_name(self):
        step = section(self.skill.body, 'Step 7: Visualize with monet:paint')
        self.assertIn('Invoke `monet:paint` via the Skill tool', step)
        self.assertNotRegex(self.text, r'paint/SKILL\.md')

    def test_user_config_fallbacks(self):
        self.assertNotIn('${user_config', self.text)
        self.assertEqual(skillkit.missing_fallbacks(self.skill.body), [])

    def test_names_no_other_skill_md(self):
        self.assertEqual(skillkit.skill_md_refs(self.text, 'pr'), [])

    def test_generic_text(self):
        for path in sorted(SKILL.rglob('*')):
            if path.is_file() and '__pycache__' not in path.parts:
                for n, line in enumerate(path.read_text(encoding='utf-8').splitlines(), 1):
                    self.assertIsNone(PRIVATE.search(line), f'{path}:{n}')
        self.assertIn("posted under the user's name", self.text)


if __name__ == '__main__':
    unittest.main()
