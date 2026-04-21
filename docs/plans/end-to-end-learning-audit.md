# VibePilot End-to-End Learning Audit
# Generated: 2026-04-20, verified against architecture docs + committed code
# ARCHITECTURE FIRST: Docs describe intent, code shows current state

## The Full Task Lifecycle (from WYNTK)

```
1. HUMAN pushes PRD to GitHub (docs/prd/*.md)
   ↓ Governor detects via Supabase Realtime (GitHub webhook → Supabase)
2. PLANNER creates plan + tasks from PRD
   ↓ Analyzes PRD, creates plan in Supabase, breaks into atomic tasks
3. SUPERVISOR reviews plan
   ↓ Validates breakdown, checks dependencies, approves/revise
4. COUNCIL reviews plan (3 lenses: user_alignment, architecture, feasibility)
   ↓ Independent votes → consensus → approved/blocked/revision_needed
   ↓ Council reviews PLANS only, not outputs. Supervisor owns output review.
5. Tasks become AVAILABLE (dependencies met)
   ↓ Governor emits EventTaskAvailable
6. ROUTER selects model + routing path (internal/web/mcp)
   ↓ Writes to tasks.assigned_to, sets routing_flag
7a. INTERNAL: Model generates code in isolated worktree
  OR
7b. COURIER: Dispatches to free web platform via browser-use
   ↓
8. SUPERVISOR reviews ALL outputs (code quality, task compliance)
   ↓ Approve → testing. Reject → back to task runner.
9. TESTER validates (go test + lint/typecheck)
   ↓ Pass → merge. Fail → back to task runner.
10. AUTO-MERGE (shadow merge safety, conflict resolution)
   ↓
11. DASHBOARD shows progress (read-only, realtime)

PARALLEL:
- SYSTEM RESEARCH: Daily 6 AM UTC, researches models/platforms/pricing
  → Findings go to Council review → accepted/rejected
  → Has "WHAT I'VE LEARNED" section designed for feedback accumulation
- MAINTENANCE: Handles approval, merge, cleanup operations
```

## Agent Process Tracking Status (verified against code)

### PLANNER (handlers_plan.go, 498 lines)
**Trigger:** EventPlanCreated (PRD detected → plan needed)
**Flow:** Route to planner model → generate plan → supervisor reviews plan
**Roles:** planner, supervisor
**Current tracking:**
  - RecordUsage: YES (planner tokens)
  - RecordCompletion: YES (planner + supervisor timing)
  - RecordRateLimit: YES
  - recordSuccess/recordFailure: NO (uses RecordCompletion true/false)
**Missing:**
  - Plan revision tracking: how many rounds before approval? Not recorded per-model.
  - Supervisor model learning: supervisor approves/revises plans, quality not tracked.

### COUNCIL (handlers_council.go, 539 lines)
**Trigger:** EventCouncilReview (plan approved by supervisor → council review)
**Flow:** 3 members vote independently (parallel or sequential depending on model availability)
  - Each member gets a different lens (user_alignment, architecture, feasibility)
  - Each routed independently through cascade (can use different models)
  - Votes: APPROVED / REVISION_NEEDED / BLOCKED
  - Consensus determines outcome
  - Max 6 rounds, then supervisor decides
**Current tracking: NONE (0%)**
  - RecordUsage: NO — tokens consumed by each council member, not tracked
  - RecordCompletion: NO — timing for each member's review, not tracked
  - Model learning: NO — which models give good council votes, not tracked
  - Vote quality: NO — correlation between model vote and eventual plan success, not tracked
  - Uses SelectRouting (per-member!) but records nothing
  - Uses session.Run (3 calls per council!) but records nothing
  - Stores reviews in DB (store_council_reviews) but no learning signal
**Impact:** Council is expensive (3 model calls per plan, can go 6 rounds = 18 calls max).
  With zero tracking, the router cannot learn which models are good council members.

### TASK EXECUTION (handlers_task.go, 1263 lines)
**Trigger:** EventTaskAvailable (task with met dependencies)
**Flow:** Route to model → execute in worktree → supervisor reviews output
**Current tracking: 95%**
  - Full lifecycle: RecordUsage, RecordCompletion, recordSuccess/Failure, model learning
  - Rate limiting, connector cooldown, cost calculation
  - Task runs with full token/cost data
  - Courier result handling (EventCourierResult)
**Missing:**
  - Courier platform reliability learning (platform success rates)
  - Cross-task dependency learning

### TASK REVIEW / SUPERVISOR (handlers_task.go, same file)
**Trigger:** EventTaskReview (output ready for review)
**Flow:** Supervisor reviews output quality → approve/reject/needs_revision
**Current tracking: 90%**
  - RecordCompletion: YES (with reviewStart timing)
  - update_model_learning: YES (on approval)
  - recordModelLearning: YES (on all outcomes)
**Missing:**
  - Review quality learning: does supervisor A approve code that fails tests?
  - No feedback loop from test results back to supervisor model

### TESTING (handlers_testing.go, 604 lines)
**Trigger:** EventTaskTesting (supervisor approved → run tests)
**Flow:** go build + go test → pass/fail
**Current tracking: 80%**
  - RecordCompletion: YES (pass/fail)
  - update_model_learning: YES (on failure, with failure detail)
  - Failed model stored in routing_flag_reason for exclusion
**Missing:**
  - Test coverage tracking over time
  - Model-specific test pass rates

### SYSTEM RESEARCH (handlers_research.go, 415 lines)
**Trigger:** EventResearchReady (daily scheduled research)
**Flow:** Route to research model → investigate landscape → produce findings
  → EventResearchCouncil → council reviews suggestions → accepted/rejected
**Current tracking: NONE (0%)**
  - RecordUsage: NO — tokens for research, not tracked
  - RecordCompletion: NO — timing, not tracked
  - Model learning: NO — which models produce useful research, not tracked
  - Research outcome feedback: NO — suggestion accepted/rejected never feeds back to model
  - The prompt has a "WHAT I'VE LEARNED" section but it's never populated from code
**Impact:** Research runs daily. Without outcome feedback, the research model never improves.
  Bad suggestions keep coming. Good suggestion patterns aren't reinforced.

### MAINTENANCE (handlers_maint.go, 308 lines)
**Triggers:** EventMaintenanceCmd, EventTaskApproval, EventTaskMergePending
**Flow:** Execute maintenance commands, handle approval/merge workflow
**Current tracking: 20%**
  - recordSuccess: YES (maintenance completion)
  - recordModelSuccess: YES (model success on maintenance)
**Missing:**
  - Token usage for maintenance operations
  - Duration tracking
  - Maintenance command success rates per model

## Learning Coverage Summary

| Process | Usage | Completion | Success/Fail | Model Learning | Coverage |
|---------|-------|------------|--------------|----------------|----------|
| Planner | YES | YES | partial | NO | 70% |
| Council (3x per plan) | NO | NO | NO | NO | 0% |
| Task Execution | YES | YES | YES | YES | 95% |
| Task Review (Supervisor) | YES | YES | YES | YES | 90% |
| Testing | NO | YES | YES | YES | 80% |
| Research | NO | NO | NO | NO | 0% |
| Research Council | NO | NO | NO | NO | 0% |
| Maintenance | NO | NO | partial | NO | 20% |

## What the Router Can vs Cannot Learn

The router now uses GetModelLearnedScore(modelID, taskType) to prefer models
with proven success. But it only has data for:

CAN learn: planning (partial), task execution, task review, testing
CANNOT learn: council review, research, research council, maintenance

This means:
- A model terrible at council review (rubber-stamps bad plans) gets no penalty
- A model great at research (finds actionable insights) gets no bonus
- A model that fails maintenance operations gets no demerit
- The router treats them all as "neutral 0.5 score" — same as a brand new model

## Priority: Where Learning Matters Most

1. **Council (0% tracked, HIGH cost)** — 3-18 model calls per plan, 0 data.
   If we learn which models give rigorous reviews vs rubber stamps, we can
   route council to the best reviewers. Direct impact on plan quality.

2. **Research (0% tracked, DAILY cost)** — Runs every day. Suggestion quality
   never feeds back. The prompt is designed for learning ("WHAT I'VE LEARNED")
   but the code never populates it. Missed feedback loop by design.

3. **Maintenance (20% tracked)** — Regular operations, minimal tracking.
   Lower priority since maintenance is less model-dependent.

4. **Planner (70% tracked)** — Missing plan revision count and model learning.
   Medium priority — most of the tracking is there.

## IMPORTANT: Architecture Alignment

Before implementing any tracking changes, each handler's learning must respect:

1. **Config over code** — Learning behavior should be configurable, not hardcoded
2. **No vendor lock-in** — Tracking must work with any model, any provider
3. **Dashboard is sacred** — Any new data the governor writes must match what
   the dashboard expects. Check DASHBOARD_AUDIT.md before adding fields.
4. **Supabase Realtime, never poll** — Any learning data writes trigger realtime
5. **JSONB everywhere** — All new tracking data in JSONB columns
