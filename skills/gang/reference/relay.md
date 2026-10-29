# Relay

Read this file when the handoff relay hook says that this session's context is at the relay threshold. `<skill>`, `<repo>`, `<dir>`, and `<gang>` are as in `SKILL.md`, and `<session>` is the lease holder from the run card. The hook ships with this plugin. It names `<dir>/run/relay.md`, the copy of this file that the coordinator makes at launch.

The relay moves the coordinator of a run to a new background session. The new session takes the driver lease and resumes from git and the run files, and the user pastes no prompt.

If this session is not the coordinator of run `<slug>`, do not follow these steps. Tell the user that the context is high, and ask what to do. An agent that sees the notice in its own context ignores it. A reminder notice that comes while this session is in these steps changes nothing: go on from where you are.

Before step 1, run `<gang> status`. If its `driver` line does not name `<session>`, this session no longer drives the run, for example because it handed off before a restart. Then follow "After the handoff" only.

## Hand off

1. Finish the step in progress. Start no new work.
2. If an engine segment runs, let it stop at a checkpoint.
   - Run `<gang> standing show`. If it has no `STOP:` line, run `<gang> standing stop "handoff relay"`. If it has one, keep it: the engine is stopping already, and that line names the next action of the new session.
   - Never stop the engine with TaskStop, since that kills agents in flight. Tasks in flight still finish their reviews, fixes, and merges, so the wait can take an hour or more.
   - Until the Workflow completion notification comes, keep the event loop going, and handle each event as "Event loop" in `<skill>/reference/coordinator.md` says, with one exception: never launch the engine again. For a `PR_RED` event, add the CI task as "Red wave PR" steps 1 to 5 say, and put its launch under Next in the handoff.
   - When the notification comes, do item 1 of the relaunch checklist, and log the stop under `## Log`. If the stop line is the "handoff relay" one, run `<gang> standing clear-stop`.
3. Stop each background task of this session with TaskStop: the Monitor and any other watcher. Note the command of each task other than the Monitor. End the heartbeat with ScheduleWakeup and `stop: true`. A gate job runs as its own process, not as a task of this session, so leave it running.
4. Copy the newest words of the user: `node <skill>/scripts/relay.mjs words --out <dir>/run/owner-words.md`. If it does not exit 0, note why under State, and go on.
5. If `<dir>/run/tracker.txt` holds a link, update the tracker page now, while this session still holds the lease, as in `<skill>/reference/tracker.md`, with the note "Handing the run to a new session". At this update, act on no comment. Wait for the line that the writer returns, and log it, before step 6.
6. Write the handoff to `<dir>/run/handoff/<stamp>.md`. `<stamp>` is the output of `date +%Y-%m-%d-%H%M`. Give it the sections Goal, Done, Next, Blocked, Decisions, Corrections, Gotchas, and State. Under State, list:
   - the slug, the last run tag and its stop reason, merge-on-green, and a stop line that step 2 kept;
   - the tracker page link, each open tracker discussion, and each tracker comment in progress;
   - each open wave PR, and each event line that the Monitor printed after the last one that this session handled;
   - each gate job that runs, and each task other than the Monitor that step 3 stopped, with its command.
7. Write the kickoff: `node <skill>/scripts/relay.mjs prompt --dir <dir> --slug <slug> --handoff <dir>/run/handoff/<stamp>.md`. Add `--merge-on-green` when the run uses it. It writes `<stamp>.prompt.txt` beside the handoff, with the standing orders and the words of the user in it, word for word, and it prints the command for step 9.
   - Exit 2 with "no permission mode" means that it found no mode for this session. Run it again with `--mode <mode>`, the mode that this session runs in.
   - A session that is not a background job has no respawn flags, so the command carries only the model and the effort. Check that it carries what the next session needs, such as `--plugin-dir`, and add what it lacks. A coordinator that needs such flags is best started as a background job, whose respawn flags carry them.
8. Release the driver lease: `<gang> lock release driver --holder <session>`. If it exits 4, another session holds the lease: stop driving, and tell the user.
9. From `<repo>`, run the command that step 7 printed. Note the session id that it prints.
10. Wait for the new session to take the lease. Every 30 seconds, for 10 minutes at most, run `<gang> status`, until its `driver` line names a holder other than `<session>`. A Monitor with an until-loop can do the wait. `claude agents --json`, run as a normal call, shows the new session and its status meanwhile.
11. If step 9 fails, auto mode blocks it, or no other holder shows up in 10 minutes, the relay failed.
    - Run `<gang> lock acquire driver --holder <session> --ttl 12h`. Exit 4 means that the new session took the lease after all, so go on with step 12.
    - If you got the lease back and step 9 printed an id, run `claude stop <id>`, so that no stray session can take the run.
    - Run `node <skill>/scripts/relay.mjs mute`, so that the hook gives this session no more notices. Write "relay failed: <reason>" on the run card and under `## Log`, and tell the user what failed.
    - Arm the Monitor and the heartbeat again, start again the other tasks that step 3 stopped, relaunch as the relaunch checklist says, and keep working. Do not try a relay again in this session.
12. Run `node <skill>/scripts/relay.mjs mute`. With TaskUpdate, set the run card and each open task of this session to completed, and say in each one that the work moved to the new session, with its name and id.
13. Tell the user in one line: "Handed off run <slug> to <name> (<id>). Open it with: claude attach <id>."

## After the handoff

From now on, this session only carries messages between the user and the live session.

- Act on no notification and no event.
- When the user writes to this session, run the `words` command of step 4. It prints the uuid of each new entry. Find the live session with `claude agents --json`: the newest running session whose name continues the name of this session. A session in between handed off too, so skip it. Send it this with SendMessage: "The user wrote in <name of this session>. Read entry <uuid> of <dir>/run/owner-words.md, and act on it." Never put the words of the user in the message itself.
- Show the user each message that a session of this run sends here, such as an answer or a question.
- When no live session shows up, tell the user, and do nothing else.
