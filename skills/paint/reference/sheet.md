# paint: sheet layout

A sheet is one dense image that answers one question. It borrows from the engineering drawing: a
fixed canvas, a zone grid on the edges, lettered panels, and a title block. A reader takes it in at
a glance. The PNG pastes into Slack, a doc, or a pull request.

Use a sheet only when the request asks for one: `--sheet`, "one sheet", "sheet this", or "one image
I can paste". Typical sheets are a PR at a glance, an RCA at a glance, and a design on one
page. For many parts, data, or a what-if, the page layout fits better.

## Rules

| Item | Rule |
|---|---|
| Canvas | 1600 by 900 CSS pixels. The PNG is 3200 by 1800. |
| Zones | Columns 1 to 8 on the top and bottom edges, rows A to D on the sides. |
| Citations | A zone is a letter and a digit, such as C3. A panel is its letter alone, such as panel C. |
| Panels | 4 to 6, lettered A to F in reading order. Each panel answers one question with one visual. |
| Panel head | One line: the letter, a title, and a tag. A title has 6 words at most and a tag 4. In a one-column panel, the title has 30 characters at most, and the title and the tag have 36 at most together. Write heads in sentence case, not in capitals. |
| Title block | Title, subject, owner, date, and "Sheet 1 of 1", at the bottom of the corner cell. The owner is the user's name, or `git config user.name` when the session does not give one. |
| Text | No paragraphs. A label has 12 words at most. Use one term for one thing. |
| Data | Every number comes from the session, as on any paint page. |
| Color | Tokens of the chosen flavor only. Accent marks the main path, success a pass, danger a fail. |
| Motion | None. Set `<html data-anim="off">` on every sheet. |
| Theme | The PNG uses the light theme. The HTML keeps the theme toggle. |
| Fit | Each panel holds its content without clipping. Cut words before you shrink the font. |

## Panel patterns

| Pattern | Use for |
|---|---|
| Flow | a process or a request path: boxes and arrows in inline SVG |
| Tree | a hierarchy, such as an ownership chain |
| Annotated example | one line of code or text, with labeled callouts under it |
| Status table | items with a pass or fail mark and one short reason each |
| Limit gauges | values against a maximum, one bar each |
| Timeline | dated events on one axis |
| Key numbers | 2 to 4 numbers with short labels |
| Before and after | two short columns that show the change |

Base components such as `.tbl`, `.metric`, `.bar`, and `.code` work inside a panel. The skeleton
below makes them compact there. For a limit gauge, give each `.bar` a third child,
`<span class="value">`, after its track. The fill holds no text, and a value has 7 characters at
most, such as `56 / 70`.

## Grid

The grid has 4 columns and 2 rows. The class `w2` makes a panel two columns wide. The last cell of
the bottom row is the corner. The title block sits at the bottom of the corner, and one panel may sit
above it.

| Panels | Top row | Bottom row |
|---|---|---|
| 6 | A (w2), B, C | D (w2), E, corner with F over the title block |
| 5 | A (w2), B, C | D (w2), E, corner with the title block |
| 4 | A (w2), B (w2) | C (w2), D, corner with the title block |

## Skeleton

The sheet replaces the page's `<main>` and hero. Its `<body>` holds the theme toggle, the markup
below, and the scripts, and nothing else. Inline the base CSS and the flavor's token block first, as
for a page, then this CSS. Put the sheet script in its own `<script>`, after the base script.

```html
<style>
body { margin: 0; background: var(--bg-secondary); }
html:not([data-export]) .sheet-fit { margin-top: 64px; }
.sheet-fit { overflow: hidden; }
.sheet { position: relative; width: 1600px; height: 900px; padding: 26px; box-sizing: border-box;
  background: var(--bg-primary); color: var(--text-primary); font: 13px/1.4 var(--font-text);
  transform-origin: top left; }
.zones { position: absolute; display: flex; justify-content: space-around;
  font: 11px var(--font-mono); color: var(--text-tertiary); }
.zones.top { top: 6px; left: 26px; right: 26px; } .zones.bottom { bottom: 6px; left: 26px; right: 26px; }
.zones.left, .zones.right { flex-direction: column; top: 26px; bottom: 26px; }
.zones.left { left: 9px; } .zones.right { right: 9px; }
.grid { height: 100%; box-sizing: border-box; border: 1.5px solid var(--text-primary); padding: 12px;
  display: grid; grid-template-columns: repeat(4, minmax(0, 1fr));
  grid-template-rows: repeat(2, minmax(0, 1fr)); gap: 12px; }
.panel { border: 1px solid var(--border); display: flex; flex-direction: column; min-width: 0; min-height: 0; }
.panel.w2 { grid-column: span 2; }
.panel header { display: flex; align-items: center; gap: 8px; padding: 7px 10px; white-space: nowrap;
  overflow: hidden; border-bottom: 1px solid var(--border); }
.panel header b { background: var(--text-primary); color: var(--bg-primary); font: 600 12px var(--font-mono);
  padding: 2px 7px; }
.panel h2 { font-size: 15px; margin: 0; }
.panel .tag { margin-left: auto; font: 11px var(--font-mono); color: var(--text-tertiary); }
.panel .body { flex: 1; min-height: 0; overflow: hidden; overflow-wrap: break-word; padding: 10px 12px; }
.panel .body > svg { display: block; }
.panel .tbl th, .panel .tbl td { padding: 4px 8px; font-size: 12px; }
.panel .metric .value { font-size: 26px; }
.panel .code { padding: 8px 10px; font-size: 11px; white-space: pre-wrap; }
.panel .bar { grid-template-columns: minmax(64px, 120px) 1fr 48px; margin: 4px 0; }
.panel .bar .track { height: 14px; }
.panel .bar .fill { width: var(--w); padding: 0; font-size: 0; }
.panel .bar .value { font: 11px var(--font-mono); white-space: nowrap; }
.corner { display: flex; flex-direction: column; gap: 12px; min-width: 0; min-height: 0; }
.corner .panel { flex: 1; }
.title-block { flex: none; margin-top: auto; border: 1px solid var(--text-primary);
  display: grid; grid-template-columns: 1fr 1fr; }
.title-block div { border: 1px solid var(--border); padding: 5px 8px; }
.title-block div.wide { grid-column: span 2; }
.title-block small { display: block; font: 10px var(--font-mono); color: var(--text-tertiary); }
[data-export] #theme-toggle { display: none; }
[data-export] * { transition: none !important; animation: none !important; }
</style>

<div class="sheet-fit"><div class="sheet" id="sheet">
  <div class="zones top"><span>1</span><span>2</span><span>3</span><span>4</span><span>5</span><span>6</span><span>7</span><span>8</span></div>
  <div class="zones bottom"><span>1</span><span>2</span><span>3</span><span>4</span><span>5</span><span>6</span><span>7</span><span>8</span></div>
  <div class="zones left"><span>A</span><span>B</span><span>C</span><span>D</span></div>
  <div class="zones right"><span>A</span><span>B</span><span>C</span><span>D</span></div>
  <div class="grid">
    <section class="panel w2" data-panel="A">
      <header><b>A</b><h2>Panel title</h2><span class="tag">short tag</span></header>
      <div class="body"><!-- one visual --></div>
    </section>
    <!-- panels B to E, each with its own data-panel letter -->
    <div class="corner">
      <!-- optional panel F, same markup as A without w2 -->
      <div class="title-block">
        <div class="wide"><small>Title</small><b>Sheet title</b></div>
        <div><small>Subject</small>PR, RCA, or design</div><div><small>Owner</small>Name</div>
        <div><small>Date</small>YYYY-MM-DD</div><div><small>Sheet</small>1 of 1</div>
      </div>
    </div>
  </div>
</div></div>

<script>
(() => {
  // #export hides the toggle, forces light, and skips scaling.
  const html = document.documentElement;
  const sheet = document.getElementById('sheet');
  const exporting = location.hash === '#export';
  if (exporting) { html.dataset.export = '1'; html.dataset.theme = 'light'; }
  const fit = () => {
    const k = exporting ? 1 : Math.min(1, html.clientWidth / 1600);
    sheet.style.transform = 'scale(' + k + ')';
    sheet.parentElement.style.height = 900 * k + 'px';
  };
  addEventListener('resize', fit);
  fit();
  // List clipped panels on <html data-overflow>, or "none" when every panel fits.
  const over = el => el.scrollHeight > el.clientHeight + 1 || el.scrollWidth > el.clientWidth + 1;
  // A text box includes the font's ascent, so its top edge gets a quarter of one line as slack.
  const textOut = svg => [...svg.querySelectorAll('text')].some(t => {
    const r = t.getBoundingClientRect(), box = svg.getBoundingClientRect();
    const em = parseFloat(getComputedStyle(t).fontSize) * (svg.getScreenCTM() || { a: 1 }).a;
    return r.left < box.left - 1 || r.right > box.right + 1
      || r.top < box.top - Math.max(1, 0.25 * em) || r.bottom > box.bottom + 1;
  });
  const clipped = [...document.querySelectorAll('.panel')].filter(p => {
    const body = p.querySelector('.body');
    if (!body || over(body) || over(p.querySelector('header'))) return true;
    if ([...p.querySelectorAll('.bar .value')].some(over)) return true;
    return [...p.querySelectorAll('svg')].some(textOut);
  }).map(p => p.dataset.panel || (p.querySelector('header b') || {}).textContent || '?');
  // frame: the grid outgrew the canvas, grew a third row, or the canvas left the page corner.
  const rows = new Set([...sheet.querySelector('.grid').children].map(c => c.offsetTop)).size;
  const at = sheet.getBoundingClientRect();
  if (rows !== 2 || sheet.scrollWidth > 1601 || sheet.scrollHeight > 901
      || (exporting && (Math.abs(at.left) > 1 || Math.abs(at.top) > 1))) clipped.push('frame');
  html.dataset.overflow = clipped.join(',') || 'none';
})();
</script>
```

## Export and check

Write the HTML first, with Step 5 of the skill. Then make the PNG with the installed Chrome, in one
call that also dumps the page for the check. Use a throwaway profile, never a real one. On macOS:

```bash
page="<absolute path of the page, without .html>"
chrome="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
to="$(command -v timeout || command -v gtimeout)" || echo 'timeout is missing: brew install coreutils'
prof="$(mktemp -d)"
"$to" -k 2 20 "$chrome" --headless=new --disable-gpu --hide-scrollbars --no-first-run \
  --no-default-browser-check --use-mock-keychain --user-data-dir="$prof" \
  --window-size=1600,900 --force-device-scale-factor=2 --enable-logging=stderr --log-level=0 \
  --dump-dom --screenshot="$page.png" "file://$page.html#export" \
  > "$prof/dom.html" 2> "$prof/chrome.log"
pkill -f "$prof"
grep -o '<html[^>]*' "$prof/dom.html" | grep -o 'data-overflow="[^"]*"' \
  || { echo 'overflow check did not run'
       grep -h -E 'Uncaught.*source: file://|Page load failed|command not found|failed to run command|permission denied' \
         "$prof/chrome.log" || tail -n 3 "$prof/chrome.log"; }
rm -rf "$prof"
```

Chrome does not exit by itself after it writes the PNG, so `timeout` ends it, and the call returns
exit status 124. That is normal. `pkill` stops the Chrome processes that name the profile. Chrome's
crash handler, and sometimes an updater check that Chrome starts, can stay up to about 90 seconds
after the call. `pkill` cannot match them, and they exit by themselves, so leave them. Chrome
writes its log to the profile.
The command reads the check from the `<html>` tag only, so text on the sheet cannot fake it. When
the check did not run, it prints the log lines that say why: a script error, a page that did not
load, or a missing command. When none of those match, it prints the last lines of the log.

1. Read the check line. It must be exactly `data-overflow="none"`. A letter names a clipped panel,
   and `?` names a clipped panel without a letter. `frame` means the grid outgrew the canvas, grew
   a third row, or left the page corner. Any other output means the check did not run, and the
   log lines under it say why. Fix the cause, then run the export again.
2. View the PNG with your image-reading tool, not with `open`. Fix any label that wraps badly or
   crowds a neighbor. The check cannot see text clipped inside a bar track or an SVG shape. It also
   misses a word that wraps inside a narrow column. Read every value in the PNG.
3. Keep the PNG beside the HTML, with the same name.
