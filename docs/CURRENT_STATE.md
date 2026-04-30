# VibePilot Current State
> Last updated: April 30, 2026 — 03:55 UTC

## System Status
- **Governor**: RUNNING (PID 590976, systemd service, `Restart=always` active)
- **Database**: Local PostgreSQL 16 on x220 (`localhost:5432`, db=vibepilot, user=vibes)
  - system.json correctly set to `"type": "postgres"`
  - 159 RPC functions
  - 69 tables
- **GitHub**: `VibesTribe/VibePilot` (public, branch `main`, last: e62d960a)
- **Dashboard**: React frontend on Vercel (`vibeflow-dashboard.vercel.app`). Reads from governor `/api/dashboard` (local PG). SSE live updates wired.
- **Webhooks**: `webhooks.vibestribe.rocks` (Cloudflare Tunnel) → local governor port 8080
- **Realtime**: Supabase Realtime REMOVED. Replaced by `pg_notify` + SSE bridge.
- **Backup**: `~/vibepilot/scripts/pg-dump-and-push.sh` exists, cron at 3am → VibesTribe/knowledgebase (private repo)

## Architecture: Local PostgreSQL (since April 22, 2026)

Supabase replaced as primary database with local PostgreSQL. Motivations:
- Zero egress, zero polling risk
- Simple `pg_dump` backup for full restore
- Low latency (localhost vs remote)

### Post-Migration Fixes (commit e3767ba5)
| Issue | Root Cause | Fix |
|-------|-----------|-----|
| Rehydration queries fail | `buildSelectQuery` produced `ORDER BY` before `WHERE` (Go map random iteration) | Collect WHERE/ORDER BY/LIMIT separately, assemble in correct order |
| Vault can't decrypt secrets | Same SQL ordering bug + UUID `[16]byte` not converted to string | Fixed SQL ordering + added `[16]byte` → UUID string conversion |
| Timestamp parse warnings | Go `Time.String()` format vs RFC3339Nano | Added `parseTime()` helper with fallback formats |
| Routing validation error | `routing.json` referenced nonexistent `gemini-2.5-flash` | Changed to `gemini-2.5-flash-lite` (actual model ID) |

### Supabase is Fully Removed
- **Realtime subscriptions**: REMOVED. Replaced by `pg_notify('vp_changes')` triggers → pgnotify listener → SSE broker
- **REST API calls**: REMOVED. All reads/writes go to local PG via pgx
- **Config**: `system.json` set to `"type": "postgres"`, `DATABASE_URL` env var for connection

## Courier Routing Fix (April 30, 2026)

Two bugs prevented web/courier routing from ever working. Router correctly selected web routing but downstream code couldn't act on it.

### Bug 1: claim_task status mismatch
- Migration 130 added `available` status for zero-dependency tasks
- `claim_task` RPC only matched `status = 'pending'` -- zero-dependency tasks sat at `available`, claims always failed
- After 5 model retries (all failed same way), handler fell through to internal execution
- **Fix**: Migration 133 -- `WHERE status IN ('pending', 'available')`

### Bug 2: RoutingFlag override
- Router returned `RoutingFlag: "web"` with PlatformID and ConnectorID
- Handler at line 234 ignored it and derived flag from connector type (`gemini-api-courier` is `type=api` → `"internal"`)
- Even if claim had succeeded, courier dispatch check would have failed
- **Fix**: Use `routingResult.RoutingFlag` when explicitly set, fall back to `deriveRoutingFlag()` only when empty

### Files Changed
| File | Commit | Detail |
|------|--------|--------|
| `docs/supabase-schema/133_fix_claim_task_for_available.sql` | 84441bdc | claim_task accepts 'available' status |
| `governor/cmd/governor/handlers_task.go` | e62d960a | Respect router's RoutingFlag, remove redundant transition_task |

## Courier Agent Pipeline

### Architecture: GitHub Actions + Governor API

```
Governor → router selects routing_flag="web"
        → CourierRunner.dispatch() sends repository_dispatch to GitHub
        → GitHub Actions spins up ubuntu-latest + browser-use + playwright
        → courier_run.py navigates to web platform, pastes prompt, extracts response
        → courier_run.py POSTs result to /api/courier/result (governor API)
        → Governor writes to task_runs via record_courier_result RPC
        → pg_notify trigger fires on task_runs INSERT
        → EventRouter routes EventCourierResult
        → Task transitions to "review"
        → SSE broadcast → dashboard live update
```

### Implementation Status

| Component | Status | Commit | Detail |
|-----------|--------|--------|--------|
| claim_task accepts 'available' status | Done | 84441bdc | Migration 133 |
| Handler respects router RoutingFlag | Done | e62d960a | No more override to 'internal' |
| Race condition fix (double dispatch) | Done | 66d4d373 | Removed redundant transition_task |
| Model capabilities + courier markers | Done | bc0197a7 | 11 models marked courier: true |
| PlatformID/PlatformURL in RoutingResult | Done | e4e807ca | router.go carries destination info |
| CourierRunner on TaskHandler struct | Done | e4e807ca | Wired through main.go |
| Web routing branch in executeTask | Done | e4e807ca | executeCourierTask() method |
| GitHub Actions workflow | Done | b0b55235 | .github/workflows/courier_dispatch.yml |
| courier_run.py script | Done | b0b55235 | scripts/courier_run.py (browser-use) |
| pg_notify + SSE bridge | Done | 0c0fae03 | pgnotify/listener.go + webhooks/sse.go |
| Courier result endpoint | Done | a0b4336f | POST /api/courier/result |

### Courier Execution: Open Questions

Two execution paths under consideration:

1. **Gemini free API (internal)** -- Direct API call, no browser. Imagen 3 available on free tier for image tasks. Fast, simple. Limited to what the API offers.

2. **Browser-use courier (external)** -- Spins up browser to access web-only platforms (ChatGPT free web for DALL-E, Ideogram, etc.). Two options:
   - **GitHub Actions runners** (original plan): Free tier, parallel execution, clean VMs per run. Resource isolation. Each runner installs what it needs.
   - **Browser Harness** (newly evaluated): Thin CDP harness (~592 lines), connects to real Chrome. Has free remote browser cloud (3 concurrent). Self-healing with domain skills. Very new but promising.

Decision pending: start with Gemini free API as internal executor for what it can do, add browser layer incrementally.

## Model Fleet (67 models)

| Provider | Active | Benched | Paused | Connector |
|----------|--------|---------|--------|-----------|
| Groq | 7 | 0 | 0 | groq-api |
| OpenRouter | 19 | 0 | 0 | openrouter-api |
| Google Gemini | 4 | 0 | 1 | gemini-api-courier/researcher/visual/general |
| NVIDIA NIM | 3 | 0 | 0 | nvidia-api |
| Web (browser) | 16 | 0 | 0 | Various web connectors |
| Other | 0 | 6 | 1 | Various |

### Gemini 4-Project Setup
4 independent Google Cloud projects, each with own API key and free-tier quota:
| Project | Key | Model | Role | Rate Limit |
|---------|-----|-------|------|------------|
| Courier | GEMINI_COURIER_KEY | gemini-2.5-flash-lite | Stable workhorse | 15 RPM / 1000 RPD |
| Researcher | GEMINI_RESEARCHER_KEY | gemini-3.1-flash-lite-preview | Best intelligence | 15 RPM / 1500 RPD |
| Visual/Brain | GEMINI_VISUAL_TESTER_KEY | gemini-3-flash-preview | Code fixing, visual QA | 15 RPM / 1500 RPD |
| General | GEMINI_GENERAL_KEY | gemini-2.5-flash-lite | Legacy fallback | 15 RPM / 1500 RPD |

**Combined free capacity**: 60 RPM / ~5500 RPD, $0 cost.

## Connectors/Destinations (26 total, 22 active)

### CLI Destinations (1 active)
| ID | Status | Notes |
|----|--------|-------|
| hermes | active | Primary CLI agent |
| opencode | inactive | Memory heavy (~700MB) |
| claude-code | inactive | No free tier |
| kimi | inactive | Available via NVIDIA NIM |

### API Connectors (7 active)
| ID | Status | Notes |
|----|--------|-------|
| groq-api | active | 7 models |
| openrouter-api | active | 19 free models, $0 credit |
| nvidia-api | active | 3 models via NIM |
| gemini-api-courier | active | Courier project |
| gemini-api-researcher | active | Researcher project |
| gemini-api-visual | active | Visual/Brain project |
| gemini-api-general | active | General/fallback project |

### Web Connectors (14 active)
Browser-use connectors for courier agents. All verified working April 20, 2026.

| Connector | Platform | Best For | Notes |
|-----------|----------|----------|-------|
| chatgpt-web | chatgpt.com | General | Google SSO |
| claude-web | claude.ai | Coding, reasoning | Google SSO |
| gemini-web | gemini.google.com | General, vision | Google SSO |
| deepseek-web | chat.deepseek.com | Coding, R1 reasoning | Google SSO |
| qwen-web | chat.qwen.ai | Coding, multilingual | Google SSO |
| mistral-web | chat.mistral.ai | Vision, coding | Google SSO |
| notegpt-web | notegpt.io | Quick queries | No auth, 3 free/day |
| kimi-web | kimi.com | Agent tasks | Google SSO |
| huggingchat-web | huggingface.co | Open source | No auth, MCP |
| aistudio-web | aistudio.google.com | Apps, tools | Google SSO |
| poe-web | poe.com | Multi-model | Google SSO, 3K pts/day |
| chatbox-web | app.chatbox.ai | Quick GPT access | No auth |
| aizolo-web | chat.aizolo.com | Research, fallback | Free tier limited |
| perplexity-web | perplexity.ai | Search + citations | Google SSO, 5 Pro/day |

## Vault (Local PostgreSQL secrets_vault)
AES-GCM encrypted, PBKDF2 SHA256 100k iterations. 15 keys stored.

## Config-Driven Routing

All hardcoded magic numbers eliminated. Token estimation, timeouts, and routing are config-driven.

### Token Estimation
- `EstimateTokens(content, role)` in router.go computes from actual content
- Code-aware: structural characters ({, }, etc.) detected for denser token ratio
- Response budget by role: planner 2x, task_runner 1x, supervisor 0.5x, courier 0.25x
- Result compared against each model's `context_limit` in `CanMakeRequestVia()`

### Config-Driven Timeouts
All timeouts read from system.json (fallbacks only when config is nil):
- `system.execution.default_timeout_seconds` (fallback 300)
- `system.db.http_timeout_seconds` (fallback 30)
- `system.courier.timeout_seconds` (fallback 30) — courier agents (browser-use, slower)
- Per-connector `timeout_seconds` in connectors.json overrides the global default

## Database Stats (April 30, 2026)
| Table | Rows |
|-------|------|
| models | 67 |
| plans | 1 (e2e-hello-world, review) |
| tasks | 1 (T001, merged) |
| task_runs | 3 |
| orchestrator_events | 78 |
| learned_heuristics | 1 |
| model_scores | 0 |
| RPC functions | 159 |
| Total tables | 69 |

## Budget
- **OpenRouter**: $0 credit account. Max spend limit configured. No payment added.
- **Groq**: Free tier
- **Gemini**: 4x free tier (no billing on any project)
- **NVIDIA NIM**: Free tier
- **GLM-5 (Hermes layer)**: Z.AI Pro subscription, ends May 1, 2026. NOT renewing at $90/3mo.
- **Total API cost**: $0/month (all free tiers)

## Hardware
- **Machine**: Lenovo x220, 16GB RAM, ~12GB free
- **OS**: Linux (user-level systemd services)
- **Local PostgreSQL 16**: vibepilot database, 69 tables, 159 RPC functions
- **Local inference**: Too slow (2 tok/s tested). Cloud API only.

## Known Issues (non-blocking)
1. **jcodemunch transport error** -- CodeMap refresh fails on startup (pre-existing, graceful fallback to existing map.md).
2. **SSE wired but not E2E tested with live dashboard** -- code compiles, wiring verified, not tested with running governor + live browser.
3. **4 models reference inactive connectors** -- deepseek-chat, deepseek-reasoner, meta/llama-3.3-70b-instruct, moonshotai/kimi-k2-instruct reference deepseek-api or nvidia-api which are not active. Warnings only, not errors.

## Key Architecture Docs
| Doc | Purpose |
|-----|---------|
| docs/CURRENT_STATE.md | This file |
| docs/CURRENT_ISSUES.md | Detailed issue tracker |
| governor/config/models.json | 67 models |
| governor/config/connectors.json | 26 destinations, 22 active |
| governor/config/system.json | Backend config (`"type": "postgres"`) |
| config/prompts/*.md | Agent prompts (planner, supervisor, council, etc.) |
