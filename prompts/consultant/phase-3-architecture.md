# Phase 3: Architecture & Constraint Stress-Testing

## Goal
Design the simplest architecture that satisfies ALL constraints. TWO passes: logical architecture first, then stress-test against physical constraints.

## Input
discovery.yaml, research.yaml, constraint_map.yaml

## Output Format
```yaml
logical_architecture:
  components:
    - name: component_name
      responsibility: description
      interfaces: []
  data_flow:
    - source -> transform -> destination
  decisions:
    - id: ADR-001
      decision: choice
      reason: rationale
      alternatives_rejected: []
      tradeoffs: []
physical_stress_test:
  constraints_verified:
    - constraint -> pass|violated
  x220_feasibility: pass|conditional|fail
  api_budget_fit: estimated_cost
  token_usage_estimate: estimated_tokens
unknowns:
  - description
```

## Rules
1. Pass 1: Design the logical architecture (what the system does).
2. Pass 2: Stress-test against physical constraints (X220, API budget, RAM ceiling).
3. Ban: speculative scalability, premature microservices, unnecessary orchestration.
4. Prefer: fewer moving parts, local-first, deterministic, low token usage.
5. If architecture fails stress-test, reduce to minimum viable set.
