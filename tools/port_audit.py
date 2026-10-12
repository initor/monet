#!/usr/bin/env python3
"""Account for every file that the bootstrap staged under import/ (DOC-10).

Usage: python3 tools/port_audit.py [--repo DIR] [--snapshot GIT_DIR COMMIT]

The bootstrap commit is the first commit that added a file under import/. The
audit follows each file in that commit's import/ tree through the commits
after it, with git's rename detection (50% similarity, no rename limit). Each
commit takes its state from its own parents, so a rename on one line of
history never moves a file that lives on another (see follow). It prints one
line per file:

  <path>: moved -> <dest>    a rename chain ends at <dest>, tracked at HEAD,
                             outside import/. A deleted import/<p> with no
                             rename (a heavy rewrite) is moved too, when <p>
                             is tracked at HEAD, after RENAMED_DIRS maps it.
                             A chain that a rename moved and a commit then
                             deleted gets no such credit.
  <path>: folded -> <dest>   deleted, and FOLDED below names the tracked file
                             that holds its content now.
  <path>: staging            no snapshot counterpart: with --snapshot, the
                             snapshot does not hold the path; without it, the
                             path matches none of the snapshot pathspecs.
                             Never a failure.
  <path>: unaccounted: <why> still tracked under import/ (also when a port
                             moved it and a later commit added it back, or a
                             rename kept it under import/), under a left-out
                             entry, deleted with no destination, or a folded
                             file whose destination is not tracked.

It then prints `import/<entry>: left out` for each entry of the 'Left out of
`import/`' bullet in specs/design.md, a backticked path after the bold label
(a dir when it ends in `/`, else that one file). A file added under import/
after the bootstrap is unaccounted too.

--snapshot GIT_DIR COMMIT also reads the snapshot's monet files with one
`git -C GIT_DIR ls-tree -r -z --name-only COMMIT -- <pathspecs>`, the
bootstrap's pathspecs: .claude/ plus each CLAUDE_PATHS entry, one
.claude/skills/<d>/ per name on the MONET_SKILL_DIRS= line of
scripts/migrate-from-dotfiles.sh, and bin/ plus each BIN_SCRIPTS entry. It
maps `.claude/<p>` to `import/<p>` and `bin/<f>` to `import/bin/<f>`, and
prints `snapshot monet files N, staged S, left out L`. The snapshot set must
equal the staged set plus the left-out files, with no overlap, and each
left-out entry must match a snapshot file. That read is the only access to
GIT_DIR.

The summary lines come last, and the very last line is `audit passed` or
`audit failed: <n> unaccounted`. Exit 0 when the audit passes, 1 when a file
is unaccounted, and 2 on a setup error (no bootstrap commit, or one at a
shallow clone's boundary; no left-out bullet, or one with no backticked
path; no MONET_SKILL_DIRS line, or one with no name; an unreadable or
non-UTF-8 input; a git error).
Stdlib only.
"""

import argparse
import os
import re
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
STAGING = "import/"
DESIGN = "specs/design.md"
MIGRATE = "scripts/migrate-from-dotfiles.sh"

# The bootstrap's snapshot pathspecs, besides one .claude/skills/<d>/ per
# MONET_SKILL_DIRS name. A trailing / marks a dir.
CLAUDE_PATHS = ("agents/monet.md", "commands/monet/", "output-styles/ste-lite.md",
                "templates/skill-template.md", "workflows/monet/", "docs/specs/")
BIN_SCRIPTS = ("check-monet-skill.py", "test-check-monet-skill.py", "test-gang.sh",
               "test-paint-explainer.py", "test-ste-check.py")

# A skill dir that the port renamed, for the rewrite rule above.
RENAMED_DIRS = {"skills/git-ship/": "skills/ship/"}

# Each staged file that a port deleted, because its content now lives in
# another file: the 17 command wrappers (P2), the help workflow (CORE-3), and
# the bin/ entry points that a port rewrote below the rename threshold (FND-4,
# FND-8). The skill template and the other bin/ scripts were renamed, so the
# rename chain finds them.
COMMANDS = ("distill-voice", "gang", "handoff", "help", "new-skill", "paint", "pr", "prep",
            "review-code", "review-local", "roadmap", "ship", "status", "teach",
            "visual-design", "voice", "web-operator")
FOLDED = {f"import/commands/monet/{name}.md": f"skills/{name}/SKILL.md" for name in COMMANDS}
FOLDED.update({
    "import/workflows/monet/help.md": "skills/help/SKILL.md",
    "import/bin/check-monet-skill.py": "tools/check_skills.py",
    "import/bin/test-check-monet-skill.py": "tools/tests/test_check_skills.py",
    "import/bin/test-gang.sh": "tools/test-gang.sh",
})

_LEFT_OUT = re.compile(r"^- \*\*Left out of `import/`\.\*\*(.*)$")
_SKILL_DIRS = re.compile(r'^MONET_SKILL_DIRS="([^"]*)"', re.M)
# A line that starts a new block at column 0: a list item, a heading, a quote,
# a fence, or a thematic break. An indented line never matches.
_NEW_BLOCK = re.compile(r"[-*+]\s|\d+[.)]\s|#|>|```|~~~|([-*_]\s*){3,}$")


class SetupError(Exception):
    """An input the audit needs is missing or unreadable: exit 2."""


def git(repo, *args, ok=(0,)):
    """git's stdout. GIT_* variables that a git hook may export are dropped."""
    env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}
    run = subprocess.run(["git", "-C", str(repo), *args], capture_output=True, text=True, env=env)
    if run.returncode not in ok:
        raise SetupError(f"git {' '.join(args[:2])} failed: {run.stderr.strip()}")
    return run.stdout


def names(out):
    return [p for p in out.split("\0") if p]


def left_out_entries(design):
    """The backticked paths after the bold label of the 'Left out of' bullet."""
    lines = design.splitlines()
    for i, line in enumerate(lines):
        m = _LEFT_OUT.match(line)
        if not m:
            continue
        text = [m.group(1)]
        for more in lines[i + 1:]:
            # A blank line, or an unindented line that starts a new block,
            # ends the bullet. An indented line (a wrapped line or a nested
            # item) continues it, and so does an unindented lazy line.
            if not more.strip() or _NEW_BLOCK.match(more):
                break
            text.append(more.strip())
        entries = re.findall(r"`([^`]+)`", " ".join(text))
        if entries:
            return entries
    raise SetupError(f"{DESIGN} has no 'Left out of `import/`' bullet with backticked paths")


def skill_dirs(script):
    m = _SKILL_DIRS.search(script)
    if not m or not m.group(1).split():
        raise SetupError(f"{MIGRATE} has no MONET_SKILL_DIRS= line with a skill name")
    return m.group(1).split()


def under(path, entry):
    """True when path is entry, or sits under it when entry ends in /."""
    return path.startswith(entry) if entry.endswith("/") else path == entry


def snapshot_paths(dirs):
    """The snapshot pathspecs, each paired with its import/ path."""
    pairs = [(".claude/" + p, STAGING + p) for p in CLAUDE_PATHS]
    pairs += [(f".claude/skills/{d}/", f"{STAGING}skills/{d}/") for d in dirs]
    pairs += [("bin/" + f, f"{STAGING}bin/{f}") for f in BIN_SCRIPTS]
    return pairs


def to_staging(path):
    """A snapshot path in import/ terms: .claude/<p> is import/<p>, bin/<f> is import/bin/<f>."""
    return STAGING + (path[len(".claude/"):] if path.startswith(".claude/") else path)


def renamed(rel):
    for old, new in RENAMED_DIRS.items():
        if rel.startswith(old):
            return new + rel[len(old):]
    return rel


def bootstrap_commit(repo, head):
    # --no-show-signature: log.showSignature=true would print gpg lines first.
    out = git(repo, "log", "--no-show-signature", "--full-history", "--no-merges", "--topo-order",
              "--reverse", "--diff-filter=A", "--format=%H", head, "--", STAGING)
    if not out.split():
        raise SetupError("no commit adds a file under import/ (a shallow clone has no bootstrap)")
    bootstrap = out.split()[0]
    # A shallow clone's boundary commit seems to add every file it holds.
    if (git(repo, "rev-parse", "--is-shallow-repository").strip() == "true"
            and len(git(repo, "rev-list", "--parents", "-n1", bootstrap).split()) == 1):
        raise SetupError(f"{bootstrap} is a shallow clone boundary, not surely the bootstrap: "
                         "fetch the full history")
    return bootstrap


def moves_in(repo, commit):
    """{old path: new path, or None for a deletion} for a non-merge commit."""
    tokens = names(git(repo, "diff-tree", "-r", "-z", "-M50%", "-l0", "--name-status",
                       "--no-commit-id", commit))
    moves, i = {}, 0
    while i < len(tokens):
        status = tokens[i]
        if status[0] in "RC":
            if status[0] == "R":
                moves[tokens[i + 1]] = tokens[i + 2]
            i += 3
            continue
        if status[0] == "D":
            moves[tokens[i + 1]] = None
        i += 2
    return moves


def follow(repo, bootstrap, head, staged):
    """Map each staged path to (last, alive) at head: last is the chain's last
    path, and alive is False when a commit deleted the file at last.

    Each commit after the bootstrap takes its state from its own parents. A
    commit whose parent the bootstrap is not an ancestor of holds no staged
    file (None). A non-merge commit applies its renames and deletions to its
    parent's state. A merge takes, per staged file, the parent that changed
    it since their merge base, or the first parent when both did. As in
    git, an octopus merges its parents in turn, and several best merge bases
    (a criss-cross) are merged into one first. A merge base that holds no
    staged file adds nothing to that merge. A path the merge resolution
    itself drops counts through the HEAD check in classify.
    """
    start = {p: (p, True) for p in staged}
    absent = dict.fromkeys(staged)
    states = {bootstrap: start}

    def merge(state, heads, other):
        """Merge other's state into state, the state of heads merged."""
        bases = git(repo, "merge-base", "--all", other, *heads, ok=(0, 1)).split()
        held = [b for b in bases if b in states]
        base = dict(states[held[0]]) if held else absent
        for i in range(1, len(held)):
            merge(base, held[:i], held[i])
        theirs = states.get(other, absent)
        for p in staged:
            if state[p] == base[p]:
                state[p] = theirs[p]

    for line in git(repo, "rev-list", "--reverse", "--topo-order", "--parents",
                    f"{bootstrap}..{head}").splitlines():
        commit, *parents = line.split()
        state = dict(states.get(parents[0], absent)) if parents else dict(absent)
        for i in range(1, len(parents)):
            merge(state, parents[:i], parents[i])
        if len(parents) == 1:
            at = {}  # a live chain's path -> the staged paths there
            for p, end in state.items():
                if end and end[1]:
                    at.setdefault(end[0], []).append(p)
            for old, new in moves_in(repo, commit).items():
                for p in at.get(old, ()):
                    state[p] = (new, True) if new is not None else (old, False)
        states[commit] = state
    return states[head]


def classify(path, last, alive, head):
    """(status, dest, problem) for a staged snapshot file whose rename chain
    ends at last, alive there or deleted there (see follow).

    The staged path itself at HEAD fails first, whatever the chain did: a port
    moved or deleted it, and a later commit added it back. The rewrite rule
    holds only for a chain that never left the staged path, so a file that a
    commit adds at <p> after a renamed chain was deleted gets no credit.
    """
    if path in head:
        return "unaccounted", None, "still tracked under import/"
    if alive and last in head:
        if last.startswith(STAGING):
            return "unaccounted", None, f"still tracked under import/ as {last}"
        return "moved", last, None
    if path in FOLDED:
        dest = FOLDED[path]
        if dest in head:
            return "folded", dest, None
        return "unaccounted", None, f"folded into {dest}, which is not tracked at HEAD"
    if last != path:
        return "unaccounted", None, f"renamed to {last}, then gone, with no folded entry"
    dest = renamed(path[len(STAGING):])
    if dest in head:
        return "moved", dest, None
    return "unaccounted", None, f"deleted with no rename, no folded entry, and no {dest} at HEAD"


def read_snapshot(git_dir, commit, pairs):
    """The snapshot's monet files, as import/ paths, from one git ls-tree read."""
    if commit.startswith("-"):
        raise SetupError(f"--snapshot COMMIT must name a commit, not an option: {commit}")
    listed = names(git(os.path.expanduser(git_dir), "ls-tree", "-r", "-z", "--name-only", commit,
                       "--", *(spec for spec, _ in pairs)))
    return {to_staging(p) for p in listed}


def audit(repo, snapshot=None):
    """Print the audit and return the number of unaccounted findings."""
    repo = Path(repo)
    try:
        design = (repo / DESIGN).read_text(encoding="utf-8")
        script = (repo / MIGRATE).read_text(encoding="utf-8")
    except OSError as e:
        raise SetupError(f"cannot read {e.filename}: {e.strerror}") from e
    except UnicodeDecodeError as e:
        raise SetupError(f"{DESIGN} or {MIGRATE} is not UTF-8: {e}") from e
    left_out = [STAGING + entry for entry in left_out_entries(design)]
    pairs = snapshot_paths(skill_dirs(script))
    monet = read_snapshot(*snapshot, pairs) if snapshot else None
    # One HEAD SHA for every git read, so a commit that lands meanwhile changes
    # none of them. specs/design.md and the migrate script come from the
    # working tree.
    head_sha = git(repo, "rev-parse", "--verify", "HEAD^{commit}").strip()
    bootstrap = bootstrap_commit(repo, head_sha)
    tree = sorted(names(git(repo, "ls-tree", "-r", "-z", "--name-only", bootstrap, "--", STAGING)))
    head = set(names(git(repo, "ls-tree", "-r", "-z", "--name-only", head_sha)))
    if monet is None:
        staged = {p for p in tree if any(under(p, imp) for _, imp in pairs)}
    else:
        staged = monet.intersection(tree)
    ends = follow(repo, bootstrap, head_sha, sorted(staged))

    counts = {"moved": 0, "folded": 0, "staging": 0, "unaccounted": 0}

    def unaccounted(path, why):
        counts["unaccounted"] += 1
        print(f"{path}: unaccounted: {why}")

    for path in tree:
        if path not in staged:
            counts["staging"] += 1
            print(f"{path}: staging")
            continue
        if any(under(path, entry) for entry in left_out):
            unaccounted(path, f"staged, but {DESIGN} leaves it out")
            continue
        status, dest, problem = classify(path, *ends[path], head)
        if problem:
            unaccounted(path, problem)
        else:
            counts[status] += 1
            print(f"{path}: {status} -> {dest}")
    for entry in left_out:
        if monet is not None and not any(under(p, entry) for p in monet):
            unaccounted(entry, "left out, but no snapshot file matches it")
        else:
            print(f"{entry}: left out")
    for path in sorted(head.difference(tree)):
        if path.startswith(STAGING):
            unaccounted(path, "added under import/ after the bootstrap")

    if monet is not None:
        dropped = {p for p in monet if any(under(p, entry) for entry in left_out)}
        for path in sorted(monet - dropped - staged):
            unaccounted(path, "in the snapshot, but neither staged nor left out")

    source = "in the snapshot" if monet is not None else "match the snapshot pathspecs"
    print(f"bootstrap {bootstrap}: {len(tree)} files under import/, {len(staged)} {source}")
    print(f"moved {counts['moved']}, folded {counts['folded']}, staging {counts['staging']}, "
          f"unaccounted {counts['unaccounted']}")
    print(f"left out of import/: {len(left_out)} entries in {DESIGN}")
    if monet is not None:
        print(f"snapshot monet files {len(monet)}, staged {len(staged)}, left out {len(dropped)}")
    print(f"audit failed: {counts['unaccounted']} unaccounted" if counts["unaccounted"] else "audit passed")
    return counts["unaccounted"]


def main(argv=None):
    parser = argparse.ArgumentParser(description="Account for every file the bootstrap staged under import/.")
    parser.add_argument("--repo", default=str(ROOT), help="the repo to audit (default: this repo)")
    parser.add_argument("--snapshot", nargs=2, metavar=("GIT_DIR", "COMMIT"),
                        help="also check the staged set against this dotfiles snapshot")
    args = parser.parse_args(argv)
    try:
        return 1 if audit(args.repo, args.snapshot) else 0
    except SetupError as e:
        print(f"port_audit.py: {e}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
