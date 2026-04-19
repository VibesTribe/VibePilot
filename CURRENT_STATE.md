# VibePilot Current State
# AUTO-UPDATED: 2026-04-19 06:22 UTC
# RULE: Update this file after ANY change set. Resume from here, never from guesses.

## Three Sources of Truth

1. **GitHub (code):** https://github.com/VibesTribe/VibePilot (main branch)
2. **Supabase (data):** https://qtpdzsinvifkgpxyxlaz.supabase.co
3. **Dashboard (rendering):** https://vibeflow-dashboard.vercel.app/ (LIVE)
   - NOTE: https://vibestribe.github.io/vibeflow/ is OLD mock prototype, NOT live

## GitHub State

Branch: main
Dirty files: M .context/knowledge.db

Recent commits:
  3067bfc5 contract registry: three sources of truth (GitHub+Supabase+Dashboard)
  2f709ec7 connectors: OpenRouter updated with credit status and current free models
  abdc8117 global manifest: complete slice map of VibePilot plumbing
  c1d7e8fb contract registry: maps dashboard expectations + Go RPC signatures + SQL contracts
  948ffa6b chore: auto-update .context knowledge layer (from 6ce7d1ce)

## Supabase State

### Task Status Distribution
Total: 11 tasks
  available: 2
  blocked: 1
  merged: 8

### Active Models (24)
  gemini-api                                    google          api
  opencode                                      zhipu           cli_subscription
  gemini-2.5-flash                              gemini-api      api
  kimi-k2-instruct                              groq-api        api
  qwen3.6-plus                                  alibaba         web
  qwen/qwen3-coder                              openrouter      api
  google/gemma-4-31b-it                         openrouter      api
  deepseek-web                                  deepseek-web    web
  nemotron-ultra-253b                           nvidia-api      api
  gemini-web                                    gemini-web      web
  glm-5                                         hermes          cli_subscription
  qwen3.5-flash                                 alibaba         api
  qwen3.5-plus                                  alibaba         api
  deepseek-reasoner                             deepseek-api    api
  nvidia/llama-3.1-nemotron-ultra-253b-v1       nvidia-api      api
  minimax-m2.7                                  minimax         api
  qwen/qwen3.6-plus                             openrouter      api
  nvidia/nemotron-3-super-120b                  openrouter      api
  z-ai/glm-4.5-air                              openrouter      api
  qwen3-32b                                     groq-api        api
  llama-3.3-70b-versatile                       groq-api        api
  llama-3.1-8b-instant                          groq-api        api
  copilot                                       copilot-web     web
  deepseek-chat                                 deepseek-api    api

### Task Run History (last 50)
  hermes: 34 runs
  groq-api: 11 runs

## Dashboard Rendering (verified 2026-04-19 06:22 UTC)
- URL: https://vibeflow-dashboard.vercel.app/
- Shows: 8/11 complete, 3 pending, 73% progress, 129K tokens
- Models tab: All active models render with logos, context limits, status
- Known issues: ROI shows $0.000000 (calc_run_costs may not be aggregating)

## Governor Config
- Config files: 11 JSON files in governor/config/
- Contract registry: governor/config/contract_registry.json
- Global manifest: governor/config/global_manifest.json
- RPC registry: governor/config/rpc_contract_registry.json
- Models: governor/config/models.json (synced with DB)
- Connectors: governor/config/connectors.json

## Infrastructure
- Dashboard code: ~/vibeflow/apps/dashboard/ (Vite + React)
- Dashboard deployed: Vercel (vibeflow-dashboard)
- Governor code: ~/VibePilot/governor/ (Go)
- Migrations: ~/VibePilot/migrations/ (112-125, all RPC fixes)
- Vault encryption: /tmp/vault_encrypt2_bin (Go only, Python incompatible)

## Known Issues / TODO
- ROI calculation shows $0 on dashboard (needs investigation)
- OpenRouter credit: -$0.57 (use free models only)
- GLM-5 subscription ends May 1
- Migrations 112-125 were all contract-break fixes (14 migrations)
- Governor remaining: max_attempts, dependency resolution, failure feedback rich fields

## Session History (recent changes)
- Apr 19: Contract registry + global manifest built
- Apr 19: Model pricing corrected to real API rates (all 25+ models)
- Apr 19: OpenRouter 5 free models added to DB + config
- Apr 19: Qwen3.6-Plus, Qwen3.5 Flash/Plus, MiniMax M2.7 added
- Apr 19: Kimi K2 marked avoid for autonomous (bans after 5hrs)
- Apr 19: Three sources of truth documented (GitHub + Supabase + Dashboard)

## Dashboard Wiring Analysis (LOOK + UNDERSTAND, Apr 19)

### WIRED AND WORKING (live Supabase data)
- Task list with real-time status updates (5 table subscriptions via Postgres changes)
- Model/Agent panel showing all 24 active models with logos, context limits, status
- Slice progress (tasks grouped by slice_id with completion counts)
- Token usage totals (summed from task_runs.tokens_used)
- Event timeline (orchestrator_events mapped to MissionEvent[])
- Real-time updates: any DB change auto-refreshes dashboard within seconds

### WIRED BUT SHOWING $0 (data pipeline gap)
- ROI calculation: Adapter code is correct (calculateROI sums cost columns from task_runs)
  BUT: task_runs rows all have platform_theoretical_cost_usd=0, total_actual_cost_usd=0, total_savings_usd=0
  ROOT CAUSE: calc_run_costs RPC in Go either not being called or returning 0
  FIX NEEDED: Trace calc_run_costs RPC call in handlers_task.go, check if it writes to those columns
- Subscription ROI: calculateSubscriptionROI reads subscription_* columns from models table
  BUT: No models have subscription_status, subscription_cost_usd populated in DB
  FIX NEEDED: Populate subscription fields for GLM-5 ($30/mo, ends May 1)

### PRESENT BUT NEEDS INVESTIGATION
- ReviewPanel: Exists with workflow dispatch to GitHub Actions (needs VITE_GH_TOKEN env)
- AgentHangar: Agent overview modal exists, reads from models/platforms
- LearningFeed: Component exists but unclear if wired to real data
- Failures component: Exists but unclear if wired
- AdminControlCenter modal: Exists in modals directory

### NOT YET BUILT (from vibeflow architecture)
- Visual QA agent integration (Gemini screenshot validation)
- Courier chat URL display (chat_urls in task_runs for revision context)
- Run task button (RunTaskButton.tsx exists - check if wired to governor API)
- Voice interface (voice.ts exists)

### DASHBOARD TECH STACK
- Vite + React (NOT Next.js)
- Supabase client for real-time subscriptions + queries
- Adapter pattern: vibepilotAdapter.ts transforms Supabase rows to Dashboard types
- Deployed: Vercel (vibeflow-dashboard.vercel.app)
- Fallback: Mock JSON files if Supabase not configured
- select("*") on tasks, task_runs, models, platforms — ANY dropped column breaks rendering

## Session Update: 2026-04-19 06:58 UTC

### Fixed: calc_run_costs contract drift
- ROOT CAUSE: SQL returned keys 'theoretical'/'actual'/'savings'
- Go struct expected: 'theoretical_cost_usd'/'actual_cost_usd'/'savings_usd'
- json.Unmarshal silently mapped nothing → every task_run wrote $0
- FIX: SQL function now returns USD-suffixed keys matching Go struct tags
- BACKFILL: All 45 existing task_runs updated with correct costs via Supabase REST
- RESULT: Dashboard ROI now shows $0.25 saved (was $0.000000)

### Dashboard state verified:
- Would Have Cost: $0.25
- Actually Cost: $0.00 (all free models)
- Total Savings: $0.25
- CAD converter: wired at 1.36 exchange rate (hardcoded)
- GLM-5 subscription: showing $30.00/mo, 11 days left, 483 tasks, $0.05/task

### This was EXACTLY the contract drift Gemini diagnosed
- Migration 122 created SQL with short keys
- Go code updated to call it with wrong JSON tag expectations  
- Both sides "correct" in isolation, mismatch at the interface
- 14 previous migrations (112-125) were likely similar contract breaks


## Session Update: 2026-04-19 07:39 UTC

### Model/Platform Cleanup
- DELETED: copilot (model + platform) — never had access, caused 16 duplicates on dashboard
- PAUSED: deepseek-chat, deepseek-reasoner — credit_remaining_usd=0, triggers Credit Needed flag
- BENCH: deepseek-web, gemini-web — browser automation not built
- BENCH: kimi-k2-instruct — blacklisted (bans after 5hrs autonomous)
- BENCH: opencode — CLI tool, not a model
- BENCH: nemotron-ultra-253b — duplicate of nvidia/llama-3.1-nemotron-ultra-253b-v1
- COURIER FIX: All Q (API) models set courier=false. Only qwen3.6-plus (web) = courier=true

### Three Review Triggers (for human review button)
1. Visual UI/UX approval (after changes)
2. System researcher suggestions (after council review)
3. API credit exhaustion (status=paused + status_reason contains "credit")

### Status Filter Chain (verified working)
- DB: status + status_reason → adapter: agentStatus → normalizeAgentStatus → filter button
- credit_needed → "credit" → 💰 Credit Needed
- cooldown (with expires_at) → "cooldown" → ⏳ Cooldown  
- in_progress → "active" → ↻ Active
- idle → "ready" → ✓ Ready

### Current Active Models (16)
Groq: llama-3.3-70b, llama-3.1-8b, qwen3-32b
OpenRouter: qwen3.6-plus:free, qwen3-coder, nemotron-3-super-120b, gemma-4-31b, glm-4.5-air
API: glm-5 (hermes), gemini-2.5-flash, gemini-api, qwen3.5-flash, qwen3.5-plus, minimax-m2.7
Web: qwen3.6-plus (courier via chat.qwen.ai)
NVIDIA: llama-3.1-nemotron-ultra-253b-v1
