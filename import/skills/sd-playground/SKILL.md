---
name: sd-playground
description: Use when the user asks for an architecture diagram, system design diagram, or design playground. Generates an interactive architecture diagram playground from a system design, as a self-contained HTML file with draggable components, connections, and annotations in a whiteboard aesthetic. For a designed overview page or visual the owner picks by eye, use visual-design instead.
---

## Preconditions

- This skill needs a system design to visualize. It can work from:
  1. A just-completed sd-mock session (preferred -- components are already in conversation context)
  2. A user-described system design (user explains their architecture)
- If no design context is available, ask the user to describe their system or run an sd-mock first.

## Step 1: Extract architecture components

Analyze the system design (from mock session context or user description) and extract:

### Components

For each component, capture:
- **Name**: Descriptive name (e.g., "User Service", "Redis Cache", "Kafka Cluster")
- **Type**: One of: `service`, `database`, `cache`, `queue`, `lb` (load balancer), `cdn`, `worker`, `gateway`, `client`
- **Description**: 1-line summary of what this component does in the system

### Connections

For each connection, capture:
- **Source**: Component name
- **Target**: Component name
- **Label**: Protocol or description (e.g., "REST API", "gRPC", "pub-sub", "replication", "WebSocket")

### Layout positioning

Assign initial (x, y) coordinates using a left-to-right, top-to-bottom flow:

| Zone | X Range | Components |
|------|---------|------------|
| Left (clients) | 50-200 | Clients, CDN, external users |
| Center-left (ingress) | 300-450 | Load balancers, API gateways |
| Center (services) | 550-750 | Application services |
| Right (storage) | 850-1050 | Databases, caches, message queues |
| Bottom (async) | 400-700 | Workers, async processors, batch jobs |

**Vertical spacing:** Place components 140px apart vertically within each zone. Start at y=80.

**Overlap prevention:** Track occupied (x, y) positions per zone. If a position is taken, shift the new component down by 140px.

### Example extraction

Given a URL shortener design:

```
Components:
  - { name: "Client", type: "client", description: "Web/mobile app making requests" }
  - { name: "Load Balancer", type: "lb", description: "Distributes traffic across service instances" }
  - { name: "URL Service", type: "service", description: "Handles URL creation and redirection" }
  - { name: "Analytics Service", type: "service", description: "Processes click events and generates reports" }
  - { name: "PostgreSQL", type: "database", description: "Stores URL mappings and metadata" }
  - { name: "Redis Cache", type: "cache", description: "Caches hot URL lookups for fast redirection" }
  - { name: "Kafka", type: "queue", description: "Buffers click events for async processing" }
  - { name: "Analytics Worker", type: "worker", description: "Consumes click events and updates analytics" }

Connections:
  - { source: "Client", target: "Load Balancer", label: "HTTPS" }
  - { source: "Load Balancer", target: "URL Service", label: "Round-robin" }
  - { source: "URL Service", target: "PostgreSQL", label: "Read/Write" }
  - { source: "URL Service", target: "Redis Cache", label: "Cache-aside" }
  - { source: "URL Service", target: "Kafka", label: "Click events" }
  - { source: "Kafka", target: "Analytics Worker", label: "Consumer group" }
  - { source: "Analytics Worker", target: "PostgreSQL", label: "Write analytics" }
```

## Step 2: Generate interactive HTML playground

**Parallelism:** The HTML generation can be decomposed into independent sub-agents:
- Sub-agent A: Generate CSS (theme, layout, animations)
- Sub-agent B: Generate JavaScript (state management, event handlers, interactivity)
- Sub-agent C: Extract and embed data (components/stories/problems/concepts as inline JSON)
Dispatch all three concurrently. Merge results into the final HTML template.

Generate a single self-contained HTML file with all CSS and JavaScript inlined. No external dependencies -- everything works offline.

### HTML structure

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Architecture Playground: {topic}</title>
  <style>/* all styles inlined */</style>
</head>
<body>
  <div id="toolbar"><!-- toolbar buttons --></div>
  <svg id="connections"><!-- arrows between components --></svg>
  <div id="canvas"><!-- draggable component boxes --></div>
  <script>/* all logic inlined */</script>
</body>
</html>
```

### Canvas features

- **Full viewport** with `overflow: hidden` and a light dot-grid background (CSS `radial-gradient` pattern, dot color `#d0d0d0` on `#fafafa` background)
- **Pan support**: Drag on empty canvas area to pan the viewport (track `translateX`, `translateY` on the canvas container)
- **Zoom support**: Mouse wheel to zoom in/out (`scale` transform, clamped between 0.3 and 3.0)
- **Responsive**: Uses percentage-based toolbar, canvas fills remaining viewport

### Component boxes (draggable)

Each component renders as a `<div>` with these visual properties:

```css
.component {
  position: absolute;
  min-width: 160px;
  max-width: 240px;
  padding: 14px 18px;
  border-radius: 12px;
  border: 2px solid;
  background: white;
  box-shadow: 2px 4px 12px rgba(0,0,0,0.08);
  cursor: grab;
  user-select: none;
  font-family: 'Segoe Print', 'Comic Sans MS', cursive;
  transition: box-shadow 0.15s;
}
.component:hover {
  box-shadow: 2px 6px 18px rgba(0,0,0,0.14);
}
.component.dragging {
  cursor: grabbing;
  box-shadow: 4px 8px 24px rgba(0,0,0,0.18);
  z-index: 1000;
}
```

**Component content layout:**
```
[emoji] [Type Badge]
Component Name (bold, 15px)
Description (12px, muted color)
```

**Type configuration:**

| Type | Emoji | Badge Color | Border Color |
|------|-------|-------------|--------------|
| `service` | `{}` (curly braces icon) | `#dbeafe` (light blue) | `#93c5fd` |
| `database` | cylinder shape or `db` icon | `#dcfce7` (light green) | `#86efac` |
| `cache` | lightning or `$` icon | `#fef3c7` (light orange) | `#fcd34d` |
| `queue` | stack or `>>` icon | `#f3e8ff` (light purple) | `#c084fc` |
| `lb` | arrows or `LB` text | `#e5e7eb` (light gray) | `#9ca3af` |
| `cdn` | globe or `CDN` text | `#e5e7eb` (light gray) | `#9ca3af` |
| `worker` | gear or `W` icon | `#fce7f3` (light pink) | `#f472b6` |
| `gateway` | shield or `GW` text | `#e5e7eb` (light gray) | `#9ca3af` |
| `client` | person or `C` icon | `#f0fdf4` (lightest green) | `#bbf7d0` |

Use text-based icons/emoji so no external icon library is needed. Suggested inline SVG or Unicode:
- service: `{ }` or gear emoji
- database: inline SVG cylinder or `DB`
- cache: lightning bolt `bolt` or `$`
- queue: `>>` arrows
- lb: load balancer icon or `LB`
- cdn: globe or `CDN`
- worker: gear or cog
- gateway: shield
- client: user silhouette

**Drag interaction:**
- `mousedown` on component -> start drag, record offset from component origin
- `mousemove` -> update component `left`/`top`, accounting for pan/zoom transforms
- `mouseup` -> end drag, update stored position
- Touch events: mirror mouse events with `touchstart`, `touchmove`, `touchend`
- After each drag move, recalculate connection arrow positions

**Double-click to edit:**
- Double-click a component -> show inline input fields for name and description
- Press Enter or click outside to confirm edits
- Press Escape to cancel

**Right-click context menu:**
- "Delete component" -- removes component and all connected arrows
- "Change type" -- shows submenu with all types, updates badge/colors
- "Add annotation" -- creates a sticky note attached to this component

### Connections (SVG arrows)

Render connections as SVG paths in an `<svg>` overlay that covers the full canvas.

**Arrow rendering:**
- Use quadratic Bezier curves for slightly curved arrows (`<path>` with `Q` command)
- Calculate start/end points at the edge of each component box (center of nearest side)
- Arrowhead: SVG `<marker>` definition with a small triangle (`<polygon>`)
- Arrow color: `#64748b` (slate gray), stroke-width: 2
- Arrow label: `<text>` element positioned at the midpoint of the curve, with a small white background rectangle behind it for readability

**Auto-routing on drag:**
- Whenever a component moves, recalculate all connected arrows
- Determine which side of the source/target box is closest and use that edge's center as the anchor point

**Click to edit label:**
- Click on an arrow label -> show an inline input to edit the label text
- Press Enter or click outside to confirm

### Annotations (sticky notes)

- Yellow background (`#fef9c3`), border (`#fde047`), slight rotation (-2 to 2 degrees random)
- Positioned near the component or connection they annotate (offset 20px right and 20px down)
- Editable: click to edit text
- Dismissable: small `x` button in top-right corner
- Font: same cursive family, 12px, dark text

### Toolbar

Fixed bar at the top of the viewport, above the canvas.

**Toolbar buttons:**

1. **"+ Component"** -- Opens a dropdown with type options (service, database, cache, queue, lb, cdn, worker, gateway, client). Clicking a type creates a new component of that type at the center of the current viewport, with placeholder name "New {Type}" and empty description.

2. **"+ Connection"** -- Enters connection mode:
   - Cursor changes to crosshair
   - Click first component (highlights it as source)
   - Click second component (creates connection with label "connects to")
   - Press Escape to cancel connection mode
   - Status text shows "Click source component..." then "Click target component..."

3. **"+ Annotation"** -- Enters annotation mode:
   - Click anywhere on canvas to place a new annotation with placeholder text "Add note..."
   - Press Escape to cancel

4. **"Export PNG"** -- Captures the canvas area as a PNG:
   - Use a `<canvas>` element to render the current state
   - Draw the grid background, all component boxes, all arrows, all annotations
   - Trigger download as `sd-diagram-{topic-slug}.png`
   - Note: This is a best-effort feature. For complex diagrams, screenshot may be more reliable.

5. **"Reset Layout"** -- Returns all components to their auto-generated initial positions from Step 1.

**Toolbar styling:**
```css
#toolbar {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  height: 48px;
  background: #1e293b;
  display: flex;
  align-items: center;
  padding: 0 16px;
  gap: 8px;
  z-index: 2000;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
}
#toolbar button {
  background: #334155;
  color: #f1f5f9;
  border: 1px solid #475569;
  border-radius: 6px;
  padding: 6px 14px;
  cursor: pointer;
  font-size: 13px;
}
#toolbar button:hover {
  background: #475569;
}
#toolbar button.active {
  background: #3b82f6;
  border-color: #60a5fa;
}
```

### Keyboard shortcuts

- `Delete` or `Backspace` -- Delete selected component (if one is selected, indicated by blue highlight border)
- `Escape` -- Cancel current mode (connection mode, annotation mode), deselect component
- `Ctrl/Cmd + Z` -- Undo last action (maintain a simple action stack: add/delete component, add/delete connection, move component)
- `Ctrl/Cmd + Shift + Z` -- Redo

### State management (JavaScript)

Maintain state in a single object:

```javascript
const state = {
  components: [
    { id: 'comp-1', name: 'URL Service', type: 'service', description: '...', x: 550, y: 80 }
  ],
  connections: [
    { id: 'conn-1', sourceId: 'comp-1', targetId: 'comp-2', label: 'REST API' }
  ],
  annotations: [
    { id: 'ann-1', text: 'Consider cache invalidation', x: 600, y: 200, attachedTo: 'comp-3' }
  ],
  viewport: { x: 0, y: 0, zoom: 1 },
  undoStack: [],
  redoStack: [],
  mode: 'default',       // 'default' | 'connecting' | 'annotating'
  selectedId: null,
  connectingFrom: null,
  initialPositions: {}    // for reset layout
};
```

### Pre-population

All components and connections extracted in Step 1 are inserted into the initial state. The user opens the diagram with their design already visible -- they refine, not build from scratch.

Store initial positions in `state.initialPositions` for the "Reset Layout" feature.

## Step 3: Write file and open in browser

1. **Create the output directory** (if it doesn't exist):
   ```bash
   mkdir -p ~/.config/interview-prep/playgrounds/
   ```

2. **Generate the filename:**
   - `topic-slug`: topic lowercased, spaces replaced with hyphens, non-alphanumeric characters (except hyphens) removed, truncated to 40 characters
   - `date`: current date in `YYYY-MM-DD` format
   - Full filename: `sd-diagram-{topic-slug}-{date}.html`

3. **Write the HTML file** using the Write tool to:
   ```
   ~/.config/interview-prep/playgrounds/sd-diagram-{topic-slug}-{date}.html
   ```

4. **Open in default browser:**
   ```bash
   open ~/.config/interview-prep/playgrounds/sd-diagram-{topic-slug}-{date}.html
   ```

5. **Print confirmation:**
   ```
   Architecture playground generated: sd-diagram-{topic-slug}-{date}.html ({N} components, {M} connections)

   Open in browser to explore your design. You can:
   - Drag components to rearrange the layout
   - Double-click to edit names and descriptions
   - Use the toolbar to add components, connections, and annotations
   - Right-click for more options (delete, change type, annotate)
   - Export as PNG when you're happy with the layout
   ```

## Constraints

- The HTML file must be completely self-contained -- inline CSS, inline JavaScript, no external dependencies. Must work offline when opened as a local file.
- Keep the generated HTML under approximately 2000 lines. Focus on usability over visual polish (functional whiteboard aesthetic).
- Component layout algorithm must produce a readable layout on first render -- no overlapping components. Use the zone-based positioning from Step 1 with vertical spacing.
- Do not include any server-side logic -- this is a static HTML file opened locally.
- Do not auto-invoke this skill after a mock -- it is on-demand only. The sd-mock skill offers it as a next step (Step 6, option 3).
- Use inline SVG or Unicode text for component type icons. No external icon libraries.
- Use CSS `font-family: 'Segoe Print', 'Comic Sans MS', cursive` for the handwritten/sketch feel (Excalidraw-style approximation).
- Export PNG is best-effort. For complex diagrams, suggest the user take a screenshot as fallback.
- The playground is ephemeral -- it does not save state back to disk. If the user wants to preserve changes, they should export as PNG or take a screenshot.

## Tone rules

Not applicable -- this skill generates a visual artifact, not conversational feedback. The only text output is the confirmation message in Step 3.
