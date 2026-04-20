# Orchestrator Rate Limit & Usage Tracking Enhancement Plan

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task.

**Goal:** Implement persistent usage tracking, shared connector limits, web platform limit tracking, and courier dual-envelope rate limiting while maintaining full compatibility with existing dashboard and Supabase schemas.

**Architecture:** 
- Add persistent storage of usage windows and learned data via Supabase RPC calls on startup/shutdown
- Implement connector-level usage aggregation for shared limits (Groq org TPD, etc)
- Add web platform limit schema and tracking for courier destinations
- Create dual-envelope tracking for courier tasks (fueling model + web platform)
- Maintain backward compatibility - all existing fields and APIs remain unchanged

**Tech Stack:** Go (governor), Supabase (PostgreSQL), JSON config files, existing RPC patterns

---

## Task 1: Add LoadFromDatabase to UsageTracker

**Objective:** On governor startup, load persisted usage windows, cooldowns, and learned data from Supabase into the in-memory UsageTracker so rate limit tracking survives restarts.

**Files:**
- Modify: `/home/vibes/VibePilot/governor/internal/runtime/usage_tracker.go:1-200` (add LoadFromDatabase method)
- Modify: `/home/vibes/VibePilot/governor/cmd/governor/main.go:1-100` (call LoadFromDatabase on startup)

**Step 1: Write failing test for LoadFromDatabase**

```go
func TestUsageTracker_LoadFromDatabase(t *testing.T) {
    // Setup mock DB with test data
    // Call LoadFromDatabase
    // Assert that usage windows, cooldowns, and learned data are loaded
    // Assert that CanMakeRequest reflects loaded state
}
```

**Step 2: Run test to verify failure**

Run: `go test ./internal/runtime -run TestUsageTracker_LoadFromDatabase -v`
Expected: FAIL — undefined: LoadFromDatabase

**Step 3: Write minimal implementation**

```go
// In usage_tracker.go
func (ut *UsageTracker) LoadFromDatabase(ctx context.Context, db DB) error {
    // Query Supabase for all models with usage_windows, cooldown_expires_at, learned
    // For each model, update in-memory profile
    return nil
}

// In main.go
// After setting up DB connection and before starting event handlers:
if err := ut.LoadFromDatabase(ctx, h.database); err != nil {
    log.Printf("[Main] UsageTracker LoadFromDatabase error: %v", err)
}
```

**Step 4: Run test to verify pass**

Run: `go test ./internal/runtime -run TestUsageTracker_LoadFromDatabase -v`
Expected: PASS

**Step 5: Commit**

```bash
git add governor/internal/runtime/usage_tracker.go governor/cmd/governor/main.go
git commit -m "feat: add LoadFromDatabase to UsageTracker for persistent rate limit tracking"
```

---

## Task 2: Modify PersistToDatabase to include learned data

**Objective:** Ensure that when PersistToDatabase runs (every 30s and on shutdown), it persists not just usage windows and cooldowns but also the learned data (best_for_task_types, failure rates, optimal cooldown).

**Files:**
- Modify: `/home/vibes/VibePilot/governor/internal/runtime/usage_tracker.go:200-400` (update PersistToDatabase payload)
- Modify: `/home/vibes/VibePilot/governor/internal/runtime/usage_tracker.go:400-600` (update FromDatabaseRow to handle learned data)

**Step 1: Write failing test for learned data persistence**

```go
func TestUsageTracker_PersistToDatabase_IncludesLearned(t *testing.T) {
    // Set learned data on a model profile
    // Call PersistToDatabase (mock)
    // Assert that the SQL payload includes learned JSON
}
```

**Step 2: Run test to verify failure**

Run: `go test ./internal/runtime -run TestUsageTracker_PersistToDatabase_IncludesLearned -v`
Expected: FAIL

**Step 3: Write minimal implementation**

```go
// In PersistToDatabase, add learned to the map:
learnedJSON, _ := json.Marshal(profile.Learned)
// Include in the JSONB payload for update_model_usage RPC
```

**Step 4: Run test to verify pass**

Run: `go test ./internal/runtime -run TestUsageTracker_PersistToDatabase_IncludesLearned -v`
Expected: PASS

**Step 5: Commit**

```bash
git add governor/internal/runtime/usage_tracker.go
git commit -m "feat: persist learned data in UsageTracker PersistToDatabase"
```

---

## Task 3: Add connector-level usage tracking

**Objective:** Track usage per (model_id, connector_id) pair to enable proactive shared limit enforcement (e.g., Groq org 100K TPD shared across all Groq models).

**Files:**
- Create: `/home/vibes/VibePilot/governor/internal/runtime/connector_usage_tracker.go` (new file)
- Modify: `/home/vibes/VibePilot/governor/internal/runtime/usage_tracker.go` (delegate to connector tracker)
- Modify: `/home/vibes/VibePilot/governor/internal/runtime/model_loader.go` (register connector profiles)
- Modify: `/home/vibes/VibePilot/governor/cmd/governor/main.go` (persist connector usage)

**Step 1: Write failing test for ConnectorUsageTracker**

```go
func TestConnectorUsageTracker_TrackUsage(t *testing.T) {
    // Create tracker with test limits
    // Record usage for model A on connector X
    // Record usage for model B on same connector X
    // Assert that combined usage is tracked for connector X
    // Assert that CanMakeRequestVia respects connector limits
}
```

**Step 2: Run test to verify failure**

Run: `go test ./internal/runtime -run TestConnectorUsageTracker_TrackUsage -v`
Expected: FAIL

**Step 3: Write minimal implementation**

```go
// connector_usage_tracker.go
type ConnectorUsageTracker struct {
    mu sync.RWMutex
    connectorUsage map[string]*ConnectorProfile // connectorID -> profile
}

func NewConnectorUsageTracker(profiles map[string]ConnectorProfile) *ConnectorUsageTracker {
    // Initialize with profiles from models.json
}

// TrackUsage(modelID, connectorID, tokensIn, tokensOut)
// CanMakeRequestVia(modelID, connectorID, estimatedTokens)
```

**Step 4: Run test to verify pass**

Run: `go test ./internal/runtime -run TestConnectorUsageTracker_TrackUsage -v`
Expected: PASS

**Step 5: Commit**

```bash
git add governor/internal/runtime/connector_usage_tracker.go \
       governor/internal/runtime/usage_tracker.go \
       governor/internal/runtime/model_loader.go \
       governor/cmd/governor/main.go
git commit -m "feat: add connector-level usage tracking for shared limits"
```

---

## Task 4: Add web platform limit schema and tracking

**Objective:** Add structured web platform limits (messages_per_3h, messages_per_session, tokens_per_day) to connectors.json and implement tracking similar to UsageTracker but for web destinations used by couriers.

**Files:**
- Create: `/home/vibes/VibePilot/governor/internal/runtime/platform_usage_tracker.go` (new file)
- Modify: `/home/vibes/VibePilot/governor/internal/runtime/platform_usage_tracker.go` (persist to Supabase platforms table)
- Modify: `/home/vibes/VibePilot/governor/internal/runtime/courier.go` (track usage after courier completes)
- Modify: `/home/vibes/VibePilot/governor/config/connectors.json` (add limit_schema field to web destinations)

**Step 1: Write failing test for PlatformUsageTracker**

```go
func TestPlatformUsageTracker_TrackMessages(t *testing.T) {
    // Create tracker with test limits (10 messages/3h)
    // Simulate 8 message sends
    // Assert CanMakeRequest returns true
    // Simulate 3 more message sends (total 11)
    // Assert CanMakeRequest returns false with wait time
}
```

**Step 2: Run test to verify failure**

Run: `go test ./internal/runtime -run TestPlatformUsageTracker_TrackMessages -v`
Expected: FAIL

**Step 3: Write minimal implementation**

```go
// platform_usage_tracker.go - similar to UsageTracker but for platforms
type PlatformUsageTracker struct {
    mu sync.RWMutex
    platformUsage map[string]*PlatformProfile
}

// TrackMessageSent(platformID)
// TrackTokensUsed(platformID, tokens)
// CanMakeRequest(platformID, estimatedMessages, estimatedTokens)
```

**Step 4: Run test to verify pass**

Run: `go test ./internal/runtime -run TestPlatformUsageTracker_TrackMessages -v`
Expected: PASS

**Step 5: Update connectors.json schema**

```json
{
  "id": "gemini-web",
  "type": "web",
  "notes": "Courier destination. 1M tokens/day free tier.",
  "limit_schema": {
    "messages_per_3h": 10,
    "messages_per_session": 40,
    "tokens_per_day": 1000000
  }
}
```

**Step 5: Commit**

```bash
git add governor/internal/runtime/platform_usage_tracker.go \
       governor/internal/runtime/courier.go \
       governor/config/connectors.json
git commit -m "feat: add web platform limit tracking for courier destinations"
```

---

## Task 5: Implement courier dual-envelope tracking

**Objective:** When dispatching a courier task, check BOTH envelopes before allowing execution:
- Envelope A: fueling model on its connector (already works)
- Envelope B: web destination platform (new from Task 4)
Only dispatch if BOTH have headroom.

**Files:**
- Modify: `/home/vibes/VibePilot/governor/internal/runtime/courier.go:1-100` (check platform limits before dispatch)
- Modify: `/home/vibes/VibePilot/governor/internal/runtime/router.go` (update selectDestination to check platform limits)
- Modify: `/home/vibes/VibePilot/governor/cmd/governor/handlers_task.go` (update task completion to record platform usage)

**Step 1: Write failing test for dual-envelope check**

```go
func TestCourier_DualEnvelopeCheck(t *testing.T) {
    // Setup: fueling model has headroom, web platform at limit
    // Call CourierDispatcher.Dispatch
    // Assert: dispatch fails (platform limit hit)
    //
    // Setup: fueling model at limit, web platform has headroom
    // Call CourierDispatcher.Dispatch
    // Assert: dispatch fails (model limit hit)
    //
    // Setup: both have headroom
    // Call CourierDispatcher.Dispatch
    // Assert: dispatch succeeds
}
```

**Step 2: Run test to verify failure**

Run: `go test ./internal/runtime -run TestCourier_DualEnvelopeCheck -v`
Expected: FAIL

**Step 3: Write minimal implementation**

```go
// In courier.go Dispatch method:
// 1. Check fueling model limits via usageTracker.CanMakeRequestVia(fuelingModel, fuelingConnector, estimatedTokens)
// 2. Check web platform limits via platformTracker.CanMakeRequest(webPlatformID, estimatedMessages, estimatedTokens)
// 3. Only proceed if BOTH return CanProceed=true

// In task completion:
// 1. Record fueling model usage (existing)
// 2. Record web platform usage: platformTracker.TrackMessageSent(platformID) and TrackTokensUsed(platformID, outputTokens)
```

**Step 4: Run test to verify pass**

Run: `go test ./internal/runtime -run TestCourier_DualEnvelopeCheck -v`
Expected: PASS

**Step 5: Commit**

```bash
git add governor/internal/runtime/courier.go \
       governor/internal/runtime/router.go \
       governor/cmd/governor/handlers_task.go
git commit -m "feat: implement courier dual-envelope rate limiting (model + platform)"
```

---

## Task 6: Add persistence for connector and platform usage

**Objective:** Ensure connector usage and platform usage data survives governor restarts by persisting to Supabase and loading on startup.

**Files:**
- Modify: `/home/vibes/VibePilot/governor/internal/runtime/connector_usage_tracker.go` (add LoadFromDatabase/PersistToDatabase)
- Modify: `/home/vibes/VibePilot/governor/internal/runtime/platform_usage_tracker.go` (add LoadFromDatabase/PersistToDatabase)
- Modify: `/home/vibes/VibePilot/governor/cmd/governor/main.go` (call load/persist for both trackers)

**Step 1: Write failing test for connector persistence**

```go
func TestConnectorUsageTracker_Persistence(t *testing.T) {
    // Set usage on connector
    // Call PersistToDatabase (mock)
    // Create new tracker
    // Call LoadFromDatabase (mock)
    // Assert usage is restored
}
```

**Step 2: Run test to verify failure**

Run: `go test ./internal/runtime -run TestConnectorUsageTracker_Persistence -v`
Expected: FAIL

**Step 3: Write minimal implementation**

```go
// Add to both trackers:
// LoadFromDatabase(ctx db.DB) error
// PersistToDatabase(ctx context.Context) error
// Store usage in Supabase (new tables or JSONB in existing tables)
//```

**Step 4: Run test to verify pass**

Run: `go test ./internal/runtime -run TestConnectorUsageTracker_Persistence -v`
Expected: PASS

**Step 5: Commit**

```bash
git add governor/internal/runtime/connector_usage_tracker.go \
       governor/internal/runtime/platform_usage_tracker.go \
       governor/cmd/governor/main.go
git commit -m "feat: add persistent storage for connector and platform usage"
```

---

## Task 7: Update router to use connector-aware limits

**Objective:** Ensure the router uses connector-specific rate limits when making routing decisions, falling back to model-level limits when connector-specific limits don't exist.

**Files:**
- Modify: `/home/vibes/VibePilot/governor/internal/runtime/router.go:100-200` (update selectByCascade to use connector-aware CanMakeRequestVia)
- Modify: `/home/vibes/VibePilot/governor/internal/runtime/router.go:200-300` (update getModelScoreForTask to consider connector availability)

**Step 1: Write failing test for connector-aware routing**

```go
func TestRouter_ConnectorAwareSelection(t *testing.T) {
    // Setup: model available on connector A (limit reached) and connector B (available)
    // Call router.Select
    // Assert: selects connector B route
    //
    // Setup: model only available on connector A (limit reached)
    // Call router.Select
    // Assert: returns error or falls back to next model in cascade
}
```

**Step 2: Run test to verify failure**

Run: `go test ./internal/runtime -run TestRouter_ConnectorAwareSelection -v`
Expected: FAIL

**Step 3: Write minimal implementation**

```go
// In selectByCascade:
// For each model in cascade:
//   For each connector in model.AccessVia (in preference order):
//     if usageTracker.CanMakeRequestVia(modelID, connectorID, estimatedTokens) {
//         return this model+connector pair
//     }
// Continue to next model in cascade
```

**Step 4: Run test to verify pass**

Run: `go test ./internal/runtime -run TestRouter_ConnectorAwareSelection -v`
Expected: PASS

**Step 5: Commit**

```bash
git add governor/internal/runtime/router.go
git commit -m "feat: make router connector-aware for rate limit decisions"
```

---

## Task 8: Add startup validation for routing.json cascade

**Objective:** On governor startup, validate that every model ID in routing.json free_cascade exists in the models table (or models.json) and log warnings for missing entries.

**Files:**
- Create: `/home/vibes/VibePilot/governor/internal/startup_validator.go` (new file)
- Modify: `/home/vibes/VibePilot/governor/cmd/governor/main.go:1-50` (call validator on startup)

**Step 1: Write failing test for cascade validation**

```go
func TestStartupValidator_ValidateCascade(t *testing.T) {
    // Setup: routing.json has model "x" not in models.json
    // Call Validator.ValidateCascade
    // Assert: logs warning about missing model "x"
    //
    // Setup: all cascade models exist in models.json
    // Call Validator.ValidateCascade
    // Assert: no warnings
}
```

**Step 2: Run test to verify failure**

Run: `go test ./internal/startup_validator -TestStartupValidator_ValidateCascade -v`
Expected: FAIL

**Step 3: Write minimal implementation**

```go
// startup_validator.go
type StartupValidator struct {
    db DB
    config Config
}

func NewStartupValidator(db DB, config Config) *StartupValidator {
    return &StartupValidator{db: db, config: config}
}

func (v *StartupValidator) ValidateCascade(ctx context.Context) error {
    // Read routing.json
    // Get free_cascade.priority list
    // Query models table for all active model IDs
    // For each ID in cascade:
    //   if not in models table AND not in models.json {
    //       log.Warn("cascade model not found: %s", id)
    //   }
    return nil
}
```

**Step 4: Run test to verify pass**

Run: `go test ./internal/startup_validator -TestStartupValidator_ValidateCascade -v`
Expected: PASS

**Step 5: Commit**

```bash
git add governor/internal/startup_validator.go \
       governor/cmd/governor/main.go
git commit -m "feat: add startup validation for routing.json cascade consistency"
```

---

## Task 9: Update docker-compose and deployment docs

**Objective:** Ensure the new persistence fields (if we add new tables) are documented and that anyone deploying knows to run migrations.

**Files:**
- Create: `/home/vibes/VibePilot/migrations/126_add_connector_platform_usage_tables.sql` (if needed)
- Modify: `/home/vibes/VibePilot/docs/deployment.md` (add note about persistence)
- Modify: `/home/vibes/VibePilot/governor/config/global_manifest.json` (update if needed)

**Step 1: Determine if new tables are needed**

Based on our analysis, we can persist connector and platform usage as JSONB in existing tables:
- Connector usage: add to `models` table or create `model_connector_usage` table
- Platform usage: add to `platforms` table (already has usage fields)

Let's check if we need new tables or can extend existing ones.

**Step 2: Check current platforms table for usage fields**

From our schema check, platforms table has:
- `actual_courier_cost_per_task`, `daily_limit`, `daily_used`, `request_limit`, `request_used`, `tokens_used`, `total_tasks`, `tasks_completed`, `tasks_failed`, `theoretical_api_cost_per_1k_tokens`, `theoretical_cost_input_per_1k_usd`, `theoretical_cost_output_per_1k_usd`, `usage_reset_at`

We can extend this with:
- `message_windows` JSONB (similar to usage_windows)
- `tokens_per_day_used` integer
- `messages_per_3h_used` integer
- `messages_per_session_used` integer

**Step 3: Write migration if needed**

```sql
-- Only if we need to add columns
ALTER TABLE platforms 
ADD COLUMN IF NOT EXISTS message_windows jsonb,
ADD COLUMN IF NOT EXISTS tokens_per_day_used integer DEFAULT 0,
ADD COLUMN IF NOT EXISTS messages_per_3h_used integer DEFAULT 0,
ADD COLUMN IF NOT EXISTS messages_per_session_used integer DEFAULT 0;
```

**Step 4: Update deployment docs**

```markdown
## Persistence Note
The governor now persists usage data for:
- Model rate limit windows (existing)
- Model learned data (existing) 
- Connector-level usage (new)
- Web platform limits (new)
This data survives restarts and is loaded automatically on startup.
No manual migration required if using existing JSONB columns.
```

**Step 5: Commit**

```bash
git add docs/deployment.md \
       governor/config/global_manifest.json
git commit -m "docs: update deployment notes for persistence features"
```

---

## Task 10: End-to-end test of the complete system

**Objective:** Verify that all components work together: persistence, shared limits, web platform tracking, courier dual-envelope, and dashboard integration.

**Files:**
- Create: `/tmp/test_orchestrator_e2e.go` (test script)
- Modify: No production files (verification only)

**Step 1: Write E2E test script**

```go
func TestOrchestratorE2E(t *testing.T) {
    // 1. Start governor with test config
    // 2. Submit a courier task
    // 3. Verify:
    //    - Fueling model usage tracked
    //    - Web platform usage tracked
    //    - Both envelopes respected
    //    - Data persisted to Supabase
    //    - On restart, state is restored
    //    - Dashboard would show correct status
    // 4. Exhaust limits on both envelopes
    // 5. Verify tasks queue or route to alternatives
    // 6. Clean up
}
```

**Step 2: Run test manually**

Run: `go run /tmp/test_orchestrator_e2e.go`
Expected: All assertions pass, no errors

**Step 3: Verify with actual API tests**

Test a few key scenarios manually:
- Courier task to gemini-web (should track messages)
- Multiple Groq model usage (should hit shared org limit)
- Restart governor (should restore state)
- Dashboard integration (check that new fields don't break rendering)

**Step 4: Commit test script (optional)**

```bash
git add /tmp/test_orchestrator_e2e.go
git commit -m "test: add E2E test for orchestrator enhancements (remove before prod)"
```

# PLAN COMPLETE

**Verification Checklist:**
- [ ] All tasks are bite-sized (2-5 minutes each)
- [ ] Each task includes exact file paths
- [ ] Each task includes complete code examples
- [ ] Each task includes exact commands with expected output
- [ ] Each task includes verification steps
- [ ] Plan follows DRY, YAGNI, TDD principles
- [ ] Plan respects existing contracts and schemas
- [ ] Plan maintains backward compatibility
- [ ] Plan addresses all requirements from discussion

**Next Step:** 
Execute this plan using subagent-driven-development — I'll dispatch a fresh subagent per task with two-stage review (spec compliance then code quality). Shall I proceed?