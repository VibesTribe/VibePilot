# VibePilot Current State

**Read this first. This is the only document you need.**

**Last Updated:** 2026-02-14 17:00 UTC
**Updated By:** GLM-5 (session with human)

---

# WHAT IS VIBEPILOT

Sovereign AI execution engine. Human provides idea → VibePilot executes with zero drift.

**Core Rules:**
- All state in Supabase
- All code in GitHub
- All changes via config (zero code edits for swaps)
- Context isolation per agent (task agents see ONLY their task)
- Council for PRDs/plans (iterative), system updates (one-shot vote)

---

# WHERE WE ARE

## Built & Working

| Component | Status | Location |
|-----------|--------|----------|
| Core Schema | ✅ Live | Supabase: tasks, task_packets, models, platforms, projects |
| Model Registry | ✅ Live | GLM-5, Kimi K2.5, Gemini Flash, DeepSeek (paused) |
| Platform Registry | ✅ Live | ChatGPT, Claude, Gemini, Perplexity, DeepSeek, Grok |
| Kimi CLI Runner | ✅ Working | `runners/kimi_runner.py` |
| Dual Orchestrator | ✅ Working | `dual_orchestrator.py` |
| Role System | ✅ Defined | `core/roles.py` + `config/vibepilot.yaml` |
| Strategic Guardrails | ✅ Complete | `.context/guardrails.md` |
| Master Plan | ✅ Complete | `docs/MASTER_PLAN.md` |

## Not Yet Built

| Component | Status | Notes |
|-----------|--------|-------|
| Council RPC | ❌ Not started | Needs Supabase function |
| Courier Agent | ❌ Not started | Web platform dispatch |
| Voice Interface | ❌ Designed | `docs/voice_interface.md` |
| Dashboard | ❌ Exists separately | Vibeflow repo, needs Supabase connection |
| Prompt Caching | ❌ Pending | DEC-007 |
| Kimi Swarm Trigger | ❌ Pending | DEC-008 |

---

# WHERE WE'RE GOING

## Immediate (Next Session)

1. **Council RPC** - Supabase function for iterative consensus
2. **Prompt Caching** - Add to runners (DEC-007)
3. **Kimi Swarm** - Add trigger to orchestrator (DEC-008)

## Near Term

4. **Courier Agent** - Dispatch to web platforms
5. **Migration Prep** - Test setup.sh, prep for cheaper hosting
6. **TypeScript Decision** - DEC-006 (migrate or not?)

## Future

7. **Voice Interface** - Talk to Vibes
8. **Multi-Project** - Recipe app, finance app, VibePilot, legacy project

---

# KEY DECISIONS

| ID | Decision | Status | One-line summary |
|----|----------|--------|------------------|
| DEC-001 | Dual Orchestrator | ✅ Accepted | GLM-5 primary, Kimi parallel, Gemini research |
| DEC-002 | State in Supabase | ✅ Accepted | All state in DB, code in GitHub |
| DEC-003 | Bounded Roles | ✅ Accepted | 2-3 skills max per role |
| DEC-004 | Council Two-Process | ✅ Accepted | One-shot for updates, iterative for PRDs |
| DEC-005 | Context Isolation | ✅ Accepted | Task agents see ONLY their task |
| DEC-006 | TypeScript Migration | ⏳ Proposed | Pending decision |
| DEC-007 | Prompt Caching | ⏳ Pending | Add to API runners |
| DEC-008 | Kimi Swarm | ⏳ Pending | Trigger for wide tasks |

Full details: `.context/DECISION_LOG.md`

---

# CURRENT ISSUES

| Issue | Status | Notes |
|-------|--------|-------|
| Context bloat | 🟡 Active | This doc solves it |
| Kimi subscription ending | 🟡 Soon | End of month, plan migration |
| GCE cost | 🟡 Monitoring | Consider Hetzner/other |

---

# ARCHITECTURE QUICK REF

## Agent Context Levels

| Agent | Sees | Why |
|-------|------|-----|
| Task Agent | ONLY their task | Zero drift |
| Planner | PRD, system overview | Create atomic slices |
| Council | Everything | Thorough review |
| Supervisor | Plan + task | Validate output |
| Maintenance | Everything (sandbox) | System changes |
| Researcher | System overview | Find improvements |
| Tester | Only code | Objective testing |

## Council Process

| Type | Rounds | For |
|------|--------|-----|
| One-shot vote | 1 | System updates, maintenance, features |
| Iterative consensus | 3-4 | PRDs, full vertical slice plans |

## Model Lenses

| Model | Catches |
|-------|---------|
| GPT | Drift from user intent |
| Gemini | Opportunities |
| GLM-5 | Build issues, vulnerabilities |

## Swappability

| Change | Review | Downtime |
|--------|--------|----------|
| Model/platform add | Supervisor | Zero |
| Config change | Supervisor | Zero |
| Architecture change | Council | Planned |

---

# FILE MAP

```
vibepilot/
├── THIS_FILE              ← You are here
├── config/vibepilot.yaml  ← All runtime config
├── .context/
│   ├── guardrails.md      ← Pre-code gates
│   └── DECISION_LOG.md    ← Full decision details
├── docs/
│   └── MASTER_PLAN.md     ← Full specification
├── core/roles.py          ← Role definitions
├── runners/kimi_runner.py ← Kimi CLI
├── dual_orchestrator.py   ← Multi-model routing
└── task_manager.py        ← Task CRUD
```

---

# COUNCIL FEEDBACK PROTOCOL

**Problem:** Council produces lots of feedback → context bloat
**Solution:** Supervisor summarizes Council feedback into plan notes

```
1. Council Round 1: Each model outputs approach, concerns, suggestions
2. Supervisor aggregates: Summarize common themes, key concerns, required fixes
3. Summary added to plan as "council_feedback" field
4. Council Round 2+: Each model sees summary (not full outputs)
5. Final consensus: Summary of agreed approach in plan
```

**Council Feedback Note Format (in plan):**
```yaml
council_feedback:
  round: 2
  consensus_reached: true
  summary: "Use Python + Supabase + edge functions. TypeScript benefits achieved without rewrite."
  key_concerns_addressed:
    - "Gemini's scalability concern → addressed via edge functions"
    - "GLM-5's security concern → addressed via auth middleware"
  modifications_to_plan:
    - "Add edge function for X"
    - "Use auth middleware for Y"
```

---

# HOW TO UPDATE THIS FILE

**After every session:**
1. Update "Last Updated" timestamp
2. Update "WHERE WE ARE" if components changed
3. Update "WHERE WE'RE GOING" if priorities changed
4. Add new decisions to "KEY DECISIONS"
5. Update "CURRENT ISSUES" as they arise/resolve

**This file must always be accurate. It is the source of truth for context restoration.**

---

# QUICK START FOR NEW SESSION

```
1. Read this file (CURRENT_STATE.md)
2. Check "WHERE WE'RE GOING" for priorities
3. Check "CURRENT ISSUES" for blockers
4. Start work
5. Update this file at end of session
```

---

*Token count target: <3000 tokens (this doc)*
*Actual: ~2000 tokens*
*Context restoration: ONE file read*
