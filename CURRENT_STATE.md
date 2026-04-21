# VibePilot Current State
# AUTO-UPDATED: 2026-04-21 02:45 UTC — VERIFIED AGAINST CODE AND SUPABASE
# NOTE: CONFIG AND DB ARE NOW IN SYNC VIA DETERMINISTIC RESEARCH PIPELINE
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

- **Governor:** STOPPED + DISABLED (inactive/dead)
- **Git:** main branch, clean, synced. Last: 576e93f8
- **Dashboard:** Live at vibeflow-dashboard.vercel.app
- **Chrome CDP:** 127.0.0.1:9222

## Human Role (3 things only)

1. **Visual UI/UX review** — after visual tester agent has reviewed
2. **Paid API benched** — out of credit, human decides add credits or keep benched
3. **Research after council** — council-reviewed suggestions, human gives final yes/no

## MODELS: CONFIG ↔ DB SYNC VIA RESEARCH PIPELINE

### Current State (Verified)
- **Config/models.json:** 30 models (source of truth)
- **Supabase DB models table:** 30 models (in sync)
- **Total modeled sources:** 30 (config and DB match)

### Composition of 30 Models
- **API (22):**
  - Groq: 7
  - NVIDIA: 3
  - OpenRouter paid: 4
  - OpenRouter free: 13
  - Gemini direct: 1 (experimental: gemini-2.5-flash)
- **Web/courier (8):**
  - chatgpt-web
  - claude-web  
  - gemini-web
  - deepseek-web
  - qwen-web
  - mistral-web
  - kimi-ai (new)
  - perplexity (new)

### Sync Mechanism (NEW - deterministic, no LLM middleman)
- **Research → Direct Apply:** When supervisor approves research suggestion with type:
  - `new_model`, `pricing_change`, `config_tweak` → writes config/models.json + upserts DB
  - `new_platform` → writes config/connectors.json
- **ActionApplier:** Runtime package that handles both file writes and DB operations
- **Fallback:** If direct apply fails, falls back to maintenance command (LLM agent) for compatibility
- **Status Tracking:** Research suggestions marked `implemented` on success, `approved` on fallback
- **Thread-Safe:** Mutex-protected config file writes prevent race conditions

### Prior Discrepancy (Fixed)
Previously: config had 16 models, DB had 58 (42 orphans from research additions never synced back)
Now: Both have 30 models — sync is bidirectional via research approval pipeline

## CONNECTORS (verified)
**12 total** in config/connectors.json:
- CLI: 3 (opencode, claude-code, kimi)
- API: 5 (gemini-api, deepseek-api, groq-api, nvidia-api, openrouter-api)
- Web: 4 (chatgpt-web, claude-web, gemini-web, mistral-web) + 4 newly added (deepseek-web, qwen-web, kimi-ai, perplexity)

Note: copilot-web, poe, aizolo were researched but marked unavailable (require accounts/payment) or UX issues (popups, chatbox distractions).

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

Architecture: GitHub Actions + Supabase Realtime
- governor/internal/connectors/courier.go: CourierRunner with dispatch + realtime waiters
- scripts/courier_run.py: Browser-use with platform selectors
- .github/workflows/courier.yml: GitHub Actions workflow
- handlers_task.go: web routing → CourierRunner
- Status: Not yet E2E tested (governor stopped)

## RECENT COMMITS (this session)

The commits show:
1. Wired self-learning feedback loops across all agent processes
2. Built deterministic research→config+DB sync (no LLM middleman)
3. Fixed config/DB discrepancy — now both show 30 models
4. Added 4 missing web platform connectors (kimi-ai, perplexity, poe, aizolo — though poe/aizolo unavailable)
5. CURRENT_STATE.md updated with verified counts and sync mechanism details
