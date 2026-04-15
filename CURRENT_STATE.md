# VibePilot Current State - 2026-04-15 (evening)

## Status: Keys secured, Hermes fallback chain solid, Governor configs stale

### The Repo Situation

Two copies on disk, both synced to main:

| Location | Purpose | State |
|---|---|---|
| `~/vibepilot/` | RUNNING copy. Compiled binary + systemd service. | Current (main). Binary rebuilt Apr 15. |
| `~/VibePilot/` | DEVELOPMENT copy. Primary working directory. | Current (main). |

**GitHub main is current** -- all work pushed April 15.

---

### What's Running

- **Governor:** systemd user service, active (running since Apr 15 00:15)
  - Binary: `~/vibepilot/governor/governor` (compiled Apr 15, includes MCP server + memory)
  - Service: `systemctl --user status vibepilot-governor`
  - Logs: `journalctl --user -u vibepilot-governor -f`
  - MCP servers: jcodemunch (52 tools) + jdocmunch (15 tools) = 67 tools connected
  - Governor MCP server: disabled in config (ready to enable for SSE port 8081)
- **Cloudflared tunnel:** live at vibestribe.rocks, sacred (don't touch)
- **Hermes agent:** accessible via dashboard chat through tunnel
- **Chrome CDP:** port 9222, bind mount active, user auto-logged into Gmail/Gemini/Sheets
  - Wrapper `/usr/bin/google-chrome-stable` includes both `--remote-debugging-port=9222` and `--user-data-dir=$HOME/.config/chrome-debug`
  - `browser_navigate` uses Playwright HEADLESS (no cookies). For logged-in sites use Python Playwright `connect_over_cdp`.
- **TTS:** edge-tts (fast, free)

### Hermes Fallback Chain (8 tiers, all free)

```
Primary:  Gemini 2.5 Flash (Google AI Studio)
  -> Groq llama-3.1-8b-instant
  -> Groq compound (agentic)
  -> NVIDIA NIM gemma-3-4b-it
  -> NVIDIA NIM llama-3.3-70b-instruct
  -> OpenRouter gemma-4-31b:free
  -> OpenRouter nemotron-3-super:free
  -> OpenRouter qwen3-coder-480b:free
  -> STOP (no local fallback)
```

### Free API Keys (all verified working)

| Provider | Key | Models | Status |
|---|---|---|---|
| Google AI Studio | in .env | Gemini 2.5/2.0/3.0 Flash | WORKING, primary |
| Groq | in .env | llama-3.1-8b, compound, qwen3-32b, llama-4-scout | WORKING |
| NVIDIA NIM | in .env | 131 models (DeepSeek-v3.2, Llama-4, Qwen3-coder) | VERIFIED, wired |
| OpenRouter | in .env | Free suffix models only (-17c balance) | WORKING |

**NOT FREE:** SiliconFlow, SambaNova, Together AI -- removed from consideration.

**WARNING: ZAI/GLM subscription ends May 1.** Current session uses GLM-5.1 via ZAI. After May 1, only the fallback chain above remains.

### Hardware: ThinkPad X220

- Intel i5-2520M (Sandy Bridge, no AVX2, no GPU)
- 16GB RAM (~10GB available)
- ~780GB disk free
- Phone WiFi tethered

---

## What Got Built (April 2026)

### 1. .context/ Knowledge Layer

| File | Size | Purpose |
|---|---|---|
| `.context/boot.md` | 12KB | Agent orientation. Tier 0 rules first, then codebase map. |
| `.context/knowledge.db` | 2.3MB | SQLite: 24 rules, 30 prompts, 15 configs, 2972 doc sections. |
| `.context/map.md` | 47KB | Full code map (functions, types, imports). |
| `.context/index.db` | 3.5MB | Code index (functions, dependencies). |
| `.context/tools/tier0-static.md` | 4.5KB | Hand-crafted single source of truth for all rules. |

### 2. Doc Alignment (all contradictions fixed)
### 3. YAML Pipeline (DAG) -- `governor/config/pipelines/code-pipeline.yaml`
### 4. Gitree (git branch management) -- `governor/internal/gitree/gitree.go` (484 lines)
### 5. MCP Server Phase 2 -- `governor/internal/mcp/governor_server.go`
### 6. 3-Layer Memory System -- `governor/internal/memory/service.go` + migration 110
### 7. Context Compaction -- `governor/internal/memory/compactor.go`
### 8. Git Worktrees -- `governor/internal/gitree/worktree.go`

---

## Governor Config Files -- STALE, NEED UPDATE

These files are read by the governor binary but contain outdated data:

| File | Problem |
|---|---|
| `config/models.json` | 10 models, mostly paid (GPT-4o, Claude, Kimi). No free models listed. |
| `config/connectors.json` | 15 connectors, but no NVIDIA NIM. Groq has no base_url. |
| `config/routing.json` | current_strategy = `kimi_priority` (stale). Needs cascade. |

**This is the next critical task (#2-3 in TODO).**

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
| `internal/memory` | 3-layer memory service + session compaction |
| `internal/webhooks` | Webhook handling |
| `internal/tools` | Tool registry |
| `cmd/governor/` | Main entry, handlers |

---

## Dashboard

https://vibeflow-dashboard.vercel.app/ (sacred, deployed from ~/vibeflow)
Source: `~/vibeflow/` (173MB, Vercel auto-deploy)

---

## On Disk

| Path | Size | What |
|---|---|---|
| `~/vibepilot/` | 165MB | RUNNING copy (main branch, compiled binary) |
| `~/VibePilot/` | 165MB | DEVELOPMENT copy (main, all work) |
| `~/vibeflow/` | 173MB | Dashboard (Vercel auto-deploy) |
| `~/vibepilot-server/` | 60KB | Restart scripts |
| `~/browser-use-env/` | 429MB | Browser Use (Playwright + Chrome CDP) |

**Stopped/disabled:**
- Ollama daemon (stopped, disabled -- ABANDONED)

---

---

## ACTIVE WORK: Migration 111 (42 Missing RPCs)

**Status:** IN PROGRESS. Migration written but has schema mismatches. Must rewrite before applying.

**File:** `docs/supabase-schema/111_missing_rpcs.sql` (WRITTEN but WRONG column refs)
**Detailed state:** `docs/SESSION_STATE_111.md` (READ THIS FIRST if resuming)

### TL;DR
Go governor calls 50 Supabase RPCs. Only 8 exist. Need to create 42 missing ones.
Migration 111 was written but references wrong column names. Key fixes needed:
- `maintenance_commands`: column is `command_type` not `type`
- `council_reviews`: has `model_id` not `reviewer_model`, no `reasoning`/`mode`
- `failure_records`: no `details` column
- `plans`: has `review_notes` not `latest_feedback`, no `tasks_needing_revision`
- `tasks`: has `failure_notes` but no `last_error`/`last_error_at`
- `learned_heuristics`: no unique constraint on (task_type, preferred_model) - ON CONFLICT will fail

### After migration is fixed:
1. Apply to Supabase
2. Verify all 50 RPCs
3. Wire MemoryService into main.go
4. Rebuild binary
5. Push to GitHub

---

**Last Updated:** 2026-04-15 (late evening - migration 111 in progress)
