# paint: explainer layout

An explainer teaches one concept to a newcomer, for a getting-started guide or an example. It puts
one figure in the middle and plain words around it: a title, a short summary, a legend, and 3 to 5
numbered steps. Each step is one part of the figure. Point at a step, and the figure answers there.
Point at a part of the figure, and its step lights up.

The figure is a Hairline figure: an isometric line drawing that answers the pointer. The
`hairline-create` skill draws it, on Hairline's engine and to Hairline's ten rules. Its folder is
`${CLAUDE_PLUGIN_ROOT}/skills/hairline-create`. This file adds what an explainer needs on top.

Use an explainer only when the request asks for one: `--explainer`, "explain this to a newcomer", or
"a picture explainer". For an expert who wants structure at a glance, a sheet fits better. For a
what-if the reader drives with inputs, the page layout fits better.

## Rules

| Item | Rule |
|---|---|
| Concept | One concept and one figure. Two concepts are two explainers. |
| Eyebrow | `Explainer · <area>`, such as `Explainer · Containers`. It appears on the page only. |
| Title | The question a newcomer asks, in 7 words at most. |
| Summary | 2 to 4 sentences of 12 words at most. Together they state the whole idea. |
| Legend | 1 or 2 sentences under the figure. They say what the object and its parts stand for. |
| Steps | 3 to 5. Each has a title of 5 words at most and one detail sentence of 12 words at most. Step N is part N of the figure. |
| In practice | Optional. One sentence and one code block of 10 lines at most, such as the file or the command that the steps describe. |
| Words | The STE-lite card applies (`${CLAUDE_PLUGIN_ROOT}/skills/voice/reference/ste-lite.md`), except its rule 4. The steps name the figure's parts in order, so they need not be actions. |
| Facts | Every number and fact comes from the session, such as a source read in it. Name each source in the footer. |
| Theme | The HTML follows the reader's theme. The PNG uses the light theme. |
| Motion | Only the figure moves, and only when pointed at. Set `<html data-anim="off">`. |

The check in the last section counts the title, the summary, the legend, and the steps against these
limits.

## Order of work

1. **Name.** Settle the base name once, as `{slug}-{date}`. When a `.md`, `.html` or `.png` of that
   name exists in the output directory, add `-2` or `-3`. Every file below takes the base name.
2. **Words.** Write `<base>.md` first: a `#` title, the summary, the steps, the optional
   `## In practice`, and a `Sources:` line. Write each step as `1. **Title.** Detail.` Check the file
   with the voice skill's checker, by absolute path:
   `python3 ${CLAUDE_PLUGIN_ROOT}/skills/voice/scripts/ste_check.py <file>`. Fix every line it
   reports. It checks only the card's caps, and the page check counts the rest.
3. **Figure concept.** Load the hairline-create skill with the Skill tool as
   `monet:hairline-create`, and take from it only what this file names. Read
   `${CLAUDE_PLUGIN_ROOT}/skills/hairline-create/concepts.md`. Offer two or three concepts in the
   format of Step 1 of hairline-create, and wait for the pick. With nobody to ask, take the one
   with the strongest rest pose and say which. Each concept names one part per step. Then add the
   legend for the picked concept to the words file, after the summary, and check the file again.
4. **Figure.** Follow Steps 2 and 3 of hairline-create, which step 3 loaded with the Skill tool as
   `monet:hairline-create`, in the output directory. Write `<name>.js`, then build, validate, and
   look at it. Keep the four additions under "The figure" below. Skip its Step 4, except its last
   line: in the confirmation, say what you could not verify.
5. **Page.** Write `<base>.html` from the skeleton below, with the words from the words file. Drop
   the period after each step title. Then fill its three scripts:
   `node ${CLAUDE_PLUGIN_ROOT}/skills/paint/scripts/explainer.mjs <page.html> <name>.js`. Run it
   again after any change to the figure. Copy any change to the words back into the words file.
6. **Check and export,** as the last section says. Then open the HTML, as Step 5 of paint's
   `SKILL.md` says for a page.

## The figure

The figure keeps all ten rules of `${CLAUDE_PLUGIN_ROOT}/skills/hairline-create/rules.md`. An explainer adds four.

- **One part per step.** The pointer picks one part at a time: the "one of many" or the "scrub and
  pick" answer of `concepts.md`. Step N is part N, and the parts read in step order across the
  figure, such as left to right or bottom to top.
- **The read-out is the step number.** It names the part under the pointer by its step, as two
  digits: `01` for step 1. At rest it says `rest`, as every Hairline figure does.
- **Declare the parts.** Add `parts` to the `hairline({ … })` call: one world point `[x, y, z]` per
  step, in step order. Each lies on its part where that part shows in the rest pose, so no other
  part covers it. It is the point that `look.mjs` takes as `--answer x,y,z`. A viewBox point does not
  work here. The page points the figure at it when a reader points at a step.
- **The legend decodes it.** Rule 10 keeps words out of the figure. The legend says what the
  object stands for and what the parts are, so the figure is never a riddle.

`${CLAUDE_PLUGIN_ROOT}/skills/paint/reference/explainer-layers.js` keeps all four: a stack of image layers whose plates the
pointer picks by height. Read it with the examples of `hairline-create` before drawing. The page
mounts the figure at the middle of its `range`, and it shows no slider.

## Skeleton

The explainer replaces the page's `<main>` and hero. Inline the base CSS and the flavor's token block
first, as for a page, then this CSS. The `<body>` holds the theme toggle, the markup below, the base
script, and the three script slots, in that order. Leave the slots empty: `explainer.mjs` fills them.

```html
<style>
body { margin: 0; background: var(--bg-secondary); }
.explainer { max-width: 1040px; margin: 0 auto; padding: 72px 24px 56px; display: grid; gap: 28px;
  grid-template-columns: minmax(0, 1fr); }
.explainer > header { display: grid; gap: 12px; }
.explainer .eyebrow { margin: 0; font: 12px/1.4 var(--font-mono); letter-spacing: .08em;
  text-transform: uppercase; color: var(--text-tertiary); }
.explainer h1 { margin: 0; font: 700 clamp(32px, 5vw, 52px)/1.06 var(--font-display);
  letter-spacing: -.02em; text-wrap: balance; overflow-wrap: anywhere; }
.explainer .summary { margin: 0; max-width: 62ch; font-size: clamp(17px, 2.2vw, 21px); line-height: 1.45;
  color: var(--text-secondary); text-wrap: pretty; overflow-wrap: anywhere; }
.explainer .fig { margin: 0; display: grid; gap: 14px; }
.explainer .frame { position: sticky; top: 64px; z-index: 1; aspect-ratio: 16 / 9;
  border: 1.5px solid var(--text-primary); border-radius: 18px; overflow: hidden;
  background: var(--bg-primary); --hairline-plate: var(--bg-primary); }
.explainer #figure { position: absolute; top: 0; left: 50%; height: 100%; aspect-ratio: 5 / 4;
  transform: translateX(-50%); }
.explainer #readout { position: absolute; top: 14px; right: 16px; font: 12px/1 var(--font-mono);
  color: var(--text-secondary); font-variant-numeric: tabular-nums; }
.explainer .legend { margin: 0; max-width: 70ch; font-size: 15px; color: var(--text-secondary);
  overflow-wrap: anywhere; }
.explainer #steps { list-style: none; margin: 14px 0 0; padding: 0; display: grid; gap: 12px;
  grid-template-columns: repeat(var(--n, 4), minmax(0, 1fr)); }
.explainer #steps li { padding: 16px 18px; border: 1px solid var(--border); border-radius: 14px;
  background: var(--bg-primary); overflow-wrap: anywhere; transition: border-color .26s, box-shadow .26s; }
.explainer #steps .n { font: 12px/1 var(--font-mono); color: var(--text-tertiary); }
.explainer #steps b { display: block; margin: 8px 0 4px; font: 600 17px/1.25 var(--font-display); }
.explainer #steps .d { font-size: 14px; line-height: 1.4; color: var(--text-secondary); }
.explainer #steps li.on { border-color: var(--text-primary); box-shadow: 0 0 0 .5px var(--text-primary); }
.explainer #steps li.on .n { color: var(--text-primary); }
.explainer #steps li:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
.explainer .practice h2 { margin: 0 0 6px; font: 600 20px/1.3 var(--font-display); }
.explainer .practice p { margin: 0 0 12px; color: var(--text-secondary); }
.explainer .practice .code { white-space: pre-wrap; overflow-wrap: anywhere; }
.explainer > footer { padding-top: 14px; border-top: 1px solid var(--border); font-size: 13px;
  color: var(--text-tertiary); }
.explainer > footer a { color: inherit; text-underline-offset: 2px; }
@media (max-width: 740px) {
  .explainer { padding: 56px 16px 40px; gap: 22px; }
  .explainer .frame { aspect-ratio: 5 / 4; }
  .explainer #steps { grid-template-columns: 1fr; }
}
[data-export] #theme-toggle { display: none; }
[data-export] * { transition: none !important; }
</style>

<main class="explainer">
  <header>
    <p class="eyebrow">Explainer · Area</p>
    <h1>The question a newcomer asks</h1>
    <p class="summary">Two to four short sentences that state the whole idea.</p>
  </header>
  <figure class="fig">
    <div class="frame"><div id="figure"></div><span id="readout" aria-hidden="true"></span></div>
    <figcaption class="legend">What the object and its parts stand for.</figcaption>
    <ol id="steps" style="--n: 4">
      <li><span class="n">01</span><b>Step title</b><span class="d">One detail sentence.</span></li>
      <!-- one li per step, in the figure's part order; --n is the number of steps -->
    </ol>
  </figure>
  <section class="practice">
    <!-- optional -->
    <h2>In practice</h2>
    <p>One sentence.</p>
    <pre class="code">The file or the command, 10 lines at most.</pre>
  </section>
  <footer>Sources: what the words rest on. Figure drawn on Hairline by Lucas Marques, MIT.</footer>
</main>
<!-- the base script goes here -->
<script id="hl-kernel"></script>
<script id="hl-host"></script>
<script type="module" id="hl-figure"></script>
```

The host script in the second slot mounts the figure and links it to the steps. A step lights up
with the bright stroke, the way the figure marks its part. A step takes focus from the keyboard, and
a tap points at it on a touch screen. The frame stays in view while the steps scroll under it, below
the theme toggle, so a step's answer is always on screen. The figure's colors come from Hairline's
own palette in the kernel. The CSS above only sets its plate to the frame's background, so the
plates hide what is behind them.

## Check and export

Make the PNG with the installed Chrome, in one call that also dumps the page for the check. A second
call loads the page in a 375px frame, the phone check. Use throwaway profiles, never a real one. Set
`step` to a step number for a PNG with that step held, written as `<page>-step<N>.png`. On macOS:

```bash
page="<absolute path of the page, without .html>"
step=""
chrome="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
to="$(command -v timeout || command -v gtimeout)" || echo 'timeout is missing: brew install coreutils'
prof="$(mktemp -d)"
png="$page${step:+-step$step}.png"
run() {
  "$to" -k 2 20 "$chrome" --headless=new --disable-gpu --hide-scrollbars --no-first-run \
    --no-default-browser-check --use-mock-keychain --virtual-time-budget=4000 \
    --enable-logging=stderr --log-level=0 "$@" 2>> "$prof/chrome.log"
}
run --user-data-dir="$prof/a" --window-size=1200,2400 --force-device-scale-factor=2 \
  --dump-dom --screenshot="$png" "file://$page.html${step:+?step=$step}#export" > "$prof/dom.html"
printf '<iframe src="file://%s.html#export" width="375" height="6000"></iframe><script>addEventListener("message", (e) => { document.documentElement.dataset.phone = e.data; });</script>' \
  "$page" > "$prof/phone.html"
run --user-data-dir="$prof/b" --window-size=600,900 --dump-dom "file://$prof/phone.html" > "$prof/phone.dom"
pkill -f "$prof"
tag="$(grep -o '<html[^>]*' "$prof/dom.html" | head -n 1)"
check="$(printf '%s' "$tag" | grep -o 'data-explainer="[^"]*"')"
height="$(printf '%s' "$tag" | grep -o 'data-height="[0-9]*"' | tr -dc '0-9')"
phone="$(grep -o '<html[^>]*' "$prof/phone.dom" | head -n 1 | grep -o 'data-phone="[^"]*"')"
if [ -n "$check" ]; then
  echo "$check"
  echo "phone: ${phone:-check did not run}"
  # sips ignores an offset of 0 0 and crops the middle, so the crop starts one row down.
  [ "$check" = 'data-explainer="ok"' ] && sips --cropToHeightWidth "$((height * 2))" 2400 --cropOffset 1 0 "$png" > /dev/null
else
  echo 'explainer check did not run'
  grep -h -E 'Uncaught.*source: file://|Page load failed|command not found|failed to run command|permission denied' \
    "$prof/chrome.log" || tail -n 3 "$prof/chrome.log"
fi
rm -rf "$prof"
```

Chrome does not exit by itself after it writes the PNG, so `timeout` ends each call, and each returns
exit status 124. That is normal. The checks read `data-explainer` and `data-phone` from the first
`<html` tag only, so text on the page cannot fake them. `#export` forces the light theme and lands
every motion at once. It points the figure at each step in turn and ends at rest, or on the step that
`step` holds. A clean check crops the PNG to the page's height, which `data-height` gives.

1. Read the two check lines. They must be exactly `data-explainer="ok"` and
   `phone: data-phone="ok"`. Any other value names each problem, such as a broken word limit or a
   step that does not light. Fix it in the figure or the page, fill the scripts again, and run the
   export again.
2. View the PNG with your image-reading tool. Check that the figure sits whole in its frame. No step
   card may break a word over two lines, and the legend must match the drawing.
3. Keep the words file, the PNG and `<name>.js` beside the HTML. The next fill needs `<name>.js`.
   The bench page and the pictures that `look.mjs` writes are working files: delete them.
