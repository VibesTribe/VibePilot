# VibePilot Architecture Analysis Report

**Date:** 2026-03-02
**Status:** CRITICAL - Major architectural issues blocking autonomous operation

---

## Executive Summary

VibePilot has fundamental architectural problems that prevent reliable autonomous operation:

1. **Processing claims are timeout-based** (10 min) instead of state-based
2. **Revision flow is fragile** - can get lost in processing void
3. **Error states are permanent** - no recovery path
4. **No "resume from where I left off" logic**
5. **Everything is SLOW** - minutes instead of seconds

This document analyzes what we have vs. what we need, and proposes a proper fix.

---

## Part 1: What Vibeflow Does Right

### 1.1 Simple State Machine
```
draft → planning → review → revision → approved → executing → merged
```
- Each state has clear transitions
- Each state knows how to resume
- No ambiguity, no lost work

### 1.2 Persistent Event Log
- **All events logged** to `events.log.jsonl`
- **State stored in `task.state.json`**
- **Any crash = read state, resume from where it left off**

### 1.3 Clean Task Format
```json
{
  "task_id": "M0.1",
  "title": "Auto Runner Script",
  "context": "Brief explanation",
  "files": ["path/to/file.ts"],
  "acceptance_criteria": [
    "Testable criterion 1",
    "Testable criterion 2"
  ]
}
```
- Simple, focused, no ambiguity
- Easy for agents to output correctly

### 1.4 Lessons for VibePilot
1. **State-based, not timeout-based**
2. **Events logged immediately** (persistent)
3. **State in one place** (trackable)
4. **Crashes = resume from state** (recoverable)
5. **Clear status progression** (no ambiguity)

---

## Part 2: What VibePilot Has Now

### 2.1 Database Schema (Good)
- `plans` table with status, revision tracking
- `tasks` table with processing claims
- `task_packets` table for prompts (versioned)
- `task_runs` table for execution history
- ROI tracking built-in

### 2.2 Processing Claims (BROKEN)
```
Set processing claim → Work → If success: clear → If failure: wait 10 min
```

**Problems:**
- 10 minute timeout blocks legitimate progress
- Revision state can get lost
- Stuck tasks can't be recovered
- No state-based recovery logic

### 2.3 Revision Flow (FRAGILE)
```
Plan rejected → revision_needed status → Planner revises → ???
```

**Problems:**
- Can disappear into processing void
- No clear tracking of what changed
- No limit on revision rounds
- Feedback can be lost

### 2.4 Error States (PERMANENT)
```
Error → Stuck forever
```

**Problems:**
- No recovery path
- No retry logic
- No escalation
- Blocks entire system

---

## Part 3: What VibePilot Should Be

### 3.1 Core Principles

1. **Fast** - Seconds, not minutes
2. **State-based** - Not timeout-based
3. **Recoverable** - Every state knows how to resume
4. **Trackable** - Every transition logged
5. **Simple** - Less code, more reliable
6. **Configurable** - No hardcoded values
7. **Agnostic** - Works with any model/CLI/API

### 3.2 State Machine Design

```
PRD detected → draft
draft → planning (planner working)
planning → review (plan created, supervisor reviewing)
review → revision_needed (rejected)
review → approved (accepted)
revision_needed → planning (planner revising)
approved → tasks_created (tasks in database)
tasks_created → executing (runners working)
executing → testing (code complete, tests running)
testing → merged (all tests pass)
testing → revision_needed (tests fail)
```

Each state transition:
1. Clear processing claim immediately
2. Log transition to database
3. Update timestamps
4. Record what happened

### 3.3 Processing Claims Design

**Current (BROKEN):**
```
Set claim → Work → Success/Failure → Wait for timeout
```

**Better:**
```
Set claim → Work → Clear claim immediately on:
  - Success (state transition)
  - Failure (error logged, retry count incremented)
  - Crash (detected on recovery)
  
Fallback: timeout (5 min, configurable)
```

**Implementation:**
- Add `processing_at` timestamp
- Add `processing_by` string (who's working)
- Add `attempts` count
- Clear on success/failure
- Timeout as fallback only

### 3.4 Recovery Design

**Current (BROKEN):**
```
Wait for timeout → Clear claim → Hope for the best
```

**Better:**
```
On startup:
  1. Find items with processing_at > timeout ago
  2. Check state:
     - If "planning" + no plan file + no claim → resume planning
     - If "review" + plan exists + no claim → resume review
     - If "revision_needed" + feedback exists → resume revision
     - If "error" + attempts < max → retry
     - If "error" + attempts >= max → escalate
  3. Clear stale claims
  4. Continue flow
```

### 3.5 Revision Flow Design

**Current (FRAGILE):**
```
Rejected → ??? → Lost in processing void
```

**Better:**
```
Rejected → revision_needed status
  ↓
Increment revision_round
Store feedback in latest_feedback JSONB
Store history in revision_history JSONB
Set tasks_needing_revision array
Clear processing claim
  ↓
Planner reads feedback
Planner revises ONLY tasks needing revision
Planner outputs updated plan
  ↓
Supervisor reviews revision
If approved → continue
If rejected → increment round again
If max_rounds → escalate to human
```

---

## Part 4: Specific Fixes Needed

### 4.1 Database Schema Changes

```sql
-- Add revision tracking to plans
ALTER TABLE plans ADD COLUMN IF NOT EXISTS revision_round INT DEFAULT 0;
ALTER TABLE plans ADD COLUMN IF NOT EXISTS revision_history JSONB DEFAULT '[]';
ALTER TABLE plans ADD COLUMN IF NOT EXISTS latest_feedback JSONB;
ALTER TABLE plans ADD COLUMN IF NOT EXISTS tasks_needing_revision TEXT[] DEFAULT '{}';

-- Add processing timestamps (already exists, verify)
ALTER TABLE plans ADD COLUMN IF NOT EXISTS processing_at TIMESTAMPTZ;
ALTER TABLE plans ADD COLUMN IF NOT EXISTS processing_by TEXT;

-- Add retry tracking to tasks
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS attempts INT DEFAULT 0;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS last_error TEXT;
```

### 4.2 Config Changes

```json
{
  "recovery": {
    "processing_timeout_seconds": 300,
    "max_revision_rounds": 6,
    "max_task_attempts": 3,
    "recovery_interval_seconds": 10
  }
}
```

### 4.3 Code Changes

**1. Clear processing claims immediately on success/failure**
- In every event handler
- Before returning
- Even on error

**2. State-based recovery**
- Check state, not just timeout
- Resume from where left off
- Log recovery actions

**3. Revision flow tracking**
- Store feedback in database
- Track revision round
- Limit retries

**4. Error state recovery**
- Retry with backoff
- Escalate to human if max attempts
- Log failure patterns

**5. Faster polling for testing**
- Reduce intervals
- Make configurable

---

## Part 5: Migration Strategy

### Phase 1: Quick Wins (Today)
1. ✅ Simplified planner prompt (done)
2. ✅ Fixed prompt packet parsing (done)
3. ⬜ Clear all processing claims (manual SQL)
4. ⬜ Reduce timeout to 5 min (config change)
5. ⬜ Add debug logging to recovery

### Phase 2: Database Fixes (This Week)
1. Add revision tracking columns
2. Add retry tracking columns
3. Update RPCs to use new columns
4. Test revision flow end-to-end

### Phase 3: Code Refactor (Next Week)
1. Implement state-based recovery
2. Clear processing claims properly
3. Add retry logic
4. Add escalation logic

### Phase 4: Testing (Week 3)
1. Full autonomous test
2. Crash recovery test
3. Revision flow test
4. Error recovery test

---

## Part 6: Immediate Actions

### Right Now:
1. Run `scripts/sql/clear_processing_claims.sql` in Supabase
2. Watch the flow with current code
3. Document every issue we see

### This Session:
1. Reduce processing timeout to 5 min
2. Add more debug logging
3. Test with fresh PRD
4. Document issues for next phase

### Next Session:
1. Add revision tracking columns
2. Fix revision flow
3. Implement proper recovery

---

## Part 7: Success Metrics

### Speed
- PRD to plan: < 30 seconds
- Plan to tasks: < 10 seconds
- Task to execution: < 5 seconds
- Total flow: < 2 minutes for simple PRD

### Reliability
- Zero lost work
- Zero stuck states
- 100% recovery from crashes
- Clear escalation path

### Quality
- All prompt packets complete
- All tasks validated
- All revisions tracked
- All errors logged

---

## Conclusion

VibePilot has the right foundation (database schema, event system, agent architecture) but critical architectural flaws in processing claims, revision flow, and error recovery.

**The fix is not more code - it's better design.**

We need:
1. State-based recovery (not timeout)
2. Proper revision tracking
3. Clear state machine
4. Fast, configurable timeouts
5. Simple, focused agent prompts

**This is achievable. The architecture is sound. We just need to fix the implementation.**
