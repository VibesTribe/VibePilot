# VibePilot Current State

**Required reading: FOUR files**
1. **THIS FILE** (`CURRENT_STATE.md`) - What, where, how, current state
2. **`docs/GOVERNOR_REBUILD_PLAN.md`** - What we did, how it works now
3. **`docs/core_philosophy.md`** - Strategic mindset and principles
4. **`docs/prd_v1.4.md`** - Complete system specification

**Read all four → Know everything → Do anything**

---

**Last Updated:** 2026-02-25
**Updated By:** GLM-5 - Session 30
**Branch:** `go-governor` (all changes pushed)

---

# SESSION 30: GOVERNOR REBUILD COMPLETE

## The Achievement

| Metric | Before | After | Change |
|--------|--------|-------|--------|
| **Total Go lines** | 8,287 | 4,517 | **-45%** |
| Agent logic in Go | ~5,800 | 0 | **-100%** |
| Modules | 18 | 9 | **-50%** |

## What Changed

**Deleted (agent logic moved to prompts):**
- `orchestrator/`, `dispatcher/`, `council/`, `planner/`, `analyst/`, `consultant/`, `agent/`
- `server/` (dashboard served by Vercel)
- `config/` (replaced by runtime/config.go)
- `pool/`, `tester/`, `courier/`, `janitor/`, `sentry/`, `throttle/`, `visual/`

**Built new:**
- `runtime/` (1,149 lines) - event loop, session, config loading
- `tools/` (956 lines) - git, db, vault, file, sandbox, web tools
- `destinations/` (170 lines) - CLI and API runners

**Simplified:**
- `db/supabase.go`: 1,280 → 198 lines (generic REST + RPC)

## Final Structure

```
governor/ (4,517 lines)
├── cmd/governor/main.go      378  - Event-driven main
├── internal/
│   ├── runtime/             1149  - Event loop, session, config
│   ├── tools/                956  - All tool implementations
│   ├── maintenance/          759  - File ops, sandbox, backup
│   ├── db/                   313  - Generic query/RPC
│   ├── vault/                253  - Secret retrieval
│   ├── gitree/               252  - Git operations
│   ├── destinations/         170  - CLI/API runners
│   └── security/             130  - Leak detection
└── pkg/types/                157  - Shared types
```

## How It Works Now

```
1. PollingWatcher checks DB for state changes (tasks, plans, commands)
2. EventRouter dispatches to correct agent
3. SessionFactory loads: prompt.md + tools + destination
4. LLM runs with tool calling loop (max 10 turns)
5. ToolRegistry validates: 2-3 tools per agent, param schemas
6. AgentPool controls parallelism: 8/module, 160 total
```

## Where Intelligence Lives

| What | Where |
|------|-------|
| Agent behavior | `config/prompts/*.md` |
| Agent definitions | `config/agents.json` |
| Tool definitions | `config/tools.json` |
| System settings | `config/system.json` |
| Model registry | `config/models.json` |
| Platforms | `config/destinations.json` |

## Key Principles

1. **Everything swappable** - models, platforms, database, git host
2. **No hardcoded decisions** - LLM decides, Go executes
3. **2-3 tools per agent** - enforced at runtime
4. **Generic RPC** - `CallRPC(name, params)` with allowlist
5. **Event-driven** - single poller, no scattered loops
6. **Lean code** - fits in LLM context for easy modification

---

# NEXT STEPS

## Testing Needed

1. Build and run the new governor
2. Verify it detects events from DB
3. Test agent sessions with real prompts
4. Confirm tool execution works
5. Verify dashboard still works (reads from DB)

## Potential Improvements

1. Add Supabase real-time subscriptions (instead of polling)
2. Add more destination runners (API implementations)
3. Add courier driver for web platform execution
4. Fine-tune prompts based on actual behavior
5. Add metrics/monitoring

---

# QUICK COMMANDS

| Command | Action |
|---------|--------|
| `cd ~/vibepilot/governor && go build ./...` | Build governor |
| `cd ~/vibepilot/governor && go run ./cmd/governor` | Run governor |
| `cd ~/vibepilot && git log --oneline -10` | Recent commits |
| `cat config/system.json` | System settings |
| `cat config/tools.json` | Tool definitions |
| `cat config/agents.json` | Agent definitions |

---

# ACTIVE MODELS

| Model | Destination | Status |
|-------|-------------|--------|
| glm-5 | opencode | ✅ ACTIVE |
| kimi | kimi CLI | Subscription ended |
| gemini-api | gemini-api | Quota exhausted |
| deepseek | deepseek-api | Credit needed |

---

# BRANCH STATUS

| Repo | Branch | Status |
|------|--------|--------|
| vibepilot | `go-governor` | Rebuild complete, needs testing |
| vibepilot | `main` | Production |
| vibeflow | `main` | Dashboard (Vercel auto-deploys) |

---

# FOR NEXT SESSION

**Read these first:**
1. `CURRENT_STATE.md` (this file)
2. `docs/GOVERNOR_REBUILD_PLAN.md` (implementation details)
3. `governor/cmd/governor/main.go` (how it starts)
4. `governor/internal/runtime/` (how it works)

**What to do:**
1. Test the new governor
2. Fix any issues found
3. Don't add agent logic to Go - keep it in prompts
4. Keep it lean and swappable
