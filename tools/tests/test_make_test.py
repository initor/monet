"""Tests for the gang step that `make test` runs after `make check` (FND-8).

Each test copies the Makefile and tools/test-gang.sh into a scratch tree and
puts stub go and node scripts first on PATH, so no Go or Node toolchain runs.
"""

import os
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SKIP_LINE = "skip gang: skills/gang/cli not present\n"

# Logs one record per call: its name, its cwd, then each argument, every field
# ended by \x1f. A stub exits 1 when its call matches $STUB_FAIL_ON: "go vet"
# or "go test" for a go subcommand, or "node" for the simulator.
STUB = """\
#!/bin/sh
printf '%s\\037' "${0##*/}" "$(pwd -P)" "$@" >> "$STUB_LOG"
echo >> "$STUB_LOG"
call="${0##*/}"
if [ "$call" = go ]; then call="go $3"; fi
if [ "$call" = "${STUB_FAIL_ON-}" ]; then exit 1; fi
"""


class MakeTest(unittest.TestCase):
    def setUp(self):
        # A space and an apostrophe in the path catch shell quoting bugs.
        self.scratch = Path(tempfile.mkdtemp(prefix="owner's dir ")).resolve()
        self.addCleanup(shutil.rmtree, self.scratch)
        self.tree = self.scratch / "tree"
        (self.tree / "tools").mkdir(parents=True)
        shutil.copy(ROOT / "Makefile", self.tree / "Makefile")
        shutil.copy(ROOT / "tools" / "test-gang.sh", self.tree / "tools" / "test-gang.sh")
        self.gang = self.tree / "skills" / "gang"
        self.gang.mkdir(parents=True)
        # A file named gang at the root: without .PHONY: gang, make would call
        # the step up to date and run nothing.
        (self.tree / "gang").write_text("", encoding="utf-8")

        bin_dir = self.scratch / "bin"
        bin_dir.mkdir()
        for name in ("go", "node"):
            (bin_dir / name).write_text(STUB, encoding="utf-8")
            (bin_dir / name).chmod(0o755)
        self.log = self.scratch / "stub.log"
        drop = ("MAKEFLAGS", "GNUMAKEFLAGS", "MFLAGS", "MAKELEVEL", "MAKEFILES", "STUB_FAIL_ON")
        self.env = {k: v for k, v in os.environ.items() if k not in drop}
        self.env["PATH"] = str(bin_dir) + os.pathsep + os.environ.get("PATH", "")
        self.env["STUB_LOG"] = str(self.log)
        self.make = shutil.which("make")
        self.assertIsNotNone(self.make, "make is not on PATH")

    def run_cmd(self, argv, cwd, **env):
        if self.log.exists():
            self.log.unlink()
        return subprocess.run(
            argv, cwd=cwd, env={**self.env, **env}, capture_output=True, text=True
        )

    def calls(self):
        if not self.log.exists():
            return []
        text = self.log.read_text(encoding="utf-8")
        return [line.split("\x1f")[:-1] for line in text.splitlines()]

    def call_names(self):
        return [c[0] + " " + c[4] if c[0] == "go" else c[0] for c in self.calls()]

    def test_skips_gang_without_cli(self):
        # skills/gang/cli absent, then a regular file instead of a dir.
        for label in ("absent", "not a dir"):
            with self.subTest(cli=label):
                if label == "not a dir":
                    (self.gang / "cli").write_text("", encoding="utf-8")
                result = self.run_cmd([self.make, "gang"], self.tree)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertIn(SKIP_LINE, result.stdout)
                self.assertEqual(self.calls(), [])

    def test_script_moved_out_of_import(self):
        self.assertFalse((ROOT / "import" / "bin" / "test-gang.sh").exists())
        self.assertTrue(os.access(ROOT / "tools" / "test-gang.sh", os.X_OK))

    def test_make_test_runs_gang_after_check(self):
        # -n prints each recipe and runs none of them.
        check = self.run_cmd([self.make, "-n", "check"], self.tree)
        test = self.run_cmd([self.make, "-n", "test"], self.tree)
        self.assertEqual((check.returncode, test.returncode), (0, 0), check.stderr + test.stderr)
        self.assertNotIn("skills/gang/cli", check.stdout)
        self.assertTrue(test.stdout.startswith(check.stdout), test.stdout)
        self.assertIn("skills/gang/cli", test.stdout[len(check.stdout):])

    def test_runs_vet_test_and_sim_in_order(self):
        cli = self.gang / "cli"
        cli.mkdir()
        script = self.tree / "tools" / "test-gang.sh"
        runs = {
            "make gang": ([self.make, "gang"], self.tree),
            "script from tools/": (["./test-gang.sh"], self.tree / "tools"),
            "script from elsewhere": ([str(script)], self.scratch),
        }
        for label, (argv, cwd) in runs.items():
            with self.subTest(run=label):
                result = self.run_cmd(argv, cwd)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertNotIn(SKIP_LINE, result.stdout)
                calls = self.calls()
                self.assertEqual([c[0] for c in calls], ["go", "go", "node"], calls)
                for call, sub in zip(calls[:2], ("vet", "test")):
                    path = call[3]
                    self.assertEqual(call[2:3] + call[4:], ["-C", sub, "./..."])
                    self.assertTrue(path.endswith("/skills/gang/cli"), path)
                    resolved = (Path(call[1]) / path).resolve()
                    self.assertEqual(resolved, cli)
                    self.assertNotIn(self.tree / "tools", resolved.parents)
                node = calls[2]
                # name, cwd, sim path, --template, template path: nothing more.
                self.assertEqual(len(node), 5, node)
                self.assertEqual(node[3], "--template")
                self.assertEqual(
                    [(Path(node[1]) / node[i]).resolve() for i in (2, 4)],
                    [self.gang / "sim" / "sim.mjs", self.gang / "engine" / "engine.template.js"],
                )

    def test_go_test_failure_stops_the_run(self):
        script = (ROOT / "tools" / "test-gang.sh").read_text(encoding="utf-8")
        self.assertIn("\nset -euo pipefail\n", script)
        (self.gang / "cli").mkdir()
        result = self.run_cmd([self.make, "gang"], self.tree, STUB_FAIL_ON="go test")
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(self.call_names(), ["go vet", "go test"])

    def test_vet_or_sim_failure_fails_make_gang(self):
        (self.gang / "cli").mkdir()
        cases = {"go vet": ["go vet"], "node": ["go vet", "go test", "node"]}
        for failing, ran in cases.items():
            with self.subTest(failing=failing):
                result = self.run_cmd([self.make, "gang"], self.tree, STUB_FAIL_ON=failing)
                self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertEqual(self.call_names(), ran)


if __name__ == "__main__":
    unittest.main()
