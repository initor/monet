"""Stdlib helpers for monet's skill contract tests.

Contract tests use this module with `import skillkit`, on the
`PYTHONPATH=$(CURDIR)/tools` that `make check` sets. It imports only the
Python standard library. Every `line` below is 1-based in the text passed in.

REPO -> Path
    The repo root (the parent of `tools/`), absolute.
skills(repo=REPO) -> list[Path]
    The sorted absolute paths of `skills/*/SKILL.md`, as the shell glob
    lists them: no dot dirs, and `SKILL.md` in that exact case.
read_skill(path) -> Skill
    A named tuple of `.name` (the skill dir name), `.path`, `.frontmatter` (a
    dict), and `.body` (the text after the closing `---` line). Raises
    ValueError naming the file when it has no `---` frontmatter block at the
    top, or when the block does not parse.
frontmatter(text) -> dict
    The frontmatter of a full file text that starts with a `---` line. Reads
    plain, single-quoted, and double-quoted scalars, `true`, `false`, and
    `null` (or an empty value), block scalars `|`, `|-`, `>`, and `>-` (a
    folded scalar joins its lines with spaces), block sequences of scalars,
    and flow sequences such as `[a, b]`. Nested maps, anchors, aliases, tags,
    multi-line plain or quoted scalars, numbers, dates, and any other YAML
    raise ValueError("line <n>: ...").
path_refs(text) -> list[tuple[str, str, int]]
    `(var, path, line)` for each `${CLAUDE_SKILL_DIR}/<path>` and
    `${CLAUDE_PLUGIN_ROOT}/<path>`, allowed-tools entries included. `var` is
    the bare name, such as `CLAUDE_SKILL_DIR`. The path stops at the first
    character outside `[A-Za-z0-9._/<>{}*-]`, trailing dots and slashes are
    trimmed, and a ref with an empty path (the dir itself) is left out.
DOTFILES_PATTERNS -> tuple[re.Pattern, ...]
    The dotfiles paths that the portability scan rejects: the `.claude` dir
    under `~`, `$HOME`, or `${HOME}`, the one under `~/.config`, and
    `/Users/<name>/` and `/home/<name>/`.
dotfiles_paths(text) -> list[tuple[int, str]]
    `(line, matched text)` for each match of DOTFILES_PATTERNS, in text order.
files(repo=REPO) -> list[str]
    The sorted POSIX paths, relative to `repo`, that `git ls-files --cached
    --others --exclude-standard` lists, minus paths under `import/`, paths
    deleted from the worktree, and dirs (a nested repo or a submodule). A new
    file not yet added is included, and so is a symlink, dangling or not. Git
    runs without the caller's GIT_* variables, so a git hook cannot redirect it.
USER_CONFIG_FALLBACKS -> dict[str, str]
    Each userConfig key mapped to its fallback sentence, verbatim from
    ruling R1.
missing_fallbacks(text) -> list[tuple[int, str]]
    `(line, key)` for each `${user_config.<key>}` use whose key has no
    fallback sentence, or that is not followed by that sentence before the
    next blank line, or whose following sentence holds a full placeholder
    where the sentence writes only `${user_config`. Line breaks and
    indentation inside the sentence are ignored.
skill_md_refs(text, own) -> list[tuple[int, str]]
    `(line, ref)` for each path that ends in `<name>/SKILL.md`, where
    `<name>` is a skill name other than `own` (ruling R8). A path stops at
    whitespace, a backtick, and any of `'"()[],;|=<>`, so paths joined by one of
    these are reported one by one. A bare `SKILL.md`,
    `${CLAUDE_SKILL_DIR}/SKILL.md`, a `<name>` that holds a placeholder, and
    other files under a skill are not reported.
"""

import json
import os
import re
import subprocess
from pathlib import Path
from typing import NamedTuple

REPO = Path(__file__).resolve().parent.parent

DOTFILES_PATTERNS = tuple(re.compile(p) for p in (
    r"~/\.claude/",
    r"\$HOME/\.claude",
    r"\$\{HOME\}/\.claude",
    r"~/\.config/\.claude",
    r"/Users/[A-Za-z0-9_.-]+/",
    r"/home/[A-Za-z0-9_.-]+/",
))

USER_CONFIG_FALLBACKS = {
    "data_dir": "If the data dir above is empty or starts with `${user_config`, "
                "it isn't configured, so use `~/.config/monet`.",
    "branch_prefix": "If the prefix above is empty or starts with `${user_config`, "
                     "it isn't configured, so use no prefix.",
}

_KEY = re.compile(r"([A-Za-z_][A-Za-z0-9_-]*):(?:[ \t](.*))?$")
_ITEM = re.compile(r"( *)-(?:[ \t](.*))?$")
_QUOTED = re.compile(r"""('(?:[^']|'')*'|"(?:[^"\\]|\\.)*")(?:[ \t]+#.*)?[ \t]*$""")
_FLOW_END = re.compile(r"\s*\]")
_FLOW_ITEM = re.compile(r"""\s*('(?:[^']|'')*'|"(?:[^"\\]|\\.)*"|[^\s,'"\[\]{}#][^,\[\]{}]*?)\s*([,\]])""")
# Plain scalars that YAML reads as a number, an infinity or NaN, or a date.
_TYPED = re.compile(r"[-+]?([0-9][0-9_]*(\.[0-9_]*)?|\.[0-9]+)([eE][-+]?[0-9]+)?|0x[0-9a-fA-F_]+"
                    r"|0o[0-7_]+|[-+]?\.(inf|Inf|INF)|\.(nan|NaN|NAN)|[0-9]{4}-[0-9]{1,2}-[0-9]{1,2}([Tt ].*)?")
_PATH_REF = re.compile(r"\$\{(CLAUDE_SKILL_DIR|CLAUDE_PLUGIN_ROOT)\}/([A-Za-z0-9._/<>{}*-]*)")
_USE = re.compile(r"\$\{user_config\.([^}\s]*)\}")
_BLANK_LINE = re.compile(r"\n[ \t\r]*\n")
_SKILL_NAME = re.compile(r"[a-z][a-z0-9]*(-[a-z0-9]+)*")
# A path to `<name>/SKILL.md`. The prefix is lazy, so a token that joins two such
# paths yields both. A path stops at `<` and `>` (an autolink or a tag), and it
# must end at SKILL.md, not go on with `/`, a word char, or an extension.
_SKILL_MD = re.compile(r"""(?:[^\s`'"()\[\],;|=<>]*?/)?([^/\s`'"()\[\],;|=<>]+)/SKILL\.md(?![\w/-]|\.\w)""")


class Skill(NamedTuple):
    name: str
    path: Path
    frontmatter: dict
    body: str


def skills(repo=REPO):
    # Like the shell glob `skills/*/SKILL.md`: no dot dirs, and the exact case of SKILL.md
    # even on a case-insensitive file system.
    root = Path(repo).resolve() / "skills"
    dirs = [d for d in root.iterdir() if not d.name.startswith(".") and d.is_dir()] if root.is_dir() else []
    return sorted(d / "SKILL.md" for d in dirs if "SKILL.md" in os.listdir(d))


def read_skill(path):
    path = Path(path)
    try:
        text = path.read_text(encoding="utf-8")
        header, body = _split(text)
        data = _parse(header)
    except ValueError as e:
        raise ValueError(f"{path}: {e}") from None
    return Skill(path.absolute().parent.name, path, data, body)


def frontmatter(text):
    return _parse(_split(text)[0])


def _split(text):
    """Return the frontmatter's (line number, line) pairs and the body after it."""
    lines = text.split("\n")
    if lines[0].rstrip() != "---":
        raise ValueError("line 1: no --- frontmatter block at the top")
    for i in range(1, len(lines)):
        if lines[i].rstrip() == "---":
            return [(n, s.rstrip("\r")) for n, s in enumerate(lines[1:i], 2)], "\n".join(lines[i + 1:])
    raise ValueError(f"line {len(lines)}: the frontmatter has no closing --- line")


def _parse(header):
    data, i = {}, 0
    while i < len(header):
        n, line = header[i]
        i += 1
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        if line[0] in " \t":
            raise ValueError(f"line {n}: unexpected indented line; nested maps and multi-line "
                             "scalars are not supported")
        m = _KEY.match(line)
        if not m:
            raise ValueError(f"line {n}: expected `key: value`; only a flat map is supported")
        key, value = m.group(1), (m.group(2) or "").strip()
        if key in data:
            raise ValueError(f"line {n}: duplicate key {key!r}")
        if value[:1] in ("|", ">"):
            data[key], i = _block(header, i, re.sub(r"[ \t]+#.*", "", value), n)
        elif not value or value.startswith("#"):
            data[key], i = _sequence(header, i)
        elif value.startswith("["):
            data[key] = _flow(value, n)
        else:
            data[key] = _scalar(value, n)
    return data


def _block(header, i, head, n):
    if head not in ("|", "|-", ">", ">-"):
        raise ValueError(f"line {n}: block scalar header {head!r} is not supported; "
                         "use |, |-, >, or >-")
    lines, indent = [], None
    while i < len(header):
        n, line = header[i]
        if line.strip():
            width = len(line) - len(line.lstrip(" "))
            indent = width if indent is None else indent
            if width == 0 or width < indent:
                break
            line = line[indent:]
        else:
            # A blank-looking line keeps the whitespace past the indent, as in YAML. Its indent
            # (the whole line, before the first text line sets the indent) holds only spaces.
            if line[:indent].strip(" "):
                raise ValueError(f"line {n}: a block scalar's indent holds a tab or another non-space "
                                 "char, which is not YAML; indent with spaces")
            line = line[indent:] if indent is not None else ""
        if head[0] == ">" and line[:1] in (" ", "\t"):
            raise ValueError(f"line {n}: more-indented lines in a folded scalar are not supported")
        lines.append(line)
        i += 1
    text = "\n".join(lines).rstrip("\n")
    if head[0] == ">":
        text = re.sub(r"(?<=[^\n])\n+(?=[^\n])", lambda m: m.group()[1:] or " ", text)
    return text + ("\n" if text and not head.endswith("-") else ""), i


def _sequence(header, i):
    """Read a block sequence of scalars, or return None when no `- ` item follows."""
    items, indent = None, None
    while i < len(header):
        n, line = header[i]
        if line.strip() and not line.lstrip().startswith("#"):
            m = _ITEM.match(line)
            if not m or (items is not None and len(m.group(1)) != indent):
                break
            if items is None:
                items, indent = [], len(m.group(1))
            items.append(_scalar((m.group(2) or "").strip(), n))
        i += 1
    return items, i


def _flow(value, n):
    items, pos = [], 1
    while True:
        end = _FLOW_END.match(value, pos)
        if end:
            pos = end.end()
            break
        m = _FLOW_ITEM.match(value, pos)
        if not m:
            raise ValueError(f"line {n}: cannot read {value!r}; a flow sequence holds scalars "
                             "and closes on its line")
        # ` #` starts a comment only in a plain item; a quoted item keeps it as text.
        if m.group(1)[0] not in "'\"" and re.search(r"\s#", m.group(1)):
            raise ValueError(f"line {n}: a comment inside a flow sequence is not supported")
        items.append(_scalar(m.group(1), n))
        pos = m.end()
        if m.group(2) == "]":
            break
    if not re.fullmatch(r"\s*(#.*)?", value[pos:]):
        raise ValueError(f"line {n}: unexpected text after the flow sequence {value!r}")
    return items


def _scalar(s, n):
    if s[:1] in ("'", '"'):
        m = _QUOTED.match(s)
        if not m:
            raise ValueError(f"line {n}: cannot read {s!r}; a quoted scalar closes on its line")
        q = m.group(1)
        if q[0] == "'":
            return q[1:-1].replace("''", "'")
        try:
            return json.loads(q, strict=False)
        except ValueError:
            raise ValueError(f"line {n}: unsupported escape in {q}") from None
    s = re.sub(r"(?:^|[ \t]+)#.*", "", s)
    if s in ("", "~", "null", "Null", "NULL"):
        return None
    if s in ("true", "True", "TRUE", "false", "False", "FALSE"):
        return s[0] in "tT"
    if s[0] in "&*!%@`|>[]{},#" or re.match(r"[-?:](\s|$)", s) or re.search(r":(\s|$)", s):
        raise ValueError(f"line {n}: {s!r} is not a plain scalar; nested maps and collections, "
                         "anchors, aliases, and tags are not supported")
    if _TYPED.fullmatch(s):
        raise ValueError(f"line {n}: YAML reads {s!r} as a number or a date; quote it")
    return s


def _line(text, pos):
    return text.count("\n", 0, pos) + 1


def path_refs(text):
    refs = [(m.group(1), m.group(2).rstrip("./"), _line(text, m.start())) for m in _PATH_REF.finditer(text)]
    return [ref for ref in refs if ref[1]]


def dotfiles_paths(text):
    hits = sorted((m.start(), m.group()) for p in DOTFILES_PATTERNS for m in p.finditer(text))
    return [(_line(text, start), match) for start, match in hits]


def _is_file(repo, p):
    full = os.path.join(repo, p.rstrip("/"))
    return os.path.islink(full) or os.path.isfile(full)


def files(repo=REPO):
    env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}
    out = subprocess.run(
        ["git", "-C", str(repo), "ls-files", "-z", "--cached", "--others", "--exclude-standard"],
        check=True, capture_output=True, encoding="utf-8", env=env).stdout
    return sorted({p for p in out.split("\0") if p and not p.startswith("import/") and _is_file(repo, p)})


def _fallback_shape(sentence):
    """Match the sentence, with a slot that takes `${user_config` or a full placeholder."""
    head, tail = sentence.split("`${user_config`")
    return re.compile(re.escape(head) + r"`\$\{user_config(\.[^}\s]*\})?`" + re.escape(tail))


_FALLBACK_SHAPES = {key: _fallback_shape(s) for key, s in USER_CONFIG_FALLBACKS.items()}


def missing_fallbacks(text):
    missing = []
    for m in _USE.finditer(text):
        key = m.group(1)
        blank = _BLANK_LINE.search(text, m.end())
        rest = " ".join(text[m.end():blank.start() if blank else len(text)].split())
        found = _FALLBACK_SHAPES[key].search(rest) if key in _FALLBACK_SHAPES else None
        if found is None or found.group(1):
            missing.append((_line(text, m.start()), key))
    return missing


def skill_md_refs(text, own):
    return [(_line(text, m.start()), m.group()) for m in _SKILL_MD.finditer(text)
            if _SKILL_NAME.fullmatch(m.group(1)) and m.group(1) != own]
