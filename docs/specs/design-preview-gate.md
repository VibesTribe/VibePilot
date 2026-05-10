# Pre-Execution Design Preview Specification

## Purpose

Before any UI task writes code, the system generates a visual mockup and waits for human approval. This prevents wasting model tokens and code review cycles on the wrong design direction.

The human sees mockups in the dashboard, picks a direction or requests changes, and only THEN does code generation begin. No more 8 failed attempts at button spacing.

Open CoDesign runs embedded in the dashboard experience. The human never needs to open a separate tab or app. All design outputs persist to GitHub as source of truth.

## Source of Truth

- Design mockups: `vibepilot/designs/{task-id}/` (HTML exports from Open CoDesign, committed to GitHub)
- Design decisions: PostgreSQL `design_reviews` table
- Approved designs: referenced in task's prompt packet for the code generator
- If X220 dies: all designs on GitHub, any machine clones and continues

## Architecture

### Components (all fully wired, no stubs)

1. **Design Trigger** (in existing pipeline, after plan approval, before task dispatch)
   - When planner creates a task with `category: "ui"` or tags containing "dashboard", "frontend", "visual", "css", "layout"
   - Task enters new status: `design_review` (between `planned` and `dispatched`)
   - Pipeline event: `design_review_triggered`

2. **Open CoDesign Headless Runner** (`governor/internal/designpreview/runner.go`)
   - Open CoDesign is an AppImage on the X220 (`~/open-codesign.AppImage`)
   - BUT: for pipeline automation, we cannot use the GUI AppImage. Instead we use the same approach as courier agents: call the Gemini API directly with a design-generation prompt, render output as HTML
   - Uses `gemini-api-visual` connector (same as Visual QA)
   - Prompt takes the task description + existing dashboard screenshot (from Visual QA baselines) and generates an HTML mockup
   - Output: HTML file saved to `designs/{task-id}/mockup-v{N}.html`
   - This is NOT a browser-use task. It is a direct API call that produces HTML.

3. **Design Review API** (governor endpoints)
   - `GET /api/design-reviews?task_id=X` -- returns mockups for a task
   - `POST /api/design-reviews/{id}/approve` -- human approves, task moves to dispatched
   - `POST /api/design-reviews/{id}/request-changes` -- human describes what to change, generates new mockup version
   - `POST /api/design-reviews/{id}/skip` -- human skips design review, task goes straight to dispatch

4. **Dashboard Design Review UI** (vibeflow)
   - When a task is in `design_review` status, it shows a preview panel
   - Panel renders the HTML mockup in a sandboxed iframe (same as Open CoDesign's approach)
   - Three buttons: "Approve", "Request Changes" (with text input), "Skip"
   - Approved mockup reference is attached to the task's prompt packet
   - All mockup versions visible (v1, v2, v3...) so human can go back to an earlier version

5. **Design-to-Code Integration**
   - When task is dispatched after design approval, the prompt packet includes:
     - `approved_design_path`: path to the approved HTML mockup on GitHub
     - `design_screenshot`: screenshot of the approved mockup (for vision models)
   - The code generator sees what it's supposed to build, not just a text description
   - This is the key value: the model matches the mockup, not the other way around

6. **Design Persistence** (`governor/internal/designpreview/persist.go`)
   - All mockup HTML files committed to `vibepilot/designs/` on GitHub
   - Each mockup gets a version number
   - Approved version is tagged in manifest
   - `designs/manifest.json` tracks all designs with status (pending, approved, rejected, superseded)

### Config Addition to `config/visualqa.json` (or separate `config/design-preview.json`)

```json
{
  "enabled": true,
  "trigger_categories": ["ui", "dashboard", "frontend"],
  "trigger_tags": ["css", "layout", "visual", "styling", "component"],
  "connector_id": "gemini-api-visual",
  "model": "gemini-3-flash-preview",
  "design_output_dir": "designs",
  "manifest_file": "designs/manifest.json",
  "max_iterations": 3,
  "auto_skip_threshold": 0,
  "git_commit_designs": true,
  "include_baseline_screenshot": true
}
```

### Database Changes

New migration: `134_design_reviews.sql`

```sql
CREATE TABLE IF NOT EXISTS design_reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES tasks(id),
    version INTEGER NOT NULL DEFAULT 1,
    status TEXT NOT NULL DEFAULT 'pending',  -- 'pending', 'approved', 'rejected', 'superseded', 'skipped'
    mockup_html_path TEXT,                   -- path in vibepilot repo
    mockup_screenshot_path TEXT,             -- screenshot for quick preview
    design_prompt TEXT NOT NULL,             -- the prompt used to generate the design
    model_id TEXT,                           -- model that generated it
    tokens_in INTEGER,
    tokens_out INTEGER,
    human_feedback TEXT,                     -- feedback from request-changes
    reviewed_at TIMESTAMPTZ,
    reviewed_by TEXT DEFAULT 'human',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(task_id, version)
);

-- Task status must support 'design_review'
-- Add to task status check constraint if one exists
```

### Design Generation Prompt Template

```
You are a UI design generator. Generate a complete, standalone HTML mockup based on the task description.

TASK: {task_title}
DESCRIPTION: {task_instructions}
EXISTING PAGE: {baseline_screenshot_description if available}

Requirements:
- All units in px (not rem/em)
- Inline CSS (standalone HTML file)
- Use Inter font family (loaded from Google Fonts CDN)
- Dark theme: background #0a0a0a, surface #141414, text #e5e5e5
- Accent color: #3b82f6 (blue-500)
- Match the existing dashboard's visual language
- The mockup should look like a real production UI, not a wireframe

Generate ONLY the HTML. No explanation.
```

### Pipeline Flow Change

Before:
  planned -> dispatched -> executing -> reviewing -> testing -> merged

After:
  planned -> design_review -> dispatched -> executing -> reviewing -> testing -> merged
                 |
                 v
          (UI tasks only)
          Generate mockup
          Wait for human
          Approve -> dispatched
          Changes -> new version
          Skip -> dispatched

For non-UI tasks: `planned` goes directly to `dispatched` (no design_review step).

### Dashboard UX

The design review appears as an inline panel in the task detail view:

```
[Task: Fix header button spacing]
Status: Design Review

+------------------------------------------+
|                                          |
|     [HTML mockup rendered in iframe]     |
|                                          |
+------------------------------------------+

[ Approve ]  [ Request Changes ]  [ Skip ]

Previous versions: v1 (rejected - "too cramped")
```

When human clicks "Request Changes", a text input appears. The feedback is stored in `design_reviews.human_feedback` and used to generate v2.

## Open CoDesign Dashboard Embedding

The user wants to use Open CoDesign directly from the dashboard without separate tabs. Two levels:

1. **Automated pipeline (above)**: The system generates mockups automatically and shows them in the dashboard. This covers 80% of cases.

2. **Interactive CoDesign (future enhancement)**: Embed Open CoDesign's web interface in an iframe within the dashboard. This requires Open CoDesign to run its local server mode, which is a v0.2.0+ feature. The dashboard would open a panel showing `localhost:{port}` in an iframe. This is NOT in scope for the initial build but the architecture supports it.

For now: the automated pipeline generates mockups, human reviews in dashboard, no separate tabs needed.

## Build Order

1. **Migration 134** -- design_reviews table
2. **Config file** -- design-preview.json with trigger categories/tags
3. **Design generator** -- Gemini API call producing HTML mockup
4. **Persistence layer** -- save to vibepilot/designs/, git commit
5. **Pipeline handler** -- intercept UI tasks at planned stage, move to design_review
6. **Governor API endpoints** -- approve/reject/skip design reviews
7. **Dashboard UI** -- design review panel with iframe rendering and buttons
8. **Prompt packet integration** -- attach approved design to dispatched task
9. **End-to-end test** -- submit UI PRD, verify mockup generated, human approves, task dispatched with design reference

## What This Is NOT

- NOT replacing Open CoDesign. CoDesign remains the interactive design tool. This is the automated pipeline version.
- NOT a design system generator. It generates one-off mockups for specific tasks.
- NOT required for all tasks. Only UI/frontend/dashboard tasks trigger it.
- NOT a substitute for the human's visual judgment. The human always approves or skips.

## X220 Resilience

- Design files: on GitHub in vibepilot/designs/
- Review state: in PostgreSQL (backed up hourly)
- Config: in vibepilot repo
- If X220 dies: clone repos, restore PG, governor starts, designs are all there
- No local-only state

## Success Criteria

1. UI task automatically pauses at design_review after planning
2. HTML mockup generated and committed to GitHub within 60 seconds
3. Dashboard shows mockup in iframe with approve/reject/skip buttons
4. Human approves, task dispatches with design reference in prompt packet
5. Human requests changes, new version generated incorporating feedback
6. Human skips, task dispatches without design (same as current flow)
7. All design files on GitHub, zero local-only state
8. Non-UI tasks skip design_review entirely (no slowdown)
