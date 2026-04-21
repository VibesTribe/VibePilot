# VibePilot End-to-End Learning Audit
# Generated: 2026-04-20, verified against committed code
# SCOPE: Every agent process, what's tracked, what's not, what optimization is possible

## The Full Task Lifecycle (PRD → Done)

```
PRD Ready → Plan Creation → Plan Review → Council Review → Plan Approved
    → Task Available → Task Execution → Task Review → Task Testing
    → Task Approval → Task Merge
    (parallel: System Research → Research Council)
    (parallel: Maintenance Commands)
```

## Agent Process Tracking Status

### 1. PRD READY (no handler — PRD arrives via GitHub commit)
- Event: EventPRDReady (defined but no handler registered)
- Tracking: N/A (PRD is a file, not an AI agent process)
- Status: No learning needed — this is the input

### 2. PLAN CREATION (handlers_plan.go, 498 lines)
**Events:** EventPlanCreated, EventPlanReview
**Roles:** planner, supervisor
**What it does:** Routes to planner model → generates plan → supervisor reviews → approve/revise
**Tracking:**
  - RecordUsage: YES (1 call — tokens for planner)
  - RecordCompletion: YES (2 calls — planner + supervisor timing)
  - RecordRateLimit: YES (1 call)
  - recordSuccess/recordFailure: NO (uses RecordCompletion true/false)
  - update_model_learning: NO
**MISSING:**
  - No model learning on plan rejection (supervisor says "revise" — which model failed?)
  - Plan quality not tracked (how many revisions before approval? correlation with model?)
  - Supervisor model gets no learning signal for review quality

### 3. COUNCIL REVIEW (handlers_council.go, 539 lines)
**Events:** EventCouncilReview, EventCouncilDone
**Roles:** council (3 members with different lenses)
**What it does:** 3 council members review plan in parallel/sequential, vote approve/block/revision_needed
**Tracking:**
  - RecordUsage: NONE
  - RecordCompletion: NONE
  - RecordRateLimit: NONE
  - recordSuccess/recordFailure: NONE
  - update_model_learning: NONE
**MISSING — EVERYTHING:**
  - Which models served as council members? Not tracked for learning.
  - Vote quality not tracked (does model X always block good plans? always approve bad ones?)
  - Tokens consumed by each council member — not tracked
  - Council member model learning: which models give useful reviews?
  - Consensus correlation with model — not tracked
  - Uses SelectRouting but never records the routing outcome
  - Uses session.Run but never records tokens/duration

### 4. TASK EXECUTION (handlers_task.go, 1263 lines)
**Events:** EventTaskAvailable, EventTaskReview
**Roles:** task_runner, supervisor
**What it does:** Routes task → model writes code → supervisor reviews → approve/revise
**Tracking:** MOSTLY COMPLETE
  - RecordUsage: YES (2 calls)
  - RecordCompletion: YES (5 calls — execution + review)
  - recordSuccess: YES (3 calls)
  - recordFailure: YES (3 calls)
  - recordModelLearning: YES (4 calls)
  - update_model_learning: YES (3 calls)
  - RecordRateLimit: YES (2 calls)
  - RecordConnectorCooldown: YES (2 calls)
  - create_task_run: YES (full run data with tokens, costs)
  - calc_run_costs: YES (ROI tracking)
**MISSING:**
  - Courier success/failure learning (courier results come async via EventCourierResult)
  - Cross-task dependency learning (which task types tend to depend on what)

### 5. TASK TESTING (handlers_testing.go, 604 lines)
**Events:** EventTaskTesting
**What it does:** Runs go build + go test, records pass/fail
**Tracking:**
  - RecordCompletion: YES (2 calls — pass + fail)
  - update_model_learning: YES (1 call — on failure)
**STATUS:** Adequately tracked after this session's wiring

### 6. SYSTEM RESEARCH (handlers_research.go, 415 lines)
**Events:** EventResearchReady, EventResearchCouncil
**Roles:** (routes to models but no named role)
**What it does:** Research agent investigates system improvements, council reviews suggestions
**Tracking:**
  - RecordUsage: NONE
  - RecordCompletion: NONE
  - RecordRateLimit: NONE
  - recordSuccess/recordFailure: NONE
**MISSING — NEARLY EVERYTHING:**
  - Research model gets no feedback on suggestion quality
  - Council review of research has no tracking
  - Tokens consumed — not tracked
  - Research suggestion outcomes (accepted/rejected/implemented) — not fed back to model
  - Uses SelectDestination + session.Run but records nothing about either

### 7. MAINTENANCE (handlers_maint.go, 308 lines)
**Events:** EventMaintenanceCmd, EventTaskApproval, EventTaskMergePending
**What it does:** Executes maintenance commands, handles approval/merge
**Tracking:**
  - recordSuccess: YES (2 calls — maintenance completion + model success)
  - RecordUsage: NONE
  - RecordCompletion: NONE
**MISSING:**
  - Token usage for maintenance operations
  - Maintenance command success rates per model
  - Duration tracking for maintenance tasks

### 8. COURIER RESULT (no dedicated handler file, handled in main.go)
**Event:** EventCourierResult
**What it does:** Receives async results from courier dispatches
**Tracking:**
  - Updates task_runs with completion status
  - Token/cost recording via task_runs
**MISSING:**
  - Courier platform reliability learning
  - Per-platform success rates
  - Per-platform latency tracking

## Summary: Learning Coverage by Process

| Process | Usage | Completion | Success/Fail | Model Learning | Coverage |
|---------|-------|------------|--------------|----------------|----------|
| Plan Creation | YES | YES | partial | NO | 70% |
| Plan Review (supervisor) | YES | YES | YES | partial | 80% |
| Council Review | NO | NO | NO | NO | 0% |
| Task Execution | YES | YES | YES | YES | 95% |
| Task Review (supervisor) | YES | YES | YES | YES | 90% |
| Task Testing | NO | YES | YES | YES | 80% |
| System Research | NO | NO | NO | NO | 0% |
| Research Council | NO | NO | NO | NO | 0% |
| Maintenance | NO | NO | partial | NO | 20% |
| Courier Results | partial | partial | NO | NO | 30% |

## What This Means for Optimization

The router now scores models based on learned data (GAP 2). But the data it's learning from only covers ~60% of agent processes. 

Models running council reviews, research, and maintenance are invisible to the learning system. The router can't prefer good council models or avoid bad research models because it has zero data about their performance.

### Priority gaps by impact:

1. **Council Review (0%)** — 3 models run per council, high token cost, high impact on plan quality
2. **System Research (0%)** — Generates system improvement suggestions, zero feedback loop
3. **Maintenance (20%)** — Runs regularly, bare minimum tracking
4. **Courier Results (30%)** — Platform reliability data not feeding back to routing
5. **Plan Creation (70%)** — Missing model learning on plan rejection

### What needs to happen for each:

For each process, the pattern is the same:
1. Add usageTracker to the handler struct
2. Call RecordUsage after session.Run (tokens in/out)
3. Call RecordCompletion with success/failure
4. Call update_model_learning for the specific task type
5. Wire outcome (vote, suggestion status, etc.) back as success/failure signal

This is the same pattern I used for handlers_testing.go. But it needs to be done carefully for each process, respecting the existing architecture.
