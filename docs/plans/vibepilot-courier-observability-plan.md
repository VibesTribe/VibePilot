# PLAN: VibePilot Courier System & Observability

## Overview
This plan implements Phase 2 (Courier System) and Phase 4 (Observability) from PRD v1.3. Phase 1 (Core Execution) is substantially complete in the Go Governor service.

## Estimated Total Context: 45,000 tokens
## Critical Path: T001 → T002 → T004 → T006 → T008

---

## Tasks

### T001: Create Platform Registry Schema
**Confidence:** 0.98
**Dependencies:** none
**Type:** feature
**Category:** database
**Requires Codebase:** false

#### Prompt Packet
```
# TASK: T001 - Create Platform Registry Schema

## CONTEXT
The PRD v1.3 describes a courier system that dispatches tasks to web platforms (ChatGPT, Claude, Gemini, etc.). We need a platform registry to track these external destinations, their limits, and performance metrics.

## WHAT TO BUILD
Create a database migration that adds a `platforms` table to track web-based AI platforms for the courier system.

## FILES TO CREATE
- `docs/supabase-schema/045_platform_registry.sql` - Platform registry migration

## TECHNICAL SPECIFICATIONS

### Database
- Table: `platforms`
- Columns:
  - id (UUID, primary key, default gen_random_uuid())
  - name (TEXT, not null, unique) - e.g., "chatgpt-free", "claude-free"
  - type (TEXT, not null, default 'web_courier')
  - url (TEXT) - Platform URL
  - gmail_account (TEXT) - Shared Gmail for login
  - capabilities (TEXT[], default '{}') - e.g., ['reasoning', 'code', 'research']
  - daily_limit (INT, default 10)
  - daily_used (INT, default 0)
  - success_count (INT, default 0)
  - failure_count (INT, default 0)
  - avg_response_time_ms (INT)
  - last_success (TIMESTAMPTZ)
  - last_failure (TIMESTAMPTZ)
  - status (TEXT, default 'active', CHECK IN ('active', 'benched', 'offline'))
  - status_reason (TEXT)
  - config (JSONB, default '{}')
  - created_at (TIMESTAMPTZ, default NOW())
  - updated_at (TIMESTAMPTZ, default NOW())

### Indexes
- idx_platforms_status ON platforms(status)
- idx_platforms_type ON platforms(type)

### RPC Functions
- `register_platform(p_name TEXT, p_url TEXT, p_capabilities TEXT[])` - Add new platform
- `update_platform_usage(p_platform_id UUID, p_success BOOLEAN, p_response_time_ms INT)` - Track usage
- `get_available_platforms(p_capability TEXT DEFAULT NULL)` - Get platforms under 80% limit

## ACCEPTANCE CRITERIA
- [ ] platforms table created with all columns
- [ ] Indexes created
- [ ] RPC functions created and working
- [ ] RLS policies allow service role full access

## TESTS REQUIRED
1. Insert platform, verify daily_used increments correctly
2. get_available_platforms returns only platforms under limit
3. update_platform_usage updates success/failure counts

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T001",
  "model_name": "[your model name]",
  "files_created": ["docs/supabase-schema/045_platform_registry.sql"],
  "files_modified": [],
  "summary": "Platform registry schema created",
  "tests_written": [],
  "notes": "Migration ready for Supabase"
}
```

## DO NOT
- Modify existing tables
- Add authentication logic
- Create application code (Go)
```

#### Expected Output
```json
{
  "files_created": ["docs/supabase-schema/045_platform_registry.sql"],
  "files_modified": [],
  "tests_required": ["verify migration runs cleanly"]
}
```

---

### T002: Create Courier Session Type
**Confidence:** 0.97
**Dependencies:** T001 (summary)
**Type:** feature
**Category:** coding
**Requires Codebase:** true

#### Prompt Packet
```
# TASK: T002 - Create Courier Session Type

## CONTEXT
The courier system needs a session type in the Go Governor that handles web platform dispatch. This is separate from CLI runners - couriers navigate to web platforms, submit prompts, and capture results.

## DEPENDENCIES

### Summary Dependencies
- T001: Platform registry schema created with platforms table for tracking web AI destinations, their limits, and performance metrics.

### Code Context Dependencies
- Read these files before starting:
  - internal/destinations/cli_runner.go (pattern for runner interface)
  - internal/destinations/api_runner.go (pattern for API destinations)
  - internal/runtime/session.go (session factory pattern)

## WHAT TO BUILD
Create a courier runner that:
1. Reads platform configuration from database
2. Opens a browser session (using Playwright or similar)
3. Navigates to the platform URL
4. Logs in using stored credentials (from vault)
5. Submits the prompt
6. Captures the response
7. Records the chat URL for later reference
8. Returns the result to the supervisor

## FILES TO CREATE
- `internal/destinations/courier_runner.go` - Courier runner implementation

## FILES TO MODIFY
- `internal/destinations/registry.go` - Register courier type

## TECHNICAL SPECIFICATIONS

### Language & Framework
- Language: Go
- Browser: Playwright for Go (github.com/playwright-community/playwright-go)
- Timeout: Configurable per platform (default 5 minutes)

### CourierRunner struct
```go
type CourierRunner struct {
    platformID string
    db         *db.DB
    vault      *vault.Vault
    browser    playwright.Browser
    timeout    time.Duration
}

func (c *CourierRunner) Run(ctx context.Context, input map[string]any) (*Result, error)
```

### Run Method Behavior
1. Get platform config from database
2. Get credentials from vault (gmail_account, password)
3. Launch browser (headless mode)
4. Navigate to platform URL
5. Handle login if required
6. Find input field and submit prompt
7. Wait for response (with timeout)
8. Capture response text and chat URL
9. Update platform usage stats
10. Close browser and return result

## ACCEPTANCE CRITERIA
- [ ] CourierRunner implements Runner interface
- [ ] Credentials retrieved from vault (not hardcoded)
- [ ] Browser launched in headless mode
- [ ] Response captured and returned
- [ ] Chat URL included in result
- [ ] Usage stats updated in platforms table
- [ ] Timeout enforced

## TESTS REQUIRED
1. Unit test: CourierRunner creation with valid config
2. Integration test: Mock platform navigation
3. Error handling: Timeout returns error

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T002",
  "model_name": "[your model name]",
  "files_created": ["internal/destinations/courier_runner.go"],
  "files_modified": ["internal/destinations/registry.go"],
  "summary": "Courier runner implemented",
  "tests_written": [],
  "notes": "Requires playwright-go dependency"
}
```

## DO NOT
- Hardcode credentials
- Store credentials in code
- Skip error handling
- Ignore timeouts
```

#### Expected Output
```json
{
  "files_created": ["internal/destinations/courier_runner.go"],
  "files_modified": ["internal/destinations/registry.go"],
  "tests_required": ["internal/destinations/courier_runner_test.go"]
}
```

---

### T003: Create ROI Calculator RPC
**Confidence:** 0.96
**Dependencies:** none
**Type:** feature
**Category:** database
**Requires Codebase:** false

#### Prompt Packet
```
# TASK: T003 - Create ROI Calculator RPC

## CONTEXT
PRD v1.3 Section 9 describes ROI tracking to compare actual costs (courier time) vs theoretical API costs. This informs platform selection and model subscription decisions.

## WHAT TO BUILD
Create database RPCs for calculating and storing ROI metrics on task completions.

## FILES TO CREATE
- `docs/supabase-schema/046_roi_calculator.sql` - ROI calculation functions

## TECHNICAL SPECIFICATIONS

### Database
Add columns to task_runs table (if not exists):
- theoretical_api_cost (DECIMAL, default 0)
- actual_cost (DECIMAL, default 0)
- savings (DECIMAL, default 0)
- savings_percentage (DECIMAL, default 0)

### RPC Functions

1. `calculate_task_roi(p_task_run_id UUID, p_tokens_used INT, p_model_id TEXT, p_duration_seconds INT)`
   - Calculates theoretical API cost from token count and model rate
   - Actual cost = duration_seconds * COURIER_RATE (configurable, default $0.10/min)
   - Savings = theoretical - actual
   - Updates task_runs row

2. `get_roi_report(p_hours INT DEFAULT 24)`
   - Returns aggregated ROI metrics for the last N hours
   - Groups by model_id, platform, task_type
   - Returns: total_tasks, success_rate, avg_tokens, avg_duration, total_theoretical, total_actual, avg_savings_pct

3. `get_model_roi_ranking()`
   - Returns models ranked by ROI efficiency
   - Helps routing decisions

### API Rate Table
Create `api_rates` table:
- model_id (TEXT, primary key)
- rate_per_1k_tokens (DECIMAL)
- effective_date (DATE)
- source (TEXT) - e.g., "openai_pricing", "anthropic_pricing"

Seed with common models:
- gpt-4: $0.03/1k input
- gpt-3.5-turbo: $0.001/1k input
- claude-3-opus: $0.015/1k input
- claude-3-sonnet: $0.003/1k input
- gemini-pro: $0.00025/1k input

## ACCEPTANCE CRITERIA
- [ ] api_rates table created and seeded
- [ ] task_runs columns added
- [ ] calculate_task_roi RPC working
- [ ] get_roi_report returns aggregated data
- [ ] get_model_roi_ranking returns sorted list

## TESTS REQUIRED
1. Verify calculate_task_roi computes correct values
2. Verify get_roi_report aggregates correctly
3. Verify savings_percentage calculated correctly

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T003",
  "model_name": "[your model name]",
  "files_created": ["docs/supabase-schema/046_roi_calculator.sql"],
  "files_modified": [],
  "summary": "ROI calculator RPCs created",
  "tests_written": [],
  "notes": "API rates seeded for common models"
}
```

## DO NOT
- Modify existing task_runs data
- Hardcode rates in functions (use table)
```

#### Expected Output
```json
{
  "files_created": ["docs/supabase-schema/046_roi_calculator.sql"],
  "files_modified": [],
  "tests_required": []
}
```

---

### T004: Implement ROI Tracker in Governor
**Confidence:** 0.95
**Dependencies:** T003 (code_context)
**Type:** feature
**Category:** coding
**Requires Codebase:** true

#### Prompt Packet
```
# TASK: T004 - Implement ROI Tracker in Governor

## CONTEXT
The Governor needs to call the ROI calculator RPCs after task completion to track costs and inform future routing decisions.

## DEPENDENCIES

### Summary Dependencies
- T003: ROI calculator RPCs created with calculate_task_roi, get_roi_report functions

### Code Context Dependencies
- Read these files before starting:
  - internal/runtime/usage_tracker.go (existing tracking patterns)
  - cmd/governor/main.go (event handlers)
  - internal/db/rpc.go (RPC calling patterns)

## WHAT TO BUILD
Add ROI tracking to the task completion flow in the Governor.

## FILES TO CREATE
- `internal/runtime/roi_tracker.go` - ROI tracking logic

## FILES TO MODIFY
- `cmd/governor/main.go` - Call ROI tracker after task completion

## TECHNICAL SPECIFICATIONS

### Language & Framework
- Language: Go
- Integration point: EventTaskCompleted and EventTaskAvailable handlers

### ROITracker struct
```go
type ROITracker struct {
    db *db.DB
}

func (r *ROITracker) RecordTaskROI(ctx context.Context, taskRunID string, tokensUsed int, modelID string, durationSeconds int) error {
    _, err := r.db.RPC(ctx, "calculate_task_roi", map[string]any{
        "p_task_run_id":      taskRunID,
        "p_tokens_used":      tokensUsed,
        "p_model_id":         modelID,
        "p_duration_seconds": durationSeconds,
    })
    return err
}

func (r *ROITracker) GetROIReport(ctx context.Context, hours int) ([]ROISummary, error) {
    data, err := r.db.RPC(ctx, "get_roi_report", map[string]any{
        "p_hours": hours,
    })
    // parse and return
}
```

### Integration Points
1. In EventTaskAvailable handler, after successful task completion:
   - Call roiTracker.RecordTaskROI with tokens, model, duration
2. In config, add:
   - roi.enabled (bool, default true)
   - roi.courier_rate_per_minute (float, default 0.10)

## ACCEPTANCE CRITERIA
- [ ] ROITracker created with RecordTaskROI method
- [ ] ROI recorded after each task completion
- [ ] Config values respected
- [ ] Errors logged but don't fail task

## TESTS REQUIRED
1. Unit test: ROITracker.RecordTaskROI calls RPC correctly
2. Integration test: Task completion triggers ROI recording

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T004",
  "model_name": "[your model name]",
  "files_created": ["internal/runtime/roi_tracker.go"],
  "files_modified": ["cmd/governor/main.go", "internal/runtime/config.go"],
  "summary": "ROI tracking integrated into task flow",
  "tests_written": [],
  "notes": "ROI recorded after each task completion"
}
```

## DO NOT
- Block task completion on ROI recording failure
- Hardcode courier rate
- Modify database schema
```

#### Expected Output
```json
{
  "files_created": ["internal/runtime/roi_tracker.go"],
  "files_modified": ["cmd/governor/main.go", "internal/runtime/config.go"],
  "tests_required": ["internal/runtime/roi_tracker_test.go"]
}
```

---

### T005: Create Observability Dashboard Schema
**Confidence:** 0.98
**Dependencies:** none
**Type:** feature
**Category:** database
**Requires Codebase:** false

#### Prompt Packet
```
# TASK: T005 - Create Observability Dashboard Schema

## CONTEXT
PRD v1.3 Section 13 describes observability metrics and a dashboard (Vibeflow). We need database views and RPCs to power the dashboard.

## WHAT TO BUILD
Create database views and RPCs for dashboard metrics.

## FILES TO CREATE
- `docs/supabase-schema/047_observability_views.sql` - Dashboard views and RPCs

## TECHNICAL SPECIFICATIONS

### Views

1. `dashboard_task_summary`
   - status, count, avg_duration_seconds
   - Filtered to last 24 hours

2. `dashboard_model_health`
   - model_id, success_rate, avg_duration, tasks_completed, current_status
   - From task_runs and models tables

3. `dashboard_platform_health`
   - platform_id, success_rate, daily_usage_pct, status
   - From platforms table

4. `dashboard_roi_metrics`
   - date, total_tasks, total_savings, avg_savings_pct
   - Aggregated daily

### RPCs

1. `get_dashboard_data()`
   - Returns JSON with all dashboard views combined
   - Single call for dashboard load

2. `get_failure_queue()`
   - Returns tasks with status='escalated' or failed 3+ times
   - For failure queue display

3. `get_active_runners()`
   - Returns currently processing tasks with runner info

## ACCEPTANCE CRITERIA
- [ ] All views created
- [ ] get_dashboard_data returns combined data
- [ ] get_failure_queue returns escalated tasks
- [ ] get_active_runners returns in-progress tasks

## TESTS REQUIRED
1. Verify views return correct aggregations
2. Verify get_dashboard_data returns all sections
3. Verify get_failure_queue filters correctly

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T005",
  "model_name": "[your model name]",
  "files_created": ["docs/supabase-schema/047_observability_views.sql"],
  "files_modified": [],
  "summary": "Dashboard views and RPCs created",
  "tests_written": [],
  "notes": "Views ready for Vibeflow dashboard"
}
```

## DO NOT
- Create frontend code
- Modify existing tables
```

#### Expected Output
```json
{
  "files_created": ["docs/supabase-schema/047_observability_views.sql"],
  "files_modified": [],
  "tests_required": []
}
```

---

### T006: Create Budget Enforcement Service
**Confidence:** 0.96
**Dependencies:** T003 (summary)
**Type:** feature
**Category:** coding
**Requires Codebase:** true

#### Prompt Packet
```
# TASK: T006 - Create Budget Enforcement Service

## CONTEXT
PRD v1.3 Section 8 describes budget thresholds (70% warning, 80% hard limit). The Governor needs to enforce these limits and throttle/pause tasks when limits are reached.

## DEPENDENCIES

### Summary Dependencies
- T003: ROI calculator tracks token usage per model

### Code Context Dependencies
- Read these files before starting:
  - internal/runtime/config.go (configuration patterns)
  - internal/runtime/usage_tracker.go (existing usage tracking)
  - cmd/governor/main.go (event routing)

## WHAT TO BUILD
Create a budget enforcement service that checks limits before task dispatch.

## FILES TO CREATE
- `internal/runtime/budget_enforcer.go` - Budget checking and enforcement

## FILES TO MODIFY
- `cmd/governor/main.go` - Check budget before task dispatch
- `internal/runtime/config.go` - Add budget configuration

## TECHNICAL SPECIFICATIONS

### Language & Framework
- Language: Go
- Integration: Before task dispatch in EventTaskAvailable

### BudgetConfig
```go
type BudgetConfig struct {
    ContextWarningPct    float64 `yaml:"context_warning_pct" default:"0.70"`
    ContextHardLimitPct  float64 `yaml:"context_hard_limit_pct" default:"0.80"`
    TokenWarningPct      float64 `yaml:"token_warning_pct" default:"0.70"`
    TokenHardLimitPct    float64 `yaml:"token_hard_limit_pct" default:"0.80"`
    RequestWarningPct    float64 `yaml:"request_warning_pct" default:"0.70"`
    RequestHardLimitPct  float64 `yaml:"request_hard_limit_pct" default:"0.80"`
}
```

### BudgetEnforcer
```go
type BudgetEnforcer struct {
    db     *db.DB
    config *BudgetConfig
}

type BudgetStatus struct {
    CanProceed     bool
    WarningLevel   string  // "none", "warning", "critical"
    BlockedReason  string
    Metrics        BudgetMetrics
}

func (b *BudgetEnforcer) CheckBudget(ctx context.Context, modelID string, estimatedTokens int) (*BudgetStatus, error)
```

### Behavior
1. Before dispatching task, call CheckBudget
2. If CanProceed is false:
   - Log blocked reason
   - Try alternate model if available
   - If no alternates, set task status to 'blocked_budget'
3. If WarningLevel is "warning":
   - Log warning
   - Continue but flag for attention

## ACCEPTANCE CRITERIA
- [ ] BudgetEnforcer checks all limit types
- [ ] Tasks blocked at hard limit
- [ ] Warnings logged at warning threshold
- [ ] Alternate model selection attempted

## TESTS REQUIRED
1. Unit test: CheckBudget returns correct status
2. Integration test: Task blocked at hard limit
3. Integration test: Warning logged at warning threshold

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T006",
  "model_name": "[your model name]",
  "files_created": ["internal/runtime/budget_enforcer.go"],
  "files_modified": ["cmd/governor/main.go", "internal/runtime/config.go"],
  "summary": "Budget enforcement implemented",
  "tests_written": [],
  "notes": "Tasks blocked when budget limits reached"
}
```

## DO NOT
- Modify database schema
- Block without logging reason
- Skip alternate model selection
```

#### Expected Output
```json
{
  "files_created": ["internal/runtime/budget_enforcer.go"],
  "files_modified": ["cmd/governor/main.go", "internal/runtime/config.go"],
  "tests_required": ["internal/runtime/budget_enforcer_test.go"]
}
```

---

### T007: Create Error Classifier
**Confidence:** 0.97
**Dependencies:** none
**Type:** feature
**Category:** coding
**Requires Codebase:** true

#### Prompt Packet
```
# TASK: T007 - Create Error Classifier

## CONTEXT
PRD v1.3 Section 7 describes error classification codes (E001-E010) with specific retry policies. The Governor needs to classify errors and apply the correct retry policy.

## DEPENDENCIES
None

### Code Context Dependencies
- Read these files before starting:
  - internal/runtime/decision.go (existing failure categorization)
  - cmd/governor/main.go (error handling patterns)

## WHAT TO BUILD
Create an error classifier that maps errors to codes and determines retry actions.

## FILES TO CREATE
- `internal/runtime/error_classifier.go` - Error classification and retry logic

## FILES TO MODIFY
- `cmd/governor/main.go` - Use ErrorClassifier for error handling

## TECHNICAL SPECIFICATIONS

### Language & Framework
- Language: Go

### Error Codes (from PRD)
```go
const (
    ErrModelError       = "E001" // Model produced invalid output
    ErrNetworkError     = "E002" // Connection/timeout
    ErrPlatformError    = "E003" // Rate limit, captcha
    ErrLogicError       = "E004" // Fundamental flaw
    ErrCLIError         = "E005" // CLI tool failure
    ErrContextOverflow  = "E006" // Exceeded context window
    ErrDependencyError  = "E007" // Dependency task failed
    ErrTestFailure      = "E008" // Tests did not pass
    ErrReviewRejection  = "E009" // Supervisor rejected
    ErrTimeout          = "E010" // Execution > 30 minutes
)
```

### RetryPolicy
```go
type RetryPolicy struct {
    MaxAttempts int
    Action      string // "same", "switch_model", "split_task", "escalate", "block"
    Backoff     string // "none", "linear", "exponential"
}

var RetryPolicies = map[string]RetryPolicy{
    ErrModelError:      {MaxAttempts: 3, Action: "switch_model", Backoff: "exponential"},
    ErrNetworkError:    {MaxAttempts: 3, Action: "same", Backoff: "exponential"},
    ErrPlatformError:   {MaxAttempts: 2, Action: "escalate", Backoff: "none"},
    ErrLogicError:      {MaxAttempts: 0, Action: "escalate", Backoff: "none"},
    ErrCLIError:        {MaxAttempts: 2, Action: "same", Backoff: "linear"},
    ErrContextOverflow: {MaxAttempts: 1, Action: "split_task", Backoff: "none"},
    ErrDependencyError: {MaxAttempts: 0, Action: "block", Backoff: "none"},
    ErrTestFailure:     {MaxAttempts: 3, Action: "same", Backoff: "none"},
    ErrReviewRejection: {MaxAttempts: 3, Action: "same", Backoff: "none"},
    ErrTimeout:         {MaxAttempts: 1, Action: "escalate", Backoff: "none"},
}
```

### ErrorClassifier
```go
type ErrorClassifier struct{}

type ClassifiedError struct {
    Code        string
    Category    string
    Message     string
    RetryPolicy RetryPolicy
}

func (c *ErrorClassifier) Classify(err error) *ClassifiedError
```

### Classification Logic
- ErrModelError: Output doesn't match expected format, JSON parse failure
- ErrNetworkError: Context deadline exceeded, connection refused
- ErrPlatformError: Rate limit message, captcha detected
- ErrLogicError: Impossible state, constraint violation
- ErrCLIError: Non-zero exit code from CLI
- ErrContextOverflow: Token count exceeds limit
- ErrTimeout: Duration > 30 minutes
- ErrTestFailure: Test command returned non-zero
- ErrReviewRejection: Supervisor decision = "fail"

## ACCEPTANCE CRITERIA
- [ ] All error codes defined
- [ ] Retry policies match PRD
- [ ] Classify() correctly identifies error types
- [ ] Integrated into error handling flow

## TESTS REQUIRED
1. Unit test: Each error type classified correctly
2. Unit test: Retry policies return correct values
3. Integration test: Error triggers correct retry action

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T007",
  "model_name": "[your model name]",
  "files_created": ["internal/runtime/error_classifier.go"],
  "files_modified": ["cmd/governor/main.go"],
  "summary": "Error classifier implemented",
  "tests_written": [],
  "notes": "All PRD error codes and retry policies implemented"
}
```

## DO NOT
- Add new error codes not in PRD
- Modify existing database schema
- Change existing error handling without classification
```

#### Expected Output
```json
{
  "files_created": ["internal/runtime/error_classifier.go"],
  "files_modified": ["cmd/governor/main.go"],
  "tests_required": ["internal/runtime/error_classifier_test.go"]
}
```

---

### T008: Create Recovery Protocol Handler
**Confidence:** 0.95
**Dependencies:** T007 (summary)
**Type:** feature
**Category:** coding
**Requires Codebase:** true

#### Prompt Packet
```
# TASK: T008 - Create Recovery Protocol Handler

## CONTEXT
PRD v1.3 Section 14 describes recovery protocols for model/platform outage and system recovery. The Governor needs to detect degradation and handle recovery.

## DEPENDENCIES

### Summary Dependencies
- T007: Error classifier provides error codes for detecting patterns

### Code Context Dependencies
- Read these files before starting:
  - cmd/governor/main.go (startup recovery, existing patterns)
  - internal/db/supabase.go (database operations)

## WHAT TO BUILD
Create a recovery handler that detects degradation patterns and manages recovery.

## FILES TO CREATE
- `internal/runtime/recovery.go` - Recovery protocol implementation

## FILES TO MODIFY
- `cmd/governor/main.go` - Integrate recovery handler

## TECHNICAL SPECIFICATIONS

### Language & Framework
- Language: Go

### RecoveryConfig
```go
type RecoveryConfig struct {
    ConsecutiveFailuresThreshold int           `yaml:"consecutive_failures_threshold" default:"3"`
    DegradedCheckInterval        time.Duration `yaml:"degraded_check_interval" default:"5m"`
    RecoveryCheckInterval        time.Duration `yaml:"recovery_check_interval" default:"1m"`
}
```

### RecoveryHandler
```go
type RecoveryHandler struct {
    db           *db.DB
    config       *RecoveryConfig
    degraded     map[string]time.Time // model/destination ID -> degraded since
    failureCount map[string]int       // model/destination ID -> consecutive failures
}

func (r *RecoveryHandler) RecordFailure(modelOrDestID string) error
func (r *RecoveryHandler) RecordSuccess(modelOrDestID string) error
func (r *RecoveryHandler) IsDegraded(modelOrDestID string) bool
func (r *RecoveryHandler) GetHealthyAlternatives(capability string) ([]string, error)
```

### Degradation Detection
1. Track consecutive failures per model/destination
2. If failures >= threshold, mark as degraded
3. Log degradation event
4. Pause dispatch to degraded entity
5. Check for recovery periodically

### Recovery Flow
1. When entity marked degraded:
   - Log event with timestamp
   - Update status in models/destinations table to 'benched'
2. Periodic recovery check:
   - Send probe task (simple validation)
   - If success, restore to 'active'
   - Log recovery event

### System Recovery (Startup)
Already implemented in main.go as `runStartupRecovery`. Ensure it:
1. Clears processing_by for stale records
2. Resets in_progress tasks to available
3. Logs recovery actions

## ACCEPTANCE CRITERIA
- [ ] Consecutive failures tracked per entity
- [ ] Degradation detected at threshold
- [ ] Status updated in database
- [ ] Healthy alternatives returned
- [ ] Recovery probe attempted periodically

## TESTS REQUIRED
1. Unit test: Degradation detected after threshold
2. Unit test: Recovery clears degraded state
3. Integration test: Tasks routed away from degraded entities

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T008",
  "model_name": "[your model name]",
  "files_created": ["internal/runtime/recovery.go"],
  "files_modified": ["cmd/governor/main.go"],
  "summary": "Recovery protocol handler implemented",
  "tests_written": [],
  "notes": "Degradation detection and recovery working"
}
```

## DO NOT
- Block all dispatch on single entity failure
- Skip logging recovery events
- Infinite retry on recovery probe
```

#### Expected Output
```json
{
  "files_created": ["internal/runtime/recovery.go"],
  "files_modified": ["cmd/governor/main.go"],
  "tests_required": ["internal/runtime/recovery_test.go"]
}
```

---

### T009: Create Health Check Endpoint
**Confidence:** 0.98
**Dependencies:** none
**Type:** feature
**Category:** api
**Requires Codebase:** true

#### Prompt Packet
```
# TASK: T009 - Create Health Check Endpoint

## CONTEXT
The Governor needs a health check endpoint for monitoring and orchestration. This is referenced in PRD Section 2.1 (Gateway metrics endpoint).

## DEPENDENCIES
None

### Code Context Dependencies
- Read these files before starting:
  - cmd/governor/main.go (main entry point)
  - config/governor.yaml.example (configuration patterns)

## WHAT TO BUILD
Add HTTP health check endpoint to Governor.

## FILES TO MODIFY
- `cmd/governor/main.go` - Add HTTP server with health endpoint

## TECHNICAL SPECIFICATIONS

### Language & Framework
- Language: Go
- Package: net/http (standard library)

### Health Endpoint
- Path: `/health`
- Method: GET
- Port: Configurable (default 8080)

### Response
```json
{
  "status": "healthy",
  "version": "2.0.0",
  "uptime_seconds": 3600,
  "database": "connected",
  "active_tasks": 5,
  "pool_size": 3
}
```

### Implementation
1. Add config field: `health_check_port` (default 8080)
2. Start HTTP server in goroutine
3. Endpoint checks:
   - Database connection (ping)
   - Returns 200 if healthy
   - Returns 503 if database unreachable

## ACCEPTANCE CRITERIA
- [ ] GET /health returns JSON status
- [ ] Returns 200 when healthy
- [ ] Returns 503 when database unreachable
- [ ] Port configurable

## TESTS REQUIRED
1. Integration test: Health check returns 200
2. Integration test: Returns 503 when database down

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T009",
  "model_name": "[your model name]",
  "files_created": [],
  "files_modified": ["cmd/governor/main.go"],
  "summary": "Health check endpoint added",
  "tests_written": [],
  "notes": "Endpoint ready at /health"
}
```

## DO NOT
- Add authentication to health endpoint
- Include sensitive data in response
- Block on health check
```

#### Expected Output
```json
{
  "files_created": [],
  "files_modified": ["cmd/governor/main.go"],
  "tests_required": []
}
```

---

### T010: Add Playwright Dependency and Documentation
**Confidence:** 0.99
**Dependencies:** T002 (summary)
**Type:** feature
**Category:** configuration
**Requires Codebase:** false

#### Prompt Packet
```
# TASK: T010 - Add Playwright Dependency and Documentation

## CONTEXT
The Courier runner (T002) requires Playwright for browser automation. This task adds the dependency and documents setup.

## DEPENDENCIES

### Summary Dependencies
- T002: Courier runner uses Playwright for web platform navigation

## WHAT TO BUILD
Add Playwright dependency to Go module and create setup documentation.

## FILES TO MODIFY
- `go.mod` - Add playwright-go dependency
- `go.sum` - Update checksums

## FILES TO CREATE
- `docs/PLAYWRIGHT_SETUP.md` - Setup instructions

## TECHNICAL SPECIFICATIONS

### Dependency
```bash
go get github.com/playwright-community/playwright-go
```

### go.mod addition
```
require github.com/playwright-community/playwright-go v0.4201.1
```

### Documentation Content
- Prerequisites (Go 1.18+)
- Installation: `go get github.com/playwright-community/playwright-go`
- Browser install: `go run github.com/playwright-community/playwright-go/cmd/playwright install`
- Environment: May need to install system dependencies on Linux
- Headless mode configuration
- Troubleshooting common issues

## ACCEPTANCE CRITERIA
- [ ] Dependency added to go.mod
- [ ] go.sum updated
- [ ] Setup documentation created
- [ ] Installation instructions clear

## TESTS REQUIRED
1. Verify `go mod tidy` succeeds
2. Verify `go build` succeeds with new dependency

## OUTPUT FORMAT
Return JSON:
```json
{
  "task_id": "T010",
  "model_name": "[your model name]",
  "files_created": ["docs/PLAYWRIGHT_SETUP.md"],
  "files_modified": ["go.mod", "go.sum"],
  "summary": "Playwright dependency and documentation added",
  "tests_written": [],
  "notes": "Run 'go mod tidy' after merge"
}
```

## DO NOT
- Add other unnecessary dependencies
- Skip documentation
```

#### Expected Output
```json
{
  "files_created": ["docs/PLAYWRIGHT_SETUP.md"],
  "files_modified": ["go.mod", "go.sum"],
  "tests_required": []
}
```

---

## Task Summary

| Task | Category | Confidence | Dependencies | Est. Context |
|------|----------|------------|--------------|--------------|
| T001 | database | 0.98 | none | 3,000 |
| T002 | coding | 0.97 | T001 | 8,000 |
| T003 | database | 0.96 | none | 4,000 |
| T004 | coding | 0.95 | T003 | 5,000 |
| T005 | database | 0.98 | none | 3,000 |
| T006 | coding | 0.96 | T003 | 6,000 |
| T007 | coding | 0.97 | none | 4,000 |
| T008 | coding | 0.95 | T007 | 5,000 |
| T009 | api | 0.98 | none | 2,000 |
| T010 | configuration | 0.99 | T002 | 1,000 |

## Execution Order

**Phase 1 (No dependencies, parallelizable):**
- T001: Platform Registry Schema
- T003: ROI Calculator RPC
- T005: Observability Views
- T007: Error Classifier
- T009: Health Check Endpoint

**Phase 2 (After Phase 1):**
- T002: Courier Session (after T001)
- T004: ROI Tracker (after T003)
- T006: Budget Enforcer (after T003)
- T008: Recovery Handler (after T007)

**Phase 3 (Final):**
- T010: Playwright Setup (after T002)

## Warnings
- T002 (Courier Runner) is the largest task at 8K context
- Playwright requires browser installation on host system
- Courier runner credentials must come from vault (security requirement)
