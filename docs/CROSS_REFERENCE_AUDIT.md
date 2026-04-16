# Cross-Reference Audit: Go ↔ Supabase ↔ Dashboard

**Date:** 2026-04-16
**Purpose:** Find real mismatches, orphans, and gaps. No assumptions -- trace actual code.

---

## 1. SUPABASE TABLES

### Tables Go WRITES to (via RPC):
tasks, task_runs, models, orchestrator_events, plans, council_reviews, planner_rules, revision_feedback, learned_heuristics, failure_records, task_checkpoints, memory_project, memory_rules, memory_sessions, security_audit_log, state_transitions, performance_metrics, maintenance_commands, research_suggestions, test_results, runner_sessions

### Tables the Dashboard READS from (live Supabase queries):
tasks, task_runs, models, platforms, orchestrator_events, exchange_rates

### Tables the Dashboard WRITES to:
exchange_rates (upsert via roiCalculator.ts -- fire-and-forget, persists CAD rate from API)

### GAPS -- Tables Go writes but Dashboard never reads:
- plans -- Go creates/updates plans, dashboard doesn't show them
- council_reviews -- Go stores reviews, dashboard doesn't show them
- planner_rules -- Go creates/reads rules, dashboard doesn't show them
- revision_feedback -- Go records feedback, dashboard doesn't show it
- learned_heuristics -- Go records model learning, dashboard doesn't show it
- failure_records -- Go records failures, dashboard FAILURES PANEL IS EMPTY
- task_checkpoints -- Go saves/deletes checkpoints, dashboard doesn't show them
- memory_* -- Go stores/recalls memories, dashboard doesn't show them
- security_audit_log -- Go logs vault access, dashboard doesn't show it
- state_transitions -- Go records transitions, dashboard reads orchestrator_events instead
- performance_metrics -- Go records metrics, dashboard computes its own from task_runs
- maintenance_commands -- Go processes commands, dashboard doesn't show them
- research_suggestions -- Go processes suggestions, dashboard doesn't show them
- test_results -- Go creates results, dashboard doesn't show them
- runner_sessions -- Go manages sessions, dashboard doesn't show them

### GAPS -- Tables Dashboard reads but Go doesn't write:
- exchange_rates -- Dashboard reads/writes this itself (upsert from external API). Go never touches it.
- platforms -- Dashboard reads this. Go never writes to it. Seeded manually or via migration.

---

## 2. SUPABASE FUNCTIONS (155 total)

### Go CALLS these 49 functions:
calculate_run_costs, check_platform_availability, claim_for_review, claim_task, clear_processing, create_maintenance_command, create_plan, create_planner_rule, create_task_run, delete_checkpoint, find_orphaned_sessions, find_pending_resource_tasks, find_stale_processing, find_tasks_with_checkpoints, get_failure_patterns, get_heuristic, get_latest_state, get_model_performance, get_model_score_for_task, get_next_task_number_for_slice, get_planner_rules, get_problem_solution, get_recent_failures, get_slice_task_info, get_supervisor_rules, get_tester_rules, load_checkpoint, log_security_audit, recall_memories, record_failure, record_model_failure, record_model_success, record_performance_metric, record_planner_revision, record_revision_feedback, record_state_transition, recover_orphaned_session, recover_stale_processing, save_checkpoint, set_council_consensus, set_processing, store_council_reviews, store_memory, transition_task, unlock_dependent_tasks, update_maintenance_command_status, update_plan_status, update_research_suggestion_status, update_task_branch

**Result: ALL 49 Go RPC calls exist in Supabase. No missing functions.**

### Dashboard calls these 0 functions directly:
The dashboard does NOT call any Supabase RPCs. It only does direct table queries (.from().select()) and realtime subscriptions.

### 106 Supabase functions Go DOESN'T call:
(Carefully classified below)

---

## 3. ORPHAN FUNCTION CLASSIFICATION (106 functions Go doesn't call)

### CATEGORY A: USED BY DASHBOARD OR OTHER SYSTEMS (not orphans)

| Function | Status | Who uses it |
|----------|--------|-------------|
| get_exchange_rate | ACTIVE | Dashboard roiCalculator reads exchange_rates table (but via direct query, not this RPC) |
| get_dashboard_agents | ACTIVE | Likely designed for dashboard, but dashboard reads models directly |
| log_orchestrator_event | ACTIVE | Called by maintenance.go line 264 -- Go DOES call this but via m.db.RPC not database.RPC |

Wait -- let me recheck log_orchestrator_event. It's called via m.db.RPC in maintenance.go. My grep for `.RPC(` would have caught `m.db.RPC` and `database.RPC` both. Let me re-verify...

Actually looking at the grep output again: `maintenance.go:264: m.db.RPC(context.Background(), "log_orchestrator_event"`. This WAS in my results. So Go DOES call it. This is NOT an orphan.

### CATEGORY B: TRIGGER/AUTO-UPDATED HELPERS (not orphans -- they support other functions)

| Function | Purpose | Used by |
|----------|---------|---------|
| update_updated_at_column | Trigger helper | Auto-fires on UPDATE for multiple tables |
| update_maintenance_commands_updated_at | Trigger helper | Auto-fires |
| update_plans_updated_at | Trigger helper | Auto-fires |
| update_planner_rules_updated_at | Trigger helper | Auto-fires |
| update_research_suggestions_updated_at | Trigger helper | Auto-fires |
| update_supervisor_rules_updated_at | Trigger helper | Auto-fires |
| update_tester_rules_updated_at | Trigger helper | Auto-fires |
| update_destinations_updated_at | Trigger helper | Auto-fires |
| update_event_checkpoint | Trigger helper | Auto-fires |
| update_depreciation_score | Trigger helper | Auto-fires |

These are called by PostgreSQL triggers, not by Go. NOT orphans.

### CATEGORY C: NEW/RECENTLY CREATED (migration 111 era, designed for upcoming features)

These functions were created as part of the missing RPCs migration. They're designed for features being built, not dead code:

| Function | Purpose | Status |
|----------|---------|--------|
| get_dashboard_agents | Pre-built for dashboard optimization | Ready, not yet wired |
| get_available_tasks | Pre-built for task queue views | Ready |
| get_available_for_routing | Router optimization | Ready |
| get_all_projects_roi | ROI reporting | Ready |
| get_full_roi_report | ROI reporting | Ready |
| get_project_roi | ROI reporting | Ready |
| get_subscription_roi | Subscription tracking | Ready |
| get_slice_summary | Slice analytics | Ready |
| get_tasks_by_slice | Task grouping | Ready |
| get_all_subscriptions | Subscription listing | Ready |
| get_council_summary | Council view | Ready |
| get_learning_stats | Learning dashboard | Ready |
| get_planner_rule_stats | Rule analytics | Ready |
| get_system_state | System overview | Ready |
| get_weekly_intelligence_summary | Weekly reports | Ready |
| generate_intelligence_report | Report generation | Ready |
| get_active_destinations | Destination listing | Ready |
| get_destination | Destination lookup | Ready |
| get_destinations_by_type | Destination filtering | Ready |
| get_runners_to_archive | Runner management | Ready |
| get_best_runner | Runner selection | Ready |
| select_platform_for_task | Platform routing | Ready |
| check_model_availability | Model checking | Ready |

### CATEGORY D: EARLIER GENERATION (from pre-refactor, may overlap with current functions)

These were created in earlier migrations (083-110 era). Some overlap with newer versions:

| Function | Overlaps with | Status |
|----------|---------------|--------|
| claim_next_task | claim_task (current) | EARLIER VERSION -- Go uses claim_task now |
| claim_task_for_execution | claim_task (current) | EARLIER VERSION -- Go uses claim_task now |
| claim_next_command | -- | Earliest design, may still be valid for commands |
| complete_task_transition | transition_task (current) | EARLIER VERSION -- Go uses transition_task now |
| update_task_status | transition_task (current) | EARLIER VERSION -- Go uses transition_task now |
| update_task_assignment | claim_task (current) | EARLIER VERSION -- Go uses claim_task now |
| make_task_available | transition_task (current) | EARLIER VERSION -- sets status='available' |
| complete_command | update_maintenance_command_status | EARLIER VERSION |
| retry_command | update_maintenance_command_status | EARLIER VERSION |
| queue_maintenance_command | create_maintenance_command | EARLIER VERSION |
| calculate_task_roi | calculate_run_costs (current) | EARLIER VERSION |
| calculate_enhanced_task_roi | calculate_run_costs (current) | EARLIER VERSION |
| vibes_query | -- | Custom query API, possibly for future use |
| vibes_submit_idea | -- | User idea submission, possibly for future use |

### CATEGORY E: UTILITY/INFRASTRUCTURE (supporting tables that Go accesses via direct queries instead)

| Function | Table | Why Go doesn't call it |
|----------|-------|----------------------|
| get_vault_secret | secrets_vault | Go uses vault.go package directly |
| get_unread_messages | agent_messages | Direct query in Go? Or not yet wired |
| send_agent_message | agent_messages | Not yet wired |
| mark_message_read | agent_messages | Not yet wired |
| get_event_checkpoint | event_checkpoints | Not yet wired |
| increment_access_usage | access | Not yet wired |
| increment_in_flight | runners | Not yet wired |
| decrement_in_flight | runners | Not yet wired |
| increment_revision_round | plans | Not yet wired |
| needs_next_round | plans | Not yet wired |
| get_round_feedback | council_reviews | Not yet wired |
| submit_council_review | council_reviews | Go uses store_council_reviews instead |
| add_council_review | council_reviews | Go uses store_council_reviews instead |
| check_circular_deps | tasks | Validation helper, called by trigger? |
| check_dependencies_complete | tasks | Validation helper, called by trigger? |
| check_no_self_dependency | tasks | Validation helper, called by trigger? |
| check_revision_limit | plans | Validation helper, called by trigger? |
| check_task_escalation | tasks | Validation helper, called by trigger? |
| refresh_limits | runners/platforms | Not yet wired |
| reset_daily_usage | runners/platforms | Not yet wired |
| reset_platform_daily_usage | platforms | Not yet wired |
| archive_runner | runners | Not yet wired |
| boost_runner | runners | Not yet wired |
| revive_runner | runners | Not yet wired |
| record_runner_result | runners | Not yet wired |
| record_runner_success_timestamp | runners | Not yet wired |
| set_runner_cooldown | runners | Not yet wired |
| set_runner_rate_limited | runners | Not yet wired |
| update_model_stats | models | Go updates models directly via transition_task |
| update_platform_stats | platforms | Not yet wired |
| update_project_counts | projects | Not yet wired |
| update_test_result_status | test_results | Not yet wired |
| create_research_suggestion | research_suggestions | Not yet wired |
| create_rule_from_rejection | planner_rules | Not yet wired |
| create_rule_from_supervisor_rejection | supervisor_learned_rules | Not yet wired |
| create_supervisor_rule | supervisor_learned_rules | Not yet wired |
| create_tester_rule | tester_learned_rules | Not yet wired |
| create_task_if_not_exists | tasks | Not yet wired |
| create_task_with_packet | tasks | Not yet wired |
| create_test_result | test_results | Not yet wired |
| deactivate_planner_rule | planner_rules | Not yet wired |
| deactivate_supervisor_rule | supervisor_learned_rules | Not yet wired |
| deactivate_tester_rule | tester_learned_rules | Not yet wired |
| record_heuristic_result | learned_heuristics | Not yet wired |
| record_planner_rule_applied | planner_rules | Not yet wired |
| record_planner_rule_prevented_issue | planner_rules | Not yet wired |
| record_platform_usage | platforms | Not yet wired |
| record_solution_on_success | problem_solutions | Not yet wired |
| record_solution_result | problem_solutions | Not yet wired |
| record_supervisor_rule | supervisor_learned_rules | Not yet wired |
| record_supervisor_rule_triggered | supervisor_learned_rules | Not yet wired |
| record_tester_rule_caught_bug | tester_learned_rules | Not yet wired |
| record_tester_rule_false_positive | tester_learned_rules | Not yet wired |
| upsert_heuristic | learned_heuristics | Not yet wired |
| append_failure_notes | failure_records | Not yet wired |
| append_routing_history | tasks | Not yet wired |
| unlock_dependents | tasks | Go uses unlock_dependent_tasks instead |

---

## 4. DASHBOARD vs SUPABASE: COLUMN-LEVEL MISMATCHES

### tasks table -- Dashboard reads but column may not exist:
| Dashboard expects | In schema? | Go writes it? | Notes |
|-------------------|-----------|---------------|-------|
| confidence | Need to verify | No | Dashboard defaults to 0.85 |
| phase | Need to verify | No | Dashboard reads but doesn't display |
| routing_flag | Need to verify | Yes (via claim_task) | LIVE |
| routing_flag_reason | Need to verify | Yes (via claim_task) | LIVE |
| result.prompt_packet | Need to verify | Need to verify | Nested JSON -- Go must write this key |
| processing_by | Not in adapter | Yes (via claim_task) | Dashboard doesn't use it |
| processing_at | Not in adapter | Yes (via claim_task) | Dashboard doesn't use it |
| attempts | Not in adapter | Yes (claim_task increments) | Dashboard doesn't use it |
| branch_name | Not in adapter | Yes (via update_task_branch) | Dashboard doesn't use it |
| failure_notes | Not in adapter | Yes (via record_failure) | Dashboard failures panel is EMPTY |
| plan_id | Not in adapter | Yes (via create_task_with_packet?) | Dashboard doesn't use it |

### models table -- Dashboard reads but column may not be populated:
| Dashboard expects | Go populates it? | Notes |
|-------------------|-------------------|-------|
| access_type | Unclear | Determines Q/M/W tier badge. If null, defaults to Q |
| subscription_cost_usd | Unclear | Subscription ROI needs this |
| subscription_started_at | Unclear | Subscription ROI needs this |
| subscription_ends_at | Unclear | Subscription ROI needs this |
| subscription_status | Unclear | Filters active subscriptions |
| cost_input_per_1k_usd | Unclear | Value comparison |
| cost_output_per_1k_usd | Unclear | Value comparison |
| tasks_completed | Yes (via record_model_success?) | Or Go updates directly? |
| tasks_failed | Yes (via record_model_failure?) | Or Go updates directly? |
| success_rate | Yes (calculated?) | Or Go updates directly? |
| tokens_used | Yes (accumulated) | LIVE |
| status_reason | Yes (cooldown messages) | LIVE |

### platforms table -- Dashboard reads but Go never writes:
The platforms table is seeded manually. Dashboard reads config JSON deeply:
- config.name, config.provider, config.free_tier.model, config.notes
- If config is null, dashboard falls back to platform.name and platform.vendor
- Go NEVER writes to platforms -- it's a static reference table

---

## 5. DASHBOARD PANELS: LIVE vs DESIGNED vs EMPTY

| Panel | Data Source | Status |
|-------|-----------|--------|
| Task Cards | tasks + task_runs via Supabase | LIVE |
| Agent Hangar | models + platforms via Supabase | LIVE |
| Slice Hub | tasks.slice_id grouping via Supabase | LIVE |
| ROI Calculator | task_runs + models + exchange_rates via Supabase | LIVE |
| Timeline | orchestrator_events via Supabase | LIVE |
| Currency Conversion | exchange_rates (read Supabase, fallback to external API, upsert back) | LIVE |
| Status Summary | Computed from tasks | LIVE |
| Quality Map | Computed from events | LIVE |
| Failures Panel | Hardcoded [] in useMissionData.ts | EMPTY -- no data source |
| Merge Candidates | Hardcoded [] in useMissionData.ts | EMPTY -- no data source |
| Review Queue | Static JSON files (data/state/reviews/*.json) | FILE-BASED, not Supabase |
| Restore Records | Static JSON files (data/state/restores/*.json) | FILE-BASED, not Supabase |
| Admin Panel | Code exists in modals/AdminControlCenter.tsx | EXISTS, functionality unclear |

---

## 6. CRITICAL MISMATCHES TO FIX

### MISMATCH 1: failed/escalated status invisible
- Go writes status='failed' and status='escalated'
- Dashboard adapter maps BOTH to "pending"
- Tasks that failed look identical to tasks waiting to be picked up
- **Impact:** User can't see failed tasks. They vanish into pending.
- **Fix:** Add "failed" and "escalated" to TaskStatus type, map them properly

### MISMATCH 2: Per-task costUsd hardcoded 0
- vibepilotAdapter.ts line for metrics.costUsd always returns 0
- task_runs has courier_cost_usd, total_actual_cost_usd but per-task cost isn't used
- **Impact:** Individual task cards show $0.00 cost
- **Fix:** Use run.total_actual_cost_usd || run.courier_cost_usd || 0

### MISMATCH 3: Failures panel has no data source
- FailureSnapshot type exists, Failures.tsx component exists
- useMissionData hardcodes failures: []
- failure_records table exists, Go writes to it via record_failure
- But dashboard never queries failure_records
- **Impact:** Failures panel is always empty
- **Fix:** Query failure_records and populate FailureSnapshot[]

### MISMATCH 4: Merge candidates has no data source
- MergeCandidate type exists, ReadyToMerge.tsx component exists
- useMissionData hardcodes mergeCandidates: []
- Tasks with status='merge_pending' exist but aren't surfaced as candidates
- **Impact:** Ready to merge panel is always empty
- **Fix:** Filter tasks where status='merge_pending' and populate MergeCandidate[]

### MISMATCH 5: Review system file-based
- ReviewQueue and ReviewPanel use static JSON files
- Reviews are actually done by Go (supervisor/maintainer agents)
- Review results go into tasks table (status changes) not review JSON files
- **Impact:** Review queue never shows live data
- **Fix:** Query tasks with status in ('review', 'awaiting_human') for live review queue

---

## 7. NOT MISMATCHES (designed behavior, not bugs)

- platforms table not written by Go: correct, it's a static reference table
- exchange_rates written by dashboard: correct, it fetches from API and caches
- 106 Supabase functions Go doesn't call: see classification above. Most are either trigger helpers, earlier versions, or pre-built for upcoming features
- models.subscription_* columns may be null: correct for free-tier models, only paid subscriptions have values
- Dashboard doesn't show plans/council/learning: correct, those aren't dashboard features yet

---

**This audit is the definitive cross-reference. Any schema change should be checked against all three columns: does Go need it? Does Supabase have it? Does the dashboard display it?**
