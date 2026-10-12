# Verify and ship

## The harness

Build it before the first option is judged. Designers may start while it is built. It renders an option in each real context the brief names, and it measures what the eye cannot settle: contrast at each size and theme, color separation for color-blind viewers, the smallest gap at the smallest size, and likeness to stock icons. Its output becomes the cards' flags.

## A real browser

- Headless Chrome with `--virtual-time-budget` fires no animation frames and no IntersectionObserver callbacks before the capture. Its screenshots are stills: use them for layout, never as proof of motion.
- For anything that moves or answers input, serve the folder on 127.0.0.1 and drive a real browser, either the Playwright browser tools or the owner's own. Sample the DOM over real time: the drawing changes while visible, stops offscreen, slows or pauses on input, and holds still under reduced motion.
- Check light and dark, desktop and a 375 px phone, keyboard focus, and the console. At 375 px, `document.documentElement.scrollWidth` must equal its `clientWidth`.
- Headless Chrome lays a page out no narrower than about 500 px, whatever the window size. Render the 375 px view inside a 375 px iframe.
- Give each headless Chrome a timeout and a temp profile inside the work folder, and delete the profiles when done.
- Without a real browser, use headless stills for layout only, and list motion and input as not seen.
- Say plainly what was not seen, such as touch on a real phone or other browsers.

## Integrity

- A generator builds the final from sources in the folder. Never hand-edit the built file. Keep every build in `history/`.
- Before shipping a change to something already live, diff the new build against the live copy. The diff holds only the intended changes, and nothing outside them moved.
- Credit every vendored asset and keep its license text with it.

## Ship

- Copy that ships goes through `voice`.
- Ship only on the owner's explicit go, through `/monet:ship`. Nothing in the work folder is published before that.
- After shipping, update `learnings.md`. Across sessions, `handoff` and memory carry the state.
