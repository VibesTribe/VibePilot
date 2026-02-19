# VibePilot Useful Commands

Copy-paste commands for common operations.

---

## Orchestrator Service

```bash
# Check if running
systemctl status vibepilot-orchestrator

# View live logs (Ctrl+C to exit)
journalctl -u vibepilot-orchestrator -f

# View recent logs (last 50 lines)
journalctl -u vibepilot-orchestrator -n 50

# View logs from last hour
journalctl -u vibepilot-orchestrator --since "1 hour ago"

# Restart the service
sudo systemctl restart vibepilot-orchestrator

# Stop the service
sudo systemctl stop vibepilot-orchestrator

# Start the service
sudo systemctl start vibepilot-orchestrator
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

# Switch to research branch
git checkout research-considerations
```

---

## Database (Supabase)

```bash
# Connect to Supabase via psql (if installed)
psql $SUPABASE_URL

# View tasks table (via orchestrator venv)
source ~/vibepilot/venv/bin/activate
python3 -c "
from supabase import create_client
import os
from dotenv import load_dotenv
load_dotenv('/home/mjlockboxsocial/vibepilot/.env')
db = create_client(os.environ['SUPABASE_URL'], os.environ['SUPABASE_KEY'])
tasks = db.table('tasks').select('id,status,title').limit(10).execute()
for t in tasks.data:
    print(f\"{t['id'][:8]}... | {t['status']:15} | {t.get('title','')[:40]}\")
"
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
grep -r "search term" ~/vibepilot --include="*.py"

# View a file
cat filename
less filename  # (q to exit)
```

---

## Dashboard (Vibeflow)

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

# Restart everything
sudo systemctl restart vibepilot-orchestrator

# Check if port is in use
lsof -i :8000

# Free up memory (clear cache)
sync && echo 3 | sudo tee /proc/sys/vm/drop_caches
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
echo "=== ORCHESTRATOR ===" && systemctl is-active vibepilot-orchestrator
```
