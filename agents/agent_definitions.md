# VibePilot Agent Definitions
## Complete Agent Specifications for Plan-Ready System

**Version:** 1.1
**Date:** 2026-02-15
**Purpose:** Zero-gap agent definitions for Planner to create atomic build tasks

---

# AGENT 0: ORCHESTRATOR ("VIBES")

## Identity
**Name:** Orchestrator (aka "Vibes")
**Type:** Dispatcher/Monitor/Optimizer (NOT an executor)
**Default Model:** System (rule-based with AI enhancement)
**Role:** Human's direct interface to VibePilot

## Purpose
The brain of VibePilot. Dispatches tasks to models, monitors execution, tracks ROI, learns from results, recommends subscriptions. Only agent the human talks to directly.

## Key Principle
**Orchestrator does NOT execute. It routes.** It watches, dispatches, learns, and optimizes. Execution happens through Runners and Couriers.

## Skills
| Skill | Description |
|-------|-------------|
| `task_dispatch` | Assign tasks to appropriate models |
| `load_balancing` | Distribute across models based on capacity |
| `roi_tracking` | Calculate and report cost savings |
| `model_selection` | Pick best model for task type |
| `performance_learning` | Update model scores from results |
| `subscription_analysis` | Recommend keep/drop/add subscriptions |
| `human_communication` | Dashboard + daily summary + direct chat |

## Tools
| Tool | Usage |
|------|-------|
| `supabase_query` | Read tasks, models, runs, ROI data |
| `supabase_update` | Update task status, model scores |
| `runner_dispatch` | Send tasks to CLI/API runners |
| `courier_dispatch` | Send tasks to web platform couriers |
| `alert_send` | Notify human/supervisor of issues |
| `email_send` | Daily summary to human |

## Routing Priority (Configurable)

```yaml
routing_priority:
  # Phase 1: In-house (never goes to web platforms)
  internal_governance:
    - glm-5      # Consultant, Planner, Council, Supervisor
    - kimi-cli   # Backup for governance if GLM unavailable
  
  # Phase 2: Task execution (CLI subscriptions first)
  task_execution:
    - kimi-cli          # Primary task executor (subscription)
    - opencode          # You are here (subscription)
    
  # Phase 3: API (paid, use sparingly)
  api_fallback:
    - deepseek-api     # $2 credit, cache when possible
    - gemini-api       # Free tier, rate limited
    
  # Phase 4: LAST RESORT ONLY (dangerous)
  gateway_fallback:
    - openrouter       # WARNING: "Free" models often not available
                       # Can charge without warning
                       # Only use if explicitly configured
```

## OpenRouter Warning

```
⚠️  OPENROUTER IS LAST RESORT ONLY ⚠️

PROBLEM: "Free" models shown may not actually be available.
         When unavailable, it routes to PAID models without warning.
         Example: $5 charged for a model that just asked for more info.

RULES:
1. Never auto-route to OpenRouter
2. Only use when human explicitly approves
3. Always check model availability before dispatch
4. Set hard spending limit ($16 credit = hard cap)
5. Log all OpenRouter usage with model + cost

PREFERENCE: Use direct APIs (DeepSeek, Gemini) or CLI (Kimi, OpenCode)
           These have predictable pricing and no bait-and-switch.
```

## Platform Types

| Type | Examples | Cost | When Used |
|------|----------|------|-----------|
| **CLI Subscription** | Kimi, OpenCode | Fixed monthly | Primary execution |
| **Direct API** | DeepSeek, Gemini | Per-token | Fallback, research |
| **Gateway** | OpenRouter | Varies | LAST RESORT |
| **Web Platform** | ChatGPT, Claude web | Free tier | Via Courier only |
| **Hugging Face** | Various free models | Free | New releases, testing |

## Input Formats

### Input A: New Task Available
```json
{
  "event": "task_available",
  "task_id": "uuid",
  "task_number": "T001",
  "task_type": "code_generation" | "research" | "review" | "...",
  "requires_codebase": true | false,
  "estimated_context": 8000,
  "dependencies": ["T000"],
  "priority": 5,
  "routing_hints": {
    "suggested_model": "kimi-k2.5",
    "requires_cli": false
  }
}
```

### Input B: Task Completed
```json
{
  "event": "task_completed",
  "task_id": "uuid",
  "model_id": "kimi-k2.5",
  "platform": "kimi-cli",
  "success": true | false,
  "tokens_used": 15000,
  "duration_seconds": 45,
  "error": "string (if failed)"
}
```

### Input C: Human Query
```json
{
  "event": "human_query",
  "query": "What's the status on auth slice?",
  "context": "dashboard" | "email" | "voice"
}
```

## Output Formats

### Output A: Task Assignment
```json
{
  "task_id": "uuid",
  "assigned_to": {
    "model_id": "kimi-k2.5",
    "platform": "kimi-cli",
    "runner_type": "cli"
  },
  "reason": "Best performance for code generation tasks (95% success rate)",
  "fallback": {
    "model_id": "deepseek-chat",
    "platform": "deepseek-api"
  }
}
```

### Output B: ROI Report
```json
{
  "period": "today" | "week" | "month",
  "summary": {
    "tasks_completed": 47,
    "tasks_failed": 3,
    "total_tokens": 847000,
    "theoretical_cost": 127.40,
    "actual_cost": 23.50,
    "savings": 103.90,
    "roi_percentage": 81.6
  },
  "by_model": [
    {
      "model_id": "kimi-k2.5",
      "tasks": 22,
      "success_rate": 0.95,
      "avg_cost_per_task": 0.09,
      "recommendation": "keep"
    }
  ],
  "subscription_recommendations": [
    "Kimi CLI: 95% success, $0.09/task. Recommend keeping.",
    "DeepSeek API: 68% success, $0.14/task. Consider dropping."
  ]
}
```

### Output C: Human Response
```json
{
  "query": "What's the status on auth slice?",
  "response": "Auth slice has 12 tasks. 8 complete, 3 in progress, 1 awaiting review. ETA: 2 hours.",
  "details": {
    "project": "auth-rbac",
    "completion_pct": 67,
    "blocking_issues": []
  }
}
```

## Process

### Task Dispatch Process
```
1. RECEIVE task_available event

2. ANALYZE task:
   - Type (code, research, review, etc.)
   - Context requirements
   - Codebase access needed?
   - Priority
   - Routing hints

3. SELECT model:
   a. Check model availability (status = active?)
   b. Check rate limits/credits
   c. Check historical performance for this task type
   d. Apply routing priority rules
   
4. VERIFY selection:
   - Is model status = active?
   - Are we under rate limit?
   - Is credit available (if API)?
   - Is context sufficient?
   
5. ASSIGN task:
   - Update task.assigned_to
   - Create task_run record
   - Dispatch to runner/courier

6. MONITOR execution:
   - Watch for completion
   - Watch for timeout
   - Watch for Watcher interventions

7. RECORD result:
   - Update model performance scores
   - Calculate cost
   - Log for ROI

8. LEARN:
   - Update success rate by task type
   - Adjust future routing decisions
```

### Model Selection Logic
```python
def select_model(task):
    # 1. Is this governance (Planner, Council, Supervisor)?
    if task.role in GOVERNANCE_ROLES:
        return select_from(["glm-5", "kimi-cli"])
    
    # 2. Does it need codebase access?
    if task.requires_codebase:
        return select_from(["kimi-cli", "opencode"])
    
    # 3. Is it research?
    if task.type == "research":
        return select_from(["gemini-2.0-flash"])  # Free tier
    
    # 4. Standard task execution
    candidates = get_active_models()
    candidates = filter_by_context(candidates, task.estimated_context)
    candidates = filter_by_rate_limit(candidates)
    candidates = filter_by_credit(candidates)
    candidates = sort_by_performance(candidates, task.type)
    
    # 5. NEVER auto-select OpenRouter
    candidates = remove_gateway_models(candidates)
    
    return candidates[0] if candidates else queue_for_later(task)
```

### Platform Exhaustion Handling
```
IF all platforms at 80% capacity:
  1. Log warning
  2. Default to CLI subscriptions (unlimited)
  3. Reserve API credits for emergencies only
  4. Queue low-priority tasks

IF truly no capacity:
  1. Pause all new task assignments
  2. Alert human: "All models at capacity"
  3. Wait for capacity to free up
  4. Resume when available
```

## Learning Mechanism

```
AFTER EVERY TASK:

1. Record outcome:
   - Success or failure
   - Tokens used
   - Duration
   - Error (if any)

2. Update model stats:
   - success_rate_by_task_type[model][type] = rolling_average
   - avg_tokens_by_task_type[model][type] = rolling_average
   - avg_duration_by_task_type[model][type] = rolling_average

3. Calculate cost:
   - If subscription: allocation = subscription_cost / tasks_this_month
   - If API: actual cost from tokens × rate
   - theoretical_cost = tokens × best_api_rate

4. Update recommendation score:
   - score = (success_rate × 0.4) + (cost_efficiency × 0.3) + (speed × 0.3)
   - Used for subscription keep/drop decisions

5. Log for daily summary
```

## Daily Summary Email

Sent to human once per day:

```
VIBES DAILY REPORT - 2026-02-15

TASKS
  Completed: 47
  Failed: 3 (2 reassigned, 1 escalated)
  In Progress: 8
  
TOKENS & COST
  Tokens Used: 847K
  Theoretical Cost: $127.40
  Actual Cost: $23.50
  Savings: $103.90 (81.6% ROI)

MODEL PERFORMANCE
  Kimi CLI:     22 tasks, 95% success, $0.09/task ⭐
  DeepSeek API:  8 tasks, 75% success, $0.14/task
  Gemini:       12 tasks, 92% success, $0.00/task (free)

RECOMMENDATIONS
  ✓ Keep Kimi CLI subscription
  ⚠ DeepSeek success rate dropping - monitor
  ✓ Gemini free tier excellent for research

CREDITS REMAINING
  DeepSeek: $1.42
  OpenRouter: $16.00 (unused - last resort)
  Gemini: Free tier (847K/1M tokens today)

BLOCKERS
  None

[View Dashboard] [Reply to Vibes]
```

## Edge Cases

| Situation | Action |
|-----------|--------|
| Model goes offline mid-task | Reassign to backup, log, alert |
| All models paused | Alert human, queue tasks |
| Credit exhausted | Pause that model, route elsewhere |
| OpenRouter "free" model unavailable | Do NOT auto-switch to paid. Alert, wait, or use different platform. |
| Human asks routing question | Provide data-backed answer |
| Task stuck in loop | Watcher handles, Orchestrator notified |

## Constraints

- NEVER execute tasks directly
- NEVER auto-route to OpenRouter (gateway fallback only)
- NEVER send internal governance to web platforms
- ALWAYS track cost per task
- ALWAYS log model performance
- ALWAYS provide ROI visibility
- Human communication is ALWAYS available

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

## Consensus Process

```
Round 1 → Reviews → Feedback consolidated → Planner revises → Round 2
Round 2 → Reviews → Feedback consolidated → Planner revises → Round 3
Round 3 → Reviews → Feedback consolidated → Planner revises → Round 4
Round 4 → Reviews → Feedback consolidated → Planner revises → Round 5
Round 5 → Reviews → Feedback consolidated → Planner revises → Round 6 (MAX)

Round 6 no consensus?
  → Identify the SPECIFIC unresolvable issue
  → Route based on issue type:
    
    IF business/scope question:
      → Human decides (not a coder issue)
    
    IF PRD ambiguity:
      → Back to Consultant for clarification
    
    IF fundamental disagreement on approach:
      → Supervisor picks best option, documents reasoning, proceeds
      → Human notified but not blocked
```

**Human escalation is RARE.** Only for:
- Business direction changes
- Scope conflicts (what features, what priority)
- Visual/UX decisions (human taste, not code)
- Ethical/legal concerns

**Human is NEVER called for:**
- Technical implementation details
- Task breakdown issues
- Code architecture disputes
- Model routing decisions

## Edge Cases
| Situation | Action |
|-----------|--------|
| Unsure about intent | REVISION_NEEDED with question for Planner to clarify |
| Technical unknown | Flag as preventative issue, don't block |
| Disagree with confidence | Note it, don't block unless critical |
| Round 6 stalemate | Supervisor decides, documents, moves forward |

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

2. LOAD expected output from task packet

3. VALIDATE:
   - Output format correct?
   - All required deliverables present?
   - Acceptance criteria met?
   - Code quality gates passed?
     - No hardcoded secrets
     - No obvious bugs
     - Follows specified patterns
     - Error handling present

4. DECIDE:
   - PASS → Queue for testing
   - FAIL → Return to runner with specific feedback
   - REROUTE → Assign different model (if model was the issue)

5. UPDATE task status

6. OUTPUT decision

NOTE: "Extra files touched" is NOT a supervisor check.
If that happens, it's a PLAN/DESIGN problem:
- Task packet wasn't specific enough → Planner fix
- Runner went rogue → Don't use that runner
- Watcher catches it in real-time
```

## Quality Gates (Configurable)
```yaml
supervisor_quality_gates:
  check_secrets: true          # No hardcoded API keys, passwords
  check_error_handling: true   # Try/catch or equivalent present
  check_patterns: true         # Follows specified patterns
  check_test_coverage: true    # Tests written for new code
  check_documentation: false   # Only if required in task
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
**Type:** Background (real-time + scheduled monitoring)
**Default Model:** System (not AI, rule-based monitoring)
**Philosophy:** Proactive prevention over reactive cure

## Purpose
Prevent problems before they happen. Catch violations instantly. Protect context windows. Guardian of system integrity.

## Skills
| Skill | Description |
|-------|-------------|
| `file_guard` | Real-time monitoring of filesystem changes |
| `context_monitoring` | Track context usage, prevent overflow |
| `loop_detection` | Spot repeating errors, stuck tasks |
| `instant_intervention` | Stop violations immediately |
| `alerting` | Notify appropriate parties |

## Tools
| Tool | Usage |
|------|-------|
| `inotify` | Real-time filesystem monitoring |
| `supabase_query` | Read task_runs for patterns |
| `supabase_update` | Update task status |
| `process_signal` | Kill runaway processes |
| `git_operations` | Revert unauthorized changes |
| `alert_send` | Send notifications |

## Prevention Layers (Permaculture Design)

| Level | What | Who | Purpose |
|-------|------|-----|---------|
| **Layer 1** | Clear task packets with explicit allowed_files | Planner | Prevention at source |
| **Layer 2** | Real-time file guard | Watcher | Instant containment |
| **Layer 3** | Post-task review | Supervisor | Quality gate |

If Watcher catches something, Layer 1 failed. But Layer 2 prevents catastrophe.

## Monitoring Types

### Type A: Real-Time (Instant)
| Detection | Action | Why Instant |
|-----------|--------|-------------|
| File change outside allowed_files | STOP task, revert, alert supervisor | 60 seconds = too much damage |
| Unauthorized file access attempt | STOP task, log, alert | Prevention not cure |

### Type B: After Each Task
| Detection | Threshold | Action |
|-----------|-----------|--------|
| Context usage | > 70% of effective | WARN (log, continue) |
| Context usage | > 80% of effective | STOP assignments, start new session |
| Token waste pattern | Repetitive > 50% | Alert, log pattern |

### Type C: Scheduled (Every 30 seconds)
| Detection | Threshold | Action |
|-----------|-----------|--------|
| Same error | 3 consecutive | Kill, flag for different model |
| Output loop | Same output 2x | Kill, alert supervisor |
| Stuck context | No progress 10 min | Kill, suggest split |
| Task timeout | 30 minutes | Kill, log duration |
| High retry count | > 3 attempts | Escalate to supervisor |

## Thresholds (Configurable in vibepilot.yaml)
```yaml
watcher_thresholds:
  context_warn_pct: 70      # Warn at 70% context
  context_stop_pct: 80      # Stop at 80% context
  error_repeat_limit: 3     # Same error 3x triggers action
  stuck_minutes: 10         # No progress threshold
  timeout_minutes: 30       # Max task duration
  retry_limit: 3            # Max attempts before escalation
  repetitive_pct: 50        # Token waste threshold
```

## Input Format
```json
{
  "check_type": "realtime_file" | "post_task" | "scheduled",
  "event": {
    "type": "file_change" | "context_check" | "error_pattern",
    "task_id": "uuid",
    "details": { ... }
  }
}
```

## Output Format
```json
{
  "timestamp": "ISO8601",
  "event_type": "file_violation" | "context_warning" | "context_stop" | "loop_detected" | "timeout",
  "task_id": "uuid",
  "action_taken": "stopped" | "reverted" | "killed" | "warned" | "escalated",
  "details": {
    "violation": "string",
    "files_affected": ["path"],
    "context_pct": 75,
    "reverted": true | false
  },
  "alert_sent": {
    "recipient": "supervisor" | "orchestrator" | "human",
    "severity": "warning" | "intervention" | "critical",
    "message": "string"
  }
}
```

## Process

### Real-Time File Guard
```
ON FILE SYSTEM EVENT (create/modify/delete):

1. IDENTIFY which task (if any) triggered this

2. LOAD allowed_files from task packet

3. CHECK:
   IF file in allowed_files:
     → Allow, log, continue
   
   IF file NOT in allowed_files:
     → STOP task immediately
     → git checkout (revert changes)
     → Log violation with details
     → Alert supervisor
     → Flag task for review

4. OUTPUT intervention report
```

### Post-Task Context Check
```
AFTER TASK COMPLETION:

1. CALCULATE context_used / context_effective

2. CHECK thresholds (from config):
   
   IF pct >= context_stop_pct (80%):
     → STOP: No new assignments
     → Alert orchestrator: "Session at X%, start fresh"
     → Queue pending tasks for new session
     → Log
   
   ELIF pct >= context_warn_pct (70%):
     → WARN: Log only
     → Continue assignments
     → Note in daily summary

3. OUTPUT status
```

### Scheduled Loop/Stuck Detection (Every 30 seconds)
```
EVERY 30 SECONDS:

1. QUERY in_progress tasks

2. FOR each task:
   
   CHECK error patterns:
   - Get last N runs (configurable)
   - Same error >= error_repeat_limit? → KILL, reassign
   
   CHECK progress:
   - Last output > stuck_minutes? → KILL, suggest split
   
   CHECK duration:
   - Running > timeout_minutes? → KILL, log
   
   CHECK attempts:
   - Attempts >= retry_limit? → Escalate, don't auto-retry

3. LOG all interventions

4. SEND alerts as needed

5. OUTPUT report
```

## Context Window Management

When context hits 80%:
```
1. Orchestrator stops accepting new tasks
2. Current task completes (if in progress)
3. Alert: "Session context exhausted. Starting fresh session."
4. New session initialized
5. Pending tasks reassigned to new session
6. Old session archived (state preserved in Supabase)
```

This prevents:
- Degraded output quality from overstuffed context
- Hallucinations from context confusion
- Wasted tokens on poor reasoning

## Edge Cases
| Situation | Action |
|-----------|--------|
| File violation but change is good | Supervisor reviews, may approve manually |
| Context 80% mid-task | Let task finish, then stop |
| New session fails | Escalate, human may need to intervene |
| Multiple violations same task | Kill, flag model as problematic |

## Constraints
- NEVER intervene with Council or Supervisor decisions
- ONLY act on execution issues, not governance
- ALWAYS log interventions with full details
- REAL-TIME for file violations (not scheduled)
- CONFIGURABLE thresholds (no hardcoded values)

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
Autonomously research improvements to VibePilot itself. Find new models, platforms, tools, approaches. Return COMPLETE data for informed decisions.

## Skills
| Skill | Description |
|-------|-------------|
| `web_research` | Search AI papers, announcements, releases |
| `model_analysis` | Gather complete model specifications |
| `platform_analysis` | Gather complete platform details |
| `trend_analysis` | Identify emerging patterns |
| `cost_monitoring` | Track pricing changes |
| `user_sentiment` | Check LM Arena, forums for feedback |

## Tools
| Tool | Usage |
|------|-------|
| `web_search` | Search the web |
| `web_fetch` | Read articles, docs, pricing pages |
| `api_query` | Query model/platform APIs for specs |
| `file_write` | Write findings to considerations |

## Research Sources

| Source | What to Check | URL |
|--------|---------------|-----|
| **Official Docs** | Pricing, limits, specs | API docs for each provider |
| **Hugging Face** | New free models, beta releases | huggingface.co/models |
| **LM Arena** | User rankings, strengths/weaknesses | lmarena.ai |
| **Reddit/Twitter** | User experiences, issues | r/LocalLLaMA, etc |
| **GitHub** | New tools, CLI releases | github.com/trending |
| **Provider Blogs** | Announcements, changes | Official blogs |

## Input Format
```json
{
  "research_areas": [
    "new_ai_models",
    "new_platforms",
    "pricing_changes",
    "free_tier_availability",
    "user_rankings",
    "new_tools"
  ],
  "current_models": ["glm-5", "kimi-k2.5", "gemini-2.0-flash", "deepseek-chat"],
  "current_platforms": ["opencode", "kimi-cli", "deepseek-api", "google-ai", "openrouter"],
  "focus_on_free": true
}
```

## Output Format (COMPREHENSIVE)

### Complete Model Report
```json
{
  "date": "2026-02-15",
  "findings": {
    "new_models": [
      {
        "name": "model-id",
        "provider": "company",
        "source": "huggingface" | "official" | "lm_arena",
        
        "specs": {
          "context_limit": 128000,
          "context_effective": 100000,
          "max_output": 8192,
          "supports_streaming": true,
          "supports_tools": true,
          "supports_vision": false
        },
        
        "pricing": {
          "type": "free" | "subscription" | "pay_per_use",
          "cost_per_1m_input": 0.28,
          "cost_per_1m_output": 0.42,
          "cost_per_1m_cached": 0.028,
          "subscription_monthly": null,
          "free_tier_available": true,
          "free_tier_limits": {
            "requests_per_minute": 15,
            "requests_per_day": 1500,
            "tokens_per_day": 1000000
          }
        },
        
        "rate_limits": {
          "requests_per_minute": null,
          "requests_per_hour": null,
          "requests_per_day": null,
          "tokens_per_day": null,
          "note": "No hard limits" | "Specific limits"
        },
        
        "performance": {
          "lm_arena_rank": 15,
          "lm_arena_elo": 1250,
          "user_strengths": ["coding", "reasoning", "fast"],
          "user_weaknesses": ["hallucinations", "chinese"],
          "best_for": ["code_generation", "technical_docs"],
          "avoid_for": ["creative_writing", "multilingual"]
        },
        
        "access": {
          "api_available": true,
          "cli_available": false,
          "web_available": true,
          "huggingface_available": true,
          "openrouter_available": true
        },
        
        "relevance": "high" | "medium" | "low",
        "action_suggested": "add_to_registry" | "council_review" | "monitor" | "ignore",
        "notes": "Why this model matters"
      }
    ],
    
    "platform_updates": [
      {
        "platform": "openrouter",
        "type": "warning" | "pricing_change" | "new_feature" | "limit_change",
        "description": "Free models often unavailable, routes to paid without warning",
        "impact": "high" | "medium" | "low",
        "recommendation": "Last resort only, set hard spending limit",
        "source_url": "..."
      }
    ],
    
    "pricing_alerts": [
      {
        "model_or_platform": "deepseek-chat",
        "change_type": "price_increase" | "price_decrease" | "limit_change" | "new_tier",
        "old_value": "$0.14/1M",
        "new_value": "$0.28/1M",
        "effective_date": "2026-03-01",
        "impact_on_vibepilot": "Increases cost per task by 2x"
      }
    ],
    
    "free_opportunities": [
      {
        "source": "huggingface",
        "model": "new-free-model",
        "context": 128000,
        "notes": "New release, free during beta",
        "expires": "2026-04-01" | null
      }
    ],
    
    "user_sentiment": [
      {
        "model": "kimi-k2.5",
        "source": "lm_arena" | "reddit" | "twitter",
        "sentiment": "positive" | "mixed" | "negative",
        "key_feedback": ["Great for long context", "Agent swarm is powerful"],
        "issues_reported": ["Occasional timeouts"]
      }
    ]
  },
  
  "summary": "Brief overview of today's findings",
  "urgent_alerts": [
    "Pricing change on DeepSeek effective March 1"
  ]
}
```

## Process
```
1. RECEIVE research parameters

2. CHECK OFFICIAL SOURCES:
   For each current model/platform:
   - Fetch pricing page
   - Fetch API docs
   - Check for announcements
   - Note any changes

3. CHECK HUGGING FACE:
   - New models with "free" tag
   - Beta releases from major providers
   - Trending models this week
   - Note context limits and access method

4. CHECK LM ARENA:
   - Current rankings
   - User comments on strengths/weaknesses
   - New models entering rankings
   - Head-to-head comparisons

5. CHECK COMMUNITY:
   - Reddit r/LocalLLaMA for new releases
   - Twitter for announcements
   - GitHub for new tools/CLIs

6. COMPILE COMPLETE DATA:
   - Every finding has full specs
   - No partial information
   - Include source URLs
   - Note confidence level

7. WRITE to docs/UPDATE_CONSIDERATIONS.md

8. ALERT supervisor if:
   - Pricing change on current platform
   - New free tier found
   - Critical security issue
   - Major new model release

9. OUTPUT findings (JSON)
```

## Data Completeness Checklist

Every model finding MUST include:
- [ ] Full name and provider
- [ ] Context limit (and effective if different)
- [ ] Pricing (all tiers)
- [ ] Rate limits (all timeframes)
- [ ] Free tier availability and limits
- [ ] Access methods (API, CLI, web, HuggingFace)
- [ ] LM Arena ranking (if available)
- [ ] User-reported strengths (min 2)
- [ ] User-reported weaknesses (min 1)
- [ ] Source URLs

## Constraints
- Runs daily, not on-demand
- Only writes to considerations file
- Never makes changes directly
- MUST return complete data, not partial
- Escalates significant findings to Council
- Mark confidence level if uncertain

---

# AGENT 9: TASK RUNNERS

## Runner Architecture

All runners share the same interface (input/output format) but differ in:
- **Access**: CLI runners see codebase, API runners don't
- **Cost**: Subscription vs pay-per-use vs free tier
- **Context**: Different limits
- **Best for**: Different task types

```
┌─────────────────────────────────────────────────────────────────┐
│                      RUNNER TYPES                                │
│                                                                  │
│  CLI RUNNERS (Codebase Access)          API RUNNERS (No Access) │
│  ┌─────────────────────┐                ┌─────────────────────┐ │
│  │ Kimi CLI            │                │ DeepSeek API        │ │
│  │ - Subscription      │                │ - Pay per use       │ │
│  │ - 128K context      │                │ - 128K context      │ │
│  │ - Parallel/swarm    │                │ - Cache support     │ │
│  └─────────────────────┘                └─────────────────────┘ │
│  ┌─────────────────────┐                ┌─────────────────────┐ │
│  │ OpenCode (GLM-5)    │                │ Gemini API          │ │
│  │ - Subscription      │                │ - Free tier         │ │
│  │ - 128K context      │                │ - 128K context      │ │
│  │ - Governance role   │                │ - Rate limited      │ │
│  └─────────────────────┘                └─────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

---

## RUNNER 9A: Kimi CLI

### Identity
**Name:** Task Runner - Kimi CLI
**Type:** CLI Execution (subscription)
**Model:** Kimi K2.5
**Access:** Full codebase access
**Cost:** ~$10/month subscription (unlimited)

### Purpose
Primary parallel task executor. Handles complex tasks requiring codebase awareness. Swarm mode for wide parallel execution.

### Specs
| Attribute | Value |
|-----------|-------|
| Context limit | 128K |
| Context effective | 100K |
| Cost model | Subscription |
| Rate limits | None |
| Best for | Code generation, parallel tasks, long context |
| Weakness | English reasoning (Chinese-optimized) |

### Features
- **Agent Swarm**: Can dispatch to multiple sub-agents for parallel work
- **Web Browsing**: Can fetch web content if needed
- **Long Context**: Handles large codebases

### When to Route Here
- Task requires codebase access
- Task has 4+ code_context dependencies
- Parallel execution beneficial
- Complex multi-file changes

---

## RUNNER 9B: OpenCode (GLM-5)

### Identity
**Name:** Task Runner - OpenCode
**Type:** CLI Execution (subscription)
**Model:** GLM-5
**Access:** Full codebase access
**Cost:** Subscription (already paying)

### Purpose
Primary governance executor AND task executor. You are here now. Handles Consultant, Planner, Council, Supervisor roles AND can execute tasks.

### Specs
| Attribute | Value |
|-----------|-------|
| Context limit | 128K |
| Context effective | 100K |
| Cost model | Subscription |
| Rate limits | None |
| Best for | Governance, reasoning, planning, review, code |
| Weakness | None significant |

### Features
- **Strong Reasoning**: Excellent for planning and review
- **Code Generation**: Capable of production code
- **Governance Native**: Built for Consultant/Planner/Council/Supervisor roles
- **Bilingual**: Strong English and Chinese

### When to Route Here
- Governance tasks (Planner, Council, Supervisor)
- Tasks requiring deep reasoning
- Code review and validation
- Complex architectural decisions
- When Kimi unavailable

### Dual Role
```
GLM-5 serves TWO purposes:
1. GOVERNANCE: Consultant, Planner, Council, Supervisor
   - These NEVER go to web platforms
   - Always stay in-house
   
2. TASK EXECUTION: When Kimi busy or task suits GLM strengths
   - Can execute any task type
   - Fallback for Kimi
```

---

## RUNNER 9C: DeepSeek API

### Identity
**Name:** Task Runner - DeepSeek API
**Type:** API Execution (pay per use)
**Model:** DeepSeek-V3.2 (deepseek-chat)
**Access:** No codebase access (API only)
**Cost:** $2 credit available

### Purpose
Cost-effective API runner for tasks that don't need codebase access. Use prompt caching for 90% savings.

### Specs
| Attribute | Value |
|-----------|-------|
| Context limit | 128K |
| Context effective | 100K |
| Cost (cache miss) | $0.28/1M input, $0.42/1M output |
| Cost (cache hit) | $0.028/1M input (90% savings!) |
| Credit available | $2.00 |
| Rate limits | None (they serve all) |
| Best for | Code generation, technical docs, reasoning |
| Weakness | Less mature than GPT/Claude |

### Features
- **Prompt Caching**: 90% cost reduction on repeated context
- **No Rate Limits**: They serve all requests
- **Strong Coding**: Excellent for code tasks
- **Low Latency**: Fast responses

### When to Route Here
- Task has NO codebase dependencies
- Task packet is self-contained
- Want to preserve CLI subscription capacity
- Cache can be used (repeated context)

### Cost Optimization
```python
# ALWAYS use caching when possible
cached_context = [
    "System prompt",
    "VibePilot conventions",
    "Shared patterns"
]

# Only pay for new tokens
result = runner.execute(
    prompt="Create user service",
    cached_context=cached_context  # 90% cheaper!
)
```

### Budget Management
```
$2 credit = approximately:
- ~200 tasks (with caching)
- ~50 tasks (without caching)

PAUSE when credit < $0.50
ALERT human when credit < $0.20
```

---

## RUNNER 9D: Gemini API

### Identity
**Name:** Task Runner - Gemini API
**Type:** API Execution (free tier)
**Model:** Gemini 2.0 Flash
**Access:** No codebase access (API only)
**Cost:** FREE (within limits)

### Purpose
Free tier runner for research and simple tasks. Rate limited but costs nothing.

### Specs
| Attribute | Value |
|-----------|-------|
| Context limit | 128K (Flash) / 1M (Pro) |
| Context effective | 100K / 800K |
| Cost | $0 (free tier) |
| Rate limits | 15 RPM, 1500 RPD, 1M tokens/day |
| Best for | Research, multimodal, fast tasks |
| Weakness | Rate limits can block |

### Features
- **Free Tier**: $0 cost within limits
- **Multimodal**: Can process images
- **Fast**: Flash model is very quick
- **Research Native**: Excellent for web research

### When to Route Here
- Research tasks
- Tasks with no codebase needed
- Low priority (can wait if rate limited)
- Multimodal tasks (images)
- Preserving paid resources

### Rate Limit Management
```
Track daily usage:
- requests_today: N/1500
- tokens_today: N/1,000,000

WARN at 70%: "Gemini at 70% daily limit"
STOP at 90%: "Gemini exhausted, use other runners"
RESET: Midnight UTC
```

---

## RUNNER SELECTION LOGIC

```python
def select_runner(task):
    """
    Select appropriate runner based on task requirements.
    Priority: Preserve subscriptions, use free tier when possible.
    """
    
    # 1. Governance tasks → GLM-5 (always)
    if task.role in GOVERNANCE_ROLES:
        return "opencode"
    
    # 2. Needs codebase → CLI runners
    if task.requires_codebase:
        # Prefer Kimi for parallel/complex, GLM-5 for reasoning
        if task.parallel_execution:
            return "kimi-cli"
        return "opencode"
    
    # 3. Research → Gemini (free)
    if task.type == "research":
        if gemini_rate_limit_ok():
            return "gemini-api"
    
    # 4. Simple code task → DeepSeek (with cache)
    if task.type == "code" and not task.requires_codebase:
        if deepseek_credit > 0.50:
            return "deepseek-api"
    
    # 5. Fallback hierarchy
    if gemini_rate_limit_ok():
        return "gemini-api"
    if deepseek_credit > 0.20:
        return "deepseek-api"
    
    # 6. Last resort: CLI (preserves for complex tasks)
    return "kimi-cli"
```

---

## SHARED RUNNER INTERFACE

All runners implement the same interface:

### Input Format
```json
{
  "task_id": "uuid",
  "task_number": "T001",
  "prompt_packet": "string (complete instructions)",
  "dependencies": [
    {
      "task_id": "uuid",
      "type": "summary" | "code_context",
      "content": "string"
    }
  ],
  "branch_name": "task/T001-short-desc",
  "expected_output": { ... },
  "cached_context": ["string"]  // For API runners
}
```

### Output Format
```json
{
  "task_id": "uuid",
  "task_number": "T001",
  "model_name": "kimi-k2.5" | "glm-5" | "deepseek-chat" | "gemini-2.0-flash",
  "runner_type": "cli" | "api",
  "status": "success" | "failed" | "blocked" | "rate_limited",
  "result": {
    "files_created": ["path"],
    "files_modified": ["path"],
    "summary": "string",
    "tests_written": ["path"],
    "notes": "string"
  },
  "chat_url": null,  // Only couriers return chat URLs
  "error": "string (if failed)",
  "block_reason": "string (if blocked)",
  "tokens_used": 15000,
  "tokens_cached": 5000,  // For API runners with cache
  "cost": 0.007,  // Actual cost (0 for subscription/free)
  "execution_time_seconds": 45
}
```

### Context Isolation (All Runners)
| Sees | Does NOT See |
|------|--------------|
| Task packet | Other tasks |
| Dependency code (CLI only) | Full PRD |
| Relevant files | Unrelated codebase |
| Expected output | Other runners' outputs |

### Constraints (All Runners)
- NEVER see full PRD
- NEVER see other tasks
- NEVER make changes outside task scope
- Return result, NOT chat URL (that's courier)
- Report tokens and cost accurately

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
    "email": "vibesagentai@gmail.com",
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

# AGENT 11: MAINTENANCE

## Identity
**Name:** Maintenance Agent
**Type:** Autonomous (handles VibePilot self-updates)
**Default Model:** GLM-5
**Runs:** Daily review + on-demand for patches

## Purpose
Keep VibePilot healthy. Apply patches, update dependencies, implement safe improvements from System Research findings. Guardian of system stability.

## Skills
| Skill | Description |
|-------|-------------|
| `dependency_update` | Update packages safely |
| `patch_application` | Apply security patches |
| `config_update` | Modify configuration safely |
| `improvement_implementation` | Implement approved changes |
| `rollback` | Revert failed changes |

## Tools
| Tool | Usage |
|------|-------|
| `file_read` | Read code, config |
| `file_write` | Modify files |
| `terminal` | Run updates, tests |
| `git_operations` | Branch, commit, PR |
| `supabase_query` | Read system state |

## Input Format
```json
{
  "action": "daily_review" | "apply_patch" | "implement_improvement",
  
  "source": "system_research" | "security_advisory" | "dependency_update" | "config_change",
  
  "change_request": {
    "type": "dependency" | "security_patch" | "config" | "code_improvement",
    "description": "string",
    "files_affected": ["path"],
    "risk_level": "low" | "medium" | "high",
    "council_approval_required": true | false
  },
  
  "findings_from_research": {
    "date": "2026-02-15",
    "relevant_findings": [...]
  }
}
```

## Output Format
```json
{
  "action_taken": "none" | "applied" | "pending_council" | "pending_human",
  
  "changes_made": [
    {
      "file": "path/to/file",
      "change_type": "modified" | "created" | "deleted",
      "description": "string"
    }
  ],
  
  "tests_run": true,
  "tests_passed": true,
  
  "rollback_available": true,
  "rollback_branch": "maintenance/rollback-2026-02-15",
  
  "council_review_requested": false,
  "council_review_reason": null,
  
  "notes": "string"
}
```

## Process

### Daily Review
```
1. READ docs/UPDATE_CONSIDERATIONS.md

2. REVIEW findings from System Research:
   - New models → Add to registry if low risk
   - Pricing changes → Alert Orchestrator
   - Security advisories → Assess and patch
   - New tools → Flag for Council review
   
3. CHECK dependencies:
   - List outdated packages
   - Check for security vulnerabilities
   - Assess update risk
   
4. FOR low-risk updates:
   - Create maintenance branch
   - Apply update
   - Run tests
   - Commit if tests pass
   
5. FOR medium/high-risk updates:
   - Document change request
   - Request Council review
   - Wait for approval
   
6. REPORT status to Supervisor
```

### Change Risk Levels

| Risk | Examples | Action |
|------|----------|--------|
| **Low** | Minor version bump, config tweak | Apply directly, test, commit |
| **Medium** | Major version bump, new feature | Council review, then apply |
| **High** | Architecture change, breaking change | Council + human approval |

### What Maintenance Can Do Without Council

- Update dependencies (minor/patch versions)
- Fix typos, documentation
- Adjust config values in vibepilot.yaml
- Add new models to registry
- Apply security patches (after testing)
- Performance optimizations (safe)

### What Requires Council Review

- Major version dependency updates
- Architecture changes
- New features
- Breaking changes
- Changes to agent prompts
- Schema modifications

## Edge Cases

| Situation | Action |
|-----------|--------|
| Tests fail after update | Rollback, log, investigate |
| Security vulnerability critical | Patch immediately, notify human |
| Dependency conflict | Document, request human input |
| Unknown change impact | Request Council review |

## Constraints

- NEVER make breaking changes without approval
- ALWAYS create rollback branch before changes
- ALWAYS run tests before committing
- ALWAYS log all changes
- NEVER modify agent prompts without Council approval

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
