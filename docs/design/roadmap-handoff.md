# monet: roadmap, handoff, resume

Design spec for three new monet skills, written 2026-08-04. Paths are repo paths in the plugin layout.

> **Superseded in part, 2026-08-05.** The `roadmap` section still describes what shipped. The
> `handoff` and `resume` sections do **not**, because one `handoff` skill replaced that pair.
>
> The plan had two steps: a durable, append-only `SESSION-HANDOFF.md`, then a separate prompt skill
> gated on that file. That ceremony did not survive contact with the actual need, which is one
> command at 80% context that prints a short paste block. Read `skills/handoff/SKILL.md` for current
> behavior. The sections below stay as a record of what was tried.

## Motivation

Two prompts recur often enough to be worth encoding:

- "Refresh the roadmap page for this work stream." The artifact is the canonical at-a-glance
  execution view: a work stream's `.planning/temp/<project>/<stream>/roadmap.html`, for two streams
  at the time. People maintain it by hand today, and that maintenance fails in one specific way.
  SVG geometry is hand-placed, so a longer lane silently clips content.
- "Write down where we are, and draft me a prompt I can paste into a fresh session." This was done
  by hand nine times over five months, across six filenames in four directories, with no template.

## Decisions

| Decision | Choice |
|---|---|
| Split | Three skills: `roadmap`, `handoff`, `resume` |
| Naming rule | The command is the plain noun of the artifact it emits |
| `roadmap` architecture | Generator, not patcher |
| `roadmap` state source | The work stream's existing plan markdown, plus live `gh` verification |
| `handoff` trigger | Human-invoked, with no auto-trigger hook |
| Artifact paths | Fixed names in the stream's own directory, for new streams only |
| Distribution | Claude Code only |

## Prior art and what is actually missing

The `paint` skill at `--flavor technical` produced the existing roadmap pages. It supplies the
chassis: the design-token block, the theme script, the `paint-theme` localStorage key, and the
hero, section, and card typography. Lines 1-95 of the two roadmap pages are byte-identical apart from
`<title>` and one added `.bx.gate` rule. That fixes precisely where the reusable skeleton ends.

The `paint` skill does **not** supply the roadmap grammar. That grammar is the master trunk, the
status lanes, the `.bx` box-state vocabulary, the `.ok`/`.live` status dots, and the viewBox
geometry. A grep of `skills/paint/reference/base.md` and `skills/paint/reference/flavors/technical.md`
for that vocabulary returns nothing. It exists only inside two output artifacts and one memory file.
That gap is what `/monet:roadmap` fills.

The `paint` skill also conflicts on lifecycle. Its Step 5 names output
`{topic-slug}-{YYYY-MM-DD}.html` and appends `-2`/`-3` on collision. The roadmap pages are
deliberately undated living documents, overwritten in place. `/monet:roadmap` must override that
step explicitly.

## Shared mechanics

Each skill is a pair, following the established house pattern:

- `skills/<name>/SKILL.md` holds 100% of the logic. Frontmatter is exactly two keys, `name` and
  `description`, with the description starting `Use when `.
- `commands/monet/<name>.md` is a wrapper of roughly 35 lines. Frontmatter is exactly four keys:
  `name: monet:<name>`, `description`, `argument-hint`, and `allowed-tools` as a YAML block
  sequence. The body uses four pseudo-XML tags in this order: `<objective>`, `<execution_context>`,
  `<context>`, and `<process>`. `<execution_context>` holds one `@`-prefixed absolute path to the
  SKILL.md, and `<context>` documents `$ARGUMENTS`.
- The wrapper's step 2 is verbatim house boilerplate: read the SKILL.md and "execute it exactly as
  written -- it is the single source of truth; don't restate or modify its steps here."

In the plugin, each wrapper folds into its skill's frontmatter and body (decision 2 in
`specs/design.md`). So no `commands/` dir ships.

Registration in `agents/monet.md` happens twice per skill. Each skill gets a row in the
Quick-reference routing table, and a numbered dispatch step before the current `13) Safety defaults`.
Three new skills renumber `Safety defaults` from 13 to 16, and the closing step from 14 to 17.

Budget each SKILL.md at 70-100 lines. Overflow goes into a `reference/` subdirectory that a step
reads lazily, the way paint reads `reference/flavors/<flavor>.md`.

**Trigger-phrase hazard.** The monet agent routes "visualize this" and "make this visual" to
`paint`. All three new descriptions must use narrow literal triggers ("roadmap", "refresh the
roadmap", "where are we", "hand off", "resume prompt"). Otherwise they capture paint's traffic.

## Skill 1: `/monet:roadmap`

### Purpose

Keep one living HTML page per work stream current, generated from data rather than hand-placed.

### Inputs and paths

`<stream-dir>` means **the directory that holds, or will hold, `roadmap.html`**. It is the first
argument, and it defaults to the current working directory.

The plan file comes in explicitly as `--plan <path>`. The skill does **not** discover it by
convention, because the two existing streams place it differently:

| Stream | `<stream-dir>` | Plan file |
|---|---|---|
| alpha | `.planning/temp/<project>/alpha/` | `.planning/temp/<project>/alpha-plan.md`, one level up |
| beta | `.planning/temp/<project>/beta/` | inside a `plan/` subdirectory |

A guess across those two shapes would fail silently on one of them. Require the path.

### Data model

One entity, `Item`:

| Field | Meaning |
|---|---|
| `id` | Stable identifier, never reused or renumbered (`alpha-nn`, `beta-nn`) |
| `label` | Short text for the box |
| `lane` | Which status lane the item sits in |
| `state` | Drives the `.bx` class, muted text, and dot |
| `caption` | Free-form string. Deliberately polymorphic: `#123 merged`, `blocked on alpha-12`, `5 items`, `review P1 #7` |
| `groupedIds` | Optional. Collapses N items into one box. The alpha page already renders 19 items as 14 boxes |

Lane membership plus prose in the caption express relationships. They are never drawn edges between
items. Keep it that way. Edge routing is not worth the complexity at this scale.

### Status vocabulary

A declared enum, not four hardcoded classes:

```
{ className, fill, stroke, strokeWidth, dash?, mutedText: bool, dot?: 'ok' | 'live' | none }
```

The current pages ship `done`, `now`, `later`, and (beta only) `gate`. Today `blocked` and
`deferred` both collapse onto `.later`, and only prose tells them apart. An enum lets a stream add a
state without forking the skill.

Dot semantics are fixed: `.ok` green means merged, `.live` blue means open.

### Geometry

**There is no family-wide geometry formula to recover.** The alpha page uses a uniform 104px lane
pitch (lane rects at y = 70, 174, 278, 382, 486). The beta page does not. Its lanes sit at 70, 230,
350, 464, 566, deltas of 160/120/114/102, hand-tuned per lane. So the generator *defines* a layout
rather than reproducing one, and a regenerated beta page looks different. This is accepted.

Defined layout, uniform pitch:

```
pitch          = 104
first_rect_y   = 70
lane_y(i)      = 92 + 104*i          # text baseline reference
rect y(i)      = lane_y(i) - 22      # verified: i=0 -> 70, i=1 -> 174, i=2 -> 278
rect height    = 44
rect rx        = 8
first box x    = 110
text pad       = 12
id text y      = lane_y(i) - 2
sub text y     = lane_y(i) + 13
pr text y      = lane_y(i) + 36
dot cx         = rect_right - 12
dot cy         = lane_y(i) - 10
bottom pad     = 60                  # measured from the LAST RECT's bottom edge, not from the
                                     # pr caption, which sits inside the pad
viewBox        = "0 0 900 H"  where H = 104 * lanes + 70
```

Worked example, 5 lanes: the last rect bottom is `70 + 104*4 + 44 = 530`, so H = 590. That matches
the alpha page exactly. The `pr` caption for that lane has a baseline at 544. That is 14px into the
60px pad, and 46px clear of the bottom edge. An implementer must not re-measure the pad from the
caption, or every page gains 46px of dead space.

The `viewBox` height must be **computed**, not authored. A check against the alpha page confirms it:
5 lanes yields 590, which matches. This fixes the silent-clipping trap that a memory note recorded:
"Extending a lane means bumping the SVG viewBox height; check max y against it after editing or
content clips silently".

Box width must also be computed, from label length against the 10px/13px SVG text advance, with a
minimum floor. Current widths are hand-fitted in the 140-230px range. The text is unwrapped, with no
`textLength` and no `foreignObject`. So the first reuse with a longer label overflows silently.

### Freshness contract

A state change updates three things **together**, as the same memory note says:

- the visible PR label text
- the status dot class (`.ok` merged, `.live` open)
- the verified-on date

The skill verifies **non-terminal PRs only**. It skips anything already rendered as merged, so a
refresh stays cheap:

```
gh pr view <n> --repo <owner/repo> --json number,state,mergedAt
```

Exactly one freshness element with one class carries the stamp. Today the same concept takes four
forms across the family:

- bold in the hero subtitle, *and* again in a footnote
- an `.eyebrow` date plus a `.foot` sentence
- an `.eyebrow` only
- a `.hero-stamp` line

Standardize on one.

### Accessibility

The skill generates the `aria-label` narration from the same items array that produces the SVG. The
current one is 619 hand-written characters that restate every box. It drifts the first time an item
changes lane.

### Template handling

Lines 1-95 ship as a literal template file, with `<title>` and any extra `.bx.<state>` rules
substituted. Two cleanups apply while templating:

- **Delete the first `:root` and `[data-theme="dark"]` blocks** (lines 9-27). They are verified
  inert: the second `:root` at line 51 redefines all 24 tokens that the first block defines. Two
  competing `:root` blocks are the most likely foot-gun for anyone who edits the output by hand.
- Drop the vestigial `data-anim="off"` attribute and the dead `.section>h2` rule, which have no
  backing code.

Do not re-derive the preamble from the paint skill on each run. The existing files encode about a
dozen deliberate deltas from the flavor defaults, such as `--max-width:960px`,
`--section-pad-y:40px`, `--card-pad:16px`, and dropped tokens. A fresh derivation would lose them.

### Output

The output is `<stream-dir>/roadmap.html`, overwritten in place and undated. This explicitly
overrides paint's Step 5 filename convention.

### Non-goals

No drawn dependency edges. No patcher mode. No publishing to claude.ai. These pages live under a
gitignored `.planning/` and are local-only by design.

## Skill 2: `/monet:handoff`

### Purpose

Write the durable record of a work stream beside the work, so a fresh session can pick it up.

### Output

`<stream-dir>/SESSION-HANDOFF.md`

### Structure

**Mandatory.** Refuse to finish without these.

- Current state, in a form that cannot be misread as still-to-do.
- Next action.
- Ground-truth identifiers at commit precision: branch name, head SHA, PR number with its live
  review and merge state, and ticket key. Also record **whether the work is pushed**. An unpushed
  local commit is invisible to every tool the next session has. The bar is a line such as
  `COMMITTED LOCALLY BUT NOT PUSHED. Branch <branch>, single commit <sha>, cut from fresh main, tree clean.`

**Actively prompted.** This band separates a good handoff from a thin one. Four to five of the
seven existing handoffs carry it.

- Decisions already made.
- Conventions and rules to obey.
- Blocked on a human, and on whom.
- Environment hazards.
- **Corrections**, as a first-class section rather than an afterthought. Five of seven handoffs
  carry at least one. They are the most expensive content to lose, because a lost correction
  reproduces the error behind it. Prompt for two things: what was gotten wrong and what the
  corrected version is, *and* which written artifact still contains the wrong version.

**Optional tail.** Verification evidence, dated history, measurement recipes.

### Append-only discipline

A new `## State as of YYYY-MM-DD` section goes at the top. The previous one becomes
`## Historical state as of YYYY-MM-DD`. Never rewrite in place. Both long-lived streams reached this
rule on their own. It preserved the entry that item `beta-06a` was abandoned, and the record that a
premise had been wrong.

### Boundary with MEMORY.md

Memory holds rules and facts that outlive the work stream and stay true whichever PR is open. The
handoff holds everything that is only true this week. The observed failure mode is a memory entry
that swallowed live PR state and rotted. Therefore: **live state goes only to the handoff**, never
to memory.

### Durability note

On its first write in a repo, the skill states once that a global git excludes file ignores
`.planning/`. That file is the one that `core.excludesFile` names, so the handoff never reaches a
remote.

## Skill 3: `/monet:resume`

### Purpose

Emit a paste-ready prompt that gets a fresh session productive immediately.

### Output

The prompt prints to the terminal. The skill writes no file by default.

### Shape

The prompt has 150-400 words, with five moves:

1. Name **one** file as the source of truth, and say to read it before acting.
2. State what is DONE, in a form that cannot be misread as still-to-do.
3. State what is BLOCKED, and on whom.
4. State the working agreement, especially any "do not act without my go-ahead" gate.
5. Give the exact first command to run.

Moves 1-4 come from the resume prompts that already worked. Move 5 comes from an existing re-entry
procedure.

### Non-goals

No SessionStart auto-injection. The mechanism exists and works: a session-state hook already emits
`hookSpecificOutput.additionalContext`, behind a setting that is off. The design explicitly did not
choose auto-injection.

## Threshold arithmetic, for reference

A custom statusline script can write a context bridge file, such as
`/tmp/claude-ctx-<session-id>.json`. Any skill can read it (verified live, zero seconds stale, from
inside a subagent). It exposes both `remaining_percentage` (raw) and `used_pct`, normalized against
a 16.5% overhead floor:

```
usable_rem = (CTX_REM - 16.5) * 100 / 83.5
norm_used  = 100 - usable_rem
```

So `used_pct` 80 corresponds to roughly 33% raw remaining. That is about two points from the 35%
warning of a context-monitor hook. Reading the number is possible. **Self-triggering on it is not**,
because nothing in the skill system invokes itself, and only hooks initiate. This record keeps the
arithmetic from being re-derived if an auto-trigger comes back later.

The bridge file only exists while an interactive statusline renders. A headless run (`claude -p`)
or a cloud routine never writes it. Any future consumer must tolerate its absence. It must also
treat data older than 60 seconds as stale, as the context-monitor hook does.

## Out of scope

- Auto-trigger hooks of any kind.
- SessionStart auto-injection of the resume prompt.
- Migrating the nine existing handoff artifacts onto the new path convention.
- A patcher mode for `/monet:roadmap`.
- Retiring older pause-work and resume-work commands that duplicate this pair. That is a separate
  cleanup, not part of this work.

## Files

All paths are repo paths. The plan also had one command wrapper per skill under
`commands/monet/`. The plugin folds each wrapper into its skill, so those files do not ship.

**Create**

```
skills/roadmap/SKILL.md
skills/roadmap/reference/template.html      # lines 1-95 preamble, cleaned
skills/roadmap/reference/geometry.md        # layout constants and formulas
skills/handoff/SKILL.md
skills/handoff/reference/sections.md        # mandatory / prompted / optional bands (not shipped)
skills/resume/SKILL.md                      # folded into handoff (not shipped)
```

**Modify**

```
agents/monet.md                             # 3 routing rows + 3 dispatch steps, renumber 13->16, 14->17
```

## Risks

- **The plan-markdown dependency.** Requiring `--plan` resolves the plan *location*, but not the
  plan *format*. `alpha-plan.md` carries a `| ID | Item | Status | Ticket | Origin |` table, and the
  beta plan uses other columns. The skill needs a declared minimum table shape: an ID column and a
  status column, both named. It should fail loudly when a plan does not match, and not render a
  half-empty page. If parsing proves brittle across streams, the fallback is an explicit items block
  passed as an argument.
- **One-time visual break.** A regenerated page has a different layout. This is most visible for
  beta, whose irregular lane pitch becomes uniform.
- **Handoff cost at low context.** A user runs `/monet:handoff` precisely when context is scarce,
  and the two exemplar handoffs are 26KB and 36KB. The skill should gather ground-truth identifiers
  by shelling out (`git`, `gh`), not by re-reading files already dropped from context.
