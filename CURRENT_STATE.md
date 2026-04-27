# VibePilot Current State
# AUTO-UPDATED: 2026-04-27 — VERIFIED AGAINST CODE AND RUNNING SYSTEM
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
- **Git:** main branch. Last: 54e6eec0
- **Dashboard:** Live at vibeflow-dashboard.vercel.app (auto-deploys from GitHub main)
- **Chrome CDP:** 127.0.0.1:9222
- **Pipeline tables:** EMPTY (truncated, ready for E2E test)
- **System counters:** ~682,772 tokens / ~139 runs lifetime

## Human Role (3 things only)

1. **Visual UI/UX review** — after visual tester agent has reviewed
2. **Paid API benched** — out of credit, human decides add credits or keep benched
3. **Research after council** — council-reviewed suggestions, human gives final yes/no

## E2E Pipeline Path (verified 2026-04-27, 26 pipeline events)

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

### Pipeline Event Types (26 total)
**Planning:** prd_committed, planner_called, plan_created, supervisor_called (plan), plan_approved, plan_rejected
**Council:** council_approved, council_rejected
**Execution:** task_dispatched, output_received, supervisor_called (task), run_completed, run_failed, revision_needed, reroute
**Testing:** test_passed, test_failed
**Merge:** task_merged_to_module, merge_conflict_detected, module_merged_to_testing, module_merge_failed, integration_merge_failed, plan_complete

### Branch Naming & Cleanup
- Task branches: auto-created per task, auto-deleted after merge to module
- Module branches: `TEST_MODULES/{sliceID}`, auto-deleted after merge to testing
- Testing branch: auto-deleted after subtree merge to main/testing/

## MODELS: 66 in config (56 active, 4 benched, 3 offline, 2 paused), routed via free_cascade

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

## RECENT COMMITS (Apr 25-27)

1. 54e6eec0 — feat: wire BuildCouncilContext for plan reviews (council policy in session.go + agents.json)
2. a1669079 — docs: update CURRENT_ISSUES with pipeline gap audit results
3. 133cd28a — feat: maintenance pg_notify triggers + plan revision handler + target files in planner
4. 34678659 — feat: CooldownWatcher + doc accuracy fixes
5. 0f65f686 — feat: wire learning RPCs + module integration test gate
6. fcf7b198 — docs: verify all open issues against actual code (8/8 verified)
7. 16d9724a — feat: module branch cleanup + maintenance agent for all merge failures
8. a08afe74 — feat: pipeline event emissions (23 event types, standalone recordPipelineEvent)
9. (multiple) — Rich pipeline lifecycle events, merge events, subtree merge to testing/

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
- **GLM-5 (Hermes layer)**: Z.AI Pro subscription, ends May 1, 2026. NOT renewing at $90/3mo.
- **Total API cost**: $0/month (all free tiers)

## Hardware
- **Machine**: Lenovo x220, 16GB RAM
- **OS**: Linux (user-level systemd services)
- **Local PostgreSQL 16**: vibepilot database, 63+ tables, 144+ RPC functions
- **Local inference**: Too slow (2 tok/s tested). Cloud API only.

## VibesTribe Repos (April 2026)
| Repo | Commits | Purpose |
|------|---------|---------|
| VibePilot | 464 | Governor, pipeline, agents |
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

## Known Gaps (verified 2026-04-27)
- Consultant agent not wired into pipeline
- Task packet context PARTIALLY FIXED — ContextBuilder wired, reads target_files from planner result, but untested E2E
- Planner context PARTIALLY FIXED — BuildPlannerContext wired for full_map policy, injects slices/rules/failures + full code map from .context/map.md (auto-refreshed via git hook)
- Council context WIRED — BuildCouncilContext now called for council policy (commit 54e6eec0), provides file tree + plan reference verification. Deeper context deferred to knowledgebase.
- Research flow DEFERRED — Researcher agent not yet running. Reports will go to knowledgebase repo (VibesTribe/knowledgebase). Council reviews from there. Implementation happens in vibepilot task branches. Dashboard DOCS button will link to knowledgebase graph.
- No auto-discovery of new free models from providers (research agent handles daily landscape checks)
- See docs/CURRENT_ISSUES.md for full details

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
