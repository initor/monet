/**
 * Layers: a container image as a stepped stack of four plates, each smaller
 * than the one under it, on a plinth. Each shows what it holds on its rim:
 * the base, rows of files; the runtime, six drums; the libraries, eight
 * crates; and your code on top, a sheet of indented lines. The pointer's
 * height picks a plate. It lifts with every plate above it, staggered
 * upwards, because a change rebuilds its layer and every layer after it. At
 * rest the code plate floats a little and holds the bright edge. The read-out
 * is the step, 01 to 04. The slider is the lift.
 *
 * The pattern: one of many, picked by the pointer's height. Tweens, a stagger
 * by distance, a hit test on each plate's resting solid, and dashed guides
 * painted behind what stands in front of them.
 */
const {
  Cam, facing, fit, prism, proj, unproj, rings, rrect, extremes, seg,
  tween, tset, tval, tdone, flatDot, mk, place, pointer, put, register, disposer, solid,
} = HL;

const H = [62, 50, 38, 26], T = [10, 7, 6, 3], CR = [10, 9, 7, 5];  // half-width, thickness and corner radius, from the base up
const RG = [0, 8, 8, 16];                                           // the gap under each plate at rest: the code plate floats
const CH = [0, 5, 6, 1.2];                                          // the height of what each plate holds
const PH = 66, PT = 3, STAG = 45, LMAX = 24;                        // the plinth, the stagger in ms, the largest lift
const Z0 = RG.map((_, k) => RG.slice(0, k + 1).reduce((a, g) => a + g, 0) + T.slice(0, k).reduce((a, t) => a + t, 0));
const TOP = (k) => Z0[k] + T[k];

/** Points along the two near edges of plate k's rim, between it and the plate above: [x, y], back to front. */
function rim(k, across, step) {
  const m = (H[k] + H[k + 1]) / 2, out = [];
  for (const d of across) for (let s = -H[k + 1] + step / 2; s < H[k + 1]; s += step) out.push([m + d, s], [s, m + d]);
  return out.sort((a, b) => a[0] + a[1] - b[0] - b[1]);
}

/** What plate k holds, as a function that draws it on a top at height z. */
function content(k, g, P, C, front) {
  if (k === 0) {
    const dots = rim(0, [-2.5, 2.5], 9).map(([x, y], i) => [x, y, flatDot(g, C, 1.05, i % 7 ? "dot off" : "dot m")]);
    return (z) => dots.forEach(([x, y, el]) => place(el, P(x, y, z)));
  }
  if (k === 1) {
    const drums = rim(1, [0], 22).map(([x, y]) => [rrect(x - 4.4, y - 4.4, x + 4.4, y + 4.4, 4.4, 6), rrect(x - 3.4, y - 3.4, x + 3.4, y + 3.4, 3.4, 6), solid(g)]);
    return (z) => drums.forEach(([ring, inner, el]) => put(el, prism(P, front, ring, inner, z, z + 5)));
  }
  if (k === 2) {
    const crates = rim(2, [0], 13).map(([x, y]) => [...rings(x - 4.2, y - 4.2, x + 4.2, y + 4.2, 1.4, 0.8), solid(g)]);
    return (z) => crates.forEach(([ring, inner, el]) => put(el, prism(P, front, ring, inner, z, z + 6)));
  }
  // your code: a sheet of five indented lines
  const [ring, inner] = rings(-20, -20, 20, 20, 3, 1), el = solid(g), lines = mk("path", { class: "nf lo" }, g);
  const L = [[0, 26], [5, 20], [5, 28], [10, 14], [0, 22]];
  return (z) => {
    put(el, prism(P, front, ring, inner, z, z + 1.2));
    lines.setAttribute("d", L.map(([dx, n], i) => seg(P(-14 + dx, -12 + i * 6, z + 1.2), P(-14 + dx + n, -12 + i * 6, z + 1.2))).join(""));
  };
}

function mount({ stage, svg, read }, value) {
  const bag = disposer();
  let lift = value, over = -1;

  // The camera is fitted to the plinth and to the code sheet at the largest lift, so no pose leaves the frame.
  const C = Cam(45, 0.5, 1.42);
  const high = TOP(3) + LMAX + 2, box = [];
  for (const [x, y] of [[-PH, -PH], [PH, PH], [PH, -PH], [-PH, PH]]) box.push([x, y, -PT]);
  for (const [x, y] of [[-H[3], -H[3]], [H[3], H[3]], [H[3], -H[3]], [-H[3], H[3]]]) box.push([x, y, high]);
  fit(C, box, 200, 166);
  const P = proj(C), front = facing(C);

  const [pr, pi] = rings(-PH, -PH, PH, PH, 12, 2);
  put(solid(svg), prism(P, front, pr, pi, -PT, 0));
  const floor = mk("path", { class: "nf dash" }, svg);

  // Bottom to top, so a plate covers what is behind it. Each plate's group holds its slab, then the
  // guides down from the plate above, which stand behind its rim, then what it holds.
  const plates = Z0.map((z0, k) => {
    const g = mk("g", {}, svg), el = solid(g);
    const [ring, inner] = rings(-H[k], -H[k], H[k], H[k], CR[k], 1.6);
    const up = k < 3 ? mk("path", { class: "nf dash" }, g) : null;
    return { k, z0, ring, inner, ends: extremes(P, ring).slice(0, 2), up, el, draw: content(k, g, P, C, front), tw: tween(0), at: NaN, low: NaN };
  });
  plates.forEach((p, k) => { p.guide = k ? plates[k - 1].up : floor; });
  plates[3].el.sil.classList.add("hi");

  /** Puts plate p at its lift, and its guides down to the plate under it. A plate that has not moved keeps its paths. */
  function draw(p, off, low) {
    if (off !== p.at) {
      const z0 = p.z0 + off;
      put(p.el, prism(P, front, p.ring, p.inner, z0, z0 + T[p.k]));
      p.draw(z0 + T[p.k]);
    }
    if (off !== p.at || low !== p.low) {
      const za = p.z0 + off, zb = p.k ? TOP(p.k - 1) + low : 0;
      p.guide.setAttribute("d", za - zb < 0.5 ? "" : p.ends.map((q) => seg(P(q.u, q.v, za), P(q.u, q.v, zb))).join(""));
    }
    p.at = off;
    p.low = low;
  }

  const loop = register(stage, (_dt, now) => {
    let moving = false;
    const offs = plates.map((p) => { if (!tdone(p.tw, now)) moving = true; return tval(p.tw, now); });
    plates.forEach((p, k) => draw(p, offs[k], k ? offs[k - 1] : 0));
    return moving;
  });
  bag.add(loop.unregister);

  // The hit test reads each plate at rest, which never moves: the topmost plate whose solid, from its
  // bottom to the top of what it holds, lies under the pointer. Off the stack, the pointer's height picks.
  const tops = plates.map((p) => P(0, 0, TOP(p.k))[1]);
  const under = (k, [sx, sy]) => [TOP(k) + CH[k], Z0[k]].some((z) => unproj(C, sx, sy, z).every((w) => Math.abs(w) <= H[k]));
  const hit = (pt) => {
    for (let k = 3; k >= 0; k--) if (under(k, pt)) return k;
    return tops.reduce((best, ty, k) => (Math.abs(ty - pt[1]) < Math.abs(tops[best] - pt[1]) ? k : best), 0);
  };

  /** Lifts plate a with every plate above it; -1 is rest. The stagger spreads from a, or from the plate let go. */
  function choose(a, again) {
    if (a === over && !again) return;
    const now = performance.now(), from = a >= 0 ? a : over;
    over = a;
    plates.forEach((p, k) => {
      tset(p.tw, a >= 0 && k >= a ? lift : 0, now, Math.abs(k - from) * STAG);
      p.el.sil.classList.toggle("hi", a < 0 ? k === 3 : k === a);
    });
    read.textContent = a < 0 ? "rest" : String(a + 1).padStart(2, "0");
    loop.wake();
  }

  bag.add(pointer(stage, { move: (p) => choose(hit(p)), leave: () => choose(-1) }));
  bag.add(() => svg.replaceChildren());

  return {
    set: (v) => { lift = v; if (over >= 0) choose(over, true); },
    destroy: bag.dispose,
  };
}

hairline({
  name: "layers",
  means: "An image's layers as a stepped stack: the pointer picks one, and it lifts with every layer above, because a change rebuilds them all.",
  rules: [1, 2, 5, 6],
  range: [10, 16, 24],
  // Each part's point is where its plate shows: the near corner of its rim, and the code sheet's middle.
  parts: [0, 1, 2, 3].map((k) => (k < 3 ? [(H[k] + H[k + 1]) / 2, (H[k] + H[k + 1]) / 2, TOP(k)] : [0, 0, TOP(k)])),
  mount,
});
