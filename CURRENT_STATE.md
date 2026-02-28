# VibePilot Current State

**Required reading: SIX files**
1. **THIS FILE** (`CURRENT_STATE.md`) - What, where, how, current state
2. **`docs/vibepilot_process.md`** - COMPLETE process flow, all roles, failure handling, learning
3. **`docs/learning_system.md`** - Learning system design, review flow, thresholds
4. **`docs/SYSTEM_REFERENCE.md`** - What we have and how it works
5. **`docs/SESSION_35_HANDOFF.md`** - Dynamic routing implementation details
6. **`docs/core_philosophy.md`** - Strategic mindset and principles

**Read all five → Know everything → Do anything**

---

**Last Updated:** 2026-02-28
**Updated By:** GLM-5 - Session 36
**Branch:** `main`
**Status:** ACTIVE - Dynamic routing committed, documentation updated, branch creation next

---

# CURRENT ARCHITECTURE

## What's Running

```
vibepilot-governor.service (Go binary)
├── Polls Supabase every 1s
├── Max 8 concurrent per module, 160 total
├── Dynamic routing via config (NO hardcoded destinations)
├── Reads secrets from vault at runtime
├── Startup recovery: finds and recovers orphaned sessions
├── Usage tracking: multi-window rate limit enforcement
└── Learning: records model success/failure after task completion
```

**Status:** `systemctl status vibepilot-governor`

## Architecture Principle

```
Model = Intelligence (thinks, outputs)
Transport/CLI = Provides tools natively (read/write/bash)
Destination = Where/how access happens (has capabilities)
Agent = Role with capabilities needed (for routing)
Prompt packet = Task + expected output format

Routing = config/routing.json (strategies, restrictions, categories)
Destinations = config/destinations.json (status, type, provides_tools)
Models = config/models.json (availability, access_via)

NO HARDCODED DESTINATIONS. Everything configurable.
```

## Dynamic Routing

```
Event fires
    ↓
selectDestination(agentID, taskID, taskType)
    ↓
Get strategy for agent (internal_only for governance agents)
    ↓
Get priority order from routing.json
    ↓
For each category: find active destination
    ↓
Return destination ID or "" if none available
```

**Internal agents (planner, supervisor, council, etc.)** → internal_only strategy → never external
**Task execution** → default strategy → external first, then internal

## Codebase Structure (Clean)

```
vibepilot/
├── governor/              # ACTIVE - Go binary (everything)
│   ├── cmd/governor/      # Main entry point + event handlers + routing
│   ├── internal/
│   │   ├── db/            # Supabase client + RPC allowlist
│   │   ├── vault/         # Secret decryption
│   │   ├── runtime/       # Events, sessions, router, usage_tracker
│   │   ├── destinations/  # CLI/API runners
│   │   └── tools/         # Tool registry
│   └── config/            # JSON configs (routing.json, destinations.json, etc.)
├── prompts/               # Agent behavior definitions (.md)
├── docs/                  # Documentation
├── scripts/               # Deploy scripts
└── legacy/                # DEAD CODE - kept for reference
```

---

## Bootstrap Keys (Secure)

| Key | Where It Lives | Who Can Read |
|-----|----------------|--------------|
| `SUPABASE_URL` | `/etc/systemd/.../override.conf` | root only |
| `SUPABASE_SERVICE_KEY` | `/etc/systemd/.../override.conf` | root only |
| `VAULT_KEY` | `/etc/systemd/.../override.conf` | root only |

**All other secrets** → Encrypted in Supabase `secrets_vault` table

**Read `docs/SECURITY_BOOTSTRAP.md` before touching credentials.**

---

## Quick Commands

| Command | Action |
|---------|--------|
| `systemctl status vibepilot-governor` | Check if running |
| `journalctl -u vibepilot-governor -f` | Live logs |
| `sudo systemctl restart vibepilot-governor` | Restart |
| `cd ~/vibepilot/governor && go build -o governor ./cmd/governor` | Build |
| `sudo scripts/deploy-governor.sh` | Full deploy |

---

## Session Progress

### DONE - Session 35
- ✅ Dynamic routing (router.go, routing.json)
- ✅ Python moved to legacy/
- ✅ TOOL: format removed

### DONE - Session 36
- ✅ Full documentation update (vibepilot_process.md)
- ✅ Failure handling flow documented
- ✅ Learning system documented (docs/learning_system.md)
- ✅ Branch creation on orchestrator assignment
- ✅ Research branch created for review docs
- ✅ "What I've Learned" sections added to agent prompts
- ✅ Supervisor decision matrix (simple/complex/human)
- ✅ Learning system documented
- ✅ System Researcher flow documented
- ✅ Branch lifecycle documented
- ✅ Courier vs Internal clarified
- ✅ Session 35 changes committed and pushed

### NEXT - Session 37+

| Priority | Task | Notes |
|----------|------|-------|
| **HIGH** | Branch creation on assignment | gitree.CreateBranch("task/T001") when Orchestrator assigns |
| **HIGH** | Wire model scoring RPC | get_model_score_for_task in Supabase |
| MEDIUM | Rate limit checking | Router checks destination limits |
| MEDIUM | API output execution | Governor parses and executes for API runners |
| LOW | Courier runner | Web platform execution implementation |
| LATER | Learning system implementation | Store/retrieve learned scores, pattern detection |

---

---

## For Next Session

**Priority 1: Branch Creation on Assignment**
- When Orchestrator assigns task, call gitree.CreateBranch("task/T001")
- Branch naming: task/{task_number} (simple, human-readable)
- Happens BEFORE runner executes
- Location: main.go EventTaskAvailable handler

**Priority 2: Wire Model Scoring RPC**
- Create get_model_score_for_task RPC in Supabase
- Router uses it to pick best model for task type
- Learning data actually used for routing

**Priority 3: Learning System Design**
- Discuss with human before implementing
- Data storage: task_runs, model_scores, failure_patterns
- All agents learn from outcomes

---

## What NOT to Do

- Don't look for keys in `.env` (it's empty)
- Don't use Python code (it's in legacy/)
- Don't hardcode keys anywhere
- Don't hardcode destination IDs in code
- Don't hardcode routing logic in code
- Don't add TOOL: format back - it's gone for good
- Don't modify cleanup script without understanding cgroup logic
- Don't hardcode any defaults - everything in config files
- Don't implement learning system without discussing with human first
