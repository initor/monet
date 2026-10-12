# Flavor: technical

Dense, utilitarian, evidence-first: an engineering-doc aesthetic for root-cause analyses, runbooks, getting-started guides, architecture explainers, and educational knowledge-sharing. Compact hero, strong monospace presence, tables and code blocks favored over decorative charts. Explains; doesn't sell.

**Pick technical when:** the content is a postmortem/RCA, a how-to or getting-started guide, a debugging writeup, a design/architecture explainer, or any teaching material. Default `data-anim="off"` (no reveal: readers scan and jump).

## Tokens

```css
:root {
  --font-display: ui-sans-serif, system-ui, "Inter", "Segoe UI", Roboto, sans-serif;
  --font-text:    ui-sans-serif, system-ui, "Inter", "Segoe UI", Roboto, sans-serif;
  --font-mono:    ui-monospace, "JetBrains Mono", SFMono-Regular, Menlo, Consolas, monospace;

  /* colors: shared showcase palette (kept consistent across all flavors) */
  --bg-primary:#ffffff; --bg-secondary:#f5f5f7; --bg-tertiary:#e8e8ed;
  --text-primary:#1d1d1f; --text-secondary:#6e6e73; --text-tertiary:#86868b;
  --accent:#0a84d6; --accent-hover:#0a74bd; --accent-subtle:rgba(10,132,214,.08);
  --success:#34a853; --warning:#e8910a; --danger:#e0392f;
  --border:rgba(0,0,0,.08); --card-bg:#ffffff; --card-shadow:none; --code-bg:#f5f5f7;

  --max-width:920px;            /* room for wide tables / code */
  --section-pad-y:44px;         /* tighter rhythm */
  --hero-min-h:auto;            /* compact: title + one-line context */
  --hero-title-size:32px;
  --section-title-size:21px;
  --body-size:16px;
  --card-radius:8px;
  --card-pad:18px;
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
.hero { min-height: auto; padding: 56px 0 20px; }
.hero h1 { font-family: var(--font-mono); font-weight: 600; letter-spacing: -.01em; }
.hero .subtitle { font-size: 1rem; }
.section { line-height: 1.45; }
.section > h2 { font-family: var(--font-mono); }
.metric .value { font-family: var(--font-mono); font-size: 1.9rem; }
.metric .label { letter-spacing: .03em; }
code, .code { font-feature-settings: "liga" 0; }
/* status-bearing callouts pull their weight here */
.callout { background: var(--bg-secondary); }
```

## Voice

- Precise and neutral. State the observation, the evidence, then the conclusion, in that order.
- Favor a table or a code block over a chart; favor a labeled flow diagram over prose when describing a sequence.
- For RCAs: lead with the root cause in one line, then the timeline/mechanism. For guides: number the steps and show the exact commands.
- No persuasion, no adjectives. The reader wants to understand or to act, not to be impressed.
