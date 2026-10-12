#!/usr/bin/env python3
"""Check that one monet skill is wired on every surface the README's "Authoring a skill" names.

    python3 bin/check-monet-skill.py <name> [--no-command]

Checks, from the repo root this file lives in:
  skill     .claude/skills/<name>/SKILL.md, with frontmatter naming <name> and a description
  command   .claude/commands/monet/<name>.md, or else the command whose execution_context points
            at skills/<name>/SKILL.md (skip with --no-command, for a routing-only skill)
  routing   a routing-table row and a dispatch line in .claude/agents/monet.md
  readme    the generated command table (for a command) and the shape table in .claude/README.md,
            both under the command's name
  portable  no machine paths in the skill or its command

Prints one line per check and exits 1 if any fails.
"""
import re
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent
AGENT = ".claude/agents/monet.md"
MACHINE = re.compile(r"/Users/[A-Za-z0-9_.-]+")


def frontmatter(text):
    m = re.match(r"---\n(.*?)\n---\n", text, re.S)
    if not m:
        return None
    try:
        import yaml
        data = yaml.safe_load(m.group(1))
        return data if isinstance(data, dict) else None
    except ImportError:
        return dict(re.findall(r"^(\w[\w-]*):\s*(.*)$", m.group(1), re.M))
    except Exception:
        return None


def wraps(path, name):
    """True when a command file's execution_context points at skills/<name>/SKILL.md."""
    m = re.search(r"<execution_context>(.*?)</execution_context>", path.read_text(), re.S)
    return bool(m) and f"skills/{name}/SKILL.md" in m.group(1)


def main(argv):
    args = [a for a in argv if not a.startswith("--")]
    if len(args) != 1 or not re.fullmatch(r"[a-z][a-z0-9]*(-[a-z0-9]+)*", args[0]):
        sys.exit(__doc__)
    name, command = args[0], "--no-command" not in argv
    tick = f"`{name}`"
    results = []

    def check(label, ok, detail):
        results.append(ok)
        print(f"{'ok  ' if ok else 'FAIL'}  {label:9} {detail}")

    skill = REPO / ".claude/skills" / name / "SKILL.md"
    meta = frontmatter(skill.read_text()) if skill.is_file() else None
    check("skill", bool(meta) and meta.get("name") == name and bool(meta.get("description")),
          f"{skill.relative_to(REPO)} with name {name} and a description")

    wrappers = []
    cmd = name
    if command:
        claude = REPO / ".claude/commands/monet" / f"{name}.md"
        if not claude.is_file():
            # A command can wrap a skill of another name, as /monet:ship wraps git-ship.
            claude = next((p for p in sorted(claude.parent.glob("*.md")) if wraps(p, name)), claude)
            cmd = claude.stem
        text = claude.read_text() if claude.is_file() else ""
        check("command", f"name: monet:{cmd}" in text and f"skills/{name}/SKILL.md" in text,
              f"{claude.relative_to(REPO)} names monet:{cmd} and points at skills/{name}/SKILL.md")
        wrappers.append(claude)

    lines = (REPO / AGENT).read_text().splitlines()
    row = any(line.startswith("|") and tick in line for line in lines)
    step = any(not line.startswith("|") and tick in line for line in lines)
    check("routing", row and step, f"{AGENT} has a routing-table row and a dispatch line naming {tick}")

    readme = (REPO / ".claude/README.md").read_text()
    block = re.search(r"<!-- BEGIN GENERATED:commands -->(.*?)<!-- END GENERATED:commands -->", readme, re.S)
    if command:
        check("readme", bool(block) and f"`/monet:{cmd}`" in block.group(1),
              f"the README's generated command table has /monet:{cmd}")
    shape = [line for line in readme.splitlines() if line.startswith("| Command **and** routing row")
             or line.startswith("| Routing row only")]
    check("readme", any(f"`{cmd}`" in line for line in shape), f"the README's shape table lists `{cmd}`")

    files = [skill] + wrappers
    hits = [f"{f.relative_to(REPO)}:{i}" for f in files if f.is_file()
            for i, line in enumerate(f.read_text().splitlines(), 1) if MACHINE.search(line)]
    check("portable", not hits, "no machine paths" + (f" (found at {', '.join(hits)})" if hits else ""))

    return 0 if all(results) else 1


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
