# VibePilot Current State

**Required reading: FOUR files**
1. **THIS FILE** (`CURRENT_STATE.md`) - What, where, how, current state
2. **`docs/SECURITY_BOOTSTRAP.md`** - How credentials work (READ THIS FIRST)
3. **`docs/GOVERNOR_HANDOFF.md`** - Full implementation details
4. **`docs/core_philosophy.md`** - Strategic mindset and principles

**Read all four → Know everything → Do anything**

---

**Last Updated:** 2026-02-27
**Updated By:** GLM-5 - Session 33
**Branch:** `go-governor` (all changes pushed)
**Status:** CLEAN - Python removed, security fixed, Go only

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
| `SUPABASE_KEY` | `/etc/systemd/.../override.conf` | root only |
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
- Governor now uses `SUPABASE_KEY` (not SERVICE_KEY)
- Keys removed from `.env`, stored in systemd override (root-only)
- Old Python orchestrator service disabled

### Python Removed
- All `.py` files → `legacy/python/`
- `venv/` → `legacy/python/venv/`
- `runners/`, `core/` → `legacy/python/`
- `requirements.txt` deleted

### New Files
- `docs/SECURITY_BOOTSTRAP.md` - Credential architecture
- `scripts/setup-bootstrap.sh` - One-time key setup
- `scripts/deploy-governor.sh` - Deploy script
- `.github/workflows/deploy-governor.yml` - CI/CD option

---

## FOR NEXT SESSION

**Read first:**
1. `docs/SECURITY_BOOTSTRAP.md` - How credentials work
2. `docs/GOVERNOR_HANDOFF.md` - Implementation details
3. `docs/core_philosophy.md` - Strategic mindset

**What to do:**
1. Test the full PRD → Plan → Review flow
2. Verify supervisor reads plan and makes decision
3. Implement task creation from approved plan

**What NOT to do:**
- Don't look for keys in `.env` (it's empty)
- Don't use Python code (it's in legacy/)
- Don't hardcode keys anywhere
