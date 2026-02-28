# Session 35 Handoff - Dynamic Routing

**Date:** 2026-02-28
**Agent:** GLM-5

---

## The Problem We Solved

### What Was Broken

1. **TOOL: Format** - Rigid, fragile, ignored by CLI tools with native tools

2. **HARDCODED DESTINATIONS** - Every event handler had `"opencode"` hardcoded:
   - 10 places in main.go
   - Ignored all config
   - No fallback
   - No routing logic
   - If opencode down, everything broken

### Why It Was Wrong

The code had:
- Config files defining everything dynamically
- Code that ignored all of it and hardcoded "opencode"

This violated every VibePilot principle:
- NOT configurable
- NOT swappable
- NOT deletable
- NOT lean/clean/optimized

---

## The Solution

### 1. Removed TOOL: Format
- Deleted parsing from tools.go
- Simplified session.go
- Agents output in expected format, Governor handles execution

### 2. Dynamic Routing
- Created `router.go` - destination selection based on config
- Created `routing.json` - strategies, agent restrictions, categories
- Removed ALL hardcoded `"opencode"` from event handlers
- Routing based on: agent type, task type, destination availability

### 3. Routing Flow

```
Event fires
    ↓
selectDestination(agentID, taskID, taskType)
    ↓
Get strategy for agent (internal_only for planner/supervisor/etc)
    ↓
Get priority order (external → internal or internal only)
    ↓
For each category in priority:
    - Get active destinations in category
    - Check if available (status=active)
    - Return first available
    ↓
If nothing available → log, skip task
```

### 4. Agent Restrictions (in routing.json)

```json
{
  "agent_restrictions": {
    "internal_only": ["planner", "supervisor", "council", "orchestrator", "maintenance", "watcher", "tester"],
    "default": ["consultant", "researcher", "courier", "task_runner"]
  }
}
```

Internal agents NEVER go to external platforms. Only task execution can.

### 5. Destination Categories (in routing.json)

```json
{
  "destination_categories": {
    "external": {"check_field": "type", "check_values": ["web"]},
    "internal": {"check_field": "type", "check_values": ["cli", "api"]}
  }
}
```

Category is determined by checking destination's type field. Not hardcoded.

---

## Files Changed

| File | Change |
|------|--------|
| `runtime/router.go` | NEW - dynamic destination selection |
| `runtime/config.go` | Added routing config loading, helper methods |
| `runtime/session.go` | Simplified, removed TOOL: |
| `runtime/tools.go` | Removed TOOL: parsing |
| `cmd/governor/main.go` | All handlers use selectDestination() |
| `config/routing.json` | NEW - strategies and restrictions |
| `config/destinations.json` | Added provides_tools |
| `config/agents.json` | Renamed tools → capabilities |
| `prompts/planner.md` | Removed TOOL:, output format only |
| `prompts/supervisor.md` | Removed TOOL:, output format only |

---

## How Routing Works Now

### In main.go:

```go
// Before (WRONG):
pool.SubmitWithDestination(ctx, sliceID, "opencode", func() error { ... })

// After (CORRECT):
destID := selectDestination("planner", taskID, "planning")
if destID == "" {
    log.Printf("No destination available")
    return
}
pool.SubmitWithDestination(ctx, sliceID, destID, func() error { ... })
```

### selectDestination function:

```go
selectDestination := func(agentID, taskID, taskType string) string {
    result, err := destRouter.SelectDestination(ctx, runtime.RoutingRequest{
        AgentID:  agentID,
        TaskID:   taskID,
        TaskType: taskType,
    })
    if err != nil || result == nil {
        // Fallback: get any available destination
        dests := destRouter.GetAvailableDestinations()
        if len(dests) > 0 {
            return dests[0]
        }
        return ""
    }
    return result.DestinationID
}
```

---

## Configuration Reference

### routing.json

```json
{
  "default_strategy": "default",
  "strategies": {
    "default": {"priority": ["external", "internal"]},
    "internal_only": {"priority": ["internal"]}
  },
  "agent_restrictions": {
    "internal_only": ["planner", "supervisor", ...],
    "default": ["courier", ...]
  },
  "destination_categories": {
    "external": {"check_field": "type", "check_values": ["web"]},
    "internal": {"check_field": "type", "check_values": ["cli", "api"]}
  }
}
```

### destinations.json

```json
{
  "destinations": [
    {"id": "opencode", "type": "cli", "status": "active", "provides_tools": [...]},
    {"id": "gemini-api", "type": "api", "status": "inactive", "provides_tools": []}
  ]
}
```

Status determines availability. Type determines category. All configurable.

---

## What's Next

### DONE This Session
- ✅ Dynamic routing implemented and deployed
- ✅ All hardcoded destinations removed
- ✅ routing.json created with strategies
- ✅ Python files moved to legacy/
- ✅ Governor running with routing logs visible

### Remaining for Future Sessions

| Priority | Task | Notes |
|----------|------|-------|
| HIGH | Add courier destinations | Web platforms (chatgpt-web, claude-web) to destinations.json with type="web" |
| HIGH | Wire model scoring RPC | get_model_score_for_task in Supabase, used by router |
| MEDIUM | Rate limit checking | Router checks if destination at limit before selecting |
| MEDIUM | API output execution | For API runners without native tools, Governor parses output and executes |
| LOW | Courier runner impl | Implementation for web platform execution |

---

## Verified Working

**Governor logs show routing in action:**
```
[Router] Agent planner using strategy internal_only with priority [internal]
[Router] Selected destination opencode (category: internal, model: glm-5)
```

**To verify routing:**
```bash
sudo journalctl -u vibepilot-governor -f | grep Router
```

---

## Key Principle

**Everything is configurable. Nothing is hardcoded.**

- Want to add a destination? Add to destinations.json, set status=active
- Want to change routing priority? Edit routing.json strategies
- Want to restrict an agent? Add to internal_only list
- Want to disable a destination? Set status=inactive

Code doesn't need to change.
