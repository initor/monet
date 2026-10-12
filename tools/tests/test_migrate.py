"""Tests for scripts/migrate-from-dotfiles.sh, the dotfiles migration script.

Every run sets HOME to a temp dir, so no test reads or changes the real HOME.
The temp HOME holds a dotfiles install of monet with synthetic personas and
interview-prep data. Paths are built with os.path.join, so this file holds no
dotfiles path and passes the portability scan with no exemption.
"""

import hashlib
import os
import re
import shlex
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SCRIPT = str(ROOT / "scripts" / "migrate-from-dotfiles.sh")
# macOS ships bash 3.2 as /bin/bash and CI has bash 5 there, so the runs below
# cover both shells across the two hosts.
BASH = "/bin/bash" if os.path.exists("/bin/bash") else "bash"

# The 28 monet skill dirs of the dotfiles snapshot.
SKILLS = sorted("""
    bq-mock bq-playground code-mock code-playground cwd-context distill-voice gang
    git-changes git-draft-commit git-draft-pr git-ship hairline-create handoff
    interview-progress interview-tracker paint pr prep review-code review-local
    roadmap sd-explainer sd-mock sd-playground teach visual-design voice web-operator
""".split())
# Each old copy, relative to the .claude dir, in the order the script moves it.
OLD = ["agents/monet.md", "commands/monet", "output-styles/ste-lite.md"] + ["skills/" + s for s in SKILLS]
# Entries under the .claude dir that are not monet.
OTHERS = ["skills/peon-ping-toggle", "skills/other", "settings.json", "CLAUDE.md",
          "agents/helper.md", "commands/other.md"]
PERSONAS = ["terse", "warm"]
PREP = ["2026-01-02_sd_full_1.json", "2026-01-03_bq_drill_1.json", "dashboard.html",
        os.path.join("playgrounds", "sd-diagram-cache-2026-01-02.html")]
STE_LINK = os.path.join("..", "skills", "voice", "reference", "ste-lite.md")
STAMP = re.compile(r"^dotfiles-\d{8}-\d{6}$")


def join(base, rel):
    return os.path.join(base, *rel.split("/"))


def write(path, data):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "wb") as f:
        f.write(data.encode("utf-8"))


def read(path):
    with open(path, "rb") as f:
        return f.read()


def entry(path):
    """('link', target), ('file', sha256), ('dir',), or None when absent."""
    if os.path.islink(path):
        return ("link", os.readlink(path))
    if os.path.isdir(path):
        return ("dir",)
    if os.path.isfile(path):
        return ("file", hashlib.sha256(read(path)).hexdigest(), os.stat(path).st_mode)
    return None


def tree(root):
    """Each path under root, relative, mapped to its entry. Links are not followed."""
    out = {}
    for dirpath, dirnames, filenames in os.walk(root):
        for name in dirnames + filenames:
            full = os.path.join(dirpath, name)
            out[os.path.relpath(full, root)] = entry(full)
    return out


def build_home(home, claude=None):
    """A dotfiles install of monet in claude (default home/.claude), plus other files."""
    claude = claude or os.path.join(home, ".claude")
    write(join(claude, "agents/monet.md"), "the monet agent\n")
    write(join(claude, "commands/monet/help.md"), "help wrapper\n")
    write(join(claude, "commands/monet/ship.md"), "ship wrapper\n")
    for name in SKILLS:
        write(join(claude, "skills/" + name + "/SKILL.md"), "name: " + name + "\n")
    ref = join(claude, "skills/voice/reference")
    write(join(ref, "ste-lite.md"), "the ste-lite card\n")
    write(join(ref, "personas.md"), "surface map: commit message -> terse\n")
    for p in PERSONAS:
        write(join(ref, "personas/" + p + ".md"), "persona " + p + ": short sentences\n")
    write(join(claude, "skills/gang/scripts/run.sh"), "#!/bin/sh\necho run\n")
    os.chmod(join(claude, "skills/gang/scripts/run.sh"), 0o755)
    os.makedirs(join(claude, "output-styles"))
    os.symlink(STE_LINK, join(claude, "output-styles/ste-lite.md"))
    for rel in OTHERS:
        name = rel + "/SKILL.md" if rel.startswith("skills/") else rel
        write(join(claude, name), "not monet: " + rel + "\n")
    for rel in PREP:
        write(os.path.join(home, ".config", "interview-prep", rel), "session " + rel + "\n")


class MigrateFromDotfiles(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        # The real path, so the script's $PWD and the paths below agree.
        self.tmp = os.path.realpath(tmp.name)
        self.home = os.path.join(self.tmp, "home")
        self.claude = os.path.join(self.home, ".claude")
        self.data = os.path.join(self.home, ".config", "monet")
        self.backup = os.path.join(self.tmp, "backup")
        build_home(self.home)

    def run_script(self, *args, home=None, shell=BASH, timeout=120):
        """Run the script in the temp dir with HOME set to a temp dir, never the real one."""
        env = dict(os.environ, HOME=home or self.home, PWD=self.tmp)
        cmd = ([shell] if shell else []) + [SCRIPT, *args]
        return subprocess.run(cmd, env=env, cwd=self.tmp, capture_output=True, text=True, timeout=timeout)

    def ok(self, *args, home=None):
        r = self.run_script(*args, home=home)
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        return r

    def copies(self, home=None, data=None):
        home = home or self.home
        data = data or self.data
        ref = join(os.path.join(home, ".claude"), "skills/voice/reference")
        pairs = [(join(ref, "personas/" + p + ".md"), join(data, "personas/" + p + ".md")) for p in PERSONAS]
        pairs.append((join(ref, "personas.md"), join(data, "personas/personas.md")))
        prep = os.path.join(home, ".config", "interview-prep")
        return pairs + [(os.path.join(prep, rel), os.path.join(data, "interview-prep", rel)) for rel in PREP]

    def moves(self, backup, home=None):
        claude = os.path.join(home or self.home, ".claude")
        return [(join(claude, rel), join(backup, rel)) for rel in OLD]

    def only_backup(self):
        names = os.listdir(os.path.join(self.data, "backup"))
        self.assertEqual(len(names), 1, names)
        self.assertRegex(names[0], STAMP)
        return os.path.join(self.data, "backup", names[0])

    def assert_nothing_lost(self, before, home, claude, data, backup):
        """Each file and link in before sits at its old path, its data-dir copy, or its backup path."""
        copy_of = dict(self.copies(home, data))
        for rel, old in before.items():
            if old is None or old[0] == "dir":
                continue
            full = os.path.join(home, rel)
            places = [full]
            # The script reads each source through home/.claude, a link or not.
            via = os.path.join(home, ".claude", os.path.relpath(full, claude))
            if via in copy_of:
                places.append(copy_of[via])
            if full in copy_of:
                places.append(copy_of[full])
            if not os.path.relpath(full, claude).startswith(".."):
                places.append(os.path.join(backup, os.path.relpath(full, claude)))
            self.assertIn(old, [entry(p) for p in places], rel)

    def test_dry_run_changes_nothing(self):
        before = tree(self.home)
        r = self.ok("--backup-dir", self.backup)
        self.assertEqual(tree(self.home), before)
        self.assertFalse(os.path.lexists(self.backup))
        steps = [line for line in r.stdout.splitlines() if line.startswith("would ")]
        expected = ["would copy %s -> %s" % pair for pair in self.copies()]
        expected += ["would move %s -> %s" % pair for pair in self.moves(self.backup)]
        self.assertEqual(sorted(steps), sorted(expected))
        # The default backup dir is timestamped under the data dir.
        r = self.ok()
        backup = os.path.join(self.data, "backup", "dotfiles-")
        moved = [line for line in r.stdout.splitlines() if line.startswith("would move ")]
        self.assertEqual(len(moved), len(OLD))
        for line in moved:
            self.assertRegex(line.split(" -> ")[1], "^" + re.escape(backup) + r"\d{8}-\d{6}/")
        # With only --data-dir, the default backup dir follows that data dir.
        data = os.path.join(self.tmp, "data")
        r = self.ok("--data-dir", data)
        backup = os.path.join(data, "backup", "dotfiles-")
        moved = [line for line in r.stdout.splitlines() if line.startswith("would move ")]
        self.assertEqual(len(moved), len(OLD))
        for line in moved:
            self.assertRegex(line.split(" -> ")[1], "^" + re.escape(backup) + r"\d{8}-\d{6}/")
        self.assertEqual(tree(self.home), before)
        self.assertFalse(os.path.lexists(data))

    def test_apply_copies_personas_and_interview_prep(self):
        # Only the *.md files in personas/ are personas.
        write(join(self.claude, "skills/voice/reference/personas/notes.txt"), "not a persona\n")
        sources = {src: read(src) for src, _ in self.copies()}
        r = self.ok("--apply")
        self.assertEqual(sorted(os.listdir(join(self.data, "personas"))),
                         sorted([p + ".md" for p in PERSONAS] + ["personas.md"]))
        for src, dst in self.copies():
            self.assertEqual(read(dst), sources[src], dst)
            self.assertIn("copied %s -> %s" % (src, dst), r.stdout.splitlines())
        # The interview-prep source stays in place.
        for rel in PREP:
            path = os.path.join(self.home, ".config", "interview-prep", rel)
            self.assertEqual(read(path), sources[path])
        # The personas reach the paths that skills/voice/SKILL.md reads.
        voice = (ROOT / "skills" / "voice" / "SKILL.md").read_text(encoding="utf-8")
        self.assertIn("${user_config.data_dir}/personas/personas.md", voice)
        self.assertIn("${user_config.data_dir}/personas/<name>.md", voice)

    def test_apply_moves_old_copies_to_backup(self):
        before = tree(self.claude)
        r = self.ok("--apply", "--backup-dir", self.backup)
        for src, dst in self.moves(self.backup):
            self.assertFalse(os.path.lexists(src), src)
            rel = os.path.relpath(src, self.claude)
            self.assertEqual(entry(dst), before[rel], dst)
            self.assertIn("moved %s -> %s" % (src, dst), r.stdout.splitlines())
        # Files inside a moved dir keep their path relative to the .claude dir.
        for rel, old in before.items():
            if any(rel == o or rel.startswith(o + os.sep) for o in map(os.path.normpath, OLD)):
                self.assertEqual(entry(os.path.join(self.backup, rel)), old, rel)
        # The relative ste-lite link still resolves inside the backup.
        link = join(self.backup, "output-styles/ste-lite.md")
        self.assertEqual(os.readlink(link), STE_LINK)
        self.assertEqual(read(link), b"the ste-lite card\n")

    def test_apply_never_deletes(self):
        before = tree(self.home)
        self.ok("--apply")
        self.assert_nothing_lost(before, self.home, self.claude, self.data, self.only_backup())

    def test_apply_through_symlinked_claude_dir(self):
        home = os.path.join(self.tmp, "linked")
        real = os.path.join(home, "dotfiles", ".claude")
        build_home(home, real)
        os.symlink(os.path.join("dotfiles", ".claude"), os.path.join(home, ".claude"))
        before = tree(home)
        self.ok("--apply", "--backup-dir", self.backup, home=home)
        link = os.path.join(home, ".claude")
        self.assertTrue(os.path.islink(link))
        self.assertEqual(os.readlink(link), os.path.join("dotfiles", ".claude"))
        for rel in OLD:
            self.assertFalse(os.path.lexists(join(real, rel)), rel)
            self.assertTrue(os.path.lexists(join(self.backup, rel)), rel)
        for rel in OTHERS:
            self.assertTrue(os.path.exists(join(real, rel)), rel)
        data = os.path.join(home, ".config", "monet")
        self.assert_nothing_lost(before, home, real, data, self.backup)
        self.ok("--check", home=home)

    def test_conflict_changes_nothing(self):
        personas = join(self.data, "personas")
        differs = [os.path.join(personas, "terse.md"), os.path.join(self.data, "interview-prep", PREP[0])]
        for path in differs:
            write(path, "a different file\n")
        # A destination with the same bytes as its source is not a conflict.
        same = os.path.join(personas, "warm.md")
        shutil.copyfile(join(self.claude, "skills/voice/reference/personas/warm.md"), same)
        before = tree(self.home)
        r = self.run_script("--apply")
        self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
        self.assertEqual(tree(self.home), before)
        self.assertFalse(os.path.lexists(os.path.join(self.data, "backup")))
        out = r.stdout + r.stderr
        conflicts = [line for line in out.splitlines() if line.startswith("conflict: ")]
        self.assertEqual(len(conflicts), len(differs), out)
        for path in differs:
            self.assertIn(path, out)
        self.assertNotIn(same, out)

    def test_conflict_with_nothing_else_to_do(self):
        # The old copies are gone, but one data-dir copy now differs from its source.
        self.ok("--apply")
        changed = os.path.join(self.data, "interview-prep", "dashboard.html")
        write(changed, "a newer dashboard\n")
        before = tree(self.tmp)
        for args in (["--apply"], []):
            r = self.run_script(*args)
            self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
            self.assertNotIn("nothing to do", r.stdout)
            conflicts = [line for line in r.stderr.splitlines() if line.startswith("conflict: ")]
            self.assertEqual(len(conflicts), 1, r.stderr)
            self.assertIn(changed, conflicts[0])
            self.assertEqual(tree(self.tmp), before)

    def test_case_only_collision_changes_nothing(self):
        # On a disk that ignores case, Personas.md and the persona map are one file.
        persona = join(self.claude, "skills/voice/reference/personas/Personas.md")
        write(persona, "a persona\n")
        before = tree(self.tmp)
        r = self.run_script("--apply")
        self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
        self.assertEqual(tree(self.tmp), before)
        conflicts = [line for line in r.stderr.splitlines() if line.startswith("conflict: ")]
        self.assertEqual(len(conflicts), 1, r.stderr)
        self.assertIn("differ only in case", conflicts[0])
        self.assertIn(persona, conflicts[0])

    @unittest.skipIf(os.geteuid() == 0, "root reads a mode-000 dir")
    def test_unreadable_sources_change_nothing(self):
        prep = os.path.join(self.home, ".config", "interview-prep")
        locked = os.path.join(prep, "locked")
        secret = os.path.join(prep, "secret.json")
        write(os.path.join(locked, "s.json"), "session\n")
        write(secret, "session\n")
        os.chmod(secret, 0)
        os.chmod(locked, 0)
        self.addCleanup(os.chmod, locked, 0o755)
        before = tree(self.claude)
        for args in (["--apply"], []):
            r = self.run_script(*args)
            self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
            self.assertEqual(tree(self.claude), before)
            self.assertFalse(os.path.lexists(self.data))
            self.assertIn("conflict: find cannot read every entry under " + prep, r.stderr)
            self.assertIn("conflict: %s is not a file that can be read" % secret, r.stderr)

    @unittest.skipIf(os.geteuid() == 0, "root writes to a read-only dir")
    def test_failed_move_names_the_backup_dir(self):
        before = tree(self.home)
        skills = join(self.claude, "skills")
        os.chmod(skills, 0o555)
        self.addCleanup(os.chmod, skills, 0o755)
        r = self.run_script("--apply")
        self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
        backup = self.only_backup()
        self.assertIn("run again with --backup-dir " + backup, r.stderr)
        # A rerun into that dir keeps one backup dir, and nothing is lost.
        os.chmod(skills, 0o755)
        self.ok("--apply", "--backup-dir", backup)
        self.assertEqual(self.only_backup(), backup)
        self.assert_nothing_lost(before, self.home, self.claude, self.data, backup)
        self.ok("--check")

    @unittest.skipIf(os.geteuid() == 0, "root reads a mode-000 dir")
    def test_unreadable_personas_dir_changes_nothing(self):
        # A dir that cannot be read or searched hides the personas, yet skills/voice would move.
        ref = join(self.claude, "skills/voice/reference")

        def unlock(path):
            if os.path.isdir(path):  # a failed run may have moved it
                os.chmod(path, 0o755)

        voice = join(self.claude, "skills/voice")
        for locked, mode in ((join(ref, "personas"), 0), (join(ref, "personas"), 0o444),
                             (join(ref, "personas"), 0o111), (ref, 0), (voice, 0), (voice, 0o600)):
            os.chmod(locked, mode)
            self.addCleanup(unlock, locked)
            r = self.run_script("--apply")
            unlock(locked)
            self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
            self.assertEqual([line for line in r.stderr.splitlines() if line.startswith("conflict: ")],
                             ["conflict: %s is a dir that cannot be read, so the personas in it "
                              "cannot be copied" % locked])
            self.assertFalse(os.path.lexists(self.data))
            self.assertTrue(os.path.exists(join(self.claude, "skills/voice/SKILL.md")))

    def test_persona_that_is_not_a_file_changes_nothing(self):
        # cp fails on a dir halfway through, and blocks forever on a FIFO.
        personas = join(self.claude, "skills/voice/reference/personas")
        adir = os.path.join(personas, "adir.md")
        pipe = os.path.join(personas, "pipe.md")
        os.makedirs(adir)
        os.mkfifo(pipe)
        before = tree(self.tmp)
        for args in (["--apply"], []):
            r = self.run_script(*args)
            self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
            self.assertEqual(tree(self.tmp), before)
            self.assertEqual(sorted(line for line in r.stderr.splitlines() if line.startswith("conflict: ")),
                             ["conflict: %s is not a file that can be read, so it cannot be copied" % p
                              for p in (adir, pipe)])

    def test_data_dir_link_into_old_copy_is_a_conflict(self):
        # A data-dir persona linked to its source has the same bytes, but dangles once skills/voice moves.
        src = join(self.claude, "skills/voice/reference/personas/terse.md")
        dst = join(self.data, "personas/terse.md")
        os.makedirs(os.path.dirname(dst))
        os.symlink(src, dst)
        before = tree(self.tmp)
        r = self.run_script("--apply")
        self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
        self.assertEqual([line for line in r.stderr.splitlines() if line.startswith("conflict: ")],
                         ["conflict: %s exists and differs from %s" % (dst, src)])
        self.assertEqual(tree(self.tmp), before)

    def test_apply_without_interview_prep(self):
        # Most users never ran a mock interview, so there is no interview-prep dir.
        shutil.rmtree(os.path.join(self.home, ".config", "interview-prep"))
        self.ok("--apply", "--backup-dir", self.backup)
        self.assertFalse(os.path.lexists(os.path.join(self.data, "interview-prep")))
        for src, dst in self.copies()[:len(PERSONAS) + 1]:
            self.assertEqual(read(dst), read(join(self.backup, os.path.relpath(src, self.claude))))
        for src, dst in self.moves(self.backup):
            self.assertFalse(os.path.lexists(src), src)
            self.assertTrue(os.path.lexists(dst), dst)
        self.ok("--check")

    def test_linked_interview_prep_root(self):
        # The interview-prep root may be a link to a dir: its files are copied, and the link stays.
        prep = os.path.join(self.home, ".config", "interview-prep")
        real = os.path.join(self.home, "prep-real")
        os.rename(prep, real)
        os.symlink(os.path.join("..", "prep-real"), prep)
        r = self.ok("--apply", "--backup-dir", self.backup)
        for rel in PREP:
            dst = os.path.join(self.data, "interview-prep", rel)
            self.assertTrue(os.path.isfile(dst) and not os.path.islink(dst), dst)
            self.assertEqual(read(dst), read(os.path.join(real, rel)), dst)
            self.assertIn("copied %s -> %s" % (os.path.join(prep, rel), dst), r.stdout.splitlines())
        self.assertEqual(os.readlink(prep), os.path.join("..", "prep-real"))

    @unittest.skipIf(os.geteuid() == 0, "root reads a mode-000 dir")
    def test_linked_interview_prep_root_with_unreadable_entry(self):
        # A dir that cannot be read under the link target would hide a session from the copy.
        prep = os.path.join(self.home, ".config", "interview-prep")
        real = os.path.join(self.home, "prep-real")
        os.rename(prep, real)
        os.symlink(os.path.join("..", "prep-real"), prep)
        locked = os.path.join(real, "locked")
        write(os.path.join(locked, "s.json"), "session\n")
        os.chmod(locked, 0)
        self.addCleanup(os.chmod, locked, 0o755)
        before = tree(self.tmp)
        for args in (["--apply"], []):
            r = self.run_script(*args)
            self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
            self.assertEqual(tree(self.tmp), before)
            self.assertEqual([line for line in r.stderr.splitlines() if line.startswith("conflict: ")],
                             ["conflict: find cannot read every entry under %s (see its errors above)" % prep])

    def test_unset_or_empty_home_changes_nothing(self):
        # With no HOME, the paths would fall under /, and --check would pass while old copies load.
        env = {k: v for k, v in os.environ.items() if k != "HOME"}
        env["PWD"] = self.tmp
        before = tree(self.tmp)
        for home in (None, ""):
            run_env = env if home is None else dict(env, HOME=home)
            for args in (["--check"], []):
                r = subprocess.run([BASH, SCRIPT, *args], env=run_env, cwd=self.tmp,
                                   capture_output=True, text=True, timeout=120)
                self.assertEqual(r.returncode, 2, (home, args, r.stdout + r.stderr))
                self.assertIn("HOME is not set", r.stderr)
                self.assertEqual(r.stdout, "")
                self.assertEqual(tree(self.tmp), before)

    def test_dir_under_a_missing_top_level_dir(self):
        # The search for an existing parent must stop at / when no part of the path exists.
        missing = "/monet-test-missing-%s" % os.urandom(8).hex()
        self.assertFalse(os.path.lexists(missing))
        data = os.path.join(missing, "monet")
        backup = os.path.join(missing, "bak")
        try:
            r = self.run_script("--data-dir", data, "--backup-dir", backup, timeout=30)
        except subprocess.TimeoutExpired:
            self.fail("the dry run hung on a dir under a missing top-level dir")
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        out = r.stdout.splitlines()
        self.assertIn("would copy %s -> %s" % (join(self.claude, "skills/voice/reference/personas.md"),
                                               join(data, "personas/personas.md")), out)
        self.assertIn("would move %s -> %s" % (join(self.claude, "agents/monet.md"),
                                               join(backup, "agents/monet.md")), out)
        self.assertFalse(os.path.lexists(missing))

    def test_dirs_inside_interview_prep_change_nothing(self):
        # Each run would copy a data dir or backup dir inside the source into itself.
        prep = os.path.join(self.home, ".config", "interview-prep")
        for args, inside in ((["--data-dir", prep], prep),
                             (["--data-dir", os.path.join(prep, "monet")], os.path.join(prep, "monet")),
                             (["--backup-dir", os.path.join(prep, "bak")], os.path.join(prep, "bak"))):
            before = tree(self.tmp)
            r = self.run_script("--apply", *args)
            self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
            self.assertEqual(tree(self.tmp), before)
            self.assertIn("conflict: %s is inside %s" % (inside, prep), r.stderr)
        # A sibling that only shares the name prefix is not inside.
        data = os.path.join(self.home, ".config", "interview-prep-monet")
        self.ok("--apply", "--data-dir", data, "--backup-dir", os.path.join(self.home, ".config", "interview-prep-bak"))
        self.assertEqual(read(os.path.join(data, "interview-prep", PREP[0])), read(os.path.join(prep, PREP[0])))

    def test_data_dir_inside_an_old_copy_changes_nothing(self):
        # The data dir would move into the backup with skills/voice, or block that move.
        for data, inside in ((join(self.claude, "commands/monet"), join(self.claude, "commands/monet")),
                             (join(self.claude, "skills/voice/data"), join(self.claude, "skills/voice")),
                             (join(self.backup, "skills/voice/reference"), join(self.backup, "skills/voice"))):
            before = tree(self.tmp)
            r = self.run_script("--apply", "--data-dir", data, "--backup-dir", self.backup)
            self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
            self.assertEqual(tree(self.tmp), before)
            self.assertEqual([line for line in r.stderr.splitlines() if line.startswith("conflict: ")],
                             ["conflict: %s is inside %s, where an old copy is or moves to" % (data, inside)])
        # A sibling that only shares the name prefix is not inside.
        data = join(self.claude, "commands/monet-data")
        self.ok("--apply", "--data-dir", data, "--backup-dir", self.backup)
        self.assertEqual(read(join(data, "personas/terse.md")),
                         read(join(self.backup, "skills/voice/reference/personas/terse.md")))
        self.assertTrue(os.path.isdir(join(self.backup, "commands/monet")))

    def test_apply_copies_with_no_old_copy_left(self):
        # Old copies gone by hand, or new sessions after a first --apply, still need a copy.
        shutil.rmtree(self.claude)
        prep = os.path.join(self.home, ".config", "interview-prep")
        r = self.ok("--apply")
        self.assertNotIn("nothing to do", r.stdout)
        for rel in PREP:
            self.assertEqual(read(os.path.join(self.data, "interview-prep", rel)), read(os.path.join(prep, rel)))
        self.assertFalse(os.path.lexists(os.path.join(self.data, "backup")))
        write(os.path.join(prep, "2026-02-01_code_full_1.json"), "a new session\n")
        r = self.ok("--apply")
        self.assertEqual(r.stdout.splitlines(), ["copied %s -> %s" % (
            os.path.join(prep, "2026-02-01_code_full_1.json"),
            os.path.join(self.data, "interview-prep", "2026-02-01_code_full_1.json"))])
        self.assertFalse(os.path.lexists(os.path.join(self.data, "backup")))
        self.assertEqual(self.ok("--apply").stdout.strip(), "nothing to do")

    @unittest.skipIf(os.geteuid() == 0, "root writes to a read-only dir")
    def test_rerun_hint_quotes_the_backup_dir(self):
        backup = os.path.join(self.tmp, "my backup")
        skills = join(self.claude, "skills")
        os.chmod(skills, 0o555)
        self.addCleanup(os.chmod, skills, 0o755)
        r = self.run_script("--apply", "--backup-dir", backup)
        self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
        hint = r.stderr.split("run again with ", 1)[1].split(" to keep one backup dir")[0]
        self.assertEqual(shlex.split(hint), ["--backup-dir", backup])

    def test_dangling_links_count_as_present(self):
        # cp would write through a dangling link at a copy destination, out of the data dir.
        outside = os.path.join(self.tmp, "outside.md")
        dst = join(self.data, "personas/terse.md")
        os.makedirs(os.path.dirname(dst))
        os.symlink(outside, dst)
        before = tree(self.tmp)
        r = self.run_script("--apply")
        self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
        self.assertIn("conflict: " + dst, r.stderr)
        self.assertEqual(tree(self.tmp), before)
        self.assertFalse(os.path.lexists(outside))
        # A dangling old copy is still an old copy: --check names it, --apply moves it.
        os.remove(dst)
        os.remove(join(self.claude, "skills/voice/reference/ste-lite.md"))
        old = join(self.claude, "output-styles/ste-lite.md")
        self.assertIn("old copy: " + old, self.run_script("--check").stdout)
        self.ok("--apply", "--backup-dir", self.backup)
        self.assertFalse(os.path.lexists(old))
        self.assertEqual(os.readlink(join(self.backup, "output-styles/ste-lite.md")), STE_LINK)
        self.ok("--check")

    def test_copies_symlinks_as_links(self):
        ref = join(self.claude, "skills/voice/reference")
        prep = os.path.join(self.home, ".config", "interview-prep")
        data_prep = os.path.join(self.data, "interview-prep")
        # Source link, its target, and its data-dir copy. One target starts with -.
        links = [(join(ref, "personas/alias.md"), "terse.md", join(self.data, "personas/alias.md")),
                 (os.path.join(prep, "latest.html"), PREP[3], os.path.join(data_prep, "latest.html")),
                 (os.path.join(prep, "dash.html"), "-dashboard.html", os.path.join(data_prep, "dash.html")),
                 (os.path.join(prep, "archive"), os.path.join(self.home, "archive"), os.path.join(data_prep, "archive"))]
        # A link to a dir is copied as a link, not as the files under its target,
        # so a target that cannot be read is no conflict.
        write(os.path.join(self.home, "archive", "2025-12-01_sd_full_1.json"), "an archived session\n")
        if os.geteuid() != 0:
            os.chmod(os.path.join(self.home, "archive"), 0)
            self.addCleanup(os.chmod, os.path.join(self.home, "archive"), 0o755)
        for src, target, _ in links:
            os.symlink(target, src)
        # A link to the same target needs no copy, and a link to another target is a conflict.
        os.makedirs(data_prep)
        os.symlink(PREP[3], links[1][2])
        os.symlink("other.html", links[2][2])
        before = tree(self.tmp)
        r = self.run_script("--apply")
        self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
        self.assertEqual([line for line in r.stderr.splitlines() if line.startswith("conflict: ")],
                         ["conflict: %s exists and differs from %s" % (links[2][2], links[2][0])])
        self.assertEqual(tree(self.tmp), before)
        os.remove(links[2][2])
        r = self.ok("--apply")
        for src, target, dst in links:
            self.assertTrue(os.path.islink(dst), dst)
            self.assertEqual(os.readlink(dst), target, dst)
        for src, target, _ in links[1:]:
            self.assertEqual(os.readlink(src), target, src)  # interview-prep stays in place
        self.assertNotIn("copied " + links[1][0], r.stdout)
        self.assertEqual(sorted(os.listdir(self.tmp)), ["home"])

    def test_collisions_and_blocked_parents_change_nothing(self):
        # A persona named personas.md and the persona map share one destination.
        write(join(self.claude, "skills/voice/reference/personas/personas.md"), "a persona\n")
        write(os.path.join(self.data, "interview-prep"), "a file, not a dir\n")
        write(self.backup, "a file, not a dir\n")
        before = tree(self.tmp)
        r = self.run_script("--apply", "--backup-dir", self.backup)
        self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
        self.assertEqual(tree(self.tmp), before)
        for text in ("%s is the destination of both" % join(self.data, "personas/personas.md"),
                     "a parent of %s is not a dir" % os.path.join(self.data, "interview-prep", PREP[0]),
                     "a parent of %s is not a dir" % join(self.backup, "agents/monet.md")):
            self.assertIn("conflict: " + text, r.stderr)

    def test_dir_values_are_made_absolute(self):
        terse = join(self.claude, "skills/voice/reference/personas/terse.md")
        monet = join(self.claude, "agents/monet.md")
        for data, backup, data_abs, backup_abs in (
                ("~/d//", "rel/bak/", os.path.join(self.home, "d"), os.path.join(self.tmp, "rel", "bak")),
                ("~", self.backup + "/", self.home, self.backup)):
            out = self.ok("--data-dir", data, "--backup-dir", backup).stdout.splitlines()
            self.assertIn("would copy %s -> %s" % (terse, join(data_abs, "personas/terse.md")), out)
            self.assertIn("would move %s -> %s" % (monet, join(backup_abs, "agents/monet.md")), out)

    def test_backup_conflict_changes_nothing(self):
        taken = join(self.backup, "agents/monet.md")
        write(taken, "an older backup\n")
        before = tree(self.tmp)
        r = self.run_script("--apply", "--backup-dir", self.backup)
        self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
        self.assertEqual(tree(self.tmp), before)
        self.assertIn("conflict: " + taken, r.stdout + r.stderr)

    def test_apply_twice_is_a_no_op(self):
        self.ok("--apply")
        after = tree(self.home)
        for args in (["--apply"], []):
            r = self.ok(*args)
            self.assertEqual(r.stdout.strip(), "nothing to do")
            self.assertEqual(tree(self.home), after)
        self.only_backup()

    def test_check_fails_while_old_copy_present(self):
        before = tree(self.home)
        r = self.run_script("--check")
        self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
        self.assertEqual(tree(self.home), before)
        named = [line[len("old copy: "):] for line in r.stdout.splitlines() if line.startswith("old copy: ")]
        self.assertEqual(sorted(named), sorted(src for src, _ in self.moves(self.backup)))
        self.ok("--apply")
        self.ok("--check")
        # One old copy back is enough to fail, and it is the only one named.
        write(join(self.claude, "commands/monet/help.md"), "help wrapper\n")
        r = self.run_script("--check")
        self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
        named = [line for line in r.stdout.splitlines() if line.startswith("old copy: ")]
        self.assertEqual(named, ["old copy: " + join(self.claude, "commands/monet")])

    def test_leaves_other_files_alone(self):
        before = tree(self.home)
        self.ok("--apply")
        after = tree(self.home)
        kept = [os.path.normpath(os.path.join(".claude", rel)) for rel in OTHERS]
        checked = 0
        for rel, old in before.items():
            if any(rel == k or rel.startswith(k + os.sep) for k in kept):
                self.assertEqual(after.get(rel), old, rel)
                checked += 1
        self.assertGreaterEqual(checked, len(OTHERS))

    def test_paths_stay_under_temp_home(self):
        data = os.path.join(self.tmp, "data")
        roots = (self.home, data, self.backup)
        dirs = ["--data-dir", data, "--backup-dir", self.backup]
        outputs = [self.run_script(*args) for args in
                   ([], dirs, ["--check"], ["--apply"] + dirs, ["--check"], ["--apply"] + dirs, ["--help"])]
        printed = 0
        for r in outputs:
            for path in re.findall(r"(?:^|\s)(/\S*)", r.stdout + r.stderr, re.M):
                printed += 1
                self.assertTrue(any(path == p or path.startswith(p + os.sep) for p in roots), path)
        self.assertGreater(printed, 0)
        for rel in tree(self.tmp):
            self.assertIn(rel.split(os.sep)[0], ("home", "data", "backup", "linked"), rel)

    def test_help_lists_exactly_the_options(self):
        # Run the script by its shebang once, so the exec bit and #! line count.
        r = self.run_script("--help", shell=None)
        self.assertEqual(r.returncode, 0, r.stderr)
        options = re.findall(r"^\s+(--[a-z-]+(?: [A-Z]+)?)", r.stdout, re.M)
        self.assertEqual(sorted(options), sorted(["--apply", "--check", "--data-dir DIR",
                                                  "--backup-dir DIR", "--help"]))
        self.assertIn("$HOME/.config/monet", r.stdout)
        self.assertIn("<data dir>/backup/dotfiles-<YYYYmmdd-HHMMSS>", r.stdout)
        # A DIR that looks like an option is a typo, not a dir named --check.
        for bad in (["--apply", "--check"], ["--nope"], ["--data-dir"], ["--apply", "--data-dir", "--check"]):
            r = self.run_script(*bad)
            self.assertEqual(r.returncode, 2, bad)
            self.assertFalse(os.path.lexists(self.data), bad)
            self.assertEqual(os.listdir(self.tmp), ["home"], bad)

    def test_script_source_rules(self):
        self.assertTrue(os.access(SCRIPT, os.X_OK))
        r = subprocess.run([BASH, "-n", SCRIPT], capture_output=True, text=True)
        self.assertEqual(r.returncode, 0, r.stderr)
        text = Path(SCRIPT).read_text(encoding="utf-8")
        self.assertIsNone(re.search(r"(?<![\w-])(rm|rmdir|unlink)(?![\w-])|-delete\b", text))
        self.assertIsNone(re.search(r"\b(mapfile|readarray)\b|\b(declare|local|typeset) -[a-zA-Z]*A", text))
        lines = re.findall(r'^MONET_SKILL_DIRS="([^"]*)"$', text, re.M)
        self.assertEqual(len(lines), 1)
        names = lines[0].split(" ")
        self.assertEqual(names, SKILLS)
        self.assertEqual(len(names), 28)
        # The script names the skill dirs on that line only (voice also names a path).
        for name in names:
            if name != "voice":
                self.assertEqual(len(re.findall(r"(?<![\w-])%s(?![\w-])" % re.escape(name), text)), 1, name)

    def test_runs_through_shebang(self):
        # The owner runs the script directly, so its #! line picks the shell for every mode.
        sources = {src: read(src) for src, _ in self.copies()}
        before = tree(self.tmp)
        r = self.run_script("--backup-dir", self.backup, shell=None)
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertEqual(tree(self.tmp), before)
        self.assertEqual(sorted(line for line in r.stdout.splitlines() if line.startswith("would ")),
                         sorted(["would copy %s -> %s" % pair for pair in self.copies()]
                                + ["would move %s -> %s" % pair for pair in self.moves(self.backup)]))
        r = self.run_script("--apply", "--backup-dir", self.backup, shell=None)
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        for src, dst in self.copies():
            self.assertEqual(read(dst), sources[src], dst)
        for src, dst in self.moves(self.backup):
            self.assertFalse(os.path.lexists(src), src)
            self.assertTrue(os.path.lexists(dst), dst)
        r = self.run_script("--check", shell=None)
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)

    def test_two_colliding_copies_change_nothing(self):
        # A persona named personas.md and the persona map are the only two copies, and they share one destination.
        ref = join(self.claude, "skills/voice/reference")
        for p in PERSONAS:
            os.remove(join(ref, "personas/" + p + ".md"))
        shutil.rmtree(os.path.join(self.home, ".config", "interview-prep"))
        persona = join(ref, "personas/personas.md")
        write(persona, "a persona\n")
        dst = join(self.data, "personas/personas.md")
        before = tree(self.tmp)
        for args in (["--apply"], []):
            r = self.run_script(*args)
            self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
            self.assertEqual(tree(self.tmp), before)
            self.assertEqual([line for line in r.stderr.splitlines() if line.startswith("conflict: ")],
                             ["conflict: %s is the destination of both %s and %s"
                              % (dst, persona, join(ref, "personas.md"))])

    def test_dangling_link_backup_dir_changes_nothing(self):
        # A backup dir that links to a disk that is not mounted cannot take any move.
        target = os.path.join(self.tmp, "unmounted", "backup")
        os.symlink(target, self.backup)
        before = tree(self.tmp)
        for args in (["--apply"], []):
            r = self.run_script(*args, "--backup-dir", self.backup)
            self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
            self.assertEqual(tree(self.tmp), before)
            self.assertFalse(os.path.lexists(os.path.dirname(target)))
            self.assertEqual(sorted(line for line in r.stderr.splitlines() if line.startswith("conflict: ")),
                             sorted("conflict: a parent of %s is not a dir, so %s cannot move there" % (dst, src)
                                    for src, dst in self.moves(self.backup)))
        # In a real backup dir, a dangling link where an old copy would go is taken too.
        os.remove(self.backup)
        taken = join(self.backup, "agents/monet.md")
        os.makedirs(os.path.dirname(taken))
        os.symlink(target, taken)
        before = tree(self.tmp)
        r = self.run_script("--apply", "--backup-dir", self.backup)
        self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
        self.assertEqual(tree(self.tmp), before)
        self.assertEqual([line for line in r.stderr.splitlines() if line.startswith("conflict: ")],
                         ["conflict: %s exists, so %s cannot move there" % (taken, join(self.claude, "agents/monet.md"))])


if __name__ == "__main__":
    unittest.main()
