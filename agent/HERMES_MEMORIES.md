# Hermes Agent Memories
# Auto-backed up from Hermes memory store. Do not edit manually.
# Last updated: 2026-04-18 (06:22 UTC)

## MEMORY

PARAM FIX: set/clear_processing use p_table/p_id. claim_task uses p_task_id/p_worker_id/p_model_id/p_routing_flag/p_routing_reason. transition_task uses p_task_id/p_new_status/p_failure_reason(TEXT)/p_result(JSONB). create_task_run uses p_courier (nullable). ALWAYS grep Go call site for exact param names before writing SQL. Migrations 118-120 BROKE working pipeline by changing RPC signatures without matching Go callers.
§
HERMES v0.9.0 (Apr 17). GLM-5 ~50s/call. FIXED: v0.8.0 empty response bug + \r in JSON. TODO: per-agent max-turns overrides, planner prompt redundancy, startup plan recovery. All CONFIG fixes, never hardcode agent patterns into shared code.
§
CONTEXT WIRED: .hermes.md=enforcement (priority 1, loaded every msg). knowledge.db=3300 docs+364 SQL schema objects+17 pipeline stages. index.db (jCodeMunch)=1974 Go symbols + 2872 vibeflow symbols (indexed separately at ~/.code-index/local-vibeflow-c8d9c778.db). map.md=Go function signatures. Post-commit hook syncs to ~/vibepilot/. TERMINAL_CWD=~/VibePilot. AUDIT DOCS: DASHBOARD_AUDIT.md, CROSS_REFERENCE_AUDIT.md in docs/.
§
E2E PROVEN Apr 18: T001+T002 merged. M122+Go fixes: claim_for_review(review+testing), deps guard, attempts+unlock, revision feedback, timeouts(5m/2m), testing parse error, gitree branch delete+worktree. Recovery auto (300s stale). Module→main merge NOT yet done. Cold start = processing recovery.
§
GOVERNOR GOTCHAS: (1) Cold start only reacts to realtime events, not existing state — must touch updated_at. (2) claim_for_review reused by testing handler, must match both statuses. (3) task_packets static table, must exist or executor skips.
§
DASHBOARD: ~/vibeflow/apps/dashboard/ SACRED. Adapter: lib/vibepilotAdapter.ts. Lifecycle: pending→in_progress→review→testing→complete→merged. Pipeline proven E2E Apr 18. M122 applied: claim_for_review(review+testing), deps guard, attempts, auto-unlock. Gitree: DeleteBranch+worktree checkout main first.
BLOCKING: Tester calls GLM-5 via hermes to "run tests" — killed every time, recovery loops. Need direct `go test` not LLM. plans table: no title/description (only prd_path,plan_path,status,complexity).
## USER PROFILE

PET PEEVE: SIMPLE DIRECT ACTION. No spawning extra models. No asking me to edit code, save files, or multi-step copy-paste. I copy ONE thing from GitHub, paste ONE place. Migrations must be self-contained and rerunnable. DON'T IMAGINE, VERIFY — review existing code/state before proposing fixes, never invent solutions. Always grep/read first.
§
User vision: n8n-like visual config-driven orchestration (draw pipelines, not code them). Courier agents to free web AI tiers via Browser Use (self-hosted, open source). Chat URLs stored in task_runs for revision context. Visual QA agent checks apps before human review. MIT/Apache only. No Apple. Conservative subagent usage with GLM. May 1 = budget cliff.
§
BURNING CONSTRAINT: 20-46K tokens boot on repo files. Compressed knowledge layer needed. Think before build.
§
CRITICAL: NEVER declare dead/mock without checking docs+user. Dashboard IS LIVE. Supabase+GitHub=truth, not local. Governor subservient to VibePilot. Must work on autopilot or it's broken. PRDs need full tech specs.
§
CONFIDENCE DECOMPOSITION: Tasks split until 95%+ each. Low=auto-decompose not just council. Not implemented yet.
§
NO CANCELLED STATUS: Factory not todo app. Tasks complete/escalate. Failed→notes→available for re-route. Learns from failures.
§
CURRENT MODEL: GLM-5 via Z.AI Pro subscription. Better for coding and complex reasoning.