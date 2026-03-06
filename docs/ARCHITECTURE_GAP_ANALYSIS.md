# VibePilot Architecture Gap Analysis

**Purpose:** Clear comparison of what SHOULD happen vs what code DOES. No guessing.

---

## 1. Complete Flow: What SHOULD Happen

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ PHASE 1: PRD CREATION                                                        │
│                                                                              │
│ Human → pushes PRD to GitHub → docs/prd/my-feature.md                       │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ PHASE 2: PLAN CREATION                                                       │
│                                                                              │
│ Governor detects PRD (Supabase Live)                                        │
│     │                                                                        │
│     ▼                                                                        │
│ Planner reads PRD                                                            │
│     │                                                                        │
│     ▼                                                                        │
│ Planner creates:                                                             │
│   - Plan file (docs/plans/my-feature-plan.md)                               │
│   - Tasks (Supabase tasks table)                                            │
│   - Prompt packets (Supabase task_packets table)                            │
│   - Dependencies between tasks                                               │
│   - Slice assignments                                                        │
│   - Confidence scores per task                                               │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ PHASE 3: SUPERVISOR REVIEW (Quality Control)                                 │
│                                                                              │
│ Supervisor reads BOTH:                                                       │
│   - PRD (from prd_path)                                                      │
│   - Plan (from plan_path)                                                    │
│                                                                              │
│ Supervisor validates:                                                        │
│   ✓ Plan matches PRD intent (full alignment)                                │
│   ✓ Tasks broken down to 95% confidence                                     │
│   ✓ Prompt packets complete and self-contained                              │
│   ✓ Modules organized per principle strategy                                │
│   ✓ No circular dependencies                                                 │
│   ✓ Internal flags where required                                            │
│                                                                              │
│ Decision:                                                                    │
│   - approved → Tasks become available                                        │
│   - needs_revision → Back to planner with feedback                          │
│   - council_review → Complex plan, route to council                         │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ PHASE 4: TASK ORCHESTRATION                                                  │
│                                                                              │
│ For each available task:                                                     │
│   1. Router selects model (writes to tasks.assigned_to)                     │
│   2. Router sets routing_flag (internal/mcp/web)                            │
│   3. Task runner executes via CLI/API/Web                                    │
│   4. Output committed to task branch                                         │
│   5. task_runs record created with tokens/costs                             │
│   6. Status → review                                                         │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ PHASE 5: OUTPUT REVIEW                                                       │
│                                                                              │
│ Supervisor reviews task output:                                              │
│   ✓ All deliverables present                                                │
│   ✓ Tests written                                                            │
│   ✓ No hardcoded secrets                                                     │
│   ✓ Output format matches expected                                          │
│                                                                              │
│ Decision:                                                                    │
│   - pass → testing                                                           │
│   - fail → return to runner with feedback                                   │
│   - reroute → different model                                                │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ PHASE 6: TESTING                                                             │
│                                                                              │
│ Tester runs:                                                                 │
│   - Unit tests                                                               │
│   - Lint                                                                     │
│   - Typecheck                                                                │
│                                                                              │
│ Result:                                                                      │
│   - passed → approval (human review)                                        │
│   - failed → back to runner                                                  │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ PHASE 7: HUMAN REVIEW & MERGE                                                │
│                                                                              │
│ Dashboard shows task in review queue                                         │
│ Human reviews diff                                                            │
│ Human approves/rejects                                                        │
│ If approved → merge to main → status = merged                               │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ PHASE 8: LEARNING (Self-Improvement)                                         │
│                                                                              │
│ System learns from:                                                          │
│   - Successes (what worked)                                                  │
│   - Failures (what didn't)                                                   │
│   - Revisions (why needed)                                                   │
│   - Routing decisions (which model for which task type)                     │
│                                                                              │
│ Updates:                                                                     │
│   - Model success rates                                                      │
│   - Supervisor rules                                                         │
│   - Routing preferences                                                      │
│   - Failure patterns to avoid                                                │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 2. What Code ACTUALLY Does

### handlers_plan.go

| Event | What Code Does | Status |
|-------|---------------|--------|
| `EventPRDReady` | Planner creates plan | ✅ Works |
| `EventPlanCreated` | Planner creates plan → Supervisor reviews | ✅ Works |
| `EventPlanReview` | Supervisor reviews plan | ✅ Works |
| `EventRevisionNeeded` | Planner revises plan | ✅ Works |

**Supervisor receives:**
```go
updatedPlan := map[string]any{
    "id":           planID,
    "prd_path":     plan["prd_path"],      // ✅ PRD path passed
    "plan_path":    plannerOutput.PlanPath, // ✅ Plan path passed
    "status":       newStatus,
    "plan_content": plannerOutput.PlanContent,
}
```

**Supervisor prompt says:**
```
1. Read plan from plan.plan_path
2. Read PRD from plan.prd_path
3. Validate each task
```

✅ **Plan review is WIRED CORRECTLY**

---

### handlers_task.go

| Event | What Code Does | Status |
|-------|---------------|--------|
| `EventTaskAvailable` | Claims task, routes, executes | ⚠️ Partial |
| `EventTaskReview` | Supervisor reviews output | ✅ Works |
| `EventTaskCompleted` | Supervisor reviews, merges | ✅ Works |

**EventTaskAvailable Issues:**

| Step | Code | Issue |
|------|------|-------|
| Claim task | `set_processing` RPC | ✅ Works |
| Load packet | `GetTaskPacket` | ✅ Works |
| Create branch | `git.CreateBranch` | ✅ Works |
| Route | `SelectDestination` | ✅ Works |
| **Write assigned_to** | `update_task_assignment` | ✅ Works |
| **Write routing_flag** | ❌ NOT CALLED | **MISSING** |
| Execute | `session.Run` | ✅ Works |
| Commit output | `git.CommitOutput` | ✅ Works |
| **Create task_runs** | `database.Insert` | ❌ **FAILS - column mismatch** |
| Set status | `update_task_status` | ✅ Works |

---

## 3. Critical Gaps

### Gap 1: task_runs Schema Mismatch

**Schema (schema_v1_core.sql):**
```sql
CREATE TABLE task_runs (
  id UUID PRIMARY KEY,
  task_id UUID,
  courier TEXT NOT NULL,
  platform TEXT NOT NULL,
  model_id TEXT,
  status TEXT,
  tokens_used INT,           -- ✅ Only this
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ
);
```

**Go code tries to insert:**
```go
"tokens_in":    tokensIn,    // ❌ COLUMN DOESN'T EXIST
"tokens_out":   tokensOut,   // ❌ COLUMN DOESN'T EXIST
```

**Dashboard expects:**
```typescript
tokens_in, tokens_out,           // ❌ NOT IN SCHEMA
courier_tokens,                  // ❌ NOT IN SCHEMA
courier_cost_usd,                // ❌ NOT IN SCHEMA
platform_theoretical_cost_usd,   // ❌ NOT IN SCHEMA
total_actual_cost_usd,           // ❌ NOT IN SCHEMA
total_savings_usd                // ❌ NOT IN SCHEMA
```

**Fix:** Update schema to add missing columns OR update Go code to only use existing columns.

---

### Gap 2: routing_flag Not Written

**handlers_task.go line 138-147:**
```go
_, err = database.RPC(ctx, "update_task_assignment", map[string]any{
    "p_task_id":     taskID,
    "p_status":      "in_progress",
    "p_assigned_to": modelID,
    // ❌ routing_flag NOT SET HERE
})
```

**Dashboard expects:**
```typescript
// vibepilotAdapter.ts line 202-206
location: deriveTaskLocation(
    task.routing_flag,  // ← NEEDS THIS
    task.assigned_to,
    run?.platform || null
)
```

**Fix:** Add `routing_flag` to `update_task_assignment` RPC call.

---

### Gap 3: Learning System Not Wired

**Schema exists:**
- `schema_v1.4_roi_enhanced.sql` - ROI tracking
- `schema_intelligence.sql` - Learning system
- `schema_council_rpc.sql` - Council review

**But NOT wired in handlers:**
- No `record_model_success` calls
- No `record_model_failure` calls
- No learning from revisions
- No routing preference updates

**Fix:** Wire learning RPCs into handlers.

---

## 4. What's Working

| Component | Status | Notes |
|-----------|--------|-------|
| PRD detection | ✅ | Supabase Live |
| Plan creation | ✅ | Planner creates plan + tasks |
| Supervisor review | ✅ | Reads PRD + plan, validates |
| Task claiming | ✅ | Atomic, race-condition safe |
| Model routing | ✅ | SelectDestination works |
| Task execution | ✅ | CLI/API runners work |
| Token extraction | ✅ | CLI returns tokens_in, tokens_out |
| Output commit | ✅ | Git commit works |
| Status updates | ✅ | RPCs work |
| Dashboard display | ✅ | Reads from Supabase |

---

## 5. What's Broken

| Component | Issue | Impact |
|-----------|-------|--------|
| task_runs insert | Column mismatch | No execution history |
| routing_flag | Not written | Dashboard shows "Web" instead of "VibePilot" |
| Learning system | Not wired | No self-improvement |

---

## 6. Priority Fix Order

### Priority 1: Fix task_runs (CRITICAL)

**Option A: Update Schema**
```sql
ALTER TABLE task_runs ADD COLUMN tokens_in INT;
ALTER TABLE task_runs ADD COLUMN tokens_out INT;
ALTER TABLE task_runs ADD COLUMN courier_tokens INT;
ALTER TABLE task_runs ADD COLUMN courier_cost_usd DECIMAL(10,4);
ALTER TABLE task_runs ADD COLUMN platform_theoretical_cost_usd DECIMAL(10,4);
ALTER TABLE task_runs ADD COLUMN total_actual_cost_usd DECIMAL(10,4);
ALTER TABLE task_runs ADD COLUMN total_savings_usd DECIMAL(10,4);
```

**Option B: Update Go Code**
```go
// Only use existing columns
_, err = database.Insert(ctx, "task_runs", map[string]any{
    "task_id":      taskID,
    "model_id":     modelID,
    "courier":      destID,
    "platform":     connectorType,
    "tokens_used":  tokensIn + tokensOut,  // Combine
    "status":       "success",
    "started_at":   runStartTime,
    "completed_at": time.Now(),
})
```

---

### Priority 2: Fix routing_flag

**handlers_task.go line 138:**
```go
_, err = database.RPC(ctx, "update_task_assignment", map[string]any{
    "p_task_id":     taskID,
    "p_status":      "in_progress",
    "p_assigned_to": modelID,
    "p_routing_flag": connectorType == "cli" ? "internal" : "web",  // ADD THIS
})
```

**Also update RPC:**
```sql
CREATE OR REPLACE FUNCTION update_task_assignment(
    p_task_id UUID,
    p_status TEXT,
    p_assigned_to TEXT,
    p_routing_flag TEXT DEFAULT 'internal'  -- ADD THIS
) ...
```

---

### Priority 3: Wire Learning System

**After task success:**
```go
_, err = database.RPC(ctx, "record_model_success", map[string]any{
    "p_model_id":  modelID,
    "p_task_type": taskType,
    "p_duration":  duration,
})
```

**After task failure:**
```go
_, err = database.RPC(ctx, "record_model_failure", map[string]any{
    "p_model_id":  modelID,
    "p_task_id":   taskID,
    "p_failure_type": failureType,
})
```

---

## 7. Summary

**Working:** Plan creation, supervisor review, task execution, status updates
**Broken:** task_runs insert (schema mismatch), routing_flag not written, learning not wired

**Next Steps:**
1. Choose Option A or B for task_runs fix
2. Add routing_flag to update_task_assignment
3. Wire learning RPCs
