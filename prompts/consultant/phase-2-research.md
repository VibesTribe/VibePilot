# Phase 2: Market Research & Gap Analysis

## Goal
Analyze the competitive landscape, identify gaps, and validate product positioning. Focus on why similar systems fail, not just what they do.

## Input
discovery.yaml, constraint_map.yaml

## Output Format
```yaml
competitors:
  - name: Competitor
    strengths: []
    weaknesses: []
    pricing: description
    user_complaints: []
    feature_gaps: []
market_gaps:
  - description
technical_patterns:
  - pattern
common_failures:
  - description
minimal_viable_feature_set:
  - feature_id
positioning:
  differentiators: []
  what_to_avoid: []
unknowns:
  - description
```

## Rules
1. Analyze why similar systems FAIL, not just what they offer.
2. Identify minimum viable feature set - do NOT inflate scope with competitor feature creep.
3. Do NOT design architecture here. Research produces validated constraints and positioning.
4. At least 3 direct competitors analyzed.
