#!/usr/bin/env python3
"""Check paint's explainer layout: the vendored Hairline kernel, the script that fills a page's
scripts, and the page's own check, run the way the skill tells an agent to run it."""
from pathlib import Path
import hashlib
import re
import shutil
import subprocess
import tempfile
import unittest

REPO = Path(__file__).resolve().parent.parent
PAINT = REPO / '.claude/skills/paint'
HAIRLINE = REPO / '.claude/skills/hairline-create'
DOC = PAINT / 'reference/explainer.md'
FILL = PAINT / 'scripts/explainer.mjs'
HOST = PAINT / 'scripts/explainer-host.js'
EXAMPLE = PAINT / 'reference/explainer-layers.js'
CHROME = Path('/Applications/Google Chrome.app/Contents/MacOS/Google Chrome')
NODE = shutil.which('node')
BROWSER = CHROME.exists() and NODE and shutil.which('sips') and (shutil.which('timeout') or shutil.which('gtimeout'))


def block(text, heading, lang):
    return re.search(r'```' + lang + r'\n(.*?)\n```', text.split(heading, 1)[1], re.S).group(1)


def page(steps=4, extra='', title='The question a newcomer asks'):
    """A page built from the docs as an agent builds one: base CSS, a flavor, the explainer skeleton."""
    base = (PAINT / 'reference/base.md').read_text()
    flavor = (PAINT / 'reference/flavors/technical.md').read_text()
    skel = block(DOC.read_text(), '## Skeleton', 'html')
    style = re.search(r'<style>\n.*?</style>', skel, re.S).group(0)
    body = skel[len(style):].replace('<!-- the base script goes here -->',
                                     '<script>\n' + block(base, '## Base JS', 'javascript') + '\n</script>')
    one = re.search(r'( *)<li>.*?</li>\n *<!-- .*? -->\n', body, re.S)
    items = ''.join(f'{one.group(1)}<li><span class="n">{i:02d}</span><b>Step {i}</b><span class="d">Detail {i}.</span></li>\n'
                    for i in range(1, steps + 1))
    body = body.replace(one.group(0), items).replace('style="--n: 4"', f'style="--n: {steps}"')
    body = body.replace('Two to four short sentences that state the whole idea.', 'One idea in short words. It fits in two sentences.')
    body = body.replace('The question a newcomer asks', title)
    body = body.replace('<footer>', extra + '<footer>')
    return ('<!DOCTYPE html>\n<html lang="en" data-theme="auto" data-anim="off">\n<head>\n<meta charset="UTF-8">\n'
            '<meta name="viewport" content="width=device-width, initial-scale=1.0">\n<title>Test</title>\n<style>\n'
            + block(base, '## Base CSS', 'css') + '\n' + block(flavor, '## Tokens', 'css') + '\n</style>\n'
            + style + '\n</head>\n<body>\n<button id="theme-toggle" aria-label="Toggle color theme"></button>\n'
            + body + '\n</body>\n</html>\n')


def fill(page_path, figure=EXAMPLE, script=FILL):
    return subprocess.run([NODE, str(script), str(page_path), str(figure)], capture_output=True, text=True)


class VendoredHairlineTests(unittest.TestCase):
    def test_kernel_matches_the_hash_in_its_first_line(self):
        k = (HAIRLINE / 'kernel.js').read_text().replace('\r\n', '\n').rstrip()
        cut = k.index('\n')
        want = re.search(r'sha256:([0-9a-f]{64})', k[:cut]).group(1)
        self.assertEqual(hashlib.sha256((k[cut + 1:] + '\n').encode()).hexdigest(), want)

    def test_the_copy_carries_its_license_and_its_pin(self):
        self.assertIn('MIT License', (HAIRLINE / 'LICENSE').read_text())
        self.assertIn('Lucas Marques', (HAIRLINE / 'LICENSE').read_text())
        self.assertRegex((HAIRLINE / 'UPSTREAM.md').read_text(), r'at commit\s+[0-9a-f]{40}')


@unittest.skipUnless(NODE, 'node is required')
class FillTests(unittest.TestCase):
    def test_scripts_parse(self):
        self.assertEqual(subprocess.run([NODE, '--check', str(HOST)]).returncode, 0)
        run = subprocess.run([NODE, '--check', '--input-type=module'], input=EXAMPLE.read_text(), text=True)
        self.assertEqual(run.returncode, 0)

    def test_fill_puts_each_script_in_its_slot_and_runs_again_unchanged(self):
        with tempfile.TemporaryDirectory() as tmp:
            p = Path(tmp) / 'x.html'
            p.write_text(page())
            self.assertEqual(fill(p).returncode, 0)
            once = p.read_text()
            self.assertEqual(once.count('var HL = '), 1)
            self.assertEqual(once.count('window.hairline = (figure)'), 1)
            self.assertEqual(once.count('name: "layers"'), 1)
            self.assertEqual(fill(p).returncode, 0)
            self.assertEqual(p.read_text(), once)

    def test_fill_refuses_a_page_without_its_slots(self):
        with tempfile.TemporaryDirectory() as tmp:
            p = Path(tmp) / 'x.html'
            p.write_text(page().replace('<script type="module" id="hl-figure"></script>', ''))
            run = fill(p)
            self.assertNotEqual(run.returncode, 0)
            self.assertIn('exactly one <script type="module" id="hl-figure">', run.stderr)

    def test_fill_refuses_an_edited_kernel(self):
        k = (HAIRLINE / 'kernel.js').read_text()
        cut = k.index('\n')
        for name, edited in [('body', k.replace('var HL = ', 'var HL =  ', 1)),
                             ('hash line', k[:cut] + ' window.injected = true;' + k[cut:])]:
            with self.subTest(edit=name), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                (root / 'paint/scripts').mkdir(parents=True)
                (root / 'hairline-create').mkdir()
                for f in (FILL, HOST):
                    shutil.copy(f, root / 'paint/scripts' / f.name)
                (root / 'hairline-create/kernel.js').write_text(edited)
                p = root / 'x.html'
                p.write_text(page())
                run = fill(p, script=root / 'paint/scripts/explainer.mjs')
                self.assertNotEqual(run.returncode, 0)
                self.assertIn('differs from its hash', run.stderr)

    def test_fill_names_its_usage_when_the_arguments_are_swapped(self):
        with tempfile.TemporaryDirectory() as tmp:
            p = Path(tmp) / 'x.html'
            p.write_text(page())
            run = subprocess.run([NODE, str(FILL), str(EXAMPLE), str(p)], capture_output=True, text=True)
            self.assertEqual(run.returncode, 2)
            self.assertIn('usage: node explainer.mjs <page.html> <figure.js>', run.stderr)


@unittest.skipUnless(BROWSER, 'needs macOS, Chrome, node, sips and timeout')
class ExportTests(unittest.TestCase):
    """Runs the check-and-export block of explainer.md as written, with a shorter timeout."""

    def export(self, html, step=''):
        with tempfile.TemporaryDirectory() as tmp:
            p = Path(tmp) / 'x.html'
            p.write_text(html)
            self.assertEqual(fill(p).returncode, 0)
            script = block(DOC.read_text(), '## Check and export', 'bash')
            head = 'page="<absolute path of the page, without .html>"\nstep=""'
            self.assertTrue(script.startswith(head))
            script = script.replace(head, f'page="{p.with_suffix("")}"\nstep="{step}"', 1)
            script = script.replace(' -k 2 20 ', ' -k 2 12 ')
            out = subprocess.run(['/bin/bash', '-c', script], capture_output=True, text=True, timeout=90).stdout
            png = p.with_suffix('').with_name('x' + (f'-step{step}' if step else '') + '.png')
            size = subprocess.run(['sips', '-g', 'pixelWidth', '-g', 'pixelHeight', str(png)],
                                  capture_output=True, text=True).stdout if png.exists() else ''
            return out, size

    def test_the_example_passes_both_checks_and_writes_a_cropped_png(self):
        out, size = self.export(page(), step='2')
        self.assertIn('data-explainer="ok"', out)
        self.assertIn('phone: data-phone="ok"', out)
        self.assertIn('pixelWidth: 2400', size)
        height = int(re.search(r'pixelHeight: (\d+)', size).group(1))
        self.assertLess(height, 4400, 'the PNG keeps the empty window below the page')

    def test_the_checks_name_each_broken_rule(self):
        html = page(steps=3, extra='<div style="width: 600px">wide</div>\n  ',
                    title='A title that runs on past the seven word limit')
        out, _ = self.export(html, step='9')
        for problem in ['the title has 10 words, 7 at most', 'the figure declares 4 parts for 3 steps',
                        '?step=9 names no step: use 1 to 3']:
            self.assertIn(problem, out)
        self.assertNotIn('the page is wider than the window', out.split('phone:')[0])
        self.assertIn('the page is wider than the window"', out.split('phone:')[1])


if __name__ == '__main__':
    unittest.main()
