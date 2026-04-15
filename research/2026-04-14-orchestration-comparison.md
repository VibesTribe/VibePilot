# Research: GSD vs Superpowers vs Claude Code -- Orchestration Trade-offs

**Date:** 2026-04-14
**Source:** https://www.youtube.com/watch?v=celLbDMGy8w
**Relevance:** HIGH -- validates VibePilot's "Vanilla with Skills" approach

## The Comparison
Three orchestration layers for Claude Code, building the same web app:

| Metric | Vanilla Claude Code | Superpowers | GSD |
|---|---|---|---|
| Total Time | 20 min | 1 hour | 1h45m |
| Total Tokens | 200k | 250k | 1.2M |
| Success Rate | 2nd attempt | 1st attempt (one-shot) | 2nd attempt |
| Planning Phase | 10 min / 50k tok | 40 min / 200k tok | 40 min / 600k tok |

## Key Findings

### 1. The Complexity Line
GSD's massive overhead (4 sub-agents, 600k tokens for planning) is NOT worth it for most tasks.
Vanilla + manual refinement beats autonomous agents in long planning loops.

### 2. Visual Prototyping Wins
Superpowers' "Visual Companion" was the ONLY one to one-shot the task.
Letting the human choose aesthetics BEFORE code is written prevents AI slop.

### 3. State Management Prevents Context Rot
GSD's strength: rigid state files updated every step. Prevents agent from forgetting
the original goal during long sessions.

### 4. Security Intelligence
Superpowers was the only one to flag a hidden admin page without auth as "security by obscurity."
Higher consulting intelligence than the others.

## What VibePilot Already Does Right
- We chose "Vanilla with Skills" -- tier0 says start simple, iterate
- .context/ system is our state management (prevents context rot)
- tier0 rules prevent over-engineering (GSD-style would violate our principles)
- Budget-conscious by design (GSD's 1.2M tokens would burn free tiers instantly)

## What We Should Adopt
1. **Pre-execution design preview** -- for UI tasks, show human a design choice BEFORE
   writing code, not just after. This is our Visual QA moved upstream.
2. **Conditional planning depth** -- simple tasks get vanilla treatment, complex tasks
   get deeper planning. Our pipeline YAML already has `when:` conditionals for this.
3. **State recorder pattern** -- after every major step, write progress to markdown.
   This is essentially what .context/ boot.md already does, but should be per-task.

## What We Should NOT Adopt
- GSD-style 4 sub-agent planning phase (600k tokens for planning is insane)
- Heavy state file management (our knowledge.db + boot.md is lighter and better)
- Any pattern that requires Claude specifically (we're model-agnostic)

## Priority: Low
These are validation of existing decisions, not new features. The one actionable item
(pre-execution design preview) can be a pipeline stage when we build courier agents.

## Conclusion
VibePilot's approach is validated. Vanilla execution + lightweight state + conditional
deep planning is the right path. The only thing to add is the visual preview gate for
UI tasks, which we already have planned as a pipeline stage.
