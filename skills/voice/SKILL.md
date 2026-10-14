---
name: voice
description: Use when prose will go out under the user's name and must not read as generated (PR descriptions, review replies, commit bodies, design docs, Slack, peer feedback), or when asked whether a draft sounds like the user. Governs how the prose reads, not what goes in it. For a PR body's structure and sections, git-draft-pr owns that. In STE mode, which /monet:voice turns on by default, it applies the STE-lite card.
argument-hint: "[target: session (default) | draft text | path | PR #/URL] [--check] [--no-ste] [--persona <name>] [--list]"
allowed-tools:
  - Read
  - Bash
  - Glob
  - Grep
  - Edit
  - AskUserQuestion
---

<!-- This file contains no em-dashes and no semicolons, on purpose. A skill that bans
     them while using them teaches the agent that the rule is decorative. -->

## Overview

Prose going out under the user's name should read as though they typed it.

This file holds the **method** and the **invariants**, which are stable. The **flavor** that is
applied lives in persona files: the user's own in their data dir, derived from real corpora and
rewritten on re-distill, or the bundled synthetic sample. Keeping the two apart is the whole
design. The method never changes when a persona is re-derived, and the persona is never invented
to patch a hole in the method.

The split also draws the line this skill exists to hold:

- **Invariants** are anti-slop. They are what makes any text read as generated, whoever wrote it.
  They apply with no sample behind them.
- **Flavor** is personal. It applies only when a persona file says so, with a sample beside it.

Promoting a flavor rule into an invariant is how the agent quietly starts writing in someone else's
voice and calling it the user's.

## How other skills load this one

Another skill that writes prose loads this one with the Skill tool as `monet:voice`. It never
reads this file by path. Such a caller passes `--no-ste` in the Skill args to stay on plain voice,
unless it asks for STE-lite. Every caller in this plugin works this way: `git-draft-commit`,
`git-draft-pr`, `pr`, `paint`, `ship`, and `visual-design`. Holding a rule in context is not the
same as applying it, so a producer loads this skill rather than relying on the agent to remember
it.

## When to use / not

- Use for: PR descriptions and review replies, commit bodies, design and strategy docs, Slack,
  peer feedback, agent instructions, any first-person text going out under the user's name.
- Use for: a check of whether a draft that already exists sounds like the user.
- Use for: STE-lite (Step 2d). `/monet:voice` applies the card in
  `${CLAUDE_SKILL_DIR}/reference/ste-lite.md` by default, and `--no-ste` turns it off. Answers to
  the user get the same card through the `ste-lite` output style, not through this skill.
- Not for: deciding **what** a PR body contains. `git-draft-pr` owns the template, the section
  caps, and the diagram rule. This skill only governs how the words read inside them.
- Not for: **deriving** the voice. That is `distill-voice`, which writes persona files to the
  user's data dir from real corpora. This skill only reads them.

## Arguments

Target: $ARGUMENTS

- Empty: the prose most recently produced in this session. This is the common case, because the
  usual trigger is a fresh draft that needs fixing before it goes out.
- Quoted or pasted text: the text itself is the draft.
- A file or directory path: read it and treat its prose as the draft. For a code file, that means
  the comments and doc strings, not the code.
- A PR number or URL: the PR body, plus any review comments by the user on it.
- `--check` anywhere: check mode only. Report violations and change nothing.
- `--list` anywhere: list mode, below. Ignore every other argument, including a target.
- `--no-ste` anywhere: plain voice. Skip Step 2d, and check with the Step 3 greps instead of the script.
- `--ste` anywhere: no change, because STE-lite is already the default.
- `--persona <name>`: use that persona instead of the surface map's result (Preconditions).

Name the surface when the target does not make it obvious: `PR description`, `PR reply, own PR`,
`Commit message`, `Design or strategy doc`, `Slack`, `Peer feedback`.

With `--list`, do only this and stop. Read the surface map as Preconditions step 3 does, data dir
first, and print both its tables: the roster and the surface map. Then, for each persona in the
roster, read `personas/<name>.md` in the data dir, or `${CLAUDE_SKILL_DIR}/reference/personas/<name>.md`
when the data dir has none, and add its move count and total sample count. Name a roster entry as
missing only when neither file exists. Do not resolve a surface, and do not touch a draft.

Editing a file in place needs the user's explicit confirmation first. Show the before and after,
then ask. Check mode never edits anything.

## Preconditions

Resolve a persona before applying any flavor. Resolution is by surface, unless a persona is named
explicitly. Each lookup tries the user's data dir first, then the bundled copy in this skill. Read
these files explicitly with the client's file-reading tool.

1. Determine the surface using Step 1.
2. If a persona is named explicitly (for example, via `--persona <name>`), skip the map. Go to
   step 4.
3. Otherwise, read the surface map in `${user_config.data_dir}/personas/personas.md`. If the data dir above is empty or starts with `${user_config`, it isn't configured, so use `~/.config/monet`.
   When that file does not exist, read the bundled `${CLAUDE_SKILL_DIR}/reference/personas.md`,
   which maps no surface. Look the surface up in the map.
4. If a persona is named or found, read `${user_config.data_dir}/personas/<name>.md`. If the data dir above is empty or starts with `${user_config`, it isn't configured, so use `~/.config/monet`.
   When that file does not exist, read `${CLAUDE_SKILL_DIR}/reference/personas/<name>.md`. So
   with no `sample.md` in the data dir, `--persona sample` applies the bundled synthetic example.
   Apply Step 2b from the persona.
5. If the map gives none, apply the Step 2a invariants only and say so once per session, naming
   the surface:

> No persona is mapped for this surface (commit message), so this pass applies invariants only.

6. If the named persona's file is missing in both places, stop and say so. Never guess.

A surface with no persona is not a gap to fill from intuition. It means no source evidences that
surface yet. Filling it anyway is the agent inventing a voice, which is the failure this skill
exists to prevent.

## Step 1: Pick the register before writing a word

The same voice reads very differently by surface. Choose the surface first, then write. Getting this
wrong is more damaging than any individual word choice, because a Slack-terse design doc and a
blog-polished review comment are both immediately wrong.

Read the register row for this surface from the resolved persona (per Preconditions) and use it.

If the resolved persona has no row for this surface, fall back to the one split that is safe to
assert without a sample, because it is a property of the medium rather than of the person:

> Chat and code review are fast and lowercase. Anything written down for others is fully formed prose.

Do not fill in the missing rows from intuition. A missing row means "unknown", not "use your default".

## Step 2a: Invariants (always, no sample required)

These are anti-slop, not personality. Apply every one, on every surface.

- **No em-dashes.** Not one, on any surface. This is the single most reliable tell. Use a comma, use
  parentheses, or split the sentence. An em-dash in the output means the voice slipped.
- **No semicolons.** Split the sentence instead.
- **Contractions, always.** it's, don't, we're, can't, won't, isn't, shouldn't.
- **One idea per sentence.** When a thought runs long, split it. Do not stack clauses.
- **Plain words.** Cut "leverage" (as a verb), "seamless", "robust", "delve", "unlock", "empower",
  "in today's fast-paced world", "it's important to note that", "worth stating plainly".
- **Ground every claim.** A name, a number, an example, a file reference. An abstract assertion with
  nothing concrete under it is filler even when it is true.
- **Point first.** The first sentence says the thing. No throat-clearing, no "in this section we
  will explore", no restating the question before answering it.
- **Capitalization follows the surface.** Lowercase sentence starts are normal in chat and review
  comments. Written-down prose gets proper capitalization.
- **Preserve the stated intent.** Tighten the language, do not move the problem. If the framing was
  "ownership is split across two systems", do not rewrite it as "distributed systems are hard".

## Step 2b: Flavor (only from the resolved persona, never invented)

Apply the resolved persona's register row and its moves. If the register row carries a
`Perspective` value other than `unknown`, write from that stance (personal I, collective we,
direct you, or impersonal) on that surface only. If the persona has a `## Signature phrases`
section, those phrases are available where they fit naturally, at most one or two per document,
never forced in. An older persona file without the column or the section simply has neither,
which means unknown, not a gap to fill. Nothing else.

Three rules govern this half:

1. **Every rule cites a sample.** A move with no verbatim sample beside it does not get applied.
   Report it as unsupported instead.
2. **Never blend two personas in one document.** Prose mixing two writers sounds like neither.
3. **Never apply a register row the persona does not evidence.** An absent row means unknown. Fall
   back to the Step 2a invariants for that surface.

A move marked `Enabled: no` is skipped. Everything else in the persona applies.

## Step 2c: When editing rather than drafting

Editing means cut repetition, improve flow, reduce density, and remove the tells. It does not mean
rewrite.

- Do not add words. If a line is too dense, split it or cut it, do not expand it.
- Preserve the underlying claim, the order of ideas, and the technical model.
- Make it sound more like the person, not more like an editor.
- If a correction is given on one sentence, extract the rule behind it and apply that rule across
  the whole draft. Patching only the flagged sentence means the same correction comes back.

## Step 2d: STE-lite (STE mode)

STE-lite is a register, not a persona. It is 8 rules adapted from ASD-STE100, the controlled
English of aircraft maintenance manuals, and they live in `${CLAUDE_SKILL_DIR}/reference/ste-lite.md`.
STE mode is the default when the user runs `/monet:voice`, on every surface, and `--no-ste` turns
it off. A skill that loads this one, such as `git-draft-pr`, `pr`, or `paint`, passes `--no-ste`
to stay on plain voice unless it asks for STE mode.

The same file is the `ste-lite` output style, through the relative symlink
`output-styles/ste-lite.md` at the plugin root. Edit `reference/ste-lite.md` only, and both
readers change.

1. Resolve the persona as usual. A runbook, a procedure, or test steps has no surface row, so it
   resolves to no persona unless `--persona` names one. The notice then says that the pass
   applies the invariants and the STE-lite card.
2. Read the card. Apply its numbered rules, its rule to keep every fact and caveat,
   and the paragraph that starts "Code, commands". Skip the frontmatter, the line about drafts
   under the user's name, and the copyright line. They only frame the output style.
3. The card wins every conflict. It replaces the Step 2a contractions rule, so contractions are 0
   on every surface. It also beats conflicting persona moves, even ones with a sample.
4. The card also beats Step 2c. Turning a procedure into numbered steps, putting the condition
   first, adding `WARNING:` lines, and rewriting a passive sentence in the active voice are allowed
   edits. Keep every technical fact intact. Do not split or edit a code block to get one action per
   step: a step may hold one code block with several commands.
5. Persona moves that do not conflict still apply, such as one bolded rule that closes a summary.

## Step 3: Check before it goes out

Runs on its own too, when the user asks whether a draft sounds like them.

**Count the mechanical ones, do not eyeball them.** For a draft on disk:

```bash
grep -o $'\xe2\x80\x94' <file> | wc -l          # em-dashes: must be 0 on every surface
grep -o ';' <file> | wc -l          # semicolons: must be 0 on every surface
grep -coE "\b\w+'(s|t|re|ll|ve|d|m)\b" <file>   # contractions: expect many in plain voice
```

In STE mode, run the checker instead of the three greps. Give it absolute paths, or pipe a draft in
context to it with `-` as the path:

```bash
python3 ${CLAUDE_SKILL_DIR}/scripts/ste_check.py <absolute path>
```

It prints each violation with its line number. It reads the file as GitHub renders it: it checks
every line except code, frontmatter, comments, and link definitions, and it reads a blockquote as
prose. In frontmatter, comments, and one-line link definitions, it counts only em-dashes. In a line,
it skips code spans, double-quoted text, link targets, and URLs. Every sentence inside a numbered
step gets the 20-word cap, in its sub-bullets, quotes, and tables too. A sentence that starts with a
lowercase word joins the sentence before it, so start a sentence with a capital, or put a name such
as `kubectl` in a code span. A fence, a comment, or an HTML block that never closes is a violation,
because the checker cannot read past it. It exits 0 when clean, 1 on a violation, and 2 when it
cannot read a file or gets none. Report its counts as mechanical facts. In STE mode, contractions are
0 on every surface, so the contraction notes below do not apply.

Card rules 1, 4, 5, and 6 stay judgment calls. So do the parts of the other rules that no script
counts, such as one topic per paragraph and keeping the small words.

Em-dash and semicolon counts are pass or fail. In plain voice, each uncontracted "it is" or "do not"
is a missing contraction.

For a draft in context, count them deliberately rather than scanning for a general impression.

Checklist:

1. Zero em-dashes? (absolute, every surface)
2. Zero semicolons, and contractions throughout? (STE mode: zero contractions.)
3. First sentence makes the point?
4. Every claim grounded in a name, number, or example?
5. Any "leverage / seamless / robust / delve / it's important to note" to cut?
6. Sentences short, one idea each?
7. Right register for the surface, per Step 1?
8. Intent preserved, or did the edit move the problem?
9. Does each flavor rule applied have a sample behind it in the resolved persona's file?
10. Does it read like a person typed it, rather than a model?

**Reporting violations:** quote the offending line, name the rule, give the replacement. Nothing
else. No preamble, no severity essay, no praise for the parts that passed. Separate the two halves,
because they carry different confidence:

- **Mechanical** (counted): em-dashes, banned words, and the raw semicolon and contraction counts.
  State these flatly as facts.
- **Judgment** (read): register fit, grounding, point-first, intent drift. Mark these as calls, and
  say when it is close.

## Constraints

- Never claim a rule belongs to a persona without a sample behind it in that persona's file.
- Never blend two personas in one document.
- Keep output bounded and skimmable. A check report is a list of violations, not an essay about
  writing.
- This skill does not change what a document says. If the prose is wrong on the facts, say so and
  stop, do not smooth it.

## Common mistakes

These are the tells, in rough order of how reliably they give the game away.

- **Em-dashes.** The agent's default has many. This one is absolute.
- **The exhaustive auto-reply.** "Good catch, fixed in `a1b2c3d`. The function now does X, mirroring
  the pattern in Y, and I added a test covering Z." That cadence of acknowledge plus SHA plus
  multi-sentence mechanism is a generated reply. A reviewer may not flag it, which is worse than
  being flagged. A reply is "done" or "updated".
- **Padding a template.** Filling each section of a PR or doc template with three polished
  paragraphs. The padding is the tell, not the headers. Fill each section concisely.
- **Restating the diff in prose.** Listing every file touched, or explaining how the new code
  mirrors an existing pattern. That is the "how", and the diff already shows it. Write the "why".
- **Cute generated phrasing.** "got the memo", "that's the whole trick", "nothing here is exotic".
  This reads as a model reaching for voice.
- **Editorial preambles aimed at the writer instead of the reader.** Do not write "be precise about
  what this does and does not do". Write the precise thing.
- **Long clause-stacked sentences.** Break them up.
- **Filling a surface with no persona from intuition.** Unknown means unknown. See Preconditions.
- **Promoting a move from a persona into an invariant.** It is flavor and it stays in the persona's
  file.
