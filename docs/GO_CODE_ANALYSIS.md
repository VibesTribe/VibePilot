# VibePilot Go Code: Complete Analysis & Rewrite Plan

## Current State: DISFUNCTIONAL

**What we see:**
- Task shows "assigned" AND "pending" simultaneously
- Status in Supabase = "review" but task never executed
- No task_runs created
- Dashboard confused because data is wrong

**Root cause:** Go code is disconnected spaghetti with gaps, not aligned with VibePilot design.

---

## Part 1: What VibePilot SHOULD Do (Design)

### Complete Flow

```
1. PRD PUSHED TO GITHUB
   ↓
2. Governor detects via webhook
   ↓
3. Creates plan record (status=draft)
   ↓
4. Planner reads PRD, creates plan with tasks
   ↓
5. Supervisor reviews plan (reads PRD + plan)
   ↓
6. If approved:
   - Create tasks in Supabase (status=available)
   - Create task_packets
   ↓
7. Router selects model for each task
   - Writes to tasks.assigned_to
   - Writes to tasks.routing_flag
   ↓
8. Task runner executes
   - Claims task atomically
   - Sets status=in_progress
   - Runs via CLI/API
   - Commits output to branch
   ↓
9. Create task_runs record
   - model_id
   - tokens_in, tokens_out
   - costs
   ↓
10. Set status=review
    ↓
11. Supervisor reviews output
    ↓
12. If pass → testing → approval → merge
    If fail → back to runner with feedback
```

### Key Principles

1. **One status at a time** - Task can only be in ONE status
2. **Status progression is linear** - available → in_progress → review → testing → approval → merged
3. **Every action writes to database** - No orphaned state
4. **Dashboard is READ-ONLY** - It displays what's in Supabase
5. **Routing is dynamic** - Based on task needs + available resources
6. **Models wear hats** - Same model can be planner, supervisor, task_runner
7. **Web platforms are destinations** - Couriers go there, not models

---

## Part 2: What Go Code ACTUALLY Does

### File Analysis

| File | Purpose | Current State |
|------|---------|---------------|
| `main.go` | Bootstrap, realtime subscriptions | ✅ Works |
| `handlers_plan.go` | Plan creation, supervisor review | ⚠️ Partial |
| `handlers_task.go` | Task execution | ❌ BROKEN |
| `validation.go` | Task parsing, creation | ⚠️ Partial |
| `router.go` | Model/destination selection | ⚠️ Confused |
| `session.go` | CLI/API execution | ✅ Works |
| `runners.go` | CLI runners | ✅ Works |

### handlers_task.go Problems

**EventTaskAvailable:**
```
1. Claims task ✅
2. Loads packet ✅
3. Creates branch ✅
4. Routes to connector ✅
5. Calls update_task_assignment ❌ FAILS (RPC not in allowlist??)
6. Submits to pool ✅
7. Pool executes ✅
8. Commits output ⚠️ (sometimes fails)
9. Creates task_runs ❌ FAILS (RPC not in allowlist??)
10. Sets status ❌ WRONG STATUS
```

**Actual logs show:**
- "Failed to update status to in_progress: RPC update_task_assignment not in allowlist"
- "Warning: Failed to create task_run record: RPC create_task_run not in allowlist"
- "Failed to commit output to task/T001: files must be an array"
- "Task e835ed3e output committed, status=error"

**But RPCs ARE in allowlist:**
```go
var defaultRPCAllowlist = map[string]bool{
    "update_task_assignment":  true,  // ← IT'S RIGHT THERE
    "create_task_run":         true,  // ← IT'S RIGHT THERE
```

**The bug:** RPC allowlist check is failing even though RPCs are in default list. Need to trace why.

### Status Confusion

**Dashboard expects:**
```typescript
const ACTIVE_STATUSES = ["assigned", "in_progress", "received", "testing"];
const PENDING_STATUSES = ["assigned", "blocked"];
```

**Go code sets:**
- After routing: status="in_progress" (but fails due to RPC issue)
- After execution: status="review"
- On error: status="error" (but "error" isn't a valid status!)

**Supabase has:**
- status="review" (set somehow despite failures)
- But task never actually executed

### The RPC Allowlist Mystery

**Code:**
```go
func (a *RPCAllowlist) Allowed(name string) bool {
    a.mu.RLock()
    defer a.mu.RUnlock()
    return a.names[name] || defaultRPCAllowlist[name]
}
```

**Called from:**
```go
func (d *DB) RPC(ctx context.Context, name string, params map[string]interface{}) ([]byte, error) {
    if !d.rpcAllowlist.Allowed(name) {
        return nil, fmt.Errorf("RPC %s not in allowlist", name)
    }
    return d.rpc(ctx, name, params)
}
```

**The check:**
- `a.names[name]` - empty map (nothing added)
- `defaultRPCAllowlist[name]` - should be true

**Why failing:** Need to verify `defaultRPCAllowlist` is actually being used. Maybe DB has its own allowlist instance that's not using defaults?

---

## Part 3: Gap Analysis

| What Should Happen | What Actually Happens | Gap |
|--------------------|----------------------|-----|
| Task status = available | ✅ Works | None |
| Router selects model | ✅ Works | None |
| Write assigned_to | ❌ RPC fails | RPC allowlist bug |
| Set status = in_progress | ❌ RPC fails | RPC allowlist bug |
| Execute task | ✅ Works | None |
| Commit output | ⚠️ Sometimes fails | "files must be an array" |
| Create task_runs | ❌ RPC fails | RPC allowlist bug |
| Set status = review | ❌ Set even on failure | Logic error |
| Dashboard shows correct status | ❌ Shows wrong | Data is wrong |

---

## Part 4: Root Causes

### 1. RPC Allowlist Bug (CRITICAL)

**Hypothesis:** DB instance has its own allowlist that's not initialized with defaults.

**Fix:** Verify `NewRPCAllowlist()` is being used OR pass defaults to it.

### 2. Status Logic Bug (HIGH)

**Problem:** Status set to "review" even when execution fails.

**Fix:** Only set "review" if:
- Execution succeeded
- Commit succeeded
- task_runs created

### 3. Git Commit Bug (MEDIUM)

**Problem:** "files must be an array"

**Fix:** Ensure files field is always an array.

### 4. No Error Recovery (MEDIUM)

**Problem:** When RPC fails, task stuck in limbo.

**Fix:** Proper error handling with retry or status reset.

---

## Part 5: Rewrite Plan

### Phase 1: Fix Critical Bugs (TODAY)

**Task 1: Fix RPC Allowlist**
- Trace why defaultRPCAllowlist isn't being used
- Fix it
- Test: verify update_task_assignment works

**Task 2: Fix Status Logic**
- Only set "review" on success
- Set "error" → "available" for retry
- Handle all failure cases

**Task 3: Fix Git Commit**
- Ensure files is always array
- Handle missing files gracefully

### Phase 2: Clean Up Handlers (NEXT SESSION)

**Task 4: Rewrite handlers_task.go**
- Single function per event
- Clear status progression
- All RPC calls wrapped with error handling
- Learning system calls

**Task 5: Rewrite router.go**
- Clear separation: models vs destinations
- Web platforms = destinations for couriers
- Models = wear hats (roles)

### Phase 3: Add Missing Features (FUTURE)

**Task 6: Wire Learning System**
- record_model_success after pass
- record_model_failure after fail
- Update routing preferences

**Task 7: Add Recovery Logic**
- Detect stuck tasks
- Auto-retry or alert
- Clean up orphaned state

---

## Part 6: Immediate Action Items

### Right Now:

1. **Find RPC allowlist bug** - Why is it failing?
2. **Fix it** - Make RPCs work
3. **Test end-to-end** - Verify task completes

### This Session:

1. Fix all 3 critical bugs
2. Verify hello world task completes
3. Update documentation

### Next Session:

1. Rewrite handlers_task.go (clean, not patch)
2. Rewrite router.go (clear model/destination separation)
3. Add comprehensive error handling

---

## Part 7: Success Criteria

**After fixes, this should work:**

1. Push PRD to GitHub
2. Plan created automatically
3. Supervisor approves
4. Task created with status=available
5. Router assigns model (assigned_to set)
6. Status changes to in_progress
7. Task executes
8. Output committed
9. task_runs created with tokens
10. Status changes to review
11. Dashboard shows: task in review, model assigned, tokens counted

**Time for hello world:** < 60 seconds from PRD push to task completion

---

## Part 8: Questions for Human

1. Should we fix bugs first OR rewrite from scratch?
2. Is there existing code we should preserve?
3. Any other critical issues I missed?
4. Priority: speed (patch) vs quality (rewrite)?
