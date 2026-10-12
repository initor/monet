// gang simulator: runs the engine against mock agents on a virtual clock, and checks invariants.
// Usage: node sim.mjs --template <engine.template.js> | --engine <run/engine.js> [--scenario <text>]
import fs from 'node:fs'
import path from 'node:path'
import process from 'node:process'
import { fileURLToPath } from 'node:url'

const KEYS = ['common', 'implement_new', 'implement_resume', 'implement_fix', 'implement_sync', 'review_full', 'review_fix', 'review_merge', 'adversarial', 'merge', 'health', 'repair']
const ROLES = ['implement', 'implement_hard', 'review', 'review_adversarial', 'fix', 'sync', 'merge', 'health', 'repair', 'plan_draft', 'plan_review']
const MODELS = ['opus', 'sonnet', 'haiku', 'fable']
const EFFORTS = ['low', 'medium', 'high', 'xhigh', 'max']
const TERMINAL = ['merged', 'blocked', 'blocked-dep', 'not-started', 'interrupted', 'parked']
const STOPS = ['DONE', 'BLOCKED', 'TRUNK_RED', 'INTERRUPTED', 'STOPPED']
const RESULT_KEYS = ['runTag', 'trunkStart', 'trunkTip', 'stop', 'reason', 'counts', 'byStatus', 'reasons', 'mergeLog', 'fixTasks', 'nextFixNumber', 'health', 'rulings', 'events']
const PH = ['TASKS_JSON', 'TASKSUM', 'DONE', 'CFG', 'PROMPTS']
const AsyncFunction = Object.getPrototypeOf(async function () {}).constructor

const arg = name => { const i = process.argv.indexOf(name); return i > 0 ? process.argv[i + 1] : null }
const templatePath = arg('--template')
const enginePath = arg('--engine')
const only = arg('--scenario')
if (!templatePath === !enginePath) {
  console.error('usage: node sim.mjs --template <engine.template.js> | --engine <engine.js> [--scenario <text>]')
  process.exit(64)
}

function fnv(str) { let h = 0x811c9dc5; for (let i = 0; i < str.length; i++) { h ^= str.charCodeAt(i); h = Math.imul(h, 0x01000193) >>> 0 } return h }
function rng(seed) { return () => { seed |= 0; seed = seed + 0x6D2B79F5 | 0; let t = Math.imul(seed ^ seed >>> 15, 1 | seed); t = t + Math.imul(t ^ t >>> 7, 61 | t) ^ t; return ((t ^ t >>> 14) >>> 0) / 4294967296 } }
const esc = s => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
const usedVars = text => [...new Set([...text.matchAll(/\{\{([A-Za-z0-9_]+)\}\}/g)].map(m => m[1]))].sort()

// ---- roles.md: one fenced text block per key, and the variables table (parsed the way gen does) ----
function parseRoles(md) {
  const lines = md.split('\n')
  const prompts = {}
  let key = null
  for (let i = 0; i < lines.length; i++) {
    const h = lines[i].match(/^## (\S+)\s*$/)
    if (h) {
      key = h[1]
      if (key in prompts) throw new Error(`roles.md: duplicate key ${key}`)
      prompts[key] = null
      continue
    }
    if (key && lines[i] === '```text') {
      if (prompts[key] !== null) throw new Error(`roles.md: ${key} has more than one text block`)
      const body = []
      for (i++; i < lines.length && lines[i] !== '```'; i++) body.push(lines[i])
      if (i >= lines.length) throw new Error(`roles.md: the ${key} block has no closing fence`)
      prompts[key] = body.join('\n')
    }
  }
  for (const k of Object.keys(prompts)) if (prompts[k] === null) throw new Error(`roles.md: ${k} has no text block`)
  if (Object.keys(prompts).sort().join(' ') !== [...KEYS].sort().join(' ')) throw new Error(`roles.md: keys are ${Object.keys(prompts).join(' ')}`)
  const table = {}
  const start = lines.indexOf('| Key | Variables |')
  if (start < 0) throw new Error('roles.md: no "| Key | Variables |" table')
  for (let i = start + 2; i < lines.length && lines[i].startsWith('|'); i++) {
    const m = lines[i].match(/^\| `([a-z_]+)` \| (.*) \|$/)
    if (!m) throw new Error('roles.md: bad table row: ' + lines[i])
    table[m[1]] = [...m[2].matchAll(/`([A-Za-z0-9_]+)`/g)].map(x => x[1]).sort()
  }
  return { prompts, table }
}

// ---- the engine source: template mode fills it, engine mode reads a generated file ----------------
function generate(src, d, sum) {
  const text = JSON.stringify(d.tasks)
  const vals = { TASKS_JSON: JSON.stringify(text), TASKSUM: String(sum ?? fnv(text)), DONE: JSON.stringify(d.done), CFG: JSON.stringify(d.cfg), PROMPTS: JSON.stringify(d.prompts) }
  let out = src
  for (const k of PH) {
    const marker = `/*@@${k}@@*/null`
    const n = out.split(marker).length - 1
    if (n !== 1) throw new Error(`template: placeholder ${k} appears ${n} times`)
    out = out.replace(marker, () => vals[k])
  }
  return out
}
const bodyOf = src => src.slice(src.indexOf('\n}\n', src.indexOf('export const meta')) + 3)
const varsOf = src => {
  const m = src.match(/^const VARS = (\{[\s\S]*?\n\})$/m)
  if (!m) throw new Error('engine: no VARS table')
  return JSON.parse(m[1])
}

const BASE_ROLES = {
  implement: { model: 'opus', effort: 'high' }, implement_hard: { model: 'opus', effort: 'xhigh' },
  review: { model: 'fable', effort: 'xhigh' }, review_adversarial: { model: 'opus', effort: 'max' },
  fix: { model: 'sonnet', effort: 'high' }, sync: { model: 'sonnet', effort: 'medium' },
  merge: { model: 'haiku', effort: 'low' }, health: { model: 'haiku', effort: 'medium' }, repair: { model: 'opus', effort: 'low' },
  plan_draft: { model: 'opus', effort: 'medium' }, plan_review: { model: 'sonnet', effort: 'low' },
}
// A seeded graph: 4 lanes of 10 tasks, deps only on earlier tasks, cross-lane deps only into the lane below,
// and 4 merge_after edges from lanes 0 and 1 into lanes 2 and 3 (so "head never implements" waiters exist).
function synthGraph(seed) {
  const R = rng(seed)
  const N = 40
  const id = i => 'T' + String(i + 1).padStart(3, '0')
  const g = []
  for (let i = 0; i < N; i++) {
    const lane = i % 4
    const pos = Math.floor(i / 4)
    const deps = new Set()
    if (pos > 0) {
      deps.add(id(4 * Math.max(0, pos - 1 - Math.floor(R() * 2)) + lane))
      if (R() < 0.4) deps.add(id(4 * Math.floor(R() * pos) + lane))
      if (lane > 0 && R() < 0.2) deps.add(id(4 * Math.floor(R() * pos) + lane - 1))
    }
    g.push({ id: id(i), title: `synthetic task ${id(i)}`, branch: 'gang/' + id(i), deps: [...deps].sort(), mergeAfter: [],
      size: 100 + Math.floor(R() * 900), risk: R() < 0.2 ? 'high' : 'low', locks: R() < 0.1 ? ['gpu'] : R() < 0.1 ? ['kind'] : [] })
  }
  for (const [w, m] of [[36, 14], [32, 19], [37, 26], [33, 31]]) g[w].mergeAfter.push(id(m))
  const level = []
  for (let i = 0; i < N; i++) level[i] = 1 + Math.max(0, ...g[i].deps.concat(g[i].mergeAfter).map(d => level[+d.slice(1) - 1]))
  const top = Math.max(...level)
  const chain = []
  for (let i = N - 1; i >= 0; i--) {
    const succ = g.filter(s => s.deps.includes(g[i].id) || s.mergeAfter.includes(g[i].id)).map(s => chain[+s.id.slice(1) - 1])
    chain[i] = Math.round(g[i].size / 20) + Math.max(0, ...succ)
  }
  const size = new Map(g.map(t => [t.id, t.size]))
  const tasks = g.map((t, i) => ({ id: t.id, title: t.title, branch: t.branch, deps: t.deps, mergeAfter: t.mergeAfter,
    wave: 1 + Math.floor((level[i] - 1) * 5 / top), chain: chain[i], risk: t.risk, locks: t.locks,
    roles: i % 7 === 3 ? { ...BASE_ROLES, review: { model: 'sonnet', effort: 'xhigh' } } : BASE_ROLES }))
  return { tasks, size }
}

function loadInput() {
  if (templatePath) {
    const template = fs.readFileSync(templatePath, 'utf8')
    const rolesPath = path.join(path.dirname(fileURLToPath(import.meta.url)), '..', 'reference', 'roles.md')
    const { prompts, table } = parseRoles(fs.readFileSync(rolesPath, 'utf8'))
    const { tasks, size } = synthGraph(42)
    const sha0 = 'a'.repeat(40)
    const cfg = { runTag: 'sim', slug: 'sim', gang: '/repo/.planning/gang/sim/run/bin/gang', dir: '/repo/.planning/gang/sim', repo: '/repo',
      integration: 'gang-sim', trunkStart: sha0, count: tasks.length, maxImplementers: 5, maxFixRounds: 3, maxRepairAttempts: 2,
      maxConflictRounds: 3, maxTrunkRed: 2, haltAfterNulls: 6, blockedStop: 3, onlyTasks: null, maxWave: 999, healthAfterMerge: true,
      nextFixNumber: 1, roles: { merge: { model: 'sonnet', effort: 'max' }, health: { model: 'haiku', effort: 'high' }, repair: { model: 'fable', effort: 'max' } } }
    return { template, tasks, size, cfg, prompts, table }
  }
  const src = fs.readFileSync(enginePath, 'utf8')
  const data = {}
  let template = src
  for (const k of PH) {
    const re = new RegExp(`^const ${k} = (.*)$`, 'm')
    const m = template.match(re)
    if (!m) throw new Error(`engine: no line "const ${k} = ..."`)
    data[k] = JSON.parse(m[1])
    template = template.replace(re, () => `const ${k} = /*@@${k}@@*/null`)
  }
  if (fnv(data.TASKS_JSON) !== data.TASKSUM) throw new Error('engine: TASKSUM does not match TASKS_JSON')
  return { template, tasks: JSON.parse(data.TASKS_JSON), size: new Map(), cfg: data.CFG, prompts: data.PROMPTS, table: null }
}
const IN = loadInput()
const VARS = varsOf(IN.template)
const COVER = new Set()
const KEYRE = {}
for (const k of KEYS) {
  if (k === 'common') continue
  const pat = t => t.split(/\{\{[A-Za-z0-9_]+\}\}/).map(esc).join('[\\s\\S]*?')
  KEYRE[k] = new RegExp('^' + (k === 'merge' ? '' : pat(IN.prompts.common) + '\\n\\n---\\n\\n') + pat(IN.prompts[k]) + '$')
}
const TIP_RE = /`git rev-parse HEAD` prints ([0-9a-f]{40})/
const DIFF_RE = /`git diff ([0-9a-f]{40}) ([0-9a-f]{40})`/
const MERGE_RE = / merge (\S+) --sha ([0-9a-f]{40})\s*$/
const BRIEF_RE = new RegExp(esc(IN.cfg.gang) + ' brief (\\S+?)`', 'g')

// ---- virtual clock --------------------------------------------------------------------------------
function clock() {
  let now = 0
  let seq = 0
  const q = []
  return {
    get now() { return now },
    sleep: ms => new Promise(r => q.push({ at: now + ms, seq: seq++, r })),
    async run(p) {
      let done = false
      let value
      let error
      p.then(v => { done = true; value = v }, e => { done = true; error = e })
      let idle = 0
      while (!done) {
        await new Promise(r => setImmediate(r))
        if (done) break
        if (!q.length) { if (++idle > 5) return { hang: true, vtime: now }; continue }
        idle = 0
        q.sort((a, b) => a.at - b.at || a.seq - b.seq)
        const t = q.shift()
        now = t.at
        t.r()
      }
      if (error) throw error
      return { value, vtime: now }
    },
  }
}

// ---- one engine segment against the mock world ---------------------------------------------------
async function runSegment(W, R, o, segNo) {
  const C = clock()
  const tasks = o.tasks || IN.tasks
  const byId = new Map(tasks.map(t => [t.id, t]))
  const done = o.done === 'world' ? [...W.merged].sort() : (o.done || [])
  if (segNo === 0) for (const id of done) W.merged.add(id)
  const fixNums = done.filter(id => /^F[0-9]+$/.test(id)).map(id => +id.slice(1))
  const cfg = { ...IN.cfg, onlyTasks: null, maxWave: 999, count: tasks.length, trunkStart: W.trunk,
    runTag: `sim${segNo + 1}`, nextFixNumber: Math.max(0, ...fixNums) + 1, ...(o.cfg || {}) }
  const S = { calls: 0, labels: {}, activeImpl: 0, maxImpl: 0, activeMerge: 0, chain: {}, newCalls: [], marks: {}, fixIds: new Set(),
    mergeCalls: [], order: [], firstFix: Infinity, fixMerged: false, greenAfterFix: Infinity, start: W.merged.size, redDone: false,
    nullRun: 0, haltVt: Infinity, retry: {}, firstRed: Infinity, survived: new Set(), seen: {}, repairing: 0 }
  const fail = m => { if (W.errors.length < 30) W.errors.push((segNo ? `segment ${segNo + 1}: ` : '') + m) }
  const mark = k => { (S.marks[k] = S.marks[k] || []).push(C.now) }
  const newSha = () => { const n = ++W.seq; return (Math.imul(n, 2654435761) >>> 0).toString(16).padStart(8, '0') + n.toString(16).padStart(32, '0') }
  const roleOf = (id, kind) => {
    const t = byId.get(id)
    if (kind === 'merge') return ['merge', cfg.roles.merge]
    if (!t) return [kind === 'new' ? 'repair' : null, cfg.roles.repair]
    const r = { new: t.risk === 'high' ? 'implement_hard' : 'implement', fix: 'fix', sync: 'sync', review: 'review', adversarial: 'review_adversarial' }
    r.resume = r.new
    return [null, t.roles[r[kind]]]
  }
  function classify(L) {
    if (L.startsWith('health ')) return { key: 'health', role: cfg.roles.health }
    const m = L.match(/^(\S+) (new|resume|fix|sync|review|adversarial|merge) \d+(?: (fix|merge))?/)
    if (!m) return null
    const [, id, kind, sub] = m
    const keys = { new: 'implement_new', resume: 'implement_resume', fix: 'implement_fix', sync: 'implement_sync', adversarial: 'adversarial', merge: 'merge' }
    const [over, role] = roleOf(id, kind)
    return { id, kind, key: over || (kind === 'review' ? 'review_' + (sub || 'full') : keys[kind]), role }
  }

  async function implementer(id, mode, L, prompt) {
    const t = byId.get(id)
    if (!t) { S.fixIds.add(id); S.firstFix = Math.min(S.firstFix, S.calls) }
    if (mode === 'new') S.chain[id] = true
    if (mode === 'fix' || mode === 'sync') S.chain[id] = false
    const slot = !!t && (mode === 'new' || (mode === 'resume' && S.chain[id]))
    if (slot) { S.activeImpl++; S.maxImpl = Math.max(S.maxImpl, S.activeImpl); if (S.activeImpl > cfg.maxImplementers) fail(`implementers over the cap: ${S.activeImpl}`) }
    if (mode === 'new') S.newCalls.push({ vt: C.now, id }) // repair starts too, so no repair starts after a stop
    if (t && mode === 'new') for (const d of t.deps) if (!W.merged.has(d)) fail(`${id} started before dependency ${d} merged`)
    if (mode === 'fix' || mode === 'sync') {
      const tm = prompt.match(TIP_RE)
      if (!tm || tm[1] !== W.tips[id]) fail(`${L}: told tip ${tm && tm[1].slice(-4)}, branch is ${String(W.tips[id]).slice(-4)}`)
    }
    const stopNow = !!t && mode === 'new' && !!o.stopRe && new RegExp(o.stopRe).test(L)
    // stopInRepairRe: this task agent waits until a repair agent runs, then reads a STOP line.
    const stopInRepair = !!t && mode === 'new' && !!o.stopInRepairRe && new RegExp(o.stopInRepairRe).test(L)
    for (let i = 0; stopInRepair && !S.repairing && i < 500; i++) await C.sleep(1)
    const blockNow = (o.blockRe && new RegExp(o.blockRe).test(L)) || (!t && o.blockFixes && mode === 'new')
    const repairNew = !t && mode === 'new'
    if (repairNew) S.repairing++
    await C.sleep(stopNow || stopInRepair || (blockNow && !(repairNew && o.slowFix)) ? 1 : repairNew && o.slowFix ? o.slowFix : 3 + Math.floor(R() * ((IN.size.get(id) || 500) / 60)))
    if (slot) S.activeImpl--
    if (repairNew) S.repairing--
    if (stopNow || stopInRepair) { mark('stopped'); return { status: 'STOPPED', branch: '', commit: '', gate_passed: false, summary: 'STOP: sim stop line', rulings: [] } }
    if (blockNow || R() < (o.pBlocked || 0)) { mark('blocked'); return { status: 'BLOCKED', branch: '', commit: '', gate_passed: false, summary: 'sim blocked', rulings: [] } }
    const c = newSha()
    W.tips[id] = c
    W.taskLine[id] = true // every mock commit message holds the line "Task: <id>"
    const forcedRed = !!t && mode === 'new' && o.redFirstNew && !S.redNewDone
    if (forcedRed) S.redNewDone = true
    if (t && (forcedRed || W.red || R() < (o.pRed || 0))) { W.gateOk[id] = false; return { status: 'TRUNK_RED', branch: t.branch, commit: c, gate_passed: false, summary: 'sim trunk red', rulings: [] } }
    const gate = !(R() < (o.pGateFail || 0))
    W.gateOk[id] = gate
    return { status: gate ? 'DONE' : 'DONE_WITH_CONCERNS', branch: t ? t.branch : 'gang/' + id, commit: c, gate_passed: gate,
      summary: gate ? 'sim done' : 'sim gate fails', concerns: '', rulings: R() < 0.1 ? ['Ruling: sim - x - y'] : [] }
  }

  async function reviewer(id, lens, n, L, prompt, rkey) {
    if (!W.gateOk[id]) fail(`${L}: reviewed a tip whose gate failed`)
    const tm = prompt.match(TIP_RE)
    if (!tm || tm[1] !== W.tips[id]) fail(`${L}: told tip ${tm && tm[1].slice(-4)}, branch is ${String(W.tips[id]).slice(-4)}`)
    const lm = prompt.match(/ledger record (\S+) <reviewed sha> --lens (review|adversarial) --blockers <N> --majors <N> --survivors <N> /)
    if (!lm || lm[1] !== id || lm[2] !== lens) fail(`${L}: the prompt's ledger record line is wrong (${lm ? lm[1] + ' ' + lm[2] : 'missing'})`)
    // A review that did not finish its checks gets exactly one retry, on the same tip, told about it.
    const key = id + ' ' + lens
    const pend = S.retry[key]
    if (pend && (pend.tip !== W.tips[id] || !prompt.includes(pend.path))) fail(`${L}: the unfinished review ${pend.path} was not retried on the same tip`)
    delete S.retry[key]
    // A fix or merge review diffs from the tip that the previous review of this task and lens saw.
    // A retry on the same tip counts as the same review.
    const seen = S.seen[key]
    if (!seen || seen.tip !== W.tips[id]) S.seen[key] = { tip: W.tips[id], prev: seen && seen.tip }
    if (rkey === 'review_fix' || rkey === 'review_merge') {
      const dm = prompt.match(DIFF_RE)
      const want = S.seen[key].prev
      if (!dm || dm[1] !== want) fail(`${L}: the diff starts at ${dm && dm[1].slice(-4)}, the previous review saw ${String(want).slice(-4)}`)
    }
    S.chain[id] = false
    await C.sleep(2 + Math.floor(R() * 6))
    const tip = W.tips[id]
    let clean = R() < (n === 1 ? (o.pClean ?? 0.6) : (o.pCleanLater ?? 0.8))
    if (lens === 'adversarial' && o.advBlockFirst && n === 1) clean = false
    const gate = R() < (o.pNotRun || 0) ? 'not-run' : 'pass'
    const mutation = !(R() < (o.pNoMutation || 0))
    const recorded = !(R() < (o.pNoLedger || 0))
    const b = clean ? 0 : 1
    // A first review that counts a surviving mutant but no blocker (pSurvivors).
    const surv = clean && n === 1 && !!o.pSurvivors && R() < o.pSurvivors ? 1 : 0
    if (surv) S.survived.add(key)
    // The verdict as Contract 4 computes it: any blocker, major, or survivor is CHANGES.
    if (recorded) W.ledger.set(`${id} ${tip} ${lens}`, b || surv ? 'CHANGES' : gate === 'pass' && mutation ? 'CLEAN' : 'INCOMPLETE')
    if (!b && !surv && !(gate === 'pass' && mutation && recorded) && !pend) S.retry[key] = { tip, path: `/${id}/${lens}-${cfg.runTag}-${n}.md` }
    return { reviewed_sha: tip, blockers: b, majors: 0, minors: 1, findings: clean ? [] : [{ severity: 'blocker', summary: `sim ${lens} finding` }],
      gate, mutation_checked: mutation, survivors: surv, ledger_recorded: recorded }
  }

  // Mirrors gang merge (Contract 6). Any refusal is an engine bug, so it also fails the scenario.
  async function merger(id, L, prompt) {
    S.activeMerge++
    if (S.activeMerge > 1) fail('concurrent merges')
    S.mergeCalls.push({ i: S.calls, id, plan: byId.has(id) })
    await C.sleep(1)
    S.activeMerge--
    const refuse = why => { fail(`${L}: gang merge refuses: ${why}`); return { status: 'REFUSED', trunk_sha: W.trunk, files: [], reason: why } }
    const pm = prompt.match(MERGE_RE)
    if (!pm || pm[1] !== id) return refuse('bad merge prompt')
    const sha = pm[2]
    const t = byId.get(id)
    if (!t && !S.fixIds.has(id)) return refuse('unknown task')
    if (sha !== W.tips[id]) return refuse('tip differs from --sha')
    if (!W.taskLine[id]) return refuse('no Task line')
    const lenses = t && t.risk === 'high' ? ['review', 'adversarial'] : ['review']
    const missing = lenses.filter(l => W.ledger.get(`${id} ${sha} ${l}`) !== 'CLEAN')
    if (missing.length) return refuse(`ledger not CLEAN for ${missing.join(', ')}`)
    if (t) for (const d of t.deps.concat(t.mergeAfter)) if (!W.merged.has(d)) return refuse(`${d} not merged yet`)
    if (W.mergedSha[id] === sha) return { status: 'ALREADY', trunk_sha: W.trunk, files: [], reason: '' }
    if (!W.gateOk[id]) fail(`${id} merged a tip whose gate failed`)
    if (!W.conflicted.has(id) && R() < (o.pConflict || 0)) { W.conflicted.add(id); return { status: 'CONFLICT', trunk_sha: W.trunk, files: ['x.go'], reason: 'merge-tree conflict' } }
    if ((o.trunkMovedOnce && !W.moved.has(id) && W.moved.add(id)) || R() < (o.pTrunkMoved || 0)) { W.trunk = newSha(); W.outside++; return { status: 'CONFLICT', trunk_sha: W.trunk, files: [], reason: 'trunk moved' } }
    W.trunk = newSha()
    W.merged.add(id)
    W.mergedSha[id] = sha
    W.order.push(id)
    S.order.push(id)
    if (!t) { W.red = false; S.fixMerged = true }
    if (o.lostMergeRe && new RegExp(o.lostMergeRe).test(L) && !W.lost.has(id)) { W.lost.add(id); return null }
    return { status: 'MERGED', trunk_sha: W.trunk, files: [], reason: '' }
  }

  async function health() {
    await C.sleep(3)
    if (R() < (o.pHealthErr || 0)) return { status: 'ERROR', trunk_sha: W.trunk, failing: [], summary: 'sim timeout' }
    if (o.alwaysRed && W.merged.size > S.start) W.red = true
    if (o.redOnce && !S.redDone && S.order.length) { S.redDone = true; W.red = true }
    if (o.redAfterStop && S.marks.stopped) W.red = true
    if (!W.red && R() < (o.pHealthRed || 0)) W.red = true
    if (W.red) S.firstRed = Math.min(S.firstRed, S.calls)
    if (!W.red && S.fixMerged && S.greenAfterFix === Infinity) S.greenAfterFix = S.calls
    return { status: W.red ? 'RED' : 'GREEN', trunk_sha: W.trunk, failing: W.red ? ['check'] : [], summary: 'sim' }
  }

  async function agent(prompt, opt) {
    if (C.now > S.haltVt) fail(`${opt.label}: started after ${cfg.haltAfterNulls} agents in a row returned no result`)
    const r = await answer(prompt, opt)
    S.nullRun = r ? 0 : S.nullRun + 1
    if (S.nullRun >= cfg.haltAfterNulls && S.haltVt === Infinity) S.haltVt = C.now
    return r
  }
  async function answer(prompt, opt) {
    W.calls++
    S.calls++
    const L = opt.label
    if (S.labels[L]) fail(`duplicate label ${L}`)
    S.labels[L] = (S.labels[L] || 0) + 1
    const c = classify(L)
    if (!c) { fail('unknown label ' + L); return null }
    if (opt.isolation !== 'worktree' || !MODELS.includes(opt.model) || !EFFORTS.includes(opt.effort) || !opt.schema || opt.schema.type !== 'object' || typeof opt.phase !== 'string') fail(`${L}: bad opts ${JSON.stringify({ ...opt, schema: !!opt.schema })}`)
    if (!c.role || opt.model !== c.role.model || opt.effort !== c.role.effort) fail(`${L}: model ${opt.model}/${opt.effort}, role wants ${c.role && c.role.model}/${c.role && c.role.effort}`)
    if (!KEYRE[c.key].test(prompt)) fail(`${L}: the prompt is not the ${c.key} prompt`)
    for (const bad of ['{{', 'undefined', '[object Object]', 'NaN']) if (prompt.includes(bad)) fail(`${L}: the prompt contains ${bad}`)
    for (const m of prompt.matchAll(BRIEF_RE)) if (m[1] !== c.id) fail(`${L}: the prompt briefs ${m[1]}`)
    COVER.add(c.key)
    if (c.key !== 'merge') COVER.add('common')
    if (o.noWorktreeAt === S.calls) { await C.sleep(1); return { status: 'BLOCKED', branch: '', commit: '', gate_passed: false, summary: 'NO-WORKTREE', rulings: [] } }
    if (S.calls > (o.nullAfter ?? Infinity) || (o.nullAfterStop && S.marks.stopped) || (o.nullRe && new RegExp(o.nullRe).test(L)) || R() < (o.pNull || 0)) { await C.sleep(1); return null }
    if (c.key === 'health') return health()
    if (c.kind === 'merge') return merger(c.id, L, prompt)
    if (c.kind === 'review' || c.kind === 'adversarial') return reviewer(c.id, c.kind, +L.split(' ')[2], L, prompt, c.key)
    return implementer(c.id, c.kind, L, prompt)
  }

  const body = bodyOf(generate(IN.template, { tasks, done, cfg, prompts: IN.prompts }))
  const fn = new AsyncFunction('agent', 'log', 'phase', 'args', 'budget', body)
  let out
  try { out = await C.run(fn(agent, () => {}, () => {}, undefined, { total: null })) } catch (e) { fail('script threw: ' + (e && e.stack || e)) }
  const res = out && out.value
  if (out && out.hang) fail(`HANG at virtual time ${out.vtime}: no timer pending and the script never returned`)
  if (out && !out.hang && !res) fail('the script returned no result')
  const scope = tasks.filter(t => t.wave <= cfg.maxWave && (!cfg.onlyTasks || cfg.onlyTasks.includes(t.id)))
  if (res) {
    for (const k of RESULT_KEYS) if (!(k in res)) fail(`the result has no ${k}`)
    const st = id => TERMINAL.find(s => (res.byStatus[s] || []).includes(id))
    for (const s of Object.keys(res.byStatus)) if (!TERMINAL.includes(s)) fail(`status ${s} is not terminal`)
    for (const t of scope) if (!st(t.id)) fail(`${t.id} has no terminal status`)
    if (!STOPS.includes(res.stop)) fail(`stop ${res.stop} is not a stop label`)
    const resMerged = new Set(res.byStatus.merged || [])
    for (const id of resMerged) if (!W.merged.has(id)) fail(`${id} reported merged, the world disagrees`)
    for (const id of W.merged) if ((byId.has(id) || S.fixIds.has(id)) && !resMerged.has(id)) fail(`${id} merged in the world, not in the result`)
    if (res.trunkTip !== W.trunk) fail(`trunkTip ${String(res.trunkTip).slice(-4)} != world ${W.trunk.slice(-4)}`)
    // Each mergeLog entry names the merged tip, so `gang ledger check <id> <sha>` is CLEAN on it (plan 05, addition C).
    for (const x of res.mergeLog) {
      const t = byId.get(x.id)
      const lenses = t && t.risk === 'high' ? ['review', 'adversarial'] : ['review']
      const bad = lenses.filter(l => W.ledger.get(`${x.id} ${x.sha} ${l}`) !== 'CLEAN')
      if (bad.length) fail(`mergeLog ${x.id} ${String(x.sha).slice(-4)}: the ledger is not CLEAN for ${bad.join(', ')}`)
      if (!/^[0-9a-f]{40}$/.test(x.trunk || '')) fail(`mergeLog ${x.id}: no trunk SHA`)
    }
    if (JSON.stringify([...res.fixTasks].sort()) !== JSON.stringify([...S.fixIds].sort())) fail(`fixTasks ${res.fixTasks} != repair agents ${[...S.fixIds]}`)
    for (const [k, v] of Object.entries(o.expect || {})) {
      const got = k === 'stop' ? res.stop : k === 'fixes' ? res.fixTasks.length : (res.counts[k] || 0)
      if (got !== v) fail(`expected ${k}=${v}, got ${got}`)
    }
    if (o.check) for (const e of o.check({ res, S, W, scope, cfg })) fail(e)
  }
  return { res, vtime: out ? out.vtime : -1, S }
}

async function scenario(name, seed, segs) {
  const R = rng(seed)
  const W = { trunk: 'a'.repeat(40), seq: 0, merged: new Set(), mergedSha: {}, order: [], tips: {}, gateOk: {}, ledger: new Map(),
    red: false, conflicted: new Set(), lost: new Set(), moved: new Set(), outside: 0, calls: 0, errors: [], taskLine: {} }
  const segsOut = []
  for (let i = 0; i < segs.length; i++) segsOut.push(await runSegment(W, R, segs[i], i))
  const sig = JSON.stringify(segsOut.map(s => s.res && [s.res.counts, s.res.mergeLog.map(x => x.id), s.res.stop, s.res.fixTasks, s.vtime])) + ` ${W.calls} ${W.trunk}`
  return { name, W, segs: segsOut, sig }
}

// ---- scenario helpers -------------------------------------------------------------------------------
const noStartAfter = (kind, nth = 1) => ({ S }) => {
  const at = (S.marks[kind] || [])[nth - 1]
  if (at === undefined) return [`no ${kind} mark number ${nth}`]
  const late = S.newCalls.filter(x => x.vt > at)
  return late.length ? [`started ${late.map(x => x.id).join(', ')} after the ${kind} answer at ${at}`] : []
}
const statusIs = (id, want) => ({ res }) => ((res.byStatus[want] || []).includes(id) ? [] : [`${id} is not ${want}`])
const all = (...fs) => ctx => fs.flatMap(f => f(ctx))
// After a stop, every task that had not started and does not depend on a blocked task ends not-started.
const unstartedStay = (kind, nth, blocked) => ({ res, S }) => {
  const at = (S.marks[kind] || [])[nth - 1]
  if (at === undefined) return [`no ${kind} mark number ${nth}`]
  const started = new Set(S.newCalls.filter(x => x.vt <= at).map(x => x.id))
  const down = new Set(blocked)
  for (let grew = true; grew;) {
    grew = false
    for (const t of IN.tasks) if (!down.has(t.id) && t.deps.some(d => down.has(d))) { down.add(t.id); grew = true }
  }
  const ns = new Set(res.byStatus['not-started'] || [])
  const bad = IN.tasks.filter(t => !down.has(t.id) && !started.has(t.id) && !ns.has(t.id)).map(t => t.id)
  return bad.length ? [`${bad.join(', ')} started or ended otherwise after the stop`] : []
}
// The named tasks are blocked, and any other blocked task is a merge_after waiter of a blocked target.
const blockedAre = ids => ({ res }) => {
  const b = res.byStatus.blocked || []
  const hard = new Set(b.concat(res.byStatus['blocked-dep'] || []))
  const extra = b.filter(id => !ids.includes(id) && !((IN.tasks.find(t => t.id === id) || { mergeAfter: [] }).mergeAfter.some(m => hard.has(m))))
  return ids.filter(id => !b.includes(id)).map(id => `${id} is not blocked`).concat(extra.map(id => `${id} is blocked with no blocked target`))
}
function ancestors(tasks) {
  const byId = new Map(tasks.map(t => [t.id, t]))
  const memo = new Map()
  const anc = id => {
    if (memo.has(id)) return memo.get(id)
    const s = new Set()
    memo.set(id, s)
    for (const d of byId.get(id).deps) { s.add(d); for (const x of anc(d)) s.add(x) }
    return s
  }
  return anc
}
// (head, mid, waiter): waiter merges after mid, mid depends on head, and the waiter's deps reach neither.
function findTriples(tasks, k) {
  const byId = new Map(tasks.map(t => [t.id, t]))
  const anc = ancestors(tasks)
  const out = []
  for (const w of tasks) {
    for (const m of w.mergeAfter) {
      const h = byId.get(m).deps.find(d => !anc(w.id).has(d))
      if (h && !anc(w.id).has(m) && out.length < k) out.push({ h, m, w: w.id })
    }
  }
  return out
}

function buildScenarios(base) {
  const T = IN.tasks
  const N = T.length
  const ids = T.map(t => t.id)
  const anc = ancestors(T)
  const roots = T.filter(t => !t.deps.length).sort((a, b) => b.chain - a.chain)
  const top = roots[0]
  const low = roots[roots.length - 1]
  const mid = T.filter(t => t.deps.length).sort((a, b) => b.chain - a.chain)[0] // a task that starts after another merges
  const cap = IN.cfg.maxImplementers
  const order = base.W.order
  const K = Math.min(15, Math.floor(N / 3))
  // Agent counts scale with the plan and stay inside the run: 60 of 172 agents for the synthetic graph.
  const at = f => Math.max(1, Math.min(base.W.calls - 2, Math.floor(base.W.calls * f)))
  const triples = findTriples(T, 4)
  const w3 = T.filter(t => t.wave <= 3).length
  const high = T.filter(t => t.risk === 'high')
  let ciN = 1
  while (ids.includes('CI' + ciN)) ciN++
  const fixId = 'CI' + ciN // the coordinator's fix task after a red wave PR (Contract 0, item 13)
  const withFix = T.concat([{ id: fixId, title: 'fix the red checks of a wave PR', branch: 'gang/' + fixId, deps: [], mergeAfter: [], wave: 1,
    chain: Math.max(...T.map(t => t.chain)) + 10, risk: 'low', locks: [], roles: T[0].roles }])
  const CHAOS = { pConflict: 0.2, pClean: 0.5, pBlocked: 0.01, pRed: 0.02, pNull: 0.02, pHealthRed: 0.03, pGateFail: 0.1, pNotRun: 0.05,
    pHealthErr: 0.02, pTrunkMoved: 0.03, pNoMutation: 0.03, pNoLedger: 0.03 }
  const S = []
  const add = (name, seed, segs, skip) => S.push({ name, seed, segs: Array.isArray(segs) ? segs : [segs], skip })
  const every = (what, label) => ({ S: s }) => T.filter(t => !s.labels[label(t)]).map(t => `${t.id} had no ${what}`)
  add('happy path', 1, { pConflict: 0.15, pClean: 1, expect: { stop: 'DONE', merged: N, blocked: 0 } })
  add('harsh reviews', 5, { pConflict: 0.15, pClean: 0.5 })
  add('gate failures', 6, { pGateFail: 0.25, pClean: 0.9 })
  add('gate failures, seed 8', 8, { pGateFail: 0.25, pClean: 0.9 })
  add('every first review finds a blocker', 10, { pClean: 0, pCleanLater: 1, expect: { stop: 'DONE', merged: N },
    check: every('fix round', t => `${t.id} fix 1 (round 1)`) })
  add('every first merge conflicts', 11, { pConflict: 1, pClean: 1, expect: { stop: 'DONE', merged: N },
    check: all(every('sync', t => `${t.id} sync 1`), every('merge review', t => `${t.id} review 2 merge`)) })
  add('review gate not run', 7, { pNotRun: 0.3, pClean: 1, pCleanLater: 1, expect: { stop: 'DONE', merged: N } })
  add('review checks incomplete (mutation, ledger)', 9, { pNoMutation: 0.15, pNoLedger: 0.15, pClean: 1, pCleanLater: 1, expect: { stop: 'DONE', merged: N } })
  add('surviving mutants with no blocker get a fix round', 44, { pSurvivors: 0.5, pClean: 1, pCleanLater: 1, expect: { stop: 'DONE', merged: N },
    check: ({ S: s }) => (s.survived.size ? [...s.survived].map(k => k.split(' ')[0]).filter(id => !s.labels[`${id} fix 1 (round 1)`]).map(id => `${id} merged a tip with a surviving mutant`) : ['no review reported a survivor']) })
  add(`resume with ${K} merged`, 3, { done: order.slice(0, K), pConflict: 0.1, pClean: 1, expect: { stop: 'DONE', merged: N } })
  add(`pilot ${low.id} only`, 2, { cfg: { onlyTasks: [low.id] }, pClean: 1, expect: { stop: 'DONE', merged: 1 } })
  add('wave limit 3', 4, { cfg: { maxWave: 3 }, pClean: 1, expect: { stop: 'DONE', merged: w3 } })
  // ---- interruptions and stops
  add(`usage limit after ${at(0.35)} agents`, 20, { nullAfter: at(0.35), pClean: 1, pConflict: 0.1, expect: { stop: 'INTERRUPTED', blocked: 0, 'blocked-dep': 0 } })
  add('usage limit, then resume from git', 40, [
    { nullAfter: at(0.35), pClean: 1, expect: { stop: 'INTERRUPTED', blocked: 0 } },
    { done: 'world', pClean: 1, pConflict: 0.1, expect: { stop: 'DONE', merged: N } }])
  add('agent outside a worktree', 29, { noWorktreeAt: at(0.15), pClean: 1, expect: { stop: 'STOPPED' } })
  // In-flight work drains: only a merge_after waiter whose target never merged may end interrupted.
  const drained = ({ res }) => {
    const ok = new Set(res.byStatus.merged || [])
    const bad = (res.byStatus.interrupted || []).filter(id => T.find(t => t.id === id).mergeAfter.every(m => ok.has(m)))
    return bad.length ? [`${bad.join(', ')} did not drain`] : []
  }
  if (mid) {
    add(`stop line when ${mid.id} starts`, 32, { stopRe: `^${mid.id} new`, pClean: 1, expect: { stop: 'STOPPED' },
      check: all(noStartAfter('stopped'), unstartedStay('stopped', 1, []), drained, statusIs(mid.id, 'not-started')) })
  } else add('stop line', 32, {}, 'the plan has no task that waits for another')
  if (roots.length >= 2 && cap >= 2) {
    add('usage limit while a stop line drains', 39, { stopRe: `^${roots[1].id} new`, nullAfterStop: true, cfg: { haltAfterNulls: 2 }, pClean: 1, expect: { stop: 'INTERRUPTED' } })
  } else add('usage limit while a stop line drains', 39, {}, 'needs two root tasks running at once')
  add(`blocked on the critical chain (${top.id})`, 33, { blockRe: `^${top.id} new`, pClean: 1, expect: { stop: 'BLOCKED' },
    check: all(blockedAre([top.id]), noStartAfter('blocked'), unstartedStay('blocked', 1, [top.id])) })
  if (roots.length >= 2 && low.chain < top.chain && T.some(t => t.deps.length && !anc(t.id).has(low.id))) {
    add(`blocked off the critical chain (${low.id}) keeps going`, 34, { blockRe: `^${low.id} new`, cfg: { blockedStop: 99 }, pClean: 1,
      expect: { stop: 'BLOCKED', 'not-started': 0, interrupted: 0 },
      check: all(blockedAre([low.id]), ({ S: s }) => (s.newCalls.some(x => x.vt > s.marks.blocked[0]) ? [] : ['no task started after the block'])) })
  } else add('blocked off the critical chain keeps going', 34, {}, 'needs a root off the critical chain and a later task that does not depend on it')
  if (roots.length >= 4) {
    const three = roots.slice(1, 4).map(t => t.id)
    add(`blocked count reaches blockedStop (${three.join(', ')})`, 35, { blockRe: `^(${three.join('|')}) new`, pClean: 1, expect: { stop: 'BLOCKED' },
      check: all(blockedAre(three), noStartAfter('blocked', 3), unstartedStay('blocked', 3, three)) })
  } else add('blocked count reaches blockedStop', 35, {}, 'the plan has fewer than 4 root tasks')
  // ---- merge_after waiters and merge races
  for (let i = 0; i < 4; i++) {
    const tr = triples[i]
    if (!tr) { add(`head never implements, chain ${i + 1}`, 23 + i, {}, 'the plan has too few merge_after chains'); continue }
    add(`${tr.h} never implements (${tr.w} waits on ${tr.m})`, 23 + i, { nullRe: `^${tr.h} new`, pClean: 1, expect: { stop: 'INTERRUPTED' },
      check: all(statusIs(tr.h, 'interrupted'), statusIs(tr.m, 'not-started'), statusIs(tr.w, 'interrupted')) })
  }
  if (triples[0]) {
    const m = triples[0].m
    const waiters = T.filter(t => t.mergeAfter.includes(m) && !anc(t.id).has(m)).map(t => t.id) // every one parks, not only the first
    add(`merge-after outside scope (${waiters.join(', ')} need ${m})`, 30, { cfg: { onlyTasks: ids.filter(x => x !== m) }, pClean: 1,
      expect: { stop: 'INTERRUPTED', parked: waiters.length }, check: all(...waiters.map(w => statusIs(w, 'parked'))) })
  } else add('merge-after outside scope', 30, {}, 'the plan has no merge_after chain')
  add('lost merge results', 28, { lostMergeRe: 'merge 1$', pClean: 1, expect: { stop: 'DONE', merged: N } })
  add('trunk moved during every first merge', 31, { trunkMovedOnce: true, pClean: 1, expect: { stop: 'DONE', merged: N },
    check: ({ S: s, W }) => (Object.keys(s.labels).some(l => / sync /.test(l)) ? ['a lost race started a sync agent'] : W.outside === N ? [] : [`the trunk moved ${W.outside} times, not ${N}`]) })
  // ---- trunk health, review lenses, relaunch, chaos
  add('TRUNK_RED on a green trunk resumes', 43, { redFirstNew: true, pClean: 1, expect: { stop: 'DONE', merged: N, fixes: 0 },
    check: ({ S: s }) => (Object.keys(s.labels).some(l => / resume 1$/.test(l)) ? [] : ['no resume agent ran']) })
  add('repair loop: health red often', 21, { redOnce: true, pHealthRed: 0.15, pConflict: 0.1, pClean: 0.9,
    check: ({ res }) => (res.fixTasks.length ? [] : ['no repair task ran']) })
  if (mid) {
    add('trunk stays red, repairs blocked', 22, { alwaysRed: true, blockFixes: true, pClean: 1, expect: { stop: 'TRUNK_RED', fixes: 2 },
      check: ({ S: s }) => (s.mergeCalls.some(x => x.plan && x.i > s.firstFix) ? ['a task merged while a repair owned the trunk, or after TRUNK_RED'] : []) })
    add('a red trunk holds merges until the repair is green', 38, { redOnce: true, slowFix: 40, pClean: 1, expect: { stop: 'DONE', merged: N + 1, fixes: 1 },
      check: ({ S: s }) => {
        const held = s.mergeCalls.filter(x => x.plan && x.i > s.firstFix && x.i < s.greenAfterFix)
        const waited = s.mergeCalls.filter(x => x.plan && x.i > s.greenAfterFix)
        return held.length ? [`merged ${held.map(x => x.id)} during the repair`] : waited.length ? [] : ['no merge waited for the repair']
      } })
  } else {
    add('trunk stays red, repairs blocked', 22, {}, 'the plan has no task that waits for another')
    add('a red trunk holds merges until the repair is green', 38, {}, 'the plan has no task that waits for another')
  }
  if (roots.length >= 2 && cap >= 2) {
    add(`a stop line during a failed repair starts no next attempt (${roots[1].id})`, 45, { redOnce: true, slowFix: 40, blockFixes: true,
      stopInRepairRe: `^${roots[1].id} new`, pClean: 1, expect: { stop: 'STOPPED', fixes: 1 }, check: noStartAfter('stopped') })
  } else add('a stop line during a failed repair starts no next attempt', 45, {}, 'needs two root tasks running at once')
  const late = order[Math.floor(N * 0.4)] // starts mid-run, while other tasks are in review
  if (roots.length >= 3 && cap >= 3 && late !== top.id) {
    add(`a red trunk during a drain ends merges (stop at ${late})`, 41, { stopRe: `^${late} new`, redAfterStop: true, pClean: 1, expect: { stop: 'STOPPED', fixes: 0 },
      check: ({ S: s }) => (s.firstRed === Infinity ? ['the trunk never went red'] : s.mergeCalls.some(x => x.plan && x.i > s.firstRed) ? ['a task merged onto a red trunk'] : []) })
  } else add('a red trunk during a drain ends merges', 41, {}, 'needs three root tasks running at once')
  if (high.length) {
    add(`risk high needs both lenses (${high.length} tasks)`, 36, { advBlockFirst: true, pClean: 1, pCleanLater: 1, expect: { stop: 'DONE', merged: N },
      check: ({ S: s }) => high.flatMap(t => (s.labels[`${t.id} fix 1 (round 1)`] && s.labels[`${t.id} adversarial 2`] ? [] : [`${t.id} merged without a second adversarial review`])) })
  } else add('risk high needs both lenses', 36, {}, 'the plan has no risk high task')
  add(`PR_RED relaunch: ${fixId} first, then the rest`, 37, [
    { tasks: withFix, done: order.slice(0, K), cfg: { onlyTasks: [fixId] }, redOnce: true, pClean: 1, expect: { stop: 'DONE', fixes: 1 },
      check: ({ res, S: s }) => (s.order[0] !== fixId ? [`${s.order[0]} merged before ${fixId}`] : res.fixTasks[0] !== 'F01' ? [`the repair task is ${res.fixTasks[0]}, not F01`] : []) },
    { tasks: withFix, done: 'world', pClean: 1, pConflict: 0.1, expect: { stop: 'DONE', merged: N + 1 } }])
  add('repair ids skip an F id already done', 42, { done: ['F01'], cfg: { nextFixNumber: 1 }, redOnce: true, pClean: 1, expect: { stop: 'DONE', merged: N + 1, fixes: 1 },
    check: ({ res }) => (res.fixTasks[0] === 'F02' ? [] : [`the repair task is ${res.fixTasks[0]}, not F02`]) })
  for (let s = 10; s < 18; s++) add(`chaos seed ${s}`, s, { ...CHAOS })
  return S
}

// ---- checks that need no scenario ----------------------------------------------------------------
function staticChecks() {
  const errs = []
  if (IN.table) {
    for (const k of KEYS) {
      const used = usedVars(IN.prompts[k]).join(' ')
      if ((IN.table[k] || []).join(' ') !== used) errs.push(`roles.md: the table lists ${k} as [${IN.table[k]}], the block uses [${used}]`)
    }
  }
  for (const k of KEYS) {
    const want = [...(VARS[k] || [])].sort().join(' ')
    if (usedVars(IN.prompts[k]).join(' ') !== want) errs.push(`VARS.${k} is [${want}], the prompt uses [${usedVars(IN.prompts[k])}]`)
  }
  // `gang gate run <id>` runs the task's VERIFY commands with the gate's slots, timeout, and locks,
  // so a prompt names those commands only to say that the gate runs them.
  for (const k of KEYS) {
    for (const s of IN.prompts[k].match(/[^.\n]*\bcommands? that VERIFY lists[^.\n]*/g) || []) {
      if (!/\bgate already runs\b/.test(s)) errs.push(`${k}: "${s.trim()}" runs the VERIFY commands outside the gate`)
    }
  }
  return errs
}
async function guardChecks() {
  const errs = []
  const try1 = async (what, d, sum, want) => {
    let calls = 0
    try {
      const fn = new AsyncFunction('agent', 'log', 'phase', 'args', 'budget', bodyOf(generate(IN.template, d, sum)))
      await fn(async () => { calls++; return null }, () => {}, () => {}, undefined, {})
      errs.push(`${what}: the engine did not throw`)
    } catch (e) {
      if (!want.test(String(e && e.message))) errs.push(`${what}: threw "${e && e.message}"`)
      if (calls) errs.push(`${what}: ${calls} agents ran before the guard`)
    }
  }
  const d = { tasks: IN.tasks, done: [], cfg: { ...IN.cfg, count: IN.tasks.length }, prompts: IN.prompts }
  await try1('bad checksum', d, fnv(JSON.stringify(IN.tasks)) + 1, /checksum/)
  await try1('task count', { ...d, cfg: { ...d.cfg, count: IN.tasks.length + 1 } }, undefined, /expected/)
  await try1('unknown prompt variable', { ...d, prompts: { ...IN.prompts, implement_new: IN.prompts.implement_new + ' {{nope}}' } }, undefined, /unknown variable/)
  await try1('missing prompt', { ...d, prompts: { ...IN.prompts, repair: undefined } }, undefined, /missing prompt/)
  await try1('unknown DONE id', { ...d, done: ['X99'] }, undefined, /unknown task/)
  await try1('raw model id', { ...d, tasks: IN.tasks.map((t, i) => (i ? t : { ...t, roles: { ...t.roles, review: { model: 'claude-opus-4', effort: 'high' } } })) }, undefined, /bad task entry/)
  return errs
}

// ---- main ------------------------------------------------------------------------------------------
function line(r) {
  const last = r.segs[r.segs.length - 1]
  const res = last.res
  const stops = r.segs.map(s => (s.res ? s.res.stop : '-')).join('/')
  const maxImpl = Math.max(...r.segs.map(s => s.S.maxImpl))
  console.log(`${r.W.errors.length ? 'FAIL' : 'ok  '} ${r.name}: segments ${r.segs.length}, vtime ${last.vtime}, agents ${r.W.calls}, max impl ${maxImpl}, stop ${stops}, counts ${res ? JSON.stringify(res.counts) : 'none'}, fixes ${res ? res.fixTasks.length : '-'}`)
  for (const e of r.W.errors) console.log('     ' + e)
}

let bad = 0
let ran = 0
const statics = staticChecks()
console.log(`${statics.length ? 'FAIL' : 'ok  '} prompts: ${KEYS.length} keys, the roles.md table, the blocks, and VARS agree`)
for (const e of statics) console.log('     ' + e)
if (statics.length) bad++
const guards = await guardChecks()
console.log(`${guards.length ? 'FAIL' : 'ok  '} guards: bad checksum, task count, unknown variable, missing prompt, unknown DONE id, raw model id`)
for (const e of guards) console.log('     ' + e)
if (guards.length) bad++
const base = await scenario('base order', 1, [{ pConflict: 0.15, pClean: 1 }])
if (base.W.errors.length || base.W.order.length !== IN.tasks.length) { console.log('FAIL base order run'); for (const e of base.W.errors) console.log('     ' + e); process.exit(1) }
for (const sc of buildScenarios(base)) {
  if (only && !sc.name.includes(only)) continue
  if (sc.skip) { console.log(`skip ${sc.name}: ${sc.skip}`); continue }
  ran++
  const a = await scenario(sc.name, sc.seed, sc.segs)
  const b = await scenario(sc.name, sc.seed, sc.segs)
  if (a.sig !== b.sig) a.W.errors.push('NON-DETERMINISTIC: two runs with the same seed differ')
  line(a)
  if (a.W.errors.length) bad++
}
if (!only) {
  const miss = KEYS.filter(k => !COVER.has(k) && (k !== 'adversarial' || IN.tasks.some(t => t.risk === 'high')))
  console.log(`${miss.length ? 'FAIL' : 'ok  '} prompt coverage: ${KEYS.length - miss.length} of ${KEYS.length} keys rendered${miss.length ? ', missing ' + miss.join(', ') : ''}`)
  if (miss.length) bad++
}
console.log(bad ? `\n${bad} check(s) FAILED` : `\nall ${ran} scenarios passed, each run twice with identical results`)
process.exitCode = bad ? 1 : 0
