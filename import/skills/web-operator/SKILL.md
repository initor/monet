---
name: web-operator
description: >-
  Drive my authenticated Chrome to read or operate Slack, Google Docs, dashboards,
  and other web UIs in dedicated tabs, through Claude-in-Chrome. Visible by
  design: I can watch every action.
  Require explicit approval for remote edits, sends, submissions, and deletions.
  For visualizing findings as a page, use paint instead.
---

## Check the browser backend first

Resolve reference paths relative to this skill directory. Inspect the tools
actually available in this session; never invent a tool or assume that reading
this file installs an integration.

- **Claude Code:** use the Claude-in-Chrome steps below and read
  `reference/safety.md`. Follow the host's browser-selection confirmation before
  using a browser.
- **No backend available:** report the missing integration and its setup
  instructions. Do not silently launch another profile or take over an open tab.

Every remote mutation needs explicit approval, including an autosaved document
edit or a test marker. Never use a live write as a probe.

## Overview

Drive the human's **real, already-authenticated Chrome** via the
`mcp__claude-in-chrome__*` tools, confined to a **dedicated MCP tab group** so
it never mixes with the human's other tabs, but fully **visible**, so the
human can watch every action land in real time. This replaced an earlier
design (a separate, hidden Chrome profile driven over Playwright CDP): that
approach hit a hard Playwright/Chrome incompatibility in headed mode and left
the human unable to find the window it opened. Using the real Chrome sidesteps
both problems: no separate profile means no separate login either, since the
human is almost always already signed in to the surfaces that matter.

Call the `mcp__claude-in-chrome__*` tools directly: there is no CLI harness
to shell out to. All paths below are relative to this skill directory only for
the reference docs; the tools themselves take no path arguments.

## When to use / not

- Use when I say "drive a browser", "operate Slack/Docs/a dashboard for me",
  "take over a Chrome window", or "log in and do X on the web".
- Use when a task needs something only reachable through *my* authenticated
  browser session (internal dashboards, a Slack thread, a Google Doc, GitHub).
- **Not** for visualizing session findings as an HTML page: use `paint`.
- **Not** for pixel/computer-use control of the whole screen: this skill only
  ever drives tabs inside the dedicated MCP tab group, never the OS cursor or
  windows outside Chrome.

## Preconditions

- The Claude-in-Chrome extension must be connected. If `mcp__claude-in-chrome__*`
  tools aren't already loaded, invoke the `claude-in-chrome` skill first (or
  `ToolSearch` for them), always before the first call in a session.
- No separate profile, port, Node, or Playwright install to check: the
  extension drives the human's own Chrome directly.

## Tool quick reference

```
tabs_context_mcp {createIfEmpty}      get/create this session's MCP tab group
tabs_create_mcp                       open a fresh tab in that group
tabs_close_mcp {tabId}                close a tab you're done with
navigate {url, tabId}                 go to a URL (or back/forward)
get_page_text {tabId}                 bulk innerText of the page
read_page {tabId, filter, ref_id}     accessibility tree + element refs
find {query, tabId}                   locate an element by natural language
computer {action, tabId, ...}         click/type/scroll/screenshot/key/hover/wait
form_input {ref, value, tabId}        set a form field's value directly
file_upload / upload_image            attach a local file or a captured image
javascript_tool {text, tabId}         run JS in the page (clipboard writes, etc.)
read_console_messages / read_network_requests   debugging
browser_batch {actions}               run several of the above in one round trip
```

## Step 1: Bring up the tab group (no launch/headless/headed dance)

1. `tabs_context_mcp({createIfEmpty: true})` once per session: this
   creates/reuses the MCP-tracked tab group tied to this conversation. That
   group **is** the isolation: a distinct set of tabs, never the human's
   regular browsing tabs.
2. For a new task, prefer `tabs_create_mcp` for a fresh tab over reusing an
   existing one, unless the human explicitly asks to use a tab that's
   already open.
3. `navigate({url, tabId})` to the target surface.
4. Since this is the human's real Chrome, they are almost always **already
   logged in**. If a surface still bounces to a login page, say so plainly
   (it's already visible, no special relaunch needed) and wait for the human
   to log in in that same tab before continuing.
5. `tabs_close_mcp` each tab as soon as you're done with it; leave one open
   only if the human wants to keep watching or asked to keep it.

## Step 2: Gather

Use `get_page_text` (bulk text), `read_page` (accessibility tree + refs, for
locating elements to act on), `computer` screenshot action (visual
confirmation), `find` (natural-language element lookup instead of guessing CSS
selectors), and `read_console_messages` / `read_network_requests` for
debugging. All read-only: report findings back in chat, no confirmation
needed.

## Step 3: Act, with the outward-action gate

For mutations:

1. **Inspect before an in-place edit.** Read the target and confirm its
   selection with a snapshot before changing it. Show the intended replacement
   and obtain approval before typing into an autosaving editor. Do not insert
   test markers or other probe content into a live document.
2. **GATE every outward / irreversible action.** Before a `computer` click (or
   `type` with submit) that **sends, posts, submits, deletes, approves, or
   pays**, STOP and show the human exactly what is about to happen: the
   rendered content and the target (thread/doc/button). Proceed only on their
   explicit go-ahead. Stage only a user-requested draft on a surface where
   staging does not publish shared content. Approval is required before typing
   into an autosaving document, not just before clicking a submit button.
3. **Rich text into Slack/Quill-style editors** (no dedicated paste tool
   exists here, best-effort workaround): write HTML + plain text to the
   clipboard with `javascript_tool` (`navigator.clipboard.write` with a
   `ClipboardItem` covering `text/html` and `text/plain`), click the target
   with `computer`, then `computer({action: "key", text: "cmd+v"})` to paste.
   Verify the rich content landed by reading the target's `innerHTML` via a
   follow-up `javascript_tool` call (check for `<code>`/`<b>` tags). Try to
   save the human's existing clipboard before writing and restore it
   afterward. This is best-effort and less proven than a dedicated primitive
   would be, so don't treat a restore failure as fatal.
4. **Verify after write.** Re-`get_page_text`/`find` (or, for Slack, confirm
   the new message appears) to prove the mutation landed exactly once, no
   duplicate sends.

See `reference/recipes/slack-reply.md` and `reference/recipes/read-surface.md`
for worked end-to-end examples.

## Step 4: Hygiene & wrap-up

- Close tabs you opened for the task with `tabs_close_mcp` once you no longer
  need them; close any that remain before finishing.
- No stop-sentinel file to check: this isn't a hidden background process;
  the human can interrupt directly at any time, or close a tab themselves.
- The session's tool-call history is the audit trail: every
  `mcp__claude-in-chrome__*` call is visible in the conversation.

## Constraints

- Confined to the dedicated MCP tab group: new tabs only; never repurpose
  the human's existing tabs unless they explicitly ask.
- Still never shell out to AppleScript / `cliclick` / `open -a` to force
  focus or bring a window forward: the extension drives pages without
  needing OS-level focus tricks.
- No remote edit or outward/irreversible action without explicit per-action
  confirmation. Read-only inspection is allowed; staging is allowed only for
  requested drafts that do not publish shared content.
- Don't claim a write succeeded without a post-write verification read.
- Keep output bounded: report findings and the one-line result, not raw page
  dumps.
