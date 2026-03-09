# VibePilot Supabase Audit Report
**Date:** 2026-03-09
**Session:** 73

---

## 📊 SUMMARY

| Category | Count | Status |
|----------|-------|--------|
| **Tables** | 49 | Many legacy/unused |
| **RPCs** | 121 | Many in allowlist, few actually called |
| **Migrations** | 95 files | Need consolidation |

---

## 🗄️ TABLES AUDIT

### ✅ ACTIVELY USED (8 tables)

| Table | Rows | Used By | Status |
|-------|------|---------|--------|
| `tasks` | 0 | Governor core | ✅ Active |
| `task_runs` | 0 | Governor core | ✅ Active |
| `plans` | 0 | Governor core | ✅ Active |
| `test_results` | 0 | Testing handler | ✅ Active |
| `research_suggestions` | 0 | Research flow | ✅ Active |
| `maintenance_commands` | 0 | Admin commands | ✅ Active |
| `models` | ? | Router | ✅ Active |
| `platforms` | ? | Router | ✅ Active |

### 📚 LEARNING TABLES (6 tables)

| Table | Rows | Status | Issue |
|-------|------|--------|-------|
| `supervisor_learned_rules` | 42 | ✅ Working | Rules being created |
| `failure_records` | 332 | ✅ Working | Failures being recorded |
| `learned_heuristics` | 0 | ⚠️ Empty | No heuristics being created |
| `lessons_learned` | 0 | ⚠️ Empty | Not being populated |
| `tester_learned_rules` | 0 | ⚠️ Empty | No tester rules being created |
| `problem_solutions` | 0 | ⚠️ Empty | No solutions being recorded |

### 🗑️ LEGACY/UNUSED TABLES (35 tables)

These tables exist but are NOT referenced in governor code:

```
access, agent_messages, agent_tasks, chat_queue, 
council_reviews, event_queue, exchange_rates, 
hardened_prd, lane_locks, model_registry, models_new, 
orchestrator_events, platform_health, problem_solutions,
project_drafts, project_structure, projects, prompts, 
roi_dashboard, runner_sessions, runners, secrets_vault, 
security_audit, skills, slice_roi, system_config, 
task_backlog, task_checkpoints, task_history, task_packets,
tools, vibes_conversations, vibes_ideas, vibes_preferences
```

**Note:** Some may be used by dashboard or other services.

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

| RPC | In Allowlist | Actually Called | Status |
|-----|--------------|-----------------|--------|
| `get_planner_rules` | ✅ | ✅ context_builder.go | Working |
| `get_supervisor_rules` | ✅ | ✅ context_builder.go | Working |
| `get_tester_rules` | ✅ | ✅ context_builder.go | Working |
| `get_recent_failures` | ✅ | ✅ context_builder.go | Working |
| `get_heuristic` | ✅ | ✅ context_builder.go | Working |
| `get_problem_solution` | ✅ | ✅ context_builder.go | Working |
| `create_planner_rule` | ✅ | ✅ handlers_council.go | Working |
| `create_supervisor_rule` | ✅ | ❌ NOT CALLED | **BROKEN** |
| `create_tester_rule` | ✅ | ❌ NOT CALLED | **BROKEN** |
| `record_supervisor_rule` | ✅ | ❌ NOT CALLED | **BROKEN** |
| `record_tester_rule_hit` | ✅ | ❌ NOT CALLED | **BROKEN** |
| `upsert_heuristic` | ✅ | ❌ NOT CALLED | **BROKEN** |
| `record_heuristic_result` | ✅ | ❌ NOT CALLED | **BROKEN** |
| `record_solution_result` | ✅ | ❌ NOT CALLED | **BROKEN** |

---

## 🐛 CRITICAL ISSUES FOUND

### 1. Module Branches Never Created
**File:** `governor/internal/gitree/gitree.go:387`
**Problem:** `CreateModuleBranch()` exists but is NEVER CALLED
**Impact:** Task branches have nowhere to merge to
**Fix:** Call `CreateModuleBranch()` when first task in a slice is created

### 2. Supervisor Rules Not Created on Rejection
**File:** `governor/cmd/governor/handlers_task.go:427-433`
**Problem:** Supervisor rejection records failure but doesn't create learning rule
**Impact:** System doesn't learn from rejections
**Fix:** Call `create_supervisor_rule` when supervisor rejects

### 3. Tester Rules Never Created
**Problem:** No code calls `create_tester_rule`
**Impact:** System doesn't learn from test failures
**Fix:** Call `create_tester_rule` when tests catch bugs

### 4. Heuristics Never Created
**Problem:** No code calls `upsert_heuristic`
**Impact:** Router doesn't learn model preferences
**Fix:** Call `upsert_heuristic` on successful task completions

### 5. Problem-Solutions Never Recorded
**Problem:** No code calls `record_solution_result`
**Impact:** System doesn't learn what fixes work
**Fix:** Call `record_solution_result` when retry succeeds

---

## 📋 LEARNING FLOW (What Should Happen)

### Planner Learning
```
1. PRD → Plan created
2. Supervisor reviews plan
3. IF rejected:
   - Create planner_rule with what went wrong
   - Store in planner_learned_rules
4. Next plan:
   - Load planner_rules via context_builder
   - Inject into planner prompt
```

### Supervisor Learning
```
1. Task completes → Supervisor reviews
2. IF issues found:
   - Create supervisor_rule with issue pattern
   - Store in supervisor_learned_rules
3. Next review:
   - Load supervisor_rules via context_builder
   - Inject into supervisor prompt
```

### Tester Learning
```
1. Tests run → Bug found
2. Create tester_rule with bug pattern
3. Store in tester_learned_rules
4. Next test:
   - Load tester_rules via context_builder
   - Inject into tester prompt
```

### Router Learning (Heuristics)
```
1. Task assigned to model
2. Task succeeds/fails
3. Record heuristic: task_type → preferred_model
4. Next routing:
   - Load heuristics via context_builder
   - Prefer successful models
```

---

## 🔧 RECOMMENDED FIXES

### Priority 1: Module Branch Creation
```go
// In handlers_plan.go, after tasks created:
for _, sliceID := range uniqueSliceIDs {
    if err := git.CreateModuleBranch(ctx, sliceID); err != nil {
        log.Printf("[Plan] Failed to create module branch %s: %v", sliceID, err)
    }
}
```

### Priority 2: Supervisor Rule Creation
```go
// In handlers_task.go, on supervisor rejection:
if decision.Decision == "reject" {
    h.database.RPC(ctx, "create_supervisor_rule", map[string]any{
        "p_trigger_pattern": "task_review",
        "p_rule_text": decision.Issues[0].Description,
        "p_action": "flag_for_review",
        "p_source": "supervisor_rejection",
        "p_source_task_id": taskID,
    })
}
```

### Priority 3: Tester Rule Creation
```go
// In handlers_testing.go, on test failure:
h.database.RPC(ctx, "create_tester_rule", map[string]any{
    "p_trigger_pattern": "test_execution",
    "p_rule_text": fmt.Sprintf("Watch for: %s", testOutput.NextAction),
    "p_action": "flag_for_fix",
    "p_source": "test_failure",
    "p_source_task_id": taskID,
})
```

### Priority 4: Heuristic Recording
```go
// In handlers_task.go, on task success:
h.database.RPC(ctx, "upsert_heuristic", map[string]any{
    "p_task_type": taskType,
    "p_condition": map[string]any{},
    "p_preferred_model": modelID,
    "p_confidence": 0.8,
    "p_source": "success_record",
})
```

---

## 🧹 CLEANUP RECOMMENDATIONS

### Tables to Consider Dropping (after verification)
- `agent_tasks` - Replaced by `tasks`
- `models_new` - Migration leftover
- `model_registry` - Replaced by `models`
- `task_backlog` - Not used
- `task_history` - Not used
- `chat_queue` - Not used
- `lane_locks` - Not used

### RPCs to Remove from Allowlist (not called)
- `claim_next_task` - Old claiming logic
- `claim_next_command` - Not used
- `check_circular_deps` - Not used
- `check_dependencies_complete` - Not used

---

## 📈 NEXT STEPS

1. **Fix module branch creation** - Critical for merge flow
2. **Add supervisor rule creation** - Enable supervisor learning
3. **Add tester rule creation** - Enable tester learning
4. **Add heuristic recording** - Enable router learning
5. **Test end-to-end learning flow**
6. **Consider schema consolidation** - 95 migrations is too many
