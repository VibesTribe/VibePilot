# VibePilot Go Code: Complete Logic Map

## The Problem

**Time to create a simple task:** 2+ minutes
**Time to execute hello world:** Never completes
**Status:** Shows "active", "pending", "assigned" all at once
**Root cause:** Go code is broken, disconnected, and doesn't match VibePilot design

---

## Part 1: File Structure

### Entry Points

| File | Purpose | Status |
|------|---------|--------|
| `cmd/governor/main.go` | Bootstrap, realtime subscriptions, event routing | ✅ Works |
| `cmd/governor/handlers_*.go` | Event handlers for plans, tasks, testing, etc. | ❌ BROKEN |
| `cmd/governor/validation.go` | Task parsing, creation | ⚠️ Partial |
| `cmd/governor/recovery.go` | Startup recovery | ✅ Works |

### Core Runtime

| File | Purpose | Status |
|------|---------|--------|
| `internal/runtime/router.go` | Model/destination selection | ⚠️ Confused |
| `internal/runtime/session.go` | Execution session | ✅ Works |
| `internal/runtime/config.go` | Config loading | ✅ Works |
| `internal/runtime/events.go` | Event types | ✅ Works |
| `internal/runtime/parallel.go` | Pool management | ✅ Works |

### Connectors

| File | Purpose | Status |
|------|---------|--------|
| `internal/connectors/runners.go` | CLI runners (opencode, kilo) | ✅ Works |
| `internal/connectors/courier.go` | Web platform courier | ❓ Not wired |

### Database

| File | Purpose | Status |
|------|---------|--------|
| `internal/db/supabase.go` | Supabase client | ✅ Works |
| `internal/db/rpc.go` | RPC allowlist, calls | ⚠️ Bug? |

### Other

| File | Purpose | Status |
|------|---------|--------|
| `internal/gitree/gitree.go` | Git operations | ✅ Works |
| `internal/webhooks/github.go` | GitHub webhook handler | ✅ Works |
| `internal/realtime/client.go` | Supabase realtime | ✅ Works |
| `internal/security/leak_detector.go` | Secret scanning | ✅ Works |

---

## Part 2: Event Flow (What SHOULD Happen)

```
PRD PUSHED TO GITHUB
    ↓
webhooks/github.go: GitHubWebhookHandler
    ↓ Detects new .md in docs/prd/
    ↓
Creates plan in Supabase (status=draft)
    ↓
realtime/client.go: Receives INSERT event
    ↓
main.go: Routes to EventPlanCreated handler
    ↓
handlers_plan.go: EventPlanCreated
    ↓
    ├─ Claim plan (set_processing)
    ├─ Router selects model (planner)
    ├─ Create session
    ├─ Run planner (reads PRD, creates plan)
    ├─ Parse plan output
    ├─ Commit plan file to git
    ├─ Trigger supervisor review
    ↓
handlers_plan.go: EventPlanCreated (supervisor review)
    ↓
    ├─ Router selects model (supervisor)
    ├─ Run supervisor (reads PRD + plan)
    ├─ Parse decision
    ├─ If approved → create tasks
    ↓
validation.go: createTasksFromApprovedPlan
    ↓
    ├─ Parse tasks from plan markdown
    ├─ Validate each task
    ├─ For each task:
    │   ├─ Call create_task_with_packet RPC
    │   ├─ Creates task (status=available)
    │   ├─ Creates task_packet
    ↓
realtime/client.go: Receives INSERT event on tasks
    ↓
main.go: Routes to EventTaskAvailable handler
    ↓
handlers_task.go: EventTaskAvailable
    ↓
    ├─ Claim task (set_processing)
    ├─ Load task_packet
    ├─ Create git branch
    ├─ Router selects model
    ├─ Call update_task_assignment RPC
    │   └─ Sets: status=in_progress, assigned_to=model_id
    ├─ Create session
    ├─ Submit to pool
    ↓
Pool executes (async):
    ↓
    ├─ session.Run (executes via CLI)
    ├─ Parse output
    ├─ Scan for leaks
    ├─ Extract tokens
    ├─ Calculate costs
    ├─ Commit output to git
    ├─ Call create_task_run RPC
    │   └─ Creates: task_runs record with tokens, costs
    ├─ Call update_task_status RPC
    │   └─ Sets: status=review
    ├─ Call record_model_success RPC
    ↓
realtime/client.go: Receives UPDATE event
    ↓
main.go: Routes to EventTaskReview handler
    ↓
handlers_task.go: EventTaskReview
    ↓
    ├─ Router selects model (supervisor)
    ├─ Run supervisor (reviews output)
    ├─ Parse decision
    ├─ If pass → status=testing
    ├─ If fail → status=available (retry)
    ↓
handlers_testing.go: EventTaskTesting
    ↓
    ├─ Run tests
    ├─ If pass → status=approval
    ├─ If fail → status=available (retry)
    ↓
Human reviews and merges
    ↓
status=merged
```

---

## Part 3: What ACTUALLY Happens (Current Broken Flow)

```
PRD PUSHED TO GITHUB
    ↓
webhooks/github.go: ✅ Works
    ↓
Creates plan in Supabase (status=draft) ✅
    ↓
realtime/client.go: ✅ Receives INSERT
    ↓
main.go: ✅ Routes to EventPlanCreated
    ↓
handlers_plan.go: EventPlanCreated
    ↓
    ├─ ✅ Claim plan
    ├─ ✅ Router selects glm-5
    ├─ ✅ Run planner
    ├─ ✅ Parse output
    ├─ ✅ Commit plan file
    ├─ ✅ Trigger supervisor
    ↓
handlers_plan.go: Supervisor review
    ↓
    ├─ ✅ Router selects glm-5
    ├─ ✅ Run supervisor
    ├─ ✅ Parse decision (approved)
    ├─ ✅ Call createTasksFromApprovedPlan
    ↓
validation.go: createTasksFromApprovedPlan
    ↓
    ├─ ✅ Parse tasks
    ├─ ✅ Validate
    ├─ ✅ Create task (status=available)
    ├─ ✅ Create task_packet
    ↓
realtime/client.go: ✅ Receives INSERT
    ↓
main.go: ✅ Routes to EventTaskAvailable
    ↓
handlers_task.go: EventTaskAvailable
    ↓
    ├─ ✅ Claim task
    ├─ ✅ Load packet
    ├─ ✅ Create branch
    ├─ ✅ Router selects glm-5
    ├─ ❌ Call update_task_assignment
    │   └─ ERROR: "RPC not in allowlist" (BUT IT IS!)
    ├─ ✅ Create session (continues despite error)
    ├─ ✅ Submit to pool
    ↓
Pool executes:
    ↓
    ├─ ✅ session.Run (executes via kilo)
    ├─ ✅ Parse output
    ├─ ✅ Scan for leaks
    ├─ ✅ Extract tokens (613 tokens)
    ├─ ❌ Commit output to git
    │   └─ ERROR: "files must be an array"
    ├─ ✅ Call create_task_run RPC
    │   └─ SUCCESS! (after rebuild)
    ├─ ❌ Call update_task_status
    │   └─ Sets status=review (EVEN THOUGH COMMIT FAILED!)
    ├─ ❌ Call record_model_success
    │   └─ NOT CALLED
    ↓
Task stuck in limbo:
    ├─ status=review (but never actually completed)
    ├─ assigned_to=glm-5 (correct)
    ├─ task_runs created (correct)
    └─ No git commit (failed)
```

---

## Part 4: The Bugs

### Bug 1: RPC Allowlist Mystery

**Symptom:** "RPC update_task_assignment not in allowlist"

**But:** It IS in the allowlist:
```go
var defaultRPCAllowlist = map[string]bool{
    "update_task_assignment":  true,  // ← RIGHT THERE
```

**Possible causes:**
1. Binary not rebuilt? → Rebuilt, still fails
2. Wrong DB instance? → Need to check
3. RPC name mismatch? → Names match
4. Race condition? → Unlikely

**Status:** UNKNOWN - need to trace actual execution

### Bug 2: Git Commit Failure

**Symptom:** "files must be an array"

**Location:** `internal/gitree/gitree.go`

**Cause:** Output parsing expects files to be array, but runner output doesn't have it

**Status:** KNOWN - need to fix output parsing

### Bug 3: Status Logic Error

**Symptom:** status=review even though commit failed

**Location:** `handlers_task.go` line ~243

**Code:**
```go
newStatus := "review"
if commitErr != nil {
    newStatus = "error"
}
```

**Problem:** Sets "review" even when commit fails, then tries to set "error" which isn't valid status

**Status:** KNOWN - need to fix logic

### Bug 4: No Error Recovery

**Symptom:** Task stuck forever when errors occur

**Problem:** No retry logic, no reset to available

**Status:** KNOWN - need to add recovery

---

## Part 5: Timing Analysis

**From logs:**

| Step | Time | Cumulative |
|------|------|------------|
| PRD pushed | 22:42:35 | 0s |
| Plan created | 22:42:35 | 0s |
| Planner started | 22:42:35 | 0s |
| Planner finished | ~22:43:00 | ~25s |
| Supervisor started | ~22:43:00 | ~25s |
| Supervisor finished | 22:43:22 | 47s |
| Task created | 22:43:22 | 47s |
| Task execution started | 22:43:22 | 47s |
| Task execution finished | 22:43:48 | 73s |

**Total time: 73 seconds for hello world**

**But:** Task never actually completes because of bugs

---

## Part 6: The Real Problems

### Problem 1: No Atomic State Transitions

**Should be:**
```
available → (claim) → in_progress → (complete) → review
```

**Actually is:**
```
available → (claim fails) → ??? → (execution continues) → review (wrong!)
```

### Problem 2: Errors Don't Stop Execution

**Should be:**
```
If RPC fails → STOP → retry → or set error status
```

**Actually is:**
```
If RPC fails → LOG ERROR → CONTINUE ANYWAY → corrupt state
```

### Problem 3: Status Values Don't Match

**Supabase schema:**
```sql
CHECK (status IN ('available', 'pending', 'in_progress', 'review', 'testing', 'approval', 'merged', 'error'))
```

**But Go code sets:**
- "error" → Not in check constraint!
- "review" → Valid but set at wrong time

### Problem 4: No Learning System Wired

**Should call:**
- `record_model_success` after pass
- `record_model_failure` after fail
- `calculate_run_costs` for ROI

**Actually calls:**
- None of them

---

## Part 7: What Needs to Happen

### Option A: Fix Bugs (Quick but Fragile)

1. Fix RPC allowlist mystery
2. Fix git commit files parsing
3. Fix status logic (only set review on success)
4. Add error recovery

**Time:** 2-4 hours
**Risk:** Still fragile, more bugs will appear

### Option B: Rewrite Handlers (Proper Solution)

1. Map complete flow (this doc)
2. Rewrite handlers_task.go from scratch
3. Proper error handling at every step
4. Atomic state transitions
5. Wire learning system

**Time:** 4-8 hours
**Risk:** Low if we follow design

### Option C: Full Rewrite (Clean Slate)

1. Keep working parts (realtime, webhooks, git, runners)
2. Rewrite all handlers
3. Clean up router confusion
4. Wire everything properly

**Time:** 1-2 days
**Risk:** Low, clean foundation

---

## Part 8: Recommendation

**DO OPTION B:** Rewrite handlers_task.go

**Why:**
- Other parts work (webhooks, realtime, git, runners)
- Only handlers are broken
- Can do it in one session
- Clean, proper solution

**Steps:**
1. Read current handlers_task.go (565 lines)
2. Map every function
3. Rewrite with:
   - Atomic state transitions
   - Error handling at every step
   - Proper status values
   - Learning system wired
4. Test with hello world
5. Verify end-to-end

---

## Part 9: Questions for Human

1. **Option A, B, or C?** (Fix bugs, rewrite handlers, full rewrite)
2. **Keep current code?** Or start fresh?
3. **Priority:** Speed (fix bugs) or quality (rewrite)?
4. **Test approach:** Hello world only, or comprehensive tests?
