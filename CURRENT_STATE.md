# VibePilot Current State

**Required reading: FIVE files**
1. **THIS FILE** (`CURRENT_STATE.md`) - What, where, how, current state
2. **`docs/SESSION_35_HANDOFF.md`** - Latest session: dynamic routing implementation
3. **`docs/SYSTEM_REFERENCE.md`** - What we have and how it works
4. **`docs/SECURITY_BOOTSTRAP.md`** - How credentials work
5. **`docs/core_philosophy.md`** - Strategic mindset and principles

**Read all five → Know everything → Do anything**

---

**Last Updated:** 2026-02-28
**Updated By:** GLM-5 - Session 35
**Branch:** `main`
**Status:** ACTIVE - Dynamic routing, TOOL: removed, learning wired

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

## Session 35 Changes (Current)

### 1. Removed TOOL: Format
- **Problem:** Rigid string parsing, fragile, OpenCode ignores it
- **Solution:** Models output in expected format, Governor handles execution
- **Files:** session.go (simplified), tools.go (removed parsing)
- **Lines removed:** ~100 lines of TOOL: parsing code

### 2. Destination Capabilities
- **New field:** `provides_tools` in destinations.json
- **CLI destinations:** Provide read/write/bash/webfetch natively
- **API destinations:** Provide nothing (Governor executes)
- **Methods:** `HasNativeTools()`, `ProvidesTool()`

### 3. Agent Capabilities
- **Renamed:** `tools` → `capabilities` in agents.json
- **Purpose:** Defines what agent NEEDS for routing decisions
- **Method:** `AgentConfig.HasCapability()`

### 4. Learning Loop Wired
- **Functions:** `recordModelSuccess()`, `recordModelFailure()`
- **Location:** main.go event handlers
- **Trigger:** After supervisor approves + tests pass
- **Tracks:** model_id, task_type, duration_seconds

### 5. Prompts Updated
- **planner.md:** Removed TOOL: references, defines output format
- **supervisor.md:** Removed TOOL: references, describes actions
- **Principle:** Prompt = behavior + output format, NOT tool calls

---

## What's Configurable (No Hardcoded Values)

| Setting | Config File | Field |
|---------|-------------|-------|
| Routing strategies | routing.json | `strategies.*.priority` |
| Agent restrictions | routing.json | `agent_restrictions` |
| Destination categories | routing.json | `destination_categories` |
| Destination capabilities | destinations.json | `provides_tools` |
| Agent capabilities | agents.json | `capabilities` |
| Destination status | destinations.json | `status` (active/inactive) |
| Model availability | models.json | `status`, `access_via` |
| Orphan threshold | system.json | `recovery.orphan_threshold_seconds` |
| Max task attempts | system.json | `recovery.max_task_attempts` |
| Buffer percentage | models.json | Each model's `buffer_pct` |
| Rate limits | models.json | Each model's `rate_limits.*` |
| Concurrency limits | system.json | `concurrency.limits.*` |

---

## Remaining Gaps (Priority Order)

### DONE - Session 35

| Task | Status |
|------|--------|
| Dynamic routing | ✅ Deployed and verified |
| Python cleanup | ✅ Moved to legacy/ |
| TOOL: format removal | ✅ Complete |

### NEXT - Future Sessions

| Priority | Task | Notes |
|----------|------|-------|
| HIGH | Add courier destinations | Web platforms to destinations.json (type: "web") |
| HIGH | Wire model scoring RPC | get_model_score_for_task in Supabase |
| MEDIUM | Rate limit checking | Router checks destination limits |
| MEDIUM | API output execution | Governor parses and executes for API runners |
| LOW | Courier runner | Web platform execution implementation |

### Verified Working

```bash
# Check routing in action
sudo journalctl -u vibepilot-governor -f | grep Router
```

Expected output:
```
[Router] Agent planner using strategy internal_only with priority [internal]
[Router] Selected destination opencode (category: internal, model: glm-5)
```

---

## For Next Session

**Priority 1: Add Web Destinations**
- Add chatgpt-web, claude-web, gemini-web to destinations.json
- Set type="web", status="active" or "inactive"
- Router will pick them for task execution (default strategy: external first)

**Priority 2: Wire Model Scoring**
- Create get_model_score_for_task RPC in Supabase
- Router uses it to pick best model for task type
- Learning data actually used for routing

**Priority 3: Test Full Flow**
- Create a real PRD
- Watch it flow through: PRD → Planner → Supervisor → Tasks → Execution
- Verify routing logs show correct destinations

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
