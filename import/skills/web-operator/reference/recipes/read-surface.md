# Recipe: read a surface into the session

Goal: pull what's on an authed web page (a Slack thread, a dashboard, a doc,
a GitHub PR) into the conversation so the agent can reason over it. Read-only,
no gate needed.

```js
// 1. Ensure the tab group is up.
tabs_context_mcp({ createIfEmpty: true })
tabs_create_mcp()                      // → tabId

// 2. Go to the surface.
navigate({ tabId, url: "https://app.slack.com/client/TXXXX/CYYYY/thread/CYYYY-1779488049.119349" })

// 3. Did it land, or bounce to login?
get_page_text({ tabId })
//   → if the text looks like a sign-in page: since this is the human's real
//     Chrome, it's already visible: just tell them plainly to log in in
//     that tab, then continue. No relaunch, no separate profile.

// 4. Gather. Pick the lens that fits:
read_page({ tabId, filter: "interactive" })     // structured: find roles/labels/elements to act on
find({ tabId, query: "message input in the open thread panel" })  // locate one specific element
computer({ action: "screenshot", tabId })       // visual confirmation
```

Then summarize the findings in chat. Nothing here mutates state, so no
confirmation is required. If the page is paginated/lazy-loaded, `navigate` the
deep link directly, or scroll via `computer`:

```js
computer({ action: "scroll", coordinate: [760, 400], scroll_direction: "down", scroll_amount: 10, tabId })
```

or, for a full-page scroll-to-bottom:

```js
javascript_tool({ tabId, text: "window.scrollTo(0, document.body.scrollHeight), true" })
```

When you're done with the tab, close it:

```js
tabs_close_mcp({ tabId })
```
