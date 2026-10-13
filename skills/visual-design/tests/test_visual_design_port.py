"""Contract tests for the visual-design port to the plugin layout (LAB-2)."""
import os
import re
import subprocess
import unittest
from pathlib import Path

import skillkit

SKILL = Path(__file__).resolve().parents[1]
REPO = SKILL.parent.parent
SKILL_MD = SKILL / 'SKILL.md'
VERIFY = SKILL / 'reference/verify-and-ship.md'
OPTIONS = SKILL / 'reference/options-page.md'
ROOT = '${CLAUDE_PLUGIN_ROOT}'
OWN = '${CLAUDE_SKILL_DIR}'
EM_DASH = chr(0x2014)
DESCRIPTION = (
    'Use when designing something visual that the owner picks by eye, such as a logo or mark, a '
    'figure, the look of a page, or an HTML overview like a system architecture page, worked out '
    'over rounds of options. For a one-pass page from session findings, use paint. For one Hairline '
    'figure, use hairline-create. For an interactive system design whiteboard, use sd-playground.')
TOOLS = ['Read', 'Write', 'Edit', 'Bash', 'Glob', 'Grep', 'Agent', 'Skill', 'AskUserQuestion']
# The acceptance grep, with each literal split by a char class so this file does not match it.
PRIVATE = re.compile(r'W[a]yne|w[a]wen|~/[.]claude|~/[.]config|git-s[h]ip')
FIRST_PERSON = re.compile(r'\bI\b|\b(?i:me|my)\b')


def git(*args):
    env = {k: v for k, v in os.environ.items() if not k.startswith('GIT_')}
    return subprocess.run(['git', '-C', str(REPO), *args], check=True, capture_output=True,
                          text=True, env=env, timeout=30).stdout


def docs():
    """SKILL.md and every .md file under reference/."""
    return [SKILL_MD, *sorted((SKILL / 'reference').rglob('*.md'))]


def flat(text):
    return ' '.join(text.split())


def section(body, heading):
    """The body of the `## heading` section, joined onto one line."""
    return flat(body.split(f'\n## {heading}\n', 1)[1].split('\n## ', 1)[0])


def step(body, n):
    """The numbered item `n.` of the Steps section, joined onto one line."""
    steps = body.split('\n## Steps\n', 1)[1].split('\n## ', 1)[0]
    return flat(re.search(rf'^{n}\. (.*?)(?=^\d\. |\Z)', steps, re.M | re.S).group(1))


class VisualDesignPort(unittest.TestCase):
    def setUp(self):
        self.skill = skillkit.read_skill(SKILL_MD)
        self.text = SKILL_MD.read_text(encoding='utf-8')

    def test_command_folded(self):
        meta = self.skill.frontmatter
        self.assertEqual(meta['name'], 'visual-design')
        self.assertEqual(meta['description'], DESCRIPTION)
        self.assertEqual(meta['argument-hint'], '[what to design, for whom] [work folder to resume]')
        self.assertEqual(meta['allowed-tools'], TOOLS)
        args = section(self.skill.body, 'Arguments')
        for needle in ('Arguments: $ARGUMENTS', 'what to design', 'for whom',
                       'If they are empty, ask', 'work folder', 'resume'):
            self.assertIn(needle, args)
        # The command's report step closes the body.
        head, _, last = self.skill.body.rstrip().rsplit('\n## ', 1)[1].partition('\n')
        self.assertEqual(head.strip(), 'Report')
        last = flat(last)
        for needle in ('the work folder', 'the round', 'its option IDs', 'what waits on the owner'):
            self.assertIn(needle, last)
        self.assertEqual(git('ls-files', 'import/skills/visual-design',
                             'import/commands/monet/visual-design.md'), '')
        for gone in ('import/skills/visual-design', 'import/commands/monet/visual-design.md',
                     'commands'):
            self.assertFalse((REPO / gone).exists(), gone)

    def test_paint_base_by_plugin_path(self):
        text = flat(OPTIONS.read_text(encoding='utf-8'))
        for ref in (f'`{ROOT}/skills/paint/reference/base.md`',
                    f'`{ROOT}/skills/paint/reference/flavors/minimal.md`'):
            self.assertIn(ref, text)
        for path in docs():
            self.assertNotIn('../', path.read_text(encoding='utf-8'), path)
        # Own files resolve against the skill dir, and no bare `reference/` path is left.
        for name in ('options-page.md', 'rounds.md', 'verify-and-ship.md'):
            self.assertIn(f'`{OWN}/reference/{name}', self.text)
        self.assertNotRegex(self.text, r'[`(]reference/')
        # One line names both tokens next to their values, for files read with a plain Read.
        names = [line for line in self.text.splitlines()
                 if re.search(r'(?<!\{)CLAUDE_PLUGIN_ROOT\b', line) and f'`{ROOT}`' in line
                 and re.search(r'(?<!\{)CLAUDE_SKILL_DIR\b', line) and f'`{OWN}`' in line]
        self.assertEqual(len(names), 1, names)

    def test_generic_text_and_no_dotfiles_paths(self):
        for path in sorted(SKILL.rglob('*')):
            if not path.is_file() or path.suffix not in ('.md', '.py'):
                continue
            text = path.read_text(encoding='utf-8')
            self.assertNotIn(EM_DASH, text, path)
            if path.suffix != '.md':
                continue
            for n, line in enumerate(text.splitlines(), 1):
                self.assertIsNone(PRIVATE.search(line), f'{path}:{n}')
            self.assertEqual(skillkit.dotfiles_paths(text), [], path)
        # Folded text speaks of the owner or the user, never I, me, or my. A quoted span
        # is what someone says, so it is left out.
        for path in docs():
            text = re.sub(r'"[^"\n]*"', '""', path.read_text(encoding='utf-8'))
            self.assertNotRegex(text, FIRST_PERSON, path)

    def test_hands_off_by_monet_name(self):
        seven = step(self.skill.body, 7)
        ship = flat(VERIFY.read_text(encoding='utf-8').split('\n## Ship\n', 1)[1])
        for text in (seven, ship):
            self.assertIn('load it with the Skill tool as `monet:voice`', text)
            self.assertIn('`monet:handoff`', text)
            self.assertIn('through `/monet:ship`', text)
            self.assertNotRegex(text, r'(?<![:\w-])`(voice|handoff)`')

    def test_no_other_skill_md_by_path(self):
        for path in docs():
            self.assertEqual(skillkit.skill_md_refs(path.read_text(encoding='utf-8'), 'visual-design'),
                             [], path)


if __name__ == '__main__':
    unittest.main()
