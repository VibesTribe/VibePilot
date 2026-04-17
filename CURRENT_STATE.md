# VibePilot Current State - 2026-04-17

## Status: Pipeline proven end-to-end. Hermes v0.8.0 has known GLM-5 bug — updating to v0.9.0 next.

### The Repo Situation

Two copies on disk, both synced to main:

|| Location | Purpose | State |
|---|---|---|
| `~/vibepilot/` | RUNNING copy. Compiled binary + systemd service. | Binary from Apr 16, includes task_runner fix. |
| `~/VibePilot/` | DEVELOPMENT copy. Primary working directory. | Current (main), all uncommitted hacks discarded. |

**GitHub main is current.** Committed changes are safe (SQL migrations + one Go fix). All panicked rewrites to CLIRunner/decision.go/recovery were discarded — those were bandaids for a hermes bug already fixed upstream.

---

### What's Running

- **Governor:** systemd user service
  - Binary: `~/vibepilot/governor/governor`
  - Service: `systemctl --user status vibepilot-governor`
  - Logs: `journalctl --user -u vibepilot-governor -f`
  - MCP servers: jcodemunch (52 tools). jDocMunch removed. jDataMunch disabled.
  - Connectors registered: hermes (cli), opencode (cli), gemini-api, groq-api, nvidia-api
- **Hermes Agent:** v0.8.0, 100 commits behind (v0.9.0 available)
  - **Known bug in v0.8.0:** Empty response recovery broken for reasoning models (GLM-5, Qwen, mimo). Fixed in commit d6785dc4 / v0.9.0.
  - Update command: `hermes update`
- **Cloudflared tunnel:** live at vibestribe.rocks, sacred (don't touch)
- **Chrome CDP:** port 9222, bind mount active, user auto-logged into Gmail/Gemini/Sheets
- **TTS:** edge-tts (fast, free)

### Pipeline Proven Working (Smoke Tests, Apr 16-17)

3 out of 4 smoke tests passed ALL stages:

```
Plan inserted → Planner creates plan → Supervisor approves → Tasks created → Task claimed by glm-5 → Hermes executes → Task moved to review
```

**Timing:** Full pipeline ~3 minutes (planner 1m30s, supervisor 1m17s, task execution 45s)

**Failures were hermes/GLM-5** (empty response bug), NOT governor logic. The 1 failure showed `🔎 preparing search_files…` as hermes output — partial streamed content that v0.9.0 fixes.

### Committed Changes (All on main, all safe)

| Commit | What | Impact |
|---|---|---|
| 4c76b4f8 - 15bfc88a | Migrations 113-114 (RPCs + tables) | Additive SQL only |
| 6ab5dd33 | Fixed system.json paths, missing tables | Config fix |
| 613b095e | Fixed reserved word in migration 114 | SQL fix |
| 746471ef | Migration 115 (4 failed RPCs) | SQL only |
| 2388c694 | Migration 116 (claim_task params) | SQL + startup_validate |
| b1f7bf91 | Migration 117 (remove model_id) | SQL only |
| 4aaed0c3 | Migration 118 (transition_task JSONB) | SQL only |
| e83e03c4 | `internal_cli` → `task_runner` in handlers_task.go | One-line Go fix |

### Supabase Schema (118 migrations applied)

- All RPCs live and verified with correct signatures
- `claim_task(UUID, TEXT, TEXT, TEXT, TEXT)` — fixed params, no model_id
- `transition_task(UUID, TEXT, JSONB)` — fixed JSONB type
- Secrets vault: 10 keys (AES-256-GCM encrypted)

### Connectors (4 API + 7 web couriers)

|| ID | Type | Status | Notes |
|---|---|---|---|
| hermes | cli | active (primary) | v0.8.0, updating to v0.9.0 |
| opencode | cli | active | |
| gemini-api | api | active | GEMINI_API_KEY |
| groq-api | api | active | GROQ_API_KEY |
| nvidia-api | api | active | NVIDIA_API_KEY |
| deepseek-api | api | benched | out of credits |
| openrouter-api | api | emergency_fallback | |
| 7 web couriers | web | active | browser-based |

### Models (16 configured)

Primary: GLM-5 via Z.AI (hermes CLI). Fallback chain: Gemini Flash → Groq → NVIDIA → OpenRouter free tiers.

**WARNING: ZAI/GLM subscription ends May 1.**

---

## What Got Built (April 2026)

### Governor (Go, ~15k lines, 16 packages)

- Event-driven runtime with session factory, agent pool, connection router
- MCP client: 52 tools from jcodemunch
- 3-layer memory (short/mid/long-term) in Supabase
- Context compaction, gitree (git abstraction), worktrees
- Model router: courier availability → connector matching → model assignment
- Pipeline: plan → planner → supervisor → council → tasks → dependency-locked claiming

### Supabase Schema (118 migrations)

- Core: tasks, plans, task_runs, orchestrator_events, platforms, models, prompts, slices
- Memory: memory_sessions, memory_project, memory_rules
- Performance: model_scores, performance_metrics, failure_records, checkpoints, state_transitions
- Security: security_audit_log, secrets_vault

### Knowledge Layer (.context/)

- lean-ctx (map.md): Go function signatures
- jCodeMunch (index.db): All code symbols
- knowledge.db: Prose, rules, prompts, docs, schema

---

## Known Issues

### To Fix (Hermes Update)
- **Hermes v0.8.0 GLM-5 bug** — empty response recovery broken for reasoning models. Fixed in v0.9.0 commit d6785dc4. Update with `hermes update`.

### Existing (Not Caused By Tonight)
- **Startup missed events** — plans inserted before governor subscribes sit in `draft`. Need boot scan for stuck plans. NOT YET IMPLEMENTED (recovery code was discarded).
- **Worktree branch leak** — task operations switch main repo branch. Pre-commit hook makes it worse.
- **ZAI subscription ends May 1** — test fallback chain before then.
- **Browser tool** — Playwright headless, no cookies. Real Chrome via CDP works.
- **Dashboard** — not reflecting real-time status correctly.

### NOT Issues (False Alarms From Tonight)
- CLIRunner is NOT broken — it's agent-agnostic, handles JSON streaming correctly
- decision.go extractJSON is NOT broken — balanced brace extraction works fine
- Governor pipeline logic is NOT broken — planner → supervisor → council → tasks flow is correct
- The problem was hermes v0.8.0 returning empty/partial output with GLM-5

---

## Hardware: ThinkPad X220

- Intel i5-2520M (Sandy Bridge, no AVX2, no GPU)
- 16GB RAM (~10GB available)
- ~780GB disk free
- Phone WiFi tethered

---

**Last Updated:** 2026-04-17 (discarded all panicked rewrites, honest assessment)
