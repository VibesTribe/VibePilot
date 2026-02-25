# VibePilot Learning System - Implementation Plan

**Version:** 2.0
**Created:** 2026-02-24
**Updated:** 2026-02-25
**Status:** All Phases COMPLETE

---

## Overview

**Goal:** Build a learning orchestrator that:
1. Routes tasks intelligently based on learned patterns
2. Knows WHY things fail and HOW to fix them
3. Learns from every outcome
4. Self-optimizes daily
5. Stays modular - changing one thing doesn't break others

**Core Principle:**
- Go = Fast, deterministic, free (handles 90%)
- LLM = Smart, adaptive, costs tokens (handles 10%)
- Supabase = Truth (everything reads/writes here)
- Everything swappable without breaking anything else

---

## Architecture Summary

```
┌─────────────────────────────────────────────────────────────────┐
│                    SUPERBASE (TRUTH)                            │
│                                                                 │
│  learned_heuristics      │ Model preferences per task type      │
│  failure_records         │ Structured failure logging           │
│  problem_solutions       │ What fixed what (proven patterns)    │
│  planner_learned_rules   │ Rules from council/supervisor        │
│  tester_learned_rules    │ What tests catch bugs                │
│  supervisor_learned_rules│ What patterns flag issues            │
│  runners (depreciation)  │ Models with archive/revival          │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
         │              │              │              │
         ▼              ▼              ▼              ▼
    ┌─────────┐   ┌─────────┐   ┌─────────┐   ┌─────────┐
    │ PLANNER │   │SUPERVISOR│  │ TESTER  │   │ORCHESTR.│
    │         │   │          │  │         │   │         │
    │Reads:   │   │Reads:    │  │Reads:   │   │Reads:   │
    │- rules  │   │- rules   │  │- rules  │  │- heuris │
    │         │   │          │  │         │  │- failure│
    │Writes:  │   │Writes:   │  │Writes:  │  │- solutn │
    │- rules  │   │- rules   │  │- rules  │  │         │
    └─────────┘   └─────────┘   └─────────┘   └─────────┘
                                                      │
                                                      ▼
                                              ┌──────────────┐
                                              │  DAILY LLM   │
                                              │  (Inside Go) │
                                              │              │
                                              │ Reads: All   │
                                              │ Writes: All  │
                                              │ Reports: GH  │
                                              └──────────────┘
```

---

## Phase 1: Core Learning Infrastructure

**Goal:** Orchestrator routes smarter, records structured failures

### Database Changes (Supabase)

| Table | Purpose |
|-------|---------|
| `learned_heuristics` | Model preferences per task type/condition |
| `failure_records` | Structured failure logging (why it failed) |
| `problem_solutions` | What fixed what (proven patterns) |

### Go Changes

| Change | File | What |
|--------|------|------|
| Structured failure recording | `orchestrator.go` | Record type, category, context |
| Check heuristics when routing | `pool/model_pool.go` | Prefer heuristics, fallback to normal |
| Check problem/solutions | `orchestrator.go` | Skip researcher if known solution |
| Exclude recently failed models | `pool/model_pool.go` | Don't route to what just failed |

### learned_heuristics Schema

```sql
CREATE TABLE learned_heuristics (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  
  -- What task does this apply to?
  task_type TEXT,
  condition JSONB DEFAULT '{}',
  
  -- What should we do?
  preferred_model TEXT,
  action JSONB DEFAULT '{}',
  
  -- How confident?
  confidence FLOAT DEFAULT 0.5,
  auto_apply BOOLEAN DEFAULT false,
  
  -- Tracking
  source TEXT,
  created_at TIMESTAMPTZ DEFAULT NOW(),
  last_applied_at TIMESTAMPTZ,
  application_count INT DEFAULT 0,
  success_rate FLOAT,
  
  -- Expiration
  expires_at TIMESTAMPTZ
);
```

### failure_records Schema

```sql
CREATE TABLE failure_records (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  task_id UUID REFERENCES tasks(id),
  task_run_id UUID REFERENCES task_runs(id),
  
  -- Structured failure info
  failure_type TEXT,
  failure_category TEXT,
  failure_details JSONB,
  
  -- What was attempted
  model_id TEXT,
  platform TEXT,
  runner_id UUID,
  
  -- Context
  task_type TEXT,
  tokens_used INT,
  duration_sec INT,
  
  created_at TIMESTAMPTZ DEFAULT NOW()
);
```

### problem_solutions Schema

```sql
CREATE TABLE problem_solutions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  
  -- The problem
  problem_pattern TEXT,
  problem_category TEXT,
  
  -- What worked
  solution_type TEXT,
  solution_model TEXT,
  solution_details JSONB,
  
  -- Proof it worked
  success_count INT DEFAULT 1,
  failure_count INT DEFAULT 0,
  
  -- For matching
  keywords TEXT[],
  
  created_at TIMESTAMPTZ DEFAULT NOW(),
  last_used_at TIMESTAMPTZ
);
```

### Failure Types

| Failure Type | Category | Example |
|--------------|----------|---------|
| `timeout` | model_issue | Task exceeded time limit |
| `rate_limited` | platform_issue | 429 from API |
| `context_exceeded` | model_issue | Token limit hit |
| `platform_down` | platform_issue | No response from platform |
| `quality_rejected` | quality_issue | Missing deliverables |
| `test_failed` | quality_issue | Tests failed |
| `empty_output` | model_issue | Model returned nothing |
| `latency_high` | platform_issue | Slow response |

---

## Phase 2: Planner Learning

**Goal:** Planner improves after every rejection

### Database Changes

| Table | Purpose |
|-------|---------|
| `planner_learned_rules` | Rules from council/supervisor feedback |

### Integration

| Change | What |
|--------|------|
| Immediate rule creation | When supervisor/council rejects, add rule |
| Inject rules into context | Planner sees rules when planning |
| Track rule effectiveness | Did rule prevent issues? |

### planner_learned_rules Schema

```sql
CREATE TABLE planner_learned_rules (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  
  -- What situation?
  applies_to TEXT,
  
  -- What did we learn?
  rule_type TEXT,
  rule_text TEXT,
  
  -- Where from?
  source TEXT,
  source_task_id UUID,
  
  -- Importance
  priority INT DEFAULT 1,
  
  -- Tracking
  times_applied INT DEFAULT 0,
  times_prevented_issue INT DEFAULT 0,
  
  created_at TIMESTAMPTZ DEFAULT NOW()
);
```

---

## Phase 3: Tester/Supervisor Learning

**Goal:** Tester runs smarter tests, Supervisor catches more issues

### Database Changes

| Table | Purpose |
|-------|---------|
| `tester_learned_rules` | What tests catch bugs |
| `supervisor_learned_rules` | What patterns flag issues |

### tester_learned_rules Schema

```sql
CREATE TABLE tester_learned_rules (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  
  -- What situation?
  applies_to TEXT,
  
  -- What tests?
  test_type TEXT,
  test_command TEXT,
  priority INT,
  
  -- Effectiveness
  caught_bugs INT DEFAULT 0,
  false_positives INT DEFAULT 0,
  
  -- Where from?
  source TEXT,
  
  created_at TIMESTAMPTZ DEFAULT NOW()
);
```

### supervisor_learned_rules Schema

```sql
CREATE TABLE supervisor_learned_rules (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  
  -- What triggers?
  trigger_pattern TEXT,
  trigger_condition JSONB,
  
  -- What to do?
  action TEXT,
  reason TEXT,
  
  -- Where from?
  source TEXT,
  
  -- Effectiveness
  times_triggered INT DEFAULT 0,
  times_caught_issue INT DEFAULT 0,
  
  created_at TIMESTAMPTZ DEFAULT NOW()
);
```

---

## Phase 4: Daily LLM Analysis

**Goal:** System self-optimizes daily

### Configuration

```yaml
governor:
  daily_analysis:
    enabled: true
    time: "00:00 UTC"
    model: "auto"  # Uses get_best_runner for analysis task
```

### What LLM Does

1. **Read from Supabase:**
   - task_runs (what happened)
   - failure_records (what failed)
   - research_suggestions (analysis)
   - learned_heuristics (current rules)

2. **Read from GitHub:**
   - research-considerations branch (online research)

3. **Produce:**
   - Updated heuristics (model preferences)
   - Boost/depreciate recommendations
   - New learned rules
   - Research summary (markdown to GitHub)

4. **Auto-apply:**
   - Heuristics (if confidence > 0.8)
   - Depreciation (bottom tier performers)
   - Revival (new info makes archived viable)

### Output Format

```json
{
  "date": "2026-02-24",
  "analysis": {
    "model_updates": [
      {"model_id": "glm-4", "action": "boost", "reason": "..."},
      {"model_id": "deepseek", "action": "archive", "reason": "..."}
    ],
    "heuristic_updates": [
      {"task_type": "coding", "condition": {"language": "python"}, "preferred_model": "glm-4"}
    ],
    "research_findings": [...]
  }
}
```

---

## Phase 5: Depreciation/Revival System

**Goal:** Models/platforms auto-promoted or archived based on performance

### Runner/Model Status Values

| Status | Meaning | Routed to? |
|--------|---------|------------|
| `active` | Fully available | Yes |
| `cooldown` | Temporarily paused | No (until expires) |
| `rate_limited` | Hit API limits | No (until reset) |
| `paused` | Human paused | No |
| `benched` | Available but not preferred | Only if no active options |
| `archived` | Deprecated | No, but can be revived |

### Depreciation Fields (add to runners)

```sql
ALTER TABLE runners ADD COLUMN depreciation_score FLOAT DEFAULT 0;
ALTER TABLE runners ADD COLUMN depreciation_reasons JSONB DEFAULT '[]';
ALTER TABLE runners ADD COLUMN last_boosted_at TIMESTAMPTZ;
ALTER TABLE runners ADD COLUMN archived_at TIMESTAMPTZ;
ALTER TABLE runners ADD COLUMN archive_reason TEXT;
```

### Depreciation Rules

| Rule | Action |
|------|--------|
| Success rate < 50% of best model | Archive |
| New research finds improvement | Revive |
| Experimental test succeeds | Boost to active |
| Experimental test fails | Re-archive |

### Lifecycle

```
         CREATED
            │
            ▼
         ACTIVE ◀──────────────────────┐
            │                          │
    ┌───────┼───────┐                  │
    ▼       ▼       ▼                  │
cooldown rate_lim paused               │
    │       │       │                  │
    └───────┴───────┘                  │
            │                          │
            ▼                          │
         BENCHED                       │
            │                          │
            ▼                          │
         ARCHIVED ─── new info ────────┘
```

---

## Key Principles

| Principle | How we maintain it |
|-----------|-------------------|
| Everything in Supabase | All learning stored in DB |
| Go = fast, deterministic | Routing is SQL + Go, no LLM per task |
| LLM = 10% edge cases | Daily analysis + escalations only |
| Modular | Each component has its own rules table |
| Swappable | Change one table doesn't affect others |
| Graceful degradation | No heuristics? System still works via task_ratings |
| Always available | Preferred model unavailable? Fallback automatically |
| Configurable | All timing, thresholds in governor.yaml |

---

## Implementation Checklist

### Phase 1 (COMPLETE)
- [x] Create `024_learning_system.sql` migration
- [x] Add failure recording to orchestrator
- [x] Add heuristic checking to pool routing
- [x] Add problem/solutions lookup
- [x] Add RPCs for new tables

### Phase 2 (COMPLETE)
- [x] Create `planner_learned_rules` table (`025_planner_learning.sql`)
- [x] Add immediate rule creation on rejection
- [x] Inject rules into planner context (planner supports it, integration pending)
- [x] Track rule effectiveness

### Phase 3 (COMPLETE - Session 30)
- [x] Create `tester_learned_rules` table (`028_tester_supervisor_learning.sql`)
- [x] Create `supervisor_learned_rules` table
- [x] Track test effectiveness (RPCs + Go methods)
- [x] Track supervisor pattern effectiveness
- [x] Wire rejection → rule creation in orchestrator

### Phase 4 (COMPLETE - Session 30)
- [x] Add daily analysis scheduler (analyst/analyst.go)
- [x] Implement LLM analysis call
- [x] Parse LLM output
- [x] Apply updates to all rule tables
- [x] Write summary to GitHub
- [x] Gather all learning tables in analysis

### Phase 5 (COMPLETE)
- [x] Add depreciation fields to runners (`026_depreciation_system.sql`)
- [x] Implement depreciation scoring
- [x] Implement revival logic
- [x] Janitor auto-archives underperformers

---

## Remaining Integration (Optional Enhancement)

### Tester Rule Injection
- [ ] Tester reads and uses `tester_learned_rules`
- [ ] Track which learned tests catch bugs

### Supervisor Rule Injection
- [ ] Supervisor reads and uses `supervisor_learned_rules`
- [ ] Run learned patterns alongside hardcoded ones

---

## Related Files

- `docs/supabase-schema/024_learning_system.sql` - Phase 1 migration
- `docs/GOVERNOR_HANDOFF.md` - Governor handoff notes
- `docs/prd_v1.4.md` - Full system specification
- `docs/SYSTEM_REFERENCE.md` - How things work

---

## Notes

- Research narrative stays in GitHub (research-considerations branch)
- Only structured/executable learning goes to Supabase
- Daily analysis configurable in governor.yaml
- All thresholds configurable
- LLM model for analysis selected via get_best_runner (same routing logic)
