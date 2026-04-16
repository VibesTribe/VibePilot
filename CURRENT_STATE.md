# VibePilot Current State - 2026-04-16

## Status: Fully operational. Schema deployed, vault stocked, worktrees WIRED AND LIVE.

### The Repo Situation

Two copies on disk, both synced to main:

| Location | Purpose | State |
|---|---|---|
| `~/vibepilot/` | RUNNING copy. Compiled binary + systemd service. | Current (main). Binary rebuilt Apr 16 00:05. |
| `~/VibePilot/` | DEVELOPMENT copy. Primary working directory. | Current (main). |

**GitHub main is current** -- all changes pushed.

---

### What's Running

- **Governor:** systemd user service, active (running since Apr 16 00:05)
  - Binary: `~/vibepilot/governor/governor` (compiled Apr 16, includes worktree wiring)
  - Service: `systemctl --user status vibepilot-governor`
  - Logs: `journalctl --user -u vibepilot-governor -f`
  - MCP servers: jcodemunch only (51 tools). jDocMunch + jDataMunch REMOVED.
  - Governor MCP server: disabled in config (ready to enable for SSE port 8081)
  - **Worktrees: ENABLED AND WIRED** -- base path `/home/vibes/VibePilot-work/`
  - **Connectors registered:** claude-code (cli), gemini-api (api), groq-api (api), nvidia-api (api)
- **Cloudflared tunnel:** live at vibestribe.rocks, sacred (don't touch)
- **Hermes agent:** accessible via dashboard chat through tunnel
- **Chrome CDP:** port 9222, bind mount active, user auto-logged into Gmail/Gemini/Sheets
- **TTS:** edge-tts (fast, free)

### Connectors (4 API + 7 web couriers)

| ID | Type | Status | Key Vault |
|---|---|---|---|
| claude-code | cli | active | none (local) |
| gemini-api | api | active | GEMINI_API_KEY |
| groq-api | api | active | GROQ_API_KEY |
| nvidia-api | api | active | NVIDIA_API_KEY |
| deepseek-api | api | **benched** | DEEPSEEK_API_KEY (out of credits) |
| openrouter-api | api | emergency_fallback | OPENROUTER_API_KEY |
| chatgpt-web | web | active | browser courier |
| claude-web | web | active | browser courier |
| gemini-web | web | active | browser courier |
| deepseek-web | web | active | browser courier |
| qwen-web | web | active | browser courier |
| mistral-web | web | active | browser courier |
| notegpt-web | web | active | browser courier |

### Models Config (16 models)

| Model | Provider | Key / Access | Rate Limit | Status |
|---|---|---|---|---|
| glm-5 | zhipu | Z.AI browser | none (courier) | active |
| gemini-2.5-flash | google | GEMINI_API_KEY | 15 RPM, 150 RPD | active |
| deepseek-chat | deepseek | DEEPSEEK_API_KEY | 30 RPM | **benched** (no credits) |
| deepseek-reasoner | deepseek | DEEPSEEK_API_KEY | 30 RPM | **benched** (no credits) |
| chatgpt-4o-mini | openai | browser courier | none | benched |
| claude-sonnet | anthropic | browser courier | none | benched |
| gemini-web | google | browser courier | none | active |
| copilot | microsoft | browser courier | none | active |
| deepseek-web | deepseek | browser courier | none | active |
| llama-3.3-70b-versatile | groq | GROQ_API_KEY | 30 RPM, 6000 RPD | active |
| llama-3.1-8b-instant | groq | GROQ_API_KEY | 30 RPM, 14400 RPD | active |
| qwen3-32b | groq | GROQ_API_KEY | 30 RPM, 1000 RPD | active |
| kimi-k2-instruct | groq | GROQ_API_KEY | 30 RPM, 1000 RPD | active |
| llama-4-maverick-17b | nvidia | NVIDIA_API_KEY | 10 RPM, 1000 RPD | active |
| deepseek-r1 | nvidia | NVIDIA_API_KEY | 10 RPM, 1000 RPD | active |
| qwen3-235b-a22b | nvidia | NVIDIA_API_KEY | 10 RPM, 1000 RPD | active |

### Supabase Schema (fully deployed)

- **111 migrations applied**, all RPCs live and verified
  - `get_model_performance()` -- returns heuristics by model
  - `get_model_score_for_task(model, type, category)` -- weighted scoring
  - `check_platform_availability()` -- returns `{"available": true}`
  - All maintenance/planner/revision/security RPCs deployed with idempotent drops
- **Secrets vault:** 10 keys stored (all encrypted AES-256-GCM)
  - GITHUB_TOKEN, GROQ_API_KEY, NVIDIA_API_KEY, OPENROUTER_API_KEY, GEMINI_API_KEY
  - DEEPSEEK_API_KEY, VIBEPILOT_GMAIL_EMAIL, VIBEPILOT_GMAIL_PASSWORD, SUPABASE_SERVICE_KEY, webhook_secret

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

**WARNING: ZAI/GLM subscription ends May 1.** After that, only the fallback chain above remains.

### Hardware: ThinkPad X220

- Intel i5-2520M (Sandy Bridge, no AVX2, no GPU)
- 16GB RAM (~10GB available)
- ~780GB disk free
- Phone WiFi tethered

---

## What Got Built (April 2026)

### 1. .context/ Knowledge Layer (three systems, zero overlap)

| System | File | Size | Covers |
|---|---|---|---|
| lean-ctx | `.context/map.md` | 51KB | Go function signatures only (compressed) |
| jCodeMunch | `.context/index.db` | 3.7MB | ALL code symbols: Go(811) + SQL(389) + YAML(334) + JSON(220) + Python(97) = 1974 |
| knowledge.db | `.context/knowledge.db` | 2.8MB | Prose+structure: 30 rules, 30 prompts, 15 configs, 3337 docs, 364 SQL schema objects, 17 pipeline stages |

| Other files | Size | Purpose |
|---|---|---|
| `.context/boot.md` | 15KB | Agent orientation (~3800 tokens boot) |

### 2. Governor (Go, ~15k lines, 16 packages)

- Event-driven runtime with session factory, agent pool, connection router
- MCP client connects 51 tools from jcodemunch (jDocMunch/jDataMunch removed)
- MCP server exposes governor tools via stdio/SSE
- 3-layer memory (short/mid/long-term) in Supabase
- Context compaction (auto-summarizes long sessions)
- **Gitree** -- full git abstraction (branch, commit, merge, rebase, conflict detection, protected branches)
- **Worktrees WIRED INTO PIPELINE:**
  - `handleTaskAvailable`: CreateWorktree + BootstrapWorktree on task claim
  - `executeTask`: passes worktree_path + repo_path to agent session
  - `handleTaskReview`: RemoveWorktree on fail/needs_revision/reroute
  - `handleTaskTesting`: ShadowMerge before real merge (conflict detection), RemoveWorktree after merge
  - `failTask`: RemoveWorktree cleanup on any failure
  - BootstrapWorktree symlinks: governor/config/*.json, .hermes.md, .context/*, prompts/, pipelines/
- **Model router** -- UsageTracker with per-model cooldowns, rate limiting, cascade fallback
- **Connectors** -- 4 active API/CLI connectors + 7 web couriers

### 3. Supabase Schema (111 migrations)

- Core tables: task_runs, task_events, agent_sessions, orchestrator_events
- Memory tables: memory_sessions, memory_project, memory_rules (migration 110)
- Security: security_audit_log, secrets_vault (AES-256-GCM encrypted)
- RPCs: check_platform_availability, get_model_performance, get_model_score_for_task, plus maintenance/planner/revision functions
- All idempotent with inline per-function DROPs

### 4. Worktree Architecture (LIVE)

- **One Worktree Per Agent:** isolated directories under `~/VibePilot-work/`
- **Shadow Merge:** dry-run conflict detection before real merge (handlers_testing.go)
- **Bootstrap Symlinks:** auto-links governor/config/, .hermes.md, .context/, prompts/, pipelines/ into worktrees
- **Standardized Branches:** `task/{slice}/{number}` naming convention
- **Auto-cleanup:** `CleanAllWorktrees()` on governor shutdown, `RemoveWorktree()` on task fail/merge
- **Fallback:** if worktree creation fails, falls back to legacy single-dir branch mode

### 5. Architecture Documentation

- `docs/ARCHITECTURE_TASK_LIFECYCLE.md` -- full task flow (current vs target), worktree wiring map
- `docs/STARTUP_GUIDE.md` -- setup, rebuild, restore instructions
- `.hermes.md` -- enforcement rules + three-system knowledge map
- `backup/` -- local config backups with templated secrets

---

## Rate Limits to Watch

- **GLM via Z.AI:** max 3 concurrent sessions or hit limits. Nemotron-3-free from OpenRouter as fallback.

## Known Issues

- **ZAI subscription ends May 1** -- need to test full fallback chain before then
- **Browser tool broken** -- Hermes browser_navigate uses Playwright headless (no cookies). Real Chrome via CDP works fine.
- **Deepseek API out of credits** -- use NVIDIA NIM deepseek-r1 or deepseek-web courier instead
- **No end-to-end pipeline test** -- YAML pipeline written but never run against real governor
- **No visual QA agent** -- courier pattern for dashboard screenshots not built yet
- **GLM-5 has no api_key_ref** -- accessed via Z.AI browser, not direct API
- **copilot-web marked unavailable** -- requires Microsoft account
- **Dashboard failures/mergeCandidates always empty** -- not populated from Supabase
- **Task status `failed`/`escalated` maps to `pending`** -- distinction invisible in dashboard
- **Per-task costUsd hardcoded 0** -- ROI shows aggregate but not per-task

---

**Last Updated:** 2026-04-16
