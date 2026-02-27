# Session 33 Handoff Document

**Date:** 2026-02-27
**Agent:** GLM-5
**Duration:** ~6 hours
**Status:** BLOCKED - Debugging opencode "signal: terminated"

---

## WHAT WAS ACCOMPLISHED

### 1. Cleaned Up OpenCode Sessions
- Deleted 52 orphaned sessions (was causing issues)
- Opencode sessions now manageable

### 2. Fixed Security Bootstrap Architecture
- Reverted to `SUPABASE_SERVICE_KEY` (not anon key)
- Keys removed from `.env`, stored only in systemd override
- Created `docs/SECURITY_BOOTSTRAP.md` documenting architecture
- Created `scripts/setup-bootstrap.sh` and `scripts/deploy-governor.sh`

### 3. Merged Branches
- Merged `go-governor` into `main`
- Deleted OLD architecture (YAML config with hardcoded keys)
- Single clean codebase: 5,695 lines Go, 24 files
- Deleted `legacy/python/` contents moved there earlier

### 4. Audit Fixes
- **Vault:** Fixed machine salt (was hostname-based, now portable)
- **Web tools:** Made configurable via `system.json`
- **Provider detection:** Added `provider` field to `destinations.json`

### 5. Governor Deployed
- Running as systemd service `vibepilot-governor`
- Binary: `/home/mjlockboxsocial/vibepilot/governor/governor`
- Config: JSON files in `governor/config/`
- Keys: `/etc/systemd/system/vibepilot-governor.service.d/override.conf`

---

## CURRENT BLOCKER

### The Problem

When governor runs Planner via opencode, it gets "signal: terminated":

```
[EventPRDReady] Planner session failed for e81e1aad: llm call failed: opencode: signal: terminated
```

### What Works
- Direct opencode CLI calls work fine
- Supervisor agent worked earlier (processed plan, set status)
- Manual test of planner prompt works

### What Doesn't Work
- Planner agent called via governor fails immediately
- No retry mechanism (event lost forever)

### Possible Causes

1. **Concurrency issue**: 4 opencode processes already running
2. **Systemd environment**: Missing something opencode needs
3. **Resource limits**: systemd cgroup limits
4. **Tool architecture mismatch**: See below

---

## TOOL ARCHITECTURE ISSUE

### Two Tool Systems

| System | Who Uses It | How It Works |
|--------|-------------|--------------|
| **OpenCode built-in** | OpenCode CLI | `read`, `write`, `bash` tools executed by opencode directly |
| **Governor custom** | API runners | `TOOL: db_update {...}` parsed by governor, executed locally |

### The Mismatch

**Governor expects:**
1. Send prompt with "AVAILABLE TOOLS: db_query, db_update, file_read, file_write"
2. Runner outputs `TOOL: tool_name {...}`
3. Governor parses, executes tool, returns result
4. Multi-turn loop

**OpenCode does:**
1. Ignores TOOL: format instruction
2. Uses its built-in `read`, `write`, `bash` tools directly
3. Never outputs `TOOL: db_update {...}` (doesn't have it)
4. Governor tool layer bypassed

### The Question

How should OpenCode (with built-in tools) do database operations?
- It can't use `TOOL: db_update {...}` format (OpenCode ignores it)
- It can't use bash curl (would need API key in prompt - security issue)

**This is a design question, not a bug.**

---

## EVENT SYSTEM FRAGILITY

### Current Design
- Events tracked in-memory (`lastSeen` map)
- If event fails → lost forever
- No retry, no persistence, no dead letter queue

### What Happened
1. Plan reset to `draft` → EventPRDReady emitted
2. Planner failed with "signal: terminated"
3. Event marked as seen, no retry
4. Plan stuck in `draft` forever

### For 50 Parallel Agents
Need:
- Event queue table in database
- Retry with exponential backoff
- Stale state detection on governor startup

---

## DATABASE STATE

| Table | Records |
|-------|---------|
| plans | 2 (e81e1aad in draft, b90ebd8e in draft) |
| tasks | 1 (T004 - old test task) |
| secrets_vault | Unknown (RLS blocked anon key, service key works) |

---

## FILES CHANGED THIS SESSION

| File | Change |
|------|--------|
| `governor/config/system.json` | Added web_tools config, uses SERVICE_KEY |
| `governor/config/destinations.json` | Added provider field |
| `governor/internal/vault/vault.go` | Fixed salt, added docs |
| `governor/internal/runtime/config.go` | Added WebToolsConfig |
| `governor/internal/tools/web_tools.go` | Made configurable |
| `governor/internal/tools/registry.go` | Pass config to web tools |
| `governor/internal/destinations/runners.go` | Provider from config |
| `docs/SECURITY_BOOTSTRAP.md` | NEW - security architecture |
| `docs/USEFUL_COMMANDS.md` | Updated with migration guide |
| `scripts/setup-bootstrap.sh` | NEW - one-time key setup |
| `scripts/deploy-governor.sh` | NEW - deploy script |
| `.github/workflows/deploy-governor.yml` | NEW - CI/CD option |
| `CURRENT_STATE.md` | Updated |
| `.env` | Emptied (keys removed) |

---

## NEXT STEPS

### Immediate Debug
1. Kill existing opencode processes, retry planner
2. If works → concurrency issue
3. If fails → check systemd environment/limits

### Architecture Decision Needed
How should OpenCode do database operations?
- Option A: Force TOOL: format somehow
- Option B: Use bash tool with injected credentials
- Option C: Runner-specific tool adapters
- Option D: Something else

### Event System Improvement
- Add `event_queue` table
- Retry mechanism
- Stale state detection

---

## QUICK COMMANDS FOR NEXT SESSION

```bash
# Check governor status
systemctl status vibepilot-governor

# Watch governor logs
journalctl -u vibepilot-governor -f

# Check current opencode processes
ps aux | grep opencode | grep -v grep

# Kill all opencode (careful - kills your session too)
pkill -f opencode

# Test planner directly
cd ~/vibepilot
opencode run --format json "$(cat prompts/planner.md)

---

INPUT:
{\"event\": \"prd_ready\", \"plan\": {\"id\": \"e81e1aad-733e-49e4-8e56-d73784dac985\", \"prd_path\": \"docs/prds/test-full-flow.md\", \"status\": \"draft\"}}

AVAILABLE TOOLS:
- db_query
- db_update
- file_read
- file_write

Respond with your output."

# Reset test plan to draft
curl -s -X PATCH "https://qtpdzsinvifkgpxyxlaz.supabase.co/rest/v1/plans?id=eq.e81e1aad-733e-49e4-8e56-d73784dac985" \
  -H "apikey: <SERVICE_KEY>" \
  -H "Authorization: Bearer <SERVICE_KEY>" \
  -H "Content-Type: application/json" \
  -d '{"status": "draft", "complexity": null, "review_notes": null}'
```

---

## KEY FILES TO READ

| File | What It Explains |
|------|------------------|
| `docs/SECURITY_BOOTSTRAP.md` | How credentials work |
| `docs/GOVERNOR_HANDOFF.md` | Full implementation details |
| `docs/USEFUL_COMMANDS.md` | Migration and commands |
| `CURRENT_STATE.md` | Current project state |
| `prompts/planner.md` | Planner agent prompt |
| `prompts/supervisor.md` | Supervisor agent prompt |

---

## UNRESOLVED QUESTIONS

1. **Why is opencode being terminated?** (Concurrency? Environment? Resources?)
2. **How should OpenCode do db operations?** (Design decision needed)
3. **Should we add event queue persistence?** (For 50 parallel agents)
4. **Supabase 45% memory usage?** (Base overhead or something wrong?)

---

## REMEMBER

- **No hardcoded keys** - ever
- **Everything swappable** - models, platforms, runners
- **Vault for secrets** - SERVICE_KEY reads/writes vault
- **Process environment only** - no .env files for keys
- **This is a test** - don't shortcut, learn from failures
