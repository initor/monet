"""Tests for the handoff relay plugin hook in hooks/.

Each test runs hooks/handoff-relay.js with node, the way Claude Code runs it
after a tool use: the hook input arrives as JSON on stdin, and the relay state
goes to CLAUDE_RELAY_STATE_DIR. Fixtures live in temp dirs. The suite skips
when node is not on PATH, so `make check` still needs only python3.
"""

import json
import os
import shutil
import subprocess
import tempfile
import threading
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
HOOKS = ROOT / "hooks"
SCRIPT = HOOKS / "handoff-relay.js"
SHIM = HOOKS / "run-node.sh"
NODE = shutil.which("node")
COMMAND = '"${CLAUDE_PLUGIN_ROOT}/hooks/run-node.sh" "${CLAUDE_PLUGIN_ROOT}/hooks/handoff-relay.js"'


def assistant(tokens, tool_id=None, sidechain=False):
    """One assistant transcript line whose request used `tokens` input tokens."""
    content = [{"type": "tool_use", "id": tool_id, "name": "Bash", "input": {}}] if tool_id else []
    usage = {"input_tokens": tokens // 2, "cache_creation_input_tokens": tokens - tokens // 2 - 1,
             "cache_read_input_tokens": 1}
    return json.dumps({"type": "assistant", "isSidechain": sidechain,
                       "message": {"id": "msg", "content": content, "usage": usage}})


def tool_result(tool_id, text="ok"):
    return json.dumps({"type": "user", "message": {"role": "user", "content": [
        {"type": "tool_result", "tool_use_id": tool_id, "content": text}]}})


@unittest.skipUnless(NODE, "node is not on PATH")
class HandoffRelay(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.tmp = Path(tmp.name)
        self.project = self.tmp / "project"
        (self.project / ".claude").mkdir(parents=True)
        self.state = self.tmp / "state"
        self.state.mkdir()
        self.transcript = self.tmp / "session.jsonl"

    def config(self, **fields):
        cfg = {"threshold_pct": 80, "window_tokens": 1000, "procedure": ".planning/relay.md"}
        cfg.update(fields)
        (self.project / ".claude" / "handoff-relay.json").write_text(json.dumps(cfg), encoding="utf-8")

    def transcript_at(self, tokens):
        self.transcript.write_text("\n".join([assistant(tokens, "toolu_own"), tool_result("toolu_own")]) + "\n",
                                   encoding="utf-8")

    def hook_input(self, session="s1", **fields):
        data = {"session_id": session, "transcript_path": str(self.transcript), "cwd": str(self.project),
                "hook_event_name": "PostToolUse", "tool_use_id": "toolu_own", "permission_mode": "acceptEdits"}
        data.update(fields)
        return json.dumps({k: v for k, v in data.items() if v is not None})

    def env(self, env_project=None):
        env = {k: v for k, v in os.environ.items() if k not in ("CLAUDE_PROJECT_DIR", "CLAUDE_RELAY_STATE_DIR")}
        env["CLAUDE_RELAY_STATE_DIR"] = str(self.state)
        if env_project is not None:
            env["CLAUDE_PROJECT_DIR"] = str(env_project)
        return env

    def run_hook(self, raw=None, env_project=None, **fields):
        proc = subprocess.run([NODE, str(SCRIPT)], input=raw if raw is not None else self.hook_input(**fields),
                              capture_output=True, text=True, env=self.env(env_project), timeout=30, check=False)
        self.assertEqual(proc.returncode, 0, proc.stderr)
        self.assertEqual(proc.stderr, "")
        return proc.stdout

    def state_file(self, session="s1"):
        return self.state / f"monet-relay-{session}.json"

    def read_state(self, session="s1"):
        return json.loads(self.state_file(session).read_text(encoding="utf-8"))

    def notice(self, out):
        """The additionalContext of a hook output, checking its envelope."""
        data = json.loads(out)
        self.assertEqual(data["hookSpecificOutput"]["hookEventName"], "PostToolUse")
        self.assertRegex(data["systemMessage"], r"^Context at [0-9.]+%: handing off to a new background session\.$")
        return data["hookSpecificOutput"]["additionalContext"]

    def test_silent_without_config(self):
        self.transcript_at(900)
        self.assertEqual(self.run_hook(), "")
        self.assertFalse(self.state_file().exists())

    def test_below_threshold_records_reading_and_mode(self):
        self.config()
        self.transcript_at(500)
        self.assertEqual(self.run_hook(), "")
        state = self.read_state()
        self.assertEqual((state["fired"], state["pct"], state["tokens"], state["permissionMode"]),
                         (False, 50, 500, "acceptEdits"))

    def test_keeps_the_last_known_mode(self):
        self.config()
        self.transcript_at(500)
        self.run_hook(permission_mode="auto")
        self.run_hook(permission_mode=None)
        self.assertEqual(self.read_state()["permissionMode"], "auto")

    def test_fires_at_threshold_with_procedure(self):
        self.config()
        self.transcript_at(800)
        text = self.notice(self.run_hook())
        self.assertTrue(text.startswith("HANDOFF RELAY: this session's context is at 80% of 1000 tokens "
                                        "(relay threshold 80%)."), text)
        procedure = self.project / ".planning" / "relay.md"
        self.assertIn(f"follow {procedure} step by step.", text)
        self.assertIn("Finish the step in progress", text)
        self.assertTrue(self.read_state()["fired"])

    def test_reminds_every_fifteenth_call(self):
        self.config()
        self.transcript_at(850)
        self.assertNotIn("(reminder)", self.notice(self.run_hook()))
        for n in range(14):
            self.assertEqual(self.run_hook(), "", f"call {n + 2}")
        self.assertTrue(self.notice(self.run_hook()).startswith("HANDOFF RELAY (reminder): "))
        self.assertEqual(self.run_hook(), "")

    def test_parallel_calls_never_repeat_the_first_notice(self):
        # Each write is one rename, so no reader ever sees a half-written state and starts over.
        self.config()
        self.transcript_at(850)
        self.notice(self.run_hook())
        torn, done = [], threading.Event()

        def read_loop():
            while not done.is_set():
                try:
                    json.loads(self.state_file().read_text(encoding="utf-8"))
                except FileNotFoundError:
                    continue
                except ValueError as e:
                    torn.append(str(e))

        reader = threading.Thread(target=read_loop)
        reader.start()
        try:
            for _ in range(3):
                procs = [subprocess.Popen([NODE, str(SCRIPT)], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                          stderr=subprocess.PIPE, text=True, env=self.env()) for _ in range(15)]
                for out, err in (p.communicate(self.hook_input(), timeout=30) for p in procs):
                    self.assertEqual(err, "")
                    if out:
                        self.assertTrue(self.notice(out).startswith("HANDOFF RELAY (reminder): "), out)
        finally:
            done.set()
            reader.join()
        self.assertEqual(torn, [])
        self.assertTrue(self.read_state()["fired"])
        self.assertEqual([p.name for p in self.state.iterdir()], ["monet-relay-s1.json"])

    def test_rearms_after_usage_drops(self):
        self.config()
        self.transcript_at(900)
        self.notice(self.run_hook())
        self.transcript_at(300)
        self.assertEqual(self.run_hook(), "")
        self.assertFalse(self.read_state()["fired"])
        self.transcript_at(900)
        self.assertTrue(self.notice(self.run_hook()).startswith("HANDOFF RELAY: "))

    def test_ignores_a_tool_use_of_a_subagent(self):
        self.config()
        self.transcript_at(900)
        self.assertEqual(self.run_hook(agent_id="a1b2c3", agent_type="general-purpose"), "")
        self.assertFalse(self.state_file().exists())
        # The next tool use of the session itself still fires.
        self.notice(self.run_hook())

    def test_counts_its_own_call_before_the_transcript_holds_it(self):
        # Claude Code can run the hook before it writes the tool_use entry.
        self.config()
        self.transcript.write_text(assistant(900) + "\n", encoding="utf-8")
        self.notice(self.run_hook(tool_use_id="toolu_not_written_yet"))

    def test_muted_session_stays_silent(self):
        self.config()
        self.transcript_at(900)
        self.state_file().write_text(json.dumps({"muted": True, "note": "kept"}), encoding="utf-8")
        self.assertEqual(self.run_hook(), "")
        self.assertEqual(self.read_state(), {"muted": True, "note": "kept"})

    def test_reads_usage_from_the_main_chain_only(self):
        self.config()
        lines = [assistant(100, "toolu_own"), tool_result("toolu_own"), assistant(990, sidechain=True)]
        self.transcript.write_text("\n".join(lines) + "\n", encoding="utf-8")
        self.assertEqual(self.run_hook(), "")
        self.assertEqual(self.read_state()["tokens"], 100)

    def test_reads_usage_beyond_the_first_tail_step(self):
        # The newest usage sits 3 MB back, behind large tool results.
        self.config()
        filler = [tool_result(f"toolu_{n}", "x" * 1_000_000) for n in range(3)]
        self.transcript.write_text("\n".join([assistant(900, "toolu_own"), *filler]) + "\n", encoding="utf-8")
        self.notice(self.run_hook())
        self.assertEqual(self.read_state()["tokens"], 900)

    def test_silent_with_no_usage(self):
        self.config()
        self.transcript.write_text(tool_result("toolu_own") + "\n", encoding="utf-8")
        self.assertEqual(self.run_hook(), "")
        self.assertFalse(self.state_file().exists())

    def test_rejects_an_unsafe_session_id(self):
        # The dirs exist, so an id that got through would leave a file in one of them.
        self.config()
        self.transcript_at(900)
        for d in ("monet-relay-..", "monet-relay-a"):
            (self.state / d).mkdir()
        for session in ("../escape", "a/b", "a\\b"):
            with self.subTest(session=session):
                self.assertEqual(self.run_hook(session=session), "")
        files = [str(p.relative_to(self.state)) for p in self.state.rglob("*") if p.is_file()]
        self.assertEqual(files, [])

    def test_bad_input_is_silent(self):
        self.config()
        for raw in ("not json", "{}", json.dumps({"session_id": "s1"}), json.dumps({"session_id": 7})):
            with self.subTest(raw=raw):
                self.assertEqual(self.run_hook(raw=raw), "")

    def test_defaults(self):
        (self.project / ".claude" / "handoff-relay.json").write_text("{}", encoding="utf-8")
        self.transcript_at(800_000)
        text = self.notice(self.run_hook())
        self.assertIn("at 80% of 1000000 tokens (relay threshold 80%).", text)
        self.assertIn(str(self.project / ".planning" / "handoff-relay.md"), text)

    def test_absolute_procedure_path_stays_absolute(self):
        procedure = self.tmp / "elsewhere" / "relay.md"
        self.config(procedure=str(procedure))
        self.transcript_at(900)
        self.assertIn(f"follow {procedure} step by step.", self.notice(self.run_hook()))

    def test_project_dir_from_env_wins_over_cwd(self):
        other = self.tmp / "other"
        (other / ".claude").mkdir(parents=True)
        (other / ".claude" / "handoff-relay.json").write_text(
            json.dumps({"window_tokens": 1000, "procedure": "go.md"}), encoding="utf-8")
        self.transcript_at(900)
        self.assertIn(str(other / "go.md"), self.notice(self.run_hook(env_project=other)))


class HookManifest(unittest.TestCase):
    def test_registers_the_relay_after_every_tool_use(self):
        data = json.loads((HOOKS / "hooks.json").read_text(encoding="utf-8"))
        self.assertEqual(list(data["hooks"]), ["PostToolUse"])
        [entry] = data["hooks"]["PostToolUse"]
        self.assertEqual(entry["matcher"], "*")
        [hook] = entry["hooks"]
        self.assertEqual(hook["type"], "command")
        self.assertEqual(hook["command"], COMMAND)
        self.assertLessEqual(hook["timeout"], 10)

    def test_scripts_exist_and_the_shim_is_executable(self):
        self.assertTrue(SCRIPT.is_file())
        self.assertTrue(os.access(SHIM, os.X_OK), "hooks/run-node.sh must be executable")

    def test_shim_runs_the_node_on_path(self):
        with tempfile.TemporaryDirectory() as tmp:
            fake = Path(tmp) / "node"
            fake.write_text('#!/bin/sh\necho "fake node: $*"\n', encoding="utf-8")
            fake.chmod(0o755)
            env = dict(os.environ, PATH=f"{tmp}:/usr/bin:/bin")
            proc = subprocess.run([str(SHIM), "relay.js", "x"], capture_output=True, text=True,
                                  env=env, timeout=30, check=False)
        self.assertEqual((proc.returncode, proc.stdout), (0, "fake node: relay.js x\n"))


if __name__ == "__main__":
    unittest.main()
