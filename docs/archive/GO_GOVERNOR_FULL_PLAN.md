# Go Governor Full Architecture Plan

**Created:** 2026-02-23
**Purpose:** Complete Go Governor implementation for end-to-end task execution
**Goal:** One task from plan to merged, fully self-contained in Go

---

## SCHEMA CHANGES REQUIRED

### 1. Add `awaiting_human` Status

Current status values in schema:
```
pending, available, in_progress, review, testing, approval, merged, escalated
```

Need to add:
```
awaiting_human
```

**Migration file needed:** `docs/supabase-schema/022_add_awaiting_human.sql`

```sql
ALTER TABLE tasks DROP CONSTRAINT tasks_status_check;
ALTER TABLE tasks ADD CONSTRAINT tasks_status_check 
  CHECK (status IN (
    'pending', 'available', 'in_progress', 'review', 
    'testing', 'approval', 'merged', 'escalated', 'awaiting_human'
  ));
```

---

## COMPLETE TASK FLOW

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    TASK BECOMES AVAILABLE                                 │
│                         (status: available)                              │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                    ORCHESTRATOR (parallel assignments):
                    ┌───────────────┴───────────────┐
                    │                               │
                    ▼                               ▼
        ┌─────────────────────┐         ┌─────────────────────┐
        │      RUNNER         │         │     MAINTENANCE     │
        │  (Courier or        │         │      WORKER         │
        │   Maintenance       │         │                     │
        │   for internal)     │         │  Create branch:     │
        │                     │         │  task/T001-desc     │
        │  Execute task       │         │                     │
        │  Produce output     │         │                     │
        └─────────────────────┘         └─────────────────────┘
                    │                               │
                    │                               │ (branch created)
                    │                               │
                    ▼                               │
        Output returned to Orchestrator             │
                    │                               │
                    └───────────────┬───────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│              MAINTENANCE COMMITS OUTPUT TO BRANCH                        │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                      SUPERVISOR REVIEW                                   │
│                                                                          │
│  Input:                                                                  │
│  • Task packet (expected output from task_packets table)                │
│  • Actual output in branch (files created, changes made)                │
│                                                                          │
│  Checks:                                                                 │
│  • Output matches expected?                                              │
│  • All expected files present?                                           │
│  • No spaghetti code?                                                    │
│  • No injected dangers (secrets, malicious patterns)?                   │
│  • No truncation?                                                        │
│  • Scope creep detected?                                                 │
│                                                                          │
│  Decision (4 options ONLY):                                              │
│  ├── APPROVE → Route to Tester                                           │
│  ├── REJECT → Return to queue with notes                                 │
│  ├── COUNCIL → Call Council (for significant/complex decisions)         │
│  └── HUMAN → Set status to awaiting_human                                │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
            ┌───────────────────────┼───────────────────────┐
            │                       │                       │
            ▼                       ▼                       ▼
       [APPROVE]               [REJECT]                [COUNCIL]
            │                       │                       │
            ▼                       ▼                       ▼
┌─────────────────────┐   ┌─────────────────┐   ┌─────────────────────┐
│      TESTER         │   │ Return to       │   │      COUNCIL        │
│                     │   │ queue with      │   │                     │
│  Run:               │   │ notes           │   │  Multi-model        │
│  • pytest           │   │                 │   │  deliberation       │
│  • lint             │   │ Attempts++      │   │  3+ lenses          │
│  • typecheck        │   │                 │   │  Iterative rounds   │
│                     │   │ If attempts >=  │   │                     │
│  Does NOT fix       │   │ max_attempts:   │   │  Returns:           │
│  Only reports       │   │ → escalated     │   │  • approved         │
│                     │   │                 │   │  • concerns         │
└─────────────────────┘   └─────────────────┘   │  • recommendations  │
            │                                     └─────────────────────┘
            │                                               │
    ┌───────┴───────┐                                       │
    │               │                                       │
    ▼               ▼                                       ▼
  PASS            FAIL                            [COUNCIL RESULT]
    │               │                                       │
    │               │                              ┌────────┴────────┐
    │               └──────► Return to queue       │                 │
    │                       with notes             │ APPROVED        │
    │                                              │                 │
    │                                              └──────┬──────────┘
    │                                                     │
    ▼                                                     │
┌─────────────────────────────────────────────────────────┴───────────┐
│                    SUPERVISOR MERGE DECISION                          │
│                                                                       │
│  If task type = ui_ux:                                               │
│    → Must go to HUMAN first (awaiting_human)                         │
│    → Human approves → Merge                                          │
│                                                                       │
│  If normal task + tests pass:                                        │
│    → Approve merge → Task complete                                   │
└───────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                      MAINTENANCE MERGE                                  │
│                                                                         │
│  1. Merge task/T001-desc → module/feature (if module exists)           │
│     OR                                                                  │
│     Merge task/T001-desc → main (if no module)                         │
│  2. Delete task branch                                                 │
│  3. Update task status → merged                                        │
│  4. Unlock dependent tasks                                             │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
                            TASK COMPLETE
                           (status: merged)
```

---

## STATUS FLOW DIAGRAM

```
                    ┌─────────────┐
                    │   pending   │  (created, dependencies not met)
                    └──────┬──────┘
                           │ (dependencies satisfied)
                           ▼
                    ┌─────────────┐
                    │  available  │  (ready for assignment)
                    └──────┬──────┘
                           │ (orchestrator assigns)
                           ▼
                ┌─────────────────────┐
                │    in_progress      │  (runner executing)
                └──────────┬──────────┘
                           │ (runner completes)
                           ▼
                    ┌─────────────┐
                    │   review    │  (supervisor reviewing)
                    └──────┬──────┘
                           │
           ┌───────────────┼───────────────┬───────────────┐
           │               │               │               │
           ▼               ▼               ▼               ▼
    ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────────┐
    │  rejected  │  │  testing   │  │  council   │  │  awaiting  │
    │            │  │            │  │  review    │  │  _human    │
    └─────┬──────┘  └─────┬──────┘  └─────┬──────┘  └─────┬──────┘
          │               │               │               │
          │               │               │               │
          │       ┌───────┴───────┐       │               │
          │       │               │       │               │
          │       ▼               ▼       │               │
          │    [PASS]          [FAIL]     │               │
          │       │               │       │               │
          │       │               └──► (back to queue)    │
          │       │                       │               │
          │       ▼                       ▼               │
          │  ┌────────────┐         ┌──────────┐         │
          │  │  approval  │         │ council  │         │
          │  │            │◄────────│ approved │         │
          │  └─────┬──────┘         └──────────┘         │
          │        │                                     │
          │        ▼                                     │
          │  ┌──────────┐                                │
          │  │  merged  │◄───────────────────────────────┘
          │  │          │  (human approved)
          │  └──────────┘
          │
          ▼
    (back to available
     with notes)
          
          │
          ▼ (if attempts >= max)
    ┌────────────┐
    │ escalated  │  (AI solves, not human)
    └────────────┘
```

---

## GO GOVERNOR COMPONENTS

### Directory Structure

```
governor/
├── cmd/governor/main.go
├── internal/
│   ├── orchestrator/
│   │   ├── orchestrator.go      # The brain - coordinates everything
│   │   ├── router.go            # Routes tasks to correct agents
│   │   ├── state.go             # Tracks task states in memory
│   │   ├── learner.go           # Learns from results, optimizes routing
│   │   └── council_router.go    # Routes council reviews to models
│   │
│   ├── sentry/
│   │   └── sentry.go            # Polls for available tasks ✅ EXISTS
│   │
│   ├── dispatcher/
│   │   └── dispatcher.go        # Dispatches to runners ✅ EXISTS
│   │
│   ├── maintenance/
│   │   ├── maintenance.go       # Main executor for internal tasks
│   │   ├── git.go               # Git operations (branch, commit, merge, delete)
│   │   ├── worker.go            # Worker pool for parallel execution
│   │   └── executor.go          # Task execution logic
│   │
│   ├── supervisor/
│   │   ├── supervisor.go        # Quality reviewer
│   │   ├── review.go            # Review logic
│   │   └── decision.go          # Decision logic (approve/reject/council/human)
│   │
│   ├── council/
│   │   ├── council.go           # Multi-model deliberation
│   │   ├── lenses.go            # Different review perspectives
│   │   └── consensus.go         # Consensus building
│   │
│   ├── tester/
│   │   ├── tester.go            # Test executor
│   │   └── executor.go          # Runs pytest, lint, typecheck
│   │
│   ├── courier/                 # Web platform dispatch ✅ EXISTS
│   │   ├── dispatcher.go
│   │   └── webhook.go
│   │
│   ├── db/
│   │   └── supabase.go          # Database client ✅ EXISTS
│   │
│   ├── pool/
│   │   └── model_pool.go        # Model selection ✅ EXISTS
│   │
│   ├── janitor/
│   │   └── janitor.go           # Cleanup stuck tasks ✅ EXISTS
│   │
│   ├── server/
│   │   ├── server.go            # HTTP API ✅ EXISTS
│   │   └── hub.go               # WebSocket ✅ EXISTS
│   │
│   ├── throttle/
│   │   └── module_limiter.go    # Rate limiting ✅ EXISTS
│   │
│   ├── config/
│   │   └── config.go            # Configuration ✅ EXISTS
│   │
│   └── security/
│       ├── leak_detector.go     # Secret scanning ✅ EXISTS
│       └── allowlist.go         # URL allowlist ✅ EXISTS
│
├── pkg/types/
│   └── types.go                 # Shared types ✅ EXISTS
│
├── go.mod
├── go.sum
├── governor.yaml
└── Makefile
```

---

## DATA STRUCTURES

### Task State (pkg/types/types.go additions)

```go
type TaskState struct {
    TaskID         string        `json:"task_id"`
    Status         string        `json:"status"`
    AssignedRunner string        `json:"assigned_runner"`
    BranchName     string        `json:"branch_name"`
    Attempts       int           `json:"attempts"`
    MaxAttempts    int           `json:"max_attempts"`
    ReviewResult   *ReviewResult `json:"review_result,omitempty"`
    TestResult     *TestResult   `json:"test_result,omitempty"`
    CouncilResult  *CouncilResult `json:"council_result,omitempty"`
}

type ReviewResult struct {
    Decision    string   `json:"decision"` // approve, reject, council, human
    Notes       string   `json:"notes"`
    Issues      []string `json:"issues"`
    ApprovedBy  string   `json:"approved_by"`
}

type TestResult struct {
    Passed      bool     `json:"passed"`
    Failures    []string `json:"failures"`
    Coverage    float64  `json:"coverage"`
    TestedBy    string   `json:"tested_by"`
}

type CouncilResult struct {
    Approved        bool              `json:"approved"`
    Consensus       string            `json:"consensus"` // unanimous, majority, split
    Reviews         map[string]Review `json:"reviews"`
    Concerns        []string          `json:"concerns"`
    Recommendations []string          `json:"recommendations"`
    Rounds          int               `json:"rounds"`
}

type Review struct {
    ModelID    string   `json:"model_id"`
    Lens       string   `json:"lens"`
    Vote       string   `json:"vote"` // approve, reject, needs_changes
    Concerns   []string `json:"concerns"`
    Confidence float64  `json:"confidence"`
}
```

---

## ORCHESTRATOR RESPONSIBILITIES

### Main Loop

```go
func (o *Orchestrator) Run(ctx context.Context) {
    for {
        select {
        case task := <-o.availableTasks:
            o.assignTask(task)
            
        case result := <-o.runnerResults:
            o.handleRunnerResult(result)
            
        case result := <-o.reviewResults:
            o.handleReviewResult(result)
            
        case result := <-o.testResults:
            o.handleTestResult(result)
            
        case result := <-o.councilResults:
            o.handleCouncilResult(result)
            
        case <-ctx.Done():
            return
        }
    }
}
```

### Assignment Logic

```go
func (o *Orchestrator) assignTask(task Task) {
    // 1. Determine routing
    routing := o.determineRouting(task)
    
    // 2. Assign to maintenance to create branch (parallel)
    go o.maintenance.CreateBranch(task.ID, task.Title)
    
    // 3. Assign to runner
    switch routing {
    case "internal":
        o.maintenance.Execute(task)
    case "web":
        o.courier.Dispatch(task)
    }
    
    // 4. Update state
    o.state.SetStatus(task.ID, "in_progress")
}
```

### Result Handling

```go
func (o *Orchestrator) handleRunnerResult(result RunnerResult) {
    // 1. Commit output to branch
    o.maintenance.CommitToBranch(result.TaskID, result.Output)
    
    // 2. Route to supervisor
    o.state.SetStatus(result.TaskID, "review")
    go o.supervisor.Review(result.TaskID)
}

func (o *Orchestrator) handleReviewResult(result ReviewResult) {
    switch result.Decision {
    case "approve":
        o.state.SetStatus(result.TaskID, "testing")
        go o.tester.Test(result.TaskID)
        
    case "reject":
        o.handleRejection(result.TaskID, result.Notes)
        
    case "council":
        go o.council.Deliberate(result.TaskID)
        
    case "human":
        o.state.SetStatus(result.TaskID, "awaiting_human")
    }
}

func (o *Orchestrator) handleTestResult(result TestResult) {
    if result.Passed {
        o.supervisor.ApproveMerge(result.TaskID)
    } else {
        o.handleRejection(result.TaskID, result.Failures)
    }
}
```

---

## SUPERVISOR RESPONSIBILITIES

### Review Logic

```go
func (s *Supervisor) Review(taskID string) ReviewResult {
    // 1. Get task packet (expected output)
    packet := s.db.GetTaskPacket(taskID)
    
    // 2. Get actual output from branch
    output := s.maintenance.ReadBranchOutput(taskID)
    
    // 3. Quality checks
    issues := []string{}
    
    // Check: expected files present
    if missing := checkExpectedFiles(packet, output); len(missing) > 0 {
        issues = append(issues, fmt.Sprintf("Missing files: %v", missing))
    }
    
    // Check: no injected secrets
    if secrets := s.security.Scan(output); len(secrets) > 0 {
        issues = append(issues, fmt.Sprintf("Secrets detected: %v", secrets))
    }
    
    // Check: no truncation
    if isTruncated(output) {
        issues = append(issues, "Output appears truncated")
    }
    
    // Check: output matches expectations
    if !matchesExpectations(packet, output) {
        issues = append(issues, "Output does not match expectations")
    }
    
    // 4. Decision
    if len(issues) > 0 {
        return ReviewResult{Decision: "reject", Issues: issues}
    }
    
    // 5. Check if needs human (UI/UX)
    task := s.db.GetTask(taskID)
    if task.Type == "ui_ux" {
        return ReviewResult{Decision: "human", Notes: "UI/UX requires human approval"}
    }
    
    // 6. Check if needs council (significant change)
    if s.needsCouncil(task) {
        return ReviewResult{Decision: "council", Notes: "Significant change requires council review"}
    }
    
    // 7. Approve
    return ReviewResult{Decision: "approve"}
}
```

---

## MAINTENANCE RESPONSIBILITIES

### Git Operations

```go
func (m *Maintenance) CreateBranch(taskID, taskTitle string) error {
    branchName := fmt.Sprintf("task/%s-%s", taskID[:8], slugify(taskTitle))
    
    // git checkout -b task/T001-desc
    cmd := exec.Command("git", "checkout", "-b", branchName, "main")
    if err := cmd.Run(); err != nil {
        return err
    }
    
    // git push -u origin task/T001-desc
    cmd = exec.Command("git", "push", "-u", "origin", branchName)
    return cmd.Run()
}

func (m *Maintenance) CommitToBranch(taskID string, output Output) error {
    // Write files
    for _, file := range output.Files {
        os.WriteFile(file.Path, []byte(file.Content), 0644)
    }
    
    // git add .
    exec.Command("git", "add", ".").Run()
    
    // git commit -m "task: T001 description"
    exec.Command("git", "commit", "-m", fmt.Sprintf("task: %s", taskID[:8])).Run()
    
    // git push
    return exec.Command("git", "push").Run()
}

func (m *Maintenance) Merge(taskID, targetBranch string) error {
    branchName := m.getBranchName(taskID)
    
    // git checkout target
    exec.Command("git", "checkout", targetBranch).Run()
    
    // git merge branch
    if err := exec.Command("git", "merge", branchName).Run(); err != nil {
        return err
    }
    
    // git push
    exec.Command("git", "push").Run()
    
    // git branch -d branch
    exec.Command("git", "branch", "-d", branchName).Run()
    
    // git push origin --delete branch
    return exec.Command("git", "push", "origin", "--delete", branchName).Run()
}
```

---

## TESTER RESPONSIBILITIES

```go
func (t *Tester) Test(taskID string) TestResult {
    branch := t.maintenance.GetBranchName(taskID)
    
    // Checkout branch
    exec.Command("git", "checkout", branch).Run()
    
    failures := []string{}
    
    // Run pytest
    if output, err := exec.Command("pytest", "--tb=short").CombinedOutput(); err != nil {
        failures = append(failures, string(output))
    }
    
    // Run lint
    if output, err := exec.Command("ruff", "check", ".").CombinedOutput(); err != nil {
        failures = append(failures, string(output))
    }
    
    // Run typecheck
    if output, err := exec.Command("mypy", ".").CombinedOutput(); err != nil {
        failures = append(failures, string(output))
    }
    
    return TestResult{
        Passed:   len(failures) == 0,
        Failures: failures,
        TestedBy: "tester",
    }
}
```

---

## COUNCIL RESPONSIBILITIES

### Deliberation Process

```go
func (c *Council) Deliberate(taskID string) CouncilResult {
    task := c.db.GetTask(taskID)
    packet := c.db.GetTaskPacket(taskID)
    
    lenses := c.determineLenses(task)
    
    // Get available models
    models := c.pool.GetAvailableModels(len(lenses))
    
    // Round 1: Independent reviews
    reviews := make(map[string]Review)
    for i, lens := range lenses {
        model := models[i % len(models)]
        review := c.executeReview(model, lens, task, packet)
        reviews[model] = review
    }
    
    // Check consensus
    if c.hasConsensus(reviews) {
        return c.buildResult(reviews, 1)
    }
    
    // Rounds 2-4: Deliberation with visibility
    for round := 2; round <= 4; round++ {
        // Share previous reviews with all models
        for _, model := range models {
            review := c.executeReviewWithHistory(model, lenses, task, packet, reviews)
            reviews[model] = review
        }
        
        if c.hasConsensus(reviews) {
            return c.buildResult(reviews, round)
        }
    }
    
    // No consensus after 4 rounds - escalate to human
    return CouncilResult{
        Approved:  false,
        Consensus: "split",
        Rounds:    4,
    }
}
```

---

## IMPLEMENTATION PHASES

### Phase 4A: Schema + Core Orchestrator
1. Add `awaiting_human` to schema (migration)
2. Orchestrator core (routing, state)
3. Wire orchestrator into main.go

### Phase 4B: Maintenance (Git)
1. Create branch function
2. Commit to branch function
3. Merge function
4. Delete branch function
5. Worker pool for parallel execution

### Phase 4C: Supervisor
1. Review logic
2. Decision logic (approve/reject/council/human)
3. Quality checks (secrets, truncation, expected files)

### Phase 4D: Tester
1. pytest execution
2. lint execution
3. typecheck execution
4. Result parsing

### Phase 4E: Council (Simplified First)
1. Single model with multiple lenses (sequential)
2. Later: Multi-model parallel deliberation

### Phase 4F: Wire Everything
1. Orchestrator coordinates all components
2. Status transitions work end-to-end
3. Branch lifecycle complete

### Phase 4G: Test End-to-End
1. Create test task
2. Run through entire pipeline
3. Verify: available → in_progress → review → testing → merged

---

## FILES TO CREATE (in order)

1. `docs/supabase-schema/022_add_awaiting_human.sql`
2. `governor/pkg/types/task_state.go`
3. `governor/internal/orchestrator/orchestrator.go`
4. `governor/internal/orchestrator/router.go`
5. `governor/internal/orchestrator/state.go`
6. `governor/internal/maintenance/maintenance.go`
7. `governor/internal/maintenance/git.go`
8. `governor/internal/maintenance/worker.go`
9. `governor/internal/supervisor/supervisor.go`
10. `governor/internal/supervisor/review.go`
11. `governor/internal/supervisor/decision.go`
12. `governor/internal/tester/tester.go`
13. `governor/internal/tester/executor.go`
14. `governor/internal/council/council.go`
15. `governor/internal/council/lenses.go`

---

## QUESTIONS / DECISIONS NEEDED

1. **Test command configuration** - Should test commands (pytest, ruff, mypy) be configurable per project, or hardcoded for now?

2. **Council model selection** - When Council is called, should it use the same models as task execution, or have a separate pool of "council-capable" models?

3. **Branch naming** - Is `task/T001-desc` the right format? Should it include slice/module info?

4. **Error handling** - If git operations fail, what should happen? Retry, escalate, notify?

---

**END OF PLAN**
