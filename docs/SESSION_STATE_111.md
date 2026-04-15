# Session State: Migration 111 - Missing RPCs
# Date: 2026-04-15
# Status: IN PROGRESS - migration written but has schema mismatches, needs rewrite

## WHAT WE'RE DOING
The Go governor binary calls 50 RPCs on Supabase. Only 8 exist. The other 42 return 404.
The governor runs but is essentially blind - nearly every operation fails silently.
We wrote a single migration (111_missing_rpcs.sql) to create all 42 missing RPCs at once.
BUT the migration references columns that don't exist in Supabase. Must fix before applying.

## FILE LOCATION
Migration file: /home/vibes/VibePilot/docs/supabase-schema/111_missing_rpcs.sql (32KB, written but WRONG)

## SCHEMA MISMATCHES FOUND (must fix in migration)

### maintenance_commands table
- WRONG: migration uses `type` column
- ACTUAL: column is `command_type`
- Affects: create_maintenance_command(), update_maintenance_command_status()

### council_reviews table
- WRONG: migration uses `reviewer_model`, `reasoning`, `mode` columns
- ACTUAL: has `model_id`, `vote`, `concerns`, `plan_id`, `created_at` (NO reasoning, NO mode)
- Affects: store_council_reviews()

### failure_records table
- WRONG: migration uses `details` column
- ACTUAL: has `id`, `task_id`, `failure_type`, `model_id`, `created_at` (NO details column)
- Affects: record_model_failure(), record_failure()

### plans table
- WRONG: migration references `latest_feedback`, `tasks_needing_revision` columns
- ACTUAL: has `review_notes` (NOT latest_feedback), NO tasks_needing_revision
- HAS: `council_mode`, `council_models`, `revision_round`, `revision_history`, `review_notes`
- Affects: update_plan_status(), record_planner_revision(), set_council_consensus()

### tasks table
- WRONG: migration references `last_error`, `last_error_at`
- ACTUAL: has `failure_notes` but NO `last_error`, NO `last_error_at`
- Affects: record_failure()

### learned_heuristics table
- HAS: `id`, `task_type`, `condition`, `preferred_model`, `action`, `confidence`, 
  `auto_apply`, `source`, `created_at`, `last_applied_at`, `application_count`,
  `success_count`, `failure_count`, `success_rate`, `expires_at`
- No UNIQUE constraint on (task_type, preferred_model) - the ON CONFLICT in migration will fail
- Affects: record_model_success(), record_model_failure()

### task_runs table
- HAS all needed columns including: `started_at`, `completed_at`
- Migration needs to add `p_started_at` and `p_completed_at` params (Go code passes them)

### memory_sessions table
- HAS: (needs column check, was empty, created by migration 110)
- The store_memory/recall_memories RPCs reference columns that may not match

### memory_project table  
- HAS: (needs column check, was empty, created by migration 110)
- RPCs reference `project_id`, `key`, `value`, `updated_at` - must verify

### memory_rules table
- HAS: (needs column check, was empty, created by migration 110)
- RPCs reference `category`, `rule_text`, `source`, `priority`, `confidence`, `updated_at` - must verify

## EXISTING RPCs (8 - already work, don't recreate)
1. find_orphaned_sessions
2. find_pending_resource_tasks
3. find_tasks_with_checkpoints
4. get_recent_failures
5. get_slice_task_info
6. get_supervisor_rules
7. get_tester_rules
8. get_heuristic
(+ record_failure EXISTS but has param mismatch - fix it in migration)

## ALL 42 MISSING RPCs WITH CORRECT PARAMS (from Go code)

### Task Lifecycle
1. claim_task(p_task_id UUID, p_worker_id TEXT, p_model_id TEXT)
2. claim_for_review(p_task_id UUID, p_reviewer_id TEXT)  
3. create_task_run(p_task_id, p_model_id, p_courier, p_platform, p_status, p_tokens_in, p_tokens_out, p_tokens_used, p_courier_model_id, p_courier_tokens, p_courier_cost_usd, p_platform_theoretical_cost_usd, p_total_actual_cost_usd, p_total_savings_usd, p_started_at, p_completed_at)
4. calculate_run_costs(p_model_id, p_tokens_in, p_tokens_out, p_courier_cost_usd)

### Plan Lifecycle
5. create_plan(p_project_id UUID, p_prd_path TEXT, p_plan_path TEXT)
6. update_plan_status(p_plan_id UUID, p_status TEXT, p_review_notes JSONB)
   - writes to `review_notes` NOT `latest_feedback`

### Processing State
7. set_processing(p_table TEXT, p_id UUID, p_processing_by TEXT)
8. clear_processing(p_table TEXT, p_id UUID)
9. find_stale_processing(p_table TEXT, p_timeout_seconds INT)
10. recover_stale_processing(p_table TEXT, p_id UUID, p_reason TEXT)

### Learning
11. record_model_success(p_model_id, p_task_type, p_duration_seconds, p_tokens_used)
12. record_model_failure(p_model_id, p_task_id, p_failure_type, p_failure_category)
    - writes to failure_records(task_id, failure_type, model_id) - NO details col
13. record_failure(p_task_id UUID, p_failure_type TEXT, p_failure_category TEXT, 
    p_failure_details JSONB, p_model_id TEXT, p_task_type TEXT)
    - writes to failure_records(task_id, failure_type, model_id) for the record
    - writes to tasks.failure_notes for the notes (NO last_error/last_error_at)

### Council
14. store_council_reviews(p_plan_id UUID, p_reviews JSONB, p_mode TEXT)
    - inserts into council_reviews(plan_id, model_id, vote, concerns) 
    - updates plans.council_mode for mode
15. set_council_consensus(p_plan_id UUID, p_consensus TEXT)
    - writes to plans.review_notes (NOT latest_feedback)

### Maintenance
16. create_maintenance_command(p_command_type TEXT, p_payload JSONB)
    - inserts into maintenance_commands(command_type, ...) NOT type
17. update_maintenance_command_status(p_id UUID, p_status TEXT, p_result_notes JSONB)
    - writes to result column
18. queue_maintenance_command(p_command_type TEXT, p_payload JSONB, p_priority INT)
    - alias for create_maintenance_command

### Research
19. update_research_suggestion_status(p_id UUID, p_status TEXT, p_review_notes JSONB)

### Dependencies
20. unlock_dependent_tasks(p_completed_task_id UUID)

### Routing
21. check_platform_availability(p_platform_id TEXT)

### Analyst
22. get_model_performance() - no params
23. get_failure_patterns(days INT)

### Security
24. log_security_audit(p_operation TEXT, p_key_name TEXT, p_allowed BOOLEAN)

### Planner Learning
25. create_planner_rule(p_applies_to TEXT, p_rule_type TEXT, p_rule_text TEXT, p_source TEXT)
26. record_planner_revision(p_plan_id UUID, p_concerns JSONB, p_tasks_needing_revision JSONB)
    - writes to plans.revision_round, plans.revision_history, plans.review_notes
27. record_revision_feedback(p_plan_id UUID, p_source TEXT, p_feedback JSONB, p_tasks_needing_revision JSONB)
    - writes to new revision_feedback table AND plans.review_notes

### Memory (3-layer)
28. store_memory(p_layer TEXT, p_key TEXT, p_value TEXT, p_ttl_sec INT)
29. recall_memories(p_layer TEXT, p_query TEXT, p_limit INT)

### Recovery
30. recover_orphaned_session(p_session_id UUID, p_reason TEXT)

### These also from the full 42 list but not in Go param extraction:
31. update_task_branch(p_task_id UUID, p_branch TEXT) - from migration 074
32. get_next_task_number_for_slice(p_slice TEXT) - from migration 093
33. save_checkpoint(UUID, TEXT, INT, TEXT, JSONB) - from migration 057
34. load_checkpoint(UUID) - from migration 057
35. delete_checkpoint(UUID) - from migration 057
36. get_model_score_for_task - from router.go
37. get_latest_state(p_entity_type TEXT, p_entity_id UUID) - from 050
38. record_state_transition(...) - from 050
39. record_performance_metric(...) - from 050

Plus a few more in the RPC allowlist but may not be actively called:
40. increment_revision_round
41. check_revision_limit
42. add_council_review

## NEW TABLES CREATED IN MIGRATION (these are fine, no conflicts)
- security_audit_log
- planner_rules
- revision_feedback

## TABLES FROM MIGRATION 050 THAT MAY NOT EXIST (need to verify)
- state_transitions (for record_state_transition)
- performance_metrics (for record_performance_metric)
These were in the schema files but returned 404 when tested.

## NEXT STEPS (do these in order)

1. FIX the migration SQL - rewrite 111_missing_rpcs.sql with correct column names
   Key fixes:
   - maintenance_commands: `type` → `command_type`
   - council_reviews: `reviewer_model` → `model_id`, remove `reasoning`/`mode`
   - failure_records: remove `details` column ref
   - plans: `latest_feedback` → `review_notes`, remove `tasks_needing_revision` ref
   - tasks: remove `last_error`/`last_error_at`, use only `failure_notes`
   - learned_heuristics: remove ON CONFLICT (no unique constraint), use INSERT + separate UPDATE
   - create_task_run: add p_started_at and p_completed_at params
   - memory tables: verify actual column names from migration 110 schema

2. VERIFY memory table columns from migration 110 schema file
   Read: ~/VibePilot/docs/supabase-schema/110_*.sql

3. APPLY migration to Supabase via SQL editor or curl

4. VERIFY all 50 RPCs return non-404 (test each one)

5. REBUILD governor binary (cd ~/VibePilot/governor && go build -o governor ./cmd/governor/)

6. WIRE MemoryService into main.go (currently only Compactor is wired)

7. PUSH to GitHub

## DASHBOARD IMPACT
The dashboard (vibeflow-dashboard.vercel.app) is READ-ONLY on Supabase.
It reads: tasks, task_runs, models, platforms, orchestrator_events.
Our RPCs only WRITE data. Dashboard should not break.
BUT: any column the dashboard expects must already exist (they do - verified).

## GOVERNOR STATE
- PID may be stale, needs pkill + restart after migration applied
- Running copy: ~/vibepilot/ (must git pull after push)
- Dev repo: ~/VibePilot/
- Start: cd ~/vibepilot/governor && source ~/.governor_env && nohup ./governor &>/tmp/governor.log &
- Before restart: pkill -f governor; sleep 1; check port 8080 free
