# Flavor: minimal

Quiet, content-first, near-monochrome. No hero splash, no shadows, hairline rules, generous whitespace, a single restrained accent from the shared palette. The content should read like a well-set document. Chrome gets out of the way.

**Pick minimal when:** the content is a plain summary, notes, a single-topic reference, or the user says "keep it simple/clean/minimal". This is also the **default when unsure**: it's the lowest-chrome, hardest-to-overdo option. Default `data-anim="off"`.

## Tokens

```css
:root {
  --font-display: ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, sans-serif;
  --font-text:    ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, sans-serif;
  --font-mono:    ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;

  /* colors: shared showcase palette (kept consistent across all flavors) */
  --bg-primary:#ffffff; --bg-secondary:#f5f5f7; --bg-tertiary:#e8e8ed;
  --text-primary:#1d1d1f; --text-secondary:#6e6e73; --text-tertiary:#86868b;
  --accent:#0a84d6; --accent-hover:#0a74bd; --accent-subtle:rgba(10,132,214,.08);
  --success:#34a853; --warning:#e8910a; --danger:#e0392f;
  --border:rgba(0,0,0,.08); --card-bg:#ffffff; --card-shadow:none; --code-bg:#f5f5f7;

  --max-width:720px;            /* narrow measure for readability */
  --section-pad-y:52px;
  --hero-min-h:auto;            /* no splash: just a title block */
  --hero-title-size:34px;
  --section-title-size:22px;
  --body-size:17px;
  --card-radius:8px;
  --card-pad:20px;
  --card-border:1px solid var(--border);
}
[data-theme="dark"] {
  --bg-primary:#000000; --bg-secondary:#1c1c1e; --bg-tertiary:#2c2c2e;
  --text-primary:#f5f5f7; --text-secondary:#a1a1a6; --text-tertiary:#86868b;
  --accent:#2997ff; --accent-hover:#4aa9ff; --accent-subtle:rgba(41,151,255,.14);
  --success:#30d158; --warning:#ffd60a; --danger:#ff453a;
  --border:rgba(255,255,255,.10); --card-bg:#1c1c1e; --card-shadow:none; --code-bg:#1c1c1e;
}
```

## Flavor rules

```css
.hero { min-height: auto; padding: 64px 0 24px; }
.hero .subtitle { font-size: 1.05rem; }
.metric .value { font-size: 2rem; }           /* understated numbers */
.section > h2 { letter-spacing: -.005em; }
```

## Voice

- Short. Say the thing, then stop. No throat-clearing intros.
- Prefer a sentence to a card and a list to a chart when either works.
- One accent color, used sparingly (links, a single emphasis). Everything else is ink on paper.
