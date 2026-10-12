#!/usr/bin/env python3
"""Measure a corpus per register, in the terms of a persona's Computed metrics.

    python3 analyze_style.py [--github-dir <dir>] [--transcripts <file>]
                             [--text <register>=<glob>]...

Registers come from the inputs: `pr-descriptions` from descriptions.json,
`pr-review` and `pr-replies` from comments.json (a comment on someone else's
PR, or on the user's own), `transcripts` from the extracted messages (split on
lines `---`), and one register per `--text` name, where each matched file is
one item. A record with `agent_authored` true is skipped and counted.

Before measuring, it strips HTML comments, fenced code blocks, table rows
(with or without outer pipes), headings, quote lines (another writer's words),
HTML tags, link URLs, and HTML entities, and it turns each inline code span
into one word. A blank line, a stripped line, or a list line starts a new
block, and a sentence ends at `.`, `!`, or `?` before a space.

Prints one line per register: items, tagged items skipped, words, em-dashes
per 1000 words, semicolons per 1000 words, contraction rate, median sentence
length with p90 and max, sentences over 30 words, bold spans per 1000 words,
and lowercase sentence-start rate. Then one line that starts
`baseline (short only):` with the same metrics over the items of at most 60
words, across all registers. Exits 2 with a usage line on bad arguments.
"""
import argparse
import glob
import json
import os
import re
import statistics
import sys
from pathlib import Path

SHORT_WORDS = 60
WORD = re.compile(r"[^\W_]+(?:['\u2019][^\W_]+)*")
FIRST = re.compile(r'[^\W_]')
# The fence rules of extract_transcripts.py: at most three spaces of indent, no backtick in a
# backtick fence's info string.
FENCE = re.compile(r' {0,3}(`{3,}(?!.*`)|~{3,})')
HEADING = re.compile(r' {0,3}#{1,6}(\s|$)')
QUOTE = re.compile(r' {0,3}>')
# A GFM delimiter row, such as `--- | :---:`. The caller also asks for a `|` in it.
DELIMITER = re.compile(r'\|?\s*:?-+:?\s*(?:\|\s*:?-+:?\s*)*\|?')
# An HTML tag. Its attributes hold no `<`, so a run of unclosed tags costs linear time.
TAG = re.compile(r'</?[A-Za-z][A-Za-z0-9-]*(?:\s[^<>]*)?/?>')
BLOCK_START = re.compile(r'\s*(?:[-*+]\s+|\d+[.)]\s+)')
# A match starts only at the first mark of a run, so a long run of dots costs linear time.
SENTENCE_END = re.compile(r'(?<![.!?])[.!?]+["\')\]\u2019\u201d*_]*(?=\s|$)')
# A span never crosses another `**` or `__`, so a long line costs linear time.
BOLD = re.compile(r'\*\*(?=\S)(?:[^*\n]|\*(?!\*))+?(?<=\S)\*\*|__(?=\S)(?:[^_\n]|_(?!_))+?(?<=\S)__')
# `'s` counts only after a word it contracts, so a possessive is not a contraction.
CONTRACTION = re.compile(r"\b([^\W_]+)['\u2019](t|re|ll|ve|d|m|s)\b", re.I)
S_STEMS = {'it', 'that', 'there', 'here', 'what', 'who', 'where', 'he', 'she', 'let', 'how', 'when', 'why'}
FULL_FORM = re.compile(r"\b(?:(?:it|that|there|here|what|who|he|she) is|i am|(?:you|we|they) are"
                       r"|(?:i|you|we|they) (?:have|will|would)"
                       r"|(?:do|does|did|is|are|was|were|has|have|had|could|would|should|must|need) not"
                       r"|will not|can ?not|let us)\b", re.I)
REGISTER = re.compile(r'[A-Za-z0-9][\w.-]*')


def strip_comments(text):
    """The text with each closed HTML comment made a space, in linear time."""
    out, start = [], 0
    while True:
        opened = text.find('<!--', start)
        closed = text.find('-->', opened + 4) if opened >= 0 else -1
        if closed < 0:
            # With no `-->` after this `<!--`, no later comment closes either.
            return ''.join(out) + text[start:]
        out += [text[start:opened], ' ']
        start = closed + 3


def prose(text):
    """The text with code, tables, headings, quotes, comments, tags, URLs, and entities taken out.

    A line taken out leaves a blank line, so the prose on either side of it stays two blocks.
    """
    text = strip_comments(text)
    kept, fence, table = [], None, False
    for line in text.split('\n'):
        m = FENCE.match(line)
        table = table and '|' in line
        if fence:
            # A closer holds only the opener's char (so no info string), at least as many of it.
            if m and len(m.group(1)) >= len(fence) and not line.strip().strip(fence[0]):
                fence = None
            line = ''
        elif m:
            fence, line = m.group(1), ''
        elif '|' in line and (table or DELIMITER.fullmatch(line.strip())):
            # A delimiter row starts a table, and its header row is the line above it. The body
            # rows run to the first line with no `|`.
            if not table and kept and '|' in kept[-1]:
                kept[-1] = ''
            table, line = True, ''
        elif line.lstrip().startswith('|') or HEADING.match(line) or QUOTE.match(line):
            line = ''
        kept.append(line)
    text = re.sub(r'`[^`\n]+`', '0', '\n'.join(kept))
    text = TAG.sub(' ', text)
    # Link text holds no `[`, so a run of unclosed brackets costs linear time.
    text = re.sub(r'!?\[([^\][]*)\]\([^)]*\)', r'\1', text)
    text = re.sub(r'<https?://[^>\s]*>|https?://\S+', ' ', text)
    return re.sub(r'&(?:[A-Za-z]+|#[0-9]+|#x[0-9A-Fa-f]+);', ' ', text)


def split(block):
    parts, start = [], 0
    for m in SENTENCE_END.finditer(block):
        parts.append(block[start:m.end()])
        start = m.end()
    parts.append(block[start:])
    return [p.strip() for p in parts if WORD.search(p)]


def sentences(text):
    out, block = [], []
    for line in text.split('\n') + ['']:
        m = BLOCK_START.match(line)
        if not line.strip() or m:
            out += split(' '.join(block))
            block = []
        if line.strip():
            block.append(line[m.end():].strip() if m else line.strip())
    return out


class Stats:
    def __init__(self):
        self.items = self.words = self.em_dashes = self.semicolons = self.bold = 0
        self.contracted = self.full = self.lower = self.letter_starts = 0
        self.lengths = []

    @property
    def starts(self):
        return self.lower, self.letter_starts

    def add(self, text):
        """Measure one item and return its word count."""
        text = prose(text)
        words = len(WORD.findall(text))
        self.items += 1
        self.words += words
        self.em_dashes += text.count('\u2014')
        self.semicolons += text.count(';')
        self.bold += len(BOLD.findall(text))
        self.contracted += sum(1 for m in CONTRACTION.finditer(text)
                               if m.group(2).lower() != 's' or m.group(1).lower() in S_STEMS)
        self.full += len(FULL_FORM.findall(text))
        for sentence in sentences(text):
            self.lengths.append(len(WORD.findall(sentence)))
            first = FIRST.search(sentence).group()
            if first.isalpha():
                self.letter_starts += 1
                self.lower += first.islower()
        return words

    def metrics(self):
        def per_k(n):
            return f'{n * 1000 / self.words:.1f}' if self.words else 'n/a'

        def rate(part, whole):
            return f'{part * 100 / whole:.0f}% ({part} of {whole})' if whole else f'n/a ({part} of {whole})'

        ordered = sorted(self.lengths)
        if ordered:
            p90 = ordered[-(-9 * len(ordered) // 10) - 1]
            length = f'{statistics.median(ordered):g} (p90 {p90}, max {ordered[-1]})'
        else:
            length = 'n/a (p90 n/a, max n/a)'
        return (f'words {self.words}, em-dashes per 1000 words {per_k(self.em_dashes)}, '
                f'semicolons per 1000 words {per_k(self.semicolons)}, '
                f'contraction rate {rate(self.contracted, self.contracted + self.full)}, '
                f'median sentence length {length}, sentences over 30 words {sum(n > 30 for n in ordered)}, '
                f'bold spans per 1000 words {per_k(self.bold)}, '
                f'lowercase sentence-start rate {rate(self.lower, self.letter_starts)}')


def read_json(path, parser):
    try:
        data = json.loads(path.read_text(encoding='utf-8'))
    except (OSError, ValueError, RecursionError) as e:
        parser.error(f'cannot read {path}: {e}')
    if not isinstance(data, list):
        parser.error(f'{path} does not hold a list of records')
    return [r for r in data if isinstance(r, dict)]


def body(record):
    """A record's body, or '' when it is missing or not a string."""
    text = record.get('body')
    return text if isinstance(text, str) else ''


def collect(args, parser):
    """Return {register: [(text, tagged)]}, in the order the registers are printed."""
    registers = {}
    if args.github_dir:
        descriptions, comments = args.github_dir / 'descriptions.json', args.github_dir / 'comments.json'
        if not descriptions.is_file() and not comments.is_file():
            parser.error(f'--github-dir {args.github_dir} holds neither descriptions.json nor comments.json')
        if descriptions.is_file():
            registers['pr-descriptions'] = [(body(r), bool(r.get('agent_authored')))
                                            for r in read_json(descriptions, parser)]
        if comments.is_file():
            records = read_json(comments, parser)
            for name, own in (('pr-review', False), ('pr-replies', True)):
                registers[name] = [(body(r), bool(r.get('agent_authored')))
                                   for r in records if bool(r.get('own_pr')) == own]
    if args.transcripts:
        if not args.transcripts.is_file():
            parser.error(f'--transcripts {args.transcripts} is not a file')
        text = args.transcripts.read_text(encoding='utf-8', errors='replace')
        registers['transcripts'] = [(m.strip(), False) for m in re.split(r'(?m)^---[ \t]*$', text) if m.strip()]
    for name, pattern in args.text:
        paths = sorted(p for p in glob.glob(os.path.expanduser(pattern), recursive=True) if os.path.isfile(p))
        if not paths:
            parser.error(f'--text {name}={pattern} matches no file')
        registers.setdefault(name, []).extend(
            (Path(p).read_text(encoding='utf-8', errors='replace'), False) for p in paths)
    return registers


def text_arg(value):
    name, sep, pattern = value.partition('=')
    if not sep or not REGISTER.fullmatch(name) or not pattern:
        raise argparse.ArgumentTypeError(f'expected <register>=<glob>, got {value!r}')
    return name, pattern


def main(argv=None):
    parser = argparse.ArgumentParser(prog='analyze_style.py',
                                     description='Measure a corpus per register for a persona file.')
    parser.add_argument('--github-dir', type=Path, help='the dir that fetch_github.py wrote')
    parser.add_argument('--transcripts', type=Path, help='the messages file that extract_transcripts.py wrote')
    parser.add_argument('--text', type=text_arg, action='append', default=[],
                        help='a register name and a glob of its files, as <register>=<glob> (repeatable)')
    args = parser.parse_args(argv)
    if not (args.github_dir or args.transcripts or args.text):
        parser.error('give at least one of --github-dir, --transcripts, and --text')
    baseline = Stats()
    for name, items in collect(args, parser).items():
        stats, tagged = Stats(), 0
        for text, is_tagged in items:
            if is_tagged:
                tagged += 1
            elif stats.add(text) <= SHORT_WORDS:
                baseline.add(text)
        print(f'{name}: items {stats.items}, tagged skipped {tagged}, {stats.metrics()}')
    print(f'baseline (short only): items {baseline.items}, {baseline.metrics()}')
    return 0


if __name__ == '__main__':
    sys.exit(main())
