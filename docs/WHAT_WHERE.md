# VibePilot: What Is Where

Quick reference for navigating the project. Update when structure changes.

> Last updated: April 30, 2026

---

## Architecture

VibePilot is a Go binary (governor) backed by local PostgreSQL. No Python runtime. No Supabase. The governor handles the entire pipeline: webhook → plan → dispatch → execute → review → test → merge.

**Two copies of the repo:**
- `~/VibePilot/` — Development copy. Edit here, git push from here.
- `~/vibepilot/` — Running copy. Binary lives here. Systemd service reads from here.

Both must stay synced on `main` branch.

---

## Key Directories

| Directory | What's There |
|-----------|--------------|
| `governor/cmd/governor/` | Go source: handlers, webhooks, main entry point |
| `governor/internal/` | Go packages: runtime, webhooks, gitree, config |
| `governor/config/` | JSON config files (models, routing, agents, connectors, system) |
| `governor/config/prompts/` | Agent prompt templates (planner.md, supervisor.md, analyst.md, council.md) |
| `governor/scripts/` | Utility scripts (daily_model_health.py) |
| `docs/` | Documentation, PRD folder, SQL schema |
| `docs/prd/` | PRD markdown files (push triggers pipeline) |
| `docs/supabase-schema/` | SQL migrations (numbered 001-132+, applied via psql) |
| `governor/bin/` | Compiled governor binary |

---

## Config Files (`governor/config/`)

| File | Purpose |
|------|---------|
| `models.json` | 58 models with providers, rate limits, buffer_pct |
| `routing.json` | Cascade strategies: `strategies.free_cascade.priority` |
| `agents.json` | Agent definitions: roles, models, context policies |
| `connectors.json` | API connector definitions (openrouter, google, groq, etc.) |
| `system.json` | System-wide settings: timeouts, trigger attempts, diagnostics |
| `tools.json` | Tool definitions (browser-use, etc.) |
| `health_report.json` | Output of daily model health check |

---

## Database Tables (PostgreSQL `vibepilot`)

| Table | What | Key Columns |
|-------|------|-------------|
| `tasks` | Work items from plans | id, title, status, plan_id, assigned_to, attempts, total_tokens_in/out, total_cost_usd |
| `task_runs` | Each model invocation | task_id, model_id, role (planner/supervisor/executor/analyst), tokens_in/out |
| `plans` | Generated from PRD | id, prd_path, status, tasks (JSONB) |
| `orchestrator_events` | Pipeline lifecycle | event_type, task_id (TEXT), details (JSONB) |
| `subscription_history` | Subscription tracking | provider, model_id, tokens_consumed, api_equivalent_cost_usd |
| `platforms` | Web AI destinations | name, status, daily_limit, daily_used |
| `models` | Model performance stats | model_id, success_rate, avg_response_time |
| `secrets_vault` | Encrypted API keys | AES-256-GCM, access via `./governor vault` |
| `maintenance_commands` | Infrastructure fixes | command_type, status, task_id |

**Migrations**: `docs/supabase-schema/NNN_*.sql`, applied with `psql -d vibepilot -f migration.sql`

---

## Dashboard (Vibeflow)

**Location:** `~/vibeflow/`

| File | What |
|------|------|
| `apps/dashboard/lib/vibepilotAdapter.ts` | Transforms governor API data to dashboard shape. Plan-to-task mapping. ROI calculation. Token metrics. |
| `apps/dashboard/lib/supabase.ts` | Supabase client (for dashboard hosting on Vercel) |
| `apps/dashboard/hooks/useMissionData.ts` | Data fetching hooks |

**Deploy**: Push to main → Vercel auto-deploys. Must include `dist/` in commit.

---

## API Keys

**Where they live:**
- `~/.hermes/.env` — Provider keys (GEMINI_API_KEY, GROQ_API_KEY, OPENROUTER_API_KEY, etc.)
- PostgreSQL `secrets_vault` table — Governor runtime keys (encrypted)
- Use `./governor vault list/get/set` to manage vault entries

**Key gotcha**: Shell key loading needs `export $(grep KEYNAME ~/.hermes/.env | tr -d '"')`. Plain `source` without `set -a` doesn't export.

---

## Governor Operations

```bash
# Status
systemctl --user status vibepilot-governor

# Restart
systemctl --user restart vibepilot-governor

# Rebuild (after Go code changes)
cd ~/vibepilot/governor && go build -o bin/governor ./cmd/governor/

# Logs
journalctl --user -u vibepilot-governor -f

# DB query
psql -d vibepilot -c "SELECT ..."

# Migration
psql -d vibepilot -f /path/to/migration.sql
```

---

## Daily Model Health Check

Script: `governor/scripts/daily_model_health.py`
Cron: 6 AM via Hermes
Output: `governor/config/health_report.json`

What it does: Fetches models from Gemini/Groq/OpenRouter APIs, health-checks cascade models (actual API calls), updates rate limits, reports new free models, applies ban list.
