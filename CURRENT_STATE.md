# VibePilot Current State
# AUTO-UPDATED: 2026-04-23 13:30 UTC — VERIFIED AGAINST CODE AND CONFIG FILES
# RULE: Update after ANY change. Resume from here, never from guesses.
# RULE: NEVER update from assumptions. ALWAYS verify against actual code/data.

## Three Sources of Truth

1. **GitHub (code):** https://github.com/VibesTribe/VibePilot — pushed=real
2. **Supabase (data):** https://qtpdzsinvifkgpxyxlaz.supabase.co — in DB=real
3. **Dashboard (live):** https://vibeflow-dashboard.vercel.app/ — rendering=working
   - Dashboard is USER DOMAIN. Hermes never modifies dashboard code.

## Hierarchy (everything serves what's above it)

```
VibePilot Architecture & Principles (modular, agnostic, no hardcoding)
  → Dashboard (what user sees and controls)
    → Supabase (data layer)
      → Governor (pipeline executor)
        → Hermes (maintenance, audit, contract enforcement)
```

## System Status

- **Governor:** STOPPED (needs restart after vault + SSE merge)
  - Binary: /home/vibes/vibepilot/governor/governor
  - Runtime repo: ~/vibepilot (rebuilt with vault CLI + API)
  - Dev repo: ~/VibePilot (synced to origin/main)
  - Start: `source ~/.governor_env && export DATABASE_URL="postgres://vibes@/vibepilot?host=/var/run/postgresql" && export VAULT_KEY="..." && cd ~/vibepilot/governor && ./governor`
- **Git:** main branch, clean. Last: f2a376d5
- **Dashboard:** Live at vibeflow-dashboard.vercel.app
- **Chrome CDP:** 127.0.0.1:9222
- **Host:** x220, up 11h38m, 15GB RAM, 2.6GB used

## Human Role (3 things only)

1. **Visual UI/UX review** — after visual tester agent has reviewed
2. **Paid API benched** — out of credit, human decides add credits or keep benched
3. **Research after council** — council-reviewed suggestions, human gives final yes/no

## MODELS: CONFIG ↔ DB SYNC VIA RESEARCH PIPELINE

### Current State (Verified 2026-04-22)
- **Config/models.json:** 57 models (source of truth)
- **By status:** 49 active, 6 benched, 2 paused
- **By access type:** 37 API, 19 web, 1 CLI subscription (GLM-5)

### Composition of 57 Models
- **API (37):**
  - Gemini direct: 4 (courier, researcher, visual, general)
  - Groq: 5 (llama-3.3-70b, llama-3.1-8b, qwen3-32b, llama-4-scout, gpt-oss-120b)
  - NVIDIA NIM: 3 (nemotron-ultra-253b, llama-3.3-70b, kimi-k2-instruct)
  - OpenRouter free: 18 (gemma variants, llama variants, nemotron, qwen3-coder, hermes-405b, ui-tars, minimax, gpt-oss-20b, deepseek)
  - DeepSeek API: 2 (paused, out of credits)
  - OpenAI: 1 (chatgpt-4o-mini, benched)
- **Web/courier (19):**
  - chatgpt-web, claude-web, gemini-web, deepseek-web, qwen-web
  - mistral-web (3 models: large, codestral, pixtral)
  - notegpt-web (deepseek-v3), kimi-web, huggingchat-web
  - aistudio-web, poe-web, chatbox-web, aizolo-web
  - perplexity-web, gemini-2.5-pro-web, gemini-3.1-pro-preview-web
  - deepseek-r1-web, kimi-k2.6-instant, kimi-k2-instruct-0905-hf
- **CLI subscription (1):**
  - GLM-5 via Z.AI Pro (hermes connector, primary model)

### Paused/Benched Models
- **Paused (2):** deepseek-chat, deepseek-reasoner (out of credits)
- **Benched (6):** chatgpt-4o-mini, claude-sonnet, gemini-web, kimi-k2-instruct (Groq), claude-web, aizolo-mix

### Sync Mechanism (deterministic, no LLM middleman)
- **Research → Direct Apply:** When supervisor approves research suggestion with type:
  - `new_model`, `pricing_change`, `config_tweak` → writes config/models.json + upserts DB
  - `new_platform` → writes config/connectors.json
- **ActionApplier:** Runtime package that handles both file writes and DB operations
- **Thread-Safe:** Mutex-protected config file writes prevent race conditions

## CONNECTORS (verified 2026-04-22)
**26 total** in config/connectors.json:
- CLI: 4 (hermes active; opencode, claude-code, kimi inactive)
- API: 7 (gemini-api-courier, gemini-api-researcher, gemini-api-visual, gemini-api-general, groq-api, openrouter-api, nvidia-api active; deepseek-api inactive)
- Web: 15 (chatgpt-web, claude-web, gemini-web, deepseek-web, qwen-web, mistral-web, notegpt-web, kimi-web, huggingchat-web, aistudio-web, poe-web, chatbox-web, aizolo-web, perplexity-web, openrouter-api all active)

## SELF-LEARNING SYSTEM — FULLY WIRED

All 6 handlers have learning coverage (verified by grep):
- plan: 10 calls, 95% coverage
- council: 6 calls, 95% coverage
- task: 21 calls, 98% coverage
- testing: 3 calls, 90% coverage
- research: 4 calls, 90% coverage
- maint: 7 calls, 95% coverage

Supervisor tracked in plan review, task review, and research review.

## COURIER SYSTEM — BUILT AND WIRED

Architecture: GitHub Actions + Governor API + SSE
- governor/internal/connectors/courier.go: CourierRunner with dispatch + channel-based result waiters
- scripts/courier_run.py: Browser-use with platform selectors, posts to governor API
- .github/workflows/courier.yml: GitHub Actions workflow
- POST /api/courier/result: Governor receives courier results, writes to task_runs, notifies waiter
- SSE bridge: pg_notify → SSEBroker → dashboard live updates
- Status: Not yet E2E tested

## CONSULTANT AGENT — BUILT, TESTED ONCE

- Prompt: prompts/consultant.md (539 lines, 20KB)
- PRD template: prompts/prd_template.md
- Successfully produced Knowledge Graph PRD (14.5KB, in docs/pending/)
- Webhook trigger bug: isPRD() matches docs/prd/pending/ — should only match docs/prd/*.md directly
- Bug needs fix before next PRD is placed in docs/prd/

## PENDING SPECS (not scheduled)

- docs/pending/vibepilot-knowledge-graph-spec.md — Knowledge graph spec (PocketBase, dashboard viz, council review, research agent). NOT ready for planning — need simple pipeline test first.

## KNOWN BUGS

1. **Webhook PRD path matching** — FIXED. isPRD() now excludes subfolders.

## VAULT MANAGEMENT — CLI + API

Architecture: AES-GCM encrypted secrets in `secrets_vault` table (local PG).
- CLI: `./governor vault set KEY "value"` / `get KEY` / `list` / `delete KEY` / `rotate-key NEWKEY`
- API: `/api/vault/set|get|list|delete|rotate-key` (Bearer token auth via GOVERNOR_ADMIN_TOKEN)
- API enables future dashboard admin + Telegram chat to manage keys
- Config-driven: vault.key_env in system.json tells governor which env var holds the master key
- Bootstrap: only DATABASE_URL + VAULT_KEY needed as env vars. All other secrets in vault.

## RECENT COMMITS

1. bd3d479c — docs: add plan (auto-created by webhook from pending PRD, should not have triggered)
2. 419cc931 — feat: add Knowledge Graph PRD (pending) + update .context
3. 7abaeed4 — docs: add Hermes subagent delegation config to CURRENT_STATE
4. b1528a1d — docs: update CURRENT_STATE with consultant agent details
5. 7fbd059e — feat(consultant): agent prompt and PRD template
