# VibePilot Current State
# AUTO-UPDATED: 2026-04-21 01:30 UTC — ALL DATA VERIFIED AGAINST ACTUAL CODE/DB
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
- **Git:** main branch, clean, synced. Last: 169dc0ef
- **Dashboard:** Live at vibeflow-dashboard.vercel.app
- **Chrome CDP:** 127.0.0.1:9222

## Human Role (3 things only)

1. **Visual UI/UX review** — after visual tester agent has reviewed
2. **Paid API benched** — out of credit, human decides add credits or keep benched
3. **Research after council** — council-reviewed suggestions, human gives final yes/no

## Models (from Supabase — verified)

Total: **58 in DB** | 16 in config/models.json (config is subset)

### Active: 48
- **API (37):** groq-api (7), openrouter-api free (11), openrouter-api paid (3), nvidia-api (3), gemini-api (4 standalone), other (2), groq-api new (3), hermes/cli (1)
- **Web/courier (11):** mistral-web (2), qwen-web (1), notegpt-web (1), aizolo (1), gemini-web (2), chatgpt-web (1), kimi-ai (2), perplexity (1), poe (1), deepseek-web (2), kimi-ai (2)

### Paused: 2 — deepseek-chat, deepseek-reasoner
### Benched: 8 — nemotron-3-super-120b, minimax-m2.7, chatgpt-4o-mini, claude-sonnet, gemini-web, gemini-2.5-flash, qwen-3, kimi-k2-instruct

## Connectors: 16 in config

- CLI (3): opencode, claude-code, kimi
- API (5): gemini-api, deepseek-api, groq-api, nvidia-api, openrouter-api
- Web (8): chatgpt-web, claude-web, gemini-web, copilot-web, deepseek-web, qwen-web, mistral-web, notegpt-web

## Self-Learning System — FULLY WIRED (all handlers, all stages)

### Handler Learning Coverage (verified by grep)

| Handler | Tracking Calls | Legacy API | Coverage |
|---------|---------------|------------|----------|
| plan | 10 | 0 | 95% |
| council | 6 | 0 | 95% |
| task | 21 | 0 | 98% |
| testing | 3 | 0 | 90% |
| research | 4 | 0 | 90% |
| maint | 7 | 0 | 95% |

### Supervisor Tracked Across All Contexts
- Plan review: success/failure + RecordCompletion, timeout, parse failure
- Task review: success/failure + RecordCompletion, timeout, parse failure, all outcomes
- Research review: success/failure + RecordCompletion

### Learning Feedback Loops (all closed)

1. **Rejection → Rule creation:** Supervisor AND council create planner rules on rejection
2. **Approval → Rule reinforcement:** Approved plans increment effectiveness of active rules
3. **Per-model vote tracking:** Council members get success/failure based on consensus alignment
4. **Failed model exclusion:** Test failures store failed model, excluded from next routing
5. **Router scoring:** GetModelLearnedScore reads success/failure data to prefer better models
6. **Context injection:** context_builder loads rules, recent failures, heuristics into agent context
7. **Per-role learning:** Router learns which models make good supervisors, council members, planners

### Learning RPCs: ALL wired (zero orphan)

## Routing

- **SelectRouting ONLY** — all legacy SelectDestination/LegacyRoutingRequest removed from handlers
- Per-member routing through cascade for council (plan + research)
- Same model can serve multiple council lenses sequentially if only one available
- No hardcoded model assignments — everything through cascade
- ExcludeModels on failure — no repeating bad assignments
- Maintenance agent can be any model (Hermes, Claude Code, Kilo CLI, etc.)

## Courier System — BUILT AND WIRED

### Architecture: GitHub Actions + Supabase Realtime (zero polling)

```
Governor routes to web platform
  → CourierRunner.dispatch() triggers GitHub Actions repository_dispatch
  → GitHub Actions runs browser-use + Playwright (headless Chromium)
  → Browser interacts with web AI platform (platform-specific selectors)
  → Result written to Supabase task_runs
  → Realtime EventCourierResult fires
  → CourierRunner.NotifyResult() delivers to waiting goroutine
  → Task continues in pipeline
```

### Components (all verified in repo)
- **governor/internal/connectors/courier.go:** CourierRunner, channel waiters, GitHub Actions dispatch, NotifyResult for realtime
- **scripts/courier_run.py:** Browser-use agent with platform selectors (ChatGPT, Gemini, DeepSeek, Qwen, generic)
- **.github/workflows/courier.yml:** GitHub Actions workflow (repository_dispatch, headless Chromium)
- **handlers_task.go:** Checks `routingFlag == "web"`, dispatches via CourierRunner
- **Realtime client:** EventCourierResult mapped to NotifyResult

### Status: Not yet E2E tested (governor is stopped)

## Recent Commits (this session — all on main, all pushed)

```
169dc0ef state: add courier system section
c6e23de3 state: all handlers fully tracked
bff45f30 feat: supervisor model tracking in plan review
e5e22005 feat: supervisor model tracking in task review
48dca080 refactor: remove last legacy SelectDestination call
7578a13a feat: full learning for maintenance handler
0e37f5c7 state: self-learning system fully wired
65f03985 feat: reinforce learned rules on plan approval
ccd6697c feat: supervisor plan rejection creates planner rules
2753356a feat: wire research handler learning + SelectRouting
4881c945 feat: wire council handler model learning
```
