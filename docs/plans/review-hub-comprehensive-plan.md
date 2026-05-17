# Review Hub: Comprehensive Build Plan

## Goal
A single unified review queue in the dashboard that aggregates ALL human review needs. Every system that produces reviewable output feeds into one queue with type-specific actions.

## Architecture: Streams Feed Into One Queue

```
Visual QA Agent ──┐
Design Preview ───┤
Research Reports ──┤──► review_items table ──► Dashboard Review Hub UI
KB Contradictions ─┤
Council Votes ─────┘
```

Every reviewable event inserts a row into a unified `review_items` table with a `type` column. The dashboard reads one table, renders type-specific actions. No more separate code paths per type.

---

## Phase 1: Backend Streams (build any order)

Each stream needs to: (1) detect events, (2) insert into review_items, (3) provide enough context for the dashboard to render type-appropriate actions.

### 1A. Visual QA Review Items

Current state: Agent runs, produces issues (208 in last run). DB tables exist (visual_qa_runs, visual_qa_issues). But NO rows flow to review.

What to build:
- After a VisualQA run completes, insert a review_item of type "visual_qa" for each run with issues_found > 0
- Payload: { run_id, pages_checked, pages_failed, issues_found, screenshot_urls }
- User actions: Approve (mark baseline), Flag Issue (sends to fix queue), Dismiss
- Wire into the existing RunVisualQA completion handler in the governor

Effort: Medium (insert logic + API endpoint for approve/flag actions)

### 1B. Design Preview Review Items

Current state: 2 test design_previews exist (both approved). Code built and compiled. But governor never triggers it on real tasks (0 rows from real tasks).

What to build:
- Fix the trigger: design preview should fire when a task reaches pre-execution stage
- After generator produces mockup, insert review_item of type "design_preview"
- Payload: { task_id, mockup_html_path, mockup_screenshot_path, design_prompt, model_id }
- User actions: Approve (proceed to PRD/execution), Revise (feedback loop with notes), Reject (skip design)
- This is the "designer" review stream

Effort: Medium-Low (trigger fix is the hard part, review_item insert is trivial)

### 1C. Research Report Review Items

Current state: 13 pending research_suggestions in DB. Governor review-queue endpoint already returns them but only as GitHub links, no inline actions.

What to build:
- Research handler already exists (463 lines in handlers_research.go)
- When research completes with status requiring human review (complex/human), insert review_item of type "research"
- Payload: { suggestion_id, title, summary, findings_path, type, complexity }
- User actions: Approve (change status to approved, route to implementation), Reject (discard with reason), Defer (keep pending)
- Replace current GitHub-link-only rendering with inline approve/reject

Effort: Low (handler exists, just needs review_item insert + dashboard actions)

### 1D. KB Contradiction Review Items

Current state: 16 kb_contradictions exist (5 resolved, rest unknown). Auto-detection runs. But NO human review flow.

What to build:
- When contradiction detection finds a new conflict, insert review_item of type "contradiction"
- The contradiction should be pre-researched: agent checks git history, code, docs, presents recommendation + evidence
- Payload: { contradiction_id, subject, claim_a, claim_b, conflict_type, recommendation, evidence }
- User actions: Confirm (accept recommendation, resolve), Override (resolve with different answer + notes), Defer (keep investigating)

Effort: Medium (needs the pre-research agent logic, review_item insert is trivial)

### 1E. Council Vote Review Items

Current state: council_reviews table exists with columns for vote, lens, confidence, concerns, etc. But 0 rows. Council system is wired in the pipeline but may not have been triggered yet.

What to build:
- When council review completes and the decision is split or low-confidence, insert review_item of type "council"
- This surfaces when the AI council cannot reach consensus and needs the human to break the tie
- Payload: { plan_id, votes_summary, concerns, suggestions }
- User actions: Approve Plan, Request Revision, Add Guidance (notes injected back to planner)

Effort: Low (pipeline event exists, just needs review_item insert on split votes)

---

## Phase 2: Unified Backend (review_items table)

### Create the review_items table

```sql
CREATE TABLE review_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  type TEXT NOT NULL CHECK (type IN ('visual_qa', 'design_preview', 'research', 'contradiction', 'council', 'credit_alert', 'task_review')),
  source_id TEXT NOT NULL,          -- FK to the source table's ID
  title TEXT NOT NULL,               -- Human-readable title
  summary TEXT,                       -- Brief description
  payload JSONB NOT NULL DEFAULT '{}', -- Type-specific data
  status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected', 'deferred', 'flagged', 'resolved')),
  priority TEXT NOT NULL DEFAULT 'medium' CHECK (priority IN ('critical', 'high', 'medium', 'low')),
  human_notes TEXT,                   -- Notes from human reviewer
  reviewed_at TIMESTAMPTZ,
  reviewed_by TEXT DEFAULT 'human',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_review_items_status ON review_items(status);
CREATE INDEX idx_review_items_type ON review_items(type);
```

### Governor API endpoints

- GET /api/review-items -- returns all pending items, grouped by type
- PATCH /api/review-items/:id -- update status, add notes
- Each stream's completion handler inserts into this table

---

## Phase 3: Dashboard Review Hub UI

### Single queue, type-aware rendering

The existing ReviewPanel + review-queue infrastructure can be extended:

1. One list view showing all pending review_items sorted by priority then date
2. Each item renders differently based on type:
   - visual_qa: shows screenshot diff, issue count, approve/flag buttons
   - design_preview: shows mockup preview, approve/revise/reject buttons
   - research: shows summary + findings, approve/reject/defer buttons
   - contradiction: shows both claims + recommendation, confirm/override buttons
   - council: shows vote breakdown + concerns, approve/revise/guidance buttons
   - credit_alert: informational only, link to top up
   - task_review: existing ReviewPanel (already built)
3. Clicking an item opens a type-specific modal with the right actions

### Build order for Phase 3
1. review_items API endpoint (backend)
2. ReviewHub component that fetches and renders the unified queue
3. Type-specific item cards (start with research since data already exists)
4. Type-specific action modals
5. Wire into existing MissionHeader review pill (replace current multi-source fetch)

---

## Effort Estimates

| Stream | Backend | Dashboard | Total |
|--------|---------|-----------|-------|
| 1A. Visual QA | Medium | Medium | ~300 lines |
| 1B. Design Preview | Medium-Low | Low | ~200 lines |
| 1C. Research | Low | Low | ~150 lines |
| 1D. Contradictions | Medium | Low | ~250 lines |
| 1E. Council | Low | Low | ~100 lines |
| Phase 2. review_items | Low | -- | ~80 lines (migration + API) |
| Phase 3. Dashboard UI | -- | Medium-High | ~400 lines |

Total: ~1500 lines of new code across Go backend + React frontend.

## Recommended Build Order

Since all Phase 1 streams are independent:
1. Phase 2 first (review_items table + API) -- the foundation
2. Then Phase 1 streams in any order (research is easiest since data exists)
3. Then Phase 3 dashboard UI
