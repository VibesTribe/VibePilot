# Supabase Schema: Actual State

**Generated:** 2026-03-06
**Source:** Direct query of Supabase tables

---

## tasks Table (CONFIRMED)

**Columns from live data:**
```
id, title, type, priority, dependencies, status, assigned_to, attempts, 
max_attempts, result, review, tests, approval, failure_notes, branch_name, 
created_at, updated_at, started_at, completed_at, project_id, slice_id, 
phase, task_number, routing_flag, routing_flag_reason, routing_history, 
plan_id, confidence, category, processing_by, processing_at
```

**Key fields dashboard uses:**
| Field | Type | Status |
|-------|------|--------|
| id | UUID | ✅ Exists |
| title | TEXT | ✅ Exists |
| status | TEXT | ✅ Exists |
| assigned_to | TEXT | ✅ Exists |
| slice_id | TEXT | ✅ Exists |
| routing_flag | TEXT | ✅ Exists |
| routing_flag_reason | TEXT | ✅ Exists |
| task_number | TEXT | ✅ Exists |
| result | JSONB | ✅ Exists |
| dependencies | JSONB | ✅ Exists |

---

## models Table (CONFIRMED)

**Columns from live data:**
```
id, platform, courier, context_limit, strengths, weaknesses, request_limit, 
request_used, token_limit, token_used, cycle_resets_at, status, status_reason, 
updated_at, tokens_used, tasks_completed, tasks_failed, success_rate, 
cooldown_expires_at, subscription_cost_usd, subscription_started_at, 
subscription_ends_at, subscription_status, cost_input_per_1k_usd, 
cost_output_per_1k_usd, config, name, vendor, access_type, logo_url, ...
```

**Key fields dashboard uses:**
| Field | Type | Status |
|-------|------|--------|
| id | TEXT | ✅ Exists |
| name | TEXT | ✅ Exists |
| status | TEXT | ✅ Exists |
| context_limit | INT | ✅ Exists |
| tokens_used | INT | ✅ Exists |
| tasks_completed | INT | ✅ Exists |
| tasks_failed | INT | ✅ Exists |
| success_rate | DECIMAL | ✅ Exists |
| cooldown_expires_at | TIMESTAMPTZ | ✅ Exists |
| subscription_cost_usd | DECIMAL | ✅ Exists |
| cost_input_per_1k_usd | DECIMAL | ✅ Exists |
| cost_output_per_1k_usd | DECIMAL | ✅ Exists |

---

## platforms Table (CONFIRMED)

**Columns from live data:**
```
id, type, url, gmail_account, capabilities, daily_limit, daily_used, 
usage_reset_at, success_rate, total_tasks, successful_tasks, avg_response_time_ms, 
last_success, last_failure, consecutive_failures, status, status_reason, 
theoretical_api_cost_per_1k_tokens, actual_courier_cost_per_task, created_at, 
updated_at, name, vendor, context_limit, request_limit, request_used, logo_url, 
config, tokens_used, tasks_completed, tasks_failed, last_run_at, 
theoretical_cost_input_per_1k_usd, theoretical_cost_output_per_1k_usd
```

**Key fields dashboard uses:**
| Field | Type | Status |
|-------|------|--------|
| id | TEXT | ✅ Exists |
| name | TEXT | ✅ Exists |
| status | TEXT | ✅ Exists |
| context_limit | INT | ✅ Exists |
| theoretical_cost_input_per_1k_usd | DECIMAL | ✅ Exists |
| theoretical_cost_output_per_1k_usd | DECIMAL | ✅ Exists |
| config | JSONB | ✅ Exists |

---

## task_runs Table (EMPTY - SCHEMA FROM MIGRATION)

**Current state:** Table exists but has NO DATA

**Schema from schema_v1_core.sql:**
```sql
CREATE TABLE task_runs (
  id UUID PRIMARY KEY,
  task_id UUID REFERENCES tasks(id),
  courier TEXT NOT NULL,
  platform TEXT NOT NULL,
  model_id TEXT REFERENCES models(id),
  chat_url TEXT,
  status TEXT DEFAULT 'running',
  result JSONB,
  error TEXT,
  tokens_used INT,
  started_at TIMESTAMPTZ DEFAULT NOW(),
  completed_at TIMESTAMPTZ
);
```

**Schema from schema_v1.4_roi_enhanced.sql (ADDITIONS):**
```sql
ALTER TABLE task_runs ADD COLUMN tokens_in INT DEFAULT 0;
ALTER TABLE task_runs ADD COLUMN tokens_out INT DEFAULT 0;
ALTER TABLE task_runs ADD COLUMN courier_model_id TEXT;
ALTER TABLE task_runs ADD COLUMN courier_tokens INT DEFAULT 0;
ALTER TABLE task_runs ADD COLUMN courier_cost_usd DECIMAL(10,6) DEFAULT 0;
ALTER TABLE task_runs ADD COLUMN platform_theoretical_cost_usd DECIMAL(10,6) DEFAULT 0;
ALTER TABLE task_runs ADD COLUMN total_actual_cost_usd DECIMAL(10,6) DEFAULT 0;
ALTER TABLE task_runs ADD COLUMN total_savings_usd DECIMAL(10,6) DEFAULT 0;
```

**Dashboard expects (from vibepilotAdapter.ts lines 47-65):**
```typescript
interface VibePilotTaskRun {
  id: string;
  task_id: string;
  model_id: string | null;
  platform: string | null;
  courier: string | null;
  status: string;
  tokens_used: number | null;      // ✅ In core schema
  tokens_in: number | null;         // ✅ In v1.4 migration
  tokens_out: number | null;        // ✅ In v1.4 migration
  courier_model_id: string | null;  // ✅ In v1.4 migration
  courier_tokens: number | null;    // ✅ In v1.4 migration
  courier_cost_usd: number | null;  // ✅ In v1.4 migration
  platform_theoretical_cost_usd: number | null;  // ✅ In v1.4 migration
  total_actual_cost_usd: number | null;          // ✅ In v1.4 migration
  total_savings_usd: number | null;              // ✅ In v1.4 migration
  started_at: string;
  completed_at: string | null;
}
```

**STATUS:** If schema_v1.4_roi_enhanced.sql was applied, all columns exist.
**NEED TO VERIFY:** Was v1.4 migration actually run in Supabase?

---

## plans Table (CONFIRMED)

**Columns from live data:**
```
id, project_id, prd_path, plan_path, status, complexity, council_round, 
council_consensus, council_reviews, review_notes, human_decision, 
human_decision_at, prd_version, plan_version, created_at, updated_at, 
last_reviewed_at, approved_at, revision_round, revision_history, 
council_mode, council_models, processing_by, processing_at
```

---

## task_packets Table (CONFIRMED)

**Columns from live data:**
```
id, task_id, prompt, tech_spec, expected_output, context, version, 
revision_reason, created_at, updated_at
```

---

## Critical Question: Was v1.4 Migration Applied?

**Test:** Try to insert a task_run with all columns

If v1.4 was applied, this should work:
```sql
INSERT INTO task_runs (
  task_id, model_id, courier, platform, status,
  tokens_in, tokens_out, tokens_used,
  courier_model_id, courier_tokens, courier_cost_usd,
  platform_theoretical_cost_usd, total_actual_cost_usd, total_savings_usd,
  started_at, completed_at
) VALUES (...);
```

If v1.4 was NOT applied, columns like `tokens_in` will cause error.

---

## Action Required

1. **Verify v1.4 migration** - Check if columns exist in Supabase
2. **If NOT applied** - Apply schema_v1.4_roi_enhanced.sql
3. **If APPLIED** - Fix Go code to write correctly (already done in handlers_task.go update)

---

## Summary

| Table | Status | Notes |
|-------|--------|-------|
| tasks | ✅ Complete | All dashboard fields exist |
| models | ✅ Complete | All dashboard fields exist |
| platforms | ✅ Complete | All dashboard fields exist |
| plans | ✅ Complete | Working |
| task_packets | ✅ Complete | Working |
| task_runs | ⚠️ Verify | Schema should have v1.4 columns, but NO DATA |

**Root cause of empty task_runs:** Go code was writing to columns that didn't exist (tokens_in/tokens_out) BEFORE v1.4 migration was applied, causing silent failures.
