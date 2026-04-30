# VibePilot: What You Need To Know

> Last updated: April 30, 2026
> Read this FIRST when starting work on VibePilot.

---

## 1. What VibePilot Is

An automated software factory. Push a PRD (markdown), get merged, tested, production-grade code. Zero human code involvement. The human does UI/UX visual review, budget decisions, and council research yes/no only.

**Current state**: Full pipeline working. E2E test passed April 29. Race conditions and token tracking fixed April 30.

---

## 2. Architecture at a Glance

```
PRD (GitHub push) → Webhook → Planner → Plan Reviewer (supervisor)
  → Council (if complex) → Task Dispatch → Executor (courier/API)
  → Supervisor Review → Testing (3 layers) → Merge to main
```

**Two repo copies**:
- `~/VibePilot/` -- Development (git push from here)
- `~/vibepilot/` -- Running copy (binary lives here, systemd service)

Both must stay synced. Always work in ~/VibePilot/, copy to ~/vibepilot/, rebuild binary.

**Tech stack**: Go governor + PostgreSQL + Next.js dashboard (~/vibeflow/) + GitHub webhooks

---

## 3. Where Things Live

| What | Where | Notes |
|------|-------|-------|
| Governor source | `governor/cmd/governor/*.go` | Go binary, handles all pipeline stages |
| Config files | `governor/config/*.json` | models, routing, connectors, agents, system |
| Agent prompts | `governor/config/prompts/*.md` | planner.md, supervisor.md, analyst.md, council.md |
| SQL migrations | `docs/supabase-schema/NNN_*.sql` | 132 migrations, numbered sequentially |
| PRD folder | `docs/prd/*.md` | PRDs pushed here trigger the pipeline |
| Dashboard | `~/vibeflow/apps/dashboard/` | Next.js app, auto-deploys to Vercel on push |
| Dashboard adapter | `vibeflow/apps/dashboard/lib/vibepilotAdapter.ts` | Transforms governor API data for dashboard |
| Hermes config | `~/.hermes/config.yaml` | Primary model + fallback chain |
| API keys | `~/.hermes/.env` | All provider keys (GEMINI, GROQ, OPENROUTER, etc.) |
| Scripts | `governor/scripts/` | daily_model_health.py, etc. |
| Health report | `governor/config/health_report.json` | Output of daily model health check |

---

## 4. Key Database Tables

| Table | What | Key Columns |
|-------|------|-------------|
| `tasks` | Individual work items | id, title, status, plan_id, assigned_to, total_tokens_in/out, total_cost_usd |
| `task_runs` | Each model invocation per task | id, task_id, model_id, role, tokens_in, tokens_out, tokens_used, connector_id |
| `plans` | Generated from PRD | id, prd_path, status, tasks (JSONB) |
| `orchestrator_events` | Pipeline lifecycle events | event_type, task_id (TEXT, can be plan UUID), details (JSONB), created_at |
| `subscription_history` | Subscription tracking over time | provider, model_id, tokens_consumed, api_equivalent_cost_usd, roi_percentage |
| `platforms` | Courier destinations (web AI platforms) | id, name, status, daily_limit, daily_used |
| `models` | Model performance tracking | model_id, success_rate, avg_response_time |
| `secrets_vault` | Encrypted API keys | AES-256-GCM, access via `./governor vault` |

**Critical**: `orchestrator_events.task_id` is TEXT, not UUID. Plan-level events use planID. Task-level events use taskID.

---

## 5. Pipeline Event Flow (29 event types)

```
PRD push → prd_committed → planner_called → plan_created
  → supervisor_called → plan_approved (or plan_rejected → revision)
  → council_review (if complex)
  → task_dispatched → output_received → supervisor_called
  → run_completed (or run_failed → revision_needed → reroute)
  → test_passed (or test_failed)
  → module_merged_to_testing → module_integration_test
  → integration_merge_failed / plan_complete
```

All recorded via `recordPipelineEvent()` in `pipeline_events.go`.

---

## 6. Token Tracking & Cost

### 6a. How Tokens Are Recorded

Every model invocation in the pipeline records tokens to `task_runs` via `record_internal_run` RPC:

| Stage | role | task_id used | When |
|-------|------|-------------|------|
| Planner | `planner` | planID | After plan generation |
| Plan Reviewer | `plan_reviewer` | planID | After plan review |
| Executor | `executor` | taskID | Courier creates run |
| Task Supervisor | `supervisor` | taskID | After output review |
| Analyst | `analyst` | taskID | After diagnostic |
| Consultant | `consultant` | planID | Future: when consultant agent exists |

**Retry loops**: Each executor attempt creates a new task_run row. All runs accumulate per task_id. Tokens are summed across all attempts.

**Plan-to-task mapping**: Planner/plan_reviewer runs use planID as task_id. The dashboard adapter builds a `planToTask` map from `tasks.plan_id` to attribute these runs to the correct task.

### 6b. What Gets Counted

- `tokens_in`: Input tokens from API response
- `tokens_out`: Output tokens from API response
- `tokens_used`: Either explicit from API or `tokens_in + tokens_out`
- Courier agents: tokens counted from our prompt input/output (not from platform)

### 6c. ROI Calculation

- **Per-task**: Sum all task_runs for a task (all stages, all attempts)
- **Per-model**: Sum across all runs using that model
- **Per-project**: Subscription savings (API equivalent cost - actual cost)
- **Header "Now"**: Live pipeline token usage
- **Header ROI**: Pipeline savings + subscription savings

---

## 7. Model Management

### 7a. Config Files

- `governor/config/models.json` -- 58 models with rate limits and providers. No Nemotron.
- `governor/config/routing.json` -- Cascades and strategies. `strategies.free_cascade.priority` lists cascade order.
- `governor/config/connectors.json` -- Connector definitions for API providers.
- `~/.hermes/config.yaml` -- Hermes primary model + fallback chain.

### 7b. Rate Limits & Cooldowns

- All models have `buffer_pct: 80` in models.json
- UsageTracker enforces: at 80% of any rate limit (RPM/RPD/TPM/TPD), model is skipped
- Cooldown: 30 minutes default, learned optimal cooldowns override
- CooldownWatcher: probes models after cooldown expires, extends if still failing

### 7c. Daily Health Check

Script: `governor/scripts/daily_model_health.py`
Cron: 6 AM daily via Hermes cron
What it does:
1. Fetches models from Gemini/Groq/OpenRouter APIs
2. Health-checks all cascade models (actual API call)
3. Updates rate limits in models.json
4. Reports new free models (doesn't auto-add)
5. Applies ban list (Nemotron, tiny models, OCR-only)
6. Writes `governor/config/health_report.json`

### 7d. Ban List

Never route to: all Nemotron variants, Ling-2.6, models < 4B params, OCR-only models, Llama-3-instruct variants.

### 7e. Fallback Chain (Hermes config)

```
Gemini 2.5 Flash → Gemini 2.0 Flash → Groq Llama 3.3 70B
→ Groq Llama 3.1 8B → Groq Compound → OpenRouter Qwen Coder
→ OpenRouter MiniMax M2.5 → OpenRouter Gemma 4
```

NOTE: Hermes must be restarted for config changes to take effect.

---

## 8. Analyst Agent (Task Diagnostic Loop)

When a task hits `diagnostic_trigger_attempts` ceiling (default 2 in system.json), the analyst agent:
1. Reads ALL accumulated failure feedback
2. Reasons backwards through failures to find root cause
3. Routes task: re-prompt, split into subtasks, exclude failed model, or surface to human
4. Records diagnosis to task metadata

**Files**: `governor/config/prompts/analyst.md`, `handlers_task.go` `runAnalystDiagnosis()`, `ParseAnalystDecision()`, `routeAnalystDecision()`

---

## 9. Governor Operations

```bash
# Check status
systemctl --user status vibepilot-governor

# Restart (ALWAYS use systemctl --user, never sudo)
systemctl --user restart vibepilot-governor

# Rebuild binary after code changes
cd ~/vibepilot/governor && go build -o ~/vibepilot/governor/bin/governor ./cmd/governor/

# View logs
journalctl --user -u vibepilot-governor -f

# Database queries
psql -d vibepilot -c "SELECT ..."

# Migrations
psql -d vibepilot -f /path/to/migration.sql
```

---

## 10. Dashboard Operations

```bash
# Dashboard lives in ~/vibeflow/
cd ~/vibeflow

# Build (must include dist/ in commit for Vercel)
npm run build

# Push to deploy
git add . && git commit -m "..." && git push origin main
```

Vercel auto-deploys on push to main.

---

## 11. Known Working Providers (as of April 30)

| Provider | Key Status | Notes |
|----------|-----------|-------|
| ZAI (GLM-5) | Working | Primary model, subscription |
| Google Gemini | Working | Free tier, 15 RPM, key via ?key= param |
| Groq | Working | Free tier, 30 RPM, Llama/Mixtral/Compound |
| OpenRouter | Working | Free models available, 20 RPM default |
| NVIDIA NIM | NOT TESTED | Key exists, untested after cleanup |
| DeepSeek API | Inactive | Platform marked inactive |

---

## 12. Git Workflow

1. Always on `main` branch. Check with `git branch --show-current`
2. Edit files in `~/VibePilot/`
3. Copy changed Go files to `~/vibepilot/` (running copy)
4. Commit with descriptive message
5. Push to GitHub FIRST
6. Rebuild binary if Go code changed
7. Restart governor if binary changed

**NEVER** push from ~/vibepilot/. Always from ~/VibePilot/.
