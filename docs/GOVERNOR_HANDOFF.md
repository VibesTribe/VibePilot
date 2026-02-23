# Governor Implementation Handoff Document

**Session:** 2026-02-23 (Session 24)
**Purpose:** Complete Go Governor with merge task system + wire merge pending UI
**Status:** Phase 4 COMPLETE, merge pending UI WIRED

---

## QUICK START

```bash
cd ~/vibepilot/governor
go build -o governor ./cmd/governor
./governor
```

**Expected output:**
```
VibePilot Governor dev starting...
Poll interval: 15s, Max concurrent: 3, Max per module: 8
Connected to Supabase
Sentry started: polling every 15s, max 3 concurrent, 8 per module
Dispatcher started
Orchestrator started
Janitor started: stuck timeout 10m0s
Server starting on :8080
```

---

# COMPLETE SYSTEM FLOW

## Task Lifecycle (End to End)

```
1. SENTRY (sentry/sentry.go)
   Polls Supabase every 15s for status=available tasks
   → Checks module limits (max 8 per slice)
   → Sends to Dispatcher channel
   
2. DISPATCHER (dispatcher/dispatcher.go)
   Receives task from Sentry
   → If type=merge: executeMerge() (skip to step 5)
   → Select best runner from Pool
   → Claim task in DB (status=in_progress)
   → Run tool (opencode/kimi) with prompt
   → Scan output for secrets (LeakDetector)
   → Record task_run in DB
   → Call OnTaskComplete() on Orchestrator
   
3. ORCHESTRATOR (orchestrator/orchestrator.go)
   OnTaskComplete(taskID, result)
   → Create branch (task/T###)
   → Commit output to branch
   → Call Supervisor.Review()
   → Process Supervisor decision:
      - APPROVE → status=approval, create merge task
      - REJECT → handleRejection()
      - HUMAN → status=awaiting_human
      - COUNCIL → status=awaiting_human (stub)
   → If APPROVE and type!=test/docs: queue for testing
   
4. SUPERVISOR (supervisor/supervisor.go)
   Review(input) → Decision
   → Check deliverables (expected vs actual files)
   → Check code quality (secrets, TODOs, print statements)
   → Return decision (APPROVE/REJECT/HUMAN/COUNCIL)

5. MERGE EXECUTION (dispatcher.executeMerge)
   Dispatcher picks up merge task (type=merge)
   → Get parent task from DB
   → Maintenance.ExecuteMerge(parentID, branchName)
   → On success: parent→merged, delete branch
   → On failure: handleMergeFailure() → retry 3x then escalate
```

---

# SUPERVISOR LOGIC (supervisor/supervisor.go)

## 4 Actions Only

| Action | Constant | When Triggered |
|--------|----------|----------------|
| APPROVE | `ActionApprove` | All checks pass, no issues |
| REJECT | `ActionReject` | Missing deliverables, secrets detected, quality issues |
| HUMAN | `ActionHuman` | Visual/ui_ux changes detected |
| COUNCIL | `ActionCouncil` | Security, auth, architecture, refactor, priority 1 |

## Review() Function Flow

```go
func (s *Supervisor) Review(ctx context.Context, input *ReviewInput) Decision {
    // 1. Check deliverables
    issues, warnings = s.checkDeliverables(input, issues, warnings)
    
    // 2. Check code quality (secrets, TODOs, etc)
    issues = s.checkCodeQuality(input.OutputContent, issues, warnings)
    
    // 3. If issues exist → REJECT
    if len(issues) > 0 {
        return Decision{Action: ActionReject, Notes: formatNotes(issues), Issues: issues}
    }
    
    // 4. If ui_ux or visual change → HUMAN
    if input.TaskType == "ui_ux" || input.VisualChange {
        return Decision{Action: ActionHuman, Reason: "Visual changes require human approval"}
    }
    
    // 5. Otherwise → APPROVE
    return Decision{Action: ActionApprove, Warnings: warnings}
}
```

## Deliverable Check (checkDeliverables)

```
Expected files: ["src/auth.py", "tests/test_auth.py"]
Actual files:    ["src/auth.py", "src/utils.py"]

Result:
- ISSUES: "Missing deliverables: tests/test_auth.py"
- WARNINGS: "Extra files created (scope creep): src/utils.py"
```

## Code Quality Check (checkCodeQuality)

**Blocks (causes REJECT):**
- `sk-` (OpenAI key)
- `ghp_` (GitHub token)
- `AKIA` (AWS key)
- `password = "` or `password='` literals

**Warns (passes but flagged):**
- `TODO` or `FIXME` comments
- `print()` statements in functions
- Output < 50 chars (truncated)

## NeedsCouncil() Logic

```go
func (s *Supervisor) NeedsCouncil(taskType, title string, priority int) bool {
    if taskType == "security" { return true }
    
    titleLower := strings.ToLower(title)
    if strings.Contains(titleLower, "auth") ||
       strings.Contains(titleLower, "authentication") ||
       strings.Contains(titleLower, "architecture") ||
       strings.Contains(titleLower, "refactor") {
        return true
    }
    
    if priority <= 1 { return true }  // Critical tasks
    
    return false
}
```

---

# ORCHESTRATOR LOGIC (orchestrator/orchestrator.go)

## OnTaskComplete() Flow

```go
func (o *Orchestrator) OnTaskComplete(ctx context.Context, taskID string, result interface{}) {
    // 1. Get task from DB
    task, err := o.db.GetTaskByID(ctx, taskID)
    
    // 2. Create branch if needed
    if task.BranchName == "" {
        branchName := generateBranchName(task)  // "task/T###"
        maintenance.CreateBranch(ctx, branchName)
        task.BranchName = branchName
        db.UpdateTaskBranch(ctx, taskID, branchName)
    }
    
    // 3. Commit output to branch
    maintenance.CommitOutput(ctx, task.BranchName, result)
    
    // 4. Process supervisor decision
    o.processSupervisorDecision(ctx, task, result)
}
```

## processSupervisorDecision() Flow

```go
func (o *Orchestrator) processSupervisorDecision(ctx context.Context, task *types.Task, result interface{}) {
    // 1. Get prompt packet (for deliverables list)
    packet := db.GetTaskPacket(ctx, taskID)
    
    // 2. Get actual files from branch
    actualFiles := maintenance.ReadBranchOutput(ctx, task.BranchName)
    
    // 3. Build review input
    reviewInput := &supervisor.ReviewInput{
        TaskID:         taskID,
        TaskType:       task.Type,
        ExpectedFiles:  packet.Deliverables,
        ActualFiles:    actualFiles,
        VisualChange:   task.Type == "ui_ux",
    }
    
    // 4. Get decision
    decision := supervisor.Review(ctx, reviewInput)
    
    // 5. Override to COUNCIL if significant change
    if decision.Action == ActionApprove && supervisor.NeedsCouncil(...) {
        decision = Decision{Action: ActionCouncil}
    }
    
    // 6. Execute decision
    switch decision.Action {
    case ActionApprove:
        // Set status=approval
        db.UpdateTaskStatus(ctx, taskID, StatusApproval, nil)
        
        // Create merge task
        mergeTaskID := generateID()
        db.CreateMergeTask(ctx, mergeTaskID, taskID, task.SliceID, task.BranchName, task.Title)
        
        // If not test/docs, run tests first
        if task.Type != "test" && task.Type != "docs" {
            db.UpdateTaskStatus(ctx, taskID, StatusTesting, result)
            queueTest(taskID)
        }
        
    case ActionReject:
        handleRejection(ctx, task, decision.Notes)
        
    case ActionHuman:
        db.UpdateTaskStatus(ctx, taskID, StatusAwaitingHuman, {"reason": decision.Reason})
        
    case ActionCouncil:
        // Stub - routes to human for now
        db.UpdateTaskStatus(ctx, taskID, StatusAwaitingHuman, {"reason": "Council review needed"})
    }
}
```

## handleRejection() Logic

```go
func (o *Orchestrator) handleRejection(ctx context.Context, task *types.Task, notes string) {
    currentTask := db.GetTaskByID(ctx, taskID)
    newAttempts := currentTask.Attempts + 1
    escalate := newAttempts >= currentTask.MaxAttempts
    
    if escalate {
        // Mark as escalated - AI will analyze
        db.UpdateTaskStatus(ctx, taskID, StatusEscalated, {
            "attempts": newAttempts,
            "failure_notes": notes,
        })
        go handleEscalation(ctx, taskID, notes)
    } else {
        // Return to queue for retry
        db.ResetTask(ctx, taskID, false)
        db.UpdateTaskStatus(ctx, taskID, StatusAvailable, {
            "attempts": newAttempts,
            "failure_notes": notes,
        })
    }
}
```

---

# MERGE ERROR LOGIC (dispatcher/dispatcher.go)

## executeMerge() Function

```go
func (d *Dispatcher) executeMerge(ctx context.Context, task types.Task) {
    // task.Type == "merge"
    // task.ParentTaskID = ID of implementation task
    
    log.Printf("Executing merge task %s for parent %s", task.ID[:8], task.ParentTaskID[:8])
    
    // 1. Mark merge task as in_progress
    db.UpdateTaskStatus(ctx, task.ID, StatusInProgress, nil)
    
    // 2. Get parent task (the one that did the work)
    parentTask := db.GetTaskByID(ctx, task.ParentTaskID)
    if parentTask == nil {
        handleMergeFailure(ctx, task, "Parent task not found")
        return
    }
    
    // 3. Check parent has a branch
    if parentTask.BranchName == "" {
        handleMergeFailure(ctx, task, "Parent task has no branch")
        return
    }
    
    // 4. Execute the merge
    err := maintenance.ExecuteMerge(ctx, task.ParentTaskID, parentTask.BranchName)
    if err != nil {
        handleMergeFailure(ctx, task, err.Error())
        return
    }
    
    // 5. SUCCESS - update both tasks
    db.UpdateTaskStatus(ctx, task.ID, StatusMerged, nil)  // Merge task done
    db.UpdateTaskStatus(ctx, task.ParentTaskID, StatusMerged, {"merged_to": "main"})
    db.UnlockDependents(ctx, task.ParentTaskID)  // Wake up blocked tasks
}
```

## handleMergeFailure() Function

```go
func (d *Dispatcher) handleMergeFailure(ctx context.Context, task types.Task, errMsg string) {
    attempts := task.Attempts + 1
    
    if attempts >= task.MaxAttempts {
        // TRIED 3 TIMES - ESCALATE
        // System Researcher should analyze and fix
        log.Printf("Merge task %s ESCALATED after %d attempts", task.ID[:8], attempts)
        db.UpdateTaskStatus(ctx, task.ID, StatusEscalated, {
            "attempts": attempts,
            "failure_notes": errMsg,
        })
    } else {
        // RETRY - put back in queue
        db.ResetTask(ctx, task.ID, false)
        db.UpdateTaskStatus(ctx, task.ID, StatusAvailable, {
            "attempts": attempts,
            "failure_notes": errMsg,
        })
        log.Printf("Merge task %s returned to queue (attempt %d/%d)", task.ID[:8], attempts, task.MaxAttempts)
    }
}
```

## Merge Flow Diagram

```
Implementation task APPROVED
         ↓
  Status: approval
         ↓
  Orchestrator creates MERGE TASK
  {
    id: <new-id>,
    type: "merge",
    parent_task_id: <impl-task-id>,
    branch_name: "task/T004",
    status: "available",
    max_attempts: 3
  }
         ↓
  Sentry picks up merge task (status=available)
         ↓
  Dispatcher sees type=merge → executeMerge()
         ↓
  maintenance.ExecuteMerge()
         ↓
         ├─ SUCCESS ──────────────────────┐
         │                                ↓
         │   Merge task → status=merged
         │   Parent task → status=merged
         │   Delete branch
         │   Unlock dependents
         │
         └─ FAIL (conflict, etc)
              ↓
         handleMergeFailure()
              ↓
         attempts++ (1 → 2 → 3)
              ↓
         ┌─ attempts < 3 ────────────────┐
         │                               ↓
         │   Reset merge task
         │   status → available
         │   (Sentry will retry)
         │
         └─ attempts >= 3 ───────────────┐
                                         ↓
            Merge task → status=escalated
            (System Researcher handles)
```

---

# MAINTENANCE OPERATIONS (maintenance/maintenance.go)

## ExecuteMerge()

```go
func (m *Maintenance) ExecuteMerge(ctx context.Context, taskID, branchName string) error {
    // 1. Merge branch to main
    if err := m.MergeBranch(ctx, branchName, "main"); err != nil {
        return err  // Merge conflict, etc - branch preserved
    }
    
    // 2. Delete branch (only after successful merge)
    m.DeleteBranch(ctx, branchName)
    
    return nil
}
```

## MergeBranch()

```go
func (m *Maintenance) MergeBranch(ctx context.Context, sourceBranch, targetBranch string) error {
    // 1. Checkout target
    git("checkout", targetBranch)
    git("pull", "origin", targetBranch)
    
    // 2. Merge source
    cmd := git("merge", sourceBranch)
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("merge: %w - %s", err, output)
        // BRANCH PRESERVED - can retry
    }
    
    // 3. Push
    git("push", "origin", targetBranch)
    return nil
}
```

## DeleteBranch()

```go
func (m *Maintenance) DeleteBranch(ctx context.Context, branchName string) error {
    git("branch", "-d", branchName)           // Local
    git("push", "origin", "--delete", branchName)  // Remote
    return nil
}
```

## CommitOutput()

```go
func (m *Maintenance) CommitOutput(ctx context.Context, branchName string, output interface{}) error {
    // 1. Checkout branch
    git("checkout", branchName)
    
    // 2. Write files from output
    if files, ok := output["files"]; ok {
        for file := range files {
            writeFile(file.path, file.content)
        }
    }
    
    // 3. If no files, write task_output.txt
    if output["files"] == nil {
        writeFile("task_output.txt", output["output"])
    }
    
    // 4. Commit
    git("add", ".")
    cmd := git("commit", "-m", "task output")
    if err := cmd.Run(); err != nil {
        if "nothing to commit" {
            return error("task produced no output")
        }
        return err
    }
    
    // 5. Push
    git("push")
}
```

---

# DISPATCHER EXECUTION (dispatcher/dispatcher.go)

## execute() Function

```go
func (d *Dispatcher) execute(ctx context.Context, task types.Task) {
    log.Printf("Task %s (routing=%s, type=%s)", task.ID[:8], task.RoutingFlag, task.Type)
    
    // 1. MERGE TASK - special handling
    if task.Type == "merge" {
        d.executeMerge(ctx, task)
        return
    }
    
    // 2. WEB COURIER - dispatch to browser runner
    if task.RoutingFlag == RoutingWeb && courier != nil {
        d.dispatchToCourier(ctx, task)
        return
    }
    
    // 3. Select runner from pool
    runner := pool.SelectBest(ctx, routingFlag, taskType)
    if runner == nil {
        d.handleFailure(ctx, task)
        return
    }
    
    // 4. Claim task
    db.ClaimTask(ctx, task.ID, runner.ModelID)
    
    // 5. Get prompt packet
    packet := db.GetTaskPacket(ctx, task.ID)
    
    // 6. Run tool
    output, tokensIn, tokensOut, err := d.runTool(ctx, runner.ToolID, packet.Prompt, 300)
    
    // 7. EMPTY OUTPUT = FAILURE
    if err == nil && strings.TrimSpace(output) == "" {
        err = errors.New("Empty output - task produced no response")
        success = false
    }
    
    // 8. Record task_run
    runID := db.RecordTaskRun(ctx, &TaskRunInput{
        TaskID: task.ID,
        ModelID: runner.ModelID,
        Status: success ? "success" : "failed",
        Result: result,
        TokensIn: tokensIn,
        TokensOut: tokensOut,
    })
    
    // 9. Calculate ROI
    db.CallROIRPC(ctx, runID)
    
    // 10. Handle failure or complete
    if !success {
        d.handleFailure(ctx, task)
        return
    }
    
    // 11. Throttle check (80% daily limit)
    if pool.ShouldThrottle(runner) {
        pool.SetCooldown(ctx, runner.ID, timeUntilMidnight())
    }
    
    // 12. Notify orchestrator
    if completer != nil {
        completer.OnTaskComplete(ctx, task.ID, result)
    }
}
```

---

# DASHBOARD STATUS MAPPING

## vibepilotAdapter.ts

```typescript
function mapTaskStatus(status: string): TaskSnapshot["status"] {
  const statusMap = {
    pending: "assigned",
    available: "assigned",
    in_progress: "in_progress",
    awaiting_human: "supervisor_review",  // FLAGGED - human needed
    testing: "testing",
    approval: "supervisor_approval",       // Merge pending indicator
    merged: "complete",
    failed: "blocked",
    escalated: "blocked",
  };
  return statusMap[status] || "assigned";
}
```

## Merge Pending Logic

```typescript
// Task level
function transformTasks(tasks) {
  return tasks.map(task => ({
    ...
    mergePending: task.status === "approval",  // Approved but not merged
  }));
}

// Slice level
function transformSlices(tasks) {
  for (task of tasks) {
    if (task.status === "approval") {
      stats.mergePending += 1;
    }
  }
}
```

---

# BRANCH LIFECYCLE (CRITICAL)

```
IMPLEMENTATION TASK STARTS
         ↓
Create branch: task/T###-desc
         ↓
Work happens (commits pushed)
         ↓
Task approved → status=approval
         ↓
Merge task created (type=merge)
         ↓
Merge task executes
         ↓
         ├─ SUCCESS ─────────────────┐
         │                           ↓
         │   Parent → status=merged
         │   DELETE BRANCH
         │
         └─ FAIL ────────────────────┐
                                     ↓
            BRANCH PRESERVED
            Retry merge (up to 3x)
            
BRANCH IS NEVER DELETED UNTIL AFTER MERGE SUCCEEDS
```

---

# KEY RULES

1. **4 Supervisor Actions ONLY**: approve, reject, council, human
2. **Empty output = FAILURE** (not silent success)
3. **`review` status should NEVER exist in DB** - process synchronously
4. **Merge failures are SYSTEM problems** - Maintenance fixes, not human
5. **Branch never deleted until merge succeeds** - Work is safe
6. **Merge is separate from implementation** - Creates child task (type=merge)
7. **`awaiting_human` triggers dashboard flag** - `review` does NOT
8. **Merge retry limit: 3** - Then escalate to System Researcher

---

# FILES REFERENCE

## Go Governor (go-governor branch)

| File | Purpose |
|------|---------|
| `internal/supervisor/supervisor.go` | 4 actions, quality checks, NeedsCouncil |
| `internal/orchestrator/orchestrator.go` | OnTaskComplete, processSupervisorDecision, handleRejection |
| `internal/dispatcher/dispatcher.go` | execute, executeMerge, handleMergeFailure |
| `internal/maintenance/maintenance.go` | CreateBranch, CommitOutput, MergeBranch, ExecuteMerge |
| `internal/db/supabase.go` | CreateMergeTask, GetMergePendingTasks, ClaimTask, RecordTaskRun |
| `pkg/types/types.go` | Task struct with ParentTaskID field |

## Dashboard (vibeflow-test branch)

| File | Purpose |
|------|---------|
| `apps/dashboard/lib/vibepilotAdapter.ts` | Status mapping, mergePending: status===approval |
| `apps/dashboard/utils/mission.ts` | SliceCatalog.mergePending, deriveSlices count |
| `apps/dashboard/components/SliceHub.tsx` | Merge pending badge on slice center |
| `apps/dashboard/styles.css` | .slice-orbit__merge-pending styling |

---

# WHAT'S NOT DONE (Phase 5)

| Feature | Status | Notes |
|---------|--------|-------|
| Council deliberation | Stub | Routes to human, multi-lens review not implemented |
| Command queue polling | Not started | Maintenance should poll maintenance_commands table |
| Config hot-reload | Not started | fsnotify watcher for governor.yaml |
| System Researcher | Stub | handleEscalation just logs, needs AI analysis |

---

# TEST DATA

```
Task ID: 98805088-9b88-469c-be91-35f74ba27e7e
Title: Test: Echo Response
Status: approval
Slice: phase4-test
Branch: task/T004

This task is approved (status=approval).
Should show merge pending badge because merge task exists.
```

---

# NEXT SESSION

1. `cat docs/GOVERNOR_HANDOFF.md` - This file
2. `cat CURRENT_STATE.md` - System state
3. `cd ~/vibepilot/governor && go build ./...` - Verify builds
4. `cd ~/vibeflow && npm run typecheck` - Verify dashboard
5. Test merge pending flow with T004
6. Implement Council deliberation (Phase 5)

---

**END OF HANDOFF - Session 24**
