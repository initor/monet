# Flavor: showcase

Polished, spacious, presentation-grade, for results you want to *land*: exec summaries, benchmark wins, comparison studies, launch recaps. Confident and clear, **not** salesy. Let whitespace and typography do the impressing; never reach for superlatives.

**Pick showcase when:** the intent is to present or impress: leadership summary, "here are the results", a recommendation you're selling, a polished recap. Default `data-anim="on"`.

## Tokens

```css
:root {
  --font-display: -apple-system, BlinkMacSystemFont, "SF Pro Display", "Helvetica Neue", Arial, sans-serif;
  --font-text:    -apple-system, BlinkMacSystemFont, "SF Pro Text", "Helvetica Neue", Arial, sans-serif;
  --font-mono:    "SF Mono", SFMono-Regular, ui-monospace, Menlo, monospace;

  --bg-primary:#ffffff; --bg-secondary:#f5f5f7; --bg-tertiary:#e8e8ed;
  --text-primary:#1d1d1f; --text-secondary:#6e6e73; --text-tertiary:#86868b;
  --accent:#0a84d6; --accent-hover:#0a74bd; --accent-subtle:rgba(10,132,214,.08);
  --success:#34a853; --warning:#e8910a; --danger:#e0392f;
  --border:rgba(0,0,0,.08); --card-bg:#ffffff; --card-shadow:0 2px 16px rgba(0,0,0,.06); --code-bg:#f5f5f7;

  --max-width:960px;
  --section-pad-y:80px;
  --hero-min-h:56vh;
  --hero-title-size:52px;
  --section-title-size:34px;
  --body-size:17px;
  --card-radius:16px;
  --card-pad:28px;
  --card-border:none;            /* showcase uses soft shadow, not borders */
}
[data-theme="dark"] {
  --bg-primary:#000000; --bg-secondary:#1c1c1e; --bg-tertiary:#2c2c2e;
  --text-primary:#f5f5f7; --text-secondary:#a1a1a6; --text-tertiary:#86868b;
  --accent:#2997ff; --accent-hover:#4aa9ff; --accent-subtle:rgba(41,151,255,.14);
  --success:#30d158; --warning:#ffd60a; --danger:#ff453a;
  --border:rgba(255,255,255,.10); --card-bg:#1c1c1e; --card-shadow:0 2px 16px rgba(0,0,0,.35); --code-bg:#1c1c1e;
}
```

## Flavor rules

```css
.hero { text-align: center; align-items: center; }
.hero .subtitle { margin: 0 auto; }
.section { border-top: none; }                 /* separate by space, not lines */
.section:nth-child(even) { background: var(--bg-secondary); margin: 0 -50vw; padding-left: 50vw; padding-right: 50vw; } /* full-bleed band; keep inner content in .section > * within max-width if used */
```

## Voice

- Lead each section with the single insight, stated plainly. One idea per section.
- Numbers speak for themselves: a metric card needs no adjective.
- Ban marketing words ("revolutionary", "seamless", "stunning", "crystal-clear", "game-changing"). If a sentence would fit in an ad, cut it.
