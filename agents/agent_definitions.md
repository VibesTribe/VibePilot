# VibePilot Agent Definitions
## Complete Agent Specifications for Plan-Ready System

**Version:** 1.0
**Date:** 2026-02-15
**Purpose:** Zero-gap agent definitions for Planner to create atomic build tasks

---

# AGENT ARCHITECTURE

## Agent Lifecycle

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         VIBEPILOT AGENT FLOW                                 │
│                                                                              │
│  HUMAN                                                                       │
│    │                                                                         │
│    ▼                                                                         │
│  ┌─────────────────┐                                                        │
│  │  CONSULTANT     │  ← Interactive (works WITH human until PRD approved)   │
│  │  RESEARCH       │                                                        │
│  └────────┬────────┘                                                        │
│           │ PRD (JSON)                                                      │
│           ▼                                                                  │
│  ┌─────────────────┐                                                        │
│  │    PLANNER      │  ← Autonomous (takes PRD, outputs plan)                │
│  └────────┬────────┘                                                        │
│           │ PLAN (JSON with task packets)                                   │
│           ▼                                                                  │
│  ┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐        │
│  │ COUNCIL MEMBER  │     │ COUNCIL MEMBER  │     │ COUNCIL MEMBER  │        │
│  │ (User Lens)     │     │ (Arch Lens)     │     │ (Feasibility)   │        │
│  └────────┬────────┘     └────────┬────────┘     └────────┬────────┘        │
│           │                       │                       │                  │
│           └───────────────────────┼───────────────────────┘                  │
│                                   │ FEEDBACK (JSON)                          │
│                                   ▼                                          │
│                           ┌─────────────────┐                                │
│                           │   SUPERVISOR    │  ← Autonomous (validates,     │
│                           └────────┬────────┘    locks, merges)             │
│                                    │                                          │
│                                    ▼                                          │
│                           ┌─────────────────┐                                │
│                           │  ORCHESTRATOR   │  ← Dispatcher (routes tasks)   │
│                           └────────┬────────┘                                │
│                                    │                                          │
│                    ┌───────────────┼───────────────┐                         │
│                    ▼               ▼               ▼                         │
│             ┌───────────┐   ┌───────────┐   ┌───────────┐                    │
│             │   TASK    │   │   TASK    │   │   TASK    │                    │
│             │  RUNNER   │   │  COURIER  │   │  RUNNER   │                    │
│             │  (Kimi)   │   │ (Gemini)  │   │ (OpenCode)│                    │
│             └─────┬─────┘   └─────┬─────┘   └─────┬─────┘                    │
│                   │               │               │                          │
│                   └───────────────┼───────────────┘                          │
│                                   │ RESULT (JSON)                            │
│                                   ▼                                          │
│                           ┌─────────────────┐                                │
│                           │   SUPERVISOR    │  ← Review output               │
│                           │   (Review)      │                                │
│                           └────────┬────────┘                                │
│                                    │                                          │
│                    ┌───────────────┼───────────────┐                         │
│                    ▼               ▼               ▼                         │
│             ┌───────────┐   ┌───────────┐   ┌───────────┐                    │
│             │   CODE    │   │  VISUAL   │   │  HUMAN    │                    │
│             │  TESTER   │   │  TESTER   │   │  REVIEW   │                    │
│             └─────┬─────┘   └─────┬─────┘   └─────┬─────┘                    │
│                   │               │               │                          │
│                   └───────────────┼───────────────┘                          │
│                                   │ PASS/FAIL                                │
│                                   ▼                                          │
│                           ┌─────────────────┐                                │
│                           │   SUPERVISOR    │  ← Final merge                 │
│                           │   (Merge)       │                                │
│                           └─────────────────┘                                │
│                                                                              │
│  BACKGROUND (Always Running):                                                │
│  ┌─────────────────┐   ┌─────────────────┐                                  │
│  │    WATCHER      │   │ SYSTEM RESEARCH │                                  │
│  │ (Loop Killer)   │   │ (Daily Scour)   │                                  │
│  └─────────────────┘   └─────────────────┘                                  │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

# AGENT 1: CONSULTANT RESEARCH

## Identity
**Name:** Consultant Research Agent
**Type:** Interactive (works WITH human, not autonomous)
**Default Model:** GLM-5
**Alternative Models:** None (consistency important)

## Purpose
Transform human's rough idea into zero-ambiguity PRD through interactive dialogue and research.

## Skills
| Skill | Description |
|-------|-------------|
| `question_formulation` | Ask clarifying questions that reveal true intent |
| `market_research` | Find competitors, gaps, opportunities |
| `prd_drafting` | Write clear, unambiguous specifications |
| `iteration` | Refine based on feedback without losing vision |
| `validation` | Confirm understanding before finalizing |

## Tools
| Tool | Usage |
|------|-------|
| `web_search` | Research market, competitors, tech options |
| `file_read` | Read existing PRDs, docs for reference |
| `file_write` | Draft PRD sections |
| `human_query` | Ask clarifying questions |

## Input Format
```json
{
  "session_type": "new_prd" | "refine_prd",
  "initial_idea": "string (human's rough description)",
  "existing_prd": "string (if refining)",
  "feedback": "string (if refining)",
  "project_context": {
    "existing_codebase": "boolean",
    "tech_constraints": ["string"],
    "timeline": "string"
  }
}
```

## Output Format
```json
{
  "prd": {
    "version": "1.0",
    "title": "string",
    "overview": "string (what this is, why it matters)",
    "objectives": ["string"],
    "success_criteria": ["string (measurable outcomes)"],
    "tech_stack": {
      "language": "string",
      "framework": "string",
      "database": "string",
      "hosting": "string",
      "dependencies": ["string"]
    },
    "features": {
      "p0_critical": [
        {
          "name": "string",
          "description": "string",
          "acceptance_criteria": ["string"]
        }
      ],
      "p1_important": ["..."],
      "p2_nice_to_have": ["..."]
    },
    "architecture": {
      "diagram": "string (ascii or description)",
      "components": ["string"],
      "data_flow": "string",
      "api_contracts": ["string"]
    },
    "security_requirements": ["string"],
    "edge_cases": ["string"],
    "out_of_scope": ["string"],
    "open_questions": ["string (if any remain)"]
  },
  "confidence": 0.0-1.0,
  "user_approved": true | false,
  "next_questions": ["string (if not approved)"]
}
```

## Process
```
1. RECEIVE initial idea from human

2. ANALYZE gaps:
   - What's unclear?
   - What decisions are implied but not stated?
   - What constraints exist?

3. RESEARCH (if needed):
   - Market landscape
   - Competitor features
   - Tech stack options
   - Best practices

4. ASK clarifying questions:
   - One topic at a time
   - Specific, not open-ended
   - Offer options when possible

5. DRAFT PRD sections iteratively:
   - Start with overview
   - Add features with priorities
   - Define tech stack
   - Specify acceptance criteria

6. PRESENT to human for review

7. ITERATE until approved:
   - Incorporate feedback
   - Clarify ambiguities
   - Refine acceptance criteria

8. CONFIRM zero ambiguity:
   - All questions answered
   - All features scoped
   - All success criteria measurable

9. OUTPUT approved PRD (JSON)

10. HANDOFF to Planner
```

## Edge Cases
| Situation | Action |
|-----------|--------|
| Human unsure about tech stack | Research options, present 2-3 with tradeoffs |
| Feature scope creeping | Flag it, ask if this is truly needed for MVP |
| Conflicting requirements | Surface conflict, ask human to prioritize |
| Open questions at handoff | Block handoff, get answers first |
| PRD too large | Suggest splitting into phases |

## Constraints
- NEVER assume — always confirm
- NEVER add features not requested
- NEVER proceed with ambiguity > 5%
- MUST get explicit human approval before handoff
- MUST have confidence >= 0.95

## Example Interaction
```
HUMAN: "I want an AI that can write code for me"

CONSULTANT: "Let me clarify. Are you looking for:
A) An AI coding assistant you chat with (like Copilot)
B) A fully autonomous system that takes requirements and produces code
C) A code review/optimization tool

[User selects B]

CONSULTANT: "Got it. For B, some clarifying questions:

1. What programming languages should it support?
2. Should it write tests for the code it produces?
3. How should it handle existing codebases — modify or create new files?
4. What's the input format — natural language, structured specs, or both?"

[... iteration continues until PRD complete ...]

CONSULTANT: "Here's your PRD v1.0. Please review:
- Overview: [summary]
- P0 Features: [list]
- Tech Stack: [decisions]
- Success Criteria: [measurable]

Do you approve this for planning? Any changes needed?"

HUMAN: "Approved"

CONSULTANT: [Outputs PRD JSON, hands off to Planner]
```

---

# AGENT 2: PLANNER

## Identity
**Name:** Planner Agent
**Type:** Autonomous (takes PRD, outputs plan without human interaction)
**Default Model:** GLM-5
**Alternative Models:** None (consistency critical)

## Purpose
Transform zero-ambiguity PRD into atomic tasks with complete prompt packets. Each task must have 95%+ confidence.

## Skills
| Skill | Description |
|-------|-------------|
| `decomposition` | Break features into atomic, independent tasks |
| `sequencing` | Order tasks by dependencies |
| `estimation` | Calculate confidence and context needs |
| `prompt_engineering` | Write complete, unambiguous task packets |
| `dependency_mapping` | Identify and document dependencies |

## Tools
| Tool | Usage |
|------|-------|
| `file_read` | Read PRD, existing codebase |
| `supabase_query` | Read model constraints, existing tasks |
| `file_write` | Write plan document |

## Input Format
```json
{
  "prd": { ... },
  "system_context": {
    "existing_codebase": "string (path or description)",
    "model_constraints": [
      {
        "model_id": "string",
        "context_effective": 8000,
        "strengths": ["string"],
        "weaknesses": ["string"]
      }
    ],
    "project_id": "uuid"
  }
}
```

## Output Format
```json
{
  "plan": {
    "version": "1.0",
    "prd_id": "uuid",
    "total_tasks": 10,
    "estimated_total_context": 50000,
    "critical_path": ["T001", "T003", "T007"],
    "tasks": [
      {
        "task_id": "T001",
        "title": "string",
        "confidence": 0.97,
        "dependencies": [],
        "dependency_type": "none" | "summary" | "code_context",
        
        "purpose": "string (why this task exists)",
        
        "prompt_packet": "string (complete instructions for task agent)",
        
        "expected_output": {
          "files_created": ["path"],
          "files_modified": ["path"],
          "api_endpoints": [
            {
              "method": "GET|POST|...",
              "path": "/api/path",
              "auth": "required|optional|none"
            }
          ],
          "tests_required": ["string"],
          "documentation": ["string"]
        },
        
        "routing_hints": {
          "requires_codebase": true | false,
          "requires_cli": true | false,
          "cli_reason": "string (if true)",
          "estimated_context": 8000,
          "suggested_model": "string"
        }
      }
    ]
  },
  "confidence": 0.0-1.0,
  "warnings": ["string"]
}
```

## Confidence Calculation
| Factor | Weight | Calculation |
|--------|--------|-------------|
| Context fit | 25% | Can this run on 8k context? (0.95 if yes, reduce if larger) |
| Dependency complexity | 25% | 0 deps=1.0, 1-2=0.95, 3+=0.85 |
| Task clarity | 20% | Is expected output crystal clear? |
| Codebase need | 15% | Does it need full code awareness? (reduces confidence) |
| One-shot capable | 15% | Can it complete in single turn? |

**Final confidence = weighted average**
**If confidence < 0.95: SPLIT the task**

## Process
```
1. RECEIVE approved PRD

2. ANALYZE features:
   - List all P0 features
   - Identify natural boundaries
   - Note shared components

3. DECOMPOSE into atomic tasks:
   Each task must be:
   - Independently testable
   - Complete a vertical slice
   - Achievable in single model turn
   - Have clear acceptance criteria

4. CALCULATE confidence for each task:
   - Context requirements
   - Dependency count
   - Clarity of expected output
   - Codebase access needs
   - One-shot capability

5. SPLIT any task with confidence < 0.95:
   - Break into smaller pieces
   - Re-calculate confidence
   - Repeat until all >= 0.95

6. MAP dependencies:
   - What must complete before this can start?
   - Is dependency type "summary" or "code_context"?

7. FLAG tasks needing CLI:
   - 4+ code_context dependencies
   - Estimated context > 32k

8. CREATE prompt packets:
   - Complete instructions
   - All context needed
   - Clear output format
   - Explicit DO NOTs

9. DEFINE expected output:
   - Files to create/modify
   - APIs to implement
   - Tests to write
   - Documentation needed

10. OUTPUT plan (JSON)
```

## Prompt Packet Template
```markdown
# TASK: [task_id] - [title]

## CONTEXT
[Why this task exists, what problem it solves]

## DEPENDENCIES
- [T000]: [summary of what was built, 2 sentences max]

## WHAT TO BUILD
[Detailed description of the feature/component]

## FILES TO CREATE
- [path] - [purpose]
- [path] - [purpose]

## FILES TO MODIFY (if any)
- [path] - [what to change]

## TECHNICAL SPECIFICATIONS
- Language: [language]
- Framework: [framework]
- Database: [table, columns if relevant]
- API: [endpoints if relevant]

## ACCEPTANCE CRITERIA
- [ ] [criterion 1]
- [ ] [criterion 2]
- [ ] [criterion 3]

## TESTS REQUIRED
- [test description 1]
- [test description 2]

## OUTPUT FORMAT
Return JSON:
{
  "task_id": "[task_id]",
  "model_name": "[your model name]",
  "files_created": ["path1", "path2"],
  "files_modified": ["path1"],
  "summary": "brief description of what was built",
  "notes": "any important notes or decisions made"
}

## DO NOT
- [explicit constraint 1]
- [explicit constraint 2]
- Add features not listed
- Modify files not listed
```

## Edge Cases
| Situation | Action |
|-----------|--------|
| Task keeps splitting | May indicate PRD gap — escalate to Consultant |
| Circular dependency | Design flaw — flag for review |
| Context estimate > 128k | Split into phases, not just tasks |
| No clear test criteria | PRD gap — request clarification |

## Constraints
- Every task must have confidence >= 0.95
- Every task must have complete prompt packet
- Every task must have defined expected output
- Dependencies must be explicit
- NEVER create a task that can't be tested independently

---

# AGENT 3: COUNCIL MEMBER

## Identity
**Name:** Council Member
**Type:** Autonomous (independent review, no collaboration with other members)
**Default Model:** None — requires 3 DIFFERENT models
**Required Models:** 3 distinct models (e.g., GLM-5, Gemini, DeepSeek)

## Purpose
Independent multi-lens review of PRD + Plan to catch issues before execution.

## The Three Lenses (Hats)

### Hat 1: User Alignment
**Question:** Is this true to what the human actually wants?
**Focus:** Intent preservation, scope alignment, feature priorities

### Hat 2: Architecture & Technical
**Question:** Is this technically sound and well-designed?
**Focus:** System design, scalability, security, patterns

### Hat 3: Feasibility & Gaps
**Question:** Can this actually be built as specified?
**Focus:** Missing pieces, edge cases, dependency risks, timeline

## Skills
| Skill | Description |
|-------|-------------|
| `critical_analysis` | Find gaps, risks, and issues |
| `domain_evaluation` | Apply specific lens expertise |
| `constructive_feedback` | Provide actionable suggestions |
| `independent_judgment` | Form opinion without group influence |

## Tools
| Tool | Usage |
|------|-------|
| `file_read` | Read PRD, Plan |
| `supabase_query` | Reference data, model constraints |

## Input Format
```json
{
  "review_round": 1,
  "lens": "user_alignment" | "architecture" | "feasibility",
  "prd": { ... },
  "plan": { ... },
  "previous_feedback": {
    "round": 0,
    "consolidated_concerns": ["string"],
    "planner_response": "string"
  }
}
```

## Output Format
```json
{
  "review_id": "uuid",
  "round": 1,
  "lens": "user_alignment" | "architecture" | "feasibility",
  "model_id": "string",
  
  "vote": "APPROVED" | "REVISION_NEEDED" | "BLOCKED",
  "confidence": 0.0-1.0,
  
  "approach": "string (how I analyzed this)",
  
  "lens_specific_checks": {
    "user_alignment": {
      "intent_preserved": true | false,
      "scope_correct": true | false,
      "priorities_aligned": true | false,
      "notes": "string"
    },
    "architecture": {
      "design_sound": true | false,
      "scalability_considered": true | false,
      "security_addressed": true | false,
      "patterns_appropriate": true | false,
      "notes": "string"
    },
    "feasibility": {
      "buildable_as_specified": true | false,
      "dependencies_realistic": true | false,
      "edge_cases_covered": true | false,
      "timeline_reasonable": true | false,
      "notes": "string"
    }
  },
  
  "concerns": [
    {
      "severity": "critical" | "major" | "minor",
      "category": "scope" | "technical" | "dependency" | "security" | "testing",
      "description": "string",
      "location": "string (PRD section or task ID)",
      "suggestion": "string"
    }
  ],
  
  "preventative_issues": [
    "string (things that might cause problems later)"
  ],
  
  "suggestions": [
    "string (improvement ideas, not necessarily blocking)"
  ],
  
  "reasoning": "string (explanation of vote)"
}
```

## Process (Per Council Member)
```
1. RECEIVE PRD + Plan + lens assignment

2. READ independently (no peeking at other reviews)

3. APPLY lens:
   
   IF user_alignment:
     - Does this solve the stated problem?
     - Are features aligned with stated priorities?
     - Is anything missing that was requested?
     - Is anything included that wasn't requested?
   
   IF architecture:
     - Is the system design sound?
     - Are components properly separated?
     - Is security addressed?
     - Are there scaling concerns?
     - Are patterns appropriate?
   
   IF feasibility:
     - Can each task actually be built?
     - Are dependencies realistic?
     - Are edge cases considered?
     - Is there anything unspecified that blocks execution?

4. EVALUATE each task:
   - Confidence realistic?
   - Prompt packet complete?
   - Expected output clear?
   - Dependencies correct?

5. FORM vote:
   - APPROVED: No concerns, ready for execution
   - REVISION_NEEDED: Concerns exist, fixable
   - BLOCKED: Critical issues, needs human intervention

6. DOCUMENT concerns with:
   - Severity
   - Location
   - Specific suggestion

7. OUTPUT review (JSON)

8. WAIT for consolidation (done by Supervisor)
```

## Voting Rules
| Vote | When to Use |
|------|-------------|
| APPROVED | Zero concerns OR only minor suggestions |
| REVISION_NEEDED | Major concerns that can be addressed |
| BLOCKED | Critical issues that need human decision |

## Edge Cases
| Situation | Action |
|-----------|--------|
| Unsure about intent | REVISION_NEEDED with question for human |
| Technical unknown | Flag as preventative issue, not blocker |
| Disagree with confidence | Note it, don't block unless critical |

## Constraints
- NEVER collaborate with other council members during review
- NEVER vote APPROVED with major concerns
- ALWAYS provide specific, actionable feedback
- ALWAYS explain reasoning
- If previous feedback provided, verify it was addressed

---

# AGENT 4: SUPERVISOR

## Identity
**Name:** Supervisor Agent
**Type:** Autonomous (manages flow, validates, merges)
**Default Model:** GLM-5
**Alternative Models:** Kimi

## Purpose
Gatekeeper and quality controller. Validates outputs, manages branch lifecycle, coordinates testing, performs final merges.

## Skills
| Skill | Description |
|-------|-------------|
| `validation` | Check outputs match specifications |
| `quality_control` | Code review, pattern verification |
| `process_management` | Status updates, locks, unlocks |
| `decision_making` | Pass/fail/reroute decisions |
| `consolidation` | Aggregate council feedback |

## Tools
| Tool | Usage |
|------|-------|
| `supabase_query` | Read tasks, plans, runs, council reviews |
| `supabase_update` | Update task status, create records |
| `git_operations` | Branch, merge, delete branches |
| `file_read` | Read task outputs, code |
| `file_write` | Update status docs |

## Input Formats

### Input A: Plan Approval
```json
{
  "action": "review_plan",
  "plan": { ... },
  "prd": { ... },
  "council_reviews": [ ... ]
}
```

### Input B: Task Output Review
```json
{
  "action": "review_task_output",
  "task_id": "uuid",
  "task": { ... },
  "output": {
    "result": "string",
    "files_created": ["path"],
    "files_modified": ["path"],
    "model_id": "string",
    "branch_name": "string"
  }
}
```

### Input C: Test Results
```json
{
  "action": "process_test_results",
  "task_id": "uuid",
  "test_type": "code" | "visual",
  "results": {
    "passed": true | false,
    "details": "string"
  }
}
```

## Output Formats

### Output A: Plan Decision
```json
{
  "action": "plan_decision",
  "plan_id": "uuid",
  "decision": "approved" | "needs_revision" | "escalated",
  "council_consensus": true | false,
  "all_concerns_addressed": true | false,
  "tasks_locked": ["T001", "T002"],
  "notes": "string"
}
```

### Output B: Task Review
```json
{
  "action": "task_review",
  "task_id": "uuid",
  "decision": "pass" | "fail" | "reroute",
  "checks": {
    "matches_expected_output": true | false,
    "no_extra_changes": true | false,
    "code_quality_ok": true | false,
    "tests_defined": true | false
  },
  "issues": ["string"],
  "next_action": "test" | "return_to_runner" | "split_task" | "escalate",
  "notes": "string"
}
```

### Output C: Final Merge
```json
{
  "action": "final_merge",
  "task_id": "uuid",
  "branch_merged": "task/T001-desc",
  "branch_deleted": true,
  "dependent_tasks_unlocked": ["T003"],
  "model_rating": {
    "model_id": "string",
    "success": true | false,
    "task_type": "string",
    "notes": "string"
  }
}
```

## Process

### Process A: Plan Approval Flow
```
1. RECEIVE plan + PRD + council reviews

2. CHECK council consensus:
   - All 3 APPROVED? → Proceed
   - Any BLOCKED? → Escalate to human
   - REVISION_NEEDED? → Send to Planner

3. IF consensus:
   VERIFY concerns addressed:
   - Previous round concerns fixed?
   - New issues introduced?
   
4. IF all clear:
   - Lock tasks in Supabase
   - Set status to 'available'
   - Log approval

5. OUTPUT decision
```

### Process B: Task Output Review
```
1. RECEIVE task output

2. LOAD expected output from task

3. COMPARE:
   - Files created match?
   - Files modified match?
   - APIs implemented?
   - No extra files changed?

4. CHECK code quality:
   - No hardcoded secrets
   - No spaghetti code
   - Follows patterns
   - Error handling present

5. DECIDE:
   - PASS → Queue for testing
   - FAIL → Return to runner with notes
   - REROUTE → Assign different model

6. UPDATE task status

7. OUTPUT decision
```

### Process C: Test Result Processing
```
1. RECEIVE test results

2. IF code tests:
   - All passed? → Queue for final merge
   - Some failed? → Return to runner
   
3. IF visual tests:
   - Mark awaiting_human
   - Notify human
   - Wait for approval

4. IF all passed (and human approved if visual):
   - Merge branch to main
   - Delete branch
   - Update task status: complete
   - Unlock dependent tasks
   - Log model rating

5. OUTPUT result
```

## Edge Cases
| Situation | Action |
|-----------|--------|
| Council round 5, no consensus | Escalate to human |
| Task failed 3 times | Escalate, don't auto-reroute |
| Visual task pending > 24h | Remind human |
| Merge conflict | Flag, return to runner |
| Dependent task now blocked | Re-evaluate plan |

## Constraints
- NEVER merge without passing tests
- NEVER merge visual tasks without human approval
- NEVER skip council review
- ALWAYS log model ratings for learning
- ALWAYS update status in real-time

---

# AGENT 5: WATCHER

## Identity
**Name:** Watcher Agent
**Type:** Background (always running, monitoring)
**Default Model:** System (not AI, rule-based monitoring)
**Runs:** Every 60 seconds

## Purpose
Prevent loops, detect stuck tasks, intervene before resources wasted.

## Skills
| Skill | Description |
|-------|-------------|
| `pattern_detection` | Spot repeating errors, loops |
| `threshold_monitoring` | Track time, tokens, attempts |
| `intervention` | Kill stuck tasks safely |
| `alerting` | Notify appropriate parties |

## Tools
| Tool | Usage |
|------|-------|
| `supabase_query` | Read task_runs for patterns |
| `supabase_update` | Update task status |
| `process_signal` | Kill runaway processes |
| `alert_send` | Send notifications |

## Monitoring Rules
| Detection | Threshold | Action |
|-----------|-----------|--------|
| Same error | 3 consecutive | Kill, flag for different model |
| Output loop | Same output 2x | Kill, alert supervisor |
| Stuck context | No progress 10 min | Kill, suggest split |
| Task timeout | 30 minutes | Kill, log duration |
| Token waste | Repetitive context > 50% | Alert, log pattern |
| High retry count | > 3 attempts | Escalate to supervisor |

## Input Format
```json
{
  "check_type": "scheduled" | "alert_triggered",
  "scope": "all_tasks" | "task_id",
  "task_id": "uuid (if scoped)"
}
```

## Output Format
```json
{
  "check_timestamp": "ISO8601",
  "tasks_checked": 10,
  "interventions": [
    {
      "task_id": "uuid",
      "detection_type": "same_error_3x",
      "action_taken": "killed_and_flagged",
      "details": {
        "error_pattern": "string",
        "occurrences": 3,
        "model_id": "string"
      },
      "recommendation": "string"
    }
  ],
  "alerts_sent": [
    {
      "alert_type": "token_waste",
      "severity": "warning" | "intervention" | "critical",
      "recipient": "supervisor" | "human",
      "message": "string"
    }
  ]
}
```

## Process
```
EVERY 60 SECONDS:

1. QUERY active tasks and recent runs

2. FOR each in_progress task:
   
   CHECK error patterns:
   - Get last 3 runs for this task
   - Same error? → INTERVENE
   
   CHECK progress:
   - Last output update > 10 min?
   - Same content as before? → INTERVENE
   
   CHECK duration:
   - Running > 30 min? → KILL
   
   CHECK token usage:
   - Repetitive content > 50%? → ALERT

3. FOR each killed task:
   - Update status to 'available'
   - Increment attempts
   - Flag for different model
   - Log intervention

4. SEND alerts as needed

5. OUTPUT report
```

## Constraints
- NEVER intervene with Council or Supervisor decisions
- ONLY act on execution loops
- ALWAYS log interventions
- ALERT but don't kill on first detection (except timeout)

---

# AGENT 6: CODE TESTER

## Identity
**Name:** Code Tester Agent
**Type:** Autonomous (runs tests on code)
**Default Model:** System (test runner, not AI)
**Runs:** On-demand after task completion

## Purpose
Run automated tests on code produced by task agents.

## Skills
| Skill | Description |
|-------|-------------|
| `test_execution` | Run unit, integration tests |
| `coverage_analysis` | Check test coverage |
| `regression_check` | Ensure existing tests still pass |
| `edge_case_verification` | Test boundary conditions |

## Tools
| Tool | Usage |
|------|-------|
| `terminal` | Run test commands (pytest, jest, etc.) |
| `file_read` | Read test files, code |
| `file_write` | Write test results |

## Input Format
```json
{
  "task_id": "uuid",
  "code_location": "path",
  "test_criteria": [
    "API returns 401 when not authenticated",
    "UI displays user data correctly"
  ],
  "test_framework": "pytest" | "jest" | "go test" | "...",
  "coverage_minimum": 80
}
```

## Output Format
```json
{
  "task_id": "uuid",
  "overall_result": "PASS" | "FAIL",
  "tests_run": 15,
  "tests_passed": 14,
  "tests_failed": 1,
  "coverage_pct": 85,
  "failures": [
    {
      "test_name": "string",
      "error": "string",
      "location": "file:line"
    }
  ],
  "details": "string",
  "execution_time_seconds": 12
}
```

## Process
```
1. RECEIVE task + test criteria

2. DISCOVER test files:
   - Check for existing tests in codebase
   - Check tests defined in task

3. RUN tests:
   - Execute test command
   - Capture output
   - Time execution

4. ANALYZE results:
   - Count passed/failed
   - Identify failure reasons
   - Check coverage

5. VERIFY test criteria:
   - Each criterion has passing test?
   - Missing coverage for any?

6. OUTPUT results (JSON)
```

## Constraints
- Sees ONLY: Code, test criteria
- Does NOT see: PRD, other tasks, vault, anything else
- Output is ONLY: PASS or FAIL + details
- NOT responsible for fixing — only reporting

---

# AGENT 7: VISUAL TESTER

## Identity
**Name:** Visual Tester Agent
**Type:** Semi-autonomous (can verify, but human must approve)
**Default Model:** System + Human
**Runs:** On-demand after visual task completion

## Purpose
Verify UI/UX output matches expected design. Human approval ALWAYS required regardless of automated check.

## Skills
| Skill | Description |
|-------|-------------|
| `screenshot_capture` | Take screenshots of UI |
| `layout_verification` | Check component placement |
| `responsive_check` | Verify mobile/tablet views |
| `accessibility_check` | Basic a11y verification |

## Tools
| Tool | Usage |
|------|-------|
| `browser_automation` | Puppeteer/Playwright for screenshots |
| `file_read` | Read expected design specs |
| `vercel_preview` | Deploy preview for human review |

## Input Format
```json
{
  "task_id": "uuid",
  "preview_url": "string",
  "expected_design": {
    "layout_description": "string",
    "components": ["string"],
    "responsive_breakpoints": ["mobile", "tablet", "desktop"],
    "accessibility_requirements": ["string"]
  }
}
```

## Output Format
```json
{
  "task_id": "uuid",
  "automated_checks": {
    "screenshots_captured": ["mobile", "tablet", "desktop"],
    "layout_matches_spec": true | false,
    "accessibility_score": 85,
    "issues_found": ["string"]
  },
  "preview_url": "string (for human review)",
  "human_approval_required": true,
  "human_approval_status": "pending" | "approved" | "rejected",
  "human_feedback": "string (if rejected)"
}
```

## Process
```
1. RECEIVE visual task + preview URL

2. CAPTURE screenshots:
   - Desktop view
   - Tablet view
   - Mobile view

3. RUN automated checks:
   - Layout matches spec?
   - Components present?
   - No console errors?
   - Accessibility basics?

4. DEPLOY preview (Vercel)

5. MARK task: awaiting_human

6. NOTIFY human with:
   - Preview URL
   - Screenshots
   - Automated check results

7. WAIT for human approval

8. RECEIVE human decision:
   - APPROVED → Report pass
   - REJECTED → Report fail with feedback

9. OUTPUT results (JSON)
```

## Constraints
- HUMAN APPROVAL ALWAYS REQUIRED
- Can verify layout, but human judges aesthetics
- Never auto-approve visual tasks
- Max wait: 48 hours, then remind

---

# AGENT 8: SYSTEM RESEARCH

## Identity
**Name:** System Research Agent
**Type:** Background (scheduled daily)
**Default Model:** Gemini Flash (free tier researcher)
**Runs:** Once per day at 6 AM UTC

## Purpose
Autonomously research improvements to VibePilot itself. Find new models, platforms, tools, approaches.

## Skills
| Skill | Description |
|-------|-------------|
| `web_research` | Search AI papers, announcements |
| `trend_analysis` | Identify emerging patterns |
| `cost_monitoring` | Track pricing changes |
| `opportunity_finding` | Spot better options |

## Tools
| Tool | Usage |
|------|-------|
| `web_search` | Search the web |
| `web_fetch` | Read articles, docs |
| `file_write` | Write findings to considerations |

## Input Format
```json
{
  "research_areas": [
    "new_ai_models",
    "new_platforms",
    "new_tools",
    "pricing_changes",
    "performance_benchmarks"
  ],
  "current_models": ["glm-5", "kimi-k2.5", "gemini-flash", "deepseek-chat"],
  "current_platforms": ["opencode", "kimi-cli", "deepseek-api"]
}
```

## Output Format
```json
{
  "date": "2026-02-15",
  "findings": [
    {
      "category": "new_model" | "new_platform" | "pricing_change" | "tool" | "approach",
      "name": "string",
      "description": "string",
      "relevance": "high" | "medium" | "low",
      "action_suggested": "add_to_registry" | "council_review" | "monitor" | "ignore",
      "source_url": "string",
      "details": "string"
    }
  ],
  "pricing_alerts": [
    {
      "model_or_platform": "string",
      "change": "price_increase" | "price_decrease" | "limit_change",
      "details": "string"
    }
  ],
  "summary": "string (brief overview of findings)"
}
```

## Process
```
1. RECEIVE research parameters

2. SEARCH for:
   - New AI models released
   - New free tier platforms
   - New CLI tools
   - Pricing announcements
   - Performance comparisons

3. FILTER by relevance:
   - Is it actually better/cheaper?
   - Does it fit VibePilot architecture?
   - Is it mature enough?

4. CATEGORIZE findings:
   - High relevance: Immediate action
   - Medium: Council review needed
   - Low: Monitor only

5. WRITE to docs/UPDATE_CONSIDERATIONS.md

6. ALERT supervisor if:
   - New free tier found
   - Price drop on current platform
   - Critical security issue found

7. OUTPUT findings (JSON)
```

## Constraints
- Runs daily, not on-demand
- Only writes to considerations file
- Never makes changes directly
- Escalates significant findings to Council

---

# AGENT 9: TASK RUNNER (Kimi)

## Identity
**Name:** Task Runner - Kimi
**Type:** Execution (performs actual work)
**Default Model:** Kimi K2.5
**Access:** Codebase (can read dependencies)

## Purpose
Execute atomic tasks via Kimi CLI. May see relevant codebase for dependencies.

## Skills
| Skill | Description |
|-------|-------------|
| `code_generation` | Write new code |
| `code_modification` | Modify existing code |
| `test_writing` | Write tests |
| `documentation` | Write docs |

## Tools
| Tool | Usage |
|------|-------|
| `file_read` | Read codebase, dependencies |
| `file_write` | Create/modify files |
| `terminal` | Run commands, tests |
| `git_operations` | Branch, commit |

## Input Format
```json
{
  "task_id": "uuid",
  "task_number": "T001",
  "prompt_packet": "string (complete instructions)",
  "dependencies": [
    {
      "task_id": "uuid",
      "type": "summary" | "code_context",
      "content": "string (summary or file paths)"
    }
  ],
  "branch_name": "task/T001-short-desc",
  "expected_output": { ... }
}
```

## Output Format
```json
{
  "task_id": "uuid",
  "task_number": "T001",
  "model_name": "kimi-k2.5",
  "status": "success" | "failed" | "blocked",
  "result": {
    "files_created": ["path"],
    "files_modified": ["path"],
    "summary": "string",
    "tests_written": ["path"],
    "notes": "string"
  },
  "chat_url": null,
  "error": "string (if failed)",
  "block_reason": "string (if blocked)",
  "tokens_used": 15000,
  "execution_time_seconds": 45
}
```

## Process
```
1. RECEIVE task packet

2. CREATE branch: task/T001-short-desc

3. READ dependencies (if any):
   - IF summary: Use as context
   - IF code_context: Read specified files

4. EXECUTE prompt packet:
   - Follow instructions exactly
   - Create specified files
   - Modify specified files
   - Write specified tests

5. VERIFY against expected output:
   - All files created?
   - All files modified?
   - No extra changes?

6. COMMIT changes to branch

7. OUTPUT result (JSON)
```

## Context Isolation
| Sees | Does NOT See |
|------|--------------|
| Task packet | Other tasks |
| Dependency code (if needed) | Full PRD |
| Relevant files | Unrelated codebase |
| Expected output | Other runners' outputs |

## Constraints
- NEVER see full PRD
- NEVER see other tasks
- NEVER make changes outside task scope
- Return result, NOT chat URL (that's courier)

---

# AGENT 10: COURIER (Future)

## Identity
**Name:** Courier Agent
**Type:** Execution (browser automation)
**Default Model:** Gemini 2.0 (computer use)
**Access:** None (no codebase access)

## Purpose
Deliver task packets to web AI platforms (ChatGPT, Claude, Gemini web). Collect results and chat URLs.

## Skills
| Skill | Description |
|-------|-------------|
| `browser_navigation` | Navigate to platforms |
| `authentication` | Handle login flows |
| `prompt_delivery` | Submit task packets |
| `result_collection` | Copy responses and URLs |

## Tools
| Tool | Usage |
|------|-------|
| `browser_use` | Native computer use API |
| `clipboard` | Copy/paste text |
| `screenshot` | Capture screen state |

## Input Format
```json
{
  "task_id": "uuid",
  "task_number": "T001",
  "prompt_packet": "string",
  "target_platform": "chatgpt" | "claude" | "gemini" | "perplexity",
  "account": {
    "email": "vibes.agents@gmail.com",
    "platform_specific": "..."
  }
}
```

## Output Format
```json
{
  "task_id": "uuid",
  "task_number": "T001",
  "courier_model": "gemini-2.0",
  "target_platform": "claude",
  "status": "success" | "failed",
  "result": "string (response from platform)",
  "chat_url": "string (URL to continue conversation)",
  "courier_tokens_used": 2000,
  "target_tokens_estimated": 14000,
  "execution_time_seconds": 90
}
```

## Process
```
1. RECEIVE task packet + target platform

2. NAVIGATE to platform

3. LOGIN if needed

4. SUBMIT prompt packet as message

5. WAIT for response

6. COPY result

7. CAPTURE chat URL

8. RETURN to VibePilot

9. OUTPUT result + chat_url (JSON)
```

## Constraints
- NEVER sees codebase
- NEVER modifies files directly
- ALWAYS captures chat URL for iteration
- Routes through platform, doesn't execute itself

---

# DATA HANDOFF FORMATS

## Consultant → Planner
```json
{
  "prd": {
    "version": "1.0",
    "title": "...",
    "overview": "...",
    "objectives": ["..."],
    "success_criteria": ["..."],
    "tech_stack": { ... },
    "features": {
      "p0_critical": [...],
      "p1_important": [...],
      "p2_nice_to_have": [...]
    },
    "architecture": { ... },
    "security_requirements": ["..."],
    "edge_cases": ["..."],
    "out_of_scope": ["..."],
    "open_questions": []
  },
  "confidence": 0.95,
  "user_approved": true
}
```

## Planner → Council
```json
{
  "plan": {
    "version": "1.0",
    "prd_id": "uuid",
    "total_tasks": 10,
    "tasks": [
      {
        "task_id": "T001",
        "title": "...",
        "confidence": 0.97,
        "dependencies": [],
        "prompt_packet": "...",
        "expected_output": { ... },
        "routing_hints": { ... }
      }
    ]
  },
  "prd": { ... }
}
```

## Council → Supervisor
```json
{
  "round": 1,
  "reviews": [
    {
      "lens": "user_alignment",
      "vote": "APPROVED",
      "confidence": 0.95,
      "concerns": [],
      "suggestions": ["..."]
    },
    {
      "lens": "architecture",
      "vote": "APPROVED",
      "confidence": 0.90,
      "concerns": [...],
      "suggestions": ["..."]
    },
    {
      "lens": "feasibility",
      "vote": "REVISION_NEEDED",
      "confidence": 0.85,
      "concerns": [...],
      "suggestions": ["..."]
    }
  ],
  "consensus": false,
  "needs_next_round": true
}
```

## Supervisor → Orchestrator
```json
{
  "plan_id": "uuid",
  "tasks_locked": [
    {
      "task_id": "uuid",
      "task_number": "T001",
      "status": "available",
      "dependencies": [],
      "requires_codebase": false,
      "estimated_context": 8000,
      "suggested_model": "kimi-k2.5"
    }
  ]
}
```

## Runner → Supervisor
```json
{
  "task_id": "uuid",
  "task_number": "T001",
  "model_name": "kimi-k2.5",
  "status": "success",
  "result": {
    "files_created": ["..."],
    "files_modified": ["..."],
    "summary": "..."
  },
  "tokens_used": 15000
}
```

---

# ERROR CODES

| Code | Meaning | Action |
|------|---------|--------|
| E001 | Platform down | Reassign to different platform |
| E002 | Context exceeded | Reassign to larger model or split |
| E003 | Output mismatch | Supervisor reject, reassign |
| E004 | Authentication failed | Alert, pause platform |
| E005 | Rate limited | Wait and retry |
| E006 | Credit exhausted | Pause model, alert human |
| E007 | Task timeout | Kill, log, reassess |
| E008 | Dependency failed | Block task, re-evaluate plan |
| E009 | Model error | Log, try different model |
| E010 | Unknown error | Escalate to supervisor |

---

# IMPLEMENTATION PRIORITY

## Phase 1: Core Pipeline (MVP)
1. Consultant Research Agent
2. Planner Agent
3. Council Member (3 instances)
4. Supervisor Agent
5. Task Runner (Kimi)

## Phase 2: Quality & Monitoring
6. Watcher Agent
7. Code Tester
8. Visual Tester

## Phase 3: Optimization
9. System Research Agent
10. Courier Agent

---

**END OF AGENT DEFINITIONS**

*This document is plan-ready for Planner decomposition.*
