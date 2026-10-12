---
name: bq-playground
description: Use when the user asks to visualize stories, see their story bank, generate a concept map, or view story coverage. Generates an interactive concept map playground visualizing your behavioral story bank, with stories as nodes connected by competency themes and question types in dual views. Requires 2+ stories from bq-mock sessions.
---

## Data dir

The data dir is `${user_config.data_dir}`. If the data dir above is empty or starts with `${user_config`, it isn't configured, so use `~/.config/monet`. Below, `<data dir>` stands for that path, and playgrounds live in `<data dir>/interview-prep/playgrounds/`. In Bash, write `<data dir>` unquoted, so a leading `~` expands. For the Write tool, expand a leading `~` to the home directory.

## Preconditions

- The user must have at least 2 stories available. Stories can come from:
  1. A just-completed bq-mock story-bank session (preferred -- stories are already in conversation context)
  2. User-provided structured story data (must include STAR+ components and competency themes)
- If fewer than 2 stories exist, inform the user: "You need at least 2 stories to generate a meaningful concept map. Use `monet:bq-mock` in story-bank mode to build stories first."
- If more than 20 stories are available, warn: "Your story bank has {N} stories. The layout works best with 3-15 stories. Beyond 20, the visualization may become crowded. Proceeding with all stories."
- Record the story count for the coverage gap summary bar.

## Step 1: Extract story data for visualization

From the available stories (conversation context or user-provided), extract a structured dataset for each story.

### Per-story extraction

For each story, capture:

| Field | Source | Example |
|-------|--------|---------|
| **Title** | Short descriptive title from story bank | "Caching Migration", "Deployment Pipeline Redesign" |
| **STAR+ breakdown** | All 8 components from bq-mock extraction | Situation, Task, Action, Result, Scope, Metrics, Prevention, Org Impact |
| **Competency themes** | 2-3 auto-tagged themes from bq-mock | `technical-leadership`, `cross-team-influence` |
| **Applicable question types** | Which behavioral categories this story answers | leadership, conflict, mentoring, failure, cross-team, strategic, operational |
| **Staff+ signals present** | Which of the 3 signals are demonstrated | influence, alignment, mentoring |
| **Strength rating** | Based on STAR+ completeness and specificity | Strong / Moderate / Needs-work |

### Strength rating criteria

Rate each story based on STAR+ completeness and Staff+ signal depth:

| Rating | Criteria |
|--------|----------|
| **Strong** | All 8 STAR+ components have specific detail AND at least 2 of 3 Staff+ signals demonstrated with concrete examples AND quantified metrics present |
| **Moderate** | Most STAR+ components present but 1-2 are vague or thin AND/OR only 1 Staff+ signal demonstrated AND/OR metrics are qualitative rather than quantified |
| **Needs-work** | Multiple STAR+ components missing or vague AND fewer than 1 Staff+ signal AND no quantified metrics |

### Applicable question type mapping

Determine which behavioral question categories each story could answer. A story can map to multiple categories.

| Category | Assign When Story Demonstrates |
|----------|-------------------------------|
| `leadership` | Led direction, drove decisions, set strategy, took ownership of outcomes |
| `conflict` | Navigated disagreements, resolved tensions, handled pushback, built consensus |
| `mentoring` | Coached others, helped someone level up, built team capability, grew individuals |
| `failure` | Handled a failure, owned a mistake, recovered from setback, learned from error |
| `cross-team` | Worked across boundaries, coordinated multiple teams, influenced without authority |
| `strategic` | Long-term investment, connected work to business strategy, identified future needs |
| `operational` | Improved reliability, reduced incidents, built processes, operational excellence |

### Coverage baseline

Build the full coverage baseline for gap detection:

**Competency theme taxonomy** (12 themes from bq-mock):
- `technical-leadership`
- `cross-team-influence`
- `conflict-resolution`
- `mentoring-and-growth`
- `decision-making-under-uncertainty`
- `operational-excellence`
- `strategic-thinking`
- `delivering-under-pressure`
- `stakeholder-management`
- `process-improvement`
- `innovation`
- `failure-and-recovery`

**Question type categories** (7 types):
- `leadership`, `conflict`, `mentoring`, `failure`, `cross-team`, `strategic`, `operational`

**Staff+ signals** (3 signals):
- `influence`, `alignment`, `mentoring`

## Step 2: Generate interactive HTML concept map

**Parallelism:** The HTML generation can be decomposed into independent sub-agents:
- Sub-agent A: Generate CSS (theme, layout, animations)
- Sub-agent B: Generate JavaScript (state management, event handlers, interactivity)
- Sub-agent C: Extract and embed data (components/stories/problems/concepts as inline JSON)
Dispatch all three concurrently. Merge results into the final HTML template.

**Story-level parallelism:** When extracting stories from conversation context or user-provided story data, the per-story STAR+ extraction and strength rating computation can be parallelized across stories. Dispatch one sub-agent per story for concurrent extraction, then aggregate results before HTML generation.

Generate a single self-contained HTML file with all CSS and JavaScript inlined. No external dependencies -- everything works offline.

### HTML structure

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Story Bank Concept Map</title>
  <style>/* all styles inlined */</style>
</head>
<body>
  <div id="gap-bar"><!-- coverage gap summary bar --></div>
  <div id="toolbar"><!-- view toggle and controls --></div>
  <div id="theme-view"><!-- theme-edge view canvas --></div>
  <div id="cluster-view" style="display:none"><!-- question-cluster view --></div>
  <div id="story-panel" style="display:none"><!-- expandable STAR+ detail panel --></div>
  <script>/* all logic inlined */</script>
</body>
</html>
```

### Visual style (dark theme)

Use a dark theme consistent with sd-playground and sd-explainer for visual coherence across interview prep playgrounds.

```css
:root {
  --bg-primary: #0f172a;       /* main background */
  --bg-secondary: #1e293b;     /* panels, toolbar */
  --bg-tertiary: #334155;      /* cards, elevated surfaces */
  --text-primary: #f1f5f9;     /* main text */
  --text-secondary: #94a3b8;   /* muted text */
  --text-muted: #64748b;       /* subtle text */
  --border: #475569;           /* borders */
  --accent: #3b82f6;           /* interactive highlights */
}

body {
  margin: 0;
  background: var(--bg-primary);
  color: var(--text-primary);
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
  overflow: hidden;
}
```

### Coverage gap summary bar (fixed top)

A fixed bar at the very top of the viewport showing aggregate coverage statistics.

**Layout:** Single horizontal bar, full width, 48px height.

**Content sections (left to right):**

```
[Total: N stories] | [Themes: X/12 covered] | [Questions: Y/7 covered] | [Staff+: Z/3 signals] | [GAPS: list of missing items]
```

**Color coding for gap indicators:**
- Full coverage (all items covered): Green text (`#22c55e`)
- Partial coverage (some missing): Yellow text (`#eab308`)
- Critical gap (< 50% coverage): Red text (`#ef4444`)

**Gap list format:** Explicitly name what is missing.
- "Missing themes: mentoring-and-growth, failure-and-recovery"
- "Missing questions: mentoring, failure"
- "Thin coverage (1 story only): conflict, strategic"
- "Underrepresented signals: mentoring"

If there are no gaps, display: "Full coverage -- no gaps detected" in green.

### Toolbar

Fixed bar below the gap summary bar, 48px height.

**Controls:**

1. **View toggle:** Two buttons -- "Theme View" and "Question View" -- styled as a toggle group. Active view button has `--accent` background. Clicking switches between views instantly (no page reload).

2. **"Export PNG"** button: Best-effort canvas rendering of the current view. Triggers download as `bq-storymap-export.png`. For complex maps, suggest screenshot as fallback.

3. **"Reset Layout"** button: Returns all story nodes to their auto-generated initial positions.

**Toolbar styling:**
```css
#toolbar {
  position: fixed;
  top: 48px;  /* below gap bar */
  left: 0;
  right: 0;
  height: 48px;
  background: var(--bg-secondary);
  display: flex;
  align-items: center;
  padding: 0 16px;
  gap: 8px;
  z-index: 2000;
  border-bottom: 1px solid var(--border);
}
```

### View 1: Theme-edge view (default)

This view shows stories as nodes connected by shared competency themes. Stories that share themes are visually linked.

**Canvas:** Full viewport below toolbar, with subtle dot-grid background (CSS radial-gradient, dot color `#1e293b` on `#0f172a`). Supports pan (drag empty area) and zoom (scroll wheel, clamped 0.3-3.0).

**Story nodes (rounded rectangles):**

```css
.story-node {
  position: absolute;
  min-width: 180px;
  max-width: 260px;
  padding: 16px 20px;
  border-radius: 12px;
  border: 3px solid;  /* color based on strength */
  background: var(--bg-tertiary);
  cursor: pointer;
  user-select: none;
  transition: box-shadow 0.15s, transform 0.1s;
}
.story-node:hover {
  box-shadow: 0 0 20px rgba(59, 130, 246, 0.3);
  transform: scale(1.02);
}
```

**Node content layout:**
```
[Strength indicator dot] Story Title (bold, 15px, --text-primary)
[Theme tag 1] [Theme tag 2] (small colored pills, 11px)
[Staff+ signal icons] (inline indicators)
```

**Strength indicator border colors:**
- Strong: `#22c55e` (green) with subtle green glow
- Moderate: `#eab308` (yellow) with subtle yellow glow
- Needs-work: `#ef4444` (red) with subtle red glow

**Theme nodes (connector nodes):**
- Smaller circular nodes (50px diameter) for each competency theme that connects 2+ stories
- Positioned between the stories they connect
- Background color from the theme color palette
- Label: abbreviated theme name (e.g., "TL" for technical-leadership)

**Theme color palette (12 deterministic colors):**

| Theme | Color | Abbreviation |
|-------|-------|--------------|
| `technical-leadership` | `#3b82f6` (blue) | TL |
| `cross-team-influence` | `#8b5cf6` (violet) | CTI |
| `conflict-resolution` | `#ef4444` (red) | CR |
| `mentoring-and-growth` | `#22c55e` (green) | MG |
| `decision-making-under-uncertainty` | `#f59e0b` (amber) | DMU |
| `operational-excellence` | `#06b6d4` (cyan) | OE |
| `strategic-thinking` | `#ec4899` (pink) | ST |
| `delivering-under-pressure` | `#f97316` (orange) | DP |
| `stakeholder-management` | `#14b8a6` (teal) | SM |
| `process-improvement` | `#a855f7` (purple) | PI |
| `innovation` | `#eab308` (yellow) | IN |
| `failure-and-recovery` | `#64748b` (slate) | FR |

**Edge rendering:**
- SVG lines from story node to theme connector node
- Stroke color matches theme color, stroke-width 2, opacity 0.6
- Subtle animated dash pattern on hover for emphasis

**Isolated story detection:**
- Stories connected to only 1 theme (or 0 shared themes with other stories) get a pulsing border animation to flag them as potential gaps
- Tooltip on isolated stories: "This story is isolated -- it shares no competency themes with other stories in your bank"

**Layout algorithm (theme-edge view):**
- Place stories in a force-directed-inspired grid: stories sharing more themes are positioned closer together
- Simple approach: sort stories by theme overlap count, place high-overlap stories centrally, low-overlap stories at periphery
- Starting position: center of canvas, spiral outward
- Horizontal spacing: 280px between nodes
- Vertical spacing: 200px between rows
- Theme connector nodes: positioned at the centroid of the stories they connect

### View 2: Question-cluster view

This view groups stories under the behavioral question categories they can answer, making question type coverage gaps immediately visible.

**Layout: Column-based**

7 columns across the viewport, one per question category:

| Column | Header |
|--------|--------|
| 1 | Leadership |
| 2 | Conflict |
| 3 | Mentoring |
| 4 | Failure |
| 5 | Cross-team |
| 6 | Strategic |
| 7 | Operational |

**Column styling:**
```css
.question-column {
  flex: 1;
  min-width: 140px;
  padding: 12px;
  border-right: 1px solid var(--border);
  min-height: calc(100vh - 96px);  /* below both bars */
}
.question-column-header {
  text-align: center;
  font-weight: 700;
  font-size: 14px;
  padding: 12px 0;
  border-bottom: 2px solid var(--border);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}
```

**Column coverage indicators:**
- **Empty column (0 stories):** Header background `#7f1d1d` (dark red), header text `#fca5a5`, pulsing border. Label appended: "GAP"
- **Thin coverage (1 story):** Header background `#78350f` (dark amber), header text `#fde68a`. Label appended: "THIN"
- **Adequate coverage (2+ stories):** Header background `var(--bg-secondary)`, normal text

**Story cards within columns:**
- Compact card format (smaller than theme-view nodes)
- Show: title, strength dot, theme tags as tiny pills
- A story can appear in multiple columns (same story, same visual identity)
- Stories appearing in multiple columns are connected by a subtle dashed line across columns (SVG overlay) to show it is the same story
- Same-story connections use `--text-muted` color, stroke-dasharray "4 4", opacity 0.4

**Click behavior:** Same as theme view -- clicking a story card opens the STAR+ detail panel.

### Shared interaction: Click to expand (STAR+ detail panel)

Clicking any story node (in either view) opens an overlay panel showing the full STAR+ breakdown.

**Panel position:** Slides in from the right side, 420px wide, full height below the bars.

**Panel content layout:**

```
[X close button, top-right]

Story Title (24px, bold)
Strength: [rating with color] | Themes: [colored pills]

--- divider ---

STAR+ Breakdown

Situation
[Full situation text]

Task
[Full task text]

Action
[Full action text, key decisions highlighted in --accent color]

Result
[Full result text with metrics highlighted]

--- divider ---

Staff+ Depth

Scope of Impact
[Summary]

Quantified Metrics
[Numbers, highlighted in green if present, red placeholder if missing]

Systemic Fix
[Long-term changes]

Org-level Impact
[Culture/process/growth changes]

--- divider ---

Staff+ Signals

[check or x] Influence: [summary or "Not addressed"]
[check or x] Alignment: [summary or "Not addressed"]
[check or x] Mentoring: [summary or "Not addressed"]

--- divider ---

Applicable Questions
[Colored pills for each question type this story answers]
```

**Panel styling:**
```css
#story-panel {
  position: fixed;
  top: 96px;  /* below both bars */
  right: 0;
  width: 420px;
  height: calc(100vh - 96px);
  background: var(--bg-secondary);
  border-left: 1px solid var(--border);
  overflow-y: auto;
  padding: 24px;
  z-index: 3000;
  transition: transform 0.2s ease;
}
#story-panel.hidden {
  transform: translateX(420px);
}
```

**Signal indicators:**
- Present signal: `#22c55e` checkmark icon + summary text
- Missing signal: `#ef4444` X icon + "Not addressed" in muted text

### Shared interaction: Hover tooltip

Hovering over a story node (in either view) shows a brief tooltip:

```
Story Title
Themes: technical-leadership, cross-team-influence
Strength: Strong
```

Tooltip follows cursor, appears after 300ms delay, light background (`var(--bg-tertiary)`) with border.

### Shared interaction: Pan and zoom (theme-edge view only)

- **Pan:** Mouse drag on empty canvas area to pan the viewport. Track `translateX`, `translateY` on the canvas container.
- **Zoom:** Mouse scroll to zoom in/out. Apply CSS `scale` transform, clamped between 0.3 and 3.0.
- Question-cluster view is a scrollable column layout (no pan/zoom needed).

### Shared interaction: Export PNG

- Best-effort canvas rendering of the current view
- Trigger download as `bq-storymap-export.png`
- For complex maps with many stories, suggest screenshot as fallback: "For best quality, use your system screenshot tool (Cmd+Shift+4 on macOS)"

### Coverage gap detection

Compute and display in the gap summary bar:

**1. Missing competency themes:**
Compare themes present across all stories against the full 12-theme taxonomy. Any theme with zero stories tagged is a missing theme.

**2. Missing question types:**
Compare question types covered across all stories against the full 7-type list. Any category with zero applicable stories is a missing question type.

**3. Thin coverage areas:**
Themes or question types with only 1 story. These are not gaps but are fragile -- losing that one story leaves a gap.

**4. Staff+ signal coverage:**
Count how many stories demonstrate each of the 3 Staff+ signals (influence, alignment, mentoring). Signals demonstrated in fewer than 30% of stories are flagged as underrepresented.

**5. Overall coverage score:**
```
Coverage = (themes covered / 12 + question types covered / 7 + signals represented / 3) / 3
```
Display as percentage in the gap bar. Below 60% shows red, 60-80% yellow, above 80% green.

### View toggle behavior

Both views are pre-rendered in the DOM. The toggle switches visibility via JavaScript:

```javascript
function switchView(view) {
  document.getElementById('theme-view').style.display = view === 'theme' ? 'block' : 'none';
  document.getElementById('cluster-view').style.display = view === 'cluster' ? 'flex' : 'none';
  // Update active button styling
  document.querySelectorAll('.view-btn').forEach(btn => btn.classList.remove('active'));
  document.querySelector(`[data-view="${view}"]`).classList.add('active');
}
```

No page reload. Instant toggle.

### Keyboard shortcuts

- `Escape` -- Close story detail panel
- `1` -- Switch to theme-edge view
- `2` -- Switch to question-cluster view
- `+` / `-` -- Zoom in / out (theme-edge view only)
- `0` -- Reset zoom to fit all stories

### State management (JavaScript)

```javascript
const state = {
  stories: [
    {
      id: 'story-1',
      title: 'Caching Migration',
      star: {
        situation: '...',
        task: '...',
        action: '...',
        result: '...',
        scope: '...',
        metrics: '...',
        prevention: '...',
        orgImpact: '...'
      },
      themes: ['technical-leadership', 'cross-team-influence'],
      questionTypes: ['leadership', 'cross-team', 'strategic'],
      staffSignals: { influence: '...', alignment: '...', mentoring: null },
      strength: 'strong',  // 'strong' | 'moderate' | 'needs-work'
      x: 0, y: 0  // position in theme view
    }
  ],
  coverage: {
    themesPresent: [],
    themesMissing: [],
    questionsPresent: [],
    questionsMissing: [],
    thinThemes: [],
    thinQuestions: [],
    signalCounts: { influence: 0, alignment: 0, mentoring: 0 },
    underrepresentedSignals: [],
    overallScore: 0
  },
  activeView: 'theme',     // 'theme' | 'cluster'
  selectedStory: null,      // story id for expanded panel
  viewport: { x: 0, y: 0, zoom: 1 },
  initialPositions: {}      // for reset layout
};
```

### Pre-population

All stories extracted in Step 1 are inserted into the initial state. The user opens the concept map with their story bank already visualized -- they explore, not build.

Store initial positions in `state.initialPositions` for the "Reset Layout" feature.

## Step 3: Write file and open in browser

1. **Create the output directory** (if it does not exist):
   ```bash
   mkdir -p <data dir>/interview-prep/playgrounds/
   ```

2. **Generate the filename:**
   - Date: current date in `YYYY-MM-DD` format
   - Full filename: `bq-storymap-{YYYY-MM-DD}.html`
   - If the file already exists for today, append a sequence number: `bq-storymap-{YYYY-MM-DD}-002.html`, `bq-storymap-{YYYY-MM-DD}-003.html`, etc.

3. **Write the HTML file** using the Write tool to:
   ```
   <data dir>/interview-prep/playgrounds/bq-storymap-{YYYY-MM-DD}.html
   ```

4. **Open in default browser:**
   ```bash
   open <data dir>/interview-prep/playgrounds/bq-storymap-{YYYY-MM-DD}.html
   ```

5. **Print confirmation:**
   ```
   Story bank concept map generated: bq-storymap-{YYYY-MM-DD}.html ({N} stories)

   Open in browser to explore your story bank. You can:
   - Toggle between Theme View and Question View
   - Click any story to see the full STAR+ breakdown
   - Hover for quick summary (title, themes, strength)
   - Pan and zoom in Theme View (drag to pan, scroll to zoom)
   - Export as PNG when you want to preserve the visualization
   ```

6. **Print the coverage gap summary inline** in the conversation (not just in the HTML):

   ```
   Coverage Summary:
   - Stories: {N} total
   - Themes: {X}/12 covered ({list missing if any})
   - Questions: {Y}/7 types covered ({list missing if any})
   - Staff+ signals: {Z}/3 represented ({list underrepresented if any})
   - Thin coverage: {list of themes/questions with only 1 story}
   - Overall coverage: {percent}%

   {If gaps exist:}
   Priority gaps to fill:
   - {Most critical missing theme or question type}
   - {Second most critical}
   ```

## Constraints

- The HTML file must be completely self-contained. No CDN links, no external CSS, no external JS. Must work offline when opened as a local file.
- Do not persist story data to disk beyond the HTML file itself. Stories live in conversation context only; interview-tracker session files hold scores and takeaways, not story text.
- Minimum 2 stories required. Maximum practical limit is ~20 stories (layout degrades beyond that -- warn the user but proceed).
- Coverage gap detection uses the full competency theme taxonomy from bq-mock (all 12 themes), not just themes present in the current stories.
- The playground is ephemeral -- same philosophy as sd-playground. Export PNG or screenshot to preserve.
- View toggle must be instant (no page reload). Both views are pre-rendered and shown/hidden via CSS/JS.
- Use the dark theme (--bg-primary: #0f172a) matching sd-playground and sd-explainer for visual consistency across interview prep playgrounds.
- Keep the generated HTML under approximately 2500 lines. Focus on usability over visual polish.
- Use text-based icons (Unicode or inline SVG) for all indicators. No external icon libraries.
- Story node layout must not overlap on first render. Use the spacing constants defined in the layout algorithm.
- Coverage gap messaging is direct: "Missing: mentoring stories" not "Consider adding stories about mentoring."

## Tone rules

- Factual and visual. The playground speaks through its layout, not through text.
- Coverage gap messaging is direct and honest:
  - "Missing: mentoring stories" not "Consider adding stories about mentoring"
  - "GAP" and "THIN" labels, not "Opportunity for improvement"
  - "0 stories" not "Room to grow"
- Strength ratings are honest: A story with vague metrics and no org impact is "needs-work", not "moderate."
- The only conversational text output is the confirmation message and coverage summary in Step 3. The HTML playground itself communicates visually.
