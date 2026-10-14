// Build the README "How it works" figure from docs/assets/how-it-works.json.
//
// Usage: node tools/how-it-works.mjs [--check]
//
// With no flag it writes docs/assets/how-it-works-light.svg and
// docs/assets/how-it-works-dark.svg and prints each file's size. --check writes
// nothing: it exits 0 when both committed SVGs match what it would build, and 1
// naming the stale file otherwise.
//
// The SVGs are generated. Never hand-edit them. To rename or hide a skill in
// the figure, edit docs/assets/how-it-works.json and rebuild.
//
// The figure draws the happy path as isometric keycaps: a chip walks the rail
// from the start to each typed command, and the helpers each command runs light
// up beside it. GitHub shows a repo SVG through <img>, so there is no script and
// no web font, the motion is CSS keyframes only, and one prefers-reduced-motion
// rule leaves a still of the finished walk. Node 22 or newer, no dependencies.
import { readFileSync, writeFileSync } from 'node:fs';

const DATA = new URL('../docs/assets/how-it-works.json', import.meta.url);
const OUT = { light: new URL('../docs/assets/how-it-works-light.svg', import.meta.url), dark: new URL('../docs/assets/how-it-works-dark.svg', import.meta.url) };
const data = JSON.parse(readFileSync(DATA, 'utf8'));

// ---------- parameters (sizes are SVG user units; the figure is 566 units wide) ----------
const PARAMS = {
  type: {
    mono: 'ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, "Liberation Mono", monospace',
    sans: '-apple-system, BlinkMacSystemFont, "Segoe UI", "Noto Sans", Helvetica, Arial, sans-serif',
    cmd: 20, note: 16.5, key: 16.5, badge: 16.5, // 12.1 and 10.0 px when GitHub shrinks the figure to a 343 px phone column
    help: 19,   // helper names: about 10 px glyph height at 343 px, after the top face's 0.866 foreshortening
    start: 19.5, // start label: same floor
    monoAdv: 0.6, sansAdv: 0.5, // width estimates per character, with slack over SF Mono and Helvetica
  },
  layout: {
    sock: 32, padL: 14, padR: 8, W: 46, H: 30, gap: 48,
    tileExtra: 14, // longer command tiles, so the ran tag fits on the front face beside the note
    startW: 70, startH: 16, startFirst: 27, startLine: 25,
    helperH: 22, helperW1: 38, helperWL: 24, helperFirst: 25, tether: 16,
    bulge: 92, margin: 14, keyLH: 26, grid: 32, press: 3,
    cmdInset: 6,          // room between the leading / and the keycap's inner rim
    tagRight: 16,         // the tag sits left of the right end, clear of the chip, socket and tether
    tetherY: 0.32,        // helper tethers leave further back on the end face, away from the tag
    loopC1: [0.72, -44],  // the fix arc lifts straight up off the socket, out of the busy right end
  },
  timing: {
    fadeIn: 0.5, startHold: 0.35, hop: 0.85,
    dwellBy: { review: 0.95, ship: 1.2, teach: 0.6 }, // long enough to flash, light the helpers and let them settle before the chip leaves
    dwell: 0.7,          // a command missing from dwellBy
    dwellFindings: 0.8,  // the failing review on the loop walk
    loop: 0.95, loopPress: 0.4, loopRevisit: 0.45,
    endHold: 0.4,        // hold the finished walk for a beat before it clears
    fadeOut: 0.5, rest: 0.5,
    clearLead: 0.15, clearTail: 0.4, // trails, tags and tints clear together at the end of each walk
  },
  easing: {
    io: 'cubic-bezier(.45,0,.25,1)', in: 'cubic-bezier(.5,0,.9,.6)', out: 'cubic-bezier(.1,.4,.5,1)',
    mid: 'cubic-bezier(.35,.15,.65,.85)', // mid-path: a soft S, so the chip slows a little into rail corners
    snap: 'cubic-bezier(.2,.8,.3,1)',     // the quick rise of a flash or a helper light
  },
  badge: { ran: 'ran', fix: 'fix' },
  // Each theme is tuned on its own ground (#ffffff, #0d1117), not inverted from the other.
  themes: {
    light: {
      grid: '#c9d1db', gridOp: 0.55,
      edge: '#aeb9c7', rim: '#ffffff',
      sTop: '#f6efe3', sFront: '#e7dccb', sEnd: '#d4c6b0', sEdge: '#c2b399', sText: '#3a2f22', // warm start tile: a person, not a command
      cmd: '#151c26', note: '#394452',
      sockFill: '#d5dce6', sockRing: '#b3bfcd',
      hTop: '#ffffff', hTopOp: 0.72, hFront: '#f2f4f7', hEnd: '#e7ebf0', hStroke: '#8794a5', hName: '#2a3542', hNote: '#4a5563',
      rail: '#cbd3de', railIn: '#f4f6f9', tether: '#8d99a9',
      loop: '#7d8998', loopLabel: '#4a5563',
      acc: '#0c7c69', lit: '#e3f5f0', litOp: 0.95,
      tokTop: '#2cc2a2', tokL: '#13957c', tokR: '#0c7462', halo: '#1fb394', haloOp: 0.16,
      key: '#4a5563',
      trail: '#13a086', // the walked rail fills with this, so the path behind the chip stays marked
      doneTop: '#d9f2ea', doneOp: 0.7, ranBg: '#e3f5f0', ranStroke: '#0c7c69', ranInk: '#0a6151',
      fixTop: '#f8d27f', fixOp: 0.92, fixRim: '#c27f0c', // findings: strong amber plus a rim, never the start tile's cream
      fixBg: '#fff1d6', fixStroke: '#b7791f', fixInk: '#764c05',
      flashTop: '#a6e9d6', flashInk: '#065c4c',   // a passing command flashes green when it runs
      aFlashTop: '#f3b84a', aFlashInk: '#3f2800', // the failing review flashes amber
      hDone: '#e3f5f0', hDoneOp: 0.7,
      bezel: '#e2e8ef', kTop: '#f7f9fb', kHi: '#ffffff', kLo: '#c3ccd8', kF1: '#e6ebf1', kF2: '#cbd3dd', kE1: '#d2d9e2', kE2: '#b5bfcb',
      arrow: '#8693a4', arrowWalked: '#0a6656', // rail arrows: grey ahead of the chip, a darker trail shade once walked
    },
    dark: {
      grid: '#2a3340', gridOp: 0.75,
      edge: '#2e394a', rim: '#3a4659',
      sTop: '#30281c', sFront: '#221c14', sEnd: '#19150f', sEdge: '#5a4a33', sText: '#f1e7d6',
      cmd: '#eef2f7', note: '#a3afbe',
      sockFill: '#121822', sockRing: '#3a4659',
      hTop: '#0d1117', hTopOp: 0.6, hFront: '#111720', hEnd: '#0e131a', hStroke: '#5f6b7c', hName: '#ccd5df', hNote: '#9aa6b5',
      rail: '#2b3543', railIn: '#10161e', tether: '#5f6b7c',
      loop: '#6f7b8c', loopLabel: '#a9b4c2',
      acc: '#46dcb9', lit: '#123a33', litOp: 0.9,
      tokTop: '#7ff5d6', tokL: '#3fd0ad', tokR: '#26a587', halo: '#46dcb9', haloOp: 0.22,
      key: '#a3afbe',
      trail: '#3cc9a8',
      doneTop: '#163a34', doneOp: 0.6, ranBg: '#123a33', ranStroke: '#46dcb9', ranInk: '#9af2da',
      fixTop: '#6b4a0c', fixOp: 0.9, fixRim: '#e3a640',
      fixBg: '#3a2a0e', fixStroke: '#e0a43d', fixInk: '#f8cc7a',
      flashTop: '#1f6f5d', flashInk: '#e4fff7', // deep enough for 5.7:1 under the flashed lettering
      aFlashTop: '#8a5a0e', aFlashInk: '#fff4dd',
      hDone: '#123a33', hDoneOp: 0.55,
      bezel: '#161d28', kTop: '#222b39', kHi: '#41506a', kLo: '#0b0f15', kF1: '#1b232f', kF2: '#10151d', kE1: '#141a23', kE2: '#0a0e13',
      arrow: '#66748a', arrowWalked: '#14735f',
    },
  },
};

// ---------- isometric projection and small helpers ----------
// Floor axes: +X runs down-right, +Y runs down-left, +Z is up. True isometric (30 degrees).
const C = Math.cos(Math.PI / 6);
const iso = (x, y, z = 0) => [(x - y) * C, (x + y) * 0.5 - z];
const r1 = (n) => Math.round(n * 10) / 10;
const r2 = (n) => Math.round(n * 100) / 100;
const pt = ([a, b]) => `${r1(a)},${r1(b)}`;
const poly = (pts) => pts.map(pt).join(' ');
const pathD = (pts) => 'M' + pts.map(pt).join('L');
const plen = (pts) => pts.slice(1).reduce((s, p, i) => s + Math.hypot(p[0] - pts[i][0], p[1] - pts[i][1]), 0);
const esc = (s) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
const TY = PARAMS.type, LY = PARAMS.layout, TM = PARAMS.timing, EZ = PARAMS.easing;
const monoW = (s, f) => s.length * TY.monoAdv * f;
const sansW = (s, f) => s.length * TY.sansAdv * f;
// text lettered on a top face, or printed on a front face
const topText = (x, y, z) => { const [a, b] = iso(x, y, z); return `matrix(${r1(C * 1000) / 1000} 0.5 ${-r1(C * 1000) / 1000} 0.5 ${r1(a)} ${r1(b)})`; };
const frontText = (x, y, z) => { const [a, b] = iso(x, y, z); return `matrix(${r1(C * 1000) / 1000} 0.5 0 1 ${r1(a)} ${r1(b)})`; };
function boxFaces({ x, y, L: l, W: w, H: h }) {
  const top = [iso(x, y, h), iso(x + l, y, h), iso(x + l, y + w, h), iso(x, y + w, h)];
  const front = [iso(x, y + w, h), iso(x + l, y + w, h), iso(x + l, y + w, 0), iso(x, y + w, 0)];
  const end = [iso(x + l, y, h), iso(x + l, y + w, h), iso(x + l, y + w, 0), iso(x + l, y, 0)];
  return { top, front, end };
}
// a rounded rectangle on a horizontal plane, as floor points with their outward normal angles
function rrect(x, y, l, w, r, n = 4) {
  const cs = [[x + r, y + r, Math.PI], [x + l - r, y + r, 1.5 * Math.PI], [x + l - r, y + w - r, 0], [x + r, y + w - r, 0.5 * Math.PI]];
  const out = [];
  for (const [cx, cy, a0] of cs) for (let i = 0; i <= n; i++) { const a = a0 + (i / n) * (Math.PI / 2); out.push({ p: [cx + r * Math.cos(a), cy + r * Math.sin(a)], a }); }
  return out;
}
// the visible side walls of an extruded rounded outline: front (+Y facing) and end (+X facing) runs of edges
function walls(ol) {
  const out = { f: [], e: [] };
  for (let i = 0; i < ol.length; i++) {
    const p = ol[i], q = ol[(i + 1) % ol.length];
    const am = Math.atan2(Math.sin(p.a) + Math.sin(q.a), Math.cos(p.a) + Math.cos(q.a));
    if (Math.cos(am) + Math.sin(am) <= 0.02) continue;
    (Math.sin(am) >= Math.cos(am) ? out.f : out.e).push([p.p, q.p]);
  }
  return out;
}

// ---------- layout ----------
const F_CMD = TY.cmd, F_NOTE = TY.note, F_HELP = TY.help, F_START = TY.start, F_KEY = TY.key;
const SOCK = LY.sock, PADL = LY.padL, PADR = LY.padR, W = LY.W, H = LY.H, GAP = LY.gap;
// command tiles stack straight down the screen; their right ends share one column
const L = PADL + Math.max(...data.typed.map((t) => Math.max(monoW(t.cmd, F_CMD), sansW(t.note, F_NOTE)))) + SOCK + PADR + LY.tileExtra;
const D = W + H + GAP;
// the start label wraps to two balanced lines
function wrap2(s) {
  const w = s.split(' ');
  let best = [s, ''], bestD = Infinity;
  for (let i = 1; i < w.length; i++) {
    const a = w.slice(0, i).join(' '), b = w.slice(i).join(' ');
    const d = Math.abs(a.length - b.length);
    if (d < bestD) { bestD = d; best = [a, b]; }
  }
  return best;
}
const startLines = wrap2(data.start.label);
const L0 = PADL + Math.max(...startLines.map((s) => sansW(s, F_START))) + SOCK + PADR;
const W0 = LY.startW, H0 = LY.startH;
const tiles = [{ id: data.start.id, kind: 'start', y: -(W0 + H + GAP), L: L0, W: W0, H: H0 }];
tiles[0].x = L + tiles[0].y - L0;
data.typed.forEach((t, i) => tiles.push({ id: t.id, kind: 'typed', cmd: t.cmd, note: t.note, x: i * D, y: i * D, L, W, H }));
const byId = Object.fromEntries(tiles.map((t) => [t.id, t]));
const sock = (t) => [t.x + t.L - SOCK / 2 - PADR / 2, t.y + t.W / 2, t.H]; // where the chip docks on a top face

// helpers: low dashed tiles in a lane to the right, one per row, each tethered to its caller.
// A helper called by a command sits on its caller's row; one called by a helper sits a row below it.
const HH = LY.helperH, HWL = LY.helperWL, TA = LY.tether;
const helpers = data.helpers.map((h) => ({
  id: h.id, kind: 'helper', names: h.names, note: h.note, calledBy: h.calledBy, H: HH,
  L: 12 + Math.max(...h.names.map((n) => monoW(n, F_HELP)), sansW(h.note, F_NOTE)) + 12,
  W: LY.helperW1 + HWL * (h.names.length - 1) + 4,
}));
const hById = Object.fromEntries(helpers.map((h) => [h.id, h]));
const KH = L + 2 * TA;
for (const h of helpers) {
  const c = byId[h.calledBy];
  if (!c) continue;
  h.y = c.y - TA;
  h.x = KH + h.y;
  const ty = c.y + c.W * LY.tetherY;
  h.tether = [[c.x + c.L, ty, HH], [c.x + c.L + TA, ty, HH], [h.x, h.y + h.W / 2, HH]];
}
for (const h of helpers) {
  const c = hById[h.calledBy];
  if (!c) continue;
  h.y = c.y + D;
  h.x = KH + h.y;
  const tx = Math.min(c.x + c.L, h.x + h.L) - 18;
  h.tether = [[tx, c.y + c.W, HH / 2], [tx, h.y, HH / 2]];
}

// rails: switchbacks on the floor between sockets
const PATH = data.path;
const rails = [];
for (let i = 0; i < PATH.length - 1; i++) {
  const A = byId[PATH[i]], B = byId[PATH[i + 1]];
  const [ax] = sock(A), [bx] = sock(B);
  const yGap = A.y + A.W + 18;
  rails.push({ from: A.id, to: B.id, pts: [[ax, A.y + A.W, 0], [ax, yGap, 0], [bx, yGap, 0], [bx, B.y - 2, 0]] });
}
// the chip's waypoints for one hop: off the socket, down the front face, along the rail, onto the next socket
function hop(A, B) {
  const r = rails.find((q) => q.from === A.id && q.to === B.id);
  const [ax, ay, az] = sock(A), [bx, by, bz] = sock(B);
  return [iso(ax, ay, az), iso(ax, A.y + A.W, A.H), iso(ax, A.y + A.W, 0), iso(...r.pts[1]), iso(...r.pts[2]), iso(bx, B.y - B.H, 0), iso(bx, by, bz)];
}

// the fix-the-findings arc, from the failing command's socket back up to the start
const LA = iso(...sock(byId[data.loop.from])), LB = iso(...sock(byId[data.loop.to]));
const lc1 = [LA[0] + LY.bulge * LY.loopC1[0], LA[1] + LY.loopC1[1]], lc2 = [LB[0] + LY.bulge, LB[1] + 8];
const bez = (t, p0, p1, p2, p3) => [0, 1].map((k) => (1 - t) ** 3 * p0[k] + 3 * (1 - t) ** 2 * t * p1[k] + 3 * (1 - t) * t * t * p2[k] + t ** 3 * p3[k]);
const loopStart = bez(0.06, LA, lc1, lc2, LB), loopEnd = bez(0.9, LA, lc1, lc2, LB);
const loopPts = Array.from({ length: 9 }, (_, i) => bez(i / 8, LA, lc1, lc2, LB));
const loopApex = bez(0.5, LA, lc1, lc2, LB);
const loopD = `M${pt(loopStart)}C${pt(lc1)} ${pt(lc2)} ${pt(loopEnd)}`;
const loopLen = plen(Array.from({ length: 41 }, (_, i) => bez(i / 40, loopStart, lc1, lc2, loopEnd)));
const railLens = rails.map((r) => plen(r.pts.map((p) => iso(...p))));

// viewBox: everything drawn, plus the loop label and the key below
const all = [];
for (const t of [...tiles, ...helpers]) { const f = boxFaces(t); all.push(...f.top, ...f.front, ...f.end); }
all.push(...loopPts, [loopApex[0] + 20 + sansW(data.loop.label, F_KEY), loopApex[1]]);
const M = LY.margin, KEY_LH = LY.keyLH;
const minX = Math.min(...all.map((p) => p[0])) - M, maxX = Math.max(...all.map((p) => p[0])) + M;
const minY = Math.min(...all.map((p) => p[1])) - M;
const keyTop = Math.max(...all.map((p) => p[1])) + M + 6;
const VW = Math.ceil(maxX - minX), VH = Math.ceil(keyTop + KEY_LH * 2 + 4 - minY);
const OX = -minX, OY = -minY;

// ---------- timeline: two walks per cycle, the second takes the fix loop ----------
const sp = (id) => iso(...sock(byId[id]));
const EIO = EZ.io;
const tok = [];                                                  // [t, point, opacity, easing]
const presses = Object.fromEntries(tiles.map((t) => [t.id, []])); // [down, up]
const reveals = rails.map(() => []);                              // per rail, the chip's waypoint times on each walk
const loopReveals = [];
const doneEv = Object.fromEntries(tiles.map((t) => [t.id, []])); // [t, 'ran' | 'fix']
const flashes = Object.fromEntries(tiles.map((t) => [t.id, []]));
const fixFlashes = Object.fromEntries(tiles.map((t) => [t.id, []]));
const hLit = Object.fromEntries(helpers.map((h) => [h.id, []])); // [on, holdEnd, offEnd]
const walks = [];                                                // [start, end]
let lastTimes = null;
// move the chip through pts in dur seconds: time by segment length, ease in on the first, out on the last
function moveAlong(t0, dur, pts) {
  const segs = pts.slice(1).map((p, i) => Math.hypot(p[0] - pts[i][0], p[1] - pts[i][1]));
  const w = segs.map((s, i) => s * (segs.length === 1 ? 1 : i === 0 || i === segs.length - 1 ? 1.6 : 1));
  const tot = w.reduce((a, b) => a + b, 0);
  let t = t0;
  lastTimes = [];
  pts.forEach((p, i) => {
    const e = i === segs.length ? EIO : segs.length === 1 ? EIO : i === 0 ? EZ.in : i === segs.length - 1 ? EZ.out : EZ.mid;
    tok.push([t, p, 1, e]);
    lastTimes.push([t, e]);
    if (i < segs.length) t += (dur * w[i]) / tot;
  });
  return t0 + dur;
}
function walk(t0, withLoop) {
  let t = t0;
  const revealed = new Set();
  tok.push([t, sp(PATH[0]), 0, EIO]);
  t += TM.fadeIn; tok.push([t, sp(PATH[0]), 1, EIO]);
  t += TM.startHold;
  let findings = withLoop;
  // a command runs: it presses, flashes (amber when it fails), lights its helpers, and shows ran or fix when it finishes
  const visit = (i, dw) => {
    const id = PATH[i];
    presses[id].push([t, t + (dw + 0.05)]);
    const isFix = findings && id === data.loop.from;
    (isFix ? fixFlashes[id] : flashes[id]).push(t);
    doneEv[id].push([t + dw - 0.05, isFix ? 'fix' : 'ran']);
    for (const h of helpers) if (h.calledBy === id) {
      hLit[h.id].push([t + 0.2, t + dw - 0.4, t + dw - 0.1]);
      for (const g of helpers) if (g.calledBy === h.id) hLit[g.id].push([t + 0.45, t + dw - 0.35, t + dw - 0.05]);
    }
  };
  const go = (i, dur) => {
    if (i === 0 && !doneEv[PATH[0]].some(([u]) => u >= t0)) doneEv[PATH[0]].push([t, 'ran']);
    t = moveAlong(t, dur, hop(byId[PATH[i]], byId[PATH[i + 1]]));
    if (!revealed.has(i)) { revealed.add(i); reveals[i].push(lastTimes); }
    const dw = findings && PATH[i + 1] === data.loop.from ? TM.dwellFindings : (TM.dwellBy[PATH[i + 1]] ?? TM.dwell);
    visit(i + 1, dw);
    t += dw;
  };
  go(0, TM.hop);
  if (withLoop) {
    findings = false;
    t = moveAlong(t, TM.loop, loopPts);
    loopReveals.push(lastTimes);
    presses[PATH[0]].push([t, t + TM.loopPress]);
    t += TM.loopRevisit;
    go(0, TM.hop);
  }
  for (let i = 1; i < PATH.length - 1; i++) go(i, TM.hop);
  t += TM.endHold;
  tok.push([t, sp(PATH[PATH.length - 1]), 1, EIO]);
  tok.push([t + TM.fadeOut, sp(PATH[PATH.length - 1]), 0, EIO]);
  walks.push([t0, t + TM.fadeOut]);
  return t + TM.fadeOut;
}
const endA = walk(0, false);
const endB = walk(endA + TM.rest, true);
const T = Math.round((endB + TM.rest) * 10) / 10;

// keyframes
const pct = (t) => `${+((t / T) * 100).toFixed(2)}%`;
function frames(name, list) {
  const out = [];
  if (list[0][0] > 0) out.push(`0%{${list[0][1]}}`);
  for (const [t, css, e] of list) out.push(`${pct(t)}{${css}${e ? `;animation-timing-function:${e}` : ''}}`);
  if (list[list.length - 1][0] < T) out.push(`100%{${list[list.length - 1][1]}}`);
  return `@keyframes ${name}{${out.join('')}}`;
}
// sort, and keep only the later of two events that land on the same offset
function tidy(list) {
  const out = [];
  for (const e of [...list].sort((a, b) => a[0] - b[0])) {
    if (out.length && pct(out[out.length - 1][0]) === pct(e[0])) out[out.length - 1] = e; else out.push(e);
  }
  return out;
}
// an opacity from 0 to 1 and back over the cycle, from ramps [t0, t1, to, easing]
function levelTrack(ramps) {
  const ev = [[0, 'opacity:0', EIO]];
  let cur = 0;
  for (const [a, b, to, e] of [...ramps].sort((p, q) => p[0] - q[0])) {
    ev.push([a, `opacity:${cur}`, e || EIO], [b, `opacity:${to}`, EIO]);
    cur = to;
  }
  return tidy(ev);
}
const clearOf = (t) => walks.find(([a, b]) => t >= a && t <= b + TM.rest)[1];
const clr = (end) => [end - TM.clearLead, end + TM.clearTail];
// a trail: stroke-dashoffset revealed in step with the chip, held, then cleared at the end of the walk
function trailTrack(len, passes, offsetsAt) {
  const H2 = r1(len + 2);
  const ev = [[0, `stroke-dashoffset:${H2};opacity:1`, EIO]];
  for (const times of passes) {
    offsetsAt(times).forEach(([t, off, e]) => ev.push([t, `stroke-dashoffset:${r1(off)};opacity:1`, e]));
    const [c0, c1] = clr(clearOf(times[0][0]));
    ev.push([c0, 'stroke-dashoffset:0;opacity:1', EIO], [c1, 'stroke-dashoffset:0;opacity:0', EIO], [c1 + 0.03, `stroke-dashoffset:${H2};opacity:0`, EIO], [c1 + 0.06, `stroke-dashoffset:${H2};opacity:1`, EIO]);
  }
  return tidy(ev);
}
// rail arrows: one on each cross run and one on each final run, with the chip's waypoint pair they sit between
const arrows = [];
rails.forEach((r, i) => {
  const [, p1, p2] = r.pts;
  const B = byId[r.to];
  const dir = Math.sign(p2[0] - p1[0]) || 1;
  arrows.push({ i, p: [(p1[0] + p2[0]) / 2, p1[1]], u: [dir, 0], s0: 3, s1: 4, cls: `aw${i}0` });
  arrows.push({ i, p: [p2[0], (p2[1] + (B.y - B.H)) / 2], u: [0, 1], s0: 4, s1: 5, cls: `aw${i}1` });
});

// ---------- SVG ----------
function build(theme) {
  const P = PARAMS.themes[theme];
  const css = [];
  const defs = [];
  const anim = [];
  const addTrack = (name, list) => { css.push(frames(name, list)); anim.push(`.${name}{animation:${name} ${T}s infinite}`); };
  const restTok = sp(PATH[PATH.length - 1]); // the reduced-motion still rests the chip on the last command
  css.push(`.m{font-family:${TY.mono}}`, `.s{font-family:${TY.sans}}`);
  css.push(`.cmd{font-size:${F_CMD}px;font-weight:600;fill:${P.cmd}}`);
  css.push(`.note{font-size:${F_NOTE}px;fill:${P.note}}`);
  css.push(`.st{font-size:${F_START}px;font-weight:600;fill:${P.sText}}`);
  css.push(`.hn{font-size:${F_HELP}px;font-weight:500;fill:${P.hName}}`);
  css.push(`.hnote{font-size:${F_NOTE}px;fill:${P.hNote}}`);
  css.push(`.lbl{font-size:${F_KEY}px;fill:${P.loopLabel}}`);
  css.push(`.key{font-size:${F_KEY}px;fill:${P.key}}`);
  // base styles are the reduced-motion still: the straight walk finished, the optional fix arc idle.
  css.push(`.tok{transform:translate(${r1(restTok[0])}px,${r1(restTok[1])}px)}`);
  css.push(`.fl,.fx,.hl{opacity:0}`, `.flt{opacity:0;fill:${P.flashInk}}`, `.bdg{font-size:${TY.badge}px;font-weight:600}`, `.tr{stroke-dashoffset:0}`);
  css.push(`.fa{opacity:0}`, `.fat{opacity:0;fill:${P.aFlashInk}}`);
  css.push(`.tl{opacity:0}`);

  css.push(frames('tok', tok.map(([t, p, o, e]) => [t, `transform:translate(${r1(p[0])}px,${r1(p[1])}px);opacity:${o}`, e])));
  anim.push(`.tok{animation:tok ${T}s infinite}`);
  for (const t of tiles) {
    const kf = [[0, 'transform:translateY(0px)', EIO]];
    for (const [a, b] of presses[t.id]) {
      kf.push([a - 0.12, 'transform:translateY(0px)', EIO], [a + 0.16, `transform:translateY(${LY.press}px)`, EIO], [b, `transform:translateY(${LY.press}px)`, EIO], [b + 0.35, 'transform:translateY(0px)', EIO]);
    }
    addTrack(`p-${t.id}`, kf);
  }
  // helpers light while their caller runs, then settle to a quiet done tint
  for (const h of helpers) {
    const lit = [], dn = [];
    for (const [on, holdEnd, offEnd] of hLit[h.id]) {
      lit.push([on, on + 0.25, 1, EZ.snap], [holdEnd, offEnd, 0]);
      const [c0, c1] = clr(clearOf(on));
      dn.push([holdEnd, offEnd, 1], [c0, c1, 0]);
    }
    addTrack(`h-${h.id}`, levelTrack(lit));
    addTrack(`k-${h.id}`, levelTrack(dn));
  }
  // ran and fix states: tint plus tag, cleared at the end of the walk; fix turns to ran on the next pass
  for (const t of tiles) {
    const ran = [], fix = [];
    for (const [a, state] of doneEv[t.id]) {
      const [c0, c1] = clr(clearOf(a));
      if (state === 'fix') {
        const later = doneEv[t.id].find(([u, s]) => u > a && s === 'ran');
        fix.push([a, a + 0.35, 1], later ? [later[0], later[0] + 0.35, 0] : [c0, c1, 0]);
      } else {
        ran.push([a, a + 0.35, 1], [c0, c1, 0]);
      }
    }
    addTrack(`d-${t.id}`, levelTrack(ran));
    if (fix.length) addTrack(`x-${t.id}`, levelTrack(fix));
  }
  // one flash per arrival: a 0.15 s rise, a 0.5 s settle
  const flash = (list) => levelTrack(list.flatMap((a) => [[a, a + 0.15, 1, EZ.snap], [a + 0.15, a + 0.65, 0]]));
  for (const t of tiles) if (flashes[t.id].length) addTrack(`f-${t.id}`, flash(flashes[t.id]));
  for (const t of tiles) if (fixFlashes[t.id].length) addTrack(`fa-${t.id}`, flash(fixFlashes[t.id]));
  rails.forEach((r, i) => {
    const sp4 = r.pts.map((p) => iso(...p));
    const a = Math.hypot(sp4[1][0] - sp4[0][0], sp4[1][1] - sp4[0][1]);
    const b = Math.hypot(sp4[2][0] - sp4[1][0], sp4[2][1] - sp4[1][1]);
    const B = byId[r.to];
    const vz = iso(r.pts[3][0], B.y - B.H, 0);
    const c = Math.hypot(vz[0] - sp4[2][0], vz[1] - sp4[2][1]);
    const len = railLens[i];
    addTrack(`t${i}`, trailTrack(len, reveals[i], (ts) => [
      [ts[2][0], len + 2, ts[2][1]], [ts[3][0], len - a, ts[3][1]], [ts[4][0], len - a - b, ts[4][1]], [ts[5][0], len - a - b - c, ts[5][1]], [ts[6][0], 0, EIO],
    ]));
  });
  addTrack('tl', trailTrack(loopLen, loopReveals, (ts) => {
    const cum = [0];
    for (let i = 1; i < loopPts.length; i++) cum.push(cum[i - 1] + Math.hypot(loopPts[i][0] - loopPts[i - 1][0], loopPts[i][1] - loopPts[i - 1][1]));
    return ts.map(([t, e], i) => [t, i === 0 ? loopLen + 2 : loopLen * (1 - cum[i] / cum[cum.length - 1]), e]);
  }));
  // a walked arrow fades in as the chip passes 55% of the way along its run
  for (const ar of arrows) {
    const ramps = [];
    for (const ts of reveals[ar.i]) {
      const tp = ts[ar.s0][0] + 0.55 * (ts[ar.s1][0] - ts[ar.s0][0]);
      const [c0, c1] = clr(clearOf(ts[0][0]));
      ramps.push([tp, tp + 0.12, 1, EZ.snap], [c0, c1, 0]);
    }
    addTrack(ar.cls, levelTrack(ramps));
  }
  css.push(...anim);
  css.push(`@media (prefers-reduced-motion:reduce){*{animation:none!important}}`);

  const g = [];
  // grid: iso lines clipped to an ellipse, fading in three bands, so the figure has no hard edge on the page
  {
    const cx = VW / 2 - OX, cy = (VH - KEY_LH * 2) / 2 - OY, rx = VW * 0.56, ry = (VH - KEY_LH * 2) * 0.56;
    const bands = [[], [], []];
    const inside = (p) => ((p[0] - cx) / rx) ** 2 + ((p[1] - cy) / ry) ** 2;
    for (const dir of ['x', 'y']) {
      for (let c = -1600; c <= 1600; c += LY.grid) {
        const at = (u) => (dir === 'x' ? iso(c, u) : iso(u, c));
        let run = null;
        for (let u = -1600; u <= 1600; u += 8) {
          const p = at(u), q = at(u + 8);
          const d = inside([(p[0] + q[0]) / 2, (p[1] + q[1]) / 2]);
          const b = d < 0.35 ? 2 : d < 0.7 ? 1 : d < 1 ? 0 : -1;
          if (b < 0) { run = null; continue; }
          if (run && run.b === b) { run.q = q; continue; }
          run = { b, p, q }; bands[b].push(run);
        }
      }
    }
    const ops = [0.25, 0.55, 1];
    bands.forEach((runs, i) => {
      if (!runs.length) return;
      g.push(`<path d="${runs.map((r) => `M${pt(r.p)}L${pt(r.q)}`).join('')}" stroke="${P.grid}" stroke-opacity="${r1(ops[i] * P.gridOp * 100) / 100}" stroke-width="1" fill="none"/>`);
    });
  }
  // rails: a raised groove, its trail, and its arrows
  const head = (p, u) => {
    const v = [-u[1], u[0]];
    const q = [[p[0] + u[0] * 7, p[1] + u[1] * 7], [p[0] - u[0] * 6 + v[0] * 6, p[1] - u[1] * 6 + v[1] * 6], [p[0] - u[0] * 2.5, p[1] - u[1] * 2.5], [p[0] - u[0] * 6 - v[0] * 6, p[1] - u[1] * 6 - v[1] * 6]];
    return poly(q.map(([x, y]) => iso(x, y, 0)));
  };
  rails.forEach((r, i) => {
    const d = pathD(r.pts.map((p) => iso(...p)));
    const len = r1(railLens[i] + 2);
    g.push(`<path d="${d}" fill="none" stroke="${P.rail}" stroke-width="9" stroke-linejoin="round" stroke-linecap="round"/>`);
    g.push(`<path d="${d}" fill="none" stroke="${P.railIn}" stroke-width="3" stroke-linejoin="round" stroke-linecap="round"/>`);
    g.push(`<path class="tr t${i}" d="${d}" fill="none" stroke="${P.trail}" stroke-width="4.6" stroke-linejoin="round" stroke-dasharray="${len} ${r1(len + 6)}"/>`);
    for (const a of arrows.filter((q) => q.i === i)) {
      g.push(`<polygon points="${head(a.p, a.u)}" fill="${P.arrow}"/>`);
      g.push(`<polygon class="${a.cls}" points="${head(a.p, a.u)}" fill="${P.arrowWalked}"/>`);
    }
  });
  // a rounded tag printed on a front face: a drawn check with "ran", a drawn bang with "fix", or a bare check
  const badge = (b, kind, cls) => {
    const word = kind === 'ran' ? PARAMS.badge.ran : kind === 'fix' ? PARAMS.badge.fix : '';
    const fs = TY.badge;
    const bh = word ? 22 : Math.min(13, b.H - 3);
    const bw = word ? 24 + word.length * 0.56 * fs + 8 : 22;
    const bg = kind === 'fix' ? P.fixBg : P.ranBg, st = kind === 'fix' ? P.fixStroke : P.ranStroke, ink = kind === 'fix' ? P.fixInk : P.ranInk;
    const my = bh / 2;
    const icon = kind === 'fix'
      ? `<path d="M12 ${r1(my - 6)}V${r1(my + 1.5)}" stroke="${ink}" stroke-width="2.2" stroke-linecap="round"/><circle cx="12" cy="${r1(my + 5.5)}" r="1.4" fill="${ink}"/>`
      : word
        ? `<path d="M7 ${r1(my + 0.5)}L10.5 ${r1(my + 4)}L17 ${r1(my - 3.5)}" fill="none" stroke="${ink}" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"/>`
        : `<path d="M6 ${r1(my + 0.2)}L9.5 ${r1(my + 3.4)}L16 ${r1(my - 3.2)}" fill="none" stroke="${ink}" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>`;
    const text = word ? `<text class="s bdg" x="22" y="${r1(my + fs * 0.36)}" fill="${ink}">${esc(word)}</text>` : '';
    return `<g class="${cls}" transform="${frontText(b.x + b.L - LY.tagRight - bw, b.y + b.W, (b.H + bh) / 2)}"><rect width="${r1(bw)}" height="${bh}" rx="${r1(Math.min(6, bh / 2))}" fill="${bg}" stroke="${st}" stroke-width="1.2"/>${icon}${text}</g>`;
  };
  // tiles, back to front
  const boxes = [...tiles, ...helpers];
  const behind = (a, b) => a.x + a.L <= b.x || a.y + a.W <= b.y;
  boxes.sort((a, b) => (behind(a, b) ? -1 : behind(b, a) ? 1 : a.x + a.y - (b.x + b.y)));
  for (const b of boxes) {
    const f = boxFaces(b);
    if (b.kind === 'helper') {
      const da = 'stroke-dasharray="5 4"';
      const tp = b.tether.map((p) => iso(...p));
      g.push(`<circle cx="${r1(tp[0][0])}" cy="${r1(tp[0][1])}" r="2.6" fill="${P.tether}"/>`);
      g.push(`<path d="${pathD(tp)}" fill="none" stroke="${P.tether}" stroke-width="1.6" stroke-dasharray="4 4" stroke-linejoin="round" stroke-linecap="round"/>`);
      g.push(`<g>`);
      g.push(`<polygon points="${poly(f.front)}" fill="${P.hFront}" stroke="${P.hStroke}" stroke-width="1.2" ${da} stroke-linejoin="round"/>`);
      g.push(`<polygon points="${poly(f.end)}" fill="${P.hEnd}" stroke="${P.hStroke}" stroke-width="1.2" ${da} stroke-linejoin="round"/>`);
      g.push(`<polygon points="${poly(f.top)}" fill="${P.hTop}" fill-opacity="${P.hTopOp}" stroke="${P.hStroke}" stroke-width="1.2" ${da} stroke-linejoin="round"/>`);
      g.push(`<polygon class="dn k-${b.id}" points="${poly(f.top)}" fill="${P.hDone}" fill-opacity="${P.hDoneOp}" stroke="none"/>`);
      // a lit helper keeps its dashes, so it never reads as a typed command
      g.push(`<g class="hl h-${b.id}"><polygon points="${poly(f.top)}" fill="${P.lit}" fill-opacity="${P.litOp}" stroke="${P.acc}" stroke-width="1.6" ${da} stroke-linejoin="round"/><polygon points="${poly(f.front)}" fill="none" stroke="${P.acc}" stroke-width="1.2" ${da} stroke-linejoin="round"/><polygon points="${poly(f.end)}" fill="none" stroke="${P.acc}" stroke-width="1.2" ${da} stroke-linejoin="round"/></g>`);
      b.names.forEach((n, i) => g.push(`<text class="m hn" transform="${topText(b.x + 12, b.y + LY.helperFirst + i * HWL, b.H)}">${esc(n)}</text>`));
      g.push(`<text class="s hnote" transform="${frontText(b.x + 12, b.y + b.W, 6.5)}">${esc(b.note)}</text>`);
      g.push(`</g>`);
      continue;
    }
    const st = b.kind === 'start';
    const ed = st ? P.sEdge : P.edge;
    const hasFix = doneEv[b.id].some(([, s]) => s === 'fix');
    let topShape = f.top, inner = f.top;
    const deferred = [];
    g.push(`<g>`);
    if (st) {
      g.push(`<polygon points="${poly(f.front)}" fill="${P.sFront}" stroke="${ed}" stroke-width="1" stroke-linejoin="round"/>`);
      g.push(`<polygon points="${poly(f.end)}" fill="${P.sEnd}" stroke="${ed}" stroke-width="1" stroke-linejoin="round"/>`);
    } else {
      // a keycap: walls extruded from a rounded outline, each graded top to bottom along its slant
      const ol = rrect(b.x, b.y, b.L, b.W, 7);
      for (const [k, segs] of Object.entries(walls(ol))) {
        const ring = [segs[0][0], ...segs.map((s) => s[1])];
        const topPts = ring.map(([x, y]) => iso(x, y, b.H));
        const pts = [...topPts, ...ring.map(([x, y]) => iso(x, y, 0)).reverse()];
        const xs = pts.map((p) => p[0]), ys = pts.map((p) => p[1]);
        const bx0 = Math.min(...xs), bx1 = Math.max(...xs), by0 = Math.min(...ys), by1 = Math.max(...ys);
        const slope = (topPts[topPts.length - 1][1] - topPts[0][1]) / ((topPts[topPts.length - 1][0] - topPts[0][0]) || 1);
        const kk = (slope * (bx1 - bx0)) / (by1 - by0);
        const left = topPts.reduce((m, p) => (p[0] < m[0] ? p : m));
        const y0n = (left[1] - by0) / (by1 - by0) - kk * ((left[0] - bx0) / (bx1 - bx0));
        const id = `k${k}-${b.id}`;
        defs.push(`<linearGradient id="${id}" x1="0" y1="${r2(y0n)}" x2="0" y2="${r2(y0n + b.H / (by1 - by0))}" gradientTransform="matrix(1 ${r2(kk)} 0 1 0 0)"><stop offset="0" stop-color="${k === 'f' ? P.kF1 : P.kE1}"/><stop offset="1" stop-color="${k === 'f' ? P.kF2 : P.kE2}"/></linearGradient>`);
        g.push(`<polygon points="${poly(pts)}" fill="url(#${id})" stroke="${ed}" stroke-width="1" stroke-linejoin="round"/>`);
      }
      topShape = ol.map((o) => iso(o.p[0], o.p[1], b.H));
      g.push(`<text class="s note" transform="${frontText(b.x + PADL, b.y + b.W, 9.5)}">${esc(b.note)}</text>`);
    }
    g.push(badge(b, st ? 'check' : 'ran', `dn d-${b.id}`));
    if (hasFix) g.push(badge(b, 'fix', `fx x-${b.id}`));
    g.push(`<g class="p-${b.id}">`);
    if (st) {
      g.push(`<polygon points="${poly(f.top)}" fill="${P.sTop}" stroke="${ed}" stroke-width="1" stroke-linejoin="round"/>`);
      g.push(`<path d="${pathD([f.top[3], f.top[0], f.top[1]])}" fill="none" stroke="${P.rim}" stroke-width="1" stroke-opacity="0.9"/>`);
    } else {
      // a bezel around an inset top; the inset's edge and back highlight go on after the tints,
      // so a lit or done key keeps its form. The highlight walks the back rim in angle order.
      g.push(`<polygon points="${poly(topShape)}" fill="${P.bezel}" stroke="${ed}" stroke-width="1" stroke-linejoin="round"/>`);
      const ins = rrect(b.x + 4, b.y + 4, b.L - 8, b.W - 8, 5);
      inner = ins.map((o) => iso(o.p[0], o.p[1], b.H));
      const turn = (o) => (o.a - 0.75 * Math.PI + 4 * Math.PI) % (2 * Math.PI);
      const backs = ins.filter((o) => Math.cos(o.a) + Math.sin(o.a) < -0.1).sort((p, q) => turn(p) - turn(q)).map((o) => iso(o.p[0], o.p[1], b.H));
      g.push(`<polygon points="${poly(inner)}" fill="${P.kTop}"/>`);
      deferred.push(`<polygon points="${poly(inner)}" fill="none" stroke="${P.kLo}" stroke-width="0.8" stroke-linejoin="round"/>`, `<path d="${pathD(backs)}" fill="none" stroke="${P.kHi}" stroke-width="1.4" stroke-linecap="round" stroke-opacity="0.95"/>`);
    }
    // tints cover the whole top, bezel included
    g.push(`<polygon class="dn d-${b.id}" points="${poly(topShape)}" fill="${P.doneTop}" fill-opacity="${st ? r2(P.doneOp * 0.6) : P.doneOp}"/>`);
    if (hasFix) g.push(`<g class="fx x-${b.id}"><polygon points="${poly(topShape)}" fill="${P.fixTop}" fill-opacity="${P.fixOp}"/><polygon points="${poly(inner)}" fill="none" stroke="${P.fixRim}" stroke-width="1.8" stroke-linejoin="round"/></g>`);
    if (flashes[b.id].length) g.push(`<polygon class="fl f-${b.id}" points="${poly(topShape)}" fill="${P.flashTop}"/>`);
    if (fixFlashes[b.id].length) g.push(`<polygon class="fa fa-${b.id}" points="${poly(topShape)}" fill="${P.aFlashTop}"/>`);
    g.push(...deferred);
    const [sx, sy] = sock(b);
    const s = 11;
    g.push(`<polygon points="${poly([iso(sx - s, sy - s, b.H), iso(sx + s, sy - s, b.H), iso(sx + s, sy + s, b.H), iso(sx - s, sy + s, b.H)])}" fill="${P.sockFill}" stroke="${P.sockRing}" stroke-width="1"/>`);
    if (st) {
      startLines.forEach((ln, i) => g.push(`<text class="s st" transform="${topText(b.x + PADL, b.y + LY.startFirst + i * LY.startLine, b.H)}">${esc(ln)}</text>`));
    } else {
      const at = topText(b.x + PADL + LY.cmdInset, b.y + b.W / 2 + 7, b.H);
      g.push(`<text class="m cmd" transform="${at}">${esc(b.cmd)}</text>`);
      g.push(`<text class="m cmd flt f-${b.id}" transform="${at}">${esc(b.cmd)}</text>`);
      if (fixFlashes[b.id].length) g.push(`<text class="m cmd fat fa-${b.id}" transform="${at}">${esc(b.cmd)}</text>`);
    }
    g.push(`</g></g>`);
  }
  // the fix arc: idle grey with an arrowhead, trailed when the chip rides it
  {
    const len = r1(loopLen + 2);
    g.push(`<path d="${loopD}" fill="none" stroke="${P.loop}" stroke-width="2" stroke-linecap="round"/>`);
    g.push(`<path class="tr tl" d="${loopD}" fill="none" stroke="${P.trail}" stroke-width="2.8" stroke-dasharray="${len} ${r1(len + 6)}"/>`);
    const a = loopEnd, b0 = bez(0.84, LA, lc1, lc2, LB);
    const ang = Math.atan2(a[1] - b0[1], a[0] - b0[0]);
    const hd = (k, s) => [a[0] + Math.cos(ang + k) * s, a[1] + Math.sin(ang + k) * s];
    g.push(`<polygon points="${poly([a, hd(Math.PI - 0.45, 9), hd(Math.PI + 0.45, 9)])}" fill="${P.loop}"/>`);
    g.push(`<text class="s lbl" x="${r1(loopApex[0] + 20)}" y="${r1(loopApex[1] + 5)}">${esc(data.loop.label)}</text>`);
  }
  // the chip: a small glowing iso tile
  {
    const c = 8.5, h = 5;
    const tp = [iso(-c, -c, h), iso(c, -c, h), iso(c, c, h), iso(-c, c, h)];
    const lf = [iso(-c, c, h), iso(c, c, h), iso(c, c, 0), iso(-c, c, 0)];
    const rt = [iso(c, -c, h), iso(c, c, h), iso(c, c, 0), iso(c, -c, 0)];
    g.push(`<g class="tok"><ellipse cx="0" cy="0" rx="24" ry="14" fill="${P.halo}" fill-opacity="${P.haloOp}"/><ellipse cx="0" cy="0" rx="14" ry="8.5" fill="${P.halo}" fill-opacity="${r2(P.haloOp * 1.6)}"/><polygon points="${poly(lf)}" fill="${P.tokL}"/><polygon points="${poly(rt)}" fill="${P.tokR}"/><polygon points="${poly(tp)}" fill="${P.tokTop}"/></g>`);
  }
  // key: a keycap sample for typed commands, a dashed sample for helpers
  {
    const kx = minX + M, ky = keyTop;
    const at = (x, y) => (pts) => poly(pts.map(([a, c]) => [a + x, c + y]));
    const keycap = (x, y) => {
      const tr = at(x, y);
      const ol = rrect(0, 0, 16, 10, 2.6, 3);
      let out = '';
      for (const [k, segs] of Object.entries(walls(ol))) {
        const ring = [segs[0][0], ...segs.map((q) => q[1])];
        out += `<polygon points="${tr([...ring.map(([a, c]) => iso(a, c, 7)), ...ring.map(([a, c]) => iso(a, c, 0)).reverse()])}" fill="${k === 'f' ? P.kF1 : P.kE1}" stroke="${P.edge}" stroke-width="0.8" stroke-linejoin="round"/>`;
      }
      out += `<polygon points="${tr(ol.map((o) => iso(o.p[0], o.p[1], 7)))}" fill="${P.bezel}" stroke="${P.edge}" stroke-width="0.8" stroke-linejoin="round"/>`;
      out += `<polygon points="${tr(rrect(1.6, 1.6, 12.8, 6.8, 1.8, 3).map((o) => iso(o.p[0], o.p[1], 7)))}" fill="${P.kTop}"/>`;
      return out;
    };
    const dashed = (x, y) => {
      const tr = at(x, y);
      const f = boxFaces({ x: 0, y: 0, L: 16, W: 10, H: 4 });
      const st = `fill="${P.hTop}" stroke="${P.hStroke}" stroke-width="1" stroke-dasharray="3 2"`;
      return `<polygon points="${tr(f.front)}" ${st}/><polygon points="${tr(f.end)}" ${st}/><polygon points="${tr(f.top)}" ${st}/>`;
    };
    g.push(keycap(kx + 6, ky + 6));
    g.push(`<text class="s key" x="${r1(kx + 28)}" y="${r1(ky + 14)}">${esc(data.key.typed)}</text>`);
    g.push(dashed(kx + 6, ky + 6 + KEY_LH));
    g.push(`<text class="s key" x="${r1(kx + 28)}" y="${r1(ky + 14 + KEY_LH)}">${esc(data.key.helper)}</text>`);
  }

  // alt text, from the data
  const runsOf = (h) => {
    const sub = data.helpers.filter((x) => x.calledBy === h.id);
    return `${h.names.join(' and ')}, which ${sub.length ? `use ${sub.map((x) => x.names.join(' and ')).join(' and ')} so the text ${sub.map((x) => x.note).join(' and ')}` : h.note}`;
  };
  const step = (t) => {
    const hs = data.helpers.filter((h) => h.calledBy === t.id);
    return `Type ${t.cmd} to ${t.note}${hs.length ? `. It runs ${hs.map(runsOf).join('. It also runs ')}` : ''}${t.id === data.loop.from ? `, and you ${data.loop.label} and review again` : ''}.`;
  };
  const alt = `How monet works. ${data.start.label}. ${data.typed.map(step).join(' ')} ${data.key.typed} ${data.key.helper}`;

  return `<svg xmlns="http://www.w3.org/2000/svg" role="img" width="${VW}" height="${VH}" viewBox="0 0 ${VW} ${VH}">
<title>${esc(alt)}</title>
<style>${css.join('\n')}</style>
<defs>${defs.join('')}</defs>
<g transform="translate(${r1(OX)} ${r1(OY)})">
${g.join('\n')}
</g>
</svg>
`;
}

// ---------- CLI ----------
const check = process.argv.includes('--check');
let stale = null;
for (const theme of ['light', 'dark']) {
  const svg = build(theme);
  const file = OUT[theme];
  const name = `docs/assets/${file.pathname.split('/').pop()}`;
  if (check) {
    let have = null;
    try { have = readFileSync(file, 'utf8'); } catch { have = null; }
    if (have !== svg && !stale) stale = name;
  } else {
    writeFileSync(file, svg);
    console.log(`wrote ${name} (${Buffer.byteLength(svg)} bytes)`);
  }
}
if (check) {
  if (stale) {
    console.log(`FAIL how-it-works ${stale}: stale. Run node tools/how-it-works.mjs.`);
    process.exit(1);
  }
  console.log('ok how-it-works');
}
