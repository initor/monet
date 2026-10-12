/*
 * paint explainer host. It mounts one Hairline figure in #figure the way the
 * bench of hairline-create mounts one, and links it to the steps in #steps.
 * Pointing at step N points the figure at its part N. When the figure's
 * read-out says "0N", step N lights up.
 *
 * ?step=N holds the pointer on step N once the page loads, for a picture of
 * that step without a mouse, as the bench's ?at= does for a point.
 *
 * #export is for the PNG and the check. It forces the light theme, lands every
 * motion at once, and checks the page: the words against the layout's limits,
 * every link both ways, the held step, and the page's width and height. It
 * writes the problems, or "ok", on <html data-explainer>, and the page's
 * height on <html data-height>. In a frame, as in the phone check, it also
 * posts the result to the parent, which cannot read this page.
 *
 * The kernel in the script before this one is Hairline's, (c) 2026 Lucas
 * Marques, MIT: github.com/lucasmarkes/hairline.
 */
(() => {
  const root = document.documentElement;
  const exporting = location.hash === "#export";
  const asked = new URLSearchParams(location.search).get("step");
  const stage = document.getElementById("figure");
  const out = document.getElementById("readout");
  const steps = [...document.querySelectorAll("#steps > li")];
  const errors = [], setup = [];
  addEventListener("error", (e) => errors.push(String((e.error && e.error.message) || e.message)));
  for (const [what, el] of [["#figure", stage], ["#readout", out], ["#steps > li", steps[0]], ["main", document.querySelector("main")]]) {
    if (!el) setup.push(`the page has no ${what}`);
  }
  if (exporting) { root.dataset.export = "1"; root.dataset.theme = "light"; }

  // The figure's projection, caught as look.mjs catches it, so a part's world point becomes a viewBox point.
  // HL's members are getters, so the figure gets a copy of HL whose proj records what it returns.
  let P = null;
  const kernel = HL;
  HL = { ...kernel, proj: (C) => (P = kernel.proj(C)) };

  let text = null, parts = [], mounted = false;
  const lit = () => steps.map((li) => li.classList.contains("on"));
  const light = (t) => {
    const n = /^\d\d$/.test(t) ? Number(t) : 0;
    steps.forEach((li, i) => li.classList.toggle("on", i === n - 1));
  };
  const read = {
    get textContent() { return text; },
    set textContent(value) { text = value == null ? "" : String(value); if (out) out.textContent = text; light(text); },
  };

  window.hairline = (figure) => {
    if (mounted) throw new Error("explainer: one figure per page");
    if (!stage) return;
    mounted = true;
    kernel.inject(document);
    stage.setAttribute("data-hairline", figure.name);
    stage.setAttribute("role", "img");
    stage.setAttribute("aria-label", figure.means);
    const svg = kernel.mk("svg", { viewBox: "0 0 400 320", "aria-hidden": "true" }, stage);
    parts = Array.isArray(figure.parts) ? figure.parts : [];
    parts.forEach((q, i) => {
      if (!(Array.isArray(q) && q.length === 3 && q.every(Number.isFinite))) setup.push(`part ${i + 1} is not a world point [x, y, z]`);
    });
    figure.mount({ stage, svg, read }, figure.range[1]);
    // A picture needs every pose landed. The kernel's loop reads reduced motion when it starts, so this comes after mount.
    if (exporting) kernel.setReducedMotion(true);
    if (text === null) read.textContent = "rest";
  };

  // One pointermove on the stage at part i, as the bench's ?at= puts the pointer at a point.
  const point = (i) => {
    const q = parts[i];
    if (!P || !stage || !Array.isArray(q) || q.length !== 3) return false;
    const [x, y] = P(...q);
    if (!Number.isFinite(x) || !Number.isFinite(y)) return false;
    const r = stage.getBoundingClientRect();
    stage.dispatchEvent(new PointerEvent("pointermove", {
      pointerType: "mouse", pointerId: 1, bubbles: true,
      clientX: r.left + (x / 400) * r.width, clientY: r.top + (y / 320) * r.height,
    }));
    return true;
  };
  const leave = () => { if (stage) stage.dispatchEvent(new PointerEvent("pointerleave", { pointerType: "mouse", pointerId: 1 })); };

  // When the mouse leaves a step or a step loses focus: the real pointer on the figure drives it, a step
  // that takes the focus points itself, a step the keyboard still holds keeps its part, and else rest.
  const settle = (e) => {
    if (stage && stage.matches(":hover")) return;
    if (e && e.type === "blur" && steps.some((li) => li.contains(e.relatedTarget))) return;
    const held = steps.indexOf(document.activeElement);
    if (held >= 0 && steps[held].matches(":focus-visible")) point(held);
    else leave();
  };

  steps.forEach((li, i) => {
    li.tabIndex = 0;
    li.addEventListener("pointerenter", (e) => { if (e.pointerType === "mouse") point(i); });
    li.addEventListener("pointerleave", (e) => { if (e.pointerType === "mouse") settle(e); });
    // A tap has no hover: it points until the next tap, and focus does the same for the keyboard.
    li.addEventListener("click", () => point(i));
    li.addEventListener("focus", () => point(i));
    li.addEventListener("blur", settle);
  });

  addEventListener("load", async () => {
    const wait = (ms) => new Promise((done) => setTimeout(done, ms));
    await wait(100);
    const held = asked === null ? -1 : Number(asked) - 1;
    if (exporting) await check(wait, held);
    else if (held >= 0) point(held);
  });

  /** The layout's word limits, counted as reference/explainer.md states them. */
  function words(problems) {
    const count = (s) => s.trim().split(/\s+/).filter(Boolean).length;
    const sentences = (s) => s.trim().split(/(?<=[.!?])\s+/).filter(Boolean);
    const most = (what, n, cap) => { if (n > cap) problems.push(`${what} has ${n} words, ${cap} at most`); };
    const title = document.querySelector(".explainer h1");
    const summary = document.querySelector(".explainer .summary");
    const legend = document.querySelector(".explainer .legend");
    if (title) most("the title", count(title.textContent), 7);
    if (summary) {
      const ss = sentences(summary.textContent);
      if (ss.length < 2 || ss.length > 4) problems.push(`the summary has ${ss.length} sentences, 2 to 4`);
      ss.forEach((t, i) => most(`summary sentence ${i + 1}`, count(t), 12));
    }
    if (legend) {
      const n = sentences(legend.textContent).length;
      if (n < 1 || n > 2) problems.push(`the legend has ${n} sentences, 1 or 2`);
    }
    if (steps.length < 3 || steps.length > 5) problems.push(`there are ${steps.length} steps, 3 to 5`);
    steps.forEach((li, i) => {
      const b = li.querySelector("b"), d = li.querySelector(".d");
      if (b) most(`step ${i + 1}'s title`, count(b.textContent), 5);
      if (d) {
        const n = sentences(d.textContent).length;
        if (n !== 1) problems.push(`step ${i + 1}'s detail has ${n} sentences, 1`);
        most(`step ${i + 1}'s detail`, count(d.textContent), 12);
      }
    });
  }

  /** Checks the page for the export and writes the problems, or "ok", on <html data-explainer>. */
  async function check(wait, held) {
    const problems = [...setup];
    try {
      words(problems);
      if (!mounted) { if (stage) problems.push("the figure never called hairline()"); }
      else if (!P) problems.push("the figure stopped before HL.proj, so no step can point at it");
      else {
        if (parts.length !== steps.length) problems.push(`the figure declares ${parts.length} parts for ${steps.length} steps`);
        for (let i = 0; i < steps.length; i++) {
          const want = String(i + 1).padStart(2, "0");
          if (!point(i)) { problems.push(`step ${i + 1} has no part to point at`); continue; }
          await wait(50);
          if (text !== want) problems.push(`step ${i + 1} reads '${text}', not '${want}'`);
          else if (lit().some((on, j) => on !== (j === i))) problems.push(`step ${i + 1} does not light alone`);
        }
        leave();
        await wait(50);
        if (text !== "rest") problems.push(`at rest the figure reads '${text}', not 'rest'`);
        if (lit().some(Boolean)) problems.push("a step stays lit at rest");
        if (asked !== null) {
          const want = String(held + 1).padStart(2, "0");
          if (!(Number.isInteger(held) && held >= 0 && held < steps.length)) problems.push(`?step=${asked} names no step: use 1 to ${steps.length}`);
          else {
            point(held);
            await wait(50);
            if (text !== want) problems.push(`?step=${asked} does not hold step ${held + 1}`);
          }
        }
      }
      // The content's own bottom: scrollHeight never reads less than the window, so it cannot say where to crop.
      const main = document.querySelector("main");
      const bottom = main ? Math.ceil(main.getBoundingClientRect().bottom + scrollY) : 0;
      if (root.scrollWidth > root.clientWidth + 1) problems.push("the page is wider than the window");
      if (bottom > innerHeight) problems.push(`the page is ${bottom}px tall and the window ${innerHeight}px: cut words`);
      root.dataset.height = String(bottom);
    } catch (e) {
      problems.push("the check stopped: " + e.message);
    } finally {
      const all = [...new Set([...errors.map((m) => "error: " + m), ...problems])];
      root.dataset.explainer = all.join("; ") || "ok";
      if (parent !== window) parent.postMessage(root.dataset.explainer, "*");
    }
  }
})();
