# paint: shared base (skeleton, components, behavior)

This is the flavor-agnostic foundation. Inline this CSS/JS into every page, then layer the chosen flavor file (`reference/flavors/<flavor>.md`) **after** it so the flavor's token block wins. All look-and-feel comes from CSS custom properties: the base never hardcodes a color, font, or size.

## Design-token contract

Every flavor file MUST define this exact set of variables (light values in `:root`, dark overrides in `[data-theme="dark"]`). The base ships neutral defaults so it renders alone, but a flavor is expected.

```css
:root {
  /* Fonts */
  --font-display: ui-sans-serif, system-ui, sans-serif;
  --font-text:    ui-sans-serif, system-ui, sans-serif;
  --font-mono:    ui-monospace, SFMono-Regular, Menlo, monospace;

  /* Palette (light) */
  --bg-primary:#fff; --bg-secondary:#f6f6f7; --bg-tertiary:#ececee;
  --text-primary:#16161a; --text-secondary:#5a5a63; --text-tertiary:#8a8a93;
  --accent:#2b6cb0; --accent-hover:#2c5282; --accent-subtle:rgba(43,108,176,.10);
  --success:#2f855a; --warning:#b7791f; --danger:#c53030;
  --border:rgba(0,0,0,.10); --card-bg:#fff; --card-shadow:none; --code-bg:#f3f3f5;

  /* Geometry / density */
  --max-width:880px;
  --section-pad-y:64px;
  --hero-min-h:42vh;
  --hero-title-size:44px;
  --section-title-size:30px;
  --body-size:17px;
  --card-radius:12px;
  --card-pad:24px;
  --card-border:1px solid var(--border);
}
[data-theme="dark"] {
  --bg-primary:#0e0e10; --bg-secondary:#16161a; --bg-tertiary:#202024;
  --text-primary:#f3f3f5; --text-secondary:#a9a9b2; --text-tertiary:#74747c;
  --accent:#5aa0ee; --accent-hover:#7bb4f2; --accent-subtle:rgba(90,160,238,.16);
  --success:#48bb78; --warning:#ecc94b; --danger:#fc8181;
  --border:rgba(255,255,255,.10); --card-bg:#16161a; --card-shadow:none; --code-bg:#16161a;
}
```

`<html>` carries two attributes the base reads:
- `data-theme="auto|light|dark"`: theme (resolved from system on load, persisted to `localStorage`).
- `data-anim="on|off"`: scroll reveal animations (a flavor may default this off).

## HTML skeleton

```html
<!DOCTYPE html>
<html lang="en" data-theme="auto" data-anim="on">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{Page Title}</title>
  <style>/* base CSS, then flavor token block */</style>
</head>
<body>
  <button id="theme-toggle" aria-label="Toggle color theme"></button>
  <main>
    <header class="hero">…</header>
    <section class="section">…</section>
  </main>
  <script>/* theme + reveal JS below */</script>
</body>
</html>
```

## Base CSS

```css
* { box-sizing: border-box; }
html { scroll-behavior: smooth; }
body {
  margin: 0; background: var(--bg-primary); color: var(--text-primary);
  font-family: var(--font-text); font-size: var(--body-size); line-height: 1.5;
  -webkit-font-smoothing: antialiased;
  transition: background-color .3s ease, color .3s ease;
}
main { max-width: var(--max-width); margin: 0 auto; padding: 0 24px; }
h1,h2,h3 { font-family: var(--font-display); line-height: 1.12; margin: 0 0 .4em; }

.hero {
  min-height: var(--hero-min-h); display: flex; flex-direction: column;
  justify-content: center; padding: 96px 0 48px;
}
.hero h1 { font-size: var(--hero-title-size); letter-spacing: -.01em; }
.hero .subtitle { color: var(--text-secondary); font-size: 1.18rem; max-width: 60ch; }

.section { padding: var(--section-pad-y) 0; border-top: 1px solid var(--border); }
.section > h2 { font-size: var(--section-title-size); }
.section p { color: var(--text-secondary); max-width: 70ch; }

/* Cards */
.card-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px,1fr)); gap: 20px; }
.card {
  background: var(--card-bg); border: var(--card-border); box-shadow: var(--card-shadow);
  border-radius: var(--card-radius); padding: var(--card-pad);
}

/* Metric */
.metric .value { font-family: var(--font-display); font-size: 2.6rem; font-weight: 700; color: var(--accent); letter-spacing: -.02em; }
.metric .label { color: var(--text-secondary); font-size: .82rem; text-transform: uppercase; letter-spacing: .05em; }
.metric .delta.pos { color: var(--success); } .metric .delta.neg { color: var(--danger); }

/* Bar chart */
.bar { display: grid; grid-template-columns: minmax(96px,160px) 1fr; align-items: center; gap: 12px; margin: 10px 0; }
.bar .label { font-size: .9rem; }
.bar .track { background: var(--bg-tertiary); border-radius: 6px; height: 26px; overflow: hidden; }
.bar .fill { height: 100%; width: 0; background: var(--accent); border-radius: 6px; display: flex; align-items: center; justify-content: flex-end; padding-right: 8px; color: #fff; font-size: .78rem; transition: width 1s cubic-bezier(.2,.7,.2,1); }
.reveal.visible .fill { width: var(--w); }

/* Donut: conic-gradient with masked hole; legend below */
.donut { width: 180px; height: 180px; border-radius: 50%;
  -webkit-mask: radial-gradient(circle, transparent 58%, #000 58%); mask: radial-gradient(circle, transparent 58%, #000 58%); }

/* Table */
.tbl { width: 100%; border-collapse: collapse; }
.tbl th { text-align: left; padding: 10px 14px; border-bottom: 2px solid var(--border); font-weight: 600; }
.tbl td { padding: 10px 14px; border-bottom: 1px solid var(--border); }
.tbl .yes { color: var(--success); } .tbl .no { color: var(--danger); }

/* Code */
.code { background: var(--code-bg); border: 1px solid var(--border); border-radius: 10px; padding: 16px; overflow:auto;
  font-family: var(--font-mono); font-size: .86rem; line-height: 1.55; }

/* Callout (note/warn/danger via accent border) */
.callout { border-left: 3px solid var(--accent); background: var(--bg-secondary); border-radius: 0 10px 10px 0; padding: 16px 20px; margin: 20px 0; }
.callout.warn { border-color: var(--warning); } .callout.danger { border-color: var(--danger); }

/* Copy panel: verbatim copy-paste payload (e.g. a drafted review comment).
   The <pre> body is raw text, NEVER rendered markdown: suggestion fences stay
   visible as text, URLs stay plain. The copy button copies it byte-for-byte. */
.copy-panel { background: var(--card-bg); border: var(--card-border); border-radius: var(--card-radius); margin: 16px 0; overflow: hidden; }
.copy-panel .meta { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; padding: 10px 14px; border-bottom: 1px solid var(--border); background: var(--bg-secondary); font-family: var(--font-mono); font-size: .78rem; color: var(--text-secondary); }
.copy-panel .meta a { color: var(--accent); text-decoration: none; overflow-wrap: anywhere; }
.copy-panel .pill { padding: 2px 8px; border-radius: 999px; font-size: .72rem; border: 1px solid var(--border); }
.copy-panel .pill.blocker { color: var(--danger); border-color: var(--danger); }
.copy-panel .pill.suggestion { color: var(--warning); border-color: var(--warning); }
.copy-panel .pill.nit { color: var(--text-tertiary); }
.copy-panel .copy-btn { margin-left: auto; padding: 4px 10px; border-radius: 6px; border: 1px solid var(--border); background: var(--card-bg); color: var(--text-primary); font-size: .75rem; cursor: pointer; }
.copy-panel .copy-btn.ok { color: var(--success); border-color: var(--success); }
.copy-panel pre { margin: 0; padding: 14px 16px; overflow: auto; font-family: var(--font-mono); font-size: .84rem; line-height: 1.55; white-space: pre-wrap; overflow-wrap: anywhere; }

/* Explorer: a what-if over session data. Native controls follow the theme. */
html { color-scheme: light; } [data-theme="dark"] { color-scheme: dark; }
.explorer { display: grid; gap: 10px; }
.explorer label { display: grid; gap: 4px; font-size: .9rem; color: var(--text-secondary); }
.explorer input, .explorer select, .explorer output { font: inherit; color: var(--text-primary);
  background: var(--bg-secondary); border: var(--card-border); border-radius: 6px; padding: 6px 8px;
  accent-color: var(--accent); }
.explorer output { display: block; font-family: var(--font-mono); }

/* Theme toggle */
#theme-toggle { position: fixed; top: 18px; right: 18px; z-index: 1000;
  width: 38px; height: 38px; border-radius: 50%; border: 1px solid var(--border);
  background: var(--card-bg); color: var(--text-primary); cursor: pointer; display: grid; place-items: center; }

/* Scroll reveal (no-op when data-anim="off") */
.reveal { opacity: 0; transform: translateY(16px); transition: opacity .6s ease, transform .6s ease; }
.reveal.visible { opacity: 1; transform: none; }
[data-anim="off"] .reveal { opacity: 1; transform: none; transition: none; }

@media (max-width: 740px) {
  :root { --hero-title-size: 32px; --section-pad-y: 44px; }
  main { padding: 0 18px; }
}
@media (prefers-reduced-motion: reduce) { .reveal, .bar .fill, body { transition: none; } }
```

Flow diagrams: render as **inline SVG** (rounded-rect nodes via `currentColor`/theme vars, quadratic-Bezier connectors, arrow `<marker>`). Never use ASCII/box-art.

Explorer markup: labeled inputs, one `<output>`, and a short script that recomputes the output on every `input` event from data captured in the session. Never fetch anything. Put the script in its own `<script>` after the base script, inside `(() => { ... })();`, so its names cannot collide with the base globals `root`, `saved`, `SUN`, `MOON`, `btn`, and `paintIcon`.

```html
<div class="explorer">
  <label>Pod labels <input id="labels" value="app=web, tier=front"></label>
  <output id="result">matches 2 of 3 selector terms</output>
</div>
```

Copy panel markup: the `<pre>` holds the payload verbatim (HTML-escape `&` and `<` only; `textContent` restores the original, so copy stays byte-identical). Do not parse the payload into prose/code/link elements:

```html
<div class="copy-panel">
  <div class="meta">
    <a href="{permalink}">{file}:{line}</a>
    <span class="pill {severity}">{severity}</span>
    <span class="pill">{category}</span>
    <button class="copy-btn">copy</button>
  </div>
  <pre>{payload}</pre>
</div>
```

## Base JS

```javascript
// Theme: resolve auto from system, persist choice.
const root = document.documentElement;
const saved = localStorage.getItem('paint-theme');
if (saved) root.dataset.theme = saved;
if (root.dataset.theme === 'auto')
  root.dataset.theme = matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
const SUN = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"/></svg>';
const MOON = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 12.8A9 9 0 1111.2 3 7 7 0 0021 12.8z"/></svg>';
const btn = document.getElementById('theme-toggle');
const paintIcon = () => btn.innerHTML = root.dataset.theme === 'dark' ? SUN : MOON;
paintIcon();
btn.addEventListener('click', () => {
  root.dataset.theme = root.dataset.theme === 'dark' ? 'light' : 'dark';
  localStorage.setItem('paint-theme', root.dataset.theme);
  paintIcon();
});

// Copy panel: copy the <pre> body byte-for-byte (textContent, not innerHTML).
document.querySelectorAll('.copy-panel .copy-btn').forEach(btn => {
  btn.addEventListener('click', async () => {
    const text = btn.closest('.copy-panel').querySelector('pre').textContent;
    try { await navigator.clipboard.writeText(text); }
    catch { const ta = document.createElement('textarea'); ta.value = text; document.body.appendChild(ta); ta.select(); document.execCommand('copy'); ta.remove(); }
    btn.classList.add('ok'); btn.textContent = 'copied';
    setTimeout(() => { btn.classList.remove('ok'); btn.textContent = 'copy'; }, 1200);
  });
});

// Scroll reveal (skipped entirely when data-anim="off")
if (root.dataset.anim !== 'off') {
  const io = new IntersectionObserver((es) => es.forEach(e => {
    if (e.isIntersecting) { e.target.classList.add('visible'); io.unobserve(e.target); }
  }), { threshold: .12 });
  document.querySelectorAll('.reveal').forEach(el => io.observe(el));
} else {
  document.querySelectorAll('.reveal').forEach(el => el.classList.add('visible'));
}
```
