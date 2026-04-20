# Courier Agent Implementation Plan

**Date:** 2026-04-20
**Status:** Ready for Implementation
**Depends on:** router.go (dual-envelope), courier.go (dispatch interface), connectors.json (7 web destinations), models.json, prompts/courier.md

---

## Architecture Reminder

The courier is a browser-operated clipboard operator. The pipeline before it handles all reasoning:

```
Planner → Supervisor → Orchestrator → Router → Courier
                                         ↓
                              Dual-envelope check:
                              A) Fueling model available? (vision + tool calling, not in cooldown)
                              B) Web platform available? (under 80% free-tier limits)
                              BOTH OK → dispatch courier
                              Either FAIL → fall back to internal routing
```

The courier does NOT think, plan, or reason. It navigates, pastes, waits, copies.

---

## Change 1: RoutingResult Must Carry Destination Info

**File:** `governor/internal/runtime/router.go`

**Problem:** `tryWebRouting()` selects a destination via `selectDestination()` and validates it with dual-envelope checks. But the returned `RoutingResult` only carries the fueling model's connector and model ID. The destination PlatformID and URL are discarded.

**Fix:** Add `PlatformID` and `PlatformURL` to `RoutingResult`:

```go
type RoutingResult struct {
    ConnectorID string
    ModelID     string
    RoutingFlag string
    Category    string
    IsFallback  bool
    PlatformID  string  // NEW: web destination ID (e.g., "deepseek-web")
    PlatformURL string  // NEW: web destination URL (e.g., "https://chat.deepseek.com")
}
```

In `tryWebRouting()`, populate these from `dest`:

```go
return &RoutingResult{
    ConnectorID: courierConn,
    ModelID:     courierModel,
    RoutingFlag: "web",
    Category:    "external",
    PlatformID:  dest.PlatformID,
    PlatformURL: dest.URL,
}
```

No other changes to the router. Dual-envelope logic stays as-is.

---

## Change 2: Remove Hardcoded RoutingFlag

**File:** `governor/cmd/governor/handlers_task.go`

**Problem:** Line 160 hardcodes `RoutingFlag: "internal"` on every task. The router's `SelectRouting()` immediately returns `selectInternal()` without checking courier availability.

**Fix:** The task handler should NOT set RoutingFlag. The router decides. Pass an empty string:

```go
routingResult, routeErr = h.connRouter.SelectRouting(ctx, runtime.RoutingRequest{
    Role:          "task_runner",
    TaskType:      taskCategory,
    RoutingFlag:   "",  // let router decide: internal, web, etc.
    ExcludeModels: failedModels,
})
```

The router's existing logic in `SelectRouting()`:
1. If `RoutingFlag == "internal"` → internal only (current hardcoded behavior)
2. If `RoutingFlag == ""` → check courier availability first, fall back to internal
3. If `RoutingFlag == "web"` → web only

The router already handles case 2. We just need to stop forcing case 1.

---

## Change 3: Task Handler Branches for Web Routing

**File:** `governor/cmd/governor/handlers_task.go`

**Problem:** `executeTask()` always creates a session via the session factory. When routing is "web", this fails because the factory has no "web" connector type registered.

**Fix:** After routing, check `routingResult.RoutingFlag`:

```go
// After claim, before execution setup:

if routingResult.RoutingFlag == "web" {
    h.executeViaCourier(ctx, task, taskPacket, routingResult, ...)
    return
}

// Existing internal execution continues below...
```

The `executeViaCourier` method:

1. Gets the fueling model's API key from vault (via `h.cfg.GetConnector(connectorID)` → `api_key_ref` → vault lookup)
2. Builds the courier packet with all required fields
3. Calls `CourierRunner.Run()` locally
4. On success, follows the SAME post-execution flow as internal:
   - `commitOutput()` to the task branch
   - `create_task_run` with tokens, costs
   - `transition_task` to "review"
   - `recordSuccess()`

The task branch, worktree, commit, review flow is IDENTICAL for courier and internal. Only the execution step differs.

---

## Change 4: CourierRunner Runs Locally (Not GitHub Actions)

**File:** `governor/internal/connectors/courier.go`

**Problem:** Current `dispatch()` sends a `repository_dispatch` to GitHub Actions. This was a workaround for GCE's 4GB RAM. On x220 with 16GB RAM and 12GB available, we can run browser-use locally.

**Fix:** Replace the GitHub dispatch with a local Python process invocation:

```go
func (r *CourierRunner) dispatch(ctx context.Context, ...) error {
    // Build the JSON payload (same as before)
    payload := map[string]interface{}{...}
    payloadJSON, _ := json.Marshal(payload)

    // Run local courier script
    cmd := exec.CommandContext(ctx, "python3",
        "/home/vibes/VibePilot/scripts/courier_run.py",
        "--payload", string(payloadJSON),
    )
    output, err := cmd.Output()
    // Parse JSON result from stdout
}
```

Keep the existing `pollCompletion()` for the Supabase-based approach as a fallback, but the local script returns results directly via stdout, so polling is unnecessary.

**GitHub Actions courier.yml** remains in the repo but is not the primary path. It can be re-enabled if we ever move to a RAM-constrained environment again.

---

## Change 5: Local Courier Script (Python)

**New file:** `scripts/courier_run.py`

This is the bridge between Go and browser-use. It receives a JSON payload and returns a JSON result.

**What it does:**
1. Parses `--payload` JSON with: task_id, prompt, web_platform_url, llm_provider, llm_model, llm_api_key
2. Instanciates browser-use with the fueling model (Gemini via langchain-google-genai, or OpenRouter via langchain-openai)
3. Navigates to `web_platform_url`
4. Pastes the prompt
5. Waits for response completion
6. Extracts the output text
7. Grabs `window.location.href` for chat_url
8. Returns JSON to stdout: `{output, chat_url, tokens_in, tokens_out}`

**Provider mapping (config-driven):**
```
provider="google"    → langchain-google-genai (ChatGoogleGenerativeAI)
provider="openrouter" → langchain-openai (ChatOpenAI, base_url=openrouter)
provider="groq"       → langchain-openai (ChatOpenAI, base_url=groq)
```

The provider comes from `connectors.json` → `provider` field for the fueling model's connector. No hardcoding.

---

## Change 6: Fix Capabilities + Add Courier Suitability to models.json

**File:** `governor/config/models.json`

### 6a. Fix Missing Capability Markers

Many models have empty or incomplete capabilities. This is critical because the router uses capabilities to decide routing. Current state:

**Needs fixing (vision + tool_calling exist but not marked):**
| Model ID | Has Vision? | Has Tool Calling? | Currently Marked |
|----------|-------------|-------------------|-----------------|
| meta-llama/llama-4-scout-17b-16e-instruct | YES (multimodal) | YES | `['code', 'reasoning']` — WRONG |
| google/gemma-4-31b-it | YES | YES | `[]` — WRONG |
| nvidia/nemotron-3-super-120b | NO | YES | `[]` — WRONG |
| z-ai/glm-4.5-air | NO | YES | `[]` — WRONG |
| minimax/minimax-m2.5 | NO | YES | `[]` — WRONG |
| nvidia/nemotron-3-super-120b-a12b | NO | YES | `[]` — WRONG |

**Corrected capabilities for existing models:**
```
meta-llama/llama-4-scout:  ['code', 'reasoning', 'vision', 'multimodal', 'tool_calling']
google/gemma-4-31b-it:     ['code', 'reasoning', 'vision', 'multimodal', 'tool_calling']
nvidia/nemotron-3-super-120b:      ['code', 'reasoning', 'tool_calling']
z-ai/glm-4.5-air:          ['code', 'reasoning', 'tool_calling']
minimax/minimax-m2.5:      ['code', 'reasoning', 'tool_calling']
nvidia/nemotron-3-super-120b-a12b: ['code', 'reasoning', 'tool_calling']
```

Also fix `gemini-2.5-flash` — has vision but missing `tool_calling`:
```
gemini-2.5-flash: ['code', 'reasoning', 'vision', 'multimodal', 'tool_calling']
```

### 6b. Set courier: true on Vision+Tool-Calling Models

Models suitable for courier fueling need `courier: true`. The router's `selectCourierModel()` filters for this flag + vision capability.

**Courier-eligible models (vision + tool_calling + free/cheap API access):**

| Model | Connector | Vision | Tool Calling | Cost | Courier Priority |
|-------|-----------|--------|--------------|------|-----------------|
| gemini-2.5-flash | gemini-api | YES | YES | FREE | 1 (best) |
| meta-llama/llama-4-scout-17b-16e-instruct | groq-api | YES | YES | FREE | 2 |
| google/gemma-4-31b-it | openrouter-api | YES | YES | FREE | 3 |
| google/gemma-4-26b-a4b-it | openrouter-api | YES | YES | FREE | 4 |
| nvidia/nemotron-nano-12b-v2-vl | openrouter-api | YES | YES | FREE | 5 |
| google/gemma-3-27b-it | openrouter-api | YES | YES | FREE | 6 |

**Cheap paid backups (courier: true, lower priority):**
| Model | Connector | Cost/1M tokens | Speciality |
|-------|-----------|---------------|------------|
| bytedance/ui-tars-1.5-7b | openrouter-api | $0.10/$0.20 | GUI automation specialist |

### 6c. Add Missing Models to models.json

These vision+tool-calling models are available on our free API connectors but not yet in models.json:

1. **google/gemma-4-26b-a4b-it:free** — OpenRouter free, 262K ctx, MoE variant
2. **nvidia/nemotron-nano-12b-v2-vl:free** — OpenRouter free, 128K ctx, video+image
3. **google/gemma-3-27b-it:free** — OpenRouter free, 131K ctx, previous gen
4. **bytedance/ui-tars-1.5-7b** — OpenRouter cheap, GUI specialist

Each new entry follows the existing model schema with:
- `courier: true`
- `capabilities: ["vision", "multimodal", "tool_calling"]`
- `access_via: ["openrouter-api"]`
- `api_key_ref: "OPENROUTER_API_KEY"`
- Appropriate rate limits (20 RPM per model, 200 RPD unfunded)
- `strengths` and `weaknesses` populated

### 6d. Update strengths/weaknesses for Courier Context

The orchestrator uses `strengths` and `weaknesses` to advise on routing. Add courier-relevant entries:

```
gemini-2.5-flash:
  strengths: ['long_context', 'multimodal', 'vision', 'fast', 'courier_fuel']
  weaknesses: ['strict_rate_limits']

meta-llama/llama-4-scout:
  strengths: ['multimodal', 'vision', 'multilingual', 'courier_fuel']
  weaknesses: ['large_memory_footprint']

google/gemma-4-31b-it:
  strengths: ['vision', 'balanced', 'large_context', 'courier_fuel']
  weaknesses: ['openrouter_rate_limits', 'quality_varies']

bytedance/ui-tars-1.5-7b:
  strengths: ['gui_automation', 'courier_fuel', 'ultra_cheap']
  weaknesses: ['small_model', 'less_reasoning']
```

---

## Change 7: TaskHandler Gets CourierRunner

**File:** `governor/cmd/governor/handlers_task.go`, `types.go` or wherever TaskHandler is created

Add `courierRunner *connectors.CourierRunner` to the TaskHandler struct.

**Instantiation** (wherever TaskHandler is created currently):
```go
courierRunner := connectors.NewCourierRunner("", "", database, 300)
// githubToken/githubRepo empty = local mode (no GitHub dispatch)
```

The CourierRunner needs the DB interface (for task_runs) and timeout. It does NOT need GitHub credentials for local mode.

---

## What Does NOT Change

- **router.go dual-envelope logic** — works correctly, just needs to return destination info
- **connectors.json web destinations** — 7 destinations already configured with limit schemas
- **platform_tracker.go** — usage tracking + persistence already working
- **courier.md prompt** — already defines the courier's role correctly
- **dashboard** — already has courier_model_id, courier_tokens, courier_cost_usd fields
- **post-execution flow** — commit, task_runs, review, testing, merge all identical
- **GitHub Actions courier.yml** — kept as fallback, not deleted

---

## Implementation Order

```
Step 1: Fix model capabilities + add courier markers to models.json  (config)
Step 2: Add missing vision models to models.json                      (config)
Step 3: Add PlatformID/PlatformURL to RoutingResult                   (router.go)
Step 4: Remove hardcoded RoutingFlag: "internal"                      (handlers_task.go)
Step 5: Add courierRunner to TaskHandler struct                        (handlers_task.go)
Step 6: Add web routing branch in executeTask                         (handlers_task.go)
Step 7: Install browser-use + playwright                               (infrastructure)
Step 8: Write courier_run.py                                           (scripts/)
Step 9: Modify CourierRunner.dispatch for local execution              (courier.go)
Step 10: Rebuild, test with chat.deepseek.com (no login)              (E2E)
```

Steps 1-2 are config only. Steps 3-6 are Go code changes. Steps 7-9 are infrastructure. Step 10 is verification.

---

## RAM Budget on x220

```
Total:              16 GB
Governor:           ~12 MB
Chrome + Playwright: ~500 MB
browser-use Python:  ~100 MB
OS + other:         ~3 GB
─────────────────────────────
Available:          ~12 GB free
```

One courier at a time uses ~600MB. Easily fits. Could run 2-3 in parallel if needed.

---

## Test Plan

1. Rebuild governor: `cd ~/VibePilot/governor && go build -o governor ./cmd/governor`
2. Restart: `systemctl --user restart vibepilot-governor`
3. Push a simple PRD (standalone task, no codebase dependency)
4. Watch logs: `journalctl --user -u vibepilot-governor -f`
5. Verify: router selects "web" routing, courier launches, output returns
6. Verify: dashboard shows courier_model_id, tokens, cost, routing_flag
7. First test with chat.deepseek.com (no login required, simplest platform)
