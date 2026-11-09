# Kickoff

The Brain follows these steps for the `kickoff` verb. Read `<skill>/reference/rules.md` first, and follow it. `<home>` is the folder of this session, and `<run>` is `<home>/.planning/source/<slug>`.

## Before the questions

The owner types `/monet:source kickoff [<file>] [--slug <slug>]`.

1. When the owner gives `<file>`, read it first with the Read tool.
2. Take from it the context, the goal, the invariants, the persona cards, and the page link.
3. Run `mkdir -p <run>/bin`, and then the build line of the skill.
4. Run `<run>/bin/monet-source init --slug <slug> --holder <session>`.
5. On exit 4, tell the owner that another session drives the run, and stop.
6. When `init` says that the run already has state, tell the owner to type `/monet:source resume`, and stop.

## Questions

Ask the owner one question at a time, with AskUserQuestion. Ask only for what the file and the run cannot give. Ask a question only when its answer changes the direction of the run. A question that needs research goes to the run, not to the owner.

The owner gives the context, the expectations, the invariants already decided, the eventual goal, the persona cards, and the run page. The skill ships no persona card.

## Charter

Write `<run>/charter.json` with the Write tool. It holds these fields.

| Field | What it holds |
|---|---|
| `version` | The number 1. |
| `goal` | The eventual goal of the run. |
| `context` | What the run must know before it starts. |
| `invariants` | The hard constraints. Each one has an `id` such as `I1`, and a `text`. |
| `deliverable` | What the run delivers to the owner. |
| `phases` | The phases in order: `research`, then `design` when the run needs it. |
| `quality_bar` | The bar that the evaluators judge each version against. |
| `personas` | The persona cards, with the fields of the next table. |
| `page` | The https link of the private Notion page of the run. |
| `grants` | Each outward step that the charter covers, as a `step` and the `words` id that grants it. |

Each persona card holds these fields.

| Field | What it holds |
|---|---|
| `id` | A name of lowercase letters, digits, and hyphens that starts with a letter, such as `code-reviewer`. |
| `description` | A short text of what the persona checks. |
| `weight` | `blocking` or `advisory`. At least one card is `blocking`, because only a blocking persona can accept. |
| `sources` | The links and files that the persona reads. |
| `phases` | Optional. A card with no phases joins a round only when the Brain picks it. |

An invariant is a hard constraint, and nobody in the run changes it. The run decides everything outside the charter. For each grant, run `<run>/bin/monet-source relay words` after the owner grants the step, and use the id that it prints.

1. Run `<run>/bin/monet-source charter lint`.
2. On exit 2, fix each `charter:` line that it prints, and run it again.
3. Run `<run>/bin/monet-source charter render --holder <session>`. It writes `<run>/charter.md`.
4. Show `<run>/charter.md` to the owner, and ask the owner to type the yes in the chat. Do not take the yes from AskUserQuestion: `relay words` copies only the messages that the owner types.
5. When the owner asks for a change, edit `charter.json`, and go back to step 1.
6. After the yes, run `<run>/bin/monet-source relay words`. It prints the id of each new entry.
7. Read the text of that entry in `<run>/owner-words.md`. When it does not confirm `charter.md`, go back to step 4.
8. Run `<run>/bin/monet-source charter seal --words <id> --holder <session>`, with the id of the yes.
9. On exit 2 with `came before charter.md was rendered`, the entry is older than the render. Go back to step 4.
10. On exit 3, render the charter again, and get a new yes of the owner.

## Briefs

After the seal, write one full brief for each persona, once. Expand the description of the card into the brief. Each brief includes the charter, so that the evaluator judges against the eventual goal and the quality bar.

1. Write the brief with the Write tool to `<run>/personas/<id>.in.md`.
2. Run `<run>/bin/monet-source persona set <id> --holder <session> < <run>/personas/<id>.in.md`.

The engine writes `<run>/personas/<id>.md` with its hash. Every check of one persona then reads the same brief.

## Preflight

1. Run `<run>/bin/monet-source preflight`. It checks the tools, the power, the lid, and the relay config.
2. It prints one `FAIL <check>: <detail>` line for each check that fails.
3. Run `gh auth status`.
4. Fetch the page with the Notion fetch tool, and read each source of each card.
5. Give the owner every failure, each `FAIL` line, and each source that does not load, in one list.
6. Ask the owner only about the items of that list. After the fixes, run the preflight again.
7. When the preflight is clean, post `[source <slug>] Page ready.` on the page.

## Pilot

The Brain starts the developer while the owner is present. This session runs in accept-edits mode.

1. Run `<run>/bin/monet-source dev start --holder <session>`. It prints a command.
2. Run that command from `<home>` with Bash.
3. Run `<run>/bin/monet-source status` every 30 seconds, for 10 minutes at most.
4. Wait until the `lease dev` line names the developer session.
5. The developer then runs the pilot of `<skill>/reference/developer.md`.
6. In the pilot, `pwd -P` must print `<run>/dev`.
7. Each tool of the developer makes one harmless call, and records it with `decision add`.
8. Read the pilot decisions in `<run>/decisions.tsv`.

When the pilot fails, or a permission prompt stops it, that is one more item of the preflight list. Only when the pilot shows a prompt, ask the owner for allow rules. These rules are for `claude --bg`, `claude agents`, and `<run>/bin/monet-source`.

## Start the run

The run starts after the seal, a clean preflight, and a clean pilot. Tell the owner that the run started, with the page link. Then follow `<skill>/reference/brain.md`.
