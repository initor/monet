#!/usr/bin/env python3
"""Check the STE-lite counter that the voice skill runs under --ste."""
from pathlib import Path
import contextlib
import importlib.util
import io
import os
import re
import subprocess
import sys
import tempfile
import unittest

REPO = Path(__file__).resolve().parent.parent
SCRIPT = REPO / '.claude/skills/voice/scripts/ste_check.py'
CARD = REPO / '.claude/skills/voice/reference/ste-lite.md'
STYLE = REPO / '.claude/output-styles/ste-lite.md'
EM_DASH, RSQUO, LDQUO, RDQUO = chr(0x2014), chr(0x2019), chr(0x201C), chr(0x201D)
SPEC = importlib.util.spec_from_file_location('ste_check', SCRIPT)
STE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(STE)


def words(n, end='.'):
    return 'Word ' + ' '.join(['word'] * (n - 1)) + end


def run(text):
    """Check text in process and return (exit status, report)."""
    out = io.StringIO()
    with contextlib.redirect_stdout(out):
        found = STE.check('draft.md', text)
    return (1 if found else 0), out.getvalue()


def stats(text):
    """Return the report's numbers: sentences, median, max, and each count."""
    head, counts = run(text)[1].splitlines()[:2]
    m = re.search(r': (\d+) sentences, median ([\d.]+) words, max (\d+)$', head)
    nums = {'sentences': int(m[1]), 'median': float(m[2]), 'max': int(m[3])}
    for part in counts.split(','):
        key, value = part.strip().rsplit(' ', 1)
        nums[key] = int(value)
    return nums


def issues(text):
    return [line.strip() for line in run(text)[1].splitlines()[2:]]


def cli(*args, stdin=None):
    return subprocess.run([sys.executable, str(SCRIPT), *args], input=stdin,
                          capture_output=True, text=True, timeout=30)


class Case(unittest.TestCase):
    def assertClean(self, *texts):
        for text in texts:
            code, out = run(text)
            self.assertEqual(code, 0, f'{text!r}\n{out}')

    def assertReports(self, text, *needles):
        code, out = run(text)
        self.assertEqual(code, 1, f'{text!r}\n{out}')
        for needle in needles:
            self.assertIn(needle, out, f'{text!r}\n{out}')

    def assertCounts(self, text, **expected):
        got = stats(text)
        self.assertEqual({k: got[k] for k in expected}, expected, repr(text))


class LimitTests(Case):
    def test_sentence_caps(self):
        self.assertClean(words(25) + '\n', '1. ' + words(20) + '\n', '- ' + words(25) + '\n')
        self.assertEqual(issues(words(26) + '\n'), ['L1: 26 words (max 25): ' + words(26)[:70].strip()])
        self.assertReports('1. ' + words(21) + '\n', 'L1: 21 words (max 20)')
        self.assertReports('1) ' + words(21) + '\n', 'L1: 21 words (max 20)')
        self.assertReports('- ' + words(26) + '\n', 'L1: 26 words (max 25)')

    def test_marker_edges(self):
        self.assertReports('123456789. ' + words(21) + '\n', 'L1: 21 words (max 20)')
        self.assertClean('- [X] ' + words(25) + '\n', '2.5 ' + words(22) + '\n')
        text = 'Run ' + words(26)
        self.assertEqual(issues(text + '\n'), [('L1: 27 words (max 25): ' + text[:70]).strip()])

    def test_paragraph_caps(self):
        self.assertClean('One. Two. Three. Four. Five. Six.\n',
                         'One. Two. Three. Four.\n\nFive. Six. Seven. Eight.\n',
                         ''.join(f'- Item {i} is short.\n' for i in range(9)),
                         ''.join(f'{i}. Run step {i}.\n' for i in range(1, 10)),
                         '| A | B |\n|---|---|\n| A. B. C. D. E. F. G. | ok |\n')
        for lead in ('', '- ', '1. ', '[^1]: '):
            self.assertReports(lead + 'A. B. C. D. E. F. G.\n', 'L1: paragraph has 7 sentences (max 6)')
        self.assertReports('Intro.\n\nA. B. C.\nD. E. F. G.\n', 'L3: paragraph has 7')

    def test_wrapped_lines_join_and_report_the_first_line(self):
        self.assertReports(words(13, end='') + '\n' + words(13) + '\n', 'L1: 26 words (max 25)')
        self.assertReports('Short one.\n' + words(26) + '\n', 'L2: 26 words')

    def test_tokens_count_as_one_word_or_none(self):
        self.assertClean('Run `' + ' '.join(['arg'] * 30) + '` now.\n',
                         'Run ``a ` ' + ' '.join(['arg'] * 30) + '`` now.\n',
                         'Run <code>' + ' '.join(['arg'] * 30) + '</code> now.\n',
                         '1. Expect "' + ' '.join(['error'] * 30) + '" in the log.\n',
                         f'1. Expect {LDQUO}' + ' '.join(['error'] * 30) + f'{RDQUO} in the log.\n',
                         'Read [guide](https://x.example "' + ' '.join(['t'] * 30) + '") now.\n',
                         words(24, end='') + ' [^1] end.\n')
        for marker in ('-', '*', '+', '- [ ]', '- [x]', '>', '> >', '> -'):
            self.assertClean(marker + ' ' + words(25) + '\n')
        self.assertClean('1. [x] ' + words(20) + '\n')
        self.assertCounts('Read [the long guide](https://x.example/a/b) now.\n', max=5)
        self.assertCounts('Read [guide](https://x.example "Some title here") now.\n', max=3)
        self.assertCounts('Run the<!-- c -->job now.\n', max=4)

    def test_tokens_without_words(self):
        self.assertCounts('Run the job now. [](#top)\n', sentences=1)
        self.assertReports('[^1] ' + words(26) + '\n', '26 words')
        self.assertCounts('[^1] One. Two three.\n', sentences=2, median=1.5)
        for br in ('<br>', '<br/>', '<br />', '<BR>'):
            self.assertCounts(f'Stop the node.{br}Then drain it.\n', sentences=2)
        self.assertCounts('Run <!--> it now.\n', max=3)
        self.assertCounts('Run <!-- a --> it; then <!-- b --> stop.\n', semicolons=1)
        self.assertCounts('Run `a`; then run `b` now.\n', semicolons=1, max=6)
        self.assertCounts('Use <code>a</code>; then <code>b</code> here.\n', semicolons=1, max=5)
        self.assertClean('Run <code class="language-sh">a; b</code> now.\n', 'Run <code >a; b</code > now.\n',
                         'Press <kbd title="semi">;</kbd> now.\n')

    def test_tokens_that_wrap_still_count_once(self):
        self.assertClean('1. If the log shows "Back-off restarting failed container app in pod app\n'
                         '   because the readiness probe timed out", restart the node agent.\n',
                         'Run `a;\nb` now.\n')
        mispair = words(12, end='') + ' "a\nb" ' + words(12, end='') + ' "c d" end.\n'
        self.assertReports(mispair, '27 words (max 25)')


class MarkTests(Case):
    def test_marks_count_with_line_numbers(self):
        self.assertEqual(issues(f'Short line.\nOne; two {EM_DASH} three.\n'),
                         ['L2: em-dash', 'L2: semicolon'])
        self.assertCounts("## Don't stop " + EM_DASH + ' ever; now\n',
                          **{'em-dashes': 1, 'semicolons': 1, 'contractions': 1})

    def test_entities_render_as_their_marks(self):
        for text in ('One &mdash; two.\n', 'One &#8212; two.\n', 'One &#x2014; two.\n'):
            self.assertCounts(text, **{'em-dashes': 1})
        for text in ('We don&rsquo;t stop.\n', 'We don&#39;t stop.\n', 'We don&#x27;t stop.\n'):
            self.assertCounts(text, contractions=1)
        self.assertCounts('Run it&#59; then stop.\n', semicolons=1)
        self.assertCounts('We fund R&D; the team ships.\n', semicolons=1)
        self.assertClean('Replace &lt;node&gt; with the host name.\n',
                         'Wait&nbsp;5&nbsp;s &rarr; go&hellip;\n',
                         'Fish &amp; chips. Say &quot;hi&quot; now.\n')

    def test_only_commonmark_references_decode(self):
        self.assertClean('Use &#8212 here.\n', 'Use &#59 here.\n', "We don&#39t stop.\n",
                         'Write &quot;use&quot;, not &quot;utilize&quot;.\n',
                         'Say &quot;wait; then go&quot; twice.\n',
                         'Say &ldquo;wait; then go&rdquo; twice.\n', 'Run &#96;a&#96; now.\n')
        self.assertCounts('Run &#96;a; b&#96; now.\n', semicolons=1)
        self.assertCounts('Use &#' + '9' * 5000 + '; here.\n', semicolons=1)
        for text in ('One &#0008212; two.\n', 'One &#x002014; two.\n', 'One &#X2014; two.\n'):
            self.assertCounts(text, **{'em-dashes': 1, 'semicolons': 0})

    def test_urls_and_link_targets_are_not_prose(self):
        self.assertClean('See [docs](https://x.example/a;b) and https://y.example/c;d now.\n',
                         'See [docs](docs/a;b.md) now.\n', 'See <https://x.example/a;b> now.\n',
                         'See https://x.example/ensure now.\n', 'Read [it](utilize.md) now.\n')
        self.assertCounts('See https://example.com/docs; then run it.\n', semicolons=1)
        self.assertCounts(f'See https://example.com/docs{EM_DASH}it is short.\n', **{'em-dashes': 1})
        self.assertReports('Read [utilize](x.md) now.\n', "'utilize'")
        self.assertClean('See &lt;https://x.example/a&gt; now.\n', 'See https://x.example/a&nbsp;. Then go.\n',
                         'See http://x.example/a;b now.\n', 'See www.example.com/a;b now.\n',
                         'See [the docs](https://x.example/app_(v2);jsessionid=a1) now.\n',
                         "Read [it](x.md 'a; b') now.\n", '## Fish &amp; chips\n',
                         '## See https://x.example/a;b\n')
        self.assertCounts('See https://example.com/docs&mdash;it is short.\n', **{'em-dashes': 0})
        self.assertCounts('See https://x.example/a;. Then go.\n', semicolons=1)
        self.assertClean('Do it prior\n<https://x.example/a> to the deploy.\n',
                         'Do it prior\nhttps://x.example/a to the deploy.\n',
                         'Get it from ftp://x.example/pub;type=a now.\n',
                         'See [the page](https://x.example/a_(b_(c));d) now.\n')
        self.assertCounts(f'Read [it](x.md "Roll back {EM_DASH} v2") now.\n', **{'em-dashes': 1})
        self.assertReports('Run it [prior](x.md) to the deploy.\n', "'prior to'")
        self.assertReports('Do it [in order](x.md) to pass.\n', "'in order to'")

    def test_code_and_double_quotes_hide_marks(self):
        self.assertClean('Run `a; b` now.\n', 'Run ``a`b; c`` now.\n', 'Run <code>a; b</code> now.\n',
                         'Press <kbd>Ctrl;</kbd> now.\n', 'The log says "a; b; c" here.\n',
                         f'The log says {LDQUO}a; b {EM_DASH} c{RDQUO} here.\n',
                         f'```sh\ngrep -o "{EM_DASH}" f; wc -l\n```\n')
        self.assertCounts("The log says 'can't connect' here.\n", contractions=1)
        self.assertClean('Run <CODE>a; b</CODE> now.\n', 'Run `a``b; c` now.\n')
        for text in ('Run ``a; b` now.\n', 'Run ``a; `b` now.\n', 'Run `a; b`` now.\n'):
            self.assertCounts(text, semicolons=1)

    def test_a_quote_pairs_only_at_word_edges(self):
        self.assertCounts('Use the 5" screen; see "the guide" for more.\n', semicolons=1)
        self.assertCounts('The 5" screen; or the 6" one.\n', semicolons=1)
        self.assertCounts('He said " then left; see "the guide" here.\n', semicolons=1)
        self.assertCounts(f'A {LDQUO}stray one; then {LDQUO}a real one{RDQUO} here.\n', semicolons=1)
        self.assertClean('Wrap the error: _"a; b"_ is what the log says.\n', 'Run "a;" now.\n',
                         f'Say {LDQUO}wait{EM_DASH}{RDQUO} twice.\n', 'Type \\`a b\\` now.\n')
        self.assertCounts('Type \\`a; b\\` to see the raw text, then stop.\n', semicolons=1)
        self.assertCounts('Type "" a; b " here.\n', semicolons=1)
        self.assertClean('Say &quot;Stop now.&quot; ' + words(24) + '\n',
                         'Say &ldquo;Stop now.&rdquo; ' + words(24) + '\n')
        self.assertCounts('Type "!" Then stop.\n', sentences=2)
        self.assertCounts('Type &quot;!&quot; Then stop.\n', sentences=2)
        self.assertCounts('Say &quot;a&quot; then; say &quot;b&quot; now.\n', semicolons=1)
        self.assertClean('Say &#34;a; b&#34; now.\n', 'Say &#x22;a; b&#x22; now.\n')

    def test_contractions_but_not_possessives(self):
        self.assertClean("Karpathy's post is short.\n", f'Karpathy{RSQUO}s post is short.\n')
        for text in ("We don't stop.\n", f'It{RSQUO}s late.\n', "IT'S LATE.\n"):
            self.assertCounts(text, contractions=1)
        self.assertCounts("We're in. I'll go. You've won. I'm here. I'd stay.\n", contractions=5)
        self.assertClean("Check the commit's message.\n", "Clear the cache's old files.\n",
                         "Read the O'Reilly book.\n", "Read the dos and don'ts.\n")
        self.assertEqual(issues("We stop here and\ndon't go on.\n"), ["L2: contraction 'don't'"])
        self.assertEqual(issues('One; two; three.\n'), ['L1: semicolon'] * 2)
        self.assertCounts('The password shows as **** in the log.\n', **{'bold spans': 0})
        self.assertCounts("That's it. There's one. Let's go. Here's one. What's it. Who's in. "
                          "He's in. She's in. Where's it.\n", contractions=9)

    def test_swap_words(self):
        self.assertEqual(issues('We utilize it.\n'), ["L1: 'utilize', write 'use'"])
        self.assertClean('Write "use", not "utilize".\n',
                         f'Write {LDQUO}use{RDQUO}, not {LDQUO}utilize{RDQUO}.\n',
                         'Run `make ensure-deps` first.\n', 'The performance is obtainable.\n')
        self.assertEqual(issues('Do this prior\nto that.\n'), ["L1: 'prior to', write 'before'"])
        self.assertEqual(issues('Do this prior to\nthat.\n'), ["L1: 'prior to', write 'before'"])
        for text in ('Ensure it runs.', 'Run it prior to the deploy.', 'Do it in order to pass.',
                     'Commence the run.', 'Obtain a token.', 'Perform the step.', 'Leverage the cache.',
                     'Facilitate the review.', 'Subsequently, stop.', 'In the event that it fails, stop.',
                     'UTILISED it.', 'We utilizing it.', 'It utilizes the cache.',
                     'Run it, ensuring the cache is warm.', 'We performed the step.', 'It obtains a token.',
                     'The run commenced at noon.', 'We are leveraging the cache.',
                     'It facilitates the review.'):
            self.assertReports(text + '\n', "write '")
        for stem, forms in (('utili', ('ze', 'zes', 'zed', 'zing', 'se', 'ses', 'sed', 'sing')),
                            ('commenc', ('e', 'es', 'ed', 'ing')), ('obtain', ('', 's', 'ed', 'ing')),
                            ('ensur', ('e', 'es', 'ed', 'ing')), ('perform', ('', 's', 'ed', 'ing')),
                            ('leverag', ('e', 'es', 'ed', 'ing')), ('facilitat', ('e', 'es', 'ed', 'ing'))):
            for form in forms:
                self.assertReports(f'We {stem}{form} it.\n', "write '")
        self.assertEqual(issues('Stop in\nthe event that it fails.\n'), ["L1: 'in the event that', write 'if'"])
        self.assertEqual(issues('Do it in\norder to pass.\n'), ["L1: 'in order to', write 'to'"])
        self.assertEqual(issues('Stop the job in the\nevent that it fails.\n'),
                         ["L1: 'in the event that', write 'if'"])


class ExtraTests(Case):
    def test_frontmatter_counts_only_em_dashes(self):
        self.assertClean('---\nname: x; y\ndescription: ' + words(40) + '\n---\n\nShort text here.\n')
        self.assertEqual(issues(f'---\ndescription: a {EM_DASH} b\n---\nText.\n'), ['L2: em-dash'])
        self.assertEqual(issues(f'---\ndescription: "a {EM_DASH} b"\n---\nText.\n'), ['L2: em-dash'])
        self.assertCounts('---\nname: x\n...\nText.\n', sentences=1)
        self.assertCounts('---\r\nname: x; y\r\n---\r\nText.\r\n', sentences=1, semicolons=0)
        self.assertClean('---\nname: x\ndescription: >\n  Long text; here\n---\nText.\n',
                         '---\n# a comment\nname: x; y\n---\nText.\n')
        self.assertCounts('---\n...\nText.\n', sentences=2)
        self.assertCounts('---\nallowed-tools: x; y\n---\nText.\n', semicolons=0)
        self.assertCounts('Intro text\nname: x; y\n---\nMore text.\n', semicolons=1)
        self.assertCounts(f'---\ndescription: a {EM_DASH} b\n---\nText.\n', **{'em-dashes': 1})

    def test_a_leading_rule_is_not_frontmatter(self):
        self.assertReports('---\n' + words(26) + '\n', '26 words')
        self.assertCounts("---\n\nWe don't stop; ever.\n\n---\n\nShort text here.\n",
                          sentences=2, semicolons=1, contractions=1)

    def test_comments_count_only_em_dashes(self):
        self.assertClean('<!-- a; b -->\n', '<!--\na; b\n-->\n',
                         'Run it <!-- start\nmiddle; part\nend --> now.\n')
        self.assertEqual(issues(f'<!-- a {EM_DASH} b -->\nText.\n'), ['L1: em-dash'])
        self.assertEqual(issues(f'<!--\na; b {EM_DASH} c\n-->\n'), ['L2: em-dash'])
        self.assertEqual(issues(f'Run the job <!-- a; b {EM_DASH} c --> now.\n'), ['L1: em-dash'])
        self.assertCounts('Run the job <!-- one two three four --> now.\n', max=4)
        self.assertEqual(issues(f'<!-- a {EM_DASH} b {EM_DASH} c -->\nText.\n'), ['L1: em-dash'] * 2)
        self.assertEqual(issues(f'<!--\n    a {EM_DASH} b\n-->\nText.\n'), ['L2: em-dash'])
        self.assertClean('<!-- TODO: a -> b; fix later -->\nText.\n', 'Run <!-- a; b --> it <!-- c --> now.\n')

    def test_empty_and_unclosed_comments(self):
        for opener in ('<!--->', '<!-->'):
            self.assertCounts(f"Run {opener} it; now.\n\nWe don't stop; ever.\n", semicolons=2, contractions=1)
            self.assertCounts(f"{opener}\nWe don't stop; ever.\n", semicolons=1, contractions=1)
        self.assertCounts("Use <!-- to start one; then close it.\n\nWe don't stop.\n",
                          semicolons=1, contractions=1)
        self.assertEqual(issues("Text.\n\n<!-- open\nWe don't stop.\n"),
                         ['L3: comment never closes, so the lines after it are not checked'])

    def test_text_around_a_comment_is_prose(self):
        self.assertCounts("<!-- note --> We don't stop.\n", contractions=1)
        self.assertCounts("<!--\nnote\n--> We don't stop.\n", contractions=1)
        self.assertCounts("Use `<!--` to open one.\n\nWe don't stop.\n", contractions=1)
        self.assertClean(words(13, end='') + '\n<!-- c -->\n' + words(13) + '\n')
        self.assertCounts("<!--\n```\n-->\nWe don't stop.\n", contractions=1)
        self.assertReports(words(13, end='') + '\nword <!-- c --> ' + words(13) + '\n', 'L1: 27 words')
        self.assertClean('<!-- c --> 1. ' + words(21) + '\n', '<!--\nc\n--> 1. ' + words(21) + '\n')
        self.assertReports('<!-- c --> # ' + words(30) + '\n', '31 words (max 25)')
        self.assertReports('<!--\nc\n-->\nA. B. C. D. E. F. G.\n', 'L4: paragraph has 7')
        self.assertReports('<!-- c -->\nA. B. C. D. E. F. G.\n', 'L2: paragraph has 7')
        self.assertReports('<!-- c -->Utilize it.\n', "'Utilize'")
        self.assertCounts('<!-- c -->A\n', sentences=1)
        self.assertClean('<!--\na -> b; c\n-->\nText.\n')
        self.assertReports('<!--\nc\n-->\n2. ' + words(21) + '\n', 'L4: 21 words (max 20)')
        self.assertReports('<!-- c -->\n2. ' + words(21) + '\n', 'L2: 21 words (max 20)')
        self.assertEqual(issues(f'<!-- c -->{EM_DASH} tail\n'), ['L1: em-dash'])
        self.assertEqual(issues(f'<!-- open {EM_DASH} x\nmore\n-->\nText.\n'), ['L1: em-dash'])
        self.assertCounts('<!-- open\nmore\n-->\nText one.\n', sentences=1)
        self.assertEqual(issues(f'Line one\nRun it <!-- a {EM_DASH} b --> now.\n'), ['L2: em-dash'])
        self.assertEqual(issues(f'Run it <!-- a\n{EM_DASH} b --> now.\n'), ['L2: em-dash'])
        self.assertCounts('Run it <!-- start\nmiddle\nend --> now and go.\n', sentences=1)


class BlockTests(Case):
    def test_fenced_code_is_skipped(self):
        self.assertClean('~~~\na; b\n~~~\n', '````\n```\na; b\n```\n````\n',
                         '```\na\n       ```\nb; c\n```\n',
                         '```\na\n```sh\nb; c\n```\n', '~~~\n```\na; b\n~~~\n', '````\na\n```\nb; c\n````\n',
                         "10. Run this:\n\n    ```sh\n    a; b\n    # don't\n    ```\n",
                         '- Run:\n\n      ~~~\n      a; b\n      ~~~\n', '> ```\n> a; b\n> ```\n')
        self.assertCounts('Use ```x``` here; see.\n', semicolons=1)
        self.assertCounts('```x``` runs here; see.\n', semicolons=1)
        self.assertCounts('```\na\n```\n\nb; c\n', semicolons=1)
        self.assertCounts('    ```\n    a\n    ```\nb; c\n', semicolons=1)
        self.assertCounts('```\na; b\n   ```\nc; d\n', semicolons=1)
        self.assertClean('```\na\n    ```\nb; c\n```\n', '- ```sh\n  a; b\n  ```\n',
                         '2. ```sh\n   a; b\n   ```\n')
        self.assertCounts('- Run:\n\n      ~~~\n      a\n      ~~~\n\nb; c\n', semicolons=1)
        self.assertCounts('10. Run this:\n\n    ```sh\n    a\n    ```\n\nWe stop; now.\n', semicolons=1)
        self.assertCounts("We don't stop.    \n", contractions=1)
        self.assertClean(words(13, end='') + '\n```\nx\n```\n' + words(13) + '\n')
        self.assertClean('Run this:\n\n<pre>\nkubectl get pods; kubectl get nodes\n</pre>\n\nThen wait.\n')
        self.assertClean('A. B. C. D.\n<pre>\nx\n</pre>\nE. F. G.\n', '<pre>a; b\nc\n</pre>\n')
        for tag in ('script', 'style', 'textarea', 'PRE'):
            self.assertClean(f'<{tag}>\na; b\n</{tag}>\n')
        self.assertClean('<pre class="x">\na; b\n</pre>\n', '<pre\na; b\n</pre>\n')
        self.assertCounts("<pre>x; y</pre>\nWe don't stop.\n", contractions=1, semicolons=0)
        self.assertEqual(issues('<pre>\nx; y\n'),
                         ['L1: HTML block never closes, so the lines after it are not checked'])
        self.assertCounts('~~Old~~ text; new text.\n', semicolons=1)

    def test_a_fence_ends_with_its_container(self):
        step = ("1. Back up the cache:\n\n   ```sh\n   tar czf cache.tgz /var/cache/app\n\n"
                "2. Delete the cache; it can't be restored.\n3. Utilize the new cache from now on.\n")
        self.assertReports(step, 'L6: semicolon', "L6: contraction 'can't'", "L7: 'Utilize'")
        self.assertCounts('> [!NOTE]\n> ~~~\n> code; here\n\nAfter; text.\n', semicolons=1)
        self.assertCounts('> ```sh\n> make build\nAfter the quote; this is prose.\n\nMore prose; here.\n',
                          semicolons=2)
        self.assertEqual(issues('Text.\n\n```\nx; y\n'),
                         ['L3: fence never closes, so the lines after it are not checked'])

    def test_a_block_start_is_never_a_lazy_line(self):
        self.assertClean('> Quote text.\n~~~\na; b\n~~~\n', '> Quote text.\n# ' + words(30) + '\n',
                         '> A. B. C. D.\n***\nE. F. G.\n', '- A. B. C. D.\n> E. F. G. H.\n',
                         '- Item text.\n<pre>\nx; y\n</pre>\n')

    def test_a_fence_cannot_interrupt_text_it_would_continue(self):
        self.assertEqual(issues('Run this text\n    ~~~\n    a; b\n    ~~~\nnow.\n'), ['L3: semicolon'])
        self.assertCounts('> quote\n    ~~~\n    a; b\n    ~~~\n', semicolons=1)

    def test_indented_code_is_skipped_only_where_it_cannot_continue_text(self):
        self.assertClean('Run this:\n\n    a; b; c\n', 'Run:\n\n\ta; b\n', '    a; b\n', '## Run\n    a; b\n',
                         'Run:\n\n    > a; b\n', '>     a; b\n')
        self.assertCounts('Text here\n    a; b\n', semicolons=1)
        self.assertCounts("Intro.\n\n  We don't stop.\n", contractions=1)
        self.assertClean('- item\n<!-- end -->\n\n    a; b\n', '1. Run:\n\n       a; b\n')
        self.assertCounts('- Item.\n\n    a; b\n', semicolons=1)

    def test_blockquotes_are_prose(self):
        self.assertCounts("> We don't stop; ever.\n", semicolons=1, contractions=1)
        self.assertCounts('> quote\ncontinued; two\n', semicolons=1)
        self.assertClean('> A. B. C. D.\n>\n> E. F. G. H.\n')
        self.assertReports('> A. B. C. D.\n> E. F. G. H.\n', 'paragraph has 8')
        self.assertReports('> - ' + words(26) + '\n', '26 words (max 25)')
        self.assertReports('> 1. ' + words(21) + '\n', '21 words (max 20)')
        self.assertEqual(issues('> [!WARNING]\n> Do not stop; it breaks.\n'), ['L2: semicolon'])
        for tag in ('NOTE', 'TIP', 'IMPORTANT', 'WARNING', 'CAUTION', 'warning'):
            self.assertClean(f'> [!{tag}]\n> ' + words(25) + '\n')
        self.assertCounts(f"> **WARNING:** It deletes the cache {EM_DASH} we can't undo it; back it up.\n",
                          **{'em-dashes': 1, 'semicolons': 1, 'contractions': 1})
        self.assertCounts("> quote\n<!-- c -->\nWe don't stop; ever.\n", semicolons=1, contractions=1)
        self.assertCounts('- Offer this:\n  > Then we go.\n', sentences=2)
        self.assertCounts('> Outer text\n> > Inner text.\n> back out.\n', sentences=2)
        self.assertCounts("> [!WARNING] Do not stop the agent; it can't restart.\n",
                          semicolons=1, contractions=1)
        self.assertCounts('>    Run it; then stop.\n', semicolons=1)
        self.assertCounts('> Intro.\n>\n>    Run it; then stop.\n', semicolons=1)
        self.assertCounts(">Don't stop.\n", contractions=1)
        self.assertCounts('> Quote one.\n   - Item two.\n', sentences=2)
        self.assertClean('   > ' + words(25) + '\n', '   - ' + words(25) + '\n')
        self.assertReports('1. Step.\n   - Sub one.\n     > A. B. C. D. E. F.\n', 'L2: paragraph has 7')

    def test_quotes_and_tables_stay_inside_their_step(self):
        self.assertReports('1. Drain the node.\n   > [!WARNING]\n' + words(21) + '\n2. Cordon the node.\n',
                           'L3: 21 words (max 20)')
        self.assertReports('1. Stop the agent on the host.\n   > WARNING: The next step deletes the cache.\n2. '
                           + words(21) + '\n', 'L3: 21 words (max 20)')
        self.assertClean('1. Stop it.\n   > WARNING: Data goes away.\n2. Delete it.\n3. Wait.\n4. Start it.\n')
        self.assertReports('> quote\n2. ' + words(21) + '\n', 'L2: 21 words (max 20)')
        reboot = ("10. Reboot the node.\n\n"
                  "    > **WARNING:** The reboot kills every pod that the drain did not evict.\n\n"
                  f"    Wait until the node reports Ready {EM_DASH} it can't take more than two minutes.\n\n"
                  "11. Verify it.\n")
        self.assertReports(reboot, 'L5: em-dash', "L5: contraction 'can't'")
        self.assertReports('9. A. B. C.\n   > D.\n\n   E. F. G.\n', 'L1: paragraph has 7')
        checks = ('11. Run the checks:\n\n    | Check | Result |\n    |---|---|\n    | disk | ok |\n\n'
                  '    If a check fails; do not uncordon the node.\n')
        self.assertReports(checks, 'L7: semicolon')
        self.assertReports('9. Check:\n\n   | A | B |\n   |---|---|\n   | x | y |\n\n   ' + words(21) + '\n',
                           'L7: 21 words (max 20)')

    def test_a_list_starts_inside_a_paragraph_only_at_one(self):
        wrapped = words(19, end='') + '\n3. ' + ' '.join(['day'] * 12) + '.\n'
        self.assertReports(wrapped, 'L1: 32 words (max 25)')
        self.assertReports('Intro text.\n1. ' + words(21) + '\n', 'L2: 21 words (max 20)')
        self.assertReports('1. First.\n3. ' + words(21) + '\n', 'L2: 21 words (max 20)')
        self.assertClean('Intro text\n0. ' + words(21) + '\n', '1234567890. ' + words(21) + '\n')
        self.assertReports('1. ' + words(19, end='') + '\n   3. ' + ' '.join(['day'] * 12) + '.\n',
                           'L1: 32 words')
        self.assertReports('1. Stop the agent.\n\n2. ' + words(21) + '\n', 'L3: 21 words (max 20)')

    def test_a_paragraph_joins_the_item_whose_column_it_reaches(self):
        self.assertReports('1. Back up the database.\n   - Use the backup tool.\n'
                           '   - Copy the file to the vault.\n\n   '
                           + words(23) + '\n', 'L5: 23 words (max 20)')
        self.assertClean('- Prepare the host:\n  1. Stop the agent.\n  2. Clear the cache.\n\n  '
                         + words(22) + '\n')
        self.assertReports('1. Drain the node. Wait for the pods.\n   - Check the pods.\n'
                           '   - Check the events.\n\n'
                           '   Cordon it. Patch it. Restart it. Watch it. Check it.\n', 'L1: paragraph has 7')
        self.assertReports('10. Rotate the keys:\n    - Create the key.\n    - Revoke the old key.\n\n    '
                           + words(21) + '\n', 'L5: 21 words (max 20)')
        self.assertClean('1. Restart the agent.\n\n  ' + words(21) + '\n',
                         '10. Restart the agent.\n\n   ' + words(21) + '\n', '1. A. B. C. D.\n\n  E. F. G.\n',
                         '1. Run:\n\n ```\n x\n ```\n\n   ' + words(21) + '\n',
                         '1. Short.\n\n ' + words(21) + '\n')
        self.assertReports('1. Restart the agent.\n\n\t' + words(21) + '\n', 'L3: 21 words (max 20)')
        self.assertReports('- A. B. C.\n  ```\n  x\n  ```\n  D. E. F. G.\n', 'L1: paragraph has 7')
        self.assertReports('- A. B. C.\n\n\tD. E. F. G.\n', 'L1: paragraph has 7')
        self.assertClean('100. Step.\n\n\tMore text; here.\n', '1.  Short.\n\n   ' + words(21) + '\n',
                         '1.    Short.\n\n   ' + words(21) + '\n', '1.     code; x\n',
                         '1.\n  ' + words(21) + '\n',
                         '[^1]: A. B. C. D.\n\n   E. F. G.\n')
        self.assertReports('1.\n   ' + words(21) + '\n', 'L2: 21 words (max 20)')
        self.assertClean('1.\n\n   ' + words(21) + '\n\n2. Start the agent.\n')
        self.assertReports('1.\n   First.\n\n   ' + words(21) + '\n', 'L4: 21 words (max 20)')
        self.assertReports('To rotate the keys:\n01. Create the key.\n02. ' + words(21)
                           + '\n03. Revoke the old key.\n',
                           'L3: 21 words (max 20)')

    def test_task_boxes_and_footnotes(self):
        self.assertClean('## Checklist\n\n- [ ] 1. ' + words(22) + '\n- [ ] 2. Drain the node.\n')
        self.assertCounts('- [ ] # ' + words(23) + '\n', sentences=1, max=24)
        notes = ('Drain the node first.[^1] Then restart the agent.[^2]\n\n'
                 '[^1]: The drain can take ten minutes on a large node with many pods.\n'
                 '[^2]: The agent restarts on its own after a crash, so do not restart it twice.\n')
        self.assertCounts(notes, sentences=4, max=15)
        self.assertClean('1. Drain the node.[^1]\n[^1]: ' + words(22) + '\n')

    def test_no_break_spaces_are_text(self):
        nbsp = chr(0xA0)
        self.assertCounts('Run the backup first.\n\n' + nbsp * 4 + 'Run the backup; it takes five minutes.\n',
                          sentences=2, semicolons=1)
        self.assertReports('One. Two. Three. Four.\n' + nbsp + '\nFive. Six. Seven.\n', 'L1: paragraph has 7')

    def test_indent_and_marks_inside_code_and_quotes(self):
        self.assertClean('   ```\n   a; b\n   ```\n', '   ~~~\n   a; b\n   ~~~\n', '   # ' + words(30) + '\n')
        self.assertEqual(issues('Run `a` and `b` and `c` now\n; then stop.\n'), ['L2: semicolon'])
        self.assertEqual(issues('Say "a" and "b" and "c" now\n; then stop.\n'), ['L2: semicolon'])
        self.assertCounts('Intro.\n\n   Text; here.\n', semicolons=1)
        self.assertEqual(issues('Run `a` and `b` and `c` now\nthen we stop;\n'), ['L2: semicolon'])
        self.assertEqual(issues('Say "a" and "b" and "c" now\nthen we stop;\n'), ['L2: semicolon'])
        self.assertEqual(issues('Run `aaaa` and `bbbb` now; then\nwe stop; later.\n'),
                         ['L1: semicolon', 'L2: semicolon'])

    def test_list_items_hold_their_paragraphs(self):
        self.assertReports('1. Short.\n\n   ' + words(21) + '\n', 'L3: 21 words (max 20)')
        self.assertReports('- A. B. C.\n\n  D. E. F. G.\n', 'L1: paragraph has 7')
        self.assertReports('1. Run this:\n\n   ```\n   x\n   ```\n\n   ' + words(21) + '\n',
                           'L7: 21 words (max 20)')
        self.assertCounts('1. Run this:\n\n   ' + words(5) + '\n', sentences=2, max=5)
        self.assertReports('1. Run this:\n   ```\n   x\n   ```\n   ' + words(21) + '\n',
                           'L5: 21 words (max 20)')
        self.assertReports('[^1]: A. B. C.\n\n    D. E. F. G.\n', 'paragraph has 7')
        self.assertClean('1. Item.\n\n## Head\n  ' + words(21) + '\n',
                         '1. Step.\n\nIntro.\n\n  ' + words(21) + '\n')
        self.assertClean('1. Short.\n\n' + words(21) + '\n',
                         '1. Run this:\n\n```\nx\n```\n\n  ' + words(21) + '\n',
                         '- Do it.\n  - ' + words(25) + '\n')
        self.assertReports('1. Do it.\n   - ' + words(21) + '\n', 'L2: 21 words (max 20)')

    def test_tables(self):
        self.assertCounts('| A; B | C |\n|---|---|\n| x | y |\n', sentences=4, semicolons=1)
        self.assertReports('A | B\n--- | ---\n' + words(26) + ' | ok\n', 'L3: 26 words (max 25)')
        self.assertCounts('A | B\n:-- | --:\nx | y\nz | w\n', sentences=6)
        self.assertCounts('| a \\| b; c | d |\n|---|---|\n', sentences=2, semicolons=1)
        self.assertCounts('A | B\n--- | ---\nx | y\n\nText one; two.\n', sentences=5, semicolons=1)
        self.assertCounts('Use a | b to pipe it; done.\n', sentences=1, semicolons=1)
        self.assertClean(words(13, end='') + '\n| a | b |\n|---|---|\n\n' + words(13) + '\n')
        self.assertReports(words(13, end='') + '\n| a | b |\n' + words(13) + '\n', '31 words')
        self.assertClean('| Check | Result |\n|---|---|\n| disk | ok |\nOne. Two. Three.\n'
                         'Four. Five. Six. Seven.\n')
        self.assertReports('1. ' + words(17, end='') + '\n   | sort -u before you count them.\n',
                           'L1: 24 words (max 20)')
        self.assertReports('| a | b |\n|---|---|\n| c | d |\n\n' + words(13, end='') + ' | ' + words(13) + '\n',
                           'L5: 27 words (max 25)')
        self.assertCounts('- Item:\n\n  A | B\n  |---|---|\n  x | y\n', sentences=5)
        self.assertCounts('Intro text\nA | B\n--- | ---\nx | y\n', sentences=5)
        self.assertCounts('Pick a | b\n| |\n', sentences=1)
        self.assertCounts('Pick a | b\n| | |\n', sentences=1)
        self.assertCounts('A | B\r\n--- | ---\r\nx | y\r\n', sentences=4)
        self.assertEqual(issues('| A; B | C |\n|---|---|\n'), ['L1: semicolon'])
        self.assertReports('> 1. Check:\n>\n>    | A | B |\n>    |---|---|\n>    | x | ' + words(21) + ' |\n',
                           'L5: 21 words (max 20)')
        self.assertClean('| Command | Effect |\n|---|---|\n'
                         '| `ps aux | grep agent` | Lists the agent; then exits. |\n',
                         '- Check:\n\n  | A | B |\n  |---|---|\n  | x | ' + words(25) + ' |\n')
        self.assertReports('1. Check:\n\n   | A | B |\n   |---|---|\n   | x | ' + words(21) + ' |\n',
                           'L5: 21 words (max 20)')
        self.assertCounts('> Intro text\n> A | B\n> --- | ---\nLazy text.\n', sentences=4)

    def test_headings_rules_and_definitions(self):
        self.assertCounts('Title\n===\nText one.\n', sentences=1)
        self.assertCounts("Don't stop\n---\nText one.\n", sentences=1, contractions=1)
        self.assertCounts('Title\n=\nText one.\n', sentences=1, max=2)
        self.assertCounts('Title one. Two.\n===\nText one.\n', sentences=1, max=2)
        self.assertEqual(issues("Intro.\n\nDon't stop\nnow\n---\nText one.\n"), ["L3: contraction 'Don't'"])
        self.assertCounts('- First one.\n\n  Title\n  ---\n', sentences=1)
        self.assertClean('One. Two. Three. Four.\n***\nFive. Six. Seven.\n',
                         'One. Two. Three. Four.\n## Next\nFive. Six. Seven.\n',
                         '[x]: https://example.com/a;b\n', '[x]: docs/a;b.md "Title; here"\n',
                         '[^1]: ' + words(25) + '\n')
        self.assertCounts('[Note]: we do not stop; ever.\n', semicolons=1)
        self.assertClean('# ' + words(30) + '\n')
        self.assertEqual(issues(f"[rb]: https://x.example/rb 'Roll back {EM_DASH} v2'\n"), ['L1: em-dash'])
        self.assertClean("[rb]: https://x.example/rb 'Roll; back'\n", '[x]: docs/a.md (Title; here)\n',
                         "[runbook]: https://x.example/runbook\n  'Roll back; then drain'\n\nSee [runbook].\n",
                         "[guide]: <docs/run book.md> 'Roll back; then drain'\n\nSee [guide].\n")
        self.assertCounts("Use the guide below.\n[guide]: https://x.example/guide 'Roll back; then drain'\n",
                          semicolons=1)
        self.assertEqual(issues(f"[rb]: https://x.example/rb\n  'Roll back {EM_DASH} v2'\n"), ['L2: em-dash'])
        self.assertCounts('(Run it; then stop.)\n', semicolons=1)
        self.assertCounts('[^1]:Run it; then stop.\n', semicolons=1)
        self.assertCounts('[^1]:stop;now\n', semicolons=1)
        self.assertCounts("[x]: /u 'T'\n(Run it; then stop.)\n", semicolons=1)
        self.assertCounts("[^1]: Don't.\n", contractions=1)
        self.assertReports('#tag ' + words(25) + '\n', '26 words')
        self.assertReports('####### ' + words(25) + '\n', '26 words')
        self.assertReports('One. Two. Three. Four.\n**\nFive. Six. Seven.\n', 'paragraph has 7')
        self.assertClean('## Run `a; b` now\n')


class SplitTests(Case):
    def test_sentence_ends_after_bold_quote_or_paren(self):
        self.assertCounts('**Rule one.** ' + words(24) + '\n', sentences=2)
        self.assertClean("Say 'stop.' " + words(24) + '\n', f'Say {RSQUO}stop.{RSQUO} ' + words(24) + '\n')
        for lead in ('Stop now!', 'Is it up?', 'Ask "why?"', 'Shout "stop!"'):
            self.assertClean(lead + ' ' + words(24) + '\n')
        self.assertClean('Say "stop." ' + words(24) + '\n', '(Run it.) ' + words(24) + '\n',
                         f'Say {LDQUO}stop.{RDQUO} ' + words(24) + '\n', '*Run it.* ' + words(24) + '\n')

    def test_split_only_before_a_capital_digit_code_quote_or_bracket(self):
        tail = ' '.join(['word'] * 23) + ' end.'
        for lead in ('5', '"Stop"', '"stop"', f'{LDQUO}Stop{RDQUO}', '(Then', '[Then]', '`make`',
                     '**Then**', '_Then_'):
            self.assertClean('Run the job now. ' + lead + ' ' + tail + '\n')
        self.assertCounts('Remove pods, jobs, etc. and then restart.\n', sentences=1)
        self.assertCounts('Use a, b, etc. Then go home now.\n', sentences=2)

    def test_abbreviations_do_not_end_a_sentence(self):
        self.assertCounts('Use a tool, e.g. the grep tool.\n', sentences=1)
        self.assertCounts('Use a tool, i.e. Grep, to find it.\n', sentences=1)
        self.assertCounts('Use a tool, e.g. `grep`, to find it.\n', sentences=1)
        self.assertCounts('Compare Python vs. Go here. Ask Dr. Lee. See No. 5 and Fig. 3 now.\n', sentences=3)
        self.assertCounts('Ask Mr. Lee. Ask Mrs. Lee. Ask Ms. Lee. Cf. The doc. Approx. Ten. Incl. Two.\n',
                          sentences=6)
        self.assertCounts('Read No. Then go. See Sec. 2 now.\n', sentences=3)
        army = ('The U.S. Army team and the U.S. Navy team ran the long test on the old cluster '
                'in the north lab and wrote the full report for the review.\n')
        self.assertReports(army, '29 words (max 25)')
        self.assertCounts('It runs in the U.S. Then it stops.\n', sentences=1)
        for text in ('Restart the systems. Then check them.', 'Update the config. 3 pods restart.',
                     'Keep it below 250 ms. Then go on.', 'Fail over to DR. Then page the team.',
                     'Start at 2 a.m. Then drain the node.'):
            self.assertCounts(text + '\n', sentences=2)
        self.assertCounts('Start at 2 a.m. UTC on Monday.\n', sentences=1)
        self.assertCounts('Ask the devs. Then go.\n', sentences=2)
        self.assertCounts('Visit the casino. 5 tables are open.\n', sentences=2)

    def test_footnote_marks(self):
        self.assertCounts('It works.[^1] Then it stops.\n', sentences=2)
        self.assertCounts('It works [^note] well.\n', max=3)


class InterfaceTests(Case):
    def test_stdin_and_many_files(self):
        r = cli('-', stdin='We utilize it.\n')
        self.assertEqual(r.returncode, 1, r.stdout)
        self.assertIn('stdin: 1 sentences', r.stdout)
        front = chr(0xFEFF) + '---\nname: x; y\ndescription: ' + words(40) + '\n---\nText.\n'
        self.assertEqual(cli('-', stdin=front).returncode, 0)
        with tempfile.TemporaryDirectory() as temp:
            bad, good, bom = Path(temp) / 'bad.md', Path(temp) / 'good.md', Path(temp) / 'bom.md'
            bad.write_text("We don't stop.\n")
            good.write_text('The card is short.\n')
            bom.write_text(front, encoding='utf-8')
            self.assertEqual(cli(str(bad), str(good)).returncode, 1)
            self.assertEqual(cli(str(good), str(bom)).returncode, 0)

    def test_unreadable_input_exits_2(self):
        with tempfile.TemporaryDirectory() as temp:
            good, latin = Path(temp) / 'good.md', Path(temp) / 'latin.md'
            good.write_text('The card is short.\n')
            latin.write_bytes('Caf\xe9 is open.\n'.encode('latin-1'))
            for target in (Path(temp) / 'missing.md', latin, Path(temp)):
                r = cli(str(target), str(good))
                self.assertEqual(r.returncode, 2, target)
                self.assertIn('cannot read', r.stderr)
                self.assertIn('good.md: 1 sentences', r.stdout)
        r = cli()
        self.assertEqual((r.returncode, r.stdout), (2, ''))
        self.assertIn('Usage:', r.stderr)
        r = subprocess.run([sys.executable, str(SCRIPT), '-'], preexec_fn=lambda: os.close(0),
                           capture_output=True, text=True, timeout=30)
        self.assertEqual(r.returncode, 2, r.stderr)
        self.assertIn('cannot read', r.stderr)

    def test_report_format(self):
        self.assertCounts('One. One two. One two three four five six seven eight nine.\n', median=2, max=9)
        self.assertCounts('One. One two.\n', median=1.5)
        self.assertCounts('**One** and **two** go.\n', **{'bold spans': 2})
        self.assertEqual(issues("## Don't stop\n\nWe utilize it.\n"),
                         ["L1: contraction 'Don't'", "L3: 'utilize', write 'use'"])
        self.assertReports('Run `make` ' + words(24) + '\n', 'L1: 26 words (max 25): Run `make` Word')
        self.assertEqual(issues('Line one.' + chr(0x2028) + 'Still one.\nWe utilize it.\n'),
                         ["L2: 'utilize', write 'use'"])
        self.assertEqual(issues('One; two.\r\nThree.\r\n'), ['L1: semicolon'])
        self.assertEqual(issues("Text one.\nWe don't stop; ever."),
                         ['L2: semicolon', "L2: contraction 'don't'"])
        self.assertEqual(run('')[1], 'draft.md: 0 sentences, median 0 words, max 0\n'
                                     '  em-dashes 0, semicolons 0, contractions 0, bold spans 0\n')

    def test_the_card_passes_its_own_rules(self):
        r = cli(str(CARD))
        self.assertEqual(r.returncode, 0, r.stdout)

    def test_output_style_links_to_the_card(self):
        self.assertTrue(STYLE.is_symlink())
        self.assertFalse(STYLE.readlink().is_absolute())
        self.assertEqual(STYLE.resolve(), CARD.resolve())


if __name__ == '__main__':
    unittest.main()
