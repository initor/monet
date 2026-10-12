---
name: monet:voice
description: Write or edit prose so it goes out sounding like me, or check whether a draft already does. Applies the invariant anti-slop mechanics always, and persona flavor only where the resolved persona has a real sample behind it. STE-lite is on by default, so its card wins over the register and over conflicting persona moves. Pass --no-ste for plain voice.
argument-hint: "[target: session (default) | draft text | path | PR #/URL] [--check] [--no-ste] [--persona <name>] [--list]"
allowed-tools:
  - Read
  - Bash
  - Glob
  - Grep
  - Edit
  - AskUserQuestion
---
<objective>
Run the `voice` skill on prose that will go out under my name: draft it, repair it, or check it. The whole point is that it stops reading as generated. With `--list`, do none of that and just show me which personas exist.
</objective>

<execution_context>
@~/.claude/skills/voice/SKILL.md
</execution_context>

<context>
Target: $ARGUMENTS

- Empty → the prose most recently produced in this session. This is the common case, because the usual trigger is something having just been drafted that needs fixing before it goes out.
- Quoted or pasted text → treat the text itself as the draft.
- A file or directory path → read it and treat its prose as the draft. For a code file that means the comments and doc strings, not the code.
- A PR number or URL → the PR body, plus any review comments of mine on it.
- `--check` anywhere in the arguments → check mode only. Report violations and change nothing.
- `--list` anywhere in the arguments → list mode. Print what personas exist and which surfaces resolve to them, then stop. Ignore every other argument, including a target. Nothing is read, drafted, or edited.
- `--no-ste` anywhere in the arguments → plain voice. Skip voice Step 2d, and check with the Step 3 greps instead of the script.
- `--ste` anywhere in the arguments → no change, because STE-lite is already the default.

STE-lite mode (voice Step 2d) is the default, on every surface. The card in `~/.claude/skills/voice/reference/ste-lite.md` wins any conflict with the register or the persona. With `--check`, check without editing: report the script's counts as facts, and card rules 1, 4, 5, and 6, plus the parts of the other rules that no script counts, as calls.

Register decides more than any individual word choice, so name the surface when the target does not make it obvious: `PR description`, `PR reply, own PR`, `Commit message`, `Design or strategy doc`, `Slack`, `Peer feedback`.
</context>

<process>
0. If `--list` is present, do only this and stop. Read `~/.claude/skills/voice/reference/personas.md` and print both of its tables: the roster (which personas exist, how many sources each rests on, which surfaces each evidences, when it was last distilled) and the surface map (which surface resolves to which persona). Then, for each persona in the roster, read `~/.claude/skills/voice/reference/personas/<name>.md` and add its move count and total sample count, because a persona resting on one sample is weaker than one resting on several and the roster does not show that. Name any roster entry whose file is missing rather than skipping it silently. Do not load the skill, do not resolve a surface, and do not touch a draft.
1. Parse `$ARGUMENTS` for the target and the `--check` and `--no-ste` flags, per the context above. If it is empty, use the prose most recently produced in this session.
2. Read the `voice` SKILL.md referenced above and execute it exactly as written, every Step, Constraint, and Common mistake. It is the single source of truth, so do not restate or modify its steps here.
3. Resolve the persona before applying any flavor. Read `~/.claude/skills/voice/reference/personas.md` and look up the
   surface. If a persona is named, read `~/.claude/skills/voice/reference/personas/<name>.md` and apply it. If none is
   mapped, apply the Step 2a invariants only and say so once, naming the surface. If `--persona
   <name>` is passed, use that persona instead of the map's result. A persona named but missing
   its file is an error. In STE-lite mode, the notice says that the pass applies the invariants and
   the STE-lite card.
4. Editing a file in place needs my explicit confirmation first. Show the before and after, then ask. Check mode never edits anything.
5. Report the mechanical findings (em-dash count, semicolon count, banned words, missing contractions) as counted facts, and the judgment findings (register fit, grounding, point first, intent drift) as calls. Do not blur the two, because they carry different confidence. In STE-lite mode, the counts from the voice skill's `scripts/ste_check.py` replace the grep counts, and a missing contraction is not a finding. Under `--no-ste`, the grep counts stand.
</process>
