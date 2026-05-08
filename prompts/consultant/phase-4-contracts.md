# Phase 4: Dependency & Interface Contract Definition

## Goal
Define exact interfaces between components. Task agents need exact data shapes, error contracts, side effects - not vague descriptions.

## Input
architecture.yaml, discovery.yaml

## Output Format
```yaml
contracts:
  - id: C-001
    component: component_name
    inputs:
      - name: parameter
        type: data_shape
        constraints: []
    outputs:
      - name: result
        type: data_shape
        failure_modes:
          - error: description
            recovery: automatic|manual|impossible
    side_effects:
      - description
    state_requirements:
      - persistent|transient|none
unknowns:
  - description
```

## Rules
1. Every interface between components must be explicitly defined.
2. Error contracts are as important as success contracts.
3. Specify recovery strategy for each failure mode.
4. If a contract cannot be fully specified, it stays in unknowns register.
