# Hermes Agent Memories
# Auto-backed up from Hermes memory store. Do not edit manually.
# Last updated: 2026-04-18 (22:28 UTC)

## MEMORY

PARAM FIX: set/clear_processing use p_table/p_id. claim_task uses p_task_id/p_worker_id/p_model_id/p_routing_flag/p_routing_reason. transition_task uses p_task_id/p_new_status/p_failure_reason(TEXT)/p_result(JSONB). create_task_run uses p_courier (nullable). ALWAYS grep Go call site for exact param names before writing SQL. Migrations 118-120 BROKE working pipeline by changing RPC signatures without matching Go callers.
§
FIXED Apr 18: Cascade routing, module→testing merge, cost calc (calc_run_costs RPC). WEBHOOK LIVE: Governor IS webhook server on :8080. Quick tunnel needs `--config /dev/null` (named tunnel catch-all bleeds). Migration 123: create_plan='draft'. handlePlanCreated does git.Pull() before reading PRD. MIGRATIONS PUSHED TO GITHUB — user applies via Supabase SQL Editor. PG overloading traps: pg_proc loop or rename function entirely.
§
E2E+VAULT FIXED Apr 18: Full pipeline proven. Vault re-encrypted all 6 keys with current VAULT_KEY (single PBKDF2+AES-GCM). Root cause was old double-derivation mismatch. Testing handler: 3-method worktree discovery, two-phase testing. Task completion = testing passed, merge best-effort. Module→main merge NOT yet done.
§
GOVERNOR GOTCHAS: Cold start only reacts to realtime events (touch updated_at). claim_for_review reused by testing. task_packets table must exist. Migration 124 applied: check_platform_availability, get_model_score_for_task, update_model_usage RPCs. UsageTracker persists every 30s. RPC allowlist in db/rpc.go — must add new RPCs there. models table `platform` is NOT NULL — must always include in insert. platform/courier on models table are DISPLAY HINTS only, not routing.
§
ZAI API: GLM-5 subscription key may not work for direct API calls — endpoint or auth might differ from CLI. INVESTIGATE before assuming key is wrong. Governor cascade retry now loops through models on planner failure (ExcludeModel field). Realtime triggers plan_created on UPDATE→draft not just INSERT.
§
THREE-TIER: Model=who, Connector=how, Platform=where. GLM-5=subscription(z.ai, 3 concurrent, May 1). Router: filter ACTIVE first. Governor reads files from GitHub, passes content — never make up prompts.
## USER PROFILE

PET PEEVE: SIMPLE DIRECT ACTION. No spawning models. Push changes to GitHub FIRST. Migrations self-contained. DON'T IMAGINE VERIFY — grep/read before proposing fixes. Check git history before claiming wrong. Do homework first.
§
User vision: n8n-like visual config-driven orchestration (draw pipelines, not code them). Courier agents to free web AI tiers via Browser Use (self-hosted, open source). Chat URLs stored in task_runs for revision context. Visual QA agent checks apps before human review. MIT/Apache only. No Apple. Conservative subagent usage with GLM. May 1 = budget cliff.
§
BURNING CONSTRAINT: 20-46K tokens boot. Think before build. May 1 = GLM budget cliff.
ORCHESTRATOR NON-NEGOTIABLE: Health, cooldown, rate limits, credit, routing — never skip. Core system.
§
CRITICAL: NEVER declare dead/mock without checking docs+user. Dashboard IS LIVE. Supabase+GitHub=truth, not local. Governor subservient to VibePilot. Must work on autopilot or it's broken. PRDs need full tech specs.
§
CONFIDENCE DECOMPOSITION: Tasks split until 95%+ each. Low=auto-decompose not just council. Not implemented yet.
§
NO CANCELLED STATUS: Factory not todo app. Tasks complete/escalate. Failed→notes→available for re-route. Learns from failures.
§
CURRENT MODEL: GLM-5 via Z.AI Pro subscription. Better for coding and complex reasoning.