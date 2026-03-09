# VibePilot Current Issues
**Last Updated:** 2026-03-09 Session 73
**Source:** Full Supabase + Code Audit

---

## 📊 AUDIT SUMMARY

| Category | Count | Status |
|----------|-------|--------|
| **Tables in Supabase** | 49 | Many legacy/unused |
| **RPCs Available** | 121 | Many in allowlist, few called |
| **Migration Files** | 95 | Need consolidation |
| **Learning Tables** | 6 | 3 empty, 3 populated |

---

## 🔴 CRITICAL ISSUES (Blocking Flow)

### 1. Module Branches Never Created

**Location:** `governor/internal/gitree/gitree.go:387`
**Problem:** `CreateModuleBranch()` exists but is NEVER CALLED
**Impact:** 
- Task branches have nowhere to merge to
- Merge to `module/<slice_id>` fails
- Task branches not deleted after completion
- Tasks get reassigned after completion instead of finalizing

**Root Cause:** No code calls `CreateModuleBranch()` when tasks are created

**Fix Required:**
```go
// In handlers_plan.go, after tasks are created from approved plan:
func createTasksFromApprovedPlan(...) error {
    // ... existing task creation code ...
    
    // Collect unique slice IDs
    sliceIDs := make(map[string]bool)
    for _, task := range tasks {
        if task.SliceID != "" {
            sliceIDs[task.SliceID] = true
        }
    }
    
    // Create module branches for each slice
    for sliceID := range sliceIDs {
        if err := git.CreateModuleBranch(ctx, sliceID); err != nil {
            log.Printf("[Plan] Warning: failed to create module branch %s: %v", sliceID, err)
            // Continue anyway - branch may already exist
        }
    }
}
```

**Files to Modify:**
- `governor/cmd/governor/handlers_plan.go`

---

### 2. Testing Flow Was Broken (FIXED Session 73)

**Location:** `governor/internal/realtime/client.go:470-471`
**Problem:** `status == "testing"` triggered `EventTaskCompleted` instead of `EventTaskTesting`
**Impact:** Testing handler never ran, tasks skipped testing phase

**Status:** ✅ FIXED in Session 73

**Before:**
```go
case status == "testing" || status == "approval":
    return string(runtime.EventTaskCompleted)
```

**After:**
```go
case status == "testing":
    return string(runtime.EventTaskTesting)
case status == "approval":
    return string(runtime.EventTaskCompleted)
```

---

### 3. Failure Notes Not Recorded in Testing

**Location:** `governor/cmd/governor/handlers_testing.go`
**Problem:** Testing failures went back to "available" with no explanation
**Impact:** Orchestrator can't intelligently reroute, no learning

**Status:** ✅ FIXED in Session 73

**Added:** `recordFailureNotes()` calls for all failure paths with detailed reasons

---

## 🟡 LEARNING SYSTEM GAPS

### 4. Supervisor Rules Not Created from Rejections

**Location:** `governor/cmd/governor/handlers_task.go:427-433`
**Problem:** Supervisor rejection records failure but doesn't create learning rule
**Impact:** System doesn't learn from supervisor rejections

**Current Code:**
```go
if decision.Decision == "needs_revision" {
    failureReason := "supervisor_reject"
    if len(decision.Issues) > 0 {
        failureReason = decision.Issues[0].Description
    }
    h.recordFailure(ctx, modelID, taskID, "supervisor_reject")
    h.recordFailureNotes(ctx, taskID, failureReason)
    // Missing: create_supervisor_rule call!
}
```

**Fix Required:**
```go
if decision.Decision == "needs_revision" {
    failureReason := "supervisor_reject"
    if len(decision.Issues) > 0 {
        failureReason = decision.Issues[0].Description
        
        // Create learning rule from rejection
        h.database.RPC(ctx, "create_supervisor_rule", map[string]any{
            "p_trigger_pattern": "task_review",
            "p_trigger_condition": map[string]any{"task_type": taskType},
            "p_rule_text": failureReason,
            "p_action": "flag_for_review",
            "p_source": "supervisor_rejection",
            "p_source_task_id": taskID,
        })
    }
    h.recordFailure(ctx, modelID, taskID, "supervisor_reject")
    h.recordFailureNotes(ctx, taskID, failureReason)
}
```

---

### 5. Tester Rules Never Created

**Location:** `governor/cmd/governor/handlers_testing.go`
**Problem:** No code calls `create_tester_rule`
**Impact:** System doesn't learn from test failures

**Table Status:** `tester_learned_rules` has 0 rows

**Fix Required:**
```go
// In handleTaskTesting, when test fails:
case "fail", "failed":
    h.recordFailure(ctx, routingResult.ModelID, taskID, "test_failed")
    h.recordFailureNotes(ctx, taskID, fmt.Sprintf("test_failed: %s", testOutput.NextAction))
    
    // Create learning rule
    h.database.RPC(ctx, "create_tester_rule", map[string]any{
        "p_trigger_pattern": "test_execution",
        "p_trigger_condition": map[string]any{"task_type": taskType},
        "p_rule_text": fmt.Sprintf("Watch for: %s", testOutput.NextAction),
        "p_action": "flag_for_fix",
        "p_source": "test_failure",
        "p_source_task_id": taskID,
    })
```

---

### 6. Heuristics Never Recorded

**Location:** No code calls `upsert_heuristic`
**Problem:** Router doesn't learn model preferences per task type
**Impact:** Same models repeatedly fail on same task types

**Table Status:** `learned_heuristics` has 0 rows

**Fix Required:**
```go
// In handlers_task.go, on successful task completion:
func (h *TaskHandler) recordSuccess(...) {
    // ... existing code ...
    
    // Record heuristic for future routing
    h.database.RPC(ctx, "upsert_heuristic", map[string]any{
        "p_task_type": taskType,
        "p_condition": map[string]any{},
        "p_preferred_model": modelID,
        "p_confidence": 0.8,
        "p_source": "success_record",
    })
}
```

---

### 7. Problem-Solutions Never Recorded

**Location:** No code calls `record_solution_result`
**Problem:** System doesn't learn what fixes work for what problems
**Impact:** Same failures repeat, no automatic remediation

**Table Status:** `problem_solutions` has 0 rows

**Fix Required:**
```go
// When a retry succeeds after initial failure:
func (h *TaskHandler) handleRetrySuccess(ctx context.Context, taskID, originalFailure, modelID string) {
    h.database.RPC(ctx, "record_solution_result", map[string]any{
        "p_failure_type": originalFailure,
        "p_task_type": taskType,
        "p_solution_type": "model_switch",
        "p_solution_model": modelID,
        "p_success": true,
    })
}
```

---

## 🟢 WORKING CORRECTLY

### Dependency Unlocking
**Location:** `handlers_testing.go:250`, `schema_dependency_rpc.sql`
**Status:** ✅ Working
- `unlock_dependent_tasks` RPC exists and is called after task merge
- Updates dependent tasks from "pending" to "available"

### Learning Context Injection
**Location:** `runtime/context_builder.go`, `runtime/session.go:158-168`
**Status:** ✅ Working
- Planner rules fetched and injected into planner prompt
- Supervisor rules fetched and injected into supervisor prompt
- Tester rules fetched and injected into tester prompt
- Recent failures fetched and injected into planner prompt

### Failure Recording
**Location:** `handlers_task.go`, `handlers_testing.go`
**Status:** ✅ Working
- `failure_records` table has 332 rows
- `record_model_failure` called on all failures
- `append_failure_notes` records detailed reasons

### Supervisor Rules (Partial)
**Location:** `supervisor_learned_rules` table
**Status:** ⚠️ Partially Working
- Table has 42 rows (being created)
- Rules fetched for context injection
- But NOT created from all rejection sources

---

## 📋 TABLE AUDIT

### Actively Used Tables (8)

| Table | Purpose | Status |
|-------|---------|--------|
| `tasks` | Core task storage | ✅ Active |
| `task_runs` | Execution records | ✅ Active |
| `plans` | PRD plans | ✅ Active |
| `test_results` | Test outcomes | ✅ Active |
| `research_suggestions` | Research queue | ✅ Active |
| `maintenance_commands` | Admin commands | ✅ Active |
| `models` | Model registry | ✅ Active |
| `platforms` | Web platforms | ✅ Active |

### Learning Tables (6)

| Table | Rows | Status | Issue |
|-------|------|--------|-------|
| `supervisor_learned_rules` | 42 | ✅ Working | Rules being created |
| `failure_records` | 332 | ✅ Working | Failures recorded |
| `learned_heuristics` | 0 | ⚠️ Empty | No code creates heuristics |
| `lessons_learned` | 0 | ⚠️ Empty | Not populated |
| `tester_learned_rules` | 0 | ⚠️ Empty | No code creates tester rules |
| `problem_solutions` | 0 | ⚠️ Empty | No code records solutions |

### Tables Used by Dashboard (Verify Before Removal)

| Table | Dashboard Usage | Recommendation |
|-------|-----------------|----------------|
| `models_new` | May be staging for new models | **CHECK DASHBOARD** |
| `model_registry` | May be alternate model source | **CHECK DASHBOARD** |
| `runners` | May track active runners | **CHECK DASHBOARD** |
| `runner_sessions` | May track sessions | **CHECK DASHBOARD** |
| `roi_dashboard` | May show ROI stats | **CHECK DASHBOARD** |
| `slice_roi` | May show per-slice ROI | **CHECK DASHBOARD** |
| `orchestrator_events` | May show event timeline | **CHECK DASHBOARD** |

### Legacy/Unused Tables (35)

These tables exist but are NOT referenced in governor code:

```
access, agent_messages, agent_tasks, chat_queue,
council_reviews, event_queue, exchange_rates,
hardened_prd, lane_locks, platform_health,
project_drafts, project_structure, projects, prompts,
secrets_vault, security_audit, skills, system_config,
task_backlog, task_checkpoints, task_history, task_packets,
tools, vibes_conversations, vibes_ideas, vibes_preferences
```

**Note:** Some may be used by dashboard or future features. Verify before dropping.

---

## 🔌 RPC AUDIT

### Top 10 Most Called RPCs

| RPC | Calls | Purpose |
|-----|-------|---------|
| `update_task_status` | 27 | Status transitions |
| `clear_processing` | 17 | Lock cleanup |
| `update_plan_status` | 12 | Plan status |
| `set_processing` | 12 | Lock acquisition |
| `update_research_suggestion_status` | 10 | Research flow |
| `find_tasks_with_checkpoints` | 10 | Recovery |
| `record_model_success` | 5 | Learning |
| `record_model_failure` | 5 | Learning |
| `update_maintenance_command_status` | 4 | Admin |
| `delete_checkpoint` | 3 | Cleanup |

### Learning RPCs Status

| RPC | In Allowlist | Actually Called | Creates Data? |
|-----|--------------|-----------------|---------------|
| `get_planner_rules` | ✅ | ✅ context_builder.go | Reads existing |
| `get_supervisor_rules` | ✅ | ✅ context_builder.go | Reads existing |
| `get_tester_rules` | ✅ | ✅ context_builder.go | Reads existing |
| `get_recent_failures` | ✅ | ✅ context_builder.go | Reads existing |
| `get_heuristic` | ✅ | ✅ context_builder.go | Reads (empty table) |
| `get_problem_solution` | ✅ | ✅ context_builder.go | Reads (empty table) |
| `create_planner_rule` | ✅ | ✅ handlers_council.go | ✅ Yes |
| `create_supervisor_rule` | ✅ | ❌ NOT CALLED | ❌ No |
| `create_tester_rule` | ✅ | ❌ NOT CALLED | ❌ No |
| `record_supervisor_rule` | ✅ | ❌ NOT CALLED | ❌ No |
| `record_tester_rule_hit` | ✅ | ❌ NOT CALLED | ❌ No |
| `upsert_heuristic` | ✅ | ❌ NOT CALLED | ❌ No |
| `record_heuristic_result` | ✅ | ❌ NOT CALLED | ❌ No |
| `record_solution_result` | ✅ | ❌ NOT CALLED | ❌ No |

### Missing from Allowlist

| RPC | Status |
|-----|--------|
| `deactivate_supervisor_rule` | ❌ Missing |
| `deactivate_tester_rule` | ❌ Missing |

---

## 📈 DASHBOARD ALIGNMENT (Session 73 Fixes)

### What Dashboard Expects

| Field | Table | Source | Status |
|-------|-------|--------|--------|
| `result.prompt_packet` | tasks | RPC writes to result | ✅ Fixed (079) |
| `slice_id` | tasks | Planner sets | ✅ Fixed (validation.go) |
| `assigned_to` | tasks | Router sets | ✅ Working |
| `routing_flag` | tasks | Router sets | ✅ Working |
| `failure_notes` | tasks | Handlers record | ✅ Fixed (testing) |
| `status` | tasks | Handlers update | ✅ Working |

### Status Value Mapping

| Governor Status | Dashboard Maps To | Trigger Event |
|-----------------|--------------------|---------------|
| `available` | `"pending"` | EventTaskAvailable |
| `in_progress` | `"in_progress"` | - |
| `review` | `"in_progress"` | EventTaskReview |
| `testing` | `"in_progress"` | EventTaskTesting (FIXED) |
| `approval` | `"supervisor_approval"` | EventTaskCompleted |
| `merged` | `"complete"` | - |
| `pending` (retry) | `"pending"` | EventTaskAvailable |

---

## 🔧 FIX PRIORITY ORDER

### Priority 1: Module Branch Creation
**Why:** Without this, tasks can never properly merge
**Impact:** Entire flow breaks at merge step
**Effort:** Medium
**Files:** `handlers_plan.go`

### Priority 2: Supervisor Rule Creation
**Why:** System doesn't learn from rejections
**Impact:** Repeated mistakes, no improvement
**Effort:** Low
**Files:** `handlers_task.go`

### Priority 3: Tester Rule Creation
**Why:** System doesn't learn from test failures
**Impact:** Same bugs repeat
**Effort:** Low
**Files:** `handlers_testing.go`

### Priority 4: Heuristic Recording
**Why:** Router doesn't learn model preferences
**Impact:** Suboptimal model selection
**Effort:** Low
**Files:** `handlers_task.go`

### Priority 5: Test Results Persistence
**Why:** No audit trail of test outcomes
**Impact:** Can't analyze test patterns
**Effort:** Low
**Files:** `handlers_testing.go`

### Priority 6: Token Extraction
**Why:** ROI shows $0
**Impact:** No cost tracking
**Effort:** Medium
**Files:** `connectors/runners.go`

### Priority 7: Schema Consolidation
**Why:** 95 migrations is unmaintainable
**Impact:** Confusion, hard to debug
**Effort:** High
**Files:** All migration files

---

## 📁 FILES MODIFIED SESSION 73

1. `docs/supabase-schema/079_dashboard_alignment.sql` - NEW migration
2. `governor/internal/realtime/client.go` - Fixed testing event
3. `governor/cmd/governor/handlers_testing.go` - Added failure notes
4. `governor/cmd/governor/validation.go` - Added slice_id parsing
5. `vibeflow/src/core/types.ts` - Added failureNotes field
6. `vibeflow/apps/dashboard/lib/vibepilotAdapter.ts` - Added failure_notes

---

## 🔗 RELATED DOCS

- [SUPABASE_AUDIT_2026-03-09.md](SUPABASE_AUDIT_2026-03-09.md) - Raw audit data
- [HOW_DASHBOARD_WORKS.md](HOW_DASHBOARD_WORKS.md) - Dashboard data expectations
- [DATA_FLOW_MAPPING.md](DATA_FLOW_MAPPING.md) - Data flow details
- [VIBEPILOT_WHAT_YOU_NEED_TO_KNOW.md](VIBEPILOT_WHAT_YOU_NEED_TO_KNOW.md) - System overview
