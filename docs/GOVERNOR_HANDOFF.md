# Governor Implementation Handoff Document

**Session:** 2026-02-23 (Session 26)
**Purpose:** System Researcher implemented - handles escalated task analysis
**Status:** System Researcher COMPLETE, Visual Testing stub remains

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

## CODEBASE OVERVIEW

### Files (22 Go files, 4,502 lines)

```
governor/
├── cmd/governor/main.go           # Entry point, wires all components (118 lines)
├── internal/
│   ├── sentry/sentry.go           # Polls Supabase for available tasks (136 lines)
│   ├── dispatcher/dispatcher.go   # Routes tasks to runners (410 lines)
│   ├── orchestrator/orchestrator.go # Coordinates supervisor/maintenance (289 lines)
│   ├── supervisor/supervisor.go   # Quality control for tasks/plans (354 lines)
│   ├── council/council.go         # Multi-lens plan review (565 lines)
│   ├── researcher/researcher.go   # Escalated task analysis (300 lines) ← NEW
│   ├── maintenance/maintenance.go # Git operations (172 lines)
│   ├── tester/tester.go           # pytest/lint execution (60 lines)
│   ├── visual/visual.go           # Visual UI testing (31 lines)
│   ├── janitor/janitor.go         # Resets stuck tasks (75 lines)
│   ├── pool/model_pool.go         # Runner selection (53 lines)
│   ├── throttle/module_limiter.go # Per-slice rate limiting (67 lines)
│   ├── security/
│   │   ├── leak_detector.go       # Secret scanning (69 lines)
│   │   └── allowlist.go           # HTTP origin validation (61 lines)
│   ├── courier/
│   │   ├── dispatcher.go          # Web platform routing (136 lines)
│   │   └── webhook.go             # Courier callback handler (87 lines)
│   ├── server/
│   │   ├── server.go              # HTTP API + WebSocket (396 lines)
│   │   └── hub.go                 # WebSocket broadcast (138 lines)
│   ├── db/supabase.go             # Database operations (713 lines)
│   └── config/config.go           # YAML configuration (115 lines)
├── pkg/types/types.go             # Shared types (157 lines)
├── go.mod                         # Dependencies
├── governor.yaml                  # Configuration
└── Makefile
```

---

## COMPLETE SYSTEM FLOW

### Task Lifecycle (End to End)

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
      - HUMAN (visual) → visual test → human review
   → If APPROVE and type!=test/docs: queue for testing
   
4. SUPERVISOR (supervisor/supervisor.go)
   Review(input) → Decision
   → Check deliverables (expected vs actual files)
   → Check code quality (secrets, TODOs, print statements)
   → Return decision (APPROVE/REJECT/HUMAN)
   
   ReviewPlan(input) → PlanDecision
   → Check plan has title/description/tasks
   → Return decision (APPROVE/REJECT/COUNCIL)
   
   ReviewResearch(input) → ResearchDecision
   → Simple (add model) → APPROVE
   → Significant (new strategy) → COUNCIL

5. MERGE EXECUTION (dispatcher.executeMerge)
   Dispatcher picks up merge task (type=merge)
   → Get parent task from DB
   → Maintenance.ExecuteMerge(parentID, branchName)
   → On success: parent→merged, delete branch
   → On failure: handleMergeFailure() → retry 3x then escalate
```

---

# SUPERVISOR LOGIC (supervisor/supervisor.go)

## Task Output Actions (3)

| Action | Constant | When Triggered |
|--------|----------|----------------|
| APPROVE | `ActionApprove` | All checks pass, no issues |
| REJECT | `ActionReject` | Missing deliverables, secrets, quality issues |
| HUMAN | `ActionHuman` | Visual/ui_ux changes (after visual testing) |

## Plan Review Actions (3)

| Action | When |
|--------|------|
| APPROVE | Simple plan, no issues |
| REJECT | Missing title/description/tasks |
| COUNCIL | Complex: auth, security, migration, >5 tasks, priority 1 |

## Research Review Actions (3)

| Action | When |
|--------|------|
| APPROVE | Simple: add model, add API key |
| REJECT | Missing title/description |
| COUNCIL | Significant: new strategy, technique, architecture |

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
    decision := o.supervisor.Review(ctx, reviewInput)
    
    // 5. Execute decision
    switch decision.Action {
    case supervisor.ActionApprove:
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
        
    case supervisor.ActionReject:
        handleRejection(ctx, task, decision.Notes)
        
    case supervisor.ActionHuman:
        // Visual testing first
        if task.Type == "ui_ux" && o.visualTester != nil {
            visualResult := o.visualTester.TestVisual(ctx, task.BranchName, packet.Deliverables)
            if !visualResult.Passed {
                handleRejection(ctx, task, "Visual testing failed: " + strings.Join(visualResult.Failures, "; "))
                return
            }
        }
        
        // Then human review
        db.UpdateTaskStatus(ctx, taskID, StatusAwaitingHuman, {"reason": decision.Reason})
    }
}
```

---

# COUNCIL LOGIC (council/council.go)

## Purpose

Council reviews **PLANS** and **RESEARCH SUGGESTIONS**, NOT task outputs.

## ReviewPlan()

```go
func (c *Council) ReviewPlan(ctx context.Context, input *PlanReviewInput) (*DeliberationResult, error) {
    lenses := c.determineLenses(input)
    // new_project: user_alignment, ideal_vision, technical_security
    // system_improvement: integration, reversibility, principle_align
    // feature: user_alignment, integration, technical_security
    
    models := c.db.GetAvailableModels(ctx, len(lenses))
    
    for round := 1; round <= maxRounds; round++ {
        reviews := c.executeRound(ctx, input, lenses, models, round, previousReviews)
        
        consensus := c.checkConsensus(reviews)
        if consensus == "APPROVED" {
            return buildResult(planID, true, "unanimous", round, reviews), nil
        }
        if consensus == "BLOCKED" {
            return buildResult(planID, false, "blocked", round, reviews), nil
        }
    }
    
    return buildResult(planID, false, "no_consensus", maxRounds, reviews), nil
}
```

## ReviewResearchSuggestion()

Same pattern but with research-specific lenses:
- `new_platform`: integration, principle_align, technical_security
- `new_strategy`: ideal_vision, principle_align, reversibility
- `architecture_change`: integration, reversibility, technical_security
- `new_technique`: ideal_vision, technical_security, principle_align

## Lens Definitions

| Lens | Consider |
|------|----------|
| user_alignment | User intent, delivers what asked |
| ideal_vision | Best approach, right direction |
| technical_security | Implementation sound, security |
| integration | Fits existing system, breaking changes |
| reversibility | Can be undone, rollback path |
| principle_align | VibePilot principles, zero lock-in |

---

# MAINTENANCE OPERATIONS (maintenance/maintenance.go)

## ExecuteMerge()

```go
func (m *Maintenance) ExecuteMerge(ctx context.Context, taskID, branchName string) error {
    // 1. Merge branch to main
    if err := m.MergeBranch(ctx, branchName, "main"); err != nil {
        return err  // Merge conflict - branch preserved
    }
    
    // 2. Delete branch (only after successful merge)
    m.DeleteBranch(ctx, branchName)
    
    return nil
}
```

---

# CONFIG OPTIONS (governor.yaml)

```yaml
governor:
  poll_interval: 15s           # How often to check for tasks
  max_concurrent: 3             # Max tasks running simultaneously
  stuck_timeout: 10m           # Reset tasks stuck this long
  max_per_module: 8            # Max concurrent per slice
  task_timeout_sec: 300        # Task execution timeout (5 min)
  council_max_rounds: 4        # Council deliberation rounds
  repo_path: .                 # Git repository path
```

---

# KEY RULES

1. **3 Supervisor Actions for TASK OUTPUTS**: approve, reject, human
2. **3 Supervisor Actions for PLANS**: approve, reject, council
3. **Empty output = FAILURE** (not silent success)
4. **`review` status should NEVER exist in DB** - process synchronously
5. **Merge failures are SYSTEM problems** - Maintenance fixes, not human
6. **Branch never deleted until merge succeeds** - Work is safe
7. **Merge is separate from implementation** - Creates child task (type=merge)
8. **`awaiting_human` triggers dashboard flag**
9. **Council reviews PLANS, not task outputs**
10. **Visual changes: visual test → human, NOT human → visual test**
11. **Escalated tasks are analyzed by Researcher** - AI decides: auto-retry, human review, or infrastructure wait

---

# SYSTEM RESEARCHER (NEW - Session 26)

## Purpose

When a task fails 3x and escalates, the Researcher analyzes the failure and decides:
1. **Auto-retry** with different model (model issue)
2. **Route to human** (task definition or dependency issue)
3. **Wait and retry** (infrastructure issue like rate limit)

## Flow

```
Task fails 3x → status=escalated → Researcher.AnalyzeEscalation()
  → Category: MODEL_ISSUE
     → Auto-retry with suggested alternative model
  → Category: TASK_DEFINITION / DEPENDENCY
     → Route to awaiting_human with analysis summary
  → Category: INFRASTRUCTURE
     → Reset attempts, return to available queue
```

## Categories

| Category | Action | Reason |
|----------|--------|--------|
| `model_issue` | Auto-retry with different model | Timeout, test failures, model couldn't handle |
| `task_definition` | Human review | Missing deliverables, poor prompt, secrets detected |
| `dependency_issue` | Human review | Dependencies wrong or blocked |
| `infrastructure` | Wait and retry | Rate limits, git failures, transient errors |

## Analysis Factors

- Failure notes patterns (timeout, rate limit, missing file)
- Task run history (which models failed)
- Prompt packet quality (missing prompt/deliverables)
- Context size (large context may overwhelm model)

---

# STUBS REMAINING

| Location | Stub | Needed |
|----------|------|--------|
| `visual/visual.go` | `TestVisual()` passes by default | Real visual testing implementation |
| `maintenance.go` | No command queue polling | Poll `maintenance_commands` table |

---

# NEXT SESSION

1. **Visual testing** - Implement real UI testing in `visual/visual.go`
2. **Maintenance command polling** - Poll `maintenance_commands` table
3. **Config hot-reload** - fsnotify watcher (optional)

---

**END OF HANDOFF - Session 26**
