# VibePilot Current State
# AUTO-UPDATED: 2026-04-21 01:15 UTC
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

### Handler Learning Coverage

| Handler | Learning Calls | Supervisor Tracked | Coverage |
|---------|---------------|-------------------|----------|
| Plan | 9 | YES (plan review) | 95% |
| Council | 6 | N/A (IS council) | 95% |
| Task | 21 | YES (task review) | 98% |
| Testing | 3 | N/A | 90% |
| Research | 4 | YES (research review) | 90% |
| Maintenance | 5 | N/A | 95% |

### Supervisor Tracked Across All Three Contexts
- **Plan review:** model success/failure + RecordCompletion, timeout, parse failure
- **Task review:** model success/failure + RecordCompletion, timeout, parse failure, all outcomes
- **Research review:** model success/failure + RecordCompletion

### Learning Feedback Loops (all closed)

1. **Rejection → Rule creation:** Supervisor AND council both create planner rules on rejection
2. **Approval → Rule reinforcement:** Approved plans increment effectiveness of active rules
3. **Per-model vote tracking:** Council members get success/failure based on vote alignment with consensus
4. **Failed model exclusion:** Test failures store failed model, excluded from next routing attempt
5. **Router scoring:** GetModelLearnedScore reads success/failure data to prefer better models
6. **Context injection:** context_builder loads rules, recent failures, heuristics into agent context
7. **Per-role learning:** Router learns which models are good supervisors, council members, planners, etc.

### Learning RPCs: ALL wired (zero orphan)

## Routing

- **SelectRouting ONLY** — all legacy SelectDestination/LegacyRoutingRequest calls removed from handlers
- Per-member routing through cascade for council (plan + research)
- Same model can serve multiple council lenses sequentially if only one available
- No hardcoded model assignments — everything through cascade
- ExcludeModels on failure — no repeating bad assignments
- Maintenance agent can be any model: Hermes, Claude Code, Kilo CLI, etc.

## Models: 57 in config, 58 in DB

### Active API (33) — all keys verified
Groq (7), OpenRouter Paid (5), OpenRouter Free (13), NVIDIA (3), Gemini (4 keys), Other (1), Hermes/CLI (1)

### Active Web Courier (16)
All need browser automation (not built yet)

### Paused (2): deepseek-chat, deepseek-reasoner
### Benched (8): various

## Recent Commits (this session)
- bff45f30 feat: full supervisor model tracking in plan review
- e5e22005 feat: full supervisor model tracking in task review
- 65f03985 feat: reinforce learned rules on plan approval
- ccd6697c feat: supervisor plan rejection creates planner rules
- 2753356a feat: wire research handler learning + upgrade to SelectRouting
- 4881c945 feat: wire council handler model learning
- 95a0c209 docs: end-to-end learning audit
- faea1e29 feat: exclude failed executor on test failure reroute (GAP 3)
- 1e7ffde1 feat: learning-driven model routing (GAP 2)
