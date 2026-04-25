# VibePilot Current State
# AUTO-UPDATED: 2026-04-25 17:35 UTC — VERIFIED AGAINST CODE AND CONFIG FILES
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

- **Governor:** RUNNING (PID 329991, systemd service, Restart=always)
  - Binary: /home/vibes/vibepilot/governor/governor
  - Config: /home/vibes/vibepilot/governor/config/ (GOVERNOR_CONFIG_DIR env var)
  - WARNING: /vibepilot/config/ is a stale git copy. Always use governor/config/.
  - Database: Local PostgreSQL 16 (system.json type=postgres)
  - Webhook: port 8080/webhooks
  - SSE: pg_notify on vp_changes → SSE broker → dashboard
  - Governor URL: https://webhooks.vibestribe.rocks (for courier callbacks)
- **Git:** main branch. Last: a64e8e2d
- **Dashboard:** Live at vibeflow-dashboard.vercel.app
- **Chrome CDP:** 127.0.0.1:9222
- **Pipeline tables:** EMPTY (truncated, ready for E2E test)
- **System counters:** 487,793 tokens / 48 runs lifetime

## Human Role (3 things only)

1. **Visual UI/UX review** — after visual tester agent has reviewed
2. **Paid API benched** — out of credit, human decides add credits or keep benched
3. **Research after council** — council-reviewed suggestions, human gives final yes/no

## E2E Pipeline Path (verified 2026-04-25)

```
1. Push PRD to docs/prd/*.md
2. GitHub webhook → webhooks.vibestribe.rocks/webhooks → governor HandlePush
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

## COURIER SYSTEM — BUGS FIXED, READY FOR E2E

### Bugs Fixed (2026-04-25, 5 bugs total)
1. courier_run.py: status "completed" → "success" (CHECK constraint mismatch)
2. record_courier_result: consolidated to single text overload with counter
3. Duplicate task_runs: update_courier_task_run replaces create_task_run
4. pgnotify EventCourierResult: queries DB instead of unmarshaling nil Record
5. pgnotify+realtime: checked "completed" but task_runs uses "success" (same root as BUG 1)

### GitHub Actions Workflow (verified)
- courier_dispatch.yml: python 3.12, browser-use, playwright+chromium
- GitHub webhook: webhooks.vibestribe.rocks/webhooks (active, push events, no secret)

## RECENT COMMITS

1. a64e8e2d — chore: sync config/ with governor/config/
2. d5422161 — fix: BUG 5 - pgnotify+realtime status 'completed' → 'success'
3. cf96ebbd — chore: add migration 128 for courier RPC fixes
4. dd7fba1c — fix: 4 courier pipeline bugs
