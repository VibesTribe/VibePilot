# VibePilot Current State

**Read this first. This is the only document you need.**

**Last Updated:** 2026-02-14 17:40 UTC
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
| DEC-009 | Council Feedback Summary | ✅ Accepted | Supervisor summarizes to prevent bloat |
| DEC-010 | Single Source of Truth | ✅ Accepted | CURRENT_STATE.md for context |

Full details: `.context/DECISION_LOG.md`

---

# CURRENT ISSUES

| Issue | Status | Notes |
|-------|--------|-------|
| Kimi subscription ending | 🟡 Soon | End of month, plan migration |
| GCE cost | 🟡 Monitoring | Consider Hetzner/other |

---

# Source of Truth Index

## Changelog (Audit Trail)

| File | Purpose | Update Frequency |
|------|---------|------------------|
| `CHANGELOG.md` | Full audit trail of all changes | After EVERY change |

**Contains:** What changed, when, why, commit hash, rollback command

**Includes:** Files added/changed/removed, merges, branch deletions, timestamps

## Supabase (All State)

| Table | Purpose | Key Fields | Update Frequency |
|-------|---------|------------|------------------|
| `tasks` | Task queue + state | id, status, priority, dependencies, project_id | Constant |
| `task_packets` | Versioned prompts | task_id, prompt, tech_spec, version | On task create/update |
| `models` | In-house model registry | id, platform, status, limits, usage | On model change |
| `platforms` | Web AI platforms | id, type, capabilities, limits, status | On platform change |
| `projects` | Multi-project tracking | id, name, status, roi_cumulative | On project change |
| `task_runs` | Execution history | task_id, model_id, platform, tokens, duration | Every execution |

**Connection:** Via `.env` → `SUPABASE_URL`, `SUPABASE_KEY`

## GitHub (All Code + Docs)

| Path | Purpose | Update When |
|------|---------|-------------|
| `CURRENT_STATE.md` | Context restoration | Every session |
| `config/vibepilot.yaml` | Runtime config | Config change |
| `.context/DECISION_LOG.md` | Full decision details | New decision |
| `.context/guardrails.md` | Pre-code gates | Rare |
| `docs/MASTER_PLAN.md` | Full specification | Architecture change |
| `core/roles.py` | Role definitions | Role change |
| `runners/kimi_runner.py` | Kimi CLI integration | Runner change |
| `dual_orchestrator.py` | Multi-model routing | Orchestrator change |
| `task_manager.py` | Task CRUD | Task logic change |
| `agents/` | Agent implementations | Agent change |

**Repo:** `git@github.com:VibesTribe/VibePilot.git`

## GCE/Terminal (Ephemeral Only)

| Path | Purpose | Persistent? | Update When |
|------|---------|-------------|-------------|
| `~/vibepilot/` | Main project | No (git clone) | After git pull |
| `~/vibepilot/venv/` | Python venv | No | After setup.sh |
| `~/vibepilot/.env` | Secrets | No (recreate) | On new machine |
| `~/.local/bin/kimi` | Kimi CLI | No | After install |

**Server:** `ssh mjlockboxsocial@vibestribe`
**Important:** Nothing on GCE is source of truth. Everything is in Supabase or GitHub.

---

# DIRECTORY INDEX

```
~/vibepilot/                      # Project root (git clone)
│
├── CURRENT_STATE.md              # THIS FILE - context restoration
├── CHANGELOG.md                  # FULL AUDIT TRAIL - every change with timestamps
├── STATUS.md                     # Quick status (deprecated, use CURRENT_STATE)
├── .env                          # Secrets (NOT in git)
├── .env.example                  # Secret template (in git)
├── .gitignore                    # Ignore rules
├── requirements.txt              # Python deps
├── setup.sh                      # One-command setup (TODO: create)
│
├── config/
│   └── vibepilot.yaml            # ALL runtime config (models, roles, prompts, thresholds)
│
├── .context/                     # Strategic documentation
│   ├── guardrails.md             # Pre-code gates, P-R-E-V-C
│   ├── DECISION_LOG.md           # Full decision details
│   ├── agent_protocol.md         # Agent coordination rules
│   ├── quick_reference.md        # Cheat sheet
│   └── ops_handbook.md           # Disaster recovery
│
├── docs/                         # Documentation
│   ├── MASTER_PLAN.md            # Full system specification
│   ├── SESSION_LOG.md            # Session history
│   ├── prd_v1.3.md               # Product requirements
│   ├── architecture_diagram.md   # Visual architecture
│   ├── voice_interface.md        # Voice interface design
│   ├── vibeflow_adoption.md      # Patterns from Vibeflow
│   ├── schema_*.sql              # Database schemas
│   └── scripts/                  # Utility scripts
│       ├── pipeline_test.py
│       └── kimi_dispatch_demo.py
│
├── core/                         # Core logic
│   ├── roles.py                  # Role definitions
│   └── __init__.py
│
├── runners/                      # Model runners
│   ├── kimi_runner.py            # Kimi CLI integration
│   └── __init__.py
│
├── agents/                       # Agent implementations
│   ├── base.py                   # Base agent class
│   ├── code_hand.py              # Coding agent
│   ├── executioner.py            # Execution agent
│   ├── council/                  # Council agents
│   │   ├── security.py
│   │   └── maintenance.py
│   └── __init__.py
│
├── scripts/                      # Utility scripts
│   └── prep_migration.sh         # Migration prep
│
├── archive/                      # Old/unused files
│   ├── agents.md
│   └── dashboard.py
│
├── venv/                         # Python venv (IGNORED)
├── __pycache__/                  # Python cache (IGNORED)
└── *.log                         # Logs (IGNORED)
```

**Keep Updated:**
- `CURRENT_STATE.md` - Every session
- `CHANGELOG.md` - After EVERY change (add, update, remove, merge)
- `config/vibepilot.yaml` - Every config change
- `.context/DECISION_LOG.md` - Every decision

**Never Edit (Generated/Cache):**
- `venv/`, `__pycache__/`, `*.log`

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

See DIRECTORY INDEX above for full structure. Key files:

| File | Purpose |
|------|---------|
| `CURRENT_STATE.md` | Context restoration (this file) |
| `CHANGELOG.md` | Full audit trail (every change, timestamps, rollback) |
| `config/vibepilot.yaml` | ALL runtime config |
| `.context/DECISION_LOG.md` | Full decision details |
| `docs/MASTER_PLAN.md` | Full specification |

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
6. **Update CHANGELOG.md with all changes made**

**CHANGELOG.md must be updated after EVERY change:**
```markdown
## HH:MM UTC - <Agent/Human>
**Commit:** `<hash>`
**Type:** Add | Update | Remove | Merge
**Files Added/Changed/Removed:** ...
**Why:** ...
**Rollback:** `git revert <hash>`
```

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

*Token count target: <4000 tokens (this doc)*
*Actual: ~3500 tokens*
*Context restoration: ONE file read*
