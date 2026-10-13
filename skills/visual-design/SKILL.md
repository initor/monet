---
name: visual-design
description: Use when designing something visual that the owner picks by eye, such as a logo or mark, a figure, the look of a page, or an HTML overview like a system architecture page, worked out over rounds of options. For a one-pass page from session findings, use paint. For one Hairline figure, use hairline-create. For an interactive system design whiteboard, use sd-playground.
argument-hint: "[what to design, for whom] [work folder to resume]"
allowed-tools:
  - Read
  - Write
  - Edit
  - Bash
  - Glob
  - Grep
  - Agent
  - Skill
  - AskUserQuestion
---

# Visual design

Design one visual thing with its owner, in rounds. Each round puts options on one page, by ID, each shown where it will live. The owner picks by eye. The pick is refined, checked in a real browser, and shipped on the owner's go.

**The owner's eye decides. Research, measurements, and panels inform the pick. They never make it.** In the logo work behind this skill, the first panel ranked the owner's pick last of eleven. After two refinement rounds, every lens ranked the refined marks above the original.

Paths: CLAUDE_SKILL_DIR, this skill's dir, is `${CLAUDE_SKILL_DIR}`, and CLAUDE_PLUGIN_ROOT, the plugin root, is `${CLAUDE_PLUGIN_ROOT}`, so a reference file below that writes either name as a `${...}` token means that path.

## Arguments

Arguments: $ARGUMENTS

Parse them for what to design, and for a work folder to resume:

- What to design and for whom, such as a logo for a team, a figure for a page, or an overview page for a system. If they are empty, ask the owner.
- A work folder to resume. When the arguments name one, resume that design instead of starting a new one.

## When to use

- A logo, mark, emoji, or wordmark. A figure or an animation. The look of a page. An HTML overview, such as a system's architecture, a roadmap, or an intro page, that the owner will judge by eye.
- Not for a one-pass page from session findings: use `paint`. One Hairline figure: `hairline-create`. A system design whiteboard: `sd-playground`.

## The work folder

All work lives in one folder: the one the owner names, else `.planning/tmp/visual-design/<topic>/` in the current repo once `git check-ignore` confirms it is ignored. If it is not ignored, ask the owner for a folder. Nothing in it is published.

| File | Holds |
|---|---|
| `goal.md` | The ask, in the owner's own words, written before anything is designed |
| `brief.md` | What the research found, each fact with its source. The real contexts the artifact must work in. Principles, what to stay clear of, open questions, and a **Decisions** table: date, decision, the owner's words |
| `research/` | Numbered notes, each with its sources and the provenance of any asset it brings in |
| `iterate.html` | Every round's options, by ID, built from a data file (`${CLAUDE_SKILL_DIR}/reference/options-page.md`) |
| `tools/` | The harness that renders and measures an option in its real contexts, then the generator of the picked direction |
| `history/` | Every build of the final artifact |
| `learnings.md` | What each round taught, updated after every round and again after shipping |

A folder that already holds `goal.md` is a design in progress. Read the Decisions in `brief.md` and the top of `iterate.html`, then resume where they point.

## Steps

1. **Write the ask down.** Copy the owner's request into `goal.md`. In `brief.md`, name the real contexts: where it is seen, at what sizes, on which grounds and themes. List every open choice, such as format, scope, length, and style. Each open choice becomes an option in round 1 or a question in the brief.
2. **Research, then build the harness.** Give every fact a source, and flag any myth you find. Build `tools/` so that an option is rendered and measured in each real context before anyone judges it (`${CLAUDE_SKILL_DIR}/reference/verify-and-ship.md`).
3. **Run a round** (`${CLAUDE_SKILL_DIR}/reference/rounds.md`). Three or more designers work independently, each from a different lens, and a skeptic tries to refute each concept. Before starting, tell the owner how long the round takes and offer a lighter one (`${CLAUDE_SKILL_DIR}/reference/rounds.md`, "Time").
4. **Put the round on the page** (`${CLAUDE_SKILL_DIR}/reference/options-page.md`). Each option gets an ID and a card that shows it in every real context, with its idea, its values and their reasons, and the harness's flags.
5. **Stop for the pick.** Open the page for the owner (`open <path>`), or give its path, and ask one question: "Round N is on `iterate.html`: <IDs>. Which one, or which parts of which?" Record the answer in Decisions, in the owner's words.
6. **Refine** (`${CLAUDE_SKILL_DIR}/reference/rounds.md`, "Converge"). Rebuild the pick as parameters, sweep one knob at a time, and give every number a reason. Each refinement is a new ID on the same page. Repeat steps 4 to 6 until the owner calls it final.
7. **Build, verify, ship** (`${CLAUDE_SKILL_DIR}/reference/verify-and-ship.md`). Build the final from its generator, keep every build in `history/`, and check it in a real browser. Write the copy through `monet:voice`: load it with the Skill tool as `monet:voice` before writing. Ship only on the owner's go, through `/monet:ship`. Then add what the work taught to `learnings.md`. Across sessions, `monet:handoff` carries the state.

## Common mistakes

| Mistake | Instead |
|---|---|
| Building one finished design, then asking "is this right?" | Round 1 is options on `iterate.html`, by ID |
| Settling format, scope, or style on the owner's behalf | Make each one an option, or a question in the brief |
| Ranking options, or marking one "recommended" | Show facts: values with their reasons, and harness flags |
| Prescriptive prompts to the designers | One brief for all, a different lens for each |
| Calling motion or hover checked from headless screenshots | Check in a real browser, or say it was not seen |
| Starting a long round without saying how long | State the time and offer a lighter round |
| Publishing, uploading, or posting a draft | Ship only on the owner's go |

## Constraints

- Don't claim results without evidence.
- Keep output bounded and skimmable: the page carries the options, and chat carries the one question.
- Never publish, upload, or post anything without the owner's go. Name the license of every asset brought in.

## Report

Report the work folder, the round and its option IDs, and what waits on the owner.
