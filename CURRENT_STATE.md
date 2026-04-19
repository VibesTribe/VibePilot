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
