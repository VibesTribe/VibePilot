# VibePilot Process - Complete System Flow

**Created:** 2026-02-19
**Updated:** 2026-02-28 (Session 36 - full clarification)
**Status:** Current
**Session:** 36

---

## Overview

This document captures the complete VibePilot process: how ideas become code, who does what, and how everything connects and learns.

---

## KEY PRINCIPLES

- Always flexible, never fragile
- Always configurable, never hardcoded
- Always agnostic and swappable, never vendor locked
- Always lean, clean, fully functional
- Great foundational architecture, no shortcuts or stubs
- Everything learns from everything

---

## ALWAYS-ON SERVICES

| Service | Why Always On |
|---------|---------------|
| **Vibes** | Human interface - must respond to voice/text anytime |
| **Orchestrator** | Task routing - must watch queue, route to runners, track status, create branches |
| **System Researcher** | Scheduled daily cron for research |

Everything else is called as needed.

---

## END-TO-END FLOW

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              IDEA → PRD                                      │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  Human: "I want to add X to project Y"                                      │
│          ↓                                                                   │
│  Consultant/Researcher agent:                                               │
│    - Converses with human (via Vibes interface)                             │
│    - Researches requirements                                                │
│    - Creates full PRD with zero ambiguity                                   │
│    - PRD saved to GitHub: docs/prd/project-name.md                          │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
          ↓
┌─────────────────────────────────────────────────────────────────────────────┐
│                              PRD → PLAN                                      │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  PRD in GitHub: docs/prd/project-name.md                                    │
│          ↓                                                                   │
│  Planner reads PRD:                                                         │
│    - Decides modules/slices (grouping related tasks)                        │
│    - Sets priority ordering                                                 │
│    - Flags: internal_only for tasks that need codebase                      │
│    - Breaks into atomic tasks (95%+ confidence each)                        │
│    - Maps dependencies between tasks                                        │
│    - Creates complete prompt packets for each task                          │
│    - Plan saved to GitHub: docs/plans/project-name-plan.md                  │
│                                                                              │
│  Each task includes:                                                        │
│    - task_id, task_number (T001, T002...)                                   │
│    - prompt_packet (full executable instructions)                           │
│    - expected_output (files, tests, deliverables)                           │
│    - dependencies (which tasks must complete first)                         │
│    - internal_only flag (true = internal runner only)                       │
│    - confidence score (must be ≥0.95)                                       │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
          ↓
┌─────────────────────────────────────────────────────────────────────────────┐
│                         SUPERVISOR PLAN REVIEW                               │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  Supervisor receives plan                                                   │
│          ↓                                                                   │
│  Assessment:                                                                │
│    - Simple plan (1-2 independent tasks)?                                   │
│        → Approve directly                                                   │
│    - Complex / could affect things?                                         │
│        → Route to Council for review                                        │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
          ↓ (if complex)
┌─────────────────────────────────────────────────────────────────────────────┐
│                         COUNCIL REVIEW                                       │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  3 Council Members review INDEPENDENTLY (no collusion)                      │
│                                                                              │
│  Lens 1: User Alignment                                                     │
│    - Does this match human's intent?                                        │
│    - Are P0/P1/P2 priorities preserved?                                     │
│    - Nothing missing? Nothing extra?                                        │
│                                                                              │
│  Lens 2: Architecture & Technical                                           │
│    - Is the design sound?                                                   │
│    - Security addressed?                                                    │
│    - Scalability considered?                                                │
│                                                                              │
│  Lens 3: Feasibility & Gaps                                                 │
│    - Can each task actually be executed?                                    │
│    - Dependencies realistic?                                                │
│    - Prompt packets complete?                                               │
│                                                                              │
│  Each member votes: APPROVED / REVISION_NEEDED / BLOCKED                   │
│          ↓                                                                   │
│  Consensus?                                                                 │
│    - All APPROVED → Plan approved                                           │
│    - Any REVISION_NEEDED → Planner fixes + learns → Resubmit               │
│    - Any BLOCKED → Escalate to human                                        │
│    - Max 6 rounds → Supervisor decides or escalate to human                │
│                                                                              │
│  PLANNER LEARNS from Council feedback:                                      │
│    - Better task breakdown patterns                                         │
│    - More accurate confidence scores                                        │
│    - When to flag internal_only                                             │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
          ↓
┌─────────────────────────────────────────────────────────────────────────────┐
│                    SUPERVISOR FINAL APPROVAL                                 │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  Supervisor sees Council approval                                           │
│          ↓                                                                   │
│  Quick check on plan                                                        │
│          ↓                                                                   │
│  Approves plan                                                              │
│          ↓                                                                   │
│  Tasks created in Supabase:                                                 │
│    - Each task: id, number, prompt_packet, status='pending'                │
│    - Dependencies mapped from plan                                          │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
          ↓
┌─────────────────────────────────────────────────────────────────────────────┐
│                         ORCHESTRATOR ROUTING                                 │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  Orchestrator (always on, systemd service):                                 │
│    - Polls Supabase for tasks                                               │
│    - Tasks become 'available' when dependencies met                         │
│    - Reads: task priority, internal_only flag, task type                   │
│    - Selects best destination using routing config + learned model scores   │
│          ↓                                                                   │
│  ORCHESTRATOR ASSIGNS TASK:                                                 │
│    1. Selects destination + model                                           │
│    2. Calls gitree.CreateBranch("task/T001")                               │
│    3. Task status → 'in_progress'                                           │
│    4. Sends to runner with destination info                                 │
│                                                                              │
│  Routing logic:                                                             │
│    - Agent restrictions (planner/supervisor/etc = internal_only)            │
│    - Task flags (internal_only = skip external)                             │
│    - Model scores (learned from past performance)                           │
│    - Destination availability (rate limits, status)                         │
│                                                                              │
│  ORCHESTRATOR LEARNS from every task result:                                │
│    - Model performance by task type                                         │
│    - Failure patterns                                                       │
│    - Optimal routing decisions                                              │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
          ↓
┌─────────────────────────────────────────────────────────────────────────────┐
│                           TASK EXECUTION                                     │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌─────────────────────────────┐  ┌─────────────────────────────┐          │
│  │      INTERNAL RUNNER        │  │         COURIER             │          │
│  ├─────────────────────────────┤  ├─────────────────────────────┤          │
│  │ Has: file_read access       │  │ Has: NO codebase access     │          │
│  │ Receives:                   │  │ Receives:                   │          │
│  │   - Task packet             │  │   - Task packet             │          │
│  │   - Relevant code files     │  │   - Platform destination    │          │
│  │ Runs on: CLI or API         │  │     (added by Orchestrator) │          │
│  │   - OpenCode CLI            │  │ Runs on: Web platforms      │          │
│  │   - Kimi CLI                │  │   - ChatGPT web             │          │
│  │   - DeepSeek API            │  │   - Claude web              │          │
│  │                             │  │   - Gemini web              │          │
│  │ Use when:                   │  │   - DeepSeek web            │          │
│  │   needs_codebase: true      │  │   - Qwen web                │          │
│  │   internal_only: true       │  │ Use when:                   │          │
│  │                             │  │   Standalone task           │          │
│  │                             │  │   No codebase needed        │          │
│  └─────────────────────────────┘  └─────────────────────────────┘          │
│                                                                              │
│  Runner commits output to task branch (task/T001)                          │
│  Task status → 'review'                                                     │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
          ↓
┌─────────────────────────────────────────────────────────────────────────────┐
│                        SUPERVISOR OUTPUT REVIEW                              │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  Supervisor receives: task + output + expected_output                       │
│          ↓                                                                   │
│  Supervisor reads task branch                                               │
│          ↓                                                                   │
│  Checks:                                                                    │
│    - All expected files present?                                            │
│    - Output matches prompt packet?                                          │
│    - No hardcoded secrets?                                                  │
│    - Pattern consistency?                                                   │
│    - No extra files touched?                                                │
│    - Truncation detected? (limit hit? model weak?)                          │
│    - Drift detected? (wrong version, wrong approach?)                       │
│    - Security issues? (injected code, suspicious patterns?)                 │
│    - Quality acceptable?                                                    │
│          ↓                                                                   │
│  Decision:                                                                  │
│    - PASS → Trigger tests                                                   │
│    - FAIL → See Failure Handling flow                                       │
│    - REROUTE → Different runner/model (if model seems incapable)            │
│                                                                              │
│  SUPERVISOR LEARNS from output quality:                                     │
│    - Better detection of truncation/drift/security                          │
│    - When to reroute vs retry                                               │
│    - Model capability patterns                                              │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## FAILURE HANDLING FLOW

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         SUPERVISOR DETECTS FAILURE                           │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  Failure types:                                                             │
│    - Truncation (output cut off, limit hit, model weak)                     │
│    - Drift (wrong version, different approach than specified)               │
│    - Security (injected code, secrets, suspicious patterns)                 │
│    - Wrong output (doesn't match expected_output)                           │
│    - Incomplete (missing files, tests failed)                               │
│                                                                              │
│  Action:                                                                    │
│    1. Branch WIPED (ClearBranch or Delete+Create)                           │
│    2. Model marked FAILED on this task                                      │
│    3. Failure reason + supervisor notes logged to task_runs                 │
│    4. Orchestrator notified                                                 │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
          ↓
┌─────────────────────────────────────────────────────────────────────────────┐
│                      ORCHESTRATOR REASSIGNS                                  │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  Orchestrator sees:                                                         │
│    - Failure reason                                                         │
│    - Supervisor notes                                                       │
│    - Model that failed                                                      │
│    - Task type                                                              │
│          ↓                                                                   │
│  Decision:                                                                  │
│    - Try different model (based on learned scores)                          │
│    - Avoid model that failed for this task type                             │
│          ↓                                                                   │
│  Reassign:                                                                  │
│    - Task branch recreated (task/T001)                                      │
│    - New runner assigned                                                    │
│    - Task status → 'in_progress'                                            │
│                                                                              │
│  ORCHESTRATOR LEARNS:                                                       │
│    - Model X fails on task type Y with truncation                           │
│    - Adjust future routing for similar tasks                                │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
          ↓
┌─────────────────────────────────────────────────────────────────────────────┐
│                    REPEATED FAILURE (same issue, different model)            │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  If SAME issue with DIFFERENT model:                                        │
│          ↓                                                                   │
│  Supervisor notes pattern:                                                  │
│    - "Multiple models failing with truncation on this task"                 │
│    - "Multiple models showing drift on this task"                           │
│          ↓                                                                   │
│  Orchestrator decision:                                                     │
│    - Try another model (if smart routing suggests it might work)            │
│    - OR send to Planner for task fix                                        │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
          ↓ (if sent to Planner)
┌─────────────────────────────────────────────────────────────────────────────┐
│                         PLANNER FIXES TASK                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  Planner receives:                                                          │
│    - Original task                                                          │
│    - Failure history (which models, what issues)                            │
│    - Supervisor notes                                                       │
│          ↓                                                                   │
│  Planner fixes:                                                             │
│    - Modify prompt packet (more specific, better context)                   │
│    - Split into 2+ smaller tasks (if too complex)                           │
│    - Both of above                                                          │
│          ↓                                                                   │
│  Modified task → Supervisor checks → Loop continues                         │
│                                                                              │
│  PLANNER LEARNS from failure patterns:                                      │
│    - Better task sizing (when to split)                                     │
│    - More specific prompt packets                                           │
│    - Better confidence estimates                                            │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## BRANCH LIFECYCLE

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         BRANCH LIFECYCLE                                     │
└─────────────────────────────────────────────────────────────────────────────┘

1. TASK ASSIGNED
   Orchestrator assigns task
       ↓
   gitree.CreateBranch("task/T001")
   Branch created (orphan, no parent history)
   
2. CODE COMMITTED
   Runner executes task
       ↓
   Output committed to task/T001
       ↓
   Supervisor reviews (reads branch)

3. TASK FAILS (branch wiped for retry)
   Supervisor detects issue
       ↓
   gitree.ClearBranch("task/T001")
       ↓
   Branch content removed, ready for new attempt
       ↓
   Reassign to different model
   
4. TASK PASSES → MODULE (after tests pass)
   Supervisor approves
       ↓
   Tests pass
       ↓
   gitree.MergeBranch("task/T001", "module/{slice_id}")
       ↓
   gitree.DeleteBranch("task/T001")
   Task status → 'complete'
   Supervisor: unlocks dependent tasks

5. MODULE → MAIN (all tasks complete + module tests pass)
   All tasks in module complete
       ↓
   Module-level integration tests pass
       ↓
   gitree.MergeBranch("module/{slice_id}", "main")
       ↓
   gitree.DeleteBranch("module/{slice_id}")
   Tag: module-{name}-v1

RESULT: Clean main, all work tagged, no stale branches
```

**Branch naming:**
- Task branches: `task/{task_number}` (e.g., task/T001, task/T002)
- Module branches: `module/{slice_id}`

---

## GITHUB - WHO DOES WHAT

| Action | Who Decides | Who Executes | Notes |
|--------|-------------|--------------|-------|
| Create task branch | Orchestrator | gitree (programmatic) | When task assigned |
| Read task branch | Supervisor | gitree (read-only) | For quality review |
| Commit code | Runner | gitree.CommitOutput() | After execution |
| Wipe branch | Supervisor decision | gitree.ClearBranch() | On failure for retry |
| Merge task → module | Supervisor decision | gitree.MergeBranch() | After tests pass |
| Delete task branch | Automatic | gitree.DeleteBranch() | After merge verified |
| Create module branch | Orchestrator | gitree.CreateModuleBranch() | First task in module |
| Merge module → main | Supervisor decision | gitree.MergeBranch() | After module complete |
| Delete module branch | Automatic | gitree.DeleteBranch() | After merge verified |
| Tag release | Supervisor decision | gitree (tag command) | After merge to main |

**gitree is NOT an agent.** It's a utility library the Governor uses programmatically.

---

## TESTS RUN

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

Visual Tester (for UI tasks):
  - Captures screenshots at all breakpoints
  - Runs automated accessibility checks
  - HUMAN APPROVAL ALWAYS REQUIRED
  - Returns: pass/fail + human feedback
```

---

## SYSTEM RESEARCHER FLOW

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                      SYSTEM RESEARCHER (Daily)                               │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  Runs: Once per day at 6 AM UTC                                             │
│  Output: docs/UPDATE_CONSIDERATIONS.md                                      │
│                                                                              │
│  Researches:                                                                │
│    - New AI models, platforms, tools                                        │
│    - Pricing changes                                                        │
│    - Free tier availability                                                 │
│    - User rankings and sentiment                                            │
│    - Security advisories                                                    │
│          ↓                                                                   │
│  Produces suggestions in UPDATE_CONSIDERATIONS.md                           │
│                                                                              │
│  SYSTEM RESEARCHER LEARNS from:                                             │
│    - Council feedback on suggestions (accepted vs rejected)                 │
│    - How system currently functions (what's working, what's not)            │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
          ↓
┌─────────────────────────────────────────────────────────────────────────────┐
│                    SUPERVISOR REVIEWS SUGGESTIONS                            │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  Supervisor sees suggestions from System Researcher                         │
│          ↓                                                                   │
│  Assessment:                                                                │
│                                                                              │
│  SIMPLE suggestion:                                                         │
│    - Approve                                                                │
│    - Send to Planner to implement                                           │
│                                                                              │
│  COMPLEX suggestion:                                                        │
│    - Call Council with full suggestions                                     │
│    - Council provides feedback                                              │
│    - Send for human review                                                  │
│                                                                              │
│  PAID API OUT OF CREDIT:                                                    │
│    - Send for human review                                                  │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## MODEL SELECTION

**Config-driven via routing.json:**

```json
{
  "default_strategy": "default",
  "strategies": {
    "default": {"priority": ["external", "internal"]},
    "internal_only": {"priority": ["internal"]}
  },
  "agent_restrictions": {
    "internal_only": ["planner", "supervisor", "council", "orchestrator", "maintenance", "watcher", "tester"],
    "default": ["consultant", "researcher", "courier", "task_runner"]
  },
  "destination_categories": {
    "external": {"check_field": "type", "check_values": ["web"]},
    "internal": {"check_field": "type", "check_values": ["cli", "api"]}
  }
}
```

**Selection flow:**
1. Agent ID → Get strategy (internal_only or default)
2. Strategy → Priority order (e.g., [external, internal])
3. For each category in priority:
   - Get active destinations in category
   - Check availability (status=active, not at limit)
   - Get model score from RPC (learned from past performance)
4. Return best destination + model

---

## COURIER vs INTERNAL RUNNER

| | Internal Runner | Courier |
|---|---|---|
| **Codebase access** | YES (file_read) | NO |
| **Receives** | Task packet + relevant code files | Task packet + platform destination |
| **Platform destination** | Determined by routing | Added by Orchestrator (knows availability) |
| **Runs on** | CLI (OpenCode, Kimi) or API (DeepSeek) | Web platforms (ChatGPT, Claude, Gemini, etc.) |
| **Returns** | Code output | Output + chat_url |
| **Use when** | `needs_codebase: true` or `internal_only: true` | Standalone task, no codebase needed |

**Flag set by:** Planner (in task plan)

---

## AGENT DEFINITIONS

### Vibes (Interface) - ALWAYS ON
- Role: Human's primary interface
- Receives: Voice, text, dashboard input
- Routes to appropriate agent
- Returns: Results to human

### Consultant/Researcher
- Role: Interactive PRD generation
- Converses with human
- Researches requirements
- Produces complete PRD
- Learns from: PRD outcomes

### Planner
- Role: Plan creation and editing
- Decides: modules/slices, priority, internal_only flags
- Creates: tasks, dependencies, prompt packets
- Edits: plan when Council/Orchestrator requests changes
- Fixes: tasks when repeated failures occur
- Learns from: Council feedback, task failure patterns

### Council
- Not a fixed agent - a function
- Function: Multi-lens review
- Lenses: User Alignment, Architecture, Feasibility
- Context: Depends on review type (project vs system)
- Routed by: Orchestrator
- Members review INDEPENDENTLY (no collusion)
- Learns from: Review outcomes, downstream failures

### Orchestrator - ALWAYS ON
- Role: Task routing, monitoring, learning, branch creation
- Always running (systemd service)
- Creates task branches when assigning
- Routes tasks to best available destination
- Knows: rate limits, refresh times, countdown
- Reassigns on failure
- Calls Planner for task fixes when needed
- Learns from: Every single task result

### Supervisor
- Role: Quality gatekeeper
- Reviews: plans (→Council or approve), output, test results
- Detects: truncation, drift, security issues
- Decides: approve/reject/reroute
- Triggers: tests
- Reviews: System Researcher suggestions
- Learns from: Output quality, failure types

### Maintenance
- Role: Git/file operator (executes commands)
- ONLY agent with: git write access, file write access
- Executes: create branch, commit, merge, delete, tag
- Applies: approved changes from System Researcher
- Reports: success/failure of each command

### Internal Runner
- Role: Execute tasks with codebase context
- Has: file_read (not write)
- Receives: task packet + relevant codebase files
- Returns: code output

### Courier
- Role: Execute tasks on web platforms
- No codebase access
- Receives: task packet + platform destination (from Orchestrator)
- Returns: output + chat_url

### Code Tester
- Role: Validate code
- Sees: code + test criteria only
- Runs: pytest, lint, typecheck
- Returns: pass/fail

### Visual Tester
- Role: Validate UI/UX
- Captures: screenshots at all breakpoints
- Runs: automated accessibility checks
- HUMAN APPROVAL ALWAYS REQUIRED
- Returns: pass/fail + human feedback

### System Researcher - SCHEDULED (daily cron)
- Role: Continuous improvement intelligence
- Has: Full system understanding
- Researches: new models, pricing, tools, improvements
- Output: docs/UPDATE_CONSIDERATIONS.md
- Learns from: Council feedback on suggestions, how system functions

---

## STATUS TRACKING

```
Supabase: tasks table
  - id, task_number, title, status, dependencies, prompt_packet
  - internal_only flag
  - expected_output

Status values:
  - pending: Created, dependencies not met
  - available: Ready to be picked up, dependencies met
  - in_progress: Runner assigned, branch created
  - review: Output returned, awaiting Supervisor
  - testing: Tests running
  - complete: Approved, tested, merged to module
  - failed: Exhausted retries or unfixable (with notes)
  - queued: No runner available

Supabase: task_runs table
  - task_id, model_id, status, tokens, output
  - failure_reason, supervisor_notes
  - duration, created_at

Supabase: model_scores table (for learning)
  - model_id, task_type, score
  - Updated after each task completion
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
       │ assigns + creates branch
       ▼
┌─────────────┐     returns     ┌─────────────┐
│   RUNNER    │ ───────────────→│   OUTPUT    │
│(Courier/    │                 │             │
│ Internal)   │                 │             │
└─────────────┘                 └──────┬──────┘
                                       │ reviewed by
                                       ▼
┌─────────────┐     approves     ┌─────────────┐
│ SUPERVISOR  │ ◄─────────────── │   TESTS     │
│  (read git) │                  │             │
└──────┬──────┘                  └─────────────┘
       │ programmatic git operations
       ▼
┌─────────────┐     executes    ┌─────────────┐
│  GITREE     │ ───────────────→│    GIT      │
│ (utility)   │                 │  (branches) │
└─────────────┘                 └─────────────┘
```

---

## LEARNING SYSTEM SUMMARY

**Everything learns from everything:**

| Agent | Learns From | What It Learns |
|-------|-------------|----------------|
| **Planner** | Council feedback, task failure patterns | Better task breakdown, accurate confidence, when to split |
| **Council** | Review outcomes, downstream failures | Better issue detection, accurate severity |
| **Orchestrator** | Every task result | Model performance by task type, optimal routing |
| **Supervisor** | Output quality, failure types | Better detection, when to reroute vs retry |
| **Consultant** | PRD outcomes | Better requirement gathering |
| **System Researcher** | Council feedback, system function | Better suggestions, relevant research |

**Data for learning:**
- `task_runs`: model, status, failure_reason, duration
- `model_scores`: model_id, task_type, score (learned)
- `failure_patterns`: task_type, failure_reason, model_id

---

## DOCUMENT STATUS

- [x] Flow documented
- [x] Roles defined
- [x] Branch lifecycle captured
- [x] Council process defined
- [x] Failure handling flow documented
- [x] Learning system documented
- [x] System Researcher flow documented
- [x] Courier vs Internal clarified
- [x] Branch creation on assignment documented
- [x] Everything learns principle captured
- [ ] Implementation (ongoing)

---

**This document preserves Session 36 decisions. If session dies, resume from here.**
