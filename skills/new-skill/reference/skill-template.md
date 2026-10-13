---
name: {{SKILL_NAME}}
description: {{SKILL_DESCRIPTION}}
# Write the description WHEN-first, third person: lead with "Use when …" and name the
# symptoms or trigger phrases that should route here. Strip workflow recaps, step/dimension
# counts, and marketing words ("Apple-quality", "crystal-clear"). Keep load-bearing safety
# facts (e.g. "never posts without confirmation"). If a sibling skill overlaps, add ONE
# disambiguator line ("For X, use <sibling> instead."). Aim for under ~400 chars.
# Quote-wrap only if it holds ': '.
# Examples:
#   pr:           "Use when a GitHub PR URL is shared and the user asks to review a pull
#                  request. For your own un-pushed local changes, use review-local instead."
#   git-changes:  "Use when drafting a commit or PR, or asked 'what changed?'. Produces a
#                  bounded digest of current uncommitted changes."
# Optional: every skill is typeable as /<name> by default (user-invocable defaults to true).
# Uncomment the line below only to hide a routing-only skill from the / menu.
# user-invocable: false
---

<!-- Scannability scaffold (add what fits this skill; keep it lightweight):
       ## Overview:       one line stating the core principle of the skill.
       ## When to use:     symptom bullets, plus a "Not for X. Use <sibling> instead." line
                           wherever a sibling skill overlaps.
       ## Common mistakes:  fold scattered don'ts into a short cheat sheet.
       ## Quick reference:  a small table ONLY where it genuinely aids scanning.
     Place these above the numbered Steps. Skip any that add no value for a small utility skill. -->

## Preconditions

<!-- Gate conditions that must be true before this skill executes.
     Examples from existing skills:
       - "This skill only applies inside a Go project." (review-code)
       - "Requires the `gh` CLI to be authenticated." (pr)
       - "Only run inside a git repo." (git-draft-commit)
     If no preconditions are needed, remove this entire section. -->

{{PRECONDITIONS}}

## Step 1: {{STEP_1_NAME}}

<!-- Gather context or detect environment.
     Bound the scope early -- determine what kind of input you're working with,
     validate prerequisites, and collect the facts needed for the core logic.
     Keep this step fast and lightweight. -->

{{STEP_1_BODY}}

## Step 2: {{STEP_2_NAME}}

<!-- Core analysis or action.
     This is the main logic of the skill -- the analysis, generation, transformation,
     or workflow that delivers the skill's value. Reference specific conventions,
     patterns, or data sources as needed. -->

{{STEP_2_BODY}}

## Step 3: {{STEP_3_NAME}}

<!-- Output the result.
     Choose the format that fits the skill's purpose:
       - Conversational: respond directly in chat (pr, git-draft-commit)
       - File: write to a specific file using the Write tool (review-code writes reviews.md)
       - YAML block: structured data in a code block (cwd-context, git-changes)
       - Mixed: YAML block + short bullet summary (git-changes)
     Be explicit about the output format so consumers know what to expect. -->

{{STEP_3_BODY}}

<!-- Add more steps as needed. Number them sequentially: Step 4, Step 5, etc.
     Existing skills range from 1 step (peon-ping-toggle) to 8 steps (review-code).
     Use as many steps as clarity requires -- each step should have a single responsibility. -->

## Constraints

<!-- Guardrails that prevent the skill from overstepping or producing unreliable output.
     Every skill should have constraints. Common patterns from existing skills: -->

- Don't claim results without evidence.
- Keep output bounded and skimmable.
- {{SKILL_SPECIFIC_CONSTRAINT}}

## Tone rules

<!-- Only include this section if the skill produces user-facing feedback, critique,
     or review output (e.g., review-code, pr, review-local).
     Remove this section entirely for pure utility or automation skills
     (e.g., git-changes, cwd-context, peon-ping-toggle).

     Common tone patterns from existing skills:
       - Frame findings as suggestions, not demands.
       - Acknowledge good patterns when found.
       - Explain the WHY behind every suggestion.
       - Never use "wrong", "bad", "incorrect" in isolation. -->

{{TONE_RULES}}
