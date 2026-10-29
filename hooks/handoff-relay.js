#!/usr/bin/env node
// Handoff relay: a PostToolUse hook that monet ships for projects that opt in.
// When a session's context reaches a threshold, the hook tells the session to
// hand its work to a new background session, by the procedure that the
// project names.
//
// Opt-in: <project>/.claude/handoff-relay.json, for example
//   { "threshold_pct": 80, "window_tokens": 1000000,
//     "procedure": ".planning/handoff-relay.md" }
// /monet:gang run writes this file for its coordinator.
//
// Usage is the input side of the newest main-chain request in the session's
// transcript (transcript_path): input + cache-write + cache-read tokens. A
// tool use by a subagent or a workflow agent runs this hook too, with the
// session's transcript, and its input carries an agent_id. So the hook skips
// every input that has an agent_id.
//
// Fires once when usage reaches the threshold, then again every REMIND_CALLS
// tool uses while it stays there. Each call records its reading and the
// session's permission mode in <state dir>/monet-relay-<session_id>.json,
// where the state dir is CLAUDE_RELAY_STATE_DIR, else /tmp. That file shows
// whether the hook runs, and gang's relay.mjs reads the mode from it. A state
// with "muted": true keeps the hook silent for that session, for example once
// the session has handed off. The file name differs from the one of older
// copies of this hook, so that two copies never share one state.

const fs = require('fs');
const path = require('path');

const REMIND_CALLS = 15;
const TAIL_STEPS = [2, 8, 32].map(mb => mb * 1024 * 1024);

function lastUsage(transcriptPath) {
  const fd = fs.openSync(transcriptPath, 'r');
  try {
    const size = fs.fstatSync(fd).size;
    for (const step of [...TAIL_STEPS, size]) {
      const len = Math.min(step, size);
      const buf = Buffer.alloc(len);
      fs.readSync(fd, buf, 0, len, size - len);
      const lines = buf.toString('utf8').split('\n');
      if (len < size) lines.shift(); // the first line may be cut
      for (let i = lines.length - 1; i >= 0; i--) {
        if (!lines[i]) continue;
        let e;
        try { e = JSON.parse(lines[i]); } catch (err) { continue; }
        const u = e && e.type === 'assistant' && !e.isSidechain && e.message && e.message.usage;
        if (u) {
          return (u.input_tokens || 0) + (u.cache_creation_input_tokens || 0) + (u.cache_read_input_tokens || 0);
        }
      }
      if (len === size) break;
    }
  } finally {
    fs.closeSync(fd);
  }
  return null;
}

// writeState replaces the state file with one rename, so that a parallel
// call never reads a half-written file.
function writeState(statePath, state) {
  const tmp = `${statePath}.${process.pid}.tmp`;
  fs.writeFileSync(tmp, JSON.stringify(state));
  fs.renameSync(tmp, statePath);
}

let input = '';
const stdinTimeout = setTimeout(() => process.exit(0), 10000);
process.stdin.setEncoding('utf8');
process.stdin.on('data', chunk => input += chunk);
process.stdin.on('end', () => {
  clearTimeout(stdinTimeout);
  try {
    const data = JSON.parse(input);
    const sessionId = data.session_id;
    if (!sessionId || typeof sessionId !== 'string' || /[/\\]|\.\./.test(sessionId) || !data.transcript_path) {
      process.exit(0);
    }

    const projectDir = process.env.CLAUDE_PROJECT_DIR || data.cwd || '';
    const configPath = path.join(projectDir, '.claude', 'handoff-relay.json');
    if (!projectDir || !fs.existsSync(configPath)) {
      process.exit(0);
    }
    if (typeof data.agent_id === 'string' && data.agent_id) {
      process.exit(0); // a tool use of a subagent or a workflow agent
    }
    const config = JSON.parse(fs.readFileSync(configPath, 'utf8'));
    const threshold = Number(config.threshold_pct) || 80;
    const windowTokens = Number(config.window_tokens) || 1000000;
    const procedure = config.procedure || '.planning/handoff-relay.md';

    const stateDir = process.env.CLAUDE_RELAY_STATE_DIR || '/tmp';
    const statePath = path.join(stateDir, `monet-relay-${sessionId}.json`);
    let state = {};
    try { state = JSON.parse(fs.readFileSync(statePath, 'utf8')) || {}; } catch (e) { state = {}; }
    if (state.muted) {
      process.exit(0);
    }

    const tokens = lastUsage(data.transcript_path);
    if (tokens === null) {
      process.exit(0);
    }
    const pct = Math.round((tokens / windowTokens) * 1000) / 10;
    const mode = typeof data.permission_mode === 'string' && data.permission_mode ? data.permission_mode : state.permissionMode;
    const reading = { pct, tokens, at: new Date().toISOString(), permissionMode: mode };

    if (pct < threshold) {
      // Below the threshold (again, after a compaction): stay armed.
      writeState(statePath, { fired: false, callsSinceFire: 0, ...reading });
      process.exit(0);
    }
    const calls = (Number(state.callsSinceFire) || 0) + 1;
    if (state.fired && calls < REMIND_CALLS) {
      writeState(statePath, { fired: true, callsSinceFire: calls, ...reading });
      process.exit(0);
    }
    const reminder = !!state.fired;
    writeState(statePath, { fired: true, callsSinceFire: 0, ...reading });

    const message = `HANDOFF RELAY${reminder ? ' (reminder)' : ''}: this session's context is at ${pct}% ` +
      `of ${windowTokens} tokens (relay threshold ${threshold}%). The user chose an automatic handoff ` +
      'for this project. Finish the step in progress and start no new work. Then follow ' +
      `${path.resolve(projectDir, procedure)} step by step. It hands the work to a new background ` +
      'session and ends the work in this one.';

    process.stdout.write(JSON.stringify({
      systemMessage: `Context at ${pct}%: handing off to a new background session.`,
      hookSpecificOutput: {
        hookEventName: 'PostToolUse',
        additionalContext: message
      }
    }));
  } catch (e) {
    // Fail silent: never block a tool use.
    process.exit(0);
  }
});
