# Governor Implementation Handoff Document

**Session:** 2026-02-22
**Purpose:** Full understanding captured for next session - NO CODING until this is reviewed
**Status:** Homework complete, ready for planning

---

## WHY WE ARE BUILDING GO GOVERNOR

### The Problem
| Current State | Problem |
|---------------|---------|
| Python orchestrator + OpenCode | 1.5GB RAM total |
| e2-micro free tier | 1GB RAM limit |
| **Result** | **Won't fit on free tier** |

### The Solution
| Component | Python (Current) | Go (Target) |
|-----------|------------------|-------------|
| RAM usage | 150MB base + 1.4GB OpenCode | 10-20MB binary |
| GCE cost | $64/mo (e2-standard-2) | $0 (e2-micro free tier) |
| Deployment | venv + pip + drift | Single static binary |
| Concurrency | GIL limits threads | Goroutines (2KB stack each) |

### What Go Enables
1. **Free tier operation** - 9.5MB binary vs 1.5GB Python stack
2. **100+ concurrent tasks** - Goroutines scale efficiently
3. **GitHub Actions offload** - Browser-use tasks on 7GB free runners
4. **Zero dependency deploy** - Single binary, no venv drift

---

## THE FULL SYSTEM (What Python Does)

### Agent Hierarchy

| Category | Agent | Role | Model | Can Decide | Can Execute | Git Write |
|----------|-------|------|-------|------------|-------------|-----------|
| **Decision** | orchestrator | Routing | gemini-2.0-flash | Yes | No | No |
| | council (3) | Plan Review | gemini-2.0-flash | Yes | No | No |
| | supervisor | Quality Gate | gemini-2.0-flash | Yes | No | No |
| **Execution** | courier | Web browser | browser-use-gemini | No | Yes | No |
| | internal_cli | CLI tasks | opencode/kimi | No | Yes | No |
| | internal_api | API calls | gemini-api | No | Yes | No |
| | tester_code | Code tests | opencode | No | Yes | No |
| | **maintenance** | Git operator | opencode | No | Yes | **YES (ONLY)** |
| **Support** | vibes | Human interface | gemini | No | No | No |
| | researcher | Daily intel | gemini | No | No | No |
| | consultant | PRD generation | gemini | No | No | No |
| | planner | Task planning | kimi | No | No | No |

**CRITICAL: Only `maintenance` agent has git write access. All git operations go through `maintenance_commands` table.**

### Task Lifecycle

```
1. IDEA → Consultant creates PRD
   ↓
2. PRD → Planner creates TASKS (vertical slices, each with full prompt_packet)
   ↓
3. TASKS → Council reviews (3 lenses: user_alignment, architecture, feasibility)
   ↓
4. Council APPROVES → Supervisor locks in tasks to Supabase
   ↓
5. Orchestrator DISPATCHES task:
   - routing_flag='web' → Courier on browser
   - routing_flag='internal' → CLI tool (opencode)
   ↓
6. Task EXECUTES, writes result to task_runs
   ↓
7. calculate_enhanced_task_roi(run_id) RPC called
   ↓
8. Task status → 'review', Supervisor reviews output
   ↓
9. If approved → 'testing', Tester runs tests
   ↓
10. If tests pass → 'approval', Supervisor commands merge
    ↓
11. Maintenance agent reads maintenance_commands, executes git merge
    ↓
12. Task → 'merged', unlock_dependent_tasks RPC called
```

### GitHub Branch Strategy

```
main
├── task/uuid-001  (Task #1 working branch)
├── task/uuid-002  (Task #2 working branch)
└── task/uuid-003  (Task #3 working branch)
```

**Branch Lifecycle:**
1. **Dispatch** → Create `task/{task_id}` from main
2. **Execute** → Courier works in branch
3. **Review** → Supervisor reviews in Supabase
4. **Approve** → Supervisor writes to `maintenance_commands`
5. **Merge** → Maintenance agent executes merge
6. **Cleanup** → GitHub auto-deletes merged branches

---

## THE ROI SYSTEM (DO NOT BREAK)

### What Gets Recorded (task_runs table)

| Column | Purpose | Source |
|--------|---------|--------|
| task_id | Which task | orchestrator |
| model_id | Which AI model | runner |
| courier | Tool name | runner |
| platform | Web platform or "internal" | runner |
| status | "success" or "failed" | runner |
| result | Full output as JSONB | runner |
| tokens_in | Input tokens | runner |
| tokens_out | Output tokens | runner |
| tokens_used | Total | calculated |

### What Gets Calculated (by RPC)

**Function:** `calculate_enhanced_task_roi(p_run_id)`

1. Gets tokens_in, tokens_out, platform from run
2. Looks up platform's `theoretical_cost_input_per_1k_usd` and `theoretical_cost_output_per_1k_usd`
3. Calculates:
   ```
   theoretical = (tokens_in/1000 * cost_in) + (tokens_out/1000 * cost_out)
   ```
4. If courier_model_id exists, adds courier costs
5. Calculates:
   ```
   savings = theoretical - actual
   ```
6. Updates task_runs with:
   - `platform_theoretical_cost_usd`
   - `total_actual_cost_usd`
   - `total_savings_usd`
7. Updates projects cumulative totals

### What Dashboard Reads (vibeflow/apps/dashboard/lib/vibepilotAdapter.ts)

- `tokens_in`, `tokens_out`, `tokens_used`
- `courier_tokens`, `courier_cost_usd`
- `platform_theoretical_cost_usd`, `total_actual_cost_usd`, `total_savings_usd`

**If Go doesn't create task_runs correctly OR doesn't call ROI RPC → Dashboard shows $0 → Months of ROI work broken.**

---

## KEY STORAGE

### Bootstrap Keys (.env - only 3 needed)
```
SUPABASE_URL=https://xxx.supabase.co
SUPABASE_KEY=eyJ...  # anon key
VAULT_KEY=LgbwdSxx...  # encryption key
```

### Secrets Vault (secrets_vault table, encrypted)
- `DEEPSEEK_API_KEY`
- `GITHUB_TOKEN`
- `GEMINI_API_KEY` (to be added)

### Access Pattern
```python
from vault_manager import get_api_key
key = get_api_key('DEEPSEEK_API_KEY')
```

**Go will need to implement vault access or receive keys via environment.**

---

## SUPABASE PATTERNS (Critical for Go)

### What DOESN'T Work via REST
```go
// WRONG - SQL expressions are string literals, not evaluated
body := map[string]interface{}{
    "attempts": "attempts + 1",  // ERROR: invalid input syntax
}
```

### What DOES Work
```go
// RIGHT - Fetch, calculate in Go, write back
task, _ := db.GetTaskByID(ctx, taskID)
newAttempts := task.Attempts + 1
body := map[string]interface{}{
    "attempts": newAttempts,  // Integer value
}
```

### Key RPC Functions

| Function | Purpose |
|----------|---------|
| `claim_next_task(courier, platform, model_id)` | Atomic claim with FOR UPDATE SKIP LOCKED |
| `calculate_enhanced_task_roi(run_id)` | Calculate and store ROI (MUST CALL) |
| `unlock_dependent_tasks(completed_task_id)` | Unlock waiting tasks |
| `claim_next_command(agent_id)` | Maintenance agent claims git command |

---

## TASK PACKET PARSING

### Database Structure
```
task_packets.prompt is a TEXT column containing a JSON STRING
```

### Example
```json
// This is what's stored in the prompt column:
{"task_id": "uuid", "prompt": "Say hello", "title": "Test", "constraints": {"max_tokens": 50}}
```

### WRONG Approach
```go
var packet PromptPacket
json.Unmarshal(data, &packet)
// packet.Prompt == "{\"task_id\": \"uuid\", \"prompt\": \"Say hello\"...}"
// Wrong - it's the whole JSON string!
```

### RIGHT Approach
```go
type TaskPacketRow struct {
    Prompt string `json:"prompt"`  // JSON string
}
var rows []TaskPacketRow
json.Unmarshal(data, &rows)

var packet PromptPacket
json.Unmarshal([]byte(rows[0].Prompt), &packet)
// packet.Prompt == "Say hello"
// Correct - the actual prompt text!
```

---

## OPENCODE EXECUTION

### WRONG (what I did)
```go
cmd := exec.Command("opencode", "--task-packet", packetJSON)
// --task-packet flag doesn't exist
```

### RIGHT (from contract_runners.py:234-240)
```go
prompt := "The actual prompt text"
cmd := exec.CommandContext(ctx, "opencode", "run", "--format", "json", prompt)
output, _ := cmd.CombinedOutput()

var result struct {
    Content      string `json:"content"`
    InputTokens  int    `json:"input_tokens"`
    OutputTokens int    `json:"output_tokens"`
}
json.Unmarshal(output, &result)
```

---

## CURRENT GO GOVERNOR STATE (Phase 1)

### What Works
- Sentry polls Supabase every 15s
- Finds available tasks
- Dispatcher skeleton routes to local/web
- Janitor detects stuck tasks
- HTTP server with `/api/tasks`, `/api/models`
- Binary: 9.5MB

### What's Broken/Incomplete
| Component | Issue |
|-----------|-------|
| Claim | Uses REST, not RPC (race conditions possible) |
| Packet parsing | Direct unmarshal (gets JSON string, not parsed) |
| OpenCode | Wrong flags (`--task-packet` doesn't exist) |
| task_runs | Not recording anything |
| ROI RPC | Not calling it |
| Failure handling | Uses `"attempts + 1"` string (fails) |
| Unlock deps | Not calling unlock RPC |

---

## WHAT GOVERNOR MUST DO (Phase 2)

### Minimal Working Implementation

1. **Claim task** - Use REST with filter OR call `claim_next_task` RPC
2. **Fetch packet** - Parse JSON string to get actual prompt
3. **Execute opencode** - `opencode run --format json "prompt"`
4. **Parse output** - Extract tokens_in, tokens_out, content
5. **Insert task_run** with ALL fields:
   - task_id, model_id, courier, platform
   - status, result (JSONB)
   - tokens_in, tokens_out, tokens_used
6. **Call ROI RPC** - `calculate_enhanced_task_roi(run_id)`
7. **Update task** - status='review' or handle_failure
8. **Unlock dependents** - Call `unlock_dependent_tasks(task_id)`

### Failure Handling Pattern
```go
func (d *DB) handleFailure(taskID string) error {
    // 1. Fetch current attempts
    task, err := d.GetTaskByID(ctx, taskID)
    if err != nil { return err }
    
    // 2. Increment in Go
    newAttempts := task.Attempts + 1
    
    // 3. Decide status
    status := "available"
    if newAttempts >= task.MaxAttempts {
        status = "escalated"
    }
    
    // 4. Write back with calculated value
    body := map[string]interface{}{
        "status": status,
        "attempts": newAttempts,
        "assigned_to": nil,
    }
    return d.Update(ctx, "tasks", taskID, body)
}
```

---

## FILES THAT WOULD BE TOUCHED

| File | Changes Needed |
|------|----------------|
| `governor/internal/db/supabase.go` | Add GetTaskByID, RecordTaskRun, CallRPC, fix packet parsing |
| `governor/internal/dispatcher/dispatcher.go` | Fix opencode command, add recording, fix failure |
| `governor/pkg/types/types.go` | Ensure TaskRun has all ROI fields |

## FILES THAT MUST NOT BE TOUCHED

| File | Why |
|------|-----|
| `docs/supabase-schema/*.sql` | Schema works, don't break |
| `vibeflow/apps/dashboard/*` | Reads from DB, if Go writes correctly it works |
| `config/prompts/*.md` | Agent prompts preserved |
| `runners/contract_runners.py` | Still used, Go calls same tools |

---

## TESTING STRATEGY

### Before Any Code
1. Create tiny test task with Python
2. Observe ALL DB writes (task_runs, ROI fields)
3. Document exact field values

### After Go Changes
1. Create identical test task
2. Run Go Governor
3. Compare DB state field-by-field
4. Verify dashboard shows same ROI

### Tiny Test Task
```python
task_id = str(uuid.uuid4())
sb.table('tasks').insert({
    'id': task_id,
    'title': 'TEST: Echo',
    'status': 'available',
    'priority': 1,
    'routing_flag': 'internal',
}).execute()

sb.table('task_packets').insert({
    'task_id': task_id,
    'prompt': json.dumps({
        'task_id': task_id,
        'prompt': 'Echo: Hello World'
    }),
    'version': 1,
}).execute()
```

---

## OPEN QUESTIONS

1. **Where does Go stop and Python take over?**
   - Go executes tasks and records results
   - Python Supervisor still reviews?
   - Python Maintenance still merges?

2. **How does courier/browser-use work on GitHub Actions?**
   - Workflow file needed: `.github/workflows/courier.yml`
   - Browser setup in runner
   - How does result get back to Supabase?

3. **Key management for Go?**
   - Go needs VAULT_KEY to decrypt secrets
   - Or pass keys via environment at startup
   - Or implement vault decryption in Go

---

## PHASE 2 COMPLETE (2026-02-22)

### What's Now Working

| Component | Status |
|-----------|--------|
| Sentry polling | ✅ Every 15s, max 3 concurrent |
| Task claiming | ✅ REST with status filter |
| Packet parsing | ✅ Two-step JSON unmarshal |
| OpenCode execution | ✅ `opencode run --format json "prompt"` |
| task_run recording | ✅ All fields: model_id, courier, tokens, status |
| ROI RPC call | ✅ calculate_enhanced_task_roi (needs service key) |
| Task status update | ✅ → review on success |
| Dependency unlock | ✅ unlock_dependent_tasks RPC |
| Failure handling | ✅ Fetch-then-update attempts |

### Test Results

```
Task: GLM-5 Test: Say Hello
Status: review
Model: glm-5
Courier: governor
Tokens: 11/265
```

### Binary Size

9.6MB - fits free tier comfortably

---

## PHASE 3: INTELLIGENT ROUTING (NOT YET IMPLEMENTED)

### Current State (Placeholder)

```yaml
runners:
  internal:
    - model_id: glm-5
      tool: opencode
```

Just uses first config entry. No availability checking.

### What's Needed

**Nothing Hardcoded. Everything Swappable.**

Models/tools can appear/disappear anytime. System must adapt.

### Routing Logic (from Python orchestrator.py:639-712)

```
1. Get active models from DB (models table)
   - status = 'active'
   - Not in cooldown (cooldown_expires_at < now or null)
   - Under rate limits

2. Filter by routing_capability
   - 'internal': CLI/API with codebase access
   - 'web': Any runner (courier/browser)
   - 'mcp': MCP-capable only

3. Score each available runner:
   - cost_priority: 0=subscription, 1=free API, 2=paid API
   - success_rate: tasks_completed / (tasks_completed + tasks_failed)
   - task_type fit: from strengths field
   - browser capability bonus for web routing

4. Return best score or None if no runners available

5. FALLBACK CHAIN (example):
   - Web platform over 80% daily? → skip
   - gemini-api rate limited? → skip
   - deepseek-api dead? → skip
   - internal via opencode available? → use it
   - Nothing available? → return None, task waits
```

### Models Table Fields (for routing)

| Field | Purpose |
|-------|---------|
| `status` | active/paused/benched |
| `status_reason` | Why paused |
| `success_rate` | Historical performance |
| `tasks_completed` | Count |
| `tasks_failed` | Count |
| `cooldown_expires_at` | Rate limit cooldown |
| `cost_priority` | 0=subscription, 1=free, 2=paid |

### NEVER Assume

- "glm via opencode is always available" - WRONG
- "opencode will exist tomorrow" - WRONG
- "current model will be best forever" - WRONG

**System must work with whatever models exist in DB at runtime.**

---

## SESSION NOTES

### What Went Wrong (First 4 Hours)
1. Started coding within 2 minutes without reading
2. Made broken changes to dispatcher and db
3. Used wrong patterns (SQL expressions, wrong flags)
4. Wasted time and tokens on broken code
5. Had to roll back everything

### What Went Right (After Homework)
1. Rolled back cleanly
2. Did actual homework (4 hours)
3. Read every relevant file
4. Created implementation patterns doc
5. Clean rewrites (8 minutes)
6. Working test immediately

### The Numbers
- Wrong approach: 2 min coding + 4 hours debugging = disaster
- Right approach: 4 hours homework + 8 min coding = working
- Context used for broken code: 58%
- Context used for working code: 7%

### Key Lesson
**Never code until you understand everything.**
3. Read: orchestrator.py, task_manager.py, contract_runners.py, supervisor.py, maintenance.py, council/*, base_runner.py, vault_manager.py, ROI schema, adapter.ts
4. Created IMPLEMENTATION_PATTERNS.md (useful)
5. This handoff document

### For Next Session
1. **READ THIS DOCUMENT FIRST**
2. Read any additional files needed
3. Plan implementation step by step
4. Test with tiny task, compare to Python
5. **NO CODE until plan approved**

---

## FILES READ THIS SESSION

- `CURRENT_STATE.md`
- `docs/SYSTEM_REFERENCE.md`
- `docs/GO_IRON_STACK.md`
- `docs/core_philosophy.md`
- `docs/WHAT_WHERE.md`
- `docs/supabase-schema/schema_v1.4_roi_enhanced.sql`
- `docs/supabase-schema/014_maintenance_commands.sql`
- `core/orchestrator.py` (full)
- `task_manager.py` (full)
- `runners/contract_runners.py` (full)
- `runners/base_runner.py` (full)
- `agents/supervisor.py`
- `agents/consultant.py`
- `agents/planner.py`
- `agents/maintenance.py`
- `agents/council/__init__.py`
- `agents/council/architect.py`
- `agents/council/security.py`
- `prompts/supervisor.md`
- `prompts/testers.md`
- `prompts/system_researcher.md`
- `vault_manager.py`
- `vibeflow/apps/dashboard/lib/vibepilotAdapter.ts`
- `.env`
- `governor/internal/db/supabase.go`
- `governor/internal/dispatcher/dispatcher.go`
- `governor/pkg/types/types.go`

---

**END OF HANDOFF - Next session starts here**
