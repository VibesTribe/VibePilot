# Governor Fix Plan

**Purpose:** Prioritized list of fixes to align Go governor with VibePilot architecture.

**Principle:** Every fix serves VibePilot's design. Go is plumbing. Supabase is the contract. Dashboard shows what state IS.

---

## Priority 1: Stop the Bleeding (unblocks all other work)

### P1.1 Fix merge target to main
**Problem:** Tasks merge to `TEST_MODULES/general` instead of `main`. Merged code is lost. Subsequent tasks fail `go test` because tests on main reference code that was never merged to main.
**Fix:** Find merge target logic in task completion handler. Change it to merge to `main`.
**Files:** `handlers_task.go` (merge section)
**Verify:** Run a task through to completion. Check that code appears on `main` branch.

### P1.2 Fix PRD path in webhook
**Problem:** Webhook watches `docs/prd/` but PRDs live in `docs/prds/`.
**Fix:** Change `isPRD()` in `github.go` line 98 to use `"docs/prds/"`.
**Files:** `internal/webhooks/github.go`
**Verify:** Push a PRD to `docs/prds/` on a test branch, merge to main, confirm webhook detects it (once webhook URL is fixed).

### P1.3 Fix webhook URL (dead GCE IP)
**Problem:** GitHub webhook points to `34.45.124.117:8080` which was GCE. Machine is now x220.
**Fix:** Set up cloudflared tunnel route for `/webhooks` on `vibestribe.rocks`, update GitHub webhook URL.
**Files:** cloudflared config, GitHub repo settings
**Verify:** Push PRD, see plan created automatically in Supabase.

---

## Priority 2: Enable Smart Routing

### P2.1 Remove agent model pins
**Problem:** `agents.json` hardcodes `model: "glm-5"` for planner, supervisor, tester, task_runner, maintenance. This bypasses cascade routing entirely.
**Fix:** Remove `"model"` field from agents that should use cascade selection. Keep model field only for agents with genuine fixed requirements (e.g., consultant must use gemini for web access).
**Files:** `config/agents.json`
**Risk:** Low. Cascade falls back to glm-5 anyway if it's the only active connector. But now it goes through proper rate limit checks first.
**Verify:** Start governor, trigger a task, check logs for `[Router] Cascade routing:` instead of `[Router] Agent X configured with model glm-5`.

### P2.2 Create missing Supabase RPCs
**Problem:** `get_model_score_for_task` and `check_platform_availability` don't exist.
**Fix:** Write migration `NNN_model_scoring.sql`:
- `get_model_score_for_task(p_model_id, p_task_type, p_task_category)` -- reads model strengths/weaknesses, success_rate, learned.best_for_task_types, returns score 0-1
- `check_platform_availability(p_platform_id)` -- checks if web destination has hit rate limits or cooldown
**Files:** New migration in `docs/supabase-schema/`
**Verify:** Call RPCs via curl, confirm non-error responses.

### P2.3 Wire ExportForDashboard to an endpoint
**Problem:** `UsageTracker.ExportForDashboard()` exists but nothing calls it.
**Fix:** Add a `/api/models/status` endpoint to webhook server that calls `ExportForDashboard()`. Dashboard can poll this. Or write to a Supabase `model_status` table on a timer.
**Files:** `internal/webhooks/server.go`, possibly new endpoint handler
**Verify:** `curl vibestribe.rocks/api/models/status` returns model health data.

---

## Priority 3: Accurate Metrics

### P3.1 Token counting from hermes output
**Problem:** Hermes CLI doesn't output token counts in parseable format. `RecordUsage()` gets called with zeros or estimates.
**Fix:** Parse hermes output for token usage. If hermes can't provide it, estimate from input/output character counts (rough: 1 token ≈ 4 chars). Better than zero.
**Files:** `internal/connectors/runners.go` (hermes output parsing)
**Alternative:** Hermes config could be changed to output JSON with token data.

### P3.2 ROI calculator
**Problem:** Pricing data exists in models.json and Supabase but nothing multiplies tokens_used by cost_per_token.
**Fix:** Create a `CalculateROI()` function in UsageTracker or a new `internal/metrics/` package:
- Read `api_cost_per_1k_tokens` or `cost_input/output_per_1k_usd` from models table
- Multiply by recorded token usage
- Store result in Supabase `metrics` table or existing field
- Dashboard reads and displays
**Files:** New file `internal/metrics/roi.go`, migration if new table needed

### P3.3 Dashboard telemetry sync
**Problem:** Dashboard shows Tokens: 0, ROI: $0 even though data exists in Supabase.
**Fix:** Once P3.1 and P3.2 are done, verify dashboard adapter reads the right fields. May need to add fields to the adapter's transform logic.
**Files:** Dashboard code (vibeflow repo, not VibePilot) -- but ONLY if Go side is providing correct data.

---

## Priority 4: Courier / Browser Use

### P4.1 Browser automation for courier agents
**Problem:** Go can't drive a browser. Courier concept needs Playwright/Python.
**Fix:** Two approaches:
  (a) Go dispatches to a Python script that runs Browser Use / Playwright against web platforms
  (b) Go dispatches to GitHub Actions workflow that has browser automation
**Current state:** `courier.go` has dispatch/poll lifecycle. It dispatches to GitHub Actions. The Actions workflow needs to be created.
**Files:** New GitHub Actions workflow, possibly `scripts/courier_browser.py`

### P4.2 Free web tier cascade
**Problem:** 10 web platforms defined in connectors.json are unreachable without browser automation.
**Impact:** VibePilot can only use CLI/API models (currently just hermes/glm-5). Free web tiers (Gemini 1M/day, DeepSeek generous free, Qwen no login) are wasted.
**Depends on:** P4.1

---

## Priority 5: Pipeline Robustness

### P5.1 Planner JSON parsing resilience
**Problem:** Planner sometimes returns malformed JSON (two objects concatenated, bad escaping).
**Fix:** Make `ParsePlannerOutput` more resilient -- try extracting multiple JSON objects, handle trailing content, strip more aggressively.
**Files:** `internal/runtime/decision.go` `extractJSON` function

### P5.2 Worktree latency
**Problem:** Worktree checkout takes 90+ seconds. Old branch approach was faster.
**Investigation needed:** Is this git performance on USB-tethered connection, or is worktree creation doing unnecessary work?

### P5.3 Server_test.go compilation error
**Problem:** `internal/webhooks/server_test.go` references code from a completed task that was merged to `TEST_MODULES/general` instead of `main`.
**Fix:** Once P1.1 is fixed and that task's code is on main, this resolves automatically.
**Temporary:** Delete or fix the test to not reference non-existent code.

---

## Execution Order

1. P1.1 (merge target) -- unblocks everything
2. P1.2 + P1.3 (webhook) -- enables PRD-driven pipeline
3. P2.1 (remove model pins) -- enables cascade routing
4. P5.3 (server_test fix) -- unblocks testing stage
5. P2.2 (missing RPCs) -- enables smart scoring
6. P2.3 (dashboard endpoint) -- enables model health visibility
7. P3.1 + P3.2 (token counting + ROI) -- enables accurate metrics
8. P3.3 (dashboard telemetry) -- shows metrics
9. P4.1 + P4.2 (courier) -- enables free web tiers
10. P5.1 + P5.2 (robustness) -- polish

Each step is independently testable. Each step makes the next one possible.
