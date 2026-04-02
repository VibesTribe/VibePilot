# VibePilot Current State - 2026-04-01 20:00

## Status: ✅ ROUTING FIX COMPLETE - Governor Running

## 🧹 CLEANUP PROCEDURES
**Before testing:** See `docs/CLEANUP.md` for full cleanup guide.
- Quick Supabase reset: `TRUNCATE task_runs, tasks, plan_revisions, plans CASCADE;`
- Clean GitHub branches: `git branch | grep "task/" | xargs git branch -D`
- Restart governor after cleanup

## Summary

Fixed the routing issue where the governor couldn't route tasks to internal agents. The router now correctly uses each agent's configured model from `agents.json` to find an appropriate connector.

## Changes Made

### 1. Router Logic Fix (`governor/internal/runtime/router.go`)
**Problem:** Router searched for models by taskType/taskCategory, returned empty for generic tasks
- **Symptom:** `[Router] No internal routing available for role planner`
- **Root Cause:** `selectModelForConnector()` searched by empty taskType/taskCategory
- **Impact:** Governor couldn't route tasks to any internal agent

**Fix Applied:**
Modified `selectInternal()` (lines 97-132):
- When `Role` is specified, read agent's configured model from agents.json
- Check if available connectors can access that model
- Route to the first matching connector

**Key code:**
```go
// If routing for a specific agent (role), use that agent's configured model
var agentModelID string
if req.Role != "" {
    agent := r.cfg.GetAgent(req.Role)
    if agent != nil && agent.Model != "" {
        agentModelID = agent.Model
        log.Printf("[Router] Agent %s configured with model %s", req.Role, agentModelID)
    }
}

// If we have an agent-specific model, check if this connector can access it
if agentModelID != "" {
    if r.canConnectorAccessModel(conn.ID, agentModelID) {
        log.Printf("[Router] Internal routing: connector=%s model=%s (from agent %s)", conn.ID, agentModelID, req.Role)
        return &RoutingResult{...}
    }
    continue
}
```

### 2. Agents Config Fix (`governor/config/agents.json`)
**Problem:** Agent definitions were missing the `model` field that the router expects

**Fix Applied:**
Added `model` field to each agent:
- `planner`: glm-5
- `supervisor`: glm-5
- `maintenance`: glm-5
- `tester`: glm-5
- `task_runner`: glm-5
- `consultant`, `council`, `orchestrator`, `researcher`, `watcher`: gemini-2.0-flash

**Note:** glm-5 routes via claude-code (active), gemini-2.0-flash would need gemini-api (inactive)

### 3. Documentation Added
**File:** `docs/BUILD_GOVERNOR.md`

Comprehensive guide covering:
- Prerequisites (Go, Git, Supabase)
- Step-by-step build instructions from GitHub clone
- Configuration setup
- Common issues and solutions
- Development workflow

## Verified Routing

Both planner and supervisor now route correctly:

```
[Router] Agent planner configured with model glm-5
[Router] Internal routing: connector=claude-code model=glm-5 (from agent planner)

[Router] Agent supervisor configured with model glm-5
[Router] Internal routing: connector=claude-code model=glm-5 (from agent supervisor)
```

## Governor Status

**Running:** Yes (check with `ps aux | grep "[g]overnor"`)
**Port:** 8080 (webhooks)
**Config:** Zero hardcoding - all routing from JSON configs
**Prompts:** Synced from GitHub `prompts/` directory

## Previous Fixes Still Active

**Fix 1: CLI Runner STDIN Bug** ✅
- Prompt written to STDIN: `echo "prompt" | claude -p`

**Fix 2: Recovery Timeout** ✅
- Increased from 60s to 360s (6 minutes)

**Fix 3: T001 Numbering Bug** ✅
- Fixed RPC result parsing for slice-based task numbering

**Fix 4: Task Execution Result** ✅
- task_runs.result now stores execution data

## Files Changed

1. `governor/internal/runtime/router.go` - Router logic fix
2. `governor/config/agents.json` - Added model field to agents
3. `docs/BUILD_GOVERNOR.md` - New documentation file
4. `docs/CLEANUP.md` - Already existed, for reference

## Next Steps

The routing fix is complete. To test the full flow:

1. **Clean up:**
   ```bash
   # In Supabase SQL Editor:
   TRUNCATE task_runs, tasks, plan_revisions, plans CASCADE;

   # In terminal:
   git branch | grep "task/" | xargs git branch -D
   ```

2. **Create a PRD** via dashboard or in `docs/prd/`

3. **Create a plan** in Supabase referencing the PRD

4. **Monitor logs:**
   ```bash
   tail -f /tmp/governor.out | grep -E "\[Router\]|\[Session\]"
   ```

5. **Verify:**
   - Router finds the route correctly
   - Planner session executes with correct prompts
   - Output parses successfully
   - Tasks are created for execution

## Architecture Notes

**Two-session model:**
1. **Monitoring session** (this terminal) - Runs the governor
2. **Execution session** (created by governor) - Runs tasks

**Routing flow:**
1. Task/Plan created → Supabase triggers realtime event
2. Governor receives event → Router determines model/connector
3. Router reads agent's configured model from agents.json
4. Checks active connectors for model access
5. Creates Claude CLI session for task execution

---

**Last Updated:** 2026-04-01 20:00
**Status:** Routing fix complete and verified
**Governor:** Running with updated config
