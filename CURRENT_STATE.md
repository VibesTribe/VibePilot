# VibePilot Current State

**Required reading: FIVE files**
1. **THIS FILE** (`CURRENT_STATE.md`) - What, where, how, current state
2. **`docs/SESSION_34_HANDOFF.md`** - Latest session details and next priorities
3. **`docs/SECURITY_BOOTSTRAP.md`** - How credentials work (READ THIS FIRST)
4. **`docs/GOVERNOR_HANDOFF.md`** - Full implementation details
5. **`docs/core_philosophy.md`** - Strategic mindset and principles

**Read all five → Know everything → Do anything**

---

**Last Updated:** 2026-02-27
**Updated By:** GLM-5 - Session 34
**Branch:** `main`
**Status:** ACTIVE - Event persistence, usage tracking, startup recovery implemented

---

# CURRENT ARCHITECTURE

## What's Running

```
vibepilot-governor.service (Go binary)
├── Polls Supabase every 1s
├── Max 8 concurrent per module, 160 total
├── OpenCode limit: 5 concurrent
├── Reads secrets from vault at runtime
├── Startup recovery: finds and recovers orphaned sessions
└── Usage tracking: multi-window rate limit enforcement
```

**Status:** `systemctl status vibepilot-governor`

## Codebase Structure (Clean)

```
vibepilot/
├── governor/              # ACTIVE - Go binary (everything)
│   ├── cmd/governor/      # Main entry point + startup recovery
│   ├── internal/
│   │   ├── db/            # Supabase client + RPC allowlist
│   │   ├── vault/         # Secret decryption
│   │   ├── runtime/       # Events, sessions, usage_tracker, model_loader
│   │   ├── destinations/  # CLI runners (opencode)
│   │   └── tools/         # Agent tools
│   └── config/            # JSON configs
├── config/                # JSON configs (models, agents, etc.)
├── prompts/               # Agent behavior definitions (.md)
├── docs/                  # Documentation
├── scripts/               # Deploy scripts
├── legacy/                # DEAD CODE - kept for reference
│   └── python/            # All Python moved here
└── .env                   # EMPTY (keys in systemd override)
```

## What's NOT Running

| Item | Status |
|------|--------|
| Python orchestrator | Disabled, moved to legacy/ |
| `.env` with keys | Empty, keys removed |
| venv/ | Moved to legacy/ |
| All `.py` files | Moved to legacy/ |

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

## Session 34 Changes (Current)

### 1. Bug Fix: "signal: terminated" 
- **Root cause:** `cleanup_zombies.sh` killed governor children
- **Fix:** Script now checks cgroup membership before killing
- **File:** `scripts/cleanup_zombies.sh`

### 2. Event Persistence & Recovery
- **New tables:** `event_checkpoints`, `runner_sessions`, `event_queue`, `system_config`
- **Schema:** `docs/supabase-schema/032_event_persistence.sql`
- **Startup recovery:** Governor finds and recovers orphaned sessions
- **RPCs added:** 8 new functions for recovery operations

### 3. Usage Tracking System
- **Multi-window tracking:** minute/hour/day/week
- **Buffer percentage:** 80% default (configurable per model)
- **Auto-calculated spacing:** Based on rate limits
- **Cooldown countdown:** Per-model configurable
- **Files:** `governor/internal/runtime/usage_tracker.go`

### 4. Model Profiles
- **Full rate limit profiles:** Per-model in `models.json`
- **API pricing:** For theoretical cost calculation
- **Per-model recovery config:** Cooldown, timeout, thresholds
- **Learned data:** best_for_task_types, failure_rates (schema ready)
- **Files:** `governor/config/models.json`, `model_loader.go`

### 5. Config Improvements
- **session.go:** Reads timeout/maxTurns from config
- **events.go:** Configurable query limits
- **runners.go:** CLI args configurable via destinations.json
- **system.json:** Added `recovery` and `defaults` sections

### 6. GCE Cleanup
- **Removed:** OpenClaw, Docker, Playwright, Python caches
- **Saved:** ~3GB disk, ~330MB RAM
- **Verified:** No orphaned terminals

---

## What's Configurable (No Hardcoded Values)

| Setting | Config File | Field |
|---------|-------------|-------|
| Orphan threshold | system.json | `recovery.orphan_threshold_seconds` |
| Max task attempts | system.json | `recovery.max_task_attempts` |
| Model failure threshold | system.json | `recovery.model_failure_threshold` |
| Buffer percentage | models.json | Each model's `buffer_pct` |
| Request spacing | models.json | Each model's `spacing_min_seconds` |
| Cooldown duration | models.json | Each model's `recovery.cooldown_minutes` |
| Rate limits | models.json | Each model's `rate_limits.*` |
| Concurrency limits | system.json | `concurrency.limits.*` |
| Event query limit | system.json | `runtime.event_query_limit` |

---

## Remaining Gaps (Priority Order)

### CRITICAL - Blocks Real Work

| Gap | Description | Effort |
|-----|-------------|--------|
| **Tool protocol** | OpenCode ignores `TOOL:` format, can't do db operations reliably | High |

### IMPORTANT - Learning Loop Not Connected

| Gap | Description | Effort |
|-----|-------------|--------|
| **record_model_success/failure** | RPCs exist, nothing calls them after task completion | Medium |
| **Orchestrator feedback** | No automatic feedback from completed tasks to routing decisions | Medium |

### FUTURE - Scale Optimization

| Gap | Description | Effort |
|-----|-------------|--------|
| Queue-based execution | Priority/weighting for 50+ concurrent | Medium |
| Multi-host distribution | Single point of failure currently | High |
| Observability | Prometheus/Grafana for metrics | Medium |

---

## For Next Session

**Priority 1: Tool Protocol**
- How should OpenCode do database operations?
- Options: TOOL: format adapter, OpenCode tool server, different protocol

**Priority 2: Wire Learning Loop**
- After task completion → call `record_model_success` or `record_model_failure`
- Supervisor or governor code needs to call these RPCs
- Orchestrator reads `models.learned` for routing decisions

**Priority 3: Test End-to-End**
- Run a full task through the system
- Verify recovery works on crash
- Verify learning accumulates

---

## What NOT to Do

- Don't look for keys in `.env` (it's empty)
- Don't use Python code (it's in legacy/)
- Don't hardcode keys anywhere
- Don't modify cleanup script without understanding cgroup logic
- Don't hardcode any defaults - everything in config files
