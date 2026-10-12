---
name: interview-progress
description: Generate an interactive HTML dashboard showing interview prep progress -- score trends, dimension histories, and automatic weak area detection across all practice sessions. Use when the user asks to view progress, check stats, see their dashboard, or track interview prep improvement over time.
---

## Preconditions

- Session data directory `~/.config/interview-prep/` must exist and contain at least one `.json` session file.
- If the directory does not exist or contains no JSON files, tell the user: "No sessions recorded yet. Run a mock interview session first to start tracking progress."

## Step 1: Load and parse all sessions

1. Run the following to find all session files:

   ```bash
   ls -1 ~/.config/interview-prep/*.json 2>/dev/null
   ```

2. Read each JSON file using the Read tool. Parse the JSON content. Skip any file that fails to parse and log a warning: "Skipping {filename}: invalid JSON."

3. Group sessions by `domain` field (`sd`, `bq`, `code`). Each group is an array of session objects.

**Parallelism:** Dispatch 3 parallel sub-agents, one per domain:
- Sub-agent 1 (SD): Load `*_sd_*.json` sessions → compute stats → detect weak areas → generate SD chart data
- Sub-agent 2 (BQ): Load `*_bq_*.json` sessions → compute stats → detect weak areas → generate BQ chart data
- Sub-agent 3 (Code): Load `*_code_*.json` sessions → compute stats → detect weak areas → generate Code chart data
Collect all three results, then merge into unified dashboard HTML.

4. Sort each group by `started_at` ascending (chronological order for trend lines).

5. Compute aggregate stats per domain:

   - **Total sessions count** for that domain
   - **Average overall score** across all sessions in the domain
   - **Score trend direction** -- compare the average `overall_score` of the last 5 sessions (or all sessions if fewer than 5) to the all-time average:
     - Improving: last-5 average is 0.3+ points above all-time average
     - Declining: last-5 average is 0.3+ points below all-time average
     - Flat: difference is less than 0.3 points
   - **Per-dimension averages** -- for each rubric dimension in the domain, compute the average score across all sessions

6. Also compute global stats:
   - Total session count across all domains
   - Date range: earliest `started_at` to latest `started_at` across all sessions

## Step 2: Detect weak areas

Weak area detection uses a combination approach. A dimension is flagged as a weak area if ANY of the following conditions are true:

### Trigger conditions

1. **Absolute floor:** The dimension's average score across all sessions for that domain is <= 2.5
2. **Relative weakness:** The dimension's average score is more than 0.8 points below the domain's overall average score (the average of all dimension averages for that domain)
3. **Declining trend:** The dimension's average score over the last 3 sessions is 0.5+ points lower than its average over the prior 3 sessions (requires at least 6 sessions)

### Minimum sessions threshold

Weak area detection activates only after **3 sessions per domain**. For domains with fewer than 3 sessions, do not flag any weak areas. Instead, note: "Need {N} more {domain} sessions for weak area detection" (where N = 3 minus current count).

### Recording weak areas

For each flagged weak area, record:

| Field | Value |
|-------|-------|
| `dimension` | The dimension key (e.g., `scalability`) |
| `domain` | The domain (`sd`, `bq`, or `code`) |
| `average_score` | Current average score for this dimension |
| `trigger` | Which condition triggered it: `floor`, `relative`, or `declining` |
| `suggestion` | "Practice more {domain_label} sessions focusing on {dimension_name}" |

Domain labels for suggestions: `sd` = "system design", `bq` = "behavioral", `code` = "coding".

## Step 3: Generate HTML dashboard

Generate a self-contained HTML file at `~/.config/interview-prep/dashboard.html`. Write the file using the Write tool.

The HTML file must be fully self-contained with all CSS inline. The only external resource allowed is Chart.js loaded from CDN: `https://cdn.jsdelivr.net/npm/chart.js`.

### Dashboard structure

Build the HTML string programmatically using all the data computed in Steps 1 and 2. The dashboard has these sections:

#### Header section

```html
<h1>Interview Prep Progress</h1>
```

Summary stats bar showing:
- Total sessions (e.g., "12 sessions")
- Per-domain counts (e.g., "5 SD, 4 BQ, 3 Coding")
- Date range (e.g., "Feb 1 - Feb 22, 2026")
- Per-domain trend indicators: arrow up (improving), arrow down (declining), or dash (flat), color-coded (green for improving, red for declining, gray for flat)

#### Weak areas summary (top of page, after header)

If any weak areas were detected across all domains:
- Show a prominent "Focus Areas" section
- List weak areas grouped by domain
- Each entry shows: dimension name, average score, trigger reason, and suggested focus
- Color-code by severity: score <= 2.0 is red, 2.0-2.5 is orange, 2.5-3.0 is yellow

If no weak areas were detected:
- Show: "No weak areas detected -- keep practicing!"

If some domains have insufficient sessions:
- Show the "Need N more sessions" notes for those domains

#### Per-domain sections

Create one section for each domain that has sessions. Only show domains with at least 1 session. Order: sd, bq, code.

Each domain section contains:

1. **Domain header** with the domain name and session count
   - Domain colors: blue (`#3B82F6`) for sd, green (`#10B981`) for bq, orange (`#F59E0B`) for code

2. **Overall score trend chart** -- A Chart.js line chart:
   - X-axis: session dates (from `started_at`, formatted as short date)
   - Y-axis: 1 to 5 scale
   - Single line showing `overall_score` for each session in chronological order
   - Line color matches the domain color
   - Include a horizontal reference line at score 3.0 (dashed, gray)

3. **Dimension score history chart** -- A Chart.js line chart with multiple lines:
   - One line per rubric dimension for the domain
   - Color-coded with distinct colors per dimension
   - Legend shows dimension names (human-readable, replacing underscores with spaces)
   - X-axis: session dates, Y-axis: 1-5 scale

4. **Session history table** -- A table with columns:
   - Date (formatted as YYYY-MM-DD)
   - Mode
   - Topic
   - Overall Score
   - One column per dimension score
   - Most recent session first (reverse chronological)
   - Score cells color-coded: 1-2 red background, 3 yellow background, 4-5 green background

5. **Weak areas panel** (only if weak areas detected for this domain):
   - Card showing each weak dimension
   - Includes: dimension name, current average, trigger reason, suggested focus

#### Graceful degradation

If Chart.js CDN is unreachable, the dashboard must still render. Wrap chart initialization in a try-catch. If `Chart` is undefined, display a note in place of each chart: "Charts unavailable (offline mode). See table below for data."

### Styling guidelines

Apply these styles inline in a `<style>` block:

- Light background (`#f8fafc`), dark text (`#1e293b`)
- Card-based layout with white backgrounds, subtle shadows (`box-shadow: 0 1px 3px rgba(0,0,0,0.1)`)
- Responsive: max-width container (1200px), centered, with padding
- Domain color palette: blue for sd, green for bq, orange for code
- Score colors in tables: 1-2 = `#fee2e2` (red), 3 = `#fef9c3` (yellow), 4-5 = `#dcfce7` (green)
- Clean sans-serif font (system font stack)
- Charts sized to fit their container (responsive: true in Chart.js config)
- Section spacing with clear visual separation between domains

### Chart.js configuration notes

For each chart, use this general pattern:

```javascript
new Chart(ctx, {
  type: 'line',
  data: { labels: [...dates], datasets: [...] },
  options: {
    responsive: true,
    scales: {
      y: { min: 1, max: 5, ticks: { stepSize: 1 } }
    },
    plugins: {
      legend: { display: true/false }
    }
  }
});
```

- Overall score charts: `legend.display: false` (single line, self-evident)
- Dimension charts: `legend.display: true` with `position: 'bottom'`
- Use `tension: 0.3` for slightly curved lines
- Point radius: 4, hover radius: 6

## Step 4: Open dashboard in browser

1. After writing the HTML to `~/.config/interview-prep/dashboard.html` using the Write tool, open it:

   ```bash
   open ~/.config/interview-prep/dashboard.html
   ```

2. Print a CLI summary to the conversation:

   ```
   Progress Dashboard generated and opened in browser.

   Sessions: {total} ({sd_count} SD, {bq_count} BQ, {code_count} Coding)
   Weak areas: {count} detected ({comma-separated list of dimension names, or "none"})
   Dashboard: ~/.config/interview-prep/dashboard.html
   ```

   Replace placeholders with actual computed values.

## Constraints

- The HTML file must be fully self-contained (inline CSS, Chart.js from CDN is the only external resource).
- Never modify session JSON files. This skill is read-only against session data.
- All-time window by default. Do NOT implement date filtering.
- If Chart.js CDN is unreachable, the dashboard must gracefully degrade (show tables without charts, not a blank page).
- Weak area detection must be automatic -- no manual tagging by the user.
- The dashboard file (`~/.config/interview-prep/dashboard.html`) is overwritten each time the skill runs. It is a generated artifact, not persistent data.
- Skip malformed JSON files silently (log a warning but continue processing).
- If a session's `scores` object is missing expected dimensions for its domain, still include the session but only chart the dimensions that are present.
- Domain dimension reference (from interview-tracker schema):
  - `sd`: requirements_clarity, architecture_design, api_design, scalability, reliability, tradeoff_articulation
  - `bq`: story_structure, scope_and_impact, decision_rationale, leadership_signals, communication_clarity
  - `code`: problem_understanding, solution_design, code_quality, correctness, complexity_analysis
