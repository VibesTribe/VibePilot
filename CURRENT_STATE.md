# VibePilot Current State - 2026-04-15 (night)

## Status: Fully operational. Schema deployed, vault stocked, connectors wired, worktrees live.

### The Repo Situation

Two copies on disk, both synced to main:

| Location | Purpose | State |
|---|---|---|
| `~/vibepilot/` | RUNNING copy. Compiled binary + systemd service. | Current (main). Binary rebuilt Apr 15 20:19. |
| `~/VibePilot/` | DEVELOPMENT copy. Primary working directory. | Current (main). |

**GitHub main is current** -- 12 commits pushed April 15 evening session.

---

### What's Running

- **Governor:** systemd user service, active (running since Apr 15 20:19)
  - Binary: `~/vibepilot/governor/governor` (compiled Apr 15, includes worktree wiring + MCP + memory)
  - Service: `systemctl --user status vibepilot-governor`
  - Logs: `journalctl --user -u vibepilot-governor -f`
  - MCP servers: jcodemunch (52 tools) + jdocmunch (15 tools) = 67 tools connected
  - Governor MCP server: disabled in config (ready to enable for SSE port 8081)
  - **Worktrees: ENABLED** -- base path `/home/vibes/VibePilot-work/`, auto-cleanup on shutdown
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
  - `get_model_performance()` -- returns heuristics by model (uses correct `preferred_model` column)
  - `get_model_score_for_task(model, type, category)` -- weighted scoring (40% base + 20% recency + 25% heuristic + 15% strengths)
  - `check_platform_availability()` -- verified returns `{"available": true}`
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

### 1. .context/ Knowledge Layer

| File | Size | Purpose |
|---|---|---|
| `.context/boot.md` | 14KB | Agent orientation. Tier 0 rules first, then codebase map. |
| `.context/knowledge.db` | 2.5MB | SQLite: rules, prompts, configs, doc sections. |
| `.context/map.md` | 51KB | Full code map (functions, types, files, packages). |
| `.context/index.db` | 3.7MB | jCodeMunch index for MCP tool queries. |

### 2. Governor (Go, ~15k lines, 16 packages)

- Event-driven runtime with session factory, agent pool, connection router
- MCP client connects 67 tools from jcodemunch + jdocmunch
- MCP server exposes governor tools via stdio/SSE
- 3-layer memory (short/mid/long-term) in Supabase
- Context compaction (auto-summarizes long sessions)
- **Gitree** -- full git abstraction (branch, commit, merge, rebase, conflict detection, protected branches)
- **Worktrees** -- parallel agent isolation (wired into handlers, shadow merge, bootstrap symlinks, auto-cleanup)
- **Model router** -- UsageTracker with per-model cooldowns, rate limiting, cascade fallback, 80% buffer
- **Connectors** -- 4 active API/CLI connectors + 7 web couriers, all configurable via connectors.json

### 3. Supabase Schema (111 migrations)

- Core tables: task_runs, task_events, agent_sessions, orchestrator_events
- Memory tables: memory_sessions, memory_project, memory_rules (migration 110)
- Security: security_audit_log, secrets_vault (AES-256-GCM encrypted)
- RPCs: check_platform_availability, get_model_performance, get_model_score_for_task, plus maintenance/planner/revision functions
- All idempotent with inline per-function DROPs

### 4. Worktree Architecture (Gemini Strategic Patterns)

- **One Worktree Per Agent:** isolated directories under `~/VibePilot-work/`
- **Shadow Merge:** dry-run conflict detection before real merge
- **Bootstrap Symlinks:** auto-links `.governor_env`, `models.json`, `vibepilot.yaml` into new worktrees
- **Standardized Branches:** `task/{id}-{slug}` naming convention
- **Auto-cleanup:** `CleanAllWorktrees()` on governor shutdown

---

## Known Issues

- **ZAI subscription ends May 1** -- need to test full fallback chain before then
- **Browser tool broken** -- Hermes browser_navigate uses Playwright headless (no cookies). Real Chrome via CDP works fine.
- **Deepseek API out of credits** -- use NVIDIA NIM deepseek-r1 or deepseek-web courier instead
- **No end-to-end pipeline test** -- YAML pipeline written but never run against real governor
- **No visual QA agent** -- courier pattern for dashboard screenshots not built yet
- **GLM-5 has no api_key_ref** -- accessed via Z.AI browser, not direct API
- **copilot-web marked unavailable** -- requires Microsoft account

---

**Last Updated:** 2026-04-15 (night)
