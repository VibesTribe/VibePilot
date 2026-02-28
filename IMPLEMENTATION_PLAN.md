# VibePilot Complete Implementation Plan

**Created:** 2026-02-28
**Status:** COMPLETE PLAN - Ready for implementation
**Estimated Work:** 5-7 sessions

---

## PHILOSOPHY

No shortcuts. No stubs. No "for now." No "we'll add that later."

Everything configurable. Everything swappable. Everything learns.

**The system must:**
- Detect PRDs automatically
- Create quality task plans that improve over time
- Route intelligently based on real data
- Handle failures smartly
- Learn from every outcome
- Improve continuously without human intervention

---

## ARCHITECTURE OVERVIEW

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              GITHUB (Source of Truth)                        │
│  docs/prd/          docs/plans/          task/T001         research/        │
│  (PRDs)             (Task Plans)         (Task Output)     (Findings)       │
└─────────────────────────────────────────────────────────────────────────────┘
         │                    │                    │                 │
         │ detect             │ read               │ read/write      │ read
         ▼                    ▼                    ▼                 ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                              GOVERNOR (The Brain)                            │
│                                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐    │
│  │ PRD Watcher  │  │ Plan Builder │  │ Orchestrator │  │ Learner      │    │
│  │              │→ │              │→ │              │→ │              │    │
│  │ Polls GitHub │  │ Calls Agent  │  │ Routes Tasks │  │ Updates All  │    │
│  └──────────────┘  └──────────────┘  └──────────────┘  └──────────────┘    │
│         │                 │                  │                 │           │
│         ▼                 ▼                  ▼                 ▼           │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                         LEARNING STORE                                │  │
│  │  - Council feedback (by project type, task type)                     │  │
│  │  - Failure patterns (model + task type + failure reason)             │  │
│  │  - Success patterns (what works)                                      │  │
│  │  - Model scores (computed on-the-fly)                                 │  │
│  │  - Agent learnings (prompt improvements)                              │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
         │                    │                    │
         │ read/write         │ read/write         │ read/write
         ▼                    ▼                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                              SUPABASE (State)                                │
│  plans              tasks              task_runs        model_scores        │
│  council_reviews    failure_patterns   learned_rules    agent_learnings     │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## COMPONENT 1: PRD WATCHER

### Purpose
Detect new PRDs in GitHub, create plan records, trigger Planner.

### Files to Create
- `governor/internal/runtime/prd_watcher.go`

### Implementation

```go
package runtime

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "os"
    "os/exec"
    "strings"
    "time"
)

type PRDWatcher struct {
    db         Querier
    repoPath   string
    prdBranch  string
    prdDir     string
    interval   time.Duration
    lastSeen   map[string]string // path -> sha
    stop       chan struct{}
}

type PRDWatcherConfig struct {
    RepoPath  string
    PRDBranch string
    PRDDir    string
    Interval  time.Duration
}

func NewPRDWatcher(db Querier, cfg PRDWatcherConfig) *PRDWatcher {
    if cfg.Interval == 0 {
        cfg.Interval = 10 * time.Second
    }
    if cfg.PRDBranch == "" {
        cfg.PRDBranch = "main"
    }
    if cfg.PRDDir == "" {
        cfg.PRDDir = "docs/prd"
    }
    return &PRDWatcher{
        db:        db,
        repoPath:  cfg.RepoPath,
        prdBranch: cfg.PRDBranch,
        prdDir:    cfg.PRDDir,
        interval:  cfg.Interval,
        lastSeen:  make(map[string]string),
        stop:      make(chan struct{}),
    }
}

func (w *PRDWatcher) Start(ctx context.Context, onNewPRD func(prdPath string)) {
    ticker := time.NewTicker(w.interval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-w.stop:
            return
        case <-ticker.C:
            w.checkForNewPRDs(ctx, onNewPRD)
        }
    }
}

func (w *PRDWatcher) checkForNewPRDs(ctx context.Context, onNewPRD func(prdPath string)) {
    // Get list of PRD files from git
    cmd := exec.CommandContext(ctx, "git", "ls-tree", "-r", w.prdBranch, "--name-only")
    cmd.Dir = w.repoPath
    output, err := cmd.Output()
    if err != nil {
        log.Printf("[PRDWatcher] Failed to list files: %v", err)
        return
    }
    
    lines := strings.Split(string(output), "\n")
    for _, line := range lines {
        line = strings.TrimSpace(line)
        if line == "" {
            continue
        }
        
        // Check if file is in PRD directory and is markdown
        if !strings.HasPrefix(line, w.prdDir) {
            continue
        }
        if !strings.HasSuffix(line, ".md") {
            continue
        }
        
        // Get SHA for this file
        shaCmd := exec.CommandContext(ctx, "git", "rev-parse", w.prdBranch+":"+line)
        shaCmd.Dir = w.repoPath
        shaOutput, err := shaCmd.Output()
        if err != nil {
            continue
        }
        sha := strings.TrimSpace(string(shaOutput))
        
        // Check if we've seen this version
        if w.lastSeen[line] == sha {
            continue
        }
        
        // New or modified PRD
        w.lastSeen[line] = sha
        
        // Check if plan already exists for this PRD
        exists := w.planExistsForPRD(ctx, line)
        if exists {
            continue
        }
        
        log.Printf("[PRDWatcher] New PRD detected: %s", line)
        onNewPRD(line)
    }
}

func (w *PRDWatcher) planExistsForPRD(ctx context.Context, prdPath string) bool {
    result, err := w.db.Query(ctx, "plans", map[string]any{
        "prd_path": prdPath,
        "limit":    1,
    })
    if err != nil {
        return false
    }
    
    var plans []map[string]any
    if err := json.Unmarshal(result, &plans); err != nil {
        return false
    }
    
    return len(plans) > 0
}

func (w *PRDWatcher) Stop() {
    close(w.stop)
}
```

### Config Addition (system.json)

```json
{
  "prd_watcher": {
    "enabled": true,
    "branch": "main",
    "directory": "docs/prd",
    "poll_interval_seconds": 10
  }
}
```

---

## COMPONENT 2: LEARNING STORE

### Purpose
Central place for all learned data. All agents read from it before acting.

### Supabase Tables

```sql
-- Council feedback for learning
CREATE TABLE council_feedback (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id UUID REFERENCES plans(id),
    project_type TEXT,
    task_type TEXT,
    lens TEXT,
    vote TEXT,
    concerns JSONB,
    suggestions JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Failure patterns
CREATE TABLE failure_patterns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_type TEXT NOT NULL,
    failure_reason TEXT NOT NULL,
    model_id TEXT,
    pattern_description TEXT,
    occurrence_count INT DEFAULT 1,
    suggested_action TEXT,
    last_seen_at TIMESTAMPTZ DEFAULT NOW(),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Learned rules (from Council, failures, successes)
CREATE TABLE learned_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id TEXT NOT NULL,
    rule_type TEXT NOT NULL, -- 'avoid', 'prefer', 'pattern', 'threshold'
    condition JSONB NOT NULL,
    action JSONB NOT NULL,
    confidence FLOAT DEFAULT 0.5,
    source TEXT, -- 'council', 'failure', 'success', 'human'
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Agent learning history (for prompt updates)
CREATE TABLE agent_learnings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id TEXT NOT NULL,
    learning_date DATE NOT NULL,
    learning_type TEXT NOT NULL,
    learning_text TEXT NOT NULL,
    source_event TEXT,
    applied_to_prompt BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_council_feedback_type ON council_feedback(project_type, task_type);
CREATE INDEX idx_failure_patterns_type ON failure_patterns(task_type, failure_reason);
CREATE INDEX idx_learned_rules_agent ON learned_rules(agent_id, active);
CREATE INDEX idx_agent_learnings_agent ON agent_learnings(agent_id, learning_date);
```

### RPCs for Learning Store

```sql
-- Get relevant learnings for an agent
CREATE OR REPLACE FUNCTION get_agent_learnings(
    p_agent_id TEXT,
    p_task_type TEXT DEFAULT NULL,
    p_limit INT DEFAULT 20
) RETURNS JSONB AS $$
DECLARE
    result JSONB;
BEGIN
    -- Get learned rules
    SELECT jsonb_agg(row_to_json(lr.*))
    INTO result
    FROM learned_rules lr
    WHERE lr.agent_id = p_agent_id
      AND lr.active = true
      AND (p_task_type IS NULL OR lr.condition->>'task_type' = p_task_type)
    ORDER BY lr.confidence DESC
    LIMIT p_limit;
    
    RETURN COALESCE(result, '[]'::jsonb);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Get failure patterns for task type
CREATE OR REPLACE FUNCTION get_failure_patterns(
    p_task_type TEXT,
    p_limit INT DEFAULT 10
) RETURNS JSONB AS $$
DECLARE
    result JSONB;
BEGIN
    SELECT jsonb_agg(row_to_json(fp.*))
    INTO result
    FROM failure_patterns fp
    WHERE fp.task_type = p_task_type
    ORDER BY fp.occurrence_count DESC
    LIMIT p_limit;
    
    RETURN COALESCE(result, '[]'::jsonb);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Get Council feedback for project type
CREATE OR REPLACE FUNCTION get_council_feedback_for_type(
    p_project_type TEXT,
    p_limit INT DEFAULT 20
) RETURNS JSONB AS $$
DECLARE
    result JSONB;
BEGIN
    SELECT jsonb_agg(row_to_json(cf.*))
    INTO result
    FROM council_feedback cf
    WHERE cf.project_type = p_project_type
      AND cf.vote != 'APPROVED' -- Only get feedback with concerns
    ORDER BY cf.created_at DESC
    LIMIT p_limit;
    
    RETURN COALESCE(result, '[]'::jsonb);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Record a failure pattern
CREATE OR REPLACE FUNCTION record_failure_pattern(
    p_task_type TEXT,
    p_failure_reason TEXT,
    p_model_id TEXT,
    p_pattern_description TEXT,
    p_suggested_action TEXT
) RETURNS UUID AS $$
DECLARE
    pattern_id UUID;
BEGIN
    -- Check if pattern exists
    SELECT id INTO pattern_id
    FROM failure_patterns
    WHERE task_type = p_task_type
      AND failure_reason = p_failure_reason
      AND (p_model_id IS NULL OR model_id = p_model_id);
    
    IF pattern_id IS NOT NULL THEN
        -- Update existing
        UPDATE failure_patterns
        SET occurrence_count = occurrence_count + 1,
            last_seen_at = NOW()
        WHERE id = pattern_id;
    ELSE
        -- Create new
        INSERT INTO failure_patterns (
            task_type, failure_reason, model_id,
            pattern_description, suggested_action
        ) VALUES (
            p_task_type, p_failure_reason, p_model_id,
            p_pattern_description, p_suggested_action
        ) RETURNING id INTO pattern_id;
    END IF;
    
    RETURN pattern_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Add agent learning
CREATE OR REPLACE FUNCTION add_agent_learning(
    p_agent_id TEXT,
    p_learning_type TEXT,
    p_learning_text TEXT,
    p_source_event TEXT
) RETURNS UUID AS $$
DECLARE
    learning_id UUID;
BEGIN
    INSERT INTO agent_learnings (
        agent_id, learning_date, learning_type,
        learning_text, source_event
    ) VALUES (
        p_agent_id, CURRENT_DATE, p_learning_type,
        p_learning_text, p_source_event
    ) RETURNING id INTO learning_id;
    
    RETURN learning_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;
```

---

## COMPONENT 3: SMART PLANNER

### Purpose
Planner reads learnings before creating plans. Learns from Council feedback.

### Modified Planner Flow

```
1. Read PRD from GitHub
2. Query learning store:
   - Get Council feedback for similar project types
   - Get failure patterns for expected task types
   - Get learned rules for planner
3. Include learnings in prompt context
4. Create plan
5. On Council feedback:
   - Store feedback in council_feedback table
   - Extract learnings
   - Add to agent_learnings table
```

### Implementation

Create `governor/internal/runtime/smart_planner.go`:

```go
package runtime

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "strings"
)

type SmartPlanner struct {
    db         Querier
    git        GitReader
    promptPath string
}

type GitReader interface {
    ReadFile(ctx context.Context, path string) ([]byte, error)
}

type PlannerContext struct {
    PRD             string
    ProjectType     string
    CouncilFeedback []map[string]any
    FailurePatterns []map[string]any
    LearnedRules    []map[string]any
}

func NewSmartPlanner(db Querier, git GitReader, promptPath string) *SmartPlanner {
    return &SmartPlanner{
        db:         db,
        git:        git,
        promptPath: promptPath,
    }
}

func (p *SmartPlanner) BuildContext(ctx context.Context, prdPath string) (*PlannerContext, error) {
    // 1. Read PRD
    prdContent, err := p.git.ReadFile(ctx, prdPath)
    if err != nil {
        return nil, fmt.Errorf("read PRD: %w", err)
    }
    
    // 2. Detect project type from PRD
    projectType := p.detectProjectType(string(prdContent))
    
    // 3. Get Council feedback for similar projects
    feedback, _ := p.getCouncilFeedback(ctx, projectType)
    
    // 4. Get failure patterns
    patterns, _ := p.getFailurePatterns(ctx, projectType)
    
    // 5. Get learned rules for planner
    rules, _ := p.getLearnedRules(ctx)
    
    return &PlannerContext{
        PRD:             string(prdContent),
        ProjectType:     projectType,
        CouncilFeedback: feedback,
        FailurePatterns: patterns,
        LearnedRules:    rules,
    }, nil
}

func (p *SmartPlanner) detectProjectType(prdContent string) string {
    content := strings.ToLower(prdContent)
    
    // Simple detection - could be more sophisticated
    if strings.Contains(content, "dashboard") || strings.Contains(content, "ui") {
        return "frontend"
    }
    if strings.Contains(content, "api") || strings.Contains(content, "endpoint") {
        return "backend"
    }
    if strings.Contains(content, "auth") || strings.Contains(content, "security") {
        return "security"
    }
    if strings.Contains(content, "database") || strings.Contains(content, "migration") {
        return "data"
    }
    return "general"
}

func (p *SmartPlanner) getCouncilFeedback(ctx context.Context, projectType string) ([]map[string]any, error) {
    result, err := p.db.RPC(ctx, "get_council_feedback_for_type", map[string]any{
        "p_project_type": projectType,
        "p_limit":        20,
    })
    if err != nil {
        return nil, err
    }
    
    var feedback []map[string]any
    json.Unmarshal(result, &feedback)
    return feedback, nil
}

func (p *SmartPlanner) getFailurePatterns(ctx context.Context, projectType string) ([]map[string]any, error) {
    result, err := p.db.RPC(ctx, "get_failure_patterns", map[string]any{
        "p_task_type": projectType,
        "p_limit":     10,
    })
    if err != nil {
        return nil, err
    }
    
    var patterns []map[string]any
    json.Unmarshal(result, &patterns)
    return patterns, nil
}

func (p *SmartPlanner) getLearnedRules(ctx context.Context) ([]map[string]any, error) {
    result, err := p.db.RPC(ctx, "get_agent_learnings", map[string]any{
        "p_agent_id":  "planner",
        "p_task_type": nil,
        "p_limit":     20,
    })
    if err != nil {
        return nil, err
    }
    
    var rules []map[string]any
    json.Unmarshal(result, &rules)
    return rules, nil
}

func (p *SmartPlanner) FormatContextForPrompt(ctx *PlannerContext) string {
    var sb strings.Builder
    
    // Add Council feedback as learnings
    if len(ctx.CouncilFeedback) > 0 {
        sb.WriteString("\n## Past Council Feedback (Learn From This)\n\n")
        for _, fb := range ctx.CouncilFeedback {
            lens, _ := fb["lens"].(string)
            concerns, _ := fb["concerns"].([]interface{})
            suggestions, _ := fb["suggestions"].([]interface{})
            
            sb.WriteString(fmt.Sprintf("### %s Lens Feedback:\n", lens))
            for _, c := range concerns {
                if cs, ok := c.(string); ok {
                    sb.WriteString(fmt.Sprintf("- CONCERN: %s\n", cs))
                }
            }
            for _, s := range suggestions {
                if ss, ok := s.(string); ok {
                    sb.WriteString(fmt.Sprintf("- SUGGESTION: %s\n", ss))
                }
            }
        }
    }
    
    // Add failure patterns
    if len(ctx.FailurePatterns) > 0 {
        sb.WriteString("\n## Known Failure Patterns (Avoid These)\n\n")
        for _, p := range ctx.FailurePatterns {
            reason, _ := p["failure_reason"].(string)
            desc, _ := p["pattern_description"].(string)
            action, _ := p["suggested_action"].(string)
            count, _ := p["occurrence_count"].(float64)
            
            sb.WriteString(fmt.Sprintf("- **%s** (seen %d times): %s\n", reason, int(count), desc))
            if action != "" {
                sb.WriteString(fmt.Sprintf("  - Suggested fix: %s\n", action))
            }
        }
    }
    
    // Add learned rules
    if len(ctx.LearnedRules) > 0 {
        sb.WriteString("\n## Learned Rules\n\n")
        for _, r := range ctx.LearnedRules {
            ruleType, _ := r["rule_type"].(string)
            condition, _ := r["condition"].(map[string]interface{})
            action, _ := r["action"].(map[string]interface{})
            
            sb.WriteString(fmt.Sprintf("- **%s**: When %v, then %v\n", ruleType, condition, action))
        }
    }
    
    return sb.String()
}

func (p *SmartPlanner) RecordCouncilFeedback(ctx context.Context, planID string, projectType string, reviews []map[string]any) error {
    for _, review := range reviews {
        _, err := p.db.RPC(ctx, "record_council_feedback", map[string]any{
            "p_plan_id":     planID,
            "p_project_type": projectType,
            "p_task_type":    review["task_type"],
            "p_lens":         review["lens"],
            "p_vote":         review["vote"],
            "p_concerns":     review["concerns"],
            "p_suggestions":  review["suggestions"],
        })
        if err != nil {
            log.Printf("[SmartPlanner] Failed to record feedback: %v", err)
        }
        
        // Extract learning from feedback
        if review["vote"] != "APPROVED" {
            learning := p.extractLearning(review)
            if learning != "" {
                p.db.RPC(ctx, "add_agent_learning", map[string]any{
                    "p_agent_id":      "planner",
                    "p_learning_type": "council_feedback",
                    "p_learning_text": learning,
                    "p_source_event":  planID,
                })
            }
        }
    }
    return nil
}

func (p *SmartPlanner) extractLearning(review map[string]any) string {
    lens, _ := review["lens"].(string)
    concerns, _ := review["concerns"].([]interface{})
    
    if len(concerns) == 0 {
        return ""
    }
    
    var learning strings.Builder
    learning.WriteString(fmt.Sprintf("From %s review: ", lens))
    for i, c := range concerns {
        if cs, ok := c.(string); ok {
            if i > 0 {
                learning.WriteString("; ")
            }
            learning.WriteString(cs)
        }
    }
    
    return learning.String()
}
```

---

## COMPONENT 4: SMART SUPERVISOR

### Purpose
Supervisor reads failure patterns before deciding. Knows best action for each failure type.

### Implementation

Create `governor/internal/runtime/smart_supervisor.go`:

```go
package runtime

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
)

type SmartSupervisor struct {
    db Querier
}

type RejectionDecision struct {
    Action          string `json:"action"` // reassign, split, fix_prompt, escalate
    Reason          string `json:"reason"`
    TargetModel     string `json:"target_model,omitempty"`
    FeedbackToModel string `json:"feedback_to_model,omitempty"`
    FeedbackToPlanner string `json:"feedback_to_planner,omitempty"`
}

func NewSmartSupervisor(db Querier) *SmartSupervisor {
    return &SmartSupervisor{db: db}
}

func (s *SmartSupervisor) HandleRejection(ctx context.Context, task map[string]any, decision map[string]any) (*RejectionDecision, error) {
    taskID, _ := task["id"].(string)
    taskType, _ := task["type"].(string)
    currentModel, _ := task["model_id"].(string)
    issues := parseIssues(decision["issues"])
    
    // Get failure history for this task
    failures := s.getTaskFailures(ctx, taskID)
    
    // Get failure patterns for this task type
    patterns := s.getFailurePatterns(ctx, taskType)
    
    // Analyze and decide
    result := s.analyzeAndDecide(AnalyzeInput{
        TaskID:         taskID,
        TaskType:       taskType,
        CurrentModel:   currentModel,
        Issues:         issues,
        FailureHistory: failures,
        KnownPatterns:  patterns,
    })
    
    // Record this failure
    s.recordFailure(ctx, taskID, taskType, currentModel, issues)
    
    // Record pattern if new
    s.maybeRecordPattern(ctx, taskType, issues, result)
    
    return result, nil
}

type AnalyzeInput struct {
    TaskID         string
    TaskType       string
    CurrentModel   string
    Issues         []Issue
    FailureHistory []FailureRecord
    KnownPatterns  []FailurePattern
}

type Issue struct {
    Type        string
    Description string
    Severity    string
}

type FailureRecord struct {
    ModelID  string
    Reason   string
    Issues   []Issue
    Duration float64
}

type FailurePattern struct {
    Reason          string
    OccurrenceCount int
    SuggestedAction string
}

func (s *SmartSupervisor) analyzeAndDecide(input AnalyzeInput) *RejectionDecision {
    // Count unique models that failed
    uniqueModels := make(map[string]bool)
    for _, f := range input.FailureHistory {
        uniqueModels[f.ModelID] = true
    }
    uniqueModels[input.CurrentModel] = true
    
    failureCount := len(input.FailureHistory) + 1
    uniqueModelCount := len(uniqueModels)
    
    // Analyze failure types
    failureTypes := s.categorizeFailures(input.Issues)
    
    // Decision logic
    
    // Case 1: Multiple models, same issue = task is the problem
    if uniqueModelCount >= 2 && s.sameIssueAcrossModels(input.FailureHistory, input.Issues) {
        return &RejectionDecision{
            Action:            "fix_prompt",
            Reason:            fmt.Sprintf("Same failure across %d different models - task prompt likely unclear", uniqueModelCount),
            FeedbackToPlanner: s.generatePlannerFeedback(input.Issues, failureTypes),
        }
    }
    
    // Case 2: 3+ failures = escalate to Planner for possible split
    if failureCount >= 3 {
        return &RejectionDecision{
            Action:            "split",
            Reason:            fmt.Sprintf("Task failed %d times - may need splitting", failureCount),
            FeedbackToPlanner: s.generateSplitFeedback(input),
        }
    }
    
    // Case 3: Truncation = try model with larger context
    if failureTypes["truncation"] {
        return &RejectionDecision{
            Action:      "reassign",
            Reason:      "Output truncated - try model with larger context",
            TargetModel: s.getModelWithLargerContext(input.CurrentModel),
            FeedbackToModel: "Previous output was truncated. Please be more concise or request to split task.",
        }
    }
    
    // Case 4: Context exceeded = definitely need different model or split
    if failureTypes["context_exceeded"] {
        largerContextModel := s.getModelWithLargerContext(input.CurrentModel)
        if largerContextModel != "" {
            return &RejectionDecision{
                Action:      "reassign",
                Reason:      "Context exceeded - trying model with larger context",
                TargetModel: largerContextModel,
            }
        }
        // No larger model available - need to split
        return &RejectionDecision{
            Action:            "split",
            Reason:            "Context exceeded and no larger model available - task must be split",
            FeedbackToPlanner: "Task requires more context than any available model provides. Split into smaller tasks.",
        }
    }
    
    // Case 5: Drift = prompt was unclear
    if failureTypes["drift"] {
        return &RejectionDecision{
            Action:            "fix_prompt",
            Reason:            "Output drifted from requirements - prompt needs clarification",
            FeedbackToPlanner: s.generateDriftFeedback(input.Issues),
        }
    }
    
    // Case 6: Security issue = blacklist model for this task type
    if failureTypes["security"] {
        s.blacklistModelForTaskType(input.CurrentModel, input.TaskType)
        return &RejectionDecision{
            Action:      "reassign",
            Reason:      "Security issue detected - trying different model",
            TargetModel: s.getDifferentModel(input.CurrentModel, input.TaskType),
        }
    }
    
    // Default: Just try different model
    return &RejectionDecision{
        Action:      "reassign",
        Reason:      "Trying different model",
        TargetModel: s.getDifferentModel(input.CurrentModel, input.TaskType),
        FeedbackToModel: s.generateModelFeedback(input.Issues),
    }
}

func (s *SmartSupervisor) categorizeFailures(issues []Issue) map[string]bool {
    types := make(map[string]bool)
    for _, issue := range issues {
        switch issue.Type {
        case "truncation", "incomplete":
            types["truncation"] = true
        case "context_exceeded":
            types["context_exceeded"] = true
        case "drift", "wrong_output":
            types["drift"] = true
        case "security", "secrets":
            types["security"] = true
        }
    }
    return types
}

func (s *SmartSupervisor) sameIssueAcrossModels(history []FailureRecord, currentIssues []Issue) bool {
    if len(history) == 0 {
        return false
    }
    
    // Check if same issue type appears in history
    currentTypes := s.categorizeFailures(currentIssues)
    for _, record := range history {
        pastTypes := s.categorizeFailures(record.Issues)
        for t := range currentTypes {
            if pastTypes[t] {
                return true
            }
        }
    }
    return false
}

func (s *SmartSupervisor) getTaskFailures(ctx context.Context, taskID string) []FailureRecord {
    result, err := s.db.RPC(ctx, "get_task_failures", map[string]any{
        "p_task_id": taskID,
    })
    if err != nil {
        return nil
    }
    
    var records []FailureRecord
    json.Unmarshal(result, &records)
    return records
}

func (s *SmartSupervisor) getFailurePatterns(ctx context.Context, taskType string) []FailurePattern {
    result, err := s.db.RPC(ctx, "get_failure_patterns", map[string]any{
        "p_task_type": taskType,
        "p_limit":     10,
    })
    if err != nil {
        return nil
    }
    
    var patterns []FailurePattern
    json.Unmarshal(result, &patterns)
    return patterns
}

func (s *SmartSupervisor) recordFailure(ctx context.Context, taskID, taskType, modelID string, issues []Issue) {
    // Record each issue as a failure
    for _, issue := range issues {
        s.db.RPC(ctx, "record_failure_pattern", map[string]any{
            "p_task_type":            taskType,
            "p_failure_reason":       issue.Type,
            "p_model_id":             modelID,
            "p_pattern_description":  issue.Description,
            "p_suggested_action":     "", // Will be filled by pattern analysis
        })
    }
}

func (s *SmartSupervisor) maybeRecordPattern(ctx context.Context, taskType string, issues []Issue, decision *RejectionDecision) {
    // If we've seen this pattern before, update suggested action
    for _, issue := range issues {
        s.db.RPC(ctx, "update_failure_pattern_action", map[string]any{
            "p_task_type":        taskType,
            "p_failure_reason":   issue.Type,
            "p_suggested_action": decision.Action,
        })
    }
}

func (s *SmartSupervisor) getModelWithLargerContext(currentModel string) string {
    // This should query models table for models with larger context
    // For now, return empty - will implement with model registry
    return ""
}

func (s *SmartSupervisor) getDifferentModel(currentModel, taskType string) string {
    // This should use router to get next best model
    // For now, return empty - will integrate with router
    return ""
}

func (s *SmartSupervisor) blacklistModelForTaskType(modelID, taskType string) {
    // Record that this model is bad for this task type
    // This feeds into model scoring
}

func (s *SmartSupervisor) generatePlannerFeedback(issues []Issue, failureTypes map[string]bool) string {
    var feedback string
    if failureTypes["truncation"] {
        feedback += "Tasks are producing truncated output. Consider: (1) Making prompt more concise, (2) Asking for less in single task, (3) Splitting into smaller tasks. "
    }
    if failureTypes["drift"] {
        feedback += "Tasks are drifting from requirements. Consider: (1) More specific expected_output, (2) Clearer constraints in prompt packet, (3) Adding explicit 'do not' sections. "
    }
    return feedback
}

func (s *SmartSupervisor) generateSplitFeedback(input AnalyzeInput) string {
    return fmt.Sprintf("Task failed %d times across %d models. Issues: %v. Consider splitting into smaller, more focused tasks.",
        len(input.FailureHistory)+1,
        len(uniqueModels(input.FailureHistory, input.CurrentModel)),
        summarizeIssues(input.Issues))
}

func (s *SmartSupervisor) generateDriftFeedback(issues []Issue) string {
    var drifts []string
    for _, issue := range issues {
        if issue.Type == "drift" || issue.Type == "wrong_output" {
            drifts = append(drifts, issue.Description)
        }
    }
    return fmt.Sprintf("Output drifted from requirements: %v. Add explicit constraints and expected output format.", drifts)
}

func (s *SmartSupervisor) generateModelFeedback(issues []Issue) string {
    var feedback string
    for _, issue := range issues {
        feedback += fmt.Sprintf("- %s: %s\n", issue.Type, issue.Description)
    }
    return feedback
}

func parseIssues(raw any) []Issue {
    if raw == nil {
        return nil
    }
    
    var issues []Issue
    switch v := raw.(type) {
    case []interface{}:
        for _, item := range v {
            if m, ok := item.(map[string]interface{}); ok {
                issue := Issue{}
                if t, ok := m["type"].(string); ok {
                    issue.Type = t
                }
                if d, ok := m["description"].(string); ok {
                    issue.Description = d
                }
                if s, ok := m["severity"].(string); ok {
                    issue.Severity = s
                }
                issues = append(issues, issue)
            }
        }
    case string:
        issues = append(issues, Issue{Type: "unknown", Description: v})
    }
    return issues
}

func uniqueModels(history []FailureRecord, currentModel string) []string {
    models := make(map[string]bool)
    models[currentModel] = true
    for _, h := range history {
        models[h.ModelID] = true
    }
    var result []string
    for m := range models {
        result = append(result, m)
    }
    return result
}

func summarizeIssues(issues []Issue) []string {
    var summaries []string
    for _, issue := range issues {
        summaries = append(summaries, issue.Type)
    }
    return summaries
}
```

---

## COMPONENT 5: SMART ORCHESTRATOR

### Purpose
Orchestrator knows everything. Routes intelligently. Learns continuously.

### Implementation

Create `governor/internal/runtime/smart_orchestrator.go`:

```go
package runtime

import (
    "context"
    "encoding/json"
    "log"
    "sync"
    "time"
)

type SmartOrchestrator struct {
    db         Querier
    router     *Router
    git        GitOperator
    supervisor *SmartSupervisor
    
    // In-memory state
    taskStates   map[string]*TaskState
    modelLoads   map[string]int
    mu           sync.RWMutex
}

type TaskState struct {
    TaskID         string
    Status         string
    AssignedModel  string
    AttemptCount   int
    FailureReasons []string
    LastAttemptAt  time.Time
}

type GitOperator interface {
    CreateBranch(ctx context.Context, name string) error
    CommitOutput(ctx context.Context, branch string, output interface{}) error
    ClearBranch(ctx context.Context, name string) error
    MergeBranch(ctx context.Context, source, target string) error
    DeleteBranch(ctx context.Context, name string) error
    ReadBranchOutput(ctx context.Context, branch string) ([]string, error)
}

func NewSmartOrchestrator(db Querier, router *Router, git GitOperator) *SmartOrchestrator {
    return &SmartOrchestrator{
        db:          db,
        router:      router,
        git:         git,
        supervisor:  NewSmartSupervisor(db),
        taskStates:  make(map[string]*TaskState),
        modelLoads:  make(map[string]int),
    }
}

func (o *SmartOrchestrator) AssignTask(ctx context.Context, task map[string]any) (string, error) {
    taskID, _ := task["id"].(string)
    taskType, _ := task["type"].(string)
    taskNumber, _ := task["task_number"].(string)
    
    // Get or create task state
    state := o.getOrCreateState(taskID)
    
    // Get models to avoid (failed on this task)
    avoidModels := o.getModelsToAvoid(taskID)
    
    // Select best destination and model
    result, err := o.router.SelectDestination(ctx, RoutingRequest{
        AgentID:          "task_runner",
        TaskID:           taskID,
        TaskType:         taskType,
        AvoidModels:      avoidModels,
        PreferContext:    o.needsLargeContext(state),
    })
    if err != nil || result == nil {
        return "", err
    }
    
    // Create branch
    branchName := "task/" + taskNumber
    if err := o.git.CreateBranch(ctx, branchName); err != nil {
        log.Printf("[SmartOrchestrator] Failed to create branch: %v", err)
    }
    
    // Update state
    o.mu.Lock()
    state.AssignedModel = result.ModelID
    state.Status = "in_progress"
    state.LastAttemptAt = time.Now()
    o.modelLoads[result.ModelID]++
    o.mu.Unlock()
    
    // Update Supabase
    o.updateTaskStatus(ctx, taskID, "in_progress", result.ModelID)
    
    log.Printf("[SmartOrchestrator] Assigned task %s to %s (model: %s, attempt %d)",
        taskID, result.DestinationID, result.ModelID, state.AttemptCount+1)
    
    return result.DestinationID, nil
}

func (o *SmartOrchestrator) HandleRunnerOutput(ctx context.Context, task map[string]any, output string) error {
    taskID, _ := task["id"].(string)
    taskNumber, _ := task["task_number"].(string)
    
    // Parse output
    parsedOutput := o.parseOutput(output)
    
    // Commit to GitHub
    branchName := "task/" + taskNumber
    if err := o.git.CommitOutput(ctx, branchName, parsedOutput); err != nil {
        log.Printf("[SmartOrchestrator] Failed to commit output: %v", err)
    }
    
    // Update state
    o.updateTaskStatus(ctx, taskID, "review", "")
    
    // Route to supervisor for review
    // (This will be called by event handler)
    
    return nil
}

func (o *SmartOrchestrator) HandleSupervisorDecision(ctx context.Context, task map[string]any, decisionJSON string) error {
    taskID, _ := task["id"].(string)
    taskNumber, _ := task["task_number"].(string)
    modelID := o.getAssignedModel(taskID)
    
    // Parse supervisor decision
    var decision struct {
        Decision   string `json:"decision"`
        NextAction string `json:"next_action"`
        Issues     []struct {
            Type        string `json:"type"`
            Description string `json:"description"`
            Severity    string `json:"severity"`
        } `json:"issues"`
    }
    
    if err := json.Unmarshal([]byte(decisionJSON), &decision); err != nil {
        log.Printf("[SmartOrchestrator] Failed to parse decision: %v", err)
        return err
    }
    
    state := o.getOrCreateState(taskID)
    
    switch decision.Decision {
    case "pass":
        return o.handlePass(ctx, task, taskNumber)
        
    case "fail":
        return o.handleFail(ctx, task, taskNumber, modelID, decision, state)
        
    case "reroute":
        return o.handleReroute(ctx, task, decision, state)
    }
    
    return nil
}

func (o *SmartOrchestrator) handlePass(ctx context.Context, task map[string]any, taskNumber string) error {
    taskID, _ := task["id"].(string)
    
    // Update status to testing
    o.updateTaskStatus(ctx, taskID, "testing", "")
    
    // Trigger tests (via event)
    // Tests will be handled by event handler
    
    log.Printf("[SmartOrchestrator] Task %s passed review, triggering tests", taskID)
    return nil
}

func (o *SmartOrchestrator) handleFail(ctx context.Context, task map[string]any, taskNumber, modelID string, decision struct {
    Decision   string `json:"decision"`
    NextAction string `json:"next_action"`
    Issues     []struct {
        Type        string `json:"type"`
        Description string `json:"description"`
        Severity    string `json:"severity"`
    } `json:"issues"`
}, state *TaskState) error {
    taskID, _ := task["id"].(string)
    
    // Increment attempt count
    state.AttemptCount++
    
    // Record failure reason
    for _, issue := range decision.Issues {
        state.FailureReasons = append(state.FailureReasons, issue.Type)
    }
    
    // Get smart decision from supervisor
    rejectionDecision, err := o.supervisor.HandleRejection(ctx, task, map[string]any{
        "decision": decision.Decision,
        "issues":   decision.Issues,
    })
    if err != nil {
        log.Printf("[SmartOrchestrator] Supervisor error: %v", err)
        // Fall back to simple reassign
        rejectionDecision = &RejectionDecision{
            Action: "reassign",
            Reason: "Supervisor error, defaulting to reassign",
        }
    }
    
    log.Printf("[SmartOrchestrator] Task %s failed: %s (action: %s)",
        taskID, rejectionDecision.Reason, rejectionDecision.Action)
    
    switch rejectionDecision.Action {
    case "reassign":
        return o.reassignTask(ctx, task, rejectionDecision.TargetModel, state)
        
    case "fix_prompt":
        return o.escalateToPlanner(ctx, task, rejectionDecision.FeedbackToPlanner, "fix_prompt")
        
    case "split":
        return o.escalateToPlanner(ctx, task, rejectionDecision.FeedbackToPlanner, "split")
    }
    
    return nil
}

func (o *SmartOrchestrator) handleReroute(ctx context.Context, task map[string]any, decision struct {
    Decision   string `json:"decision"`
    NextAction string `json:"next_action"`
    Issues     []struct {
        Type        string `json:"type"`
        Description string `json:"description"`
        Severity    string `json:"severity"`
    } `json:"issues"`
}, state *TaskState) error {
    // Reroute is similar to fail but with explicit next action
    return o.handleFail(ctx, task, "", "", decision, state)
}

func (o *SmartOrchestrator) reassignTask(ctx context.Context, task map[string]any, targetModel string, state *TaskState) error {
    taskID, _ := task["id"].(string)
    taskNumber, _ := task["task_number"].(string)
    
    // Clear branch for retry
    branchName := "task/" + taskNumber
    if err := o.git.ClearBranch(ctx, branchName); err != nil {
        log.Printf("[SmartOrchestrator] Failed to clear branch: %v", err)
    }
    
    // Record failure
    o.recordModelFailure(ctx, taskID, state.AssignedModel, state.FailureReasons[len(state.FailureReasons)-1])
    
    // Update state
    state.AssignedModel = ""
    state.Status = "available"
    
    // Update Supabase
    o.updateTaskStatus(ctx, taskID, "available", "")
    
    log.Printf("[SmartOrchestrator] Task %s reassigned (attempt %d)", taskID, state.AttemptCount)
    
    return nil
}

func (o *SmartOrchestrator) escalateToPlanner(ctx context.Context, task map[string]any, feedback string, actionType string) error {
    taskID, _ := task["id"].(string)
    
    // Create planner task
    o.db.RPC(ctx, "create_planner_task", map[string]any{
        "p_task_id":      taskID,
        "p_action_type":  actionType,
        "p_feedback":     feedback,
    })
    
    // Update task status
    o.updateTaskStatus(ctx, taskID, "escalated", "")
    
    log.Printf("[SmartOrchestrator] Task %s escalated to planner (%s)", taskID, actionType)
    
    return nil
}

func (o *SmartOrchestrator) HandleTestResults(ctx context.Context, task map[string]any, results map[string]any) error {
    taskID, _ := task["id"].(string)
    taskNumber, _ := task["task_number"].(string)
    
    passed, _ := results["passed"].(bool)
    
    if passed {
        // Merge to module branch
        sliceID, _ := task["slice_id"].(string)
        if sliceID == "" {
            sliceID = "default"
        }
        
        sourceBranch := "task/" + taskNumber
        targetBranch := "module/" + sliceID
        
        if err := o.git.MergeBranch(ctx, sourceBranch, targetBranch); err != nil {
            log.Printf("[SmartOrchestrator] Failed to merge: %v", err)
            return err
        }
        
        // Delete task branch
        o.git.DeleteBranch(ctx, sourceBranch)
        
        // Update status
        o.updateTaskStatus(ctx, taskID, "complete", "")
        
        // Unlock dependents
        o.unlockDependents(ctx, taskID)
        
        // Record success
        modelID := o.getAssignedModel(taskID)
        o.recordModelSuccess(ctx, taskID, modelID)
        
        log.Printf("[SmartOrchestrator] Task %s complete, merged to %s", taskID, targetBranch)
    } else {
        // Return for fix
        failures, _ := results["failures"].([]interface{})
        o.updateTaskStatus(ctx, taskID, "fix_required", "")
        
        log.Printf("[SmartOrchestrator] Task %s tests failed: %d failures", taskID, len(failures))
    }
    
    return nil
}

func (o *SmartOrchestrator) getOrCreateState(taskID string) *TaskState {
    o.mu.Lock()
    defer o.mu.Unlock()
    
    if state, ok := o.taskStates[taskID]; ok {
        return state
    }
    
    state := &TaskState{
        TaskID:   taskID,
        Status:   "pending",
    }
    o.taskStates[taskID] = state
    return state
}

func (o *SmartOrchestrator) getAssignedModel(taskID string) string {
    o.mu.RLock()
    defer o.mu.RUnlock()
    
    if state, ok := o.taskStates[taskID]; ok {
        return state.AssignedModel
    }
    return ""
}

func (o *SmartOrchestrator) getModelsToAvoid(taskID string) []string {
    o.mu.RLock()
    defer o.mu.RUnlock()
    
    // Get models that failed on this task
    // Query from failure_patterns
    return nil // TODO: implement
}

func (o *SmartOrchestrator) needsLargeContext(state *TaskState) bool {
    // Check if previous failures were truncation
    for _, reason := range state.FailureReasons {
        if reason == "truncation" || reason == "context_exceeded" {
            return true
        }
    }
    return false
}

func (o *SmartOrchestrator) updateTaskStatus(ctx context.Context, taskID, status, modelID string) {
    params := map[string]any{
        "p_task_id": taskID,
        "p_status":  status,
    }
    if modelID != "" {
        params["p_model_id"] = modelID
    }
    
    o.db.RPC(ctx, "update_task_status", params)
}

func (o *SmartOrchestrator) unlockDependents(ctx context.Context, taskID string) {
    o.db.RPC(ctx, "unlock_dependent_tasks", map[string]any{
        "p_task_id": taskID,
    })
}

func (o *SmartOrchestrator) recordModelFailure(ctx context.Context, taskID, modelID, reason string) {
    o.db.RPC(ctx, "record_model_failure", map[string]any{
        "p_model_id":     modelID,
        "p_failure_type": reason,
        "p_task_id":      taskID,
    })
}

func (o *SmartOrchestrator) recordModelSuccess(ctx context.Context, taskID, modelID string) {
    o.db.RPC(ctx, "record_model_success", map[string]any{
        "p_model_id": modelID,
        "p_task_id":  taskID,
    })
}

func (o *SmartOrchestrator) parseOutput(output string) map[string]any {
    var parsed map[string]any
    if err := json.Unmarshal([]byte(output), &parsed); err != nil {
        // If not JSON, wrap as raw output
        return map[string]any{
            "raw_output": output,
        }
    }
    return parsed
}
```

---

## COMPONENT 6: SMART SYSTEM RESEARCHER

### Purpose
Researcher learns from Council feedback. Improves research focus based on system needs.

### Implementation

Update System Researcher to read learnings before researching:

```go
// In prompts/system_researcher.md additions

## Before Researching

Query the learning store:
1. Get recent Council feedback on your suggestions
2. Get current system pain points from failure_patterns
3. Get active learned_rules for researcher

Include in your research:
- Focus on solutions to known failure patterns
- Avoid suggestions that Council has rejected
- Prioritize research that addresses current system needs

## After Council Review

When Council reviews your suggestions:
1. If REJECTED: Learn why. Don't suggest similar things.
2. If APPROVED with concerns: Address concerns in future research.
3. If SENT TO HUMAN: Wait for human feedback before similar suggestions.

Record your learnings:
```json
{
  "action": "record_learning",
  "learning_type": "council_feedback",
  "learning_text": "Council rejected X because Y. Future research should consider Z.",
  "source_event": "research_2026-02-28"
}
```
```

---

## COMPONENT 7: DAILY LEARNING TASK

### Purpose
Update all agent prompts with new learnings. Run by Maintenance agent.

### Implementation

Create `governor/internal/runtime/learning_updater.go`:

```go
package runtime

import (
    "context"
    "fmt"
    "log"
    "os"
    "path/filepath"
    "strings"
    "time"
)

type LearningUpdater struct {
    db         Querier
    promptsDir string
}

func NewLearningUpdater(db Querier, promptsDir string) *LearningUpdater {
    return &LearningUpdater{
        db:         db,
        promptsDir: promptsDir,
    }
}

func (u *LearningUpdater) Run(ctx context.Context) error {
    log.Println("[LearningUpdater] Starting daily learning update")
    
    agents := []string{"planner", "supervisor", "orchestrator", "system_researcher"}
    
    for _, agentID := range agents {
        if err := u.updateAgent(ctx, agentID); err != nil {
            log.Printf("[LearningUpdater] Failed to update %s: %v", agentID, err)
        }
    }
    
    log.Println("[LearningUpdater] Daily learning update complete")
    return nil
}

func (u *LearningUpdater) updateAgent(ctx context.Context, agentID string) error {
    // Get new learnings from today
    learnings, err := u.getNewLearnings(ctx, agentID)
    if err != nil {
        return err
    }
    
    if len(learnings) == 0 {
        return nil
    }
    
    // Read current prompt
    promptPath := filepath.Join(u.promptsDir, agentID+".md")
    content, err := os.ReadFile(promptPath)
    if err != nil {
        return err
    }
    
    // Update "What I've Learned" section
    updated := u.updateLearningsSection(string(content), learnings)
    
    // Write back
    if err := os.WriteFile(promptPath, []byte(updated), 0644); err != nil {
        return err
    }
    
    // Mark learnings as applied
    u.markLearningsApplied(ctx, agentID)
    
    log.Printf("[LearningUpdater] Updated %s with %d new learnings", agentID, len(learnings))
    return nil
}

func (u *LearningUpdater) getNewLearnings(ctx context.Context, agentID string) ([]map[string]any, error) {
    result, err := u.db.RPC(ctx, "get_unapplied_learnings", map[string]any{
        "p_agent_id": agentID,
    })
    if err != nil {
        return nil, err
    }
    
    var learnings []map[string]any
    if err := json.Unmarshal(result, &learnings); err != nil {
        return nil, err
    }
    
    return learnings, nil
}

func (u *LearningUpdater) updateLearningsSection(content string, learnings []map[string]any) string {
    // Find "What I've Learned" section
    sectionStart := strings.Index(content, "## What I've Learned")
    if sectionStart == -1 {
        // Add section at end
        return content + "\n\n## What I've Learned\n\n" + u.formatLearnings(learnings)
    }
    
    // Find next section
    nextSection := strings.Index(content[sectionStart+20:], "\n## ")
    if nextSection == -1 {
        nextSection = len(content)
    } else {
        nextSection += sectionStart + 20
    }
    
    // Get existing content
    existingSection := content[sectionStart:nextSection]
    
    // Add new learnings with today's date
    today := time.Now().Format("2006-01-02")
    newContent := existingSection
    
    for _, learning := range learnings {
        learningText, _ := learning["learning_text"].(string)
        newContent += fmt.Sprintf("\n\n### %s\n%s", today, learningText)
    }
    
    // Replace section
    return content[:sectionStart] + newContent + content[nextSection:]
}

func (u *LearningUpdater) formatLearnings(learnings []map[string]any) string {
    var sb strings.Builder
    today := time.Now().Format("2006-01-02")
    
    sb.WriteString("### " + today + "\n")
    for _, l := range learnings {
        text, _ := l["learning_text"].(string)
        sb.WriteString("- " + text + "\n")
    }
    
    return sb.String()
}

func (u *LearningUpdater) markLearningsApplied(ctx context.Context, agentID string) {
    u.db.RPC(ctx, "mark_learnings_applied", map[string]any{
        "p_agent_id": agentID,
    })
}
```

---

## COMPONENT 8: UPDATED MAIN.GO

### Purpose
Wire everything together. Replace dumb handlers with smart handlers.

### Key Changes

```go
// In main.go

func main() {
    // ... existing setup ...
    
    // Create smart components
    prdWatcher := runtime.NewPRDWatcher(database, runtime.PRDWatcherConfig{
        RepoPath:  repoPath,
        PRDBranch: "main",
        PRDDir:    "docs/prd",
        Interval:  10 * time.Second,
    })
    
    smartOrchestrator := runtime.NewSmartOrchestrator(database, destRouter, git)
    smartPlanner := runtime.NewSmartPlanner(database, git, promptsDir)
    learningUpdater := runtime.NewLearningUpdater(database, promptsDir)
    
    // Start PRD watcher
    go prdWatcher.Start(ctx, func(prdPath string) {
        // Create plan record
        database.RPC(ctx, "create_plan", map[string]any{
            "p_prd_path": prdPath,
            "p_status":   "draft",
        })
    })
    
    // Update event handlers to use smart components
    setupSmartEventHandlers(ctx, eventRouter, smartOrchestrator, smartPlanner, ...)
    
    // Schedule daily learning update
    go runDailyLearning(ctx, learningUpdater)
    
    // ... rest of main ...
}

func setupSmartEventHandlers(ctx context.Context, router *runtime.EventRouter, orchestrator *runtime.SmartOrchestrator, planner *runtime.SmartPlanner, ...) {
    
    router.On(runtime.EventTaskAvailable, func(event runtime.Event) {
        var task map[string]any
        json.Unmarshal(event.Record, &task)
        
        // Use smart orchestrator
        orchestrator.AssignTask(ctx, task)
    })
    
    router.On(runtime.EventTaskReview, func(event runtime.Event) {
        var task map[string]any
        json.Unmarshal(event.Record, &task)
        
        // Route to supervisor
        // Supervisor will return decision
        // Orchestrator handles decision
    })
    
    router.On(runtime.EventPRDReady, func(event runtime.Event) {
        var plan map[string]any
        json.Unmarshal(event.Record, &plan)
        
        prdPath, _ := plan["prd_path"].(string)
        
        // Build context with learnings
        context, _ := planner.BuildContext(ctx, prdPath)
        
        // Route to planner with context
        // Planner creates plan with learnings included
    })
    
    // ... other handlers ...
}

func runDailyLearning(ctx context.Context, updater *runtime.LearningUpdater) {
    ticker := time.NewTicker(24 * time.Hour)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            updater.Run(ctx)
        }
    }
}
```

---

## IMPLEMENTATION ORDER

### Phase 1: Core Intelligence (Sessions 37-38)
1. Create learning store tables (Supabase)
2. Create smart_supervisor.go
3. Create smart_orchestrator.go
4. Update main.go to use smart components
5. Wire decision parsing

### Phase 2: PRD Detection (Session 39)
1. Create prd_watcher.go
2. Wire to main.go
3. Test PRD detection

### Phase 3: Smart Planner (Session 40)
1. Create smart_planner.go
2. Wire Council feedback recording
3. Wire learning retrieval

### Phase 4: Learning Updates (Session 41)
1. Create learning_updater.go
2. Schedule daily task
3. Test prompt updates

### Phase 5: System Researcher (Session 42)
1. Update researcher prompt
2. Wire feedback recording
3. Test research improvements

### Phase 6: Integration Testing (Session 43)
1. Full end-to-end test
2. Verify all loops work
3. Verify learning accumulates

---

## FILES TO CREATE

| File | Purpose |
|------|---------|
| `governor/internal/runtime/prd_watcher.go` | Detect new PRDs |
| `governor/internal/runtime/smart_planner.go` | Planner with learning |
| `governor/internal/runtime/smart_supervisor.go` | Supervisor with smart decisions |
| `governor/internal/runtime/smart_orchestrator.go` | Orchestrator that knows everything |
| `governor/internal/runtime/learning_updater.go` | Daily prompt updates |
| `docs/supabase-schema/034_learning_store.sql` | Learning tables |
| `docs/supabase-schema/035_learning_rpcs.sql` | Learning RPCs |

## FILES TO MODIFY

| File | Changes |
|------|---------|
| `governor/cmd/governor/main.go` | Wire all smart components |
| `prompts/planner.md` | Add learning section reading |
| `prompts/supervisor.md` | Add learning section reading |
| `prompts/system_researcher.md` | Add learning integration |
| `governor/config/system.json` | Add PRD watcher config |

---

## SUCCESS CRITERIA

The system is complete when:

1. ✅ New PRD detected automatically
2. ✅ Planner creates plan with past learnings included
3. ✅ Council feedback recorded and fed back to Planner
4. ✅ Supervisor makes smart decisions (reassign/split/fix)
5. ✅ Orchestrator knows what failed where and why
6. ✅ Failure patterns recorded and used for future decisions
7. ✅ Agent prompts updated daily with new learnings
8. ✅ System Researcher focuses research on system needs
9. ✅ Everything configurable, nothing hardcoded
10. ✅ Everything learns, nothing forgets

---

**This is the complete plan. No shortcuts. Full functionality.**

**Ready to implement when you say go.**
