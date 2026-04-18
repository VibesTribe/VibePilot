# Governor State of Alignment

**Purpose:** Single source of truth for what Go code exists, what it does, and whether it aligns with VibePilot architecture. Never repeat this audit again.

**Last Updated:** 2026-04-18 (post Fix 1-3)

---

## What EXISTS and WORKS

### Config System
- `models.json` -- 15 models defined with rate limits, pricing, capabilities, recovery config, learned data (+ nemotron-ultra-253b)
- `connectors.json` -- 18 destinations (3 CLI, 5 API including groq-api active + nvidia-api new, 10 web)
- `routing.json` -- Strategies, agent restrictions, selection criteria, fallback config, `free_cascade` strategy with model priority order
- `agents.json` -- 11 agents, NO model pins (empty model field = cascade routing)
- All loaded at startup by `internal/runtime/config.go` and `internal/runtime/model_loader.go`

### Router (`internal/runtime/router.go` -- 513 lines)
- `SelectRouting()` -- Main entry point, checks routing flag, courier availability, web vs internal
- `selectInternal()` -- Agent model field empty → falls through to cascade selection
- `selectByCascade()` -- Looks for `free_cascade` strategy first, iterates model priority list, checks UsageTracker, picks first available, falls back to shortest cooldown
- `selectCourierModel()` -- Picks best active model with vision/browser capability
- `tryWebRouting()` -- Routes to courier + web platform for external tasks
- `getModelScore()` -- Calls Supabase RPC `get_model_score_for_task` (RPC DOES NOT EXIST)
- `selectDestination()` -- Picks active web destination from connectors
- `isDestinationAvailable()` -- Calls Supabase RPC `check_platform_availability` (RPC DOES NOT EXIST)

### Cascade Routing (FIXED this session)
- `free_cascade` strategy in routing.json defines priority: glm-5, llama-3.3-70b, kimi-k2, qwen3-32b, nemotron-ultra, llama-3.1-8b, deepseek-chat, gemini-2.5-flash, deepseek-reasoner
- All agents.json model pins removed -- cascade routing now runs for all agents
- groq-api connector activated (was `pending_key`, GROQ_API_KEY in vault)
- nvidia-api connector added (NVIDIA_API_KEY in vault)
- `runners.go` handles groq/nvidia/openrouter providers via `callOpenAICompatible()`

### Usage Tracker (`internal/runtime/usage_tracker.go` -- 450 lines)
- `RegisterModel()` -- Loads model profiles at startup
- `CanMakeRequest()` -- Checks cooldown, minute/hour/day/token limits, spacing requirements
- `RecordUsage()` -- Tracks tokens in minute/hour/day/week windows -- **IS CALLED** from handlers_task.go
- `RecordCompletion()` -- Tracks success/failure, updates learned data -- **IS CALLED** from handlers_task.go
- `RecordRateLimit()` -- Sets cooldown expiry, increments rate limit count -- **IS CALLED** from handlers_task.go
- `ExportForDashboard()` -- Serializes all model status to JSON -- **NEVER CALLED** from any handler or endpoint

### Model Loader (`internal/runtime/model_loader.go`)
- Reads models.json at startup
- Registers all models with UsageTracker
- Syncs model profiles to Supabase `models` table

### Supabase `models` Table (verified)
- Has columns: id, status, name, vendor, context_limit, strengths, weaknesses, config (JSONB)
- Tracks: tokens_used (1,166,695 for glm-5), tasks_completed (483), tasks_failed, success_rate
- Tracks: usage_windows (minute/hour/day/week), learned data, rate_limit_count, cooldown_expires_at
- **Pricing populated**: cost_input_per_1k_usd and cost_output_per_1k_usd set for all 11 active models
- 15 models synced from models.json

### Cost Calculation (FIXED this session)
- `handlers_task.go:319` calls `calculateCosts()` for every task run
- `calc_run_costs` Supabase RPC reads pricing from models table, returns theoretical/actual/savings
- Go writes `platform_theoretical_cost_usd`, `total_actual_cost_usd`, `total_savings_usd` to task_runs
- Dashboard ROI calculator reads these fields from task_runs and displays real savings

### Task Handlers
- `handlers_task.go` (853 lines) -- Task available event, routing, worktree creation, hermes execution, recording
- `handlers_testing.go` -- Runs `go test` in worktree's `governor/` subdirectory, merges to TEST_MODULES/<slice>, then checks if all module tasks done and merges to `testing` branch
- `handlers_plan.go` -- Planner creates plans from PRDs, parses JSON output
- `handlers_maint.go` -- Maintenance agent for code changes
- `handlers_council.go` -- Council review for complex plans
- `handlers_research.go` -- Researcher agent

### Module-to-Testing Merge (FIXED this session)
- After a task merges to `TEST_MODULES/<slice>`, `tryMergeModuleToTesting()` checks sibling tasks
- If all tasks with same slice_id + plan_id are in terminal state → merges module branch to `testing`
- `testing` branch exists on origin as merge target

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

## Fixes Applied This Session (2026-04-18)

### Fix 1: Cascade Routing
- `runners.go`: Added groq/nvidia/openrouter provider handling (all OpenAI-compatible)
- `connectors.json`: Activated groq-api, added nvidia-api connector
- `models.json`: Added nemotron-ultra-253b with nvidia access_via
- `routing.json`: Added `free_cascade` strategy with 9 models in priority order
- `agents.json`: Removed all model pins so cascade routing runs for all agents

### Fix 2: Module-to-Testing Merge
- `handlers_testing.go`: Added `tryMergeModuleToTesting()` method
- After task merge to `TEST_MODULES/<slice>`, checks if all sibling tasks are done
- If all complete → merges module branch to `testing` branch
- Created and pushed `testing` branch to origin

### Fix 3: Cost Calculation
- Supabase `models` table: Updated cost_input/output_per_1k_usd for all 11 models with real pricing
- Migration 122: Created `calc_run_costs` RPC (renamed from `calculate_run_costs` to avoid stuck overloads)
- RPC reads pricing from models table, returns theoretical/actual/savings as JSONB
- Go code already wired at handlers_task.go:319, just needed the RPC to work

---

## What DOES NOT WORK and WHY

### 1. Missing Supabase RPCs
**What:** `get_model_score_for_task` and `check_platform_availability`
**Impact:** Router falls back to default score (0.5) for all models. Platform availability always returns true. Model scoring is blind.

### 2. ExportForDashboard Never Called
**What:** `UsageTracker.ExportForDashboard()` exists but nothing calls it
**Impact:** Dashboard has no way to get live model status, usage windows, cooldown info from the governor

### 3. Token Counting for Hermes
**What:** Hermes CLI doesn't output token counts. The hermes runner in `connectors/runners.go` can't parse what isn't there.
**Impact:** Token counts for glm-5 runs may be estimates or zeros. Other runners (Gemini, OpenAI-compatible) DO parse tokens correctly.

### 4. Webhook Points to Dead GCE IP
**What:** GitHub webhook URL is `http://34.45.124.117:8080/webhooks` -- the old GCE instance
**Impact:** Pushing PRDs to GitHub does NOT auto-create plans. Manual Supabase INSERT required.
**Fix:** Set up cloudflared tunnel for x220

### 5. PRD Path Mismatch
**What:** Webhook watches `docs/prd/` (singular). Actual PRD directory is `docs/prds/` (plural).
**Impact:** Even if webhook was reachable, it wouldn't detect any PRDs.
**Fix:** Change line 98 of `github.go` to use `"docs/prds/"` or make it configurable

### 6. gemini-api and deepseek-api Inactive
**What:** Connectors exist in connectors.json but marked inactive. API keys exist in vault.
**Impact:** Models gemini-2.5-flash and deepseek-chat/reasoner can't be reached via API.
**Fix:** Activate connectors in connectors.json, verify API keys work

### 7. Courier / Browser Use Not Integrated
**What:** `courier.go` dispatches to GitHub Actions. No Playwright/Browser Use integration exists in Go.
**Impact:** Web platforms (chatgpt-web, gemini-web, etc.) in connectors.json are defined but unreachable.

### 8. Module-to-Main Merge
**What:** Module-to-testing merge works. But testing-to-main merge doesn't exist yet.
**Impact:** Code accumulates on `testing` branch but never reaches `main`.
**Note:** User wants to decide the final merge flow (testing folder approach TBD).

---

## Previously Fixed Issues (confirmed resolved)

- ~~Agent Model Overrides Short-Circuit Cascade~~ → Removed model pins from agents.json
- ~~No ROI / Cost Calculator~~ → `calc_run_costs` RPC wired, pricing populated
- ~~Merge Goes to Wrong Branch~~ → Module-to-testing merge implemented, testing branch created
