# VibePilot Current State
> Last updated: April 24, 2026 — 01:22 UTC

## System Status
- **Governor**: RUNNING (PID 169717, systemd service, `Restart=always` active)
- **Database**: Local PostgreSQL 16 on x220 (`localhost:5432`, db=vibepilot, user=vibes)
  - system.json correctly set to `"type": "postgres"`
  - 141 RPC functions
- **GitHub**: `VibesTribe/VibePilot` (public, branch `main`, last: 2fffb4ad)
- **Dashboard**: React frontend on Vercel (`vibeflow-dashboard.vercel.app`). Reads from governor `/api/dashboard` (local PG). SSE live updates wired.
- **Webhooks**: `webhooks.vibestribe.rocks` (Cloudflare Tunnel) → local governor port 8080
- **Realtime**: Supabase Realtime REMOVED. Replaced by `pg_notify` + SSE bridge.
- **Backup**: `~/vibepilot/scripts/pg-dump-and-push.sh` exists, cron at 3am → VibesTribe/knowledgebase (private repo)

## Architecture Change: Local PostgreSQL Migration (April 22, 2026)

Supabase replaced as primary database with local PostgreSQL. Motivations:
- Zero egress, zero polling risk (was pounding Supabase with usage writes)
- Simple `pg_dump` backup for full restore
- Low latency (localhost vs remote)

### Migration Path
1. Schema built by replaying all 122 Supabase migration files (PG16/17 version mismatch prevented direct pg_dump)
2. Data exported from Supabase via REST API → SQL INSERT with type-aware casting
3. `system.json` flipped from `"type": "supabase"` to `"type": "postgres"`
4. New native pgx implementation in `governor/internal/db/postgres.go`

### Post-Migration Fixes (commit e3767ba5)
| Issue | Root Cause | Fix |
|-------|-----------|-----|
| Rehydration queries fail | `buildSelectQuery` produced `ORDER BY` before `WHERE` (Go map random iteration) | Collect WHERE/ORDER BY/LIMIT separately, assemble in correct order |
| Vault can't decrypt secrets | Same SQL ordering bug + UUID `[16]byte` not converted to string | Fixed SQL ordering + added `[16]byte` → UUID string conversion in `convertValue()` |
| Timestamp parse warnings | Go `Time.String()` format vs RFC3339Nano | Added `parseTime()` helper with fallback formats |
| Routing validation error | `routing.json` referenced nonexistent `gemini-2.5-flash` | Changed to `gemini-2.5-flash-lite` (actual model ID) |

### Supabase is Fully Removed
- **Realtime subscriptions**: REMOVED. Replaced by `pg_notify('vp_changes')` triggers → pgnotify listener → SSE broker
- **REST API calls**: REMOVED. All reads/writes go to local PG via pgx
- **Config**: `system.json` set to `"type": "postgres"`, `DATABASE_URL` env var for connection

## SSE Bridge: pg_notify → Dashboard (April 23, 2026)

Replaces 5-second polling with event-driven live updates.

### Data Flow
```
PG trigger (vp_notify_change)
  fires pg_notify('vp_changes', {table, action, id, status, processing_by})
    |
    v
pgnotify/listener.go (status-aware)
  parses payload, checks status field
  |-> EventRouter.Route() — domain-specific (task_available, task_completed, etc.)
  |-> SSEBroker.Broadcast() — generic {table, action, id, status}
       |
       v
     /api/dashboard/stream (SSE endpoint in server.go)
       sends "event: change" to connected browser EventSource clients
       |
       v
     useMissionData.ts (dashboard)
       EventSource receives notification → targeted fetch from /api/dashboard (ETag cached)
       Falls back to 15s polling if SSE connection fails
```

### Files Changed (committed on branch `fix/pg-backend-sql-ordering`)
| File | Status | Detail |
|------|--------|--------|
| `governor/internal/webhooks/sse.go` | NEW | SSE broker — thread-safe fan-out, SSENotification{Table,Action,ID,Status} |
| `governor/internal/pgnotify/listener.go` | NEW | pg_notify listener — status-aware routing, dual-path (EventRouter + SSE) |
| `governor/internal/webhooks/server.go` | MODIFIED | SSE endpoint `/api/dashboard/stream`, deprecated WS stub, removed dead code |
| `governor/cmd/governor/main.go` | MODIFIED | pgnotify replaces Supabase realtime, shared SSE broker wired to both |
| `governor/config/system.json` | MODIFIED | `"type": "postgres"` |
| PG trigger `vp_notify_change()` | MODIFIED | Now includes `status` and `processing_by` in payload |
| `~/vibeflow/apps/dashboard/hooks/useMissionData.ts` | MODIFIED | EventSource replaces setInterval, 15s fallback poll |

## Courier Result Endpoint (April 23, 2026)

Courier agents (GitHub Actions) now POST results to governor API instead of Supabase REST.

### Data Flow
```
courier_run.py completes task
  POST /api/courier/result {task_id, status, result, error, tokens_in, tokens_out}
    |
    v
server.go handleCourierResult()
  validates payload
  calls CourierResultFunc callback (set from main.go)
    |
    v
  main.go callback:
    1. Writes to task_runs table via record_courier_result PG RPC
    2. Calls courierRunner.NotifyResult() to wake waiting goroutine
    |
    v
  pg_notify trigger fires on task_runs INSERT
    → SSE broadcast → dashboard live update
    → EventRouter → task transitions to review
```

### Files Changed
| File | Status | Detail |
|------|--------|--------|
| `governor/internal/webhooks/server.go` | MODIFIED | `/api/courier/result` POST endpoint + `CourierResultFunc` type + `SetCourierResultHandler()` |
| `governor/internal/connectors/courier.go` | MODIFIED | Added `governorURL` field, sends `governor_api_url` in dispatch payload |
| `governor/cmd/governor/main.go` | MODIFIED | Wires courier result handler, passes governor URL |
| `scripts/courier_run.py` | MODIFIED | `update_result()` POSTs to governor API (was `update_supabase()`) |
| PG function `record_courier_result()` | NEW | RPC that writes to task_runs with proper columns |

## Pipeline Timeline (Dashboard Logs Enhancement, April 23, 2026)

Logs panel now shows full task lifecycle by stitching data from 6 sources into a unified timeline.

### Sources
| Source | Table | Events Generated |
|--------|-------|-----------------|
| Task lifecycle | tasks | prd_committed, task_started, task_completed/failed |
| Plan lifecycle | plans | plan_created, plan_approved/rejected |
| Council reviews | council_reviews | council_approved/rejected (per reviewer) |
| Task runs | task_runs | run_completed/failed |
| Test results | test_results | test_passed/failed |
| Orchestrator | orchestrator_events | All original event types preserved |

### Features
- Filterable by source (task, plan, council, task_run, test, orchestrator)
- Each event shows: icon, label, timestamp, source badge, detail message, failure reasons
- 60 events visible (up from 40)
- Sorted chronologically (newest first)

### Files Changed
| File | Repo | Detail |
|------|------|--------|
| `apps/dashboard/hooks/useMissionData.ts` | vibeflow | Builds unified pipelineEvents array from 6 sources |
| `apps/dashboard/components/modals/MissionModals.tsx` | vibeflow | LogList with source filters, getEventMeta for 12 new event types, deriveLogCategory updates |
| `governor/internal/webhooks/server.go` | vibepilot | Dashboard endpoint now queries plans, council_reviews, test_results |

### Learning System (intact, not affected by migration)
- `learned_heuristics` — 20 entries (model performance heuristics)
- `model_scores` — 1 entry (model scoring from task outcomes)
- `lessons_learned` — 0 entries (ready for data)
- `planner_learned_rules`, `supervisor_learned_rules`, `tester_learned_rules` — tables present
- `revision_feedback` — feedback from rejected plans
- RPCs: `update_model_learning`, `record_model_success/failure`, `get_model_score_for_task`
- Handler: `recordModelLearning()` in `handlers_task.go`

## Governor Boot Sequence (current, April 24, 2026)
```
Connected to Postgres database
Loaded 57 models from database (restored usage state)
Loaded 23 prompts
All connectors registered (hermes, gemini x4, groq, openrouter, nvidia)
MCP connected (jcodemunch)
DAG pipeline loaded: code-pipeline
Courier Runner initialized
Running startup recovery...
  - No orphaned sessions
  - No tasks with checkpoints
PG Notify listener started on vp_changes
SSE broker ready
WebSocket listening at 8080
Webhook server on port 8080
Governor started (max/module: 2, max total: 4, opencode limit: 2)
```

## Model Fleet (58 models)

| Provider | Active | Benched | Paused | Connector |
|----------|--------|---------|--------|-----------|
| Groq | 7 | 0 | 0 | groq-api |
| OpenRouter | 19 | 0 | 0 | openrouter-api |
| Google Gemini | 4 | 0 | 1 | gemini-api-courier/researcher/visual/general |
| NVIDIA NIM | 3 | 0 | 0 | nvidia-api |
| Web (browser) | 16 | 0 | 0 | Various web connectors |
| Other | 0 | 6 | 1 | Various |
| **Total** | **49** | **6** | **2** | |

### Benched Models
| Model | Status | Reason |
|-------|--------|--------|
| chatgpt-4o-mini | benched | No free API access |
| claude-sonnet | benched | No free API access |
| gemini-web | benched | Web-only, superseded by API |
| kimi-k2-instruct | benched | Available via NVIDIA NIM instead |
| minimax-m2.7 | benched | Unreliable |
| nvidia/nemotron-3-super-120b | benched | Dead model ID |

### Paused Models
| Model | Status | Reason |
|-------|--------|--------|
| deepseek-chat | paused | Rate limits too aggressive |
| deepseek-reasoner | paused | Rate limits too aggressive |

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
AES-GCM encrypted, PBKDF2 SHA256 100k iterations. 15 keys stored:

| Vault Key | Purpose |
|-----------|---------|
| DEEPSEEK_API_KEY | DeepSeek API (paused models) |
| GEMINI_API_KEY | Legacy Gemini key |
| GEMINI_COURIER_KEY | Courier project API |
| GEMINI_RESEARCHER_KEY | Researcher project API |
| GEMINI_VISUAL_TESTER_KEY | Visual/Brain project API |
| GEMINI_GENERAL_KEY | General project API |
| GITHUB_TOKEN | GitHub API access |
| GROQ_API_KEY | Groq API access |
| NVIDIA_API_KEY | NVIDIA NIM API |
| OPENROUTER_API_KEY | OpenRouter ($0 credit) |
| SUPABASE_SERVICE_KEY | Supabase service role (legacy) |
| VIBEPILOT_GMAIL_EMAIL | Platform SSO |
| VIBEPILOT_GMAIL_PASSWORD | Platform SSO |
| webhook_secret | Webhook verification |
| ZAI_API_KEY | Z.AI (GLM-5 Hermes layer) |

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
        → pgnotify listener receives notification
        → EventRouter routes EventCourierResult
        → CourierRunner.NotifyResult() delivers to waiting goroutine via channel
        → Task transitions to "review"
        → SSE broadcast → dashboard live update
```

### Implementation Status

| Component | Status | Commit | Detail |
|-----------|--------|--------|--------|
| Model capabilities + courier markers | Done | bc0197a7 | 11 models marked courier: true |
| PlatformID/PlatformURL in RoutingResult | Done | e4e807ca | router.go carries destination info |
| Hardcoded RoutingFlag removed | Done | e4e807ca | Task runner passes "" (router decides) |
| CourierRunner on TaskHandler struct | Done | e4e807ca | Wired through main.go to TaskHandler |
| Web routing branch in executeTask | Done | e4e807ca | executeCourierTask() method added |
| GitHub Actions workflow | Done | b0b55235 | .github/workflows/courier_dispatch.yml |
| courier_run.py script | Done | b0b55235 | scripts/courier_run.py (browser-use) |
| pg_notify + SSE bridge (zero polling) | Done | 0c0fae03 | pgnotify/listener.go + webhooks/sse.go |
| EventCourierResult handler | Done | 0c0fae03 | EventRouter receives from pgnotify |
| Pipeline gap fixes (5 gaps) | Done | c2e94151 | Vault threading, RPC params, task_runs columns, result format |
| Vault key derivation (deriveLLMKeyRef) | Done | c2e94151 | Maps connectorID → vault key name |
| Gemini 4-project connectors | Done | 0897340f, 3a16958c | 4 independent keys, correct models |
| Local PG backend | Done | ffd29bfa, e3767ba5 | Native pgx, SQL ordering fix, UUID conversion |
| Dashboard SSE client | Done | c5a1923f | EventSource replaces polling in useMissionData.ts |
| Courier result endpoint | Done | a0b4336f | POST /api/courier/result, no more Supabase writes |
| Pipeline timeline (dashboard logs) | Done | ec907c43, c5a1923f | Unified 6-source timeline with source filters |

## Recent Commits (April 22-24, 2026)

| Commit | Description |
|--------|-------------|
| 2fffb4ad | feat: eliminate hardcoded token estimates, make all timeouts config-driven |
| 3222dac0 | fix: routing cascade puts Gemini first (4 free keys), add plan_id guard, fix race condition |
| ec907c43 | feat: add plans, council_reviews, test_results to dashboard API |
| a0b4336f | fix: courier writes to governor API instead of Supabase |
| c5a1923f | feat: unified pipeline timeline in logs panel (vibeflow) |
| 0c0fae03 | feat: SSE bridge replaces Supabase Realtime for dashboard live updates |
| e3767ba5 | fix: PG backend SQL ordering, UUID conversion, timestamp parsing, routing ref |

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
- **Local PostgreSQL 16**: vibepilot database, 63 tables, ~135 RPC functions
- **Local inference**: Too slow (2 tok/s tested). Cloud API only.

## Known Issues (non-blocking)

1. **systemd service active** -- governor runs via systemd user service with Restart=always. Survives crashes.
2. **SSE wired but not E2E tested with live dashboard** -- code compiles, wiring verified, but not tested with running governor + live browser
3. **jcodemunch CodeMap refresh** -- transport error on startup (pre-existing, graceful fallback to existing map.md)
4. **system.json database type was wrong** -- was `"supabase"` despite using local PG. Fixed to `"postgres"` April 24.
5. **Go code has hardcoded `"supabase"` case in main.go switch** -- should be purely config-driven. Works because config says `"postgres"` but the supabase path shouldn't be a special case.

## Known Gaps (pre-existing, not yet addressed)
- Maintenance agent not wired (git write access disconnected)
- Module branches never created (merge has nowhere to go)
- Worktrees disabled (all tasks share same directory)
- Orchestrator is NOT an LLM call -- just hardcoded cascade in Go
- Consultant agent not wired into pipeline (prompt and template exist, needs integration)
- Periodic jcodemunch refresh (only runs on startup currently)

## Key Architecture Docs
| Doc | Purpose |
|-----|---------|
| docs/CURRENT_STATE.md | This file |
| docs/CURRENT_ISSUES.md | Detailed issue tracker |
| docs/designs/governor-intelligence-fix.md | Intelligence overhaul design |
| governor/config/models.json | 58 models, 49 active |
| governor/config/connectors.json | 26 destinations, 22 active |
| governor/config/system.json | Backend config (`"type": "postgres"`) |
| config/prompts/*.md | Agent prompts (planner, supervisor, council, etc.) |

## Current Task State
- **Tasks**: 2 (1 available, 1 in_progress)
- **Plans**: 1 archived (plan 53db30f3, was in revision_needed loop)
- **Orchestrator Events**: 5 rows (3 approved, 2 failure)
- **Pending PRDs**: docs/prd/pending/ (not yet processed through pipeline)

## Database Stats
| Table | Rows |
|-------|------|
| models | 57 |
| platforms | 26 |
| tasks | 2 |
| task_runs | 0 |
| orchestrator_events | 5 |
| learned_heuristics | 20 |
| model_scores | 1 |
| lessons_learned | 0 |
| RPC functions | 141 |
| Total tables | 63 |

## Config-Driven Routing (April 24, 2026)

All hardcoded magic numbers eliminated. Token estimation, timeouts, and routing are config-driven.

### Token Estimation
- `EstimateTokens(content, role)` in router.go computes from actual content
- Code-aware: structural characters ({, }, etc.) detected for denser token ratio
- Response budget by role: planner 2x, task_runner 1x, supervisor 0.5x, courier 0.25x
- Result compared against each model's `context_limit` in `CanMakeRequestVia()`
- Models that can't fit the task are rejected with "exceeds_context_limit"

### Config-Driven Timeouts
All timeouts read from system.json (fallbacks only when config is nil):
- `system.execution.default_timeout_seconds` (fallback 300)
- `system.db.http_timeout_seconds` (fallback 30)
- `system.db.error_truncate_length` (fallback 200)
- `system.http.client_timeout_seconds` (fallback 30)
- `system.http.response_timeout_seconds` (fallback 30)
- `system.courier.timeout_seconds` (fallback 30)
