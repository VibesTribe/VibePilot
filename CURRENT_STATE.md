# VibePilot Current State
# AUTO-UPDATED: 2026-04-20 21:00 UTC
# RULE: Update this file after ANY change set. Resume from here, never from guesses.
# RULE: NEVER update from assumptions. ALWAYS verify against actual code/data first.

## Three Sources of Truth

1. **GitHub (code):** https://github.com/VibesTribe/VibePilot — pushed=real
2. **Supabase (data):** https://qtpdzsinvifkgpxyxlaz.supabase.co — in DB=real
3. **Dashboard (live):** https://vibeflow-dashboard.vercel.app/ — rendering=working
   - Dashboard is USER DOMAIN. Hermes never modifies dashboard code.

## System Status

- **Governor:** STOPPED + DISABLED (won't auto-start on boot)
- **Git:** main branch, clean, synced with origin
- **Dashboard:** Live, 0 tasks, 0 task_runs (clean slate)
- **Chrome CDP:** 127.0.0.1:9222

## Models: 57 in config, 58 in DB (synced)

### Active API (33) — all keys verified working via vault
**Groq (7):** llama-4-scout, gpt-oss-120b, compound, compound-mini, llama-3.3-70b, llama-3.1-8b, qwen3-32b
**OpenRouter Paid (5):** gemma-4-31b-it, glm-4.5-air, minimax-m2.5, nemotron-3-super-120b-a12b, nemotron-3-super-120b
**OpenRouter Free (12):** gemma-4-26b-a4b-it, gemma-3-27b-it, gemma-3-12b-it, gemma-3-4b-it, gemma-3n-e2b-it, gemma-3n-e4b-it, llama-3.2-3b, llama-3.3-70b, hermes-3-llama-3.1-405b, qwen3-coder, nemotron-3-nano-30b, nemotron-nano-12b-v2-vl, gpt-oss-20b
**NVIDIA NIM (3):** llama-3.3-70b-instruct, kimi-k2-instruct, nemotron-ultra-253b-v1 (format issue not auth)
**Gemini (4 keys):** 2.5-flash-lite (Courier), 3.1-flash-lite-preview (Researcher), 3-flash-preview (Visual), 2.5-flash-lite (General)
**Other (1):** bytedance/ui-tars-1.5-7b (courier vision)
**Hermes/CLI (1):** glm-5 (interactive only, not pipeline-routable, ends May 1)

### Active Web Courier (16)
All need browser automation (not built yet). gemini-2.5-pro, deepseek-r1/v3, qwen-2.5/3.6-plus, mistral-large/codestral/pixtral, chatgpt-4o-mini-chatbox, perplexity, poe, aizolo, kimi-k2.6, gpt-4o-mini-chatbox

### Paused (2): deepseek-chat, deepseek-reasoner (out of credit)
### Benched (8): chatgpt-4o-mini, claude-sonnet, gemini-web, kimi-k2-instruct, minimax-m2.7, nemotron-3-super-120b, gemini-2.5-flash (legacy), qwen-3 (legacy name)

## Connectors: 26 total (7 API, 14 web, 1 CLI, 4 inactive)

## Secrets Vault: 15 entries, 10 decrypt OK, 4 can't (different encryption)

## Learning System — FULLY WIRED (all 5 gaps resolved)

### Data Collection (recording on every lifecycle event)
| Handler | RecordUsage | RecordCompletion | recordSuccess/Failure | update_model_learning |
|---------|-------------|-----------------|----------------------|----------------------|
| handlers_task.go | YES | YES | YES | YES |
| handlers_plan.go | YES | YES | YES | YES |
| handlers_maint.go | — | — | YES | — |
| handlers_testing.go | YES (via usageTracker) | YES | — | YES (on failure) |

### Router Intelligence (GAP 2 resolved, commit 1e7ffde1)
- selectByCascade scores candidates by GetModelLearnedScore(modelID, taskType)
- Score 0-1 based on BestForTaskTypes (+0.2), AvoidForTaskTypes (-0.5), FailureRateByType (-0.2/fail)
- Primary sort by score (0.1 threshold), tiebreaker by load balance
- Logs include score + taskType for debugging

### Test Failure Rerouting (GAP 3 resolved, commit faea1e29)
- Testing handler stores failed executor model ID in routing_flag_reason
- Task handler reads it on next available pickup and excludes from routing
- Prevents re-assigning task to same model that failed tests

### Config-DB Sync (GAP 5 resolved, commit 191dfc8f)
- All 57 config models now in Supabase DB (58 total with 2 benched legacy entries)
- 0 status mismatches

### Remaining: GAP 4 (Dashboard analytics — user domain, not Hermes)

## Key Architecture Rules
- Credentials in Supabase vault only. No .env files.
- Token counting always client-side. Never trust external counts.
- Groq needs User-Agent header or gets Cloudflare 1010.
- Governor disabled during build phase.
- Hermes never touches dashboard code.

## Recent Commits (this session)
- faea1e29 feat: exclude failed executor model on test failure reroute (GAP 3)
- 1e7ffde1 feat: learning-driven model routing (GAP 2)
- f13b29d2 feat: wire test pass/fail learning into testing handler
- 62fa81c5 docs: update learning system state
- 1b1cc612 fix: add missing reviewStart variable for supervisor review learning
- 191dfc8f fix: sync 25 models to Supabase DB (GAP 5)
- b2f11095 docs: learning system analysis
- 022c2908 state: verified full system audit
