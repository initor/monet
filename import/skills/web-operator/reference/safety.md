# web-operator: safety model

The whole point of this skill is that the agent operates inside *your*
authenticated browser session. That's powerful and it's exactly why the
guarantees below are non-negotiable. Don't add an automation path that
breaks one.

## G1 - User-selected authenticated browser

Claude uses Claude-in-Chrome, which requires the user's browser connection
approval. Never choose a profile, attach to a CDP port, or substitute a separate
browser silently. If login is needed, the user performs it directly.

## G2: Tab-group isolation

The agent is confined to its own MCP-tracked tab group: new tabs it opens
itself, never your regular browsing tabs. It only reuses a tab you're already
on if you explicitly ask for that. Closing tabs it's done with keeps the
group tidy.

## G3: Visible by design

Every action renders in your real browser, in real time: you can watch it
happen. This is a deliberate trade against the old design's "hidden by
default, never raise the window" model: invisibility bought nothing here once
the human wants to watch, and it cost us an entire debugging session hunting
for a window that had no reliable way to surface itself.

Visibility does not mean the agent grabs for your attention. It still never
shells out to AppleScript, `cliclick`, or `open -a` to force focus or bring a
window to the front: the extension drives pages over its own channel and
does not need OS-level input automation to do it. If a task seems to need
that, it's out of scope for this skill.

## G4: Outward actions are gated

Read-only inspection needs no mutation approval. Before any remote edit,
send, post, submission, deletion, approval, or payment, show the intended
content and target and wait for explicit approval. Autosaving document editors
write remotely as soon as you type. A composer draft can be staged only when
the user requested it and doing so does not publish shared content.

## G5 - Inspect before write, verify after write

- **Inspect**: read the target and confirm its selection with a fresh snapshot.
  Do not insert test markers into live documents. Obtain approval before the
  first write, not just before a final submit button.
- **Verify**: after a mutation, re-read/confirm the new state (e.g. the new
  Slack message appears, the PR shows "approved") so you know it landed
  exactly once. Never claim success blind.

## G6 - Clipboard hygiene (best-effort)

The following recipe applies to Claude-in-Chrome. Never assume clipboard
support from a different backend.

There's no dedicated rich-paste primitive in the Claude backend. The workaround
(`javascript_tool` clipboard write + `computer` `cmd+v`) should attempt to
save your existing clipboard before writing and restore it after, but this
is best-effort, not guaranteed, since the browser's clipboard permissions
can be flakier from a background-scripted write than from a real user
gesture. Don't treat a restore failure as fatal; just don't silently drop it
either.

## G7: Audit trail

Every call through the selected browser backend is a visible tool call in the
conversation. That transcript is the audit trail. Do not claim another backend's
tab IDs or results as evidence of actions in this session.
