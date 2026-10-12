#!/usr/bin/env python3
"""The portability, privacy, and prose scans that `make check` runs.

Usage: python3 tools/scan.py [portability | privacy | prose]...

With no args it runs all three scans, else only the named ones, in the order
given. Each scan prints `FAIL <scan> <path>:<line>: <reason>` once per line
and rule that it flags, or `ok <scan>` when it flags nothing. It exits 1 on
any hit, 2 on a usage or setup error, and 0 otherwise.

Every scan reads its file list from skillkit.files(): tracked and untracked
files that git does not ignore, outside import/. A file with a NUL byte in
its first 8 KiB is binary and skipped, and no scan reads a symlink's content.

portability (FND-5): a skillkit.DOTFILES_PATTERNS match in a file that
    tools/portability-allow.txt does not list, and a symlink whose target is
    an absolute path (line 1, the one line that git stores for a link).
privacy (FND-6): an email address (ruling R3) other than the built-in
    allowed ones, and a match of an extra regex. Extra regexes come one per
    line, blank and `#` lines skipped, from the file that
    $MONET_PRIVACY_PATTERNS names when it is set and non-empty, else from
    ~/.config/monet/privacy-patterns when it exists. A hit names the regex's
    line in that file, never the matched text, the address, or the regex.
prose (FND-7): an em-dash (U+2014) in a file whose name ends in .md, in any
    case.
"""

import os
import re
import sys
import warnings
from pathlib import Path

# `make check` runs this file directly, so keep the import below from writing
# tools/__pycache__ into the tree.
sys.dont_write_bytecode = True
import skillkit  # noqa: E402

SCANS = ("portability", "privacy", "prose")
ALLOWLIST = "tools/portability-allow.txt"
PATTERNS_ENV = "MONET_PRIVACY_PATTERNS"
BINARY_PROBE = 8192
EM_DASH = "\u2014"
EMAIL = re.compile(r"(?<![A-Za-z0-9._%+/@-])[A-Za-z0-9._%+-]+@[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)*\.[A-Za-z]{2,}")
# The built-in allowed addresses, by domain and by (local part, domain).
ALLOWED_DOMAINS = {"example.com", "example.org", "example.net", "users.noreply.github.com"}
ALLOWED_ADDRESSES = {("git", "github.com")}


class SetupError(Exception):
    """A missing or unreadable input file, or an invalid regex: exit 2."""


def allowed_email(address):
    local, _, domain = address.lower().partition("@")
    return local == "noreply" or domain in ALLOWED_DOMAINS or (local, domain) in ALLOWED_ADDRESSES


def _read_lines(path):
    try:
        # utf-8-sig drops a leading BOM, which would hide the first regex or entry.
        return Path(path).read_text(encoding="utf-8-sig").split("\n")
    except (OSError, ValueError) as e:
        raise SetupError(f"cannot read {path}: {getattr(e, 'strerror', None) or e}") from None


def allowlist(repo=skillkit.REPO):
    """The repo paths in the portability allowlist. A `#` starts a comment."""
    entries = (line.split("#", 1)[0].strip() for line in _read_lines(Path(repo) / ALLOWLIST))
    return {entry for entry in entries if entry}


def privacy_patterns():
    """(line number, compiled regex) for each extra privacy regex."""
    env = os.environ.get(PATTERNS_ENV, "")
    path = Path(env) if env else Path.home() / ".config" / "monet" / "privacy-patterns"
    if not env and not path.exists():
        return []
    patterns = []
    for n, line in enumerate(_read_lines(path), 1):
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        try:
            # A re warning can quote the regex (Python 3.9 and 3.10 warn on a
            # flag that 3.11 rejects), so every warning makes the regex invalid.
            with warnings.catch_warnings():
                warnings.simplefilter("error")
                patterns.append((n, re.compile(line)))
        except (re.error, OverflowError, RecursionError, Warning) as e:
            # The regex is never printed, since it may name a private term.
            pos = getattr(e, "pos", None)
            where = f" ({type(e).__name__})" if isinstance(e, Warning) else ""
            where += "" if pos is None else f" (error at position {pos})"
            raise SetupError(f"{path} line {n}: invalid regex{where}") from None
    return patterns


def load(repo):
    """(path, symlink target or None, text or None) for each file to scan."""
    entries = []
    for path in skillkit.files(repo):
        full = os.path.join(repo, path)
        if os.path.islink(full):
            entries.append((path, os.readlink(full), None))
            continue
        with open(full, "rb") as f:
            data = f.read()
        binary = b"\0" in data[:BINARY_PROBE]
        entries.append((path, None, None if binary else data.decode("utf-8", errors="replace")))
    return entries


def _lines(text):
    return enumerate(text.split("\n"), 1) if text is not None else ()


def portability(entries, allow):
    hits = []
    for path, link, text in entries:
        if link is not None and os.path.isabs(link):
            hits.append((path, 1, "symlink to an absolute path"))
        if path not in allow:
            hits += [(path, n, f"dotfiles path matching {p.pattern}")
                     for n, line in _lines(text) for p in skillkit.DOTFILES_PATTERNS if p.search(line)]
    return hits


def privacy(entries, patterns):
    hits = []
    for path, _, text in entries:
        for n, line in _lines(text):
            if any(not allowed_email(m.group()) for m in EMAIL.finditer(line)):
                hits.append((path, n, "email address not on the allowed list"))
            hits += [(path, n, f"matches line {k} of the privacy patterns file")
                     for k, p in patterns if p.search(line)]
    return hits


def prose(entries):
    return [(path, n, "em-dash (U+2014)") for path, _, text in entries if path.lower().endswith(".md")
            for n, line in _lines(text) if EM_DASH in line]


def main(argv=None, repo=skillkit.REPO):
    names = list(dict.fromkeys(sys.argv[1:] if argv is None else argv)) or list(SCANS)
    try:
        unknown = [name for name in names if name not in SCANS]
        if unknown:
            raise SetupError(f"unknown scan {unknown[0]!r}; the scans are {', '.join(SCANS)}")
        allow = allowlist(repo) if "portability" in names else set()
        patterns = privacy_patterns() if "privacy" in names else []
    except SetupError as e:
        print(f"scan: error: {e}", file=sys.stderr)
        return 2
    entries = load(repo)
    runs = {"portability": lambda: portability(entries, allow),
            "privacy": lambda: privacy(entries, patterns), "prose": lambda: prose(entries)}
    failed = False
    for name in names:
        hits = runs[name]()
        for path, n, reason in hits:
            print(f"FAIL {name} {path}:{n}: {reason}")
        if not hits:
            print(f"ok {name}")
        failed = failed or bool(hits)
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
