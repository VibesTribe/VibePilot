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
**Updated By:** GLM-5 - Session 33
**Branch:** `main` (go-governor merged)
**Status:** BLOCKED - Debugging opencode "signal: terminated"

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

## Current Blocker

**OpenCode "signal: terminated" when called from governor**

- Planner agent fails immediately
- Supervisor worked earlier
- Direct opencode CLI calls work fine
- Possible causes: concurrency, systemd environment, resources
- See `docs/SESSION_33_HANDOFF.md` for full analysis

---

## FOR NEXT SESSION

**Read first:**
1. `docs/SESSION_33_HANDOFF.md` - Current blocker and analysis
2. `docs/SECURITY_BOOTSTRAP.md` - How credentials work
3. `docs/GOVERNOR_HANDOFF.md` - Implementation details

**What to debug:**
1. Why opencode is terminated (concurrency? environment?)
2. Tool architecture: How should OpenCode do db operations?
3. Event system: Add persistence/retry for 50 parallel agents

**What NOT to do:**
- Don't look for keys in `.env` (it's empty)
- Don't use Python code (it's in legacy/)
- Don't hardcode keys anywhere
