# VibePilot Current State - 2026-04-03

## Status: Governor Fixed and Running

### Critical Fixes Applied ✅
Fixed "connector not registered" errors in task and plan review handlers:

**File: `governor/cmd/governor/handlers_task.go`**
- Line 328: Changed `factory.CreateWithContext(ctx, "supervisor", taskType)` to `factory.CreateWithConnector(ctx, "supervisor", taskType, routingResult.DestinationID)`
- Line 360: Changed retry session to use `factory.CreateWithConnector(ctx, "supervisor", "review", routingResult.DestinationID)`

**File: `governor/cmd/governor/handlers_plan.go`**
- Line 247: Changed `factory.CreateWithContext(ctx, "supervisor", "review")` to `factory.CreateWithConnector(ctx, "supervisor", "review", routingResult.ConnectorID)`
- Line 276: Changed retry session to use `factory.CreateWithConnector(ctx, "supervisor", "review", routingResult.ConnectorID)`

**Root Cause:** Supervisor sessions were using `CreateWithContext` which didn't pass the connector ID, causing "connector not registered" errors.

**Committed:** `214a7eb8` - "fix: use CreateWithConnector for supervisor sessions"

---

## How to Start Governor (IMPORTANT)

### Bootstrap Credentials Location
The 3 bootstrap credentials are stored in `/home/vibes/vibepilot-server/restart_governor.sh`:

1. **SUPABASE_URL** - Your Supabase project URL
2. **SUPABASE_SERVICE_KEY** - Service role key (admin access)
3. **VAULT_KEY** - Decrypts the Supabase vault

**These were set up during initial server installation and are NOT in GitHub Secrets.**

### Starting Governor

**Method 1: Use the restart script (RECOMMENDED)**
```bash
~/vibepilot-server/restart_governor.sh
```

**Method 2: Manual start**
```bash
cd ~/vibepilot/governor
export SUPABASE_URL="https://qtpdzsinvifkgpxyxlaz.supabase.co"
export SUPABASE_SERVICE_KEY="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
export VAULT_KEY="P9jFR25vbjcNxG2S3lx4ZCyspfGLd7wZYliZWLjqKLc="
nohup ./governor > /tmp/governor.out 2>&1 &
```

### Checking Governor Status
```bash
# Check if running
ps aux | grep "[g]overnor"

# View logs
tail -f /tmp/governor.out

# Check dashboard
open http://localhost:3000
```

---

## Server Setup Information

**Server:** Linux Mint Cinnamon on ThinkPad X220
**Purpose:** VibePilot agent execution server
**Setup Directory:** `/home/vibes/vibepilot-server/`

**Key Setup Files:**
- `restart_governor.sh` - Starts governor with credentials
- `install.sh` - Initial server setup
- `quick-status.sh` - Quick status check
- `monitor.sh` - Monitoring tools

---

## GitHub Workflow vs Local Script

**Note:** The GitHub workflow `.github/workflows/deploy-governor.yml` references a `self-hosted` runner that is NOT configured on this server. Do NOT rely on that workflow for deployment.

**Use the local restart script instead:** `~/vibepilot-server/restart_governor.sh`

---

## Current Governor Status

**Running:** Yes (started 2026-04-03 19:58)
**PID:** 350786
**Config:**
- Realtime connected to Supabase
- 16 prompts synced
- Webhooks on port 8080
- Max 1 concurrent per module, 2 total
- Using glm-5 model via claude-code connector

**Logs:** `/tmp/governor.out`

---

## Important Notes for Next Session

1. **DO NOT** look for credentials in:
   - `.env` files (don't exist)
   - systemd override files (not set up)
   - GitHub Secrets (workflow requires self-hosted runner)

2. **DO** use the restart script:
   ```bash
   ~/vibepilot-server/restart_governor.sh
   ```

3. **DO NOT** modify the credential values in `restart_governor.sh` unless you know what you're doing.

4. **If governor crashes:** Run the restart script again.

5. **To check what's working:** Open dashboard at `http://localhost:3000`

---

## Recent Changes

- **2026-04-03 19:58:** Governor restarted with credentials, operational
- **2026-04-03 19:55:** Pushed fixes for CreateWithConnector to GitHub
- **2026-04-03:** Fixed handler_task.go and handlers_plan.go connector issues

---
**Last Updated:** 2026-04-03 20:00
