# VibePilot Current State
**Updated: April 18, 2026**

## System Status: PARTIALLY OPERATIONAL

### What Works
- GitHub webhook delivery (webhooks.vibestribe.rocks permanent URL)
- Governor realtime subscriptions and event processing
- Full pipeline chain: Plan → Tasks → Execute → Review → Test → Merge
- Vault encryption/decryption (all 6 API keys re-encrypted and working)
- Task completion tracking (testing passed = task complete, merge best-effort)
- Named tunnel "vibes" for dashboard/chat (sacred, never modify)
- Quick tunnel via `--config /dev/null`

### Pipeline End-to-End (Verified Apr 18)
1. Webhook push → plan created → tasks decomposed
2. Executor (glm-5/hermes) writes code in worktree
3. Supervisor reviews and approves
4. Testing handler discovers worktree, runs go build + targeted tests
5. Task marked complete with model tracking data
6. Best-effort merge to module branch
7. Dashboard tracks live via realtime subscriptions

### API Connectors
| Connector | Status | Notes |
|-----------|--------|-------|
| hermes (GLM-5) | ACTIVE | CLI-based, local. Subscription ends May 1 |
| nvidia-api (nemotron-ultra) | ACTIVE | Free tier, working |
| groq-api (llama-3.3-70b, etc) | ACTIVE | Free tier, re-enabled after vault fix |
| gemini-api (gemini-2.5-flash) | ACTIVE | Free tier, re-enabled after vault fix |
| deepseek-api | INACTIVE | Out of credit |

### Models in models.json but NOT in Supabase (10 models)
The following models exist in config/models.json but were never synced to the Supabase
models table. Migration 124 + model_loader upsert fix will create them on next restart:
- llama-3.3-70b-versatile, llama-3.1-8b-instant, qwen3-32b (groq)
- nemotron-ultra-253b (nvidia)
- gemini-2.5-flash-preview (gemini)
- kimi-k2-instruct (kimi/web)
- gpt-4o, gpt-4o-mini (openai/web)
- claude-sonnet-4-5 (anthropic/web)
- claude-haiku-4-5 (anthropic/web)

## Migrations Pending Application
**Migration 124**: `124_model_orchestrator_rpc.sql`
- `check_platform_availability(p_platform_id)` - checks model status, cooldown, credit
- `get_model_score_for_task(p_model_id, p_task_type, p_task_category)` - scores model for routing
- `update_model_usage(p_model_id, ...)` - persists usage/cooldown/learned data

Apply via Supabase Dashboard → SQL Editor.

## What Was Fixed Today (Apr 18)
1. **Vault encryption** - All 6 API keys re-encrypted with current VAULT_KEY
2. **Testing handler** - Filesystem worktree discovery (3 methods), two-phase testing (build+test)
3. **Task completion semantics** - Testing passed = complete. Merge is best-effort. Model data preserved.
4. **Model loader** - Upsert (insert-then-update) so new models auto-created in Supabase
5. **Usage persistence** - UsageTracker.PersistToDatabase() every 30 seconds to Supabase
6. **Querier interface** - Expanded to include RPC for persistence

## Known Issues / Not Yet Implemented
1. **Credit tracking not wired** - credit_remaining_usd exists in DB but no code deducts from it
   after task runs. Need to record cost per task_run and deduct from model's credit.
2. **Cascade retry bug** - When model fails (vault decrypt), cascade retries same model instead
   of falling through. Needs investigation in router cascade logic.
3. **Testing-to-main merge (step 3)** - Module→main merge not implemented. User wants testing
   folder approach for easy cleanup.
4. **Planner output parsing** - GLM-5 sometimes produces malformed JSON with backticks.
5. **Module→main merge** - Not yet implemented.
6. **E2E with real project** - Only tested with hello package. Needs real project PRD.

## Architecture Hierarchy (Non-Negotiable)
1. VibePilot architecture/flow
2. Dashboard (live, realtime, DO NOT TOUCH)
3. Supabase (state and contracts)
4. Go code (servant to all above)

## Critical Files
- `governor/config/models.json` - Model definitions with rate limits, pricing, capabilities
- `governor/config/connectors.json` - API connector configs (destinations array)
- `governor/internal/runtime/router.go` - Cascade routing, platform availability checks
- `governor/internal/runtime/usage_tracker.go` - Rate limiting, cooldown, learned data
- `governor/internal/runtime/model_loader.go` - Loads models.json, syncs to Supabase
- `governor/cmd/governor/handlers_testing.go` - Two-phase testing with worktree discovery
- `governor/cmd/governor/handlers_task.go` - Task execution, rate limit detection
- `migrations/124_model_orchestrator_rpc.sql` - Platform health + usage persistence RPCs

## Machine
- ThinkPad X220, Debian Linux
- USB tether to phone for Supabase connectivity
- Go 1.24.3 at /home/vibes/go/bin/go
- systemd user service: vibepilot-governor
- No direct PostgreSQL access (IPv6). Migrations via Supabase SQL Editor only.
- Git hooks require: `git -c core.hooksPath=/dev/null`

## Budget Timeline
- GLM-5 subscription: ends **May 1, 2026**
- Free tiers: Groq, NVIDIA, Gemini (rate limited but functional)
- Must prove pipeline with real projects before subscription ends
