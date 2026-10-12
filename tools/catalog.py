#!/usr/bin/env python3
"""Generate the skill catalog in README.md from each skills/*/SKILL.md.

Usage: python3 tools/catalog.py [--root DIR] (--check | --write)

The catalog is a table between the BEGIN and END markers below, with one row
per skill, sorted by name: `/monet:<name>` (or `monet:<name>` for a skill with
`user-invocable: false`) and the first sentence of its `description`.

--check exits 0 when the catalog is current, and 1 with the drift when it is
stale, or when README.md, a marker, or a skill's frontmatter is missing or
broken. --write rewrites only the text between the markers. `make check` runs
--check. Stdlib only, and frontmatter is read with tools/skillkit.py.
"""

import argparse
import difflib
import re
import sys
from pathlib import Path

sys.dont_write_bytecode = True
sys.path.insert(0, str(Path(__file__).resolve().parent))
import skillkit  # noqa: E402

BEGIN = "<!-- BEGIN GENERATED:skills -->"
END = "<!-- END GENERATED:skills -->"
# A sentence ends at . ! or ?, plus any closing quote or bracket, before a
# space and a capital, a digit, code, a quote, or a bracket. So "e.g. this"
# does not end one.
_END = re.compile(r"[.!?][\"')\]]*(?=\s+[A-Z0-9`\"(\[])")


class CatalogError(Exception):
    """README.md, a marker, or a skill's frontmatter is missing or broken."""


def first_sentence(text):
    text = " ".join(text.split())
    m = _END.search(text)
    return text[:m.end()] if m else text


def row(name, description, invocable=True):
    ref = f"/monet:{name}" if invocable else f"monet:{name}"
    return f"| `{ref}` | {first_sentence(description).replace('|', chr(92) + '|')} |"


def render(root):
    """The text that belongs between the markers."""
    rows = []
    for path in skillkit.skills(root):
        try:
            skill = skillkit.read_skill(path)
        except ValueError as e:
            raise CatalogError(str(e)) from None
        meta = skill.frontmatter
        name, description = meta.get("name") or skill.name, meta.get("description")
        if not isinstance(description, str) or not description.strip():
            raise CatalogError(f"{path}: no `description` in the frontmatter")
        rows.append((name, row(name, description, meta.get("user-invocable") is not False)))
    table = ["| Skill | What it does |", "| --- | --- |"] + [r for _, r in sorted(rows)]
    return "\n" + "\n".join(table) + "\n\n"


def split(readme):
    """Return the README text before, between, and after the markers."""
    try:
        text = readme.read_text(encoding="utf-8")
    except OSError as e:
        raise CatalogError(f"cannot read {readme}: {e.strerror}") from None
    lines = text.splitlines(keepends=True)
    begins = [i for i, line in enumerate(lines) if line.strip() == BEGIN]
    ends = [i for i, line in enumerate(lines) if line.strip() == END]
    if len(begins) != 1 or len(ends) != 1 or ends[0] < begins[0]:
        raise CatalogError(f"{readme}: needs one {BEGIN} marker line, then one {END} marker line")
    b, e = begins[0], ends[0]
    return "".join(lines[:b + 1]), "".join(lines[b + 1:e]), "".join(lines[e:])


def main(argv=None):
    parser = argparse.ArgumentParser(description="Generate the README skill catalog.")
    parser.add_argument("--root", default=str(skillkit.REPO), help="repo root (default: this repo)")
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument("--check", action="store_true", help="exit 1 when the catalog is stale")
    mode.add_argument("--write", action="store_true", help="rewrite the catalog in place")
    args = parser.parse_args(argv)
    readme = Path(args.root) / "README.md"
    try:
        head, current, tail = split(readme)
        want = render(args.root)
    except CatalogError as e:
        print(f"catalog: error: {e}", file=sys.stderr)
        return 1
    if args.write:
        if current != want:
            readme.write_text(head + want + tail, encoding="utf-8")
            print(f"catalog: rewrote the skill catalog in {readme}")
        return 0
    if current == want:
        print("ok catalog")
        return 0
    print(f"FAIL catalog {readme}: the skill catalog is stale. "
          "Run python3 tools/catalog.py --write. The drift (- README, + generated):")
    diff = difflib.unified_diff(current.splitlines(), want.splitlines(), "README.md", "generated",
                                lineterm="", n=0)
    print("\n".join(line for line in list(diff)[2:] if not line.startswith("@@")))
    return 1


if __name__ == "__main__":
    sys.exit(main())
