# VibePilot Useful Commands

Copy-paste commands for common operations.

---

## MIGRATING TO NEW HOST

If you need to move VibePilot to a new server:

```bash
# 1. On new server - install Go
sudo apt update && sudo apt install -y golang-go

# 2. Clone the repo
git clone https://github.com/VibesTribe/VibePilot.git ~/vibepilot
cd ~/vibepilot

# 3. Build the governor
cd governor && go build -o governor ./cmd/governor

# 4. Set up bootstrap keys (you'll need the 3 keys from GitHub Secrets)
sudo scripts/setup-bootstrap.sh
# Enter when prompted:
#   - SUPABASE_URL
#   - SUPABASE_KEY
#   - VAULT_KEY

# 5. Deploy the service
sudo scripts/deploy-governor.sh

# 6. Verify it's running
systemctl status vibepilot-governor
```

**That's it.** Everything else (tasks, plans, vault secrets) is in Supabase and GitHub.

---

## Governor Service (Go - Current)

```bash
# Check if running
systemctl status vibepilot-governor

# View live logs (Ctrl+C to exit)
journalctl -u vibepilot-governor -f

# View recent logs (last 50 lines)
journalctl -u vibepilot-governor -n 50

# View logs from last hour
journalctl -u vibepilot-governor --since "1 hour ago"

# Restart the service
sudo systemctl restart vibepilot-governor

# Stop the service
sudo systemctl stop vibepilot-governor

# Start the service
sudo systemctl start vibepilot-governor

# Rebuild and restart (after code changes)
cd ~/vibepilot/governor && go build -o governor ./cmd/governor && sudo systemctl restart vibepilot-governor
```

---

## Git Basics

```bash
# Check current branch
git branch --show-current

# Check status (what changed)
git status

# See recent commits
git log --oneline -5

# Get latest from remote
git pull

# Switch to main branch
git checkout main

# Switch to go-governor branch (current development)
git checkout go-governor
```

---

## Vault Management (Encrypted Secrets)

```bash
# Set env vars first
export DATABASE_URL="postgres://vibes@/vibepilot?host=/var/run/postgresql"
export VAULT_KEY="$(grep VAULT_KEY ~/.config/systemd/user/vibepilot-governor.service.d/override.conf | cut -d'"' -f2)"
cd ~/vibepilot/governor

# List all keys
./governor vault list

# Add or update a key (copy paste done)
./governor vault set GROQ_API_KEY "gsk_..."

# Decrypt and view a key
./governor vault get GITHUB_TOKEN

# Delete a key
./governor vault delete OLD_KEY

# Rotate master encryption key (re-encrypts all secrets)
./governor vault rotate-key NEW_BASE64_KEY
# IMPORTANT: Update VAULT_KEY in systemd override after rotation
```

---

## Local PostgreSQL (replaces Supabase)

```bash
# Connect (peer auth, no password)
psql -d vibepilot

# Quick query - see current tasks
psql -d vibepilot -c "SELECT id, status, title FROM tasks ORDER BY updated_at DESC LIMIT 5;"

# Check model count
psql -d vibepilot -c "SELECT count(*) FROM models WHERE status='active';"

# Check vault contents (encrypted)
psql -d vibepilot -c "SELECT key_name FROM secrets_vault ORDER BY key_name;"

# Backup
~/vibepilot/scripts/pg-dump-and-push.sh
```

---

## Database (Supabase) — LEGACY

```bash
# Quick query - see current plans
source ~/.env 2>/dev/null || true
curl -s "$SUPABASE_URL/rest/v1/plans?select=id,status,title" \
  -H "apikey: $SUPABASE_KEY" \
  -H "Authorization: Bearer $SUPABASE_KEY" | python3 -m json.tool

# Quick query - see pending tasks
curl -s "$SUPABASE_URL/rest/v1/tasks?select=id,status,title&status=eq.pending" \
  -H "apikey: $SUPABASE_KEY" \
  -H "Authorization: Bearer $SUPABASE_KEY" | python3 -m json.tool
```

---

## Server Info

```bash
# Check disk space
df -h

# Check memory
free -h

# Check running processes (top 10 by memory)
ps aux --sort=-%mem | head -11

# Check who's logged in
who

# Check uptime
uptime
```

---

## File Navigation

```bash
# Go to vibepilot directory
cd ~/vibepilot

# List files
ls -la

# Find a file
find ~ -name "filename" 2>/dev/null

# Search inside files
grep -r "search term" ~/vibepilot --include="*.go"

# View a file
cat filename
less filename  # (q to exit)
```

---

## Dashboard & Links

```bash
# Dashboard URL
https://vibeflow-dashboard.vercel.app/

# VibePilot GitHub
https://github.com/VibesTribe/VibePilot

# Vibeflow GitHub
https://github.com/VibesTribe/vibeflow
```

---

## Emergency

```bash
# Kill a stuck process (replace PID with actual number)
kill -9 12345

# Restart the governor
sudo systemctl restart vibepilot-governor

# Check if port is in use
lsof -i :8000

# Free up memory (clear cache)
sync && echo 3 | sudo tee /proc/sys/vm/drop_caches

# If governor won't start - check logs
journalctl -u vibepilot-governor -n 100
```

---

## Quick Health Check

Run this to see overall system status:

```bash
echo "=== UPTIME ===" && uptime && \
echo "" && \
echo "=== DISK ===" && df -h / && \
echo "" && \
echo "=== MEMORY ===" && free -h && \
echo "" && \
echo "=== GOVERNOR ===" && systemctl is-active vibepilot-governor
```

---

## OpenCode Sessions (Cleanup)

```bash
# List all sessions
opencode session list

# Delete a specific session
opencode session delete <session-id>

# Count sessions
opencode session list | wc -l
```

---

## Key Files to Know

| File | Purpose |
|------|---------|
| `CURRENT_STATE.md` | What's happening now |
| `docs/SECURITY_BOOTSTRAP.md` | How credentials work |
| `docs/GOVERNOR_HANDOFF.md` | Governor implementation details |
| `governor/config/system.json` | Governor configuration |
| `prompts/*.md` | Agent behavior definitions |
