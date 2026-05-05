# PRD: LogAct Intent Logging and Safety Voter

**Status:** DRAFT  
**Created:** 2026-05-05  
**Scope:** Phase 1 (Intent capture + voter)  
**Source:** LogAct research (Meta), user discussion, JourneyKits analysis

---

## 1. Summary

Adds a pre-execution safety layer to VibePilot's task pipeline. Before a model executes a task, its intent is logged and verified by a cheap safety voter model. This prevents wasted tokens on off-the-rails execution, catches prompt injection, and creates an immutable record of what was supposed to happen vs what did happen.

### User Intent

Stop models from going off the rails before they spend 200k tokens doing the wrong thing. A watcher agent that checks "does this plan make sense" before execution, using the Gemini Visual Tester API key since it has spare capacity.

### Success Criteria

- Every task execution has an intent record in orchestrator_events before the model runs
- Safety voter reviews intent within 5 seconds using Gemini 3 Flash
- Voter blocks dangerous, misaligned, or wasteful executions
- Blocked executions include specific reason and suggested correction
- No increase in pipeline latency for approved tasks (voter runs in parallel with execution prep)

---

## 2. Architecture Decisions

### Tech Stack

| Layer | Technology | Rationale |
|-------|-----------|-----------|
| Intent capture | Go (handlers_task.go) | Same process as task execution, no new service |
| Safety voter | Gemini 3 Flash via API | Free tier, fast, different model than executor (cross-validation) |
| Intent storage | orchestrator_events table | Already append-only, just add new event_type |
| Configuration | system.json + agents.json | Existing config surface, no new files |

### System Design

Current flow:
task_dispatched -> model executes -> output_received -> supervisor reviews

New flow:
task_dispatched -> intent_captured -> voter_check -> (approved) -> model executes -> output_received
                                        -> (blocked)  -> task_blocked event -> analyst or retry

The voter is a lightweight Gemini 3 Flash call that receives:
1. The task prompt (what we asked)
2. The plan context (what the PRD says)
3. The model's intent summary (what it plans to do)

It returns: approved, blocked, or needs_review with a reason.

### Patterns and Conventions

- Voter uses a DIFFERENT model than the executor (cross-validation principle from LogAct)
- Intent is logged BEFORE execution starts, never after
- Voter timeout is 10 seconds. If voter times out, proceed with execution (fail-open, not fail-closed)
- Blocked tasks go to analyst for diagnosis, not straight back to executor

---

## 3. Requirements

### P1 -- Must Have (MVP)

#### FR-001: Intent Capture
**Priority:** P1  
**Traceability:** LogAct research item 1 "Intent logging before execution"  
**Description:** Before a task model begins execution, the governor captures what the model intends to do. This is a structured record logged to orchestrator_events.

**Scenarios:**
- GIVEN a task in "dispatched" state, WHEN the model is about to be called, THEN an "intent_captured" event is written to orchestrator_events with the task prompt, plan context, and model ID
- GIVEN the intent capture fails, WHEN the error is transient, THEN log a warning and proceed with execution (intent capture must not block the pipeline)

**Acceptance:** orchestrator_events table has "intent_captured" entries with task_id, model_id, and intent details in the details JSONB field.

---

#### FR-002: Safety Voter
**Priority:** P1  
**Traceability:** LogAct research item 2 "Safety voter as cheap model cross-checks intent"  
**Description:** A cheap model (Gemini 3 Flash) reviews the task intent before execution proceeds. It checks for: scope alignment (is the model about to do something way bigger than asked), security risks (is the model about to touch secrets or auth), and prompt injection indicators.

**Scenarios:**
- GIVEN a task to "add a button to the dashboard", WHEN the model's intent says "rewrite the entire authentication module", THEN the voter blocks execution with reason "scope_mismatch"
- GIVEN a task to "fix a CSS color", WHEN the model's intent says "fix a CSS color in Header.tsx", THEN the voter approves within 5 seconds
- GIVEN the voter API call times out after 10 seconds, WHEN no response received, THEN proceed with execution and log "voter_timeout" event
- GIVEN the voter returns "blocked", WHEN the reason is "security_risk", THEN route directly to analyst, do not retry with same model

**Acceptance:** Governor logs show voter check for each task. Blocked tasks appear in orchestrator_events with voter reasoning.

---

#### FR-003: Voter Configuration
**Priority:** P1  
**Traceability:** System must be configurable without code changes  
**Description:** Safety voter behavior is configurable via system.json.

**Scenarios:**
- GIVEN system.json has "safety_voter.enabled": false, WHEN a task dispatches, THEN skip voter check entirely
- GIVEN system.json has "safety_voter.timeout_seconds": 10, WHEN voter takes 11 seconds, THEN proceed with execution

**Acceptance:** New system.json section for safety_voter with enabled, model, timeout_seconds, connector fields.

---

### P2 -- Should Have

#### FR-004: Voter Metrics
**Priority:** P2  
**Traceability:** Need to measure voter effectiveness  
**Description:** Track voter decisions in task_runs or a new table for analysis.

**Scenarios:**
- GIVEN 50 tasks have run through the voter, WHEN querying voter stats, THEN see approval_rate, block_rate, and top block reasons

**Acceptance:** Dashboard can show voter statistics.

---

### P3 -- Nice to Have

#### FR-005: Voter Learning
**Priority:** P3  
**Traceability:** Self-improve harness pattern from JourneyKits  
**Description:** Voter learns from supervisor outcomes. If voter approved but supervisor later failed, voter adjusts sensitivity.

---

## 4. Data Contracts

### Entities

```
orchestrator_events (existing table, new event types):
  event_type: "intent_captured" | "voter_check"
  task_id: TEXT (existing)
  model_id: TEXT (existing)
  details: JSONB
    For intent_captured:
      task_prompt: TEXT (what was asked)
      plan_context: TEXT (relevant plan excerpt)
      model_id: TEXT (which model will execute)
      intent_summary: TEXT (what model plans to do)
    For voter_check:
      voter_model: TEXT (which model voted)
      decision: "approved" | "blocked" | "timeout"
      reason: TEXT (why blocked, null if approved)
      scope_aligned: BOOLEAN
      security_risk: BOOLEAN
      confidence: FLOAT
```

### Configuration Schema

```
system.json addition:
  safety_voter:
    enabled: BOOLEAN [true]
    connector_id: STRING ["gemini-api-visual"]
    model: STRING ["gemini-3-flash-preview"]
    timeout_seconds: INTEGER [10]
    fail_open: BOOLEAN [true]  (proceed on timeout/error)
    block_categories: ARRAY ["scope_mismatch", "security_risk", "prompt_injection", "resource_waste"]
```

---

## 5. Implementation Notes

### File Organization

- handlers_task.go: Add intent capture in executeTask() and executeCourierTask() before the model call
- New file: governor/internal/runtime/safety_voter.go -- voter logic isolated from task execution
- system.json: Add safety_voter config section
- connectors.json: gemini-api-visual connector already exists, reuse it
- RPC allowlist: No new RPCs needed, voter writes to orchestrator_events directly

### Dependencies

- Gemini 3 Flash API via existing gemini-api-visual connector
- GEMINI_VISUAL_TESTER_KEY from vault (already stored)
- Existing orchestrator_events table (no schema changes)

### Voter Prompt

The voter receives a compact prompt:
- Task prompt (first 2000 chars)
- PRD excerpt (first 1000 chars)
- Model intent summary (generated from task packet, first 500 chars)

Voter returns JSON:
{"decision": "approved"|"blocked", "reason": "...", "scope_aligned": bool, "security_risk": bool}

### Environment

- No new env vars
- Uses existing GEMINI_VISUAL_TESTER_KEY from vault
- No new database tables or migrations

---

## 6. Testing Strategy

| Type | Coverage Target | Tools |
|------|----------------|-------|
| Unit | 90% | Go testing, voter logic |
| Integration | 80% | Test with real Gemini API |
| E2E | 60% | Full pipeline run with voter enabled |

### Critical Test Scenarios

1. Task with reasonable scope gets approved by voter
2. Task with scope mismatch (asking for button, model intends module rewrite) gets blocked
3. Voter timeout falls through to execution (fail-open)
4. Voter disabled in config skips check entirely
5. Blocked task routes to analyst for diagnosis

---

## 7. Quality Checklist

- [x] Every FR has a scenario with GIVEN/WHEN/THEN
- [x] Every FR traces to a user intent or research finding
- [x] All data contracts are fully typed
- [x] Error handling defined for every external call (voter timeout, API error)
- [x] Edge cases addressed (voter down, timeout, misconfigured)
- [x] Testing strategy defined
- [x] File organization follows existing conventions
- [x] No unresolved NEEDS CLARIFICATION markers
- [x] Architecture decisions have rationale
- [x] Maintenance story clear (voter is a Go routine, no external service)

---

## 8. Open Questions

None. All decisions made:
- Gemini Visual Tester key for voter (confirmed)
- Fail-open on timeout (confirmed)
- No new tables, use existing orchestrator_events (confirmed)
- Voter runs synchronously before execution, not in parallel (confirmed -- simpler, and 5s voter is acceptable latency)
