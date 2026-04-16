# VibePilot Setup & Startup Guide

For humans and agents. Last updated: April 15, 2026.

## Fresh Machine Setup (Disaster Recovery)

### Prerequisites
- Linux machine (tested on Ubuntu 24.04, ThinkPad X220)
- Internet connection
- GitHub account with access to VibesTribe/VibePilot
- Password manager with: Supabase URL, Supabase Service Key, Vault Key

### Steps

```bash
# 1. Clone the repo
git clone https://github.com/VibesTribe/VibePilot.git ~/VibePilot

# 2. Copy to running location
cp -r ~/VibePilot ~/vibepilot

# 3. Install Go (if not installed)
# Ubuntu: sudo apt install golang-go
# Or download from https://go.dev/dl/

# 4. Install Node.js (for Hermes)
# curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.3/install.sh | bash
# nvm install 24

# 5. Install Hermes agent
# Follow Hermes docs or copy from backup

# 6. Restore local configs
bash ~/VibePilot/backup/restore.sh

# 7. Create governor environment file
cat > ~/.governor_env << 'EOF'
export SUPABASE_URL="paste_from_password_manager"
export SUPABASE_SERVICE_KEY="paste_from_password_manager"
export VAULT_KEY="paste_from_password_manager"
EOF
chmod 600 ~/.governor_env

# 8. Build the governor
cd ~/VibePilot/governor
go build -o ~/vibepilot/governor/governor ./cmd/governor/

# 9. Start everything
systemctl --user daemon-reload
systemctl --user enable --now vibepilot-governor
systemctl --user enable --now hermes-gateway

# 10. Verify
systemctl --user status vibepilot-governor
systemctl --user status hermes-gateway
journalctl --user -u vibepilot-governor -n 20
```

---

## Governor Quick Reference

### What It Is
Go binary that orchestrates AI agents. Reads tasks from Supabase, routes them to models, manages git branches.

### Where It Lives
- **Source code:** `~/VibePilot/governor/`
- **Running binary:** `~/vibepilot/governor/governor`
- **Config files:** `~/vibepilot/governor/config/` (models.json, connectors.json, routing.json, system.json)
- **Prompt templates:** `~/vibepilot/prompts/` or `~/VibePilot/prompts/`

### Common Commands

```bash
# Check status
systemctl --user status vibepilot-governor

# View logs (follow mode)
journalctl --user -u vibepilot-governor -f

# View recent logs
journalctl --user -u vibepilot-governor -n 50

# Restart after config changes or rebuild
systemctl --user restart vibepilot-governor

# Stop
systemctl --user stop vibepilot-governor

# Rebuild after code changes
cd ~/VibePilot/governor
go build -o ~/vibepilot/governor/governor ./cmd/governor/
systemctl --user restart vibepilot-governor
```

### Config File Locations (IMPORTANT -- two copies)

| File | Dev copy (edit here) | Running copy (governor reads this) |
|---|---|---|
| models.json | ~/VibePilot/governor/config/models.json | ~/vibepilot/governor/config/models.json |
| connectors.json | ~/VibePilot/governor/config/connectors.json | ~/vibepilot/governor/config/connectors.json |
| routing.json | ~/VibePilot/governor/config/routing.json | ~/vibepilot/governor/config/routing.json |
| system.json | ~/VibePilot/governor/config/system.json | ~/vibepilot/governor/config/system.json |
| agents.json | ~/VibePilot/governor/config/agents.json | ~/vibepilot/governor/config/agents.json |

The post-commit hook auto-syncs config files to the running copy. But if you edit without committing, you need to manually copy.

### Credentials

- **NO .env files.** Credentials are in Supabase vault (encrypted).
- Governor loads them from `~/.governor_env` (sourced by systemd override).
- To read a secret from vault: use the Go vault package or query `secrets_vault` table.
- To update a secret: use `~/VibePilot/governor/cmd/encrypt_secret/` or Hermes memory tool.

### The Two-Copy System

```
~/VibePilot/     = DEV copy. Edit code here. Commit from here.
~/vibepilot/     = RUNNING copy. Governor binary runs from here. Config read from here.
```

Always edit in ~/VibePilot/, commit, and the post-commit hook syncs to ~/vibepilot/.
If you edit ~/vibepilot/ directly, your changes will be overwritten on next commit.

---

## Hermes Agent Quick Reference

### What It Is
Python agent (Hermes) that chats via Telegram/Dashboard. Has tools for terminal, browser, file ops, etc.

### Where It Lives
- **Install:** `~/.hermes/hermes-agent/`
- **Config:** `~/.hermes/config.yaml`
- **Memory:** `~/.hermes/memory/`
- **Skills:** `~/.hermes/skills/`
- **Context:** reads `.hermes.md` from `~/VibePilot/` (set via TERMINAL_CWD)

### Common Commands

```bash
# Check status
systemctl --user status hermes-gateway

# View logs
journalctl --user -u hermes-gateway -f

# Restart
systemctl --user restart hermes-gateway
```

---

## Supabase Quick Reference

### What It Is
Postgres database + realtime. The governor's brain. All task state, memory, vault, schema lives here.

### How To Access
- **Dashboard:** https://supabase.com/dashboard (login with GitHub)
- **SQL Editor:** Dashboard > SQL Editor (paste SQL and run)
- **REST API:** via governor's Supabase client (auto-configured from ~/.governor_env)

### Schema Migrations
See `.hermes.md` rule 4 or `docs/STARTUP_GUIDE.md` rule 4.
The ONLY way: write numbered SQL file -> push to GitHub -> human applies via SQL Editor.

### Current Schema Version
111 migrations. All in `docs/supabase-schema/NNN_*.sql`.

---

## Files On GitHub (What Survives Machine Loss)

Everything here is pushed to `https://github.com/VibesTribe/VibePilot`:

- `.hermes.md` -- agent rules (loaded every Hermes message)
- `AGENTS.md` -- cross-agent rules pointer
- `.context/` -- knowledge layer (rules, code index, code map)
- `CURRENT_STATE.md` -- what's deployed and running
- `TODO.md` -- what's left to do
- `VIBEPILOT_WHAT_YOU_NEED_TO_KNOW.md` -- architecture details
- `agent/HERMES_MEMORIES.md` -- Hermes memory backup
- `docs/` -- all documentation and SQL schemas
- `backup/` -- local config templates (systemd, hermes config)
- `scripts/` -- automation scripts
- `governor/` -- all Go source code
- `config/` -- all JSON config files

### What's NOT on GitHub (must recreate)
- `~/.governor_env` -- 3 secrets (Supabase URL, Service Key, Vault Key)
- Chrome login sessions
- SSH keys

---

## Cron Jobs (Automated Maintenance)

| Job | Schedule | What It Does |
|---|---|---|
| Memory backup | Every 4h | Copies Hermes memory to `agent/HERMES_MEMORIES.md` on GitHub |
| Context sync | Every 6h | Rebuilds `.context/` and pushes to GitHub if changed |
| Post-commit hook | On every commit | Syncs config + context to running copy at ~/vibepilot/ |
