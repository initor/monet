---
name: code-playground
description: Generate an interactive coding playground for Java interview practice. Creates a self-contained HTML file with syntax-highlighted code editor, test case visualization, collapsible problem statement, and elapsed timer. Use when the user asks for a coding playground, wants to practice a coding problem interactively, or when running a coding mock.
---

## Preconditions

- This skill needs a coding problem to visualize. It can work from:
  1. A code-mock session in progress (preferred -- problem details are already in conversation context)
  2. A user-described problem (user provides title, description, method signature, test cases)
- If no problem context is available, ask the user to describe their problem or start a code-mock first.

## Step 1: Extract problem data

From the mock session context or user description, extract:

- **Problem title**: Short name (e.g., "Two Sum", "Merge Intervals")
- **Problem description**: Full description with constraints (2-5 sentences)
- **Method signature** (if applicable): Java method signature the user needs to implement (e.g., `public int[] twoSum(int[] nums, int target)`)
- **Class structure** (if applicable): For problems requiring a class design (e.g., LRU Cache), provide the class skeleton
- **Test cases**: Array of objects with:
  - `name`: Short description (e.g., "Basic case", "Empty input", "Single element")
  - `input`: Formatted input string
  - `expected`: Expected output string
  - `isEdge`: Boolean indicating if this is an edge case (displayed differently)
- **Hints** (optional): If the user has already received hints during the mock, include them for reference in the playground

### Example extraction

Given a Two Sum Variants problem from a mock session:

```
Problem title: "Two Sum Variants"
Problem description: "Given an array of integers and a target sum, find all unique pairs of numbers that add up to the target. Return the pairs sorted in ascending order. Array length 0 to 10^5, values -10^9 to 10^9, duplicates possible."
Method signature: "List<int[]> findPairs(int[] nums, int target)"
Test cases:
  - { name: "Basic case", input: "nums = [1, 3, 2, 2, 3, 4], target = 5", expected: "[[1,4], [2,3]]", isEdge: false }
  - { name: "Empty array", input: "nums = [], target = 5", expected: "[]", isEdge: true }
  - { name: "Single element", input: "nums = [5], target = 5", expected: "[]", isEdge: true }
  - { name: "All zeros", input: "nums = [0, 0, 0], target = 0", expected: "[[0,0]]", isEdge: true }
  - { name: "Negative numbers", input: "nums = [-1, 2, -3, 4], target = 1", expected: "[[-3,4], [-1,2]]", isEdge: false }
```

## Step 2: Generate self-contained HTML

**Parallelism:** The HTML generation can be decomposed into independent sub-agents:
- Sub-agent A: Generate CSS (theme, layout, animations)
- Sub-agent B: Generate JavaScript (state management, event handlers, interactivity)
- Sub-agent C: Extract and embed data (components/stories/problems/concepts as inline JSON)
Dispatch all three concurrently. Merge results into the final HTML template.

Generate a single HTML file. The HTML must be fully self-contained and functional when opened in any modern browser. No external dependencies -- everything works offline.

### HTML structure

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Coding Playground: {problem-title}</title>
  <style>/* all styles inlined */</style>
</head>
<body>
  <div id="header"><!-- title bar with timer --></div>
  <div id="problem-panel"><!-- collapsible problem statement --></div>
  <div id="editor-panel"><!-- code editor with line numbers --></div>
  <div id="test-panel"><!-- test case visualization --></div>
  <script>/* all logic inlined */</script>
</body>
</html>
```

### Layout: Stacked design

The playground uses a stacked vertical layout. All sections span the full width and stack top to bottom.

```
+---------------------------------------------------+
| [Problem Title]                    [Timer: 00:00]  |
+---------------------------------------------------+
| > Problem Statement (collapsible)                  |
|   [Full problem description with constraints]      |
|   [Example I/O]                                    |
+---------------------------------------------------+
| Code Editor                                        |
|   [Pre-populated with method signature/class]      |
|   [Java syntax highlighting]                       |
|   [Line numbers]                                   |
|   [Tab key inserts spaces, not focus change]       |
+---------------------------------------------------+
| Test Cases                                         |
|   [Test 1: name | input | expected | status]      |
|   [Test 2: name | input | expected | status]      |
|   [Edge cases visually distinct]                   |
+---------------------------------------------------+
```

### Header section

**Layout:** Horizontal bar at the top, full width.

**Left side:** Problem title in bold, large text (22px).

**Right side:** Elapsed timer display.

**Timer behavior:**
- Starts at `00:00` on page load
- Increments every second
- Display format: `MM:SS`
- No hard cutoff -- purely for time awareness
- Include a Start/Pause toggle button next to the timer display
- Timer button text: "Pause" when running, "Resume" when paused
- Timer text color: `#94a3b8` (muted) normally, `#f59e0b` (amber warning) after 30 minutes, `#ef4444` (red) after 45 minutes

**Header styling:**
```css
#header {
  position: sticky;
  top: 0;
  z-index: 100;
  background: #1e293b;
  padding: 12px 24px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-bottom: 1px solid #334155;
}
#header h1 {
  margin: 0;
  font-size: 22px;
  font-weight: 700;
  color: #f1f5f9;
}
.timer-display {
  font-family: 'SF Mono', 'Fira Code', 'Consolas', monospace;
  font-size: 20px;
  color: #94a3b8;
  letter-spacing: 1px;
}
.timer-btn {
  background: #334155;
  color: #f1f5f9;
  border: 1px solid #475569;
  border-radius: 6px;
  padding: 4px 12px;
  cursor: pointer;
  font-size: 12px;
  margin-left: 8px;
}
.timer-btn:hover {
  background: #475569;
}
```

### Problem statement panel

**Behavior:**
- Collapsible/expandable with click on header
- **Expanded on first load** (user needs to read the problem first)
- Click the panel header to toggle collapse
- Smooth slide animation using `max-height` transition

**Content includes:**
- Full problem description
- Constraints (formatted as bullet list)
- Example input/output from the first test case (formatted in a code block)

**Panel header:**
- Arrow indicator: `>` when collapsed, `v` when expanded
- Header text: "Problem Statement"
- Header background slightly lighter than body for distinction

**Styling:**
```css
#problem-panel {
  border-bottom: 1px solid #334155;
}
.problem-header {
  background: #1e293b;
  padding: 10px 24px;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 8px;
  user-select: none;
  font-weight: 600;
  font-size: 14px;
  color: #94a3b8;
}
.problem-header:hover {
  background: #253347;
}
.problem-header .arrow {
  transition: transform 0.2s;
  font-size: 12px;
}
.problem-header .arrow.expanded {
  transform: rotate(90deg);
}
.problem-body {
  background: #0f172a;
  padding: 0 24px;
  max-height: 0;
  overflow: hidden;
  transition: max-height 0.3s ease, padding 0.3s ease;
}
.problem-body.expanded {
  max-height: 600px;
  padding: 16px 24px;
}
.problem-body p {
  color: #cbd5e1;
  font-size: 14px;
  line-height: 1.6;
  margin: 0 0 12px 0;
}
.problem-body .constraints {
  color: #94a3b8;
  font-size: 13px;
}
.problem-body .example-block {
  background: #1e293b;
  border-radius: 6px;
  padding: 12px 16px;
  font-family: 'SF Mono', 'Fira Code', 'Consolas', monospace;
  font-size: 13px;
  color: #e2e8f0;
  margin: 8px 0;
  white-space: pre-wrap;
}
```

### Code editor area

The code editor uses a `<textarea>` element with a synchronized line number gutter. This approach is simpler and more reliable than `contenteditable` while still providing a good coding experience.

**Pre-population:**
- For method-signature problems, pre-populate with a class wrapper:
  ```java
  class Solution {
      public int[] twoSum(int[] nums, int target) {
          // Your solution here
      }
  }
  ```
- For class-design problems (e.g., MinStack, MedianFinder), pre-populate with the class skeleton:
  ```java
  class MinStack {
      public MinStack() {
          // Initialize
      }

      public void push(int val) {
          // TODO
      }

      public void pop() {
          // TODO
      }

      public int top() {
          // TODO
      }

      public int getMin() {
          // TODO
      }
  }
  ```
- For description-only problems (user defines the interface), pre-populate with a minimal class and a comment prompting the user:
  ```java
  class Solution {
      // Define your method signature and implement the solution
      // Problem: {problem title}
  }
  ```
- **Code editor must never be empty.** Always pre-populate with at least the class structure.

**Syntax highlighting approach:**
Use a lightweight overlay-based approach. The textarea is transparent (visible cursor and typing), and a `<pre><code>` element behind it displays the syntax-highlighted version. Highlighting is applied on each input event using regex-based token replacement.

**Highlighting rules (Java keywords):**
```javascript
const HIGHLIGHT_RULES = [
  // Keywords (blue)
  { pattern: /\b(abstract|assert|boolean|break|byte|case|catch|char|class|const|continue|default|do|double|else|enum|extends|final|finally|float|for|goto|if|implements|import|instanceof|int|interface|long|native|new|package|private|protected|public|return|short|static|strictfp|super|switch|synchronized|this|throw|throws|transient|try|void|volatile|while)\b/g, className: 'keyword' },
  // Types (cyan)
  { pattern: /\b(String|Integer|Long|Double|Float|Boolean|Character|Byte|Short|List|ArrayList|LinkedList|Map|HashMap|TreeMap|Set|HashSet|TreeSet|Queue|PriorityQueue|Deque|ArrayDeque|Stack|ListNode|TreeNode)\b/g, className: 'type' },
  // Strings (green)
  { pattern: /"(?:[^"\\]|\\.)*"/g, className: 'string' },
  // Single-line comments (gray)
  { pattern: /\/\/.*$/gm, className: 'comment' },
  // Multi-line comments (gray)
  { pattern: /\/\*[\s\S]*?\*\//g, className: 'comment' },
  // Numbers (orange)
  { pattern: /\b\d+\.?\d*[fFdDlL]?\b/g, className: 'number' },
  // Booleans and null (purple)
  { pattern: /\b(true|false|null)\b/g, className: 'literal' }
];
```

**Highlighting colors (dark theme):**
```css
.keyword { color: #7dd3fc; }   /* sky-300 -- Java keywords */
.type    { color: #67e8f9; }   /* cyan-300 -- types */
.string  { color: #86efac; }   /* green-300 -- strings */
.comment { color: #64748b; }   /* slate-500 -- comments */
.number  { color: #fdba74; }   /* orange-300 -- numbers */
.literal { color: #c4b5fd; }   /* violet-300 -- booleans, null */
```

**Line numbers:**
- Displayed in a gutter on the left side of the editor
- Gutter width: 48px, right-aligned numbers
- Gutter background: `#0c1222` (slightly darker than editor)
- Gutter text color: `#475569` (muted)
- Line numbers update dynamically as the user types (synced with textarea scroll)

**Tab key behavior:**
- Pressing Tab inserts 4 spaces at the cursor position (prevents focus change)
- Implement via `keydown` event listener on the textarea
- `event.preventDefault()` when Tab is pressed
- Insert 4 spaces at `selectionStart`

**Auto-indent on newline:**
- When Enter is pressed, detect the indentation level of the current line
- Insert a newline followed by the same whitespace prefix
- If the current line ends with `{`, add an additional 4 spaces of indentation

**Editor sizing:**
- Minimum height: 400px
- The editor should grow with content but cap at approximately 60vh
- Scrollable when content exceeds the visible area
- Both the textarea and the highlight overlay scroll together

**Editor styling:**
```css
.editor-container {
  position: relative;
  background: #0f172a;
  border-bottom: 1px solid #334155;
  display: flex;
  min-height: 400px;
  max-height: 60vh;
}
.line-numbers {
  width: 48px;
  background: #0c1222;
  color: #475569;
  font-family: 'SF Mono', 'Fira Code', 'Consolas', monospace;
  font-size: 14px;
  line-height: 1.5;
  padding: 16px 8px 16px 0;
  text-align: right;
  user-select: none;
  overflow: hidden;
  white-space: pre-wrap;
}
.editor-wrapper {
  flex: 1;
  position: relative;
  overflow: auto;
}
.editor-highlight {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  padding: 16px;
  font-family: 'SF Mono', 'Fira Code', 'Consolas', monospace;
  font-size: 14px;
  line-height: 1.5;
  color: #e2e8f0;
  white-space: pre-wrap;
  word-wrap: break-word;
  pointer-events: none;
}
.editor-textarea {
  position: relative;
  width: 100%;
  min-height: 100%;
  padding: 16px;
  font-family: 'SF Mono', 'Fira Code', 'Consolas', monospace;
  font-size: 14px;
  line-height: 1.5;
  color: transparent;
  caret-color: #f1f5f9;
  background: transparent;
  border: none;
  outline: none;
  resize: none;
  white-space: pre-wrap;
  word-wrap: break-word;
  -webkit-text-fill-color: transparent;
}
```

### Test case section

Display all test cases in a card-based layout below the code editor.

**Section header:**
- Title: "Test Cases" with count (e.g., "Test Cases (5)")
- Background: `#1e293b`

**Test case cards:**
Each test case renders as a card with:
- **Name** (bold, 14px): e.g., "Basic case"
- **Input** (monospace, 13px): formatted input string
- **Expected** (monospace, 13px): expected output string
- **Status indicator**: pill badge in top-right corner
  - "Not run" (default): gray background (`#334155`), gray text (`#94a3b8`)
  - "Pass": green background (`#166534`), green text (`#86efac`)
  - "Fail": red background (`#7f1d1d`), red text (`#fca5a5`)
- **Edge case badge**: If `isEdge` is true, show an orange "Edge" badge (`#78350f` background, `#fde68a` text) next to the test name

**Status toggling:**
Since this is a static HTML playground without a Java runtime, test case status is toggled manually by the user. Each card has a small clickable status area that cycles through: Not run -> Pass -> Fail -> Not run.

This lets the user mark test case results as they mentally trace through their solution or after pasting their code back into chat for evaluation.

**Card styling:**
```css
.test-section {
  background: #0f172a;
  padding: 16px 24px 24px;
}
.test-section-header {
  background: #1e293b;
  padding: 10px 24px;
  font-weight: 600;
  font-size: 14px;
  color: #94a3b8;
  border-bottom: 1px solid #334155;
}
.test-cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 12px;
  padding-top: 16px;
}
.test-card {
  background: #1e293b;
  border: 1px solid #334155;
  border-radius: 8px;
  padding: 14px 16px;
  position: relative;
}
.test-card.edge-case {
  border-color: #b45309;
  border-width: 2px;
}
.test-card-name {
  font-weight: 600;
  font-size: 14px;
  color: #f1f5f9;
  margin-bottom: 8px;
  display: flex;
  align-items: center;
  gap: 8px;
}
.edge-badge {
  background: #78350f;
  color: #fde68a;
  font-size: 11px;
  font-weight: 600;
  padding: 1px 6px;
  border-radius: 4px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}
.test-label {
  font-size: 11px;
  color: #64748b;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  margin-top: 6px;
}
.test-value {
  font-family: 'SF Mono', 'Fira Code', 'Consolas', monospace;
  font-size: 13px;
  color: #cbd5e1;
  background: #0f172a;
  padding: 6px 10px;
  border-radius: 4px;
  margin-top: 2px;
  word-break: break-all;
}
.status-pill {
  position: absolute;
  top: 12px;
  right: 12px;
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 10px;
  cursor: pointer;
  user-select: none;
  transition: background 0.15s;
}
.status-pill.not-run {
  background: #334155;
  color: #94a3b8;
}
.status-pill.pass {
  background: #166534;
  color: #86efac;
}
.status-pill.fail {
  background: #7f1d1d;
  color: #fca5a5;
}
```

### Theme

Dark theme consistent with other interview-prep playgrounds.

```css
:root {
  --bg-primary: #0f172a;       /* Slate 900 -- main background */
  --bg-secondary: #1e293b;     /* Slate 800 -- panels, headers */
  --bg-tertiary: #334155;      /* Slate 700 -- elevated surfaces */
  --text-primary: #f1f5f9;     /* Slate 50 -- main text */
  --text-secondary: #94a3b8;   /* Slate 400 -- muted text */
  --text-muted: #64748b;       /* Slate 500 -- subtle text */
  --border: #334155;           /* Slate 700 -- borders */
  --accent: #3b82f6;           /* Blue 500 -- interactive highlights */
}

body {
  margin: 0;
  padding: 0;
  background: var(--bg-primary);
  color: var(--text-primary);
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
  overflow-x: hidden;
}

/* Scrollbar styling for dark theme */
::-webkit-scrollbar {
  width: 8px;
}
::-webkit-scrollbar-track {
  background: var(--bg-primary);
}
::-webkit-scrollbar-thumb {
  background: var(--bg-tertiary);
  border-radius: 4px;
}
```

### Keyboard shortcuts

- `Tab` -- Insert 4 spaces (captured, does not leave editor)
- `Enter` -- New line with the current line's indentation (4 more spaces after a line ending in `{`)

These are the only keys the editor handles. List no other shortcuts on the page.

### JavaScript implementation outline

The script section at the bottom of the HTML includes all interactive behavior:

```javascript
// State
let timerSeconds = 0;
let timerRunning = true;
let timerInterval = null;

// Timer
function startTimer() {
  timerInterval = setInterval(() => {
    if (timerRunning) {
      timerSeconds++;
      updateTimerDisplay();
    }
  }, 1000);
}

function updateTimerDisplay() {
  const mins = String(Math.floor(timerSeconds / 60)).padStart(2, '0');
  const secs = String(timerSeconds % 60).padStart(2, '0');
  const display = document.getElementById('timer-display');
  display.textContent = `${mins}:${secs}`;
  // Color changes for time awareness
  if (timerSeconds >= 2700) {       // 45 min
    display.style.color = '#ef4444';
  } else if (timerSeconds >= 1800) { // 30 min
    display.style.color = '#f59e0b';
  }
}

function toggleTimer() {
  timerRunning = !timerRunning;
  document.getElementById('timer-btn').textContent = timerRunning ? 'Pause' : 'Resume';
}

// Problem panel toggle
function toggleProblem() {
  const body = document.getElementById('problem-body');
  const arrow = document.getElementById('problem-arrow');
  body.classList.toggle('expanded');
  arrow.classList.toggle('expanded');
}

// Syntax highlighting
function highlightCode(code) {
  // Escape HTML entities first
  let highlighted = code
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
  // Apply highlighting rules in order (comments and strings first to prevent keyword matching inside them)
  // Use a token-based approach: mark regions, then apply classes
  // Simplified approach: apply regex replacements in precedence order
  // ... (apply HIGHLIGHT_RULES from the spec above)
  return highlighted;
}

// Line numbers sync
function updateLineNumbers() {
  const textarea = document.getElementById('code-textarea');
  const lineNums = document.getElementById('line-numbers');
  const lines = textarea.value.split('\n').length;
  lineNums.textContent = Array.from({ length: lines }, (_, i) => i + 1).join('\n');
}

// Tab key handling
function handleKeydown(event) {
  if (event.key === 'Tab') {
    event.preventDefault();
    const textarea = event.target;
    const start = textarea.selectionStart;
    const end = textarea.selectionEnd;
    textarea.value = textarea.value.substring(0, start) + '    ' + textarea.value.substring(end);
    textarea.selectionStart = textarea.selectionEnd = start + 4;
    updateEditor();
  }
  if (event.key === 'Enter') {
    event.preventDefault();
    const textarea = event.target;
    const start = textarea.selectionStart;
    const beforeCursor = textarea.value.substring(0, start);
    const afterCursor = textarea.value.substring(textarea.selectionEnd);
    // Get current line indentation
    const currentLine = beforeCursor.split('\n').pop();
    const indent = currentLine.match(/^\s*/)[0];
    // Extra indent after opening brace
    const extraIndent = currentLine.trimEnd().endsWith('{') ? '    ' : '';
    const newText = '\n' + indent + extraIndent;
    textarea.value = beforeCursor + newText + afterCursor;
    textarea.selectionStart = textarea.selectionEnd = start + newText.length;
    updateEditor();
  }
}

function updateEditor() {
  const textarea = document.getElementById('code-textarea');
  const highlight = document.getElementById('code-highlight');
  highlight.innerHTML = highlightCode(textarea.value);
  updateLineNumbers();
}

// Scroll sync between textarea and highlight overlay
function syncScroll() {
  const textarea = document.getElementById('code-textarea');
  const highlight = document.getElementById('code-highlight');
  const lineNums = document.getElementById('line-numbers');
  highlight.scrollTop = textarea.scrollTop;
  highlight.scrollLeft = textarea.scrollLeft;
  lineNums.scrollTop = textarea.scrollTop;
}

// Test case status cycling
function cycleStatus(cardElement) {
  const pill = cardElement.querySelector('.status-pill');
  const states = ['not-run', 'pass', 'fail'];
  const labels = ['Not run', 'Pass', 'Fail'];
  const current = states.indexOf(pill.dataset.status);
  const next = (current + 1) % states.length;
  pill.dataset.status = states[next];
  pill.className = 'status-pill ' + states[next];
  pill.textContent = labels[next];
}

// Initialize
document.addEventListener('DOMContentLoaded', () => {
  startTimer();
  updateEditor();
  // Problem panel starts expanded
  document.getElementById('problem-body').classList.add('expanded');
  document.getElementById('problem-arrow').classList.add('expanded');
});
```

## Step 3: Write and open playground

1. **Ensure directory exists:**
   ```bash
   mkdir -p ~/.config/interview-prep/playgrounds
   ```

2. **Generate the filename:**
   - `problem-slug`: problem title lowercased, spaces replaced with hyphens, non-alphanumeric (except hyphens) removed, truncated to 40 characters
   - `date`: current date in `YYYY-MM-DD` format
   - Full filename: `code-playground-{problem-slug}-{date}.html`

   Example: `code-playground-two-sum-variants-2026-02-23.html`

3. **Write the HTML file** using the Write tool to:
   ```
   ~/.config/interview-prep/playgrounds/code-playground-{problem-slug}-{date}.html
   ```

4. **Open in default browser:**
   ```bash
   open ~/.config/interview-prep/playgrounds/code-playground-{problem-slug}-{date}.html
   ```

5. **Print confirmation:**
   ```
   Coding playground generated: code-playground-{problem-slug}-{date}.html ({N} test cases)

   Open in browser to write your solution. You can:
   - Expand/collapse the problem statement for reference
   - Write your Java solution in the code editor
   - Track your elapsed time with the timer
   - Review test cases below the editor
   - Click test case status to mark Pass/Fail as you trace through

   When you're ready, share your solution back in chat for evaluation.
   ```

## Constraints

- HTML must be fully self-contained (no external dependencies, no CDN links). Must work offline when opened as a local file.
- Dark theme mandatory (`#0f172a` background) matching interview-prep playground conventions.
- Tab key must insert 4 spaces, not change focus. Implement via `keydown` event with `preventDefault`.
- Timer starts on page load, no hard cutoff. Timer color changes at 30min (amber) and 45min (red) for awareness.
- Problem statement is collapsible but expanded on first load. User clicks header to toggle.
- Code editor must be pre-populated with method signature or class structure. Never generate an empty editor.
- File goes to `~/.config/interview-prep/playgrounds/` directory (same as other playgrounds).
- Filename follows pattern: `code-playground-{problem-slug}-{date}.html`.

## Tone rules

- Confirmation message is brief and actionable.
- Direct the user back to chat when ready for evaluation: "When you're ready, share your solution back in chat for evaluation."
- Do not overload the UI with instructions -- keep it clean. The playground header, problem panel, editor, and test section are self-explanatory.
- The playground is a tool, not a tutorial. No instructional popups or onboarding flows.
- If hints were provided during the mock, they may be included as a subtle reference in the problem panel (collapsed subsection), but do not make hints prominent -- the user should try to solve independently.
