# Agent-content filtering

A corpus is only worth distilling when it holds what the author typed. PR bodies, review comments,
and chat turns often hold text that an agent wrote for the author. Distilled back in, that text
gets relabelled as the author's voice. This file says how `distill-voice` keeps it out. The
scripts tag what they can detect, and the agent applies a cutoff that the user confirms.

## What the transcript extractor drops

`extract_transcripts.py` keeps only the turns the user typed. It drops a record that a tool, a
hook, or another agent put in the user's place: sidechain and meta records, tool results,
compact summaries, and messages that another agent or a task sent. It drops text that the CLI
writes into a user turn: command echoes, injected reminders, shell input and output, hook output,
and interrupt markers (a text that starts with a lowercase tag such as `<bash-stdout>`, or with
`[Request interrupted by user`). It also drops pasted dumps: a text over 2000 characters or 40
lines, or one that holds a fenced block over 20 lines. What it writes carries no tags, so read it
for voice as it is.

## Tells

A tell is a mark that generated text carries and the author's own prose rarely does. A tell can
match a human sentence, such as one that says a thing was written by someone, and the rule below
to tag when in doubt accepts that loss. `fetch_github.py` checks each body for the tells below.
When one matches, it sets `agent_authored` to true and lists the tell's id in `agent_tells`. A
tagged item stays in the file, so the count of what was excluded, and why, stays visible.

| Tell | What marks it | Where it shows up |
|---|---|---|
| `attribution` | A line that says the text was generated, created, written, or drafted with or by an AI tool, a `Co-authored-by:` trailer that names one, or the robot emoji that coding agents put in their footers. | PR descriptions, mostly in the footer. |
| `session-link` | A link to an agent chat or session, such as a `claude.ai/code/` or `chatgpt.com/share/` URL. | Descriptions and comments that point at the session that wrote them. |
| `template` | A `Summary` heading and a `Test plan` heading in the same body, the default PR layout of coding agents. | PR descriptions. |

For a directory corpus, apply the same tells by reading the files or with `grep`, and keep a list
of the files they catch.

Em-dashes, semicolons, length, and polish aren't tells. People use all of them. They feed the
cutoff step instead.

## The cutoff step

Tells catch the text that says where it came from. The cutoff catches the rest.

1. Run `analyze_style.py` on the raw gather. It skips tagged items and prints each register above
   a `baseline (short only):` line. Short items are rarely generated, so the baseline is the
   closest measure of the author's own hand.
2. Compare each register with the baseline. When a register's em-dash or semicolon rate is more
   than double the baseline's, and at least 2 per 1000 words, its long items are contaminated.
3. Propose a cutoff for that register. It's a word count above which an item is excluded, a date
   from which the author drafted with an agent, or both. The earliest tagged item's date is a good
   first guess for the date.
4. Show the user 3 to 5 borderline items: untagged items just past the cutoff, and just inside it.
   Quote only enough of each to judge. Ask the user to confirm the cutoff, or to move it.
5. Apply the confirmed cutoff in the scratch copy. Set `agent_authored` to true and add `cutoff` to
   `agent_tells` on each item it catches, then rerun `analyze_style.py`. Transcript messages and
   `--text` files carry no tags, so for those registers delete each caught message from a scratch
   copy of the messages file, or narrow the glob to leave out each caught file. Repeat from step 2
   until each register's rates sit near the baseline.
6. Record the result in the persona's Sources note: items gathered, and items excluded by each
   tell and by the cutoff.

## Rules

- Never quote a tagged item. It isn't a sample, a signature phrase, or a borderline case to show
  the user.
- Never measure a tagged item. `analyze_style.py` skips them, so a count by hand skips them too.
- Only the user removes a tag. The agent doesn't untag an item on its own judgment.
- When in doubt, tag. A small clean corpus beats a large contaminated one.
