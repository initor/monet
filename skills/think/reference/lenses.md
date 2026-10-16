# The three lenses

Paraphrased from Felix Rieseberg's article (13 October 2026). Speed, the fourth kind of ambition, is left out on purpose: the same work faster needs no new method.

## Routing tells (no flag given)

| Tell in the task | Lens |
|---|---|
| A result people will meet about, decide on, or report up | `--higher` |
| A known task where the quality of execution is the point | `--deeper` |
| Creative work, writing for an audience (a README, a deck, a pitch), a stuck problem, or a wish for fresh angles | `--wider` |

Altitude works poorly on creative work. If the user forces `--higher` on creative work, say so in one line and carry on.

## --higher (Altitude)

Ask what should happen next, and what the goal beyond the goal is. The model already runs a best guess at the outcome, so give it the outcome.

Lens block, three rungs, one line each, recommended rung marked:

1. **Task, with full context.** Do the task from all the raw material, and say what changed, why, and what to do about it.
2. **The decisions it serves.** Find the most urgent decisions (three at most), and draft options with the case for and against each.
3. **The process it belongs to.** Name the purpose the task serves, design a better way to reach it, show from past history that it would have done better, and run it beside the old way for one cycle.

Add one line, **Durable**: what access (data, version-to-commit mapping, real environments) would let the model own this outcome without being fed task after task. Example from the article: "fix this crash" became "keep the apps crash-free", which first needed crash reports, version mapping, and Windows, macOS, and Linux machines to reproduce on.

## --deeper (Depth)

Do the task the way a field-leading expert would, until even that expert might learn something.

Lens block, answer four questions, one or two lines each:

1. What would a top professional in this field do here that wasn't asked for?
2. What is the rawest material they would start from, instead of a pre-digested version?
3. Which specific methods and instruments would they use?
4. What, other than the model or the user, would show the result is right?

Pull in whichever of these techniques fits, each as a clause in the Prompt line:

- **Their instruments, not their vocabulary.** Told to act like a pro, models switch words but keep their methods. Insist on the instrument, such as a database and a statistical model instead of a markdown table.
- **Challenge a belief.** Name one thing the team believes, and ask for an attempt to reproduce it from raw data.
- **New knowledge.** Once inside the data, ask for a blind spot, and for proof that it predicts on data the model has not seen.

## --wider (Range)

Bring in more fields than used to be worth the cost. Subagents make a crew cheap.

Lens block:

- **Verbs.** Restate the task as what it does, not what it is (from Tom Kelley and Jonathan Littman, The Art of Innovation). The article's example: a quarterly review is less a deck than persuading busy people of a causal story so they can decide, under uncertainty, within half an hour. One line.
- **Crew.** Two to four fields the verbs point to, each with the procedure it would run on this task. Flaky CI, for example, is keeping a shared signal trustworthy enough that people act on it, which points to statistical process control, alert-fatigue research from on-call practice, and outbreak tracing.
- **Red team.** One or two adversarial fields and their attack, such as a short seller building the case against, or a forensic accountant tracing where the numbers could be wrong.

Use the crew to find fields and techniques. Never ask the model to write like a short seller, ask it to do what a short seller does to research a position.
