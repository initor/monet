"""Contract for tools/port_audit.py, the audit that accounts for every file
the bootstrap staged under import/ (DOC-10), and for the repo state after
import/ is gone.

Each PortAudit test builds a temp repo: a base commit with a synthetic
specs/design.md and scripts/migrate-from-dotfiles.sh, a bootstrap commit that
stages files under import/, then port commits. The snapshot test builds a temp
snapshot repo too. RepoState reads this repo.
"""

import hashlib
import os
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "tools"))
import port_audit  # noqa: E402

TOOL = ROOT / "tools" / "port_audit.py"

# Synthetic left-out entries, unlike the real ones, so a run that prints these
# shows that the tool reads design.md and keeps no copy of its list.
DESIGN = """# design

## Source

- **Staging.** A bootstrap PR copies the monet files into `import/`.
- **Left out of `import/`.** Three files: `skills/voice/reference/personas/private.md`,
  and the two scripts in `skills/voice/scripts/`. A task writes new content.
- **History.** Only files come over, never `skills/voice/history.md`.
"""
LEFT_OUT = ["import/skills/voice/reference/personas/private.md: left out",
            "import/skills/voice/scripts/: left out"]
MIGRATE = '#!/usr/bin/env bash\nMONET_SKILL_DIRS="git-ship voice"\nMONET_OTHER_ENTRIES="agents/monet.md"\n'
IDENTITY = ("-c", "user.name=Port Audit", "-c", "user.email=audit@example.com", "-c", "commit.gpgsign=false")


def git(repo, *args, date=None):
    """Run git in repo, without GIT_* variables that a git hook may export, and
    without the global and system config (hooks, signing). Fixtures add with -f,
    since git reads the default global ignore file, which may list .claude/.
    date sets the commit dates."""
    env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}
    env.update(GIT_CONFIG_GLOBAL=os.devnull, GIT_CONFIG_NOSYSTEM="1")
    if date:
        env.update(GIT_AUTHOR_DATE=date, GIT_COMMITTER_DATE=date)
    return subprocess.run(["git", "-C", str(repo), *args], check=True, capture_output=True,
                          text=True, env=env).stdout


def body(tag, n=10):
    """n lines of 48 chars, unique to tag, so no two tags look alike to rename detection."""
    return "".join(f"{tag[:6]:<6} {i} {hashlib.sha256(f'{tag}/{i}'.encode()).hexdigest()[:39]}\n"
                   for i in range(n))


def edit(text, lines, tag):
    """text with the given line numbers replaced by lines of the same length."""
    out = text.splitlines(keepends=True)
    for i in lines:
        out[i] = f"{tag[:6]:<6} {i} {hashlib.sha256(f'{tag}/edit/{i}'.encode()).hexdigest()[:39]}\n"
    return "".join(out)


class Repo:
    """A temp repo with the base commit made, ready for a bootstrap."""

    def __init__(self, path, design=DESIGN, migrate=MIGRATE):
        self.path = path
        path.mkdir(parents=True)
        git(path, "init", "-q")
        self.write("specs/design.md", design)
        self.write("scripts/migrate-from-dotfiles.sh", migrate)
        self.commit("base")
        self.bootstrap = None

    def write(self, rel, text):
        full = self.path / rel
        full.parent.mkdir(parents=True, exist_ok=True)
        full.write_text(text, encoding="utf-8")

    def link(self, rel, target):
        full = self.path / rel
        full.parent.mkdir(parents=True, exist_ok=True)
        os.symlink(target, full)

    def move(self, src, dst):
        (self.path / dst).parent.mkdir(parents=True, exist_ok=True)
        git(self.path, "mv", src, dst)

    def remove(self, rel):
        git(self.path, "rm", "-q", "-r", rel)

    def commit(self, message, date=None):
        git(self.path, "add", "-A", "-f")
        git(self.path, *IDENTITY, "commit", "-q", "--no-verify", "-m", message, date=date)
        return git(self.path, "rev-parse", "HEAD").strip()

    def stage(self, files):
        """The bootstrap commit: write each import/ path in files, then commit."""
        for rel, text in files.items():
            self.write(rel, text)
        self.bootstrap = self.commit("stage the snapshot")

    def trunk(self):
        return git(self.path, "rev-parse", "--abbrev-ref", "HEAD").strip()

    def merge(self, branch):
        """Start a --no-ff merge of branch with no commit. A conflict (exit 1)
        is left for the caller to resolve before commit()."""
        subprocess.run(["git", "-C", str(self.path), *IDENTITY, "merge", "-q", "--no-ff", "--no-commit",
                        branch], capture_output=True, check=False,
                       env={**{k: v for k, v in os.environ.items() if not k.startswith("GIT_")},
                            "GIT_CONFIG_GLOBAL": os.devnull, "GIT_CONFIG_NOSYSTEM": "1"})

    def pull(self, *branches):
        """A clean --no-ff merge of branches, an octopus when there are several."""
        git(self.path, *IDENTITY, "merge", "-q", "--no-ff", "--no-edit", "--allow-unrelated-histories",
            *branches)

    def audit(self, *args, extra_env=None):
        """Run the tool on this repo. extra_env adds variables after the GIT_*
        filter, so a test can export what a git hook would."""
        env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}
        env.update(extra_env or {})
        run = subprocess.run([sys.executable, str(TOOL), "--repo", str(self.path), *args],
                             capture_output=True, text=True, check=False, timeout=120, env=env)
        return run.returncode, run.stdout.splitlines(), run.stderr


class PortAudit(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.tmp = Path(tmp.name).resolve()
        self.repo = Repo(self.tmp / "repo")

    def assertPassed(self, code, lines, err):
        self.assertEqual(code, 0, "\n".join(lines) + err)
        self.assertEqual(lines[-1], "audit passed")
        self.assertFalse([line for line in lines if ": unaccounted" in line])

    def assertUnaccounted(self, code, lines, path):
        self.assertEqual(code, 1, "\n".join(lines))
        self.assertTrue([line for line in lines if line.startswith(path + ": unaccounted: ")],
                        "\n".join(lines))
        self.assertTrue(lines[-1].startswith("audit failed: "), lines[-1])

    def test_follows_rename_chain(self):
        notes = body("notes")
        self.repo.stage({"import/skills/git-ship/SKILL.md": body("ship"),
                         "import/skills/voice/notes.md": notes})
        self.repo.move("import/skills/git-ship", "skills/git-ship")
        self.repo.commit("port git-ship")
        self.repo.move("skills/git-ship", "skills/ship")
        self.repo.commit("rename git-ship to ship")
        # Each rename edits 3 of 10 lines, so each step stays a rename, while the
        # two ends share only 4 lines: one diff from the bootstrap sees no rename.
        self.repo.move("import/skills/voice/notes.md", "skills/voice/draft.md")
        self.repo.write("skills/voice/draft.md", edit(notes, (0, 1, 2), "first"))
        self.repo.commit("port the notes")
        self.repo.move("skills/voice/draft.md", "skills/voice/reference/notes.md")
        self.repo.write("skills/voice/reference/notes.md",
                        edit(edit(notes, (0, 1, 2), "first"), (3, 4, 5), "second"))
        self.repo.commit("move the notes")
        whole = git(self.repo.path, "diff", "-M", "--name-status", self.repo.bootstrap, "HEAD")
        self.assertIn("D\timport/skills/voice/notes.md", whole)

        code, lines, err = self.repo.audit()
        self.assertPassed(code, lines, err)
        self.assertIn("import/skills/git-ship/SKILL.md: moved -> skills/ship/SKILL.md", lines)
        self.assertIn("import/skills/voice/notes.md: moved -> skills/voice/reference/notes.md", lines)

    def test_rename_threshold_is_fifty_percent(self):
        # 11 of 20 lines kept (55%) is a rename to a new path; 9 of 20 (45%) is
        # a deletion, and the new path is not import/<p>, so it is unaccounted.
        kept, redone = body("kept", 20), body("redone", 20)
        self.repo.stage({"import/skills/voice/kept.md": kept, "import/skills/voice/redone.md": redone})
        self.repo.remove("import")
        self.repo.write("skills/voice/kept-port.md", edit(kept, range(9), "port"))
        self.repo.write("skills/voice/redone-port.md", edit(redone, range(11), "port"))
        self.repo.commit("port with heavy edits")
        scores = git(self.repo.path, "diff-tree", "-r", "-M40%", "--name-status", "--no-commit-id", "HEAD")
        self.assertIn("R055\timport/skills/voice/kept.md\tskills/voice/kept-port.md", scores)
        self.assertIn("R045\timport/skills/voice/redone.md\tskills/voice/redone-port.md", scores)

        code, lines, _ = self.repo.audit()
        self.assertUnaccounted(code, lines, "import/skills/voice/redone.md")
        self.assertIn("import/skills/voice/kept.md: moved -> skills/voice/kept-port.md", lines)

    def test_rename_limit_covers_a_port_of_five_files(self):
        # One commit ports 5 files to new basenames with 2 of 10 lines edited
        # (R080), as the voice and paint port commits did. That is 5 by 5
        # inexact candidates, over a rename limit of 4, so the audit must lift
        # the limit, also over a repo's own diff.renameLimit.
        texts = {f"import/skills/voice/part{i}.md": body(f"part{i}") for i in range(5)}
        self.repo.stage(texts)
        self.repo.remove("import")
        for i in range(5):
            self.repo.write(f"skills/voice/reference/piece-{i}.md",
                            edit(texts[f"import/skills/voice/part{i}.md"], (0, 1), "port"))
        self.repo.commit("port five files with small edits")
        diff = ["diff-tree", "-r", "-M50%", "--name-status", "--no-commit-id", "HEAD"]
        self.assertEqual(git(self.repo.path, *diff, "-l0").count("R080\t"), 5)
        self.assertNotIn("R0", git(self.repo.path, *diff, "-l4"))

        for limit in (None, "1"):
            with self.subTest(diff_rename_limit=limit):
                if limit:
                    git(self.repo.path, "config", "diff.renameLimit", limit)
                code, lines, err = self.repo.audit()
                self.assertPassed(code, lines, err)
                for i in range(5):
                    self.assertIn(f"import/skills/voice/part{i}.md: moved -> "
                                  f"skills/voice/reference/piece-{i}.md", lines)

    def test_ported_file_edited_in_place_stays_moved(self):
        self.repo.stage({"import/skills/voice/notes.md": body("notes")})
        self.repo.move("import/skills/voice/notes.md", "skills/voice/draft.md")
        self.repo.commit("port the notes")
        self.repo.write("skills/voice/draft.md", body("rewritten"))
        self.repo.commit("rewrite every line in place")
        self.assertIn("M\tskills/voice/draft.md",
                      git(self.repo.path, "diff-tree", "-r", "-M50%", "--name-status", "--no-commit-id", "HEAD"))

        code, lines, err = self.repo.audit()
        self.assertPassed(code, lines, err)
        self.assertIn("import/skills/voice/notes.md: moved -> skills/voice/draft.md", lines)

    def test_ports_merged_from_a_side_branch(self):
        # A merge takes each file from the parent that moved it, and HEAD
        # decides whether the destination survived the merge.
        def merged(name, resolve=None, after=None):
            repo = Repo(self.tmp / name)
            repo.stage({"import/skills/voice/a.md": body("a"), "import/skills/voice/b.md": body("b")})
            trunk = repo.trunk()
            git(repo.path, "checkout", "-q", "-b", "side")
            repo.move("import/skills/voice/a.md", "skills/voice/x.md")
            repo.commit("port a on the side branch")
            git(repo.path, "checkout", "-q", trunk)
            repo.move("import/skills/voice/b.md", "skills/voice/y.md" if resolve else "skills/voice/x.md")
            repo.commit("port b on the trunk")
            # The two-renames case conflicts, and the test resolves it.
            repo.merge("side")
            (resolve or (lambda r: r.write("skills/voice/x.md", body("a") + body("b"))))(repo)
            for stray in (repo.path / "skills" / "voice").glob("x.md~*"):
                stray.unlink()
            repo.commit("merge side")
            self.assertEqual(len(git(repo.path, "rev-list", "--merges", "HEAD").split()), 1)
            if after:
                after(repo)
            return repo.audit()

        with self.subTest("clean"):
            code, lines, err = merged("clean", resolve=lambda r: None)
            self.assertPassed(code, lines, err)
            self.assertIn("import/skills/voice/a.md: moved -> skills/voice/x.md", lines)
        with self.subTest("merge drops the ported file"):
            code, lines, _ = merged("dropped", resolve=lambda r: (r.path / "skills/voice/x.md").unlink())
            self.assertUnaccounted(code, lines, "import/skills/voice/a.md")
            self.assertIn("import/skills/voice/a.md: unaccounted: renamed to skills/voice/x.md, then gone, "
                          "with no folded entry", lines)
        with self.subTest("two branches rename onto one path"):
            code, lines, err = merged("onto_one")
            self.assertPassed(code, lines, err)
            self.assertIn("import/skills/voice/a.md: moved -> skills/voice/x.md", lines)
            self.assertIn("import/skills/voice/b.md: moved -> skills/voice/x.md", lines)
        with self.subTest("two branches rename onto one path, then a rename moves both"):
            code, lines, err = merged("onto_one_renamed", after=lambda r: (
                r.move("skills/voice/x.md", "skills/voice/z.md"), r.commit("rename x.md")))
            self.assertPassed(code, lines, err)
            self.assertIn("import/skills/voice/a.md: moved -> skills/voice/z.md", lines)
            self.assertIn("import/skills/voice/b.md: moved -> skills/voice/z.md", lines)
        with self.subTest("two branches rename onto one path, then a commit deletes it"):
            # The deletion ends both chains, so a new file at the path later
            # carries neither of them.
            code, lines, _ = merged("onto_one_deleted", after=lambda r: (
                r.remove("skills/voice/x.md"), r.commit("drop x.md"),
                r.write("skills/voice/x.md", body("unrelated")), r.commit("a new x.md")))
            self.assertUnaccounted(code, lines, "import/skills/voice/a.md")
            for staged in ("import/skills/voice/a.md", "import/skills/voice/b.md"):
                self.assertIn(f"{staged}: unaccounted: renamed to skills/voice/x.md, then gone, "
                              "with no folded entry", lines)

    def test_rename_on_a_parallel_branch_does_not_move_a_chain(self):
        # An unrelated file at the port's destination, renamed away on a
        # parallel branch, must not carry the staged file with it, whichever
        # branch the port is on. The port is then deleted, so the audit fails.
        x, u = "skills/voice/x.md", "skills/voice/u.md"
        for name, port_on_side in (("port on the trunk", False), ("port on the side", True)):
            with self.subTest(name):
                repo = Repo(self.tmp / name.replace(" ", "_"))
                repo.stage({"import/skills/voice/a.md": body("a")})

                def port():
                    repo.move("import/skills/voice/a.md", x)
                    repo.commit("port a")

                def unrelated():
                    repo.write(x, body("unrelated"))
                    repo.commit("add an unrelated x.md")
                    repo.move(x, u)
                    repo.commit("rename it to u.md")

                trunk = repo.trunk()
                git(repo.path, "checkout", "-q", "-b", "side")
                (port if port_on_side else unrelated)()
                git(repo.path, "checkout", "-q", trunk)
                (unrelated if port_on_side else port)()
                repo.merge("side")
                repo.commit("merge side")
                repo.remove(x)
                repo.commit("drop x.md")

                code, lines, _ = repo.audit()
                self.assertUnaccounted(code, lines, "import/skills/voice/a.md")
                self.assertIn(f"import/skills/voice/a.md: unaccounted: renamed to {x}, then gone, "
                              "with no folded entry", lines)

    def test_merges_a_branch_without_the_bootstrap(self):
        # A branch forked before the bootstrap, and an unrelated history with
        # no merge base, hold no staged file. Merging either keeps the port of
        # a.md, and keeps c.md (not ported yet) for its port after the merge.
        def forked(repo):
            git(repo.path, "checkout", "-q", "-b", "side", "HEAD~2")

        def orphan(repo):
            git(repo.path, "checkout", "-q", "--orphan", "side")
            git(repo.path, "rm", "-q", "-r", "--cached", ".")
            for rel in ("import", "specs", "scripts", "skills"):
                shutil.rmtree(repo.path / rel, ignore_errors=True)

        for name, branch in (("forked before the bootstrap", forked), ("unrelated history", orphan)):
            with self.subTest(name):
                repo = Repo(self.tmp / name.replace(" ", "_"))
                repo.stage({"import/skills/voice/a.md": body("a"), "import/skills/voice/c.md": body("c")})
                repo.move("import/skills/voice/a.md", "skills/voice/a.md")
                repo.commit("port a")
                trunk = repo.trunk()
                branch(repo)
                repo.write("notes/side.md", body("side"))
                repo.commit("a side commit")
                repo.write("notes/side.md", body("side, again"))
                repo.commit("another side commit")
                git(repo.path, "checkout", "-q", trunk)
                repo.pull("side")
                repo.move("import/skills/voice/c.md", "skills/voice/c-port.md")
                repo.commit("port c")

                code, lines, err = repo.audit()
                self.assertPassed(code, lines, err)
                self.assertIn("import/skills/voice/a.md: moved -> skills/voice/a.md", lines)
                self.assertIn("import/skills/voice/c.md: moved -> skills/voice/c-port.md", lines)

    def test_merges_whose_first_parent_lacks_the_bootstrap(self):
        # The first parent, or a parent merged before, holds no staged file,
        # so the merge takes the port from the trunk. Each port goes to a path
        # other than skills/voice/a.md, so the rewrite rule cannot hide a
        # wrong chain.
        def pull_into_old_branch(repo):
            # A branch forked before the bootstrap pulls the ported trunk.
            trunk = repo.trunk()
            repo.move("import/skills/voice/a.md", "skills/voice/ported.md")
            repo.commit("port a")
            git(repo.path, "checkout", "-q", "-b", "side", repo.bootstrap + "~1")
            repo.write("notes/side.md", body("side"))
            repo.commit("a side commit")
            repo.pull(trunk)

        def pull_into_orphan(repo):
            trunk = repo.trunk()
            repo.move("import/skills/voice/a.md", "skills/voice/ported.md")
            repo.commit("port a")
            git(repo.path, "checkout", "-q", "--orphan", "side")
            git(repo.path, "rm", "-q", "-r", "--cached", ".")
            for rel in ("import", "specs", "scripts", "skills"):
                shutil.rmtree(repo.path / rel, ignore_errors=True)
            repo.write("notes/side.md", body("side"))
            repo.commit("an orphan commit")
            repo.pull(trunk)

        def pull_before_and_after(repo):
            # The side branch pulls the trunk once before the bootstrap (a
            # parent outside the range) and once after the port.
            trunk = repo.trunk()
            repo.move("import/skills/voice/a.md", "skills/voice/ported.md")
            repo.commit("port a")
            git(repo.path, "checkout", "-q", "-b", "side", repo.bootstrap + "~2")
            repo.write("notes/side.md", body("side"))
            repo.commit("a side commit")
            repo.pull(repo.bootstrap + "~1")
            repo.pull(trunk)

        def bootstrap_through_a_merge(repo):
            # The bootstrap lands on a branch, and the trunk, which moved on
            # without it, merges it before the port.
            trunk = repo.trunk()
            git(repo.path, "checkout", "-q", "-b", "staging", repo.bootstrap)
            git(repo.path, "checkout", "-q", trunk)
            git(repo.path, "reset", "-q", "--hard", repo.bootstrap + "~1")
            repo.write("notes/trunk.md", body("trunk"))
            repo.commit("the trunk moves on")
            repo.pull("staging")
            repo.move("import/skills/voice/a.md", "skills/voice/ported.md")
            repo.commit("port a")

        for history in (pull_into_old_branch, pull_into_orphan, pull_before_and_after,
                        bootstrap_through_a_merge):
            with self.subTest(history.__name__):
                repo = Repo(self.tmp / history.__name__)
                repo.write("notes/before.md", body("before"))
                repo.commit("a commit before the bootstrap")
                repo.stage({"import/skills/voice/a.md": body("a")})
                history(repo)
                self.assertTrue(git(repo.path, "rev-list", "--merges", "HEAD").split())

                code, lines, err = repo.audit()
                self.assertPassed(code, lines, err)
                self.assertIn("import/skills/voice/a.md: moved -> skills/voice/ported.md", lines)

    def test_octopus_merge_takes_the_port_from_any_parent(self):
        # The port is on the third parent. In the second octopus, b1 and b2
        # fork from a commit c that ports a.md to x.md, and b2 then moves it:
        # the base of b2 and the trunk merged with b1 is c, not the bootstrap.
        for name, shared in (("port on the third parent", False), ("parents share the port", True)):
            with self.subTest(name):
                repo = Repo(self.tmp / name.replace(" ", "_"))
                repo.stage({"import/skills/voice/a.md": body("a")})
                trunk = repo.trunk()
                git(repo.path, "checkout", "-q", "-b", "c")
                if shared:
                    repo.move("import/skills/voice/a.md", "skills/voice/x.md")
                    repo.commit("port a")
                git(repo.path, "checkout", "-q", "-b", "b1")
                repo.write("notes/b1.md", body("b1"))
                repo.commit("an unrelated commit")
                git(repo.path, "checkout", "-q", "-b", "b2", "c")
                repo.move("skills/voice/x.md" if shared else "import/skills/voice/a.md", "skills/voice/ported.md")
                repo.commit("move a")
                git(repo.path, "checkout", "-q", trunk)
                repo.write("notes/trunk.md", body("trunk"))
                repo.commit("a trunk commit")
                repo.pull("b1", "b2")
                self.assertEqual(len(git(repo.path, "rev-list", "--parents", "-n1", "HEAD").split()), 4)

                code, lines, err = repo.audit()
                self.assertPassed(code, lines, err)
                self.assertIn("import/skills/voice/a.md: moved -> skills/voice/ported.md", lines)

    def test_criss_cross_merge_uses_every_merge_base(self):
        # Lines l1 and l2 each hold the port and a merge of x1, a commit
        # forked before the bootstrap, so both are best merge bases of l1 and
        # l2. Plain merge-base picks x1 (the later date), which holds no
        # staged file. l2 deletes the port and adds an unrelated x.md, and
        # merging l2 into l1 keeps that: the chain is gone.
        x = "skills/voice/x.md"
        self.repo.stage({"import/skills/voice/a.md": body("a")})
        self.repo.move("import/skills/voice/a.md", x)
        port = self.repo.commit("port a")
        git(self.repo.path, "checkout", "-q", "-b", "x1", self.repo.bootstrap + "~1")
        self.repo.write("notes/x1.md", body("x1"))
        x1 = self.repo.commit("x1", date="2090-01-01 00:00:00 +0000")
        for line in ("l1", "l2"):
            git(self.repo.path, "checkout", "-q", "-b", line, port)
            git(self.repo.path, *IDENTITY, "merge", "-q", "--no-ff", "-m", f"merge x1 into {line}", "x1")
        self.repo.remove(x)
        self.repo.commit("drop x.md")
        self.repo.write(x, body("unrelated"))
        self.repo.commit("a new x.md")
        self.assertEqual(sorted(git(self.repo.path, "merge-base", "--all", "l1", "l2").split()), sorted([x1, port]))
        self.assertEqual(git(self.repo.path, "merge-base", "l1", "l2").strip(), x1)
        git(self.repo.path, "checkout", "-q", "l1")
        self.repo.pull("l2")

        code, lines, _ = self.repo.audit()
        self.assertUnaccounted(code, lines, "import/skills/voice/a.md")
        self.assertIn(f"import/skills/voice/a.md: unaccounted: renamed to {x}, then gone, "
                      "with no folded entry", lines)

    def test_conflicting_renames_keep_the_first_parent(self):
        # Both branches rename one staged file, each to its own path. The merge
        # takes the first parent's path, here the one that the resolution kept.
        self.repo.stage({"import/skills/voice/a.md": body("a")})
        trunk = self.repo.trunk()
        git(self.repo.path, "checkout", "-q", "-b", "side")
        self.repo.move("import/skills/voice/a.md", "skills/voice/side.md")
        self.repo.commit("port a on the side branch")
        git(self.repo.path, "checkout", "-q", trunk)
        self.repo.move("import/skills/voice/a.md", "skills/voice/trunk.md")
        self.repo.commit("port a on the trunk")
        self.repo.merge("side")
        (self.repo.path / "skills" / "voice" / "side.md").unlink(missing_ok=True)
        self.repo.write("skills/voice/trunk.md", body("a"))
        self.repo.commit("merge side, keep the trunk path")

        code, lines, err = self.repo.audit()
        self.assertPassed(code, lines, err)
        self.assertIn("import/skills/voice/a.md: moved -> skills/voice/trunk.md", lines)

    @unittest.skipUnless(shutil.which("ssh-keygen"), "needs ssh-keygen to sign the bootstrap")
    def test_signed_bootstrap_with_show_signature(self):
        # With log.showSignature=true, git log prints a signature line before
        # the SHA of a signed commit. The bootstrap search must not read it.
        key = self.tmp / "key"
        subprocess.run(["ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "audit", "-f", str(key)],
                       check=True, capture_output=True)
        self.repo.write("import/agents/monet.md", body("agent"))
        git(self.repo.path, "add", "-A", "-f")
        git(self.repo.path, *IDENTITY, "-c", "gpg.format=ssh", "-c", f"user.signingkey={key}",
            "commit", "-q", "--no-verify", "-S", "-m", "stage the snapshot")
        self.repo.move("import/agents/monet.md", "agents/monet.md")
        self.repo.commit("port the agent")
        git(self.repo.path, "config", "log.showSignature", "true")
        shown = git(self.repo.path, "log", "--format=%H", "-1", "HEAD~1")
        self.assertNotRegex(shown.splitlines()[0], r"^[0-9a-f]{40}$")

        code, lines, err = self.repo.audit()
        self.assertPassed(code, lines, err)
        self.assertIn("import/agents/monet.md: moved -> agents/monet.md", lines)

    def test_rewrite_without_rename_counts_as_moved(self):
        self.repo.stage({"import/skills/voice/SKILL.md": body("voice old"),
                         "import/skills/git-ship/SKILL.md": body("ship old")})
        self.repo.remove("import")
        self.repo.write("skills/voice/SKILL.md", body("voice new"))
        self.repo.write("skills/ship/SKILL.md", body("ship new"))
        self.repo.commit("rewrite voice and ship")
        show = git(self.repo.path, "show", "-M", "--name-status", "--format=", "HEAD")
        self.assertIn("D\timport/skills/voice/SKILL.md", show)
        self.assertIn("D\timport/skills/git-ship/SKILL.md", show)

        code, lines, err = self.repo.audit()
        self.assertPassed(code, lines, err)
        self.assertIn("import/skills/voice/SKILL.md: moved -> skills/voice/SKILL.md", lines)
        self.assertIn("import/skills/git-ship/SKILL.md: moved -> skills/ship/SKILL.md", lines)

    def test_flags_file_left_in_import(self):
        self.repo.stage({"import/agents/monet.md": body("agent"),
                         "import/skills/voice/SKILL.md": body("voice")})
        self.repo.move("import/skills/voice", "skills/voice")
        self.repo.commit("port voice")

        code, lines, _ = self.repo.audit()
        self.assertUnaccounted(code, lines, "import/agents/monet.md")
        self.assertIn("import/skills/voice/SKILL.md: moved -> skills/voice/SKILL.md", lines)

    def test_flags_staged_path_added_back_under_import(self):
        # A port rewrote or moved the file, and a later commit added the staged
        # path back: it is tracked under import/, whatever the chain did.
        for name, port in (("rewritten", lambda r: (r.remove("import"),
                                                     r.write("skills/voice/SKILL.md", body("new")))),
                           ("moved", lambda r: r.move("import/skills/voice", "skills/voice"))):
            with self.subTest(name):
                repo = Repo(self.tmp / name)
                repo.stage({"import/skills/voice/SKILL.md": body("voice")})
                port(repo)
                repo.commit("port voice")
                repo.write("import/skills/voice/SKILL.md", body("back"))
                repo.commit("add the staged path back")

                code, lines, _ = repo.audit()
                self.assertUnaccounted(code, lines, "import/skills/voice/SKILL.md")
                self.assertEqual([line for line in lines if line.startswith("import/skills/voice/SKILL.md:")],
                                 ["import/skills/voice/SKILL.md: unaccounted: still tracked under import/"])

    def test_deleted_chain_does_not_follow_a_new_file_at_its_path(self):
        # The port moved the file, a later commit deleted it, and a new file
        # took its path: the chain ended at the deletion, so neither the new
        # file nor a later rename of it carries the chain on.
        self.repo.stage({"import/skills/voice/notes.md": body("notes")})
        self.repo.move("import/skills/voice/notes.md", "skills/voice/draft.md")
        self.repo.commit("port the notes")
        self.repo.remove("skills/voice/draft.md")
        self.repo.commit("drop the notes")
        self.repo.write("skills/voice/draft.md", body("unrelated"))
        self.repo.commit("a new draft")

        code, lines, _ = self.repo.audit()
        self.assertUnaccounted(code, lines, "import/skills/voice/notes.md")
        self.assertNotIn("import/skills/voice/notes.md: moved -> skills/voice/draft.md", lines)

        self.repo.move("skills/voice/draft.md", "skills/voice/final.md")
        self.repo.commit("rename the new draft")
        code, lines, _ = self.repo.audit()
        self.assertIn("import/skills/voice/notes.md: unaccounted: renamed to skills/voice/draft.md, "
                      "then gone, with no folded entry", lines)

    def test_moved_to_own_path_then_deleted_is_not_a_rewrite(self):
        # The port moved import/<p> to <p>, a later commit deleted it, and a new
        # file took <p>: the rewrite rule covers only a deletion at import/<p>.
        self.repo.stage({"import/skills/voice/notes.md": body("notes")})
        self.repo.move("import/skills/voice/notes.md", "skills/voice/notes.md")
        self.repo.commit("port the notes")
        self.repo.remove("skills/voice/notes.md")
        self.repo.commit("drop the notes")
        self.repo.write("skills/voice/notes.md", body("unrelated"))
        self.repo.commit("new notes")

        code, lines, _ = self.repo.audit()
        self.assertUnaccounted(code, lines, "import/skills/voice/notes.md")
        self.assertNotIn("import/skills/voice/notes.md: moved -> skills/voice/notes.md", lines)

    def test_flags_rename_onto_a_staging_only_path(self):
        # A rename inside import/ onto a bootstrap path keeps a snapshot file
        # under import/, though no file is added there after the bootstrap.
        self.repo.stage({"import/agents/monet.md": body("agent"), "import/README.md": body("readme")})
        self.repo.remove("import/README.md")
        self.repo.commit("drop the readme")
        self.repo.move("import/agents/monet.md", "import/README.md")
        self.repo.commit("rename the agent inside import/")

        code, lines, _ = self.repo.audit()
        self.assertUnaccounted(code, lines, "import/agents/monet.md")
        self.assertNotIn("import/agents/monet.md: moved -> import/README.md", lines)

    def test_default_mode_audits_bin_entry_points(self):
        # Without --snapshot, a snapshot bin/<f> is staged as import/bin/<f>.
        self.assertEqual(port_audit.FOLDED["import/bin/test-gang.sh"], "tools/test-gang.sh")
        self.repo.stage({"import/bin/test-gang.sh": body("gang"),
                         "import/bin/test-ste-check.py": body("ste"),
                         "import/bin/other.sh": body("other")})
        self.repo.remove("import/bin/test-gang.sh")
        self.repo.write("tools/test-gang.sh", body("gang rewritten"))
        self.repo.move("import/bin/test-ste-check.py", "skills/voice/tests/test_ste_check.py")
        self.repo.commit("port the bin entry points")

        code, lines, err = self.repo.audit()
        self.assertPassed(code, lines, err)
        self.assertIn("import/bin/test-gang.sh: folded -> tools/test-gang.sh", lines)
        self.assertIn("import/bin/test-ste-check.py: moved -> skills/voice/tests/test_ste_check.py", lines)
        self.assertIn("import/bin/other.sh: staging", lines)
        self.assertIn("moved 1, folded 1, staging 1, unaccounted 0", lines)

    def test_flags_unlisted_deletion(self):
        self.repo.stage({"import/skills/voice/SKILL.md": body("voice"),
                         "import/skills/voice/extra.md": body("extra"),
                         "import/commands/monet/voice.md": body("command")})
        self.repo.move("import/skills/voice/SKILL.md", "skills/voice/SKILL.md")
        self.repo.remove("import")
        self.repo.commit("port voice, fold its command, drop extra.md")

        code, lines, _ = self.repo.audit()
        self.assertUnaccounted(code, lines, "import/skills/voice/extra.md")
        self.assertIn("import/commands/monet/voice.md: folded -> skills/voice/SKILL.md", lines)
        self.assertNotIn("import/commands/monet/voice.md: unaccounted", "\n".join(lines))

    def test_folded_file_renamed_before_it_went(self):
        # A port renamed the voice command, then a commit deleted it (linear),
        # or a merge resolution dropped it (merge). It is in the folded table,
        # so it is folded wherever its chain ended.
        def deleted(repo):
            repo.move("import/commands/monet/voice.md", "skills/voice/command.md")
            repo.commit("rename the command")
            repo.remove("skills/voice/command.md")
            repo.commit("drop the command")

        def dropped_by_a_merge(repo):
            trunk = repo.trunk()
            git(repo.path, "checkout", "-q", "-b", "side")
            repo.move("import/commands/monet/voice.md", "skills/voice/command.md")
            repo.commit("rename the command")
            git(repo.path, "checkout", "-q", trunk)
            repo.write("notes/trunk.md", body("trunk"))
            repo.commit("a trunk commit")
            repo.merge("side")
            (repo.path / "skills" / "voice" / "command.md").unlink()
            repo.commit("merge side, drop the command")
            self.assertEqual(len(git(repo.path, "rev-list", "--parents", "-n1", "HEAD").split()), 3)

        for history in (deleted, dropped_by_a_merge):
            with self.subTest(history.__name__):
                repo = Repo(self.tmp / history.__name__)
                repo.stage({"import/commands/monet/voice.md": body("command"),
                            "import/skills/voice/SKILL.md": body("voice")})
                repo.move("import/skills/voice", "skills/voice")
                repo.commit("port voice")
                history(repo)
                self.assertNotIn("skills/voice/command.md", git(repo.path, "ls-files"))

                code, lines, err = repo.audit()
                self.assertPassed(code, lines, err)
                self.assertIn("import/commands/monet/voice.md: folded -> skills/voice/SKILL.md", lines)

    def test_folded_table(self):
        # Acceptance line 1 names what the table holds: the command wrappers,
        # the help workflow, and the bin/ entry points that a port rewrote.
        commands = ("distill-voice", "gang", "handoff", "help", "new-skill", "paint", "pr", "prep",
                    "review-code", "review-local", "roadmap", "ship", "status", "teach",
                    "visual-design", "voice", "web-operator")
        folded = {f"import/commands/monet/{name}.md": f"skills/{name}/SKILL.md" for name in commands}
        folded.update({"import/workflows/monet/help.md": "skills/help/SKILL.md",
                       "import/bin/check-monet-skill.py": "tools/check_skills.py",
                       "import/bin/test-check-monet-skill.py": "tools/tests/test_check_skills.py",
                       "import/bin/test-gang.sh": "tools/test-gang.sh"})
        self.assertEqual(len(folded), 21)
        self.assertEqual(port_audit.FOLDED, folded)

    def test_skill_dirs_line_is_a_shell_word_list(self):
        # The migrate script reads MONET_SKILL_DIRS as a shell word list, and
        # only the assignment at column 0 counts. Each variant names voice, so
        # a voice file left under import/ fails the audit.
        variants = {"tab": 'MONET_SKILL_DIRS="git-ship\tvoice"\n',
                    "lines": 'MONET_SKILL_DIRS="git-ship\n  voice\n"\n',
                    "comment first": '# Override: MONET_SKILL_DIRS="other"\nMONET_SKILL_DIRS="git-ship voice"\n'}
        for name, line in variants.items():
            with self.subTest(name):
                repo = Repo(self.tmp / name.replace(" ", "_"), migrate="#!/usr/bin/env bash\n" + line)
                repo.stage({"import/skills/voice/SKILL.md": body("voice")})

                code, lines, _ = repo.audit()
                self.assertUnaccounted(code, lines, "import/skills/voice/SKILL.md")
                self.assertIn("import/skills/voice/SKILL.md: unaccounted: still tracked under import/", lines)

    def test_flags_folded_file_without_destination(self):
        self.assertEqual(port_audit.FOLDED["import/commands/monet/voice.md"], "skills/voice/SKILL.md")
        self.repo.stage({"import/commands/monet/voice.md": body("command")})
        self.repo.remove("import")
        self.repo.commit("drop the command")

        code, lines, _ = self.repo.audit()
        self.assertUnaccounted(code, lines, "import/commands/monet/voice.md")

    def test_flags_file_added_under_import_after_bootstrap(self):
        self.repo.stage({"import/skills/voice/SKILL.md": body("voice")})
        self.repo.move("import/skills/voice", "skills/voice")
        self.repo.write("import/late.md", body("late"))
        self.repo.commit("port voice, add a late file")

        code, lines, _ = self.repo.audit()
        self.assertUnaccounted(code, lines, "import/late.md")

    def test_flags_staged_file_on_left_out_list(self):
        self.repo.stage({"import/skills/voice/scripts/a.py": body("script")})
        self.repo.move("import/skills/voice", "skills/voice")
        self.repo.commit("port voice")

        code, lines, _ = self.repo.audit()
        self.assertUnaccounted(code, lines, "import/skills/voice/scripts/a.py")
        # One line per file: the move is not listed as well.
        self.assertEqual(len([line for line in lines if line.startswith("import/skills/voice/scripts/a.py:")]), 1)

    def test_lists_staging_only_file_without_failing(self):
        # A name that extends a file pathspec (agents/monet.md) is not under it.
        self.repo.stage({"import/README.md": body("readme"),
                         "import/agents/monet.md": body("agent"),
                         "import/agents/monet.md.orig": body("orig")})
        self.repo.move("import/agents/monet.md", "agents/monet.md")
        self.repo.commit("port the agent")

        code, lines, err = self.repo.audit()
        self.assertPassed(code, lines, err)
        self.assertIn("import/README.md: staging", lines)
        self.assertIn("import/agents/monet.md.orig: staging", lines)
        self.assertIn("import/agents/monet.md: moved -> agents/monet.md", lines)
        self.assertIn("moved 1, folded 0, staging 2, unaccounted 0", lines)
        # No snapshot was read, so the summary does not claim one.
        self.assertIn(f"bootstrap {self.repo.bootstrap}: 3 files under import/, "
                      "1 match the snapshot pathspecs", lines)

    def test_parses_left_out_list_from_design(self):
        # A file entry (no trailing /) covers that one path, not a longer one,
        # and a dir entry covers only paths under the dir, not a sibling file.
        self.repo.stage({"import/skills/voice/SKILL.md": body("voice"),
                         "import/skills/voice/reference/personas/private.md.bak": body("bak"),
                         "import/skills/voice/scripts.md": body("sibling")})
        self.repo.move("import/skills/voice", "skills/voice")
        self.repo.commit("port voice")

        code, lines, err = self.repo.audit()
        self.assertPassed(code, lines, err)
        self.assertIn("import/skills/voice/reference/personas/private.md.bak: "
                      "moved -> skills/voice/reference/personas/private.md.bak", lines)
        self.assertIn("import/skills/voice/scripts.md: moved -> skills/voice/scripts.md", lines)
        self.assertEqual([line for line in lines if line.endswith(": left out")], LEFT_OUT)
        # The bullet holds a lazy line (no indent), and any indented line, a
        # nested item or a wrapped line that starts with a number among them.
        # A blank or whitespace-only line, or an unindented line that starts a
        # new block, ends it. A lazy line that starts with *** is no break.
        label = "- **Left out of `import/`.** One: `a.md`\n"
        for follow, entries in (("and `b/`.", ["a.md", "b/"]),
                                ("  - and `b/`", ["a.md", "b/"]),
                                ("  2026. Also `b/`", ["a.md", "b/"]),
                                ("- **Next.** `c.md`", ["a.md"]),
                                ("* `c.md`", ["a.md"]),
                                ("+ `c.md`", ["a.md"]),
                                ("1. `c.md`", ["a.md"]),
                                ("1) `c.md`", ["a.md"]),
                                ("## Next `c.md`", ["a.md"]),
                                ("> note `c.md`", ["a.md"]),
                                ("```\n`c.md`\n```", ["a.md"]),
                                ("~~~\n`c.md`\n~~~", ["a.md"]),
                                ("---\n`c.md`", ["a.md"]),
                                ("***\n`c.md`", ["a.md"]),
                                ("___\n`c.md`", ["a.md"]),
                                ("***Note:*** and `b/`", ["a.md", "b/"]),
                                ("\nA paragraph `c.md`", ["a.md"]),
                                ("   \nA paragraph `c.md`", ["a.md"])):
            with self.subTest(follow=follow):
                self.assertEqual(port_audit.left_out_entries(label + follow + "\n"), entries)
        self.assertIn("left out of import/: 2 entries in specs/design.md", lines)

        # The real design.md names 4 entries, 2 of them dirs, and a run on the
        # fixture above prints none of them.
        real = port_audit.left_out_entries((ROOT / "specs" / "design.md").read_text(encoding="utf-8"))
        self.assertEqual(len(real), 4, real)
        self.assertEqual(len([e for e in real if e.endswith("/")]), 2, real)
        for entry in real:
            self.assertFalse([line for line in lines if entry in line], entry)

    def test_missing_left_out_bullet_is_a_setup_error(self):
        repo = Repo(self.tmp / "nobullet", design=DESIGN.replace("Left out of", "Kept out of"))
        repo.stage({"import/agents/monet.md": body("agent")})

        code, _, err = repo.audit()
        self.assertEqual(code, 2)
        self.assertIn("Left out of", err)

    def test_setup_errors_exit_2(self):
        def no_bootstrap():
            return Repo(self.tmp / "none"), ()

        def no_skill_dirs():
            repo = Repo(self.tmp / "nodirs", migrate="#!/usr/bin/env bash\n")
            repo.stage({"import/agents/monet.md": body("agent")})
            return repo, ()

        def empty_skill_dirs():
            repo = Repo(self.tmp / "emptydirs", migrate='#!/usr/bin/env bash\nMONET_SKILL_DIRS=""\n')
            repo.stage({"import/skills/voice/SKILL.md": body("voice")})
            return repo, ()

        def bullet_without_paths():
            design = DESIGN.replace(DESIGN[DESIGN.index("- **Left out"):DESIGN.index("- **History")],
                                    "- **Left out of `import/`.** Nothing.\n")
            repo = Repo(self.tmp / "nopaths", design=design)
            repo.stage({"import/agents/monet.md": body("agent")})
            return repo, ()

        def non_utf8_design():
            repo = Repo(self.tmp / "latin")
            repo.stage({"import/agents/monet.md": body("agent")})
            (repo.path / "specs" / "design.md").write_bytes(b"\xff\xfe" + DESIGN.encode())
            return repo, ()

        def no_design():
            repo = Repo(self.tmp / "nodesign")
            repo.stage({"import/agents/monet.md": body("agent")})
            (repo.path / "specs" / "design.md").unlink()
            return repo, ()

        def design_is_a_dir():
            repo = Repo(self.tmp / "designdir")
            repo.stage({"import/agents/monet.md": body("agent")})
            (repo.path / "specs" / "design.md").unlink()
            (repo.path / "specs" / "design.md").mkdir()
            return repo, ()

        def repo_is_a_file():
            repo = Repo.__new__(Repo)
            repo.path = self.tmp / "afile"
            repo.path.write_text("not a repo\n", encoding="utf-8")
            return repo, ()

        def bad_snapshot():
            repo = Repo(self.tmp / "badsnap")
            repo.stage({"import/agents/monet.md": body("agent")})
            return repo, ("--snapshot", str(self.tmp / "missing"), "HEAD")

        def shallow():
            # A depth-2 clone ends at the bootstrap, which then seems to add
            # every file it holds, so the audit cannot trust it.
            repo = Repo(self.tmp / "full")
            repo.stage({"import/agents/monet.md": body("agent")})
            repo.move("import/agents/monet.md", "agents/monet.md")
            repo.commit("port the agent")
            git(self.tmp, "clone", "-q", "--depth", "2", (repo.path).as_uri(), "shallow")
            clone = Repo.__new__(Repo)
            clone.path = self.tmp / "shallow"
            return clone, ()

        for setup, words in ((no_bootstrap, "no commit adds a file under import/"),
                             (no_skill_dirs, "no MONET_SKILL_DIRS= line"),
                             (empty_skill_dirs, "no MONET_SKILL_DIRS= line with a skill name"),
                             (bullet_without_paths, "with backticked paths"),
                             (non_utf8_design, "is not UTF-8"),
                             (no_design, "cannot read"),
                             (design_is_a_dir, "cannot read"),
                             (repo_is_a_file, "cannot read"),
                             (bad_snapshot, "git ls-tree"),
                             (shallow, "shallow clone boundary")):
            with self.subTest(setup.__name__):
                repo, args = setup()
                code, lines, err = repo.audit(*args)
                self.assertEqual(code, 2, "\n".join(lines) + err)
                self.assertIn(words, err)

    def test_snapshot_pathspecs_match_the_bootstrap(self):
        # The exact read that acceptance names, in its order.
        pairs = port_audit.snapshot_paths(["git-ship", "voice"])
        self.assertEqual([spec for spec, _ in pairs], [
            ".claude/agents/monet.md", ".claude/commands/monet/", ".claude/output-styles/ste-lite.md",
            ".claude/templates/skill-template.md", ".claude/workflows/monet/", ".claude/docs/specs/",
            ".claude/skills/git-ship/", ".claude/skills/voice/",
            "bin/check-monet-skill.py", "bin/test-check-monet-skill.py", "bin/test-gang.sh",
            "bin/test-paint-explainer.py", "bin/test-ste-check.py"])
        # Default mode audits one file per pathspec: each is a snapshot file,
        # not staging, so each one left under import/ fails.
        files = [imp + "f.md" if imp.endswith("/") else imp for _, imp in pairs]
        self.repo.stage({rel: body(rel) for rel in files})

        code, lines, _ = self.repo.audit()
        self.assertEqual(code, 1, "\n".join(lines))
        self.assertEqual([line for line in lines if line.startswith("import/") and "left out" not in line],
                         [f"{rel}: unaccounted: still tracked under import/" for rel in sorted(files)])

    def test_snapshot_commit_must_not_be_an_option(self):
        snap, _ = self.snapshot()
        pairs = port_audit.snapshot_paths(["voice"])
        with self.assertRaisesRegex(port_audit.SetupError, "not an option"):
            port_audit.read_snapshot(str(snap), "--output=" + str(self.tmp / "out"), pairs)
        self.assertFalse((self.tmp / "out").exists())

    def snapshot(self):
        """A temp snapshot repo with monet files, left-out files, and other files."""
        snap = self.tmp / "snapshot"
        snap.mkdir()
        git(snap, "init", "-q")
        files = {
            ".claude/agents/monet.md": "agent", ".claude/agents/other.md": "other agent",
            ".claude/commands/monet/voice.md": "command", ".claude/commands/other.md": "other command",
            ".claude/skills/voice/SKILL.md": "voice", ".claude/skills/voice/reference/ste-lite.md": "card",
            ".claude/skills/voice/reference/personas/private.md": "persona",
            ".claude/skills/voice/scripts/a.py": "script a", ".claude/skills/voice/scripts/b.py": "script b",
            ".claude/skills/git-ship/SKILL.md": "ship", ".claude/skills/other/SKILL.md": "other skill",
            ".claude/settings.json": "settings", "bin/test-gang.sh": "gang", "bin/other.sh": "other bin",
            "README.md": "readme",
        }
        for rel, tag in files.items():
            full = snap / rel
            full.parent.mkdir(parents=True, exist_ok=True)
            full.write_text(body(tag), encoding="utf-8")
        link = snap / ".claude" / "output-styles" / "ste-lite.md"
        link.parent.mkdir(parents=True)
        os.symlink("../skills/voice/reference/ste-lite.md", link)
        git(snap, "add", "-A", "-f")
        git(snap, *IDENTITY, "commit", "-q", "--no-verify", "-m", "snapshot")
        return snap, git(snap, "rev-parse", "HEAD").strip()

    def test_snapshot_mode_counts(self):
        snap, commit = self.snapshot()
        staged = {"import/agents/monet.md": "agent", "import/commands/monet/voice.md": "command",
                  "import/skills/voice/SKILL.md": "voice", "import/skills/voice/reference/ste-lite.md": "card",
                  "import/skills/git-ship/SKILL.md": "ship", "import/bin/test-gang.sh": "gang",
                  "import/README.md": "readme", "import/docs/specs/NOTE.md": "note"}
        self.repo.link("import/output-styles/ste-lite.md", "../skills/voice/reference/ste-lite.md")
        self.repo.stage({rel: body(tag) for rel, tag in staged.items()})
        self.repo.move("import/agents", "agents")
        self.repo.move("import/output-styles", "output-styles")
        self.repo.move("import/skills/voice", "skills/voice")
        self.repo.move("import/skills/git-ship", "skills/ship")
        self.repo.move("import/bin/test-gang.sh", "tools/test-gang.sh")
        self.repo.remove("import/commands")
        self.repo.write("tools/test-gang.sh", edit(body("gang"), (0,), "tweak"))
        self.repo.commit("port everything")

        code, lines, err = self.repo.audit("--snapshot", str(snap), commit[:7])
        self.assertPassed(code, lines, err)
        self.assertIn("snapshot monet files 10, staged 7, left out 3", lines)
        self.assertIn(f"bootstrap {self.repo.bootstrap}: 9 files under import/, 7 in the snapshot", lines)
        self.assertIn("moved 6, folded 1, staging 2, unaccounted 0", lines)
        self.assertIn("import/README.md: staging", lines)
        # Inside a pathspec dir and still under import/, but the snapshot
        # does not hold it: staging only, so no failure.
        self.assertIn("import/docs/specs/NOTE.md: staging", lines)
        self.assertIn("import/commands/monet/voice.md: folded -> skills/voice/SKILL.md", lines)
        self.assertIn("import/output-styles/ste-lite.md: moved -> output-styles/ste-lite.md", lines)

        # A monet file that the snapshot gains, and a left-out entry that no
        # snapshot file matches, each fail the audit. A staged file that the
        # snapshot lost has no counterpart now, so it is staging only. A file
        # next to the left-out dir scripts/ is not under it.
        (snap / ".claude/skills/voice/new.md").write_text(body("new"), encoding="utf-8")
        (snap / ".claude/skills/voice/scripts.md").write_text(body("sibling"), encoding="utf-8")
        git(snap, "rm", "-q", "-r", ".claude/skills/git-ship/SKILL.md", ".claude/skills/voice/scripts")
        git(snap, "add", "-A", "-f")
        git(snap, *IDENTITY, "commit", "-q", "--no-verify", "-m", "drift")
        code, lines, _ = self.repo.audit("--snapshot", str(snap), "HEAD")
        self.assertUnaccounted(code, lines, "import/skills/voice/new.md")
        self.assertUnaccounted(code, lines, "import/skills/voice/scripts/")
        self.assertUnaccounted(code, lines, "import/skills/voice/scripts.md")
        self.assertNotIn("import/skills/voice/scripts/: left out", lines)
        self.assertIn("import/skills/git-ship/SKILL.md: staging", lines)
        self.assertIn("snapshot monet files 9, staged 6, left out 1", lines)
        self.assertEqual(lines[-1], "audit failed: 3 unaccounted")


class History(unittest.TestCase):
    """Merge shapes that pin which base a merge reads in follow(): an octopus
    takes each parent's base from every earlier head, and a merge with several
    best merge bases takes all of them into account. In each, a file the
    merge keeps at x.md is unrelated to the port, so the audit must fail."""

    setUp = PortAudit.setUp
    assertUnaccounted = PortAudit.assertUnaccounted

    def assertGone(self, repo, x):
        self.assertEqual(git(repo.path, "show", f"HEAD:{x}"), body("unrelated"))
        code, lines, _ = repo.audit()
        self.assertUnaccounted(code, lines, "import/skills/voice/a.md")
        self.assertNotIn(f"import/skills/voice/a.md: moved -> {x}", lines)
        self.assertIn(f"import/skills/voice/a.md: unaccounted: renamed to {x}, then gone, "
                      "with no folded entry", lines)

    def test_octopus_unrelated_readd_is_not_a_move(self):
        # The trunk ports a.md to x.md in c. b1 forks from the bootstrap, and
        # b2 forks from c, drops x.md, then adds an unrelated x.md. With the
        # trunk among the earlier heads, b2's base is c, so the octopus keeps
        # b2's x.md. With b1 alone it is the bootstrap, which keeps the port.
        x, repo = "skills/voice/x.md", self.repo
        repo.stage({"import/skills/voice/a.md": body("a")})
        trunk = repo.trunk()
        repo.move("import/skills/voice/a.md", x)
        port = repo.commit("port a")
        git(repo.path, "checkout", "-q", "-b", "b1", repo.bootstrap)
        repo.write("notes/b1.md", body("b1"))
        repo.commit("an unrelated commit")
        git(repo.path, "checkout", "-q", "-b", "b2", port)
        repo.remove(x)
        repo.commit("drop x.md")
        repo.write(x, body("unrelated"))
        repo.commit("an unrelated x.md")
        git(repo.path, "checkout", "-q", trunk)
        repo.write("notes/trunk.md", body("trunk"))
        tip = repo.commit("a trunk commit")
        self.assertEqual(git(repo.path, "merge-base", "--all", "b2", tip, "b1").split(), [port])
        self.assertEqual(git(repo.path, "merge-base", "--all", "b2", "b1").split(), [repo.bootstrap])
        repo.pull("b1", "b2")
        self.assertEqual(len(git(repo.path, "rev-list", "--parents", "-n1", "HEAD").split()), 4)
        self.assertGone(repo, x)

    def test_two_best_bases_use_all_bases(self):
        # From the bootstrap, p1 ports a.md to x.md and p2 makes an unrelated
        # commit. l1 merges p2 into p1 and l2 merges p1 into p2, so p1 and p2
        # are both best merge bases of l1 and l2. l2 drops x.md and adds an
        # unrelated x.md, and l1 merges l2: the bases merged hold the port, so
        # the merge keeps l2's x.md. A base read from p2 alone keeps the port.
        # git lists the bases by date, so the dates set the order in which
        # follow() merges them, and the result must not depend on it.
        x = "skills/voice/x.md"
        for name, port_date, other_date in (("the newer base ports a.md", "2081", "2080"),
                                            ("the older base ports a.md", "2080", "2081")):
            with self.subTest(name):
                repo = Repo(self.tmp / name.replace(" ", "_"))
                repo.stage({"import/skills/voice/a.md": body("a")})
                git(repo.path, "checkout", "-q", "-b", "p2")
                repo.write("notes/p2.md", body("p2"))
                p2 = repo.commit("an unrelated commit", date=f"{other_date}-01-01 00:00:00 +0000")
                git(repo.path, "checkout", "-q", "-b", "p1", repo.bootstrap)
                repo.move("import/skills/voice/a.md", x)
                p1 = repo.commit("port a", date=f"{port_date}-01-01 00:00:00 +0000")
                for line, start, other in (("l1", "p1", "p2"), ("l2", "p2", "p1")):
                    git(repo.path, "checkout", "-q", "-b", line, start)
                    repo.pull(other)
                repo.remove(x)
                repo.commit("drop x.md")
                repo.write(x, body("unrelated"))
                repo.commit("an unrelated x.md")
                self.assertEqual(sorted(git(repo.path, "merge-base", "--all", "l1", "l2").split()),
                                 sorted([p1, p2]))
                git(repo.path, "checkout", "-q", "l1")
                repo.pull("l2")
                self.assertGone(repo, x)


class Invocation(unittest.TestCase):
    """How the tool reads its repo and its snapshot from the environment and
    the command line: the --repo and --snapshot GIT_DIR arguments win over
    GIT_* variables, and a --snapshot GIT_DIR may start with ~."""

    setUp = PortAudit.setUp
    assertPassed = PortAudit.assertPassed
    assertUnaccounted = PortAudit.assertUnaccounted
    snapshot = PortAudit.snapshot

    def test_default_repo_is_this_repo(self):
        # The VERIFY command passes no --repo: the tool audits the repo that
        # holds it, not its tools/ dir.
        self.assertEqual(port_audit.ROOT, ROOT)
        with mock.patch.object(port_audit, "audit", return_value=0) as audit:
            self.assertEqual(port_audit.main([]), 0)
        audit.assert_called_once_with(str(ROOT), None)

    def test_exported_git_dir_does_not_redirect_the_audit(self):
        # ported moves a.md out of import/ and passes; staged keeps it under
        # import/ and fails. A git hook may export GIT_DIR and GIT_WORK_TREE of
        # another repo: the tool must still audit the --repo it was given.
        ported, staged = self.repo, Repo(self.tmp / "staged")
        for repo in (ported, staged):
            repo.stage({"import/skills/voice/a.md": body("a")})
        ported.move("import/skills/voice/a.md", "skills/voice/a.md")
        ported.commit("port a")
        for target, other in ((ported, staged), (staged, ported)):
            with self.subTest(target=target.path.name):
                exported = {"GIT_DIR": str(other.path / ".git"), "GIT_WORK_TREE": str(other.path)}
                code, lines, err = target.audit(extra_env=exported)
                if target is ported:
                    self.assertPassed(code, lines, err)
                    self.assertIn("import/skills/voice/a.md: moved -> skills/voice/a.md", lines)
                else:
                    self.assertUnaccounted(code, lines, "import/skills/voice/a.md")
                    self.assertIn("import/skills/voice/a.md: unaccounted: still tracked under import/",
                                  lines)

    def test_snapshot_git_dir_expands_home(self):
        # The VERIFY command passes ~/.config, which a shell expands. Run with
        # no shell, the tool must expand ~ itself, against HOME.
        snap, _ = self.snapshot()
        self.assertEqual(snap, self.tmp / "snapshot")
        self.repo.stage({"import/agents/monet.md": body("agent")})
        self.repo.move("import/agents/monet.md", "agents/monet.md")
        self.repo.commit("port the agent")
        code, lines, err = self.repo.audit("--snapshot", "~/snapshot", "HEAD", extra_env={"HOME": str(self.tmp)})
        self.assertEqual(code, 1, "\n".join(lines) + err)
        self.assertIn("import/agents/monet.md: moved -> agents/monet.md", lines)
        self.assertIn("snapshot monet files 10, staged 1, left out 3", lines)
        self.assertUnaccounted(code, lines, "import/skills/voice/SKILL.md")


class RepoState(unittest.TestCase):
    """The repo once DOC-10 removed import/."""

    def test_import_dir_absent(self):
        self.assertEqual(git(ROOT, "ls-files", "import"), "")
        self.assertFalse((ROOT / "import").exists())

    def test_no_personal_persona_tracked(self):
        self.assertEqual(git(ROOT, "ls-files", "skills/voice/reference/personas/*").split(),
                         ["skills/voice/reference/personas/sample.md"])

    def test_no_component_points_at_import(self):
        # The port contract tests under skills/*/tests/ assert that import/ paths
        # are gone. They load nothing from it, so this check leaves them out.
        hits = subprocess.run(
            ["git", "-C", str(ROOT), "grep", "-n", "import/", "--", "skills", "agents", "output-styles",
             ":(exclude,glob)skills/*/tests/**"],
            capture_output=True, text=True, check=False,
            env={k: v for k, v in os.environ.items() if not k.startswith("GIT_")})
        self.assertEqual(hits.returncode, 1, hits.stdout + hits.stderr)


if __name__ == "__main__":
    unittest.main()
