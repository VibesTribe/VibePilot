# VibePilot Current State
**Updated: April 18, 2026 16:20**

## System Status: OPERATIONAL (with known issues)

### What Works
- GitHub webhook delivery (webhooks.vibestribe.rocks permanent URL)
- Governor realtime subscriptions and event processing
- Full pipeline chain: Plan → Tasks → Execute → Review → Test → Merge
- Vault encryption/decryption (all 6 API keys working in governor)
- Task completion tracking (testing passed = task complete, merge best-effort)
- Named tunnel "vibes" for dashboard/chat (sacred, never modify)
- Model orchestrator: 25 models synced to Supabase, usage persistence every 30s
- Migration 124 applied: check_platform_availability, get_model_score_for_task, update_model_usage RPCs

### Pipeline End-to-End (Verified Apr 18)
1. Webhook push → plan created → tasks decomposed
2. Executor (glm-5/hermes) writes code in worktree
3. Supervisor reviews and approves
4. Testing handler discovers worktree, runs go build + targeted tests
5. Task marked complete with model tracking data
6. Best-effort merge to module branch
7. Dashboard tracks live via realtime subscriptions

### Active Issues (Apr 18)

**1. GLM-5 rate limiting**
GLM-5 hits 429 "Insufficient balance" intermittently. Not permanent — comes back.
When it fails, Hermes falls through to nemotron-ultra (nvidia) which works.

**2. Cascade retry bug (CRITICAL)**
When primary model fails, Hermes retries 3 times on same model BEFORE falling through.
Then when it DOES fall through, Groq and NVIDIA return 401 (Invalid API Key).
This means the encrypted API keys in secrets_vault may not decrypt correctly
when used OUTSIDE the governor (i.e. by Hermes CLI directly).
Governor vault decryption works fine. Hermes CLI may use different key path.

**3. API key auth failures on Groq + NVIDIA via Hermes**
Groq returns 401 "Invalid API Key", NVIDIA returns 401 "Authentication failed".
These work from governor (proven by E2E pipeline) but fail from Hermes CLI fallback.
Root cause likely: Hermes CLI uses OpenRouter-style config, not governor's vault.

### Three-Tier Architecture (Model ≠ Connector ≠ Platform)
- **Model** = who does the thinking (deepseek-chat, llama-3.3-70b, glm-5)
- **Connector** = how to reach it (API key, CLI, browser session)
- **Platform** = where to send it (deepseek-api, groq-api, chatgpt-web)
- One model can use MULTIPLE connectors with different pricing/credit/limits
- Fisherman analogy: Model=fisherman, Connector=gear/credit, Platform=river
- NEVER conflate these. Routing picks best model THEN best connector.

### Models (25 in Supabase)
Active: glm-5, opencode, deepseek-chat, deepseek-reasoner, gemini-2.0-flash,
        gemini-2.5-flash, gemini-api, gemini-web, copilot, deepseek-web,
        llama-3.3-70b-versatile, llama-3.1-8b-instant, qwen3-32b,
        kimi-k2-instruct (via groq-api, NOT kimi subscription), nemotron-ultra-253b
Benched: claude-haiku-4-5, claude-sonnet-4-5, claude-sonnet, gpt-4o, gpt-4o-mini,
         chatgpt-4o-mini, gemini-flash, kimi-cli, kimi-internal, kimi-k2.5
Cancelled subscription: kimi-k2 (kimi platform, does NOT affect kimi-k2-instruct via groq)

### Connectors (connectors.json)
Active: hermes (CLI), gemini-api, groq-api, nvidia-api
Inactive: opencode, claude-code, kimi, deepseek-api (out of credit), openrouter-api (emergency)
Web: chatgpt-web, claude-web, gemini-web, deepseek-web, qwen-web, mistral-web, notegpt-web

### Migrations Applied
1-122: Historical (see Supabase schema docs)
123: create_plan status fix (draft not pending)
124a: Drop old check_platform_availability function
124b: Create check_platform_availability, get_model_score_for_task, update_model_usage RPCs

### Outstanding Fixes (Priority Order)
1. Fix cascade retry bug — fall through on failure, not retry same model 3 times
2. Improve planner output parsing — handle GLM-5 malformed JSON with backticks
3. Testing-to-main merge (step 3) — testing folder approach
4. Wire credit tracking — record cost per task_run, deduct from credit_remaining_usd
5. Fix Groq/NVIDIA 401s from Hermes CLI (separate from governor vault)

### Machine
- ThinkPad X220, Debian Linux
- USB tether to phone for Supabase connectivity
- Go 1.24.3 at /home/vibes/go/bin/go
- systemd user service: vibepilot-governor
- No direct PostgreSQL access (IPv6). Migrations via Supabase SQL Editor.
- Git hooks require: `git -c core.hooksPath=/dev/null`

### Budget Timeline
- GLM-5 subscription: ends **May 1, 2026** (~12 days)
- Free tiers: Groq, NVIDIA, Gemini (rate limited but functional)
- Must prove pipeline with real projects before subscription ends
