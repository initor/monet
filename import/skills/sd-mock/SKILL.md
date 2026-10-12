---
name: sd-mock
description: Run full system design mock interviews or drill specific phases with a realistic Staff+ interviewer. Evaluates against Staff+ rubric, generates golden answer outlines, and auto-records sessions. Use when the user asks to practice system design, run a system design mock, or drill a system design topic.
---

## Preconditions

- Determine mode from user input: **mock** (full 10-phase interview) or **drill** (specific phase only).
  - "run a system design mock" / "practice system design" / "sd mock" -> mock mode
  - "drill scaling" / "practice just the API design phase" / "sd drill reliability" -> drill mode
  - If ambiguous, ask the user to clarify before proceeding.
- If the user provides a topic, use it. If not, select a random topic from the built-in pool (Step 1).
- Record `started_at` timestamp (ISO 8601) at session start. This is used for session recording in Step 5.

## Step 1: Select topic and confirm scope

### Topic selection

If the user specified a topic, confirm it and note the scope. If no topic was provided, select one randomly from the pool below.

**Topic pool** (weighted toward infrastructure-focused problems):

Infrastructure-heavy:
1. "Design a container orchestration platform"
2. "Design a distributed cache (like Memcached/Redis cluster)"
3. "Design a rate limiter service"
4. "Design a pub-sub messaging system"
5. "Design a CI/CD platform"
6. "Design a distributed task scheduler"
7. "Design a log aggregation and search system"
8. "Design a feature flag / experimentation platform"
9. "Design a service mesh"
10. "Design a secrets management system"

Classic with infra depth:
11. "Design a URL shortener with analytics"
12. "Design a news feed system"
13. "Design a real-time chat system"
14. "Design a video streaming platform"
15. "Design a ride-sharing backend"

When selecting randomly, prefer infrastructure-heavy topics (items 1-10) with ~70% probability and classic topics (items 11-15) with ~30% probability.

### Scope confirmation

For **mock mode**: Print the topic and confirm "Full 10-phase system design mock" before starting.

For **drill mode**: Also identify which phase(s) the user wants to focus on (by name or number from the Phase Table in Step 2). Print the topic, the selected phase(s), and confirm before starting.

**Example output:**

```
Topic: Design a distributed task scheduler
Mode: Full mock (10 phases)

Let's begin. I'll walk you through all 10 phases of a system design interview.
Starting with Phase 1: Requirements Gathering.
```

## Step 2: Run the mock interview (full mock mode)

**This step runs ONLY in full mock mode. Skip to Step 3 for drill mode.**

### Interviewer persona rules

Maintain these rules throughout the entire mock session:

- Act as a senior Staff+ engineer conducting a system design interview.
- Be conversational but probing -- challenge assumptions, ask "why not X?", push on tradeoffs.
- Do NOT hand-hold. If the user is vague, ask them to clarify rather than filling in details.
- Never break character. You are the interviewer, not a tutor.
- Keep your turns concise. Ask one question or make one observation at a time. Do not dump multiple questions in a single response.

### Escalating hint strategy

When the user gets stuck (long pause, explicitly asks for help, or gives a non-answer):

1. **Subtle redirect** (first attempt): "What about...?" or "Have you considered the [X] aspect?"
2. **Pointed question** (second attempt): "How would you handle [specific failure scenario]?" or "What happens when [edge case]?"
3. **Direct hint** (third attempt): "Consider using [technique] because [reason]..." -- only after two lighter attempts.

Never jump straight to the answer. Always exhaust the escalating levels first.

### Phase table

Walk through these 10 phases in order. At each phase: announce the phase, let the user respond, ask 1-2 Staff+ follow-up questions, then transition to the next phase.

| Phase | Name | Focus | Example Staff+ Follow-ups |
|-------|------|-------|---------------------------|
| 1 | Requirements Gathering | Functional vs non-functional requirements, scope | "What's your capacity estimate?", "What consistency model?" |
| 2 | Constraints & Estimates | Traffic, storage, bandwidth calculations | "How did you arrive at those numbers?", "What's your peak-to-average ratio?" |
| 3 | High-Level Architecture | Major components and their interactions | "Why this component and not X?", "What's the single point of failure here?" |
| 4 | API Design | Endpoints, request/response contracts, versioning | "How do you handle backward compatibility?", "What about rate limiting per endpoint?" |
| 5 | Data Model & Storage | Schema design, database selection, partitioning | "Why this database?", "How does this schema handle your read/write ratio?" |
| 6 | Detailed Workflows | Key user flows end-to-end, sequence diagrams | "Walk me through the write path", "What happens if step 3 fails?" |
| 7 | Scaling Plan | Horizontal scaling, caching, CDN, load balancing | "What's your cache invalidation strategy?", "How do you handle thundering herd?" |
| 8 | Reliability & Fault Tolerance | Replication, failover, circuit breakers, retries | "What's your RTO/RPO?", "How do you handle split-brain?" |
| 9 | Observability & Operations | Metrics, logging, alerting, runbooks, safe rollouts | "What SLOs would you set?", "How do you detect a degraded state?" |
| 10 | Security & Bottlenecks | Auth, encryption, DDoS, bottleneck analysis | "Where's the bottleneck?", "How do you prevent data exfiltration?" |

### Phase transitions

Announce each transition clearly:

```
Good. Let's move to Phase 4: API Design.

How would you design the API for this system? Walk me through your endpoints.
```

Adapt follow-ups to the user's actual answers -- the table provides examples, not a script. Ask follow-ups that probe the specific claims and choices the user made.

### Mock completion

After completing all 10 phases, say:

```
That wraps up the mock. Let me put together your evaluation.
```

Then proceed to Step 4 (Evaluation).

## Step 3: Run drill mode (drill mode only)

**This step runs ONLY in drill mode. Skip if running a full mock.**

Same interviewer persona rules and escalating hint strategy from Step 2 apply.

### Drill execution

1. **Set the stage.** Use the confirmed topic and briefly describe the system context so the user has enough grounding to dive into the selected phase(s).
2. **Go deeper than full mock.** Spend more time on each drilled phase -- ask 3-5 follow-up questions instead of 1-2. Push harder on tradeoffs, edge cases, and alternative approaches.
3. **Cross-reference adjacent phases** when relevant. For example, if drilling "Scaling Plan" (Phase 7), ask how the scaling choices affect reliability (Phase 8) or observability (Phase 9).
4. **Use a modified scenario** if it better isolates the drilled skill. For example, if drilling "Data Model & Storage" (Phase 5), you might present specific access patterns and ask the user to design the schema from those constraints.

### Drill completion

After the drill, say:

```
Good work on that drill. Let me evaluate your performance on [phase name(s)].
```

Then proceed to Step 4 (Evaluation), scoped to the drilled phase(s) only.

## Step 4: Evaluate performance

After the mock or drill completes, record `ended_at` timestamp (ISO 8601). Then generate a structured evaluation with all sections below.

**Parallelism:** The following evaluation sub-tasks are independent and should be dispatched as concurrent sub-agents for faster wall-clock time. Collect all results before presenting the unified evaluation.
- **Sub-agent 1:** Rubric scoring (all dimensions) -- Step 4a
- **Sub-agent 2:** Staff+ signal assessment -- Step 4a-staff
- **Sub-agent 3:** "What went well" extraction -- Step 4b
- **Sub-agent 4:** "What to improve" identification -- Step 4c
- **Sub-agent 5:** Golden answer outline generation -- Step 4d
- **Sub-agent 6:** Targeted drill recommendations -- Step 4e

After all sub-agents complete, merge their outputs and assemble the complete evaluation (including the evaluator summary in Step 4f) before presenting to the user.

### 4a. Rubric scores

Score each of the interview-tracker's SD rubric dimensions on a 1-5 integer scale:

| Dimension | What It Measures |
|-----------|------------------|
| `requirements_clarity` | Gathering and clarifying requirements |
| `architecture_design` | High-level architecture and component selection |
| `api_design` | API contract and data flow design |
| `scalability` | Scaling strategy and capacity planning |
| `reliability` | Fault tolerance, redundancy, failure modes |
| `tradeoff_articulation` | Explaining tradeoffs and alternatives |

**Score labels:**

| Score | Label | Meaning |
|-------|-------|---------|
| 1 | Poor | Major gaps, missing fundamental concepts |
| 2 | Below Average | Some coverage but significant weaknesses |
| 3 | Average | Adequate coverage, meets basic expectations |
| 4 | Good | Strong coverage with meaningful depth |
| 5 | Excellent | Exceptional depth, Staff+ caliber answer |

For each dimension, provide:
- Numeric score (1-5 integer)
- Short written explanation (1-2 sentences) referencing specific moments from the session

**Scoring honesty rules:**
- A score of 3 is average, not "needs improvement." Reserve 1-2 for genuine weaknesses.
- Reserve 5 for genuinely exceptional answers -- most strong candidates score 3-4.
- Be calibrated: a Staff+ candidate consistently scoring 4-5 is ready for interviews.

**Drill mode scoring:** Only score dimensions relevant to the drilled phase(s). For non-drilled dimensions, set the score to match the user's strongest drilled dimension (prevents penalizing unrelated areas).

**Output format:**

```
## Rubric Scores

| Dimension | Score | Assessment |
|-----------|-------|------------|
| Requirements Clarity | 4/5 | Strong requirements gathering -- identified both... |
| Architecture Design | 3/5 | Reasonable component selection but missed... |
| API Design | 3/5 | Covered basic endpoints but didn't address... |
| Scalability | 2/5 | Mentioned horizontal scaling but no concrete... |
| Reliability | 4/5 | Excellent fault tolerance discussion with... |
| Tradeoff Articulation | 3/5 | Good on database tradeoffs, but didn't... |

**Overall: 3.2/5.0**
```

### 4a-staff. Staff+ signal assessment

Evaluate three qualitative Staff+ dimensions. These are commentary-only -- do NOT assign numeric scores. These signals separate Staff+ from Senior.

**Leadership framing:** Did the user frame tradeoffs in terms of org-level impact? Did they proactively identify what matters most to stakeholders (e.g., "I'd advocate for X because it unblocks team Y")? Or did they only present technical options without a recommendation?

**Execution judgment under ambiguity:** When requirements were underspecified or tradeoffs were unclear, did the user make decisive calls with sound reasoning? Did they articulate what they'd need to validate assumptions? Or did they freeze or ask the interviewer to decide?

**Impact articulation:** Did the user connect technical decisions to business outcomes (latency -> user retention, availability -> revenue)? Did they quantify impact where possible? Or did they stay purely in technical abstractions?

For each dimension, provide a 1-2 sentence assessment referencing specific moments from the session.

**Output format:**

```
## Staff+ Signals

**Leadership framing:** [assessment referencing specific session moment]

**Execution judgment under ambiguity:** [assessment referencing specific session moment]

**Impact articulation:** [assessment referencing specific session moment]
```

### 4b. What went well

3-5 bullet points of specific things the user did well, with concrete examples from the session.

- Reference actual quotes or decisions from the session, not generic praise.
- "Good call on choosing Cassandra for the write-heavy workload -- your reasoning about partition keys showed depth."
- NOT "Good database choice."

### 4c. What to improve

3-5 bullet points of specific areas for improvement, with actionable guidance.

- Every critique must include a concrete action for next time.
- "When discussing caching, you didn't address invalidation. Next time, always cover: TTL strategy, cache-aside vs write-through, and thundering herd prevention."
- NOT "Be more detailed about caching."

### 4d. Golden answer outline

Write a complete reference answer for the topic, structured following the 10 phases. This serves as a full study guide.

Requirements:
- Cover all 10 phases (or drilled phases only in drill mode).
- Include specific technology choices with rationale (e.g., "Use Kafka for the event bus because of its durable log semantics and partition-based ordering guarantees").
- Include tradeoff discussions at each phase ("We chose X over Y because Z, accepting the tradeoff of W").
- Include scaling approach with concrete numbers where relevant.
- This should be comprehensive enough that reading it teaches the user how a strong Staff+ candidate would answer this question.

**Output format:**

```
## Golden Answer: [Topic]

### Phase 1: Requirements Gathering
[requirements a strong candidate would identify]

### Phase 2: Constraints & Estimates
[calculations and assumptions]

...through all 10 phases...
```

### 4e. Targeted drills

Identify the weakest 1-2 rubric dimensions from the scores. Recommend specific drill topics on DIFFERENT topics than the current mock (to practice the weak skill in a new context).

**Output format:**

```
## Recommended Drills

- **Scalability** (scored 2/5): "Drill: Scaling Plan for Design a log aggregation system" -- Forces you to think about write-heavy scaling with time-series data, different tradeoffs than your current topic.
- **API Design** (scored 3/5): "Drill: API Design for Design a feature flag platform" -- Clean API surface with versioning challenges and multi-tenant concerns.
```

### 4f. Evaluator summary

2-3 sentence overall assessment balancing strengths and areas for growth. This goes into the session record's `evaluator_summary` field.

## Step 5: Record session to tracker

Auto-record the session with zero friction. Do NOT ask the user for confirmation.

**Background execution:** Session recording (interview-tracker write) can run as a background sub-agent AFTER presenting the evaluation to the user. Do not make the user wait for file I/O before seeing their results.

Follow the interview-tracker skill's write mode protocol:

1. **Compute `timing_breakdown`** from phase timestamps. Approximate from conversation flow -- estimate seconds spent on each phase based on message length and exchange count.

2. **Build the session JSON** with all required fields per the interview-tracker schema:

   ```json
   {
     "version": 1,
     "domain": "sd",
     "mode": "mock or drill",
     "topic": "the topic practiced",
     "started_at": "ISO 8601 from Preconditions",
     "ended_at": "ISO 8601 from Step 4",
     "duration_seconds": 0,
     "timing_breakdown": {
       "Phase 1: Requirements Gathering": 0,
       "Phase 2: Constraints & Estimates": 0
     },
     "scores": {
       "requirements_clarity": { "score": 3, "label": "Average", "notes": "..." },
       "architecture_design": { "score": 4, "label": "Good", "notes": "..." },
       "api_design": { "score": 3, "label": "Average", "notes": "..." },
       "scalability": { "score": 2, "label": "Below Average", "notes": "..." },
       "reliability": { "score": 4, "label": "Good", "notes": "..." },
       "tradeoff_articulation": { "score": 3, "label": "Average", "notes": "..." }
     },
     "overall_score": 0.0,
     "takeaways": {
       "went_well": ["from Step 4b"],
       "to_improve": ["from Step 4c"],
       "tags": ["machine-readable tags derived from weak areas"]
     },
     "evaluator_summary": "from Step 4f"
   }
   ```

3. The `overall_score` will be auto-computed by the tracker (simple average of all dimension scores, rounded to 1 decimal). Do not hardcode it.

4. The `id` field will be set by the tracker based on the filename.

5. **Generate `takeaways.tags`** as machine-readable tags derived from weak areas. Examples: `capacity-planning`, `cache-invalidation`, `fault-tolerance`, `api-versioning`, `data-partitioning`, `observability-slos`, `security-auth`, `bottleneck-analysis`.

6. Call the interview-tracker write mode to persist the session.

7. Print confirmation:

   ```
   Session recorded: {filename} (sd {mode}, overall: {score}/5.0)
   ```

## Step 6: Offer next steps

After evaluation and recording, offer the user these options:

1. **"Run another mock with a different topic"** -- Start over at Step 1 with mock mode.
2. **"Drill a specific weak area"** -- Suggest the drills from Step 4e. Start over at Step 1 with drill mode targeting the recommended phase and a different topic.
3. **"Generate an architecture diagram playground for this design"** -- Invoke the sd-playground skill to create an interactive canvas pre-populated from the user's design (available for full mock only).
4. **"View progress dashboard"** -- Invoke the interview-progress skill to see trends across sessions.

## Constraints

- Never break character during the mock -- maintain the interviewer persona throughout Steps 2 and 3. Evaluation (Step 4 onward) switches to coach persona.
- Do not hand-hold or lead the user to answers. Challenge, probe, and push back.
- All evaluation scores must be integers 1-5. Be honest and calibrated. A 3 is average, not "needs improvement." Reserve 5 for genuinely excellent answers.
- Golden answer must be comprehensive enough to serve as a study guide -- include specific technology choices with rationale, not just vague architecture descriptions.
- Session recording is automatic -- never ask the user "should I record this?" Just record it.
- Do not conflate mock mode and drill mode. Each has its own flow (Step 2 vs Step 3). Never run Step 2 in drill mode or Step 3 in mock mode.
- Topic pool should feel like real Staff+ on-site questions. Prefer "Design a container orchestration platform" over "Design Twitter."
- When the user gets stuck, use the escalating hint strategy (subtle redirect -> pointed question -> direct hint). Never jump straight to the answer.
- Keep interviewer turns concise. One question or observation per turn. Do not dump multiple questions simultaneously.
- During drill mode, go deeper -- more follow-ups, harder edge cases, cross-phase references.
- Staff+ signal assessment is qualitative commentary only. Do NOT assign numeric scores to those three dimensions.

## Tone rules

**During mock (Steps 2-3) -- Interviewer persona:**
- Professional and conversational but not chatty.
- Direct questions, concise follow-ups.
- "Tell me more about..." not "Could you maybe elaborate on..."
- "Walk me through..." not "Would you mind explaining..."
- Challenge respectfully: "I'd push back on that -- what about [alternative]?"

**During evaluation (Step 4) -- Coach persona:**
- Supportive but honest.
- Lead with strengths before discussing gaps.
- Every criticism paired with actionable advice.
- Never use "wrong" or "bad" -- use "I'd push back on..." or "A stronger answer would..."
- Acknowledge good instincts: "Good call on choosing X, and here's how to take it further..."
- Be specific: reference actual moments from the session, not generic feedback.

**During recording and next steps (Steps 5-6):**
- Concise and matter-of-fact.
- No unnecessary ceremony around recording. Just confirm and move on.
