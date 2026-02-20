# VibePilot Current State

**Required reading: FIVE files**
1. **THIS FILE** (`CURRENT_STATE.md`) - What, where, how, current state
2. **`docs/WHAT_WHERE.md`** - Where everything is located
3. **`docs/prd_v1.4.md`** - Complete system specification
4. **`docs/core_philosophy.md`** - Strategic mindset and inviolable principles
5. **`CHANGELOG.md`** - History, changes, rollback commands

**Read all five → Know everything → Do anything**

---

**Last Updated:** 2026-02-20 18:00 UTC
**Updated By:** GLM-5 + Kimi (Session 19: Real-time Communication + Session Persistence)
**Session Focus:** Fixed terminal crash root cause, implemented real-time GLM-Kimi communication via Supabase

**Schema Location:** `docs/supabase-schema/` (all SQL files)
**Progress:** Dependencies migrated to JSONB, all 5 RPC functions working, task flow operational, real-time agent messaging

---

# SESSION 19 SUMMARY (2026-02-20)

## What We Fixed

### 1. Terminal Crash Root Cause ✅ (Kimi)
- **Problem:** 16 zombie opencode processes consuming 3-4GB RAM, swap at 90%
- **Fix:** Killed zombies, freed 2.6GB RAM, swap now at 15%
- **Prevention:** Hourly auto-cleanup via cron, tmux for persistent sessions

### 2. Real-Time Agent Communication ✅ (GLM-5)
- Updated `start_session.sh` to check Supabase messages PRIMARY
- Created `scripts/notify_done.sh` for task completion alerts
- Both agents now coordinate via `agent_messages` table, not files

### 3. Session Persistence ✅ (Kimi)
- `scripts/agent_sessions.sh` - tmux session manager
- `scripts/start_agent_session.sh` - start persistent sessions
- Sessions survive terminal crashes

## Commands for Real-Time Coordination
```bash
# Session start (checks Supabase messages)
./start_session.sh glm-5

# After completing work
./scripts/notify_done.sh glm-5 "Task description"

# Check messages anytime
python3 scripts/check_agent_mail.py glm-5

# Persistent sessions (tmux)
~/vibepilot/scripts/agent_sessions.sh status
~/vibepilot/scripts/agent_sessions.sh attach opencode
```

## Files Modified
- `start_session.sh` - Supabase messages PRIMARY
- `scripts/notify_done.sh` - Task completion notification
- `scripts/agent_sessions.sh` - tmux session manager (Kimi)
- `scripts/start_agent_session.sh` - session starter (Kimi)

---

# SESSION 18 SUMMARY (2026-02-20)

## What We Fixed

### 1. Command Queue RLS ✅
- Added `SUPABASE_SERVICE_KEY` to vault
- Updated `agents/maintenance.py` and `agents/supervisor.py` to use service key
- Fixed `claim_next_command` RPC to return `cmd_status` (was ambiguous with PL/pgSQL)

### 2. All Integration Tests Passing ✅
```
RESULTS: 8 passed, 0 failed
```

### 3. Orchestrator as Systemd Service ✅
- Installed `vibepilot-orchestrator.service`
- Enabled on boot
- Running and polling task queue

## Files Modified
- `agents/maintenance.py` - Service key support
- `agents/supervisor.py` - Service key support
- `tests/test_full_flow.py` - Service key support, cmd_status check
- `docs/supabase-schema/014_maintenance_commands.sql` - RPC return type note
- `docs/supabase-schema/015_fix_claim_rpc_return_status.sql` - Migration to fix RPC

---

# SESSION 15 SUMMARY (2026-02-18/19)

## What We Fixed

### 1. Dependencies Column: UUID[] → JSONB ✅

**Problem:** RPC functions expected JSONB but column was UUID[]. Functions crashed with operator errors.

**Solution:**
- Ran 13 SQL migrations (005-013)
- Migrated `dependencies` column to JSONB
- Fixed all 5 RPC functions

**Files Created:**
```
docs/supabase-schema/
├── 005_dependencies_jsonb.sql      - Main migration
├── 006_fix_dependencies_data.sql   - Data cleanup attempt
├── 007_fix_deps_v2.sql             - Another cleanup attempt
├── 008_fix_rpc_strip_quotes.sql    - RPC fix for double-quoted UUIDs
├── 009_fix_claim_next_task.sql     - Fix duplicate functions
├── 010_check_duplicates.sql        - Diagnostic query
├── 011_nuclear_claim_next_task.sql - Force drop all versions
├── 012_find_claim_signatures.sql   - Find exact signatures
└── 013_fix_claim_final.sql         - Final fix (3-arg + 4-arg drop)
```

### 2. All RPC Functions Working ✅

| Function | Status | Notes |
|----------|--------|-------|
| `check_dependencies_complete` | ✅ Working | Returns boolean |
| `unlock_dependent_tasks` | ✅ Working | Returns unlocked task IDs |
| `get_available_tasks` | ✅ Working | 8 rows returned |
| `claim_next_task` | ✅ Working | Claims task atomically |
| `get_available_for_routing` | ✅ Working | 7 rows returned |

### 3. Task Flow Operational ✅

```
pending → approve_plan() → locked (has deps) or available (no deps)
                              ↓
                    parent merges → unlock fires → available
                              ↓
                    claim_next_task → in_progress → review → merged
```

### 4. Dashboard Fixes ✅

- Token data cleaned (24K → 1.4K, removed hardcoded test values)
- CSS model line cutoff fixed
- ROI panel collapsible sections working

### 5. Agent Coordination ✅

- Created `AGENT_CHAT.md` for GLM-Kimi communication
- Created `inbox/` system for task delegation
- Kimi (research) and GLM-5 (code) roles defined
- Session tracking in `ACTIVE_SESSIONS.md`

---

## What We Created

| File | Purpose |
|------|---------|
| `run_orchestrator.py` | Service entry point for orchestrator |
| `scripts/vibepilot-orchestrator.service` | systemd unit file |
| `scripts/cleanup_task_runs.py` | Fix bad token data |
| `AGENT_CHAT.md` | GLM-Kimi communication channel |
| `inbox/README.md` | Inbox system documentation |
| `inbox/kimi/*.md` | Tasks for Kimi |
| `inbox/glm-5/*.md` | Tasks for GLM-5 |

---

## What's Left

| Priority | Task | Notes |
|----------|------|-------|
| 1 | ~~Orchestrator as systemd service~~ | ✅ DONE - Running, enabled on boot |
| 2 | Council implementation | Currently placeholder |
| 3 | Executioner connection | Tests don't run after review |
| 4 | First autonomous task flow | Ready to test |
| 5 | Data cleanup | Old test tasks still in DB |

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

# TASK STATUS FLOW

```
pending ──► approve_plan() ──┬──► available (no deps) ──► in_progress
                             │
                             └──► locked (has deps)
                                        │
                                        │ [parent task merges]
                                        │ [unlock_dependent_tasks RPC fires]
                                        ▼
                                   available ──► in_progress
```

**Full status lifecycle per PRD:**
```
pending → available → in_progress → review → testing → approved → merged
     ↑            │          │         │           │
     └────────────┴──────────┴─────────┴───────────┘
                  (loops back on failure)

Special states:
- locked: Awaiting dependencies
- escalated: Max attempts exceeded
- awaiting_human: Visual/manual testing needed
```

---

# NEXT STEPS (In Order)

## 1. Orchestrator as systemd Service ✅ DONE

**Status:** Installed, running, enabled on boot

**Commands:**
```bash
sudo systemctl status vibepilot-orchestrator  # Check status
sudo systemctl stop vibepilot-orchestrator    # Stop
sudo systemctl restart vibepilot-orchestrator # Restart
journalctl -u vibepilot-orchestrator -f       # View logs
```

## 2. First Autonomous Task Flow (READY TO TEST)

**Status:** Infrastructure complete, ready for first real task

**What to test:**
- Create a real task via dashboard
- Watch orchestrator pick it up
- Runner executes
- Supervisor reviews
- Maintenance commits

## 3. Full Council Implementation (NOT IMPLEMENTED)

**Status:** Simplified placeholder only

**Current:** `call_council()` does basic checks, auto-approves

**Needed:**
- 3 independent model reviews (different models, different hats)
- User Alignment hat
- Architecture hat
- Feasibility hat
- Iterative consensus (up to 4 rounds)
- Real voting mechanism

## 3. Executioner Connection (NOT IMPLEMENTED)

**Status:** Executioner agent exists but not wired

**Needed:**
- After supervisor review passes → route to Executioner
- Run tests → update task with results
- Pass/fail → appropriate status transition

## 4. Data Cleanup (PARTIAL)

**Token Issues:** ✅ Fixed - cleaned bad test data

**Tasks to Clean Up:**
- Old test tasks with status issues
- Duplicate task_runs from infinite retry bug (fixed now)
- Tasks stuck in invalid states

---

# AGENT COORDINATION

## GLM-5 + Kimi Communication

**Primary Channel:** `AGENT_CHAT.md` - Check at session start

**Inbox System:**
- `inbox/kimi/` - Tasks for Kimi (research)
- `inbox/glm-5/` - Tasks for GLM-5 (code)

**Branch Ownership:**
| Agent | Branch | Focus |
|-------|--------|-------|
| kimi | research-considerations | Research, docs, analysis |
| glm-5 | main | Core orchestration, infrastructure |

**Session Tracking:** `ACTIVE_SESSIONS.md`

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
| `cat CURRENT_STATE.md` | This file |
| `cat CHANGELOG.md` | Full history |
| `cat AGENT_CHAT.md` | GLM-Kimi chat |
| `./check_chat.sh` | Check for new messages |
| `git log --oneline -5` | Recent commits |
| `cd ~/vibepilot && git checkout main && git pull` | Get latest |

---

# KIMI USAGE PRIORITY

**Kimi subscription: 7 days left at $0.99, then $19/mo**

Use Kimi for:
- Research tasks (web access)
- Parallel sub-agent tasks (up to 100)
- Any task requiring browser/vision/multimodal

Maximize usage before renewal decision.
