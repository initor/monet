# Relay

This file is a copy at `<run>/relay/brain.md` or `<run>/relay/dev.md`, and the name of the copy gives your role. `<role>` is `brain` for the Brain and `dev` for the developer. `<run>` is two folders above the copy. `<skill>` is the folder that your kickoff named for `<skill>/reference/rules.md`, and `<session>` is your holder id. Follow the rules of that file in each step.

Start here when the relay hook, or `<run>/bin/monet-source context --role <role>`, says that the context of this session needs a relay. The relay moves this session to a new background session, which resumes from the run files. A reminder notice that comes during these steps changes nothing: go on from where you are.

Before step 1, run `<run>/bin/monet-source status`. If its `lease <role>` line does not name `<session>`, this session handed off already. Then the Brain follows "After the handoff" only, and the developer stops.

## Hand off

1. Finish the step in progress, and start no new work. Wait for each evaluator subagent in flight to return.
2. Stop each background task of this session with TaskStop, such as the Monitor. Note the command of each task other than the Monitor. End the heartbeat with ScheduleWakeup and `stop: true`.
3. Copy the newest words of the owner with `<run>/bin/monet-source relay words`. If it does not exit 0, note why under State, and go on.
4. The Brain updates the page now, while it still holds the lease. It runs `<run>/bin/monet-source report > <run>/packet.json`, and sends the reporter with that path, as `<skill>/reference/brain.md` says. Only a `wrote <time>` result leads to `<run>/bin/monet-source report done --at <time> --row <log_next> --holder <session>`. The developer skips this step.
5. Write the handoff to `<run>/handoff/<role>-<stamp>.md`, where `<stamp>` is the output of `date -u +%Y%m%dT%H%M%SZ`. Give it the sections Goal, Done, Next, Blocked, Decisions, Corrections, Gotchas, and State. Under State, list each event line that this session did not handle, and each task that step 2 stopped.
6. Write the kickoff with `<run>/bin/monet-source relay prompt --role <role> --handoff <run>/handoff/<role>-<stamp>.md --holder <session>`. It writes the kickoff beside the handoff, and it prints the command for step 8.
   - Exit 2 with "no permission mode" means that it found no mode for this session. Run it again with `--mode <mode>`, the mode of this session.
   - If it still fails, or it fails in another way, follow "If the relay fails".
7. The Brain releases its lease with `<run>/bin/monet-source lock release brain --holder <session>`, because the new Brain starts with `lock acquire brain`. The developer keeps the `dev` lease, because the new session takes it over with `dev takeover`.
8. Run the command that step 6 printed, and note the session id that it prints.
9. Every 30 seconds, for 10 minutes at most, run `<run>/bin/monet-source status` until its `lease <role>` line names another holder. If step 8 fails, auto mode blocks it, or no other holder shows up, follow "If the relay fails".
10. Run `<run>/bin/monet-source relay mute`, so that the hook gives this session no more notices.
11. The Brain tells the owner in one line: "Handed off run <slug> to <name> (<id>)." Then it follows "After the handoff". The developer stops here.

## If the relay fails

1. The Brain runs `<run>/bin/monet-source lock acquire brain --holder <session>`. On exit 4, the new session holds the lease, so go on with step 10 of "Hand off".
2. The developer runs `<run>/bin/monet-source status`. If its `lease dev` line names another holder, go on with step 10 of "Hand off".
3. If step 8 printed an id, run `claude stop <id>`, so that no stray session can take the run.
4. Run `<run>/bin/monet-source relay mute`, so that the hook gives this session no more notices.
5. The Brain writes `relay failed: <reason>` on the page, and tells the owner what failed.
6. The developer runs `<run>/bin/monet-source decision add "relay failed: <reason>" --reason "the relay failed" --holder <session>`.
7. Arm the Monitor and the heartbeat again, start again each task that step 2 stopped, and keep working. Do not try a relay again in this session.

## After the handoff

The developer skips this section. From now on, this Brain session only carries messages between the owner and the live Brain session.

- Act on no notification and no event.
- When the owner writes to this session, run `<run>/bin/monet-source relay words`. It prints the id of each new entry.
- Find the live session with `claude agents --json`: the session that step 8 started, or the newest session whose name continues the name of that session. Skip a session in between, since it handed off too.
- Send the live session this with SendMessage: "The owner wrote in <name of this session>. Read entry <id> of `<run>/owner-words.md`, and act on it." Never put the words of the owner in the message itself.
- Show the owner each message that a session of this run sends here, such as an answer or a question.
- When no live session shows up, tell the owner, and do nothing else.
