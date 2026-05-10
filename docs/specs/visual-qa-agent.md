# Visual QA Agent Specification

## Purpose

Automated visual regression detection for the Vibeflow dashboard. After every dashboard deploy (Vercel) or after any CSS/UI code change, the Visual QA agent captures screenshots, compares them against approved baselines stored on GitHub, and flags visual regressions.

This replaces manual CSS debugging. The agent that tried 8 times to fix button spacing without a visual validation loop is exactly the problem this solves.

## Source of Truth

GitHub is source of truth for EVERYTHING:
- Baseline screenshots: `vibepilot/baselines/screenshots/` (PNG files committed to repo)
- Baseline metadata: `vibepilot/baselines/manifest.json` (page URLs, viewport sizes, last-approved hash)
- Visual QA results: pipeline events in PostgreSQL `orchestrator_events` table
- Dashboard pass/fail: served from PostgreSQL via governor API, displayed in vibeflow dashboard

If the X220 dies, all baselines are on GitHub. Any machine can clone the repo and continue.

## Architecture

### Components (all must be fully wired, no stubs)

1. **Screenshot Capture Service** (`governor/internal/visualqa/capture.go`)
   - Uses Playwright (already installed for browser-use/courier agents)
   - Headless Chromium on the X220
   - Captures full-page screenshots at defined viewport sizes
   - Stores captures temporarily in `/tmp/vqa-captures/`
   - Trigger: webhook from Vercel deploy OR manual trigger from dashboard OR post-merge pipeline event

2. **Baseline Manager** (`governor/internal/visualqa/baseline.go`)
   - Baselines live in `vibepilot/baselines/` directory, committed to GitHub
   - `manifest.json` tracks: page URL, viewport width, baseline filename, last-approved-commit-sha
   - When a new baseline is approved: git add, git commit, git push to vibepilot repo
   - Baseline approval happens via dashboard (human clicks "Approve as new baseline")
   - First run: auto-creates baselines if none exist (no baseline = auto-approve first capture)

3. **Comparison Engine** (`governor/internal/visualqa/compare.go`)
   - Uses Gemini 3 Flash Preview (via `gemini-api-visual` connector, GEMINI_VISUAL_TESTER_KEY)
   - Sends both baseline and current capture to Gemini with structured prompt
   - Prompt asks for: pixel-level diff summary, layout shift detection, text overlap, broken alignment, color shifts, missing elements
   - Returns structured JSON: `{passed: bool, confidence: float, differences: [{type, severity, region, description}]}`
   - Does NOT use pixel-diff libraries (fragile, false positives from anti-aliasing). Uses vision model (semantic comparison).

4. **Pipeline Integration** (`governor/cmd/governor/handlers_visualqa.go`)
   - New pipeline event types: `visual_qa_triggered`, `visual_qa_passed`, `visual_qa_failed`, `visual_qa_baseline_approved`
   - Trigger sources:
     a. Vercel deploy webhook (POST to governor `/api/webhooks/vercel-deploy`)
     b. Post-merge hook after vibeflow changes land on main
     c. Manual trigger from dashboard button
   - On failure: creates pipeline event with diff details, surfaces in dashboard review queue
   - On pass: creates pipeline event, no human action needed

5. **Dashboard Integration** (vibeflow changes)
   - New "Visual QA" tab or section in dashboard
   - Shows: last run timestamp, pass/fail status, side-by-side baseline vs current if failed
   - "Approve as baseline" button on failed comparisons (human decides if the change is intentional)
   - "Re-run Visual QA" button for manual triggers
   - Results persisted in PostgreSQL, survives X220 death

6. **Vercel Deploy Webhook** (`governor/cmd/governor/webhook_vercel.go`)
   - Vercel sends deploy success webhook to governor
   - Governor triggers Visual QA capture for all registered pages
   - Registered pages list in `config/visualqa.json`

### Config File: `config/visualqa.json`

```json
{
  "enabled": true,
  "capture_pages": [
    {"url": "https://vibeflow-dashboard.vercel.app", "name": "dashboard-main", "viewports": [1280, 768, 375]},
    {"url": "https://vibeflow-dashboard.vercel.app/tasks", "name": "tasks-page", "viewports": [1280, 768]},
    {"url": "https://vibeflow-dashboard.vercel.app/roi", "name": "roi-page", "viewports": [1280]}
  ],
  "baseline_dir": "baselines/screenshots",
  "manifest_file": "baselines/manifest.json",
  "connector_id": "gemini-api-visual",
  "model": "gemini-3-flash-preview",
  "capture_timeout_seconds": 60,
  "comparison_timeout_seconds": 30,
  "auto_approve_first_baseline": true,
  "git_commit_baselines": true,
  "temp_dir": "/tmp/vqa-captures"
}
```

### Database Changes

New migration: `133_visual_qa.sql`

```sql
CREATE TABLE IF NOT EXISTS visual_qa_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    triggered_by TEXT NOT NULL,  -- 'vercel_deploy', 'manual', 'post_merge'
    trigger_detail TEXT,         -- deploy URL, commit SHA, etc
    status TEXT NOT NULL DEFAULT 'running',  -- 'running', 'passed', 'failed', 'error'
    pages_checked INTEGER DEFAULT 0,
    pages_passed INTEGER DEFAULT 0,
    pages_failed INTEGER DEFAULT 0,
    results JSONB,              -- array of {page, viewport, passed, differences}
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    duration_ms INTEGER
);

-- Pipeline events already support arbitrary event_type, just use:
-- 'visual_qa_triggered', 'visual_qa_passed', 'visual_qa_failed'
```

### Gemini Comparison Prompt

```
You are a visual QA agent comparing two screenshots of the same web page.

BASELINE: The approved reference screenshot.
CURRENT: The newly captured screenshot after a code change.

Compare them semantically. Ignore minor anti-aliasing or sub-pixel differences.

Check for:
1. Layout shifts (elements moved, resized, or overlapping)
2. Missing elements (buttons, text, images that disappeared)
3. New unexpected elements
4. Text content changes (unintended text changes)
5. Color/theme changes (unintended color shifts)
6. Alignment issues (centering, spacing, padding changes)

Return JSON:
{
  "passed": true/false,
  "confidence": 0.0-1.0,
  "summary": "one sentence overall assessment",
  "differences": [
    {
      "type": "layout_shift|missing_element|new_element|text_change|color_change|alignment",
      "severity": "critical|minor|cosmetic",
      "region": "top-left|top-center|top-right|mid-left|center|mid-right|bottom-left|bottom-center|bottom-right",
      "description": "what specifically changed"
    }
  ]
}

Only flag differences that represent actual regressions. Intentional design changes will be approved separately by the human. Focus on broken layouts, missing content, and visual bugs.
```

## Build Order (dependencies first, no stubs)

1. **Migration 133** -- visual_qa_runs table
2. **Config file** -- config/visualqa.json with page list
3. **Screenshot capture** -- Playwright headless, captures to /tmp
4. **Baseline manager** -- read/write manifest.json, git commit baselines
5. **Comparison engine** -- Gemini vision API call with structured prompt
6. **Pipeline handler** -- webhook endpoint, triggers capture+compare, writes events
7. **Dashboard UI** -- Visual QA section with side-by-side and approve button
8. **End-to-end test** -- trigger manual run, verify baselines created on GitHub, verify dashboard shows results

Each step must be complete and tested before moving to the next. No "we'll wire this later."

## What This Is NOT

- NOT a pixel-diff tool. Uses semantic comparison via Gemini.
- NOT a replacement for human design review. It catches regressions, not bad design decisions.
- NOT a separate service. It runs inside the governor process, same as all other handlers.
- NOT a substitute for CSS fixes. It validates that fixes worked, it doesn't write CSS.

## X220 Resilience

- Baselines: on GitHub in vibepilot repo
- Run history: in PostgreSQL (backed up hourly to knowledgebase repo via pg-backup cron)
- Config: in vibepilot repo (config/visualqa.json)
- If X220 dies: clone repos, restore PG backup, governor starts up, Visual QA works
- No local-only state

## Success Criteria

1. Manual trigger from dashboard captures all configured pages within 60 seconds
2. Baseline images appear in vibepilot/baselines/ on GitHub after first run
3. Intentional CSS change triggers visual_qa_failed with specific diff description
4. Human can approve new baseline from dashboard, new image committed to GitHub
5. Vercel deploy automatically triggers Visual QA (if webhook configured)
6. Zero local-only state. Everything on GitHub or PostgreSQL.
