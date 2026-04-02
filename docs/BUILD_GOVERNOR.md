# Building VibePilot Governor from Scratch

## Overview
This guide covers setting up and building the VibePilot Governor from a fresh GitHub clone.

## Prerequisites

### Required Software
- **Go**: 1.22 or later
- **Git**: For cloning the repository
- **Supabase Account**: For database and realtime subscriptions
- **Claude Code CLI**: For task execution (optional but recommended)

### Verify Go Installation
```bash
go version
# Should show go1.22.x or later
```

## Step 1: Clone Repository

```bash
# Clone the VibePilot repository
cd ~
git clone https://github.com/yourusername/vibepilot.git
cd vibepilot
```

## Step 2: Set Up Environment Variables

The governor requires Supabase credentials:

```bash
# Set Supabase environment variables
export SUPABASE_URL="https://your-project-id.supabase.co"
export SUPABASE_SERVICE_KEY="your-service-role-key"

# Optional: For encrypted secrets
export VAULT_KEY="your-encryption-key"
```

**Get these from:**
- Supabase Dashboard → Project Settings → API
- Copy the `URL` and `service_role` key

## Step 3: Build the Governor

```bash
# Navigate to governor directory
cd governor

# Build the binary
go build -o bin/governor ./cmd/governor

# Verify the binary was created
ls -la bin/governor
```

**Expected output:** `bin/governor` executable file

## Step 4: Verify Configuration Files

The governor requires these config files in `governor/config/`:

- `agents.json` - Agent definitions with model assignments
- `connectors.json` - Available connectors (CLI, API, Web)
- `models.json` - Model profiles and capabilities
- `tools.json` - Tool definitions
- `routing.json` - Routing rules
- `plan_lifecycle.json` - Plan lifecycle configuration

**Check configs exist:**
```bash
ls -la config/
```

If missing, copy from the repository:
```bash
cp -r config.example/* config/
# or manually create from templates
```

## Step 5: Start the Governor

```bash
# From the governor directory
cd /home/vibes/vibepilot/governor

# Start with environment variables
SUPABASE_URL="https://your-project.supabase.co" \
SUPABASE_SERVICE_KEY="your-service-key" \
./bin/governor
```

**Expected startup output:**
```
VibePilot Governor 2.0.0 (commit: dev, built: unknown)
Connected to database
Synced N prompts from /path/to/prompts to Supabase
Core state machine initialized
Registered: claude-code (claude)
Running startup recovery...
[Recovery] No orphaned sessions found
[Realtime] Connected successfully
[Realtime] Subscribed to table plans (event: *)
[Realtime] Subscribed to table tasks (event: *)
Governor started (webhooks: port 8080/webhooks, ...)
```

## Step 6: Verify Governor is Running

```bash
# Check process
ps aux | grep "[g]overnor"

# Test webhook endpoint
curl http://localhost:8080/webhooks
# Should return 404 or webhook response
```

## Step 7: Run in Background (Production)

```bash
# Start with nohup for background execution
SUPABASE_URL="https://your-project.supabase.co" \
SUPABASE_SERVICE_KEY="your-service-key" \
nohup ./bin/governor > /tmp/governor.out 2>&1 &

# Monitor logs
tail -f /tmp/governor.out
```

## Common Issues

### Issue: "go: module not found"
**Solution:**
```bash
cd governor
go mod download
go mod tidy
```

### Issue: "Database credentials required"
**Solution:** Set SUPABASE_URL and SUPABASE_SERVICE_KEY environment variables

### Issue: "Port 8080 already in use"
**Solution:**
```bash
# Kill existing process
lsof -ti:8080 | xargs kill -9

# Or use different port (requires code change)
```

### Issue: "Config file not found"
**Solution:** Verify config files exist in `governor/config/`

### Issue: "Claude-code connector not found"
**Solution:** Ensure Claude Code CLI is installed and in PATH
```bash
which claude
# Should return path to claude binary
```

## Development Workflow

### Rebuild after code changes
```bash
cd governor
go build -o bin/governor ./cmd/governor

# Restart governor
pkill -f "governor.*governor"
# Then start again as in Step 5 or 7
```

### View logs
```bash
# If running in background
tail -f /tmp/governor.out

# Or specific log sections
tail -100 /tmp/governor.out | grep "\[Router\]"
```

### Clean restart
```bash
# Stop all governors
pkill -9 -f "governor"

# Clear old logs
rm -f /tmp/governor.out

# Start fresh
SUPABASE_URL="..." SUPABASE_SERVICE_KEY="..." ./bin/governor
```

## Quick Start Script

Save as `start-governor.sh`:

```bash
#!/bin/bash
cd /home/vibes/vibepilot/governor

# Stop existing
pkill -f "governor.*governor" 2>/dev/null
sleep 2

# Build
echo "Building governor..."
go build -o bin/governor ./cmd/governor || exit 1

# Start
echo "Starting governor..."
SUPABASE_URL="https://your-project.supabase.co" \
SUPABASE_SERVICE_KEY="your-service-key" \
nohup ./bin/governor > /tmp/governor.out 2>&1 &

sleep 3
echo "Governor started. Logs: /tmp/governor.out"
tail -20 /tmp/governor.out
```

Make executable and run:
```bash
chmod +x start-governor.sh
./start-governor.sh
```

## Architecture Notes

The governor is a stateless Go service that:
1. Listens to Supabase realtime events (plans, tasks, commands)
2. Routes tasks to appropriate models based on agent config
3. Creates Claude CLI sessions for task execution
4. Manages the two-session model (monitoring + execution)

**Key files:**
- `cmd/governor/main.go` - Entry point
- `internal/runtime/router.go` - Task routing logic
- `internal/runtime/config.go` - Configuration management
- `internal/state/` - State machine and lifecycle
- `config/` - Configuration files

**Config source of truth:**
- GitHub `prompts/` directory → synced to Supabase
- `config/agents.json` → agent model assignments
- `config/models.json` → model capabilities and access
- `config/connectors.json` → connector status and configuration

**Zero hardcoding policy:**
All routing is determined by config files. No model names or connector IDs are hardcoded in the Go code.
