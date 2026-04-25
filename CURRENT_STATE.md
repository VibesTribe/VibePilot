# VibePilot Current State
# AUTO-UPDATED: 2026-04-25 16:55 UTC — VERIFIED AGAINST CODE AND CONFIG FILES
# RULE: Update after ANY change. Resume from here, never from guesses.
# RULE: NEVER update from assumptions. ALWAYS verify against actual code/data.

## Three Sources of Truth

1. **GitHub (code):** https://github.com/VibesTribe/VibePilot — pushed=real
2. **Local PostgreSQL (data):** localhost:5432, db=vibepilot, user=vibes — in DB=real
3. **Dashboard (live):** https://vibeflow-dashboard.vercel.app/ — rendering=working
   - Dashboard is USER DOMAIN. Hermes never modifies dashboard code.

## Hierarchy (everything serves what's above it)

```
VibePilot Architecture & Principles (modular, agnostic, no hardcoding)
  → Dashboard (what user sees and controls)
    → PostgreSQL (data layer, local)
      → Governor (pipeline executor)
        → Hermes (maintenance, audit, contract enforcement)
```

## System Status

- **Governor:** RUNNING (PID 327784, systemd service, Restart=always)
  - Binary: /home/vibes/vibepilot/governor/governor
  - Runtime repo: ~/vibepilot (config, binary, prompts)
  - Database: Local PostgreSQL 16 (system.json type=postgres)
  - Webhook: port 8080/webhooks
  - SSE: pg_notify on vp_changes → SSE broker → dashboard
  - Governor URL: https://webhooks.vibestribe.rocks (for courier callbacks)
- **Git:** main branch. Last: cf96ebbd
- **Dashboard:** Live at vibeflow-dashboard.vercel.app
- **Chrome CDP:** 127.0.0.1:9222
- **Host:** x220, up 4d4h, 15GB RAM, 4.6GB used
- **Pipeline tables:** EMPTY (truncated, ready for E2E test)
- **System counters:** 487,793 tokens / 48 runs lifetime

## Human Role (3 things only)

1. **Visual UI/UX review** — after visual tester agent has reviewed
2. **Paid API benched** — out of credit, human decides add credits or keep benched
3. **Research after council** — council-reviewed suggestions, human gives final yes/no

## ROUTING: CONFIG-DRIVEN, NO HARDCODED VALUES (2026-04-24)

### Token Estimation
- `EstimateTokens(content, role)` computes from actual content length
- Code-aware: detects structural characters ({, }, [, ], etc.) for denser token ratio (~3 chars/token vs ~4 for prose)
- Response budget by role: planner 2x, task_runner 1x, supervisor 0.5x, courier 0.25x
- No hardcoded magic numbers — estimate comes from the actual task content

### Context Limit Enforcement
- `CanMakeRequestVia()` checks `estimatedTokens > model.ContextLimit`
- Models that can't fit the task are rejected with reason "exceeds_context_limit"
- Cascade automatically skips them and tries next model in priority order
- Context limit comes from models.json per-model `context_limit` field

### Config-Driven Timeouts
- All timeouts read from system.json with sensible fallbacks:
  - `system.execution.default_timeout_seconds` → GetRunnerTimeoutSecs() (fallback 300)
  - `system.db.http_timeout_seconds` → GetDBHTTPTimeoutSecs() (fallback 30)
  - `system.db.error_truncate_length` → GetDBErrorTruncateLen() (fallback 200)
  - `system.http.client_timeout_seconds` → GetHTTPClientTimeoutSecs() (fallback 30)
  - `system.http.response_timeout_seconds` → GetHTTPIdleTimeoutSecs() (fallback 30)
  - `system.courier.timeout_seconds` → GetCourierTimeoutSecs() (fallback 30) — courier agents (browser-use, slower)
- Per-connector `timeout_seconds` in connectors.json overrides the global default
- Tasks are tiny, one-shot, designed for small-context weaker models
- Fast: task execution, supervisor review, testing, simple research
- Moderate: PRD processing, plan generation, complex research suggestions
- Slowest: courier agents (real browser navigation on free web tiers)
- All adjustable via config, no code changes needed

## MODELS: CONFIG ↔ DB SYNC VIA RESEARCH PIPELINE

### Current State (Verified 2026-04-25)
- **Config/models.json:** 16 model entries
- **DB:** models loaded at startup from config (no hot reload, restart required)
- **UsageTracker:** Restores all models from DB on startup

### Sync Mechanism (deterministic, no LLM middleman)
- **Research → Direct Apply:** When supervisor approves research suggestion with type:
  - `new_model`, `pricing_change`, `config_tweak` → writes config/models.json + upserts DB
  - `new_platform` → writes config/connectors.json
- **ActionApplier:** Runtime package that handles both file writes and DB operations
- **Thread-Safe:** Mutex-protected config file writes prevent race conditions

## CONNECTORS (verified 2026-04-24)
**20 total** in config/connectors.json:
- CLI: 4 (hermes active; opencode, claude-code, kimi inactive)
- API: 7 (gemini-api-courier, gemini-api-researcher, gemini-api-visual, gemini-api-general, groq-api, openrouter-api, nvidia-api active; deepseek-api inactive)
- Web: 15 active (chatgpt, claude, gemini, deepseek, qwen, mistral, notegpt, kimi, huggingchat, aistudio, poe, chatbox, aizolo, perplexity, openrouter)

## SELF-LEARNING SYSTEM — FULLY WIRED

All 6 handlers have learning coverage (verified by grep):
- plan: 10 calls, 95% coverage
- council: 6 calls, 95% coverage
- task: 21 calls, 98% coverage
- testing: 3 calls, 90% coverage
- research: 4 calls, 90% coverage
- maint: 7 calls, 95% coverage

## COURIER SYSTEM — BUILT, BUGS FIXED, READY FOR E2E TEST

Architecture: GitHub Actions + Governor API + SSE
- governor/internal/connectors/courier.go: CourierRunner with dispatch + channel-based result waiters
- scripts/courier_run.py: Browser-use with platform selectors, posts to governor API
- POST /api/courier/result: Governor receives courier results, writes to task_runs, notifies waiter
- SSE bridge: pg_notify → SSEBroker → dashboard live updates
- governor_external_url: https://webhooks.vibestribe.rocks (in system.json)

### Bugs Fixed (2026-04-25, commit dd7fba1c)
1. **courier_run.py status**: "completed" → "success" to match task_runs CHECK constraint
2. **record_courier_result RPC**: Consolidated from 2 overloads (text/jsonb) to single text overload that always calls increment_lifetime_counters
3. **Duplicate task_runs**: executeCourierTask now calls update_courier_task_run (updates existing row) instead of create_task_run (which created duplicates)
4. **pgnotify EventCourierResult handler**: Now queries DB for task_run data instead of unmarshaling nil event.Record

## CONSULTANT AGENT — BUILT, TESTED ONCE

- Prompt: prompts/consultant.md (539 lines, 20KB)
- PRD template: prompts/prd_template.md
- Successfully produced Knowledge Graph PRD (14.5KB, in docs/pending/)

## VAULT MANAGEMENT — CLI + API

Architecture: AES-GCM encrypted secrets in `secrets_vault` table (local PG).
- CLI: `./governor vault set KEY "value"` / `get KEY` / `list` / `delete KEY` / `rotate-key NEWKEY`
- API: `/api/vault/set|get|list|delete|rotate-key` (Bearer token auth via GOVERNOR_ADMIN_TOKEN)
- Config-driven: vault.key_env in system.json tells governor which env var holds the master key
- Bootstrap: only DATABASE_URL + VAULT_KEY needed as env vars. All other secrets in vault.

## LOCAL PG RPCs: 146 total (verified 2026-04-25)

## RECENT COMMITS

1. cf96ebbd — chore: add migration 128 for courier RPC fixes
2. dd7fba1c — fix: 4 courier pipeline bugs (status mismatch, duplicate RPC, nil Record, counter)
3. 9c580e6d — Remove all Supabase from courier pipeline, add external callback URL
4. e0ebb144 — fix: task handler respects planner routing_flag from DB
5. 5d5f43fb — feat: add human_review status — three mandatory human triggers
