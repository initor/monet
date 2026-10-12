# Recipe: post a gated reply to a Slack thread

Goal: reply in a Slack thread you're authed to, with rich formatting (code,
bold), **without** sending until you confirm. Clipboard HTML paste into Quill
beats typed markdown and `execCommand`, but there's no dedicated paste tool
in this backend, so it's a clipboard-write + `cmd+v` workaround.

## Steps

```js
// 1. Browser up + on the thread.
tabs_context_mcp({ createIfEmpty: true })
tabs_create_mcp()                      // → tabId
navigate({ tabId, url: "https://app.slack.com/client/TXXXX/CYYYY/thread/CYYYY-1779488049.119349" })
```

```js
// 2. Stage the reply: write HTML + plain-text to the clipboard, then paste
//    into the composer. This does NOT send.
javascript_tool({
  tabId,
  text: `
    const html = "<b>Root cause:</b> a stale registration <code>app-17.example.internal_8443</code> blocked the swap. Reaped it; pod is Ready.";
    const plain = "Root cause: a stale registration app-17.example.internal_8443 blocked the swap. Reaped it; pod is Ready.";
    // Save the human's existing clipboard first (best-effort, plain text only).
    window.__webOpSavedClip = await navigator.clipboard.readText().catch(() => null);
    await navigator.clipboard.write([new ClipboardItem({
      'text/html': new Blob([html], {type: 'text/html'}),
      'text/plain': new Blob([plain], {type: 'text/plain'}),
    })]);
    'clipboard written';
  `
})

// Click the composer, then paste.
find({ tabId, query: "message input in the open thread panel" })   // → ref
computer({ action: "left_click", ref, tabId })
computer({ action: "key", text: "cmd+v", tabId })

// Verify the rich content landed (code/bold tags present).
javascript_tool({
  tabId,
  text: `
    const el = document.querySelector("[data-qa='threads_flexpane'] [data-qa='message_input'] [contenteditable='true']");
    ({ code: el.querySelectorAll('code').length, bold: el.querySelectorAll('b,strong').length, len: (el.innerText||'').length });
  `
})

// Restore the saved clipboard now that the paste has landed.
javascript_tool({
  tabId,
  text: `
    const saved = window.__webOpSavedClip;
    delete window.__webOpSavedClip;
    saved == null ? 'nothing saved to restore'
      : await navigator.clipboard.writeText(saved).then(() => 'clipboard restored', e => 'restore failed: ' + e.message);
  `
})
```

## 3. GATE: confirm before sending

Show the human exactly what is staged (the rendered reply + the target
thread) and **wait for explicit go-ahead.** Do not click send on your own
initiative: this is the outward action the skill gates on.

```js
// 4. Only after the human says go: click send, then verify a new message
//    appeared (exactly one, no duplicate).
find({ tabId, query: "send button in the open thread composer" })  // → ref
computer({ action: "left_click", ref, tabId })

get_page_text({ tabId })   // confirm the new reply is now in the thread
```

## Notes

- Slack DOM structure drifts; if `find` misses the composer or send button,
  fall back to `read_page` on the thread pane to inspect the live structure.
- @-mentions and `:emoji:` shortcodes can mangle on paste; use bare usernames /
  plain text in the content, or let the human add mentions after.
- Append your signature (e.g. ` (by :weezy: bot @<session-id>)`) to the
  staged content before pasting if that's house style: keep it in what the
  human sees before the gate, not added after.
- The clipboard save and restore is best-effort (see `reference/safety.md` G6):
  it keeps only plain text, and a restore failure is not fatal. Report a
  failed save or restore to the human rather than dropping it silently.
