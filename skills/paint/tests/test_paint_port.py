"""Contract tests for the paint port to the plugin layout (LAB-1)."""
import os
import re
import subprocess
import unittest
from pathlib import Path

import skillkit

SKILL = Path(__file__).resolve().parents[1]
REPO = SKILL.parent.parent
SKILL_MD = SKILL / 'SKILL.md'
EXPLAINER = SKILL / 'reference/explainer.md'
ROOT = '${CLAUDE_PLUGIN_ROOT}'
OWN = '${CLAUDE_SKILL_DIR}'
HAIRLINE = ROOT + '/skills/hairline-create'
EM_DASH = chr(0x2014)
TEXT_SUFFIXES = {'.md', '.py', '.js', '.mjs'}
DESCRIPTION = (
    'Use when the user asks to visualize, "paint", or turn session research/analysis/data into a '
    'shareable self-contained HTML page, or wants a visual summary of findings. With --sheet, it '
    'makes one dense image instead, as HTML plus a PNG. With --explainer, it teaches one concept to '
    'a newcomer, with one Hairline figure and plain words. For a visual the owner picks by eye in '
    'rounds, use visual-design.')
# The acceptance grep, with each literal split by a char class so this file does not match it.
PRIVATE = re.compile(r'W[a]yne|w[a]wen|~/[.]claude|~/[.]config|[.][.]/voice|[.][.]/hairline-create')


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


def step(text, n):
    """The numbered item `n.` of the explainer's order of work, joined onto one line."""
    work = text.split('## Order of work\n', 1)[1].split('\n## ', 1)[0]
    return flat(re.search(rf'^{n}\. (.*?)(?=^\d\. |\Z)', work, re.M | re.S).group(1))


class PaintPort(unittest.TestCase):
    def setUp(self):
        self.skill = skillkit.read_skill(SKILL_MD)
        self.text = SKILL_MD.read_text(encoding='utf-8')
        self.explainer = EXPLAINER.read_text(encoding='utf-8')

    def test_command_folded(self):
        meta = self.skill.frontmatter
        self.assertEqual(meta['name'], 'paint')
        self.assertEqual(meta['description'], DESCRIPTION)
        self.assertEqual(meta['argument-hint'],
                         '[topic or focus] [--flavor showcase|minimal|technical] [--sheet | --explainer]')
        self.assertEqual(meta['allowed-tools'], ['Read', 'Bash', 'Write', 'AskUserQuestion', 'Skill'])
        body = self.skill.body
        args = section(body, 'Arguments')
        for needle in ('Arguments: $ARGUMENTS', 'A topic or focus',
                       'If it is empty, infer the dominant content.',
                       '`--flavor showcase|minimal|technical` forces a flavor', '`--sheet` switches',
                       '`--explainer` to the explainer layout'):
            self.assertIn(needle, args)
        # The command's report step closes the body.
        head, _, last = body.rstrip().rsplit('\n## ', 1)[1].partition('\n')
        self.assertEqual(head.strip(), 'Report')
        last = flat(last)
        for needle in ('filename', 'the PNG for a sheet or an explainer', 'section, panel, or step count',
                       'flavor'):
            self.assertIn(needle, last)
        self.assertEqual(git('ls-files', 'import/skills/paint', 'import/commands/monet/paint.md',
                             'import/bin/test-paint-explainer.py'), '')
        for gone in ('import/skills/paint', 'import/commands/monet/paint.md',
                     'import/bin/test-paint-explainer.py', 'commands'):
            self.assertFalse((REPO / gone).exists(), gone)

    def test_voice_and_hairline_by_plugin_path(self):
        text = self.explainer
        for ref in (f'`{HAIRLINE}`', f'`{HAIRLINE}/concepts.md`', f'`{HAIRLINE}/rules.md`',
                    f'`{ROOT}/skills/voice/reference/ste-lite.md`',
                    f'`python3 {ROOT}/skills/voice/scripts/ste_check.py',
                    f'`node {ROOT}/skills/paint/scripts/explainer.mjs <page.html> <name>.js`'):
            self.assertIn(ref, flat(text))
        for path in docs():
            self.assertNotIn('../', path.read_text(encoding='utf-8'), path)
        # Own files resolve against the skill dir.
        for name in ('reference/base.md', 'reference/sheet.md', 'reference/explainer.md',
                     'reference/flavors/<flavor>.md', 'scripts/explainer.mjs'):
            self.assertIn(f'`{OWN}/{name}`', self.text)
        # One line names both tokens next to their values, for files read with a plain Read.
        names = [line for line in self.text.splitlines()
                 if re.search(r'(?<!\{)CLAUDE_PLUGIN_ROOT\b', line) and f'`{ROOT}`' in line
                 and re.search(r'(?<!\{)CLAUDE_SKILL_DIR\b', line) and f'`{OWN}`' in line]
        self.assertEqual(len(names), 1, names)

    def test_no_dotfiles_paths_or_name(self):
        self.assertIn("under the user's name", flat(self.text))
        for path in docs():
            text = path.read_text(encoding='utf-8')
            for n, line in enumerate(text.splitlines(), 1):
                self.assertIsNone(PRIVATE.search(line), f'{path}:{n}')
            self.assertEqual(skillkit.dotfiles_paths(text), [], path)
        for path in sorted(SKILL.rglob('*')):
            if path.is_file() and path.suffix in TEXT_SUFFIXES:
                self.assertNotIn(EM_DASH, path.read_text(encoding='utf-8'), path)
        tests = (SKILL / 'tests/test_paint_explainer.py').read_text(encoding='utf-8')
        self.assertIn("PAINT = Path(__file__).resolve().parents[1]", tests)
        self.assertIn("HAIRLINE = PAINT.parent / 'hairline-create'", tests)
        self.assertNotIn('.claude/', tests)

    def test_voice_and_hairline_loaded_as_monet_skills(self):
        body = flat(self.skill.body)
        self.assertIn('Before writing prose, load it with the Skill tool as `monet:voice`', body)
        self.assertIn('`--no-ste`', body)
        for n in (3, 4):
            self.assertIn('Skill tool as `monet:hairline-create`', step(self.explainer, n), n)
        # hairline-create's own paths arrive expanded when the Skill tool loads it, so no
        # line maps them by hand.
        for path in docs():
            for n, line in enumerate(path.read_text(encoding='utf-8').splitlines(), 1):
                self.assertFalse('hairline-create' in line and 'CLAUDE_SKILL_DIR' in line,
                                 f'{path}:{n}')

    def test_no_skill_placeholders_left(self):
        for path in sorted(SKILL.rglob('*.md')):
            text = path.read_text(encoding='utf-8')
            # Each needle is split so a whole-dir grep for the placeholder does not match this file.
            for placeholder in ('<this' + ' skill>', '<voice' + ' skill>'):
                self.assertNotIn(placeholder, text, path)

    def test_no_other_skill_md_by_path(self):
        for path in docs():
            self.assertEqual(skillkit.skill_md_refs(path.read_text(encoding='utf-8'), 'paint'), [], path)


if __name__ == '__main__':
    unittest.main()
