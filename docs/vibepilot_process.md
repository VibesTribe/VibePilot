# VibePilot Process - Complete System Flow

**Created:** 2026-02-19
**Status:** Draft - Human Review Required
**Session:** 16

---

## Overview

This document captures the complete VibePilot process: how ideas become code, who does what, and how everything connects.

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
  - Breaks into atomic tasks (95%+ confidence each)
  - Maps dependencies
  - Creates complete prompt packets
  - Plan saved to GitHub: docs/plans/project-name-plan.md
```

**What is Planner?**
- Role: Task decomposition
- Reads PRD, codebase context
- Produces: task breakdown, dependencies, prompt packets
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
Orchestrator calls available models with plan:
  - Lens 1: User Alignment (does this match what human wanted?)
  - Lens 2: Architecture (is it technically sound?)
  - Lens 3: Feasibility (can it actually be built as specified?)
    ↓
Models return votes → Orchestrator aggregates → Supervisor
    ↓
Consensus? 
  - YES → Plan approved
  - NO → Back to Planner with feedback
```

**Council hats for NEW PROJECT:**
- User Alignment: Does this match human's intent?
- Architecture: Is the design sound?
- Feasibility: Can each task actually be executed?

**Council hats for SYSTEM IMPROVEMENT:**
- Architecture: Does this fit existing system?
- Security: Any vulnerabilities introduced?
- Integration: Does it break anything?
- Reversibility: Can we undo if needed?

---

### 4. Approved Plan → Orchestrator

```
Supervisor approves plan
    ↓
Tasks created in Supabase:
  - Each task: task_id, title, prompt_packet, status='available', dependencies
    ↓
Orchestrator (always on, systemd service):
  - Polls Supabase for 'available' tasks
  - Checks dependencies met
  - Routes to appropriate runner
```

**What is Orchestrator?**
- Role: Task routing and monitoring
- Always running (systemd service)
- Watches Supabase queue
- Knows: model availability, rate limits, capabilities
- Routes tasks to best available runner
- Tracks: success rates, token usage, model performance
- Powered by: Lightweight model (Gemini Flash) or runs as code

---

### 5. Orchestrator → Runner

```
Orchestrator sees available task T001
    ↓
Checks task requirements:
  - needs_codebase? → Route to internal runner
  - no_codebase? → Route to courier
    ↓
Available runners?
  - Internal CLI (Kimi, GLM-5): Has codebase access
  - Courier: No codebase, uses web platforms
    ↓
Orchestrator assigns: "T001 → Internal CLI (Kimi)"
    ↓
Task status → 'in_progress'
```

**What is Courier Agent?**
- Role: Execute tasks on web AI platforms
- No codebase access
- Receives: task packet only
- Goes to: ChatGPT, Claude, Gemini (web), Perplexity
- Returns: output + chat_url
- Powered by: Browser automation + web platform

**What is Internal Runner?**
- Role: Execute tasks with codebase context
- Has: file_read access (not write)
- Receives: task packet + relevant codebase files
- Returns: code output
- Powered by: Kimi CLI, GLM-5, DeepSeek API

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
Orchestrator:
  - Retries periodically (checks rate limits)
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
  - metadata: model, tokens, duration
    ↓
Orchestrator:
  - Logs to task_runs table
  - Updates task status → 'review'
  - Notifies Supervisor: "T001 ready for review"
```

---

### 8. Supervisor Reviews

```
Supervisor receives: task + output + expected_output
    ↓
Checks:
  - All expected files present?
  - Output matches packet?
  - No hardcoded secrets?
  - Pattern consistency?
  - No extra files touched?
    ↓
Decision:
  - PASS → Trigger tests
  - FAIL → Return to runner with specific feedback
  - REROUTE → Different runner/model (if model seems incapable)
```

**What is Supervisor?**
- Role: Quality gatekeeper
- Reviews: output vs expected
- Decides: approve/reject/reroute
- Triggers: tests, git commands
- Commands: Maintenance (via command queue)
- Calls: Council (via Orchestrator)
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
  - FAIL → Back to runner with test failures
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
Task status → 'merged'
    ↓
Supervisor: unlocks dependent tasks
```

**When is a task done?**
- Output matches packet ✓
- Supervisor approved ✓
- Tests passed ✓
- Merged to module branch ✓
- Task branch deleted ✓
- Status = 'complete' in Supabase

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

## ROLES VS MODELS

**Core Principle:** Agent ≠ Fixed Identity

| Concept | What It Is |
|---------|------------|
| **Role** | Skills + Tools + Scope + Prompt (fixed definition) |
| **Model** | Compute power (swappable) |
| **Agent** | Role + Model (temporary pairing) |

Example:
- Supervisor role: decide, review, approve
- Today powered by: GLM-5
- Tomorrow could be powered by: Kimi, DeepSeek, whatever's available

---

## BRANCH LIFECYCLE

```
1. Supervisor assigns task → Maintenance creates task/T001

2. Runner returns code → Supervisor approves → Maintenance commits to task/T001

3. Tests pass → Maintenance merges task/T001 → module/user-auth → deletes task/T001

4. All module tasks complete + module tests pass → Maintenance merges module/user-auth → main → deletes module/user-auth → tags

Result: Clean main, no stale branches
```

---

## GITHUB - WHO DOES WHAT

| Action | Decides | Executes |
|--------|---------|----------|
| Create task branch | Supervisor | Maintenance |
| Commit code to task branch | Supervisor (approves output) | Maintenance |
| Merge task → module | Supervisor (after tests pass) | Maintenance |
| Delete task branch | Automatic (after merge verified) | Maintenance |
| Create module branch | Supervisor (first task in module passes) | Maintenance |
| Merge module → main | Supervisor (after module complete + tests) | Maintenance |
| Delete module branch | Automatic (after merge verified) | Maintenance |
| Tag release | Supervisor | Maintenance |

**Rule: Only Maintenance touches git. Everyone else commands.**

---

## COUNCIL - HOW IT'S CALLED

```
1. Supervisor receives plan → needs review

2. Supervisor → Orchestrator: "Route council review for this doc, lenses: architecture, feasibility, security"

3. Orchestrator checks available models:
   - 3 available? → Assign 1 lens each, parallel
   - 1 available? → Same model does 3 sequential passes
   - None available? → Queue, retry

4. Models review independently, return votes

5. Orchestrator aggregates → returns to Supervisor

6. Supervisor checks consensus:
   - All approve? → Plan approved
   - Any revision needed? → Back to Planner
   - Blocked? → Escalate to human
```

**No fixed Council agents. Any available model. Orchestrator routes.**

---

## AGENT DEFINITIONS

### Vibes (Interface)
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
- Role: Task decomposition
- Reads PRD + codebase
- Creates atomic tasks with prompt packets
- Needs: file_read (read-only)
- Powered by: Any capable model

### Council
- Not a fixed agent
- Function: Multi-lens review
- Lenses: User Alignment, Architecture, Feasibility, Security
- Routed by: Orchestrator
- Powered by: Any 1-3 available models

### Orchestrator
- Role: Task routing and monitoring
- Always on (systemd service)
- Routes tasks to runners
- Tracks performance
- Powered by: Lightweight model or code

### Supervisor
- Role: Quality gatekeeper
- Reviews output vs expected
- Commands Maintenance
- Triggers Council via Orchestrator
- Powered by: Any capable model

### Maintenance
- Role: File/git operator
- ONLY agent with write/git access
- Polls command queue
- Executes commands
- Powered by: Any capable model

### Runner (Internal)
- Role: Execute tasks with codebase context
- Has: file_read (not write)
- Returns: code output
- Powered by: Kimi CLI, GLM-5, DeepSeek API

### Courier
- Role: Execute tasks on web platforms
- No codebase access
- Returns: output + chat_url
- Powered by: Browser automation

### Tester
- Role: Validate code
- Runs: tests, lint, typecheck
- Returns: pass/fail
- Powered by: Any capable model

### System Researcher
- Role: Continuous improvement intelligence
- Runs: Daily (scheduled)
- Researches: new models, pricing, tools
- Output: docs/UPDATE_CONSIDERATIONS.md
- Powered by: Any model with web access

---

## STATUS TRACKING (How Orchestrator Knows)

```
Supabase: tasks table
  - id, title, status, dependencies, prompt_packet

Status values:
  - pending: Created, not yet available
  - available: Ready to be picked up
  - in_progress: Runner assigned
  - review: Output returned, awaiting Supervisor
  - testing: Tests running
  - merged: Merged to module
  - complete: All done
  - failed: Exhausted retries
  - blocked: Dependencies not met

Supabase: task_runs table
  - task_id, model_id, status, tokens, output, created_at

Orchestrator queries these tables to know:
  - What needs assignment
  - What's stuck
  - What needs reassignment
```

---

## REASSIGNMENT / SPLIT LOGIC

```
Orchestrator sees:
  - task_runs for T001: 3 failed attempts with same model
    ↓
Orchestrator decision:
  - Try different model?
  - Flag for Supervisor to consider split?
  - Mark as blocked and alert?
    ↓
If model issue: Route to different model
If task issue: Supervisor reviews, may ask Planner to split
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
│(Interface)  │                 │  (GitHub)   │
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
│             │                 │(via Orchestrator)
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
│             │                  │             │
└──────┬──────┘                  └─────────────┘
       │ commands
       ▼
┌─────────────┐     executes    ┌─────────────┐
│ MAINTENANCE │ ───────────────→│    GIT      │
│             │                 │  (branches) │
└─────────────┘                 └─────────────┘
```

---

## IMPLEMENTATION PRIORITY

1. maintenance_commands table + schema
2. Maintenance agent: polls queue, executes commands
3. Supervisor refactor: removes git, inserts commands
4. Council review queue (same pattern)
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
- [ ] Human review
- [ ] Implementation

---

**This document preserves Session 16 decisions. If session dies, resume from here.**
