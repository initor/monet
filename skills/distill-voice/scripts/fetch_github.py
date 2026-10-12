#!/usr/bin/env python3
"""Fetch a GitHub user's PR descriptions and comments through the gh CLI.

    python3 fetch_github.py --user <login> (--orgs <a,b> | --repos <o/r,...>) --out <dir>
                            [--since YYYY-MM-DD] [--until YYYY-MM-DD]
                            [--backoff-base <seconds>] [--max-retries <n>]

Calls only `gh api`, with the account that `gh auth status` shows. For each org
or repo it searches the PRs the user authored (created in the window), and the
PRs the user commented on or reviewed. It then reads each of those PRs'
conversation comments, review comments, and review bodies, and keeps the ones
the user wrote in the window.

Writes `<dir>/descriptions.json` and `<dir>/comments.json`. Each record has a
boolean `agent_authored` and the list `agent_tells` of the tells that matched
(see the skill's reference/agent-content-filtering.md). A comment record also
has `kind` (`issue_comment`, `review_comment`, or `review`) and `own_pr`.

A call that gh fails with "rate limit" on stderr (any case) is retried after
`backoff-base * 2**n` seconds, up to `--max-retries` times, and the longest
wait may be at most one day. Any other failure stops the run, and nothing is
written. Exits 2 with a usage line on bad
arguments, and 1 when gh fails.
"""
import argparse
import datetime
import json
import math
import re
import shutil
import subprocess
import sys
import time
from pathlib import Path
from urllib.parse import urlencode

PER_PAGE = 100
SEARCH_PAGES = 10  # The search API returns at most 1000 results per query.
MAX_WAIT = 24 * 3600  # The longest retry wait in seconds. time.sleep refuses about 9.2e9.
LOGIN = re.compile(r'[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?')
# A managed-user login adds an underscore and a short code, as in `octo_acme`.
USER = re.compile(r'[A-Za-z0-9](?:[A-Za-z0-9_-]*[A-Za-z0-9])?')
REPO = re.compile(r'[A-Za-z0-9_-]+/[A-Za-z0-9._-]+')
AI_TOOL = r'(?:claude|copilot|chatgpt|gpt-?\d[\w.-]*|codex|cursor|gemini|devin|aider)'

# Each tell maps to a test on an item's body. The ids are documented in
# reference/agent-content-filtering.md.
TELLS = {
    'attribution': re.compile(
        r'(?im)\b(?:generated|created|written|drafted)\s+(?:with|by|using)\s+(?:the\s+)?\[?' + AI_TOOL + r'\b'
        r'|^\s*co-authored-by:.*\b' + AI_TOOL + r'\b|\U0001f916').search,
    'session-link': re.compile(
        r'(?i)\b(?:claude\.ai/(?:code|chat|share)/|chatgpt\.com/(?:share|c|g)/|chat\.openai\.com/(?:share|c)/'
        r'|gemini\.google\.com/(?:share|app)/)').search,
    'template': lambda text: bool(re.search(r'(?im)^#{1,3}\s*summary\s*$', text)
                                  and re.search(r'(?im)^#{1,3}\s*test plan\s*$', text)),
}


class GhError(Exception):
    pass


def tells(text):
    return [name for name, test in TELLS.items() if text and test(text)]


def gh_api(endpoint, args):
    """Run `gh api <endpoint>` and return its JSON, retrying a rate-limited call."""
    for attempt in range(args.max_retries + 1):
        proc = subprocess.run(['gh', 'api', endpoint], capture_output=True, text=True)
        if proc.returncode == 0:
            try:
                return json.loads(proc.stdout or 'null')
            except (ValueError, RecursionError):
                raise GhError(f'gh api {endpoint}: the output is not JSON') from None
        err = proc.stderr.strip()
        if 'rate limit' not in err.lower():
            raise GhError(f'gh api {endpoint} failed: {err}')
        if attempt == args.max_retries:
            raise GhError(f'gh api {endpoint} failed after {args.max_retries} retries: {err}')
        delay = args.backoff_base * 2 ** attempt
        print(f'fetch_github: rate limited on {endpoint}, retry {attempt + 1} of {args.max_retries} '
              f'in {delay:g}s', file=sys.stderr)
        time.sleep(delay)


def search(qualifiers, args):
    query = ' '.join(qualifiers)
    items, total = [], 0
    for page in range(1, SEARCH_PAGES + 1):
        endpoint = 'search/issues?' + urlencode({'q': query, 'per_page': PER_PAGE, 'page': page})
        data = gh_api(endpoint, args) or {}
        if not isinstance(data, dict) or not isinstance(data.get('items') or [], list):
            raise GhError(f'gh api {endpoint}: expected a search result object')
        batch, total = data.get('items') or [], data.get('total_count') or 0
        items += batch
        if len(batch) < PER_PAGE:
            break
    if total > len(items):
        print(f'fetch_github: {query!r} matched {total} PRs, and search returns {len(items)}. '
              'Narrow it with --since and --until.', file=sys.stderr)
    return items


def pages(path, args):
    items, page = [], 1
    while True:
        endpoint = f'{path}?' + urlencode({'per_page': PER_PAGE, 'page': page})
        batch = gh_api(endpoint, args) or []
        if not isinstance(batch, list):
            raise GhError(f'gh api {endpoint}: expected a list')
        items += batch
        if len(batch) < PER_PAGE:
            return items
        page += 1


def repo_of(item):
    url = item.get('repository_url') or ''
    return url.split('/repos/', 1)[1] if '/repos/' in url else '/'.join(item['html_url'].split('/')[3:5])


def login_of(item):
    return ((item.get('user') or {}).get('login') or '').lower()


def in_window(date, args):
    day = (date or '')[:10]
    return not day or ((not args.since or day >= args.since) and (not args.until or day <= args.until))


def tagged(record):
    record['agent_tells'] = tells(record['body'])
    record['agent_authored'] = bool(record['agent_tells'])
    return record


def created_range(args):
    if args.since and args.until:
        return [f'created:{args.since}..{args.until}']
    return [f'created:>={args.since}'] if args.since else [f'created:<={args.until}'] if args.until else []


def fetch(args):
    user = args.user.lower()
    scopes = [f'org:{o}' for o in args.orgs] if args.orgs else [f'repo:{r}' for r in args.repos]
    authored, commented = {}, {}
    for scope in scopes:
        for item in search(['type:pr', f'author:{args.user}', scope] + created_range(args), args):
            authored.setdefault((repo_of(item), item['number']), item)
        updated = [f'updated:>={args.since}'] if args.since else []
        for who in ('commenter', 'reviewed-by'):
            for item in search(['type:pr', f'{who}:{args.user}', scope] + updated, args):
                commented.setdefault((repo_of(item), item['number']), item)
    descriptions = [tagged({'repo': repo, 'number': number, 'url': item.get('html_url'),
                            'title': item.get('title') or '', 'body': item.get('body') or '',
                            'created_at': item.get('created_at')})
                    for (repo, number), item in authored.items()]
    comments = []
    for (repo, number), item in commented.items():
        own = login_of(item) == user
        for kind, path, date_key in (('issue_comment', f'repos/{repo}/issues/{number}/comments', 'created_at'),
                                     ('review_comment', f'repos/{repo}/pulls/{number}/comments', 'created_at'),
                                     ('review', f'repos/{repo}/pulls/{number}/reviews', 'submitted_at')):
            for note in pages(path, args):
                body, date = note.get('body') or '', note.get(date_key)
                if login_of(note) == user and body.strip() and in_window(date, args):
                    comments.append(tagged({'repo': repo, 'number': number, 'kind': kind, 'own_pr': own,
                                            'url': note.get('html_url') or item.get('html_url'),
                                            'body': body, 'created_at': date}))
    order = lambda r: (r['created_at'] or '', r['repo'], r['number'])
    return sorted(descriptions, key=order), sorted(comments, key=order)


def csv(pattern, what):
    def parse(value):
        items = [v.strip() for v in value.split(',') if v.strip()]
        if not items or not all(pattern.fullmatch(v) for v in items):
            raise argparse.ArgumentTypeError(f'expected a comma-separated list of {what}, got {value!r}')
        return items
    return parse


def day(value):
    try:
        return datetime.date.fromisoformat(value).isoformat()
    except ValueError:
        raise argparse.ArgumentTypeError(f'expected a date as YYYY-MM-DD, got {value!r}') from None


def main(argv=None):
    parser = argparse.ArgumentParser(prog='fetch_github.py',
                                     description="Fetch a user's PR descriptions and comments with gh.")
    parser.add_argument('--user', required=True, help='the GitHub login whose writing to fetch')
    scope = parser.add_mutually_exclusive_group(required=True)
    scope.add_argument('--orgs', type=csv(LOGIN, 'orgs'), help='orgs to search, comma-separated')
    scope.add_argument('--repos', type=csv(REPO, 'owner/repo names'), help='repos to search, comma-separated')
    parser.add_argument('--out', type=Path, required=True, help='the dir to write the JSON files to')
    parser.add_argument('--since', type=day, help='first day to include, YYYY-MM-DD')
    parser.add_argument('--until', type=day, help='last day to include, YYYY-MM-DD')
    parser.add_argument('--backoff-base', type=float, default=30.0,
                        help='seconds before the first retry of a rate-limited call (default 30)')
    parser.add_argument('--max-retries', type=int, default=6, help='retries of a rate-limited call (default 6)')
    args = parser.parse_args(argv)
    if not USER.fullmatch(args.user):
        parser.error(f'--user {args.user!r} is not a GitHub login')
    if args.since and args.until and args.since > args.until:
        parser.error('--since is after --until')
    if not math.isfinite(args.backoff_base) or args.backoff_base < 0 or args.max_retries < 0:
        parser.error('--backoff-base must be a finite number, and neither it nor --max-retries negative')
    # log2 keeps the check cheap for a huge --max-retries, where 2**n would be a huge int.
    if args.max_retries and args.backoff_base and args.max_retries - 1 > math.log2(MAX_WAIT / args.backoff_base):
        parser.error(f'the longest retry wait, --backoff-base * 2**(--max-retries - 1), is over {MAX_WAIT} s')
    if args.out.exists() and not args.out.is_dir():
        parser.error(f'--out {args.out} is a file, not a dir')
    if any(p.exists() and not p.is_dir() for p in args.out.parents):
        parser.error(f'--out {args.out} is under a file')
    if not shutil.which('gh'):
        print('fetch_github: gh is not on PATH. Install the GitHub CLI and run gh auth login.', file=sys.stderr)
        return 1
    try:
        descriptions, comments = fetch(args)
    except GhError as e:
        print(f'fetch_github: {e}', file=sys.stderr)
        return 1
    args.out.mkdir(parents=True, exist_ok=True)
    for name, records in (('descriptions.json', descriptions), ('comments.json', comments)):
        (args.out / name).write_text(json.dumps(records, indent=2, ensure_ascii=False) + '\n', encoding='utf-8')
    count = lambda records: f'{len(records)} ({sum(r["agent_authored"] for r in records)} tagged)'
    print(f'fetch_github: wrote {count(descriptions)} descriptions and {count(comments)} comments to {args.out}',
          file=sys.stderr)
    return 0


if __name__ == '__main__':
    sys.exit(main())
