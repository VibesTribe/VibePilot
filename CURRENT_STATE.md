# VibePilot Current State
# AUTO-UPDATED: 2026-04-19 20:30 UTC
# RULE: Update this file after ANY change set. Resume from here, never from guesses.

## Three Sources of Truth

1. **GitHub (code):** https://github.com/VibesTribe/VibePilot — pushed=real
2. **Supabase (data):** https://qtpdzsinvifkgpxyxlaz.supabase.co — in DB=real
3. **Dashboard (live):** https://vibeflow-dashboard.vercel.app/ — rendering=working

## Active Models (33 total, 26 API + 7 web courier)

### Groq API (7) — Free tier, org-level 100K TPD shared (now tracked)
- llama-3.3-70b-versatile, llama-3.1-8b-instant, qwen3-32b
- meta-llama/llama-4-scout-17b-16e-instruct, openai/gpt-oss-120b
- groq/compound, groq/compound-mini

### OpenRouter Free (6) — Account at -$0.57, free models only
- google/gemma-4-31b-it, z-ai/glm-4.5-air, minimax/minimax-m2.5
- nvidia/nemotron-3-super-120b, nvidia/nemotron-3-super-120b-a12b, openai/gpt-oss-120b

### NVIDIA NIM (3) — Free tier
- meta/llama-3.3-70b-instruct, moonshotai/kimi-k2-instruct, nvidia/llama-3.1-nemotron-ultra-253b-v1

### Gemini API (1) — Key verified working
- gemini-2.5-flash

### Hermes/CLI (1) — Z.AI subscription, ends May 1
- glm-5 (hermes interactive only)

### Web Courier Destinations (untested, need browser automation)
- gemini-web: gemini-2.5-pro
- deepseek-web: deepseek-r1, deepseek-v3 (also on notegpt-web)
- qwen-web: qwen-2.5, qwen-3
- mistral-web: mistral-large, codestral, pixtral
- notegpt-web: deepseek-r1, deepseek-v3

## Paused Models (2)
- deepseek-chat — out of credit
- deepseek-reasoner — out of credit

## Benched Models (5)
- chatgpt-4o-mini — Web-only via browser use (ChatGPT free tier)
- claude-sonnet — Web-only via browser use (Anthropic free tier)
- gemini-web — Web-only via browser use (Gemini free tier)
- kimi-k2-instruct — Benched from Groq, use NVIDIA version instead
- minimax-m2.7 — No API access, only available via OpenRouter as m2.5

## Orchestrator Enhancements (this session)

### New: Persistent Usage Tracking
- Usage windows, cooldowns, and learned data now survive governor restarts
- LoadFromDatabase on startup, PersistToDatabase every 30s + on shutdown

### New: Connector-Level Shared Limits
- ConnectorUsageTracker aggregates usage across models on same connector
- Groq org 100K TPD now proactively tracked (not just reactive after 429)
- shared_limits field added to connectors.json for connector-level caps

### New: Web Platform Limit Tracking
- PlatformUsageTracker with structured limit_schema per web destination
- 7 web destinations configured with messages/3h/8h/day/session + tokens/day limits
- 80% buffer threshold same as model-level tracking

### New: Courier Dual-Envelope Routing
- Router checks BOTH fueling model limits AND web platform limits before dispatching courier
- Falls back to internal routing if either envelope has no headroom

### New: Startup Cascade Validation
- Validates all model IDs in routing.json cascade exist in models.json
- Logs errors for dead entries before they cause silent routing failures

### New: Connector/Platform Persistence
- connector_usage table (migration 126) for connector state
- platforms.usage_windows column for web destination state
- Both loaded on startup, persisted every 30s

## Dashboard Status Model
- ✓ Ready — active, idle
- ↻ Active — active, working on tasks
- ⏳ Cooldown — paused with cooldown_expires_at timer
- 💰 Credit Needed — paused + status_reason contains "credit"
- ⚠ Issue — everything else non-active (benched, deprecated, no key)
- **NOTHING is hidden** — benched models visible with reason

## Three Review Triggers (escalate to human)
1. Visual UI/UX approval after changes
2. System researcher suggestions after council review
3. API credit exhaustion (status=paused + reason contains "credit")

## Key Architecture Rules
- Models table = models we can route tasks to
- Platforms table = destinations where couriers go
- Connectors = how we reach the model (API keys, endpoints)
- One model can be reached via multiple connectors
- Pipeline picks best model FIRST, then picks best available connector
- Benched ≠ invisible. Everything shows with truth about why.
- Token counting always on OUR side (prompt packets + outputs), never trust external counts

## Files Changed This Session
- governor/config/models.json — removed 5 broken models, synced with DB
- governor/config/connectors.json — removed zai-api/copilot-web, added shared_limits + limit_schema
- governor/config/routing.json — removed dead cascade entries
- governor/internal/runtime/usage_tracker.go — LoadFromDatabase, connector/platform delegation
- governor/internal/runtime/connector_tracker.go — NEW: shared connector limits
- governor/internal/runtime/platform_tracker.go — NEW: web platform limits
- governor/internal/runtime/router.go — dual-envelope courier, connector-aware internal routing
- governor/internal/runtime/model_loader.go — loads connector/platform limits from config
- governor/internal/runtime/config.go — PlatformLimitSchema, SharedLimits structs
- governor/cmd/governor/startup_validate.go — cascade model ID validation
- governor/cmd/governor/main.go — LoadFromDatabase on startup
- migrations/126_connector_and_platform_usage_persistence.sql — NEW

## Contract Registry & Global Manifest
- governor/config/contract_registry.json — 5 dashboard tables, 49 Go→SQL RPCs (2 new)
- governor/config/global_manifest.json — 9 slices, 3 data flows, 4 shared-state joints
