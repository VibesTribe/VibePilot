# Courier Agent Fueling Strategy

**Date:** 2026-04-19  
**Status:** Research Complete, Ready for Review  
**Depends on:** models.json, connectors.json, router.go (courier model selection), courier.go (dispatch), courier.yml (GitHub Actions), dashboard (ROI tracking)

---

## What Courier Agents Do

Couriers are browser-operated clipboard operators. They do NOT think, plan, or reason. The pipeline before them handles all of that:

```
Planner → Supervisor/Council → Orchestrator → Courier
                                    ↓
                            "Take this prompt to 
                             chat.deepseek.com,
                             paste it, wait, 
                             bring back output + URL"
```

The courier's fueling model (the LLM powering browser-use) only needs to:
1. See the page (vision) -- find the text input, buttons, status indicators
2. Act (tool calling) -- click, type, paste, scroll
3. Know when it's done -- detect response completed, grab output + chat URL

This is not complex reasoning. It's "find the chat box, paste this, hit send, wait, copy what comes back." Even weak vision models can handle this pattern at 95%+ success.

---

## Token Consumption Per Courier Task

browser-use works in agent loops: screenshot → model decides action → execute → screenshot → repeat.

| Phase | Steps | Tokens/Step | Subtotal |
|-------|-------|-------------|----------|
| Navigate to platform | 5-10 | ~3,000 | 15,000-30,000 |
| Find input, click, paste | 2-3 | ~2,500 | 5,000-7,500 |
| Wait for response (poll) | 2-3 | ~2,000 | 4,000-6,000 |
| Copy output, grab URL | 1-2 | ~2,000 | 2,000-4,000 |
| **Total** | **~15-20** | | **~30,000-50,000** |

Midpoint estimate: **50,000 tokens per courier task** (fueling model only).

The actual task prompt + response happens on the destination platform's web UI -- that's free. The fueling model only pays for browser navigation overhead.

---

## The Hard Constraint: Vision + Tool Calling

browser-use requires BOTH capabilities. Without vision the agent is blind. Without tool calling it can't act.

| Provider | Vision | Tool Calling | Free? |
|----------|--------|--------------|-------|
| **Gemini (Google API)** | YES | YES | YES (15 RPM) |
| **OpenRouter free tier** | varies | varies | YES (some) |
| **Groq** | llama-4-scout only | YES | YES |
| **NVIDIA NIM** | NO | YES | YES |
| **SiliconFlow** | GLM-4.1V-9B (free) | YES | YES (limited) |
| **Puter** | browser JS only | N/A | User-pays, not suitable |

Only Gemini and select OpenRouter models give us both vision AND tool calling for free.

---

## Fueling Model Inventory (What We Can Actually Use)

### TIER 1: Direct Free API Keys (Fastest, No Middleman)

| Model | Provider | Context | Cost | Limit | Notes |
|-------|----------|---------|------|-------|-------|
| gemini-2.5-flash | Google API | 1M | FREE | 15 RPM / 1500 RPD | Best option. Period. |
| gemini-2.0-flash | Google API | 1M | FREE | 15 RPM / 1500 RPD | Good backup. Deprecation timeline unclear. |

**Multi-project strategy:** Each Google Cloud project gets independent free-tier quota. 4 projects = 60 RPM, 6000 RPD, 4M TPM -- all free, all isolated. Courier spike won't kill researcher.

Recommended project split:
- `vibepilot-couriers` -- courier agent vision/navigation
- `vibepilot-researcher` -- daily system researcher
- `vibepilot-visual-tester` -- QA screenshot analysis
- `vibepilot-backup` -- spare capacity

### TIER 2: OpenRouter Free Vision Models

| Model | Context | Cost | Notes |
|-------|---------|------|-------|
| google/gemma-4-31b-it:free | 262K | FREE | Newest Gemma, solid vision |
| google/gemma-4-26b-a4b-it:free | 262K | FREE | MoE variant, efficient |
| nvidia/nemotron-nano-12b-v2-vl:free | 128K | FREE | Video+image, small |
| google/gemma-3-27b-it:free | 131K | FREE | Previous gen, still decent |
| openrouter/free | 200K | FREE | Router model, quality varies |

**Risk:** OpenRouter free models rotate. What's free today may not be tomorrow. Our account is at -$0.57 (overdraft). Free models may stop working if they enforce the balance.

### TIER 3: Ultra-Cheap Fallbacks (< $4/month at 20 tasks/day)

| Model | Provider | In/Out per 1M | Monthly Cost | Notes |
|-------|----------|---------------|-------------|-------|
| bytedance/ui-tars-1.5-7b | OpenRouter | $0.10/$0.20 | $3.90 | Purpose-built for GUI automation! |
| meta-llama/llama-4-scout | OpenRouter | $0.08/$0.30 | $4.38 | Multimodal, 327K ctx |
| google/gemini-2.5-flash-lite | OpenRouter | $0.10/$0.40 | $5.70 | Flash variant via OR |
| qwen/qwen3-vl-32b-instruct | OpenRouter | $0.10/$0.42 | $5.93 | Vision + reasoning |
| google/gemma-4-26b-a4b-it | OpenRouter | $0.08/$0.35 | $4.83 | Non-free variant |

### TIER 4: SiliconFlow (Direct, Cheaper Than OpenRouter for Same Models)

| Model | Cost | Notes |
|-------|------|-------|
| THUDM/GLM-4.1V-9B-Thinking | FREE | Vision + thinking, 9B. Free on SF. |
| Qwen/Qwen2.5-VL-32B-Instruct | ¥1.89/1M (~$0.26) | Cheaper than OR for same model |
| Qwen/Qwen3-VL-8B-Instruct | ¥0.50/2.00 (~$0.07/$0.28) | Ultra cheap small VL model |

SiliconFlow is 46-66% cheaper than OpenRouter for the same open-source models because they host directly. Worth setting up an account for courier fuel specifically. $1 minimum top-up vs OpenRouter's $5.

---

## Recommended Routing Cascade

The router's `selectCourierModel()` already filters for `vision` or `browser` capability and picks the highest-scored active model. We configure the cascade in models.json:

```
Courier Fuel Cascade (by priority):
1. gemini-2.5-flash (courier-project key)     -- FREE, 1M ctx, best quality
2. gemini-2.5-flash (main-project key)         -- FREE, same model, separate quota
3. google/gemma-4-31b-it:free (OpenRouter)     -- FREE, 262K ctx
4. google/gemma-4-26b-a4b-it:free (OpenRouter) -- FREE, 262K ctx, MoE
5. bytedance/ui-tars-1.5-7b (OpenRouter)       -- $3.90/mo, GUI specialist
6. nvidia/nemotron-nano-12b-v2-vl:free (OR)    -- FREE, 128K ctx
```

All entries in models.json with `capabilities: ["vision", ...]` and `courier: true`. No hardcoding. System researcher can add/remove/reorder as landscape shifts.

---

## What Needs To Change In Our Code

### 1. GitHub Actions Workflow (courier.yml) -- OVERHAUL NEEDED

**Current problems:**
- Hardcodes `gemini-2.0-flash` via `langchain_google_genai`
- Doesn't read `llm_provider`/`llm_model` from dispatch payload
- Doesn't navigate to `web_platform_url`
- Returns `str(result)` with no chat_url extraction
- Uses `len(output) // 4` for token counting

**Fix:** The workflow must:
- Read `llm_provider`, `llm_model`, `llm_api_key`, `web_platform_url` from `client_payload`
- Use langchain's provider-agnostic interface (langchain-openai for OpenRouter-compatible, langchain-google-genai for Gemini)
- Navigate to `web_platform_url` before starting the task
- Extract chat_url from browser address bar after response
- Report tokens using tiktoken or langchain's built-in counting

### 2. models.json -- Add Courier Fuel Models

Add entries for each fueling model with `courier: true` and `capabilities: ["vision", "multimodal"]`:
- google/gemma-4-31b-it (already exists but needs `courier: true`, `vision` cap)
- google/gemma-4-26b-a4b-it (needs new entry)
- nvidia/nemotron-nano-12b-v2-vl (needs new entry)
- bytedance/ui-tars-1.5-7b (needs new entry)
- Additional Gemini API keys as separate "virtual" models per project

### 3. connectors.json -- Add SiliconFlow Connector

New connector entry for SiliconFlow (OpenAI-compatible API):
```json
{
  "id": "siliconflow-api",
  "name": "SiliconFlow API",
  "type": "api",
  "status": "active",
  "base_url": "https://api.siliconflow.cn/v1",
  "api_key_ref": "SILICONFLOW_API_KEY",
  "auth_method": "bearer"
}
```

### 4. Courier Prompt (courier.md) -- Minor Updates

- Clarify that courier receives `web_platform_url` and must navigate there
- Add instruction to capture chat_url from address bar
- Note that fueling model is decided by orchestrator, not courier

### 5. Multi-Key Support in .env / config

For multi-project Gemini keys:
```
GEMINI_API_KEY=courier-project-key        # primary (courier project)
GEMINI_API_KEY_2=researcher-project-key   # researcher project  
GEMINI_API_KEY_3=visual-tester-key        # visual tester project
GEMINI_API_KEY_4=backup-key               # spare
```

models.json entries reference different `api_key_ref` values per project.

---

## What Does NOT Need To Change

- **router.go** -- `selectCourierModel()` already filters by vision capability and dual-envelope checks. Works correctly.
- **courier.go** -- `Run()` already reads `browser_llm_provider`, `browser_llm_model`, `browser_llm_api_key`, `web_platform_url` from the task packet and passes them in the GitHub dispatch. Payload is correct.
- **Dashboard** -- Already has `courier_model_id`, `courier_tokens`, `courier_cost_usd` fields. Already groups runs by executor vs courier role. ROI calculator already handles courier costs.
- **Architecture principles** -- Config-driven, no hardcoding, swappable. Already aligned.

---

## Implementation Order

1. **Create additional Google Cloud projects** for separate Gemini API keys (manual, user action)
2. **Add courier fuel models to models.json** with `courier: true` and `vision` capability
3. **Add SiliconFlow connector** to connectors.json (when account is set up)
4. **Overhaul courier.yml** to read provider/model/URL from dispatch payload
5. **Test end-to-end** with gemini-2.5-flash fueling a task on chat.deepseek.com
6. **Wire up OpenRouter free vision models** as fallbacks
7. **System researcher** monitors model availability, updates cascade as things change

---

## Future-Proofing Rules

1. **No hardcoded models** -- Everything reads from models.json / connectors.json
2. **System researcher** flags deprecated models, suggests replacements
3. **Dashboard** shows courier fuel model health (rate limits, success rate, cost)
4. **Easy swap** -- changing a fueling model = one config edit, zero code changes
5. **Free models expire** -- OpenRouter free tier rotates. Plan for it. Cascade has paid fallbacks.
6. **New platforms** -- adding a web destination = one connectors.json entry + one browser-use navigation template
7. **Token counting is ours** -- we count prompt tokens in, output tokens out. Never trust external counts.
