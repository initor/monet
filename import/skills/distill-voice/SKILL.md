---
name: distill-voice
description: Use when the user hands you a piece of writing and wants its mechanics extracted into a named persona that the voice skill can apply. Writes reference/personas/<name>.md. Never mines the user's own writing unprompted.
---

## Overview

Take a source the user hands over, measure it, extract its moves with verbatim samples, and write
one persona file. `voice` reads what this skill writes. This skill never reads a draft in order to
correct it, and it never goes looking for material on its own.

Two modes. **Single-source** distills one hand-picked document, the original flow. **Corpus** takes
a scope the user names (their transcripts, their GitHub comments in an org, a directory) and runs
the gathering pipeline under `scripts/`, with the agent-content filter between gathering and
distilling. Both modes end in the same Steps 3 to 7. The corpus never chooses itself.

## When to use / not

- Use when the user points at writing they want the mechanics of: a file, a PR, a doc, pasted text.
- Not for correcting prose. That is `voice`.
- Corpus mode still has the user choose the input. The user names the scope, and the
  agent-content filter (`reference/agent-content-filtering.md`) excludes what the author did not
  actually type. Those two gates together are what keep generated prose from being distilled back
  in and relabelled authentic. Never gather a corpus the user did not ask for.

## Invocation

    /monet:distill-voice <source> --as <persona> [--surface <surface>]
    /monet:distill-voice --corpus <scope> --as <persona>

`<source>` is a file path, a PR URL, a PR number, or pasted text.
`<persona>` is a lowercase single word naming the author.
`<surface>` is a row from the surface map in `reference/personas.md`.
`<scope>` names a gatherable corpus: `transcripts`, `github:<login> orgs:<o1,o2>` or
`github:<login> repos:<owner/r1,...>` (optionally `since:YYYY-MM-DD until:YYYY-MM-DD`), or a
directory of files. In corpus mode the surface flag is dropped, because each register found in the
corpus maps to its own surface in Step 2.

`reference/personas.md` and `reference/personas/` are in the `voice` skill's directory
(`~/.claude/skills/voice/reference/`), because `voice` reads them there. Every other `reference/`
and `scripts/` path in this file is relative to this skill's own directory.

## Steps

1. Resolve and fetch the source. For a PR, fetch the body or the changed file, not the diff summary.
   For a corpus, run the Corpus pipeline below instead, then continue from Step 2 with its output.
2. Confirm which surface it evidences. Ask when it is not obvious. One source evidences one surface.
   A corpus register also evidences exactly one surface (transcripts evidence `Chat, instructing an
   agent`, review comments evidence `PR review, someone else's code`, and so on). Confirm the
   mapping with the user before distilling.
3. Run the mechanical pass and compute the metrics table.
4. Extract moves. Each one needs at least one verbatim sample copied exactly.
5. Replace the persona if it already exists, otherwise create it. A re-distill is a fresh pull: it
   supersedes the old sources and samples (see Re-distilling an existing persona).
6. Write that one persona file, then update its roster row in `reference/personas.md`.
7. Report what was measured, which moves were extracted, and which register rows filled. For each
   surface the run evidences, print the `## Surface map` row you suggest. Never write the map.

Four hard rules:

- No move without at least one verbatim sample. A rule with no sample is the agent inventing a voice.
- No register row the source does not evidence. An absent row is correct. A guessed row is not.
- Exactly one persona file written per run. Never touch another persona.
- Replacing an existing persona requires confirmation from the user first.

## Corpus pipeline (corpus mode only)

Gathering and filtering happen before anything is measured or quoted. Ported from
ronaknnathani/relay `build-write-like-me`, which encodes rate-limit-safe fetching and the
agent-content filter.

1. **Gather** into a scratch directory (never into `reference/`):
   - `transcripts`: `python3 scripts/extract_transcripts.py --out <dir>/transcripts/messages.txt`.
     It keeps the user's own typed turns and drops tool results, pasted dumps, and
     agent-orchestration prompts recorded as user turns.
   - `github:<login> ...`: `python3 scripts/fetch_github.py --user <login> --orgs <o1,o2> --out
     <dir>/github/ [--since ... --until ...]`. Confirm `gh auth status` and the active account
     first, then run it in the background, because search is heavily throttled and a large org
     takes minutes. It writes `descriptions.json` and `comments.json`, each record tagged
     `agent_authored`.
   - a directory: read the files as they are.
2. **Filter.** Read `reference/agent-content-filtering.md` and apply it. The scripts tag suspected
   agent content rather than deleting it. Decide the cutoffs, show the user a few borderline cases,
   and never quote a tagged item as a sample.
3. **Measure** with `python3 scripts/analyze_style.py --github-dir <dir>/github --transcripts
   <dir>/transcripts/messages.txt --text <register>=<glob>`. It prints per-register metrics in the
   persona table's terms, plus a short-only baseline line. When the short-only baseline and the
   full register diverge on em-dashes or semicolons, the long items are contaminated. Trust the
   short baseline and tighten the filter.
4. **Read for voice.** Numbers do not capture voice. Read the cleaned corpus (through subagents for
   a large one) and pull per register: candidate moves, each with verbatim samples, and recurring
   phrases with counts. Then continue with Steps 2 to 7 as for any source.
5. **Record the filtering** in the persona's Sources note: how many items gathered, how many
   excluded, by which tell.

## Persona file format

The file lives at `reference/personas/<name>.md`. Open it with exactly these two lines:

    # Persona: <name>

    Written by `distill-voice`. Read by `voice`. Never hand-edited.

Then these sections, in this order, with no others (Signature phrases only when evidenced):

**`## Sources`** is a table with columns `#`, `Source`, `Surface`, `Words`, `Distilled`, one row per
source. Follow it with a short prose note on authorship confidence if anything about the source is
ambiguous or unverified.

**`## Computed metrics`** is a table with columns `Metric`, `Value`, `Notes`, carrying the
measurements from Step 3: em-dashes per 1000 words, semicolons per 1000 words, contraction rate,
median sentence length with p90 and max, sentences over 30 words, bold spans per 1000 words
(excluding table row labels), and lowercase sentence-start rate.

**`## Register rows evidenced`** is a table with columns `Surface`, `Caps`, `Length`, `Stance`,
`Perspective`, `Tells`. `Perspective` records the authorial stance the source evidences on that
surface: personal `I`, collective `we`, direct `you`, or impersonal. Write `unknown` when the
source does not settle it. Never promote one surface's perspective to another. Include only surfaces the source actually evidences. An unevidenced surface is omitted
entirely, not included blank. The surface names come from the surface map in
`reference/personas.md`: `Design or strategy doc`, `PR description`, `paint page copy`,
`Commit message`, `PR review, someone else's code`, `PR reply, own PR`, `Slack`,
`Chat, instructing an agent`, `Peer feedback`.

**`## Moves`** holds one `### <move name>` heading per move, a short prose description of the
mechanic, then one or more `- Sample: "<verbatim text copied exactly from the source>"` lines.

**`## Signature phrases`** is optional and holds recurring phrases the author actually types, one
per line as `- "<verbatim phrase>" (source #<n>, seen <k>x)`. A phrase needs at least two
occurrences in the sources to be listed. One occurrence is a sample inside a move, not a signature.
Omit the section entirely when nothing qualifies.

There is NO `Adopted:` field on a move. That flag was removed by design. Selecting a persona
applies all of its moves unless a move carries `Enabled: no` as an explicit escape hatch.
`Enabled: no` is the only gating field.

**Roster row.** After writing the persona file, update its row in the `## Roster` table in
`reference/personas.md`, or append one row when the persona is new:

    | `<persona>` | `personas/<name>.md` | <source count> | <surfaces evidenced> | <date> |

Do not touch the `## Surface map` table. It stays hand-gated: a person decides when the evidence
is strong enough to change a surface's persona. Instead, Step 7's report prints the suggested row
for each surface the run evidences, in the map's own columns, for the user to apply by hand:

    | <surface> | `<persona>` | <what evidences it, with counts> |

## Step 3 in detail: the mechanical pass

Count, do not estimate. Strip fenced code blocks, tables, and link URLs before measuring prose.

```bash
grep -o $'\xe2\x80\x94' <file> | wc -l          # em-dashes
grep -o ';' <file> | wc -l                      # semicolons
grep -oE "\b\w+'(s|t|re|ll|ve|d|m)\b" <file> | wc -l   # contractions
```

Compute and record: em-dashes per 1000 words, semicolons per 1000 words, contraction rate, median
sentence length with p90 and max, count of sentences over 30 words, bold spans per 1000 words
excluding table row labels, and lowercase sentence-start rate.

## Step 4 in detail: extracting moves

A move is a repeatable mechanic another writer could adopt. It is not a summary of what the piece
says.

Three tests before writing one down:

1. Can you quote it? If not, it is an impression, not a move.
2. Does it repeat, or is it one occurrence? One occurrence is a sample, not a pattern. Say which.
3. Is it a mechanic or a verbal tic? Both belong in the persona. Tics matter more, because they are
   what makes prose recognisable, and they are what a reader notices when borrowed.

## Re-distilling an existing persona

Re-distilling replaces. The fresh pull supersedes the old sources and samples of that persona, so
nothing is merged and nothing needs deduplicating. A merge would count the same messages twice
whenever `--corpus transcripts` is re-run.

Before writing, show the user which old sources, moves, and register rows the new file drops, and
get confirmation. Then write the file from this run's sources alone: the Sources table lists only
them, every metric is computed from them, and every sample is quoted from them. A source the user
wants kept goes into the new pull.

A persona still describes one author. Never fill it from another writer's sources, which produces
prose that sounds like nobody.

## Constraints

- Never write a persona file for an author the user did not name.
- Never fill a register row from another row of the same persona.
- Never edit `SKILL.md` of the `voice` skill. Method and data stay separate.
- Report the sample count per move. A move resting on one sample is weaker than one resting on three,
  and the user should see which is which.

## Common mistakes

- Paraphrasing a sample. The verbatim text is the evidence. A paraphrase is the agent's voice.
- Filling every register row from one source. One doc evidences one surface.
- Carrying an old source, sample, or metric into a re-distill. The fresh pull supersedes them.
- Extracting the argument of the piece rather than its mechanics. What it says is not how it reads.
