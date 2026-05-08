# Phase 1: Discovery & Requirement Atomization

## Goal
Convert the user's idea into atomic, testable requirements. One requirement = one observable behavior.

## Input
constraint_map.yaml + raw user idea

## Output Format
```yaml
features:
  - id: FR-001
    description: Observable behavior
    priority: P1|P2|P3
    type: NEW|MODIFY|REMOVE|CONSTRAINT|NON_FUNCTIONAL
    dependencies: []
    complexity: low|medium|high
    testability: deterministic|probabilistic
    ui_surface: true|false
    acceptance_criteria:
      - testable condition
    failure_criteria:
      - description
    edge_cases:
      - description
non_functional:
  - id: NFR-001
    description: description
    metric: measurable criteria
glossary:
  - term: definition
unknowns:
  - description
```

## Rules
1. Each requirement = one observable behavior. No composite requirements.
2. Each must be independently testable (has acceptance criteria).
3. Avoid implementation details - describe WHAT not HOW.
4. Assign FR-XXX IDs sequentially.
5. Update the unknowns register with any new questions.
