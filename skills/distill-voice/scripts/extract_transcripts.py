#!/usr/bin/env python3
"""Pull the user's own typed turns out of Claude Code transcripts.

    python3 extract_transcripts.py --out <file> [--projects-dir <dir>]

Reads each `<dir>/*/*.jsonl` (one session per file, one JSON record per line),
in path order, and never reads a file under a session's `subagents/` dir. The
projects dir defaults to `projects` in the Claude Code config dir under the
home dir.

A record is kept only when it is a typed user turn: `type` is `user`,
`isSidechain` and `isMeta` are not true, it has none of the keys
`toolUseResult`, `sourceToolUseID`, and `isCompactSummary`, and its `origin`
(when it is an object) has none of the keys `from`, `senderTaskId`, and
`producer`. Its `message.content` is a string or a list of blocks, and only
`text` blocks count. A text is dropped when the CLI put it there: it starts
with a lowercase tag such as `<command-name>`, `<local-command-stdout>`,
`<system-reminder>`, or `<bash-stdout>`, or with `[Request interrupted by user`.
A text is also dropped when it is a pasted dump (over 2000 characters or over
40 lines), or when it holds a fenced block over 20 lines.

Writes the kept texts of each record as one message, the messages separated by
a line `---`. A character that UTF-8 cannot encode, such as a lone surrogate,
is written as `?`. A line that is not a JSON object is skipped and counted on
stderr, and a UTF-8 byte order mark at the start of a file is ignored. Exits 2
with a usage line on bad arguments.
"""
import argparse
import json
import re
import sys
from pathlib import Path

DROP_KEYS = ('toolUseResult', 'sourceToolUseID', 'isCompactSummary')
ORIGIN_KEYS = ('from', 'senderTaskId', 'producer')
# Text the CLI writes into a user turn: a lowercase tag (`<command-name>`, `<local-command-stdout>`,
# `<system-reminder>`, `<bash-input>`, `<bash-stdout>`, `<user-prompt-submit-hook>`, ...) or the interrupt
# marker. Claude Code's own test for text the user did not type has the same shape. A text that starts
# `<command-` or `<local-command-` counts even when the tag is cut short.
INJECTED = re.compile(r'<(?:command-|local-command-|[a-z][\w-]*[\s>])|\[Request interrupted by user')
MAX_CHARS, MAX_LINES, MAX_FENCE_LINES = 2000, 40, 20
# A fence opens after at most three spaces (a tab makes an indented code line), and a backtick
# fence's info string holds no backtick, so a one-line ```code``` span opens no fence.
FENCE = re.compile(r' {0,3}(`{3,}(?!.*`)|~{3,})')
SEPARATOR = '\n---\n'


def default_projects_dir():
    return Path.home() / '.claude' / 'projects'


def is_typed_turn(record):
    if record.get('type') != 'user' or record.get('isSidechain') is True or record.get('isMeta') is True:
        return False
    if any(key in record for key in DROP_KEYS):
        return False
    origin = record.get('origin')
    return not (isinstance(origin, dict) and any(key in origin for key in ORIGIN_KEYS))


def texts(record):
    message = record.get('message')
    content = message.get('content') if isinstance(message, dict) else None
    if isinstance(content, str):
        return [content]
    if isinstance(content, list):
        return [block['text'] for block in content if isinstance(block, dict)
                and block.get('type') == 'text' and isinstance(block.get('text'), str)]
    return []


def longest_fence(lines):
    """The line count inside the longest fenced block. An unclosed fence runs to the end."""
    longest, opener, count = 0, None, 0
    for line in lines:
        m = FENCE.match(line)
        if opener is None:
            if m:
                opener, count = m.group(1), 0
        # A closer holds only the opener's char (so no info string), at least as many of it.
        elif m and len(m.group(1)) >= len(opener) and not line.strip().strip(opener[0]):
            longest, opener = max(longest, count), None
        else:
            count += 1
    return max(longest, count) if opener else longest


def keep(text):
    if not text or INJECTED.match(text) or len(text) > MAX_CHARS:
        return False
    lines = text.splitlines()
    return len(lines) <= MAX_LINES and longest_fence(lines) <= MAX_FENCE_LINES


def sessions(root):
    return [p for p in sorted(root.glob('*/*.jsonl')) if p.is_file() and p.parent.name != 'subagents']


def extract(root):
    """Return (messages, malformed line count, file count)."""
    messages, malformed, files = [], 0, sessions(root)
    for path in files:
        with path.open(encoding='utf-8-sig', errors='replace') as f:
            for line in f:
                if not line.strip():
                    continue
                try:
                    record = json.loads(line)
                except (ValueError, RecursionError):
                    # RecursionError: a deeply nested line is malformed too, not a crash.
                    record = None
                if not isinstance(record, dict):
                    malformed += 1
                    continue
                if is_typed_turn(record):
                    kept = [t.strip() for t in texts(record) if keep(t.strip())]
                    if kept:
                        messages.append('\n\n'.join(kept))
    return messages, malformed, len(files)


def main(argv=None):
    parser = argparse.ArgumentParser(prog='extract_transcripts.py',
                                     description="Extract the user's typed turns from Claude Code transcripts.")
    parser.add_argument('--projects-dir', type=Path, default=None,
                        help='the Claude Code projects dir (default: projects in the config dir under home)')
    parser.add_argument('--out', type=Path, required=True, help='the messages file to write')
    args = parser.parse_args(argv)
    root = args.projects_dir or default_projects_dir()
    if not root.is_dir():
        parser.error(f'projects dir {root} does not exist')
    if args.out.is_dir():
        parser.error(f'--out {args.out} is a dir, not a file')
    if any(p.exists() and not p.is_dir() for p in args.out.parents):
        parser.error(f'--out {args.out} is under a file')
    messages, malformed, files = extract(root)
    args.out.parent.mkdir(parents=True, exist_ok=True)
    # A lone surrogate (valid JSON, such as `\ud83d`) cannot be encoded, so it is written as `?`.
    args.out.write_text(SEPARATOR.join(messages) + '\n' if messages else '', encoding='utf-8',
                        errors='replace')
    print(f'extract_transcripts: {len(messages)} messages from {files} sessions written to {args.out}, '
          f'{malformed} malformed lines skipped', file=sys.stderr)
    return 0


if __name__ == '__main__':
    sys.exit(main())
