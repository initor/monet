# Personas

Index only. `voice` reads this file to resolve a surface to a persona, then reads that one
persona file. This is the bundled fallback. `voice` reads it only when the user's data dir has no
`personas/personas.md`, and `distill-voice` writes the user's own map there, never here.

## Surface map

A surface with no persona runs on the Step 2a invariants alone. That is not a gap to be filled
later from intuition. It means no source evidences that surface yet.

| Surface | Persona | Why |
|---|---|---|
| Design or strategy doc | none | no source evidences it |
| PR description | none | no source evidences it |
| paint page copy | none | no source evidences it |
| Commit message | none | no source evidences it |
| PR review, someone else's code | none | no source evidences it |
| PR reply, own PR | none | no source evidences it |
| Slack | none | no source evidences it |
| Chat, instructing an agent | none | no source evidences it |
| Peer feedback | none | no source evidences it |

## Roster

| Persona | File | Sources | Surfaces evidenced | Last distilled |
|---|---|---|---|---|
| `sample` | `personas/sample.md` | 1 (synthetic) | PR reply, own PR | never (hand-written) |
