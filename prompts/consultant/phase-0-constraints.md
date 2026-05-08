# Phase 0: Constraint & Operating Envelope Extraction

## Goal
Extract all explicit and implicit constraints BEFORE any functional decomposition begins. Constraints are globally influential - without them, later phases produce locally optimal but globally invalid decisions.

## Input
Raw user idea and initial conversation.

## Output Format
```yaml
constraints:
  hard:
    - description
  soft:
    - description
  forbidden_patterns:
    - description
  reversibility:
    - decision
preferences:
  - description
unknowns:
  - description
assumptions:
  - description
contradictions:
  - description
```

## Rules
1. Extract all explicit constraints from the user's description.
2. Do NOT infer missing values - mark as unknowns.
3. Do NOT resolve ambiguity silently.
4. Separate: hard constraints, soft preferences, unknowns, contradictions.
5. This output is IMMUTABLE context for all later phases.
