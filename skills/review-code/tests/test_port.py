"""Contract tests for the review-code port to the plugin layout (CORE-8)."""
import os
import re
import subprocess
import unittest
from pathlib import Path

import skillkit

SKILL = Path(__file__).resolve().parent.parent
REPO = SKILL.parent.parent
SKILL_MD = SKILL / 'SKILL.md'
REFERENCES = ('effective-go', 'code-review-comments', 'k8s-conventions', 'guideline-urls')
# Built so that this file does not hold the words it scans for.
PRIVATE = re.compile(r'git-[s]hip|[W]ayne|[w]awen|~/[.]claude')


def git(*args):
    env = {k: v for k, v in os.environ.items() if not k.startswith('GIT_')}
    return subprocess.run(['git', '-C', str(REPO), *args], check=True, capture_output=True,
                          text=True, env=env, timeout=30).stdout


class ReviewCodePort(unittest.TestCase):
    def setUp(self):
        self.skill = skillkit.read_skill(SKILL_MD)
        self.text = SKILL_MD.read_text(encoding='utf-8')

    def test_frontmatter_folds_command(self):
        meta = self.skill.frontmatter
        self.assertEqual(meta['name'], 'review-code')
        self.assertEqual(meta['argument-hint'], '[path/to/repo]')
        self.assertEqual(meta['allowed-tools'], ['Read', 'Bash', 'Glob', 'Grep', 'Write'])
        args = self.skill.body.split('## Arguments\n', 1)[1].split('\n## ', 1)[0]
        for needle in ('$ARGUMENTS', '`go.mod`', 'current working directory', 'stop',
                       'use it as the working directory for the review'):
            self.assertIn(needle, args)
        self.assertEqual(git('ls-files', 'import/skills/review-code', 'import/commands/monet/review-code.md'),
                         '')

    def test_reference_paths_use_skill_dir(self):
        refs = [(var, path) for var, path, _ in skillkit.path_refs(self.text)]
        tracked = set(skillkit.files(REPO))
        for name in REFERENCES:
            self.assertIn(('CLAUDE_SKILL_DIR', f'reference/{name}.md'), refs, name)
            self.assertIn(f'skills/review-code/reference/{name}.md', tracked, name)
        # Every mention of a reference file goes through the skill dir.
        bare = self.text.replace('${CLAUDE_SKILL_DIR}/reference/', '')
        self.assertEqual(re.findall(r'reference/[\w-]+\.md', bare), [])

    def test_names_no_other_skill_md(self):
        self.assertEqual(skillkit.skill_md_refs(self.text, 'review-code'), [])
        self.assertNotIn('${user_config', self.text)
        self.assertEqual(skillkit.missing_fallbacks(self.skill.body), [])

    def test_generic_text(self):
        for path in sorted(SKILL.rglob('*')):
            if path.is_file() and '__pycache__' not in path.parts:
                for n, line in enumerate(path.read_text(encoding='utf-8').splitlines(), 1):
                    self.assertIsNone(PRIVATE.search(line), f'{path}:{n}')


if __name__ == '__main__':
    unittest.main()
