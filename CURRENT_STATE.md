# VibePilot Current State
# AUTO-UPDATED: 2026-04-19 08:24 UTC
# RULE: Update this file after ANY change set. Resume from here, never from guesses.

## Three Sources of Truth

1. **GitHub (code):** https://github.com/VibesTribe/VibePilot — pushed=real
2. **Supabase (data):** https://qtpdzsinvifkgpxyxlaz.supabase.co — in DB=real
3. **Dashboard (live):** https://vibeflow-dashboard.vercel.app/ — rendering=working

## Active Models (12) — All backed by verified API keys

### Groq (3) — Free tier, rate limited
- llama-3.3-70b-versatile (96K context)
- llama-3.1-8b-instant (96K context)
- qwen3-32b (96K context)

### OpenRouter Free (5) — Account at -$0.57, free models work
- qwen/qwen3.6-plus:free (197K context)
- qwen/qwen3-coder:free (197K context)
- nvidia/nemotron-3-super-120b:free (197K context)
- google/gemma-4-31b-it:free (197K context)
- z-ai/glm-4.5-air:free (98K context)

### NVIDIA NIM (1) — Free tier
- nvidia/llama-3.1-nemotron-ultra-253b-v1 (96K context)

### Gemini API (2) — Key verified working (50 models available)
- gemini-2.5-flash (750K context)
- gemini-api (750K context)

### Z.AI (1) — $30/mo subscription, ends May 1
- glm-5 (152K context) — hermes interactive only

## Paused Models (5)
- deepseek-chat — out of credit (💰 Credit Needed on dashboard)
- deepseek-reasoner — out of credit (💰 Credit Needed on dashboard)
- gemini-2.0-flash — DEPRECATED by Google June 1 2026

## Benched Models (14) — All visible as ⚠ Issue on dashboard
- qwen3.5-flash, qwen3.5-plus — No API key (Alibaba)
- minimax-m2.7 — No API key
- deepseek-web, gemini-web — browser automation not built
- huggingchat — browser automation not built
- claude-sonnet, chatgpt-4o-mini — no API key
- opencode — CLI tool, not a model
- kimi-k2-instruct — bans after 5hrs autonomous use
- And others (duplicates, cancelled subs)

## Deleted This Session
- copilot (model + platform) — never had access, caused 16 duplicates
- nemotron-ultra-253b — duplicate of nvidia/ prefixed version
- qwen3.6-plus (alibaba web) — same model exists via OpenRouter

## Dashboard Status Model (cleaned up this session)
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

## Q/W/M Agent Tier System
- **Q** = Internal (direct API, no courier needed) — courier=false
- **W** = Web courier dispatched to platform — courier=true (when automation built)
- **M** = MCP connection (future)
- Platforms are DESTINATIONS, not models. Removed from dashboard rendering.

## Key Architecture Rules
- Models table = models we can route tasks to
- Platforms table = destinations where couriers go
- Connectors = how we reach the model (API keys, endpoints)
- One model can be reached via multiple connectors
- Pipeline picks best model FIRST, then picks best available connector
- Benched ≠ invisible. Everything shows with truth about why.

## ROI Status
- $0.25 saved across 45 task_runs
- ROI bug fixed: SQL keys aligned with Go struct (theoretical_cost_usd not theoretical)
- All runs backfilled with correct costs

## GLM-5 Subscription
- $30.00/mo, 11 days left (expires May 1)
- 483 tasks completed, $0.05/task
- NOT renewing at $90/3mo

## API Keys in Vault (11)
ZAI_API_KEY, GROQ_API_KEY, OPENROUTER_API_KEY, GEMINI_API_KEY (verified working),
DEEPSEEK_API_KEY (out of credit), NVIDIA_API_KEY, GITHUB_TOKEN, SUPABASE_SERVICE_KEY,
VIBEPILOT_GMAIL_EMAIL, VIBEPILOT_GMAIL_PASSWORD, webhook_secret

## Files Changed This Session
- ~/VibePilot/governor/config/models.json — synced with DB
- ~/vibeeflow/apps/dashboard/lib/vibepilotAdapter.ts — platform rendering removed, all statuses visible
- ~/vibeeflow/apps/dashboard/utils/icons.ts — NVIDIA icon added
- ~/vibeeflow/apps/dashboard/assets/agents/nvidia.svg — new icon asset

## Contract Registry & Global Manifest
- ~/VibePilot/governor/config/contract_registry.json — 5 dashboard tables, 47 Go→SQL RPCs
- ~/VibePilot/governor/config/global_manifest.json — 9 slices, 3 data flows, 4 shared-state joints
