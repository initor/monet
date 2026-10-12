---
name: bq-mock
description: Use when the user asks to practice behavioral questions, build stories, run a behavioral mock, or add stories to their story bank. Builds STAR+ stories collaboratively or runs live behavioral mocks with Staff+ evaluation.
---

## Preconditions

- Determine mode from user input:
  - "build a story" / "story bank" / "add a story" / "bq story" -> **story-bank mode** (Step 1)
  - "run a behavioral mock" / "practice behavioral" / "bq mock" -> **mock mode** (Step 2)
  - If ambiguous, ask the user to clarify: "Do you want to build stories for your bank, or run a live behavioral mock?"
- Record `started_at` timestamp (ISO 8601) at session start. This is used for session recording in Step 4.

## Data dir

The data dir is `${user_config.data_dir}`. If the data dir above is empty or starts with `${user_config`, it isn't configured, so use `~/.config/monet`. Below, `<data dir>` stands for that path, and sessions live in `<data dir>/interview-prep/`. In Bash, write `<data dir>` unquoted, so a leading `~` expands. For the Write tool, expand a leading `~` to the home directory.

## Step 1: Story-bank mode -- collaborative STAR+ extraction

**This step runs ONLY in story-bank mode. Skip to Step 2 for mock mode.**

Claude acts as a coach who extracts stories through interview-style questioning. This is NOT a form to fill out -- it is a conversation. Probe, challenge, and dig for detail at every turn.

### Story extraction flow

1. **Open with a prompt.** Ask the user for a situation to start with. Offer suggestions but let the user bring their own topic:

   ```
   Let's build a story. Tell me about a time you:
   - Led a cross-team initiative
   - Solved a hard technical problem under pressure
   - Influenced a decision without having authority
   - Mentored someone through a significant growth moment

   Or bring your own -- what's a situation you want to capture?
   ```

2. **Probe each STAR+ component sequentially.** Do NOT move to the next component until the current one has sufficient depth. Ask follow-ups within each component as needed.

   | Component | Probing Questions |
   |-----------|-------------------|
   | **Situation** | "Set the scene. What team were you on? What was the business context? What was at stake if nothing changed?" |
   | **Task** | "What specifically were you responsible for? What was YOUR role versus the team's? Why were you the right person for this?" |
   | **Action** | "Walk me through exactly what you did. Step by step. What decisions did you make and why? What alternatives did you consider and reject?" |
   | **Result** | "What was the measurable outcome? Numbers? Before and after?" |
   | **Scope of impact** | "How far did this reach beyond your immediate team? Who else benefited?" |
   | **Quantified metrics** | "Can you put numbers on this? Users affected? Time saved? Revenue impact? Latency improvement? Incidents prevented?" |
   | **Long-term prevention / systemic fix** | "What did you do to make sure this class of problem never happened again? Was there a systemic change?" |
   | **Org-level impact** | "How did this change the team's culture, processes, or capabilities? Did anyone grow because of this work?" |

3. **Push hard for Staff+ signals on EVERY story.** Do NOT mark a story complete until at least 2 of these 3 are addressed:

   - **Influence**: "Who did you need to convince? How did you get alignment from skeptics? What was the hardest pushback you received and how did you handle it?"
   - **Alignment**: "How did this connect to the organization's priorities? How did you frame this for leadership? Why should anyone above your skip-level care about this?"
   - **Mentoring**: "Did you bring anyone along on this? How did you help others level up? Did someone grow as a direct result of this project?"

   If the user says a signal does not apply to this story, probe once more: "Are you sure? Even a brief example of [signal] would strengthen this story significantly." If they still have nothing, accept it but note the gap.

4. **Present the complete STAR+ breakdown.** After all components are extracted, display a structured summary:

   ```
   ## Story: [Short Title]

   **Situation:** [condensed version]
   **Task:** [condensed version]
   **Action:** [condensed version with key decisions highlighted]
   **Result:** [condensed version with metrics]

   **Staff+ Depth:**
   - Scope of impact: [summary]
   - Quantified metrics: [numbers]
   - Systemic fix: [what changed long-term]
   - Org-level impact: [culture/process/growth changes]

   **Staff+ Signals:**
   - Influence: [summary or "Not addressed"]
   - Alignment: [summary or "Not addressed"]
   - Mentoring: [summary or "Not addressed"]
   ```

5. **Auto-tag with competency themes.** Assign 2-3 tags from the taxonomy below based on the story content. Display them with the summary.

### Competency theme taxonomy

Use these tags for auto-tagging stories. Each story gets 2-3 tags.

| Tag | When to Apply |
|-----|---------------|
| `technical-leadership` | Led technical direction, made architecture decisions, set technical strategy |
| `cross-team-influence` | Worked across team boundaries, coordinated with other orgs, influenced without authority |
| `conflict-resolution` | Navigated disagreements, resolved tensions, built consensus from opposing views |
| `mentoring-and-growth` | Coached others, helped someone level up, built team capability |
| `decision-making-under-uncertainty` | Made high-stakes calls with incomplete information, managed ambiguity |
| `operational-excellence` | Improved reliability, reduced incidents, built operational processes |
| `strategic-thinking` | Identified long-term investments, connected work to business strategy |
| `delivering-under-pressure` | Hit deadlines under constraints, managed crises, shipped despite obstacles |
| `stakeholder-management` | Managed expectations, communicated with leadership, navigated organizational dynamics |
| `process-improvement` | Improved workflows, reduced toil, created repeatable processes |
| `innovation` | Introduced novel approaches, created new tools or systems, challenged status quo |
| `failure-and-recovery` | Handled a failure, owned a mistake, turned a setback into learning |

6. **Store the story in the conversation context** for potential use in mock mode (Step 2). Print the final summary with tags:

   ```
   Story banked: "[Short Title]"
   Tags: technical-leadership, cross-team-influence
   Staff+ signals: influence (strong), alignment (strong), mentoring (not addressed)

   Ready to build another story, or want to run a mock using your stories?
   ```

### Story-bank session completion

After the user indicates they are done building stories (or after completing the current story if they do not want to continue):
- If only story-bank mode was used, offer Step 3 for story quality feedback (optional but recommended). If the user declines, skip Steps 3 and 4 and go to Step 5: with no evaluation there are no scores, so nothing is recorded.
- If user wants to switch to mock mode, proceed to Step 2.

## Step 2: Mock mode -- live behavioral interview

**This step runs ONLY in mock mode. Skip if running story-bank mode only.**

Claude acts as a realistic Staff+ behavioral interviewer -- challenging, specific, and unwilling to accept vague answers. This should feel like a real 45-minute behavioral round.

### Interviewer persona rules

Maintain these rules throughout the entire mock session:

- Act as a senior Staff+ engineering leader conducting a behavioral interview.
- Be conversational but probing -- challenge vague answers, ask for specifics, push on impact.
- Do NOT hand-hold. If the user gives a vague answer, push harder rather than accepting it.
- Never break character. You are the interviewer, not a tutor.
- Keep your turns concise. Ask one question at a time. Do not stack multiple questions.
- Escalating challenge: Start with open questions, then probe gaps.
  - First: "Can you tell me more about that?"
  - Second: "Can you be more specific about the numbers?"
  - Third: "I need to hear a concrete result -- what metric moved and by how much?"
- Never be rude or dismissive, but never let vague answers slide.

### Question selection

1. **Check for story bank.** If the user built stories in Step 1 during this session, note them for mixed questioning.

2. **Select 3-4 behavioral questions** for the session:
   - **If story bank exists:** 1-2 questions targeting stories in the bank (tests delivery and polish) + 1-2 cold questions outside the bank (tests adaptability). Do NOT tell the user which questions target banked stories.
   - **If no story bank:** All cold questions.

3. **Question categories to draw from** (select questions that cover different competency areas):

   | Category | Example Questions |
   |----------|-------------------|
   | **Leadership & influence** | "Tell me about a time you drove a technical decision that others disagreed with." |
   | **Conflict & resolution** | "Describe a situation where you had a significant disagreement with a peer or manager. How did you resolve it?" |
   | **Mentoring & growth** | "Tell me about someone whose career you significantly impacted. What did you do and what happened?" |
   | **Failure & learning** | "Tell me about a project that failed or didn't meet expectations. What did you learn?" |
   | **Cross-team delivery** | "Describe a time you delivered something that required coordination across multiple teams." |
   | **Strategic thinking** | "Tell me about a time you identified and drove a long-term technical investment." |
   | **Operational excellence** | "Tell me about a time you improved a system's reliability or operational posture significantly." |
   | **Delivering under pressure** | "Tell me about a time you had to deliver on a critical deadline with significant constraints." |

   When selecting questions, vary the categories. Do not ask two questions from the same category in one session.

### Mock execution

For **each question** (3-4 total):

1. **Ask the question.** Cold open, no preamble. Just the question.

2. **Listen to the user's answer.** Let them speak without interrupting.

3. **Follow up aggressively with 2-3 follow-ups** pushing for specifics and Staff+ depth. Draw from:

   - "What was YOUR role specifically, not the team's?"
   - "How did you measure that impact?"
   - "What would you do differently if you faced this again?"
   - "Who pushed back and how did you handle it?"
   - "What was the timeline? How long did this take?"
   - "What did you sacrifice or deprioritize to make this happen?"
   - "How did you get buy-in from leadership?"
   - "What was the most difficult conversation you had during this?"
   - "Walk me through the decision-making process. What were the alternatives?"
   - "How did this affect people beyond your immediate team?"

   Adapt follow-ups to the user's specific answer -- do not use a scripted list. Ask follow-ups that probe the weakest parts of their response.

4. **Transition to the next question.** Brief acknowledgment, then move on:

   ```
   Got it. Let's move on.

   Tell me about a time you [next question]...
   ```

5. **If the user provides a story that contradicts facts from a previously banked story**, flag the inconsistency:

   ```
   Hold on -- earlier when we built your [story title] story, you mentioned [fact X].
   But just now you said [contradicting fact Y]. Can you clarify?
   ```

### Mock completion

After completing all 3-4 questions:

```
That wraps up the mock. Let me put together your evaluation.
```

Then proceed to Step 3 (Evaluation).

## Step 3: Evaluate performance

Run after mock mode (Step 2) completes. Also run optionally after story-bank mode (Step 1) for story quality feedback -- in that case, scope the evaluation to the stories built rather than mock answers. If the user declines story-bank feedback, this step does not run.

**Parallelism:** The following evaluation sub-tasks are independent and should be dispatched as concurrent sub-agents for faster wall-clock time. Collect all results before presenting the unified evaluation.
- **Sub-agent 1:** Rubric scoring (all dimensions) -- Step 3a
- **Sub-agent 2:** Staff+ signal assessment -- Step 3b
- **Sub-agent 3:** "What went well" extraction -- Step 3c
- **Sub-agent 4:** "What to improve" identification -- Step 3d
- **Sub-agent 5:** Golden answer rewrite -- Step 3e
- **Sub-agent 6:** Targeted drill recommendations -- Step 3g

After all sub-agents complete, merge their outputs and assemble the complete evaluation (including story selection assessment in Step 3f if applicable, and session summary in Step 3h) before presenting to the user.

### 3a. Rubric scores

Score against the 5 BQ dimensions from the `monet:interview-tracker` schema. For each dimension, provide a numeric score (1-5 integer), the label, and brief notes referencing specific moments from the session.

| Dimension Key | What It Measures |
|---------------|------------------|
| `story_structure` | STAR+ format completeness (Situation, Task, Action, Result, Impact) |
| `scope_and_impact` | Demonstrating Staff+ scope and organizational influence |
| `decision_rationale` | Explaining decision-making with evidence and tradeoffs |
| `leadership_signals` | Mentoring, alignment, and cross-team influence |
| `communication_clarity` | Conciseness, coherence, and compelling narrative |

**Score labels:**

| Score | Label | Meaning |
|-------|-------|---------|
| 1 | Poor | Major gaps, missing fundamental components |
| 2 | Below Average | Some coverage but significant weaknesses |
| 3 | Average | Adequate coverage, meets basic expectations |
| 4 | Good | Strong coverage with meaningful depth |
| 5 | Excellent | Exceptional depth, Staff+ caliber answer |

**Scoring honesty rules:**
- A score of 3 is average, not "needs improvement." Reserve 1-2 for genuine weaknesses.
- Reserve 5 for genuinely exceptional answers -- most strong candidates score 3-4.
- Be calibrated: a Staff+ candidate consistently scoring 4-5 is ready for interviews.
- A vague answer with no metrics is a 2, not a 3. Do not inflate scores to be nice.

**Output format:**

```
## Rubric Scores

| Dimension | Score | Assessment |
|-----------|-------|------------|
| Story Structure | 4/5 | Strong STAR format with clear situation and action, but result... |
| Scope & Impact | 3/5 | Mentioned team-level impact but didn't articulate org-level... |
| Decision Rationale | 3/5 | Described what was done but didn't explain alternatives... |
| Leadership Signals | 2/5 | No mention of mentoring or cross-team influence... |
| Communication Clarity | 4/5 | Concise and well-organized, good narrative flow... |

**Overall: 3.2/5.0**
```

### 3b. Staff+ signal assessment

Evaluate three qualitative Staff+ dimensions. These are **commentary only -- do NOT assign numeric scores.** These signals separate Staff+ from Senior.

**Leadership framing:** Did the user frame their stories in terms of organizational thinking and ownership? Did they show awareness of stakeholder priorities? Or did they present themselves as an individual contributor who happened to be on the project?

**Execution judgment:** When discussing difficult decisions, did the user articulate why they chose their approach over alternatives? Did they show awareness of tradeoffs and constraints? Or did they describe what happened without explaining their reasoning?

**Impact articulation:** Did the user connect their actions to measurable outcomes? Did they quantify results? Or did they stay in qualitative abstractions ("it was really impactful")?

For each dimension, provide a 1-2 sentence assessment referencing specific moments from the session.

**Output format:**

```
## Staff+ Signals

**Leadership framing:** [assessment referencing specific session moment]

**Execution judgment:** [assessment referencing specific session moment]

**Impact articulation:** [assessment referencing specific session moment]
```

### 3c. What went well

2-3 specific strengths from this session with concrete examples.

- Reference actual quotes or decisions from the session, not generic praise.
- "Your caching migration story had excellent STAR+ structure -- you set the situation clearly, quantified the result (40% latency reduction), and connected it to the team's OKR."
- NOT "Good story structure."

### 3d. What to improve

2-3 specific areas for growth with concrete suggestions.

- Every critique must include a concrete action for next time.
- "When discussing the deployment pipeline project, you said 'it had a big impact' but never quantified. Next time, always close with a number: 'Deploy time went from 45 minutes to 8 minutes, and we went from 2 deploys/week to 12.'"
- NOT "Be more specific about impact."

### 3e. Golden answer rewrite

For the **weakest answer** in the session, provide a complete rewrite in Staff+ framing. This is a study guide showing the user exactly how a Staff+ candidate would deliver this answer.

The rewrite must be a **complete, ready-to-deliver answer** -- not bullet points or an outline.

Structure the rewrite to demonstrate:

1. **Opening** (set context quickly with stakes): 1-2 sentences that establish the situation and why it mattered. Hook the interviewer immediately.

2. **STAR+ flow with emphasis on decisions and impact**: Walk through the story with clear transitions. Highlight decision points explicitly: "I had three options: A, B, and C. I chose B because..."

3. **Quantified results and org-level outcomes**: Close with concrete numbers and describe the ripple effects beyond the immediate team.

4. **Lessons and systemic changes**: End with what changed permanently as a result of this work.

**Output format:**

```
## Golden Answer Rewrite

**Original question:** [the question that received the weakest answer]

**Why this answer needs work:** [1-2 sentences identifying the gap]

**Staff+ version:**

[Complete, polished answer written in first person, ready to deliver verbatim in an interview. 300-500 words. Written as flowing paragraphs, not bullet points.]
```

### 3f. Story selection assessment (mock mode only, when story bank exists)

For each question where the user used a story from the bank, assess whether a different story would have been a stronger match.

**Output format:**

```
## Story Selection Assessment

**Q: "[question text]"**
You used: [story title]
Assessment: [Was this the right story? Would a different banked story have been stronger? Why?]
```

If the user's story selection was optimal, say so: "Strong match -- this story's emphasis on [X] directly addresses what the question is testing."

If no story bank exists, skip this section entirely.

### 3g. Targeted drills

Identify the 1-2 weakest competency areas from the rubric scores. Suggest specific question types to practice next, on DIFFERENT topics than what was covered in this session.

**Output format:**

```
## Targeted Drills

Your weakest area was **[dimension]** (scored [N]/5).

Try these next:
- "[Specific behavioral question targeting the weak area]" -- Forces you to [what this question tests].
- "[Another specific question]" -- Practices [what this develops].
```

### 3h. Session summary

2-3 sentence overall assessment balancing strengths and areas for growth. This goes into the session record's `evaluator_summary` field.

Be honest and specific. "Your storytelling structure is strong with clear STAR components, but you consistently undersell impact by staying qualitative. Practice closing every answer with a concrete metric."

## Step 4: Record session

After evaluation (Step 3) completes, automatically record the session using the `monet:interview-tracker` write protocol, run inline as the steps below give it. Do NOT ask the user for confirmation -- just record it. If Step 3 did not run (story-bank feedback declined), skip this step: there are no scores, and nothing is recorded.

**Background execution:** Session recording (the inline `monet:interview-tracker` write) can run as a background sub-agent AFTER presenting the evaluation to the user. Do not make the user wait for file I/O before seeing their results. Give the sub-agent the data dir resolved above.

### Recording protocol

1. **Record `ended_at` timestamp** (ISO 8601).

2. **Compute `duration_seconds`** from `started_at` and `ended_at`.

3. **Build session JSON** following the `monet:interview-tracker` canonical schema:

   ```json
   {
     "version": 1,
     "domain": "bq",
     "mode": "mock or story-bank",
     "topic": "Primary question or story topic from the session",
     "started_at": "ISO 8601 from Preconditions",
     "ended_at": "ISO 8601 from this step",
     "duration_seconds": 0,
     "timing_breakdown": {
       "Question 1: [topic]": 0,
       "Question 2: [topic]": 0
     },
     "scores": {
       "story_structure": { "score": 3, "label": "Average", "notes": "..." },
       "scope_and_impact": { "score": 3, "label": "Average", "notes": "..." },
       "decision_rationale": { "score": 3, "label": "Average", "notes": "..." },
       "leadership_signals": { "score": 3, "label": "Average", "notes": "..." },
       "communication_clarity": { "score": 3, "label": "Average", "notes": "..." }
     },
     "overall_score": 0.0,
     "takeaways": {
       "went_well": ["from Step 3c"],
       "to_improve": ["from Step 3d"],
       "tags": ["machine-readable tags derived from weak areas"]
     },
     "evaluator_summary": "from Step 3h"
   }
   ```

4. **Compute `overall_score`** as the simple average of all 5 dimension scores, rounded to 1 decimal place.

5. **Set `id`** to the filename without the `.json` extension.

6. **Generate `takeaways.tags`** as machine-readable tags derived from weak areas. Examples: `star-format`, `impact-quantification`, `leadership-framing`, `mentoring-examples`, `conflict-resolution`, `decision-rationale`, `story-selection`, `communication-structure`, `cross-team-influence`, `failure-ownership`.

7. **Follow the `monet:interview-tracker` write protocol** to persist the session under the data dir resolved above:
   - Ensure storage directory: `mkdir -p <data dir>/interview-prep/`
   - Generate filename: `{YYYY-MM-DD}_bq_{mode}_{NNN}.json`
   - Validate all required fields
   - Write the file to `<data dir>/interview-prep/{filename}` with 2-space indented formatting

8. **Print confirmation:**

   ```
   Session recorded: {filename} (bq {mode}, overall: {score}/5.0)
   ```

## Step 5: Next steps

After session recording (or after declined story-bank feedback), suggest next actions based on the mode used:

**If story-bank mode:**
```
What's next?
- "Build another story" -- Add to your story bank
- "Run a mock using your stories" -- Practice delivering under interview pressure
- "Generate a concept map of your story bank" -- Visualize coverage and gaps
- "View progress dashboard" -- See your trends across sessions
```

**If mock mode:**
```
What's next?
- "Build new stories for weak areas" -- Your [weakest area] could use more stories
- "Run another behavioral mock" -- Practice with fresh questions
- "Generate a concept map of your story bank" -- Visualize coverage and gaps
- "View progress dashboard" -- See your trends across sessions
```

## Constraints

- Never reveal the question list before asking. Each question in mock mode must feel like a cold open.
- In story-bank mode, probe for all STAR+ components AND all 3 Staff+ signals (influence, alignment, mentoring) at least once. If a signal doesn't apply after re-probing, accept it but note the gap in the summary.
- Do not provide golden answer rewrites during the mock itself -- only in evaluation (Step 3).
- Always record evaluated sessions to `<data dir>/interview-prep/` (Step 4). Never skip recording. Never ask "should I record this?" The one exception: a story-bank session whose optional feedback the user declined has no scores, so nothing is recorded.
- When scoring, do not inflate scores to be nice. A vague answer with no metrics is a 2, not a 3.
- Story-bank mode does NOT have time pressure. The user should feel coached, not rushed. Mock mode should feel like a real interview with natural pacing.
- Keep mock sessions to 3-4 questions. Do not extend to 5+ even if the user asks -- suggest starting a new session instead: "Let's keep this session tight at 3-4 questions. Want to start a fresh mock with new questions?"
- If the user provides a story in mock mode that contradicts facts from a previously built story, flag the inconsistency immediately.
- Do not conflate story-bank mode and mock mode. Each has its own flow (Step 1 vs Step 2). Never run Step 1 in mock mode or Step 2 in story-bank mode.
- Maintain the interviewer persona throughout Step 2. Do not break character to explain technique or offer coaching mid-mock. Save all coaching for evaluation (Step 3).

## Tone rules

**During story extraction (Step 1) -- Coach persona:**
- Patient, probing, encouraging.
- "That's the kind of detail I'm looking for. Now tell me about the impact..."
- "Good -- can you put a number on that?"
- "I want to hear more about how you convinced the skeptics."
- Push for depth without making the user feel interrogated.
- Celebrate strong details: "That's a Staff+ level answer right there."

**During mock (Step 2) -- Interviewer persona:**
- Professional and conversational but not chatty.
- Direct questions, concise follow-ups.
- "Tell me more about..." not "Could you maybe elaborate on..."
- "Walk me through..." not "Would you mind explaining..."
- Challenge respectfully: "I want to push on that -- you said 'big impact' but I haven't heard a number yet."
- Never accept "we did X" without clarifying "what did YOU do?"

**During evaluation (Step 3) -- Coach persona:**
- Supportive but honest.
- Lead with strengths before discussing gaps.
- Every criticism paired with actionable advice.
- Use "AND" not "BUT": "This was a strong start, AND here's how to make it Staff+ caliber."
- Never use "wrong" or "bad" -- use "I'd push back on..." or "A stronger answer would..."
- Be specific: reference actual moments from the session, not generic feedback.
- Address the user directly: "You mentioned..." not "The candidate mentioned..."

**During recording and next steps (Steps 4-5):**
- Concise and matter-of-fact.
- No unnecessary ceremony around recording. Just confirm and move on.
- Never use corporate jargon ("leverage synergies"). Use plain engineering language.
