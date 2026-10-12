"""Tests for the corpus scripts of distill-voice: synthetic fixtures, temp dirs, and a fake gh."""
import ast
import contextlib
import importlib.util
import io
import json
import os
import subprocess
import sys
import tempfile
import time
import types
import unittest
from pathlib import Path
from unittest import mock

SKILL = Path(__file__).resolve().parent.parent
SCRIPTS = SKILL / 'scripts'
EM_DASH = '\u2014'

# The fake gh logs its args, decodes the endpoint, and answers from routes.json. A route can
# fail its first `rate_limit` calls the way gh does when GitHub throttles it.
FAKE_GH = '''import json, os, re, sys, urllib.parse
state = os.environ['FAKE_GH_DIR']
args = sys.argv[1:]
with open(os.path.join(state, 'calls.log'), 'a') as f:
    f.write(json.dumps(args) + '\\n')
endpoint = urllib.parse.unquote_plus(args[-1]) if args else ''
routes = json.load(open(os.path.join(state, 'routes.json')))
hits_path = os.path.join(state, 'hits.json')
hits = json.load(open(hits_path)) if os.path.exists(hits_path) else {}
for i, route in enumerate(routes):
    if re.search(route['match'], endpoint):
        n = hits.get(str(i), 0)
        hits[str(i)] = n + 1
        json.dump(hits, open(hits_path, 'w'))
        if n < route.get('rate_limit', 0):
            sys.stderr.write('gh: API Rate Limit exceeded for user ID 1. (HTTP 403)\\n')
            sys.exit(1)
        if 'error' in route:
            sys.stderr.write(route['error'] + '\\n')
            sys.exit(1)
        sys.stdout.write(json.dumps(route['body']))
        sys.exit(0)
sys.stderr.write('gh: Not Found (HTTP 404)\\n')
sys.exit(1)
'''


def load(name):
    spec = importlib.util.spec_from_file_location(name, SCRIPTS / f'{name}.py')
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def run(name, *args, env=None):
    return subprocess.run([sys.executable, str(SCRIPTS / f'{name}.py'), *map(str, args)],
                          capture_output=True, text=True, timeout=60, env=env)


class TempDir(unittest.TestCase):
    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.tmp = Path(self._tmp.name)

    def tearDown(self):
        self._tmp.cleanup()


def user(text=None, content=None, **extra):
    """A synthetic user record in the Claude Code transcript shape."""
    record = {'type': 'user', 'isSidechain': False, 'uuid': 'u', 'sessionId': 's',
              'message': {'role': 'user', 'content': text if content is None else content}}
    record.update(extra)
    return record


class ExtractTranscripts(TempDir):
    def write(self, rel, *records):
        path = self.tmp / 'projects' / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        with path.open('a', encoding='utf-8') as f:
            for record in records:
                f.write((record if isinstance(record, str) else json.dumps(record)) + '\n')

    def extract(self, projects=None, env=None):
        out = self.tmp / 'out/messages.txt'
        result = run('extract_transcripts', '--projects-dir', projects or self.tmp / 'projects',
                     '--out', out, env=env)
        self.assertEqual(result.returncode, 0, result.stderr)
        text = out.read_text(encoding='utf-8')
        return (text.rstrip('\n').split('\n---\n') if text else []), result.stderr

    def test_keeps_only_typed_user_turns(self):
        self.write('proj-a/s1.jsonl', user('first typed turn'),
                   {'type': 'assistant', 'message': {'role': 'assistant',
                                                     'content': [{'type': 'text', 'text': 'agent reply'}]}},
                   {'type': 'system', 'content': 'system note'},
                   {'type': 'attachment', 'message': {'role': 'user', 'content': 'attached text'}},
                   {'type': 'file-history-snapshot', 'snapshot': {}},
                   user(content=[{'type': 'text', 'text': 'second typed turn'}]))
        self.write('proj-b/s2.jsonl', user('  third, in another project  '))
        messages, _ = self.extract()
        self.assertEqual(messages, ['first typed turn', 'second typed turn', 'third, in another project'])

    def test_drops_sidechain_meta_and_injected_records(self):
        self.write('proj/s.jsonl',
                   user('sidechain', isSidechain=True), user('not sidechain', isSidechain=False),
                   user('meta', isMeta=True), user('not meta', isMeta=False),
                   user('tool result', toolUseResult={'stdout': 'x'}),
                   user('source tool', sourceToolUseID='toolu_1'),
                   user('compact', isCompactSummary=True),
                   user('agent message', origin={'from': 'agent-1'}),
                   user('task message', origin={'senderTaskId': 't1'}),
                   user('tool made', origin={'producer': 'hook'}),
                   user('plain origin', origin={'kind': 'keyboard'}))
        messages, _ = self.extract()
        self.assertEqual(messages, ['not sidechain', 'not meta', 'plain origin'])

    def test_keeps_only_text_blocks(self):
        self.write('proj/s.jsonl',
                   user(content=[{'type': 'tool_result', 'tool_use_id': 't', 'content': 'tool output'},
                                 {'type': 'text', 'text': 'kept text'},
                                 {'type': 'image', 'source': {'type': 'base64', 'data': 'AAAA'}}]),
                   user(content=[{'type': 'tool_result', 'tool_use_id': 't', 'content': 'only a result'}]),
                   user(content=[{'type': 'image', 'source': {}}]),
                   # Only the block type decides: a non-text block with a text string is still dropped.
                   user(content=[{'type': 'tool_result', 'tool_use_id': 't', 'text': 'result with a text key'},
                                 {'type': 'image', 'text': 'image caption'}]),
                   user(content=[{'type': 'text', 'text': 'one record'}, {'type': 'text', 'text': 'two blocks'}]))
        messages, _ = self.extract()
        self.assertEqual(messages, ['kept text', 'one record\n\ntwo blocks'])

    def test_drops_command_and_reminder_text(self):
        self.write('proj/s.jsonl',
                   user('<command-name>/clear</command-name>'),
                   user('<command-message>clear</command-message>'),
                   user('<local-command-stdout>done</local-command-stdout>'),
                   user('<system-reminder>be brief</system-reminder>'),
                   user('\n <system-reminder>indented</system-reminder>'),
                   user(content=[{'type': 'text', 'text': '<system-reminder>ctx</system-reminder>'},
                                 {'type': 'text', 'text': 'the typed part'}]),
                   user('a text that names <command-name> later'))
        messages, _ = self.extract()
        self.assertEqual(messages, ['the typed part', 'a text that names <command-name> later'])

    def test_drops_text_the_cli_inserts(self):
        self.write('proj/s.jsonl',
                   user('<bash-input>ls -la</bash-input>'),
                   user('<bash-stdout>total 0</bash-stdout><bash-stderr></bash-stderr>'),
                   user('<user-prompt-submit-hook>checked</user-prompt-submit-hook>'),
                   user('<command-'),
                   user('<local-command-'),
                   # A space after the tag name ends it as well as `>` does.
                   user('<task-notification id="1">done</task-notification>'),
                   user('[Request interrupted by user]'),
                   user('[Request interrupted by user for tool use]'),
                   user('<B>not a lowercase tag</B>'),
                   user('<3 thanks'),
                   # A lowercase word after `<` is a tag only when a space or `>` ends it.
                   user('<https://example.com/pr/1> can you look'),
                   user('<user@example.com> wrote this'),
                   user('[Request a review] from the team'))
        messages, _ = self.extract()
        self.assertEqual(messages, ['<B>not a lowercase tag</B>', '<3 thanks', '<https://example.com/pr/1> can you look',
                                    '<user@example.com> wrote this', '[Request a review] from the team'])

    def test_fence_closes_only_on_a_matching_fence(self):
        body = lambda *lines: '\n'.join(('dump:',) + lines)
        filler = lambda n: ['x'] * n
        # Each block below runs 26 lines unless a line inside it closes the fence early.
        longer = body('````', *filler(15), '```', *filler(10), '````')
        other_char = body('```', *filler(15), '~~~', *filler(10), '```')
        info = body('```', *filler(15), '```python', *filler(10), '```')
        # Four spaces of indent make an indented code line, not a fence opener.
        indented = body('    ```', *filler(25))
        tabbed = body('\t```', *filler(25))
        # A backtick fence's info string holds no backtick, so a one-line code span opens no fence.
        inline = body('```npm test``` fails', *filler(25))
        closed_indented = body('```', *filler(5), '   ```  ', *filler(25))
        self.write('proj/s.jsonl',
                   *(user(t) for t in (longer, other_char, info, indented, tabbed, inline, closed_indented)))
        messages, _ = self.extract()
        self.assertEqual(messages, [indented, tabbed, inline, closed_indented])

    def test_drops_pasted_dumps(self):
        fence = lambda n: '\n'.join(['look at this:', '```'] + ['code line'] * n + ['```'])
        self.write('proj/s.jsonl',
                   user('a' * 2000), user('b' * 2001),
                   user('\n'.join(['forty'] * 40)), user('\n'.join(['forty-one'] * 41)),
                   user(fence(20)), user(fence(21)),
                   user('\n'.join(['open fence:', '~~~'] + ['x'] * 21)))
        messages, _ = self.extract()
        self.assertEqual(messages, ['a' * 2000, '\n'.join(['forty'] * 40), fence(20)])

    def test_longest_of_several_fences_decides(self):
        block = lambda n, close=True: ['```'] + ['code line'] * n + (['```'] if close else [])
        text = lambda *blocks: '\n'.join(['dump:'] + [line for b in blocks for line in b + ['then:']])
        # A long block is not hidden by a short one after it, closed or not.
        long_then_short = text(block(21), block(1))
        long_then_unclosed = text(block(25), block(1, close=False))
        # Each block counts from zero: 5 and 16 lines are two blocks under the limit, not one of 21.
        short_then_short = text(block(5), block(16))
        self.write('proj/s.jsonl', *(user(t) for t in (long_then_short, long_then_unclosed, short_then_short)))
        messages, _ = self.extract()
        self.assertEqual(messages, [short_then_short])

    def test_skips_subagents_dir(self):
        self.write('proj/s1.jsonl', user('main session'))
        self.write('proj/s1/subagents/agent-1.jsonl', user('subagent prompt'))
        self.write('proj/s1/s1.jsonl', user('nested deeper'))
        messages, _ = self.extract()
        self.assertEqual(messages, ['main session'])
        # Pointed at a session dir, the glob reaches subagents/ itself, and it is still skipped.
        messages, _ = self.extract(projects=self.tmp / 'projects/proj/s1')
        self.assertEqual(messages, [])

    def test_counts_malformed_lines(self):
        # A line nested deep enough to raise RecursionError in json is malformed, not a crash.
        self.write('proj/s.jsonl', user('before'), '{not json', '', '[1, 2]', '"text"', '[' * 1_000_000,
                   user('after'))
        # A byte order mark at the start of a file does not cost the first line.
        (self.tmp / 'projects/proj/bom.jsonl').write_text('\ufeff' + json.dumps(user('after a bom')) + '\n',
                                                          encoding='utf-8')
        messages, stderr = self.extract()
        self.assertEqual(messages, ['after a bom', 'before', 'after'])
        self.assertIn('4 malformed lines skipped', stderr)

    def test_reads_a_session_with_bad_utf8(self):
        # A session that is still being written can end mid-line, here inside a multi-byte character.
        # A bad byte reads as U+FFFD, and the cut final line counts as malformed.
        path = self.tmp / 'projects/proj/s.jsonl'
        path.parent.mkdir(parents=True)
        path.write_bytes(json.dumps(user('before')).encode() + b'\n'
                         + b'{"type": "user", "message": {"content": "bad \xff byte"}}\n'
                         + b'{"type": "user", "message": {"content": "caf\xc3')
        messages, stderr = self.extract()
        self.assertEqual(messages, ['before', 'bad \ufffd byte'])
        self.assertIn('1 malformed lines skipped', stderr)

    def test_lone_surrogate_does_not_crash(self):
        # A writer that cuts a surrogate pair in half leaves a lone `\ud83d` escape in valid JSON.
        self.write('proj/s.jsonl', user('before'), user('cut emoji \ud83d here'), user('after'))
        out = self.tmp / 'out/messages.txt'
        out.parent.mkdir()
        out.write_text('an earlier run\n', encoding='utf-8')
        result = run('extract_transcripts', '--projects-dir', self.tmp / 'projects', '--out', out)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertNotIn('Traceback', result.stderr)
        # The lone half is written as `?`, and every message reaches --out.
        self.assertEqual(out.read_text(encoding='utf-8'), 'before\n---\ncut emoji ? here\n---\nafter\n')

    def test_default_projects_dir_comes_from_home(self):
        source = (SCRIPTS / 'extract_transcripts.py').read_text(encoding='utf-8')
        self.assertNotIn('~/' + '.claude', source)
        home = self.tmp / 'home'
        path = home / '.claude/projects/proj/s.jsonl'
        path.parent.mkdir(parents=True)
        path.write_text(json.dumps(user('from the home dir')) + '\n', encoding='utf-8')
        out = self.tmp / 'messages.txt'
        result = run('extract_transcripts', '--out', out, env={**os.environ, 'HOME': str(home)})
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(out.read_text(encoding='utf-8'), 'from the home dir\n')


def search(*items):
    return {'total_count': len(items), 'incomplete_results': False, 'items': list(items)}


def pr(repo, number, author, body='', title='t', created='2024-02-01T10:00:00Z'):
    return {'number': number, 'title': title, 'body': body, 'user': {'login': author},
            'html_url': f'https://github.com/{repo}/pull/{number}', 'created_at': created,
            'repository_url': f'https://api.github.com/repos/{repo}', 'pull_request': {}}


def note(author, body, created='2024-03-01T00:00:00Z', url='https://github.com/x/y/pull/1#c', **extra):
    return {'user': {'login': author}, 'body': body, 'created_at': created, 'html_url': url, **extra}


CLAUDE_FOOTER = '\U0001f916 Generated with [Claude Code](https://claude.com/claude-code)'
TEMPLATE = '## Summary\n- fix the flaky test\n\n## Test plan\n- [x] ran it'


class FetchGithub(TempDir):
    def gh(self, routes):
        bin_dir, state = self.tmp / 'bin', self.tmp / 'gh-state'
        bin_dir.mkdir(exist_ok=True)
        state.mkdir(exist_ok=True)
        fake = bin_dir / 'gh'
        fake.write_text(f'#!{sys.executable}\n' + FAKE_GH, encoding='utf-8')
        fake.chmod(0o755)
        (state / 'routes.json').write_text(json.dumps(routes), encoding='utf-8')
        for leftover in ('calls.log', 'hits.json'):
            (state / leftover).unlink(missing_ok=True)
        return {**os.environ, 'PATH': f'{bin_dir}{os.pathsep}{os.environ["PATH"]}', 'FAKE_GH_DIR': str(state)}

    def calls(self):
        log = self.tmp / 'gh-state/calls.log'
        return [json.loads(line) for line in log.read_text().splitlines()] if log.exists() else []

    def fetch(self, env, *args):
        return run('fetch_github', '--user', 'octo', '--out', self.tmp / 'out', '--backoff-base', '0', *args,
                   env=env)

    def read(self, name):
        return json.loads((self.tmp / 'out' / name).read_text(encoding='utf-8'))

    def test_writes_tagged_records_with_fake_gh(self):
        own = pr('acme/widgets', 1, 'octo', 'Adds a cache in front of the widget store.')
        other = pr('acme/widgets', 7, 'someone')
        gadget = pr('acme/gadgets', 3, 'someone')
        env = self.gh([
            {'match': r'^search/issues\?q=.* author:octo', 'body': search(
                own, pr('acme/widgets', 2, 'octo', TEMPLATE + '\n\n' + CLAUDE_FOOTER))},
            {'match': r'^search/issues\?q=.* commenter:octo', 'body': search(other, own)},
            {'match': r'^search/issues\?q=.* reviewed-by:octo', 'body': search(other, gadget)},
            {'match': r'^repos/acme/widgets/issues/7/comments\?', 'body': [
                note('octo', 'looks good, one nit below'), note('someone', 'thanks'),
                note('octo', 'after the window', created='2024-08-01T00:00:00Z'),
                note('octo', 'before the window', created='2023-12-31T23:00:00Z')]},
            {'match': r'^repos/acme/widgets/pulls/7/comments\?', 'body': [
                note('octo', 'this loop never exits, see https://claude.ai/code/session_01abc', path='a.py')]},
            {'match': r'^repos/acme/widgets/pulls/7/reviews\?', 'body': [
                {'user': {'login': 'octo'}, 'body': '', 'submitted_at': '2024-03-02T00:00:00Z'},
                {'user': {'login': 'octo'}, 'body': 'Approving with one nit.', 'state': 'APPROVED',
                 'submitted_at': '2024-03-02T00:00:00Z', 'html_url': 'https://github.com/acme/widgets/pull/7#r'}]},
            {'match': r'^repos/acme/widgets/issues/1/comments\?', 'body': [note('octo', 'fixed in the next commit')]},
            {'match': r'^repos/acme/gadgets/pulls/3/reviews\?', 'body': [
                {'user': {'login': 'OCTO'}, 'body': 'LGTM', 'submitted_at': '2024-04-01T00:00:00Z'}]},
            {'match': r'^repos/[^?]+/(comments|reviews)\?', 'body': []},
        ])
        result = self.fetch(env, '--orgs', 'acme,acme-labs', '--since', '2024-01-01', '--until', '2024-06-30')
        self.assertEqual(result.returncode, 0, result.stderr)
        descriptions = self.read('descriptions.json')
        self.assertEqual([(d['repo'], d['number'], d['agent_authored'], d['agent_tells']) for d in descriptions],
                         [('acme/widgets', 1, False, []), ('acme/widgets', 2, True, ['attribution', 'template'])])
        self.assertEqual(descriptions[0]['body'], 'Adds a cache in front of the widget store.')
        self.assertEqual(descriptions[0]['url'], 'https://github.com/acme/widgets/pull/1')
        comments = self.read('comments.json')
        got = sorted((c['repo'], c['number'], c['kind'], c['own_pr'], c['body'], c['agent_authored'])
                     for c in comments)
        self.assertEqual(got, sorted([
            ('acme/widgets', 7, 'issue_comment', False, 'looks good, one nit below', False),
            ('acme/widgets', 7, 'review_comment', False,
             'this loop never exits, see https://claude.ai/code/session_01abc', True),
            ('acme/widgets', 7, 'review', False, 'Approving with one nit.', False),
            ('acme/widgets', 1, 'issue_comment', True, 'fixed in the next commit', False),
            ('acme/gadgets', 3, 'review', False, 'LGTM', False)]))
        for record in descriptions + comments:
            self.assertIs(type(record['agent_authored']), bool)
        tagged = [c for c in comments if c['agent_authored']]
        self.assertEqual(tagged[0]['agent_tells'], ['session-link'])
        calls = self.calls()
        self.assertTrue(calls)
        self.assertTrue(all(call[0] == 'api' for call in calls), calls)
        queries = [json.dumps(call) for call in calls if call[-1].startswith('search/issues')]
        for org in ('acme', 'acme-labs'):
            self.assertTrue(any(f'org%3A{org}+' in q or f'org%3A{org}&' in q for q in queries), org)
        self.assertTrue(any('created%3A2024-01-01..2024-06-30' in q for q in queries), queries)
        # Each PR's comment endpoints are fetched once, though two searches found it.
        pr7 = [c for c in calls if c[-1].startswith('repos/acme/widgets/issues/7/comments')]
        self.assertEqual(len(pr7), 1)

    def test_window_includes_its_first_and_last_day(self):
        notes = [note('octo', 'last second of the until day', created='2024-06-30T23:59:59Z'),
                 note('octo', 'first second of the since day', created='2024-01-01T00:00:00Z'),
                 note('octo', 'the day before', created='2023-12-31T23:59:59Z'),
                 note('octo', 'the day after', created='2024-07-01T00:00:00Z')]
        env = self.gh([
            {'match': r'^search/issues\?q=.* commenter:octo', 'body': search(pr('acme/widgets', 7, 'someone'))},
            {'match': r'^search/issues', 'body': search()},
            {'match': r'^repos/acme/widgets/issues/7/comments\?', 'body': notes},
            # A review comment is dated when it was written, not when it was last edited.
            {'match': r'^repos/acme/widgets/pulls/7/comments\?', 'body': [
                note('octo', 'review comment edited later', created='2023-12-31T23:59:59Z',
                     updated_at='2024-03-05T00:00:00Z'),
                note('octo', 'review comment in the window', created='2024-03-03T00:00:00Z',
                     updated_at='2024-07-01T00:00:00Z')]},
            # A review has no created_at. It is dated when it was submitted.
            {'match': r'^repos/acme/widgets/pulls/7/reviews\?', 'body': [
                {'user': {'login': 'octo'}, 'body': 'review before the window',
                 'submitted_at': '2023-12-31T23:59:59Z'},
                {'user': {'login': 'octo'}, 'body': 'review in the window',
                 'submitted_at': '2024-03-02T00:00:00Z'}]},
            {'match': r'^repos/[^?]+/(comments|reviews)\?', 'body': []}])
        result = self.fetch(env, '--repos', 'acme/widgets', '--since', '2024-01-01', '--until', '2024-06-30')
        self.assertEqual(result.returncode, 0, result.stderr)
        # Records come out in date order, each with the date it was written.
        self.assertEqual([(c['kind'], c['body'], c['created_at']) for c in self.read('comments.json')],
                         [('issue_comment', 'first second of the since day', '2024-01-01T00:00:00Z'),
                          ('review', 'review in the window', '2024-03-02T00:00:00Z'),
                          ('review_comment', 'review comment in the window', '2024-03-03T00:00:00Z'),
                          ('issue_comment', 'last second of the until day', '2024-06-30T23:59:59Z')])
        # A one-day window is valid, and it keeps that day.
        result = self.fetch(self.gh([
            {'match': r'^search/issues\?q=.* author:octo', 'body': search(
                pr('acme/widgets', 2, 'octo', 'Later.', created='2024-03-01T18:00:00Z'),
                pr('acme/widgets', 1, 'octo', 'Earlier.', created='2024-03-01T09:00:00Z'))},
            {'match': r'^search/issues', 'body': search()}]),
            '--repos', 'acme/widgets', '--since', '2024-03-01', '--until', '2024-03-01')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue(any('created%3A2024-03-01..2024-03-01' in c[-1] for c in self.calls()))
        self.assertEqual([(d['number'], d['created_at']) for d in self.read('descriptions.json')],
                         [(1, '2024-03-01T09:00:00Z'), (2, '2024-03-01T18:00:00Z')])

    def test_repos_scope_uses_repo_qualifier(self):
        env = self.gh([{'match': r'^search/issues', 'body': search()}])
        result = self.fetch(env, '--repos', 'acme/widgets,acme/gadgets', '--since', '2024-01-01')
        self.assertEqual(result.returncode, 0, result.stderr)
        queries = ' '.join(call[-1] for call in self.calls())
        self.assertIn('created%3A%3E%3D2024-01-01', queries)
        self.assertIn('updated%3A%3E%3D2024-01-01', queries)
        self.assertIn('repo%3Aacme%2Fwidgets', queries)
        self.assertIn('repo%3Aacme%2Fgadgets', queries)
        self.assertNotIn('org%3A', queries)
        self.assertEqual(self.read('descriptions.json'), [])
        self.assertEqual(self.read('comments.json'), [])

    def test_until_alone_bounds_created(self):
        env = self.gh([{'match': r'^search/issues', 'body': search()}])
        result = self.fetch(env, '--repos', 'acme/widgets', '--until', '2024-06-30')
        self.assertEqual(result.returncode, 0, result.stderr)
        authored = [c[-1] for c in self.calls() if 'author%3Aocto' in c[-1]]
        self.assertEqual(len(authored), 1)
        self.assertIn('created%3A%3C%3D2024-06-30', authored[0])

    def test_reads_every_page(self):
        first = {'total_count': 101, 'items': [pr('acme/widgets', n, 'octo', f'Body {n}.') for n in range(1, 101)]}
        env = self.gh([
            {'match': r'^search/issues\?q=.* author:octo.*&page=1$', 'body': first},
            {'match': r'^search/issues\?q=.* author:octo.*&page=2$',
             'body': {'total_count': 101, 'items': [pr('acme/widgets', 101, 'octo', 'Body 101.')]}},
            {'match': r'^search/issues\?q=.* commenter:octo.*&page=1$', 'body': search(pr('acme/widgets', 7, 'x'))},
            {'match': r'^search/issues', 'body': search()},
            {'match': r'^repos/acme/widgets/issues/7/comments\?per_page=100&page=1$',
             'body': [note('octo', f'comment {n}') for n in range(100)]},
            {'match': r'^repos/acme/widgets/issues/7/comments\?per_page=100&page=2$', 'body': [note('octo', 'last')]},
            {'match': r'^repos/[^?]+/(comments|reviews)\?', 'body': []},
        ])
        result = self.fetch(env, '--repos', 'acme/widgets')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(len(self.read('descriptions.json')), 101)
        self.assertEqual(len(self.read('comments.json')), 101)
        self.assertEqual(len([c for c in self.calls() if 'author%3Aocto' in c[-1]]), 2)
        self.assertEqual(len([c for c in self.calls() if c[-1].startswith('repos/acme/widgets/issues/7/')]), 2)

    def test_accepts_managed_user_login(self):
        env = self.gh([{'match': r'^search/issues', 'body': search()}])
        result = run('fetch_github', '--user', 'octo_acme', '--out', self.tmp / 'out', '--backoff-base', '0',
                     '--repos', 'acme/widgets,octo_acme/dotfiles', env=env)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue(any('author%3Aocto_acme' in c[-1] for c in self.calls()))
        self.assertTrue(any('repo%3Aocto_acme%2Fdotfiles' in c[-1] for c in self.calls()))

    def test_matches_the_login_in_any_case(self):
        # GitHub search ignores case, so --user Octo finds the PRs of octo. The notes and the
        # PR author must then match it in any case too, or every note is dropped silently.
        env = self.gh([
            {'match': r'(?i)^search/issues\?q=.* author:octo', 'body': search(pr('acme/widgets', 1, 'octo'))},
            {'match': r'(?i)^search/issues\?q=.* commenter:octo',
             'body': search(pr('acme/widgets', 1, 'octo'), pr('acme/widgets', 7, 'someone'))},
            {'match': r'^search/issues', 'body': search()},
            {'match': r'^repos/acme/widgets/issues/1/comments\?', 'body': [note('octo', 'fixed in the next commit')]},
            {'match': r'^repos/acme/widgets/issues/7/comments\?', 'body': [note('Octo', 'one nit below')]},
            {'match': r'^repos/[^?]+/(comments|reviews)\?', 'body': []},
        ])
        result = run('fetch_github', '--user', 'Octo', '--out', self.tmp / 'out', '--backoff-base', '0',
                     '--repos', 'acme/widgets', env=env)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue(any('author%3AOcto' in c[-1] for c in self.calls()), self.calls())
        self.assertEqual([d['number'] for d in self.read('descriptions.json')], [1])
        self.assertEqual(sorted((c['number'], c['own_pr'], c['body']) for c in self.read('comments.json')),
                         [(1, True, 'fixed in the next commit'), (7, False, 'one nit below')])

    def test_retries_rate_limit(self):
        env = self.gh([{'match': r'^search/issues\?q=.* author:octo', 'rate_limit': 2,
                        'body': search(pr('acme/widgets', 1, 'octo', 'A plain body.'))},
                       {'match': r'^search/issues', 'body': search()}])
        result = self.fetch(env, '--repos', 'acme/widgets')
        self.assertEqual(result.returncode, 0, result.stderr)
        authored = [c for c in self.calls() if 'author%3Aocto' in c[-1]]
        self.assertEqual(len(authored), 3)
        self.assertEqual(result.stderr.lower().count('rate limit'), 2)
        self.assertEqual([d['number'] for d in self.read('descriptions.json')], [1])
        # gh exiting 0 is a success, though its stderr says rate limit.
        fetch = load('fetch_github')
        ok = subprocess.CompletedProcess(['gh'], 0, '[]', 'gh: API rate limit is close\n')
        with mock.patch.object(fetch.subprocess, 'run', return_value=ok) as gh:
            self.assertEqual(fetch.gh_api('search/issues', types.SimpleNamespace(max_retries=2, backoff_base=0)), [])
        self.assertEqual(gh.call_count, 1)

    def test_gives_up_after_retries_and_on_other_errors(self):
        env = self.gh([{'match': r'^search/issues', 'rate_limit': 99, 'body': search()}])
        result = self.fetch(env, '--repos', 'acme/widgets', '--max-retries', '2')
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(len(self.calls()), 3)
        self.assertIn('rate limit', result.stderr.lower())
        env = self.gh([{'match': r'^search/issues', 'error': 'gh: Server Error (HTTP 502)'}])
        result = self.fetch(env, '--repos', 'acme/widgets')
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(len(self.calls()), 1)
        self.assertIn('HTTP 502', result.stderr)
        self.assertFalse((self.tmp / 'out/descriptions.json').exists())
        for routes in ([{'match': r'^search/issues', 'body': [1, 2]}],
                       [{'match': r'^search/issues\?q=.* commenter:', 'body': search(pr('acme/widgets', 7, 'x'))},
                        {'match': r'^search/issues', 'body': search()},
                        {'match': r'^repos/', 'body': {'message': 'not a list'}}]):
            result = self.fetch(self.gh(routes), '--repos', 'acme/widgets')
            self.assertEqual(result.returncode, 1, result.stderr)
            self.assertIn('fetch_github: gh api', result.stderr)
            self.assertNotIn('Traceback', result.stderr)
        # Output nested too deep for json is an error, not a crash.
        fetch = load('fetch_github')
        deep = subprocess.CompletedProcess(['gh'], 0, '[' * 1_000_000, '')
        with mock.patch.object(fetch.subprocess, 'run', return_value=deep):
            with self.assertRaisesRegex(fetch.GhError, 'is not JSON'):
                fetch.gh_api('search/issues', types.SimpleNamespace(max_retries=0, backoff_base=0))

    def test_backoff_doubles_from_the_default_base(self):
        fetch = load('fetch_github')
        limited = subprocess.CompletedProcess(['gh'], 1, '', 'gh: API rate limit exceeded (HTTP 403)\n')
        for extra, expected in (([], [30, 60, 120, 240, 480, 960]),
                                # A longest wait of exactly one day is allowed.
                                (['--backoff-base', '43200', '--max-retries', '2'], [43200, 86400]),
                                # With no retries there is no wait, so any base is allowed.
                                (['--backoff-base', '1e10', '--max-retries', '0'], [])):
            sleeps = []
            with mock.patch.object(fetch.subprocess, 'run', return_value=limited) as gh, \
                    mock.patch.object(fetch.time, 'sleep', sleeps.append), \
                    mock.patch.object(fetch.shutil, 'which', return_value='/usr/bin/gh'), \
                    contextlib.redirect_stderr(io.StringIO()):
                code = fetch.main(['--user', 'octo', '--repos', 'acme/widgets', '--out', str(self.tmp / 'out'),
                                   *extra])
            self.assertEqual(code, 1)
            self.assertEqual(sleeps, expected)
            self.assertEqual(gh.call_count, len(expected) + 1)
            self.assertEqual(gh.call_args.args[0][:2], ['gh', 'api'])

    def test_tells(self):
        tells = load('fetch_github').tells
        self.assertEqual(tells('Fixes the cache.\n\nCo-Authored-By: Claude <noreply@anthropic.com>'),
                         ['attribution'])
        self.assertEqual(tells('Generated by Copilot'), ['attribution'])
        self.assertEqual(tells(CLAUDE_FOOTER), ['attribution'])
        self.assertEqual(tells('\U0001f916 done'), ['attribution'])
        self.assertEqual(tells('see https://chatgpt.com/share/abc'), ['session-link'])
        self.assertEqual(tells(TEMPLATE), ['template'])
        self.assertEqual(tells('## Summary\nonly a summary'), [])
        self.assertEqual(tells('We generated the fixtures with a script. Claude is a name.'), [])
        self.assertEqual(tells(None), [])


def write_json(path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(data), encoding='utf-8')


NOTES_A = '\n'.join([
    "It is a test. It's short and plain; it works.",
    '',
    "**Bold** start here. we don't stop.",
    '',
    '| **Label** | value; with semicolon |',
    '|---|---|',
    '| **Row** | x |',
    '',
    '```bash',
    'echo "it\'s code; ignore"',
    '```',
    '',
    f'See [the docs](https://example.com/a;b) {EM_DASH} now.',
])
NOTES_B = 'Alpha ' + ' '.join(['beta'] * 30) + '.'
LONG = ' '.join(['Long'] + ['word'] * 59) + f' {EM_DASH} done.'
METRICS = ('words {}, em-dashes per 1000 words {}, semicolons per 1000 words {}, contraction rate {}, '
           'median sentence length {} (p90 {}, max {}), sentences over 30 words {}, '
           'bold spans per 1000 words {}, lowercase sentence-start rate {}')


class AnalyzeStyle(TempDir):
    def test_prints_register_metrics_and_baseline(self):
        github = self.tmp / 'github'
        write_json(github / 'descriptions.json', [
            {'body': 'Adds a cache. It is fast.', 'agent_authored': False},
            {'body': f'## Summary\nA {EM_DASH} B; C.', 'agent_authored': True}])
        write_json(github / 'comments.json', [
            {'body': "can't merge this yet; the build is red.", 'own_pr': False, 'agent_authored': False},
            {'body': 'Done.', 'own_pr': True, 'agent_authored': False},
            {'body': f'Nice {EM_DASH} very nice; ship it.', 'own_pr': False, 'agent_authored': True}])
        transcripts = self.tmp / 'messages.txt'
        transcripts.write_text('\n---\n'.join(['hey can you fix the build', "it's broken again, the test fails",
                                               LONG]) + '\n', encoding='utf-8')
        (self.tmp / 'notes').mkdir()
        (self.tmp / 'notes/a.md').write_text(NOTES_A, encoding='utf-8')
        (self.tmp / 'notes/b.md').write_text(NOTES_B, encoding='utf-8')
        result = run('analyze_style', '--github-dir', github, '--transcripts', transcripts,
                     '--text', f'notes={self.tmp}/notes/*.md')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout.splitlines(), [
            'pr-descriptions: items 1, tagged skipped 1, '
            + METRICS.format(6, '0.0', '0.0', '0% (0 of 1)', 3, 3, 3, 0, '0.0', '0% (0 of 2)'),
            'pr-review: items 1, tagged skipped 1, '
            + METRICS.format(8, '0.0', '125.0', '100% (1 of 1)', 8, 8, 8, 0, '0.0', '100% (1 of 1)'),
            'pr-replies: items 1, tagged skipped 0, '
            + METRICS.format(1, '0.0', '0.0', 'n/a (0 of 0)', 1, 1, 1, 0, '0.0', '0% (0 of 1)'),
            'transcripts: items 3, tagged skipped 0, '
            + METRICS.format(73, '13.7', '0.0', '100% (1 of 1)', 6, 61, 61, 1, '0.0', '67% (2 of 3)'),
            'notes: items 2, tagged skipped 0, '
            + METRICS.format(51, '19.6', '19.6', '67% (2 of 3)', 4, 31, 31, 1, '19.6', '17% (1 of 6)'),
            'baseline (short only): items 7, '
            + METRICS.format(78, '12.8', '25.6', '67% (4 of 6)', 4, 8, 31, 1, '12.8', '33% (4 of 12)'),
        ])

    def test_baseline_takes_items_of_at_most_60_words(self):
        for name, n in (('sixty', 60), ('sixty-one', 61)):
            (self.tmp / f'{name}.md').write_text(' '.join(['Word'] * n) + '.', encoding='utf-8')
        result = run('analyze_style', '--text', f'a={self.tmp}/sixty.md', '--text', f'b={self.tmp}/sixty-one.md')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue(result.stdout.splitlines()[-1].startswith('baseline (short only): items 1, words 60,'),
                        result.stdout)

    def test_prints_n_a_for_an_empty_register_and_baseline(self):
        # Every item tagged leaves the register empty, and one long file leaves the baseline empty.
        write_json(self.tmp / 'github/descriptions.json', [{'body': 'Generated text.', 'agent_authored': True}])
        (self.tmp / 'long.md').write_text(' '.join(['Plain'] + ['word'] * 78) + '.', encoding='utf-8')
        result = run('analyze_style', '--github-dir', self.tmp / 'github', '--text', f'long={self.tmp}/long.md')
        self.assertEqual(result.returncode, 0, result.stderr)
        empty = METRICS.format(0, 'n/a', 'n/a', 'n/a (0 of 0)', 'n/a', 'n/a', 'n/a', 0, 'n/a', 'n/a (0 of 0)')
        self.assertEqual(result.stdout.splitlines(), [
            'pr-descriptions: items 0, tagged skipped 1, ' + empty,
            'long: items 1, tagged skipped 0, '
            + METRICS.format(79, '0.0', '0.0', 'n/a (0 of 0)', 79, 79, 79, 1, '0.0', '0% (0 of 1)'),
            'baseline (short only): items 0, ' + empty,
        ])

    def test_trailing_emoji_or_emoticon(self):
        # A part after the last sentence end with no word character in it is not a sentence.
        style = load('analyze_style')
        for text, words in (('Thanks. \U0001f44d', 1), ('Done. :)', 1), ('Ship it. -', 2)):
            stats = style.Stats()
            self.assertEqual(stats.add(text), words, text)
            self.assertEqual((stats.lengths, stats.starts), ([words], (0, 1)), text)
        # The script still prints its metrics for a register of such items.
        (self.tmp / 'a.md').write_text('Thanks. \U0001f44d', encoding='utf-8')
        (self.tmp / 'b.md').write_text('Done. :)', encoding='utf-8')
        result = run('analyze_style', '--text', f'notes={self.tmp}/*.md')
        self.assertEqual(result.returncode, 0, result.stderr)
        metrics = METRICS.format(2, '0.0', '0.0', 'n/a (0 of 0)', 1, 1, 1, 0, '0.0', '0% (0 of 2)')
        self.assertEqual(result.stdout.splitlines(), ['notes: items 2, tagged skipped 0, ' + metrics,
                                                      'baseline (short only): items 2, ' + metrics])

    def test_reads_files_that_are_not_utf8(self):
        # An older note saved as Latin-1 is read as it is: the bad byte becomes U+FFFD, not a traceback.
        (self.tmp / 'old.md').write_bytes(b'Caf\xe9 notes. It is fine.')
        (self.tmp / 'messages.txt').write_bytes(b'caf\xe9 again\n---\nit works\n')
        result = run('analyze_style', '--transcripts', self.tmp / 'messages.txt', '--text', f'old={self.tmp}/old.md')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertNotIn('Traceback', result.stderr)
        lines = result.stdout.splitlines()
        self.assertEqual([line.split(':', 1)[0] for line in lines], ['transcripts', 'old', 'baseline (short only)'])
        self.assertTrue(lines[0].startswith('transcripts: items 2, tagged skipped 0, words '), lines[0])
        self.assertTrue(lines[1].startswith('old: items 1, tagged skipped 0, words '), lines[1])

    def test_counts_contractions_bold_lists_and_headings(self):
        stats = load('analyze_style').Stats()
        words = stats.add("## A heading\nThe team's plan. It's done. Let's go. We do not stop.\n\n"
                          '- first item\n- __second__ **item**')
        self.assertEqual(words, 15)
        self.assertEqual(stats.lengths, [3, 2, 2, 4, 2, 2])
        self.assertEqual((stats.contracted, stats.full, stats.bold), (2, 1, 2))
        self.assertEqual(stats.starts, (2, 6))

    def test_counts_typographic_apostrophes(self):
        stats = load('analyze_style').Stats()
        # Smart quotes from an editor count the same as straight ones.
        self.assertEqual(stats.add('We don\u2019t stop. It\u2019s done. We\u2019ll see.'), 7)
        self.assertEqual((stats.contracted, stats.full), (3, 0))

    def test_counts_every_contraction_suffix(self):
        stats = load('analyze_style').Stats()
        # SKILL.md Step 3 lists the suffixes as (s|t|re|ll|ve|d|m). The other test covers s, t, and ll.
        self.assertEqual(stats.add("We're close. I'm on it. I've seen it. I'd ship it."), 11)
        self.assertEqual((stats.contracted, stats.full), (4, 0))

    def test_ends_a_sentence_at_a_question_or_exclamation_mark(self):
        stats = load('analyze_style').Stats()
        self.assertEqual(stats.add('Why does it fail? It reads twice! Fix it.'), 9)
        self.assertEqual((stats.lengths, stats.starts), ([4, 3, 2], (0, 3)))

    def test_numbered_and_star_lists_start_blocks(self):
        stats = load('analyze_style').Stats()
        # Each list line starts a block, and its marker is not a word or a sentence.
        self.assertEqual(stats.add('Steps:\n1. Run the tests\n2) Open it\n* adds a cache\n+ drops it'), 13)
        self.assertEqual((stats.lengths, stats.starts), ([1, 3, 2, 3, 2], (2, 5)))

    def test_strips_code_tables_links_and_comments(self):
        style = load('analyze_style')
        prose = style.prose('Run `a; b` now <!-- hidden; note --> and [read](https://x.test/a;b) '
                            'https://y.test/c;d &amp; done. See [docs](./a;b.md) too.\n'
                            '| **x;** | y |\n```\nz;\n```\n~~~\nq;\n~~~')
        self.assertNotIn(';', prose)
        self.assertNotIn('**', prose)
        self.assertIn('read', prose)
        self.assertEqual(style.WORD.findall(prose), ['Run', '0', 'now', 'and', 'read', 'done', 'See', 'docs', 'too'])

    def test_strips_quotes_tags_and_tables_without_outer_pipes(self):
        style = load('analyze_style')
        # A quote is another writer's words, so only the reply is measured.
        stats = style.Stats()
        self.assertEqual(stats.add(f'> **Potential issue** {EM_DASH} it never exits; add a guard.\n\nFixed.'), 1)
        self.assertEqual((stats.em_dashes, stats.semicolons, stats.bold, stats.lengths), (0, 0, 0, [1]))
        # A quote line ends the block above it, though no blank line follows.
        stats = style.Stats()
        stats.add('Before it\n> quoted; text\nmy reply.')
        self.assertEqual((stats.semicolons, stats.lengths, stats.starts), (0, [2, 2], (1, 2)))
        # Tags and their attributes are markup, not words or sentences.
        stats = style.Stats()
        self.assertEqual(stats.add('Fixes the layout.\n\n<img width="640" alt="Screenshot at 10 00" '
                                   'src="https://github.com/user-attachments/assets/abc"><br/>\n'
                                   '<details>\n\nSee the run.\n</details>'), 6)
        self.assertEqual((stats.lengths, stats.starts), ([3, 3], (0, 2)))
        # GFM makes the outer pipes optional. The header row and the body rows go, and the
        # table ends at the first line with no `|`.
        stats = style.Stats()
        self.assertEqual(stats.add('Key | Value\n:--- | ---:\n**Label** | value; x\nmore | rows\n'
                                   'After it.\nThen a | b.'), 5)
        self.assertEqual((stats.bold, stats.semicolons, stats.lengths), (0, 0, [2, 3]))
        # A delimiter row with no `|` line above it has no header row, so the prose above it stays.
        self.assertEqual(style.prose('Results so far:\n|---|---|\n| 1 | 2 |\nDone.'), 'Results so far:\n\n\nDone.')
        # A line with a `|` and no delimiter row below it is prose.
        self.assertEqual(style.prose('Use a | b; then c.\nIt works.'), 'Use a | b; then c.\nIt works.')
        # A tab makes an indented code line, not a heading, as for fences and quotes.
        self.assertEqual(style.prose('\t# Kept; here.'), '\t# Kept; here.')
        # A rule line with no `|` starts no table.
        self.assertEqual(style.prose('Intro.\n---\nUse a | b.'), 'Intro.\n---\nUse a | b.')

    def test_prose_fence_closes_only_on_a_matching_fence(self):
        prose = load('analyze_style').prose
        for text in ('````\n```\nin; code\n```\n````\nAfter.',
                     '```\n~~~\nin; code\n~~~\n```\nAfter.',
                     '```\n```python\nin; code\n```\nAfter.'):
            self.assertEqual(prose(text).split(), ['After.'], text)
        # Four spaces of indent do not open a fence, so the text after it stays prose.
        self.assertEqual(prose('    ```\nKept; here.').split(), ['```', 'Kept;', 'here.'])
        self.assertEqual(prose('\t```\nKept; here.').split(), ['```', 'Kept;', 'here.'])
        # A one-line code span opens no fence, so the lines after it stay prose.
        self.assertEqual(prose('```npm test``` fails.\nIt is red.').split()[-3:], ['It', 'is', 'red.'])

    def test_scans_stay_linear_on_unclosed_marks(self):
        style = load('analyze_style')
        stats = style.Stats()
        start = time.monotonic()
        stats.add('.' * 40000 + 'a')
        stats.add('[' * 40000 + 'a')
        stats.add('<!--' * 20000 + 'a')
        # The scans that restart at each mark took about 20 s here in all.
        self.assertLess(time.monotonic() - start, 2)
        self.assertEqual(style.prose('a <!-- one --> b <!-- two --> c <!-- open'), 'a   b   c <!-- open')

    def test_median_is_mid_value_and_rate_counts_letter_starts(self):
        style = load('analyze_style')
        stats = style.Stats()
        stats.add('One two. Three four five six. 42 is a number. Seven eight nine ten.')
        self.assertEqual(stats.lengths, [2, 4, 4, 4])
        self.assertEqual(stats.starts, (0, 3))
        self.assertIn('median sentence length 4 (p90 4, max 4)', stats.metrics())
        stats.add('Yes it is.')
        self.assertIn('median sentence length 4 (p90 4, max 4)', stats.metrics())
        stats = style.Stats()
        stats.add('One two three. One.')
        self.assertIn('median sentence length 2 (p90 3, max 3)', stats.metrics())

    def test_counts_only_sentences_over_30_words(self):
        stats = load('analyze_style').Stats()
        stats.add(' '.join(['Word'] * 30) + '. ' + ' '.join(['Word'] * 31) + '.')
        self.assertEqual(stats.lengths, [30, 31])
        self.assertIn('sentences over 30 words 1,', stats.metrics())

    def test_bold_scan_is_linear_on_a_long_line(self):
        stats = load('analyze_style').Stats()
        start = time.monotonic()
        stats.add('**a ' * 20000)
        # A span that may cross other `**` marks rescans the line from each one, about 8 s here.
        self.assertLess(time.monotonic() - start, 2)
        self.assertEqual(stats.bold, 0)

    def test_skips_a_body_that_is_not_text(self):
        write_json(self.tmp / 'github/descriptions.json', [{'body': 42}, {'body': 'Plain text here.'}])
        result = run('analyze_style', '--github-dir', self.tmp / 'github')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue(result.stdout.startswith('pr-descriptions: items 2, tagged skipped 0, words 3,'),
                        result.stdout)


class Usage(TempDir):
    def assertUsage(self, result):
        self.assertEqual(result.returncode, 2, result.stderr)
        self.assertIn('usage:', result.stderr)

    def test_bad_arguments_print_usage(self):
        # No gh on PATH: a fetch_github guard that lets a bad argument through exits 1, never reaches GitHub.
        no_gh = {**os.environ, 'PATH': str(self.tmp / 'no-bin')}
        fetch = lambda *args: run('fetch_github', '--user', 'octo', *args, env=no_gh)
        self.assertUsage(run('extract_transcripts'))
        self.assertUsage(run('extract_transcripts', '--out', self.tmp / 'o', '--projects-dir', self.tmp / 'none'))
        self.assertUsage(fetch('--out', self.tmp))
        self.assertUsage(fetch('--out', self.tmp, '--orgs', 'a', '--repos', 'a/b'))
        self.assertUsage(fetch('--out', self.tmp, '--repos', 'nodash'))
        self.assertUsage(fetch('--out', self.tmp, '--orgs', 'a', '--since', '2024-13-01'))
        self.assertUsage(run('extract_transcripts', '--out', self.tmp, '--projects-dir', self.tmp))
        self.assertUsage(run('fetch_github', '--user', 'not a login', '--out', self.tmp, '--orgs', 'a', env=no_gh))
        for base in ('nan', 'inf', '-1'):
            self.assertUsage(fetch('--out', self.tmp, '--orgs', 'a', '--backoff-base', base))
        # A longest wait over one day is refused, where time.sleep would raise past about 9.2e9 s.
        for wait in (('--backoff-base', '86401', '--max-retries', '1'), ('--backoff-base', '1e10'),
                     ('--max-retries', '100'), ('--max-retries', '10000000000')):
            self.assertUsage(fetch('--out', self.tmp, '--orgs', 'a', *wait))
        self.assertUsage(fetch('--out', self.tmp, '--orgs', 'a', '--since', '2024-07-01', '--until', '2024-06-30'))
        # With no retries left, every search would read as empty and the run would write an empty corpus.
        self.assertUsage(fetch('--out', self.tmp, '--orgs', 'a', '--max-retries', '-1'))
        # An --out that names a file, or a path under one, fails before any gh call or read.
        (self.tmp / 'file').write_text('', encoding='utf-8')
        result = fetch('--out', self.tmp / 'file', '--orgs', 'a')
        self.assertUsage(result)
        self.assertIn('is a file', result.stderr)
        result = fetch('--out', self.tmp / 'file/sub', '--orgs', 'a')
        self.assertUsage(result)
        self.assertIn('is under a file', result.stderr)
        result = run('extract_transcripts', '--out', self.tmp / 'file/messages.txt', '--projects-dir', self.tmp)
        self.assertUsage(result)
        self.assertIn('is under a file', result.stderr)
        self.assertUsage(run('analyze_style'))
        self.assertUsage(run('analyze_style', '--text', 'no-register-here'))
        (self.tmp / 'a.md').write_text('Some text.', encoding='utf-8')
        self.assertUsage(run('analyze_style', '--text', f'={self.tmp}/a.md'))
        self.assertUsage(run('analyze_style', '--text', f'notes={self.tmp}/missing/*.md'))
        self.assertUsage(run('analyze_style', '--github-dir', self.tmp / 'empty'))
        (self.tmp / 'deep').mkdir()
        (self.tmp / 'deep/descriptions.json').write_text('[' * 1_000_000, encoding='utf-8')
        result = run('analyze_style', '--github-dir', self.tmp / 'deep')
        self.assertUsage(result)
        self.assertNotIn('Traceback', result.stderr)

    def test_scripts_use_only_the_standard_library(self):
        stdlib = getattr(sys, 'stdlib_module_names', None) or {
            'argparse', 'ast', 'contextlib', 'datetime', 'glob', 'io', 'json', 'math', 'os', 'pathlib', 're',
            'shutil', 'statistics', 'subprocess', 'sys', 'time', 'urllib'}
        paths = sorted(SCRIPTS.glob('*.py'))
        self.assertEqual([p.name for p in paths],
                         ['analyze_style.py', 'extract_transcripts.py', 'fetch_github.py'])
        for path in paths:
            tree = ast.parse(path.read_text(encoding='utf-8'))
            names = {alias.name for node in ast.walk(tree) if isinstance(node, ast.Import) for alias in node.names}
            names |= {node.module for node in ast.walk(tree) if isinstance(node, ast.ImportFrom) and node.module}
            self.assertTrue(names, path.name)
            for name in names:
                self.assertIn(name.split('.')[0], stdlib, f'{path.name}: {name}')
                self.assertNotIn(name.split('.')[0], {'http', 'socket', 'ssl'}, f'{path.name}: {name}')
                self.assertNotEqual(name, 'urllib.request', path.name)


if __name__ == '__main__':
    unittest.main()
