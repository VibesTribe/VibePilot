# Governor State of Alignment

**Purpose:** Single source of truth for what Go code exists, what it does, and whether it aligns with VibePilot architecture. Never repeat this audit again.

**Last Updated:** 2025-04-18

---

## What EXISTS and WORKS

### Config System
- `models.json` -- 15 models defined with rate limits, pricing, capabilities, recovery config, learned data
- `connectors.json` -- 17 destinations (3 CLI, 4 API, 10 web)
- `routing.json` -- Strategies, agent restrictions, selection criteria, fallback config
- `agents.json` -- 11 agents with model assignments, capabilities, connector references
- All loaded at startup by `internal/runtime/config.go` and `internal/runtime/model_loader.go`

### Router (`internal/runtime/router.go` -- 513 lines)
- `SelectRouting()` -- Main entry point, checks routing flag, courier availability, web vs internal
- `selectInternal()` -- Uses agent model override OR cascade selection
- `selectByCascade()` -- Iterates model priority, checks UsageTracker, picks first available, falls back to shortest cooldown
- `selectCourierModel()` -- Picks best active model with vision/browser capability
- `tryWebRouting()` -- Routes to courier + web platform for external tasks
- `getModelScore()` -- Calls Supabase RPC `get_model_score_for_task` (RPC DOES NOT EXIST)
- `selectDestination()` -- Picks active web destination from connectors
- `isDestinationAvailable()` -- Calls Supabase RPC `check_platform_availability` (RPC DOES NOT EXIST)

### Usage Tracker (`internal/runtime/usage_tracker.go` -- 450 lines)
- `RegisterModel()` -- Loads model profiles at startup
- `CanMakeRequest()` -- Checks cooldown, minute/hour/day/token limits, spacing requirements
- `RecordUsage()` -- Tracks tokens in minute/hour/day/week windows -- **IS CALLED** from handlers_task.go:282
- `RecordCompletion()` -- Tracks success/failure, updates learned data (avg duration, best/avoid task types) -- **IS CALLED** from handlers_task.go:260 and :350
- `RecordRateLimit()` -- Sets cooldown expiry, increments rate limit count -- **IS CALLED** from handlers_task.go:258
- `ExportForDashboard()` -- Serializes all model status to JSON -- **NEVER CALLED** from any handler or endpoint

### Model Loader (`internal/runtime/model_loader.go`)
- Reads models.json at startup
- Registers all models with UsageTracker
- Syncs model profiles to Supabase `models` table

### Supabase `models` Table (verified)
- Has columns: id, status, name, vendor, context_limit, strengths, weaknesses, config (JSONB)
- Tracks: tokens_used (1,166,695 for glm-5), tasks_completed (483), tasks_failed (1,381), success_rate (0.26)
- Tracks: usage_windows (minute/hour/day/week), learned data, rate_limit_count, cooldown_expires_at
- Has: rate_limits (JSONB), api pricing, subscription cost fields
- 15 models synced from models.json

### Task Handlers
- `handlers_task.go` (853 lines) -- Task available event, routing, worktree creation, hermes execution, recording
- `handlers_testing.go` -- Runs `go test` in worktree's `governor/` subdirectory (FIXED this session)
- `handlers_plan.go` -- Planner creates plans from PRDs, parses JSON output
- `handlers_maint.go` -- Maintenance agent for code changes
- `handlers_council.go` -- Council review for complex plans
- `handlers_research.go` -- Researcher agent

### Webhook Server (`internal/webhooks/github.go`)
- Detects PRD files pushed to `docs/prd/` (singular)
- Auto-creates plans via `createPlanForPRD()` Supabase RPC
- PRD path check: `strings.HasPrefix(file, "docs/prd/")` (MISMATCH: actual PRDs at `docs/prds/`)

### Courier Runner (`internal/connectors/courier.go`)
- Dispatches tasks to GitHub Actions workflow
- Polls for completion via Supabase task_runs
- Full dispatch/poll/fail lifecycle exists
- Requires GitHub Actions workflow to be set up (NOT YET DONE)

### Other Packages
- `internal/gitree/` -- Worktree management (exists, wired)
- `internal/db/` -- Supabase RPC layer
- `internal/realtime/` -- Supabase realtime subscription
- `internal/runtime/` -- Session factory, context builder, parallel execution, events
- `internal/mcp/` -- MCP server for governor
- `internal/vault/` -- Encrypted credential management
- `internal/security/` -- Leak detector
- `internal/memory/` -- Memory compaction
- `internal/dag/` -- YAML DAG pipeline engine
- `internal/tools/` -- Tool registry (db, file, git, sandbox, vault, web tools)

---

## What DOES NOT WORK and WHY

### 1. Missing Supabase RPCs
**What:** `get_model_score_for_task` and `check_platform_availability`
**Impact:** Router falls back to default score (0.5) for all models. Platform availability always returns true. Model scoring is blind.
**Why:** These RPCs were designed but never created in Supabase migrations.

### 2. Agent Model Overrides Short-Circuit Cascade
**What:** `agents.json` has `"model": "glm-5"` for planner, supervisor, tester, task_runner, maintenance
**Impact:** `selectInternal()` finds the agent model override first and routes directly there. The cascade logic in `selectByCascade()` never runs for these agents.
**Why:** agents.json was set up with fixed models before cascade routing was implemented. Nobody removed the pins.
**Fix:** Remove `model` field from agents that should use cascade, or make it optional with empty string meaning "use cascade."

### 3. ExportForDashboard Never Called
**What:** `UsageTracker.ExportForDashboard()` exists but nothing calls it
**Impact:** Dashboard has no way to get live model status, usage windows, cooldown info from the governor
**Why:** No endpoint or event handler was wired to expose this data

### 4. No ROI / Cost Calculator
**What:** models.json has pricing data (`input_per_1m_usd`, `output_per_1m_usd`). Supabase has `api_cost_per_1k_tokens`, `subscription_cost`. No code aggregates these.
**Impact:** Dashboard shows ROI $0, Tokens 0. No cost tracking happens.
**Why:** Nobody wrote the calculator that reads pricing data and multiplies by recorded token usage

### 5. Token Counting Inaccurate
**What:** `RecordUsage()` is called but hermes doesn't report exact token counts. The `tokensIn` and `tokensOut` values passed are estimates (likely 0 or rough guesses).
**Impact:** Supabase shows `tokens_used: 1,166,695` but this may not be accurate. Dashboard shows Tokens 0.
**Why:** Hermes CLI doesn't output token usage in a parseable format. The runner (`connectors/runners.go`) would need to parse token data from hermes output.

### 6. Webhook Points to Dead GCE IP
**What:** GitHub webhook URL is `http://34.45.124.117:8080/webhooks` -- the old GCE instance
**Impact:** Pushing PRDs to GitHub does NOT auto-create plans. Manual Supabase INSERT required.
**Fix:** Either (a) set up cloudflared tunnel for webhook, or (b) use an alternative trigger mechanism

### 7. PRD Path Mismatch
**What:** Webhook watches `docs/prd/` (singular). Actual PRD directory is `docs/prds/` (plural).
**Impact:** Even if webhook was reachable, it wouldn't detect any PRDs.
**Fix:** Change line 98 of `github.go` to use `"docs/prds/"` or make it configurable

### 8. Merge Goes to Wrong Branch
**What:** Completed tasks merge to `TEST_MODULES/general` instead of `main`
**Impact:** Merged code sits on a dead branch. `go test ./...` on main fails because tests reference code that was never merged to main. This causes EVERY subsequent task to fail testing.
**Fix:** Merge target should be `main`, not a test modules branch

### 9. Courier / Browser Use Not Integrated
**What:** `courier.go` dispatches to GitHub Actions. No Playwright/Browser Use integration exists in Go.
**Impact:** Web platforms (chatgpt-web, gemini-web, etc.) in connectors.json are defined but unreachable. Free web tier models cannot be used.
**Why:** Courier concept requires browser automation which needs Python/Playwright, not Go. The Go code has the dispatch mechanism but no browser automation.

### 10. Dashboard Telemetry Not Connected
**What:** Dashboard adapter reads from Supabase. Governor writes to Supabase. But the specific fields the dashboard needs (token counts, ROI, active model status) are either not populated or not in the format the dashboard expects.
**Impact:** Dashboard shows correct task counts and statuses for tasks that flow through, but Tokens: 0, ROI: $0, no model health info.
