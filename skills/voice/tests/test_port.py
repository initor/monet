"""Contract tests for the voice port to the plugin layout (CORE-2, CORE-5, CORE-6)."""
import os
import re
import subprocess
import unittest
from pathlib import Path

import skillkit

SKILL = Path(__file__).resolve().parent.parent
REPO = SKILL.parent.parent
SKILL_MD = SKILL / 'SKILL.md'
MAP = SKILL / 'reference/personas.md'
SAMPLE = SKILL / 'reference/personas/sample.md'
DATA = '${user_config.data_dir}/personas/'
BUNDLED = '${CLAUDE_SKILL_DIR}/reference/'
# Built so that this file does not hold the words it scans for.
PRIVATE = re.compile(r'[w]ayne|lessons[.]md|~/[.]claude', re.I)


def git(*args):
    env = {k: v for k, v in os.environ.items() if not k.startswith('GIT_')}
    return subprocess.run(['git', '-C', str(REPO), *args], check=True, capture_output=True,
                          text=True, env=env, timeout=30).stdout


def table(text, heading):
    """Return the body rows of the first table under `## heading`, as lists of cells."""
    section = text.split(f'## {heading}\n', 1)[1].split('\n## ', 1)[0]
    rows = [line.strip().strip('|').split('|') for line in section.splitlines() if line.startswith('|')]
    return [[cell.strip() for cell in row] for row in rows[2:]]


class VoicePort(unittest.TestCase):
    def setUp(self):
        self.skill = skillkit.read_skill(SKILL_MD)
        self.text = SKILL_MD.read_text(encoding='utf-8')

    def test_frontmatter_folds_command(self):
        meta = self.skill.frontmatter
        self.assertEqual(meta['name'], 'voice')
        for flag in ('--check', '--no-ste', '--persona <name>', '--list'):
            self.assertIn(flag, meta['argument-hint'])
        self.assertEqual(meta['allowed-tools'], ['Read', 'Bash', 'Glob', 'Grep', 'Edit', 'AskUserQuestion'])
        body = self.skill.body
        self.assertIn('$ARGUMENTS', body)
        for flag in ('`--check`', '`--no-ste`', '`--persona <name>`', '`--list`'):
            self.assertIn(flag, body)
        self.assertIn("Editing a file in place needs the user's explicit confirmation first", body)
        self.assertEqual(git('ls-files', 'import/skills/voice', 'import/commands/monet/voice.md',
                             'import/output-styles', 'import/bin/test-ste-check.py'), '')

    def test_persona_from_data_dir(self):
        body = self.skill.body
        steps = body.split('## Preconditions\n', 1)[1].split('\n## ', 1)[0]
        for name in ('personas.md', 'personas/<name>.md'):
            data = steps.index(DATA + name.replace('personas/', ''))
            self.assertLess(data, steps.index(BUNDLED + name), name)
        self.assertIn('`--persona sample`', steps)
        self.assertIn('`sample.md`', steps)
        self.assertTrue(SAMPLE.is_file())
        listing = body.split('With `--list`', 1)[1].split('\n\n', 1)[0]
        self.assertIn('data dir', listing)
        self.assertIn(BUNDLED + 'personas/<name>.md', listing)
        self.assertIn('Name a roster entry as missing only when neither file exists.',
                      ' '.join(listing.split()))

    def test_bundled_map_maps_no_surface(self):
        text = MAP.read_text(encoding='utf-8')
        surfaces = table(text, 'Surface map')
        self.assertGreaterEqual(len(surfaces), 9)
        self.assertEqual({row[1] for row in surfaces}, {'none'})
        roster = table(text, 'Roster')
        self.assertEqual([row[:2] for row in roster], [['`sample`', '`personas/sample.md`']])
        self.assertEqual(git('ls-files', 'skills/voice/reference/personas/').split(),
                         ['skills/voice/reference/personas/sample.md'])

    def test_sample_persona_sections(self):
        lines = SAMPLE.read_text(encoding='utf-8').splitlines()
        self.assertEqual(lines[0], '# Persona: sample')
        self.assertEqual(lines[1], 'Hand-written synthetic example. Not written by `distill-voice`.')
        heads = [line[3:] for line in lines if line.startswith('## ')]
        self.assertEqual(heads, ['Sources', 'Computed metrics', 'Register rows evidenced', 'Moves',
                                 'Signature phrases'])
        text = '\n'.join(lines)
        sources = text.split('## Sources\n', 1)[1].split('\n## ', 1)[0]
        self.assertIn('Every sample', sources)
        self.assertIn('synthetic', sources)
        moves = text.split('## Moves\n', 1)[1].split('\n## ', 1)[0].split('\n### ')[1:]
        self.assertGreaterEqual(len(moves), 2)
        for move in moves:
            self.assertRegex(move, r'\n- Sample: "[^"]+"', move.splitlines()[0])

    def test_no_owner_name(self):
        for root in (SKILL, REPO / 'output-styles'):
            for path in sorted(root.rglob('*')):
                if path.is_file() and '__pycache__' not in path.parts:
                    for n, line in enumerate(path.read_text(encoding='utf-8').splitlines(), 1):
                        self.assertIsNone(PRIVATE.search(line), f'{path}:{n}')
        self.assertEqual(re.findall(r'\b(he|him|his)\b', self.text), [])
        self.assertEqual(re.findall(r'\b(my|me|My|Me)\b', self.text), [])
        self.assertIn('`output-styles/ste-lite.md` at the plugin root', self.text)
        card = (SKILL / 'reference/ste-lite.md').read_text(encoding='utf-8').splitlines()
        self.assertEqual(card[19], "A draft that goes out under the user's name follows the voice skill instead.")

    def test_data_dir_fallback_sentence(self):
        self.assertIn('${user_config.data_dir}', self.text)
        self.assertEqual(skillkit.missing_fallbacks(self.text), [])
        # R1 says the next sentence, which is tighter than the skillkit window.
        flat = ' '.join(self.text.split())
        uses = list(re.finditer(re.escape('${user_config.data_dir}'), flat))
        self.assertEqual(len(uses), 2)
        for use in uses:
            end = re.compile(r'[.!?]\s').search(flat, use.end()).end()
            self.assertTrue(flat.startswith(skillkit.USER_CONFIG_FALLBACKS['data_dir'], end),
                            flat[use.start():end + 80])

    def test_names_no_other_skill_md(self):
        self.assertEqual(skillkit.skill_md_refs(self.text, 'voice'), [])

    def test_callers_load_by_skill_tool(self):
        section = self.skill.body.split('## How other skills load this one\n', 1)[1].split('\n## ', 1)[0]
        section = ' '.join(section.split())
        for needle in ('Skill tool', '`monet:voice`', 'never reads this file by path', '`--no-ste`',
                       '`git-draft-pr`', '`pr`', '`paint`'):
            self.assertIn(needle, section)
        self.assertNotIn("this skill's directory", self.skill.body)
        self.assertNotRegex(section, r'(?i)\b(variables?|maps?|mapping|reader)\b')
        self.assertNotRegex(self.skill.body, r'(?i)\bvariables?\b')


    def test_every_caller_stays_on_plain_voice(self):
        # A paragraph in another skill that loads monet:voice through the Skill tool must pass
        # --no-ste, and this skill must name that caller where it says how callers load it.
        section = self.skill.body.split('## How other skills load this one\n', 1)[1].split('\n## ', 1)[0]
        section = ' '.join(section.split())
        callers = set()
        for path in sorted((REPO / 'skills').glob('*/**/*.md')):
            name = path.relative_to(REPO / 'skills').parts[0]
            if name == 'voice':
                continue
            for para in path.read_text(encoding='utf-8').split('\n\n'):
                if 'Skill tool' in para and '`monet:voice`' in para:
                    callers.add(name)
                    with self.subTest(file=str(path.relative_to(REPO))):
                        self.assertIn('--no-ste', para)
        self.assertTrue(callers, 'no skill loads monet:voice')
        for name in sorted(callers):
            with self.subTest(caller=name):
                self.assertIn(f'`{name}`', section)

if __name__ == '__main__':
    unittest.main()
