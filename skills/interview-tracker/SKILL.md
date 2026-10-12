---
name: interview-tracker
description: Use when asked to record, read, and manage interview preparation practice sessions, or when a mock skill records its results. Persists session data with rubric scores, takeaways, and timing metadata to the interview-prep folder of the user's monet data dir.
---

## Data dir

The data dir is `${user_config.data_dir}`. If the data dir above is empty or starts with `${user_config`, it isn't configured, so use `~/.config/monet`. Below, `<data dir>` stands for that path, and sessions live in `<data dir>/interview-prep/`. In Bash, write `<data dir>` unquoted, so a leading `~` expands. For the Write tool, expand a leading `~` to the home directory.

## Preconditions

None. This skill creates the storage directory if it does not exist.

## Step 1: Ensure storage directory

Run the following to guarantee the storage directory exists:

```bash
mkdir -p <data dir>/interview-prep/
```

This is idempotent and safe to call every time.

## Step 2: Session schema reference

Every session file is a JSON document conforming to this canonical schema. All domains share a unified format differentiated by the `domain` field.

### Required fields

```json
{
  "version": 1,
  "id": "2026-02-22_sd_mock_001",
  "domain": "sd | bq | code",
  "mode": "mock | drill | story-bank",
  "topic": "string -- what was practiced (e.g., 'Design a URL shortener')",
  "started_at": "ISO 8601 timestamp",
  "ended_at": "ISO 8601 timestamp",
  "duration_seconds": 0,
  "timing_breakdown": {
    "phase_or_section_name": 0
  },
  "scores": {
    "dimension_name": {
      "score": 3,
      "label": "Average",
      "notes": "string -- brief evaluator notes for this dimension"
    }
  },
  "overall_score": 3.0,
  "takeaways": {
    "went_well": ["string -- things done well"],
    "to_improve": ["string -- areas needing improvement"],
    "tags": ["string -- machine-readable topic tags for weak area detection"]
  },
  "evaluator_summary": "string -- 2-3 sentence overall assessment"
}
```

**Field details:**

| Field | Type | Description |
|-------|------|-------------|
| `version` | integer | Schema version, always `1` for now |
| `id` | string | Unique identifier matching the filename (without `.json`) |
| `domain` | string | One of `sd`, `bq`, `code` |
| `mode` | string | One of `mock`, `drill`, `story-bank` |
| `topic` | string | What was practiced |
| `started_at` | string | ISO 8601 timestamp when session began |
| `ended_at` | string | ISO 8601 timestamp when session ended |
| `duration_seconds` | number | Total session duration in seconds |
| `timing_breakdown` | object | Keys are phase/section names, values are seconds spent |
| `scores` | object | Keys are rubric dimension names, values are score objects |
| `scores.*.score` | integer | 1-5 rating |
| `scores.*.label` | string | Human-readable label for the score |
| `scores.*.notes` | string | Brief evaluator notes for this dimension |
| `overall_score` | number | Simple average of all dimension scores, rounded to 1 decimal |
| `takeaways.went_well` | array | Strings describing what went well |
| `takeaways.to_improve` | array | Strings describing areas for improvement |
| `takeaways.tags` | array | Machine-readable tags for weak area detection (e.g., `capacity-planning`, `star-format`, `edge-cases`) |
| `evaluator_summary` | string | 2-3 sentence overall assessment |

### Score labels mapping

| Score | Label |
|-------|-------|
| 1 | Poor |
| 2 | Below Average |
| 3 | Average |
| 4 | Good |
| 5 | Excellent |

### Domain-specific rubric dimensions

Each domain defines its own scoring dimensions. When recording a session, the `scores` object must contain exactly the dimensions listed for that domain.

#### System Design (`sd`) -- 6 dimensions

| Dimension Key | What It Measures |
|---------------|------------------|
| `requirements_clarity` | Gathering and clarifying requirements |
| `architecture_design` | High-level architecture and component selection |
| `api_design` | API contract and data flow design |
| `scalability` | Scaling strategy and capacity planning |
| `reliability` | Fault tolerance, redundancy, failure modes |
| `tradeoff_articulation` | Explaining tradeoffs and alternatives |

#### Behavioral (`bq`) -- 5 dimensions

| Dimension Key | What It Measures |
|---------------|------------------|
| `story_structure` | STAR+ format completeness (Situation, Task, Action, Result, Impact) |
| `scope_and_impact` | Demonstrating Staff+ scope and organizational influence |
| `decision_rationale` | Explaining decision-making with evidence and tradeoffs |
| `leadership_signals` | Mentoring, alignment, and cross-team influence |
| `communication_clarity` | Conciseness, coherence, and compelling narrative |

#### Coding (`code`) -- 5 dimensions

| Dimension Key | What It Measures |
|---------------|------------------|
| `problem_understanding` | Clarifying questions, edge cases, constraint identification |
| `solution_design` | Algorithm choice, data structure selection, approach explanation |
| `code_quality` | Clean code, naming, modularity, readability |
| `correctness` | Handling edge cases, off-by-one errors, null checks |
| `complexity_analysis` | Time and space complexity articulation |

## Step 3: Record a session (write mode)

When called with session data to record:

1. **Generate the filename.** Format: `{YYYY-MM-DD}_{domain}_{mode}_{NNN}.json` where `NNN` is a zero-padded 3-digit sequence number.

   To determine `NNN`:
   ```bash
   ls -1 <data dir>/interview-prep/{date}_{domain}_{mode}_*.json 2>/dev/null
   ```
   Parse the highest existing sequence number and increment by 1. If no matching files exist, start at `001`.

   **Collision guard:** After determining the target filename, check if it already exists. If it does (e.g., from a concurrent session), increment `NNN` and recheck until a free slot is found. This prevents silent overwrites from parallel mock sessions.

2. **Validate required fields.** Every field in the schema above is required, except `id`, which step 4 sets from the filename. If any field is missing or has an invalid type, report which fields are missing/invalid and stop. Do not write a partial session.

   Validation rules:
   - `version` must be `1`
   - `domain` must be one of `sd`, `bq`, `code`
   - `mode` must be one of `mock`, `drill`, `story-bank` (note: `story-bank` is valid only for `bq` domain)
   - `scores` must contain exactly the dimensions defined for the given domain (see rubric tables above)
   - Each score must be an integer from 1 to 5
   - Each score label must match the score labels mapping
   - `started_at` and `ended_at` must be valid ISO 8601 timestamps
   - `duration_seconds` must be a non-negative number, computed as `ceil(ended_at - started_at)` in seconds, never estimated
   - `takeaways.went_well`, `takeaways.to_improve`, and `takeaways.tags` must each be non-empty arrays

3. **Compute `overall_score`.** Calculate the simple average of all `scores.*.score` values, rounded to 1 decimal place. Set this value in the session data, overriding any provided value.

4. **Set the `id` field.** Set it to the filename without the `.json` extension (e.g., `2026-02-22_sd_mock_001`).

5. **Write the file.** Use the Write tool to write the JSON to `<data dir>/interview-prep/{filename}` with 2-space indented formatting.

6. **Confirm.** Print a confirmation line:
   ```
   Session recorded: {filename} ({domain} {mode}, overall: {overall_score}/5.0)
   ```

## Step 4: List sessions (read mode)

When called to list or read sessions:

1. **List session files.**
   ```bash
   ls -1 <data dir>/interview-prep/*.json 2>/dev/null
   ```

2. **Read and parse each file.** For each JSON file, extract the key fields.

3. **Apply filters.** If a domain filter is provided (e.g., "show only sd sessions"), only include sessions matching that domain.

4. **Sort by date descending** (most recent first). Use the `started_at` field for sorting.

5. **Output a summary table** in the conversation:

   ```
   | Date       | Domain | Mode  | Topic                          | Overall |
   |------------|--------|-------|--------------------------------|---------|
   | 2026-02-22 | sd     | mock  | Design a URL shortener         | 3.8/5.0 |
   | 2026-02-21 | bq     | mock  | Tell me about a time you led.. | 4.2/5.0 |
   ```

6. **Include summary statistics** below the table:
   - Total sessions (and per-domain counts)
   - Average overall score (and per-domain averages)
   - Date range (first session to most recent)

## Constraints

- Never modify an existing session file. Sessions are append-only.
- Always validate the schema before writing. Reject incomplete data with a clear error listing all missing or invalid fields.
- File naming must be deterministic -- given the same date/domain/mode, sequence numbers increment predictably.
- This skill defines the schema and handles persistence only. It does NOT evaluate sessions. Evaluation happens in the domain-specific mock skills (`sd-mock`, `bq-mock`, `code-mock`), which call this skill to persist their results.
- Score values must be integers from 1 to 5 inclusive. No fractional scores.
- The `overall_score` is always recomputed on write -- never trust a caller-provided value.
- If the storage directory is empty or does not exist when listing, report "No sessions recorded yet" instead of an error.
