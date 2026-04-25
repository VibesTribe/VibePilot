# VibePilot Current State
# AUTO-UPDATED: 2026-04-25 18:25 UTC — VERIFIED AGAINST CODE AND CONFIG FILES
# RULE: Update after ANY change. Resume from here, never from guesses.
# RULE: NEVER update from assumptions. ALWAYS verify against actual code/data.

## Three Sources of Truth

1. **GitHub (code):** https://github.com/VibesTribe/VibePilot — pushed=real
2. **Local PostgreSQL (data):** localhost:5432, db=vibepilot, user=vibepilot — in DB=real
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

- **Governor:** RUNNING (systemd service, Restart=always)
  - Binary: /home/vibes/vibepilot/governor/governor
  - Config: /home/vibes/vibepilot/governor/config/ (GOVERNOR_CONFIG_DIR env var)
  - WARNING: /vibepilot/config/ is a stale git copy. Always use governor/config/.
  - Database: Local PostgreSQL 16 (system.json type=postgres)
  - Webhook: port 8080/webhooks
  - SSE: pg_notify on vp_changes → SSE broker → dashboard
  - Governor URL: https://webhooks.vibestribe.rocks (for courier callbacks)
  - GitHub webhook: configured with secret (vp_webhook_2026_secret, stored in vault)
  - Vault: all secrets encrypted with current x220 VAULT_KEY, decrypt verified
- **Git:** main branch. Last: ef612db1
- **Dashboard:** Live at vibeflow-dashboard.vercel.app
- **Chrome CDP:** 127.0.0.1:9222
- **Pipeline tables:** EMPTY (truncated, ready for E2E test)
- **System counters:** ~487,793 tokens / ~48 runs lifetime

## Human Role (3 things only)

1. **Visual UI/UX review** — after visual tester agent has reviewed
2. **Paid API benched** — out of credit, human decides add credits or keep benched
3. **Research after council** — council-reviewed suggestions, human gives final yes/no

## E2E Pipeline Path (verified 2026-04-25)

```
1. Push PRD to docs/prd/*.md
2. GitHub webhook (with secret) → webhooks.vibestribe.rocks/webhooks → governor HandlePush
3. create_plan RPC → INSERT plans (status=draft)
4. pgnotify → EventPlanCreated → handlePlanCreated
5. Planner: SelectRouting(role=planner, routingFlag=internal) → free_cascade → API model
6. Planner output parsed → plan file written to docs/plans/ → committed
7. Plan status → "review" → pgnotify → EventPlanReview
8. Supervisor review → if approved → createTasksFromApprovedPlan
9. Tasks created (status=pending, routing_flag from plan)
10. pgnotify → EventTaskAvailable → handleTaskAvailable
11. Task routed via SelectRouting(role=task_runner, routingFlag from task)
12. For internal tasks: hermes/API connector runs task
13. For courier tasks: CourierRunner → GitHub Actions → courier_run.py → POST result
14. Supervisor reviews output → testing → merge
```

## MODELS: 53 in config, routed via free_cascade

### Active API Connectors (internal execution)
- hermes (CLI) — glm-5
- gemini-api-courier — gemini-2.5-flash-lite
- gemini-api-researcher — gemini-3.1-flash-lite-preview
- gemini-api-visual — gemini-3-flash-preview
- gemini-api-general — gemini-2.5-flash
- groq-api — llama-3.3-70b, qwen3-32b, etc.
- nvidia-api — nemotron-ultra-253b, llama-3.3-70b, kimi-k2
- openrouter-api — many free models

### Agents (governor/config/agents.json v2.3)
- All agents have empty model field = cascade routing via free_cascade
- No supabase references (cleaned)
- context_policy per agent: planner=full_map, task_runner=targeted, most=file_tree

## COURIER SYSTEM — ALL BUGS FIXED, READY FOR E2E

### Bugs Fixed (2026-04-25, 5 bugs total)
1. courier_run.py: status "completed" → "success" (CHECK constraint mismatch)
2. record_courier_result: consolidated to single text overload with counter
3. Duplicate task_runs: update_courier_task_run replaces create_task_run
4. pgnotify EventCourierResult: queries DB instead of unmarshaling nil Record
5. pgnotify+realtime: checked "completed" but task_runs uses "success" (same root as BUG 1)

### GitHub Actions Workflow (verified)
- courier_dispatch.yml: python 3.12, browser-use, playwright+chromium
- GitHub webhook: webhooks.vibestribe.rocks/webhooks (active, push events, secret configured)
- Courier packet now includes callback_url instead of supabase_url/supabase_key

## x220 MIGRATION CLEANUP (2026-04-25)

### What was wrong after moving to x220
- Vault secrets encrypted with old machine's key → governor couldn't decrypt
- system.json still referenced SUPABASE_URL/SUPABASE_SERVICE_KEY env vars
- webhooks.enabled was false in system.json
- GitHub webhook had no secret configured
- /config/ was stale copy, wildly diverged from governor/config/ (the actual config)
- Supabase references throughout Go codebase (87 occurrences in active paths)

### What was fixed
- Vault webhook_secret re-encrypted with current x220 VAULT_KEY
- system.json: removed SUPABASE env refs, webhooks.enabled→true, claude-code→hermes
- GitHub webhook secret configured (stored in vault + GitHub)
- /config/ synced with governor/config/, 10 stale files removed
- pr-dispatch.yml: replaced broken Supabase calls with no-op (governor webhook handles PRDs)
- Go codebase Supabase refs reduced 87→26 (remaining are comments/deprecated tools)
- realtime/client.go renamed to .deprecated (zero imports, dead Supabase WebSocket client)
- config.go defaults: type supabase→postgres, env vars updated, GetRealtimeURL stubbed
- handlers_task.go: supabase_url/supabase_key in courier packet → callback_url
- cmd/vault_encrypt: new utility for encrypting vault secrets

## RECENT COMMITS

1. ef612db1 — fix: remove remaining Supabase references from codebase
2. 5c40f7f8 — fix: restore url_env/key_env for config validator
3. 1ab3e41d — fix: clean up x220 migration issues
4. b3ae84c5 — docs: update CURRENT_STATE with E2E readiness
5. a64e8e2d — chore: sync config/ with governor/config/
6. d5422161 — fix: BUG 5 - pgnotify+realtime status 'completed' → 'success'
7. cf96ebbd — chore: add migration 128 for courier RPC fixes
8. dd7fba1c — fix: 4 courier pipeline bugs
