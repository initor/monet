#!/usr/bin/env node
/**
 * Fills the three script slots of an explainer page:
 * `node explainer.mjs <page.html> <figure.js>`.
 * The slots are hl-kernel (the kernel of the hairline-create skill), hl-host
 * (explainer-host.js, beside this file) and hl-figure (the figure). It
 * replaces whatever each slot holds, so it runs again after any edit. It
 * refuses a kernel that differs from the hash in its own first line, as the
 * validate.mjs of hairline-create does.
 */
import { createHash } from "node:crypto";
import { readFileSync, realpathSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

const here = (p) => fileURLToPath(new URL(p, import.meta.url));
const unix = (s) => s.replace(/\r\n/g, "\n");

/** The kernel, once its first line is exactly its hash comment and its body matches that hash. */
export function kernel(path = here("../../hairline-create/kernel.js")) {
  const k = unix(readFileSync(path, "utf8")).trimEnd(), cut = k.indexOf("\n");
  const want = /^\/\* hairline kernel sha256:([0-9a-f]{64}) \*\/$/.exec(k.slice(0, cut))?.[1];
  if (!want || want !== createHash("sha256").update(k.slice(cut + 1) + "\n").digest("hex")) {
    throw new Error(`explainer: ${path} differs from its hash. Copy hairline-create again; the kernel is never edited.`);
  }
  return k;
}

const SLOTS = [
  ["hl-kernel", '<script id="hl-kernel">'],
  ["hl-host", '<script id="hl-host">'],
  ["hl-figure", '<script type="module" id="hl-figure">'],
];

/** The page with each slot holding its script. Throws on a missing or doubled slot, checked first. */
export function fill(page, figure, k = kernel(), host = readFileSync(here("./explainer-host.js"), "utf8")) {
  let out = unix(page);
  for (const [, open] of SLOTS) {
    const at = out.indexOf(open);
    if (at < 0 || out.indexOf(open, at + 1) >= 0) throw new Error(`explainer: the page needs exactly one ${open}</script>`);
    if (out.indexOf("</script>", at) < 0) throw new Error(`explainer: ${open} is never closed`);
  }
  const body = { "hl-kernel": k, "hl-host": unix(host).trimEnd(), "hl-figure": unix(figure).trim() };
  for (const [id, text] of Object.entries(body)) {
    if (/<\/script/i.test(text)) throw new Error(`explainer: ${id} contains "</script", which would end its slot early`);
  }
  for (const [id, open] of SLOTS) {
    const from = out.indexOf(open) + open.length, to = out.indexOf("</script>", from);
    out = out.slice(0, from) + "\n" + body[id] + "\n" + out.slice(to);
  }
  return out;
}

if (process.argv[1] && realpathSync(process.argv[1]) === realpathSync(here("./explainer.mjs"))) {
  const [page, figure] = process.argv.slice(2);
  if (!page || !figure || !/\.html?$/i.test(page) || /\.html?$/i.test(figure)) {
    console.error("usage: node explainer.mjs <page.html> <figure.js>");
    process.exit(2);
  }
  writeFileSync(page, fill(readFileSync(page, "utf8"), readFileSync(figure, "utf8")));
  console.log(page);
}
