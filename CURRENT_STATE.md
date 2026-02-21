# VibePilot Current State

**Required reading: FIVE files**
1. **THIS FILE** (`CURRENT_STATE.md`) - What, where, how, current state
2. **`docs/WHAT_WHERE.md`** - Where everything is located
3. **`docs/prd_v1.4.md`** - Complete system specification
4. **`docs/core_philosophy.md`** - Strategic mindset and inviolable principles
5. **`CHANGELOG.md`** - History, changes, rollback commands

**Read all five → Know everything → Do anything**

---

**Last Updated:** 2026-02-21 06:15 UTC
**Updated By:** GLM-5 - Pipeline test day 1 complete, critical gaps identified
**Session Focus:** Infrastructure wiring + first pipeline test revealed core gaps

**Schema Location:** `docs/supabase-schema/` (all SQL files)
**Progress:** Foundation wired, test revealed routing intelligence gaps

---

# SESSION 21 SUMMARY (2026-02-21)

## Infrastructure Wiring Complete (GLM-5 + Kimi)

### 1. Supervisor Final Approval → Tasks Created ✅
**Problem:** Council approves plan, but no trigger to create tasks
**Solution:** `SupervisorAgent.approve_plan_and_create_tasks(plan_path)`
**Key Insight:** Tasks created as "available" (not pending) because plan is already approved
**Commit:** `e44ce394`

### 2. Maintenance Integrated in Orchestrator ✅
**Problem:** Maintenance agent existed but never ran (commands queued forever)
**Solution:** Added `_process_maintenance_commands()` to orchestrator._tick()
**Decision:** Option B (integrated, not separate service) - if orchestrator dies, no git work anyway
**Commit:** `9f7b1ae2`

### 3. State Machine Clarified ✅
**Human Clarified:**
- `approved` = tests pass, approved = ready for git
- `merged` = git operations complete = FINAL STATUS
- No `merged → complete` transition needed

### 4. First Pipeline Test Attempted ⚠️
**Test Task:** Change "Vibeflow" to "VibePilot" in MissionHeader.tsx
**PRD:** Created in `docs/prd/vibepilot-rename-dashboard-prd.md`
**Plan:** Created in `docs/plans/vibeflow-test-plan.json`
**Branch:** `vibeflow-test` in vibeflow repo

---

## CRITICAL GAPS DISCOVERED (MUST FIX)

### Gap 1: Routing Intelligence Missing

**Problem:** Tasks have `routing_flag` but orchestrator doesn't make smart decisions

**What Happened:**
- Task T001 created with `routing_flag="web"`
- No runner supports "web" routing
- Task escalated after max retries

**What Should Happen:**
- Orchestrator or Council analyzes task requirements
- Dashboard work requires codebase access → should route to "internal" (Kimi/GLM-5)
- System auto-selects appropriate routing based on task analysis

**Current Valid Routing Flags:** `internal`, `web`
**Current Runners:** kimi-cli, opencode (both CLI with codebase), courier (browser, no codebase)

### Gap 2: Available Agent Awareness

**Problem:** Orchestrator doesn't know which agents are actually available

**What Happened:**
- Kimi quota exceeded mid-test
- Orchestrator kept trying to route to unavailable agents
- Should have known only GLM-5 was available

**What Should Happen:**
- Orchestrator tracks agent availability (quota status, online/offline)
- Routes only to available agents
- Council/orchestrator considers "who's available" before routing

### Gap 3: Multiple Parallel Tasks

**Problem:** 17 available tasks in queue, orchestrator processing one at a time

**What Should Happen:**
- Orchestrator dispatches multiple tasks in parallel
- Respects max_workers config
- Handles old tasks + new tasks added dynamically

### Gap 4: Planner Plan Parsing

**Problem:** "Could not parse plan from LLM response"

**What Happened:**
- Planner (via kimi-cli) returned unparseable response
- Idea processing failed repeatedly

**What Should Happen:**
- Robust plan parsing with validation
- Fallback/retry with different model if parsing fails
- Better error handling

---

## WHAT NEEDS BUILDING (Priority Order)

### 1. Smart Routing Intelligence (CRITICAL)
**Owner:** GLM-5 or Kimi (needs coordination)

**Tasks:**
- [ ] Orchestrator analyzes task before routing
- [ ] Detects: needs codebase? needs browser? simple API?
- [ ] Maps task type → appropriate routing_flag
- [ ] Council catches bad routing decisions

**Example Logic:**
```
Task: "Edit React component in dashboard"
Analysis: Requires codebase access, file editing
Decision: route to "internal" (Kimi or GLM-5 with codebase)

Task: "Research competitor pricing"
Analysis: Needs web browsing, no codebase
Decision: route to "courier" or "web"
```

### 2. Agent Availability Tracking
**Owner:** GLM-5

**Tasks:**
- [ ] Track each agent's quota status
- [ ] Track online/offline status
- [ ] Only route to available agents
- [ ] Alert when no agents available

### 3. Parallel Task Dispatch
**Owner:** GLM-5

**Tasks:**
- [ ] Verify max_workers respected
- [ ] Multiple tasks dispatch simultaneously
- [ ] Track each independently

### 4. Robust Plan Parsing
**Owner:** TBD

**Tasks:**
- [ ] Validate LLM response before accepting
- [ ] Fallback to different model if parse fails
- [ ] Better error messages

### 5. Council Routing Review
**Owner:** Kimi (front-end) + GLM-5 (back-end coordination)

**Tasks:**
- [ ] Council reviews routing decisions
- [ ] Catches "web" routing for codebase tasks
- [ ] Can veto and reroute

---

## CURRENT SYSTEM STATE

| Component | Status | Notes |
|-----------|--------|-------|
| Orchestrator | ✅ Running | systemd service, polling every 2s |
| Consultant | ✅ Wired | Routes through orchestrator |
| Planner | ⚠️ Partial | Routes through orchestrator but parsing fails |
| Council | ⚠️ Simplified | Placeholder, not full implementation |
| Supervisor | ✅ Wired | approve_plan_and_create_tasks works |
| Maintenance | ✅ Wired | Integrated in _tick() |
| Executioner | ✅ Exists | Wired but not tested |
| Runner Pool | ⚠️ Gaps | No smart routing, no availability tracking |

---

## TEST TASK STATUS

**Task T001:** Change brand name in MissionHeader
- Status: `escalated` (failed after max retries)
- Issue: Bad routing_flag, no available runner
- Fix: Updated routing_flag to "internal"
- Next: Reset and retry after smart routing built

---

## TOMORROW'S SESSION

1. **Reset test tasks** - Clean state
2. **Build smart routing** - Analyze task → pick right runner
3. **Build agent availability tracking** - Know who's online
4. **Retry pipeline test** - Verify end-to-end

---

# ACTIVE MODELS (Current State)

| Model ID | Status | Access Via | Notes |
|----------|--------|------------|-------|
| kimi-cli | quota_exceeded | kimi-cli | Was active, now paused |
| glm-5 / opencode | active | opencode CLI | Only available agent |
| gemini-api | paused | API | quota_exhausted |
| deepseek-chat | paused | API | credit_needed |
| gpt-4o, gpt-4o-mini | benched | N/A | Web platform only |
| claude-sonnet-4-5, claude-haiku-4-5 | benched | N/A | Web platform only |

---

# TASK STATUS FLOW

```
pending ──► approve_plan() ──┬──► available (no deps) ──► in_progress
                             │
                             └──► locked (has deps)
                                        │
                                        │ [parent task merges]
                                        │ [unlock_dependent_tasks RPC fires]
                                        ▼
                                   available ──► in_progress
```

**Full status lifecycle:**
```
pending → available → in_progress → review → testing → approved → merged (FINAL)

Special states:
- locked: Awaiting dependencies
- escalated: Max attempts exceeded
- awaiting_human: Visual/manual testing needed
```

---

# QUICK COMMANDS

| Command | Action |
|---------|--------|
| `cat CURRENT_STATE.md` | This file |
| `cat AGENT_CHAT.md` | GLM-Kimi chat |
| `git log --oneline -10` | Recent commits |
| `sudo journalctl -u vibepilot-orchestrator -f` | Orchestrator logs |
| `python -c "from agents.supervisor import SupervisorAgent; s=SupervisorAgent(); print(s.approve_plan_and_create_tasks('docs/plans/plan.json'))"` | Create tasks from plan |

---

# FILES MODIFIED THIS SESSION

| File | Change | Commit |
|------|--------|--------|
| `core/orchestrator.py` | Added `_process_maintenance_commands()` | `9f7b1ae2` |
| `agents/maintenance.py` | Added `execute()` method | `9f7b1ae2` |
| `agents/supervisor.py` | Added `approve_plan_and_create_tasks()` | `e44ce394` |
| `agents/supervisor.py` | Added `set_orchestrator()` | `01ac0b1c` |
| `prompts/consultant.md` | Added ZERO ASSUMPTIONS RULE | Session 19 |
| `docs/plans/vibeflow-test-plan.json` | Test plan created | `5f91245f` |

---

# KIMI USAGE PRIORITY

**Kimi subscription: Currently quota exceeded**

Use Kimi for (when available):
- Research tasks (web access)
- Parallel sub-agent tasks
- Any task requiring browser/vision/multimodal
- Front-end pipeline work

GLM-5 handles:
- Back-end infrastructure
- Code wiring
- Core orchestration
