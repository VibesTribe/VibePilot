# VibePilot vs Cole Medin's PIV Loop: Intent & Quality Analysis
**Date:** 2026-04-30
**Purpose:** Compare our pipeline's intent/context passing vs Cole's approach, identify gaps

---

## How VibePilot Works Today (Verified From Code)

### The Chain
```
Consultant → PRD → Planner → Plan (tasks with packets) → Orchestrator routes → Executor runs → Supervisor reviews
```

### What Each Stage Produces

**Consultant** produces a PRD (stored... where? NOT in task_runs, NOT in tasks table. Needs verification.)

**Planner** produces a plan with tasks. Each task has:
- task_id, slice_id, phase, title, purpose, objectives
- deliverables (file paths)
- expected_output (files_created, files_modified, tests_pass)
- dependencies, routing_flag, confidence
- **Target Files** (exact paths for context injection)
- A **Prompt Packet** (the actual instructions the executor gets)
- An **Expected Output** field

**Executor** gets: prompt_packet + expected_output + injected file contents from Target Files

**Supervisor** gets during output review (line 931 of handlers_task.go):
```json
{
  "event": "task_review",
  "task_packet": {the full TaskPacket struct},
  "task_run": {run metadata},
  "output_files": {git diff of what changed},
  "task_instructions": {if exists},
  "task_number": {e.g. "P1-T001"}
}
```

TaskPacket struct contains:
- Prompt (the task instructions)
- TechSpec (optional)
- ExpectedOutput (what the planner said should be produced)
- Context (optional JSON)

### What SUPERVISOR HAS for review:
1. ✅ Task prompt/instructions (what executor was told to do)
2. ✅ Expected output (what planner said should result)
3. ✅ Actual output (git diff / output files)
4. ❌ **ORIGINAL PRD** (not passed to supervisor during output review)
5. ❌ **Planner's purpose/intent** (the "why" behind the task)
6. ❌ **Slice context** (which slice this belongs to, what it's supposed to achieve)

### The Gap
Supervisor can compare: "did the executor produce what the planner asked for?"
Supervisor CANNOT compare: "does this task output still align with the original PRD?"

This is exactly the drift detection Cole is talking about.

---

## How Cole Medin's PIV Loop Works

### The Chain
```
/prime (load context) → /create-prd (interactive extraction) → /plan (sub-agent research) → /implement (fresh context) → /validate (5-layer pyramid)
```

### Key Differences

**1. Context Reset Between Stages**
- Cole deliberately CLEARS context between planning and implementation.
- Implementation gets ONLY the plan document, nothing else.
- This forces the plan to be self-contained and complete.
- VibePilot doesn't do this -- executor gets prompt_packet but also inherits whatever context the system injects.

**2. PRD is the Anchor for Everything**
- The PRD is the SINGLE source of truth.
- Plan traces back to PRD requirements.
- Implementation is validated against the plan which traces to PRD.
- Final output is checked against the PRD for drift.
- In VibePilot, the PRD is referenced by the planner but NOT carried through to supervisor review.

**3. Interactive PRD Creation (/create-prd)**
- Before writing any PRD, the agent MUST ask clarifying questions.
- "The most dangerous thing is the model making assumptions."
- Forces ambiguity resolution BEFORE planning.
- VibePilot's consultant prompt presumably does this, but we should verify.

**4. Sub-Agents for Research Only**
- During /plan, sub-agents explore codebase in parallel.
- They return summaries but NEVER implement.
- Main context window stays clean.
- This is the "context forking" pattern -- similar to our courier model.

**5. Five-Layer Validation Pyramid**
```
Layer 5: Manual testing            ← You
Layer 4: Code review               ← You + AI
Layer 3: Integration / E2E         ← Agent + browser automation
Layer 2: Unit tests                ← Agent handles
Layer 1: Type checking + linting   ← Agent handles
```
- Goal: push line between 3 and 4 as far down as possible.
- VibePilot's supervisor does Layer 3-4. Our tester handles Layer 2.
- Cole automates Layers 1-3. Human only touches 4-5.

**6. PRD Template Structure**
From the repo, Cole's PRD includes:
- Feature description
- Requirements (functional + non-functional)
- Technical approach
- File structure (which files change)
- Validation strategy (how to know it's done)
- Edge cases and error handling

---

## Comparison: What We Can Learn

### Where VibePilot is STRONGER

1. **Automated routing** -- Cole routes manually (human picks internal vs web). VibePilot's orchestrator does this automatically based on routing flags and model performance history. This is genuinely more advanced.

2. **Slice isolation** -- Our planner's isolation rules are more rigorous than Cole's vertical slices. The "no cross-slice code dependencies" rule with explicit interfaces is architectural discipline that Cole doesn't formalize to the same degree.

3. **Self-learning from failures** -- Our orchestrator tracks model success rates and adapts routing. Cole's system is static -- it doesn't learn from past executions.

4. **Courier architecture** -- We can dispatch to free web AI tiers in parallel. Cole uses one agent at a time (Claude Code).

### Where Cole's Approach is STRONGER (Our Gaps)

1. **PRD as anchor throughout pipeline** -- This is the biggest gap. Our PRD is consumed by the planner and then DROPPED. The supervisor never sees it. Drift detection is impossible without it.

2. **Context discipline** -- Fresh context for implementation. Our executor inherits system context that may include irrelevant noise. Cole's approach forces plan completeness.

3. **Interactive PRD creation** -- "Questions before PRD" prevents assumption drift at the source. Need to verify our consultant does this.

4. **Validation pyramid** -- Clear separation of what's automated vs what needs human eyes. Our supervisor tries to do everything in one pass.

5. **Commandification** -- "If you type something more than twice, make it a command." Cole standardizes workflows into reusable commands. Our prompts are one-off documents, not parameterized commands.

---

## What Should We Actually Do?

### Option A: Add PRD to Supervisor Review Context (Minimal Fix)
- Store PRD content in the tasks table or pass it through the review handler
- Supervisor gets: PRD + task prompt + expected output + actual output
- Can now detect drift: "this output solves the task but contradicts PRD requirement FR-007"
- Risk: bloating supervisor context (PRDs can be long)
- Effort: Small DB change + handler update

### Option B: Add Planner Intent Field (The LogAct Proposal)
- Add `intent` to TaskPacket: "This task exists because PRD requires X. It should produce Y. Success means Z."
- This is a COMPRESSED version of the PRD-to-task traceability
- Supervisor compares actual output against intent, not the full PRD
- Risk: Intent quality depends on planner quality
- Effort: Medium (planner prompt change + TaskPacket struct + handler)

### Option C: Both (Belt and Suspenders)
- Planner writes intent per task (compressed traceability)
- Supervisor gets intent + can optionally pull full PRD for complex reviews
- Best coverage but most work

### My Recommendation: Option B First, Then A

Option B gives us 80% of the value for 20% of the work:
- The planner ALREADY writes purpose, objectives, and expected_output per task
- We just need to make the supervisor USE these explicitly
- The "intent" is already there in the planner's output -- it's just not being passed to the supervisor during output review

Looking at the code: the planner produces `purpose` and `objectives` per task. The supervisor gets `task_packet` which has `Prompt` and `ExpectedOutput`. But `purpose` and `objectives` are in the plan, not in the TaskPacket struct. They're lost between planning and review.

The fix isn't adding a new field. It's carrying forward what the planner already writes.

---

## About the PRD Quality Point

You're right: "For planner to do what you want, PRD has to be top notch."

Cole's approach: force the agent to ask questions BEFORE writing the PRD. "Reduce assumptions" is one of his 5 golden rules.

Our approach: consultant produces the PRD. Need to verify the consultant prompt enforces the same discipline. If it doesn't, the entire downstream chain is compromised -- garbage in, garbage out.

This is a SEPARATE improvement from the supervisor review gap. Both matter.

---

## Updated Plan for Item 2 (Intent/Context Gap)

### Immediate (before legacy ends):
1. Verify consultant prompt forces question-asking before PRD
2. Carry planner's `purpose` + `objectives` forward into TaskPacket or review context
3. Supervisor prompt already asks "Does output match what prompt requested?" -- verify it has the data to answer

### After knowledgebase is running:
4. Add PRD reference to supervisor review for complex tasks
5. Consider context reset between planning and execution (Cole's pattern)
6. Standardize prompts into parameterized commands (Cole's commandification)
