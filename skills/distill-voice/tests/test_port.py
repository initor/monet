"""Contract tests for the distill-voice port to the plugin layout (CORE-7)."""
import importlib.util
import os
import re
import subprocess
import unittest
from pathlib import Path

import skillkit

SKILL = Path(__file__).resolve().parent.parent
REPO = SKILL.parent.parent
SKILL_MD = SKILL / 'SKILL.md'
FILTER = SKILL / 'reference/agent-content-filtering.md'
DATA = '${user_config.data_dir}/personas/'
SEED = '${CLAUDE_PLUGIN_ROOT}/skills/voice/reference/personas.md'
SCRIPTS = ('extract_transcripts', 'fetch_github', 'analyze_style')
# Built at runtime, so that this file does not hold the words it scans for.
UPSTREAM = 'rel' + 'ay'
PRIVATE = re.compile('|'.join(('way' + 'ne', UPSTREAM, re.escape('~/' + '.claude'))), re.I)


def git(*args):
    env = {k: v for k, v in os.environ.items() if not k.startswith('GIT_')}
    return subprocess.run(['git', '-C', str(REPO), *args], check=True, capture_output=True,
                          text=True, env=env, timeout=30).stdout


def load(name):
    spec = importlib.util.spec_from_file_location(name, SKILL / f'scripts/{name}.py')
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class DistillVoicePort(unittest.TestCase):
    def setUp(self):
        self.skill = skillkit.read_skill(SKILL_MD)
        self.text = SKILL_MD.read_text(encoding='utf-8')
        self.flat = ' '.join(self.text.split())

    def test_frontmatter_folds_command(self):
        meta = self.skill.frontmatter
        self.assertEqual(meta['name'], 'distill-voice')
        hint = meta['argument-hint']
        for part in ('<source: path | PR #/URL | pasted text>', '--as <persona>', '[--surface <surface>]',
                     '--corpus <transcripts | github:<login> orgs:...|repos:... | dir>'):
            self.assertIn(part, hint)
        self.assertEqual(meta['allowed-tools'], ['Read', 'Write', 'Edit', 'Bash', 'Glob', 'Grep',
                                                 'WebFetch', 'AskUserQuestion'])
        self.assertTrue(meta['description'].startswith('Use when'))
        self.assertNotIn('reference/personas', meta['description'])
        self.assertIn("the user's data dir", meta['description'])
        body = ' '.join(self.skill.body.split())
        self.assertIn('Target: $ARGUMENTS', body)
        for needle in ('`--as <persona>` names the author. Required.',
                       'Ask for the persona when it is missing, rather than guessing an author.',
                       '`--surface <surface>` names the row',
                       '`--corpus <scope>` switches to corpus mode',
                       'No `--surface` in this mode, because each register',
                       'Everything else is the source'):
            self.assertIn(needle, body)
        self.assertEqual(git('ls-files', 'import/skills/distill-voice',
                             'import/commands/monet/distill-voice.md'), '')
        self.assertFalse((REPO / 'commands').exists())

    def test_writes_to_data_dir(self):
        self.assertIn(f'writes the persona to `{DATA}<name>.md` and its roster row to `{DATA}personas.md`',
                      self.flat)
        # The seed runs only when the map is missing, so it never overwrites the user's roster.
        self.assertIn(f"When that map doesn't exist yet, seed it first: create the `personas/` dir and "
                      f'copy `{SEED}` there unchanged.', self.flat)
        self.assertIn('Seed the map first when it is missing (see Where the persona goes).', self.flat)
        # Every plugin path is a script, the filter, or the seed source, never a write target.
        refs = {(var, path) for var, path, _ in skillkit.path_refs(self.text)}
        expected = {('CLAUDE_SKILL_DIR', f'scripts/{name}.py') for name in SCRIPTS}
        expected |= {('CLAUDE_SKILL_DIR', 'reference/agent-content-filtering.md'),
                     ('CLAUDE_PLUGIN_ROOT', 'skills/voice/reference/personas.md')}
        self.assertEqual(refs, expected)
        tracked = set(skillkit.files(REPO))
        for var, path in refs:
            base = SKILL if var == 'CLAUDE_SKILL_DIR' else REPO
            self.assertIn((base / path).relative_to(REPO).as_posix(), tracked, path)
        # Each script runs as `python3 ${CLAUDE_SKILL_DIR}/scripts/<name>.py`, and the filter is
        # always named by its plugin path.
        for m in re.finditer(r'scripts/(\w+)\.py', self.text):
            self.assertTrue(self.text.endswith('python3 ${CLAUDE_SKILL_DIR}/', 0, m.start()), m.group())
        for m in re.finditer(r'agent-content-filtering\.md', self.text):
            self.assertTrue(self.text.endswith('${CLAUDE_SKILL_DIR}/reference/', 0, m.start()))
        # The old write targets in this skill's own reference/ dir are gone. Only the seed source
        # still names a reference/personas path.
        self.assertNotIn('reference/personas', self.text.replace(SEED, ''))
        for name in SCRIPTS:
            self.assertTrue((SKILL / f'scripts/{name}.py').is_file(), name)

    def test_data_dir_fallback_sentence(self):
        self.assertEqual(skillkit.missing_fallbacks(self.text), [])
        # R1 says the next sentence, which is tighter than the skillkit window.
        uses = list(re.finditer(re.escape('${user_config.data_dir}'), self.flat))
        self.assertGreaterEqual(len(uses), 2)
        for use in uses:
            end = re.compile(r'[.!?]\s').search(self.flat, use.end()).end()
            self.assertTrue(self.flat.startswith(skillkit.USER_CONFIG_FALLBACKS['data_dir'], end),
                            self.flat[use.start():end + 80])

    def test_names_no_other_skill_md(self):
        self.assertEqual(skillkit.skill_md_refs(self.text, 'distill-voice'), [])

    def test_no_write_target_under_plugin(self):
        # The bundled map is named once, as the source of the seed copy. The roster row goes to the
        # map in the data dir, never to the plugin's copy.
        self.assertEqual(self.text.count(SEED), 1)
        self.assertIn('Write that one persona file, then update its roster row in the map.', self.flat)
        self.assertIn('update its row in the `## Roster` table in the map, or append one row', self.flat)
        # Every plugin path sits where it is read or run: the seed source, the filter, or a script.
        filter_ref = '${CLAUDE_SKILL_DIR}/reference/agent-content-filtering.md'
        reads = (re.escape(f'copy `{SEED}` there unchanged.'),
                 re.escape(f'agent-content filter (`{filter_ref}`) excludes what the author did not'),
                 re.escape(f'Read `{filter_ref}` and apply it.'),
                 r'`python3 \$\{CLAUDE_SKILL_DIR\}/scripts/\w+\.py --')
        spans = []
        for pattern in reads:
            found = [m.span() for m in re.finditer(pattern, self.flat)]
            self.assertTrue(found, pattern)
            spans += found
        uses = re.finditer(r'\$\{CLAUDE_(?:PLUGIN_ROOT|SKILL_DIR)\}|agent-content-filtering\.md', self.flat)
        for use in uses:
            self.assertTrue(any(start <= use.start() < end for start, end in spans),
                            self.flat[max(use.start() - 60, 0):use.end() + 60])

    def test_no_upstream_or_private_text(self):
        for path in sorted(SKILL.rglob('*')):
            if path.is_file() and '__pycache__' not in path.parts:
                text = path.read_text(encoding='utf-8')
                for n, line in enumerate(text.splitlines(), 1):
                    self.assertIsNone(PRIVATE.search(line), f'{path}:{n}')
                    self.assertNotIn('\u2014', line, f'{path}:{n}')
        for path in (SKILL_MD, FILTER):
            self.assertNotRegex(path.read_text(encoding='utf-8'), r'(?i)\b(ported|vendored|adapted)\b')
        self.assertEqual(re.findall(r'\b(my|me|My|Me|he|him|his)\b', self.text), [])

    def test_filter_reference_matches_fetcher(self):
        text = FILTER.read_text(encoding='utf-8')
        flat = ' '.join(text.split())
        tells = load('fetch_github').TELLS
        self.assertGreaterEqual(len(tells), 3)
        tells_section = text.split('## Tells\n', 1)[1].split('\n## ', 1)[0]
        listed = re.findall(r'^\| `([a-z-]+)` \|', tells_section, re.M)
        self.assertEqual(sorted(listed), sorted(tells))
        # The tells section itself says that a match sets the tag.
        self.assertIn('`agent_authored`', tells_section)
        cutoff = text.split('## The cutoff step\n', 1)[1].split('\n## ', 1)[0]
        self.assertRegex(cutoff, r'\n1\. ')
        self.assertIn('`cutoff`', cutoff)
        self.assertIn('Never quote a tagged item', flat)
        self.assertIn('`agent_authored`', flat)
        self.assertIn('`agent_tells`', flat)
        steps = ' '.join(self.skill.body.split('## Corpus pipeline', 1)[1].split('\n## ', 1)[0].split())
        self.assertIn('never quote a tagged item as a sample', steps)


if __name__ == '__main__':
    unittest.main()
