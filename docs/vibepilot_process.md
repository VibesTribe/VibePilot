# VibePilot Process - Complete System Flow

**Created:** 2026-02-19
**Updated:** 2026-02-19 (Session 16 clarifications)
**Status:** Human Review Complete
**Session:** 16

---

## Overview

This document captures the complete VibePilot process: how ideas become code, who does what, and how everything connects.

---

## ALWAYS-ON SERVICES

| Service | Why Always On |
|---------|---------------|
| **Vibes** | Human interface - must respond to voice/text anytime |
| **Orchestrator** | Task routing - must watch queue, route to runners, track status |
| **System Researcher** | Scheduled daily cron for research |

Everything else is called as needed.

---

## END-TO-END FLOW

### 1. Idea → PRD

```
Human: "I want to add X to project Y"
    ↓
Consultant/Researcher agent:
  - Converses with human (via Vibes interface)
  - Researches requirements
  - Creates full PRD with zero ambiguity
  - PRD saved to GitHub: docs/prd/project-name.md
```

**What is Consultant/Researcher?**
- Role: Interactive PRD generation
- Helps human clarify vision
- Gathers all requirements
- Produces complete PRD
- Powered by: Any capable model (GLM-5, Kimi, etc.)

---

### 2. PRD → Plan

```
PRD in GitHub: docs/prd/project-name.md
    ↓
Planner reads PRD:
  - Decides modules/slices (grouping related tasks)
  - Sets priority ordering
  - Flags: needs_codebase (internal) vs no_codebase (courier)
  - Breaks into atomic tasks (95%+ confidence each)
  - Maps dependencies between tasks
  - Creates complete prompt packets for each task
  - Plan saved to GitHub: docs/plans/project-name-plan.md
```

**What is Planner?**
- Role: Plan creation and editing
- Reads PRD + codebase context
- Decides: module structure, priority, runner type flags
- Creates: task breakdown, dependencies, prompt packets
- Edits: plan when Council requests changes
- Needs: file_read for codebase context (read-only)
- Powered by: Any capable model

---

### 3. Plan → Council Review

```
Plan in GitHub: docs/plans/project-name-plan.md
    ↓
Supervisor: "This plan needs Council review"
    ↓
Supervisor → Orchestrator: "Route council review"
    ↓
Orchestrator calls available models with appropriate context
    ↓
Models return votes → Orchestrator aggregates → Supervisor
    ↓
Consensus? 
  - YES → Plan approved
  - NO → Back to Planner with feedback
```

**Council Context Depends on Review Type:**

| Review Type | Context Provided to Council |
|-------------|----------------------------|
| **New Project Plan** | PRD + Plan |
| **System Improvement** | System researcher doc (UPDATE_CONSIDERATIONS.md) + Full system understanding + VibePilot principles (core_philosophy.md) |

**Council hats for NEW PROJECT:**
- User Alignment: Does this match human's intent?
- Architecture: Is the design sound?
- Feasibility: Can each task actually be executed?

**Council hats for SYSTEM IMPROVEMENT:**
- Architecture: Does this fit existing system?
- Security: Any vulnerabilities introduced?
- Integration: Does it break anything?
- Reversibility: Can we undo if needed?
- Principle Alignment: Does it follow VibePilot principles?

---

### 4. Approved Plan → Tasks Created

```
Supervisor approves plan
    ↓
Tasks created in Supabase:
  - Each task: task_id, title, prompt_packet, status='pending', dependencies
  - Dependencies mapped from plan
    ↓
Tasks become 'available' when dependencies met
```

---

### 5. Orchestrator → Runner

```
Orchestrator (always on, systemd service):
  - Polls Supabase for 'available' tasks
  - Checks dependencies met
  - Checks runner availability
  - Routes to appropriate runner
    ↓
Task requirements checked:
  - needs_codebase? → Route to internal runner
  - no_codebase? → Route to courier
    ↓
Orchestrator assigns: "T001 → Internal CLI (Kimi)"
    ↓
Task status → 'in_progress'
```

**What is Orchestrator?**
- Role: Task routing, monitoring, and learning
- Always running (systemd service)
- Watches Supabase queue
- Knows: model availability, rate limits, capabilities, refresh times
- Shows: countdown to rate limit resets (e.g., "Gemini available in 4h 23m")
- Ensures: stays under 80% threshold when possible
- Routes tasks to best available runner
- Tracks: success rates, token usage, model performance
- Learns: which model for which task type, improves routing over time
- Powered by: Lightweight model (Gemini Flash) or runs as code

---

### 6. If No Runner Available

**Conditions:**
- All web platforms rate limited (free tier exhausted)
- Internal runners at capacity
- Platform down (ChatGPT outage)
- Daily limit hit

**What happens:**
```
Orchestrator: "No runner available for T001"
    ↓
Task status → 'queued'
    ↓
Orchestrator shows countdown:
  - "ChatGPT rate limit resets in 2h 15m"
  - "Gemini daily quota resets in 6h 00m"
    ↓
Orchestrator:
  - Retries when platforms refresh (knows exact times)
  - Routes to different platform if available
  - If critical priority: alert human
  - If can wait: queue until runner available
```

---

### 7. Runner Returns Output

```
Runner completes T001
    ↓
Returns to Orchestrator:
  - status: success/failed
  - output: code files
  - metadata: model, tokens, duration, chat_url (if courier)
    ↓
Orchestrator:
  - Logs to task_runs table
  - Tracks model/platform performance
  - Updates task status → 'review'
  - Notifies Supervisor: "T001 ready for review"
```

**NOTE: Task is NOT complete yet. Output returned ≠ task done.**

---

### 8. Supervisor Reviews

```
Supervisor receives: task + output + expected_output
    ↓
Supervisor reads task branch (has read access to git)
    ↓
Checks:
  - All expected files present?
  - Output matches packet?
  - No hardcoded secrets?
  - Pattern consistency?
  - No extra files touched?
  - Quality acceptable?
    ↓
Decision:
  - PASS → Trigger tests
  - FAIL → Return to runner with specific feedback (tracked as attempt)
  - REROUTE → Different runner/model (if model seems incapable)
```

**What is Supervisor?**
- Role: Quality gatekeeper
- Has: git read access (to review branches)
- Does NOT have: git write access
- Reviews: output vs expected
- Decides: approve/reject/reroute
- Triggers: tests
- Commands: Maintenance via command queue
- Calls: Council via Orchestrator
- Powered by: Any capable model

---

### 9. Tests Run

```
Supervisor approves output
    ↓
Triggers Code Tester:
  - Receives: task branch code + test criteria
  - Runs: unit tests, lint, typecheck
  - Returns: pass/fail + details
    ↓
Result:
  - PASS → Ready for merge
  - FAIL → Back to runner with test failures (tracked as attempt)
```

**What is Tester?**
- Role: Validate code
- Sees: code + test criteria only
- Runs: pytest, lint, typecheck
- Returns: pass/fail
- Powered by: Any capable model (OpenCode, Kimi)

---

### 10. Merge to Module

```
Tests pass
    ↓
Supervisor → Maintenance: "Merge task/T001 → module/user-auth"
    ↓
Maintenance:
  - Creates module branch if not exists
  - Merges task → module
  - Verifies merge succeeded
  - Deletes task branch
  - Reports: success + commit hash
    ↓
Task status → 'complete' (NOW task is done)
    ↓
Supervisor: unlocks dependent tasks
```

**When is a task SUCCESS?**
- Output matches packet ✓
- Supervisor approved ✓
- Tests passed ✓
- Merged to module branch ✓
- Task branch deleted ✓
- Status = 'complete' in Supabase

**When is a task FAILURE?**
- Runner failed and exhausted retries
- Supervisor rejected and no reroute possible
- Tests failed repeatedly
- Tracked with: failure reason, which model, which stage failed

---

### 11. Module Complete → Main

```
All tasks in module complete
    ↓
Supervisor triggers module tests:
  - Integration tests across all module tasks
  - Module-level validation
    ↓
Module tests pass
    ↓
Supervisor → Maintenance: "Merge module/user-auth → main, tag, delete"
    ↓
Maintenance:
  - Merges to main
  - Tags: module-user-auth-v1
  - Deletes module branch
  - Reports: success
```

---

## BRANCH LIFECYCLE - DETAILED

```
┌─────────────────────────────────────────────────────────────────┐
│                     BRANCH LIFECYCLE                             │
└─────────────────────────────────────────────────────────────────┘

1. TASK ASSIGNED
   Orchestrator assigns task → Supervisor notified
   Supervisor → Maintenance: "Create task/T001-desc"
   Maintenance: creates branch, reports success
   
2. CODE COMMITTED
   Runner returns code → Supervisor reviews (reads branch)
   Supervisor approves → Maintenance: "Commit to task/T001"
   Maintenance: commits code, reports success
   
3. TASK → MODULE (after tests pass)
   Supervisor → Maintenance: "Merge task/T001 → module/user-auth, delete task branch"
   Maintenance: 
     - Creates module branch if needed
     - Merges task → module
     - Deletes task branch
     - Reports success
   
4. MODULE → MAIN (all tasks complete + module tests pass)
   Supervisor → Maintenance: "Merge module/user-auth → main, tag, delete"
   Maintenance:
     - Merges to main
     - Tags: module-name-v1
     - Deletes module branch
     - Reports success

RESULT: Clean main, all work tagged, no stale branches
```

---

## GITHUB - WHO DOES WHAT

| Action | Who Decides | Who Executes | Notes |
|--------|-------------|--------------|-------|
| Create task branch | Supervisor | Maintenance | After Orchestrator assigns |
| Read task branch | Supervisor | - | For quality review |
| Commit code | Supervisor (approves) | Maintenance | After review passes |
| Merge task → module | Supervisor | Maintenance | After tests pass |
| Delete task branch | Automatic | Maintenance | After merge verified |
| Create module branch | Supervisor | Maintenance | First task in module passes |
| Merge module → main | Supervisor | Maintenance | After module complete + tests |
| Delete module branch | Automatic | Maintenance | After merge verified |
| Tag release | Supervisor | Maintenance | After merge to main |

**Rule: Only Maintenance has git write access. Supervisor has read access for review.**

---

## REASSIGNMENT - TWO SOURCES

### Source 1: Orchestrator (Automatic)

```
Orchestrator sees:
  - task_runs for T001: 3 failed attempts with same model
    ↓
Orchestrator decision:
  - Try different model automatically
  - Track: which models fail on which task types
  - Learn: avoid this model for this task type
```

### Source 2: Supervisor (Quality Decision)

```
Supervisor reviews output:
  - Output looks wrong for this model's capability
  - Task seems too complex for this runner type
    ↓
Supervisor decision:
  - REROUTE to different model
  - Request SPLIT if task too complex
  - Mark FAILED with detailed notes if unfixable
```

---

## COUNCIL - HOW IT'S CALLED

```
1. Supervisor determines review needed

2. Supervisor → Orchestrator: "Route council review for this doc, lenses: X, Y, Z"

3. Orchestrator checks available models:
   - 3 available? → Assign 1 lens each, parallel execution
   - 1 available? → Same model does 3 sequential passes (independent votes)
   - None available? → Queue until model available

4. Orchestrator provides context based on review type:
   - Project review: PRD + Plan
   - System review: UPDATE_CONSIDERATIONS.md + system context + principles

5. Models review independently, return votes

6. Orchestrator aggregates → returns to Supervisor

7. Supervisor checks consensus:
   - All approve? → Plan approved
   - Any revision needed? → Back to Planner with feedback
   - Blocked? → Escalate to human
```

**No fixed Council agents. Any available model. Orchestrator routes.**

---

## AGENT DEFINITIONS

### Vibes (Interface) - ALWAYS ON
- Role: Human's primary interface
- Receives: Voice, text, dashboard input
- Routes to appropriate agent
- Returns: Results to human
- Powered by: Any conversational model

### Consultant/Researcher
- Role: Interactive PRD generation
- Converses with human
- Researches requirements
- Produces complete PRD
- Powered by: Any capable model

### Planner
- Role: Plan creation and editing
- Decides: modules/slices, priority, runner type flags
- Creates: tasks, dependencies, prompt packets
- Edits: plan when Council requests changes
- Needs: file_read (read-only)
- Powered by: Any capable model

### Council
- Not a fixed agent - a function
- Function: Multi-lens review
- Lenses: User Alignment, Architecture, Feasibility, Security
- Context: Depends on review type (project vs system)
- Routed by: Orchestrator
- Powered by: Any 1-3 available models

### Orchestrator - ALWAYS ON
- Role: Task routing, monitoring, learning
- Always running (systemd service)
- Routes tasks to runners
- Knows: rate limits, refresh times, countdown
- Ensures: 80% threshold
- Tracks: success/failure at every level
- Learns: improves routing over time
- Powered by: Lightweight model or code

### Supervisor
- Role: Quality gatekeeper
- Has: git read access (for review)
- Does NOT have: git write access
- Reviews: output vs expected
- Decides: approve/reject/reroute
- Commands: Maintenance via queue
- Calls: Council via Orchestrator
- Powered by: Any capable model

### Maintenance
- Role: Git/file operator (executes commands)
- ONLY agent with: git write access, file write access
- Polls: command queue
- Executes: create branch, commit, merge, delete, tag
- Reports: success/failure of each command
- Powered by: Any capable model

### Runner (Internal)
- Role: Execute tasks with codebase context
- Has: file_read (not write)
- Receives: task packet + relevant codebase files
- Returns: code output
- Powered by: Kimi CLI, GLM-5, DeepSeek API

### Courier
- Role: Execute tasks on web platforms
- No codebase access
- Returns: output + chat_url
- Powered by: Browser automation

### Tester
- Role: Validate code
- Sees: code + test criteria only
- Runs: tests, lint, typecheck
- Returns: pass/fail
- Powered by: Any capable model

### System Researcher - SCHEDULED (daily cron)
- Role: Continuous improvement intelligence
- Has: Full system understanding
- Knows: what models/platforms we use, what matters
- Researches: new models, pricing, tools, improvements
- Output: docs/UPDATE_CONSIDERATIONS.md
- Powered by: Any model with web access

---

## STATUS TRACKING (How Orchestrator Knows)

```
Supabase: tasks table
  - id, title, status, dependencies, prompt_packet

Status values:
  - pending: Created, dependencies not met
  - available: Ready to be picked up, dependencies met
  - in_progress: Runner assigned
  - review: Output returned, awaiting Supervisor
  - testing: Tests running
  - complete: Approved, tested, merged to module (SUCCESS)
  - failed: Exhausted retries or unfixable (FAILURE with notes)
  - queued: No runner available (shows countdown)
  - blocked: Dependencies stuck

Supabase: task_runs table
  - task_id, model_id, status, tokens, output, failure_reason, created_at

Orchestrator queries to know:
  - What needs assignment
  - What's stuck
  - What needs reassignment
  - Model success rates by task type
  - Platform reliability
```

---

## SYSTEM DIAGRAM

```
┌─────────────┐
│   HUMAN     │
│  (Voice/Dash)│
└──────┬──────┘
       │ talks to
       ▼
┌─────────────┐     creates     ┌─────────────┐
│   VIBES     │ ───────────────→│    PRD      │
│ (ALWAYS ON) │                 │  (GitHub)   │
└─────────────┘                 └──────┬──────┘
                                       │ read by
                                       ▼
┌─────────────┐     creates     ┌─────────────┐
│   PLANNER   │ ───────────────→│    PLAN     │
│             │                 │  (GitHub)   │
└─────────────┘                 └──────┬──────┘
                                       │ reviewed by
                                       ▼
┌─────────────┐     routes      ┌─────────────┐
│ORCHESTRATOR │ ◄────────────── │  COUNCIL    │
│ (ALWAYS ON) │                 │(via Orchestrator)
└──────┬──────┘                 └─────────────┘
       │ assigns
       ▼
┌─────────────┐     returns     ┌─────────────┐
│   RUNNER    │ ───────────────→│   OUTPUT    │
│(Courier/Internal)│            │             │
└─────────────┘                 └──────┬──────┘
                                       │ reviewed by
                                       ▼
┌─────────────┐     approves     ┌─────────────┐
│ SUPERVISOR  │ ◄─────────────── │   TESTS     │
│  (read git) │                  │             │
└──────┬──────┘                  └─────────────┘
       │ commands (queue)
       ▼
┌─────────────┐     executes    ┌─────────────┐
│ MAINTENANCE │ ───────────────→│    GIT      │
│ (write git) │                 │  (branches) │
└─────────────┘                 └─────────────┘
```

---

## IMPLEMENTATION PRIORITY

1. maintenance_commands table + schema
2. Maintenance agent: polls queue, executes commands
3. Supervisor refactor: removes git write, keeps read, inserts commands
4. Council review function via Orchestrator
5. Runner contract enforcement
6. Branch lifecycle code
7. Full task flow end-to-end
8. Integration test

---

## DOCUMENT STATUS

- [x] Flow documented
- [x] Roles defined
- [x] Branch lifecycle captured
- [x] Council process defined
- [x] Council context clarified (project vs system)
- [x] Task complete/failure definitions clarified
- [x] Orchestrator rate limit countdown added
- [x] Planner scope expanded
- [x] Reassignment sources documented
- [x] Always-on services identified
- [x] Human review complete
- [ ] Implementation

---

**This document preserves Session 16 decisions. If session dies, resume from here.**
