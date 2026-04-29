# VibePilot Current State
# AUTO-UPDATED: 2026-04-29 — VERIFIED AGAINST CODE AND RUNNING SYSTEM (+ cost tracking overhaul)
# RULE: Update after ANY change. Resume from here, never from guesses.
# RULE: NEVER update from assumptions. ALWAYS verify against actual code/data.

## Three Sources of Truth

1. **GitHub (code):** https://github.com/VibesTribe/VibePilot — pushed=real
2. **Local PostgreSQL (data):** localhost:5432, db=vibepilot, user=vibes — in DB=real
3. **Dashboard (live):** https://vibeflow-dashboard.vercel.app/ — rendering=working
   - Dashboard is USER DOMAIN. Additive-only, never remove without explicit OK.

## Hierarchy (everything serves what's above it)

```
VibePilot Architecture & Principles (modular, agnostic, no hardcoding)
  → Dashboard (what user sees and controls) — DASHBOARD IS SACRED
    → PostgreSQL (data layer, local)
      → Governor (pipeline executor)
        → Hermes (maintenance, audit, contract enforcement)
```

## System Status

- **Governor:** RUNNING (systemd service, Restart=always)
  - Binary: /home/vibes/vibepilot/governor/governor
  - Config: /home/vibes/vibepilot/governor/config/ (GOVERNOR_CONFIG_DIR env var)
  - WARNING: /vibepilot/config/ is a stale git copy. Always use governor/config/.
  - Database: Local PostgreSQL 16 (system.json type=postgres)
  - Webhook: port 8080/webhooks
  - SSE: pg_notify on vp_changes → SSE broker → dashboard
  - Governor URL: https://webhooks.vibestribe.rocks (for courier callbacks)
  - GitHub webhook: configured with secret (vp_webhook_2026_secret, stored in vault)
  - Vault: all secrets encrypted with current x220 VAULT_KEY, decrypt verified
- **Git:** main branch. Last: 670698fd (VibePilot), a0ee1fc82 (Vibeflow)
- **Dashboard:** Live at vibeflow-dashboard.vercel.app (auto-deploys from GitHub main)
- **Chrome CDP:** 127.0.0.1:9222
- **Pipeline tables:** 1 task (merged), 1 plan (review), 65 plans (draft from PRDs), 12 orchestrator_events
- **System counters:** ~689,120 tokens / ~143 runs lifetime
- **Cost tracking:** Phase 1-4 complete. Every model touch now records task_run rows with cost data.

## Human Role (3 things only)

1. **Visual UI/UX review** — after visual tester agent has reviewed
2. **Paid API benched** — out of credit, human decides add credits or keep benched
3. **Research after council** — council-reviewed suggestions, human gives final yes/no

## E2E Pipeline Path (verified 2026-04-28, 29 pipeline event call sites)

```
1. Push PRD to docs/prd/*.md
2. GitHub webhook → webhooks.vibestribe.rocks/webhooks → governor HandlePush
3. create_plan RPC → INSERT plans (status=draft) → prd_committed event
4. pgnotify → EventPlanCreated → handlePlanCreated → planner_called event
5. Planner: SelectRouting(role=planner) → free_cascade → API model → plan_created event
6. Plan file written to docs/plans/ → committed to GitHub
7. Plan status → "review" → pgnotify → EventPlanReview → supervisor_called event
8. Supervisor review → plan_approved / plan_rejected event
9. Council (if complex) → council_approved / council_rejected event
10. Tasks created (status=pending, routing_flag from plan)
11. pgnotify → EventTaskAvailable → handleTaskAvailable → task_dispatched event
12. Task routed via SelectRouting(role=task_runner)
13. Execution: internal (API) or courier (browser-use)
14. Output received → output_received event
15. Supervisor reviews output → supervisor_called event
16. Decision: run_completed / run_failed / revision_needed / reroute
17. Testing handler: 3 layers (artifact validation, semgrep, native tests) → test_passed/failed
18. Task merges to module branch, task branch deleted → task_merged_to_module event
19. Merge conflict? → merge_conflict_detected → maintenance agent (NOT task reassignment)
20. All module tasks done → merge module to testing → module_merged_to_testing, module branch deleted
21. Module merge fails? → module_merge_failed → maintenance agent
22. All modules done → subtree merge testing → main/testing/ → plan_complete event
23. Integration merge fails? → integration_merge_failed → maintenance agent
24. Testing branch deleted after merge to main
```

### Key Principle: Merge failures = infrastructure, not task failures
- Task agent is DONE after supervisor approves output and tests pass
- Merge conflicts at any level (task, module, integration) go to maintenance internal agent
- No model penalty, no task reassignment for merge issues

### Pipeline Event Types (29 call sites across 5 handlers)
**Planning:** prd_committed, planner_called, plan_created, supervisor_called (plan), plan_approved, plan_rejected
**Council:** council_approved, council_feedback
**Execution:** task_dispatched, output_received, supervisor_called (task), run_completed, run_failed, revision_needed, reroute
**Testing:** test_passed, test_failed
**Merge:** task_merged_to_module, merge_conflict_detected, module_integration_test, module_merged_to_testing, module_merge_failed, module_integration_test, integration_merge_failed, plan_complete

### Branch Naming & Cleanup
- Task branches: auto-created per task, auto-deleted after merge to module
- Module branches: `TEST_MODULES/{sliceID}`, auto-deleted after merge to testing
- Testing branch: auto-deleted after subtree merge to main/testing/

## MODELS: 67 in config (56 active, 4 benched, 3 offline, 2 paused), routed via free_cascade

### Active API Connectors (internal execution)
- hermes (CLI) — glm-5
- gemini-api-courier — gemini-2.5-flash-lite
- gemini-api-researcher — gemini-3.1-flash-lite-preview
- gemini-api-visual — gemini-3-flash-preview
- gemini-api-general — gemini-2.5-flash
- groq-api — llama-3.3-70b, qwen3-32b, etc.
- nvidia-api — nemotron-ultra-253b, llama-3.3-70b, kimi-k2
- openrouter-api — many free models

### Agents (governor/config/agents.json v2.3)
- All agents have empty model field = cascade routing via free_cascade
- context_policy per agent: planner=full_map, task_runner=targeted, council=council, most=file_tree

## COURIER SYSTEM — READY FOR E2E

All 5 courier bugs fixed (Apr 25):
1. courier_run.py: status "completed" → "success" (CHECK constraint)
2. record_courier_result: single text overload with counter updates
3. Duplicate task_runs: update_courier_task_run replaces create_task_run
4. pgnotify EventCourierResult: queries DB instead of nil Record
5. pgnotify+realtime: status string consistency fix

## Dashboard Enhancements (Apr 27)

### Pipeline Timeline (Logs Modal)
- 26 semantic event types with human-readable labels in getEventMeta()
- Filterable by source (task, plan, council, task_run, test, orchestrator)
- Each event shows: icon, label, timestamp, source badge, detail message
- Events map to meaningful progress: "Plan approved by supervisor", "Task dispatched to Gemini", etc.

### ROI Calculator (Tokens/ROI Modal)
- ProjectTracker: localStorage, persists across sessions, clearable
- SessionTracker: localStorage, labelable, reset on demand
- Module/Model breakdowns from task_runs data
- User says "it needs work" — revisions pending

## Governor Architecture (Apr 27)

### Testing Handler — 3-Layer Gate
1. **Layer 1: Artifact Validation** — verify expected output files exist, format checks (JSON parses, etc.)
2. **Layer 2: Semgrep Static Analysis** — ERROR severity = fail, WARNING = log only
3. **Layer 3: Native Test Suite** — go test, npm test, pytest (if project has one)

### Merge Flow
1. Test passes → task status = "complete" → maintenance handler
2. Maintenance handler: shadow merge → merge task to module → delete task branch
3. All module tasks done → merge module to testing → delete module branch
4. All modules done → subtree merge testing to main/testing/ → delete testing branch
5. Any merge conflict → maintenance agent (NOT model reassignment)

### File Locations
- Supervisor prompt: `prompts/supervisor.md` (208 lines, lines 77-95 updated for output_files)
- Planner prompt: `prompts/planner.md` (243 lines, standalone output example)
- Pipeline events: `governor/cmd/governor/pipeline_events.go` (standalone recordPipelineEvent)
- Testing handler: `governor/cmd/governor/handlers_testing.go`
- Maintenance handler: `governor/cmd/governor/handlers_maint.go`
- Task handler: `governor/cmd/governor/handlers_task.go`
- Plan handler: `governor/cmd/governor/handlers_plan.go`
- Council handler: `governor/cmd/governor/handlers_council.go`
- Git operations: `governor/internal/gitree/gitree.go` (MergeBranch, MergeBranchToSubdir, DeleteBranch, CommitAndPush)
- Dashboard: `~/vibeflow/apps/dashboard/` (MissionModals.tsx, useMissionData.ts)

## RECENT COMMITS (Apr 25-29)

1. 670698fd — chore: update migration 132 with record_internal_run RPC (Apr 29)
2. e22f1e99 — feat: per-task full cost tracking - Phase 2 (Apr 29)
3. 6d37581f — feat: cost tracking data foundation - Phase 1 (Apr 29)
4. c323d640 — feat: analyst agent for diagnostic ceiling (Apr 28)
5. b347f866 — fix: items 2-7 from gap analysis cleanup (Apr 28)
6. 4a94a00f — fix: supervisor sees everything on branch, multi-strategy parser, one prompt template (Apr 28)
7. b4cf7f1e — fix: 5 fragilities + E2E test + diagnostic ceiling + web-first routing + transition_task SOT (Apr 28)

### Vibeflow Commits
1. a0ee1fc82 — feat: header alerts banner for subscription/credit threshold warnings - Phase 4 (Apr 29)
2. 7613e19a7 — feat: cost tracking dashboard overhaul - Phase 3 (Apr 29)

## Knowledgebase (VibesTribe/knowledgebase — 11 commits, ready to build)

- **Repo**: https://github.com/VibesTribe/knowledgebase (exists, 11 commits)
- **Storage**: Local PostgreSQL (NOT SQLite — already running, proven)
- **Human-readable backup**: Markdown + Frontmatter files in repo
- **Index**: map.json for low-token agent discovery
- **Dashboard**: DOCS button links to vis.js graph (nodes colored by status)
- **Researcher**: GitHub Actions cron (2x daily), reads from sources.txt RSS list
- **Flow**: Researcher deposits reports to knowledgebase repo → supervisor auto-approves simple additions → council reviews complex ones → feedback appended to report → comes to human via knowledgebase link → implementation happens in vibepilot task branches
- **Institutional memory**: Every model/tool/API ever researched, when adopted/rejected, why, relationships, reconsideration when updates fix past rejection reasons
- **Status**: Not yet operational. Will be built and dogfooded by VibePilot after E2E verified.

## Budget
- **OpenRouter**: $0 credit account. No payment added.
- **Groq**: Free tier
- **Gemini**: 4x free tier (no billing on any project)
- **NVIDIA NIM**: Free tier
- **GLM-5 (Hermes layer)**: Z.AI Pro subscription, EXPIRES APR 30, 2026. $45/3mo grandfathered rate DEAD. New price $200/3mo.
  - 786.6M tokens consumed over 3 months = $1,439.56 at API rates = 3,099% ROI
  - Decision needed: Z.AI ($200/3mo) vs DeepSeek V4 Flash (~$51/mo) vs DeepSeek V4 Pro (~$160/mo discounted until May 31)
- **Total project spend**: ~$135 across all platforms over ~6 months
- **Total API cost going forward**: $0/month (all free tiers) unless GLM-5 replacement chosen

## Cost Tracking System (deployed 2026-04-29)

### Data Foundation (Phase 1, commit 6d37581f)
- `subscription_history` table: tracks all subscriptions, persists when archived
- `project_snapshots` table: archive/clear functionality for project totals
- `task_runs` new columns: `role`, `token_source`, `total_actual_cost_usd`
- `tasks` new columns: `total_tokens_in`, `total_tokens_out`, `total_cost_usd`, `total_api_equivalent_usd`, `model_count`
- GLM-5 history seeded: $45, 786.6M tokens, 3,099% ROI

### Per-Task Tracking (Phase 2, commit e22f1e99)
- `record_internal_run` RPC: creates task_run rows for planner, supervisor, analyst model calls
- Token estimation for web platforms: ~4 chars/token when API doesn't report counts
- `aggregate_task_costs` RPC called on task completion to sum all runs into task totals
- Webhook sites: planner (handlers_plan.go), supervisor (handlers_task.go), analyst (handlers_task.go)
- Courier result handler estimates tokens from output length when counts are 0

### Dashboard Overhaul (Phase 3, commit 7613e19a7)
- ROI panel reordered: Project-to-Date → Slices → Models → Subscriptions → Session
- Subscription History section with archived subscription display
- Header token pill toggles between "Live" and "Project" mode on click
- EventTone type expanded with "warning" for alert events

### Alerting (Phase 4, commit a0ee1fc82)
- Header alerts banner polls `/api/project/alerts` every 60s
- Shows subscription expiry and credit threshold warnings
- GLM-5 already flagged: "2 days remaining"

### USD/CAD Converter
- Present in RoiPanel (MissionModals.tsx): USD | CAD toggle buttons
- Defaults to 1.36, fetches live rate from governor DB (exchange_rates table) then exchangerate-api.com fallback
- Every dollar amount in ROI panel converts when toggled
- Exchange rate in DB: 1.36 USD/CAD, seeded 2026-02-16, source="seed"

## Hardware
- **Machine**: Lenovo x220, 16GB RAM
- **OS**: Linux (user-level systemd services)
- **Local PostgreSQL 16**: vibepilot database, 66 tables, 150 RPC functions
- **Local inference**: Too slow (2 tok/s tested). Cloud API only.

## VibesTribe Repos (April 2026)
| Repo | Commits | Purpose |
|------|---------|---------|
| VibePilot | 1792 | Governor, pipeline, agents |
| vibeflow | 55 | Dashboard UI |
| knowledgebase | 11 | Institutional memory (building next) |
| vibes-agent-context | 48 | Hermes memory backups |

## Router Architecture (config-driven, NOT hardcoded)

The orchestrator/router is fully config-driven via `routing.json` (free_cascade strategy with 21 models in priority order).

### Cascade Selection
- `getModelCascade()` reads `routing.json` strategies.free_cascade.priority
- `selectByCascade()` iterates with round-robin rotation for load distribution
- Each candidate scored by `GetModelLearnedScore()` (0-1): best_for_task_type +0.2, avoid -0.5, failure history penalty
- Tiebreaker: least-loaded by minute request count

### Rate Limiting & Health
- `UsageTracker.CanMakeRequestVia()` checks per-model: cooldown, RPM, RPH, RPD, TPM, TPD, context_limit, spacing
- `ConnectorUsageTracker`: shared limits across models on same connector (e.g., Groq org-level TPD)
- `PlatformUsageTracker`: web platform free-tier tracking (messages per 3h/8h/day/session)
- Buffer percentage (configurable, default 80%) prevents hitting hard limits

### Cooldown System
- On rate limit: `RecordRateLimit()` sets cooldown (configurable per model, default 30 min)
- On connector limit: `RecordConnectorCooldown()` puts ALL models on that connector in cooldown
- Cooldown auto-expires when timer runs out
- **Persistence**: `PersistToDatabase()` saves all cooldown state on shutdown, `LoadFromDatabase()` restores on startup
- **Re-verification**: `CooldownWatcher` probes models whose cooldown expired to verify they're actually healthy before the router sends traffic

### Startup Health Checks
- `registerConnectors()` probes each API connector with a minimal request (15s timeout)
- `CooldownWatcher` starts after cooldown state is restored, probes recently-expired models
- Failed probes extend cooldown; healthy probes log confirmation

### Learned Scoring
- `RecordCompletion()` tracks avg task duration, best/avoid task types, failure rates per model
- Learned data persists to DB, restored on restart
- Scoring: baseline 0.5, best_for_task +0.2, avoid_for_task -0.5, failure penalty up to -0.4

## Architecture Principles (enforced)
- Nothing is ever blocked. All feedback routes to the right agent for revision.
- No hardcoded values. Everything configurable via system.json. EXCEPTION: supervisor timeout 2min (Bug 3)
- Dead code removed promptly. No orphan event types or unreachable paths. EXCEPTION: 35 unused allowlist entries, supabase.go compiled but unreachable
- Human only involved for: visual UI/UX review, API budget, research after council.
- Self-correcting: council→planner→consultant chain flows backward when needed.
- Learning system is WIRED — 13 missing RPCs added to allowlist (Fix #1), now fully operational

## Gap Analysis Fixes (deployed 2026-04-28, 8 fixes applied)

### FIX #1 (CRITICAL) — RPC Allowlist: 13 Missing RPCs Added
- `rpc.go` allowlist expanded from 91 to 101 entries
- Added: add_bookmark, calc_run_costs, create_rule_from_rejection, get_change_approvals, get_failure_patterns, get_model_performance, get_slice_task_info, queue_maintenance_command, recall_memories, record_planner_rule_prevented_issue, store_memory, update_maintenance_command_status, update_model_learning

### FIX #2 (CRITICAL) — Dependency Chain: available/locked Status Support
- Migration 130: Added 'available' and 'locked' to tasks CHECK constraint
- `unlock_dependent_tasks` and `unlock_dependents` recreated: now look for `status IN ('locked', 'pending')`, set to 'available'
- `validation.go`: Zero-dependency tasks created as 'available' (not 'pending'), tasks with deps as 'pending'
- `pgnotify/listener.go`: Added "available" case mapping to EventTaskAvailable

### FIX #3 (HIGH) — Missing DB Functions Created
- Migration 131: `get_change_approvals(p_change_id TEXT)` — stub returning empty set (no table yet)
- Migration 131: `queue_maintenance_command(p_command TEXT, p_params JSONB)` — inserts into maintenance_commands

### FIX #4 (HIGH) — commitOutput Error Handling
- Both call sites in `handlers_task.go` now log errors instead of discarding with `_, _ :=`

### FIX #5 (MEDIUM) — Configurable Supervisor Timeout
- `config.go`: Added `ReviewTimeoutSeconds` to ExecutionConfig
- `handlers_task.go`: Reads from config with 2-minute fallback
- `system.json`: `"review_timeout_seconds": 120` added to execution section

### FIX #6 (MEDIUM) — Webhook "complete" → EventTaskApproval
- `webhooks/server.go` mapToEventType: Changed EventTaskCompleted → EventTaskApproval
- Now fires handleTaskApproved (merge flow), matching pgnotify behavior

### FIX #7 — Binary Rebuilt + Governor Restarted
- Clean startup, all health checks passing, dashboard API responding

### FIX #8 — Docs Updated + Pushed to GitHub

### Original 10 Bugs Status (Apr 25 → Apr 28)
1. task_packets never written — FIXED (commit 61b1a3da)
2. commitOutput on main repo not worktree — FIXED
3. Supervisor timeout hardcoded — FIXED (Fix #5, now configurable)
4. Testing can't find output in worktree — FIXED
5. Stale Supabase-era prompts in DB — STILL PRESENT (harmless, governor reads filesystem)
6. commitOutput errors silently ignored — FIXED (Fix #4)
7. STATUS_ORDER missing human_review — FIXED
8. transition_task no status validation — FIXED
9. Duplicate task creation race — FIXED
10. Task stuck at review after max attempts — UNCLEAR (no explicit terminal state)

## Remaining Known Gaps (verified 2026-04-28)

### OUTPUT PIPELINE — FIXED (commit 4a94a00f)
The perpetual parsing failure problem. Root causes found and fixed:
1. **Prompt template asked for wrong format** — `task_runner_simple.md` asked for `files_created: ["file1.py"]` (string paths, no content) while parser expected `{path, content}` objects. Template rewritten.
2. **Dead prompt templates** — `task_runner.md` and `task_runner_consecutive.md` deleted (zero references, only `task_runner_simple.md` was ever used).
3. **Parser was a binary JSON gate** — `ParseTaskRunnerOutput` returned nil if JSON extraction failed, no fallback. Rewritten as multi-strategy cascade: (1) clean JSON, (2) JSON in code fences, (3) code blocks as individual files, (4) raw text fallback.
4. **Supervisor was blind** — `handleTaskReview` only read files the PLANNER predicted (keyhole). Now uses `DiffWorktreeFiles` to show supervisor EVERYTHING that changed on the task branch. Supervisor is a model — it can judge deliverable vs commentary vs extra. Let it.
5. **Supervisor prompt updated** — now says "compare what was requested vs what was delivered, you judge." Handles binary files (video, images) as `[binary file, N bytes]`.

### Architecture insight
The task agent works on ONE task, ONE branch, ONE worktree. `git diff --name-only main..HEAD` = exactly the task output. No pre-filtering needed. The supervisor sees the diff and the original prompt, judges accordingly. This works for code, video, images, research — anything. The model is the quality gate, not regex.

### Still open
- Consultant agent not wired into pipeline (separate scope)
- Research flow DEFERRED — Researcher agent not yet running
- No auto-discovery of new free models (research agent handles daily checks)
- `get_change_approvals` is a stub — no `change_approvals` table exists yet
- Bug 10: No explicit terminal state when max retries exceeded in review loop
- Stale Supabase-era prompts in DB rows (harmless, governor reads filesystem)
- CooldownWatcher pollInterval hardcoded 2 min
- managed_repo.go email hardcoded governor@vibepilot.dev
- **GLM-5 Z.AI subscription expires Apr 30** — decision needed on replacement
- **Debug console.logs in MissionModals.tsx CAD toggle** — need cleanup next session
- **ROI calculator math** — user says it needs work, positioning changes pending
- **task_runs was empty before Phase 2** — new wiring needs E2E proof with real task data

## Pipeline Data Fixes (deployed 2026-04-28, commit 61b1a3da)
- **Task packets now stored in task_packets table** — prompt_packet survives result JSONB overwrites during execution
- **transition_task merges JSONB** — COALESCE(result, '{}') || COALESCE(p_result, '{}') prevents prompt_packet nuking
- **Plan handlers emit pipeline events** — 6 events added: planner_called, plan_created, supervisor_called, plan_approved, plan_rejected, council_review
- **orchestrator_events.task_id is TEXT** — supports both plan-level (planID) and task-level (UUID) events
- **vp_notify_change trigger guard** — checks for status column before referencing OLD.status
- **All plan events use recordPipelineEvent** — canonical function in pipeline_events.go (not the duplicate recordEvent)
- **Build fixes** — APIRunner.HealthCheck uses existing struct fields; registerConnectors uses context.Background()
- **E2E test verified** — Task d5823cd1 completed full loop: 3 attempts, 2 failures with feedback, 3rd passed, tested, merged. Output pipeline redesigned with supervisor full-view (4a94a00f).

## Human Review Workflow Status (Updated 2026-04-27)
✅ **Fix #1: Governor creates review JSON files on human_review** - IMPLEMENTED
  - Modified: `governor/cmd/governor/handlers_testing.go`
  - Added `createReviewFile()` function that generates JSON files in `data/state/reviews/{task_id}.json` when ui_ux tasks enter human_review status
  - Added `commitAndPushReviewFile()` using Gitree's CommitAndPush for atomic Git operations
  - Status: Code written, requires `go build ./cmd/governor/` verification and service restart

✅ **Fix #2: approve_review.yml calls governor /api/task/review** - ALREADY COMPLETE
  - Verified: `vibeflow/.github/workflows/approve_review.yml` contains curl step to notify governor on approval

✅ **Fix #3: request_changes.yml calls governor /api/task/review** - ALREADY COMPLETE  
  - Verified: `vibeflow/.github/workflows/request_changes.yml` contains equivalent curl step for rejection

✅ **Fix #4: Stale webhook mapper cleanup** — DONE (2026-04-28)
  - Location: `governor/internal/webhooks/server.go`
  - Fix: Changed EventTaskCompleted → EventTaskApproval in mapToEventType for "complete" status
  - Now fires the correct merge handler instead of dead-end event

### End-to-End Flow Verification
Once Fix #1 builds successfully:
1. ui_ux task completes testing → status = human_review
2. Governor creates `{task_id}.json` with `review: "pending"` in `data/state/reviews/`
3. Governor commits/pushes to GitHub via managed repo
4. Dashboard detects file → shows in Mission Control with "Review Now" button
5. User approves/rejects → GitHub Actions workflow updates JSON + pushes
6. Workflow calls `POST /api/task/review` with `{task_id, action: "approve"/"reject"}`
7. Governor transitions task via DB → complete (approve) or pending (reject)

Zero dashboard changes required - respects "sacred territory" constraint.

## Learning System (FIXED 2026-04-27, commit 0f65f686)
All learning RPCs now wired into handlers:
- **Supervisor rules**: `record_supervisor_rule` + `create_rule_from_rejection` called on supervisor fail/needs_revision
- **Tester rules**: `create_tester_rule` called on test failure (DB function created)
- **Heuristics**: `upsert_heuristic` called on task success (DB function created)
- **Problem-solutions**: `record_solution_on_success` called on task success
- **Module integration test**: `go build ./...` gate before module-to-testing merge
- **Code map refresh**: git post-checkout hook auto-regenerates .context/

## Vault (Local PostgreSQL secrets_vault)
AES-GCM encrypted, PBKDF2 SHA256 100k iterations. 15 keys stored.

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
