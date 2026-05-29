# VibePilot Disaster Recovery Guide

**Last updated:** 2026-05-29 (post-SSHD-failure rebuild)

---

## What Happened

On 2026-05-24, the primary SSHD (Seagate ST1000LX015 1TB) started failing on the ThinkPad X220. The system was rebuilt on a new 1TB SSD. This document captures what was needed, what was lost, and the streamlined restore path.

---

## System Overview

| Component | Details |
|-----------|---------|
| Machine | Lenovo ThinkPad X220, i5-2520M, 16GB RAM |
| OS | Linux Mint 22.3 |
| SSD | 1TB (~907GB usable) |
| RAM usage | ~1.7GB idle (Hermes 189MB, PostgreSQL 243MB, Cinnamon 200MB) |
| Swap | 2GB |

---

## Source of Truth Map

Everything below is what you need to reconstruct from scratch.

### GitHub Repos (primary source)

| Repo | What's in it | Branch |
|------|-------------|--------|
| `VibesTribe/VibePilot` | Governor Go source, prompts, configs (agents.json, models.json, routing.json, etc.), scripts, docs | `main` |
| `VibesTribe/vibeflow` | Dashboard, telemetry state, GitHub Actions workflows | `main` |
| `VibesTribe/knowledgebase` | DB backups (split into <100MB gzip chunks), KB scripts, SQL schemas | `backups` branch for DB, `db-backup-YYYYMMDD` for dated snapshots |
| `VibesTribe/vibes-agent-context` | Hermes cron job definitions, context files | `main` |

### PostgreSQL Database

- **Location:** Local PostgreSQL 16, database `vibepilot`
- **Tables:** 101 tables including model_catalog (586 models), secrets_vault (15 encrypted entries), prompts, plans, tasks, etc.
- **Backup method:** `pg_dump` + split into <90MB chunks + gzip + push to GitHub `backups` branch
- **Backup script:** `~/knowledgebase/split-backup.sh`
- **Restore:** `cat vibepilot-part-*.gz | gunzip | psql -d vibepilot`

### Hermes Agent

- **Version:** 0.15.1 (installed via hermes CLI)
- **Config:** `~/.hermes/config.yaml`
- **Env vars:** `~/.hermes/.env` (API keys)
- **Skills:** `~/.hermes/skills/` (153 skills total)
- **Cron jobs:** `~/.hermes/cron/jobs.json` (backed up to vibes-agent-context repo)
- **Session DB:** `~/.hermes/sessions/` (SQLite, local only)
- **Memories:** `~/.hermes/memories/MEMORY.md` and `USER.md`

### Governor Service

- **Binary:** `/home/vibes/vibepilot/governor/governor` (v2.0.0)
- **Source:** `~/vibepilot/governor/` (Go source)
- **Configs:** `~/vibepilot/governor/config/` (models.json, routing.json, etc.)
- **Managed repo:** `~/.governor/repos/VibesTribe-VibePilot` (auto-cloned from GitHub)
- **Systemd service:** `~/.config/systemd/user/vibepilot-governor.service`
- **VAULT_KEY override:** `~/.config/systemd/user/vibepilot-governor.service.d/override.conf`

### Credentials

- **Location:** `~/Documents/vibepilot important 1.txt`
- **Backup stash:** `~/.hermes/vibepilot-credentials-backup.txt`
- **Vault:** PostgreSQL `secrets_vault` table (encrypted with VAULT_KEY)
- **Git auth:** `~/.git-credentials` (GitHub PAT)
- **Vault CLI:** `~/vibepilot/governor/governor vault <set|get|list|delete>`

---

## Restore Procedure (Clean SSD)

### Step 1: OS + Base Setup

```bash
# Install Linux Mint, then:
sudo apt update && sudo apt install -y postgresql-16 git python3 python3-venv
```

### Step 2: PostgreSQL

```bash
sudo -u postgres createuser -s vibes
createdb vibepilot
# Restore from GitHub backup:
git clone https://github.com/VibesTribe/knowledgebase.git ~/knowledgebase
cd ~/knowledgebase
git checkout backups
cat vibepilot-part-*.gz | gunzip | psql -d vibepilot
```

### Step 3: Hermes Agent

```bash
# Install Hermes (follow docs at hermes-agent.nousresearch.com)
curl -fsSL https://hermes-agent.nousresearch.com/install.sh | bash
# Restore .env with API keys
# Restore config.yaml with fallback providers
# Restore skills from ~/.hermes/skills/ (in vibes-agent-context repo backup)
```

### Step 4: Governor

```bash
# Clone VibePilot repo
git clone https://github.com/VibesTribe/VibePilot.git ~/vibepilot
# Build governor (or restore binary)
cd ~/vibepilot/governor && go build -o governor .
# Install systemd service
cp scripts/governor.service ~/.config/systemd/user/vibepilot-governor.service
# Add VAULT_KEY override
mkdir -p ~/.config/systemd/user/vibepilot-governor.service.d/
# ADD VAULT_KEY (from credentials file)
systemctl --user daemon-reload
systemctl --user enable --now vibepilot-governor
```

### Step 5: GitHub PAT

```bash
# Generate new Classic PAT at https://github.com/settings/tokens/new
# Scopes needed: repo, read:org, workflow
echo "https://x-access-token:YOUR_PAT@github.com" > ~/.git-credentials
chmod 600 ~/.git-credentials
# Update governor vault:
DATABASE_URL="postgres://vibes@/vibepilot?host=/var/run/postgresql" \
  VAULT_KEY="YOUR_VAULT_KEY" \
  ~/vibepilot/governor/governor vault set GITHUB_TOKEN YOUR_PAT
```

### Step 6: Verify

```bash
systemctl --user status vibepilot-governor
journalctl --user -u vibepilot-governor --no-pager -n 20
# Should show: no validation errors, all connectors registered, health checks OK
```

---

## What Was NOT Backed Up (Lessons Learned)

These were lost or difficult to recover during the SSD failure:

| Item | Impact | Prevention |
|------|--------|-----------|
| **Hermes session DB** | Lost all conversation history | Sessions are ephemeral, acceptable loss |
| **GitHub PAT** | Had to regenerate | Now stored in governor vault + git-credentials |
| **Old .env structure/comments** | Lost descriptive comments | Merged into clean file without comments |
| **GitHub Actions workflows** | Were running on schedule, burning API credits | All disabled. Re-enable individually as needed |
| **Hermes .netrc** | Lost | Reconstructed from git-credentials |
| **SSH keys** | None existed | N/A |
| **Browser state/cache** | Lost | Acceptable |
| **Custom .bashrc/aliases** | Lost | Minimal, not critical |

---

## What Worked Well

1. **PostgreSQL backup via GitHub** -- split+gzip kept files under GitHub's 100MB limit, restore was clean
2. **Governor managed repo** -- auto-clones from GitHub on start, no local-only state
3. **VibePilot configs in GitHub** -- identical between old and new drives, zero merge needed
4. **Secrets vault** -- encrypted in PostgreSQL, decrypted with VAULT_KEY, survived migration perfectly
5. **Skills in filesystem** -- easy to bulk copy from old drive, deduplication was straightforward

---

## What Could Be Better

1. **GitHub PAT in vault** -- security layer truncates tokens in agent context, making it hard to store programmatically. Store via direct terminal command.
2. **Credential file truncation** -- the credentials doc was storing truncated PATs (first 7 + last 4 chars). Full tokens must be stored in git-credentials or vault only.
3. **Cron jobs not in GitHub** -- now backed up to vibes-agent-context repo, but should be version-controlled properly
4. **DB backup automation** -- currently manual. Should be a cron job (pg_dump + split + push to GitHub)
5. **Routing validation** -- models in routing.json that don't exist in models.json cause degraded mode. Should be validated on commit.
6. **Supabase references** -- Supabase was removed but left references in GitHub Actions (Keep Supabase Awake), vault (SUPABASE_SERVICE_KEY), and workflows. Now cleaned.

---

## Active Credentials (as of 2026-05-29)

Stored in `~/Documents/vibepilot important 1.txt` with ACTIVE / NOT CURRENTLY USING sections.

**ACTIVE:**
- VAULT_KEY (governor encryption)
- Z.AI / GLM API key
- 4x Gemini API keys (courier, researcher, visual tester, general)
- Groq API key
- NVIDIA NIM API key
- 2x OpenRouter API keys
- GitHub Classic PAT
- Gmail (app password)
- Raindrop.IO

**NOT CURRENTLY USING:**
- DeepSeek, Supabase, Deepgram, SiliconFlow, Spaceagent, Telegram, Tailscale/OpenClaw, Kimi (dead), old GitHub PATs, old GLM, EC2 SSH key, LiteLLM

---

## System Services

| Service | Status | Auto-start |
|---------|--------|-----------|
| PostgreSQL 16 | active | yes (system) |
| vibepilot-governor | active | yes (user) |
| Hermes Agent | on-demand | via CLI |
