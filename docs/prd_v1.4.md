# VibePilot v1.4 — Comprehensive PRD
## Sovereign AI Execution Engine

**Version:** 1.4
**Status:** Production Foundation
**Date:** 2026-02-15
**Audience:** All models, all sessions, operators

---

# 0. Quick Start (For New Sessions)

**Read these files first:**
1. `CURRENT_STATE.md` — Where we are, what's working
2. This file (`docs/prd_v1.4.md`) — Complete system specification
3. `CHANGELOG.md` — Recent changes

**Before any action:**
- Read context. Think. Ask questions. Then decide.
- Never react. Never "fix it" without understanding.

---

# 1. What VibePilot Is

A sovereign AI execution engine that turns ideas into production code through coordinated multi-agent execution.

**Core Principles:**
- **Modular & Swappable** — Every component replaceable without cascade
- **State External** — All state in Supabase, all code in GitHub
- **Vendor Agnostic** — Models change, system persists
- **Governance First** — Council approval before execution
- **ROI Tracked** — Every task has measurable value

**What It Is NOT:**
- A chatbot or conversational assistant
- Dependent on any single vendor
- A replacement for human oversight on visual/UI decisions

---

# 2. The Complete Pipeline

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           HUMAN INPUT                                    │
│                     (Idea, PRD, or Feature Request)                      │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                          CONSULTANT AGENT                                │
│                                                                          │
│  Input: Rough idea or feature request                                    │
│  Output: Zero-ambiguity PRD with:                                        │
│          - Full tech specs                                               │
│          - User intent confirmed                                         │
│          - All questions answered                                        │
│          - Success criteria defined                                      │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                            PLANNER AGENT                                 │
│                                                                          │
│  Input: Approved PRD                                                     │
│  Output: Full plan with atomic tasks, each:                              │
│          - 95%+ confidence (or split further)                            │
│          - Dependencies mapped                                           │
│          - Prompt packet ready                                           │
│          - Expected output defined                                       │
│          - Codebase awareness flagged if needed                          │
│                                                                          │
│  Confidence Factors:                                                     │
│          - Real-world context limits (not benchmarks)                    │
│          - Dependency count and type (summary vs code_context)           │
│          - Task complexity                                                │
│          - Model constraints (free tier limits, request caps)            │
│                                                                          │
│  If confidence < 95%: SPLIT the task                                     │
│  If task has 4+ deps requiring code awareness: FLAG for CLI              │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                     COUNCIL REVIEW (3 Independent Models)                │
│                                                                          │
│  Reviews: FULL PRD + FULL PLAN as single documents                       │
│                                                                          │
│  Three lenses:                                                           │
│          1. User Alignment — True to user intent?                        │
│          2. Architecture — Technically sound?                            │
│          3. Feasibility — Can this actually be built?                    │
│                                                                          │
│  Process: Iterative consensus (3-4 rounds typical)                       │
│  Result: APPROVED or revision needed                                     │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                          SUPERVISOR AGENT                                │
│                                                                          │
│  Final check before orchestrator:                                        │
│          - Council feedback addressed                                    │
│          - Plan meets PRD                                                │
│          - All tasks have complete prompt packets                        │
│                                                                          │
│  On approval: Tasks → Supabase, status = locked/available                │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                           ORCHESTRATOR                                   │
│                                                                          │
│  NOT an executor. A dispatcher/monitor:                                  │
│          - Watches task queue                                            │
│          - Assigns tasks to available models                             │
│          - Manages rate limits, timeouts, credits                        │
│          - Learns from success/failure ratings                           │
│          - Routes based on:                                              │
│            - Task requirements (codebase access, context needs)          │
│            - Model availability and limits                               │
│            - Historical performance per task type                        │
│                                                                          │
│  Routing Priority:                                                       │
│          1. Web platforms (couriers) — until 80% limit                   │
│          2. CLI subscriptions (Kimi, OpenCode) — default fallback        │
│          3. API (DeepSeek) — last resort, cost                           │
│                                                                          │
│  Learning Mechanism:                                                     │
│          - Sees every task result (success/failure)                      │
│          - Tracks tokens in/out per model per task type                  │
│          - Tracks what failed and why                                    │
│          - Analyzes patterns across hundreds of tasks                    │
│          - Updates model performance scores                              │
│          - Recommends subscriptions worth renewing                       │
│                                                                          │
│  If ALL platforms at 80%:                                               │
│          - Default to CLI subscriptions                                  │
│          - Reserve API credits for crucial/emergency only                │
│          - If truly nothing: pause all tasks, alert human                │
│                                                                          │
│  Human Communication:                                                    │
│          - Only agent human talks to directly                            │
│          - Dashboard + daily summary email                               │
│          - "Hey Vibes, what's the status?" → live response               │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                         TASK EXECUTION                                   │
│                                                                          │
│  Two paths:                                                              │
│                                                                          │
│  ┌─────────────────────┐         ┌─────────────────────┐                │
│  │       RUNNERS       │         │      COURIERS       │                │
│  │  (Task Agents)      │         │  (Web Delivery)     │                │
│  │                     │         │                     │                │
│  │  - Kimi CLI         │         │  - ChatGPT web      │                │
│  │  - OpenCode         │         │  - Claude web       │                │
│  │  - DeepSeek API     │         │  - Gemini web       │                │
│  │  - Gemini API       │         │  - Perplexity       │                │
│  │                     │         │                     │                │
│  │  Direct execution   │         │  Navigate, submit,  │                │
│  │  May see codebase   │         │  collect result     │                │
│  │  (for dependencies) │         │  + chat URL         │                │
│  │  NO chat URL        │         │  NO codebase access │                │
│  └─────────────────────┘         └─────────────────────┘                │
│                                                                          │
│  Runner task return:                                                     │
│          - result + task_id + model_name                                 │
│                                                                          │
│  Courier task return:                                                    │
│          - result + chat_url + task_id + model_name                      │
│                                                                          │
│  Each task:                                                              │
│          1. Create branch: task/T001-short-desc                          │
│          2. Execute prompt packet                                        │
│          3. Return result (runner) or result + chat_url (courier)        │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                        SUPERVISOR REVIEW                                 │
│                                                                          │
│  Checks output against prompt packet:                                    │
│          - Matches expected output?                                      │
│          - No extra/unexpected changes?                                  │
│          - Code clean, no spaghetti?                                     │
│          - Branch ready for merge?                                       │
│                                                                          │
│  If PASS → Testing                                                       │
│  If FAIL → Return with notes (reassign, split, or different model)      │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                           TESTING                                        │
│                                                                          │
│  Two types:                                                              │
│                                                                          │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │  CODE TESTING (Automated)                                        │    │
│  │  - Unit tests                                                    │    │
│  │  - Integration tests                                             │    │
│  │  - Edge cases                                                    │    │
│  │  - Regression check                                              │    │
│  │  PASS → Proceed to merge                                         │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                                                                          │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │  VISUAL/UI TESTING (Human Required)                              │    │
│  │  - Browser/vision model can verify layout                        │    │
│  │  - BUT human must approve regardless                             │    │
│  │  - Dashboard preview via Vercel                                  │    │
│  │  Status: awaiting_human                                          │    │
│  └─────────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                      SUPERVISOR FINAL APPROVAL                           │
│                                                                          │
│  After tests pass (and human approves visual):                           │
│          1. Merge branch to main                                         │
│          2. Delete branch                                                │
│          3. Update task status: complete                                 │
│          4. Unlock dependent tasks                                       │
│          5. Log model rating (success/failure by task type)              │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                         ORCHESTRATOR LEARNS                              │
│                                                                          │
│  Updates model registry:                                                 │
│          - Success rate per task type                                    │
│          - Context efficiency                                            │
│          - Reliability score                                             │
│                                                                          │
│  Future routing gets smarter.                                            │
└─────────────────────────────────────────────────────────────────────────┘
```

---

# 3. Roles & Agents

## 3.1 Role vs Model vs Skills vs Tools

| Concept | What It Is | Example |
|---------|------------|---------|
| **Model** | The AI engine | GLM-5, Kimi, DeepSeek |
| **Role** | The job/hat | Planner, Supervisor, Executor |
| **Skills** | What the role can do | decompose, validate, execute |
| **Tools** | What the role can use | file_read, git_operations, supabase_query |

**Models wear hats. Hats are defined in config. Swapping models doesn't change roles.**

## 3.2 Role Definitions

| Role | Responsibilities | Sees | Output |
|------|-----------------|------|--------|
| **Consultant Research** | Deep research with user until PRD approved | Market, competition, tech stacks, features, gaps | Zero-ambiguity PRD (user approved) |
| **Planner** | Breaks PRD into atomic tasks | PRD, system state, model constraints | Full plan with prompt packets |
| **Council Member** | Independent review | Full PRD + Plan | Vote + concerns + reasoning |
| **Supervisor** | Validates output, merges | Task output, expected output | Pass/Fail + notes |
| **Task Agent (Runner)** | Executes single task via CLI/API | Task packet + relevant codebase (dependencies) | Result (no chat URL) |
| **Courier** | Web platform delivery | Task packet + platform | Result + chat_url |
| **Tester (Code)** | Automated testing | Code, test criteria | Pass/Fail + details |
| **Tester (Visual)** | UI verification | Visual output | Human approval needed |
| **Maintenance** | System updates | Full system | Safe changes |
| **System Research** | Daily web scouring for improvements | AI papers, models, platforms, tools | Considerations file (for council review) |
| **Watcher** | Prevents loops, detects drift | Model outputs, error patterns | Alerts, automatic interventions |

## 3.3 Research Agents

### Consultant Research Agent

**Purpose:** Works directly with user from idea to approved PRD.

**Researches:**
- Market landscape
- Competitor features and gaps
- Tech stack options
- Marketing/positioning
- User intent clarification

**Process:**
1. User presents rough idea
2. Agent asks clarifying questions
3. Agent researches market/competition
4. Agent drafts PRD sections
5. User reviews, provides feedback
6. Iterate until user approves final PRD
7. PRD handed to Planner

**NOT automated. Interactive with human until PRD is signed off.**

### System Research Agent

**Purpose:** Daily autonomous research to improve VibePilot itself.

**Searches for:**
- New AI papers and breakthroughs
- New models (open source, free tier, lightweight)
- New platforms (zero lock-in, free tiers)
- New tools and frameworks
- Better strategies and approaches

**Output:** Findings go to `docs/UPDATE_CONSIDERATIONS.md`

**Review Process:**
- Maintenance supervisor reviews daily
- Council reviews significant findings
- New models/platforms: direct add to registry
- Other changes: Council approval before implementation

**Runs:** Daily (scheduled), not on-demand

## 3.4 Watcher Agent

**Purpose:** Prevents models from getting stuck in loops of doom.

**Monitors:**
- Same error 3+ times in a row
- Same output pattern repeating
- Context not progressing (stuck)
- Task running > 30 minutes
- Token waste (repetitive context)

**Actions:**
| Detection | Action |
|-----------|--------|
| Same error 3x | Kill task, flag for different model |
| Output loop | Kill task, alert supervisor |
| Stuck context | Kill task, suggest split |
| Timeout | Kill task, log duration |
| Token waste | Alert, log pattern |

**Works with:**
- CLI runners (Kimi, OpenCode)
- IDE integrations (future MCP support)
- API runners

**Does NOT intervene with Council or Supervisor decisions. Only execution loops.**

## 3.5 Context Isolation

| Role | What They See | What They Don't See |
|------|---------------|---------------------|
| Task Agent (Runner) | Task packet + relevant code (dependencies focused) | Other tasks, full PRD, unrelated codebase |
| Task Agent (Courier) | Task packet only | Any codebase, other tasks, PRD |
| Planner | PRD, system overview, model registry | Individual task outputs |
| Supervisor | Task output, expected output | Other parallel tasks |
| Council | Full PRD + Plan | Code implementation details |
| System Research | Web, AI landscape | Internal task states |
| Watcher | Model outputs, error logs | PRD, business logic |

**Prevents drift, hallucination, and scope creep.**

## 3.6 Council Process Detail

**When Council is Called:**
- Supervisor calls Council when plan is complete (before approval)
- Supervisor calls Council when system changes are recommended (before maintenance implementation)

**Council Round Process:**

```
Round 1:
┌─────────────────────────────────────────────────────────────┐
│  SUPERVISOR sends: Full PRD + Full Plan                     │
│                                                              │
│  THREE COUNCIL MEMBERS (different models, different hats)   │
│  - User Alignment Hat: True to user intent?                 │
│  - Architecture Hat: Technically sound?                     │
│  - Feasibility Hat: Can this be built?                      │
│                                                              │
│  Each reviews INDEPENDENTLY (no chatting between members)   │
│  Each outputs: APPROVED or NOT YET + issues/concerns        │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  FEEDBACK CONSOLIDATION                                      │
│                                                              │
│  All concerns aggregated into single document               │
│  Sent back to ALL council members                           │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
         ┌────────────────────┴────────────────────┐
         │                                         │
    ALL APPROVED                            NOT YET
         │                                         │
         ▼                                         ▼
   Supervisor approves              Planner addresses concerns
   Plan → Tasks                     New version of plan
                                            │
                                            ▼
                                    Council Round 2
                                    (repeat until consensus)
```

**Iteration Limit:** 4 rounds typical. If no consensus by round 5, escalate to human.

**Final Vote:** If Supervisor unsure after changes, calls Council for final vote/revisions.

## 3.7 Security: Vault Access Control

**Who Can Access Vault:**

| Role | Can Access Vault? | How |
|------|-------------------|-----|
| Orchestrator | YES | To pass keys to runners (internal only) |
| Supervisor | NO | Never needs keys |
| Council | NO | Never needs keys |
| Planner | NO | Never needs keys |
| Task Agents | NO | Keys passed TO them, they don't fetch |
| Tester | NO | Only sees code, nothing else |
| Courier | NO | Never needs keys |
| Watcher | NO | Only sees outputs |

**Implementation:**
- VaultManager runs in orchestrator process only
- Task agents receive keys as parameters, never import vault_manager
- No agent can print all secrets — they only see what's passed to them
- If prompt injection says "print vault keys" — agent has no vault access to print

## 3.8 Tester Isolation

**Tester sees ONLY:**
- Code returned from task
- Test criteria from task packet

**Tester does NOT see:**
- Vault
- Other tasks
- PRD
- Model that created code
- Branch name
- Any system state

**Tester's only job:**
- Run tests on code
- Return: PASS or FAIL + details

**Tester is NOT:**
- Orchestrator
- Supervisor
- Anything other than a code tester

## 3.9 Credit & Rate Limit Tracking

**Per Model/API:**

```
Available Credit: $X.XX
Cost per 1M tokens (in): $X.XX
Cost per 1M tokens (out): $X.XX

Task Run:
- Tokens in: N
- Tokens out: N
- Cost: (in/1M * rate_in) + (out/1M * rate_out)
- Remaining credit: $X.XX - cost
```

**Rate Limits:**

| Model | Requests/min | Requests/hour | Requests/day | Tokens/day |
|-------|--------------|---------------|--------------|------------|
| (tracked in models table) |

**When Credit Exhausted:**
- Model status → `paused`
- `status_reason` → "Credit exhausted"
- Orchestrator routes around
- Dashboard alert
- Daily email mentions it
- Human tops up → status → `active`

## 3.10 Task Failure & Branch Lifecycle

**Task Failure Handling:**

| Failure Type | Action |
|--------------|--------|
| Platform down | Reassign to different platform |
| Context exceeded | Reassign to larger context model or split task |
| Output doesn't match packet | Supervisor rejects, delete code, reassign |
| 3+ reassignments (different platforms) | **PLAN PROBLEM** — shouldn't happen, escalate to supervisor |

**Branch Lifecycle:**

```
Task starts → Branch created (task/T001-desc)
                   │
                   ▼
           Code submitted
                   │
                   ▼
         Supervisor reviews
                   │
        ┌──────────┴──────────┐
        │                     │
     REJECT                APPROVE
        │                     │
        ▼                     ▼
   Delete code            Testing
   Branch stays                │
        │              ┌───────┴───────┐
        │              │               │
        │           PASS            FAIL
        │              │               │
        │              ▼               ▼
        │          Supervisor      Fix and retest
        │          final approval
        │              │
        ▼              ▼
   [waiting for        Merge to main
    approved code]          │
                            ▼
                      Delete branch
                      Task complete
```

**Code is NEVER merged until:**
1. Supervisor approved
2. Tests passed
3. (If visual) Human approved
4. Supervisor final approval

## 3.11 PRD Changes Mid-Project

**Scenario:** User wants to add feature or change requirements after tasks started.

**Process:**

```
User request for change
        │
        ▼
Clarify with user (Consultant Research if needed)
        │
        ▼
Send to Planner
        │
        ▼
Planner creates NEW tasks or modifies existing
        │
        ▼
Supervisor checks, may call Council if significant
        │
        ▼
If approved → New tasks added to queue
If existing tasks affected → locked until re-evaluated
```

**Version Control:**
- PRD versioned (v1.0, v1.1, etc.)
- Plan versioned to match PRD
- Changelog tracks all changes

## 3.12 Human Notification

| Channel | What | Frequency |
|---------|------|-----------|
| Dashboard | Everything live | Real-time |
| Daily Email | Summary: tasks completed, failures, credits, ROI | Once/day |
| Alert | Platform down, all paused, critical failures | Immediate |

**Daily Email Contents:**
- Tasks completed: X
- Tasks failed: Y (with reasons)
- Tokens used: Z
- ROI: $X saved
- Credit status: all models
- Model performance: top/bottom performers
- System research: notable findings

## 3.13 Multi-Project Handling

**Each Project Has:**
- Separate GitHub repository
- Separate PRD
- Separate plans
- Separate tasks
- Project-specific dashboard view

**Shared Across Projects:**
- Models registry (same models available)
- Orchestrator (one orchestrator, multiple project queues)
- Vault (same secrets)
- Dashboard (switch between projects)

**Orchestrator Priority:**
- Projects can have priority levels
- Critical projects get first pick of models
- Lower priority waits if resources constrained

## 3.14 Prompt Storage

**Where Prompts Live:**
- `config/vibepilot.yaml` — Human-readable, editable in GitHub
- Dashboard admin panel — Edit without touching code

**Format:**
```yaml
roles:
  planner:
    description: "Breaks work into atomic tasks"
    prompt: |
      [Full prompt text here]
      Human can edit this directly.
    skills: [decompose, sequence, estimate]
```

**Source of Truth:** The YAML file in GitHub. Dashboard reads from it, can push edits back.

**Version Control:** Every prompt change → commit to GitHub → tracked in changelog.

## 3.15 Deployment Flow

**After Merge to Main:**

```
Task merged to main
        │
        ▼
Code sits in correct folder in main
        │
        ▼
Project continues until all tasks complete
        │
        ▼
Full project review (supervisor + council)
        │
        ▼
Final double-check: security, edge cases, tests
        │
        ▼
Deploy to live production environment
(Separate from VibePilot — customer's infrastructure)
```

**VibePilot is NOT the deployment platform.** It produces code that gets deployed elsewhere.

**Pre-Deploy Checklist:**
- All tests pass
- Security review complete
- Edge cases handled
- Documentation updated
- Human sign-off

## 3.16 Data Retention

| Data Type | Retention | After Project Complete |
|-----------|-----------|------------------------|
| task_runs | Until project complete | Archived |
| chat_urls | Until project complete | Archived |
| PRD versions | Forever | Archived |
| Plan versions | Forever | Archived |
| Model performance data | Rolling 90 days | Summarized, then archived |
| Daily emails | 30 days | Deleted |
| Audit logs | Forever | Archived |

**Archive Location:** Separate Supabase project or export to storage (S3, Backblaze)

---

# 4. Data Model

## 4.1 Core Tables

### plans
```sql
CREATE TABLE plans (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID REFERENCES projects(id),
  prd_content TEXT,                    -- Full PRD document
  plan_content TEXT,                   -- Full plan with all tasks
  status TEXT DEFAULT 'draft',         -- draft, council_review, approved, active
  council_rounds INT DEFAULT 0,
  council_feedback JSONB,              -- Summary of council iterations
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW(),
  approved_at TIMESTAMPTZ,
  approved_by TEXT                     -- Supervisor model
);
```

### tasks
```sql
CREATE TABLE tasks (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  plan_id UUID REFERENCES plans(id),
  task_number TEXT,                    -- T001, T002, etc.
  title TEXT,
  description TEXT,                    -- What and why
  prompt_packet TEXT,                  -- Full template for executor
  expected_output JSONB,               -- Files, APIs, tests expected
  dependencies JSONB,                  -- [{task_id, type: summary|code_context}]
  requires_codebase BOOLEAN DEFAULT FALSE,
  confidence FLOAT,                    -- 0.95+ required
  status TEXT DEFAULT 'locked',        -- locked, available, assigned, in_progress, review, testing, awaiting_human, complete, failed
  assigned_model TEXT,
  result TEXT,
  chat_url TEXT,                       -- If courier delivered
  branch_name TEXT,                    -- task/T001-short-desc
  tokens_used INT,
  runtime_seconds INT,
  model_rating JSONB,                  -- {model_id, success, notes}
  attempts INT DEFAULT 0,
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW(),
  completed_at TIMESTAMPTZ
);
```

### models
```sql
CREATE TABLE models (
  id TEXT PRIMARY KEY,                 -- glm-5, kimi-k2.5, deepseek-chat
  platform TEXT,                       -- opencode, kimi-cli, deepseek-api
  type TEXT,                           -- runner, courier
  context_limit INT,                   -- Benchmark limit
  context_effective INT,               -- Real-world usable
  request_limits JSONB,                -- {per_minute, per_hour, per_day}
  request_used JSONB,                  -- {per_minute, per_hour, per_day}
  strengths TEXT[],
  weaknesses TEXT[],
  task_ratings JSONB,                  -- {task_type: {success: N, fail: N, avg_tokens: N}}
  status TEXT DEFAULT 'active',        -- active, paused, timeout, offline
  status_reason TEXT,
  cooldown_until TIMESTAMPTZ,
  last_success TIMESTAMPTZ,
  last_failure TIMESTAMPTZ,
  
  -- Credit tracking (for paid APIs)
  api_cost_per_1m_in FLOAT,            -- Cost per 1M input tokens
  api_cost_per_1m_out FLOAT,           -- Cost per 1M output tokens
  credit_available FLOAT,              -- Current credit balance
  credit_used_this_month FLOAT,
  subscription_cost FLOAT,             -- Monthly subscription cost (if any)
  subscription_renews_at TIMESTAMPTZ,
  
  -- Performance metrics (updated by orchestrator)
  total_tasks_completed INT DEFAULT 0,
  success_rate FLOAT DEFAULT 0,
  avg_tokens_per_task INT DEFAULT 0,
  recommendation_score FLOAT DEFAULT 0  -- For subscription renewal decisions
);
```

### task_runs
```sql
CREATE TABLE task_runs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  task_id UUID REFERENCES tasks(id),
  model_id TEXT REFERENCES models(id),
  platform TEXT,
  courier TEXT,
  status TEXT,
  result JSONB,
  
  -- Token tracking
  tokens_in INT,                       -- Input tokens
  tokens_out INT,                      -- Output tokens
  tokens_total INT,                    -- Total
  
  -- Cost tracking
  theoretical_cost FLOAT,              -- What it would cost via API
  actual_cost FLOAT,                   -- What it actually cost (0 for free tier)
  savings FLOAT,                       -- theoretical - actual
  
  -- Failure tracking
  failure_reason TEXT,
  failure_code TEXT,                   -- E001, E002, etc.
  
  created_at TIMESTAMPTZ DEFAULT NOW(),
  completed_at TIMESTAMPTZ,
  duration_seconds INT
);
```

### secrets_vault
```sql
CREATE TABLE secrets_vault (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  key_name TEXT UNIQUE,
  encrypted_value TEXT,
  created_at TIMESTAMPTZ DEFAULT NOW()
);
```

## 4.2 Status Flow

```
locked → available → assigned → in_progress → review → testing → [awaiting_human] → complete
                              ↑                 │         │
                              └─────────────────┴─────────┘
                                           (loops on failure)

Special states:
- failed (3+ attempts)
- escalated (needs human decision)
```

---

# 5. Planner Specification

## 5.1 What Planner Receives

- Full PRD (zero-ambiguity, all questions answered)
- System overview (tech stack, existing components)
- Model constraints (from registry: context_effective, request_limits, strengths/weaknesses)

## 5.2 Confidence Calculation

Planner calculates confidence based on:

| Factor | Weight | Question |
|--------|--------|----------|
| Context fit | 25% | Can this run on lowest viable model? (4k-8k context) |
| Dependency complexity | 25% | 0 deps = easy, 1-2 = medium, 3+ = complex |
| Task clarity | 20% | Is expected output crystal clear? |
| Codebase need | 15% | Does it need full code awareness? |
| One-shot capable | 15% | Can it complete in single turn? |

**If confidence < 95%: SPLIT the task**

## 5.3 Dependency Types

| Type | Meaning | Impact on Routing |
|------|---------|-------------------|
| `summary` | 2 sentences, any model can use | Low context burden |
| `code_context` | Needs actual code from dependency | Higher context, may need CLI |

## 5.4 Task Output Template

Each task in plan:

```yaml
task_id: T001
title: "Implement user profile display"
confidence: 0.97
dependencies: []  # or [{task_id: T000, type: summary}]

# What and why (for context, not execution)
purpose: |
  User needs to view their profile data.
  Part of user management slice.
  
# Exact prompt packet for task agent
prompt_packet: |
  Create profile view feature.
  
  FILES TO CREATE:
  - src/pages/ProfilePage.tsx
  - src/api/profile.ts
  - src/tests/ProfilePage.test.ts
  
  DATABASE:
  - Table: users (exists)
  - Columns: id, email, name, avatar_url, created_at
  
  API:
  - GET /api/profile (auth required)
  - Response: { id, email, name, avatar_url, created_at }
  
  TESTS REQUIRED:
  - API returns 401 when not authenticated
  - UI displays user data correctly
  
  OUTPUT FORMAT:
  - Task number: T001
  - Model name: [your model name]
  - Code files created
  
  DO NOT:
  - Add edit functionality
  - Add settings
  - Add features not listed

# Expected output for supervisor validation
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
  tests_required:
    - "API returns 401 when not authenticated"
    - "UI displays user data correctly"

# Routing hints for orchestrator
requires_codebase: false
estimated_context: 8k
```

## 5.5 CLI Flag

If task has 4+ dependencies requiring `code_context`, OR estimated context > 32k:

```yaml
requires_cli: true
cli_reason: "4+ code_context dependencies"
```

Orchestrator will route to CLI subscription (Kimi, OpenCode), not free tier API.

---

# 6. Runners vs Couriers

## 6.1 Runners (Programmatic Execution)

| Runner | Type | Access | When Used |
|--------|------|--------|-----------|
| Kimi CLI | Subscription | Codebase | Complex tasks, CLI-flagged |
| OpenCode | Subscription | Codebase | Primary in-house |
| DeepSeek API | Pay-per-use | None | Last resort |
| Gemini API | Free tier | None | Research tasks |

**No browser needed. Direct programmatic execution.**

## 6.2 Couriers (Browser Automation)

**What Couriers ARE:**
A model with full multimodal browser use capability (not Playwright, not Selenium — native computer use). This model navigates, logs in, enters prompts, waits, copies results, and returns.

**Courier Model Requirements:**
- Full browser use (computer use API)
- Multimodal (can see screen, interact)
- Can handle login flows
- Can copy/paste text and URLs

**Courier Model Examples:**
| Model | Cost | Browser Use |
|-------|------|-------------|
| Gemini 2.0 | Subscription | Native computer use |
| Claude Computer Use | Pay-per-use | Native computer use |
| (Future options) | ? | ? |

**Target Platforms (where courier delivers):**

| Platform | Type | Capabilities |
|----------|------|--------------|
| ChatGPT | Free tier | Reasoning, code |
| Claude | Free tier | Reasoning, code |
| Gemini | Free tier | Reasoning, vision |
| Perplexity | Free tier | Research |
| Grok | Free tier | Reasoning |

**Courier lifecycle:**
1. Receive task packet
2. Navigate to target platform
3. Log in if needed (shared Gmail: vibes.agents@gmail.com)
4. Enter task packet as prompt
5. Wait for response
6. Copy result + chat URL
7. Return to VibePilot

**Courier Cost:**
- Courier model has subscription OR API cost
- This cost is attributed per task for ROI calculation
- We track: tokens in/out for courier AND target platform

**Chat URL purpose:**
- Revisit for revisions without full context
- Efficient iteration
- Audit trail

---

# 7. GitHub Integration

## 7.1 Branch Per Task

- Naming: `task/T001-short-desc`
- Created when task starts
- Supervisor merges when complete
- Branch deleted after merge

## 7.2 Orchestrator Visibility

Orchestrator watches:
- Branch creation (task started)
- Branch commits (progress)
- PR creation (ready for review)
- Merge (task complete)
- Branch deletion (cleanup)

---

# 8. Vault (Secret Management)

## 8.1 Purpose

- API keys encrypted, never in .env files
- Prevents prompt injection leaking secrets
- Portable across migrations

## 8.2 Bootstrap Keys (Required)

These 3 keys must be set manually:
- `SUPABASE_URL`
- `SUPABASE_KEY`
- `VAULT_KEY` (Fernet key)

## 8.3 Stored in Vault

- `DEEPSEEK_API_KEY`
- `GITHUB_TOKEN`
- `GEMINI_API_KEY`
- `OPENROUTER_API_KEY`

## 8.4 Usage

```python
from vault_manager import get_api_key

key = get_api_key('DEEPSEEK_API_KEY')
```

Runners use vault automatically. No `os.getenv()` for API keys.

---

# 9. Dashboard

## 9.1 Mission Control

Real-time view of:
- Active tasks by status
- Token usage (project total)
- Model performance
- ROI snapshot

## 9.2 Model Cards

Each model shows:
- Status (active/paused/timeout)
- Context (effective, not benchmark)
- Credits/limits
- Performance metrics
- Current assignments

## 9.3 Task Cards

Each task shows:
- Confidence %
- Dependencies
- Prompt packet (expandable, editable)
- Model assigned
- Tokens used
- Runtime
- Activity log

## 9.4 Admin Panel

- Add/remove API keys
- Swap models for roles
- Update model limits
- Credit top-ups

## 9.5 Watcher Alerts

Real-time alerts from Watcher Agent:
- Loop detection warnings
- Timeout notifications
- Token waste alerts
- Model intervention logs

Alert severity:
- **Warning** — Pattern detected, monitoring
- **Intervention** — Task killed, reassigned
- **Critical** — Multiple models affected

## 9.6 Preview

- Vercel integration for visual tasks
- Human review before merge

## 9.6 Vibes (Voice Interface)

- Full audio chat with system
- "Hey Vibes, what's the status on auth slice?"
- Dashboard responds with live data

---

# 10. ROI Tracking

## 10.1 Philosophy: Real World, Not Benchmarks

**How we know which model/platform is best:**
- NOT benchmarks (trained for, not real)
- NOT marketing claims
- NOT theoretical comparisons
- **REAL TASKS sent to REAL PLATFORMS with REAL RESULTS tracked**

**Continuous Evaluation:**
```
┌─────────────────────────────────────────┐
│  Send test tasks to all free platforms  │
│  (ChatGPT, Claude, Gemini, Perplexity)  │
└─────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────┐
│  Track: success, tokens, time, quality  │
└─────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────┐
│  Analyze: Which performed best?         │
│  Which saved most money?                │
└─────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────┐
│  Route more tasks to winner             │
│  Keep testing others at lower volume    │
└─────────────────────────────────────────┘
                    │
                    ▼
          (repeat forever — market changes, we adapt)
```

**Best Cheapest Most Effective Wins:**
- Today: Gemini as courier
- Tomorrow: Whoever is cheaper/better
- Next Month: Something we haven't heard of yet
- **Every model is swappable based on real performance data**

## 10.2 Full Task Cost Tracking

**A task's cost = ALL attempts until complete**

```
Task T001: "Implement user profile"

Attempt 1: Assigned to DeepSeek API
  - Tokens in: 8,000
  - Tokens out: 4,000
  - Result: FAILED (output didn't match spec)
  - Cost: $0.017

Attempt 2: Assigned to Claude web (via Gemini courier)
  - Courier tokens: 2,000
  - Claude tokens in: 8,000
  - Claude tokens out: 6,000
  - Result: FAILED (syntax errors)
  - Cost: $0.20 (courier allocation)

Decision: Split task into T001a and T001b

Attempt 3 (T001a): Assigned to Kimi CLI
  - Tokens in: 5,000
  - Tokens out: 3,000
  - Result: PASSED
  - Cost: $0.10 (subscription allocation)

Attempt 4 (T001b): Assigned to ChatGPT web (via Gemini courier)
  - Courier tokens: 1,500
  - ChatGPT tokens in: 4,000
  - ChatGPT tokens out: 3,000
  - Result: PASSED
  - Cost: $0.15 (courier allocation)

════════════════════════════════════════
TOTAL TASK COST: $0.017 + $0.20 + $0.10 + $0.15 = $0.467
TOTAL TOKENS: 44,500 (all attempts)
THEORETICAL COST (if all via single API): $0.89
SAVINGS: $0.423
════════════════════════════════════════
```

**Dashboard shows:**
- Per task: All attempts, all costs, final result
- Per model: Success rate, avg cost per task type, total spend
- Per project: Total actual cost, total theoretical cost, savings

## 10.3 ROI Calculator (Live)

**Always visible in dashboard. Always up to date.**

```
┌─────────────────────────────────────────────────────────────┐
│                    ROI CALCULATOR                           │
├─────────────────────────────────────────────────────────────┤
│  This Month                                                 │
│  ─────────────────────────────────────────────────────────  │
│  Tasks Completed:        47                                 │
│  Tasks Failed:           3                                  │
│  Total Attempts:         58 (avg 1.23 per task)             │
│                                                             │
│  Tokens Used:                                               │
│  - Total In:             312K                               │
│  - Total Out:            198K                               │
│                                                             │
│  Costs:                                                     │
│  - Actual (subscriptions + API):    $12.40                  │
│  - Theoretical (if all API):        $89.20                  │
│  - Savings:                         $76.80                  │
│  - ROI:                             86.1%                    │
│                                                             │
│  By Model:                                                  │
│  ─────────────────────────────────────────────────────────  │
│  Kimi CLI        22 tasks  95% success  $0.09/task          │
│  DeepSeek API    8 tasks   75% success  $0.03/task          │
│  ChatGPT web     12 tasks  92% success  $0.15/task          │
│  Claude web      5 tasks   80% success  $0.18/task          │
│                                                             │
│  Recommendation: Route more to Kimi CLI (best cost/success) │
└─────────────────────────────────────────────────────────────┘
```

## 10.4 Courier Cost Attribution

**Courier model costs us subscription.** That cost is spread across all tasks the courier handles.

```python
courier_subscription_monthly = $X
courier_tasks_this_month = N
courier_cost_per_task = courier_subscription_monthly / N

# Example:
# Gemini subscription: $20/month
# Tasks delivered: 150
# Cost per task: $0.13
```

**Courier model is chosen by:**
- Has native browser use (computer use API)
- Is cheapest effective option available
- Is swappable anytime something better emerges

## 10.5 Different Execution Scenarios

| Execution Type | Actual Cost | Theoretical Cost | Savings |
|----------------|-------------|------------------|---------|
| Courier → Claude web | Courier subscription ÷ tasks | Claude API rates | High |
| Courier → ChatGPT web | Courier subscription ÷ tasks | OpenAI API rates | High |
| Direct → DeepSeek API | Real money spent | Same as actual | $0 (visibility only) |
| CLI → Kimi subscription | Kimi subscription ÷ tasks | Kimi API rates (if existed) | Variable |

## 10.6 Why This Matters

- **Courier choice:** Which courier subscription is worth it? ROI tells us.
- **Platform choice:** Which free web platforms save us most?
- **Subscription decisions:** Renew Kimi? ROI vs alternatives tells us.
- **Hosting costs:** GCE vs Hetzner — total cost of ownership.
- **Model performance:** Which models earn their keep?

## 10.7 Per Project Aggregate

```
Project: Auth & RBAC
Tasks completed: 47
Total attempts: 58
Total tokens: 847K

Theoretical cost (if all via APIs): $127.40
Actual cost (all attempts, all models): $23.50
Total savings: $103.90
ROI: 81.6%
```

## 10.8 Model Performance (Informs Routing)

- Success rate by task type
- Average tokens per task type
- Average runtime
- Reliability score
- **Cost per successful task** — Failed attempts count toward this
- **Recommendation score** — Should we keep subscribing?

---

# 11. Migration

# 11. Migration

## 11.1 Portability

Everything needed to move in 30 minutes:
- Code: GitHub
- State: Supabase
- Secrets: Vault (in Supabase)
- Setup: `./setup.sh`

## 11.2 Bootstrap New Machine

```bash
git clone git@github.com:VibesTribe/VibePilot.git
cd VibePilot

# Set 3 bootstrap keys
export SUPABASE_URL=...
export SUPABASE_KEY=...
export VAULT_KEY=...

./setup.sh
```

Vault provides all other secrets automatically.

---

# 12. Anti-Patterns (Forbidden)

| Forbidden | Why |
|-----------|-----|
| Reactive "fix it" without context | Breaks connected systems |
| Task agent sees other tasks | Drift, scope creep |
| Same model reviewing own work | Blind spots |
| Silent architecture changes | No audit trail |
| Secrets in .env files | Prompt injection risk |
| Agent-to-agent chat | Token waste |
| Manual hotfixes | Traceability lost |
| Skipping Council | Governance gap |

---

# 13. Success Criteria

VibePilot succeeds when:

1. **Any component swappable** without system failure
2. **Tasks complete deterministically** with predictable outcomes
3. **ROI tracked** on every execution
4. **Errors classified** and handled appropriately
5. **No vendor lock-in**
6. **Human oversight** where needed (visual, security, architecture)
7. **System learns** from every task
8. **New session starts** with three files: CURRENT_STATE.md + this PRD + CHANGELOG.md
9. **Watcher prevents** loop-of-doom scenarios
10. **Research agents** keep system improving daily

---

# 14. Key Files

| File | Purpose |
|------|---------|
| `CURRENT_STATE.md` | Where we are, what's working |
| `CHANGELOG.md` | Audit trail |
| `docs/prd_v1.4.md` | This document - complete system spec |
| `docs/UPDATE_CONSIDERATIONS.md` | System research findings (daily update) |
| `config/vibepilot.yaml` | Roles, prompts, thresholds |
| `vault_manager.py` | Secret management |
| `orchestrator.py` | Task dispatch |
| `runners/` | Programmatic execution (CLI/API) |
| `agents/` | Courier agents (browser automation) |

---

**End of VibePilot v1.4 PRD**

*This document is the complete system specification.*
*Changes require Council approval.*
