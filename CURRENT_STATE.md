# VibePilot Current State

**Required reading: FIVE files**
1. **THIS FILE** (`CURRENT_STATE.md`) - What, where, how, current state
2. **`docs/WHAT_WHERE.md`** - Where everything is located
3. **`docs/prd_v1.4.md`** - Complete system specification
4. **`docs/core_philosophy.md`** - Strategic mindset and inviolable principles
5. **`CHANGELOG.md`** - History, changes, rollback commands

**Read all five → Know everything → Do anything**

---

**Last Updated:** 2026-02-18 22:00 UTC
**Updated By:** GLM-5 (Session 14: Foundation redesign)
**Session Focus:** Fixing the entire foundation - data model, pipeline flow, orchestrator

**Schema Location:** `docs/supabase-schema/` (all SQL files)
**Progress:** Data model redesign COMPLETE, orchestrator uses new schema, rate limits tracked

---

## SQL TO RUN

Run these in Supabase SQL Editor (in order):
1. ✅ `001_data_model_redesign.sql` - Creates new tables
2. ✅ `002_rls_fix.sql` - Disables RLS on new tables
3. ✅ `003_tools_columns_fix.sql` - Adds columns to tools table
4. ⏳ `004_usage_rpc.sql` - Creates usage tracking functions (NEEDS TO RUN)

---

# WHAT IS VIBEPILOT

Sovereign AI execution engine. Human provides idea → VibePilot executes with zero drift.

**Core Principles (see docs/core_philosophy.md):**
- Zero vendor lock-in - everything swappable
- Modular & swappable - change one thing, nothing else breaks
- Exit ready - pack up, hand over to anyone
- Reversible - if it can't be undone, it can't be done
- Always improving - new ideas evaluated daily

**The Vision:**
```
User → "Hey Vibes, I want feature X" → Vibes triggers pipeline
                                              ↓
                              Consultant → PRD → Planner → Tasks
                                              ↓
                              Supervisor reviews → Council vets → Approves
                                              ↓
                              Tasks become "available" → Orchestrator picks up
                                              ↓
                              Routes to best available runner → Executes → Learns
```

**Vibes** = The conversational interface. User talks to Vibes like talking to me now. Vibes IS the system.

---

# CURRENT SESSION (14) - FOUNDATION REDESIGN

## What We Discovered

### The Real Problem (Type 1 Error)

The `models` table conflates everything:
- AI models (kimi-k2.5, glm-5, deepseek-chat)
- Tools/interfaces (opencode, kimi-cli)
- Access methods (api, subscription, web_free_tier)
- Rate limits (single values, no windows)
- Usage tracking (broken)

**Result:** Orchestrator cannot answer "What models can I use right now?"

### Pipeline Flow Broken

- Planner writes tasks as `pending`
- Orchestrator looks for `available`
- **Nothing transitions between them**
- Supervisor exists but never triggered
- Council never called

### Orchestrator Not a Service

- Requires manual `orch.start()` call
- Should be always-on, watching queue
- Previous session bypassed instead of fixing

### No Multi-Window Rate Limits

- We killed Gemini API in 60 seconds
- No RPM (requests/minute) tracking
- No rolling window tracking
- Can't respect 80% threshold

## What We Fixed (Committed)

| Fix | File | Commit |
|-----|------|--------|
| Increment attempts on failure | `task_manager.py` | `24960cdc` |
| Check attempts before dispatch | `core/orchestrator.py` | `24960cdc` |
| Clear assigned_to on return to queue | `task_manager.py` | `24960cdc` |
| CSS By Model cut-off fix | `vibeflow/styles.css` | `ea034ef9` (branch) |

## What We Documented

| Document | Purpose |
|----------|---------|
| `docs/DATA_MODEL_REDESIGN.md` | Full schema redesign plan |
| `docs/rate_limits/gemini_api_free_tier.json` | Gemini limits researched by Kimi |
| `docs/supabase-schema/001_data_model_redesign.sql` | NEW - SQL for new tables |
| `scripts/populate_new_schema.py` | Script to populate tables from config |

## What We Cleaned Up

- Benched ghost models not in RUNNER_REGISTRY
- Unbenched kimi-k2.5 (the actual Kimi model)
- Updated AGENTS.md: user constraints (no click/copy/paste), OpenCode context
- Consolidated schema files: removed duplicate `supabase/migrations/`, kept `docs/supabase-schema/`

---

# ACTIVE MODELS (Current State)

| Model ID | Status | Access Via | Notes |
|----------|--------|------------|-------|
| kimi-cli | active | kimi-cli (subscription) | 7 days left at $0.99 |
| kimi-internal | active | kimi-cli | Same as above |
| kimi-k2.5 | active | kimi-cli | Unbenched - the actual Kimi model |
| gemini-api | paused | API | quota_exhausted |
| gemini-2.0-flash | paused | API | quota_exhausted |
| deepseek-chat | paused | API | credit_needed |
| gpt-4o, gpt-4o-mini | benched | N/A | Web platform only, no API key |
| claude-sonnet-4-5, claude-haiku-4-5 | benched | N/A | Web platform only, no API key |
| opencode, glm-5 | benched | N/A | Tool, not a model |

---

# NEXT STEPS (In Order)

## 1. Data Model Redesign (COMPLETE)

**Status:** Tables created, populated, verified

**New Tables:**
- `models_new`: 10 models (AI capabilities only)
- `tools`: 4 tools (opencode, kimi-cli, direct-api, courier)
- `access`: 15 access records (links models to tools with limits, usage)
- `task_history`: Ready for learning data

**Active Access (by priority):**
| Priority | Model | Tool | Method |
|----------|-------|------|--------|
| 0 | kimi-internal | kimi-cli | subscription |
| 0 | glm-5 | opencode | subscription |
| 1 | gpt-4o, claude-* | courier | web_free_tier |
| paused | gemini-*, deepseek-* | direct-api | api |

**Next:** Update orchestrator to query new `access` table for routing decisions

## 2. Update Orchestrator (In Progress)

**Status:** RunnerPool loads from new access table

**Done:**
- `RunnerPool._load_from_database()` queries `access` table with joins
- Runners keyed as `model_id:tool_id` (e.g., `kimi-internal:kimi-cli`)
- Loads rate limits, capabilities, priority from new schema

**Current runners (from new schema):**
| Priority | Runner Key | Method | Routing |
|----------|------------|--------|---------|
| 0 | kimi-internal:kimi-cli | subscription | internal, web, mcp |
| 0 | glm-5:opencode | subscription | internal, mcp |
| 1 | gpt-4o:courier | web_free_tier | web |
| 1 | claude-sonnet-4-5:courier | web_free_tier | web |
| 1 | claude-haiku-4-5:courier | web_free_tier | web |
| 1 | gpt-4o-mini:courier | web_free_tier | web |

**Next:**
- Add rate limit checking before dispatch (check 80% threshold)
- Add usage tracking after task completion (update access table)
- Test end-to-end task dispatch with new schema

## 3. Pipeline Auto-Flow (SIGNIFICANT GAP)

**Status:** Major implementation gap identified

**What PRD says should happen:**
```
Planner creates plan → Council reviews → Supervisor approves → Tasks become "available"
```

**What currently happens:**
```
Planner creates plan → Tasks written as "pending" → NOTHING transitions them → BLOCKED
```

**What's missing:**
1. **Pre-execution plan review** - Supervisor should vet plans before tasks go to available
2. **Council coordination** - Supervisor calls Council for plan review
3. **Approval trigger** - After Council + Supervisor approve, tasks become "available"

**Implementation needed:**
- `supervisor.review_plan()` - Reviews plan before execution
- `supervisor.call_council()` - Coordinates Council review
- `supervisor.approve_plan()` - Transitions tasks from "pending" to "available"
- Trigger after planner completes

**Status flow per PRD:**
```
pending → available → in_progress → review → testing → approval → merged
              ↑            │          │         │          │
              └────────────┴──────────┴─────────┴──────────┘
                           (loops back on failure)
```

Current gap: `pending → available` transition not implemented.

## 4. Orchestrator as Service (Pending)

**Status:** Not yet implemented

- Run continuously, not manual start
- Watch queue, dispatch, learn
- Maybe systemd service or background process

## 5. Rate Limit Data Collected

**DeepSeek API:** `docs/rate_limits/deepseek_api.json`
- No fixed rate limits (dynamic throttling)
- 5M free tokens for new users
- Pricing: $0.28/1M input (cache miss), $0.42/1M output

**Kimi/Moonshot API:** `docs/rate_limits/kimi_moonshot_api.json`
- Tier-based limits ($1 min to activate)
- Tier0: 3 RPM, 500K TPM, 1.5M TPD
- Kimi K2.5: 256K context, multimodal

**Gemini API:** `docs/rate_limits/gemini_api_free_tier.json`
- gemini-2.5-flash: 10 RPM, 250 RPD, 250K TPM
- Daily reset at midnight PT

## 3. Orchestrator as Service

- Run continuously, not manual start
- Watch queue, dispatch, learn
- Maybe systemd service or background process

## 4. Rate Limit Tracking

- Multi-window (RPM, RPD, TPM, TPD)
- Rolling windows
- 80% threshold enforcement
- Cooldown periods

---

# GEMINI API FREE TIER LIMITS

Researched by Kimi, stored in `docs/rate_limits/gemini_api_free_tier.json`

| Model | RPM | RPD | TPM | TPD |
|-------|-----|-----|-----|-----|
| gemini-2.5-pro | 5 | 100 | 250K | - |
| gemini-2.5-flash | 10 | 250 | 250K | - |
| gemini-2.5-flash-lite | 15 | 1000 | 250K | - |
| gemini-1.5-flash | 15 | 1500 | 1M | - |
| gemini-1.5-pro | 2 | 50 | 32K | - |

**Reset:** Daily at Midnight PT (08:00 UTC)

**80% Safe Limits:**
| Model | Safe RPM | Safe RPD |
|-------|----------|----------|
| gemini-2.5-flash | 8 | 200 |
| gemini-2.5-flash-lite | 12 | 800 |
| gemini-1.5-flash | 12 | 1200 |

---

# VIBEFLOW DASHBOARD

**Live Dashboard:** https://vibeflow-dashboard.vercel.app/
**GitHub Repo:** https://github.com/VibesTribe/vibeflow

**Git Rules:**
- Dashboard/UI → Feature branch, human approves → merge
- Backend → Less risky, can go direct
- **Never push dashboard to main without human approval**

---

# QUICK COMMANDS

| Command | Action |
|---------|--------|
| `cat docs/DATA_MODEL_REDESIGN.md` | Full schema redesign plan |
| `cat docs/rate_limits/gemini_api_free_tier.json` | Gemini limits |
| `git log --oneline -5` | Recent commits |
| `kimi --yolo --prompt "..."` | Send task to Kimi for research |
| `cd ~/vibepilot && git checkout main && git pull` | Get latest |

---

# KIMI USAGE PRIORITY

**Kimi subscription: 7 days left at $0.99, then $19/mo**

Use Kimi for:
- Research tasks (web access)
- Parallel sub-agent tasks (up to 100)
- Any task requiring browser/vision/multimodal

Maximize usage before renewal decision.
