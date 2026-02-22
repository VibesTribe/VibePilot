# Phase 3: Intelligent Routing - Implementation Plan

**Created:** 2026-02-22
**Status:** Planning Complete, Ready for Implementation
**Depends On:** Phase 2 (Task Execution - COMPLETE)

---

## 1. GOALS

### Primary Goals
1. **Smart routing** - Orchestrator selects best available model based on real data
2. **Self-managing limits** - Cooldowns, rate limits auto-expire; credit depletion alerts human
3. **Learning** - Every task improves future routing decisions
4. **Zero code changes for new models** - Add via INSERT, system adapts

### Core Principles
- Nothing hardcoded - everything from DB
- Simple additions = INSERT only, no code changes
- Complex changes = Council → Human → full flow
- Status transitions are automatic where possible
- Human only involved when money/decisions needed

---

## 2. SCHEMA CHANGES

### 2.1 New Table: runners

Combines model + tool + capabilities. This is what orchestrator actually dispatches to.

```sql
CREATE TABLE runners (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  
  -- Components
  model_id UUID REFERENCES models(id) ON DELETE CASCADE,
  tool_id UUID REFERENCES tools(id) ON DELETE CASCADE,
  
  -- Capabilities
  routing_capability TEXT[] DEFAULT '{web}',
  -- Values: 'internal' (CLI/API with codebase), 'web' (courier), 'mcp' (future)
  
  -- Priority (lower = better)
  cost_priority INT DEFAULT 2 CHECK (cost_priority BETWEEN 0 AND 2),
  -- 0 = subscription (best), 1 = free API, 2 = paid API
  
  -- Status
  status TEXT DEFAULT 'active' CHECK (status IN (
    'active', 'cooldown', 'rate_limited', 'paused', 'benched'
  )),
  status_reason TEXT,
  
  -- Performance tracking
  strengths TEXT[] DEFAULT '{}',  -- e.g. {'coding', 'research', 'planning'}
  task_ratings JSONB DEFAULT '{}',  -- e.g. {'coding': {'success': 10, 'fail': 2}}
  
  -- Limit tracking
  daily_used INT DEFAULT 0,
  daily_limit INT,
  daily_reset_at TIMESTAMPTZ,
  
  cooldown_expires_at TIMESTAMPTZ,
  rate_limit_reset_at TIMESTAMPTZ,
  
  -- Timestamps
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Indexes for routing queries
CREATE INDEX idx_runners_routing ON runners(status, routing_capability);
CREATE INDEX idx_runners_priority ON runners(cost_priority ASC) WHERE status = 'active';
```

### 2.2 New Table: tools

CLI tools, browser automation, MCP connectors.

```sql
CREATE TABLE tools (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  
  name TEXT NOT NULL UNIQUE,  -- e.g. 'opencode', 'kimi-cli', 'browser-use'
  type TEXT NOT NULL CHECK (type IN ('cli', 'browser', 'mcp')),
  
  -- For CLI tools
  command TEXT,  -- e.g. 'opencode', 'kimi'
  
  -- Resource requirements
  ram_requirement_mb INT DEFAULT 500,
  
  -- Status
  status TEXT DEFAULT 'active' CHECK (status IN ('active', 'deprecated', 'removed')),
  
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW()
);
```

### 2.3 Modify: models table

Add credit tracking and rate limiting.

```sql
-- Add to existing models table
ALTER TABLE models ADD COLUMN IF NOT EXISTS
  access_type TEXT DEFAULT 'api' CHECK (access_type IN ('subscription', 'free_tier', 'paid_api'));

ALTER TABLE models ADD COLUMN IF NOT EXISTS
  credit_remaining_usd DECIMAL(10,4) DEFAULT 0;

ALTER TABLE models ADD COLUMN IF NOT EXISTS
  credit_alert_threshold DECIMAL(10,4) DEFAULT 1.00;

ALTER TABLE models ADD COLUMN IF NOT EXISTS
  rate_limit_requests_per_minute INT DEFAULT 60;

ALTER TABLE models ADD COLUMN IF NOT EXISTS
  context_limit INT DEFAULT 128000;

ALTER TABLE models ADD COLUMN IF NOT EXISTS
  cost_per_1k_tokens_in DECIMAL(10,6) DEFAULT 0;

ALTER TABLE models ADD COLUMN IF NOT EXISTS
  cost_per_1k_tokens_out DECIMAL(10,6) DEFAULT 0;
```

### 2.4 Modify: platforms table

Add daily limit tracking.

```sql
-- Add to existing platforms table
ALTER TABLE platforms ADD COLUMN IF NOT EXISTS
  daily_limit INT DEFAULT 100;

ALTER TABLE platforms ADD COLUMN IF NOT EXISTS
  theoretical_cost_input_per_1k_usd DECIMAL(10,6);

ALTER TABLE platforms ADD COLUMN IF NOT EXISTS
  theoretical_cost_output_per_1k_usd DECIMAL(10,6);
```

### 2.5 RPC: get_best_runner

Intelligent model selection.

```sql
CREATE OR REPLACE FUNCTION get_best_runner(
  p_routing TEXT,
  p_task_type TEXT DEFAULT NULL
)
RETURNS UUID AS $$
DECLARE
  v_runner_id UUID;
BEGIN
  -- Find best available runner for routing capability
  SELECT r.id INTO v_runner_id
  FROM runners r
  JOIN models m ON r.model_id = m.id
  WHERE r.status = 'active'
    AND p_routing = ANY(r.routing_capability)
    AND (r.cooldown_expires_at IS NULL OR r.cooldown_expires_at < NOW())
    AND (r.rate_limit_reset_at IS NULL OR r.rate_limit_reset_at < NOW())
    AND (m.credit_remaining_usd IS NULL OR m.credit_remaining_usd > m.credit_alert_threshold)
  ORDER BY
    r.cost_priority ASC,
    CASE 
      WHEN r.task_ratings ? p_task_type 
      THEN (r.task_ratings->p_task_type->>'success')::float / 
           NULLIF((r.task_ratings->p_task_type->>'success')::int + 
                  (r.task_ratings->p_task_type->>'fail')::int, 0)
      ELSE 0.5
    END DESC,
    r.daily_used ASC NULLS LAST
  LIMIT 1;
  
  RETURN v_runner_id;
END;
$$ LANGUAGE plpgsql;
```

### 2.6 RPC: record_runner_result

Update stats after task.

```sql
CREATE OR REPLACE FUNCTION record_runner_result(
  p_runner_id UUID,
  p_task_type TEXT,
  p_success BOOLEAN,
  p_tokens_used INT
)
RETURNS VOID AS $$
BEGIN
  -- Update runner's task_ratings
  UPDATE runners r
  SET 
    task_ratings = jsonb_set(
      COALESCE(r.task_ratings, '{}'::jsonb),
      ARRAY[p_task_type],
      jsonb_build_object(
        'success', COALESCE((r.task_ratings->p_task_type->>'success')::int, 0) + 
                   CASE WHEN p_success THEN 1 ELSE 0 END,
        'fail', COALESCE((r.task_ratings->p_task_type->>'fail')::int, 0) + 
                CASE WHEN p_success THEN 0 ELSE 1 END
      )
    ),
    daily_used = r.daily_used + 1,
    updated_at = NOW()
  WHERE id = p_runner_id;
  
  -- Update model's tokens_used
  UPDATE models m
  SET tokens_used = m.tokens_used + p_tokens_used,
      tasks_completed = m.tasks_completed + CASE WHEN p_success THEN 1 ELSE 0 END,
      tasks_failed = m.tasks_failed + CASE WHEN p_success THEN 0 ELSE 1 END,
      updated_at = NOW()
  FROM runners r
  WHERE r.id = p_runner_id AND m.id = r.model_id;
END;
$$ LANGUAGE plpgsql;
```

### 2.7 RPC: refresh_limits

Called by Janitor to auto-refresh expired limits.

```sql
CREATE OR REPLACE FUNCTION refresh_limits()
RETURNS TABLE(runner_id UUID, action TEXT) AS $$
BEGIN
  -- Reset daily limits
  UPDATE runners
  SET daily_used = 0,
      daily_reset_at = NOW() + INTERVAL '1 day',
      status = CASE 
        WHEN status = 'cooldown' AND (cooldown_expires_at IS NULL OR cooldown_expires_at < NOW()) 
        THEN 'active'::text
        ELSE status
      END
  WHERE daily_reset_at IS NOT NULL AND daily_reset_at < NOW()
  RETURNING id, 'daily_reset'::TEXT;
  
  -- Clear cooldowns
  UPDATE runners
  SET status = 'active',
      cooldown_expires_at = NULL
  WHERE status = 'cooldown' AND cooldown_expires_at < NOW()
  RETURNING id, 'cooldown_cleared'::TEXT;
  
  -- Clear rate limits
  UPDATE runners
  SET status = 'active',
      rate_limit_reset_at = NULL
  WHERE status = 'rate_limited' AND rate_limit_reset_at < NOW()
  RETURNING id, 'rate_limit_cleared'::TEXT;
END;
$$ LANGUAGE plpgsql;
```

---

## 3. GO IMPLEMENTATION

### 3.1 New Package: internal/pool/model_pool.go

Intelligent model selection.

```go
package pool

import (
    "context"
    "time"
    
    "github.com/vibepilot/governor/internal/db"
)

type ModelPool struct {
    db *db.DB
}

func New(database *db.DB) *ModelPool {
    return &ModelPool{db: database}
}

type Runner struct {
    ID               string
    ModelID          string
    ToolID           string
    RoutingCapability []string
    CostPriority     int
    Status           string
    TaskRatings      map[string]Rating
}

type Rating struct {
    Success int `json:"success"`
    Fail    int `json:"fail"`
}

// SelectBest finds optimal runner for task
func (p *ModelPool) SelectBest(ctx context.Context, routing string, taskType string) (*Runner, error) {
    // Call get_best_runner RPC
    runnerID, err := p.db.GetBestRunner(ctx, routing, taskType)
    if err != nil {
        return nil, err
    }
    if runnerID == "" {
        return nil, nil // No available runner
    }
    
    return p.GetRunner(ctx, runnerID)
}

// RecordResult updates stats after task completes
func (p *ModelPool) RecordResult(ctx context.Context, runnerID string, taskType string, success bool, tokens int) error {
    return p.db.RecordRunnerResult(ctx, runnerID, taskType, success, tokens)
}

// SetCooldown puts runner in cooldown (80% limit hit)
func (p *ModelPool) SetCooldown(ctx context.Context, runnerID string, duration time.Duration) error {
    return p.db.SetRunnerCooldown(ctx, runnerID, time.Now().Add(duration))
}

// SetRateLimited marks runner as rate limited
func (p *ModelPool) SetRateLimited(ctx context.Context, runnerID string, resetAt time.Time) error {
    return p.db.SetRunnerRateLimited(ctx, runnerID, resetAt)
}
```

### 3.2 Modify: internal/db/supabase.go

Add RPC calls for model pool.

```go
// Add these methods to DB struct

func (d *DB) GetBestRunner(ctx context.Context, routing string, taskType string) (string, error) {
    data, err := d.rpc(ctx, "get_best_runner", map[string]interface{}{
        "p_routing":   routing,
        "p_task_type": taskType,
    })
    if err != nil {
        return "", err
    }
    
    var result string
    if err := json.Unmarshal(data, &result); err != nil {
        return "", err
    }
    return result, nil
}

func (d *DB) RecordRunnerResult(ctx context.Context, runnerID string, taskType string, success bool, tokens int) error {
    _, err := d.rpc(ctx, "record_runner_result", map[string]interface{}{
        "p_runner_id":   runnerID,
        "p_task_type":   taskType,
        "p_success":     success,
        "p_tokens_used": tokens,
    })
    return err
}

func (d *DB) RefreshLimits(ctx context.Context) error {
    _, err := d.rpc(ctx, "refresh_limits", nil)
    return err
}

func (d *DB) SetRunnerCooldown(ctx context.Context, runnerID string, expiresAt time.Time) error {
    body := map[string]interface{}{
        "status":              "cooldown",
        "cooldown_expires_at": expiresAt.Format(time.RFC3339),
        "updated_at":          time.Now().UTC().Format(time.RFC3339),
    }
    _, err := d.rest(ctx, "PATCH", "runners?id=eq."+runnerID, body)
    return err
}

func (d *DB) SetRunnerRateLimited(ctx context.Context, runnerID string, resetAt time.Time) error {
    body := map[string]interface{}{
        "status":              "rate_limited",
        "rate_limit_reset_at": resetAt.Format(time.RFC3339),
        "updated_at":          time.Now().UTC().Format(time.RFC3339),
    }
    _, err := d.rest(ctx, "PATCH", "runners?id=eq."+runnerID, body)
    return err
}
```

### 3.3 Modify: internal/dispatcher/dispatcher.go

Use model pool instead of config.

```go
type Dispatcher struct {
    db        *db.DB
    cfg       *config.Config
    pool      *pool.ModelPool
    leakDetector *security.LeakDetector
}

func New(database *db.DB, cfg *config.Config, leakDetector *security.LeakDetector) *Dispatcher {
    return &Dispatcher{
        db:           database,
        cfg:          cfg,
        pool:         pool.New(database),
        leakDetector: leakDetector,
    }
}

func (d *Dispatcher) execute(ctx context.Context, task types.Task) {
    // Get best runner from pool
    runner, err := d.pool.SelectBest(ctx, string(task.RoutingFlag), task.Type)
    if err != nil || runner == nil {
        log.Printf("Dispatcher: no runner for %s (routing=%s)", task.ID[:8], task.RoutingFlag)
        d.handleFailure(ctx, task)
        return
    }
    
    // Claim task
    if err := d.db.ClaimTask(ctx, task.ID, runner.ModelID); err != nil {
        log.Printf("Dispatcher: claim failed for %s: %v", task.ID[:8], err)
        return
    }
    
    // ... execute task ...
    
    // Record result AND update runner stats
    success := execErr == nil
    d.pool.RecordResult(ctx, runner.ID, task.Type, success, tokensIn+tokensOut)
}
```

### 3.4 Modify: internal/janitor/janitor.go

Add limit refresh job.

```go
func (j *Janitor) Run(ctx context.Context) {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            j.resetStuckTasks(ctx)
            j.refreshLimits(ctx)
        }
    }
}

func (j *Janitor) refreshLimits(ctx context.Context) {
    if err := j.db.RefreshLimits(ctx); err != nil {
        log.Printf("Janitor: failed to refresh limits: %v", err)
    }
}
```

---

## 4. STATUS TRANSITIONS

### Automatic Transitions (Janitor handles)

| From | To | Trigger | Resolution |
|------|-----|---------|------------|
| active | cooldown | 80% daily limit reached | Auto after cooldown timer |
| active | rate_limited | API rate limit hit | Auto after rate reset timer |
| cooldown | active | cooldown_expires_at < now | Janitor refresh_limits() |
| rate_limited | active | rate_limit_reset_at < now | Janitor refresh_limits() |

### Manual Transitions (Human via Vibes)

**Human required for:**
- Money decisions (credit top-up, subscriptions)
- System change decisions (architecture, new approaches)
- Visual UI/UX decisions (dashboard changes, interface revisions)
- Final approval on complex changes

| From | To | Trigger | Resolution |
|------|-----|---------|------------|
| active | paused | Credit depleted | Human tops up credit |
| paused | active | Credit added | Vibes updates DB |
| active | benched | Poor performance | Council review → Human approval |
| benched | active | Council + Human approve | Supervisor updates |

### 80% Throttle Logic

```go
// In dispatcher, after task completes on web platform
if runner.DailyLimit > 0 {
    usage := float64(runner.DailyUsed) / float64(runner.DailyLimit)
    if usage >= 0.80 {
        // Enter cooldown for remainder of day
        cooldownUntil := nextMidnight()
        d.pool.SetCooldown(ctx, runner.ID, cooldownUntil)
        log.Printf("Dispatcher: runner %s at %.0f%% daily, cooling down", runner.ID[:8], usage*100)
    }
}
```

---

## 5. IMPLEMENTATION ORDER

### Phase 3.1: Schema (30 min)
1. Create tools table
2. Create runners table
3. Add columns to models
4. Add columns to platforms
5. Create RPC functions
6. Seed initial data

### Phase 3.2: DB Layer (45 min)
1. Add GetBestRunner RPC call
2. Add RecordRunnerResult RPC call
3. Add RefreshLimits RPC call
4. Add status update methods

### Phase 3.3: Model Pool (30 min)
1. Create internal/pool package
2. Implement SelectBest
3. Implement RecordResult
4. Implement cooldown/rate limit methods

### Phase 3.4: Dispatcher Integration (30 min)
1. Replace selectModel() with pool.SelectBest()
2. Add RecordResult after task completion
3. Add 80% throttle check for web platforms

### Phase 3.5: Janitor Enhancement (15 min)
1. Add refreshLimits job
2. Run every minute

### Phase 3.6: Testing (30 min)
1. Create test task
2. Verify pool selection
3. Verify stats update
4. Verify cooldown behavior
5. Compare with Python orchestrator output

---

## 6. TESTING STRATEGY

### Unit Tests
- Model pool selection logic
- Rating calculation
- Status transitions

### Integration Tests
1. Create test task with routing_flag='internal'
2. Verify correct runner selected
3. Complete task successfully
4. Verify task_ratings updated
5. Verify daily_used incremented

### Cooldown Test
1. Set runner daily_limit=2
2. Run 2 tasks
3. Verify runner enters cooldown
4. Wait for cooldown or manually clear
5. Verify runner active again

### Comparison Test
1. Run same task through Python orchestrator
2. Run same task through Go Governor
3. Compare task_runs records
4. Compare model stats updates
5. Verify identical behavior

---

## 7. ROLLBACK PLAN

If issues arise:

1. **Schema rollback:**
   ```sql
   DROP TABLE IF EXISTS runners;
   DROP TABLE IF EXISTS tools;
   -- Revert models/platforms column additions
   ```

2. **Code rollback:**
   ```bash
   git checkout HEAD~1 -- governor/internal/dispatcher/
   git checkout HEAD~1 -- governor/internal/db/
   ```

3. **Keep Python running:**
   - Python orchestrator continues during testing
   - Go Governor runs in parallel
   - Compare outputs before cutover

---

## 8. SUCCESS CRITERIA

Phase 3 complete when:

- [ ] Schema created, RPCs working
- [ ] Model pool selects best available runner
- [ ] Stats update after each task
- [ ] Cooldown triggers at 80%
- [ ] Rate limits auto-expire
- [ ] Daily limits reset at midnight
- [ ] Dashboard shows correct model usage
- [ ] Adding new model = INSERT only (no code change)

---

## 9. FUTURE ENHANCEMENTS (Post-Phase 3)

- Vibes notification on credit depletion
- Dashboard admin for API key management
- A/B testing between models
- Predictive routing based on task complexity
- Model performance reports

---

**END OF PLAN**

*This document is the source of truth for Phase 3 implementation.*
*Do not deviate without updating this document first.*
