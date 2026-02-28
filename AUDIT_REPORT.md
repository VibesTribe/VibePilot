# VibePilot Full Code Audit Report

**Date:** 2026-02-28
**Auditor:** GLM-5
**Scope:** Complete Governor codebase (~7000 lines Go)

---

## EXECUTIVE SUMMARY

**Verdict: The system is a shell without a brain.**

We built infrastructure (config, polling, routing) but NOT the logic that makes decisions and takes actions. Governor routes to agents, receives structured JSON decisions, then does nothing with them except log.

---

## WHAT WE HAVE (Working)

| Component | File | Status | Notes |
|-----------|------|--------|-------|
| Config loading | config.go | ✅ WORKS | Loads from JSON, configurable |
| Dynamic routing | router.go | ✅ WORKS | Uses routing.json, strategies work |
| Model scoring | router.go:110-132 | ✅ WORKS | Calls get_model_score_for_task RPC |
| Event polling | events.go | ✅ WORKS | Polls Supabase every 1s |
| Branch creation | main.go:207-216 | ✅ WORKS | Creates task/T001 on assignment |
| Startup recovery | main.go:684-726 | ✅ WORKS | Recovers orphaned sessions |
| gitree library | gitree.go | ✅ EXISTS | Functions exist but NOT CALLED |
| Vault security | vault.go | ✅ WORKS | Proper encryption, no leaks |
| GitHub client | github/client.go | ✅ EXISTS | Created but NOT WIRED |

---

## WHAT WE DON'T HAVE (Critical Gaps)

| Gap | File | Lines | Problem |
|-----|------|-------|---------|
| **Decision parsing** | main.go | 186-538 | ALL handlers log output, don't parse it |
| **State transitions** | main.go | - | No status updates based on decisions |
| **Rejection handling** | main.go | - | No wipe/reassign/escalate logic |
| **Failure tracking** | main.go | - | recordModelFailure logs but doesn't count |
| **GitHub commits** | main.go | - | gitree.CommitOutput exists, never called |
| **PRD detection** | main.go | - | github/client.go exists, not wired |

---

## DETAILED FINDINGS

### 1. HARDCODING ISSUES

| Location | Line | Issue | Severity |
|----------|------|-------|----------|
| main.go | 122 | `./config` hardcoded default path | LOW |
| main.go | 649-651 | Recovery defaults (300s, 3, 3) hardcoded | LOW |
| config.go | 283-319 | Large default config block | LOW |
| config.go | 428-429 | Protected branches default | LOW |
| config.go | 436-440 | HTTP allowlist default | LOW |
| config.go | 447-457 | Events config default | LOW |
| config.go | 464-469 | Sandbox config default | LOW |
| config.go | 474-481 | WebTools config default | LOW |
| config.go | 487-494 | Runtime config default | LOW |
| config.go | 500-501 | Returns "default" as fallback | LOW |
| config.go | 515-516 | Returns "internal" as fallback | LOW |
| config.go | 528-529 | Returns "internal" as fallback | LOW |
| router.go | 112, 120, 123 | Returns 0.5 as fallback score | LOW |
| router.go | 150, 155 | Returns "pause_and_alert" as fallback | LOW |
| gitree.go | 15 | DefaultGitTimeout = 60s | LOW |
| gitree.go | 154 | Commit message "task output" hardcoded | LOW |
| gitree.go | 249 | Commit message "clear for retry" hardcoded | LOW |
| github/client.go | 25-26 | Default repo "VibesTribe/VibePilot" | LOW |

**Assessment:** Hardcoding is minimal and all are reasonable defaults with config overrides. NOT A MAJOR ISSUE.

---

### 2. SECURITY ISSUES

| Location | Line | Issue | Severity |
|----------|------|-------|----------|
| gitree.go | 246 | `git clean -fd` deletes untracked files | MEDIUM |
| gitree.go | 254 | Force push `-f` could overwrite history | MEDIUM |

**Assessment:** These are intentional (clearing failed task branches) but should have safeguards. Could be dangerous in wrong directory.

---

### 3. CRITICAL FUNCTIONALITY GAPS

#### 3.1 Event Handlers Don't Parse Decisions

**Location:** main.go:186-538 (ALL event handlers)

**What they do:**
```go
result, err := session.Run(ctx, map[string]any{"task": task, "event": "task_review"})
log.Printf("[EventTaskReview] Task %s reviewed: %s", taskID, truncateOutput(result.Output))
return nil
```

**What they should do:**
```go
result, err := session.Run(ctx, input)

// Parse the decision
var decision struct {
    Action     string `json:"action"`
    Decision   string `json:"decision"`
    NextAction string `json:"next_action"`
}
json.Unmarshal([]byte(result.Output), &decision)

// Take action based on decision
switch decision.Decision {
case "pass":
    updateTaskStatus(ctx, taskID, "testing")
    triggerTests(ctx, taskID)
case "fail":
    wipeBranch(ctx, branchName)
    recordFailure(ctx, taskID, modelID, decision.Issues)
    reassignTask(ctx, taskID, differentModel)
case "reroute":
    escalateToPlanner(ctx, taskID, decision.ReturnFeedback)
}
```

**Impact:** ENTIRE SYSTEM DOESN'T WORK. Agents output decisions, Governor ignores them.

---

#### 3.2 No State Transitions

**Location:** main.go (missing)

**What's missing:**
- Function to update task status in Supabase
- Function to unlock dependent tasks
- Function to trigger tests after approval
- Function to merge branches after tests pass

**Required functions:**
```go
func updateTaskStatus(ctx context.Context, db *db.DB, taskID, status string) error
func unlockDependents(ctx context.Context, db *db.DB, taskID string) error
func triggerTests(ctx context.Context, taskID string) error
func mergeBranch(ctx context.Context, git *gitree.Gitree, source, target string) error
```

---

#### 3.3 No Rejection Handling

**Location:** main.go (missing)

**What's missing:**
- Branch wipe on rejection
- Failure counting per task
- Reassignment logic
- Escalation to Planner after N failures

**Required functions:**
```go
func handleRejection(ctx context.Context, task Task, decision SupervisorDecision) error {
    // 1. Wipe branch
    git.ClearBranch(ctx, branchName)
    
    // 2. Record failure
    failures := getFailureCount(task.ID)
    
    // 3. Decide next action
    if failures >= 3 {
        // Escalate to Planner
        escalateToPlanner(ctx, task, decision.Issues)
    } else {
        // Reassign to different model
        differentModel := selectDifferentModel(task.ModelID)
        reassignTask(ctx, task.ID, differentModel)
    }
}
```

---

#### 3.4 gitree.CommitOutput Never Called

**Location:** gitree.go (exists), main.go (not called)

**The function exists:**
```go
func (g *Gitree) CommitOutput(ctx context.Context, branchName string, output interface{}) error
```

**But it's never called from main.go.** Runner output is logged but not committed to GitHub.

---

#### 3.5 PRD Detection Not Wired

**Location:** github/client.go (exists), main.go (not wired)

**The client exists:**
```go
func (c *Client) ListPRDs(ctx context.Context, branch string) ([]File, error)
```

**But main.go never:**
- Creates a github client
- Polls for new PRDs
- Creates plan records in Supabase

---

### 4. WHAT THE PROMPTS EXPECT

The supervisor.md prompt defines output format:

```json
{
  "action": "task_review",
  "decision": "pass" | "fail" | "reroute",
  "next_action": "test" | "return_to_runner" | "split_task" | "escalate",
  "issues": [],
  "return_feedback": {...}
}
```

Governor should parse this and act on `decision` and `next_action`. It doesn't.

---

## ARCHITECTURE ASSESSMENT

### What's Well Designed

1. **Config system** - JSON-based, swappable, no hardcoding in logic
2. **Routing** - Strategy-based, configurable, uses learned scores
3. **Vault** - Secure, encrypted, proper audit logging
4. **gitree** - Good API, protected branches, clean functions
5. **Events** - Clean polling, configurable status filters

### What's Broken

1. **Decision loop** - Agents output JSON, Governor treats as text
2. **State machine** - No transitions, tasks never move forward
3. **Learning loop** - Recording happens but no action taken
4. **GitHub integration** - Code exists, not wired

---

## WHAT NEEDS TO BE BUILT

### Priority 1: Decision Parser

```go
func parseAgentOutput(output string) (*AgentDecision, error) {
    var decision AgentDecision
    if err := json.Unmarshal([]byte(output), &decision); err != nil {
        return nil, err
    }
    return &decision, nil
}
```

### Priority 2: State Transition Handler

```go
func handleSupervisorDecision(ctx context.Context, task Task, decision AgentDecision) error {
    switch decision.Decision {
    case "pass":
        return transitionToTesting(ctx, task)
    case "fail":
        return handleRejection(ctx, task, decision)
    case "reroute":
        return escalateToPlanner(ctx, task, decision)
    }
}
```

### Priority 3: Rejection Handler

```go
func handleRejection(ctx context.Context, task Task, decision AgentDecision) error {
    // Wipe branch
    // Count failures
    // Reassign or escalate
}
```

### Priority 4: GitHub Commit

```go
func commitTaskOutput(ctx context.Context, git *gitree.Gitree, taskID string, output string) error {
    return git.CommitOutput(ctx, "task/"+taskID, parseOutput(output))
}
```

### Priority 5: PRD Detection

```go
func detectNewPRDs(ctx context.Context, gh *github.Client, db *db.DB) error {
    files, _ := gh.ListPRDs(ctx, "main")
    for _, file := range files {
        if !planExists(file.Path) {
            createPlan(ctx, db, file.Path)
        }
    }
}
```

---

## LINE COUNT BY FILE

| File | Lines | Purpose |
|------|-------|---------|
| main.go | 726 | Entry point, event handlers |
| config.go | 585 | Config loading |
| events.go | 492 | Event polling |
| usage_tracker.go | 450 | Rate limiting |
| runners.go | 393 | CLI/API runners |
| maintenance.go | 346 | Git operations |
| vault.go | 337 | Secret encryption |
| gitree.go | 284 | Git utilities |
| db_tools.go | 255 | Database tools |
| validation.go | 248 | Input validation |
| courier.go | 239 | Web platform runner |
| sandbox_tools.go | 245 | Sandbox operations |
| web_tools.go | 231 | Web fetch tools |
| model_loader.go | 217 | Model config loading |
| supabase.go | 217 | Database client |
| parallel.go | 197 | Concurrency pool |
| git_tools.go | 185 | Git tools |
| sandbox.go | 165 | Sandbox runner |
| router.go | 158 | Destination routing |
| rpc.go | 152 | RPC allowlist |
| session.go | 140 | Agent sessions |
| tools.go | 136 | Tool registry |
| file_tools.go | 129 | File operations |
| types.go | 122 | Type definitions |
| registry.go | 84 | Tool registration |
| github/client.go | 94 | GitHub API client |
| leak_detector.go | 69 | Secret scanning |
| vault_tools.go | 41 | Vault tools |
| **TOTAL** | **6937** | |

---

## RECOMMENDATIONS

### Immediate (Next Session)

1. **Implement decision parser** - Parse agent JSON outputs
2. **Implement state transitions** - Update Supabase based on decisions
3. **Implement rejection handler** - Wipe, count, reassign, escalate
4. **Wire gitree.CommitOutput** - Actually commit runner output
5. **Wire PRD detection** - Poll GitHub, create plans

### Medium Term

1. Add rate limit checking to router
2. Add API output execution for API runners
3. Implement courier runner for web platforms
4. Add daily learning task

### Long Term

1. Pattern detection in failures
2. Agent prompt auto-updates with learnings
3. Full dashboard integration

---

## FILES TO CREATE

1. `governor/internal/runtime/decision.go` - Decision parsing
2. `governor/internal/runtime/transitions.go` - State transitions
3. `governor/internal/runtime/rejection.go` - Rejection handling
4. `governor/internal/runtime/prd_watcher.go` - PRD detection

## FILES TO MODIFY

1. `governor/cmd/governor/main.go` - Wire decision handling to all event handlers

---

## CONCLUSION

The infrastructure is solid. Config, routing, polling, vault, gitree - all well designed.

The brain is missing. Governor receives agent decisions but doesn't parse them or act on them.

**We need to build the decision loop, not more infrastructure.**

---

**End of Audit Report**
