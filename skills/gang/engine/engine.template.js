export const meta = {
  name: 'gang-engine',
  description: 'gang: implement, review, and merge the ready tasks of a plan into the integration branch (no push, no PR)',
  whenToUse: 'Launched by the /monet:gang coordinator on the run/engine.js that gang gen wrote',
  phases: [
    { title: 'Tasks', detail: 'implement, review, fix, and merge each ready task' },
    { title: 'Health', detail: 'health runs on the integration branch, and repair tasks' },
  ],
}

// ---- data that gang gen writes (Contract 8) ------------------------------------------
const TASKS_JSON = /*@@TASKS_JSON@@*/null
const TASKSUM = /*@@TASKSUM@@*/null
const DONE = /*@@DONE@@*/null
const CFG = /*@@CFG@@*/null
const PROMPTS = /*@@PROMPTS@@*/null

// The variables that each prompt key gets. It must equal the table in reference/roles.md.
const VARS = {
  "common": ["gang", "repo"],
  "implement_new": ["gang", "id", "integration"],
  "implement_resume": ["gang", "id", "integration", "redSummary", "trunkVerdict", "trunkSha"],
  "implement_fix": ["gang", "id", "integration", "tip", "round", "maxRounds", "review", "gateFailure"],
  "implement_sync": ["gang", "id", "branch", "integration", "tip", "trunk", "files"],
  "review_full": ["gang", "id", "branch", "integration", "tip", "n", "runTag", "retryOf"],
  "review_fix": ["gang", "id", "branch", "integration", "tip", "n", "runTag", "retryOf", "prevTip", "prevReview"],
  "review_merge": ["gang", "id", "branch", "integration", "tip", "n", "runTag", "retryOf", "prevTip"],
  "adversarial": ["gang", "id", "branch", "integration", "tip", "n", "runTag", "retryOf"],
  "merge": ["gang", "id", "tip"],
  "health": ["gang", "integration"],
  "repair": ["gang", "id", "integration", "redSha", "failing", "healthSummary", "recent"]
}
const ROLES = ['implement', 'implement_hard', 'review', 'review_adversarial', 'fix', 'sync', 'merge', 'health', 'repair', 'plan_draft', 'plan_review']
const MODELS = ['opus', 'sonnet', 'haiku', 'fable']
const EFFORTS = ['low', 'medium', 'high', 'xhigh', 'max']

// ---- guards: throw before any spawn ----------------------------------------------------
function fnv(str) { let h = 0x811c9dc5; for (let i = 0; i < str.length; i++) { h ^= str.charCodeAt(i); h = Math.imul(h, 0x01000193) >>> 0 } return h }
if (typeof TASKS_JSON !== 'string' || fnv(TASKS_JSON) !== TASKSUM) throw new Error('data guard: the task table checksum does not match')
if (!CFG || typeof CFG !== 'object' || !Array.isArray(DONE) || !PROMPTS || typeof PROMPTS !== 'object') throw new Error('data guard: generator data missing')
const TASKS = JSON.parse(TASKS_JSON)
if (!Array.isArray(TASKS) || TASKS.length !== CFG.count) throw new Error(`data guard: expected ${CFG.count} tasks, got ${Array.isArray(TASKS) ? TASKS.length : 'none'}`)
const okRole = r => !!r && MODELS.includes(r.model) && EFFORTS.includes(r.effort)
for (const r of ['merge', 'health', 'repair']) if (!okRole(CFG.roles && CFG.roles[r])) throw new Error('data guard: bad run-level role ' + r)
const IDS = new Set(TASKS.map(t => t && t.id))
if (IDS.size !== TASKS.length) throw new Error('data guard: duplicate task ids')
TASKS.forEach((t, i) => {
  const bad = typeof t.id !== 'string' || typeof t.branch !== 'string' || !Array.isArray(t.deps) || !Array.isArray(t.mergeAfter)
    || !Array.isArray(t.locks) || typeof t.wave !== 'number' || typeof t.chain !== 'number' || !['low', 'high'].includes(t.risk)
    || !t.roles || ROLES.some(r => !okRole(t.roles[r])) || t.deps.concat(t.mergeAfter).some(d => !IDS.has(d))
  if (bad) throw new Error(`data guard: bad task entry ${i} (${t.id})`)
})
if (CFG.onlyTasks && CFG.onlyTasks.some(id => !IDS.has(id))) throw new Error('data guard: onlyTasks names an unknown task')

function fill(text, vars) {
  return text.replace(/\{\{([A-Za-z0-9_]+)\}\}/g, (m, k) => {
    if (!Object.prototype.hasOwnProperty.call(vars, k)) throw new Error(`prompt: unknown variable {{${k}}}`)
    return String(vars[k])
  })
}
for (const k of Object.keys(VARS)) {
  if (typeof PROMPTS[k] !== 'string') throw new Error('prompt guard: missing prompt ' + k)
  fill(PROMPTS[k], Object.fromEntries(VARS[k].map(v => [v, ''])))
}
const BASE = { gang: CFG.gang, repo: CFG.repo, integration: CFG.integration, runTag: CFG.runTag }
function render(key, v) {
  const all = { ...BASE, ...v }
  const pick = names => Object.fromEntries(names.map(n => {
    if (all[n] === undefined || all[n] === null) throw new Error(`prompt ${key}: no value for {{${n}}}`)
    return [n, all[n]]
  }))
  const body = fill(PROMPTS[key], pick(VARS[key]))
  return key === 'merge' ? body : fill(PROMPTS.common, pick(VARS.common)) + '\n\n---\n\n' + body
}

const short = s => (s || '').slice(0, 7)
const errText = e => String((e && e.message) || e)
const isSha = s => /^[0-9a-f]{40}$/.test(s || '')

// ---- concurrency helpers ----------------------------------------------------------------
function deferred() {
  let resolve, reject
  const promise = new Promise((a, b) => { resolve = a; reject = b })
  promise.catch(() => {})
  return { promise, resolve, reject, settled: false }
}
function settle(d, ok, val) { if (!d || d.settled) return; d.settled = true; if (ok) d.resolve(val); else d.reject(val) }
function makeMutex() {
  let tail = Promise.resolve()
  return fn => { const run = tail.then(fn, fn); tail = run.then(() => {}, () => {}); return run }
}
const mergeLock = makeMutex()
let wakeD = deferred()
function wake() { const d = wakeD; wakeD = deferred(); settle(d, true) }
const bg = new Set()
function track(p) {
  const q = p.catch(e => log('background error: ' + errText(e))).finally(() => { bg.delete(q); wake() })
  bg.add(q)
  return q
}

// ---- state --------------------------------------------------------------------------------
const tasks = new Map(TASKS.map((t, idx) => [t.id, { ...t, idx, kind: 'plan' }]))
const ALL = [...tasks.values()]
const inScope = t => !!t && t.kind === 'plan' && t.wave <= CFG.maxWave && (!CFG.onlyTasks || CFG.onlyTasks.includes(t.id))
const SCOPE = ALL.filter(inScope)
const status = new Map()
const reason = new Map()
const merged = new Map(ALL.map(t => [t.id, deferred()]))
const rec = {}
const R = id => (rec[id] = rec[id] || { events: [], rulings: [], n: {} })
const nth = (id, kind) => { const r = R(id); r.n[kind] = (r.n[kind] || 0) + 1; return r.n[kind] }
const note = (id, s) => { R(id).events.push(s) }
for (const id of DONE) {
  if (!tasks.has(id)) {
    if (/^F[0-9]+$/.test(id)) continue // a repair task of an earlier segment
    throw new Error('data guard: DONE lists unknown task ' + id)
  }
  status.set(id, 'merged'); settle(merged.get(id), true)
}
let implRunning = 0
let nullsInRow = 0
let halted = null // { kind, why }: no new agent at all
let stop = null // { kind, why }: no new task starts, in-flight work drains
let trunkTip = CFG.trunkStart
let lastGreen = null
let trunkRed = false // set with every repair start, cleared by a green health run
let nextFix = CFG.nextFixNumber
const mergeLog = []
const fixTasks = []
const healthLog = []
const HARD = new Set(['blocked', 'blocked-dep'])
const DEAD = new Set(['blocked', 'blocked-dep', 'interrupted', 'parked', 'not-started'])
const FIX_ROLES = Object.fromEntries(ROLES.map(r => [r, CFG.roles[r === 'merge' || r === 'health' ? r : 'repair']]))

const IMPL_SCHEMA = { type: 'object', properties: {
  status: { type: 'string', enum: ['DONE', 'DONE_WITH_CONCERNS', 'BLOCKED', 'TRUNK_RED', 'STOPPED'] },
  branch: { type: 'string' }, commit: { type: 'string' }, gate_passed: { type: 'boolean' },
  summary: { type: 'string' }, concerns: { type: 'string' }, rulings: { type: 'array', items: { type: 'string' } },
}, required: ['status', 'branch', 'commit', 'gate_passed', 'summary', 'rulings'] }
const REVIEW_SCHEMA = { type: 'object', properties: {
  reviewed_sha: { type: 'string' },
  blockers: { type: 'integer', minimum: 0 }, majors: { type: 'integer', minimum: 0 }, minors: { type: 'integer', minimum: 0 },
  findings: { type: 'array', items: { type: 'object', properties: {
    severity: { type: 'string', enum: ['blocker', 'major', 'minor'] }, summary: { type: 'string' } }, required: ['severity', 'summary'] } },
  gate: { type: 'string', enum: ['pass', 'fail', 'not-run'] },
  mutation_checked: { type: 'boolean' }, survivors: { type: 'integer', minimum: 0 }, ledger_recorded: { type: 'boolean' },
}, required: ['reviewed_sha', 'blockers', 'majors', 'minors', 'findings', 'gate', 'mutation_checked', 'survivors', 'ledger_recorded'] }
const MERGE_SCHEMA = { type: 'object', properties: {
  status: { type: 'string', enum: ['MERGED', 'ALREADY', 'CONFLICT', 'REFUSED'] },
  trunk_sha: { type: 'string' }, files: { type: 'array', items: { type: 'string' } }, reason: { type: 'string' },
}, required: ['status', 'trunk_sha', 'files', 'reason'] }
const HEALTH_SCHEMA = { type: 'object', properties: {
  status: { type: 'string', enum: ['GREEN', 'RED', 'ERROR'] },
  trunk_sha: { type: 'string' }, failing: { type: 'array', items: { type: 'string' } }, summary: { type: 'string' },
}, required: ['status', 'trunk_sha', 'failing', 'summary'] }

// ---- agent calls --------------------------------------------------------------------------
async function call(prompt, opts) {
  for (let attempt = 1; attempt <= 2; attempt++) {
    if (halted) return null
    const o = attempt === 1 ? opts : { ...opts, label: opts.label + ' (retry)' }
    let r = null
    try { r = await agent(prompt, o) } catch (e) { log(`${o.label}: agent error: ${errText(e)}`) }
    if (r) {
      nullsInRow = 0
      if (JSON.stringify(r).includes('NO-WORKTREE')) { halt('STOPPED', `${o.label} started in the main checkout instead of a worktree`); return null }
      return r
    }
    nullsInRow++
    log(`${o.label}: no result (${nullsInRow} in a row)`)
    if (nullsInRow >= CFG.haltAfterNulls) { halt('INTERRUPTED', `${nullsInRow} agents in a row returned no result, which usually means a usage limit`); return null }
  }
  return null
}
const A = (label, phaseName, schema, role) => ({ label, phase: phaseName, schema, model: role.model, effort: role.effort, isolation: 'worktree' })
const phaseOf = t => (t.kind === 'fix' ? 'Health' : 'Tasks')

function mark(t, st, why) {
  status.set(t.id, st)
  if (why) reason.set(t.id, why)
  settle(merged.get(t.id), false, new Error(`${t.id} ${st}`))
}
// Give every task that can no longer start a terminal status, so that no merge-after waiter hangs.
function doom() {
  let changed = true
  while (changed) {
    changed = false
    for (const t of SCOPE) {
      if (status.has(t.id)) continue
      const hard = t.deps.find(d => HARD.has(status.get(d)))
      const soft = t.deps.find(d => DEAD.has(status.get(d)) || (status.get(d) !== 'merged' && !inScope(tasks.get(d))))
      const end = halted || stop
      if (hard) { mark(t, 'blocked-dep', `depends on ${hard}, which is blocked`); changed = true }
      else if (soft) { mark(t, 'not-started', `depends on ${soft} (${status.get(soft) || 'outside this run'})`); changed = true }
      else if (end) { mark(t, 'not-started', `${end.kind}: ${end.why}`); changed = true }
    }
  }
}
function halt(kind, why) {
  if (halted) return
  halted = { kind, why }
  log(`HALT ${kind}: ${why}`)
  doom()
  wake()
}
function setStop(kind, why) {
  if (stop) return
  stop = { kind, why }
  log(`STOP ${kind}: ${why}`)
  doom()
  wake()
}
function onBlocked(t, why) {
  log(`${t.id} BLOCKED: ${why}`)
  const top = Math.max(...SCOPE.filter(x => status.get(x.id) !== 'merged').map(x => x.chain))
  const n = SCOPE.filter(x => status.get(x.id) === 'blocked').length
  if (t.chain >= top) setStop('BLOCKED', `${t.id} is blocked on the critical chain: ${why}`)
  else if (n >= CFG.blockedStop) setStop('BLOCKED', `${n} tasks are blocked (limit ${CFG.blockedStop})`)
}
function finish(t, st, why) {
  status.set(t.id, st)
  if (why) reason.set(t.id, why)
  note(t.id, st.toUpperCase() + (why ? ': ' + why : ''))
  if (st !== 'merged') {
    settle(merged.get(t.id), false, new Error(`${t.id} ${st}`))
    if (st === 'blocked' && t.kind === 'plan') onBlocked(t, why)
    doom()
  }
  wake()
  return st
}
// A failed step ends the task: blocked, or interrupted (a STOPPED answer also sets the stop).
function failed(t, r) {
  if (r.stopped) setStop('STOPPED', r.stopped)
  return r.blocked ? finish(t, 'blocked', r.blocked) : finish(t, 'interrupted', r.why || r.stopped)
}

function implRole(t, mode) {
  if (mode === 'fix') return t.roles.fix
  if (mode === 'sync') return t.roles.sync
  return t.risk === 'high' ? t.roles.implement_hard : t.roles.implement
}
function implPrompt(t, mode, ctx) {
  if (mode === 'new') {
    return t.kind === 'fix'
      ? render('repair', { id: t.id, redSha: t.redSha, failing: t.failing, healthSummary: t.healthSummary, recent: t.recent })
      : render('implement_new', { id: t.id })
  }
  if (mode === 'resume') return render('implement_resume', { id: t.id, redSummary: ctx.redSummary, trunkVerdict: ctx.trunkVerdict, trunkSha: ctx.trunkSha })
  if (mode === 'fix') {
    return render('implement_fix', { id: t.id, tip: ctx.tip, round: ctx.round, maxRounds: CFG.maxFixRounds,
      review: ctx.review || 'none', gateFailure: ctx.gateFailure || 'none' })
  }
  return render('implement_sync', { id: t.id, branch: t.branch, tip: ctx.tip, trunk: ctx.trunk, files: ctx.files })
}

// One implementer-type agent (new, resume, fix, or sync), with TRUNK_RED arbitration.
async function implementStep(t, mode, ctx) {
  for (let red = 0; ; red++) {
    const label = `${t.id} ${mode} ${nth(t.id, mode)}${mode === 'fix' ? ' (round ' + ctx.round + ')' : ''}`
    const r = await call(implPrompt(t, mode, ctx), A(label, phaseOf(t), IMPL_SCHEMA, implRole(t, mode)))
    if (!r) return { interrupted: true, why: halted ? halted.why : `${mode} agent returned no result twice` }
    if (Array.isArray(r.rulings)) R(t.id).rulings.push(...r.rulings.map(x => `${mode}: ${x}`))
    if (r.status === 'STOPPED') return { stopped: `stop line: ${r.summary || 'STOP'}` }
    if (r.status === 'TRUNK_RED') {
      note(t.id, `TRUNK_RED (${mode}): ${r.summary}`)
      if (t.kind === 'fix') return { blocked: `the repair task reported TRUNK_RED: ${r.summary}` }
      if (red >= CFG.maxTrunkRed) {
        return ctx.trunkVerdict === 'green'
          ? { blocked: `gate commands keep failing outside this task while ${CFG.integration} is green: ${r.summary}` }
          : { interrupted: true, why: `${CFG.integration} stayed red: ${r.summary}` }
      }
      const v = await arbitrate(t)
      if (v.stuck) return { interrupted: true, why: v.stuck }
      ctx = { ...ctx, trunkVerdict: v.verdict, trunkSha: v.sha, redSummary: r.summary || 'TRUNK_RED', tip: isSha(r.commit) ? r.commit : ctx.tip }
      mode = 'resume'
      continue
    }
    if (r.status === 'BLOCKED') return { blocked: r.summary || `${mode} agent reported BLOCKED` }
    if (!isSha(r.commit)) return { blocked: `${mode} agent returned no valid commit SHA (${r.commit})` }
    if (!r.gate_passed) note(t.id, `${mode} reports a failing gate: ${r.summary}`)
    return { tip: r.commit, gateOk: !!r.gate_passed, summary: r.summary || 'gate failed', res: r }
  }
}

// ---- review: one reviewer per lens, two lenses for risk high ------------------------------
const reviewPath = (t, lens, n) => `${CFG.dir}/run/tasks/${t.id}/${lens}-${CFG.runTag}-${n}.md`
const lensesOf = t => (t.risk === 'high' ? ['review', 'adversarial'] : ['review'])
// Clean matches the ledger's CLEAN (Contract 4): no blocker, major, or surviving mutant, and every check ran.
const isClean = (r, tip) => !!r && r.reviewed_sha === tip && r.blockers === 0 && r.majors === 0 && r.survivors === 0 && r.gate === 'pass' && r.mutation_checked === true && r.ledger_recorded === true
const incomplete = r => r.blockers === 0 && r.majors === 0 && r.survivors === 0 && r.gate !== 'fail' && !(r.gate === 'pass' && r.mutation_checked === true && r.ledger_recorded === true)

async function reviewOnce(t, tip, lens, mode, x, retryOf) {
  const n = nth(t.id, lens)
  const path = reviewPath(t, lens, n)
  const v = { id: t.id, branch: t.branch, tip, n, retryOf }
  let key = 'adversarial'
  let label = `${t.id} adversarial ${n}`
  let role = t.roles.review_adversarial
  if (lens === 'review') {
    key = 'review_' + mode
    label = `${t.id} review ${n}${mode === 'full' ? '' : ' ' + mode}`
    role = t.roles.review
    if (mode !== 'full') v.prevTip = x.prevTip
    if (mode === 'fix') v.prevReview = x.prevReview
  }
  const r = await call(render(key, v), A(label, phaseOf(t), REVIEW_SCHEMA, role))
  if (r) { r.path = path; r.lens = lens; if (!isSha(r.reviewed_sha)) r.reviewed_sha = tip }
  return r
}
// A review whose checks did not all run gets one more reviewer, told about the first one.
async function lensReview(t, tip, lens, mode, x) {
  let r = await reviewOnce(t, tip, lens, mode, x, 'none')
  if (r && incomplete(r)) {
    note(t.id, `${lens} at ${short(tip)} did not finish its checks (gate ${r.gate}, mutation ${r.mutation_checked}, ledger ${r.ledger_recorded}): one more`)
    r = await reviewOnce(t, tip, lens, mode, x, r.path)
  }
  return r
}

// Leaves only when the last implementer-type agent reported a passing gate AND every lens is clean.
async function qualityLoop(t, tip, gateOk, gateSummary, mode, prevTip) {
  let rs = []
  let reviewed = false
  let prev = prevTip
  for (let round = 1; ; round++) {
    if (gateOk) {
      const rmode = reviewed ? 'fix' : mode
      const main = rs.find(r => r.lens === 'review')
      rs = await Promise.all(lensesOf(t).map(l => lensReview(t, tip, l, rmode, { prevTip: prev, prevReview: main && main.path })))
      if (rs.some(r => !r)) return { interrupted: true, why: halted ? halted.why : 'a reviewer returned no result twice' }
      reviewed = true
      prev = tip // a later fix review diffs from the tip that this review saw
      for (const r of rs) if (r.reviewed_sha !== tip) note(t.id, `${r.lens} saw ${short(r.reviewed_sha)}, not ${short(tip)}`)
      const sum = rs.map(r => `${r.lens} ${r.blockers}b ${r.majors}m gate ${r.gate}`).join(', ')
      if (rs.every(r => isClean(r, tip))) { note(t.id, `clean (${rmode}) at ${short(tip)}: ${sum}`); return { tip } }
      note(t.id, `not clean (${rmode}) at ${short(tip)}: ${sum}`)
    }
    if (round > CFG.maxFixRounds) {
      const top = gateOk
        ? rs.flatMap(r => r.findings || []).filter(f => f.severity !== 'minor').slice(0, 3).map(f => f.summary).join(' | ') || 'checks did not finish'
        : `gate fails: ${gateSummary}`
      return { blocked: `open problems after ${CFG.maxFixRounds} fix rounds: ${top}` }
    }
    const open = gateOk ? rs.filter(r => !isClean(r, tip)).map(r => r.path).join(' and ') : null
    const fx = await implementStep(t, 'fix', { round, tip, review: open, gateFailure: gateOk ? null : gateSummary })
    if (fx.interrupted || fx.blocked || fx.stopped) return fx
    tip = fx.tip
    gateOk = fx.gateOk
    gateSummary = fx.summary
  }
}

// ---- health, arbitration, and repair ---------------------------------------------------------
const healthCache = new Map()
let healthSeq = 0
function healthAt(sha, label) {
  if (!healthCache.has(sha)) {
    const n = ++healthSeq
    const p = call(render('health', {}), A(`health ${n} ${label} @${short(sha)}`, 'Health', HEALTH_SCHEMA, CFG.roles.health))
      .then(h => {
        if (!h || h.status === 'ERROR') { healthCache.delete(sha); if (h) log(`health ${label} @${short(sha)}: error (${h.summary})`); return h }
        h.sha = isSha(h.trunk_sha) ? h.trunk_sha : sha
        healthLog.push({ sha: short(h.sha), label, status: h.status })
        if (h.status === 'GREEN') { lastGreen = h.sha; trunkRed = false }
        log(`health ${label} @${short(h.sha)}: ${h.status}${h.failing && h.failing.length ? ' (' + h.failing.join(', ') + ')' : ''}`)
        return h
      })
    healthCache.set(sha, p)
  }
  return healthCache.get(sha)
}
const goodHealth = h => !!h && h.status === 'GREEN'

async function arbitrate(t) {
  const h = await healthAt(trunkTip, 'arbiter')
  if (!h) return { stuck: halted ? halted.why : 'the health run for TRUNK_RED returned no result' }
  if (h.status === 'ERROR') return { stuck: `the health run for TRUNK_RED could not run: ${h.summary}` }
  if (goodHealth(h)) { note(t.id, `arbiter: ${CFG.integration} green at ${short(h.sha)}`); return { verdict: 'green', sha: h.sha } }
  trunkRed = true
  const ok = await repair(h.sha, h)
  if (!ok) return { stuck: stop ? stop.why : halted ? halted.why : `${CFG.integration} is red and its repair failed` }
  note(t.id, `arbiter: ${CFG.integration} repaired to ${short(trunkTip)}`)
  return { verdict: 'repaired', sha: trunkTip }
}

// The merges after the trunk SHA `sha`, newest first, each with its merge commit on the trunk.
function mergesSince(sha) {
  const i = sha ? mergeLog.findIndex(m => m.trunk === sha) : -1
  return mergeLog.slice(i + 1).reverse().map(m => `${m.id} (${short(m.trunk)})`).join(', ') || `none in this run (see git log ${CFG.integration})`
}
function newFixId() {
  let id
  do { id = 'F' + String(nextFix++).padStart(2, '0') } while (tasks.has(id) || DONE.includes(id))
  return id
}

let repairing = null
function repair(redSha, report) {
  if (repairing) return repairing
  if (stop || halted) return Promise.resolve(false)
  const p = (async () => {
    let sha = redSha
    let rep = report
    for (let a = 1; a <= CFG.maxRepairAttempts; a++) {
      if (halted || stop) return false // a stop or a halt starts no further attempt
      if (trunkTip !== sha) {
        const h0 = await healthAt(trunkTip, 'recheck')
        if (goodHealth(h0)) return true
        if (h0 && h0.status === 'RED') { sha = h0.sha; rep = h0 }
      }
      const id = newFixId()
      const ft = { id, idx: 100000 + nextFix, kind: 'fix', title: `repair ${CFG.integration}`, branch: 'gang/' + id,
        deps: [], mergeAfter: [], locks: [], wave: 0, chain: 0, risk: 'low', roles: FIX_ROLES,
        redSha: sha, failing: (rep.failing || []).join(', ') || 'unknown', healthSummary: rep.summary || 'none', recent: mergesSince(lastGreen) }
      tasks.set(id, ft)
      merged.set(id, deferred())
      status.set(id, 'running')
      fixTasks.push(id)
      log(`${CFG.integration} red at ${short(sha)}: repair attempt ${a} as ${id}`)
      const res = await runTask(ft, false)
      if (res !== 'merged') continue
      const h = await healthAt(trunkTip, `after-${id}`)
      if (goodHealth(h)) { log(`${CFG.integration} green again at ${short(trunkTip)}`); return true }
      if (h && h.status === 'RED') { sha = h.sha; rep = h }
    }
    if (!halted) setStop('TRUNK_RED', `${CFG.integration} stays red after ${CFG.maxRepairAttempts} repair attempts`)
    return false
  })()
  repairing = p
  p.then(() => { if (repairing === p) repairing = null }, () => { if (repairing === p) repairing = null })
  return p
}

let healthPending = null
let healthLoopOn = false
function scheduleHealth(sha) {
  if (!CFG.healthAfterMerge) return
  healthPending = sha
  if (!healthLoopOn) { healthLoopOn = true; track(healthLoop()) }
}
async function healthLoop() {
  try {
    while (healthPending && !halted) {
      const sha = healthPending
      healthPending = null
      const h = await healthAt(sha, 'merge')
      if (h && h.status === 'RED') { trunkRed = true; await repair(h.sha, h) }
    }
  } finally { healthLoopOn = false }
}

function onMerged(t, sha) {
  log(`${t.id} merged as ${short(sha)} (${SCOPE.filter(x => status.get(x.id) === 'merged').length}/${SCOPE.length} in scope)`)
  scheduleHealth(sha)
  wake()
}

// ---- merge: a red trunk holds every merge except the repair task's own ------------------------
// While a repair runs, merges wait for it. With no repair running (it failed, or a stop or halt
// refused it), a red trunk ends the task instead of merging onto red.
const HELD = { held: true }
const held = t => t.kind !== 'fix' && (!!repairing || trunkRed)
async function mergeStep(t, tip) {
  for (let tries = 0; tries < 2;) {
    if (halted) return null
    if (held(t)) {
      if (!repairing) return HELD
      await repairing.catch(() => false)
      continue
    }
    const m = await mergeLock(() => (halted || held(t) ? HELD : call(render('merge', { id: t.id, tip }), A(`${t.id} merge ${nth(t.id, 'merge')}`, phaseOf(t), MERGE_SCHEMA, CFG.roles.merge))))
    if (m === HELD) continue
    if (m) return m
    tries++
  }
  return null
}

// ---- one task, start to merge ------------------------------------------------------------------
async function runTask(t, holdsSlot) {
  let slot = holdsSlot
  const freeSlot = () => { if (slot) { slot = false; implRunning--; wake() } }
  try {
    note(t.id, 'start')
    const im = await implementStep(t, 'new', {})
    freeSlot()
    if (im.stopped && t.kind === 'plan') { setStop('STOPPED', im.stopped); return finish(t, 'not-started', im.stopped) }
    if (im.interrupted || im.blocked || im.stopped) return failed(t, im)
    note(t.id, `implemented ${short(im.tip)} (${im.res.status}, gate ${im.gateOk ? 'pass' : 'FAIL'})`)
    let q = await qualityLoop(t, im.tip, im.gateOk, im.summary, 'full', null)
    if (q.interrupted || q.blocked || q.stopped) return failed(t, q)
    let tip = q.tip
    if (t.mergeAfter.length) {
      const outside = t.mergeAfter.filter(id => status.get(id) !== 'merged' && !inScope(tasks.get(id)))
      if (outside.length) return finish(t, 'parked', `reviewed clean at ${short(tip)}, waits for ${outside.join(', ')} outside this run`)
      note(t.id, 'waiting for merge-after ' + t.mergeAfter.join(', '))
      try { await Promise.all(t.mergeAfter.map(id => merged.get(id).promise)) } catch (e) {
        const hard = t.mergeAfter.filter(id => HARD.has(status.get(id)))
        return hard.length && !halted
          ? finish(t, 'blocked', `merge-after task ${hard.join(', ')} is blocked`)
          : finish(t, 'interrupted', `merge-after task did not merge in this run (${t.mergeAfter.map(id => id + ' ' + (status.get(id) || '?')).join(', ')})`)
      }
    }
    for (let c = 0; ;) {
      const m = await mergeStep(t, tip)
      if (m === HELD) return finish(t, 'interrupted', `${CFG.integration} is red, so merges stop until a repair turns it green`)
      if (!m) return finish(t, 'interrupted', halted ? halted.why : 'merge agent returned no result')
      if (isSha(m.trunk_sha) && m.status !== 'REFUSED') trunkTip = m.trunk_sha
      if (m.status === 'MERGED' || m.status === 'ALREADY') {
        if (!isSha(m.trunk_sha)) return finish(t, 'blocked', `merge returned no trunk SHA: ${JSON.stringify(m)}`)
        // sha is the merged tip, which the ledger keys on, and trunk is the integration SHA after the merge.
        if (!mergeLog.some(x => x.id === t.id)) mergeLog.push({ id: t.id, sha: tip, trunk: m.trunk_sha })
        R(t.id).tip = tip
        R(t.id).mergedAs = m.trunk_sha
        finish(t, 'merged')
        settle(merged.get(t.id), true)
        onMerged(t, m.trunk_sha)
        return 'merged'
      }
      if (m.status === 'CONFLICT') {
        const files = Array.isArray(m.files) ? m.files : []
        if (++c > CFG.maxConflictRounds) return finish(t, 'blocked', `still conflicts with ${CFG.integration} after ${c - 1} merge rounds: ${files.join(', ') || m.reason}`)
        if (!files.length) { note(t.id, `merge lost a race (${m.reason || 'trunk moved'}): merge again`); continue }
        note(t.id, `conflict with ${CFG.integration} ${short(m.trunk_sha)} in ${files.join(', ')}`)
        const sy = await implementStep(t, 'sync', { tip, files: files.join(', '), trunk: m.trunk_sha })
        if (sy.interrupted || sy.blocked || sy.stopped) return failed(t, sy)
        q = await qualityLoop(t, sy.tip, sy.gateOk, sy.summary, 'merge', tip)
        if (q.interrupted || q.blocked || q.stopped) return failed(t, q)
        tip = q.tip
        continue
      }
      return finish(t, 'blocked', `merge ${m.status}: ${m.reason || ''}`)
    }
  } finally { freeSlot() }
}

// ---- main loop -----------------------------------------------------------------------------------
phase('Tasks')
log(`start ${CFG.runTag}: ${DONE.length} merged already, ${SCOPE.filter(t => !status.has(t.id)).length} to run, ${CFG.integration} at ${short(trunkTip)}`)
const inflight = new Map()
while (true) {
  const w = wakeD.promise
  doom()
  if (!halted && !stop) {
    const ready = SCOPE.filter(t => !status.has(t.id) && t.deps.every(d => status.get(d) === 'merged'))
      .sort((a, b) => b.chain - a.chain || a.idx - b.idx)
    for (const t of ready) {
      if (implRunning >= CFG.maxImplementers) break
      status.set(t.id, 'running')
      implRunning++
      const p = runTask(t, true)
        .catch(e => { if (status.get(t.id) === 'running') finish(t, 'blocked', 'engine error: ' + errText(e)) })
        .finally(() => { inflight.delete(t.id); wake() })
      inflight.set(t.id, p)
    }
  }
  if (inflight.size === 0 && bg.size === 0) break
  await w
}
while (bg.size) await Promise.all([...bg])
doom()

for (const t of SCOPE) if (!status.has(t.id)) { status.set(t.id, 'not-started'); reason.set(t.id, 'never became ready in this run') }
const by = {}
for (const [id, st] of status) (by[st] = by[st] || []).push(id)
const blockedIds = SCOPE.filter(t => status.get(t.id) === 'blocked').map(t => t.id)
const [stopKind, stopWhy] = SCOPE.every(t => status.get(t.id) === 'merged') ? ['DONE', 'every task in scope merged']
  : halted ? [halted.kind, halted.why]
  : stop ? [stop.kind, stop.why]
  : blockedIds.length ? ['BLOCKED', `blocked: ${blockedIds.join(', ')}`]
  : ['INTERRUPTED', 'some tasks in scope did not merge in this run']
return {
  runTag: CFG.runTag,
  trunkStart: CFG.trunkStart,
  trunkTip,
  stop: stopKind,
  reason: stopWhy,
  counts: Object.fromEntries(Object.entries(by).map(([k, v]) => [k, v.length])),
  byStatus: by,
  reasons: Object.fromEntries(reason),
  mergeLog,
  fixTasks,
  nextFixNumber: nextFix,
  health: healthLog,
  rulings: Object.fromEntries(Object.entries(rec).filter(([, r]) => r.rulings.length).map(([id, r]) => [id, r.rulings])),
  events: Object.fromEntries(Object.entries(rec).map(([id, r]) => [id, r.events.slice(-10)])),
}
