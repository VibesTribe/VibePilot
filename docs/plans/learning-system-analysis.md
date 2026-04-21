# VibePilot Learning System Analysis
# Generated: 2026-04-20, verified against committed code on main

## What "Learning" Means in VibePilot

The learning system records outcomes from every stage of the task lifecycle
so the router can make better decisions about which model to pick next time.

Data flows: Event → Record RPC → Supabase models.learned JSONB → Router reads it

## Lifecycle Stages & Learning Hooks

### 1. PLAN CREATION (handlers_plan.go)
- RecordUsage: YES (line 158, after successful plan generation)
- RecordCompletion: YES (line 159, planning task)
- record_model_success: YES (line 381, on plan approval)
- record_model_failure: YES (line 143, 317 on plan failure/rejection)
- update_model_learning: NO (not called directly)

### 2. TASK EXECUTION (handlers_task.go)
- RecordUsage: YES (line 380, after model responds)
- RecordCompletion: YES (line 358/460/563, success/fail/courier)
- recordSuccess: YES (line 456/560, on task completion)
- recordFailure: YES (line 783/897, on task failure)
- update_model_learning: YES (called inside recordSuccess/recordFailure)
- RecordRateLimit: YES (line 350, on 429)
- RecordConnectorCooldown: YES (line 355, on connector cooldown)

### 3. TASK TESTING (handlers_testing.go)
- RecordCompletion: YES (test pass: records true, test fail: records false)
- update_model_learning: YES (on test failure, with failure_class/category/detail)
- getExecutorModelID: YES (looks up which model wrote the code via task_runs)
- RecordUsage: NO (testing is local go test, no API tokens)

### 4. SUPERVISOR REVIEW (handlers_task.go - review section)
- RecordCompletion: NO (review approvals/rejections not recorded)
- update_model_learning: NO (no success/failure signal for reviewer model)
- GAP: When supervisor approves or rejects, the supervisor model gets no feedback

### 5. MAINTENANCE (handlers_maint.go)
- recordSuccess: YES (line 144, on maintenance task completion)
- RecordUsage: NO
- RecordCompletion: NO

## Where Learning Data Lives

### Supabase: models.learned (JSONB column per model)
Updated by update_model_learning RPC. Shape:
{
  "success_rates": { "planning": 0.85, "code": 0.72, "code_test": 0.65 },
  "total_runs": 42,
  "failure_counts": { "test_failure": 5, "timeout": 2 },
  "last_updated": "2026-04-20T..."
}

### Supabase: model_usage table
Written by RecordUsage RPC. Tracks tokens_in, tokens_out per model per window.

### In-Memory: UsageTracker
- Model usage windows (tokens per time window)
- Cooldown state (when model hit rate limit, when it expires)
- Connector shared limits (org-level tracking)
- Loaded from DB on startup, persisted every 30s + shutdown

### In-Memory: ConnectorUsageTracker
- Tracks shared connector limits (e.g., Groq 100K TPD org-level)
- Persisted to connector_usage table (migration 126)

### In-Memory: PlatformUsageTracker
- Tracks web platform message/token limits
- Persisted to platforms.usage_windows column

## How Router Uses Learning Data

The router (runtime/router.go) currently:
1. Reads models.json for cascade order
2. Checks UsageTracker for cooldowns/rate limits
3. Checks connector limits
4. Does NOT yet read models.learned for routing decisions

This is the Phase 3 gap: collected data exists but isn't feeding back into routing.

## Learning Feedback Loop Gaps

### GAP 1: Supervisor Review Learning (HIGH IMPACT)
WHERE: handlers_task.go review section (around "approved"/"pass" case)
WHAT: When supervisor approves or rejects a task, neither the supervisor model
      nor the executor model gets a learning signal.
WHY MATTERS: Supervisor approval = strongest quality signal. If Model A's code
      consistently passes review and Model B's doesn't, the router should prefer A.
FIX: Add RecordCompletion for reviewer model + update_model_learning on approval/rejection

### GAP 2: Learning → Router Feedback (CRITICAL)
WHERE: runtime/router.go SelectDestination
WHAT: Router doesn't read models.learned data to adjust cascade preferences
WHY MATTERS: Without this, all the data we collect has ZERO effect on routing.
      Model A could have 95% success and Model B 10% and they get equal priority.
FIX: After cascade filtering, sort candidates by learned success_rate for the task type

### GAP 3: Test Failure → Task Re-assignment (MEDIUM)
WHERE: handlers_testing.go test failure path
WHAT: When tests fail, task goes back to "available" but no routing preference
      is set. Could go to same model that failed.
WHY MATTERS: Model writes bad code → tests fail → same model tries again → same result
FIX: Add routing_flag or routing_preference to avoid recently-failed model

### GAP 4: Cross-Model Comparison Analytics (LOW)
WHERE: Dashboard (new component needed)
WHAT: No way to see which models perform best on which task types
WHY MATTERS: User needs visibility to make manual benching/promotion decisions
FIX: Dashboard LearningFeed component reads models.learned, shows comparison

### GAP 5: Config ↔ DB Sync for Learning (OPERATIONAL)
WHERE: models.json ↔ Supabase models table
WHAT: 25 models exist in config but not in DB. If router routes to them,
      create_task_run fails (FK constraint) and no learning data is recorded.
WHY MATTERS: Silent data loss. Task runs with missing model IDs = no learning.
FIX: Sync all models.json entries to Supabase before next governor start

## Implementation Priority

1. GAP 5 (Config↔DB sync) — Without this, learning data is silently lost
2. GAP 1 (Supervisor review learning) — Small code change, high value signal
3. GAP 2 (Learning→Router feedback) — Closes the loop, but depends on having data first
4. GAP 3 (Test failure re-assignment) — Prevents spinning on failing model
5. GAP 4 (Dashboard analytics) — Visibility, not functional

## What's Already Done vs What's Needed

DONE (committed on main):
- Phase 1 data collection: task execution, planning, testing all record outcomes
- UsageTracker with persistence across restarts
- Connector + platform usage tracking with persistence
- Rate limit recording with cooldown
- Vault decryption for API keys
- Courier pipeline framework

NEEDED (not yet implemented):
- Config↔DB sync (25 models missing from DB)
- Supervisor review learning hooks
- Router reading learned data for model preference
- Test failure routing preference
- Dashboard learning analytics
