# VibePilot v1.3 — Comprehensive PRD
## Sovereign AI Execution Engine

**Version:** 1.3  
**Status:** Foundational Rebuild  
**Date:** 2026-02-13  
**Audience:** GLM-5, Future Models, Engineers, Operators

---

# 1. Executive Summary

VibePilot is a **sovereign, modular AI software factory** that orchestrates multi-agent execution across web platforms and in-house CLI tools.

**Core Philosophy:**
- All vendors will fail eventually → Design for swap
- State lives outside models → Supabase as source of truth
- Execution is ephemeral → No persistent compute
- Governance before execution → Council approval required
- Real-world data drives decisions → ROI on every task

**What VibePilot Is:**
- An orchestration engine
- A task factory with deterministic execution
- A model-agnostic dispatch system
- A learning system that evolves with the market

**What VibePilot Is NOT:**
- A chatbot
- A conversational assistant
- Dependent on any single vendor
- A replacement for human oversight on UI/UX

---

# 2. System Architecture

## 2.1 High-Level Flow

```
IDEA → CONSULTANT → PRD → PLANNER → ATOMIC PLAN
                                         ↓
                                    COUNCIL REVIEW
                                         ↓
                               ┌────────┴────────┐
                               │                 │
                          APPROVED           REVISION
                               │              REQUIRED
                               ↓                 │
                           DIRECTOR ←────────────┘
                               │
                    ┌──────────┼──────────┐
                    │          │          │
               IN-HOUSE    COURIER     COURIER
               (Complex)   (Small)    (Small)
                    │          │          │
                    ↓          ↓          ↓
               OpenCode    Platform A  Platform B
               Kimi CLI    (Web AI)    (Web AI)
                    │          │          │
                    └──────────┼──────────┘
                               │
                               ↓
                          SUPERVISOR
                               │
                    ┌──────────┴──────────┐
                    │                     │
                PASSED                 FAILED
                    │                     │
                    ↓                     ↓
               MAINTENANCE          FAILURE ANALYSIS
               (Merge)              (Classify → Reassign/Escalate)
                    │
                    ↓
               COMPLETE
               (Dependencies Unlocked)
```

## 2.2 Component Layers

### Layer 1: Gateway (Angie/Nginx)
- HTTPS termination
- Internal routing
- Auth header enforcement
- Metrics endpoint
- **Must not:** Contain business logic, store state

### Layer 2: Director (Vibes)
- Poll Supabase for queued tasks
- Classify task complexity
- Route to in-house OR courier
- Enforce retry policy
- Enforce timeout policy (30 min, 80% threshold)
- Track model/platform performance
- Log ROI metrics

### Layer 3: State (Supabase)
- PRDs
- Atomic tasks
- Task states
- Retry counters
- Model registry
- Platform registry
- Council decisions
- ROI metrics
- Chat URLs
- Execution logs

### Layer 4: Execution Hands

| Hand | Type | When Used |
|------|------|-----------|
| **In-House CLI** | Primary | Complex tasks, dependencies, context-heavy, system-critical |
| **Courier** | Dispatch | Small tasks, independent, parallel-capable |

---

# 3. Task Routing Logic (Director Decision Matrix)

## 3.1 In-House vs Courier Decision

```python
def route_task(task):
    # Always in-house
    if task.type in ['orchestration', 'supervision', 'council', 'maintenance', 'testing']:
        return 'in-house'
    
    # Check dependencies
    if len(task.dependencies) > 2:
        return 'in-house'  # Context window risk
    
    # Check context size
    if task.estimated_context > MAX_CONTEXT * 0.8:
        return 'in-house'  # Stay under 80% threshold
    
    # Check criticality
    if task.priority <= 2:  # Critical
        return 'in-house'
    
    # Check system alignment needs
    if task.requires_full_codebase_context:
        return 'in-house'
    
    # Otherwise, send to courier for web platform
    return 'courier'
```

## 3.2 Platform Selection (Courier Destinations)

```python
def select_platform(task):
    platforms = get_active_platforms()
    
    # Filter by capability
    capable = [p for p in platforms if task.type in p.capabilities]
    
    # Filter by limits
    available = [p for p in capable if p.usage < p.limit * 0.8]
    
    # Rank by ROI
    ranked = sorted(available, key=lambda p: p.roi_score, reverse=True)
    
    # Return best platform
    return ranked[0] if ranked else None
```

---

# 4. Execution Hands Detail

## 4.1 In-House CLI Runner

**Environment:**
- OpenCode CLI (GLM-5 subscription) — CURRENT
- Kimi CLI (subscription, expires end of month) — TEMPORARY
- GitHub Actions (ephemeral)
- Full codebase access

**When Dispatched:**
- Task has 3+ dependencies
- Task requires full codebase understanding
- Task is system-critical (orchestrator, council, tests)
- Task estimated context > 80% of limit
- Task involves architecture changes

**Lifecycle:**
1. Director dispatches task packet
2. Runner receives: prompt + tech_spec + context + dependencies
3. Execute instruction
4. Validate exit code
5. Run lint/tests
6. Create feature branch
7. Push PR
8. Return result to Supervisor

## 4.2 Courier Runner

**Environment:**
- Gmail account: `vibesagentai@gmail.com`
- Web platform access (free tiers)
- Playwright (for platform interaction if needed)

**Destinations (Web Platforms):**
- ChatGPT (OpenAI)
- Claude (Anthropic)
- Gemini (Google)
- Perplexity
- DeepSeek Web
- Grok (X)
- Other free-tier AI platforms

**When Dispatched:**
- Task is independent (no dependencies)
- Task is small/medium complexity
- Task doesn't require full codebase
- Task can be parallelized
- Task is for research/analysis/generation (not system modification)

**Lifecycle:**
1. Director creates task packet
2. Courier picks up packet
3. Courier navigates to selected platform
4. Courier submits prompt
5. Courier waits for response
6. Courier captures: result + chat URL
7. Courier returns to VibePilot
8. Supervisor reviews result

**Chat URL Purpose:**
- Revisit conversation for revisions
- Avoid full context repetition
- Cost-efficient iteration

---

# 5. Council Governance

## 5.1 Council Composition

Three independent roles, each potentially different model:

| Role | Focus | Checks |
|------|-------|--------|
| **Structural Validator** | Architecture alignment | Docker, multi-stage, non-root, tech stack |
| **Specification Precision Reviewer** | PRD alignment | Gaps, conflicts, edge cases, completeness |
| **Feasibility Analyst** | Buildability | Resources, timeline, risk, dependencies |

## 5.2 Council Trigger Events

- New Plan (before execution)
- System Update Proposal
- New Feature Request
- Architecture Change
- New Tool/Model Integration
- Error Pattern Detection (repeated failures)

## 5.3 Council Process

```
1. Supervisor calls Council
2. Each member receives:
   - System Summary (comprehensive)
   - PRD (relevant sections)
   - Plan/Proposal
   - Role-specific prompt
   
3. Each member reviews INDEPENDENTLY
   - No agent-to-agent chat (token efficiency)
   - Different models = different blind spots
   
4. Results aggregated:
   - 3 APPROVED → Proceed
   - Any REVISION REQUIRED → Return to Planner
   - Any BLOCKED → Escalate to human
```

## 5.4 Council Output Format

```json
{
  "member": "structural_validator",
  "model": "gemini-2.0-flash",
  "result": "APPROVED",
  "confidence": 0.95,
  "notes": "Architecture aligns. Docker multi-stage confirmed.",
  "concerns": []
}
```

---

# 6. Task Lifecycle & States

## 6.1 State Flow

```
pending → available → in_progress → review → testing → approval → merged
              ↑            │          │         │          │
              │            │          │         │          │
              └────────────┴──────────┴─────────┴──────────┘
                           (loops back on failure)
                           
Additional states:
- escalated (3 failures, needs attention)
- blocked (dependency not met)
```

## 6.2 State Definitions

| State | Meaning |
|-------|---------|
| `pending` | Created, dependencies not checked |
| `available` | Dependencies met, ready for pickup |
| `in_progress` | Claimed by runner |
| `review` | Output generated, supervisor review |
| `testing` | Tests running |
| `approval` | Tests passed, final approval |
| `merged` | Complete, branch merged |
| `escalated` | 3+ failures, needs human attention |
| `blocked` | Dependency failed |

## 6.3 Max Attempts: 3

After 3 failures:
- Auto-escalate to `escalated` status
- Log failure pattern
- Notify human (future: via dashboard)
- Options: Reassign, Split, Refine prompt, Switch model

---

# 7. Error Classification & Retry Policy

## 7.1 Error Types

| Code | Type | Description | Retry |
|------|------|-------------|-------|
| E001 | MODEL_ERROR | Model produced invalid output | Up to 3 |
| E002 | NETWORK_ERROR | Connection/timeout issue | Up to 3 |
| E003 | PLATFORM_ERROR | Web platform issue (rate limit, captcha) | 2, then escalate |
| E004 | LOGIC_ERROR | Fundamental flaw in approach | Escalate immediately |
| E005 | CLI_ERROR | CLI tool failure | Up to 2 |
| E006 | CONTEXT_OVERFLOW | Exceeded context window | Split task |
| E007 | DEPENDENCY_ERROR | Dependency task failed | Block until resolved |
| E008 | TEST_FAILURE | Tests did not pass | Return to in_progress |
| E009 | REVIEW_REJECTION | Supervisor rejected output | Return with notes |
| E010 | TIMEOUT | Execution > 30 minutes | Kill + classify |

## 7.2 Retry Policy

```python
RETRY_POLICY = {
    'E001': {'max': 3, 'action': 'same_or_switch_model'},
    'E002': {'max': 3, 'action': 'exponential_backoff'},
    'E003': {'max': 2, 'action': 'escalate'},
    'E004': {'max': 0, 'action': 'escalate_immediately'},
    'E005': {'max': 2, 'action': 'check_cli_health'},
    'E006': {'max': 1, 'action': 'split_task'},
    'E007': {'max': 0, 'action': 'block_until_dependency_resolved'},
    'E008': {'max': 3, 'action': 'return_to_in_progress'},
    'E009': {'max': 3, 'action': 'return_with_notes'},
    'E010': {'max': 1, 'action': 'kill_and_classify'},
}
```

---

# 8. Budget & Limit Enforcement

## 8.1 Thresholds

| Metric | Warning | Hard Limit | Action |
|--------|---------|------------|--------|
| Context window | 70% | 80% | Timeout, reassign |
| Token usage (monthly) | 70% | 80% | Throttle non-critical |
| Platform daily limit | 70% | 80% | Switch platform |
| Model request limit | 70% | 80% | Switch model |

## 8.2 Budget Tracking

```sql
-- Tracked in models/platforms table
- token_used / token_limit
- request_used / request_limit
- cycle_resets_at (monthly/daily)
```

## 8.3 Budget Exceeded Response

```
If monthly ceiling reached:
1. Pause new non-critical tasks
2. Log event
3. Alert human
4. Allow critical tasks only
```

---

# 9. ROI Calculator

## 9.1 Purpose

Track actual costs vs theoretical API costs to inform:
- Which platforms to use
- Which models are worth subscribing to
- Cost per task type
- Efficiency metrics

## 9.2 Calculation

```python
def calculate_roi(task_run):
    # Actual cost
    courier_cost = task_run.courier_time_minutes * COURIER_RATE
    platform_cost = 0  # Free tier
    
    # Theoretical API cost
    tokens = task_run.tokens_used
    api_rate = get_api_rate(task_run.model_id)
    theoretical_cost = (tokens / 1000) * api_rate
    
    # ROI
    roi = {
        'actual_cost': courier_cost,
        'theoretical_api_cost': theoretical_cost,
        'savings': theoretical_cost - courier_cost,
        'savings_percentage': (theoretical_cost - courier_cost) / theoretical_cost * 100,
        'success': task_run.success,
        'model': task_run.model_id,
        'platform': task_run.platform,
        'task_type': task_run.task_type,
        'duration_seconds': task_run.duration,
    }
    
    return roi
```

## 9.3 ROI Report (Nightly)

```sql
-- Generated nightly
SELECT 
    model_id,
    platform,
    task_type,
    COUNT(*) as total_tasks,
    AVG(CASE WHEN success THEN 1 ELSE 0 END) as success_rate,
    AVG(tokens_used) as avg_tokens,
    AVG(duration_seconds) as avg_duration,
    SUM(theoretical_api_cost) as total_theoretical_cost,
    SUM(actual_cost) as total_actual_cost,
    AVG(savings_percentage) as avg_savings_pct
FROM task_runs
WHERE completed_at > NOW() - INTERVAL '24 hours'
GROUP BY model_id, platform, task_type;
```

---

# 10. Model & Platform Registry

## 10.1 Models (In-House)

| Field | Description |
|-------|-------------|
| `id` | Model identifier (deepseek-chat, glm-5, kimi) |
| `platform` | opencode, kimi-cli |
| `courier` | Which courier delivers to this model |
| `context_limit` | Maximum context window |
| `strengths` | ['code', 'reasoning', 'planning'] |
| `weaknesses` | ['vision', 'large-context'] |
| `request_limit` | Monthly limit |
| `request_used` | Current usage |
| `token_limit` | Monthly token limit |
| `token_used` | Current token usage |
| `status` | active, benched, paused, offline |
| `status_reason` | 'out of credits', 'subscription ended' |
| `cycle_resets_at` | When limits refresh |

## 10.2 Platforms (Web Destinations)

| Field | Description |
|-------|-------------|
| `id` | Platform identifier (chatgpt-free, claude-free) |
| `type` | 'web_courier' |
| `url` | Platform URL |
| `gmail_account` | Shared Gmail for login |
| `capabilities` | ['reasoning', 'code', 'research', 'analysis'] |
| `daily_limit` | Max tasks per day |
| `daily_used` | Current day usage |
| `success_rate` | Historical success rate |
| `avg_response_time_ms` | Performance metric |
| `last_success` | Timestamp |
| `last_failure` | Timestamp |
| `status` | active, benched, offline |

---

# 11. Swappability Protocol

## 11.1 Replaceable Components

| Component | Replacement Rules |
|-----------|-------------------|
| GLM-5 | Pass Council review, maintain API compatibility |
| Kimi CLI | No execution lifecycle change |
| OpenCode | Swap for another CLI tool |
| Supabase | Migrate state cleanly |
| GitHub Actions | Maintain ephemeral execution |
| Angie/Nginx | Gateway only, no logic |
| Web Platforms | Add/remove anytime, update registry |

## 11.2 Replacement Process

```
1. Council review proposed replacement
2. Run OK-probe (test task)
3. Verify performance metrics
4. Update registry
5. Route traffic gradually
6. Monitor for issues
7. Full cutover or rollback
```

---

# 12. Human Approval Requirements

## 12.1 Always Requires Human

| Task Type | Reason |
|-----------|--------|
| UI/UX changes | Visual verification required |
| Security changes | Risk assessment |
| Architecture changes | Strategic impact |
| New model integration | Resource allocation |
| Budget overrides | Financial authority |

## 12.2 Human Escalation

| Trigger | Action |
|---------|--------|
| 3+ task failures | Escalate for review |
| Model/Platform degradation | Alert for decision |
| Budget threshold | Approval to continue |
| Council BLOCKED | Human arbitration |

---

# 13. Observability

## 13.1 Tracked Metrics

| Metric | Purpose |
|--------|---------|
| Success rate per model | Routing decisions |
| Success rate per platform | Platform selection |
| Average tokens per task | Budget forecasting |
| Average duration per task | Performance baseline |
| Retry frequency | Quality indicator |
| ROI score | Value tracking |
| Confidence rating | Plan quality |

## 13.2 Dashboard (Vibeflow)

- Real-time task status
- Model/platform health
- Budget consumption
- ROI metrics
- Active runners
- Failure queue

---

# 14. Recovery Protocol

## 14.1 Model/Platform Outage

```
1. Detect failure (consecutive failures > 3)
2. Pause dispatch to affected model/platform
3. Flag degraded state
4. Allow manual override to secondary
5. Log vendor failure event
6. Resume when restored
```

## 14.2 System Recovery

```
If primary system fails:
1. All state in Supabase (survives)
2. Runners are ephemeral (recreate)
3. No persistent compute (nothing to restore)
4. Restart Director → Resume from queue
```

---

# 15. Definition of Done

A task is **COMPLETE** only when:

- [ ] Code compiles (if applicable)
- [ ] Tests pass
- [ ] PR created
- [ ] Supervisor validation logged
- [ ] ROI metrics recorded
- [ ] Confidence score stored
- [ ] Branch merged
- [ ] Branch deleted
- [ ] Dependencies unlocked

---

# 16. Anti-Patterns (Forbidden)

| Forbidden | Reason |
|-----------|--------|
| Silent architecture changes | No drift |
| Self-modifying orchestration | Stability |
| Infinite retry loops | Token waste |
| Direct execution without PRD | Accountability |
| Manual hotfix edits | Traceability |
| Model-driven schema mutation | Control |
| Agent-to-agent chat | Token efficiency |
| Same model reviewing own work | Blind spots |

---

# 17. Implementation Phases

## Phase 0: Audit (COMPLETE)
- Environment check
- Schema setup
- Key validation

## Phase 1: Core Execution (IN PROGRESS)
- TaskManager operational
- In-house CLI runner working
- Supervisor review working
- Merge workflow working

## Phase 2: Courier System
- Platform registry
- Courier agent
- Gmail session management
- Chat URL capture

## Phase 3: Council Activation
- Three independent reviewers
- Review workflow
- Approval gates

## Phase 4: Full Production
- Multi-platform dispatch
- ROI calculator
- Full observability
- Dashboard integration

---

# 18. Success Criteria

VibePilot is successful when:

1. **Any component can be swapped** without system failure
2. **Tasks complete deterministically** with predictable outcomes
3. **ROI is tracked** on every execution
4. **Errors are classified** and handled appropriately
5. **No single vendor** can hold the system hostage
6. **Human oversight** is applied where needed
7. **System evolves** with the AI landscape

---

# 19. Appendix: Quick Reference

## 19.1 Key Commands

```bash
# Run task from queue
python task_manager.py --claim

# Check model status
python task_manager.py --models

# View available tasks
python task_manager.py --available

# Run orchestrator
python orchestrator.py
```

## 19.2 Key Files

| File | Purpose |
|------|---------|
| `task_manager.py` | Task CRUD, claiming, status |
| `orchestrator.py` | Main dispatch loop |
| `agents/` | Agent implementations |
| `docs/schema_v1_core.sql` | Database schema |
| `docs/schema_safety_patches.sql` | Safety constraints |

## 19.3 Key Tables

| Table | Purpose |
|-------|---------|
| `tasks` | Task queue + state |
| `task_packets` | Versioned prompts |
| `models` | In-house model registry |
| `task_runs` | Execution history + chat URLs |

---

**End of VibePilot v1.3 PRD**

*Changes require Council approval.*
