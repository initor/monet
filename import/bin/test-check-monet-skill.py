#!/usr/bin/env python3
"""Check bin/check-monet-skill.py: a fully wired skill passes, a routing-only skill passes with
--no-command, a skill wrapped by a command of another name passes under that command's name, and a
missing or drifted skill fails with one FAIL line per missing surface."""
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

REPO = Path(__file__).resolve().parent.parent
CHECK = REPO / 'bin/check-monet-skill.py'


def run(*args, repo=None):
    script = CHECK if repo is None else repo / 'bin/check-monet-skill.py'
    return subprocess.run([sys.executable, str(script), *args], capture_output=True, text=True)


class CheckMonetSkill(unittest.TestCase):
    def test_wired_command_skill_passes(self):
        result = run('paint')
        self.assertEqual(result.returncode, 0, result.stdout)
        self.assertNotIn('FAIL', result.stdout)

    def test_routing_only_skill_passes_without_command(self):
        result = run('hairline-create', '--no-command')
        self.assertEqual(result.returncode, 0, result.stdout)
        self.assertNotIn('command', result.stdout.split('ok    skill', 1)[1].split('ok    routing')[0])

    def test_routing_checks_only_the_claude_agent(self):
        result = run('roadmap')
        self.assertEqual(result.returncode, 0, result.stdout)
        self.assertIn('.claude/agents/monet.md', result.stdout)

    def test_missing_skill_fails_on_every_surface(self):
        result = run('no-such-skill')
        self.assertEqual(result.returncode, 1)
        for label in ('skill', 'command', 'routing', 'readme'):
            self.assertIn(f'FAIL  {label}', result.stdout)

    def test_bad_name_prints_usage(self):
        result = run('Not_A_Name')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('check-monet-skill.py <name>', result.stderr)

    def test_one_dropped_routing_row_fails_only_routing(self):
        # A copy of the repo's checked files where the agent loses paint's routing-table row.
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            for rel in ('bin/check-monet-skill.py', '.claude/README.md', '.claude/agents/monet.md',
                        '.claude/commands/monet/paint.md'):
                (root / rel).parent.mkdir(parents=True, exist_ok=True)
                shutil.copy2(REPO / rel, root / rel)
            shutil.copytree(REPO / '.claude/skills/paint', root / '.claude/skills/paint')
            agent = root / '.claude/agents/monet.md'
            agent.write_text(''.join(line for line in agent.read_text().splitlines(keepends=True)
                                     if not (line.startswith('|') and '`paint`' in line)))
            result = run('paint', repo=root)
            fails = [line for line in result.stdout.splitlines() if line.startswith('FAIL')]
            self.assertEqual(result.returncode, 1)
            self.assertEqual(len(fails), 1, result.stdout)
            self.assertIn('routing', fails[0])

    def test_command_with_another_name_passes_under_that_name(self):
        # A fixture repo where /monet:tool wraps the git-tool skill, as /monet:ship wraps git-ship.
        # status.md names the skill's SKILL.md outside an execution_context, so it must not match.
        files = {
            '.claude/skills/git-tool/SKILL.md': '---\nname: git-tool\ndescription: A fixture skill.\n---\n',
            '.claude/commands/monet/tool.md': ('---\nname: monet:tool\ndescription: Run git-tool.\n---\n'
                                               '<execution_context>\n@~/.claude/skills/git-tool/SKILL.md\n'
                                               '</execution_context>\n'),
            '.claude/commands/monet/status.md': ('---\nname: monet:status\n---\n'
                                                 'Lists skills/git-tool/SKILL.md among the skills.\n'),
            '.claude/agents/monet.md': ('| "use the tool" | `git-tool` | codebase | yes |\n\n'
                                        '- If I ask to use the tool, use `git-tool`.\n'),
            '.claude/README.md': ('| Command **and** routing row | `tool` | the common case |\n\n'
                                  '<!-- BEGIN GENERATED:commands -->\n| `/monet:tool` | Run git-tool |\n'
                                  '<!-- END GENERATED:commands -->\n'),
        }
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            (root / 'bin').mkdir()
            shutil.copy2(CHECK, root / 'bin/check-monet-skill.py')
            for rel, text in files.items():
                (root / rel).parent.mkdir(parents=True, exist_ok=True)
                (root / rel).write_text(text)
            result = run('git-tool', repo=root)
            self.assertEqual(result.returncode, 0, result.stdout)
            self.assertNotIn('FAIL', result.stdout)
            for detail in ('.claude/commands/monet/tool.md names monet:tool', 'has /monet:tool',
                           'lists `tool`', 'naming `git-tool`'):
                self.assertIn(detail, result.stdout)

if __name__ == '__main__':
    unittest.main()
