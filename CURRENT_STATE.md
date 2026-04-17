# VibePilot Current State - 2026-04-17

## Status: Pipeline working end-to-end. Dashboard fixes in migration 119 (needs applying).

### What needs to happen on wake:
1. Apply migration 119 in Supabase SQL editor (copy from GitHub `migrations/119_fix_claim_task_and_create_task_run.sql`)
2. Rebuild governor binary (already built locally, needs copy)
3. Test pipeline -- dashboard should now show active agent on tasks

### The Repo Situation

Two copies on disk, both synced to main:

| Location | Purpose | State |
|---|---|---|
| `~/vibepilot/` | RUNNING copy. Compiled binary + systemd service. | Binary from this session, includes \r fix |
| `~/VibePilot/` | DEVELOPMENT copy. Primary working directory. | Current (main), all fixes committed |

**GitHub main is current.** Latest commit: `bc839255` (migration 119 + \r fix)

---

### What's Running

- **Governor:** systemd user service
  - Binary: `~/vibepilot/governor/governor`
  - Service: `systemctl --user status vibepilot-governor`
  - Logs: `journalctl --user -u vibepilot-governor -f`
  - MCP servers: jcodemunch (52 tools). jDocMunch removed. jDataMunch disabled.
  - Connectors registered: hermes (cli), opencode (cli), gemini-api, groq-api, nvidia-api
- **Hermes Agent:** v0.9.0 (updated from v0.8.0)
  - v0.8.0 bug FIXED: empty response recovery for GLM-5 (commit d6785dc4)
  - v0.9.0 also fixes: partial streamed content on connection failure
- **Cloudflared tunnel:** live at vibestrike.rocks, sacred (don't touch)
- **Chrome CDP:** port 9222, bind mount active
- **TTS:** edge-tts (fast, free)

### Pipeline Proven Working (April 2026)

Full pipeline proven 4 times:
```
Plan inserted → Planner creates plan → Supervisor approves → Tasks created → Task claimed by glm-5 → Hermes executes → Task moved to review
```

**Latest run:** Plan f0245756 → task d69dac9e completed execution, moved to `review` status. The /hello endpoint was actually written to server.go by hermes.

**Timing:** ~4-5 minutes total (planner ~2min at GLM-5 speed, task execution ~2min)

### Fixes Made This Session (All Committed to main)

| What | Why | Where |
|---|---|---|
| Hermes v0.8.0 → v0.9.0 | GLM-5 empty response bug | `hermes update` |
| `\r` strip in extractJSON | GLM-5 includes carriage returns in JSON | `decision.go` line 258 |
| `assigned_to = p_model_id` in claim_task | Dashboard shows agent via `tasks.assigned_to` | Migration 119 |
| `routing_flag` + `routing_flag_reason` in claim_task | Dashboard location badge | Migration 119 |
| `create_task_run` RPC | Dashboard token counts and ROI panel | Migration 119 |

### Supabase Schema (119 migrations, migration 119 NOT YET APPLIED)

- `claim_task(UUID, TEXT, TEXT, TEXT, TEXT)` -- sets `assigned_to`, `processing_by`, `routing_flag`
- `transition_task(UUID, TEXT, TEXT)` -- clears `processing_by`, keeps `assigned_to`
- `create_task_run(UUID, TEXT, ...)` -- NEW, records tokens/costs/ROI

### Dashboard Contract (See `docs/HOW_DASHBOARD_WORKS.md`)

**CRITICAL: Dashboard is READ-ONLY. Fix Go/Supabase, never the dashboard.**

Dashboard expects from Supabase:
1. `tasks.assigned_to` = model ID (e.g. "glm-5") ← Fixed in migration 119
2. `task_runs` records with tokens/costs ← Fixed in migration 119
3. `tasks.slice_id` = slice grouping ← Already set by planner in validation.go
4. `tasks.routing_flag` = "internal"/"mcp"/"web" ← Fixed in migration 119
5. `task_runs` cost/savings columns ← Already calculated by Go
6. `models.status` = "active"/"paused" ← Already in models table
7. `orchestrator_events` for timeline ← NOT YET IMPLEMENTED (empty timeline OK)

### Key Dashboard Data Flow
```
Governor writes to Supabase → Dashboard reads via Realtime → User sees updates
tasks.assigned_to = "glm-5" → adaptVibePilotToDashboard → owner: "agent.glm-5" → shows agent icon
task_runs tokens/costs → calculateROI() → ROI panel
```

---

## Known Issues

### Fixed This Session
- ~~Hermes v0.8.0 GLM-5 empty response~~ → Updated to v0.9.0
- ~~`\r` in JSON crashing parser~~ → Stripped in extractJSON
- ~~`assigned_to` never set~~ → claim_task now sets it to model ID
- ~~`create_task_run` RPC missing~~ → Created in migration 119

### Still To Fix
- **Orchestrator events not written** — Go never inserts into `orchestrator_events`. Dashboard timeline empty.
- **Startup missed events** — plans inserted before governor subscribes sit in `draft`. Need boot scan for stuck plans. NOT YET IMPLEMENTED.
- **Worktree branch leak** — task operations switch main repo branch instead of isolated worktree.
- **ZAI subscription ends May 1** — test fallback chain before then.
- **`received` status never used** — Dashboard expects `in_progress → received → review` flow. Governor jumps straight to execution.

### NOT Issues (False Alarms)
- CLIRunner is NOT broken — it's agent-agnostic
- decision.go extractJSON is NOT broken (was just missing \r strip)
- Governor pipeline logic is NOT broken
- The problem was hermes v0.8.0 + missing RPCs, not the governor

---

## Hardware: ThinkPad X220
- Intel i5-2520M (Sandy Bridge, no AVX2, no GPU)
- 16GB RAM (~10GB available)
- ~780GB disk free
- Phone WiFi tethered

**Last Updated:** 2026-04-17 05:30 (after session fixes, before sleep)
