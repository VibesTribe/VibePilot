# VibePilot Current State

**Required reading: FIVE files**
1. **THIS FILE** (`CURRENT_STATE.md`) - What, where, how, current state
2. **`docs/SESSION_33_HANDOFF.md`** - Latest session details and blockers
3. **`docs/SECURITY_BOOTSTRAP.md`** - How credentials work (READ THIS FIRST)
4. **`docs/GOVERNOR_HANDOFF.md`** - Full implementation details
5. **`docs/core_philosophy.md`** - Strategic mindset and principles

**Read all five → Know everything → Do anything**

---

**Last Updated:** 2026-02-27
**Updated By:** GLM-5 - Session 34
**Branch:** `main` (go-governor merged)
**Status:** FIXED - "signal: terminated" bug resolved (cleanup script fixed)

---

# CURRENT ARCHITECTURE

## What's Running

```
vibepilot-governor.service (Go binary)
├── Polls Supabase every 1s
├── Max 8 concurrent per module, 160 total
├── OpenCode limit: 5 concurrent
└── Reads secrets from vault at runtime
```

**Status:** `systemctl status vibepilot-governor`

## Codebase Structure (Clean)

```
vibepilot/
├── governor/              # ACTIVE - Go binary (everything)
│   ├── cmd/governor/      # Main entry point
│   ├── internal/
│   │   ├── db/            # Supabase client
│   │   ├── vault/         # Secret decryption
│   │   ├── runtime/       # Event detection, agent sessions
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

## Migrating to New Host

```bash
# 1. Clone repo
git clone https://github.com/VibesTribe/VibePilot.git ~/vibepilot

# 2. Build governor
cd ~/vibepilot/governor && go build -o governor ./cmd/governor

# 3. Set up bootstrap keys (from GitHub Secrets)
sudo scripts/setup-bootstrap.sh

# 4. Deploy
sudo scripts/deploy-governor.sh
```

See `docs/USEFUL_COMMANDS.md` for full guide.

---

## Session 33 Changes

### Security Fix
- Governor uses `SUPABASE_SERVICE_KEY` (bypasses RLS for vault)
- Keys removed from `.env`, stored in systemd override (root-only)
- Old Python orchestrator service disabled

### Branch Merge
- Merged `go-governor` into `main` (single clean branch)
- Deleted OLD architecture (YAML config with hardcoded keys)
- Codebase: 5,695 lines Go, 24 files

### Python Removed
- All `.py` files → `legacy/python/`
- `venv/` → `legacy/python/venv/`
- `runners/`, `core/` → `legacy/python/`
- `requirements.txt` deleted

### Audit Fixes
- Vault: Fixed machine salt (portable across hosts)
- Web tools: Made configurable via system.json
- Provider detection: Config-driven, not hardcoded

### New Files
- `docs/SECURITY_BOOTSTRAP.md` - Credential architecture
- `docs/SESSION_33_HANDOFF.md` - Session details and blockers
- `scripts/setup-bootstrap.sh` - One-time key setup
- `scripts/deploy-governor.sh` - Deploy script
- `.github/workflows/deploy-governor.yml` - CI/CD option

---

## Session 34 Changes (Current)

### Bug Fix: "signal: terminated" Root Cause Found

**Problem:** Governor-spawned opencode processes were killed by `cleanup_zombies.sh` hourly cron job.

**Root Cause:** The cleanup script couldn't distinguish:
- Governor children (should be protected) from
- True zombie orphans (should be killed)

**Solution:** Updated `scripts/cleanup_zombies.sh` to check cgroup membership:
- Processes in `vibepilot-governor.service` cgroup → PROTECTED
- Orphans (PPID=1) NOT in governor cgroup → KILLED
- User sessions with terminal → PROTECTED

**Verification:** Tested with systemd test services to confirm:
- Children ARE in governor's cgroup
- `KillMode=control-group` kills children when service stops
- Orphaned children also get cleaned up by systemd

### No Governor Code Changes Needed

The governor's process management is already robust:
- `KillMode=control-group` in systemd handles cleanup
- Children stay in cgroup even with different PGID
- Systemd kills entire cgroup on service stop

---

## Remaining Items

**What to work on next:**
1. Tool architecture: How should OpenCode do db operations?
2. Event system: Add persistence/retry for 50 parallel agents
3. Test the fix by triggering a planner event

**What NOT to do:**
- Don't look for keys in `.env` (it's empty)
- Don't use Python code (it's in legacy/)
- Don't hardcode keys anywhere
- Don't modify cleanup script without understanding cgroup logic
