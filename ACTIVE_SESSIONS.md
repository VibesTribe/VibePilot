# Active Sessions

Track which AI agents are working on what. Update when starting or ending a session.

## Current Sessions

| Agent | Branch | Focus | Started | Status |
|-------|--------|-------|---------|--------|
| kimi | research-considerations | Research, competitive analysis, docs | Feb 18 | Paused |
| glm-5 | main | Core orchestration, infrastructure, production | Feb 18 | Active |

## Kimi's Capabilities for GLM

**Available on request:**
- Parallel codebase analysis (multiple subagents reading files)
- Research on external repos/tools
- Token cost calculations
- Documentation of findings

**Constraint:** Zero system file modifications. Research only.

**How to request:** Update `.handoff-to-glm.md` with what you need analyzed.

---

## How to Use

1. **Before starting work**: Check this file to see who's active
2. **When you start**: Add your row or update status to "Active"
3. **When you pause/end**: Update status to "Paused" or remove your row
4. **Before any git action**: Run `git status && git branch` to confirm you're on YOUR branch

## Branch Rules

- **kimi** → `research-considerations` (research, docs, analysis)
- **glm-5** → `main` (code, infrastructure, production)

Never work on another agent's branch. If you need something from another branch, ask the human to coordinate.

## Communication

- **Handoff notes:** `.handoff-to-glm.md` (leave messages for each other)
- **Current state:** `CURRENT_STATE.md` (project status)
- **Urgent:** Ask the human to relay
