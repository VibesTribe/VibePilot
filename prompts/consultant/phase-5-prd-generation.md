# Phase 5: PRD Generation & Critique-Revise

## Part A: PRD Generation

Generate a planner-optimized PRD from all phase outputs. Use the VibePilot PRD template at `config/templates/prd-template.md`.

### Requirements
1. Every requirement traces to a FR-XXX, NFR, or ARCH ID.
2. Every acceptance criterion is testable (Given/When/Then).
3. Every failure criterion is specified (not just success paths).
4. Resource constraints are embedded in the architecture section.
5. Include a planner compatibility block:
```
features:
  - id: FR-001
    target_file: path/to/file
    dependencies: []
    complexity: low|medium|high
    testability: deterministic|probabilistic
```

## Part B: Critique-Revise

Run TWO independent reviews on the generated PRD:

### Review 1 - Completeness
Find gaps, missing requirements, unresolved ambiguities. Attempt to break the specification.

### Review 2 - Minimalism  
Find over-engineering, speculative features, unnecessary complexity.

### Review Output
```yaml
review:
  type: completeness|minimalism
  findings:
    - severity: critical|major|minor
      location: section
      issue: description
      suggested_fix: description
  verdict: pass|revision_needed|reject
```

### Rules
1. Do NOT ask "Review this PRD" - ask "Attempt to break this specification."
2. Find: ambiguous language, contradictory requirements, hidden assumptions, untestable behaviors.
3. Maximum revision cycles: 2.
4. After 2 cycles, escalate with specific disagreement details.
