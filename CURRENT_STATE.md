# VibePilot Current State
# AUTO-UPDATED: 2026-04-21 00:30 UTC
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
  ↓ governs
Dashboard (what user sees and controls)
  ↓ reads from
Supabase (data layer)
  ↓ fed by
Governor (pipeline executor)
  ↓ assisted by
Hermes (maintenance, audit, contract enforcement)
```

## System Status

- **Governor:** STOPPED + DISABLED
- **Git:** main branch, clean, synced with origin
- **Dashboard:** Live, 0 tasks
- **Chrome CDP:** 127.0.0.1:9222

## Human Role (VERY LIMITED — 3 things only)

1. **Visual UI/UX review** — after visual tester agent has reviewed
2. **Paid API benched** — out of credit, human decides add credits or keep benched
3. **Research after council** — council-reviewed suggestions, human gives final yes/no

## Self-Learning System — FULLY WIRED (all agents, all stages)

### Data Collection (every agent process records outcomes)

| Process | Model Tracking | Outcome Recording | Creates Rules | Coverage |
|---------|---------------|-------------------|---------------|----------|
| Planner | YES | YES | YES (on approval: reinforce) | 95% |
| Supervisor (plan review) | YES | YES | YES (on rejection: create rules) | 95% |
| Council (plan) | YES | YES (per-member) | YES (on block/revise) | 95% |
| Task Execution | YES | YES | YES | 95% |
| Task Review (supervisor) | YES | YES | YES | 95% |
| Testing | YES | YES | YES (on failure) | 90% |
| Research (supervisor review) | YES | YES | N/A | 90% |
| Research Council | YES | YES (per-member) | N/A | 85% |
| Maintenance | partial | partial | NO | 40% |

### Learning Feedback Loops

1. **Rejection → Rule creation:** Supervisor and council both create planner rules on rejection
2. **Approval → Rule reinforcement:** Approved plans increment effectiveness of active rules
3. **Per-model vote tracking:** Council members get success/failure based on vote alignment with consensus
4. **Failed model exclusion:** Test failures store failed model, excluded from next routing attempt
5. **Router scoring:** GetModelLearnedScore reads success/failure data to prefer better models
6. **Context injection:** context_builder loads rules, recent failures, heuristics into agent context

### Learning RPCs: 16 wired, 0 orphan

All learning RPCs in the schema are now called from Go code.

## Routing

- **SelectRouting** (current API): cascade with learned scoring, rate-limit aware
- **SelectDestination** (legacy wrapper): delegates to SelectRouting
- Council members: per-member routing through cascade, ExcludeModels on failure
- Same model can serve multiple council lenses sequentially if only one available
- No hardcoded model assignments — everything through cascade

## Models: 57 in config, 58 in DB

### Active API (33) — all keys verified
Groq (7), OpenRouter Paid (5), OpenRouter Free (13), NVIDIA (3), Gemini (4 keys), Other (1), Hermes/CLI (1)

### Active Web Courier (16)
All need browser automation (not built yet)

### Paused (2): deepseek-chat, deepseek-reasoner
### Benched (8): various

## Recent Commits (this session)
- 65f03985 feat: reinforce learned rules on plan approval
- ccd6697c feat: supervisor plan rejection creates planner rules
- 2753356a feat: wire research handler learning + upgrade to SelectRouting
- 4881c945 feat: wire council handler model learning
- 95a0c209 docs: end-to-end learning audit
- faea1e29 feat: exclude failed executor on test failure reroute (GAP 3)
- 1e7ffde1 feat: learning-driven model routing (GAP 2)
