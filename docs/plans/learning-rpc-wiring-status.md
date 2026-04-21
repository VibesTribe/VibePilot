# VibePilot Learning RPC Wiring Status
# Generated: 2026-04-20, verified by grepping all Go source files
# Schema has 20+ learning RPCs defined. Here's what's actually called.

## WIRED AND ACTIVE

| RPC | Calls | Where Used | Purpose |
|-----|-------|------------|---------|
| record_model_success | 7 | maint, plan, task, helpers, rpc, validate | Record model success on completion |
| record_model_failure | 7 | plan, task, helpers, rpc, validate | Record model failure |
| update_model_learning | 4 | task, testing | Update models.learned JSONB |
| record_performance_metric | 5 | plan, rpc, validate, state | Duration + success tracking |
| record_state_transition | 5 | rpc, validate, state | Full audit trail of state changes |
| record_failure | 4 | task, rpc, validate | Structured failure logging |
| create_planner_rule | 3 | council, rpc, validate | Learn from council rejections |
| get_planner_rules | 4 | context_builder, rpc, validate | Load learned rules for planner context |
| record_planner_revision | 4 | council, rpc, validate | Track plan revision history |
| get_recent_failures | 3 | context_builder, rpc, validate | Load recent failures for agent context |
| get_heuristic | 3 | context_builder, rpc, validate | Load routing heuristics |
| get_problem_solution | 4 | context_builder, rpc, validate | Look up known solutions |
| upsert_heuristic | 1 | rpc | Create/update routing heuristic |
| record_heuristic_result | 1 | rpc | Track heuristic effectiveness |
| record_solution_result | 1 | rpc | Track solution effectiveness |
| record_planner_rule_applied | 1 | rpc | Track when a learned rule was used |

## ORPHAN (exists in schema, never called from Go)

| RPC | Designed Purpose | Why Orphan |
|-----|-----------------|------------|
| record_planner_rule_prevented_issue | Track when a learned rule prevented a bad outcome | Never wired - would need to detect when a rule fires |
| create_rule_from_rejection | Auto-create planner rule from rejection feedback | Never wired - would need supervisor/council rejection handler |

## CRITICAL INSIGHT: context_builder.go

The context_builder.go file reads from get_planner_rules, get_recent_failures, 
get_heuristic, and get_problem_solution. This means the feedback loop is PARTIALLY 
there -- the governor READS learned data to build agent context. But the WRITE side 
is incomplete:

- Council rejection → create_planner_rule: WIRED (handlers_council.go calls it)
- Supervisor rejection → create_planner_rule: NOT WIRED
- Test failure → record_failure: WIRED (handlers_task.go)
- Test failure → update_model_learning: WIRED (handlers_testing.go)
- Model success → record_model_success: WIRED (multiple handlers)
- Model failure → record_model_failure: WIRED (multiple handlers)

## THE REAL GAP

It's not that the learning infrastructure doesn't exist. It does.
The gap is:

1. Council handler calls create_planner_rule but NOT record_usage, NOT record_completion,
   NOT update_model_learning for the council member models themselves.

2. Research handler calls NONE of the learning RPCs. No record of which models
   produce good research or bad research.

3. The feedback loop for "what worked" reinforcement is weak. We record failures
   well. We don't reinforce successes back to the specific model + task type.

4. The context_builder loads learned data but the router doesn't use it for
   model scoring yet (GetModelLearnedScore was just added this session but 
   only reads UsageTracker in-memory data, not the Supabase learning tables).

5. record_planner_rule_prevented_issue and create_rule_from_rejection are orphan --
   the system can create rules but can't verify they helped.
