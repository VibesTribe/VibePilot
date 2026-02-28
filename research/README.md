# Research Directory

This branch contains research findings and human review documents.

## Structure

```
research/
├── YYYY-MM-DD-short-name.md    # Review documents for human
└── ...
```

## Document Lifecycle

1. **Created:** When Council reviews a complex suggestion
2. **Updated:** System Researcher adds Council feedback
3. **Flagged:** Appears in human dashboard as "Review Needed"
4. **Decided:** Human approves/rejects/asks questions
5. **Closed:** Status updated, Maintenance implements if approved

## Naming Convention

`YYYY-MM-DD-short-descriptive-name.md`

Examples:
- `2026-02-28-grok-api-addition.md`
- `2026-02-28-vector-db-rag-system.md`
- `2026-03-01-pricing-update-deepseek.md`

## Branch Rules

- This is the `research` branch
- Merge to main only after human approval
- Documents are permanent record
- Link to these from dashboard
