#!/usr/bin/env python3
"""Lint the plugin's skills, from the repo root this file lives in (FND-4).

    python3 tools/check_skills.py [<name> ...]

With no names, it checks every skills/*/SKILL.md. With names, it checks only
those skills, and the repo-wide checks still run. Each check prints an `ok`,
`FAIL`, or `skip` line:

  skill        skills/<name>/SKILL.md exists (a named skill only)
  frontmatter  the frontmatter parses with skillkit.read_skill
  name         `name` equals the skill's dir name
  keys         every frontmatter key is one Claude Code reads for a skill
  description  `description` is non-empty, starts with "Use when", and is at
               most 500 characters
  paths        each ${CLAUDE_SKILL_DIR}/<p> in a file under skills/<name>/
               resolves against skills/<name>/, and each
               ${CLAUDE_PLUGIN_ROOT}/<p> in a file under skills/, agents/, or
               output-styles/ resolves against the repo root, to a file that
               skillkit.files() lists or a dir that holds one. A path with a
               placeholder segment (holding <, {, or *) is checked up to the
               segment before it.
  routing      each monet:<name> in agents/monet.md names a skill, an agent,
               or an output style
  commands     no commands/ dir holds a file, since commands fold into skills

Every skill gets every check: there is no exemption list (ruling R4). With no
skills/ dir and no names, it prints a skip line and exits 0. It exits 1 when
any check fails, 2 on a bad argument, and 0 otherwise.
"""

import os
import posixpath
import re
import sys
from pathlib import Path

sys.dont_write_bytecode = True
import skillkit  # noqa: E402  (tools/ is on sys.path as this script's dir)

REPO = Path(__file__).resolve().parent.parent
USAGE = "usage: python3 tools/check_skills.py [<name> ...]"
NAME = re.compile(r"[a-z][a-z0-9]*(-[a-z0-9]+)*")
KEYS = ("name", "description", "argument-hint", "allowed-tools", "model",
        "disable-model-invocation", "user-invocable")
MAX_DESCRIPTION = 500
AGENT = "agents/monet.md"
ROUTE = re.compile(r"(?<![\w-])monet:([a-z][a-z0-9]*(?:-[a-z0-9]+)*)")
PLACEHOLDER = re.compile(r"[<{*]")
REPO_WIDE = ("agents/", "output-styles/")

failed = False


def report(status, label, detail):
    global failed
    failed = failed or status == "FAIL"
    print(f"{status:4}  {label:11}  {detail}")


def check(ok, label, detail):
    report("ok" if ok else "FAIL", label, detail)


def check_frontmatter(name):
    rel = f"skills/{name}/SKILL.md"
    path = REPO / rel
    try:
        skill = skillkit.read_skill(path)
    except (ValueError, OSError) as e:
        check(False, "frontmatter", str(e).replace(str(path), rel))
        return
    check(True, "frontmatter", f"{rel} parses")
    meta = skill.frontmatter
    check(meta.get("name") == name and bool(NAME.fullmatch(name)), "name",
          f"{rel}: name is {meta.get('name')!r}, and the dir name is {name!r}")
    unknown = sorted(set(meta) - set(KEYS))
    check(not unknown, "keys", f"{rel}: " + (f"unknown keys {', '.join(unknown)}; the known keys are "
                                              if unknown else "every key is one of ") + ", ".join(KEYS))
    desc = meta.get("description")
    desc = desc.strip() if isinstance(desc, str) else ""
    check(desc.startswith("Use when") and len(desc) <= MAX_DESCRIPTION, "description",
          f"{rel}: description is {len(desc)} chars (at most {MAX_DESCRIPTION}), and "
          + ("starts" if desc.startswith("Use when") else "must start") + ' with "Use when"')


def resolve(var, ref, rel, tracked, dirs):
    """Return the repo path that a ref names, checked up to a placeholder, or None when it leaves the repo."""
    parts = ref.split("/")
    cut = next((i for i, part in enumerate(parts) if PLACEHOLDER.search(part)), len(parts))
    base = "/".join(rel.split("/")[:2]) if var == "CLAUDE_SKILL_DIR" else "."
    target = posixpath.normpath(posixpath.join(base, *parts[:cut]))
    if target == ".." or target.startswith("../"):
        return None
    return target if target == "." or target in tracked or target in dirs else None


def check_paths(prefixes, tracked):
    """Check path refs in each tracked file under one of `prefixes`, one ok line per prefix."""
    dirs = {posixpath.dirname(f) for f in tracked}
    dirs |= {d for f in list(dirs) for d in _parents(f)}
    for prefix in prefixes:
        count, bad = 0, []
        for rel in (f for f in tracked if f.startswith(prefix)):
            try:
                text = (REPO / rel).read_text(encoding="utf-8")
            except (OSError, UnicodeDecodeError):
                continue
            for var, ref, line in skillkit.path_refs(text):
                if var == "CLAUDE_SKILL_DIR" and not (rel.startswith("skills/") and rel.count("/") >= 2):
                    continue
                count += 1
                if resolve(var, ref, rel, tracked, dirs) is None:
                    bad.append(f"{rel}:{line}: ${{{var}}}/{ref} resolves to no tracked file or dir")
        for detail in bad:
            check(False, "paths", detail)
        if not bad:
            check(True, "paths", f"{prefix}: {count} path refs resolve")


def _parents(path):
    while path:
        path = posixpath.dirname(path)
        yield path


def check_routing():
    agent = REPO / AGENT
    if not agent.is_file():
        report("skip", "routing", f"{AGENT} is absent")
        return
    bad, count = [], 0
    for n, line in enumerate(agent.read_text(encoding="utf-8").splitlines(), 1):
        for m in ROUTE.finditer(line):
            count += 1
            name = m.group(1)
            targets = (f"skills/{name}/SKILL.md", f"agents/{name}.md", f"output-styles/{name}.md")
            if not any((REPO / t).is_file() for t in targets):
                bad.append(f"{AGENT}:{n}: monet:{name} names no skill, agent, or output style")
    for detail in bad:
        check(False, "routing", detail)
    if not bad:
        check(True, "routing", f"{AGENT}: {count} monet:<name> refs resolve")


def check_commands():
    held = sorted(os.path.relpath(os.path.join(d, f), REPO)
                  for d, _, names in os.walk(REPO / "commands") for f in names)
    more = f" and {len(held) - 1} more files" if len(held) > 1 else ""
    check(not held, "commands", f"commands/ holds {held[0]}{more}; commands fold into skills "
          "(design.md decision 2)" if held else "no file under commands/")


def main(argv):
    if any(not NAME.fullmatch(arg) for arg in argv):
        print(USAGE, file=sys.stderr)
        return 2
    if not argv and not (REPO / "skills").is_dir():
        report("skip", "skills", "no skills/ dir, so there is nothing to lint")
        return 0
    names = list(dict.fromkeys(argv)) or [p.parent.name for p in skillkit.skills(REPO)]
    for name in names:
        rel = f"skills/{name}/SKILL.md"
        if argv:
            check((REPO / rel).is_file(), "skill", f"{rel} " + ("exists" if (REPO / rel).is_file()
                                                              else "does not exist"))
        if (REPO / rel).is_file():
            check_frontmatter(name)
    tracked = set(skillkit.files(REPO))
    check_paths([f"skills/{name}/" for name in argv] or ["skills/"], tracked)
    check_paths(REPO_WIDE, tracked)
    check_routing()
    check_commands()
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
