# VibePilot Master Plan

**Version:** 1.0  
**Status:** Source of Truth  
**Purpose:** Zero-ambiguity specification for all agents  
**Rule:** If it's not in this document, it doesn't exist. If it's ambiguous, fix the document.

---

# 1. System Identity

## 1.1 What VibePilot Is

VibePilot is a **sovereign AI execution engine** that transforms ideas into production code through coordinated multi-agent execution.

**Core Principle:** Human provides idea → VibePilot executes with zero drift from specification.

## 1.2 What VibePilot Is NOT

| NOT This | Why |
|----------|-----|
| Chatbot | No conversation, just execution |
| General AI assistant | No open-ended tasks |
| Model-dependent | All models swappable |
| Platform-dependent | All platforms swappable |

## 1.3 Architecture Philosophy

```
┌─────────────────────────────────────────────────────────────┐
│                    HUMAN (User)                              │
│                                                              │
│  Provides: Ideas, Approvals, UI/UX decisions                 │
│  Receives: Results, Escalations, ROI reports                 │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                    VIBEPILOT SYSTEM                          │
│                                                              │
│  ┌─────────────────────────────────────────────────────┐    │
│  │              CONTEXT ISOLATION LAYER                 │    │
│  │                                                       │    │
│  │  Controls what each agent can see/know               │    │
│  │  Prevents drift by limiting context scope             │    │
│  └─────────────────────────────────────────────────────┘    │
│                            │                                 │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐    │
│  │ PLANNER  │  │ COUNCIL  │  │SUPERVISOR│  │RESEARCHER│    │
│  │          │  │          │  │          │  │          │    │
│  │ See: PRD │  │See: ALL  │  │See: Plan │  │See: Sys  │    │
│  │ See: Sys │  │          │  │     Task │  │Overview  │    │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘    │
│                            │                                 │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐    │
│  │   TASK   │  │MAINTEN-  │  │  TESTER  │  │ COURIER  │    │
│  │  AGENT   │  │  ANCE    │  │          │  │          │    │
│  │          │  │          │  │          │  │          │    │
│  │See: ONLY │  │See: ALL  │  │See: ONLY │  │See: ONLY │    │
│  │   TASK   │  │(sandbox) │  │   CODE   │  │  PACKET  │    │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘    │
│                                                              │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                    STATE (Supabase)                          │
│                    CODE (GitHub)                             │
│                                                              │
│  These are the ONLY sources of truth                         │
└─────────────────────────────────────────────────────────────┘
```

---

# 2. Agent Context Levels

## 2.1 Context Isolation Matrix

| Agent | Sees PRD | Sees Plan | Sees System | Sees Other Tasks | Sees Code | Sees All |
|-------|----------|-----------|-------------|------------------|-----------|----------|
| **Planner** | ✅ Full | N/A | ✅ Overview | ❌ | ❌ | ❌ |
| **Council** | ✅ Full | ✅ Full | ✅ Full | ✅ Relevant | ❌ | ✅ Review |
| **Supervisor** | ❌ | ✅ Full | ❌ | ✅ Dependencies | ✅ Task | ❌ |
| **Task Agent** | ❌ | ❌ | ❌ | ❌ | ✅ Task only | ❌ |
| **Maintenance** | ✅ Full | ✅ Full | ✅ Full | ✅ All | ✅ All | ✅ Sandbox |
| **Researcher** | ✅ Overview | ❌ | ✅ Overview | ❌ | ❌ | ❌ |
| **Tester** | ❌ | ❌ | ❌ | ❌ | ✅ Code only | ❌ |
| **Courier** | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ (packet only) |

## 2.2 Why Context Isolation Matters

| Problem | Cause | Solution |
|---------|-------|----------|
| Drift | Agent adds "helpful" features | Only show what's needed |
| Hallucination | Agent invents context | Limit context scope |
| Conflicts | Agents have different assumptions | Single source of truth |
| Scope creep | Agent sees too much | Minimal context |

## 2.3 Context Injection Protocol

When an agent starts a task, the system injects ONLY the required context:

```python
def inject_context(agent_role, task_id):
    """
    Returns ONLY the context that agent is allowed to see.
    Any attempt to access more = protocol violation.
    """
    context = {}
    
    if agent_role == 'planner':
        context['prd'] = get_prd()
        context['system_overview'] = get_system_overview()
        context['existing_code_summary'] = get_code_summary()  # High-level only
        
    elif agent_role == 'council_member':
        context['prd'] = get_prd()
        context['plan'] = get_plan(task_id)
        context['system_state'] = get_full_system_state()
        context['related_tasks'] = get_related_tasks(task_id)
        
    elif agent_role == 'supervisor':
        context['plan'] = get_plan_for_task(task_id)
        context['task'] = get_task(task_id)
        context['dependencies'] = get_completed_dependencies(task_id)
        context['output'] = get_task_output(task_id)
        
    elif agent_role == 'task_agent':
        context['task'] = get_task(task_id)  # ONLY the task
        context['prompt'] = get_task_prompt(task_id)
        context['expected_output'] = get_expected_output_spec(task_id)
        # NOTHING else
        
    elif agent_role == 'maintenance':
        context['prd'] = get_prd()
        context['system_state'] = get_full_system_state()
        context['change_request'] = get_change_request(task_id)
        context['all_code'] = get_all_code()  # Full access
        context['sandbox_mode'] = True  # Must test before live
        
    elif agent_role == 'researcher':
        context['system_overview'] = get_system_overview()
        context['research_brief'] = get_research_brief(task_id)
        
    elif agent_role == 'tester':
        context['code'] = get_code_to_test(task_id)  # ONLY code
        context['test_criteria'] = get_test_criteria(task_id)
        
    elif agent_role == 'courier':
        context['packet'] = get_task_packet(task_id)  # ONLY packet
        context['destination'] = get_platform_destination(task_id)
    
    return context
```

---

# 3. Plan Creation Protocol

## 3.1 PRD Requirements (Before Planning)

A PRD is **not ready for planning** until it passes all checks:

| Check | Requirement | Validation |
|-------|-------------|------------|
| Completeness | All features specified | No "TBD" or "figure out later" |
| Clarity | No ambiguous terms | Every term defined |
| Testability | All features testable | Acceptance criteria defined |
| Feasibility | All features buildable | Dependencies identified |
| Atomicity | Features can be sliced vertically | Each slice = complete feature |
| No Assumptions | Everything explicit | No "as usual" or "standard" |

## 3.2 Vertical Slicing Principle

**Definition:** A vertical slice is a task that delivers a complete, testable feature from UI to database.

**Example - BAD (Horizontal):**
```
Task 1: Create database schema
Task 2: Create API endpoints
Task 3: Create UI components
Task 4: Write tests
```
❌ Problem: Task 1-3 are useless alone, Task 4 can't test incomplete features

**Example - GOOD (Vertical):**
```
Task 1: User can view their profile (schema + API + UI + test)
Task 2: User can edit their profile (API + UI + test, extends Task 1)
Task 3: User can upload avatar (storage + API + UI + test, extends Task 1)
```
✅ Each task is independently complete and testable

## 3.3 Task Specification Format

Every task in a plan MUST include:

```yaml
task_id: T001
title: "User can view their profile"
confidence: 0.98  # Must be >= 0.95

# What this task does (for Planner/Supervisor)
description: |
  Implement profile viewing functionality.
  User navigates to /profile, sees their data.
  Data comes from users table.

# Exact prompt for Task Agent (zero ambiguity)
prompt: |
  Create profile view feature.
  
  FILES TO CREATE:
  - src/pages/ProfilePage.tsx
  - src/api/profile.ts
  - src/tests/ProfilePage.test.ts
  
  DATABASE:
  - Table: users (already exists)
  - Columns: id, email, name, avatar_url, created_at
  
  API ENDPOINTS:
  - GET /api/profile
    - Auth required: yes (session token)
    - Response: { id, email, name, avatar_url, created_at }
    - Error: 401 if not authenticated
  
  UI REQUIREMENTS:
  - Route: /profile
  - Components: ProfileCard (displays user info)
  - States: loading, error, success
  - No edit functionality (separate task)
  
  TESTS REQUIRED:
  - API returns user data when authenticated
  - API returns 401 when not authenticated
  - UI displays user data correctly
  - UI shows error state on API failure
  - UI shows loading state while fetching
  
  DO NOT:
  - Add edit functionality (separate task)
  - Add settings (separate task)
  - Add any fields not listed above
  - Add animations or transitions
  - Add features "just in case"

# Exact expected output (for Tester)
expected_output:
  files_created:
    - src/pages/ProfilePage.tsx
    - src/api/profile.ts
    - src/tests/ProfilePage.test.ts
  files_modified: []
  api_endpoints:
    - method: GET
      path: /api/profile
      auth: required
      response_shape: { id: string, email: string, name: string, avatar_url: string|null, created_at: string }
  ui_routes:
    - path: /profile
      components: [ProfileCard]
  tests_required:
    - "API returns user data when authenticated"
    - "API returns 401 when not authenticated"
    - "UI displays user data correctly"
    - "UI shows error state on API failure"
    - "UI shows loading state while fetching"

# Dependencies (for Supervisor)
dependencies: []
  # No dependencies - this is a base feature

# Integration points (for Maintenance)
integration_points:
  - auth_middleware: "Must check session token"
  - users_table: "Read-only access"
  - router: "Add /profile route"

# Rollback plan (if task fails)
rollback:
  - delete src/pages/ProfilePage.tsx
  - delete src/api/profile.ts
  - delete src/tests/ProfilePage.test.ts
  - no database changes (read-only)
```

## 3.4 Prompt Engineering Rules

Every task prompt follows these rules to prevent drift:

| Rule | Example |
|------|---------|
| Explicit file names | "Create src/pages/ProfilePage.tsx" not "create profile page" |
| Explicit data shapes | `{ id: string, email: string }` not "user data" |
| Explicit API contracts | "GET /api/profile returns 401 if not authenticated" |
| Explicit DO NOT list | "DO NOT add edit functionality" |
| No open-ended instructions | "Add loading state" not "make it look nice" |
| Test criteria explicit | "UI shows loading state while fetching" not "test it works" |
| No "as appropriate" | Everything specified exactly |

## 3.5 Confidence Threshold

| Confidence | Action |
|------------|--------|
| >= 0.95 | Task is atomic enough |
| < 0.95 | Split task into smaller tasks |
| < 0.85 | PRD may be ambiguous - return to Planner |

---

# 4. Council Review Protocol

## 4.1 When Council Reviews

| Trigger | Council Required |
|---------|------------------|
| New plan (all tasks) | Yes, before any execution |
| Architecture change | Yes |
| System update | Yes |
| New model integration | Yes |
| New platform integration | Yes |
| Maintenance task | Yes |
| Single feature task | No (Planner already reviewed) |

## 4.2 Council Composition

Three independent reviewers, each with different focus:

| Role | Focus | Key Questions |
|------|-------|---------------|
| **Structural Validator** | Architecture | Does this fit the system architecture? Any conflicts? |
| **Specification Precision Reviewer** | PRD Alignment | Does this match the PRD exactly? Any gaps or additions? |
| **Feasibility Analyst** | Buildability | Can this be built as specified? Any risks? |

## 4.3 Council Review Process

```
1. Each member receives:
   - Full PRD
   - Complete Plan (all tasks)
   - System State (current state)
   - Their role-specific prompt
   
2. Each member reviews INDEPENDENTLY:
   - No communication between members
   - No "I think X means Y" discussions
   - If ambiguous, flag as BLOCKED
   
3. Results aggregated:
   - 3 APPROVED → Plan proceeds
   - 2 APPROVED, 1 REVISION → Fix issues, re-review
   - Any BLOCKED → Escalate to human
   
4. No consensus after 2 rounds → Human arbitration
```

## 4.4 Council Vote Format

```json
{
  "member": "structural_validator",
  "model": "gemini-2.0-flash",
  "vote": "APPROVED",
  "confidence": 0.95,
  "checks": {
    "architecture_alignment": true,
    "no_conflicts": true,
    "follows_patterns": true,
    "integration_safe": true
  },
  "concerns": [],
  "notes": "Architecture aligns. No conflicts detected."
}
```

## 4.5 Blocking Conditions

Council member MUST vote BLOCKED if:

| Condition | Why Block |
|-----------|-----------|
| Ambiguity in any task | Agent could interpret multiple ways |
| PRD mismatch | Task does something not in PRD |
| Missing test criteria | Can't verify completion |
| Dependency unclear | Order of execution ambiguous |
| Integration conflict | Could break existing system |
| Implicit assumptions | "As usual" or "standard" found |

---

# 5. Supervisor Protocol

## 5.1 Supervisor Role

Supervisor validates that task output matches the plan - no more, no less.

## 5.2 Supervisor Checklist

Before marking task as READY_FOR_TESTING:

```
☐ Output matches expected_output spec exactly
☐ All files created/modified as specified
☐ All API endpoints match specification
☐ All UI components match specification
☐ No extra files created
☐ No extra functionality added
☐ No "nice to have" additions
☐ Code follows style guide
☐ No hardcoded values (use config)
☐ No secrets in code
☐ Error handling matches spec
```

## 5.3 Supervisor Rejection

If output doesn't match spec:

```json
{
  "status": "REJECTED",
  "reason": "Output doesn't match specification",
  "issues": [
    {
      "expected": "GET /api/profile returns 401",
      "actual": "GET /api/profile returns 403",
      "severity": "must_fix"
    },
    {
      "expected": "ProfileCard component only",
      "actual": "ProfileCard + ProfileEdit components created",
      "severity": "must_fix",
      "note": "Edit functionality not requested in this task"
    }
  ],
  "action": "return_to_task_agent",
  "notes": "Fix issues and resubmit. Do not add features."
}
```

---

# 6. Maintenance Protocol

## 6.1 What Maintenance Handles

| Change Type | Examples |
|-------------|----------|
| Add model | New model to registry, config update |
| Remove model | Deprecate model, update routing |
| Swap model | Change default model for role |
| Add platform | New web AI platform to registry |
| Remove platform | Deprecate platform |
| Update config | Change thresholds, prompts, tools |
| Update role | Add/remove skills, change tools |
| System update | Architecture changes, new components |

## 6.2 Maintenance Process

```
1. Human creates maintenance request
2. Maintenance Agent creates change plan
3. Council reviews change plan
4. Council approves
5. Maintenance Agent implements IN SANDBOX
6. Full system test in sandbox
7. If tests pass → Apply to live
8. If tests fail → Debug, fix, re-test
9. Update documentation
10. Log change in DECISION_LOG.md
```

## 6.3 Sandbox Testing Requirements

Before any maintenance change goes live:

```
☐ All existing tests pass
☐ New functionality tests pass
☐ No regressions detected
☐ Integration tests pass
☐ Performance benchmarks met
☐ No security vulnerabilities introduced
☐ Documentation updated
☐ Config changes validated
☐ Rollback plan tested
```

## 6.4 Live Maintenance Checklist

```
☐ Sandbox tests all passed
☐ Human approval received
☐ Backup of current state created
☐ Change applied
☐ Smoke tests run
☐ Monitoring confirmed healthy
☐ Documentation updated
☐ DECISION_LOG.md entry created
```

---

# 7. Researcher Protocol

## 7.1 Researcher Role

Finds improvements, new models, new platforms, new approaches.

## 7.2 What Researcher Sees

| Sees | Why |
|------|-----|
| System overview | Understand what exists |
| Current models | Know what's being used |
| Current platforms | Know what's available |
| Cost/performance data | Identify optimization opportunities |
| PRD overview | Understand system goals |

## 7.3 What Researcher Does NOT See

| Doesn't See | Why |
|-------------|-----|
| Specific task details | Not relevant |
| Code implementation | Not relevant |
| User data | Privacy |
| Secrets | Security |

## 7.4 Research Output Format

```json
{
  "research_type": "new_model_evaluation",
  "subject": "Claude 3.5 Sonnet",
  "summary": "New model released with improved coding capabilities",
  "relevance": "Could replace GLM-5 for complex coding tasks",
  "pros": [
    "Better at complex reasoning",
    "Larger context window (200k)",
    "Better at following instructions"
  ],
  "cons": [
    "More expensive ($3/1M input)",
    "Rate limits on API",
    "No CLI option (API only)"
  ],
  "recommendation": "Add to model registry as option for complex tasks",
  "implementation_notes": "Would need API runner, no CLI support",
  "confidence": 0.85
}
```

---

# 8. Tester Protocol

## 8.1 Tester Role

Validates code against test criteria - nothing else.

## 8.2 What Tester Sees

| Sees | Why |
|------|-----|
| Code to test | Must run tests |
| Test criteria | Must know what to test |
| Expected behavior | Must know correct output |

## 8.3 What Tester Does NOT See

| Doesn't See | Why |
|-------------|-----|
| PRD | Could bias testing |
| Plan | Could bias testing |
| Other tasks | Irrelevant |
| Task purpose | Test behavior, not intent |

## 8.4 Test Report Format

```json
{
  "task_id": "T001",
  "tests_run": 5,
  "tests_passed": 4,
  "tests_failed": 1,
  "results": [
    {
      "test": "API returns user data when authenticated",
      "status": "PASSED",
      "duration_ms": 45
    },
    {
      "test": "API returns 401 when not authenticated",
      "status": "FAILED",
      "expected": "401 Unauthorized",
      "actual": "403 Forbidden",
      "severity": "critical"
    },
    {
      "test": "UI displays user data correctly",
      "status": "PASSED",
      "duration_ms": 120
    },
    {
      "test": "UI shows error state on API failure",
      "status": "PASSED",
      "duration_ms": 85
    },
    {
      "test": "UI shows loading state while fetching",
      "status": "PASSED",
      "duration_ms": 50
    }
  ],
  "verdict": "FAILED",
  "blocking_issues": 1,
  "recommendation": "Return to task agent for fix"
}
```

---

# 9. Swappability Matrix

## 9.1 What Can Be Swapped

| Component | Swap Method | Requires Council | Downtime |
|-----------|-------------|------------------|----------|
| Default model for role | Edit config | No | Zero |
| Add new model | Add to registry + config | Yes (new integration) | Zero |
| Remove model | Set status to 'paused' in config | No | Zero |
| Add new platform | Add to registry + config | Yes (new integration) | Zero |
| Remove platform | Set status to 'offline' in config | No | Zero |
| Role skills | Edit config | No (if reducing) | Zero |
| Role tools | Edit config | No | Zero |
| Prompts | Edit config | No | Zero |
| Thresholds | Edit config | No | Zero |
| Orchestrator model | Edit config | No | Zero |
| Supervisor model | Edit config | No | Zero |

## 9.2 Swap Procedure (No Council Required)

```
1. Edit config/vibepilot.yaml
2. Save file
3. Config hot-reloads (no restart needed)
4. Verify change in next task execution
5. Log change in DECISION_LOG.md
```

## 9.3 Swap Procedure (Council Required)

```
1. Create maintenance request
2. Maintenance Agent creates plan
3. Council reviews
4. Implement in sandbox
5. Test in sandbox
6. Apply to live
7. Log in DECISION_LOG.md
```

## 9.4 Config Structure for Swappability

```yaml
# All swaps happen in this file
# No code changes needed for any swap

models:
  glm-5:
    status: active  # Change to: paused, offline, benched
    role: primary_executor  # Change role assignment
    
roles:
  supervisor:
    default_model: glm-5  # Swap: change to kimi-k2.5, deepseek-chat
    skills: [review, validate, approve]  # Swap: add/remove skills
    tools: [file_read, supabase_query]  # Swap: add/remove tools
    
prompts:
  supervisor: |
    # Swap: edit prompt text
    You are the Supervisor...
    
thresholds:
  max_task_attempts: 3  # Swap: change value
  context_max_pct: 80  # Swap: change value
```

---

# 10. Source of Truth

## 10.1 What Is Source of Truth

| Source | Contains | Who Updates |
|--------|----------|-------------|
| **Supabase** | All runtime state | System (automatically) |
| **GitHub** | All code + documentation | Maintenance Agent (with approval) |
| **config/vibepilot.yaml** | All configuration | Maintenance Agent (with approval) |
| **docs/MASTER_PLAN.md** | System specification | Human + Council |
| **docs/DECISION_LOG.md** | All decisions | Agents (automatically) |

## 10.2 What Is NOT Source of Truth

| NOT Source | Why |
|------------|-----|
| Agent memory | Ephemeral, lost on restart |
| Local files | Not in Supabase or GitHub |
| Chat history | Not persistent |
| Model context | Lost when session ends |

## 10.3 Conflict Resolution

If sources conflict:

```
1. Supabase = State truth
2. GitHub = Code truth
3. MASTER_PLAN.md = Specification truth
4. config/vibepilot.yaml = Configuration truth

Priority: Specification > Configuration > Code > State
```

---

# 11. Anti-Patterns (Forbidden)

| Forbidden | Why | Prevention |
|-----------|-----|------------|
| Ambiguous PRD | Agent fills gaps | PRD checklist before planning |
| Missing DO NOT list | Agent adds features | Required in every task prompt |
| Implicit assumptions | Drift from intent | Explicit specification only |
| Agent-to-agent chat | Token waste, confusion | Context isolation |
| Same model reviewing own work | Blind spots | Different model for each Council member |
| Live maintenance without sandbox | System breakage | Mandatory sandbox testing |
| Task agent sees full system | Drift, hallucination | Context isolation enforcement |
| "As usual" or "standard" | Ambiguity | Explicit specification only |
| Missing test criteria | Can't verify completion | Required in every task |
| Horizontal slicing | Incomplete features | Vertical slicing enforced |

---

# 12. Decision Log Integration

All decisions that affect this document must be logged:

```markdown
## DEC-XXX: [Decision Title]

**Date:** YYYY-MM-DD
**Status:** Accepted
**Context:** Why this decision was needed

### Decision
[What was decided]

### Impact on MASTER_PLAN.md
- Section X.Y: [what changed]
- Rationale: [why]

### Review Notes
- Council approved: [date]
- Human approved: [date]
```

---

# 13. Recovery Protocol

## 13.1 If System Drifts

```
1. Stop all task execution
2. Identify drift source (which agent, which task)
3. Review task prompt for ambiguity
4. Update MASTER_PLAN.md with clarification
5. Re-run task with updated context
6. Council review if significant drift
```

## 13.2 If Agent Hallucinates

```
1. Stop task execution
2. Identify hallucination source
3. Check context isolation (did agent see too much?)
4. Reduce context scope if needed
5. Update prompt for clarity
6. Re-run task
```

## 13.3 If Maintenance Fails

```
1. Rollback to last known good state
2. Analyze failure in sandbox
3. Fix issue
4. Re-test in sandbox
5. Apply fix to live
6. Update MASTER_PLAN.md with lessons learned
```

---

# 14. Glossary

| Term | Definition |
|------|------------|
| **Atomic task** | Smallest unit of work that delivers complete, testable feature |
| **Context isolation** | Limiting what an agent can see to prevent drift |
| **Council** | Three independent reviewers with different focuses |
| **Drift** | Agent output diverging from specification |
| **Hallucination** | Agent inventing information not in context |
| **Horizontal slice** | Task that covers one layer (database OR API OR UI) - BAD |
| **Maintenance** | System changes (models, platforms, config, architecture) |
| **Sandbox** | Isolated testing environment that mirrors production |
| **Source of truth** | Authoritative location for a type of data |
| **Supervisor** | Validates task output matches specification |
| **Swappability** | Ability to change components without code changes |
| **Vertical slice** | Task that covers all layers for one feature - GOOD |

---

# 15. Document Maintenance

## 15.1 When to Update This Document

| Trigger | Update Required |
|---------|-----------------|
| New architecture decision | Yes - add to relevant section |
| New agent role | Yes - add context level |
| Process change | Yes - update protocol |
| Anti-pattern discovered | Yes - add to forbidden list |
| Glossary term needed | Yes - add definition |

## 15.2 Update Process

```
1. Identify need for update
2. Draft changes
3. Council review (if architectural)
4. Human approval
5. Update document
6. Commit to GitHub
7. Log in DECISION_LOG.md
```

---

**End of VibePilot Master Plan v1.0**

*This document is the specification. Everything else implements this.*
*If anything conflicts with this document, this document wins.*
*If anything is ambiguous, clarify in this document before proceeding.*
