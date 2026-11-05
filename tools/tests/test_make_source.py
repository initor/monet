"""Tests for the source step that `make test` runs after `make check` (PLAN-A1).

Each test copies the Makefile and tools/test-source.sh into a scratch tree and
puts a stub go first on PATH, so no Go toolchain runs.
"""

import os
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SKIP_LINE = "skip source: skills/source/cli not present\n"

# Logs one record per call: its cwd, then each argument, every field ended by
# \x1f. The stub exits 1 when "go <subcommand>" matches $STUB_FAIL_ON.
STUB = """\
#!/bin/sh
printf '%s\\037' "$(pwd -P)" "$@" >> "$STUB_LOG"
echo >> "$STUB_LOG"
if [ "go $3" = "${STUB_FAIL_ON-}" ]; then exit 1; fi
"""


class MakeSource(unittest.TestCase):
    def setUp(self):
        # A space and an apostrophe in the path catch shell quoting bugs.
        self.scratch = Path(tempfile.mkdtemp(prefix="owner's dir ")).resolve()
        self.addCleanup(shutil.rmtree, self.scratch)
        self.tree = self.scratch / "tree"
        (self.tree / "tools").mkdir(parents=True)
        shutil.copy(ROOT / "Makefile", self.tree / "Makefile")
        shutil.copy(ROOT / "tools" / "test-source.sh", self.tree / "tools" / "test-source.sh")
        self.cli = self.tree / "skills" / "source" / "cli"
        self.cli.parent.mkdir(parents=True)
        # A file named source at the root: without .PHONY: source, make would
        # call the step up to date and run nothing.
        (self.tree / "source").write_text("", encoding="utf-8")
        bin_dir = self.scratch / "bin"
        bin_dir.mkdir()
        (bin_dir / "go").write_text(STUB, encoding="utf-8")
        (bin_dir / "go").chmod(0o755)
        self.log = self.scratch / "stub.log"
        drop = ("MAKEFLAGS", "GNUMAKEFLAGS", "MFLAGS", "MAKELEVEL", "MAKEFILES", "STUB_FAIL_ON")
        self.env = {k: v for k, v in os.environ.items() if k not in drop}
        self.env["PATH"] = str(bin_dir) + os.pathsep + os.environ.get("PATH", "")
        self.env["STUB_LOG"] = str(self.log)
        self.make = shutil.which("make")
        self.assertIsNotNone(self.make, "make is not on PATH")

    def run_make(self, *argv, **env):
        return subprocess.run([self.make, *argv], cwd=self.tree, env={**self.env, **env},
                              capture_output=True, text=True)

    def calls(self):
        text = self.log.read_text(encoding="utf-8")
        return [line.split("\x1f")[:-1] for line in text.splitlines()]

    def test_skips_source_without_cli(self):
        result = self.run_make("source")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn(SKIP_LINE, result.stdout)
        self.assertFalse(self.log.exists(), "the skip ran go")

    def test_make_test_runs_source_after_check(self):
        # -n prints each recipe and runs none of them.
        check, test = self.run_make("-n", "check"), self.run_make("-n", "test")
        self.assertEqual((check.returncode, test.returncode), (0, 0), check.stderr + test.stderr)
        self.assertNotIn("tools/test-source.sh", check.stdout)
        self.assertTrue(test.stdout.startswith(check.stdout), test.stdout)
        self.assertIn("tools/test-source.sh", test.stdout[len(check.stdout):])

    def test_runs_vet_then_test_in_cli(self):
        self.cli.mkdir()
        result = self.run_make("source")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertNotIn(SKIP_LINE, result.stdout)
        calls = self.calls()
        want = [["-C", "vet", "./..."], ["-C", "test", "./..."]]
        self.assertEqual([c[1:2] + c[3:] for c in calls], want, calls)
        for cwd, _, path, *_ in calls:
            self.assertEqual((Path(cwd) / path).resolve(), self.cli)

    def test_go_test_failure_fails_make_source(self):
        self.cli.mkdir()
        result = self.run_make("source", STUB_FAIL_ON="go test")
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual([c[3] for c in self.calls()], ["vet", "test"])


if __name__ == "__main__":
    unittest.main()
