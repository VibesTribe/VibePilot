# VibePilot Go Rewrite: Complete Specification

## Context Window: 72% - Must Be Precise

**This document is the single source of truth for the rewrite.**
**Any AI session should be able to execute this without confusion.**

---

## Part 1: Files to KEEP (Do Not Modify)

### Core Infrastructure (Working)

| File | Purpose | Status |
|------|---------|--------|
| `cmd/governor/main.go` | Bootstrap, subscriptions, routing | ✅ KEEP |
| `internal/db/supabase.go` | Database client | ✅ KEEP |
| `internal/db/rpc.go` | RPC allowlist | ✅ KEEP |
| `internal/realtime/client.go` | Supabase realtime | ✅ KEEP |
| `internal/webhooks/github.go` | GitHub webhook handler | ✅ KEEP |
| `internal/webhooks/server.go` | Webhook server | ✅ KEEP |
| `internal/gitree/gitree.go` | Git operations | ✅ KEEP |
| `internal/security/leak_detector.go` | Secret scanning | ✅ KEEP |
| `internal/vault/vault.go` | Credential vault | ✅ KEEP |
| `internal/runtime/config.go` | Config loading | ✅ KEEP |
| `internal/runtime/session.go` | Execution session | ✅ KEEP |
| `internal/runtime/parallel.go` | Pool management | ✅ KEEP |
| `internal/runtime/events.go` | Event types | ✅ KEEP |
| `internal/connectors/runners.go` | CLI runners | ✅ KEEP |
| `internal/tools/*` | All tools | ✅ KEEP |

### Supporting Files (Working)

| File | Purpose | Status |
|------|---------|--------|
| `cmd/governor/types.go` | Type definitions | ✅ KEEP |
| `cmd/governor/helpers.go` | Helper functions | ✅ KEEP |
| `cmd/governor/recovery.go` | Startup recovery | ✅ KEEP |
| `cmd/governor/adapters.go` | Model adapters | ✅ KEEP |

---

## Part 2: Files to REWRITE (Delete and Recreate)

### Event Handlers (All Broken)

| File | Lines | Why Rewrite |
|------|-------|-------------|
| `cmd/governor/handlers_plan.go` | ~730 | Task creation broken, status wrong |
| `cmd/governor/handlers_task.go` | ~565 | Execution broken, no error handling |
| `cmd/governor/handlers_testing.go` | ~200 | Not wired properly |
| `cmd/governor/handlers_council.go` | ~150 | Not wired |
| `cmd/governor/handlers_research.go` | ~100 | Not wired |
| `cmd/governor/handlers_maint.go` | ~50 | Minor, can keep |

### Supporting Logic (Partially Broken)

| File | Lines | Why Rewrite |
|------|-------|-------------|
| `cmd/governor/validation.go` | ~306 | Task parsing works, creation broken |
| `internal/runtime/router.go` | ~163 | Confused model/destination |

---

## Part 3: Database Schema (Source of Truth)

### Tables

```sql
-- PLANS
plans (
  id UUID PRIMARY KEY,
  project_id UUID,
  prd_path TEXT,
  plan_path TEXT,
  status TEXT,  -- draft, review, approved, rejected, revision_needed
  complexity TEXT,
  council_mode BOOLEAN,
  council_models JSONB,
  council_reviews JSONB,
  council_consensus TEXT,
  review_notes JSONB,
  human_decision TEXT,
  revision_round INT,
  revision_history JSONB,
  created_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ
)

-- TASKS
tasks (
  id UUID PRIMARY KEY,
  plan_id UUID REFERENCES plans(id),
  task_number TEXT,
  title TEXT,
  type TEXT,
  status TEXT CHECK (status IN (
    'available', 'pending', 'in_progress', 'review', 
    'testing', 'approval', 'merged', 'error'
  )),
  priority INT,
  confidence FLOAT,
  category TEXT,
  dependencies JSONB,  -- Array of task IDs
  assigned_to TEXT,    -- Model ID
  routing_flag TEXT CHECK (routing_flag IN ('internal', 'web', 'mcp')),
  routing_flag_reason TEXT,
  routing_history JSONB,
  result JSONB,
  processing_by TEXT,
  processing_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ,
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ
)

-- TASK PACKETS
task_packets (
  id UUID PRIMARY KEY,
  task_id UUID REFERENCES tasks(id),
  prompt TEXT,
  expected_output TEXT,
  context JSONB,
  version INT,
  revision_reason TEXT,
  created_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ
)

-- TASK RUNS
task_runs (
  id UUID PRIMARY KEY,
  task_id UUID REFERENCES tasks(id),
  model_id TEXT,
  courier TEXT,
  platform TEXT,
  status TEXT,
  tokens_in INT,
  tokens_out INT,
  tokens_used INT,
  courier_model_id TEXT,
  courier_tokens INT,
  courier_cost_usd DECIMAL(10,6),
  platform_theoretical_cost_usd DECIMAL(10,6),
  total_actual_cost_usd DECIMAL(10,6),
  total_savings_usd DECIMAL(10,6),
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ
)

-- MODELS
models (
  id TEXT PRIMARY KEY,
  name TEXT,
  status TEXT,  -- active, paused, error
  context_limit INT,
  tasks_completed INT,
  tasks_failed INT,
  success_rate DECIMAL(5,4),
  tokens_used INT,
  cost_input_per_1k_usd DECIMAL(10,6),
  cost_output_per_1k_usd DECIMAL(10,6),
  subscription_cost_usd DECIMAL(10,2),
  cooldown_expires_at TIMESTAMPTZ,
  config JSONB
)

-- PLATFORMS
platforms (
  id TEXT PRIMARY KEY,
  name TEXT,
  status TEXT,
  context_limit INT,
  theoretical_cost_input_per_1k_usd DECIMAL(10,6),
  theoretical_cost_output_per_1k_usd DECIMAL(10,6),
  config JSONB
)
```

### RPCs (Must Exist)

```sql
-- Task management
create_task_with_packet(p_plan_id, p_task_number, p_title, p_type, p_prompt, 
                        p_status, p_priority, p_confidence, p_category, 
                        p_routing_flag, p_routing_flag_reason, p_dependencies,
                        p_expected_output, p_context) RETURNS UUID

update_task_status(p_task_id, p_status) RETURNS VOID

update_task_assignment(p_task_id, p_status, p_assigned_to, 
                       p_routing_flag, p_routing_flag_reason) RETURNS JSONB

create_task_run(p_task_id, p_model_id, p_courier, p_platform, p_status,
                p_tokens_in, p_tokens_out, p_tokens_used,
                p_courier_model_id, p_courier_tokens, p_courier_cost_usd,
                p_platform_theoretical_cost_usd, p_total_actual_cost_usd,
                p_total_savings_usd, p_started_at, p_completed_at) RETURNS UUID

-- Learning
record_model_success(p_model_id, p_task_type, p_duration_seconds, p_tokens_used) RETURNS VOID

record_model_failure(p_model_id, p_task_id, p_failure_type, p_failure_category) RETURNS VOID

-- Learning: Heuristics
get_heuristic(p_task_type, p_condition) RETURNS JSONB

upsert_heuristic(p_task_type, p_condition, p_preferred_model, p_action, p_confidence, p_source) RETURNS UUID

record_heuristic_result(p_heuristic_id, p_success) RETURNS VOID

-- Learning: Planner Rules
create_planner_rule(p_pattern, p_rule_type, p_action, p_confidence) RETURNS UUID

get_planner_rules(p_rule_type) RETURNS TABLE(...)

record_planner_rule_applied(p_rule_id, p_plan_id, p_success) RETURNS VOID

-- Learning: Supervisor Rules
create_supervisor_rule(p_pattern, p_rule_type, p_action, p_confidence) RETURNS UUID

get_supervisor_rules(p_rule_type) RETURNS TABLE(...)

record_supervisor_rule(p_task_id, p_rule_id, p_triggered_by, p_outcome) RETURNS VOID

-- Processing
set_processing(p_table, p_id, p_processing_by) RETURNS BOOLEAN
clear_processing(p_table, p_id) RETURNS VOID

-- Checkpoints
save_checkpoint(p_task_id, p_step, p_progress, p_output, p_files) RETURNS UUID
load_checkpoint(p_task_id) RETURNS JSONB
delete_checkpoint(p_task_id) RETURNS VOID

-- Dependencies
unlock_dependent_tasks(p_completed_task_id) RETURNS VOID
```

---

## Part 4: Configuration Files (Source of Truth)

### models.json

```json
{
  "models": [
    {
      "id": "glm-5",
      "name": "GLM-5",
      "status": "active",
      "access_via": ["opencode", "kilo"],
      "context_limit": 128000,
      "cost_input_per_1k_usd": null,
      "cost_output_per_1k_usd": null,
      "subscription_cost_usd": null
    }
  ]
}
```

### connectors.json

```json
{
  "destinations": [
    {
      "id": "kilo",
      "type": "cli",
      "status": "active",
      "command": "kilo",
      "cli_args": ["run", "-m", "zhipuai-coding-plan/glm-5"]
    },
    {
      "id": "chatgpt-web",
      "type": "web",
      "status": "active",
      "url": "https://chatgpt.com"
    }
  ]
}
```

### routing.json

```json
{
  "strategies": {
    "planner": "internal_only",
    "supervisor": "internal_only",
    "task_runner": "default",
    "researcher": "internal_only"
  },
  "priority": {
    "internal_only": ["internal"],
    "default": ["external", "internal"]
  }
}
```

---

## Part 5: Event Flow (Exact Sequence)

### Event 1: Plan Created

**Trigger:** INSERT on plans table (status=draft)

**Handler:** `EventPlanCreated` in handlers_plan.go

**Steps:**
1. Claim plan (set_processing)
2. Read PRD from prd_path
3. Router select planner model (strategy: internal_only)
4. Create session with planner role
5. Execute planner (input: PRD content)
6. Parse planner output:
   - plan_path
   - plan_content (markdown with tasks)
   - total_tasks
7. Commit plan file to git
8. Trigger supervisor review (call supervisor RPC or emit event)

**On Error:**
- Set plan status = "error"
- Clear processing
- Log error

**Output:** Plan file committed, status = "review"

---

### Event 2: Plan Review

**Trigger:** Plan status = "review" (from EventPlanCreated or manual)

**Handler:** `EventPlanReview` in handlers_plan.go

**Steps:**
1. Claim plan (set_processing)
2. Read PRD from prd_path
3. Read plan from plan_path
4. Router select supervisor model
5. Create session with supervisor role
6. Execute supervisor (input: PRD + plan)
7. Parse supervisor decision:
   - decision: "approved" | "needs_revision" | "council_review"
   - concerns: []
   - tasks_needing_revision: []
8. If approved:
   - Call createTasksFromPlan()
   - Set plan status = "approved"
9. If needs_revision:
   - Set plan status = "revision_needed"
   - Store concerns in review_notes
10. If council_review:
    - Set council_mode = true
    - Set plan status = "council_review"

**On Error:**
- Set plan status = "error"
- Clear processing

**Output:** Tasks created (if approved)

---

### Event 3: Task Available

**Trigger:** INSERT on tasks table (status=available)

**Handler:** `EventTaskAvailable` in handlers_task.go

**Steps:**
1. Claim task (set_processing)
2. Load task_packet
3. Create git branch (task/T001)
4. Router select task_runner model
5. Call update_task_assignment RPC:
   - status = "in_progress"
   - assigned_to = model_id
   - routing_flag = "internal" (for coding) or "web" (for research)
6. Save checkpoint (if enabled)
7. Create session with task_runner role
8. Execute task_runner (input: prompt_packet)
9. Parse output:
   - files: []
   - summary: ""
   - status: "success" | "failed"
10. Scan output for leaks
11. Commit output to git branch
12. Call create_task_run RPC:
    - model_id
    - tokens_in, tokens_out
    - costs
13. If success:
    - Call update_task_status("review")
    - Call record_model_success()
14. If failure:
    - Call update_task_status("available")
    - Call record_model_failure()
15. Delete checkpoint

**On Error at Any Step:**
- Call record_model_failure()
- Call update_task_status("available")
- Clear processing

**Output:** Task in review (success) or available for retry (failure)

---

### Event 4: Task Review

**Trigger:** Task status = "review"

**Handler:** `EventTaskReview` in handlers_task.go

**Steps:**
1. Claim task
2. Router select supervisor model
3. Create session with supervisor role
4. Execute supervisor (input: task + output)
5. Parse decision:
   - decision: "pass" | "fail" | "reroute"
   - issues: []
6. If pass:
   - Call update_task_status("testing")
7. If fail:
   - Record failure details
   - Call update_task_status("available")
8. If reroute:
   - Update routing_flag
   - Call update_task_status("available")

**Output:** Task in testing (pass) or available (fail)

---

### Event 5: Task Testing

**Trigger:** Task status = "testing"

**Handler:** `EventTaskTesting` in handlers_testing.go

**Steps:**
1. Claim task
2. Run tests (go test, npm test, etc.)
3. Parse test results
4. If pass:
   - Call update_task_status("approval")
5. If fail:
   - Call update_task_status("available")
   - Record test failure

**Output:** Task in approval (pass) or available (fail)

---

## Part 6: Router Logic (Clear Separation)

### Models vs Destinations

**MODELS** (wear role hats):
- glm-5, kimi, deepseek, gemini
- Can be: planner, supervisor, task_runner, researcher, council_member
- Selected based on: role, task_type, success_rate, availability

**DESTINATIONS** (where execution happens):
- kilo (CLI), opencode (CLI), gemini-api (API)
- chatgpt-web (web), claude-web (web)
- Selected based on: task needs, model access_via

### Routing Flow

```
1. Get role (planner, supervisor, task_runner, etc.)
2. Get strategy for role (internal_only, default, etc.)
3. Get priority order (["internal"] or ["external", "internal"])
4. For each category in priority:
   a. Get available destinations in category
   b. For each destination:
      - Check if destination is active
      - Check if destination type is executable (cli, api)
      - Get models available via destination
      - Select best model (by success_rate for task_type)
      - If model found, return (destination, model)
5. If nothing found, return error
```

### Destination Type Mapping

| Type | Executable? | Use For |
|------|-------------|---------|
| cli | YES | Coding, testing, file operations |
| api | YES | Quick queries, no file access |
| web | NO | Courier destinations only |

---

## Part 7: Error Handling Rules

### Rule 1: Fail Fast

If ANY step fails, STOP immediately. Do not continue.

### Rule 2: Atomic State

Only change task status if ALL previous steps succeeded.

### Rule 3: Always Clear Processing

Use `defer clear_processing()` to ensure lock is released.

### Rule 4: Record Failures

Always call `record_model_failure()` on error.

### Rule 5: Retry Logic

On failure, set status="available" to allow retry (up to max_attempts).

---

## Part 8: Learning System (Self-Improvement)

### Overview

VibePilot learns from EVERY task. It improves at every level:
- **Model selection** → Which model for which task type
- **Planner rules** → How to break down PRDs better
- **Supervisor rules** → What to check for quality
- **Routing preferences** → When to use internal vs web
- **Failure patterns** → What to avoid

### Learning Tables

```sql
-- Model performance tracking
models (
  tasks_completed INT,
  tasks_failed INT,
  success_rate DECIMAL(5,4),
  learned JSONB  -- {"best_for": ["coding"], "avoid_for": ["research"]}
)

-- Heuristics (discovered routing optimizations)
learned_heuristics (
  task_type TEXT,
  condition JSONB,      -- {"language": "python", "complexity": "high"}
  preferred_model TEXT,
  action JSONB,         -- {"timeout_adjustment": 60}
  confidence FLOAT,
  application_count INT,
  success_count INT,
  success_rate FLOAT
)

-- Planner rules (how to plan better)
planner_rules (
  pattern TEXT,         -- "multi-file feature"
  rule_type TEXT,       -- "task_breakdown"
  action JSONB,         -- {"split_threshold": 3}
  confidence FLOAT,
  application_count INT,
  success_count INT
)

-- Supervisor rules (what to check)
supervisor_rules (
  pattern TEXT,         -- "missing_error_handling"
  rule_type TEXT,       -- "quality_check"
  action JSONB,         -- {"require": "error_test"}
  confidence FLOAT,
  triggered_count INT,
  prevented_failures INT
)

-- Failure records (what went wrong)
failure_records (
  task_id UUID,
  failure_type TEXT,    -- "timeout", "rate_limited", "quality_rejected"
  failure_category TEXT, -- "model_issue", "platform_issue", "quality_issue"
  failure_details JSONB,
  model_id TEXT,
  platform TEXT
)
```

### When Learning Happens

| Event | Learning Action | RPC Called |
|-------|-----------------|------------|
| Task passes supervisor | Record model success | `record_model_success()` |
| Task fails execution | Record model failure | `record_model_failure()` |
| Task fails supervisor | Create/update supervisor rule | `record_supervisor_rule()` |
| Plan needs revision | Create/update planner rule | `record_planner_rule()` |
| Heuristic applied | Track success/failure | `record_heuristic_result()` |
| Routing decision made | Update model success_rate | (automatic via triggers) |

### Learning Flow

```
TASK COMPLETES
    ↓
Was it successful?
    ├─ YES → record_model_success()
    │         ├─ Increment tasks_completed
    │         ├─ Update success_rate
    │         └─ Update learned.best_for
    │
    └─ NO → record_model_failure()
              ├─ Increment tasks_failed
              ├─ Update success_rate
              ├─ Create failure_record
              └─ Analyze pattern
                   └─ If pattern repeats:
                        ├─ Create learned_heuristic
                        ├─ Create supervisor_rule
                        └─ Update routing preferences
```

### How Router Uses Learning

```go
func (r *Router) selectModelForDestination(destID, taskType) string {
    // 1. Check learned heuristics first
    heuristic := get_heuristic(taskType, currentConditions)
    if heuristic != nil && heuristic.confidence > 0.8 {
        return heuristic.preferred_model
    }
    
    // 2. Fall back to success_rate
    models := get_models_via_destination(destID)
    sort_by(models, func(m) { 
        return m.success_rate_for_task_type(taskType)
    })
    
    return models[0].id
}
```

### How Supervisor Uses Learning

```go
func (s *Supervisor) reviewTask(task, output) Decision {
    // 1. Get all active rules
    rules := get_supervisor_rules("quality_check")
    
    // 2. Check each rule
    for _, rule := range rules {
        if matchesPattern(output, rule.pattern) {
            record_supervisor_rule_applied(rule.id, task.id)
            if !checkCondition(output, rule.action) {
                return Decision{
                    Decision: "fail",
                    Issues: [{rule.pattern, rule.action.require}],
                }
            }
        }
    }
    
    // 3. Standard checks
    // ...
}
```

### How Planner Uses Learning

```go
func (p *Planner) createPlan(prd) Plan {
    // 1. Get planner rules
    rules := get_planner_rules("task_breakdown")
    
    // 2. Apply rules to task creation
    for _, rule := range rules {
        if matchesPattern(prd, rule.pattern) {
            applyRule(tasks, rule.action)
            record_planner_rule_applied(rule.id, plan.id)
        }
    }
    
    // 3. Standard planning
    // ...
}
```

### Learning Principles

1. **Every outcome is recorded** - No task completes without learning
2. **Patterns become rules** - If something fails 3x, create a rule
3. **Confidence grows with repetition** - More applications = higher confidence
4. **Rules expire** - Re-learn if rule is stale (> 7 days)
5. **Human can override** - Rules have source: 'llm_analysis' | 'human'

### What Gets Learned

| Level | What's Learned | Example |
|-------|----------------|---------|
| **Model** | Which model for which task | "glm-5 best for Go coding" |
| **Routing** | When to use internal vs web | "Tasks with deps → internal" |
| **Planner** | How to break down tasks | "Multi-file → split by file" |
| **Supervisor** | What to check for quality | "Require error tests for I/O" |
| **Timeouts** | How long tasks take | "Python tests need 120s" |
| **Costs** | Actual vs theoretical | "Subscription saves $X" |

---

## Part 9: File Structure (After Rewrite)

```
cmd/governor/
├── main.go              (KEEP - bootstrap)
├── types.go             (KEEP - types)
├── helpers.go           (KEEP - helpers)
├── recovery.go          (KEEP - recovery)
├── adapters.go          (KEEP - adapters)
├── handlers_plan.go     (REWRITE - 400 lines)
├── handlers_task.go     (REWRITE - 500 lines)
├── handlers_testing.go  (REWRITE - 150 lines)
├── handlers_council.go  (REWRITE - 100 lines)
├── handlers_research.go (REWRITE - 100 lines)
└── validation.go        (REWRITE - 200 lines)

internal/runtime/
├── router.go            (REWRITE - 150 lines)
├── ... (rest KEEP)
```

**Total rewrite:** ~1,600 lines
**Time estimate:** 4-6 hours

---

## Part 10: Testing Plan

### Test 1: Hello World

1. Push PRD: "Create pkg/hello/hello.go with Hello() string"
2. Verify:
   - Plan created (status=approved)
   - Task created (status=available)
   - Task assigned (assigned_to=glm-5)
   - Task executed (status=review)
   - task_runs created (tokens > 0)
   - Git branch has output

### Test 2: Error Recovery

1. Push PRD that will fail
2. Verify:
   - Task set back to available
   - record_model_failure called
   - Can retry

### Test 3: Dependencies

1. Push PRD with 2 tasks (T002 depends on T001)
2. Verify:
   - T001 status=available
   - T002 status=pending
   - After T001 completes, T002 status=available

---

## Part 11: Success Criteria

**After rewrite, ALL of these must work:**

1. ✅ PRD pushed → Plan created (< 30s)
2. ✅ Plan reviewed → Tasks created (< 30s)
3. ✅ Task available → Task assigned (< 5s)
4. ✅ Task assigned → Task in_progress (< 5s)
5. ✅ Task executed → Output committed (< 60s for hello world)
6. ✅ task_runs created with tokens
7. ✅ Dashboard shows correct status at each step
8. ✅ Errors trigger retry (status=available)
9. ✅ Learning system records success/failure
10. ✅ Dependencies block until parent completes

**Total time for hello world:** < 2 minutes end-to-end

---

## Part 12: Rewrite Order

### Session 1 (This Session - If Time Permits)

1. Rewrite validation.go (task parsing, creation)
2. Rewrite router.go (clear model/destination)
3. Test: Can create tasks correctly

### Session 2 (Next Session)

1. Rewrite handlers_plan.go
2. Test: PRD → Plan → Tasks

### Session 3

1. Rewrite handlers_task.go
2. Test: Task execution end-to-end

### Session 4

1. Rewrite handlers_testing.go
2. Rewrite handlers_council.go
3. Rewrite handlers_research.go
4. Test: Full flow

---

## Part 13: Questions Before Starting

1. **Start with validation.go + router.go** (foundation) OR
2. **Start with handlers_task.go** (biggest impact)?

**My recommendation:** Start with validation.go + router.go to fix foundation first.

---

## Ready to Execute

This specification is complete. Any AI session can now:
1. Read this document
2. Understand exactly what to rewrite
3. Understand exactly how it should work
4. Execute without confusion

**Confirm to start:** Say "START REWRITE" and specify which session (1, 2, 3, or 4)
