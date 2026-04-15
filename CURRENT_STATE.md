# VibePilot Current State - 2026-04-15

## Status: MCP Server + Memory System Built, Governor Rebuilt

### The Repo Situation

Two copies on disk, both synced to main:

| Location | Purpose | State |
|---|---|---|
| `~/vibepilot/` | RUNNING copy. Compiled binary + systemd service. | Current (main). Binary rebuilt Apr 15. |
| `~/VibePilot/` | DEVELOPMENT copy. Primary working directory. | Current (main). |

**GitHub main is current** -- all work pushed April 15.

---

### What's Running

- **Governor:** systemd user service, active
  - Binary: `~/vibepilot/governor/governor` (compiled Apr 15, includes MCP server + memory)
  - Service: `systemctl --user status vibepilot-governor`
  - Logs: `journalctl --user -u vibepilot-governor -f`
  - MCP servers: jcodemunch (52 tools) + jdocmunch (15 tools) = 67 tools connected
  - Governor MCP server: disabled in config (ready to enable for SSE port 8081)
- **Cloudflared tunnel:** live at vibestribe.rocks, sacred (don't touch)
- **Hermes agent:** accessible via dashboard chat through tunnel
- **Chrome CDP:** port 9222 for browser automation
- **TTS:** edge-tts (fast, free)

### Hardware: ThinkPad X220

- Intel i5-2520M (Sandy Bridge, no AVX2, no GPU)
- 16GB RAM (~10GB available)
- ~780GB disk free
- Phone WiFi tethered (planning ethernet + headless mode)

---

## What Got Built (April 2026)

### 1. .context/ Knowledge Layer (new)

Replaces scattered docs with a 3-tier system agents can actually query:

| File | Size | Purpose |
|---|---|---|
| `.context/boot.md` | 12KB (~2838 tok) | Agent orientation. Tier 0 rules first, then codebase map. |
| `.context/knowledge.db` | 2.3MB | SQLite: 24 rules, 30 prompts, 15 configs, 2972 doc sections. |
| `.context/map.md` | 47KB | Full code map (functions, types, imports). |
| `.context/index.db` | 3.5MB | Code index (functions, dependencies). |
| `.context/tools/tier0-static.md` | 4.5KB | Hand-crafted single source of truth for all rules. |
| `.context/tools/build-knowledge-db.py` | Pure python3 | Builds knowledge.db from tier0 + supplementary sources. |
| `.context/build.sh` | Shell | Rebuilds all `.context/` files. Runs as pre-commit hook. |

**Tier 0** = non-negotiable rules baked into boot.md (always loaded). Includes:
- 4 principles, 6 absolute rules, 3 operational rules, 5 human boundaries
- Correct roles: Supervisor (plan review + output review + researcher approvals), Council (complex plans + architecture), Human (visual UX + architecture yes/no only)
- Merge system: fully automated, zero human involvement

### 2. Doc Alignment (all contradictions fixed)

Every doc in the repo now tells the same story as tier0-static.md:
- `agents/agent_definitions.md` -- flow diagram corrected (Supervisor before Council, auto-merge)
- `VIBEPILOT_WHAT_YOU_NEED_TO_KNOW.md` -- human role, supervisor output review, dashboard description
- `ARCHITECTURE.md` -- removed "approval" gate from task state machine
- `config/prompts/supervisor.md` -- added plan review, auto-merge
- `config/prompts/council.md` -- complex plans + architecture only, Supervisor-only escalation

### 3. YAML Pipeline (DAG)

`governor/config/pipelines/code-pipeline.yaml` -- event-driven pipeline config:
- Flow A: Plan -> Supervisor review -> (Council if complex) -> Create tasks
- Flow B: Execute task -> Supervisor output review -> Testing -> Auto-merge
- Flow C: System research -> Supervisor -> (Council if architecture) -> Human yes/no
- Conditional branches via `when:` expressions
- Human approval gates ONLY for visual UI/UX and architecture decisions

### 4. Gitree (git branch management)

`governor/internal/gitree/gitree.go` (484 lines) -- solves parallel agent git conflicts:
- Each task gets an orphan branch (`task/T001`) with clean empty workspace
- Module branches (`TEST_MODULES/sliceID`) aggregate completed task branches
- Protected branches (main) can't be directly modified
- Branch operations: create, commit output, merge, delete, clear for retry
- Agents never touch main -- task branches merge to module branches, modules merge to main
- `CreateBranchFrom()` creates task branches FROM a source branch (not orphan)
- `CommitOutput()` writes files and pushes atomically
- `ClearBranch()` resets a branch for retry without deleting it

### 5. MCP Server Phase 2 (new Apr 15)

`governor/internal/mcp/governor_server.go` -- exposes governor tools AS an MCP server:
- Any external agent (Claude Code, Codex, OpenCode) can connect via MCP protocol
- Two transport modes: stdio (pipe) and SSE (HTTP on port 8081)
- Auto-registers all 20+ tools from the tool registry
- Config in system.json under `governor_mcp` (disabled by default, ready to enable)
- Graceful shutdown wired into main.go

### 6. 3-Layer Memory System (new Apr 15)

`governor/internal/memory/service.go` + `docs/supabase-schema/110_memory_system.sql`:
- Layer 1 (short-term): `memory_sessions` -- per-agent-run context, TTL auto-expires
- Layer 2 (mid-term): `memory_project` -- project-scoped key/value state
- Layer 3 (long-term): `memory_rules` -- learned rules with confidence scoring
- Go service: StoreShortTerm/GetShortTerm, StoreProjectState/GetProjectState, StoreRule/GetRulesByCategory
- CleanExpired maintenance for session TTL
- **Migration 110 applied** -- tables live in Supabase

### 7. Context Compaction (new Apr 15)

`governor/internal/memory/compactor.go` -- automatic session summary generation:
- After every session.Run(), compresses result into SessionSummary struct
- Tries all decision types: supervisor, task runner, planner, council, test
- Stores summaries in short-term memory (1hr TTL)
- BuildCompactionContext() feeds recent history to next agent's prompt
- Non-blocking, never errors -- compaction failures don't break sessions
- Wired into task execution, supervisor review, and council vote handlers

### 8. Git Worktrees (new Apr 15)

`governor/internal/gitree/worktree.go` -- parallel agent workspace isolation:
- CreateWorktree(taskID, branch) -- each task gets its own checkout
- RemoveWorktree(taskID) -- cleanup after completion
- ListWorktrees, PruneWorktrees, CleanAllWorktrees
- Base path: /home/vibes/VibePilot-work/ (configurable)
- Disabled by default in system.json, ready to enable

### 9. Supabase Schema

109 migration files in `docs/supabase-schema/`. Core tables: tasks, models, platforms.
Schema versioning: v1.0 -> v1.1 (routing) -> v1.2 (platforms) -> v1.3 (config JSONB) -> v1.4 (ROI enhanced).
+ 93 numbered migrations for incremental changes.

---

## Go Governor Source

14,419 lines of Go across these packages:

| Package | Purpose |
|---|---|
| `internal/gitree` | Git branch management (orphan branches, parallel agents) |
| `internal/dag` | YAML pipeline engine (configurable DAGs) |
| `internal/core` | State machine, checkpointing |
| `internal/db` | Supabase client, RPCs |
| `internal/runtime` | Router, agents, sessions |
| `internal/security` | Leak detection (API keys, secrets) |
| `internal/vault` | Supabase vault access |
| `internal/connectors` | API connector implementations |
| `internal/realtime` | Supabase realtime subscriptions |
| `internal/maintenance` | System maintenance operations |
| `internal/mcp` | MCP protocol support (client + server) |
| `internal/memory` | 3-layer memory service (short/mid/long-term) + session compaction |
| `internal/webhooks` | Webhook handling |
| `internal/tools` | Tool registry |
| `cmd/governor/` | Main entry, handlers (plan, task, council, testing, research, maintenance) |

---

## Config Layer

| File | Lines | Purpose |
|---|---|---|
| `config/roles.json` | 198 | 13 role definitions (Supervisor, Council, Courier, etc.) |
| `config/agents.json` | 285 | Agent definitions with capabilities |
| `config/models.json` | 146 | Model definitions |
| `config/connectors.json` | 183 | API connector configs |
| `config/destinations.json` | 490 | Destination platforms |
| `config/platforms.json` | 649 | Web AI platform configs |
| `config/routing.json` | 327 | Strategy configs (currently `kimi_priority` -- stale) |
| `config/system.json` | 50 | Runtime settings (concurrency, timeouts) |
| `config/plan_lifecycle.json` | 124 | Plan states, consensus rules |
| `config/tools.json` | 176 | Tool definitions |
| `config/skills.json` | 236 | Skill definitions |

---

## Research Done

- `research/2026-04-14-free-model-rolodex.md` -- 7 free API providers verified (Groq, Google, OpenRouter, SambaNova, NVIDIA NIM, SiliconFlow, HuggingFace). Only Google AI Studio key exists.
- `research/2026-04-08-journeykits-landscape-analysis.md` -- 95 kits scanned, 20 mapped to VibePilot gaps.

---

## Dashboard

https://vibeflow-dashboard.vercel.app/ (sacred, deployed from ~/vibeflow)
Source: `~/vibeflow/` (173MB, Vercel auto-deploy)

---

## On Disk

| Path | Size | What |
|---|---|---|
| `~/vibepilot/` | 165MB | RUNNING copy (main branch, compiled binary) |
| `~/VibePilot/` | 165MB | DEVELOPMENT copy (research-update-april2026, all new work) |
| `~/vibeflow/` | 173MB | Dashboard (Vercel auto-deploy) |
| `~/vibepilot-server/` | 60KB | Restart scripts |
| `~/browser-use-env/` | 429MB | Browser Use (Playwright + Chrome CDP) |

**Stopped/disabled:**
- Ollama daemon (stopped, disabled, ready if needed)
- No local models pulled (x220 can't run useful inference)

---

## How to Start Governor

```bash
# Check status
systemctl --user status vibepilot-governor

# View logs
journalctl --user -u vibepilot-governor -f

# Restart
systemctl --user restart vibepilot-governor

# Bootstrap credentials in:
# ~/.config/systemd/user/vibepilot-governor.service.d/override.conf
```

---

**Last Updated:** 2026-04-15
